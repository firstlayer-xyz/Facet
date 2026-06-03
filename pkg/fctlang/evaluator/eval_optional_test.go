package evaluator

import "testing"

// Optional methods at runtime. We can't declare helper functions in
// stdlibIfThenCubeWithSetup's `setup` block (it's spliced inside Main),
// so each test defines its optional-producer as a zero-arg lambda. The
// lambda's body returns the value under test; the predicate then exercises
// the optional methods on the result.

// ── Or — extract with default ─────────────────────────────────────────────

func TestEvalOptionalOrPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 42 };
    var x = maybe().Or(default: 0);`,
		`x == 42`)
}

func TestEvalOptionalOrAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var x = maybe().Or(default: 99);`,
		`x == 99`)
}

// ── IsSome / IsNone ───────────────────────────────────────────────────────

func TestEvalOptionalIsSomeTrue(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 5 };`,
		`maybe().IsSome()`)
}

func TestEvalOptionalIsSomeFalse(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };`,
		`!maybe().IsSome()`)
}

func TestEvalOptionalIsNoneFlipsIsSome(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };`,
		`maybe().IsNone()`)
}

// ── Equality with nil ─────────────────────────────────────────────────────

func TestEvalOptionalEqualsNilWhenAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };`,
		`maybe() == nil`)
}

func TestEvalOptionalEqualsNilWhenPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 7 };`,
		`maybe() != nil`)
}

// TestEvalOptionalWidening confirms a definite Number flows into a
// Number?-typed slot via implicit widening at the return boundary.
func TestEvalOptionalWidening(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var src = fn() Number? { return 100 };
    var x = src().Or(default: -1);`,
		`x == 100`)
}
