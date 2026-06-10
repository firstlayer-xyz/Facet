package checker

import (
	"strings"
	"testing"
)

// Each iteration of a for-yield/fold body runs against its own copy of the
// enclosing scope, so an assignment to an outer variable would be silently
// discarded at runtime — the checker rejects it.
func TestCheckAssignToOuterFromForBodyRejected(t *testing.T) {
	errs := checkSource(t, `
fn Main() Solid {
    var total = 0
    var r = for i [1, 2, 3] {
        total = total + i
        yield i
    }
    return Cube(s: total * 1 mm)
}
`)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "cannot assign to outer variable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an outer-assignment error, got: %v", errs)
	}
}

// Assigning to the loop variable or to a var declared inside the body stays
// within the iteration scope and is fine.
func TestCheckAssignToLoopLocalAllowed(t *testing.T) {
	errs := checkSource(t, `
fn Main() Solid {
    var r = for i [1, 2, 3] {
        var c = i
        c = c + 1
        yield c
    }
    return Cube(s: r[0] * 1 mm)
}
`)
	for _, e := range errs {
		if strings.Contains(e.Message, "cannot assign to outer variable") {
			t.Fatalf("loop-local assignment wrongly rejected: %v", errs)
		}
	}
}

// The same rule applies inside fold bodies.
func TestCheckAssignToOuterFromFoldBodyRejected(t *testing.T) {
	errs := checkSource(t, `
fn Main() Solid {
    var count = 0
    var total = fold acc, v [1, 2, 3] {
        count = count + 1
        yield acc + v
    }
    return Cube(s: total * 1 mm)
}
`)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "cannot assign to outer variable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an outer-assignment error, got: %v", errs)
	}
}

// A struct type that contains itself by value has no finite value — the
// declaration is rejected (an Optional or array field breaks the cycle).
func TestCheckRecursiveStructTypeRejected(t *testing.T) {
	errs := checkSource(t, `
type A {
    next A
}
fn Main() Solid {
    return Cube(s: 1 mm)
}
`)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "contains itself") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a recursive-type error, got: %v", errs)
	}
}

// Indirect cycles (A → B → A) are also rejected; Optional self-reference is fine.
func TestCheckRecursiveStructTypeIndirectAndOptional(t *testing.T) {
	errs := checkSource(t, `
type A {
    b B
}
type B {
    a A
}
fn Main() Solid {
    return Cube(s: 1 mm)
}
`)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "contains itself") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an indirect recursive-type error, got: %v", errs)
	}

	errs = checkSource(t, `
type Node {
    next Node?
}
fn Main() Solid {
    return Cube(s: 1 mm)
}
`)
	for _, e := range errs {
		if strings.Contains(e.Message, "contains itself") {
			t.Fatalf("Optional self-reference wrongly rejected: %v", errs)
		}
	}
}

// Value-semantics mutability rules (mirrored by the evaluator): array-element
// receivers and module-level structs reject field assignment; deep const roots
// are caught through field chains.
func TestCheckFieldAssignMutabilityRules(t *testing.T) {
	errs := checkSource(t, `
fn Main() Solid {
    var pts = [Vec3{x: 1 mm, y: 1 mm, z: 1 mm}]
    pts[0].x = 99 mm
    return Cube(s: 1 mm)
}
`)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "array element") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an array-element error, got: %v", errs)
	}

	errs = checkSource(t, `
var cfg = Vec3{x: 1 mm, y: 1 mm, z: 1 mm}
fn Poke() Number {
    cfg.x = 99 mm
    return 0
}
fn Main() Solid {
    return Cube(s: cfg)
}
`)
	found = false
	for _, e := range errs {
		if strings.Contains(e.Message, "module-level") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a module-level mutation error, got: %v", errs)
	}

	errs = checkSource(t, `
type Outer {
    inner Vec3
}
fn Main() Solid {
    const cfg = Outer{inner: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}}
    cfg.inner.x = 99 mm
    return Cube(s: cfg.inner)
}
`)
	found = false
	for _, e := range errs {
		if strings.Contains(e.Message, "const") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a deep-const error, got: %v", errs)
	}
}
