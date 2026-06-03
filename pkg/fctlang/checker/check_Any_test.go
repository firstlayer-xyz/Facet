package checker

import "testing"

// `Any` is the dynamic type: a parameter typed `Any` accepts indexing, nested
// indexing, and arithmetic with no static type error — the checks defer to
// runtime (the value is dynamically tagged). This is what lets the OpenSCAD
// transpiler model OpenSCAD's typeless values. Pinning the return to a
// concrete type lets the result flow into typed call sites unchanged.
func TestCheckAnyParamIsDynamic(t *testing.T) {
	expectNoErrors(t, `fn pick(v Any) Number { return v[0][1] + 1 }
fn Main() Solid { return Cube(s: pick(v: [[1, 2], [3, 4]])) }`)
}

// A function may also declare an `Any` return type, and an `Any` result is
// itself indexable.
func TestCheckAnyReturn(t *testing.T) {
	expectNoErrors(t, `fn first(v Any) Any { return v[0] }
fn Main() Solid { return Cube(s: first(v: [[5, 6], [7, 8]])[0]) }`)
}

// A concrete body (a list or number literal) may be returned through an `Any`
// return type. The transpiler relies on this: its helpers build concrete
// values but declare `Any` returns, e.g. `fn getMiddlePoint() Any { return [...] }`.
func TestCheckAnyReturnFromConcreteBody(t *testing.T) {
	expectNoErrors(t, `fn mid() Any { return [1, 2, 3] }
fn scalar() Any { return 5 }
fn Main() Solid { return Cube(s: mid()[0]) + Cube(s: scalar()) }`)
}

// `Any` must not weaken typo detection: an unrecognized type name on a
// parameter still errors (only the literal keyword `Any` is the dynamic type).
func TestCheckUnknownParamTypeStillErrors(t *testing.T) {
	expectError(t, `fn bad(v Bogus) Number { return 1 }
fn Main() Solid { return Cube(s: bad(v: 1)) }`, "unknown type")
}

// `Any` participates in the generic-group rule: two `Any` params declared
// separately (`a Any, b Any`) each get their own type slot, so they can take
// different concrete types — Pair-shaped helpers work naturally.
func TestCheckAnyGroupSeparateDeclarationsAllowDifferentTypes(t *testing.T) {
	expectNoErrors(t, `
fn Pair(a Any, b Any) Any { return a }
fn Main() Number { return Pair(a: 10, b: "hello") + 1 }
`)
}

// Params declared together (`a, b Any`) share one type slot — the checker
// rejects calls where the args don't resolve to the same concrete type.
func TestCheckAnyGroupSharedDeclarationRequiresSameType(t *testing.T) {
	expectError(t, `
fn Pair(a, b Any) Any { return a }
fn Main() Number { return Pair(a: 10, b: "hello") + 1 }
`, "generic type conflict")
}
