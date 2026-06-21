package checker

import (
	"facet/pkg/fctlang/parser"
	"strings"
	"testing"
)

// checkWithLib type-checks mainSrc against a single in-memory library whose
// source is libSrc, imported as `github.com/x/lib@main`.
func checkWithLib(t *testing.T, mainSrc, libSrc string) *Result {
	t.Helper()
	prog := parseTestProg(t, mainSrc)
	libKey := "/test/libs/github.com/x/lib"
	parsed, err := parser.Parse(libSrc, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse lib source: %v", err)
	}
	prog.Sources[libKey] = parsed
	prog.Imports["github.com/x/lib@main"] = libKey
	return Check(prog)
}

func hasCheckErr(res *Result, substr string) bool {
	for _, e := range res.Errors {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

// A library `const` is importable via `Lib.NAME`.
func TestCheckLibraryConstImportOK(t *testing.T) {
	res := checkWithLib(t,
		`var L = lib "github.com/x/lib@main"
fn Main() Solid { return Cube(s: Vec3{x: L.GRID, y: L.GRID, z: L.GRID}) }`,
		`const GRID = 42 mm`)
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected checker errors: %v", res.Errors)
	}
}

// A library `var` is NOT importable — only immutable consts cross the boundary.
func TestCheckLibraryVarImportRejected(t *testing.T) {
	res := checkWithLib(t,
		`var L = lib "github.com/x/lib@main"
fn Main() Solid { return Cube(s: Vec3{x: L.COUNT, y: 1 mm, z: 1 mm}) }`,
		`var COUNT = 5 mm`)
	if !hasCheckErr(res, "only const globals are importable") {
		t.Fatalf("expected const-only error, got: %v", res.Errors)
	}
}

// Accessing a name the library does not export is an error.
func TestCheckLibraryMissingConstRejected(t *testing.T) {
	res := checkWithLib(t,
		`var L = lib "github.com/x/lib@main"
fn Main() Solid { return Cube(s: Vec3{x: L.NOPE, y: 1 mm, z: 1 mm}) }`,
		`const GRID = 42 mm`)
	if !hasCheckErr(res, "no exported const") {
		t.Fatalf("expected missing-const error, got: %v", res.Errors)
	}
}
