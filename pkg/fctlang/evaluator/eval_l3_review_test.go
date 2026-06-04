package evaluator

import (
	"context"
	"strings"
	"testing"
)

// TestEvalLambdaCoercesArgsToDeclaredType confirms a lambda's args are
// coerced against its declared param types (Number → Length, etc.) the
// same way as a top-level fn. Without coercion, a lambda declared
// `fn(x Length)` called with a bare Number would see the Number in the
// body and any Length op would fail with a confusing type error.
func TestEvalLambdaCoercesArgsToDeclaredType(t *testing.T) {
	src := `
fn Main() Solid {
    var Scale = fn(s Length) Length { return s * 2 }
    var width = Scale(s: 5)
    return Cube(s: Vec3{x: width, y: width, z: width})
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("lambda arg coercion should produce Length, got: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

// TestEvalDuplicateNamedArgErrors confirms that a repeated named argument
// produces an explicit error rather than silently taking the last-supplied
// value. Pre-fix, `Foo(x: 1, x: 2)` set x to 2 without warning.
func TestEvalDuplicateNamedArgErrors(t *testing.T) {
	src := `
fn Foo(x Number) Number { return x }
fn Main() Solid {
    var bad = Foo(x: 1, x: 2)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected duplicate-arg error")
	}
	if !strings.Contains(err.Error(), "duplicate argument") {
		t.Fatalf("expected duplicate-argument error, got: %v", err)
	}
}

// TestEvalLibraryUsesStdlibOperators confirms that library code can use
// the stdlib operator functions registered for Vec3 / Vec2 / Solid /
// Sketch. Pre-fix, newLibEval and the libEval in evalLibExpr created a
// sub-evaluator with nil opFuncs, so `vec1 + vec2` inside a lib body fell
// through to "incompatible types Vec3 and Vec3".
func TestEvalLibraryUsesStdlibOperators(t *testing.T) {
	src := `
var T = lib "github.com/firstlayer-xyz/facetlibs/threads@f95a707"
fn Main() Solid { return T.Thread(size: "m3").Outside(length: 2 mm) }
`
	prog := parseTestProg(t, src)
	resolveTestProg(t, prog, "", nil)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("library should be able to use stdlib operators, got: %v", err)
	}
}
