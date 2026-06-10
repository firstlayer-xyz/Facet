package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Deep-review regressions: NaN/Inf containment, range/concat caps, and
// comparison trichotomy. Each pins a verified silently-wrong-geometry bug or
// an unbounded-allocation hole.

func evalExpectError(t *testing.T, src, wantSubstr string) {
	t.Helper()
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatalf("expected an error containing %q, got success", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error containing %q, got: %v", wantSubstr, err)
	}
}

// Sqrt of a negative is a domain error (like Asin/Acos), not a NaN that
// silently erases the model.
func TestEvalSqrtNegativeErrors(t *testing.T) {
	evalExpectError(t, `
fn Main() {
    return Cube(s: Vec3{x: Sqrt(n: -1) * 1 mm, y: 1 mm, z: 1 mm});
}
`, "non-negative")
}

// Pow overflow / fractional power of a negative base has no finite result.
func TestEvalPowNonFiniteErrors(t *testing.T) {
	evalExpectError(t, `
fn Main() {
    return Cube(s: Vec3{x: Pow(base: 10, exp: 400) * 1 mm, y: 1 mm, z: 1 mm});
}
`, "no finite result")
}

// A non-finite dimension reaching a geometry builtin errors at the boundary
// rather than passing the kernel's NaN-blind `x <= 0` guards. (Multiplication
// overflow is the remaining Inf producer once Sqrt/Pow are guarded.)
func TestEvalInfDimensionErrors(t *testing.T) {
	evalExpectError(t, `
fn Main() {
    var huge = Pow(base: 10, exp: 308) * 10;
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}).Move(x: huge * 1 mm);
}
`, "finite")
}

// The range cap holds even when the estimated size overflows int — the
// comparison happens in float space.
func TestEvalHugeRangeRejectedNotOOM(t *testing.T) {
	evalExpectError(t, `
fn Main() {
    var n = Pow(base: 10, exp: 19);
    var r = [0 : n];
    return Cube(s: Vec3{x: r[0] * 1 mm + 1 mm, y: 1 mm, z: 1 mm});
}
`, "limit")
}

// The documented-inclusive range endpoint survives fractional-step rounding:
// [0 : 0.7 : 0.1] has 8 elements ending at ~0.7.
func TestEvalRangeInclusiveEndpointWithFractionalStep(t *testing.T) {
	src := `
fn Main() {
    var r = [0 : 0.7 : 0.1];
    return Cube(s: Vec3{x: Size(of: r) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 8, 1, 1, 0.1)
}

// Exponential array growth via concat is capped instead of OOMing the host.
// acc starts as the first element ([0], an array) and doubles per iteration.
func TestEvalConcatDoublingCapped(t *testing.T) {
	evalExpectError(t, `
fn Main() {
    var seeds = for i [0 : 60] {
        yield [i];
    };
    var big = fold acc, s seeds {
        yield acc + acc;
    };
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`, "limit")
}

// Comparison trichotomy: with epsilon equality, 0.1+0.2 == 0.3 must imply
// (0.1+0.2 <= 0.3) and NOT (0.1+0.2 > 0.3). (Single-level ifs so this test
// is independent of the nested-if scoping fix on the control-flow branch.)
func TestEvalComparisonTrichotomy(t *testing.T) {
	src := `
fn Main() {
    var a = 0.1 + 0.2;
    var w = 1;
    if a == 0.3 {
        w = w + 1;
    }
    if a <= 0.3 {
        w = w + 3;
    }
    if a > 0.3 {
        w = 99;
    }
    return Cube(s: Vec3{x: w * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// w = 1 + 1 (==) + 3 (<=), and NOT 99 (>): width 5.
	assertMeshSize(t, mesh, 5, 1, 1, 0.1)
}
