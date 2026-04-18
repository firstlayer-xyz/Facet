package evaluator

import "testing"

// TestTypeNameFunction guards against the bug flagged in the 2026-04-16
// main-branch review (fctlang Critical #4): *functionVal used to report as
// "ast.Function" — a Go implementation detail leaking into user-facing
// error messages.  Users should see the clean name "Function".
func TestTypeNameFunction(t *testing.T) {
	fv := &functionVal{}
	got := typeName(fv)
	if got != "Function" {
		t.Errorf("typeName(*functionVal) = %q, want %q", got, "Function")
	}
}
