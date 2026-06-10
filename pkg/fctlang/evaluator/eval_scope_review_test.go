package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Deep-review regressions: block scoping, yield routing, comprehension
// re-evaluation, and recursion guards. Each test pins a behavior that used to
// produce silently wrong geometry or a process crash.

// An assignment inside a doubly-nested if must reach the function scope —
// blocks scope by shadow-restore, not by copy, so depth doesn't matter.
func TestEvalNestedIfAssignPropagates(t *testing.T) {
	src := `
fn Main() {
    var x = 1;
    if true {
        if true {
            x = 5;
        }
    }
    return Cube(s: Vec3{x: x * 1 mm, y: 5 mm, z: 5 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.1)
}

// A var declared in a block shadows the enclosing binding and is rolled back
// on exit; the enclosing value must survive.
func TestEvalBlockVarShadowRestores(t *testing.T) {
	src := `
fn Main() {
    var x = 5;
    if true {
        var x = 99;
        x = x + 1;
    }
    return Cube(s: Vec3{x: x * 1 mm, y: 5 mm, z: 5 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 5, 5, 0.1)
}

// A for-yield inside a fold body collects its own yields: the inner if-yield
// must go to the loop's array, not the fold accumulator. total =
// 10 + (20+2) + (30+2) = 64.
func TestEvalForYieldInsideFoldRoutesToLoop(t *testing.T) {
	src := `
fn Main() {
    var total = fold acc, v [10, 20, 30] {
        var evens = for x [1, 2, 3, 4] {
            if x == 2 {
                yield x;
            }
        };
        yield acc + v + evens[0];
    };
    return Cube(s: Vec3{x: total * 1 mm, y: 5 mm, z: 5 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 64, 5, 5, 0.1)
}

// The single-clause for-yield iterable expression evaluates exactly once. The
// debug tracer records one step per geometry op, so a doubly-evaluated
// MakeBox() would double the Cube steps.
func TestEvalForYieldIterableEvaluatedOnce(t *testing.T) {
	src := `
fn MakeBoxes() []Solid {
    return [Cube(s: Vec3{x: 2 mm, y: 2 mm, z: 2 mm})];
}
fn Main() {
    var moved = for s MakeBoxes() {
        yield s.Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm});
    };
    return moved[0];
}
`
	prog := parseTestProg(t, src)
	result, err := EvalDebug(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// Cube(s: Vec3) delegates to Cube(x,y,z,r): one user call = 2 Cube steps.
	cubes := 0
	for _, s := range result.Steps {
		if s.Op == "Cube" {
			cubes++
		}
	}
	if cubes != 2 {
		t.Errorf("expected 2 Cube steps (one call, delegated), got %d — the iterable was evaluated %d times", cubes, cubes/2)
	}
}

// A recursive method must hit the call-depth guard, not overflow the Go stack
// (which would kill the process).
func TestEvalRecursiveMethodDepthCapped(t *testing.T) {
	src := `
type C {
    n Number;
}
fn C.Loop() Number {
    return self.Loop();
}
fn Main() {
    var c = C { n: 1 };
    return Cube(s: Vec3{x: c.Loop() * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil || !strings.Contains(err.Error(), "maximum call depth") {
		t.Fatalf("expected a call-depth error, got: %v", err)
	}
}

// A struct type containing itself by value has no finite zero value — the
// evaluator must error cleanly (zeroStruct cycle guard), not recurse until the
// Go stack overflows and kills the process. (The checker also rejects the type
// declaration itself — see checker.TestCheckRecursiveStructTypeRejected.)
func TestEvalRecursiveStructTypeErrors(t *testing.T) {
	src := `
type A {
    next A;
}
fn Main() {
    var a = A {};
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected an error for a self-containing struct type")
	}
}
