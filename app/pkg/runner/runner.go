package runner

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

// Config holds injected dependencies that the runner cannot own directly
// (they belong to the application layer).
type Config struct {
	LibDir      string
	ResolveOpts func() *loader.Options // called per-build to get current settings
}

// ProgramRunner owns the build pipeline and checker cache.
type ProgramRunner struct {
	ctx       context.Context
	callbacks Callbacks
	libCache  *loader.LibCache
	config    Config

	// Build state — protected by mu
	mu           sync.Mutex
	cancelBuild  context.CancelFunc
	buildDone    chan struct{} // closed when current build goroutine finishes
	lastResult   *RunResult
	buildPending bool
	resultNotify chan struct{} // closed when a new result arrives; replaced on each notify

	// Persistent program state — protected by progMu
	progMu sync.Mutex
	prog   loader.Program // zero until first UpdateSource
}

// Prog returns the current program state, or nil if not yet loaded.
func (r *ProgramRunner) Prog() loader.Program {
	r.progMu.Lock()
	defer r.progMu.Unlock()
	return r.prog
}

// New creates a runner with its own library cache.
func New(ctx context.Context, cb Callbacks, cfg Config) *ProgramRunner {
	return &ProgramRunner{
		ctx:          ctx,
		callbacks:    cb,
		config:       cfg,
		libCache:     loader.NewLibCache(),
		resultNotify: make(chan struct{}),
	}
}

// UpdateSource parses the given source, updates the program, re-checks,
// and pushes a check-only result (no eval). Debounces 500ms.
func (r *ProgramRunner) UpdateSource(key string, source string) {
	r.startBuild(func(ctx context.Context) {
		r.doUpdateSource(ctx, key, source)
	})
}

// Run triggers an immediate evaluation from prog[key] with the given entry point.
// No debounce.
func (r *ProgramRunner) Run(key string, entryPoint string, overrides map[string]interface{}) {
	r.progMu.Lock()
	prog := r.prog
	r.progMu.Unlock()

	if prog.Sources == nil {
		return
	}

	r.startBuildImmediate(func(ctx context.Context) {
		r.doRun(ctx, prog, key, entryPoint, overrides, false)
	})
}

// Debug triggers an immediate debug evaluation from prog[key] with the given entry point.
// No debounce.
func (r *ProgramRunner) Debug(key string, entryPoint string, overrides map[string]interface{}) {
	r.progMu.Lock()
	prog := r.prog
	r.progMu.Unlock()

	if prog.Sources == nil {
		return
	}

	r.startBuildImmediate(func(ctx context.Context) {
		r.doRun(ctx, prog, key, entryPoint, overrides, true)
	})
}

// RekeySource moves a source entry from oldKey to newKey in the program map.
// Used after saving a scratch file to a real path.
func (r *ProgramRunner) RekeySource(oldKey, newKey string) {
	r.progMu.Lock()
	defer r.progMu.Unlock()
	if r.prog.Sources == nil {
		return
	}
	if src, ok := r.prog.Sources[oldKey]; ok {
		src.Path = newKey
		r.prog.Sources[newKey] = src
		delete(r.prog.Sources, oldKey)
	}
}

// RemoveSource removes a source from the program if it is not referenced
// by any other source's lib imports. Triggers a re-check and pushes a new result.
func (r *ProgramRunner) RemoveSource(key string) {
	r.progMu.Lock()
	if r.prog.Sources == nil {
		r.progMu.Unlock()
		return
	}
	// Check if any other source references this key via a LibExpr
	referenced := false
	for srcKey, src := range r.prog.Sources {
		if srcKey == key {
			continue
		}
		for _, g := range src.Globals {
			if le, ok := g.Value.(*parser.LibExpr); ok {
				if diskPath, ok := r.prog.Imports[le.Path]; ok && diskPath == key {
					referenced = true
					break
				}
			}
		}
		if referenced {
			break
		}
	}
	if !referenced {
		delete(r.prog.Sources, key)
		for importPath, diskPath := range r.prog.Imports {
			if diskPath == key {
				delete(r.prog.Imports, importPath)
			}
		}
	}
	r.progMu.Unlock()
}

// Reset cancels any in-flight build and clears all state.
func (r *ProgramRunner) Reset() {
	r.Stop()
	r.progMu.Lock()
	r.prog = loader.Program{}
	r.progMu.Unlock()
	r.libCache.Clear()
	r.mu.Lock()
	r.lastResult = nil
	r.mu.Unlock()
}

// Stop cancels the current build without starting a new one.
func (r *ProgramRunner) Stop() {
	r.mu.Lock()
	if r.cancelBuild != nil {
		r.cancelBuild()
	}
	r.mu.Unlock()
}

// --- Build orchestration ---

// startBuild cancels any in-flight build, debounces 500ms, then runs buildFn.
func (r *ProgramRunner) startBuild(buildFn func(ctx context.Context)) {
	r.mu.Lock()
	if r.cancelBuild != nil {
		r.cancelBuild()
	}
	prevDone := r.buildDone

	ctx, cancel := context.WithCancel(context.Background())
	r.cancelBuild = cancel
	r.buildPending = true
	done := make(chan struct{})
	r.buildDone = done
	r.mu.Unlock()

	go func() {
		defer close(done)

		if prevDone != nil {
			<-prevDone
		}

		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return
		}

		buildFn(ctx)
	}()
}

// startBuildImmediate cancels any in-flight build, then runs buildFn without debounce.
func (r *ProgramRunner) startBuildImmediate(buildFn func(ctx context.Context)) {
	r.mu.Lock()
	if r.cancelBuild != nil {
		r.cancelBuild()
	}
	prevDone := r.buildDone

	ctx, cancel := context.WithCancel(context.Background())
	r.cancelBuild = cancel
	r.buildPending = true
	done := make(chan struct{})
	r.buildDone = done
	r.mu.Unlock()

	go func() {
		defer close(done)

		if prevDone != nil {
			<-prevDone
		}

		if ctx.Err() != nil {
			return
		}

		buildFn(ctx)
	}()
}

// --- UpdateSource implementation ---

func (r *ProgramRunner) doUpdateSource(ctx context.Context, key string, source string) {
	defer func() {
		if r.callbacks.OnIdle != nil {
			r.callbacks.OnIdle()
		}
		runtime.GC()
	}()

	if r.callbacks.OnStart != nil {
		r.callbacks.OnStart()
	}

	// Parse the new source
	kind := parser.SourceUser
	if strings.HasPrefix(key, "example:") {
		kind = parser.SourceExample
	}
	src, parseErr := parser.Parse(source, "", kind)
	if parseErr != nil {
		var se *parser.SourceError
		var errs []parser.SourceError
		if errors.As(parseErr, &se) {
			errs = []parser.SourceError{*se}
		} else {
			errs = []parser.SourceError{{Message: parseErr.Error()}}
		}
		result := &RunResult{Errors: errs}
		r.pushResult(result)
		return
	}

	r.progMu.Lock()

	// First call: initialize with full loader.Load
	if r.prog.Sources == nil {
		r.progMu.Unlock()

		prog, err := loader.Load(ctx, source, key, kind, r.config.LibDir, r.resolveOpts())
		if err != nil {
			result := &RunResult{Errors: []parser.SourceError{SourceErrorFromErr(err)}}
			r.pushResult(result)
			return
		}

		r.progMu.Lock()
		r.prog = prog
		r.progMu.Unlock()
	} else {
		src.Path = key
		src.Text = source
		r.prog.Sources[key] = src

		// Resolve any new library imports
		r.progMu.Unlock()
		if err := loader.ResolveLibraries(ctx, r.prog, key, r.config.LibDir, r.resolveOpts()); err != nil {
			result := &RunResult{Errors: []parser.SourceError{SourceErrorFromErr(err)}}
			r.pushResult(result)
			return
		}

		// Prune orphaned libraries: rebuild Imports from current LibExpr nodes,
		// keeping resolved disk paths, then remove Sources not referenced.
		r.progMu.Lock()
		activeImports := make(map[string]string)
		for _, s := range r.prog.Sources {
			for _, g := range s.Globals {
				if le, ok := g.Value.(*parser.LibExpr); ok {
					if diskPath, resolved := r.prog.Imports[le.Path]; resolved {
						activeImports[le.Path] = diskPath
					}
				}
			}
		}
		r.prog.Imports = activeImports
		usedPaths := map[string]bool{key: true, loader.StdlibPath: true}
		for _, diskPath := range activeImports {
			usedPaths[diskPath] = true
		}
		for srcKey, s := range r.prog.Sources {
			if !usedPaths[srcKey] && s.Kind != parser.SourceUser {
				delete(r.prog.Sources, srcKey)
			}
		}
		r.progMu.Unlock()
	}

	if ctx.Err() != nil {
		return
	}

	// Type-check
	r.progMu.Lock()
	prog := r.prog
	r.progMu.Unlock()

	checked := checker.Check(prog)

	// Build doc index from active source
	docText := ""
	if docSrc := prog.Sources[key]; docSrc != nil {
		docText = docSrc.Text
	}

	result := &RunResult{
		Errors:       checked.Errors,
		VarTypes:     checked.VarTypes,
		Declarations: checked.Declarations,
		DocIndex:     r.buildDocIndex(docText),
	}
	if checked.Prog.Sources != nil {
		result.EntryPoints = GetEntryPoints(checked.Prog, checked.InferredReturnTypes)
	}

	// Push check-only result (no eval)
	r.pushResult(result)
}

// --- Run/Debug implementation ---

func (r *ProgramRunner) doRun(ctx context.Context, prog loader.Program, key string, entryPoint string, overrides map[string]interface{}, debug bool) {
	defer func() {
		if r.callbacks.OnIdle != nil {
			r.callbacks.OnIdle()
		}
		runtime.GC()
	}()

	if r.callbacks.OnStart != nil {
		r.callbacks.OnStart()
	}

	start := time.Now()

	// Re-check to get fresh result metadata
	checked := checker.Check(prog)

	docText := ""
	if docSrc := prog.Sources[key]; docSrc != nil {
		docText = docSrc.Text
	}

	result := &RunResult{
		Errors:       checked.Errors,
		VarTypes:     checked.VarTypes,
		Declarations: checked.Declarations,
		DocIndex:     r.buildDocIndex(docText),
	}
	if checked.Prog.Sources != nil {
		result.EntryPoints = GetEntryPoints(checked.Prog, checked.InferredReturnTypes)
	}

	if len(checked.Errors) > 0 || entryPoint == "" {
		result.Success = false
		r.pushResult(result)
		return
	}

	if debug {
		evalResult, err := evaluator.EvalDebug(ctx, prog, key, overrides, entryPoint)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			result.Success = false
			result.Errors = append(result.Errors, SourceErrorFromErr(err))
			r.pushResult(result)
			return
		}
		evalResult.Final = RenderMeshes(evalResult.Solids)
		result.Success = true
		result.Boxes = SolidBBoxes(evalResult.Solids)
		result.Time = time.Since(start).Seconds()
		result.Solids = evalResult.Solids
		result.DebugResult = evalResult
		r.pushResult(result)
		return
	}

	evalResult, err := evaluator.Eval(ctx, prog, key, overrides, entryPoint)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		result.Success = false
		result.Errors = append(result.Errors, SourceErrorFromErr(err))
		r.pushResult(result)
		return
	}

	merged := manifold.MergeExtractDisplayMeshes(evalResult.Solids)
	stats := evalResult.Stats
	stats.Triangles += merged.IndexCount / 3
	stats.Vertices += merged.VertexCount
	if len(evalResult.Solids) > 0 {
		stats.BBoxMin = [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
		stats.BBoxMax = [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
		for _, s := range evalResult.Solids {
			mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
			stats.BBoxMin[0] = math.Min(stats.BBoxMin[0], mnX)
			stats.BBoxMin[1] = math.Min(stats.BBoxMin[1], mnY)
			stats.BBoxMin[2] = math.Min(stats.BBoxMin[2], mnZ)
			stats.BBoxMax[0] = math.Max(stats.BBoxMax[0], mxX)
			stats.BBoxMax[1] = math.Max(stats.BBoxMax[1], mxY)
			stats.BBoxMax[2] = math.Max(stats.BBoxMax[2], mxZ)
		}
	}

	result.Success = true
	result.Stats = &stats
	result.Boxes = SolidBBoxes(evalResult.Solids)
	result.Time = time.Since(start).Seconds()
	result.Mesh = merged
	result.PosMap = evalResult.PosMap
	result.Solids = evalResult.Solids
	r.pushResult(result)
}

func (r *ProgramRunner) pushResult(result *RunResult) {
	// Populate Sources from the program's parsed sources
	r.progMu.Lock()
	if r.prog.Sources != nil {
		sources := make(map[string]SourceEntry)
		for key, src := range r.prog.Sources {
			sources[key] = SourceEntry{Text: src.Text, Kind: src.Kind}
		}
		result.Sources = sources
	}
	r.progMu.Unlock()

	if r.callbacks.OnResult != nil {
		r.callbacks.OnResult(result)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastResult = result
	r.buildPending = false
	close(r.resultNotify)
	r.resultNotify = make(chan struct{})
}

// WaitForResult blocks until a build result is available.
func (r *ProgramRunner) WaitForResult(ctx context.Context) *RunResult {
	r.mu.Lock()
	if !r.buildPending && r.lastResult != nil {
		result := r.lastResult
		r.mu.Unlock()
		return result
	}
	ch := r.resultNotify
	r.mu.Unlock()

	select {
	case <-ch:
		r.mu.Lock()
		result := r.lastResult
		r.mu.Unlock()
		return result
	case <-ctx.Done():
		return nil
	}
}

// LastResult returns the most recent build result, or nil.
func (r *ProgramRunner) LastResult() *RunResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastResult
}

// --- Internal helpers ---

func (r *ProgramRunner) resolveOpts() *loader.Options {
	opts := r.config.ResolveOpts()
	if opts == nil {
		opts = &loader.Options{}
	}
	opts.Cache = r.libCache
	return opts
}

func (r *ProgramRunner) buildDocIndex(source string) []doc.DocEntry {
	entries := doc.BuildDocIndex(source, nil)
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.Name+"|"+e.Library] = true
	}
	for _, dir := range []string{r.config.LibDir, loader.DefaultGitCacheDir()} {
		for _, e := range doc.BuildLibDocEntries(dir) {
			key := e.Name + "|" + e.Library
			if !seen[key] {
				seen[key] = true
				entries = append(entries, e)
			}
		}
	}
	return entries
}
