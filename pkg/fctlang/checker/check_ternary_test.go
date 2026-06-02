package checker

import "testing"

// TestCheckTernaryWellTyped confirms a well-formed ternary type-checks
// without errors and the arms agree on type.
func TestCheckTernaryWellTyped(t *testing.T) {
	expectNoErrors(t, `fn Main() Number { return true ? 1 : -1 }`)
}

// TestCheckTernaryRejectsNonBoolCond confirms the condition must be Bool.
func TestCheckTernaryRejectsNonBoolCond(t *testing.T) {
	expectError(t, `fn Main() Number { return 1 ? 1 : 0 }`, "must be Bool")
}

// TestCheckTernaryRejectsMismatchedArms confirms then-type and else-type
// must unify.
func TestCheckTernaryRejectsMismatchedArms(t *testing.T) {
	expectError(t, `fn Main() Number { return true ? 1 : "no" }`, "must agree on type")
}

// TestCheckTernaryUsableAsValue confirms the result type flows into
// surrounding contexts — return, var, etc.
func TestCheckTernaryUsableAsValue(t *testing.T) {
	expectNoErrors(t, `
fn Main() Solid {
    var size = true ? 10 mm : 5 mm;
    return Cube(s: Vec3{x: size, y: size, z: size})
}
`)
}
