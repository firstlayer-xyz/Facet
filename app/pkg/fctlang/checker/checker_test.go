package checker

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/stdlib"
	"strings"
	"testing"
)

// parse is a test helper wrapping the parser.
func parse(src string) (*parser.Source, error) {
	return parser.Parse(src, "", parser.SourceUser)
}

const testMainKey = "/test/main.fct"

// testStdlibLibs returns a Program with stdlib parsed and seeded.
func testStdlibLibs() loader.Program {
	prog := loader.NewProgram()
	stdSrc, err := parser.Parse(stdlib.StdlibSource, "", parser.SourceUser)
	if err != nil {
		panic("stdlib parse error: " + err.Error())
	}
	stdSrc.Path = loader.StdlibPath
	stdSrc.Text = stdlib.StdlibSource
	prog.Sources[loader.StdlibPath] = stdSrc
	return prog
}

// parseTestProg is a test helper that parses source and wraps it in a loader.Program.
func parseTestProg(t *testing.T, src string) loader.Program {
	t.Helper()
	s, err := parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s
	return prog
}

// inferVarTypesFromSource is a test helper that parses source and returns inferred var types for main.
func inferVarTypesFromSource(t *testing.T, src string) map[string]string {
	t.Helper()
	prog := parseTestProg(t, src)
	return Check(prog).VarTypes[testMainKey]
}

// helper to parse and check, returning errors
func checkSource(t *testing.T, src string) []parser.SourceError {
	t.Helper()
	prog := parseTestProg(t, src)
	return Check(prog).Errors
}

// helper: expect no errors
func expectNoErrors(t *testing.T, src string) {
	t.Helper()
	errs := checkSource(t, src)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d:", len(errs))
		for _, e := range errs {
			t.Errorf("  %v", e)
		}
	}
}

// helper: expect at least one error containing substr
func expectError(t *testing.T, src string, substr string) {
	t.Helper()
	errs := checkSource(t, src)
	if len(errs) == 0 {
		t.Fatalf("expected error containing %q, got none", substr)
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error containing %q, got:", substr)
		for _, e := range errs {
			t.Errorf("  %v", e)
		}
	}
}

// ---------------------------------------------------------------------------
// Valid programs produce no errors
// ---------------------------------------------------------------------------

func TestCheckBasicCube(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`)
}

func TestCheckInferredReturnType(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`)
}

func TestCheckFunctionWithTypedParams(t *testing.T) {
	expectNoErrors(t, `
fn Box(size Length) Solid {
    return Cube(size: Vec3{x: size, y: size, z: size});
}

fn Main() Solid {
    return Box(size: 10 mm);
}
`)
}

func TestCheckBinaryOps(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = 10 mm + 5 mm;
    var b = 10 mm - 5 mm;
    var c = 10 mm * 5 mm;
    var d = 10 mm / 5 mm;
    var e = 10 + 5;
    var f = 45 deg + 90 deg;
    var g = true && false;
    return Cube(size: Vec3{x: a, y: b, z: c});
}
`)
}

func TestCheckMethodChains(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
        .Translate(v: Vec3 { x: 5 mm, y: 0 mm, z: 0 mm })
        .Rotate(rx: 0 deg, ry: 45 deg, rz: 0 deg, pivot: WorldOrigin);
}
`)
}

func TestCheckSolidBooleanOps(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Sphere(radius: 5 mm);
    return a + b - Cylinder(bottom: 3 mm, top: 3 mm, height: 10 mm);
}
`)
}

func TestCheckSketchOps(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Square(x: 10 mm, y: 10 mm) + Circle(radius: 5 mm);
    return s.Extrude(height: 5 mm);
}
`)
}

func TestCheckNumberToLengthCoercion(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid { return Cube(size: Vec3{x: 10, y: 20, z: 30}); }`)
}

func TestCheckComparisons(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = 10 mm > 5 mm;
    var b = 10 < 20;
    var c = 45 deg == 45 deg;
    var d = true != false;
    if a { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
}
`)
}

func TestCheckForYield(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var pts = for i [0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * 5 mm, y: Sin(a: i * 60 deg) * 5 mm};
    };
    return Polygon(points: pts).Extrude(height: 5 mm);
}
`)
}

func TestCheckFold(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var cubes = []Solid[Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}), Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: 10 mm, y: 0 mm, z: 0 mm })];
    return fold a, b cubes {
        yield a + b;
    };
}
`)
}

func TestCheckGlobalVars(t *testing.T) {
	expectNoErrors(t, `
var size = 10 mm;

fn Main() {
    return Cube(size: Vec3{x: size, y: size, z: size});
}
`)
}

func TestCheckOptionalParams(t *testing.T) {
	expectNoErrors(t, `
fn Box(size Length, height Length = 5 mm) Solid {
    return Cube(size: Vec3{x: size, y: size, z: height});
}

fn Main() {
    return Box(size: 10 mm);
}
`)
}

func TestCheckOptionalParamOverride(t *testing.T) {
	expectNoErrors(t, `
fn Box(size Length, height Length = 5 mm) Solid {
    return Cube(size: Vec3{x: size, y: size, z: height});
}

fn Main() {
    return Box(size: 10 mm, height: 20 mm);
}
`)
}


func TestCheckIfElse(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = 10;
    if x > 5 {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Sphere(radius: 5 mm);
    }
}
`)
}

func TestCheckStruct(t *testing.T) {
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn Main() {
    var box = Dims { w: 10 mm, h: 20 mm, d: 5 mm };
    return Cube(size: Vec3{x: box.w, y: box.h, z: box.d});
}
`)
}

func TestCheckStructMethod(t *testing.T) {
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
    d Length;
}

fn Dims.ToCube() Solid {
    return Cube(size: Vec3{x: self.w, y: self.h, z: self.d});
}

fn Main() {
    var box = Dims { w: 10 mm, h: 20 mm, d: 5 mm };
    return box.ToCube();
}
`)
}

func TestCheckStructParam(t *testing.T) {
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
}

fn MakeBox(dims Dims) Solid {
    return Cube(size: Vec3{x: dims.w, y: dims.h, z: 10 mm});
}

fn Main() {
    var d = Dims { w: 5 mm, h: 8 mm };
    return MakeBox(dims: d);
}
`)
}

func TestCheckTrigFunctions(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Sin(a: 45 deg);
    var c = Cos(a: 45 deg);
    var t = Tan(a: 45 deg);
    var a = Asin(n: 1);
    var b = Acos(n: 0);
    var d = Atan2(y: 1, x: 0);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckMathFunctions(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Sqrt(n: 4);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckStringMethods(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = "hello";
    var l = Size(of: s);
    var sub = s.SubStr(start: 0, length: 3);
    var hp = s.HasPrefix(prefix: "he");
    var hs = s.HasSuffix(suffix: "lo");
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckAngArithmetic(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = 45 deg + 90 deg;
    var b = 90 deg - 45 deg;
    var c = 90 deg / 45 deg;
    var d = 45 deg * 2;
    var e = 2 * 45 deg;
    var f = 90 deg / 2;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckLengthDivLength(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var ratio = 20 mm / 10 mm;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckMathConstants(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = PI;
    var b = TAU;
    var c = E;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckLoadMesh(t *testing.T) {
	expectNoErrors(t, `
fn Main() Solid { return LoadMesh(path: "/tmp/model.stl"); }
`)
}

func TestCheckLibExprMissing(t *testing.T) {
	expectError(t, `
var T = lib "facet/gears";

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "not resolved")
}

func TestCheckSolidSlice(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return s.Slice(height: 5 mm).Extrude(height: 3 mm);
}
`)
}

func TestCheckSolidProject(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return s.Project().Extrude(height: 3 mm);
}
`)
}

func TestCheckSolidVolume(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var v = s.Volume();
    return s;
}
`)
}

func TestCheckSolidSurfaceArea(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var a = s.SurfaceArea();
    return s;
}
`)
}

func TestCheckSketchArea(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = Square(x: 10 mm, y: 10 mm);
    var a = s.Area();
    return s.Extrude(height: 5 mm);
}
`)
}

func TestCheckSphereDefaultSegments(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Sphere(radius: 10 mm); }`)
}

func TestCheckCylinderDefaultSegments(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cylinder(bottom: 5 mm, top: 5 mm, height: 10 mm); }`)
}

func TestCheckCircleDefaultSegments(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Circle(radius: 5 mm).Extrude(height: 10 mm); }`)
}

func TestCheckSketchFillet(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Square(x: 20 mm, y: 20 mm).Fillet(radius: 2 mm).Extrude(height: 5 mm);
}
`)
}

func TestCheckSketchChamfer(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Square(x: 20 mm, y: 20 mm).Chamfer(distance: 2 mm).Extrude(height: 5 mm);
}
`)
}

func TestCheckLinearPattern(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).LinearPattern(count: 3, spacingX: 10 mm, spacingY: 0 mm, spacingZ: 0 mm);
}
`)
}

func TestCheckCircularPattern(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: 10 mm, y: 0 mm, z: 0 mm }).CircularPattern(count: 6);
}
`)
}

func TestCheckSketchLinearPattern(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Square(x: 5 mm, y: 5 mm).LinearPattern(count: 3, spacingX: 10 mm, spacingY: 0 mm).Extrude(height: 5 mm);
}
`)
}

func TestCheckSketchCircularPattern(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Square(x: 5 mm, y: 5 mm).Translate(v: Vec2 { x: 10 mm, y: 0 mm }).CircularPattern(count: 4).Extrude(height: 5 mm);
}
`)
}

func TestCheckRevolve(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Circle(radius: 3 mm).Translate(v: Vec2 { x: 10 mm, y: 0 mm }).Revolve();
}
`)
}

func TestCheckRevolvePartial(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    return Circle(radius: 3 mm).Translate(v: Vec2 { x: 10 mm, y: 0 mm }).Revolve(degrees: 180 deg);
}
`)
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestCheckArgTypeMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    return Cylinder(bottom: 45 deg, top: 10 mm, height: 5 mm);
}
`, "must be Length, got Angle")
}

func TestCheckOperatorTypeMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm + 45 deg;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "incompatible types")
}

func TestCheckUndefinedVariable(t *testing.T) {
	expectError(t, `
fn Main() {
    return Cube(size: Vec3{x: x, y: 10 mm, z: 10 mm});
}
`, "undefined variable")
}

func TestCheckWrongArgCount(t *testing.T) {
	expectError(t, `
fn Main() {
    return Cube(size: 10 mm, extra: 20 mm);
}
`, `has no parameter named "extra"`)
}

func TestCheckMethodOnWrongReceiver(t *testing.T) {
	// Area() is a Sketch method, not a Solid method
	expectError(t, `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var a = s.Area();
    return s;
}
`, "has no method")
}

func TestCheckMethodOnBuiltInTypeRejected(t *testing.T) {
	// Cannot define methods on built-in types like Solid, String, Length
	expectError(t, `
fn Solid.MyMethod() Number { return 1 }
fn Main() Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) }
`, "cannot define method on type")
}

func TestCheckMethodOnStringRejected(t *testing.T) {
	expectError(t, `
fn String.Reverse() String { return self }
fn Main() Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) }
`, "cannot define method on type")
}

func TestCheckMethodOnUserStructAllowed(t *testing.T) {
	// Methods on user-defined types should be allowed
	expectNoErrors(t, `
type Box { w Length; h Length; d Length }
fn Box.Volume() Number { return 1 }
fn Main() Solid { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) }
`)
}

func TestCheckStructFieldTypeMismatch(t *testing.T) {
	expectError(t, `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 45 deg, h: 10 mm };
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`, "must be Length, got Angle")
}

func TestCheckStructMissingTypedFieldAllowed(t *testing.T) {
	// Typed fields get zero values when omitted
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 10 mm };
    return Cube(size: Vec3{x: d.w, y: d.w, z: 10 mm});
}
`)
}

func TestCheckStructUnknownField(t *testing.T) {
	expectError(t, `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 10 mm, h: 10 mm, z: 10 mm };
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`, "unknown field")
}

func TestCheckStructFieldAccess(t *testing.T) {
	expectError(t, `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 10 mm, h: 10 mm };
    return Cube(size: Vec3{x: d.z, y: d.h, z: 10 mm});
}
`, "has no field")
}

func TestCheckUnknownStruct(t *testing.T) {
	expectError(t, `
fn Main() {
    var d = Foo { x: 10 mm };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "unknown struct type")
}

func TestCheckIfCondNotBool(t *testing.T) {
	expectError(t, `
fn Main() {
    if 10 mm { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
}
`, "if condition must be a Bool")
}

func TestCheckForYieldIterNotArray(t *testing.T) {
	expectError(t, `
fn Main() {
    var pts = for i 10 {
        yield i;
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "expected Array to iterate over")
}

func TestCheckFoldIterNotArray(t *testing.T) {
	expectError(t, `
fn Main() {
    var result = fold a, b 10 {
        yield a + b;
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "expected Array to iterate over")
}

func TestCheckUnknownFunction(t *testing.T) {
	expectError(t, `
fn Main() {
    return DoSomething(x: 10 mm);
}
`, "unknown function")
}

func TestCheckLogicalOpTypeMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm && true;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "left operand must be Bool")
}

func TestCheckSolidOperatorMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var a = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Sphere(radius: 5 mm);
    var c = a * b;
    return a;
}
`, "incompatible types Solid and Solid")
}

func TestCheckSketchOperatorMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var a = Square(x: 10 mm, y: 10 mm);
    var b = Circle(radius: 5 mm);
    var c = a * b;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "incompatible types Sketch and Sketch")
}

func TestCheckAngleOperatorMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 45 deg * 90 deg;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "not supported on Angle")
}

func TestCheckComparisonMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm > 45 deg;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "incompatible types")
}

func TestCheckFieldAccessOnNonStruct(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm;
    var y = x.foo;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "cannot access field")
}

func TestCheckTooManyArgs(t *testing.T) {
	expectError(t, `
fn Main() {
    return Sphere(radius: 10 mm, segments: 32, extra: 99);
}
`, `has no parameter named "extra"`)
}

func TestCheckUserFuncArgTypeMismatch(t *testing.T) {
	expectError(t, `
fn Box(size Length) Solid {
    return Cube(size: Vec3{x: size, y: size, z: size});
}

fn Main() {
    return Box(size: 45 deg);
}
`, "must be Length, got Angle")
}

func TestCheckTooFewArgsOptional(t *testing.T) {
	expectError(t, `
fn Box(w, h Length, d Length = 5 mm) Solid {
    return Cube(size: Vec3{x: w, y: h, z: d});
}

fn Main() {
    return Box(w: 10 mm);
}
`, "expects 2 to 3 arguments, got 1")
}

// ---------------------------------------------------------------------------
// TypeUnknown propagation (no cascading errors)
// ---------------------------------------------------------------------------

func TestCheckUnknownNoCascade(t *testing.T) {
	// Missing library is reported, but should not cascade into type errors
	// for downstream usage of the unknown result.
	src := `
var T = lib "facet/gears";

fn Main() {
    var result = T.SomeFunction(x: 10 mm);
    return Cube(size: Vec3{x: result, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	errs := Check(prog).Errors
	// Should have the library load error
	hasLibErr := false
	for _, e := range errs {
		if strings.Contains(e.Message, "not resolved") {
			hasLibErr = true
		}
		// Should NOT have cascading arg type errors for Cube
		if strings.Contains(e.Message, "must be Length") {
			t.Errorf("unexpected cascading error: %v", e)
		}
	}
	if !hasLibErr {
		t.Error("expected 'not resolved' error")
	}
}

func TestCheckStringConcat(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var s = "hello" + " world";
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckStringUnsupportedOp(t *testing.T) {
	expectError(t, `
fn Main() {
    var s = "hello" - "world";
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "not supported on String")
}

func TestCheckBoolComparisonNotLT(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = true < false;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "incompatible types")
}

func TestCheckElseIfCondNotBool(t *testing.T) {
	expectError(t, `
fn Main() {
    if true {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else if 42 {
        return Sphere(radius: 5 mm);
    }
}
`, "else-if condition must be a Bool")
}

func TestCheckNumberAngleIncompatible(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 + 45 deg;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "incompatible types")
}

func TestCheckImplicitReturn(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }`)
}


func TestCheckImplicitReturnIfElse(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    if true {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Sphere(radius: 5 mm);
    }
}
`)
}

func TestCheckImplicitYieldForYield(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var pts = for i [0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * 5 mm, y: Sin(a: i * 60 deg) * 5 mm};
    };
    return Polygon(points: pts).Extrude(height: 5 mm);
}
`)
}

func TestCheckImplicitReturnFold(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var cubes = []Solid[Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}), Cube(size: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Translate(v: Vec3 { x: 10 mm, y: 0 mm, z: 0 mm })];
    return fold a, b cubes {
        yield a + b;
    };
}
`)
}

func TestCheckStructDuplicateField(t *testing.T) {
	expectError(t, `
type Dims {
    w Length;
    h Length;
}

fn Main() {
    var d = Dims { w: 10 mm, w: 20 mm, h: 10 mm };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "duplicate field")
}

func TestCheckAssertBoolCondition(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    assert true;
    assert 1 < 2;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckAssertNonBool(t *testing.T) {
	expectError(t, `
fn Main() {
    assert 42;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "assert condition must be a Bool")
}

func TestCheckUnaryMinusNumber(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = -10;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckUnaryMinusLength(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = -10 mm;
    return Cube(size: Vec3{x: x, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckUnaryMinusAngle(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = -45 deg;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckUnaryMinusOnBool(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = -true;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "unary minus not supported on Bool")
}

func TestCheckBooleanNot(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = !true;
    var y = !false;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckBooleanNotOnNumber(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = !42;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "operator ! requires Bool")
}

func TestCheckPow(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = Pow(base: 2, exp: 10);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckFloorCeilRound(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var a = Floor(n: 3.7);
    var b = Ceil(n: 3.2);
    var c = Round(n: 3.5);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

// ---------------------------------------------------------------------------
// Constraint type checking tests
// ---------------------------------------------------------------------------

func TestCheckConstraintLengthRangeValid(t *testing.T) {
	expectNoErrors(t, `
var w = 10 mm where [1:100] mm;
fn Main() {
    return Cube(size: Vec3{x: w, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckConstraintNumberRangeValid(t *testing.T) {
	expectNoErrors(t, `
var x = 50 where [0:100];
fn Main() {
    return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckConstraintAngleRangeValid(t *testing.T) {
	expectNoErrors(t, `
var a = 45 deg where [0:360] deg;
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckConstraintUnitMismatchLength(t *testing.T) {
	// mm unit on a Number variable should error
	expectError(t, `
var x = 50 where [0:100] mm;
fn Main() {
    return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`, "length unit, but variable is Number")
}

func TestCheckConstraintUnitMismatchAngle(t *testing.T) {
	// deg unit on a Length variable should error
	expectError(t, `
var w = 10 mm where [0:100] deg;
fn Main() {
    return Cube(size: Vec3{x: w, y: 10 mm, z: 10 mm});
}
`, "angle unit, but variable is Length")
}

func TestCheckConstraintEnumValid(t *testing.T) {
	expectNoErrors(t, `
var s = "m3" where ["m3", "m4", "m5"];
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckConstraintEnumTypeMismatch(t *testing.T) {
	expectError(t, `
var s = "m3" where [1, 2, 3];
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "constraint enum element")
}

func TestCheckConstraintFreeForm(t *testing.T) {
	expectNoErrors(t, `
var x = 42 where [];
fn Main() {
    return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckConstraintSteppedRange(t *testing.T) {
	expectNoErrors(t, `
var x = 10 where [0:100:5];
fn Main() {
    return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`)
}

// ---------------------------------------------------------------------------
// Array indexing type checking
// ---------------------------------------------------------------------------

func TestCheckArrayIndex(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var arr = []Length[5 mm, 10 mm, 15 mm];
    var x = arr[0];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckArrayIndexNotArray(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm;
    var y = x[0];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "cannot index Length")
}

func TestCheckArrayIndexNotNumber(t *testing.T) {
	expectError(t, `
fn Main() {
    var arr = []Number[1, 2, 3];
    var y = arr[5 mm];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "index must be a Number")
}

// ---------------------------------------------------------------------------
// Constraint value validation
// ---------------------------------------------------------------------------

func TestCheckConstraintValueOutOfRangeNumber(t *testing.T) {
	expectError(t, `
var x = 150 where [0:100];
fn Main() { return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm}); }
`, "out of range")
}

func TestCheckConstraintValueInRangeNumber(t *testing.T) {
	expectNoErrors(t, `
var x = 50 where [0:100];
fn Main() { return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm}); }
`)
}

func TestCheckConstraintValueOutOfRangeLength(t *testing.T) {
	expectError(t, `
var w = 200 mm where [1:100] mm;
fn Main() { return Cube(size: Vec3{x: w, y: 10 mm, z: 10 mm}); }
`, "out of range")
}

func TestCheckConstraintValueInRangeLength(t *testing.T) {
	expectNoErrors(t, `
var w = 50 mm where [1:100] mm;
fn Main() { return Cube(size: Vec3{x: w, y: 10 mm, z: 10 mm}); }
`)
}

func TestCheckConstraintValueOutOfRangeAngle(t *testing.T) {
	expectError(t, `
var a = 400 deg where [0:360] deg;
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: a, ry: 0 deg, rz: 0 deg, pivot: WorldOrigin); }
`, "out of range")
}

func TestCheckConstraintValueInRangeAngle(t *testing.T) {
	expectNoErrors(t, `
var a = 45 deg where [0:360] deg;
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Rotate(rx: a, ry: 0 deg, rz: 0 deg, pivot: WorldOrigin); }
`)
}

func TestCheckConstraintEnumStringNotInSet(t *testing.T) {
	expectError(t, `
var s = "m20" where ["m3", "m4", "m5"];
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
`, "not in the allowed set")
}

func TestCheckConstraintEnumStringInSet(t *testing.T) {
	expectNoErrors(t, `
var s = "m4" where ["m3", "m4", "m5"];
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
`)
}

func TestCheckConstraintExclusiveRange(t *testing.T) {
	expectError(t, `
var x = 100 where [0:<100];
fn Main() { return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm}); }
`, "out of range")
}

func TestCheckConstraintExclusiveRangeInRange(t *testing.T) {
	expectNoErrors(t, `
var x = 99 where [0:<100];
fn Main() { return Cube(size: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm}); }
`)
}

// ---------------------------------------------------------------------------
// Error positions — all errors must have line > 0 for editor squigglies
// ---------------------------------------------------------------------------

func TestCheckUndefinedVariableHasLineNumber(t *testing.T) {
	errs := checkSource(t, `
fn Main() {
    var x = foo;
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
	if len(errs) == 0 {
		t.Fatal("expected error for undefined variable")
	}
	for _, e := range errs {
		if e.Line == 0 {
			t.Errorf("error has line 0 (won't show squiggly): %s", e.Message)
		}
	}
}

func TestCheckIfConditionErrorHasLineNumber(t *testing.T) {
	errs := checkSource(t, `
fn Main() {
    var x = Cube(size: Vec3{x: 2 mm, y: 2 mm, z: 2 mm});
    if 42 { x = Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
    return x;
}
`)
	if len(errs) == 0 {
		t.Fatal("expected error for non-Bool if condition")
	}
	for _, e := range errs {
		if e.Line == 0 {
			t.Errorf("error has line 0 (won't show squiggly): %s", e.Message)
		}
	}
}

func TestCheckAssignmentValid(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = 10 mm;
    x = 20 mm;
    return Cube(size: Vec3{x: x, y: x, z: x});
}
`)
}

func TestCheckAssignmentTypeMismatch(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10 mm;
    x = true;
    return Cube(size: Vec3{x: x, y: x, z: x});
}
`, "cannot assign")
}

func TestCheckAssignmentUndefined(t *testing.T) {
	expectError(t, `
fn Main() {
    y = 10 mm;
    return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`, "undefined")
}

// ---------------------------------------------------------------------------
// Return-path analysis tests
// ---------------------------------------------------------------------------

func TestCheckMissingReturn(t *testing.T) {
	expectError(t, `
fn Foo() Number {
    var x = 5;
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "does not return on all code paths")
}

func TestCheckIfWithoutElse(t *testing.T) {
	expectError(t, `
fn Bar() Number {
    if true {
        return 1;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "does not return on all code paths")
}

func TestCheckIfElseAllReturn(t *testing.T) {
	expectNoErrors(t, `
fn Baz() Number {
    if true {
        return 1;
    } else {
        return 2;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckIfElseIfElseAllReturn(t *testing.T) {
	expectNoErrors(t, `
fn Choose(x Number) Number {
    if x > 10 {
        return 1;
    } else if x > 5 {
        return 2;
    } else {
        return 3;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckIfElseIfMissingElse(t *testing.T) {
	expectError(t, `
fn Choose(x Number) Number {
    if x > 10 {
        return 1;
    } else if x > 5 {
        return 2;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "does not return on all code paths")
}

func TestCheckForYieldNoReturnCheck(t *testing.T) {
	// For-yield functions use yield, not return — should not trigger the check
	expectNoErrors(t, `
fn Main() {
    var pts = for i [0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * 5 mm, y: Sin(a: i * 60 deg) * 5 mm};
    };
    return Polygon(points: pts).Extrude(height: 5 mm);
}
`)
}


func TestCheckVarsInsideIfBranches(t *testing.T) {
	// Variables defined inside if branches should not produce false
	// "undefined variable" errors during return type consistency checks.
	// Regression test for Moon/Blocks demo checker bug.
	expectNoErrors(t, `
fn MakePart(x Number) Solid {
    if x > 0 {
        var size = x * 10;
        return Cube(size: Vec3{x: size mm, y: size mm, z: size mm});
    } else {
        var half = x / 2;
        return Sphere(radius: half mm);
    }
}

fn Main() {
    return MakePart(x: 5);
}
`)
}

func TestCheckConsistentReturnTypes(t *testing.T) {
	expectNoErrors(t, `
fn Pick(x Number) Length {
    if x > 0 {
        return 10 mm;
    } else {
        return 20 mm;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckNumberLengthReturnCoercion(t *testing.T) {
	// Number → Length coercion should be allowed across return types
	expectNoErrors(t, `
fn Pick(x Number) {
    if x > 0 {
        return 10 mm;
    } else {
        return 20;
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckIfAsStatementNoFalsePositive(t *testing.T) {
	// If without else used as a statement (ExprStmt) should NOT trigger
	// "does not return on all code paths" when the function returns after it.
	expectNoErrors(t, `
fn Foo(flag Bool) Number {
    if flag {
        var x = 5;
    }
    return 10;
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckIfElseAsStatementNoFalsePositive(t *testing.T) {
	// If/else used as a statement (ExprStmt) followed by a return is fine.
	expectNoErrors(t, `
fn Foo(flag Bool) Number {
    if flag {
        var x = 5;
    } else {
        var y = 10;
    }
    return 20;
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckIfSideEffectThenReturn(t *testing.T) {
	// If with assignment side-effect, followed by returning the variable.
	expectNoErrors(t, `
fn Foo(flag Bool) Solid {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    if flag {
        s = s.Translate(v: Vec3 { x: 5 mm, y: 0 mm, z: 0 mm });
    }
    return s;
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckAnonymousStructReturn(t *testing.T) {
	// Returning anonymous structs should work.
	expectNoErrors(t, `
fn Foo() {
    return { x: 5 mm, y: 10 mm };
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckAnonymousStructConsistentReturn(t *testing.T) {
	// Multiple returns of anonymous structs should be consistent (both typeStruct).
	expectNoErrors(t, `
fn Foo(flag Bool) {
    if flag {
        return { x: 5 mm };
    } else {
        return { y: 10 mm };
    }
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

func TestCheckNamedStructWithNumberCoercion(t *testing.T) {
	// Named struct fields should accept Number→Length coercion.
	expectNoErrors(t, `
type Pt {
    x Length;
    y Length;
}

fn MakePt(n Number) Pt {
    return Pt { x: n, y: n };
}

fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`)
}

// ---------------------------------------------------------------------------
// InferVarTypes — unannotated function return types
// ---------------------------------------------------------------------------

func TestInferVarTypesUnannotatedReturnSolid(t *testing.T) {
	src := `
fn MyFunc() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}

fn Main() {
    var s = MyFunc();
    return s;
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["s"]; !ok || got != "Solid" {
		t.Errorf("expected s → Solid, got %q (ok=%v)", got, ok)
	}
}

func TestInferVarTypesUnannotatedReturnSketch(t *testing.T) {
	src := `
fn MySketch() {
    return Square(x: 10 mm, y: 10 mm);
}

fn Main() {
    var s = MySketch();
    return s.Extrude(height: 5 mm);
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["s"]; !ok || got != "Sketch" {
		t.Errorf("expected s → Sketch, got %q (ok=%v)", got, ok)
	}
}

func TestInferVarTypesUnannotatedReturnStruct(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

fn MakeDims() {
    return Dims { w: 10 mm, h: 20 mm };
}

fn Main() {
    var d = MakeDims();
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["d"]; !ok || got != "Dims" {
		t.Errorf("expected d → Dims, got %q (ok=%v)", got, ok)
	}
}

// ---------------------------------------------------------------------------
// InferVarTypes — reassigned variables
// ---------------------------------------------------------------------------

func TestInferVarTypesReassignment(t *testing.T) {
	src := `
fn Main() {
    var s = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    s = Sphere(radius: 5 mm);
    return s;
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["s"]; !ok || got != "Solid" {
		t.Errorf("expected s → Solid, got %q (ok=%v)", got, ok)
	}
}

func TestInferVarTypesAnnotatedReturnUnchanged(t *testing.T) {
	// Annotated functions should still work as before
	src := `
fn MyFunc() Solid {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}

fn Main() {
    var s = MyFunc();
    return s;
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["s"]; !ok || got != "Solid" {
		t.Errorf("expected s → Solid, got %q (ok=%v)", got, ok)
	}
}

func TestInferVarTypesStructMethodChain(t *testing.T) {
	// Variable from a struct-returning function, then calling a method on it
	src := `
type Dims {
    w Length;
    h Length;
}

fn MakeDims() Dims {
    return Dims { w: 10 mm, h: 20 mm };
}

fn Dims.ToCube() Solid {
    return Cube(size: Vec3{x: self.w, y: self.h, z: 10 mm});
}

fn Main() {
    var d = MakeDims();
    var s = d.ToCube();
    return s;
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["d"]; !ok || got != "Dims" {
		t.Errorf("expected d → Dims, got %q (ok=%v)", got, ok)
	}
	if got, ok := vt["s"]; !ok || got != "Solid" {
		t.Errorf("expected s → Solid, got %q (ok=%v)", got, ok)
	}
}

func TestCheckStructMethodCallFromFuncReturn(t *testing.T) {
	// Calling a method on a variable that holds a struct returned by a function
	// should not produce type errors
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
}

fn MakeDims() Dims {
    return Dims { w: 10 mm, h: 20 mm };
}

fn Dims.ToCube() Solid {
    return Cube(size: Vec3{x: self.w, y: self.h, z: 10 mm});
}

fn Main() {
    var d = MakeDims();
    return d.ToCube();
}
`)
}

// ---------------------------------------------------------------------------
// Global struct variable type propagation
// ---------------------------------------------------------------------------

func TestCheckGlobalStructFieldAccess(t *testing.T) {
	// Global struct vars should propagate struct names so field access works
	expectNoErrors(t, `
type Dims {
    w Length;
    h Length;
}

var d = Dims { w: 10 mm, h: 20 mm };

fn Main() {
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`)
}

func TestCheckGlobalStructInvalidField(t *testing.T) {
	expectError(t, `
type Dims {
    w Length;
    h Length;
}

var d = Dims { w: 10 mm, h: 20 mm };

fn Main() {
    return Cube(size: Vec3{x: d.z, y: d.h, z: 10 mm});
}
`, "has no field")
}

func TestInferVarTypesGlobalStruct(t *testing.T) {
	src := `
type Dims {
    w Length;
    h Length;
}

var d = Dims { w: 10 mm, h: 20 mm };

fn Main() {
    return Cube(size: Vec3{x: d.w, y: d.h, z: 10 mm});
}
`
	vt := inferVarTypesFromSource(t, src)
	if vt == nil {
		t.Fatal("InferVarTypes returned nil")
	}
	if got, ok := vt["d"]; !ok || got != "Dims" {
		t.Errorf("expected d → Dims, got %q (ok=%v)", got, ok)
	}
}

// ---------------------------------------------------------------------------
// Library struct name collision tests
// ---------------------------------------------------------------------------

// setupCollisionChecker creates a checker with two mock libraries that both
// define a struct named "Config" (with different fields) and methods on it.
func setupCollisionChecker() (*checker, *typeEnv) {
	// User program with two library imports
	s, _ := parse(`
var A = lib "fake/libA";
var B = lib "fake/libB";
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
`)
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s

	// Mock library A: struct Config with field "width"
	libA, _ := parse(`
type Config {
    width Length;
}
fn MakeConfig(w Length) Config {
    return Config { width: w };
}
fn Config.GetWidth() Length {
    return self.width;
}
`)
	// Mock library B: struct Config with field "height"
	libB, _ := parse(`
type Config {
    height Length;
}
fn MakeConfig(h Length) Config {
    return Config { height: h };
}
fn Config.GetHeight() Length {
    return self.height;
}
`)

	prog.Sources["fake/libA"] = libA
	prog.Sources["fake/libB"] = libB

	c := initChecker(prog)
	c.currentSrcKey = testMainKey
	c.inferredReturns = make(map[string]typeInfo)
	c.inferredReturnStructs = make(map[string]string)
	c.libVarToPath["A"] = "fake/libA"
	c.libVarToPath["B"] = "fake/libB"

	// Phase 1: register struct declarations with qualified names only
	for _, entry := range []struct{ name, path string }{{"A", "fake/libA"}, {"B", "fake/libB"}} {
		libProg := prog.Sources[entry.path]
		for _, sd := range libProg.StructDecls {
			qualified := entry.name + "." + sd.Name
			c.structDecls[qualified] = sd
		}
	}
	// Phase 2: register methods with qualified receiver names only
	for _, entry := range []struct{ name, path string }{{"A", "fake/libA"}, {"B", "fake/libB"}} {
		libProg := prog.Sources[entry.path]
		for _, fn := range libProg.Functions {
			if fn.ReceiverType != "" {
				qualified := entry.name + "." + fn.ReceiverType
				c.stdMethods[qualified] = append(c.stdMethods[qualified], fn)
			}
		}
	}

	env := c.seedGlobalEnv()
	env.set("A", simple(typeLibrary))
	env.set("B", simple(typeLibrary))
	return c, env
}

func TestCollisionBareNameRegistered(t *testing.T) {
	c, _ := setupCollisionChecker()
	// Bare "Config" IS registered (last source wins), plus qualified entries
	if _, ok := c.structDecls["Config"]; !ok {
		t.Error("bare Config should exist (from one of the libraries)")
	}
	if _, ok := c.structDecls["A.Config"]; !ok {
		t.Error("A.Config should exist in structDecls")
	}
	if _, ok := c.structDecls["B.Config"]; !ok {
		t.Error("B.Config should exist in structDecls")
	}
}

func TestCollisionQualifiedFieldAccess(t *testing.T) {
	c, env := setupCollisionChecker()

	// Simulate: var a = A.MakeConfig(10 mm);
	// resolveStructName for A.MakeConfig should return "A.Config"
	mcExpr := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "A"},
		Method:   "MakeConfig",
	}
	sn := c.resolveStructName(mcExpr, env)
	if sn != "A.Config" {
		t.Fatalf("expected A.Config, got %q", sn)
	}

	// Set up variable with qualified struct name
	env.set("a", structTI("A.Config"))

	// Field access: a.width should succeed (A.Config has "width")
	faExpr := &parser.FieldAccessExpr{
		Receiver: &parser.IdentExpr{Name: "a"},
		Field:    "width",
		Pos:      parser.Pos{1, 1},
	}
	ft := c.inferExpr(faExpr, env)
	if ft.ft != typeLength {
		t.Errorf("a.width should be Length, got %s", ft.displayName())
	}

	// Field access: a.height should fail (A.Config has no "height")
	faExpr2 := &parser.FieldAccessExpr{
		Receiver: &parser.IdentExpr{Name: "a"},
		Field:    "height",
		Pos:      parser.Pos{1, 1},
	}
	ft2 := c.inferExpr(faExpr2, env)
	if ft2.ft != typeUnknown {
		t.Errorf("a.height should be unknown (wrong struct), got %s", ft2.displayName())
	}
	// Check that error message uses bare name, not qualified
	found := false
	for _, e := range c.errors {
		if strings.Contains(e.Message, "Config has no field") && !strings.Contains(e.Message, "A.Config") {
			found = true
		}
	}
	if !found {
		t.Error("expected error with bare 'Config' (not 'A.Config') for missing field")
		for _, e := range c.errors {
			t.Logf("  error: %s", e.Message)
		}
	}
}

func TestCollisionMethodDispatch(t *testing.T) {
	c, env := setupCollisionChecker()

	// Set up variables with qualified struct names
	env.set("a", structTI("A.Config"))
	env.set("b", structTI("B.Config"))

	// a.GetWidth() should succeed (A.Config method)
	mc1 := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "a"},
		Method:   "GetWidth",
		Pos:      parser.Pos{1, 1},
	}
	rt1 := c.checkMethodCall(mc1, env)
	if rt1.ft != typeLength {
		t.Errorf("a.GetWidth() should return Length, got %s", rt1.displayName())
	}

	// a.GetHeight() should fail (that's B.Config's method)
	errsBefore := len(c.errors)
	mc2 := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "a"},
		Method:   "GetHeight",
		Pos:      parser.Pos{1, 1},
	}
	c.checkMethodCall(mc2, env)
	if len(c.errors) <= errsBefore {
		t.Error("a.GetHeight() should produce an error (wrong struct's method)")
	}

	// b.GetHeight() should succeed (B.Config method)
	mc3 := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "b"},
		Method:   "GetHeight",
		Pos:      parser.Pos{1, 1},
	}
	rt3 := c.checkMethodCall(mc3, env)
	if rt3.ft != typeLength {
		t.Errorf("b.GetHeight() should return Length, got %s", rt3.displayName())
	}
}

func TestCollisionAutocompleteUsesBareNames(t *testing.T) {
	c, env := setupCollisionChecker()

	// Record a variable with qualified struct name
	env.set("a", structTI("A.Config"))
	c.recordVarType("a", env)

	// Autocomplete should show bare name "Config", not "A.Config"
	if got := c.srcVarTypes()["a"]; got != "Config" {
		t.Errorf("autocomplete should use bare name Config, got %q", got)
	}
}

// setupNonCollidingChecker creates a checker with a single library struct.
// Verifies backward compatibility: non-colliding library structs still
// work with bare names in user function params.
func setupNonCollidingChecker() (*checker, *typeEnv) {
	s, _ := parse(`
var T = lib "fake/lib";
fn Main() { return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}); }
`)
	libProg, _ := parse(`
type Widget {
    size Length;
}
fn MakeWidget(s Length) Widget {
    return Widget { size: s };
}
fn Widget.GetSize() Length {
    return self.size;
}
`)
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s
	prog.Sources["fake/lib"] = libProg

	c := initChecker(prog)
	c.currentSrcKey = testMainKey
	c.inferredReturns = make(map[string]typeInfo)
	c.inferredReturnStructs = make(map[string]string)
	c.libVarToPath["T"] = "fake/lib"

	for _, sd := range libProg.StructDecls {
		qualified := "T." + sd.Name
		c.structDecls[qualified] = sd
	}
	for _, fn := range libProg.Functions {
		if fn.ReceiverType != "" {
			qualified := "T." + fn.ReceiverType
			c.stdMethods[qualified] = append(c.stdMethods[qualified], fn)
		}
	}

	env := c.seedGlobalEnv()
	env.set("T", simple(typeLibrary))
	return c, env
}

func TestNonCollidingBareNameNotRegistered(t *testing.T) {
	c, _ := setupNonCollidingChecker()
	// Bare name IS registered (all sources are equal)
	if _, ok := c.structDecls["Widget"]; !ok {
		t.Error("bare Widget should exist")
	}
	if _, ok := c.structDecls["T.Widget"]; !ok {
		t.Error("T.Widget should exist")
	}
}

func TestNonCollidingMethodsViaBothKeys(t *testing.T) {
	c, env := setupNonCollidingChecker()

	// Set up variable with qualified struct name (as resolveStructName returns)
	env.set("w", structTI("T.Widget"))

	// Method call should work via qualified key in stdMethods
	mc := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "w"},
		Method:   "GetSize",
		Pos:      parser.Pos{1, 1},
	}
	rt := c.checkMethodCall(mc, env)
	if rt.ft != typeLength {
		t.Errorf("w.GetSize() should return Length, got %s", rt.displayName())
	}
}

func TestLibReturnStructMethodDispatch(t *testing.T) {
	// Verify that calling a library function that returns a struct gives
	// the variable a qualified struct name, so method dispatch works.
	c, env := setupNonCollidingChecker()

	// Simulate: var w = T.MakeWidget(10 mm);
	mcExpr := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "T"},
		Method:   "MakeWidget",
		Args:     []parser.Expr{&parser.NumberLit{Value: 10}},
		Pos:      parser.Pos{1, 1},
	}
	retType := c.checkMethodCall(mcExpr, env)
	if retType.ft != typeStruct {
		t.Fatalf("T.MakeWidget() should return struct, got %s", retType.displayName())
	}
	if retType.structName != "T.Widget" {
		t.Fatalf("T.MakeWidget() should return T.Widget, got %q", retType.structName)
	}

	// Assign to variable
	env.set("w", retType)

	// w.GetSize() should work — method dispatch uses qualified name
	mc := &parser.MethodCallExpr{
		Receiver: &parser.IdentExpr{Name: "w"},
		Method:   "GetSize",
		Pos:      parser.Pos{2, 1},
	}
	errsBefore := len(c.errors)
	rt := c.checkMethodCall(mc, env)
	if rt.ft != typeLength {
		t.Errorf("w.GetSize() should return Length, got %s", rt.displayName())
	}
	if len(c.errors) > errsBefore {
		t.Errorf("w.GetSize() should not produce errors, got: %v", c.errors[errsBefore:])
	}
}

func TestArrayTypeInference(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // expected varTypes["x"]
	}{
		{
			name: "homogeneous Number[]",
			src:  `var x = []Number[1, 2, 3]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[]Number",
		},
		{
			name: "homogeneous Length[]",
			src:  `var x = []Length[1 mm, 2 mm]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[]Length",
		},
		{
			name: "Number+Length coercion to Length[]",
			src:  `var x = []Length[1, 2 mm, 3]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[]Length",
		},
		{
			name: "homogeneous 2D Number[][]",
			src:  `var x = []Number[[1, 2], [3, 4]]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[][]Number",
		},
		{
			name: "empty array",
			src:  `var x = []Number[]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[]Number",
		},
		{
			name: "homogeneous Bool[]",
			src:  `var x = []Bool[true, false, true]; fn Main() Solid { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`,
			want: "[]Bool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := parseTestProg(t, tt.src)
			c := initChecker(prog)
			c.currentSrcKey = testMainKey
			c.inferredReturns = make(map[string]typeInfo)
			c.inferredReturnStructs = make(map[string]string)
			env := c.seedGlobalEnv()
			for _, g := range prog.Sources[testMainKey].Globals {
				ti := c.inferExpr(g.Value, env)
				env.set(g.Name, ti)
				c.recordVarType(g.Name, env)
			}
			got := c.srcVarTypes()["x"]
			if got != tt.want {
				t.Errorf("varTypes[x] = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckInferredArrayNumber(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = [1, 2, 3];
    return Cube(size: Vec3{x: x[0] * 1 mm, y: 1 mm, z: 1 mm});
}
`)
}

func TestCheckInferredArraySolid(t *testing.T) {
	expectNoErrors(t, `
fn Main() []Solid {
    return [Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}), Cube(size: Vec3{x: 2 mm, y: 2 mm, z: 2 mm})];
}
`)
}

func TestCheckInferredArrayNested(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var x = [[1, 2], [3, 4]];
    return Cube(size: Vec3{x: x[0][0] * 1 mm, y: 1 mm, z: 1 mm});
}
`)
}

func TestCheckInferredArrayMixedError(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = [1, "hello"];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "array has mixed types")
}

func TestCheckHeterogeneousArrayError(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = [1, "a"];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "array has mixed types")
}

func TestCheckInferredArrayMixedNumberSolid(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = [1, Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "array has mixed types")
}

func TestCheckInferredArrayNestedInconsistent(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = [[1, 2], [3, "a"]];
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`, "array has mixed types")
}

func TestCheckInferredArrayWrongTypeForParam(t *testing.T) {
	expectError(t, `
fn Foo(items []Solid) {
    return items[0];
}
fn Main() {
    return Foo(items: [1, 2, 3]);
}
`, "must be []Solid")
}

func TestCheckInferredArrayNumberLengthPromotion(t *testing.T) {
	// Number and Length in same array should promote to []Length, not error
	expectNoErrors(t, `
fn Main() {
    var x = [1 mm, 2];
    return Cube(size: Vec3{x: x[0], y: x[1] * 1 mm, z: 1 mm});
}
`)
}

func TestCheckNumberToLengthCoercionParam(t *testing.T) {
	// Bare number coerced to Length when function param expects Length
	expectNoErrors(t, `
fn Box(w Length) {
    return Cube(size: Vec3{x: w, y: 1 mm, z: 1 mm});
}
fn Main() {
    return Box(w: 10);
}
`)
}

func TestCheckNumberToLengthCoercionOperator(t *testing.T) {
	// Bare number coerced to Length in mixed arithmetic
	expectNoErrors(t, `
fn Main() {
    var x = 5 mm + 2;
    return Cube(size: Vec3{x: x, y: 1 mm, z: 1 mm});
}
`)
}

func TestCheckNumberToAngleCoercionParam(t *testing.T) {
	// Bare number coerced to Angle when function param expects Angle
	expectNoErrors(t, `
fn Twist(a Angle) {
    return Cube(size: 10 mm).Rotate(rx: a, ry: 0 deg, rz: 0 deg, pivot: WorldOrigin);
}
fn Main() {
    return Twist(a: 45);
}
`)
}

func TestCheckShadowOuterVariable(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10;
    if true {
        var x = 20;
    }
    return Cube(size: x * 1 mm);
}
`, "shadows outer variable")
}

func TestCheckShadowParameter(t *testing.T) {
	expectError(t, `
fn Main(size Length = 10 mm where [1:50] mm) {
    if true {
        var size = 20 mm;
    }
    return Cube(size: size);
}
`, "shadows outer variable")
}

func TestCheckShadowType(t *testing.T) {
	expectError(t, `
type Dims {
    w Length
}
fn Main() {
    var Dims = 10;
    return Cube(size: Dims * 1 mm);
}
`, "shadows type")
}

func TestCheckNoShadowFunctionName(t *testing.T) {
	// Shadowing a function name with a variable is allowed
	expectNoErrors(t, `
fn Helper() {
    return Cube(size: 10 mm);
}
fn Main() {
    var Helper = 5;
    return Cube(size: Helper * 1 mm);
}
`)
}

func TestCheckSameScope(t *testing.T) {
	expectError(t, `
fn Main() {
    var x = 10;
    var x = 20;
    return Cube(size: x * 1 mm);
}
`, "already defined in this scope")
}

func TestCheckNoShadowSameLevel(t *testing.T) {
	// Two variables in sibling blocks don't shadow each other
	expectNoErrors(t, `
fn Main() {
    if true {
        var x = 10;
    }
    if true {
        var x = 20;
    }
    return Cube(size: 10 mm);
}
`)
}

func TestCheckPositionalStructLit(t *testing.T) {
	expectNoErrors(t, `
type Foo {
    a Number
    b Length
}
fn Main() {
    var f = Foo { a: 1, b: 2 mm };
    return Cube(size: Vec3{x: f.b, y: f.b, z: f.b});
}
`)
}

func TestCheckPositionalStructLitTooMany(t *testing.T) {
	expectError(t, `
type Foo {
    a Number
    b Length
}
fn Main() {
    var f = Foo { a: 1, b: 2 mm, c: 3 };
    return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`, "unknown field")
}

func TestCheckPositionalStructLitMissingTypedFieldAllowed(t *testing.T) {
	// Typed fields get zero values when omitted
	expectNoErrors(t, `
type Foo {
    a Number
    b Length
    c Length
}
fn Main() {
    var f = Foo { a: 1, b: 2 mm };
    return Cube(size: Vec3{x: f.a * 1 mm, y: 1 mm, z: 1 mm});
}
`)
}

func TestCheckPositionalStructLitWithDefaults(t *testing.T) {
	expectNoErrors(t, `
type Foo {
    a Number
    b Length
    c Length = 5 mm
}
fn Main() {
    var f = Foo { a: 1, b: 2 mm };
    return Cube(size: Vec3{x: f.b, y: f.b, z: f.b});
}
`)
}

func TestCheckTypedArrayConstructor(t *testing.T) {
	expectNoErrors(t, `
type Pt {
    x Number
    y Number
}
fn Main() {
    var pts = []Pt[{ x: 1, y: 2 }, { x: 3, y: 4 }];
    return Cube(size: Vec3{x: pts[0].x * 1 mm, y: pts[0].y * 1 mm, z: 1 mm});
}
`)
}

func TestCheckTypedArrayConstructorNamedFields(t *testing.T) {
	expectNoErrors(t, `
type Pt {
    x Number
    y Number
}
fn Main() {
    var pts = []Pt[{ x: 1, y: 2 }, { x: 3, y: 4 }];
    return Cube(size: Vec3{x: pts[0].x * 1 mm, y: pts[0].y * 1 mm, z: 1 mm});
}
`)
}

// ---------------------------------------------------------------------------
// Overload resolution
// ---------------------------------------------------------------------------

func TestCheckOverloadMinNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: Min(a: 5, b: 3) * 1 mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadMinLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: Min(a: 5 mm, b: 3 mm), y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadMinMixed(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: Min(a: 5, b: 3 mm), y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadAbsAngle(t *testing.T) {
	expectNoErrors(t, `fn Main() { var a = Abs(a: -45 deg); return Cube(size: Vec3{x: Sin(a: a) * 10 mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadLerpLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { return Cube(size: Vec3{x: Lerp(from: 0 mm, to: 20 mm, t: 0.5), y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadNumberFromLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var n = Number(from: 10 mm); return Cube(size: Vec3{x: n mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadNumberFromString(t *testing.T) {
	expectNoErrors(t, `fn Main() { var n = Number(from: "10"); return Cube(size: Vec3{x: n mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadStringFromBool(t *testing.T) {
	expectNoErrors(t, `fn Main() { var s = String(a: true); assert s == "true"; return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadSizeString(t *testing.T) {
	expectNoErrors(t, `fn Main() { var n = Size(of: "hello"); return Cube(size: Vec3{x: n mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadSizeArray(t *testing.T) {
	expectNoErrors(t, `fn Main() { var n = Size(of: []Number[1, 2, 3]); return Cube(size: Vec3{x: n mm, y: 1 mm, z: 1 mm}); }`)
}

func TestCheckOverloadNoMatchError(t *testing.T) {
	expectError(t, `
fn Foo(a Number) Number { return a; }
fn Foo(a Length) Length { return a; }
fn Main() { Foo(a: true); return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`, "no matching overload")
}

func TestCheckOverloadUserDefined(t *testing.T) {
	expectNoErrors(t, `
fn Double(a Number) Number { return a * 2; }
fn Double(a Length) Length { return a * 2; }
fn Main() { return Cube(size: Vec3{x: Double(a: 5 mm), y: 1 mm, z: 1 mm}); }
`)
}

func TestCheckOverloadNumberToAngleCoercion(t *testing.T) {
	// Bare number should be accepted where Angle is expected (Number→Angle coercion)
	expectNoErrors(t, `
fn Foo(a Angle) Angle { return a; }
fn Main() { var a = Foo(a: 45); return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`)
}

// ---------------------------------------------------------------------------
// Duplicate function detection tests
// ---------------------------------------------------------------------------

func TestCheckDuplicateFunctions(t *testing.T) {
	expectError(t, `
fn A() { }
fn A() { }
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`, "function A() has ambiguous signature")
}

func TestCheckDuplicateFunctionsWithParams(t *testing.T) {
	expectError(t, `
fn A(x Length = 1 mm) { }
fn A(x Length = 1 mm) { }
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`, "function A() has ambiguous signature")
}

func TestCheckLegitimateOverload(t *testing.T) {
	// Different required param counts with no overlap — no ambiguity
	expectNoErrors(t, `
fn A(x Length) Length { return x; }
fn A(x Length, y Length) Length { return x + y; }
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`)
}

func TestCheckDefaultParamAmbiguity(t *testing.T) {
	// fn A() and fn A(x Length = 1mm) overlap at arity 0
	expectError(t, `
fn A() Length { return 1 mm; }
fn A(x Length = 1 mm) Length { return x; }
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`, "function A() has ambiguous signature")
}

func TestCheckDuplicateMethods(t *testing.T) {
	expectError(t, `
type Foo { x Length; }
fn Foo.Bar() Length { return self.x; }
fn Foo.Bar() Length { return self.x; }
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`, "method Foo.Bar() has ambiguous signature")
}

// ---------------------------------------------------------------------------
// Library unknown param type tests
// ---------------------------------------------------------------------------

func TestCheckLibraryUnknownParamType(t *testing.T) {
	libSrc, _ := parse(`
fn A(x Whatever = 1) { }
`)
	libs := testStdlibLibs()
	libs.Sources["fake/mylib"] = libSrc

	s, _ := parse(`
var mylib = lib "fake/mylib";
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`)
	libs.Sources[testMainKey] = s
	prog := libs
	errs := Check(prog).Errors
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, `A() parameter "x" has unknown type "Whatever"`) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about unknown library param type, got: %v", errs)
	}
}

func TestCheckLibraryKnownParamType(t *testing.T) {
	libSrc, _ := parse(`
fn A(x Length = 1 mm) { }
`)
	libs := testStdlibLibs()
	libs.Sources["fake/mylib"] = libSrc

	s, _ := parse(`
var mylib = lib "fake/mylib";
fn Main() { return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}); }
`)
	libs.Sources[testMainKey] = s
	prog := libs
	errs := Check(prog).Errors
	for _, e := range errs {
		if strings.Contains(e.Message, "unknown type") {
			t.Errorf("unexpected error: %v", e)
		}
	}
}

func TestCheckInferredArrayFuncArgCoercion(t *testing.T) {
	expectNoErrors(t, `
type Point {
    x Length
    y Length
}
fn UsePoints(points []Point) {
    return Cube(size: Vec3{x: points[0].x, y: points[0].y, z: 1 mm});
}
fn Main() {
    return UsePoints(points: [Point{x: 1 mm, y: 2 mm}, Point{x: 3 mm, y: 4 mm}]);
}
`)
}

func TestCheckInferredArrayFuncArgAnonStruct(t *testing.T) {
	expectNoErrors(t, `
type Point {
    x Length
    y Length
}
fn UsePoints(points []Point) {
    return Cube(size: Vec3{x: points[0].x, y: points[0].y, z: 1 mm});
}
fn Main() {
    return UsePoints(points: []Point[{}, {x: 1 mm}]);
}
`)
}

func TestCheckInferredArrayReturnCoercion(t *testing.T) {
	expectNoErrors(t, `
type Point {
    x Length
    y Length
}
fn MakePoints() []Point {
    return [{x: 1 mm, y: 2 mm}, {x: 3 mm, y: 4 mm}];
}
fn Main() {
    var pts = MakePoints();
    return Cube(size: Vec3{x: pts[0].x, y: pts[0].y, z: 1 mm});
}
`)
}

func TestCheckLambdaTypeMatch(t *testing.T) {
	// Lambda with matching signature should be accepted as function arg
	expectNoErrors(t, `
fn Transform(s Solid, f fn(Solid) Solid) Solid {
    return s;
}
fn Main() {
    return Transform(s: Cube(size: 10 mm), f: fn(s Solid) Solid { return s });
}
`)
}

func TestCheckLambdaTypeMismatch(t *testing.T) {
	// Lambda with wrong param type should error
	expectError(t, `
fn Transform(s Solid, f fn(Solid) Solid) Solid {
    return s;
}
fn Main() {
    return Transform(s: Cube(size: 10 mm), f: fn(n Number) Number { return n });
}
`, "must be fn(Solid) Solid")
}

func TestCheckLambdaVarParam(t *testing.T) {
	// Lambda passed to fn(var) var param should be accepted (var matches any)
	expectNoErrors(t, `
fn Transform(s Solid, f fn(var) var) Solid {
    return s;
}
fn Main() {
    return Transform(s: Cube(size: 10 mm), f: fn(s Solid) Solid { return s });
}
`)
}

func TestCheckCallFunctionVariable(t *testing.T) {
	// Calling a function-typed variable should be validated
	expectNoErrors(t, `
fn Apply(f fn(Number) Number, x Number) Number {
    return f(x: x);
}
fn Main() {
    var result = Apply(f: fn(n Number) Number { return n * 2 }, x: 5);
    return Cube(size: result * 1 mm);
}
`)
}

func TestCheckCallFunctionVariableReturnType(t *testing.T) {
	// Return type of function-typed variable should flow through
	expectNoErrors(t, `
fn DoIt(f fn(Length) Solid, x Length) Solid {
    return f(x: x);
}
fn Main() {
    return DoIt(f: fn(x Length) Solid { return Cube(size: x) }, x: 10 mm);
}
`)
}
