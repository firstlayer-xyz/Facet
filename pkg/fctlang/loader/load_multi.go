package loader

import (
	"context"
	"strings"

	"facet/pkg/fctlang/parser"
	"facet/share/stdlib"
)

// LoadMulti parses multiple user-provided sources and resolves all library dependencies.
// sources maps disk paths to source text. key identifies the primary source file
// (the one containing the entry point).
func LoadMulti(ctx context.Context, sources map[string]string, key string, libDir string, opts *Options) (Program, error) {
	prog := NewProgram()

	// Parse and add all user sources (preserve raw text on Source for frontend display).
	//
	// A resolved library source can be echoed back here keyed by its virtual
	// "git+..." backing when the editor has it open as a view-only tab. It is not
	// a user project root: the user file that imports it loads the canonical copy
	// from the cache, and its relative imports ("../sibling") only resolve within
	// the git tree it was read from, which is not reconstructable from this raw
	// text. Skip it entirely so it never enters prog.Sources — otherwise the
	// checker walks it and flags its now-unresolved imports, blocking the render.
	for path, text := range sources {
		if IsVirtualSourceKey(path) {
			continue
		}
		kind := parser.SourceUser
		if strings.HasPrefix(path, "example:") {
			kind = parser.SourceExample
		}
		src, err := parser.Parse(text, "", kind)
		if err != nil {
			return Program{}, err
		}
		src.Path = path
		src.Text = text
		prog.Sources[path] = src
	}

	// Parse and add stdlib
	stdSrc, err := parser.Parse(stdlib.StdlibSource, "", parser.SourceStdLib)
	if err != nil {
		return Program{}, err
	}
	stdSrc.Path = StdlibPath
	stdSrc.Text = stdlib.StdlibSource
	prog.Sources[StdlibPath] = stdSrc

	// Resolve library imports from every user source. Range the input map, not
	// prog.Sources — ResolveLibraries adds resolved libraries to prog.Sources as
	// it runs, and mutating a map mid-range is unsafe. Virtual keys are skipped
	// for the same reason the first loop drops them: they are not roots.
	for path := range sources {
		if IsVirtualSourceKey(path) {
			continue
		}
		if err := ResolveLibraries(ctx, prog, path, libDir, opts); err != nil {
			return Program{}, err
		}
	}
	return prog, nil
}
