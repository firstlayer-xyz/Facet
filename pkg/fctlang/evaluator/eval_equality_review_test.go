package evaluator

import (
	"context"
	"testing"
)

// Deep-review regressions: the structural-equality predicate (valuesEqual)
// must agree with `==` — it used to return a silent false for structs,
// arrays, and Length↔Number, so IndexOf over such arrays never matched.

// IndexOf finds a Length element when probed with a bare Number (5 mm == 5),
// and finds struct elements by field-wise equality.
func TestEvalIndexOfAgreesWithEquality(t *testing.T) {
	src := `
fn Main() {
    var lengths = [5 mm, 10 mm];
    var i = IndexOf(arr: lengths, value: 10) ?? 99;

    var pts = [Vec2{x: 1 mm, y: 2 mm}, Vec2{x: 3 mm, y: 4 mm}];
    var j = IndexOf(arr: pts, value: Vec2{x: 3 mm, y: 4 mm}) ?? 99;

    // i=1, j=1 → width 3.
    return Cube(s: Vec3{x: (i + j + 1) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 3, 1, 1, 0.1)
}

// Some(struct) == Some(struct) compares the inner values field-wise, agreeing
// with the unwrapped comparison.
func TestEvalOptionalStructEquality(t *testing.T) {
	src := `
fn Pick(give Bool) Vec2? {
    if give {
        return Vec2{x: 3 mm, y: 4 mm};
    }
    return nil;
}
fn Main() {
    var a = Pick(give: true);
    var b = Pick(give: true);
    var w = 1;
    if a == b {
        w = w + 4;
    }
    return Cube(s: Vec3{x: w * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 1, 1, 0.1)
}

// An Optional compares against a definite value (the checker accepts it):
// Some(5) == 5, None != 5, and the mirrored definite == Optional.
func TestEvalOptionalDefiniteComparison(t *testing.T) {
	src := `
fn Maybe(give Bool) Number? {
    if give {
        return 5;
    }
    return nil;
}
fn Main() {
    var some = Maybe(give: true);
    var none = Maybe(give: false);
    var w = 1;
    if some == 5 {
        w = w + 1;
    }
    if 5 == some {
        w = w + 2;
    }
    if none != 5 {
        w = w + 3;
    }
    return Cube(s: Vec3{x: w * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// 1 + 1 + 2 + 3 = 7.
	assertMeshSize(t, mesh, 7, 1, 1, 0.1)
}
