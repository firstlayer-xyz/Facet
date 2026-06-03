package checker

import "testing"

// `any` is the dynamic type: a parameter typed `any` accepts indexing, nested
// indexing, and arithmetic with no static type error — the checks defer to
// runtime (the value is dynamically tagged). This is what lets the OpenSCAD
// transpiler model OpenSCAD's typeless values.
func TestCheckAnyParamIsDynamic(t *testing.T) {
	expectNoErrors(t, `fn pick(v any) Number { return v[0][1] + 1 }
fn Main() Solid { return Cube(s: pick(v: [[1, 2], [3, 4]])) }`)
}

// A function may also declare an `any` return type, and an `any` result is
// itself indexable.
func TestCheckAnyReturn(t *testing.T) {
	expectNoErrors(t, `fn first(v any) any { return v[0] }
fn Main() Solid { return Cube(s: first(v: [[5, 6], [7, 8]])[0]) }`)
}

// `any` must not weaken typo detection: an unrecognized type name on a
// parameter still errors (only the literal keyword `any` is the dynamic type).
func TestCheckUnknownParamTypeStillErrors(t *testing.T) {
	expectError(t, `fn bad(v Bogus) Number { return 1 }
fn Main() Solid { return Cube(s: bad(v: 1)) }`, "unknown type")
}
