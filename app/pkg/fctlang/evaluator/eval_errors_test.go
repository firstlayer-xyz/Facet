package evaluator

import (
	"context"
	"strings"
	"testing"
)

func TestEvalErrorNoMain(t *testing.T) {
	src := `fn Foo() Solid { return Cube(size: Vec3{x: 1 mm, y: 2 mm, z: 3 mm}); }`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for missing Main()")
	}
	if !strings.Contains(err.Error(), "Main") {
		t.Errorf("error should mention Main: %v", err)
	}
}

func TestEvalErrorUnknownFunction(t *testing.T) {
	src := `fn Main() Solid { return Nonexistent(a: 1 mm, b: 2 mm, c: 3 mm); }`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for unknown function")
	}
	if !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("error should mention unknown function: %v", err)
	}
}

func TestEvalErrorReturnTypeMismatch(t *testing.T) {
	// Function declares Length return but returns a Solid
	src := `
fn Bad() Length {
    return Cube(size: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}

fn Main() Solid {
    var x = Bad();
    return Cube(size: Vec3{x: x, y: x, z: x});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for return type mismatch")
	}
	if !strings.Contains(err.Error(), "declared return type Length") {
		t.Errorf("error should mention return type mismatch: %v", err)
	}
}

func TestEvalErrorArgTypeMismatch(t *testing.T) {
	// Function expects Solid param but gets Length
	src := `
fn Wrap(s Solid) Solid {
    return s;
}

fn Main() Solid {
    return Wrap(s: 10 mm);
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for argument type mismatch")
	}
	if !strings.Contains(err.Error(), "must be Solid") {
		t.Errorf("error should mention type mismatch: %v", err)
	}
}

func TestEvalErrorMethodOnLength(t *testing.T) {
	src := `
fn Main() {
    var s = 10 mm;
    return s.Translate(v: Vec3 { x: 1 mm, y: 2 mm, z: 3 mm });
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for method on length")
	}
	if !strings.Contains(err.Error(), "cannot call method") {
		t.Errorf("error should mention cannot call method: %v", err)
	}
}

func TestEvalErrorUnknownMethod(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).DoSomething();
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if !strings.Contains(err.Error(), "no method") {
		t.Errorf("error should mention no method: %v", err)
	}
}

func TestEvalErrorAngleExpectedGotLength(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Rotate(rx: 10 mm, ry: 0 deg, rz: 0 deg, pivot: WorldOrigin);
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for passing Length where Angle expected")
	}
	if !strings.Contains(err.Error(), "Angle") {
		t.Errorf("error should mention Angle: %v", err)
	}
}

func TestEvalErrorForYieldNonArray(t *testing.T) {
	src := `
fn Main() {
    var x = for i 10 mm {
        yield i;
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for iterating over non-array")
	}
	if !strings.Contains(err.Error(), "Array") {
		t.Errorf("error should mention Array: %v", err)
	}
}

func TestEvalErrorPolygonTooFewPoints(t *testing.T) {
	src := `
fn Main() {
    var pts = []Vec2[{x: 0 mm, y: 0 mm}, {x: 1 mm, y: 0 mm}];
    return Polygon(points: pts).Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for too few points")
	}
	if !strings.Contains(err.Error(), "at least 3") {
		t.Errorf("error should mention at least 3 points: %v", err)
	}
}

func TestEvalIfNoElseError(t *testing.T) {
	src := `
fn Main() {
    if false {
        return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    }
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for if with no else when condition is false")
	}
	if !strings.Contains(err.Error(), "must return") {
		t.Errorf("error should mention must return: %v", err)
	}
}

func TestEvalErrorRangePositiveStepCountingDown(t *testing.T) {
	src := `
fn Main() {
    var pts = for i[10:0:1] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for positive step when counting down")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Errorf("error should mention positive step: %v", err)
	}
}

func TestEvalErrorRangeNegativeStepCountingUp(t *testing.T) {
	src := `
fn Main() {
    var pts = for i[0:10:-1] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for negative step when counting up")
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Errorf("error should mention negative step: %v", err)
	}
}

func TestEvalErrorRangeZeroStep(t *testing.T) {
	src := `
fn Main() {
    var pts = for i[0:10:0] {
        yield Vec2{x: i, y: 0 mm};
    };
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for zero step")
	}
	if !strings.Contains(err.Error(), "zero") {
		t.Errorf("error should mention zero: %v", err)
	}
}
