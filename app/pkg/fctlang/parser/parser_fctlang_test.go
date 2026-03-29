package parser_test

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestParseMinimal(t *testing.T) {
	src := `fn Main() Solid { return Cylinder(bottom: 10, top: 10, height: 10); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prog.Functions()) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions()))
	}
	fn := prog.Functions()[0]
	if fn.ReturnType != "Solid" {
		t.Errorf("return type = %q, want %q", fn.ReturnType, "Solid")
	}
	if fn.Name != "Main" {
		t.Errorf("name = %q, want %q", fn.Name, "Main")
	}
	if len(fn.Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(fn.Params))
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	ret, ok := fn.Body[0].(*parser.ReturnStmt)
	if !ok {
		t.Fatalf("expected parser.ReturnStmt, got %T", fn.Body[0])
	}
	call, ok := ret.Value.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr, got %T", ret.Value)
	}
	if call.Name != "Cylinder" {
		t.Errorf("call name = %q, want %q", call.Name, "Cylinder")
	}
	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(call.Args))
	}
	for i, arg := range call.Args {
		na, ok := arg.(*parser.NamedArg)
		if !ok {
			t.Errorf("arg %d: expected parser.NamedArg, got %T", i, arg)
			continue
		}
		num, ok := na.Value.(*parser.NumberLit)
		if !ok {
			t.Errorf("arg %d: expected parser.NumberLit value, got %T", i, na.Value)
			continue
		}
		if num.Value != 10 {
			t.Errorf("arg %d: value = %v, want 10", i, num.Value)
		}
	}
}

func TestParseMultiFunction(t *testing.T) {
	src := `
# helper function
fn Bar() Solid {
    return Sphere(radius: 5);
}

fn Main() Solid {
    return Bar();
}
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prog.Functions()) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(prog.Functions()))
	}

	bar := prog.Functions()[0]
	if bar.Name != "Bar" {
		t.Errorf("first function name = %q, want %q", bar.Name, "Bar")
	}

	main := prog.Functions()[1]
	if main.Name != "Main" {
		t.Errorf("second function name = %q, want %q", main.Name, "Main")
	}

	// Bar() returns a call to Sphere with 1 arg
	ret, ok := bar.Body[0].(*parser.ReturnStmt)
	if !ok {
		t.Fatalf("expected parser.ReturnStmt")
	}
	call, ok := ret.Value.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr")
	}
	if call.Name != "Sphere" {
		t.Errorf("call name = %q, want %q", call.Name, "Sphere")
	}
	if len(call.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(call.Args))
	}

	// Main() returns a call to Bar with 0 args
	ret2, _ := main.Body[0].(*parser.ReturnStmt)
	call2, ok := ret2.Value.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr in Main, got %T", ret2.Value)
	}
	if call2.Name != "Bar" {
		t.Errorf("call name = %q, want %q", call2.Name, "Bar")
	}
	if len(call2.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(call2.Args))
	}
}

func TestParseFunctionWithParams(t *testing.T) {
	src := `fn Make(a, b Solid) Solid { return a; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Type != "Solid" || fn.Params[0].Name != "a" {
		t.Errorf("param 0 = %+v", fn.Params[0])
	}
	if fn.Params[1].Type != "Solid" || fn.Params[1].Name != "b" {
		t.Errorf("param 1 = %+v", fn.Params[1])
	}
	ret := fn.Body[0].(*parser.ReturnStmt)
	ident, ok := ret.Value.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("expected parser.IdentExpr, got %T", ret.Value)
	}
	if ident.Name != "a" {
		t.Errorf("ident name = %q, want %q", ident.Name, "a")
	}
}

func TestParseFloatLiteral(t *testing.T) {
	src := `fn Main() Solid { return Sphere(radius: 3.14); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	na, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	num, ok := na.Value.(*parser.NumberLit)
	if !ok {
		t.Fatalf("expected parser.NumberLit value, got %T", na.Value)
	}
	if num.Value != 3.14 {
		t.Errorf("value = %v, want 3.14", num.Value)
	}
}

func TestParseRatioLiteral(t *testing.T) {
	src := `fn Main() Solid { return Cube(size: {x: 1/2 mm, y: 3/4 mm, z: 7/8 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	if len(call.Args) != 1 {
		t.Fatalf("expected 1 arg (struct literal), got %d", len(call.Args))
	}
	na, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	sl, ok := na.Value.(*parser.StructLitExpr)
	if !ok {
		t.Fatalf("expected parser.StructLitExpr value, got %T", na.Value)
	}
	if len(sl.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(sl.Fields))
	}
	want := []float64{0.5, 0.75, 0.875}
	for i, field := range sl.Fields {
		ue, ok := field.Value.(*parser.UnitExpr)
		if !ok {
			t.Errorf("field %d: expected parser.UnitExpr, got %T", i, field.Value)
			continue
		}
		if ue.Unit != "mm" {
			t.Errorf("field %d: unit = %q, want %q", i, ue.Unit, "mm")
		}
		if ue.IsAngle {
			t.Errorf("field %d: expected IsAngle = false", i)
		}
		num, ok := ue.Expr.(*parser.NumberLit)
		if !ok {
			t.Errorf("field %d: inner expr: expected parser.NumberLit, got %T", i, ue.Expr)
			continue
		}
		if num.Value != want[i] {
			t.Errorf("field %d: value = %v, want %v", i, num.Value, want[i])
		}
	}
}

func TestParseRatioPlainNumber(t *testing.T) {
	// Ratio without a unit → plain NumberLit
	src := `fn Main() Solid { return Cube(size: {x: 5 mm, y: 1/2, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	na, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	sl, ok := na.Value.(*parser.StructLitExpr)
	if !ok {
		t.Fatalf("expected parser.StructLitExpr value, got %T", na.Value)
	}
	num, ok := sl.Fields[1].Value.(*parser.NumberLit)
	if !ok {
		t.Fatalf("field 1: expected parser.NumberLit, got %T", sl.Fields[1].Value)
	}
	if num.Value != 0.5 {
		t.Errorf("value = %v, want 0.5", num.Value)
	}
}

func TestParseLengthLiteral(t *testing.T) {
	src := `fn Main() Solid { return Cylinder(bottom: 1 ft, top: 2.5 cm, height: 100 mm); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(call.Args))
	}
	wantValues := []float64{1, 2.5, 100}
	wantUnit := []string{"ft", "cm", "mm"}
	for i, arg := range call.Args {
		na, ok := arg.(*parser.NamedArg)
		if !ok {
			t.Errorf("arg %d: expected parser.NamedArg, got %T", i, arg)
			continue
		}
		ue, ok := na.Value.(*parser.UnitExpr)
		if !ok {
			t.Errorf("arg %d: expected parser.UnitExpr value, got %T", i, na.Value)
			continue
		}
		if ue.Unit != wantUnit[i] {
			t.Errorf("arg %d: unit = %q, want %q", i, ue.Unit, wantUnit[i])
		}
		if ue.IsAngle {
			t.Errorf("arg %d: expected IsAngle = false", i)
		}
		num, ok := ue.Expr.(*parser.NumberLit)
		if !ok {
			t.Errorf("arg %d: inner expr: expected parser.NumberLit, got %T", i, ue.Expr)
			continue
		}
		if math.Abs(num.Value-wantValues[i]) > 1e-10 {
			t.Errorf("arg %d: value = %v, want %v", i, num.Value, wantValues[i])
		}
	}
}

func TestParseVarStatement(t *testing.T) {
	src := `fn Main() Solid {
		var size = 10 mm;
		return Cube(size: {x: size, y: size, z: size});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	v, ok := fn.Body[0].(*parser.VarStmt)
	if !ok {
		t.Fatalf("expected parser.VarStmt, got %T", fn.Body[0])
	}
	if v.Name != "size" {
		t.Errorf("var name = %q, want %q", v.Name, "size")
	}
	ue, ok := v.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr, got %T", v.Value)
	}
	if ue.Unit != "mm" {
		t.Errorf("unit = %q, want %q", ue.Unit, "mm")
	}
	if ue.IsAngle {
		t.Error("expected IsAngle = false")
	}
	num, ok := ue.Expr.(*parser.NumberLit)
	if !ok {
		t.Fatalf("inner expr: expected parser.NumberLit, got %T", ue.Expr)
	}
	if num.Value != 10 {
		t.Errorf("value = %v, want 10", num.Value)
	}
}

func TestInferredReturnType(t *testing.T) {
	src := `fn Main() { return Cube(size: {x: 1, y: 2, z: 3}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Functions()) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions()))
	}
	fn := prog.Functions()[0]
	if fn.ReturnType != "" {
		t.Errorf("expected empty return type, got %q", fn.ReturnType)
	}
	if fn.Name != "Main" {
		t.Errorf("expected name 'Main', got %q", fn.Name)
	}
}

func TestErrorEmptyInput(t *testing.T) {
	prog, err := parser.Parse("", "", parser.SourceUser)
	if err != nil {
		t.Fatalf("empty input should not be an error, got: %v", err)
	}
	if len(prog.Functions()) != 0 {
		t.Errorf("expected 0 functions, got %d", len(prog.Functions()))
	}
}

func TestComments(t *testing.T) {
	src := `
# this is a comment
fn Main() {
    var s = 10 mm; # inline comment
    return Cube(size: {x: s, y: s, z: s});
}
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Functions()) != 1 {
		t.Fatalf("expected 1 function, got %d", len(prog.Functions()))
	}
}

func TestErrorUnexpectedCharacter(t *testing.T) {
	src := `fn Main() Solid { return @; }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected error for unexpected character")
	}
}

func TestParseDotCall(t *testing.T) {
	src := `fn Main() { return Cube(size: {x: 10, y: 10, z: 10}).Translate(v: {x: 1, y: 2, z: 3}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	mc, ok := ret.Value.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected parser.MethodCallExpr, got %T", ret.Value)
	}
	if mc.Method != "Translate" {
		t.Errorf("method = %q, want %q", mc.Method, "Translate")
	}
	if len(mc.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(mc.Args))
	}
	call, ok := mc.Receiver.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr receiver, got %T", mc.Receiver)
	}
	if call.Name != "Cube" {
		t.Errorf("receiver call name = %q, want %q", call.Name, "Cube")
	}
}

func TestParseDotChain(t *testing.T) {
	src := `fn Main() { return Cube(size: {x: 10, y: 10, z: 10}).Translate(v: {x: 1, y: 2, z: 3}).Rotate(rx: 0, ry: 45, rz: 0); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	// Outermost should be Rotate
	mc, ok := ret.Value.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected parser.MethodCallExpr, got %T", ret.Value)
	}
	if mc.Method != "Rotate" {
		t.Errorf("outer method = %q, want %q", mc.Method, "Rotate")
	}
	// Inner should be Translate
	inner, ok := mc.Receiver.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected inner parser.MethodCallExpr, got %T", mc.Receiver)
	}
	if inner.Method != "Translate" {
		t.Errorf("inner method = %q, want %q", inner.Method, "Translate")
	}
	// Innermost should be Cube call
	call, ok := inner.Receiver.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr, got %T", inner.Receiver)
	}
	if call.Name != "Cube" {
		t.Errorf("call name = %q, want %q", call.Name, "Cube")
	}
}

func TestParseDotOnVariable(t *testing.T) {
	src := `fn Main() {
		var box = Cube(size: {x: 10, y: 10, z: 10});
		return box.Translate(v: {x: 1, y: 2, z: 3});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[1].(*parser.ReturnStmt)
	mc, ok := ret.Value.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected parser.MethodCallExpr, got %T", ret.Value)
	}
	if mc.Method != "Translate" {
		t.Errorf("method = %q, want %q", mc.Method, "Translate")
	}
	ident, ok := mc.Receiver.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("expected parser.IdentExpr receiver, got %T", mc.Receiver)
	}
	if ident.Name != "box" {
		t.Errorf("ident name = %q, want %q", ident.Name, "box")
	}
}

func TestParseDotOnParenExpr(t *testing.T) {
	src := `fn Main() { return (Cube(size: {x: 10, y: 10, z: 10}) + Sphere(radius: 5)).Translate(v: {x: 0, y: 0, z: 0}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	mc, ok := ret.Value.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected parser.MethodCallExpr, got %T", ret.Value)
	}
	if mc.Method != "Translate" {
		t.Errorf("method = %q, want %q", mc.Method, "Translate")
	}
	if len(mc.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(mc.Args))
	}
	// Receiver should be a BinaryExpr (union)
	_, ok = mc.Receiver.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr receiver, got %T", mc.Receiver)
	}
}

func TestParseAngleLiteralDeg(t *testing.T) {
	src := `fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 45 deg, ry: 0 deg, rz: 0 deg); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	mc := ret.Value.(*parser.MethodCallExpr)
	if mc.Method != "Rotate" {
		t.Fatalf("method = %q, want %q", mc.Method, "Rotate")
	}
	na, ok := mc.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("arg 0: expected parser.NamedArg, got %T", mc.Args[0])
	}
	ue, ok := na.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("arg 0: expected parser.UnitExpr value, got %T", na.Value)
	}
	if ue.Unit != "deg" {
		t.Errorf("arg 0: unit = %q, want %q", ue.Unit, "deg")
	}
	if !ue.IsAngle {
		t.Error("arg 0: expected IsAngle = true")
	}
	num, ok := ue.Expr.(*parser.NumberLit)
	if !ok {
		t.Fatalf("arg 0: inner expr: expected parser.NumberLit, got %T", ue.Expr)
	}
	if num.Value != 45 {
		t.Errorf("arg 0: value = %v, want 45", num.Value)
	}
}

func TestParseAngleLiteralRad(t *testing.T) {
	src := `fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: 3.14159265358979 rad, ry: 0 deg, rz: 0 deg); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	mc := ret.Value.(*parser.MethodCallExpr)
	naRad, ok := mc.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("arg 0: expected parser.NamedArg, got %T", mc.Args[0])
	}
	ue, ok := naRad.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("arg 0: expected parser.UnitExpr value, got %T", naRad.Value)
	}
	if ue.Unit != "rad" {
		t.Errorf("arg 0: unit = %q, want %q", ue.Unit, "rad")
	}
	if !ue.IsAngle {
		t.Error("arg 0: expected IsAngle = true")
	}
	num, ok := ue.Expr.(*parser.NumberLit)
	if !ok {
		t.Fatalf("arg 0: inner expr: expected parser.NumberLit, got %T", ue.Expr)
	}
	if math.Abs(num.Value-3.14159265358979) > 0.001 {
		t.Errorf("arg 0: value = %v, want ~3.14159265358979", num.Value)
	}
}

func TestParseArrayLiteral(t *testing.T) {
	src := `fn Main() { return []Length[1 mm, 2 mm, 3 mm]; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	arr, ok := ret.Value.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("expected parser.ArrayLitExpr, got %T", ret.Value)
	}
	if len(arr.Elems) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr.Elems))
	}
}

func TestParseArrayTrailingComma(t *testing.T) {
	src := `fn Main() { return []Number[1, 2, 3,]; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	arr, ok := ret.Value.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("expected parser.ArrayLitExpr, got %T", ret.Value)
	}
	if len(arr.Elems) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr.Elems))
	}
}

func TestParseEmptyArray(t *testing.T) {
	src := `fn Main() { return []; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	arr, ok := ret.Value.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("expected parser.ArrayLitExpr, got %T", ret.Value)
	}
	if len(arr.Elems) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arr.Elems))
	}
}

func TestParseRangeExclusive(t *testing.T) {
	src := `fn Main() { return [0:<6]; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	rng, ok := ret.Value.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("expected parser.RangeExpr, got %T", ret.Value)
	}
	start, ok := rng.Start.(*parser.NumberLit)
	if !ok {
		t.Fatalf("expected parser.NumberLit start, got %T", rng.Start)
	}
	if start.Value != 0 {
		t.Errorf("start = %v, want 0", start.Value)
	}
	end, ok := rng.End.(*parser.NumberLit)
	if !ok {
		t.Fatalf("expected parser.NumberLit end, got %T", rng.End)
	}
	if end.Value != 6 {
		t.Errorf("end = %v, want 6", end.Value)
	}
	if rng.Step != nil {
		t.Errorf("expected nil step")
	}
	if !rng.Exclusive {
		t.Errorf("expected Exclusive = true")
	}
}

func TestParseRangeInclusive(t *testing.T) {
	src := `fn Main() { return [0:6]; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	rng, ok := ret.Value.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("expected parser.RangeExpr, got %T", ret.Value)
	}
	if rng.Exclusive {
		t.Errorf("expected Exclusive = false")
	}
}

func TestParseRangeStep(t *testing.T) {
	src := `fn Main() { return [0:10:2]; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	rng, ok := ret.Value.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("expected parser.RangeExpr, got %T", ret.Value)
	}
	if rng.Step == nil {
		t.Fatal("expected non-nil step")
	}
	stepNum, ok := rng.Step.(*parser.NumberLit)
	if !ok {
		t.Fatalf("expected parser.NumberLit step, got %T", rng.Step)
	}
	if stepNum.Value != 2 {
		t.Errorf("step = %v, want 2", stepNum.Value)
	}
	if rng.Exclusive {
		t.Errorf("expected Exclusive = false")
	}
}

func TestParseForYield(t *testing.T) {
	src := `fn Main() {
		var pts = for i[0:<6] {
			yield Vec2{x: i, y: i};
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(fy.Clauses))
	}
	if fy.Clauses[0].Var != "i" {
		t.Errorf("loop var = %q, want %q", fy.Clauses[0].Var, "i")
	}
	if _, ok := fy.Clauses[0].Iter.(*parser.RangeExpr); !ok {
		t.Errorf("expected parser.RangeExpr iterator, got %T", fy.Clauses[0].Iter)
	}
	if len(fy.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(fy.Body))
	}
	if _, ok := fy.Body[0].(*parser.YieldStmt); !ok {
		t.Errorf("expected parser.YieldStmt, got %T", fy.Body[0])
	}
}

func TestParseFold(t *testing.T) {
	src := `fn Main() {
		var result = fold a, b [0:<3] {
			yield a + b;
		};
		return result;
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	ff, ok := v.Value.(*parser.FoldExpr)
	if !ok {
		t.Fatalf("expected parser.FoldExpr, got %T", v.Value)
	}
	if ff.AccVar != "a" {
		t.Errorf("acc var = %q, want %q", ff.AccVar, "a")
	}
	if ff.ElemVar != "b" {
		t.Errorf("elem var = %q, want %q", ff.ElemVar, "b")
	}
	if _, ok := ff.Iter.(*parser.RangeExpr); !ok {
		t.Errorf("expected parser.RangeExpr iterator, got %T", ff.Iter)
	}
}


func TestParseBoolLiterals(t *testing.T) {
	src := `fn Main() { return true; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	bl, ok := ret.Value.(*parser.BoolLit)
	if !ok {
		t.Fatalf("expected parser.BoolLit, got %T", ret.Value)
	}
	if bl.Value != true {
		t.Errorf("expected true, got false")
	}

	src2 := `fn Main() { return false; }`
	prog2, err := parser.Parse(src2, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret2 := prog2.Functions()[0].Body[0].(*parser.ReturnStmt)
	bl2, ok := ret2.Value.(*parser.BoolLit)
	if !ok {
		t.Fatalf("expected parser.BoolLit, got %T", ret2.Value)
	}
	if bl2.Value != false {
		t.Errorf("expected false, got true")
	}
}

func TestParseComparison(t *testing.T) {
	src := `fn Main() { return 10 mm < 20 mm; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	bin, ok := ret.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr, got %T", ret.Value)
	}
	if bin.Op != "<" {
		t.Errorf("expected op '<', got %q", bin.Op)
	}
}

func TestParseLogicalOps(t *testing.T) {
	src := `fn Main() { return true && false || true; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	// || is lower precedence than &&, so tree is: (true && false) || true
	bin, ok := ret.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr, got %T", ret.Value)
	}
	if bin.Op != "||" {
		t.Errorf("expected op '||', got %q", bin.Op)
	}
	lhs, ok := bin.Left.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr for lhs, got %T", bin.Left)
	}
	if lhs.Op != "&&" {
		t.Errorf("expected lhs op '&&', got %q", lhs.Op)
	}
}

func TestParseIfStmt(t *testing.T) {
	src := `fn Main() {
		if true {
			return 10 mm;
		} else {
			return 20 mm;
		}
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ifS, ok := prog.Functions()[0].Body[0].(*parser.IfStmt)
	if !ok {
		t.Fatalf("expected parser.IfStmt, got %T", prog.Functions()[0].Body[0])
	}
	if len(ifS.Then) != 1 {
		t.Errorf("expected 1 then statement, got %d", len(ifS.Then))
	}
	if ifS.Else == nil {
		t.Fatal("expected else clause")
	}
	if len(ifS.Else) != 1 {
		t.Errorf("expected 1 else statement, got %d", len(ifS.Else))
	}
}

func TestParseIfElseIfStmt(t *testing.T) {
	src := `fn Main() {
		if 1 < 2 {
			return 10 mm;
		} else if 2 < 3 {
			return 20 mm;
		} else {
			return 30 mm;
		}
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ifS, ok := prog.Functions()[0].Body[0].(*parser.IfStmt)
	if !ok {
		t.Fatalf("expected parser.IfStmt, got %T", prog.Functions()[0].Body[0])
	}
	if len(ifS.ElseIfs) != 1 {
		t.Errorf("expected 1 else-if clause, got %d", len(ifS.ElseIfs))
	}
	if ifS.Else == nil {
		t.Fatal("expected else clause")
	}
}

func TestParseIfAsExpressionRejected(t *testing.T) {
	// "if" is a statement, not an expression — it cannot appear in expression position
	src := `fn Main() { return if true { return 10; }; }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected parse error for if in expression position, got nil")
	}
}


func TestParseStringLiteral(t *testing.T) {
	src := `fn Main() { return "hello"; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	sl, ok := ret.Value.(*parser.StringLit)
	if !ok {
		t.Fatalf("expected parser.StringLit, got %T", ret.Value)
	}
	if sl.Value != "hello" {
		t.Errorf("value = %q, want %q", sl.Value, "hello")
	}
}

func TestParseLibExpr(t *testing.T) {
	src := `var T = lib "facet/gears";
fn Main() { return Cube(size: {x: 10, y: 10, z: 10}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Globals()) != 1 {
		t.Fatalf("expected 1 global, got %d", len(prog.Globals()))
	}
	g := prog.Globals()[0]
	if g.Name != "T" {
		t.Errorf("global name = %q, want %q", g.Name, "T")
	}
	le, ok := g.Value.(*parser.LibExpr)
	if !ok {
		t.Fatalf("expected parser.LibExpr, got %T", g.Value)
	}
	if le.Path != "facet/gears" {
		t.Errorf("lib path = %q, want %q", le.Path, "facet/gears")
	}
}

func TestParseDefaultParam(t *testing.T) {
	src := `fn Make(size Length, count Number = 32) Solid { return Cube(size: Vec3{x: size, y: size, z: size}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Default != nil {
		t.Error("param 0 should have no default")
	}
	if fn.Params[1].Default == nil {
		t.Fatal("param 1 should have a default")
	}
	num, ok := fn.Params[1].Default.(*parser.NumberLit)
	if !ok {
		t.Fatalf("expected parser.NumberLit default, got %T", fn.Params[1].Default)
	}
	if num.Value != 32 {
		t.Errorf("default value = %v, want 32", num.Value)
	}
}

func TestParseDefaultParamAfterType(t *testing.T) {
	// Default comes after type: x, y Length, z Length = 0
	// means x and y are required grouped params, z has default.
	src := `fn Foo(x, y Length, z Length = 0) Number { return x + y + z; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Default != nil {
		t.Error("param x should have no default")
	}
	if fn.Params[1].Default != nil {
		t.Error("param y should have no default")
	}
	if fn.Params[2].Default == nil {
		t.Fatal("param z should have a default")
	}
	// All three share the same type.
	for i, p := range fn.Params {
		if p.Type != "Length" {
			t.Errorf("param %d type = %q, want Length", i, p.Type)
		}
	}
}

func TestParseDefaultParamNonTrailing(t *testing.T) {
	// Defaults on non-trailing params are allowed (named args required at call sites).
	src := `fn Foo(a Number = 1, b Number, c Number = 3) Number { return a + b + c; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Default == nil {
		t.Error("param a should have a default")
	}
	if fn.Params[1].Default != nil {
		t.Error("param b should have no default")
	}
	if fn.Params[2].Default == nil {
		t.Error("param c should have a default")
	}
}

func TestParseDefaultParamOldSyntaxError(t *testing.T) {
	// Old syntax: name = value Type → should now error (type must come before default).
	src := `fn Make(a = 1 Number) { return a; }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected error for old default syntax (name = value Type)")
	}
}

func TestParseDefaultParamWithConstraint(t *testing.T) {
	// Default + where constraint: name Type = default where [constraint]
	src := `fn ABC(a Number = 1 where [1:10:1]) Number { return a; }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(fn.Params))
	}
	p := fn.Params[0]
	if p.Name != "a" {
		t.Errorf("param name = %q, want %q", p.Name, "a")
	}
	if p.Type != "Number" {
		t.Errorf("param type = %q, want %q", p.Type, "Number")
	}
	if p.Default == nil {
		t.Error("param should have a default")
	}
	if p.Constraint == nil {
		t.Error("param should have a constraint")
	}
}

func TestParseStructDecl(t *testing.T) {
	src := `
type Vec3 {
    x Length;
    y Length;
    z Length;
}

fn Main() { return Cube(size: {x: 10, y: 10, z: 10}); }
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.StructDecls()) != 1 {
		t.Fatalf("expected 1 struct decl, got %d", len(prog.StructDecls()))
	}
	sd := prog.StructDecls()[0]
	if sd.Name != "Vec3" {
		t.Errorf("struct name = %q, want %q", sd.Name, "Vec3")
	}
	if len(sd.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(sd.Fields))
	}
	for i, f := range sd.Fields {
		if f.Type != "Length" {
			t.Errorf("field %d type = %q, want %q", i, f.Type, "Length")
		}
	}
	if sd.Fields[0].Name != "x" || sd.Fields[1].Name != "y" || sd.Fields[2].Name != "z" {
		t.Errorf("field names = %q %q %q, want x y z", sd.Fields[0].Name, sd.Fields[1].Name, sd.Fields[2].Name)
	}
}

func TestParseStructLit(t *testing.T) {
	src := `
type Vec3 {
    x Length;
    y Length;
    z Length;
}

fn Main() {
    var v = Vec3 { x: 10 mm, y: 20 mm, z: 30 mm };
    return Cube(size: {x: 10, y: 10, z: 10});
}
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	sl, ok := v.Value.(*parser.StructLitExpr)
	if !ok {
		t.Fatalf("expected parser.StructLitExpr, got %T", v.Value)
	}
	if sl.TypeName != "Vec3" {
		t.Errorf("type name = %q, want %q", sl.TypeName, "Vec3")
	}
	if len(sl.Fields) != 3 {
		t.Fatalf("expected 3 field inits, got %d", len(sl.Fields))
	}
	if sl.Fields[0].Name != "x" || sl.Fields[1].Name != "y" || sl.Fields[2].Name != "z" {
		t.Errorf("field names = %q %q %q, want x y z", sl.Fields[0].Name, sl.Fields[1].Name, sl.Fields[2].Name)
	}
}

func TestParseFieldAccess(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 10 mm, h: 5 mm };
    return Cube(size: {x: d.w, y: d.h, z: d.w});
}
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[1].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	naFA, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("arg 0: expected parser.NamedArg, got %T", call.Args[0])
	}
	sl, ok := naFA.Value.(*parser.StructLitExpr)
	if !ok {
		t.Fatalf("arg 0: expected parser.StructLitExpr value, got %T", naFA.Value)
	}
	fa, ok := sl.Fields[0].Value.(*parser.FieldAccessExpr)
	if !ok {
		t.Fatalf("field 0: expected parser.FieldAccessExpr, got %T", sl.Fields[0].Value)
	}
	if fa.Field != "w" {
		t.Errorf("field = %q, want %q", fa.Field, "w")
	}
	ident, ok := fa.Receiver.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("expected parser.IdentExpr receiver, got %T", fa.Receiver)
	}
	if ident.Name != "d" {
		t.Errorf("receiver = %q, want %q", ident.Name, "d")
	}
}

func TestParseStructMethod(t *testing.T) {
	src := `
type Vec3 {
    x Length;
    y Length;
    z Length;
}

fn Vec3.Length() Number {
    return Sqrt(n: self.x * self.x);
}

fn Main() { return Cube(size: {x: 10, y: 10, z: 10}); }
`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Functions()) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(prog.Functions()))
	}
	fn := prog.Functions()[0]
	if fn.Name != "Length" {
		t.Errorf("name = %q, want %q", fn.Name, "Length")
	}
	if fn.ReceiverType != "Vec3" {
		t.Errorf("receiver type = %q, want %q", fn.ReceiverType, "Vec3")
	}
	if fn.ReturnType != "Number" {
		t.Errorf("return type = %q, want %q", fn.ReturnType, "Number")
	}
}

func TestParseImplicitReturnFunction(t *testing.T) {
	// Without explicit return, the last expression becomes an ExprStmt.
	src := `fn Main() { Cube(size: {x: 10, y: 10, z: 10}) }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	es, ok := fn.Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt, got %T", fn.Body[0])
	}
	call, ok := es.Expr.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr, got %T", es.Expr)
	}
	if call.Name != "Cube" {
		t.Errorf("call name = %q, want %q", call.Name, "Cube")
	}

	// With explicit return, it stays a ReturnStmt.
	src2 := `fn Main() { return Cube(size: {x: 10, y: 10, z: 10}); }`
	prog2, err := parser.Parse(src2, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn2 := prog2.Functions()[0]
	if len(fn2.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn2.Body))
	}
	ret, ok := fn2.Body[0].(*parser.ReturnStmt)
	if !ok {
		t.Fatalf("expected parser.ReturnStmt, got %T", fn2.Body[0])
	}
	call2, ok := ret.Value.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected parser.CallExpr, got %T", ret.Value)
	}
	if call2.Name != "Cube" {
		t.Errorf("call name = %q, want %q", call2.Name, "Cube")
	}
}

func TestParseBlockExprRemoved(t *testing.T) {
	// Block expressions are no longer supported — { expr } as an expression should fail to parse
	src := `fn Main() { return Cylinder(bottom: 10 mm, top: 10 mm, height: { 5 mm }); }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected parse error for block expression, got nil")
	}
}

func TestParseIfStmtBranches(t *testing.T) {
	// if is a statement — branches contain ExprStmt for bare expressions
	src := `fn Main() {
		if true { 10 mm } else { 20 mm }
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	ifS, ok := fn.Body[0].(*parser.IfStmt)
	if !ok {
		t.Fatalf("expected parser.IfStmt, got %T", fn.Body[0])
	}
	if len(ifS.Then) != 1 {
		t.Fatalf("expected 1 then stmt, got %d", len(ifS.Then))
	}
	_, ok = ifS.Then[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt in then, got %T", ifS.Then[0])
	}
	if len(ifS.Else) != 1 {
		t.Fatalf("expected 1 else stmt, got %d", len(ifS.Else))
	}
	_, ok = ifS.Else[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt in else, got %T", ifS.Else[0])
	}
}

func TestParseIfStmtElseIf(t *testing.T) {
	src := `fn Main() {
		if 1 < 2 { 10 mm } else if 2 < 3 { 20 mm } else { 30 mm }
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	ifS := fn.Body[0].(*parser.IfStmt)
	if len(ifS.ElseIfs) != 1 {
		t.Fatalf("expected 1 else-if clause, got %d", len(ifS.ElseIfs))
	}
	_, ok := ifS.ElseIfs[0].Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt in else-if body, got %T", ifS.ElseIfs[0].Body[0])
	}
}

func TestParseImplicitYieldForYield(t *testing.T) {
	src := `fn Main() {
		var pts = for i [0:<6] {
			Vec2{x: i, y: i}
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(fy.Body))
	}
	_, ok = fy.Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt, got %T", fy.Body[0])
	}
}

func TestParseImplicitReturnFold(t *testing.T) {
	src := `fn Main() {
		var result = fold a, b [0:<3] {
			a + b
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	ff, ok := v.Value.(*parser.FoldExpr)
	if !ok {
		t.Fatalf("expected parser.FoldExpr, got %T", v.Value)
	}
	if len(ff.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(ff.Body))
	}
	_, ok = ff.Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt, got %T", ff.Body[0])
	}
}

func TestParseImplicitReturnMixed(t *testing.T) {
	src := `fn Main() {
		var x = 10 mm;
		x + 5 mm
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	_, ok := fn.Body[0].(*parser.VarStmt)
	if !ok {
		t.Fatalf("expected parser.VarStmt, got %T", fn.Body[0])
	}
	es, ok := fn.Body[1].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected parser.ExprStmt, got %T", fn.Body[1])
	}
	_, ok = es.Expr.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr, got %T", es.Expr)
	}
}

func TestParseAssert(t *testing.T) {
	src := `fn Main() {
		assert true;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	a, ok := fn.Body[0].(*parser.AssertStmt)
	if !ok {
		t.Fatalf("expected parser.AssertStmt, got %T", fn.Body[0])
	}
	bl, ok := a.Cond.(*parser.BoolLit)
	if !ok {
		t.Fatalf("expected parser.BoolLit condition, got %T", a.Cond)
	}
	if !bl.Value {
		t.Error("expected true condition")
	}
	if a.Message != nil {
		t.Errorf("expected nil message, got %v", a.Message)
	}
}

func TestParseAssertWithMessage(t *testing.T) {
	src := `fn Main() {
		assert 1 < 2, "one should be less than two";
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	a := fn.Body[0].(*parser.AssertStmt)
	sl, ok := a.Message.(*parser.StringLit)
	if !ok {
		t.Fatalf("expected parser.StringLit message, got %T", a.Message)
	}
	if sl.Value != "one should be less than two" {
		t.Errorf("message = %q, want %q", sl.Value, "one should be less than two")
	}
	_, ok = a.Cond.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected parser.BinaryExpr condition, got %T", a.Cond)
	}
}

func TestParseBareExpressionMidBody(t *testing.T) {
	// Bare expressions mid-body are valid ExprStmt.
	src := `fn Main() { Cube(size: {x: 10, y: 10, z: 10}); return Cube(size: {x: 1, y: 1, z: 1}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*parser.ExprStmt); !ok {
		t.Fatalf("expected parser.ExprStmt, got %T", fn.Body[0])
	}
}

func TestParseBooleanNot(t *testing.T) {
	src := `fn Main() {
		var x = !true;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	u, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if u.Op != "!" {
		t.Errorf("op = %q, want %q", u.Op, "!")
	}
	bl, ok := u.Operand.(*parser.BoolLit)
	if !ok {
		t.Fatalf("operand: expected parser.BoolLit, got %T", u.Operand)
	}
	if !bl.Value {
		t.Error("operand should be true")
	}
}

func TestParseUnaryMinusNumber(t *testing.T) {
	src := `fn Main() {
		var x = -5;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	u, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if u.Op != "-" {
		t.Errorf("op = %q, want %q", u.Op, "-")
	}
	num, ok := u.Operand.(*parser.NumberLit)
	if !ok {
		t.Fatalf("operand: expected parser.NumberLit, got %T", u.Operand)
	}
	if num.Value != 5 {
		t.Errorf("operand value = %v, want 5", num.Value)
	}
}

func TestParseUnaryMinusLength(t *testing.T) {
	src := `fn Main() {
		var x = -5 mm;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	u, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if u.Op != "-" {
		t.Errorf("op = %q, want %q", u.Op, "-")
	}
	ue, ok := u.Operand.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("operand: expected parser.UnitExpr, got %T", u.Operand)
	}
	if ue.Unit != "mm" {
		t.Errorf("operand unit = %q, want %q", ue.Unit, "mm")
	}
	if ue.IsAngle {
		t.Error("operand: expected IsAngle = false")
	}
}

func TestParseUnaryMinusAngle(t *testing.T) {
	src := `fn Main() {
		var x = -45 deg;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	u, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if u.Op != "-" {
		t.Errorf("op = %q, want %q", u.Op, "-")
	}
	ue, ok := u.Operand.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("operand: expected parser.UnitExpr, got %T", u.Operand)
	}
	if ue.Unit != "deg" {
		t.Errorf("operand unit = %q, want %q", ue.Unit, "deg")
	}
	if !ue.IsAngle {
		t.Error("operand: expected IsAngle = true")
	}
}

func TestParseUnaryMinusParenExpr(t *testing.T) {
	src := `fn Main() {
		var x = -(10 + 5);
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	u, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if u.Op != "-" {
		t.Errorf("op = %q, want %q", u.Op, "-")
	}
	inner, ok := u.Operand.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("operand: expected parser.BinaryExpr, got %T", u.Operand)
	}
	if inner.Op != "+" {
		t.Errorf("inner op = %q, want %q", inner.Op, "+")
	}
}

func TestParseDoubleNegation(t *testing.T) {
	src := `fn Main() {
		var x = --5;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	outer, ok := v.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected parser.UnaryExpr, got %T", v.Value)
	}
	if outer.Op != "-" {
		t.Errorf("outer op = %q, want %q", outer.Op, "-")
	}
	inner, ok := outer.Operand.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("inner: expected parser.UnaryExpr, got %T", outer.Operand)
	}
	if inner.Op != "-" {
		t.Errorf("inner op = %q, want %q", inner.Op, "-")
	}
}

func TestParseMultiVarForYield(t *testing.T) {
	src := `fn Main() {
		var pts = for i [0:<3], j [0:<3] {
			yield Vec2{x: i, y: j};
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(fy.Clauses))
	}
	if fy.Clauses[0].Var != "i" {
		t.Errorf("clause 0 var = %q, want %q", fy.Clauses[0].Var, "i")
	}
	if fy.Clauses[1].Var != "j" {
		t.Errorf("clause 1 var = %q, want %q", fy.Clauses[1].Var, "j")
	}
}

func TestParseMultiVarForYieldThreeClauses(t *testing.T) {
	src := `fn Main() {
		var pts = for i [0:<2], j [0:<2], k [0:<2] {
			yield i + j + k;
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Clauses) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(fy.Clauses))
	}
}

// ---------------------------------------------------------------------------
// Constrained variable tests
// ---------------------------------------------------------------------------

func TestParseVarConstraintRange(t *testing.T) {
	src := `var x = 10 where [0:100];
fn Main() { return Cube(size: {x: x, y: x, z: x}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prog.Globals()) != 1 {
		t.Fatalf("expected 1 global, got %d", len(prog.Globals()))
	}
	g := prog.Globals()[0]
	if g.Name != "x" {
		t.Errorf("name = %q, want %q", g.Name, "x")
	}
	if _, ok := g.Value.(*parser.NumberLit); !ok {
		t.Errorf("value: expected parser.NumberLit, got %T", g.Value)
	}
	rng, ok := g.Constraint.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.RangeExpr, got %T", g.Constraint)
	}
	start := rng.Start.(*parser.NumberLit)
	end := rng.End.(*parser.NumberLit)
	if start.Value != 0 || end.Value != 100 {
		t.Errorf("range = [%v:%v], want [0:100]", start.Value, end.Value)
	}
}

func TestParseVarConstraintLengthRange(t *testing.T) {
	src := `var w = 10 mm where [1:100] mm;
fn Main() { return Cube(size: {x: w, y: w, z: w}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	if _, ok := g.Value.(*parser.UnitExpr); !ok {
		t.Errorf("value: expected parser.UnitExpr, got %T", g.Value)
	}
	cr, ok := g.Constraint.(*parser.ConstrainedRange)
	if !ok {
		t.Fatalf("constraint: expected parser.ConstrainedRange, got %T", g.Constraint)
	}
	if cr.Unit != "mm" {
		t.Errorf("unit = %q, want %q", cr.Unit, "mm")
	}
	start := cr.Range.Start.(*parser.NumberLit)
	end := cr.Range.End.(*parser.NumberLit)
	if start.Value != 1 || end.Value != 100 {
		t.Errorf("range = [%v:%v], want [1:100]", start.Value, end.Value)
	}
}

func TestParseVarConstraintLengthBounds(t *testing.T) {
	src := `var w2 = 10 mm where [1 mm:100 mm];
fn Main() { return Cube(size: {x: w2, y: w2, z: w2}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	rng, ok := g.Constraint.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.RangeExpr, got %T", g.Constraint)
	}
	if _, ok := rng.Start.(*parser.UnitExpr); !ok {
		t.Errorf("start: expected parser.UnitExpr, got %T", rng.Start)
	}
	if _, ok := rng.End.(*parser.UnitExpr); !ok {
		t.Errorf("end: expected parser.UnitExpr, got %T", rng.End)
	}
}

func TestParseVarConstraintAngleRange(t *testing.T) {
	src := `var a = 45 deg where [0:360] deg;
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	ue, ok := g.Value.(*parser.UnitExpr)
	if !ok {
		t.Errorf("value: expected parser.UnitExpr, got %T", g.Value)
	} else {
		if ue.Unit != "deg" {
			t.Errorf("value unit = %q, want %q", ue.Unit, "deg")
		}
		if !ue.IsAngle {
			t.Error("value: expected IsAngle = true")
		}
	}
	cr, ok := g.Constraint.(*parser.ConstrainedRange)
	if !ok {
		t.Fatalf("constraint: expected parser.ConstrainedRange, got %T", g.Constraint)
	}
	if cr.Unit != "deg" {
		t.Errorf("unit = %q, want %q", cr.Unit, "deg")
	}
}

func TestParseVarConstraintEnum(t *testing.T) {
	src := `var s = "m3" where ["m3", "m4", "m5"];
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	if _, ok := g.Value.(*parser.StringLit); !ok {
		t.Errorf("value: expected parser.StringLit, got %T", g.Value)
	}
	arr, ok := g.Constraint.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.ArrayLitExpr, got %T", g.Constraint)
	}
	if len(arr.Elems) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(arr.Elems))
	}
}

func TestParseVarConstraintStepped(t *testing.T) {
	src := `var stepped = 5 where [0:100:5];
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	rng, ok := g.Constraint.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.RangeExpr, got %T", g.Constraint)
	}
	if rng.Step == nil {
		t.Fatal("expected non-nil step")
	}
	step := rng.Step.(*parser.NumberLit)
	if step.Value != 5 {
		t.Errorf("step = %v, want 5", step.Value)
	}
}

func TestParseVarConstraintExclusive(t *testing.T) {
	src := `var exclusive = 5 where [0:<100];
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	rng, ok := g.Constraint.(*parser.RangeExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.RangeExpr, got %T", g.Constraint)
	}
	if !rng.Exclusive {
		t.Error("expected Exclusive = true")
	}
}

func TestParseVarConstraintFreeform(t *testing.T) {
	src := `var freeform = 42 where [];
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	arr, ok := g.Constraint.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("constraint: expected parser.ArrayLitExpr, got %T", g.Constraint)
	}
	if len(arr.Elems) != 0 {
		t.Errorf("expected 0 elements (free-form), got %d", len(arr.Elems))
	}
}

func TestParseVarNoConstraint(t *testing.T) {
	src := `var plain = 42;
fn Main() { return Cube(size: {x: 10 mm, y: 10 mm, z: 10 mm}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := prog.Globals()[0]
	if g.Constraint != nil {
		t.Errorf("expected nil constraint, got %T", g.Constraint)
	}
}

func TestParseVarConstraintLocal(t *testing.T) {
	src := `fn Main() {
		var x = 10 where [0:100];
		return Cube(size: {x: x, y: x, z: x});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	if v.Constraint == nil {
		t.Fatal("expected non-nil constraint on local var")
	}
	if _, ok := v.Constraint.(*parser.RangeExpr); !ok {
		t.Errorf("constraint: expected parser.RangeExpr, got %T", v.Constraint)
	}
}

func TestParseForYieldEnumerate(t *testing.T) {
	src := `fn Main() {
		var result = for i, v []Length[1 mm, 2 mm, 3 mm] {
			yield v;
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(fy.Clauses))
	}
	if fy.Clauses[0].Index != "i" {
		t.Errorf("index var = %q, want %q", fy.Clauses[0].Index, "i")
	}
	if fy.Clauses[0].Var != "v" {
		t.Errorf("loop var = %q, want %q", fy.Clauses[0].Var, "v")
	}
}

func TestParseForYieldEnumerateWithCartesian(t *testing.T) {
	src := `fn Main() {
		var result = for i, v [1, 2], j [0:<3] {
			yield v;
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if len(fy.Clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(fy.Clauses))
	}
	// First clause: enumerate
	if fy.Clauses[0].Index != "i" {
		t.Errorf("clause 0 index = %q, want %q", fy.Clauses[0].Index, "i")
	}
	if fy.Clauses[0].Var != "v" {
		t.Errorf("clause 0 var = %q, want %q", fy.Clauses[0].Var, "v")
	}
	// Second clause: regular (no enumerate)
	if fy.Clauses[1].Index != "" {
		t.Errorf("clause 1 index = %q, want empty", fy.Clauses[1].Index)
	}
	if fy.Clauses[1].Var != "j" {
		t.Errorf("clause 1 var = %q, want %q", fy.Clauses[1].Var, "j")
	}
}

func TestParseForYieldRegularNoIndex(t *testing.T) {
	// Existing syntax should still work — Index should be empty
	src := `fn Main() {
		var result = for v [1, 2, 3] {
			yield v;
		};
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[0].(*parser.VarStmt)
	fy, ok := v.Value.(*parser.ForYieldExpr)
	if !ok {
		t.Fatalf("expected parser.ForYieldExpr, got %T", v.Value)
	}
	if fy.Clauses[0].Index != "" {
		t.Errorf("index = %q, want empty", fy.Clauses[0].Index)
	}
	if fy.Clauses[0].Var != "v" {
		t.Errorf("var = %q, want %q", fy.Clauses[0].Var, "v")
	}
}

func TestParseIndexExpr(t *testing.T) {
	src := `fn Main() {
		var arr = []Number[10, 20, 30];
		var x = arr[0];
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[1].(*parser.VarStmt)
	idx, ok := v.Value.(*parser.IndexExpr)
	if !ok {
		t.Fatalf("expected parser.IndexExpr, got %T", v.Value)
	}
	recv, ok := idx.Receiver.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("receiver: expected parser.IdentExpr, got %T", idx.Receiver)
	}
	if recv.Name != "arr" {
		t.Errorf("receiver = %q, want %q", recv.Name, "arr")
	}
	num, ok := idx.Index.(*parser.NumberLit)
	if !ok {
		t.Fatalf("index: expected parser.NumberLit, got %T", idx.Index)
	}
	if num.Value != 0 {
		t.Errorf("index value = %v, want 0", num.Value)
	}
}

func TestParseIndexChained(t *testing.T) {
	src := `fn Main() {
		var arr = []Number[[1, 2], [3, 4]];
		var x = arr[0][1];
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[1].(*parser.VarStmt)
	outer, ok := v.Value.(*parser.IndexExpr)
	if !ok {
		t.Fatalf("expected outer parser.IndexExpr, got %T", v.Value)
	}
	inner, ok := outer.Receiver.(*parser.IndexExpr)
	if !ok {
		t.Fatalf("receiver: expected inner parser.IndexExpr, got %T", outer.Receiver)
	}
	_, ok = inner.Receiver.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("inner receiver: expected parser.IdentExpr, got %T", inner.Receiver)
	}
}

func TestParseIndexWithDot(t *testing.T) {
	// arr[0].x should parse as FieldAccess(Index(arr, 0), "x")
	src := `fn Main() {
		var pts = []Vec2[{x: 1 mm, y: 2 mm}];
		var x = pts[0].x;
		return Cube(size: {x: 10, y: 10, z: 10});
	}`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseUnitExprParen(t *testing.T) {
	// (1 + 2) mm should parse as UnitExpr wrapping a BinaryExpr
	src := `fn Main() { return Cylinder(bottom: (1 + 2) mm, top: 10, height: 10); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	naUP, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	ue, ok := naUP.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr value, got %T", naUP.Value)
	}
	if ue.Unit != "mm" {
		t.Errorf("unit = %q, want %q", ue.Unit, "mm")
	}
	if ue.IsAngle {
		t.Error("expected IsAngle = false")
	}
	if _, ok := ue.Expr.(*parser.BinaryExpr); !ok {
		t.Errorf("inner expr: expected parser.BinaryExpr, got %T", ue.Expr)
	}
}

func TestParseUnitExprCallResult(t *testing.T) {
	// Foo() mm should parse as UnitExpr wrapping a CallExpr
	src := `fn Foo() Number { 42 }
	fn Main() { return Cylinder(bottom: Foo() mm, top: 10, height: 10); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[1].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	naCR, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	ue, ok := naCR.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr value, got %T", naCR.Value)
	}
	if ue.Unit != "mm" {
		t.Errorf("unit = %q, want %q", ue.Unit, "mm")
	}
	inner, ok := ue.Expr.(*parser.CallExpr)
	if !ok {
		t.Fatalf("inner expr: expected parser.CallExpr, got %T", ue.Expr)
	}
	if inner.Name != "Foo" {
		t.Errorf("inner call name = %q, want %q", inner.Name, "Foo")
	}
}

func TestParseUnitExprAngle(t *testing.T) {
	// (90 / 2) deg should parse as UnitExpr with IsAngle=true
	src := `fn Main() { return Cube(size: {x: 10, y: 10, z: 10}).Rotate(rx: (90 / 2) deg, ry: 0 deg, rz: 0 deg); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	mc := ret.Value.(*parser.MethodCallExpr)
	naUA, ok := mc.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", mc.Args[0])
	}
	ue, ok := naUA.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr value, got %T", naUA.Value)
	}
	if ue.Unit != "deg" {
		t.Errorf("unit = %q, want %q", ue.Unit, "deg")
	}
	if !ue.IsAngle {
		t.Error("expected IsAngle = true")
	}
}

func TestParseUnitExprNoDoubleLiteral(t *testing.T) {
	// 5 mm should parse as UnitExpr wrapping NumberLit(5)
	src := `fn Main() { return Cylinder(bottom: 5 mm, top: 10, height: 10); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[0].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	naND, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	ue, ok := naND.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr value for '5 mm', got %T", naND.Value)
	}
	if ue.Unit != "mm" {
		t.Errorf("unit = %q, want %q", ue.Unit, "mm")
	}
	if ue.IsAngle {
		t.Error("expected IsAngle = false")
	}
	num, ok := ue.Expr.(*parser.NumberLit)
	if !ok {
		t.Fatalf("inner expr: expected parser.NumberLit, got %T", ue.Expr)
	}
	if num.Value != 5 {
		t.Errorf("value = %v, want 5", num.Value)
	}
}

func TestParseUnitExprVariable(t *testing.T) {
	// x mm should parse as UnitExpr wrapping IdentExpr
	src := `fn Main() {
		var x = 5;
		return Cylinder(bottom: x mm, top: 10, height: 10);
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ret := prog.Functions()[0].Body[1].(*parser.ReturnStmt)
	call := ret.Value.(*parser.CallExpr)
	naUV, ok := call.Args[0].(*parser.NamedArg)
	if !ok {
		t.Fatalf("expected parser.NamedArg, got %T", call.Args[0])
	}
	ue, ok := naUV.Value.(*parser.UnitExpr)
	if !ok {
		t.Fatalf("expected parser.UnitExpr value, got %T", naUV.Value)
	}
	if ue.Unit != "mm" {
		t.Errorf("unit = %q, want %q", ue.Unit, "mm")
	}
	ident, ok := ue.Expr.(*parser.IdentExpr)
	if !ok {
		t.Fatalf("inner: expected parser.IdentExpr, got %T", ue.Expr)
	}
	if ident.Name != "x" {
		t.Errorf("inner name = %q, want %q", ident.Name, "x")
	}
}

func TestParseAssignment(t *testing.T) {
	src := `fn Main() { var x = 10 mm; x = 20 mm; return Cube(size: {x: x, y: x, z: x}); }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn := prog.Functions()[0]
	if len(fn.Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(fn.Body))
	}
	assign, ok := fn.Body[1].(*parser.AssignStmt)
	if !ok {
		t.Fatalf("expected parser.AssignStmt, got %T", fn.Body[1])
	}
	if assign.Name != "x" {
		t.Errorf("name = %q, want %q", assign.Name, "x")
	}
}

func TestParseAssignmentRejectUnderscore(t *testing.T) {
	src := `fn Main() { _foo = 10; return Cube(size: {x: 1 mm, y: 1 mm, z: 1 mm}); }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected error for _-prefixed assignment")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error should mention reserved: %v", err)
	}
}

func TestParseCompoundAssignment(t *testing.T) {
	ops := []struct {
		op    string
		binOp string
	}{
		{"+=", "+"},
		{"-=", "-"},
		{"*=", "*"},
		{"/=", "/"},
		{"%=", "%"},
		{"^=", "^"},
	}
	for _, tc := range ops {
		src := fmt.Sprintf(`fn Main() { var x = 10 mm; x %s 5 mm; return Cube(size: {x: x, y: x, z: x}); }`, tc.op)
		prog, err := parser.Parse(src, "", parser.SourceUser)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.op, err)
		}
		fn := prog.Functions()[0]
		assign, ok := fn.Body[1].(*parser.AssignStmt)
		if !ok {
			t.Fatalf("%s: expected parser.AssignStmt, got %T", tc.op, fn.Body[1])
		}
		if assign.Name != "x" {
			t.Errorf("%s: name = %q, want %q", tc.op, assign.Name, "x")
		}
		bin, ok := assign.Value.(*parser.BinaryExpr)
		if !ok {
			t.Fatalf("%s: expected parser.BinaryExpr value, got %T", tc.op, assign.Value)
		}
		if bin.Op != tc.binOp {
			t.Errorf("%s: binary op = %q, want %q", tc.op, bin.Op, tc.binOp)
		}
	}
}

func TestParseBareArrayLiteral(t *testing.T) {
	src := `fn Main() { var x = [1, 2, 3]; }`
	s, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(s.Functions()) != 1 {
		t.Fatalf("expected 1 function, got %d", len(s.Functions()))
	}
	body := s.Functions()[0].Body
	if len(body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(body))
	}
	vs, ok := body[0].(*parser.VarStmt)
	if !ok {
		t.Fatalf("expected VarStmt, got %T", body[0])
	}
	arr, ok := vs.Value.(*parser.ArrayLitExpr)
	if !ok {
		t.Fatalf("expected ArrayLitExpr, got %T", vs.Value)
	}
	if arr.TypeName != "" {
		t.Errorf("expected empty TypeName, got %q", arr.TypeName)
	}
	if len(arr.Elems) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr.Elems))
	}
}

func TestParseReservedKeywords(t *testing.T) {
	reserved := []string{"while", "break", "continue", "match", "case", "import", "export", "map"}
	for _, kw := range reserved {
		t.Run(kw, func(t *testing.T) {
			src := fmt.Sprintf(`fn Main() { var %s = 10; }`, kw)
			_, err := parser.Parse(src, "", parser.SourceUser)
			if err == nil {
				t.Fatalf("expected error for reserved keyword %q, got none", kw)
			}
			if !strings.Contains(err.Error(), "reserved keyword") {
				t.Errorf("expected 'reserved keyword' error, got: %v", err)
			}
		})
	}
}

// ensure math import stays used
var _ = math.Pi
