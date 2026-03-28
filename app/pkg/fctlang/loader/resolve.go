package loader

import (
	"context"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"os"
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

// Program holds all parsed sources keyed by disk path.
// StdlibPath ("::std") is the only non-path key (stdlib is embedded).
// Imports maps import paths (from lib "..." expressions) to disk path keys.
type Program struct {
	Sources map[string]*parser.Source // disk path → parsed source
	Imports map[string]string         // import path → disk path key
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

// Resolve maps an import path to its disk path key.
// If the import path is already a disk path key, returns it unchanged.
func (p Program) Resolve(importPath string) string {
	if dp, ok := p.Imports[importPath]; ok {
		return dp
	}
	return importPath
}

// Load parses source and resolves all library dependencies, returning a Program.
// key is the disk path (or scratch path) for the source being loaded.
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
	// Embedded libs (no fctFile) never go stale.
	if r.fctFile == "" {
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

type resolvedLib struct {
	src     *parser.Source
	dir     string    // filesystem directory (for go-to-definition)
	fctFile string    // absolute path to .fct file (for mtime check); empty for embedded
	modTime time.Time // mtime of fctFile when parsed
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
	return r.resolveSource(prog.Sources[key])
}

type resolver struct {
	ctx           context.Context
	libDir        string
	gitCacheDir   string
	installedLibs map[string]string
	cache         *LibCache              // cross-invocation cache (optional)
	prog          Program                // output program (shared across recursion)
	visited       map[string]*resolvedLib // within-invocation dedup (keyed by import path)
	stack         map[string]bool         // cycle detection (keyed by import path)
}

// resolveSource resolves all LibExpr nodes in a source's globals,
// writing results into r.prog.Sources (keyed by disk path) and r.prog.Imports.
func (r *resolver) resolveSource(src *parser.Source) error {
	// Collect LibExpr nodes that need resolving
	type libEntry struct {
		varName string
		le      *parser.LibExpr
	}
	var entries []libEntry
	for _, g := range src.Globals {
		if le, ok := g.Value.(*parser.LibExpr); ok {
			// Already resolved if we have an import mapping for this path
			if _, ok := r.prog.Imports[le.Path]; ok {
				continue
			}
			entries = append(entries, libEntry{varName: g.Name, le: le})
		}
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
		// Check if already resolved within this invocation (dedup)
		if rl, ok := r.visited[e.le.Path]; ok {
			results[i] = result{idx: i, rl: rl}
			continue
		}
		// Check cross-invocation cache
		if r.cache != nil {
			if rl, ok := r.cache.get(e.le.Path); ok {
				r.visited[e.le.Path] = rl
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
			rl, err := r.loadLib(le.Path)
			results[idx] = result{idx: idx, rl: rl, err: err}
		}(i, e.le)
	}
	wg.Wait()

	// Apply results and recursively resolve transitive deps
	for i, res := range results {
		if res.err != nil {
			return &parser.SourceError{
				Line:    entries[i].le.Pos.Line,
				Col:     entries[i].le.Pos.Col,
				Message: fmt.Sprintf("could not load library %q: %v", entries[i].le.Path, res.err),
			}
		}
		rl := res.rl
		// Compute disk path key for this library
		importPath := entries[i].le.Path
		diskPath := importPath // fallback: use import path if no fctFile
		if rl.fctFile != "" {
			diskPath = rl.fctFile
		} else if rl.dir != "" {
			name := filepath.Base(rl.dir)
			diskPath = filepath.Join(rl.dir, name+".fct")
		}
		rl.src.Path = diskPath
		r.prog.Sources[diskPath] = rl.src
		r.prog.Imports[importPath] = diskPath

		// Cache within invocation
		r.visited[importPath] = rl
		// Cache across invocations
		if r.cache != nil {
			r.cache.set(entries[i].le.Path, rl)
		}

		// Recursively resolve transitive deps
		if r.stack[entries[i].le.Path] {
			return &parser.SourceError{
				Line:    entries[i].le.Pos.Line,
				Col:     entries[i].le.Pos.Col,
				Message: fmt.Sprintf("circular library dependency: %q", entries[i].le.Path),
			}
		}
		r.stack[entries[i].le.Path] = true
		if err := r.resolveSource(rl.src); err != nil {
			return err
		}
		delete(r.stack, entries[i].le.Path)
	}
	return nil
}

// loadLib resolves and parses a single library .fct file.
func (r *resolver) loadLib(rawPath string) (*resolvedLib, error) {
	if err := validateLibPath(rawPath); err != nil {
		return nil, err
	}
	lp, err := ParseLibPath(rawPath)
	if err != nil {
		return nil, err
	}

	if lp.IsLocal {
		return r.loadLocalLib(rawPath)
	}
	return r.loadRemoteLib(rawPath, lp)
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
		return &resolvedLib{src: src, dir: fsDir, fctFile: fsPath, modTime: info.ModTime()}, nil
	}

	// Fall back to embedded stdlib
	embeddedPath := "libraries/" + rawPath + "/" + base + ".fct"
	if data, err := stdlib.Libraries.ReadFile(embeddedPath); err == nil {
		src, err := parser.Parse(string(data), "", parser.SourceLibrary)
		if err != nil {
			return nil, fmt.Errorf("parse embedded %s: %w", embeddedPath, err)
		}
		src.Path = rawPath
		return &resolvedLib{src: src, dir: ""}, nil
	}

	return nil, fmt.Errorf("library %q not found", rawPath)
}

// loadRemoteLib resolves a remote library (e.g., "github.com/user/repo@tag").
func (r *resolver) loadRemoteLib(rawPath string, lp *LibPath) (*resolvedLib, error) {
	// Check installedLibs overrides first (library authors)
	if r.installedLibs != nil {
		libID := fmt.Sprintf("%s/%s/%s", lp.Host, lp.User, lp.Repo)
		if overrideDir, ok := r.installedLibs[libID]; ok {
			dir := overrideDir
			if lp.SubPath != "" {
				dir = filepath.Join(overrideDir, lp.SubPath)
			}
			base := filepath.Base(dir)
			fctFile := filepath.Join(dir, base+".fct")
			if info, statErr := os.Stat(fctFile); statErr == nil {
				data, err := os.ReadFile(fctFile)
				if err != nil {
					return nil, fmt.Errorf("read %s: %w", fctFile, err)
				}
				src, err := parser.Parse(string(data), "", parser.SourceLibrary)
				if err != nil {
					return nil, fmt.Errorf("parse %s: %w", fctFile, err)
				}
				src.Path = rawPath
				return &resolvedLib{src: src, dir: dir, fctFile: fctFile, modTime: info.ModTime()}, nil
			}
		}
	}

	// Try git cache
	baseDir := filepath.Join(r.gitCacheDir, lp.Host, lp.User, lp.Repo, lp.Ref)
	if lp.SubPath != "" {
		baseDir = filepath.Join(baseDir, lp.SubPath)
	}
	base := filepath.Base(baseDir)
	fctFile := filepath.Join(baseDir, base+".fct")
	if info, statErr := os.Stat(fctFile); statErr == nil {
		data, err := os.ReadFile(fctFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fctFile, err)
		}
		src, err := parser.Parse(string(data), "", parser.SourceLibrary)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", fctFile, err)
		}
		src.Path = rawPath
		return &resolvedLib{src: src, dir: baseDir, fctFile: fctFile, modTime: info.ModTime()}, nil
	}

	// Try git clone
	dir, err := resolveLibPath(r.ctx, r.libDir, r.gitCacheDir, r.installedLibs, rawPath)
	if err != nil {
		return nil, err
	}
	src, err := loadLibraryDir(dir)
	if err != nil {
		// Only attempt git pull for paths inside the git cache dir.
		// Installed lib overrides point into the user's working tree —
		// pulling/recloning those would destroy the project.
		if strings.HasPrefix(dir, r.gitCacheDir+string(filepath.Separator)) {
			if pullErr := pullIfGitRepo(r.ctx, dir); pullErr == nil {
				src, err = loadLibraryDir(dir)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	src.Path = rawPath
	// Stat the resolved file for mtime tracking
	name := filepath.Base(dir)
	resolvedFctFile := filepath.Join(dir, name+".fct")
	var modTime time.Time
	if info, statErr := os.Stat(resolvedFctFile); statErr == nil {
		modTime = info.ModTime()
	}
	return &resolvedLib{src: src, dir: dir, fctFile: resolvedFctFile, modTime: modTime}, nil
}

// pullIfGitRepo checks if dir itself is a git repo and runs git pull.
// It does NOT walk up the directory tree — doing so could find an unrelated
// parent repo (e.g., the user's project) and destroy it during reclone.
func pullIfGitRepo(ctx context.Context, dir string) error {
	if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
		return PullRepo(ctx, dir)
	}
	return fmt.Errorf("not a git repo: %s", dir)
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

// loadLibraryDir parses the single .fct file in dir and returns the Source
// with Text populated.
func loadLibraryDir(dir string) (*parser.Source, error) {
	name := filepath.Base(dir)
	file := filepath.Join(dir, name+".fct")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("expected %s.fct in %s", name, dir)
	}
	src, err := parser.Parse(string(data), "", parser.SourceLibrary)
	if err != nil {
		return nil, fmt.Errorf("%s.fct: %w", name, err)
	}
	return src, nil
}
