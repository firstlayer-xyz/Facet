package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Optional runtime semantics. The Optional API is closed and language-level:
// `??` for fallback, `== nil` / `!= nil` for presence checks, and
// `if var x = opt { … }` for scoped binding. Optionals deliberately have
// no methods.

// ── ?? — fallback ─────────────────────────────────────────────────────────

func TestEvalOptionalFallbackPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 42 };
    var x = maybe() ?? 0;`,
		`x == 42`)
}

func TestEvalOptionalFallbackAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var x = maybe() ?? 99;`,
		`x == 99`)
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
    var x = src() ?? -1;`,
		`x == 100`)
}

// TestEvalOptionalHasNoMethods locks in that Optional values have no
// method surface. The checker rejects the method call at compile time;
// the evaluator carries the same error as a defence-in-depth path for
// values that reach a method call via `Any`/`var` and bypass static
// typing.
func TestEvalOptionalHasNoMethods(t *testing.T) {
	src := `fn Main() Solid {
    var maybe = fn() Number? { return 5 };
    var bad = maybe().IsSome();
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected error for Optional method call")
	}
	if !strings.Contains(err.Error(), "Optional") || !strings.Contains(err.Error(), "no method") {
		t.Fatalf("expected 'Optional ... no methods' error, got: %v", err)
	}
}
