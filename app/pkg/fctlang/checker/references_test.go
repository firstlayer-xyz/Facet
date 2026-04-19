package checker

import (
	"testing"

	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
)

// checkSourceResult is a test helper that parses a single-file program, runs
// the checker, and returns the full Result. It fails the test if the checker
// reports any errors.
func checkSourceResult(t *testing.T, src string) *Result {
	t.Helper()
	prog := parseTestProg(t, src)
	res := Check(prog)
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected checker errors: %v", res.Errors)
	}
	return res
}

func TestReferencesMapInitialized(t *testing.T) {
	res := checkSourceResult(t, `fn Main() { return 0 }`)
	if res.References == nil {
		t.Fatal("Result.References is nil; expected initialized map")
	}
}

func TestTypeEnvTracksDeclPosition(t *testing.T) {
	c := initChecker(loader.Program{})
	env := c.newEnv()
	pos := parser.Pos{Line: 5, Col: 3}
	env.bind("x", simple(typeNumber), pos, "var")
	got, ok := env.lookupPos("x")
	if !ok {
		t.Fatal("lookupPos(x) returned !ok after bind")
	}
	if got != pos {
		t.Errorf("lookupPos(x) = %+v, want %+v", got, pos)
	}
}

func TestReferenceForLocalVar(t *testing.T) {
	src := `fn Main() Number {
    var x = 10
    return x
}`
	res := checkSourceResult(t, src)
	got, ok := res.References[":3:12"]
	if !ok {
		t.Fatalf("no reference for x at :3:12; map=%v", res.References)
	}
	if got.Line != 2 || got.Col != 9 {
		t.Errorf("reference target = %+v, want line 2, col 9", got)
	}
}

func TestReferenceForGlobalConst(t *testing.T) {
	src := `const K = 42
fn Main() Number { return K }`
	res := checkSourceResult(t, src)
	got, ok := res.References[":2:27"]
	if !ok {
		t.Fatalf("no reference for K at :2:27; map=%v", res.References)
	}
	if got.Line != 1 || got.Col != 7 {
		t.Errorf("target = %+v, want line 1, col 7", got)
	}
	if got.Kind != "const" {
		t.Errorf("target.Kind = %q, want const", got.Kind)
	}
}

func TestReferenceForFunctionParam(t *testing.T) {
	src := `fn Double(n Number) Number { return n * 2 }
fn Main() Number { return Double(n: 3) }`
	res := checkSourceResult(t, src)
	got, ok := res.References[":1:37"]
	if !ok {
		t.Fatalf("no reference for n; map=%v", res.References)
	}
	if got.Line != 1 || got.Col != 11 {
		t.Errorf("target = %+v, want line 1, col 11", got)
	}
	if got.Kind != "param" {
		t.Errorf("Kind = %q, want param", got.Kind)
	}
}

func TestReferenceForCall(t *testing.T) {
	src := `fn Helper() Number { return 1 }
fn Main() Number { return Helper() }`
	res := checkSourceResult(t, src)
	// "return Helper()" on line 2 — Helper at col 27.
	got, ok := res.References[":2:27"]
	if !ok {
		t.Fatalf("no reference for Helper(); map=%v", res.References)
	}
	if got.Kind != "fn" {
		t.Errorf("Kind = %q, want fn", got.Kind)
	}
	// Helper is at line 1; Function.Pos is the 'fn' keyword position, so col 1.
	if got.Line != 1 {
		t.Errorf("target line = %d, want 1", got.Line)
	}
}

func TestReferenceForStdlibCall(t *testing.T) {
	src := `fn Main() Solid { return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}) }`
	res := checkSourceResult(t, src)
	// "return Cube(" — Cube at col 26.
	got, ok := res.References[":1:26"]
	if !ok {
		t.Fatalf("no reference for Cube(); map=%v", res.References)
	}
	if got.File == "" {
		t.Error("expected File to point at stdlib source, got empty")
	}
	if got.Kind != "fn" {
		t.Errorf("Kind = %q, want fn", got.Kind)
	}
}

// TestReferenceForMethodCall verifies that method-call sites resolve to the
// target method's declaration. The recording happens at the single chokepoint
// in checkFuncArgs (Task 4); this test pins that coverage for the method-call
// path specifically.
func TestReferenceForMethodCall(t *testing.T) {
	src := `fn Main() Solid {
    var c = Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
    return c.Move(x: 5 mm)
}`
	res := checkSourceResult(t, src)
	// "    return c.Move(" — `M` of Move is at col 14 on line 3.
	got, ok := res.References[":3:14"]
	if !ok {
		t.Fatalf("no reference for .Move; map=%v", res.References)
	}
	if got.Kind != "fn" {
		t.Errorf("Kind = %q, want fn", got.Kind)
	}
	if got.File == "" {
		t.Error("expected File to point at stdlib source for Move, got empty")
	}
}

// TestReferenceForMultilineMethodCall pins the original bug that motivated
// this whole plan: a method whose name starts on a continuation line (the
// receiver and the `.Method` are on separate lines). The regex-based approach
// previously failed here because it only examined the current line's text.
// The AST-driven references map is indexed by the method-name token's own
// position, so line continuation is irrelevant.
func TestReferenceForMultilineMethodCall(t *testing.T) {
	src := `fn Main() Solid {
    var c = Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
    return c
        .Move(x: 5 mm)
}`
	res := checkSourceResult(t, src)
	// Line 4: "        .Move(" — 8 spaces, then ".", then `M` at col 10.
	got, ok := res.References[":4:10"]
	if !ok {
		t.Fatalf("no reference for .Move on continuation line; map=%v", res.References)
	}
	if got.Kind != "fn" {
		t.Errorf("Kind = %q, want fn", got.Kind)
	}
}

func TestReferenceForFieldAccess(t *testing.T) {
	src := `type P { x Number; y Number }
fn Main() Number {
    var p = P{x: 1, y: 2}
    return p.x
}`
	res := checkSourceResult(t, src)
	// "    return p.x" — `x` at col 14 on line 4. VERIFY by counting.
	got, ok := res.References[":4:14"]
	if !ok {
		t.Fatalf("no reference for .x; map=%v", res.References)
	}
	if got.Kind != "field" {
		t.Errorf("Kind = %q, want field", got.Kind)
	}
	// Decl of x: "type P { x Number; ..." — x at line 1, col 10. VERIFY.
	if got.Line != 1 || got.Col != 10 {
		t.Errorf("target = %+v, want line 1, col 10", got)
	}
}

func TestReferenceForNamedArg(t *testing.T) {
	src := `fn Double(n Number) Number { return n * 2 }
fn Main() Number { return Double(n: 3) }`
	res := checkSourceResult(t, src)
	// "fn Main() Number { return Double(" is 33 chars, so `n` is at col 34.
	got, ok := res.References[":2:34"]
	if !ok {
		t.Fatalf("no reference for named arg n; map=%v", res.References)
	}
	// Parameter `n` declared on line 1, col 11:
	// "fn Double(" is 10 chars, so `n` at col 11.
	if got.Line != 1 || got.Col != 11 {
		t.Errorf("target = %+v, want line 1, col 11", got)
	}
	if got.Kind != "param" {
		t.Errorf("Kind = %q, want param", got.Kind)
	}
	// Task 7 spec also requires ReturnType to be the param's declared type.
	if got.ReturnType != "Number" {
		t.Errorf("ReturnType = %q, want Number", got.ReturnType)
	}
}

func TestReferenceForNamedArgInMethodCall(t *testing.T) {
	src := `fn Main() Solid {
    var c = Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
    return c.Move(x: 5 mm)
}`
	res := checkSourceResult(t, src)
	// "    return c.Move(" is 18 chars, so `x` is at col 19 on line 3.
	got, ok := res.References[":3:19"]
	if !ok {
		t.Fatalf("no reference for method-call named arg x; map=%v", res.References)
	}
	if got.Kind != "param" {
		t.Errorf("Kind = %q, want param", got.Kind)
	}
	if got.File == "" {
		t.Error("expected File to point at stdlib source for Move's x param, got empty")
	}
}

func TestReferenceForStructLit(t *testing.T) {
	src := `type P { x Number; y Number }
fn Main() P { return P{x: 1, y: 2} }`
	res := checkSourceResult(t, src)
	// "return P{...}" — P at line 2, col 22. VERIFY.
	got, ok := res.References[":2:22"]
	if !ok {
		t.Fatalf("no reference for struct lit type P; map=%v", res.References)
	}
	if got.Kind != "type" {
		t.Errorf("Kind = %q, want type", got.Kind)
	}
	// StructDecl.Pos points at the `type` keyword — line 1, col 1.
	if got.Line != 1 || got.Col != 1 {
		t.Errorf("type target = %+v, want line 1, col 1", got)
	}
	// "x: 1" — x at line 2, col 24. VERIFY.
	xRef, ok := res.References[":2:24"]
	if !ok {
		t.Fatalf("no reference for field init x; map=%v", res.References)
	}
	if xRef.Kind != "field" {
		t.Errorf("field init Kind = %q, want field", xRef.Kind)
	}
	// Field `x` is declared at "type P { x ..." — line 1, col 10.
	if xRef.Line != 1 || xRef.Col != 10 {
		t.Errorf("field init target = %+v, want line 1, col 10", xRef)
	}
}
