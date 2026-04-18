package loader

import (
	"context"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"facet/app/stdlib"
)

// ---------------------------------------------------------------------------
// Program — the top-level result of Load
// ---------------------------------------------------------------------------

const (
	// StdlibPath is the well-known key for the standard library.
	// This is the only non-filesystem key in a Program.
	StdlibPath = "::std"
)

// Program holds all parsed sources keyed by their canonical identifier —
// either an absolute disk path (main source, overrides, local libs) or a
// virtual-tree URI (cached remote libs; see gitfetch.LibSourceScheme).
// StdlibPath ("::std") is the only non-path key (stdlib is embedded).
// Imports maps import paths (from lib "..." expressions) to Sources keys.
type Program struct {
	Sources map[string]*parser.Source // canonical key → parsed source
	Imports map[string]string         // import path → canonical key
}

// NewProgram creates an empty Program.
func NewProgram() Program {
	return Program{
		Sources: make(map[string]*parser.Source),
		Imports: make(map[string]string),
	}
}

// Std returns the standard library source, or nil if not set.
func (p Program) Std() *parser.Source { return p.Sources[StdlibPath] }

// Resolve maps an import path to its Sources key.
// If the import path is already a canonical key, returns it unchanged.
func (p Program) Resolve(importPath string) string {
	if dp, ok := p.Imports[importPath]; ok {
		return dp
	}
	return importPath
}

// Load parses source and resolves all library dependencies, returning a Program.
// key is the canonical identifier for the source being loaded (usually a disk
// path for a main source; scratch/virtual for non-file sources).
// The standard library is automatically parsed and added under StdlibPath.
func Load(ctx context.Context, source string, key string, kind parser.SourceKind, libDir string, opts *Options) (Program, error) {
	src, err := parser.Parse(source, "", kind)
	if err != nil {
		return Program{}, err
	}
	src.Path = key

	stdSrc, err := parser.Parse(stdlib.StdlibSource, "", parser.SourceStdLib)
	if err != nil {
		return Program{}, fmt.Errorf("stdlib: %w", err)
	}
	stdSrc.Path = StdlibPath

	prog := NewProgram()
	prog.Sources[key] = src
	prog.Sources[StdlibPath] = stdSrc

	if err := ResolveLibraries(ctx, prog, key, libDir, opts); err != nil {
		return Program{}, err
	}
	return prog, nil
}

// ---------------------------------------------------------------------------
// Library resolution
// ---------------------------------------------------------------------------

// LibCache caches parsed library programs across invocations.
// Not thread-safe — callers must serialize access (the runner's build pipeline does this).
// Create one at app startup and pass it via Options.
type LibCache struct {
	cache map[string]*resolvedLib
}

// NewLibCache creates a new empty library cache.
func NewLibCache() *LibCache {
	return &LibCache{cache: make(map[string]*resolvedLib)}
}

// Clear removes all cached entries. Call when a library is installed or updated.
func (lc *LibCache) Clear() {
	lc.cache = make(map[string]*resolvedLib)
}

func (lc *LibCache) get(path string) (*resolvedLib, bool) {
	r, ok := lc.cache[path]
	if !ok {
		return nil, false
	}
	// Embedded / virtualized libs (no fctFile on disk) never go stale
	// through mtime — virtual keys are content-addressed by SHA.
	if r.fctFile == "" || !filepath.IsAbs(r.fctFile) {
		return r, true
	}
	// Check if the file has been modified since we cached it.
	info, err := os.Stat(r.fctFile)
	if err != nil || info == nil || info.ModTime().After(r.modTime) {
		// File changed or deleted — evict and miss.
		delete(lc.cache, path)
		return nil, false
	}
	return r, true
}

func (lc *LibCache) set(path string, r *resolvedLib) {
	lc.cache[path] = r
}

// resolvedLib is a successfully-loaded library plus the context needed to
// resolve its own relative imports. One of tree/diskDir is set; root is the
// containment boundary (empty for contexts that don't allow relative imports,
// e.g. embedded stdlib or main sources loaded under a non-absolute key).
type resolvedLib struct {
	src *parser.Source

	// tree is the library's source backing — virtual (go-git) or physical
	// (disk). Nil only for embedded stdlib libs (no relative imports there).
	tree *LibTree

	// subDir is the tree-relative directory of this lib's .fct file. For a
	// root-level lib this is "" (the root itself); for "github.com/x/y/sub"
	// it's "sub".
	subDir string

	// root is the containment boundary for relative imports issued from
	// this lib. For physical trees it is an absolute filesystem path; for
	// virtual trees it is "" and containment is checked via path.Clean
	// refusing to escape the tree. Empty string on a non-virtual tree
	// means relative imports are disallowed.
	root string

	fctFile string    // canonical key of this lib's .fct — disk path OR URI
	modTime time.Time // mtime for disk files; zero for virtual/embedded
}

// parentCtx describes the importing source's context for relative-import
// resolution. Exactly one backing is active:
//
//   - Physical: dir + root are set (absolute filesystem paths). Relative
//     imports resolve via filepath.Join and are checked with filepath.Rel.
//   - Virtual: tree is set; subDir is the tree-relative directory of the
//     importer. Relative imports resolve via path.Clean and escape-check
//     against the tree root (".." out of the tree is rejected).
//
// A zero-value parentCtx disallows relative imports entirely (e.g. embedded
// stdlib, or main sources loaded under a non-absolute key).
type parentCtx struct {
	// Physical context:
	dir  string
	root string

	// Virtual context:
	tree   *LibTree
	subDir string
}

// allowsRelative reports whether relative imports are permitted from this
// context. Either a physical root or a virtual tree has to be set.
func (p parentCtx) allowsRelative() bool {
	return p.root != "" || p.tree != nil
}

// Options configures library resolution.
type Options struct {
	GitCacheDir   string            // default: DefaultGitCacheDir()
	InstalledLibs map[string]string // libID → local dir overrides
	Cache         *LibCache         // reuse parsed libraries across invocations; nil = no caching
}

// ResolveLibraries walks all LibExpr nodes in prog[key] (and transitively in
// resolved library sources), resolves their paths, parses the library .fct
// files, and populates prog.Sources and prog.Imports.
// Returns an error if any library cannot be loaded.
func ResolveLibraries(ctx context.Context, prog Program, key string, libDir string, opts *Options) error {
	r := &resolver{
		ctx:     ctx,
		libDir:  libDir,
		prog:    prog,
		visited: make(map[string]*resolvedLib),
		stack:   make(map[string]bool),
	}
	if opts != nil {
		r.gitCacheDir = opts.GitCacheDir
		r.installedLibs = opts.InstalledLibs
		r.cache = opts.Cache
	}
	if r.gitCacheDir == "" {
		r.gitCacheDir = DefaultGitCacheDir()
	}
	// Main-source context: relative imports resolve against the source's own
	// directory. The root is the same — main-source relatives cannot escape
	// their own directory. Scratch/virtual keys (not absolute filesystem
	// paths) get an empty root, which disallows relative imports.
	pctx := parentCtx{}
	if filepath.IsAbs(key) {
		dir := filepath.Dir(key)
		pctx = parentCtx{dir: dir, root: dir}
	}
	return r.resolveSource(prog.Sources[key], pctx)
}

type resolver struct {
	ctx           context.Context
	libDir        string
	gitCacheDir   string
	installedLibs map[string]string
	cache         *LibCache               // cross-invocation cache (optional)
	prog          Program                 // output program (shared across recursion)
	visited       map[string]*resolvedLib // within-invocation dedup (keyed by import path)
	stack         map[string]bool         // cycle detection (keyed by import path)
}

// resolveSource resolves all LibExpr nodes in a source's globals,
// writing results into r.prog.Sources (keyed by canonical key) and r.prog.Imports.
// pctx is the containment/resolution context for relative imports in this
// source.
func (r *resolver) resolveSource(src *parser.Source, pctx parentCtx) error {
	// Collect LibExpr nodes that need resolving. For each, we first compute
	// le.Resolved (when applicable) so that the cache / Imports lookups use
	// a stable, collision-free key — two sources both doing `lib "./knurling"`
	// resolve to different canonical paths and thus different Imports entries.
	type libEntry struct {
		varName string
		le      *parser.LibExpr
	}
	var entries []libEntry
	for _, g := range src.Globals() {
		le, ok := g.Value.(*parser.LibExpr)
		if !ok {
			continue
		}
		if err := r.annotateResolved(le, pctx); err != nil {
			return &parser.SourceError{
				Line:    le.Pos.Line,
				Col:     le.Pos.Col,
				Message: fmt.Sprintf("could not resolve library %q: %v", le.Path, err),
			}
		}
		// Already resolved if we have an import mapping for this key
		if _, ok := r.prog.Imports[le.Key()]; ok {
			continue
		}
		entries = append(entries, libEntry{varName: g.Name, le: le})
	}
	if len(entries) == 0 {
		return nil
	}

	// Resolve in parallel for direct deps
	type result struct {
		idx int
		rl  *resolvedLib
		err error
	}
	results := make([]result, len(entries))
	var wg sync.WaitGroup
	for i, e := range entries {
		key := e.le.Key()
		// Check if already resolved within this invocation (dedup)
		if rl, ok := r.visited[key]; ok {
			results[i] = result{idx: i, rl: rl}
			continue
		}
		// Check cross-invocation cache
		if r.cache != nil {
			if rl, ok := r.cache.get(key); ok {
				r.visited[key] = rl
				results[i] = result{idx: i, rl: rl}
				continue
			}
		}

		wg.Add(1)
		go func(idx int, le *parser.LibExpr) {
			defer wg.Done()
			if err := r.ctx.Err(); err != nil {
				results[idx] = result{idx: idx, err: err}
				return
			}
			rl, err := r.loadLib(le, pctx)
			results[idx] = result{idx: idx, rl: rl, err: err}
		}(i, e.le)
	}
	wg.Wait()

	// Apply results and recursively resolve transitive deps
	for i, res := range results {
		le := entries[i].le
		key := le.Key()
		if res.err != nil {
			return &parser.SourceError{
				Line:    le.Pos.Line,
				Col:     le.Pos.Col,
				Message: fmt.Sprintf("could not load library %q: %v", le.Path, res.err),
			}
		}
		rl := res.rl
		canonicalKey := rl.fctFile
		if canonicalKey == "" {
			// Embedded stdlib libs have no fctFile — fall back to the
			// import path as the canonical key.
			canonicalKey = key
		}
		rl.src.Path = canonicalKey
		r.prog.Sources[canonicalKey] = rl.src
		r.prog.Imports[key] = canonicalKey

		// Cache within invocation
		r.visited[key] = rl
		// Cache across invocations
		if r.cache != nil {
			r.cache.set(key, rl)
		}

		// Recursively resolve transitive deps
		if r.stack[key] {
			return &parser.SourceError{
				Line:    le.Pos.Line,
				Col:     le.Pos.Col,
				Message: fmt.Sprintf("circular library dependency: %q", le.Path),
			}
		}
		r.stack[key] = true
		childCtx := buildChildCtx(rl)
		if err := r.resolveSource(rl.src, childCtx); err != nil {
			return err
		}
		delete(r.stack, key)
	}
	return nil
}

// buildChildCtx constructs the parentCtx used when recursing into rl.src to
// resolve its own relative imports. Preserves the virtual/physical distinction.
func buildChildCtx(rl *resolvedLib) parentCtx {
	if rl.tree == nil {
		return parentCtx{}
	}
	if rl.tree.IsVirtual() {
		return parentCtx{tree: rl.tree, subDir: rl.subDir}
	}
	// Physical: dir is the lib's on-disk directory; root is the lib's
	// containment root (same dir for main sources, override/repo root for
	// overrides, etc.).
	dir := rl.tree.DiskDir()
	if rl.subDir != "" {
		dir = filepath.Join(dir, filepath.FromSlash(rl.subDir))
	}
	return parentCtx{dir: dir, root: rl.root}
}

// annotateResolved sets le.Resolved (when applicable) so that downstream cache
// / Imports lookups use a collision-free key. For relative imports this is
// the canonical disk path OR virtual URI of the target .fct; for everything
// else Resolved is left empty and Key() falls back to le.Path.
func (r *resolver) annotateResolved(le *parser.LibExpr, pctx parentCtx) error {
	if le.Resolved != "" {
		return nil
	}
	if !strings.HasPrefix(le.Path, "./") && !strings.HasPrefix(le.Path, "../") {
		return nil
	}
	if !pctx.allowsRelative() {
		return fmt.Errorf("relative imports are not allowed from this source")
	}
	lp, err := ParseLibPath(le.Path)
	if err != nil {
		return err
	}
	if pctx.tree != nil {
		// Virtual parent — resolve within the tree.
		subDir, err := resolveRelativeVirtual(pctx, lp.SubPath)
		if err != nil {
			return err
		}
		name := path.Base(subDir)
		subPath := subDir + "/" + name + ".fct"
		le.Resolved = pctx.tree.SourceKey(subPath)
		return nil
	}
	// Physical parent — resolve on disk.
	abs, err := resolveRelativeDir(pctx, lp.SubPath)
	if err != nil {
		return err
	}
	name := filepath.Base(abs)
	le.Resolved = filepath.Join(abs, name+".fct")
	return nil
}

// resolveRelativeDir joins subPath onto pctx.dir, cleans the result, and
// verifies it stays inside pctx.root. Returns the absolute directory.
func resolveRelativeDir(pctx parentCtx, subPath string) (string, error) {
	abs := filepath.Clean(filepath.Join(pctx.dir, subPath))
	rootClean := filepath.Clean(pctx.root)
	rel, err := filepath.Rel(rootClean, abs)
	if err != nil {
		return "", fmt.Errorf("containment check: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("relative import escapes containment root %q", rootClean)
	}
	return abs, nil
}

// resolveRelativeVirtual resolves subPath against pctx.subDir within a
// virtual tree. Rejects results that escape the tree root. Returns the
// tree-relative directory (slash-separated, no leading slash).
func resolveRelativeVirtual(pctx parentCtx, subPath string) (string, error) {
	// Join parent dir with the relative segment, then normalize. path.Clean
	// collapses leading "./" and absorbs each ".." against a preceding
	// segment; any residual ".." at the start means the result escaped the
	// tree root.
	joined := path.Clean(path.Join(pctx.subDir, subPath))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return "", fmt.Errorf("relative import escapes tree root of %q", pctx.tree.Origin())
	}
	if joined == "." {
		return "", fmt.Errorf("relative import resolves to tree root, not a library")
	}
	return joined, nil
}

// loadLib resolves and parses a single library .fct file.
func (r *resolver) loadLib(le *parser.LibExpr, pctx parentCtx) (*resolvedLib, error) {
	rawPath := le.Path
	lp, err := ParseLibPath(rawPath)
	if err != nil {
		return nil, err
	}

	if lp.IsRelative {
		return r.loadRelativeLib(le, lp, pctx)
	}
	if err := validateLibPath(rawPath); err != nil {
		return nil, err
	}
	if lp.IsLocal {
		return r.loadLocalLib(rawPath)
	}
	return r.loadRemoteLib(rawPath, lp)
}

// loadRelativeLib loads a "./x" / "../x" import. annotateResolved has already
// set le.Resolved to the canonical key (disk path for physical parents, URI
// for virtual parents); this function reads the corresponding .fct through
// whichever backing the parent uses.
func (r *resolver) loadRelativeLib(le *parser.LibExpr, lp *LibPath, pctx parentCtx) (*resolvedLib, error) {
	if !pctx.allowsRelative() {
		return nil, fmt.Errorf("relative imports are not allowed from this source")
	}
	if le.Resolved == "" {
		return nil, fmt.Errorf("relative import %q has no resolved path", le.Path)
	}

	if pctx.tree != nil {
		// Virtual parent — read through the same tree so all relatives in
		// the same repo share the single bare clone.
		subDir, err := resolveRelativeVirtual(pctx, lp.SubPath)
		if err != nil {
			return nil, err
		}
		name := path.Base(subDir)
		subPath := subDir + "/" + name + ".fct"
		data, err := pctx.tree.ReadFile(subPath)
		if err != nil {
			return nil, fmt.Errorf("relative import %q: %w", le.Path, err)
		}
		src, err := parser.Parse(string(data), "", parser.SourceCached)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", le.Resolved, err)
		}
		src.Path = le.Resolved
		return &resolvedLib{
			src:     src,
			tree:    pctx.tree,
			subDir:  subDir,
			fctFile: le.Resolved,
		}, nil
	}

	// Physical parent — read from disk.
	fctFile := le.Resolved
	info, err := os.Stat(fctFile)
	if err != nil {
		return nil, fmt.Errorf("relative import %q: %w", le.Path, err)
	}
	data, err := os.ReadFile(fctFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fctFile, err)
	}
	src, err := parser.Parse(string(data), "", parser.SourceCached)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", fctFile, err)
	}
	src.Path = le.Resolved
	return &resolvedLib{
		src:     src,
		tree:    PhysicalTree(filepath.Dir(fctFile)),
		root:    pctx.root, // inherit parent's containment root
		fctFile: fctFile,
		modTime: info.ModTime(),
	}, nil
}

// loadLocalLib resolves a local/built-in library (e.g., "facet/gears").
func (r *resolver) loadLocalLib(rawPath string) (*resolvedLib, error) {
	base := filepath.Base(rawPath)

	// Check filesystem library dir first (concrete path for navigation)
	fsDir := filepath.Join(r.libDir, rawPath)
	fsPath := filepath.Join(fsDir, base+".fct")
	if info, statErr := os.Stat(fsPath); statErr == nil {
		data, err := os.ReadFile(fsPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fsPath, err)
		}
		src, err := parser.Parse(string(data), "", parser.SourceLibrary)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", fsPath, err)
		}
		src.Path = rawPath
		// Local libs don't support relative imports — root stays empty.
		return &resolvedLib{
			src:     src,
			tree:    PhysicalTree(fsDir),
			fctFile: fsPath,
			modTime: info.ModTime(),
		}, nil
	}

	// Fall back to embedded stdlib
	embeddedPath := "libraries/" + rawPath + "/" + base + ".fct"
	if data, err := stdlib.Libraries.ReadFile(embeddedPath); err == nil {
		src, err := parser.Parse(string(data), "", parser.SourceCached)
		if err != nil {
			return nil, fmt.Errorf("parse embedded %s: %w", embeddedPath, err)
		}
		src.Path = rawPath
		// Embedded libs have no tree and no root; relative imports impossible.
		return &resolvedLib{src: src}, nil
	}

	return nil, fmt.Errorf("library %q not found", rawPath)
}

// loadRemoteLib resolves a remote library (e.g., "github.com/user/repo@tag").
// Resolution order:
//  1. installedLibs overrides (library authors working against a local copy)
//  2. Virtual tree backed by the shared bare clone (read via go-git)
func (r *resolver) loadRemoteLib(rawPath string, lp *LibPath) (*resolvedLib, error) {
	if r.installedLibs != nil {
		if overrideDir, ok := r.installedLibs[lp.RepoID()]; ok {
			return r.loadFromOverride(rawPath, lp, overrideDir)
		}
	}

	tree, err := EnsureLib(r.ctx, r.gitCacheDir, lp)
	if err != nil {
		return nil, err
	}
	subDir := strings.Trim(lp.SubPath, "/")
	name := lp.Repo
	if subDir != "" {
		name = path.Base(subDir)
	}
	subPath := name + ".fct"
	if subDir != "" {
		subPath = subDir + "/" + name + ".fct"
	}
	data, err := tree.ReadFile(subPath)
	if err != nil {
		return nil, fmt.Errorf("remote lib %q: %w", rawPath, err)
	}
	key := tree.SourceKey(subPath)
	src, err := parser.Parse(string(data), "", parser.SourceCached)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}
	src.Path = key
	return &resolvedLib{
		src:     src,
		tree:    tree,
		subDir:  subDir,
		fctFile: key,
	}, nil
}

// loadFromOverride loads a library from a user-specified working tree. Used
// by library authors to develop against an uncommitted copy.
func (r *resolver) loadFromOverride(rawPath string, lp *LibPath, overrideDir string) (*resolvedLib, error) {
	dir := overrideDir
	subDir := strings.Trim(lp.SubPath, "/")
	if subDir != "" {
		dir = filepath.Join(overrideDir, filepath.FromSlash(subDir))
	}
	base := filepath.Base(dir)
	fctFile := filepath.Join(dir, base+".fct")
	info, err := os.Stat(fctFile)
	if err != nil {
		return nil, fmt.Errorf("installed override %q: %w", rawPath, err)
	}
	data, err := os.ReadFile(fctFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fctFile, err)
	}
	src, err := parser.Parse(string(data), "", parser.SourceLibrary)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", fctFile, err)
	}
	src.Path = fctFile
	// Override acts like a local working copy of the remote repo — root is
	// the override dir itself so relative imports stay within it.
	return &resolvedLib{
		src:     src,
		tree:    PhysicalTree(overrideDir),
		subDir:  subDir,
		root:    overrideDir,
		fctFile: fctFile,
		modTime: info.ModTime(),
	}, nil
}

// validateLibPath checks that a library path has at least 2 segments (vendor/name),
// does not contain ".." components, and is not an absolute path.
func validateLibPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("library path %q: absolute paths are not allowed", path)
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			return fmt.Errorf("library path %q: '..' is not allowed", path)
		}
	}
	if !strings.Contains(path, "/") {
		return fmt.Errorf("library path %q: must have at least 2 segments (e.g. \"vendor/name\")", path)
	}
	return nil
}
