package evaluator

import (
	"context"
	"strings"
	"testing"
)

// TestOverloadMinNumber verifies Min(Number, Number) returns Number.
func TestOverloadMinNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Min(a: 15, b: 10);   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadMinLength verifies Min(Length, Length) returns Length.
func TestOverloadMinLength(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Min(a: 15 mm, b: 10 mm);   # Length result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadMinMixed verifies Min(Number, Length) coerces to Length.
func TestOverloadMinMixed(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Min(a: 15, b: 10 mm);   # coerce 15 → 15 mm, result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadMinLengthMixed verifies Min(Length, Number) coerces Number to Length.
func TestOverloadMinLengthMixed(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Min(a: 15 mm, b: 10);   # coerce 10 → 10 mm, result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadMaxAngle verifies Max(Angle, Angle) returns Angle.
func TestOverloadMaxAngle(t *testing.T) {
	src := `
fn Main() Solid {
    var a = Max(a: 30 deg, b: 60 deg);   # 60 deg
    var s = Sin(a: a) * 10 mm;           # Sin(60°) ≈ 0.866, s ≈ 8.66 mm
    return Cube(size: Vec3{x: s, y: s, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 8.66, 8.66, 10, 0.1)
}

// TestOverloadAbsNumber verifies Abs(Number) returns Number.
func TestOverloadAbsNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Abs(a: -10);   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadAbsLength verifies Abs(Length) returns Length.
func TestOverloadAbsLength(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Abs(a: -10 mm);   # Length result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadLerpNumber verifies Lerp(Number, Number, Number) returns Number.
func TestOverloadLerpNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Lerp(from: 0, to: 20, t: 0.5);   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadLerpLength verifies Lerp(Length, Length, Number) returns Length.
func TestOverloadLerpLength(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Lerp(from: 0 mm, to: 20 mm, t: 0.5);   # Length result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadLerpAngle verifies Lerp(Angle, Angle, Number) returns Angle.
func TestOverloadLerpAngle(t *testing.T) {
	src := `
fn Main() Solid {
    var a = Lerp(from: 0 deg, to: 60 deg, t: 0.5);   # Angle result = 30 deg
    var s = Sin(a: a) * 20 mm;                     # Sin(30°) = 0.5, s = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadNumberFromNumber verifies Number(Number) is the identity.
func TestOverloadNumberFromNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Number(from: 10);   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadNumberFromLength verifies Number(Length) extracts the mm value.
func TestOverloadNumberFromLength(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Number(from: 10 mm);   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadNumberFromString verifies Number(String) parses numeric strings.
func TestOverloadNumberFromString(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Number(from: "10");   # Number result = 10
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadStringFromLength verifies String(Length) converts to string.
func TestOverloadStringFromLength(t *testing.T) {
	src := `
fn Main() Solid {
    var s = String(a: 10 mm);   # "10"
    var n = Number(from: s);       # back to 10
    return Cube(size: Vec3{x: n mm, y: n mm, z: n mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadStringFromBool verifies String(Bool) converts to string.
func TestOverloadStringFromBool(t *testing.T) {
	src := `
fn Main() Solid {
    var s = String(a: true);   # "true"
    assert s == "true"
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadSizeArray verifies Size(Array) returns element count.
func TestOverloadSizeArray(t *testing.T) {
	src := `
fn Main() Solid {
    var arr = []Number[1, 2, 3, 4, 5]
    var s = Size(of: arr);   # 5
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.1)
}

// TestOverloadSizeString verifies Size(String) returns character count.
func TestOverloadSizeString(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Size(of: "hello");   # 5
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.1)
}

// TestOverloadUserDefined verifies type-based overload resolution for user functions.
func TestOverloadUserDefined(t *testing.T) {
	src := `
fn Double(a Number) Number { return a * 2; }
fn Double(a Length) Length { return a * 2; }

fn Main() Solid {
    var s = Double(a: 5 mm);   # Length overload → 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadMinAngleMixed verifies Min(Angle, Number) coerces to Angle.
func TestOverloadMinAngleMixed(t *testing.T) {
	src := `
fn Main() Solid {
    var a = Min(a: 60 deg, b: 30);   # coerce 30 → 30 deg, result = 30 deg
    var s = Sin(a: a) * 20 mm;       # Sin(30°) = 0.5, s = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadAbsAngle verifies Abs(Angle) returns Angle.
func TestOverloadAbsAngle(t *testing.T) {
	src := `
fn Main() Solid {
    var a = Abs(a: -30 deg);      # Angle result = 30 deg
    var s = Sin(a: a) * 20 mm;    # Sin(30°) = 0.5, s = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadLerpMixed verifies Lerp(Number, Length, Number) coerces to Length.
func TestOverloadLerpMixed(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Lerp(from: 0 mm, to: 20, t: 0.5);   # coerce 20 → 20 mm, result = 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadAbsZero verifies Abs(0) selects the Number overload.
func TestOverloadAbsZero(t *testing.T) {
	src := `
fn Main() Solid {
    var s = Abs(a: 0);   # Number result = 0, not Length
    return Cube(size: Vec3{x: (s + 10) mm, y: (s + 10) mm, z: (s + 10) mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadUserDefinedWithDefaults verifies overload resolution with default params.
func TestOverloadUserDefinedWithDefaults(t *testing.T) {
	src := `
fn Scale(a Number, factor Number = 2) Number { return a * factor; }
fn Scale(a Length) Length { return a * 2; }

fn Main() Solid {
    var s = Scale(a: 5 mm);   # Length overload → 10 mm
    return Cube(size: Vec3{x: s, y: s, z: s});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadNoMatch verifies error message when no overload matches.
func TestOverloadNoMatch(t *testing.T) {
	src := `
fn Foo(a Number) Number { return a; }
fn Foo(a Length) Length { return a; }

fn Main() Solid {
    Foo(a: true);
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected error for no matching overload")
	}
	if got := err.Error(); !strings.Contains(got, "no matching overload") {
		t.Errorf("expected 'no matching overload' error, got: %v", got)
	}
}

// TestOverloadUserExtendsStdlib verifies user-defined overloads extend stdlib functions.
func TestOverloadUserExtendsStdlib(t *testing.T) {
	src := `
fn Clamp(val, lo, hi Number) Number {
    return Max(a: lo, b: Min(a: val, b: hi));
}

fn Main() Solid {
    var s = Clamp(val: 15, lo: 0, hi: 10);    # user Clamp = 10, uses stdlib Min/Max
    return Cube(size: Vec3{x: s mm, y: s mm, z: s mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// TestOverloadStructMethod verifies type-based overload resolution for struct methods.
func TestOverloadStructMethod(t *testing.T) {
	src := `
type Vec {
    x Length;
    y Length;
}

fn Vec.Scale(factor Number) Vec {
    return Vec { x: self.x * factor, y: self.y * factor };
}

fn Vec.Scale(offset Length) Vec {
    return Vec { x: self.x + offset, y: self.y + offset };
}

fn Main() Solid {
    var v = Vec { x: 5 mm, y: 5 mm };
    var v2 = v.Scale(factor: 2);         # Number overload: 10 mm, 10 mm
    return Cube(size: Vec3{x: v2.x, y: v2.y, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}
