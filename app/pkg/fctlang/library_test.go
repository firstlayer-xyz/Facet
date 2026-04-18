package fctlang

import (
	fctchecker "facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/parser"
	"strings"
	"testing"
)

// parseLibTestProg builds a loader.Program with a library seeded into Libs.
// The library source is parsed and placed at the given libPath, and the
// main program source is expected to reference it via lib "libPath".
func parseLibTestProg(t *testing.T, mainSrc, libSrc, libPath string) *fctchecker.Result {
	t.Helper()

	prog := parseTestProg(t, mainSrc)

	lib, err := parser.Parse(libSrc, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse library: %v", err)
	}
	lib.Path = libPath
	lib.Text = libSrc
	prog.Sources[libPath] = lib

	return fctchecker.Check(prog)
}

func TestLibraryAndProgramSameTypeName(t *testing.T) {
	const libSrc = `
type Point {
	x Number
	y Number
}

fn NewPoint(x Number, y Number) Point { return Point{x: x, y: y} }

fn Point.Sum() Number { return self.x + self.y }
`
	const mainSrc = `
var mylib = lib "test/mylib"

type Point {
	a Number
	b Number
	c Number
}

fn Main() Number {
	var lp = mylib.NewPoint(x: 1, y: 2)
	var libSum = lp.Sum()
	var local = Point{a: 10, b: 20, c: 30}
	return libSum + local.a
}
`
	result := parseLibTestProg(t, mainSrc, libSrc, "test/mylib")

	for _, e := range result.Errors {
		t.Errorf("unexpected checker error at %d:%d: %s", e.Line, e.Col, e.Message)
	}
}

func TestCannotAddMethodToLibraryType(t *testing.T) {
	const libSrc = `
type Widget {
	size Number
}

fn NewWidget(size Number) Widget { return Widget{size: size} }
`
	const mainSrc = `
var mylib = lib "test/mylib"

fn Widget.Double() Number { return self.size * 2 }

fn Main() Number {
	var w = mylib.NewWidget(size: 5)
	return w.Double()
}
`
	result := parseLibTestProg(t, mainSrc, libSrc, "test/mylib")

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "cannot define method on type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected checker error about defining method on library type, got errors: %v", result.Errors)
	}
}

// parseTwoLibProg parses mainSrc plus two library sources seeded at distinct
// paths — simulating the same repo imported twice at different commit hashes.
// Both libraries can define overlapping type and function names; the checker
// should treat them as independent namespaces.
func parseTwoLibProg(t *testing.T, mainSrc, libSrcA, libSrcB, libPathA, libPathB string) *fctchecker.Result {
	t.Helper()
	prog := parseTestProg(t, mainSrc)

	libA, err := parser.Parse(libSrcA, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse library A: %v", err)
	}
	libA.Path = libPathA
	libA.Text = libSrcA
	prog.Sources[libPathA] = libA

	libB, err := parser.Parse(libSrcB, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse library B: %v", err)
	}
	libB.Path = libPathB
	libB.Text = libSrcB
	prog.Sources[libPathB] = libB

	return fctchecker.Check(prog)
}

// TestSameLibraryTwoHashesCoexist verifies that importing the same library
// repo under two different refs (e.g. commit hashes) yields two independent
// namespaces. Same-named types from each import are distinct identities and
// may appear side-by-side in the same program.
func TestSameLibraryTwoHashesCoexist(t *testing.T) {
	// Shared *source*, simulating two copies of the same lib at different commits.
	const libSrc = `
type Widget {
	size Number
}

fn NewWidget(size Number) Widget { return Widget{size: size} }

fn Widget.Double() Number { return self.size * 2 }
`
	const mainSrc = `
var A = lib "github.com/x/libA@hash1"
var B = lib "github.com/x/libA@hash2"

fn Main() Number {
	var a = A.NewWidget(size: 3)
	var b = B.NewWidget(size: 7)
	return a.Double() + b.Double()
}
`
	result := parseTwoLibProg(t, mainSrc, libSrc, libSrc,
		"github.com/x/libA@hash1", "github.com/x/libA@hash2")

	for _, e := range result.Errors {
		t.Errorf("unexpected checker error at %d:%d: %s", e.Line, e.Col, e.Message)
	}
}

// TestSameLibraryTwoHashesTypesAreDistinct verifies that a Widget from the
// hash1 import is NOT assignable to a parameter typed against the hash2
// import's Widget — the checker must reject the mix-up.
func TestSameLibraryTwoHashesTypesAreDistinct(t *testing.T) {
	const libSrc = `
type Widget {
	size Number
}

fn NewWidget(size Number) Widget { return Widget{size: size} }
`
	// accept() is declared in alias B's namespace but receives an A.Widget.
	// The checker should reject this because A.Widget and B.Widget are
	// different types despite sharing a source.
	const mainSrc = `
var A = lib "github.com/x/libA@hash1"
var B = lib "github.com/x/libA@hash2"

fn accept(w B.Widget) Number { return w.size }

fn Main() Number {
	var a = A.NewWidget(size: 3)
	return accept(w: a)
}
`
	result := parseTwoLibProg(t, mainSrc, libSrc, libSrc,
		"github.com/x/libA@hash1", "github.com/x/libA@hash2")

	if len(result.Errors) == 0 {
		t.Fatal("expected checker error when passing A.Widget where B.Widget is required, got none")
	}
	// Sanity: the error should be about the argument type, not something unrelated.
	var typeMismatch bool
	for _, e := range result.Errors {
		msg := strings.ToLower(e.Message)
		if strings.Contains(msg, "widget") || strings.Contains(msg, "type") || strings.Contains(msg, "argument") {
			typeMismatch = true
		}
	}
	if !typeMismatch {
		t.Errorf("expected a type-mismatch error mentioning Widget/type/argument; got %v", result.Errors)
	}
}

func TestCannotAddQualifiedMethodToLibraryType(t *testing.T) {
	// fn mylib.Widget.Double() — two dots — should be rejected at parse time.
	// The parser only supports fn Type.Method (one dot), so this is a syntax error.
	const mainSrc = `
var mylib = lib "test/mylib"

fn mylib.Widget.Double() Number { return self.size * 2 }

fn Main() Number { return 0 }
`
	_, err := parser.Parse(mainSrc, "", parser.SourceUser)
	if err == nil {
		t.Errorf("expected parse error for qualified library method (fn mylib.Widget.Double), got none")
	} else {
		t.Logf("got expected parse error: %v", err)
	}
}
