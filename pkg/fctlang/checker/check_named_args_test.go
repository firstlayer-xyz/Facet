package checker

import "testing"

// All user-facing calls require named arguments. The rule is enforced once in
// requireNamedArgs and shared by the free-function (checkCall) and method
// (checkMethodCall) paths; library calls flow through the method path too.

func TestCheckFreeFunctionRequiresNamedArgs(t *testing.T) {
	expectError(t, `fn Main() Solid { return Cube(10 mm); }`, "arguments must be named")
}

// Regression: positional method arguments used to slip past the checker and
// only fail at eval. They must now be a check-time error, like free functions.
func TestCheckMethodCallRequiresNamedArgs(t *testing.T) {
	expectError(t, `fn Main() Solid { return Cube(s: 10 mm).Rotate(90 deg, 0 deg, 0 deg); }`, "arguments must be named")
}

func TestCheckCallsAcceptNamedArgs(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid { return Cube(s: 10 mm).Rotate(z: 90 deg); }`)
}
