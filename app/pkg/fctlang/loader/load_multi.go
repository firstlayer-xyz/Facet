package loader

import (
	"context"
	"strings"

	"facet/app/pkg/fctlang/parser"
	"facet/app/stdlib"
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

	// Resolve library imports from all user sources
	for path := range sources {
		if err := ResolveLibraries(ctx, prog, path, libDir, opts); err != nil {
			return Program{}, err
		}
	}
	return prog, nil
}
