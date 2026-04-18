package evaluator

import "testing"

// TestTypeNameFunction pins the user-facing type name for *functionVal to
// "Function" — Go implementation details (e.g. "ast.Function") must not
// leak into user-facing error messages.
func TestTypeNameFunction(t *testing.T) {
	fv := &functionVal{}
	got := typeName(fv)
	if got != "Function" {
		t.Errorf("typeName(*functionVal) = %q, want %q", got, "Function")
	}
}
