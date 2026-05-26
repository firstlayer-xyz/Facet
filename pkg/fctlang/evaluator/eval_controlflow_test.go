package evaluator

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEvalIfElseIf(t *testing.T) {
	// x == 2 → else-if branch → 10mm cube
	src := `
fn Main() {
    var x = 2;
    if x == 1 {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    } else if x == 2 {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
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

func TestEvalIfInForLoop(t *testing.T) {
	// Use if inside for-yield to conditionally generate geometry
	src := `
fn Main() {
    var cubes = for i[0:<4] {
        var c = Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
        if i >= 2 {
            c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
        }
        yield c;
    };
    var result = fold a, b cubes {
        yield a + b;
    };
    return result;
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

func TestEvalForYieldRange(t *testing.T) {
	// for-yield over range, union cubes at different positions
	src := `
fn Main() {
    var cubes = for i[0:<3] {
        yield Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Move(v: Vec3 { x: i * 10 mm, y: 0 mm, z: 0 mm });
    };
    var result = fold a, b cubes {
        yield a + b;
    };
    return result;
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

func TestEvalForYieldArray(t *testing.T) {
	// for-yield iterating over array literal
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm, 15 mm];
    var cubes = for s sizes {
        yield Cube(s: Vec3{x: s, y: s, z: s});
    };
    var result = fold a, b cubes {
        yield a + b;
    };
    return result;
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

func TestEvalFoldEmptyArray(t *testing.T) {
	// Fold over empty array should error
	src := `
fn Main() {
    var result = fold a, b []Number[] {
        yield a + b;
    };
    return result;
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error from fold on empty array")
	}
	if !strings.Contains(err.Error(), "empty array") {
		t.Fatalf("expected 'empty array' error, got: %v", err)
	}
}

func TestEvalForExprAsArgument(t *testing.T) {
	// For-yield directly as function argument (no temp variable)
	src := `
fn Main() {
    var r = 10 mm;
    return Polygon(points: for i[0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(z: 5 mm);
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

func TestEvalPolygonExtrude(t *testing.T) {
	// Polygon replaces NewNgon — build hexagon from points
	src := `
fn Main() {
    var r = 5 mm;
    return Polygon(points: for i[0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
    }).Extrude(z: 10 mm);
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

func TestEvalPolygonHexagon(t *testing.T) {
	src := `
fn Main() {
    var r = 10 mm;
    var pts = for i[0:<6] {
        yield Vec2{x: Cos(a: i * 60 deg) * r, y: Sin(a: i * 60 deg) * r};
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

func TestEvalAssertPass(t *testing.T) {
	src := `
fn Main() {
    assert true;
    assert 1 < 2;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalAssertFail(t *testing.T) {
	src := `
fn Main() {
    assert false;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected assertion error")
	}
	if !strings.Contains(err.Error(), "assertion failed") {
		t.Fatalf("expected 'assertion failed' error, got: %v", err)
	}
}

func TestEvalAssertFailWithMessage(t *testing.T) {
	src := `
fn Main() {
    assert 1 > 2, "math is broken";
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected assertion error")
	}
	if !strings.Contains(err.Error(), "math is broken") {
		t.Fatalf("expected 'math is broken' in error, got: %v", err)
	}
}

func TestEvalAssertInForYield(t *testing.T) {
	src := `
fn Main() {
    var cubes = for i [0:<3] {
        assert i >= 0;
        yield Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalDivisionByZero(t *testing.T) {
	src := `
fn Main() {
    var x = 10 / 0;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected division by zero error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected 'division by zero' error, got: %v", err)
	}
}

func TestEvalDivisionByZeroLength(t *testing.T) {
	src := `
fn Main() {
    var x = 10 mm / 0 mm;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected division by zero error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected 'division by zero' error, got: %v", err)
	}
}

func TestEvalUnaryMinusNumber(t *testing.T) {
	src := `
fn Main() {
    var x = 10;
    var y = -x;
    assert y == -10;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusLength(t *testing.T) {
	src := `
fn Main() {
    var x = 10 mm;
    var y = -x;
    assert y == -10 mm;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusAngle(t *testing.T) {
	src := `
fn Main() {
    var x = 45 deg;
    var y = -x;
    assert y == -45 deg;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusExpression(t *testing.T) {
	src := `
fn Main() {
    var x = -(3 + 7);
    assert x == -10;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusDoubleNegation(t *testing.T) {
	src := `
fn Main() {
    var x = --10;
    assert x == 10;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusFunctionCall(t *testing.T) {
	src := `
fn Main() {
    var x = -Sin(a: 90 deg);
    assert x == -1;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalUnaryMinusInArithmetic(t *testing.T) {
	src := `
fn Main() {
    var x = 5 + -3;
    assert x == 2;
    var y = 10 mm + -3 mm;
    assert y == 7 mm;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalBooleanNot(t *testing.T) {
	src := `
fn Main() {
    assert !false;
    assert !(!true);
    var x = 5;
    assert !(x > 10);
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalBooleanNotFalse(t *testing.T) {
	src := `
fn Main() {
    assert !true;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected assertion failure")
	}
	if !strings.Contains(err.Error(), "assertion failed") {
		t.Fatalf("expected 'assertion failed' error, got: %v", err)
	}
}

func TestEvalPow(t *testing.T) {
	src := `
fn Main() {
    assert Pow(base: 2, exp: 10) == 1024;
    assert Pow(base: 3, exp: 0) == 1;
    assert Pow(base: 5, exp: 1) == 5;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalFloor(t *testing.T) {
	src := `
fn Main() {
    assert Floor(n: 3.7) == 3;
    assert Floor(n: -2.3) == -3;
    assert Floor(n: 5) == 5;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalCeil(t *testing.T) {
	src := `
fn Main() {
    assert Ceil(n: 3.2) == 4;
    assert Ceil(n: -2.7) == -2;
    assert Ceil(n: 5) == 5;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalRound(t *testing.T) {
	src := `
fn Main() {
    assert Round(n: 3.4) == 3;
    assert Round(n: 3.5) == 4;
    assert Round(n: -2.5) == -3;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalHullSpheresCube(t *testing.T) {
	// Build a rounded cube by hulling 8 spheres at cube corners
	src := `
fn Main() {
    var size = 20 mm;
    var r = 2 mm;
    var spheres = for i [0:<8] {
        var x = (i % 2) * size;
        var y = (Floor(n: i / 2) % 2) * size;
        var z = (Floor(n: i / 4)) * size;
        yield Sphere(r: r).Move(v: Vec3 { x: x, y: y, z: z });
    };
    return Hull(arr: spheres);
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

func TestEvalMultiVarForYield(t *testing.T) {
	// 3x3 grid of spheres using multi-variable for
	src := `
fn Main() {
    var spheres = for i [0:<3], j [0:<3] {
        yield Sphere(r: 2 mm).Move(v: Vec3 { x: i * 10 mm, y: j * 10 mm, z: 0 mm });
    };
    return fold a, b spheres { yield a + b; };
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

func TestEvalMultiVarForYieldThreeClauses(t *testing.T) {
	// 2x2x2 grid — 8 cubes
	src := `
fn Main() {
    var cubes = for i [0:<2], j [0:<2], k [0:<2] {
        yield Cube(s: Vec3{x: 3 mm, y: 3 mm, z: 3 mm}).Move(v: Vec3 { x: i * 10 mm, y: j * 10 mm, z: k * 10 mm });
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalMultiVarForYieldDirect(t *testing.T) {
	// Multi-var for directly as argument to Hull
	src := `
fn Main() {
    var r = 2 mm;
    return Hull(arr: for i [0:<2], j [0:<2], k [0:<2] {
        yield Sphere(r: r).Move(v: Vec3 { x: i * 20 mm, y: j * 20 mm, z: k * 20 mm });
    });
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

func TestEvalYieldInsideIf(t *testing.T) {
	// Filter: only yield when condition is true
	src := `
fn Main() {
    var cubes = for i [0:<6] {
        if i >= 3 {
            yield Cube(s: Vec3{x: i * 1 mm, y: i * 1 mm, z: i * 1 mm});
        }
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalYieldInsideIfElse(t *testing.T) {
	// Yield different things based on condition
	src := `
fn Main() {
    var cubes = for i [0:<4] {
        if i % 2 == 0 {
            yield Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
        } else {
            yield Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
        }
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalYieldInsideIfMultiple(t *testing.T) {
	// Multiple yields inside a single if branch
	src := `
fn Main() {
    var cubes = for i [0:<3] {
        if i > 0 {
            yield Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
            yield Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
        }
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalYieldFilterEmpty(t *testing.T) {
	// All filtered out — should produce empty array, fold should error
	src := `
fn Main() {
    var arr = for i [0:<5] {
        if i > 100 {
            yield Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
        }
    };
    return fold a, b arr { yield a + b; };
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error from fold on empty array")
	}
	if !strings.Contains(err.Error(), "empty array") {
		t.Fatalf("expected 'empty array' error, got: %v", err)
	}
}

func TestEvalRecursion(t *testing.T) {
	// Recursive function: factorial
	src := `
fn Fact(n Number) Number {
    if n <= 1 { return 1; } else { return n * Fact(n: n - 1); }
}

fn Main() {
    assert Fact(n: 5) == 120;
    assert Fact(n: 1) == 1;
    assert Fact(n: 0) == 1;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
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

func TestEvalRecursionGeometry(t *testing.T) {
	// Recursive geometry: Sierpinski-like subdivision
	src := `
fn Stack(n Number, size Length) Solid {
    if n <= 0 {
        return Cube(s: Vec3{x: size, y: size, z: size});
    } else {
        var half = size / 2;
        var sub = Stack(n: n - 1, size: half);
        return sub + sub.Move(v: Vec3 { x: half, y: 0 mm, z: 0 mm });
    }
}

fn Main() {
    return Stack(n: 3, size: 20 mm);
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

func TestEvalBareYieldGuard(t *testing.T) {
	// Bare yield; skips the iteration — only even numbers collected
	// nums = [0, 2, 4], solids has 3 cubes, union them
	src := `
fn Main() {
    var solids = for i [0:5] {
        if i % 2 != 0 {
            yield;
        }
        yield Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
    };
    return fold acc, s solids {
        yield acc + s;
    };
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected mesh, got nil")
	}
}

func TestEvalBareYieldGuardFilter(t *testing.T) {
	// Use bare yield to filter: collect only values > 2
	// nums = [3, 4, 5], 3 cubes translated and unioned
	src := `
fn Main() {
    var cubes = for i [1:5] {
        if i <= 2 {
            yield;
        }
        yield Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}).Move(v: Vec3 { x: i * 1 mm, y: 0 mm, z: 0 mm });
    };
    return fold acc, s cubes {
        yield acc + s;
    };
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected mesh, got nil")
	}
}

func TestEvalBareYieldInsideBlock(t *testing.T) {
	// Bare yield inside a nested block (if body) within for-yield
	// Odd numbers only: [1, 3, 5]
	src := `
fn Main() {
    var cubes = for i [1:6] {
        if i % 2 == 0 {
            yield;
        }
        yield Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}).Move(v: Vec3 { x: i * 1 mm, y: 0 mm, z: 0 mm });
    };
    return fold acc, s cubes {
        yield acc + s;
    };
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected mesh, got nil")
	}
}

func TestEvalCancelledContextInForLoop(t *testing.T) {
	src := `
fn Main() {
    var result = for i[0:<1000] {
        yield Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
    };
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := Eval(ctx, prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Constraint validation tests
// ---------------------------------------------------------------------------

func TestEvalConstrainedVarValidDefault(t *testing.T) {
	src := `
var x = 50 where [0:100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintOverride(t *testing.T) {
	src := `
var x = 50 where [0:100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	overrides := map[string]interface{}{"x": 75.0}
	_, err := Eval(context.Background(), prog, testMainKey, overrides, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintOutOfRange(t *testing.T) {
	src := `
var x = 50 where [0:100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	overrides := map[string]interface{}{"x": 150.0}
	_, err := Eval(context.Background(), prog, testMainKey, overrides, "Main")
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out-of-range error, got: %v", err)
	}
}

func TestEvalConstraintInclusiveUpperBound(t *testing.T) {
	src := `
var x = 100 where [0:100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintExclusiveUpperBound(t *testing.T) {
	src := `
var x = 99 where [0:<100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	// 99 should be valid
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintExclusiveRejectsUpperBound(t *testing.T) {
	src := `
var x = 100 where [0:<100];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected out-of-range error for exclusive upper bound")
	}
}

func TestEvalConstraintSteppedRange(t *testing.T) {
	src := `
var x = 10 where [0:100:5];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	// 10 is on step boundary (0, 5, 10, ...)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintUnitRange(t *testing.T) {
	src := `
var w = 10 mm where [1:100] mm;
fn Main() {
    return Cube(s: Vec3{x: w, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintEnumValid(t *testing.T) {
	src := `
var s = "m3" where ["m3", "m4", "m5"];
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintEnumRejectsNonMember(t *testing.T) {
	src := `
var s = "m6" where ["m3", "m4", "m5"];
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for enum value not in allowed set")
	}
	if !strings.Contains(err.Error(), "not in allowed set") {
		t.Fatalf("expected 'not in allowed set' error, got: %v", err)
	}
}

func TestEvalConstraintFreeForm(t *testing.T) {
	src := `
var x = 42 where [];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestEvalConstraintFreeFormOverride(t *testing.T) {
	src := `
var x = 42 where [];
fn Main() {
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	overrides := map[string]interface{}{"x": 999.0}
	_, err := Eval(context.Background(), prog, testMainKey, overrides, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Enumerate (for i, v arr) tests
// ---------------------------------------------------------------------------

func TestEvalForYieldEnumerate(t *testing.T) {
	// Enumerate: index should be 0-based Number
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm, 15 mm];
    var cubes = for i, s sizes {
        yield Cube(s: Vec3{x: s, y: s, z: s}).Move(v: Vec3 { x: i * 20 mm, y: 0 mm, z: 0 mm });
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalForYieldEnumerateIndex(t *testing.T) {
	// Verify the index values are correct (0, 1, 2)
	src := `
fn Main() {
    var arr = []Length[10 mm, 20 mm, 30 mm];
    var indices = for i, v arr {
        assert i >= 0;
        assert i < 3;
        yield Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}).Move(v: Vec3 { x: i * 5 mm, y: 0 mm, z: 0 mm });
    };
    return fold a, b indices { yield a + b; };
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

func TestEvalForYieldEnumerateWithCartesian(t *testing.T) {
	// Enumerate + regular cartesian product
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm];
    var cubes = for i, s sizes, j [0:<2] {
        yield Cube(s: Vec3{x: s, y: s, z: s}).Move(v: Vec3 { x: i * 20 mm, y: j * 20 mm, z: 0 mm });
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalForYieldEnumerateNoIndex(t *testing.T) {
	// Regular for (no enumerate) — should still work as before
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm, 15 mm];
    var cubes = for s sizes {
        yield Cube(s: Vec3{x: s, y: s, z: s});
    };
    return fold a, b cubes { yield a + b; };
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

// ---------------------------------------------------------------------------
// Array indexing tests
// ---------------------------------------------------------------------------

func TestEvalArrayIndex(t *testing.T) {
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm, 15 mm];
    var s = sizes[1];
    assert s == 10 mm;
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
}

func TestEvalArrayIndexComputed(t *testing.T) {
	src := `
fn Main() {
    var arr = []Length[5 mm, 10 mm, 15 mm];
    var last = arr[Size(of: arr) - 1];
    assert last == 15 mm;
    return Cube(s: Vec3{x: last, y: last, z: last});
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

func TestEvalArrayIndexOutOfRange(t *testing.T) {
	src := `
fn Main() {
    var arr = []Number[1, 2, 3];
    var x = arr[5];
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected 'out of range' error, got: %v", err)
	}
}

func TestEvalArrayIndexNegative(t *testing.T) {
	// Negative indices wrap: -1 = last element, -2 = second to last, etc.
	src := `
fn Main() {
    var arr = []Number[1, 2, 3];
    var x = arr[-1];
    return Cube(s: Vec3{x: x * 1 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalArrayIndexNegativeOutOfRange(t *testing.T) {
	src := `
fn Main() {
    var arr = []Number[1, 2, 3];
    var x = arr[-4];
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected 'out of range' error, got: %v", err)
	}
}

func TestEvalArrayIndexNotArray(t *testing.T) {
	src := `
fn Main() {
    var x = 10;
    var y = x[0];
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for indexing non-array")
	}
	if !strings.Contains(err.Error(), "cannot index") {
		t.Fatalf("expected 'cannot index' error, got: %v", err)
	}
}

func TestEvalArrayIndexChained(t *testing.T) {
	src := `
fn Main() {
    var nested = []Length[[5 mm, 10 mm], [15 mm, 20 mm]];
    var s = nested[1][0];
    assert s == 15 mm;
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
}

func TestEvalParallelForYield50Cubes(t *testing.T) {
	src := `
fn Main() {
    var parts = for i [0:<50] {
        yield Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3 { x: i * 15 mm, y: 0 mm, z: 0 mm });
    };
    return fold a, b parts { yield a + b; };
}
`
	prog := parseTestProg(t, src)
	start := time.Now()
	mesh, err := evalMerged(context.Background(), prog, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
	t.Logf("50 cubes: %d vertices, %d triangles, took %v", len(mesh.Vertices)/3, len(mesh.Indices)/3, elapsed)
}

func TestEvalArrayIndexInForYield(t *testing.T) {
	src := `
fn Main() {
    var sizes = []Length[5 mm, 10 mm, 15 mm];
    var cubes = for i [0:<Size(of: sizes)] {
        var s = sizes[i];
        yield Cube(s: Vec3{x: s, y: s, z: s}).Move(v: Vec3 { x: i * 20 mm, y: 0 mm, z: 0 mm });
    };
    return fold a, b cubes { yield a + b; };
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

func TestEvalAssignment(t *testing.T) {
	src := `
fn Main() {
    var x = 10 mm;
    x = 20 mm;
    return Cube(s: Vec3{x: x, y: x, z: x});
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

func TestEvalAssignmentUndefined(t *testing.T) {
	src := `
fn Main() {
    y = 10 mm;
    return Cube(s: Vec3{x: y, y: y, z: y});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for assigning to undefined variable")
	}
	if !strings.Contains(err.Error(), "undefined") {
		t.Errorf("error should mention undefined: %v", err)
	}
}

func TestEvalAssignmentInForYield(t *testing.T) {
	src := `
fn Main() {
    var cubes = for i [0:<3] {
        var s = 5 mm;
        s = s + i mm;
        yield Cube(s: Vec3{x: s, y: s, z: s});
    };
    var result = fold a, b cubes {
        yield a + b;
    };
    return result;
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

func TestEvalCompoundAssignment(t *testing.T) {
	src := `
fn Main() {
    var x = 5 mm;
    x += 5 mm;
    return Cube(s: Vec3{x: x, y: x, z: x});
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

func TestEvalCompoundAssignmentAllOps(t *testing.T) {
	// x starts at 20, -= 5 → 15, *= 2 → 30, /= 3 → 10, used as length
	src := `
fn Main() {
    var x = 20 mm;
    x -= 5 mm;
    x *= 2;
    x /= 3;
    return Cube(s: Vec3{x: x, y: x, z: x});
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

func TestEvalConstReassignError(t *testing.T) {
	src := `
fn Main() {
    const x = 10 mm;
    x = 20 mm;
    return Cube(s: Vec3{x: x, y: x, z: x});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for reassigning const")
	}
	if !strings.Contains(err.Error(), "cannot reassign const") {
		t.Errorf("error should mention const reassignment: %v", err)
	}
}

func TestEvalConstFieldMutateError(t *testing.T) {
	src := `
type Point { x Number; y Number }
fn Main() {
    const p = Point{x: 1, y: 2};
    p.x = 3;
    return Cube(s: Vec3{x: p.x * 1 mm, y: p.y * 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for mutating field on const")
	}
	if !strings.Contains(err.Error(), "cannot mutate field on const") {
		t.Errorf("error should mention const field mutation: %v", err)
	}
}

func TestEvalConstReadOk(t *testing.T) {
	src := `
fn Main() {
    const s = 10 mm;
    return Cube(s: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestEvalConstCompoundAssignError(t *testing.T) {
	src := `
fn Main() {
    const x = 10 mm;
    x += 5 mm;
    return Cube(s: Vec3{x: x, y: x, z: x});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for compound assignment on const")
	}
	if !strings.Contains(err.Error(), "cannot reassign const") {
		t.Errorf("error should mention const reassignment: %v", err)
	}
}
