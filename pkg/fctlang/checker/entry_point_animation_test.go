package checker

import (
	"testing"
)

// An entry point returning the stdlib Animation type, whose `frame` field is a
// lambda, must type-check cleanly and validate as an entry point.
func TestAnimationEntryTypeChecks(t *testing.T) {
	src := `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }}
}
`
	prog := parseTestProg(t, src)
	checked := Check(prog)
	if len(checked.Errors) > 0 {
		t.Fatalf("expected no checker errors, got: %v", checked.Errors)
	}
	if errp := checked.ValidateEntryPoint(testMainKey, "Main"); errp != nil {
		t.Fatalf("Animation entry should validate, got: %v", errp)
	}
}
