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
