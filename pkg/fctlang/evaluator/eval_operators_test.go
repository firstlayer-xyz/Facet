package evaluator

import (
	"context"
	"testing"
)

func TestEvalArithmetic(t *testing.T) {
	// w = 5+5 = 10mm, h = 20-10 = 10mm, d = 2*5 = 10mm
	src := `
fn Main() {
    var w = 5 mm + 5 mm;
    var h = 20 mm - 10 mm;
    var d = 2 mm * 5;
    return Cube(s: Vec3{x: w, y: h, z: d});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestEvalLengthPlusNumberLiteral: a bare numeric literal on the Number side of
// a Length ± Number (or % Number) expression is "untyped" and coerces to a
// Length.  `5 mm + 3` → 8 mm.  The error counterpart is
// TestErrorLengthPlusNumberVariable in eval_errors_test.go.
func TestEvalLengthPlusNumberLiteral(t *testing.T) {
	src := `
fn Main() {
    var w = 5 mm + 3;
    var h = 10 mm - 2;
    var d = 3 + 5 mm;
    return Cube(s: Vec3{x: w, y: h, z: d});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 8, 8, 8, 0.1)
}

// TestEvalLengthPlusNegativeLiteral: a negated literal still counts as a
// literal for coercion purposes (see parser.IsNumericLiteral).
func TestEvalLengthPlusNegativeLiteral(t *testing.T) {
	src := `
fn Main() {
    var w = 5 mm + -2;
    return Cube(s: Vec3{x: w, y: w, z: w});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 3, 3, 3, 0.1)
}

func TestEvalArithmeticPrecedence(t *testing.T) {
	// 2 + 3 * 4 = 14, not 20
	src := `
fn Main() {
    var s = 2 mm + 3 mm * 4;
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 14, 14, 14, 0.1)
}

func TestEvalDivisionAndMod(t *testing.T) {
	// s = 20mm / 2 = 10mm (Length / Number = Length).
	// m = 13 % 5 = 3 (Number % Number; boundary-coerces to 3mm at Vec3.z).
	// Under strict units Length % Number is a dimension error — use
	// Number() if you want to strip units inside an expression.
	src := `
fn Main() {
    var s = 20 mm / 2;
    var m = 13 % 5;
    return Cube(s: Vec3{x: s, y: s, z: m});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 3, 0.1)
}

func TestEvalParenExpr(t *testing.T) {
	// (2 + 3) * 4 = 20
	src := `
fn Main() {
    var s = (2 mm + 3 mm) * 4;
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 20, 20, 20, 0.1)
}

func TestEvalBooleanUnion(t *testing.T) {
	src := `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) + Sphere(r: 8 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalBooleanDifference(t *testing.T) {
	src := `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) - Sphere(r: 8 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalBooleanIntersection(t *testing.T) {
	src := `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) & Sphere(r: 8 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchBooleanOps(t *testing.T) {
	src := `
fn Main() {
    var a = Square(x: 10 mm, y: 10 mm);
    var b = Circle(r: 5 mm);
    var combined = a + b;
    var diff = a - b;
    var inter = a & b;
    return combined.Extrude(z: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAngleRad(t *testing.T) {
	// pi/2 rad = 90 degrees
	src := `
fn Main() {
    var box = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Rotate(x: 1.5707963267948966 rad, y: 0 deg, z: 0 deg, around: Vec3{});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAngleArithmetic(t *testing.T) {
	// Number * Angle in a loop
	src := `
fn Main() {
    var pts = for i[0:<4] {
        yield Vec2{x: Cos(a: i * 90 deg) * 10 mm, y: Sin(a: i * 90 deg) * 10 mm};
    };
    return Polygon(points: pts).Extrude(z: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalAngleAccumulateAdd(t *testing.T) {
	// 359 deg + 2 deg = 361 deg (angles accumulate, no wrapping)
	src := `
fn Main() {
    if 359 deg + 2 deg == 361 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("359 deg + 2 deg should equal 361 deg: %v", err)
	}
}

func TestEvalAngleAccumulateSub(t *testing.T) {
	// 1 deg - 2 deg = -1 deg (angles can be negative)
	src := `
fn Main() {
    if 1 deg - 2 deg == -1 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("1 deg - 2 deg should equal -1 deg: %v", err)
	}
}

func TestEvalAngleAccumulateMul(t *testing.T) {
	// 90 deg * 5 = 450 deg (no wrapping, needed for helix/twist)
	src := `
fn Main() {
    if 90 deg * 5 == 450 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("90 deg * 5 should equal 450 deg: %v", err)
	}
}

func TestEvalAngleLiteral360(t *testing.T) {
	// 360 deg is 360, not 0 (needed for full-rotation computations)
	src := `
fn Main() {
    if 360 deg == 360 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("360 deg should equal 360 deg: %v", err)
	}
}

func TestEvalAngleAccumulateRadAdd(t *testing.T) {
	// pi rad + pi rad = 360 deg (2*pi radians = 360 degrees)
	src := `
fn Main() {
    if 3.14159265358979 rad + 3.14159265358979 rad == 360 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("pi rad + pi rad should equal 360 deg: %v", err)
	}
}

func TestEvalAngleAccumulateRadSub(t *testing.T) {
	// 0.1 rad - 0.2 rad = -0.1 rad ≈ -5.73 deg (negative angle)
	src := `
fn Main() {
    var a = 0.1 rad - 0.2 rad;
    if a > -6 deg && a < -5 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("0.1 rad - 0.2 rad should be ~-5.73 deg: %v", err)
	}
}

func TestEvalAngleRadLiteral2Pi(t *testing.T) {
	// 2*pi rad = 360 deg (not wrapped to 0)
	src := `
fn Main() {
    if 6.28318530717959 rad == 360 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("2*pi rad should equal 360 deg: %v", err)
	}
}

func TestEvalAngleAccumulateRadMul(t *testing.T) {
	// 1 rad * 7 = 7 rad ≈ 401.07 deg (accumulates past 360)
	src := `
fn Main() {
    var a = 1 rad * 7;
    if a > 401 deg && a < 402 deg {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("1 rad * 7 should be ~401 deg: %v", err)
	}
}

func TestEvalComparisonLength(t *testing.T) {
	// 10 < 20 is true → 10mm cube
	src := `
fn Main() {
    if 10 mm < 20 mm {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalComparisonNumber(t *testing.T) {
	// 5 > 3 is true → 10mm cube
	src := `
fn Main() {
    if 5 > 3 {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalComparisonEquality(t *testing.T) {
	// 10 == 10 is true → 10mm cube
	src := `
fn Main() {
    if 10 mm == 10 mm {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalBoolComparison(t *testing.T) {
	src := `
fn Main() {
    if true == true {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalLogicalAndOr(t *testing.T) {
	src := `
fn Main() {
    if true && true {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalShortCircuit(t *testing.T) {
	// false && anything → false → else branch → 10mm cube
	src := `
fn Main() {
    if false && true {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    } else {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalTrigFunctions(t *testing.T) {
	// Sin(30 deg) = 0.5, Cos(60 deg) = 0.5 → 5mm x 5mm x 10mm cube
	src := `
fn Main() {
    var s = Sin(a: 30 deg) * 10 mm;
    var c = Cos(a: 60 deg) * 10 mm;
    return Cube(s: Vec3{x: s, y: c, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 5, 5, 10, 0.1)
}

func TestEvalMathMin(t *testing.T) {
	// Min(10, 20) = 10
	src := `
fn Main() {
    var s = Min(a: 10 mm, b: 20 mm);
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalMathMax(t *testing.T) {
	// Max(5, 15) = 15
	src := `
fn Main() {
    var s = Max(a: 5 mm, b: 15 mm);
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 15, 15, 15, 0.1)
}

func TestEvalMathAbs(t *testing.T) {
	// Abs(-10) = 10
	src := `
fn Main() {
    var s = Abs(a: -10 mm);
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalMathSqrt(t *testing.T) {
	// Sqrt(4) * 5 = 2 * 5 = 10
	src := `
fn Main() {
    var s = Sqrt(n: 4) * 5 mm;
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalMathLerp(t *testing.T) {
	// Lerp(0, 20, 0.5) = 10
	src := `
fn Main() {
    var s = Lerp(from: 0 mm, to: 20 mm, t: 0.5);
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestEvalLengthTimesNumber verifies Length*Number = Length (scale).
// If the product becomes a dimensionless Number, Cube's size field (Vec3 of
// Length) will reject it and the evaluation will fail.
func TestEvalLengthTimesNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 5 mm * 2;
    return Cube(s: Vec3{x: w, y: w, z: w});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestEvalNumberTimesLength verifies Number*Length = Length (commutative).
func TestEvalNumberTimesLength(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 2 * 5 mm;
    return Cube(s: Vec3{x: w, y: w, z: w});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestEvalLengthDivNumber verifies Length/Number = Length.
func TestEvalLengthDivNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 10 mm / 2;
    return Cube(s: Vec3{x: w, y: w, z: w});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.1)
}
