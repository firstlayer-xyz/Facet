package evaluator

import (
	"context"
	"testing"
)

// Deep-review regressions: name resolution. The stdlib must be hermetic —
// user definitions must not hijack stdlib internals — and lambdas/struct
// defaults must resolve free names lexically, not dynamically.

// A user function named like a stdlib helper must not hijack calls made FROM
// stdlib bodies: Normalize() calls Sqrt() internally and must get the real one.
// (The user's Sqrt still wins for the user's own calls.)
func TestEvalStdlibInternalsResistUserShadowing(t *testing.T) {
	src := `
fn Sqrt(n Number) Number {
    return n;
}
fn Main() {
    var u = Normalize(v: Vec3{x: 3 mm, y: 0 mm, z: 0 mm});
    return Cube(s: Vec3{x: u.x * 10, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// Real Sqrt → unit vector x=1 → width 10. Hijacked Sqrt(9)=9 → x=1/3 → ~3.3.
	assertMeshSize(t, mesh, 10, 1, 1, 0.1)
}

// A user global redefining a stdlib constant shadows it for USER code only;
// stdlib bodies keep seeing the stdlib's own value (hermetic stdGlobals).
func TestEvalStdlibGlobalsHermetic(t *testing.T) {
	src := `
var PI = 3;
fn Main() {
    // User code sees the user's PI…
    var w = PI;
    // …while a stdlib body that uses PI internally still gets the real one.
    var c = Cos(a: Acos(n: -1));
    return Cube(s: Vec3{x: (w - c) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// w=3 (user PI), c=Cos(180°)=-1 → width 4.
	assertMeshSize(t, mesh, 4, 1, 1, 0.1)
}

// Struct field defaults evaluate in module scope: a same-named LOCAL in the
// instantiating function must not change the default (no dynamic scoping).
func TestEvalStructDefaultUsesModuleScope(t *testing.T) {
	src := `
var w = 5;
type S {
    n Number = w;
}
fn Helper() Number {
    var w = 77;
    var s = S {};
    return s.n + w * 0;
}
fn Main() {
    return Cube(s: Vec3{x: Helper() * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 1, 1, 0.1)
}

// A lambda's free names resolve against its DEFINING globals even when the
// lambda is invoked from a stdlib body (FindIndex) — the stdlib's hermetic
// globals must not be substituted for the user's.
func TestEvalLambdaKeepsDefiningGlobals(t *testing.T) {
	src := `
var threshold = 7;
fn Main() {
    var i = FindIndex(arr: [1, 5, 9], pred: fn(n Any) Bool {
        return n > threshold;
    });
    var w = (i ?? 0) + 1;
    return Cube(s: Vec3{x: w * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// 9 > 7 at index 2 → w = 3. If the lambda lost the user's globals the
	// reference would error (or match a different threshold).
	assertMeshSize(t, mesh, 3, 1, 1, 0.1)
}
