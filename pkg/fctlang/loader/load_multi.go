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

	// Parse and add all user sources (preserve raw text on Source for frontend display)
	for path, text := range sources {
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

	// Resolve library imports from all user sources.
	//
	// A resolved remote-library source can appear here keyed by its virtual
	// "git+..." backing when the editor has it open as a view-only tab and
	// echoes it back in the sources map. Such a key is not a user project root:
	// its relative imports ("../sibling") only resolve within the git tree it
	// was read from, which is not reconstructable from the raw text here. The
	// user source that imports it resolves it correctly from the cache, so skip
	// it as a root — resolving it standalone would fail its relative imports.
	for path := range sources {
		if strings.HasPrefix(path, LibSourceScheme) {
			continue
		}
		if err := ResolveLibraries(ctx, prog, path, libDir, opts); err != nil {
			return Program{}, err
		}
	}
	return prog, nil
}
