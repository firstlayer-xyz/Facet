package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Slice expressions must error on out-of-range bounds instead of silently
// clamping. The old behavior (`arr[5:2] → empty`, `arr[-99:2] → arr[0:2]`)
// hid programmer mistakes.

func TestEvalArraySliceStartGreaterThanEnd(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [10, 20, 30, 40, 50];
    var bad = a[5:2];
    return Cube(s: Vec3{x: Size(of: bad) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for slice start > end")
	}
	if !strings.Contains(err.Error(), "slice") {
		t.Errorf("error should mention slice: %v", err)
	}
}

func TestEvalArraySliceStartOutOfRange(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [10, 20, 30];
    var bad = a[10:11];
    return Cube(s: Vec3{x: Size(of: bad) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for slice start beyond array length")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range: %v", err)
	}
}

func TestEvalArraySliceEndOutOfRange(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [10, 20, 30];
    var bad = a[0:99];
    return Cube(s: Vec3{x: Size(of: bad) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for slice end beyond array length")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range: %v", err)
	}
}

func TestEvalArraySliceLargeNegativeStart(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [10, 20, 30];
    // -99 + 3 = -96, still negative → out of range
    var bad = a[-99:2];
    return Cube(s: Vec3{x: Size(of: bad) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for large negative start")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error should mention out of range: %v", err)
	}
}

func TestEvalArraySliceNonInteger(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [10, 20, 30, 40, 50];
    var bad = a[0.5:2];
    return Cube(s: Vec3{x: Size(of: bad) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error for non-integer slice index")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error should mention integer: %v", err)
	}
}

func TestEvalArraySliceFullRangeValid(t *testing.T) {
	// Regression: arr[:len] and arr[len:len] must still be valid.
	src := `
fn Main() Solid {
    var a = [10, 20, 30];
    var full = a[0:3];
    var empty = a[3:3];
    return Cube(s: Vec3{x: Size(of: full) * 1 mm, y: (Size(of: empty) + 1) * 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("unexpected error on valid full-range slice: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Error("expected non-empty mesh")
	}
}
