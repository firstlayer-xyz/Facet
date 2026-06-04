package evaluator

import (
	"context"
	"strings"
	"testing"
)

// TestEvalSketchOpFuncDispatch confirms that a user-defined operator on
// Sketches reaches the dispatch table. Pre-fix, the Sketch branch of
// evalBinary errored on any op outside +/-/& before dispatch could fire,
// asymmetric with the Solid branch which uses a fall-through pattern.
func TestEvalSketchOpFuncDispatch(t *testing.T) {
	src := `
fn %(a, b Sketch) Sketch { return a + b }
fn Main() Solid { return (Circle(r: 5 mm) % Square(s: 10 mm)).Extrude(z: 1 mm) }
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("user-defined Sketch operator should dispatch, got: %v", err)
	}
}

// TestEvalBoolArrayComparisonErrors confirms that Bool == Array (and the
// reverse) no longer silently coerce via len > 0. The checker rejects this
// for typed code; the runtime path was a backdoor through `Any`/`var`.
func TestEvalBoolArrayComparisonErrors(t *testing.T) {
	// Force the comparison through Any so the checker can't pre-reject it
	// — the runtime is what we're testing here.
	src := `
fn AnyEq(a Any, b Any) Bool { return a == b }
fn Main() Solid {
    var x = AnyEq(a: true, b: [1, 2, 3])
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected Bool == Array to error at runtime")
	}
	if !strings.Contains(err.Error(), "incompatible types") {
		t.Fatalf("expected incompatible-types error, got: %v", err)
	}
}
