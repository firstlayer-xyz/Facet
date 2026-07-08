package checker

import "testing"

// TestCheckIndexedStructFieldAccess pins the fix for resolveStructName's missing
// IndexExpr case: field and method access on an element of a struct array used
// to go completely unchecked (resolveStructName returned "" for arr[i]).
func TestCheckIndexedStructFieldAccess(t *testing.T) {
	// A valid field on an indexed struct element type-checks.
	expectNoErrors(t, `
type P {
    x Length
    y Length
}
fn Main() Solid {
    var pts = [P{x: 1 mm, y: 2 mm}]
    var v = pts[0].y
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}`)

	// A bogus field on an indexed struct element is now flagged (was silent).
	expectError(t, `
type P {
    x Length
    y Length
}
fn Main() Solid {
    var pts = [P{x: 1 mm, y: 2 mm}]
    var v = pts[0].nope
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}`, "nope")
}
