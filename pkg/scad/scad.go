// Package scad transpiles OpenSCAD source into idiomatic Facet source.
package scad

import (
	"fmt"

	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/parser"
	"facet/pkg/scad/emit"
	scadparser "facet/pkg/scad/parser"
)

// Result is the outcome of a successful transpile.
type Result struct {
	Facet string // the formatted .fct source
}

// Transpile converts OpenSCAD source into formatted Facet source. If any
// construct cannot be faithfully translated, it returns no output and an error
// listing every untranslatable construct with its source location — never a
// placeholder. The emitted text is round-tripped through Facet's parser +
// formatter, so a malformed emission is also an error rather than junk output.
func Transpile(src, path string) (Result, error) {
	file, err := scadparser.Parse(src)
	if err != nil {
		return Result{}, fmt.Errorf("scad parse %s: %w", path, err)
	}
	facetText, errs := emit.File(file)
	if err := emit.ErrorList(path, errs); err != nil {
		return Result{}, err
	}
	return reformat(facetText, path)
}

// reformat parses Facet text and returns it canonically formatted. It is the
// final stage of every transpile.
func reformat(facetText, path string) (Result, error) {
	srcAST, err := parser.Parse(facetText, path, parser.SourceUser)
	if err != nil {
		return Result{}, fmt.Errorf("scad: emitted invalid Facet: %w\n--- emitted ---\n%s", err, facetText)
	}
	return Result{Facet: formatter.Format(srcAST)}, nil
}
