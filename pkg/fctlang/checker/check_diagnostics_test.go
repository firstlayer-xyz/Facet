package checker

import "testing"

// A diagnostic in a function body must be reported exactly once. The checker
// walks each body up to three times (Pass-1 return-type inference, Pass-2's
// validating checkStmts, and collectReturnTypes), and only the validating walk
// should emit — otherwise every body error appears 2–3 times.
func TestCheckNoDuplicateBodyDiagnostics(t *testing.T) {
	cases := []string{
		"fn Main() { return bogusVar }",       // unannotated: Pass-1 + Pass-2 + collectReturnTypes
		"fn Main() Solid { return bogusVar }", // annotated: Pass-2 + collectReturnTypes
	}
	for _, src := range cases {
		errs := checkSource(t, src)
		if len(errs) != 1 {
			t.Errorf("expected exactly 1 diagnostic for %q, got %d:", src, len(errs))
			for _, e := range errs {
				t.Errorf("  %v", e)
			}
		}
	}
}

// Suppressing the auxiliary walks must not hide the inconsistent-return-types
// diagnostic, which is emitted from the collectReturnTypes result after emission
// is re-enabled.
func TestCheckInconsistentReturnTypesStillReported(t *testing.T) {
	expectError(t, `
fn F(b Bool) {
    if b {
        return 1
    }
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}

fn Main() Solid {
    return Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm})
}
`, "inconsistent return types")
}
