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
