package evaluator

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Positive tests — var as a type annotation on function params/returns
// ---------------------------------------------------------------------------

func TestEvalVarParamAcceptsNumber(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid { var n = Identity(x: 42); return Cube(size: Vec3{x: n mm, y: n mm, z: n mm}) }
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalVarParamAcceptsLength(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid { var n = Identity(x: 10 mm); return Cube(size: Vec3{x: n, y: n, z: n}) }
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalVarParamAcceptsSolid(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid { return Identity(x: Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})) }
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}

func TestEvalVarParamAcceptsString(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid {
	var s = Identity(x: "hello")
	assert s == "hello"
	return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval: %v", err)
	}
}

func TestEvalVarParamAcceptsBool(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid {
	var b = Identity(x: true)
	assert b
	return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval: %v", err)
	}
}

func TestEvalVarParamAcceptsAngle(t *testing.T) {
	src := `
fn Identity(x var) var { return x }
fn Main() Solid {
	var a = Identity(x: 90 deg)
	return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 0 deg, ry: 0 deg, rz: a, pivot: WorldOrigin)
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval: %v", err)
	}
}

func TestEvalVarArrayParam(t *testing.T) {
	src := `
fn First(arr []var) var { return arr[0] }
fn Main() Solid {
	var n = First(arr: [10, 20, 30])
	return Cube(size: Vec3{x: n mm, y: n mm, z: n mm})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

func TestEvalVarReturnInferredFromParam(t *testing.T) {
	// var return type should pass through whatever the param type is
	src := `
fn Double(x var) var { return x + x }
fn Main() Solid {
	var n = Double(x: 5)
	return Cube(size: Vec3{x: n mm, y: n mm, z: n mm})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

func TestEvalVarParamMultipleArgs(t *testing.T) {
	// Multiple var params — each independently accepts any type
	src := `
fn Pair(a var, b var) var { return a }
fn Main() Solid {
	var n = Pair(a: 10, b: "hello")
	return Cube(size: Vec3{x: n mm, y: n mm, z: n mm})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

func TestEvalVarParamPassedToConcreteFunction(t *testing.T) {
	// var param value passed to a function expecting a concrete type
	src := `
fn Wrap(x var) Solid { return Cube(size: Vec3{x: x, y: x, z: x}) }
fn Main() Solid { return Wrap(x: 10 mm) }
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

func TestEvalVarParamInForYield(t *testing.T) {
	// var param used in a for-yield loop
	src := `
fn Repeat(n var, count Number) []var {
	return for i [0:<count] { yield n }
}
fn Main() Solid {
	var arr = Repeat(n: 5 mm, count: 3)
	return Cube(size: Vec3{x: arr[0], y: arr[1], z: arr[2]})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.5)
}

func TestEvalVarParamWithStruct(t *testing.T) {
	src := `
type Pair { a Length; b Length }
fn GetFirst(p var) var { return p.a }
fn Main() Solid {
	var p = Pair { a: 10 mm, b: 20 mm }
	var v = GetFirst(p: p)
	return Cube(size: Vec3{x: v, y: p.b, z: v})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	assertMeshSize(t, mesh, 10, 20, 10, 0.5)
}

// ---------------------------------------------------------------------------
// Negative tests — var type misuse that should produce runtime errors
// ---------------------------------------------------------------------------

func TestEvalVarParamTypeMismatchAtUse(t *testing.T) {
	// Passing a string where a Length is needed at the use site
	src := `
fn Wrap(x var) Solid { return Cube(size: Vec3{x: x, y: x, z: x}) }
fn Main() Solid { return Wrap(x: "hello") }
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error when string is used as Length")
	}
}

func TestEvalVarReturnUsedAsWrongType(t *testing.T) {
	// var function returns a string, caller tries to use it as Length
	src := `
fn GetVal() var { return "not a length" }
fn Main() Solid {
	var v = GetVal()
	return Cube(size: Vec3{x: v, y: v, z: v})
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error when string used as Length")
	}
}

func TestEvalVarParamBadArithmetic(t *testing.T) {
	// var accepts a bool, but bool doesn't support +
	src := `
fn Double(x var) var { return x + x }
fn Main() Solid {
	var n = Double(x: true)
	return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for bool + bool")
	}
	if !strings.Contains(err.Error(), "Bool") {
		t.Errorf("error should mention Bool: %v", err)
	}
}

func TestEvalVarArrayParamNonArray(t *testing.T) {
	// []var param but caller passes a non-array
	src := `
fn First(arr []var) var { return arr[0] }
fn Main() Solid {
	var n = First(arr: 42)
	return Cube(size: Vec3{x: n mm, y: n mm, z: n mm})
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error when Number passed to []var param")
	}
}

func TestEvalVarArrayParamEmptyArray(t *testing.T) {
	// []var with empty array — indexing should fail
	src := `
fn First(arr []var) var { return arr[0] }
fn Main() Solid {
	var n = First(arr: [])
	return Cube(size: Vec3{x: n mm, y: n mm, z: n mm})
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected out-of-range error for empty array")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Checker tests — var type annotation in static analysis
// ---------------------------------------------------------------------------

func TestCheckVarParamNoError(t *testing.T) {
	expectNoErrors(t, `
fn Identity(x var) var { return x }
fn Main() Solid { return Identity(x: Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})) }
`)
}

func TestCheckVarArrayParamNoError(t *testing.T) {
	expectNoErrors(t, `
fn First(arr []var) var { return arr[0] }
fn Main() Solid { var n = First(arr: []Number[10,]); return Cube(size: Vec3{x: n mm, y: n mm, z: n mm}) }
`)
}

func TestCheckVarParamMultipleTypes(t *testing.T) {
	// Calling same var function with different types — no checker error
	expectNoErrors(t, `
fn Identity(x var) var { return x }
fn Main() Solid {
	var a = Identity(x: 42)
	var b = Identity(x: "hello")
	var c = Identity(x: 10 mm)
	return Cube(size: Vec3{x: c, y: c, z: c})
}
`)
}

func TestCheckVarReturnUsedAsSolid(t *testing.T) {
	// var return type used directly as Solid — checker should allow
	expectNoErrors(t, `
fn MakeBox(s var) Solid { return Cube(size: Vec3{x: s, y: s, z: s}) }
fn Main() Solid { return MakeBox(s: 10 mm) }
`)
}

func TestCheckVarGroupConsistency(t *testing.T) {
	// Consecutive var params form a group — checker enforces same concrete type
	src := `
fn Add(a var, b var) var { return a + b }
fn Main() Solid {
	var n = Add(a: 10, b: "hello")
	return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}
`
	errs := checkSource(t, src)
	hasConflict := false
	for _, e := range errs {
		if strings.Contains(e.Message, "generic type conflict") {
			hasConflict = true
			break
		}
	}
	if !hasConflict {
		t.Error("expected 'generic type conflict' error for Add(Number, String)")
	}
}

func TestCheckVarArrayReturnType(t *testing.T) {
	expectNoErrors(t, `
fn Wrap(x var) []var { return []Length[x, x, x] }
fn Main() Solid {
	var arr = Wrap(x: 5 mm)
	return Cube(size: Vec3{x: arr[0], y: arr[1], z: arr[2]})
}
`)
}
