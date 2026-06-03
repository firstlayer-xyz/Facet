package checker

import "testing"

// TestVarGroupSeparateDeclarationsAllowDifferentTypes confirms that two
// var params declared independently (`a var, b Any`) do NOT form a same-type
// group — each accepts its own concrete type.
func TestVarGroupSeparateDeclarationsAllowDifferentTypes(t *testing.T) {
	expectNoErrors(t, `
fn Pair(a Any, b Any) Any { return a }
fn Main() Number { return Pair(a: 10, b: "hello") + 1 }
`)
}

// TestVarGroupSharedDeclarationRequiresSameType confirms that a grouped
// declaration (`a, b Any`) DOES enforce same-type-across-args.
func TestVarGroupSharedDeclarationRequiresSameType(t *testing.T) {
	expectError(t, `
fn Pair(a, b Any) Any { return a }
fn Main() Number { return Pair(a: 10, b: "hello") + 1 }
`, "generic type conflict")
}
