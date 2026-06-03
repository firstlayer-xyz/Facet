package evaluator

import (
	"context"
	"testing"
)

// TestEvalArrayAppendNestedViaSingletonWrap documents the idiom for appending
// an array as a single element of an outer array: wrap the inner array in
// another `[]` so the right-hand side is a 1-element array of arrays. Without
// the wrap, `+` concatenates element-by-element.
func TestEvalArrayAppendNestedViaSingletonWrap(t *testing.T) {
	src := `
fn Main() Solid {
    var rows = [[1, 2], [3, 4]];
    var grown = rows + [[5, 6]];
    assert Size(of: grown) == 3;
    assert grown[2][0] == 5;
    assert grown[2][1] == 6;
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestEvalArrayAppendNestedTypedSingletonWrap is the typed-literal form of
// the same idiom — the singleton on the right uses an explicit element type
// tag.
func TestEvalArrayAppendNestedTypedSingletonWrap(t *testing.T) {
	src := `
fn Main() Solid {
    var rows = []Number[[1, 2], [3, 4]];
    var grown = rows + []Number[[5, 6]];
    assert Size(of: grown) == 3;
    assert grown[0][0] == 1;
    assert grown[2][1] == 6;
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestEvalArrayConcatMergesInnerElements is the contrasting case: without
// the singleton wrap, `+` between two arrays of the SAME shape concatenates
// at the outer level — the elements interleave. This is the existing
// array-plus-array semantics; the singleton-wrap test above is what you
// reach for when that's NOT what you want.
func TestEvalArrayConcatMergesInnerElements(t *testing.T) {
	src := `
fn Main() Solid {
    var a = [[1, 2], [3, 4]];
    var b = [[5, 6], [7, 8]];
    var merged = a + b;
    assert Size(of: merged) == 4;
    assert merged[0][0] == 1;
    assert merged[2][0] == 5;
    assert merged[3][1] == 8;
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestEvalArrayAppendScalarToFlat confirms that `+` between an array and a
// non-array scalar appends the scalar — no wrapping needed at the flat level.
// (The wrap idiom only matters when the element you want to append is itself
// an array.)
func TestEvalArrayAppendScalarToFlat(t *testing.T) {
	src := `
fn Main() Solid {
    var xs = [1, 2, 3];
    var grown = xs + 4;
    assert Size(of: grown) == 4;
    assert grown[3] == 4;
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}
