package loader

import (
	"context"
	"facet/pkg/fctlang/parser"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"facet/share/stdlib"
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

// LibPathToNamespace maps a raw `lib "..."` import path to the
// canonical namespace string that identifies the library at the type
// and documentation level: `host/user/repo[/subpath]` for remote refs,
// or the raw path for local libraries.
//
// The `@ref` portion is *intentionally* dropped — it controls which
// tree the loader resolves to, but it isn't part of the library's
// identity. Both the checker (when typing a `var T = lib "..."`
// binding) and the doc layer use this shape so the editor's
// completion can match `Library:<namespace>` against
// `DocEntry.Library`. Returns the raw path on a parse failure or for
// local libraries.
func LibPathToNamespace(rawPath string) string {
	lp, err := ParseLibPath(rawPath)
	if err != nil {
		return rawPath
	}
	if lp.IsLocal {
		return rawPath
	}
	ns := lp.Host + "/" + lp.User + "/" + lp.Repo
	if lp.SubPath != "" {
		ns = ns + "/" + strings.Trim(lp.SubPath, "/")
	}
	return ns
}

// Resolve maps an import path to its Sources key.
// If the import path is already a canonical key, returns it unchanged.
func (p Program) Resolve(importPath string) string {
	if dp, ok := p.Imports[importPath]; ok {
		return dp
	}
	return importPath
}

// IsLibrarySource reports whether srcKey is the resolved canonical key of a
// library imported via a LibExpr anywhere in the program.
func (p Program) IsLibrarySource(srcKey string) bool {
	for _, src := range p.Sources {
		for _, g := range src.LibImports() {
			le := g.Value.(*parser.LibExpr)
			if p.Resolve(le.Key()) == srcKey {
				return true
			}
		}
	}
	return false
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
	Cache         *Cache            // backing storage for the bare-clone cache; nil = NativeCache()

	// RemoteFetch, when non-nil, replaces the go-git bare clone for remote
	// libraries with on-demand HTTP fetches. It resolves a tree-relative
	// subPath (e.g. "fasteners/fasteners.fct") within the given lib's repo to
	// file bytes. This exists for the browser/wasm build, where go-git's git
	// smart-HTTP clone is blocked by CORS: the wasm build supplies a fetcher
	// that maps github.com libs to a CORS-friendly mirror (jsDelivr). Native
	// builds leave this nil and clone as before.
	RemoteFetch func(lp *LibPath, subPath string) ([]byte, error)
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
		r.remoteFetch = opts.RemoteFetch
	}
	if r.gitCacheDir == "" {
		r.gitCacheDir = DefaultGitCacheDir()
	}
	if r.cache == nil {
		r.cache = NativeCache()
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
	cache         *Cache                  // bare-clone storage backend (defaults to NativeCache)
	remoteFetch   func(lp *LibPath, subPath string) ([]byte, error) // non-nil = HTTP-fetch remote libs (wasm); nil = git clone
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
	// Virtual (git) and HTTP (jsDelivr) trees both resolve relative imports by
	// reading sibling paths through the same tree.
	if rl.tree.IsVirtual() || rl.tree.IsHTTP() {
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
		_, subPath, err := virtualRelTarget(pctx, lp.SubPath)
		if err != nil {
			return err
		}
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

// virtualRelTarget maps a virtual relative import to its tree-relative directory
// and .fct path, applying the convention that a module named X lives at X/X.fct.
func virtualRelTarget(pctx parentCtx, sub string) (subDir, subPath string, err error) {
	subDir, err = resolveRelativeVirtual(pctx, sub)
	if err != nil {
		return "", "", err
	}
	name := path.Base(subDir)
	return subDir, subDir + "/" + name + ".fct", nil
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
		subDir, subPath, err := virtualRelTarget(pctx, lp.SubPath)
		if err != nil {
			return nil, err
		}
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
//  2. RemoteFetch hook (browser/wasm: HTTP fetch via a CORS-friendly mirror)
//  3. Virtual tree backed by the shared bare clone (read via go-git)
func (r *resolver) loadRemoteLib(rawPath string, lp *LibPath) (*resolvedLib, error) {
	if r.installedLibs != nil {
		if overrideDir, ok := r.installedLibs[lp.RepoID()]; ok {
			return r.loadFromOverride(rawPath, lp, overrideDir)
		}
	}

	var tree *LibTree
	var err error
	if r.remoteFetch != nil {
		// Browser/wasm path: no git clone (CORS-blocked). Back the tree with
		// on-demand HTTP fetches; relative imports read through the same tree.
		lpCopy := *lp
		tree = HTTPTree(&lpCopy, func(subPath string) ([]byte, error) {
			return r.remoteFetch(&lpCopy, subPath)
		})
	} else {
		tree, err = ensureLib(r.ctx, r.cache, r.gitCacheDir, lp, false /*forceFetch*/)
	}
	if err != nil {
		return nil, err
	}
	subDir := strings.Trim(lp.SubPath, "/")
	// A module named X lives at X/X.fct; at the repo root the module name
	// defaults to the repo name.
	var subPath string
	if subDir == "" {
		subPath = lp.Repo + ".fct"
	} else {
		subPath = subDir + "/" + path.Base(subDir) + ".fct"
	}
	data, err := tree.ReadFile(subPath)
	if err != nil {
		// Common case: the user imported a bare repo path
		// ("github.com/user/repo@ref") for a repo that's actually a
		// meta-collection of modules in subdirectories — no top-level
		// <repo>.fct exists. Walk the tree to enumerate available
		// module names so the error can name them. Limited to the
		// bare-path case (subDir == "") because that's where the
		// failure mode is opaque; with a subdir the user already
		// scoped to a directory that simply doesn't contain the file.
		if subDir == "" {
			if modules := listLibraryModules(tree); len(modules) > 0 {
				return nil, fmt.Errorf(
					"remote lib %q: no top-level %s — did you mean one of: %s? "+
						"(append /<module> to the lib path, e.g. \"%s/%s%s\")",
					rawPath, subPath, strings.Join(modules, ", "),
					strings.SplitN(rawPath, "@", 2)[0], modules[0],
					func() string {
						if i := strings.Index(rawPath, "@"); i >= 0 {
							return rawPath[i:]
						}
						return ""
					}(),
				)
			}
		}
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

// ListCachedRepoModules returns the names of top-level library modules
// (directories `<name>/` containing `<name>.fct`) in a cached bare
// clone identified by `repoID` of the form `host/user/repo`. Offline
// only — does not fetch. Empty result if the repo isn't cached or
// has no top-level module folders. Used by the editor's `lib "..."`
// completion to suggest subpaths after the user types the repo URL
// and a trailing slash.
func ListCachedRepoModules(gitCacheDir, repoID string) []string {
	parts := strings.SplitN(strings.Trim(repoID, "/"), "/", 3)
	if len(parts) != 3 {
		return nil
	}
	sharedDir := filepath.Join(gitCacheDir, parts[0], parts[1], parts[2], sharedRepoName)
	repo, err := openCachedRepo(NativeCache(), sharedDir)
	if err != nil {
		return nil
	}
	sha, err := resolveRepoHead(repo)
	if err != nil {
		return nil
	}
	tree := &LibTree{repo: repo, sha: sha, origin: repoID}
	return listLibraryModules(tree)
}

// listLibraryModules walks a tree and returns names of top-level library
// modules — directories `<name>/` that contain `<name>.fct`. Used to
// build an actionable error when a user imports a repo's bare path
// for a repo that's a meta-collection of submodules instead of a
// single library. Sorted, deduplicated, and capped to keep error
// messages readable. Best-effort: any Walk error returns nil.
func listLibraryModules(tree *LibTree) []string {
	seen := make(map[string]bool)
	_ = tree.Walk(func(subPath string, _ io.Reader) error {
		// Match top-level `name/name.fct`.
		parts := strings.Split(subPath, "/")
		if len(parts) != 2 {
			return nil
		}
		dir, file := parts[0], parts[1]
		if !strings.HasSuffix(file, ".fct") {
			return nil
		}
		base := strings.TrimSuffix(file, ".fct")
		if base != dir {
			return nil
		}
		seen[dir] = true
		return nil
	})
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	const maxModules = 12
	if len(out) > maxModules {
		out = out[:maxModules]
	}
	return out
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
func validateLibPath(p string) error {
	// Lib paths are URL-style (slash-separated) regardless of the host
	// OS, so use the slash-aware check from `path` rather than
	// filepath.IsAbs, which on Windows returns false for "/etc/foo" and
	// would let an absolute Unix path through.
	if path.IsAbs(p) || (len(p) >= 2 && p[1] == ':') {
		return fmt.Errorf("library path %q: absolute paths are not allowed", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return fmt.Errorf("library path %q: '..' is not allowed", p)
		}
	}
	if !strings.Contains(p, "/") {
		return fmt.Errorf("library path %q: must have at least 2 segments (e.g. \"vendor/name\")", p)
	}
	return nil
}
