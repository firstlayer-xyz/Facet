package parser

import (
	"strings"
	"testing"
)

// TestParseReturnInsideForYieldHasRefactorHint confirms the parser-level
// rejection of `return` inside a for-yield loop carries the refactor hint:
// the previous "use yield instead" message was misleading because yield
// contributes to the loop's array, not the enclosing function's return.
func TestParseReturnInsideForYieldHasRefactorHint(t *testing.T) {
	src := `fn Main() Solid {
    var bad = for i [0:<3] {
        return Cube(s: 10 mm)
    }
    return Cube(s: 10 mm)
}
`
	_, err := Parse(src, "main.fct", SourceUser)
	if err == nil {
		t.Fatal("expected parse error for return inside for-yield")
	}
	if !strings.Contains(err.Error(), "extract the loop into its own function") {
		t.Fatalf("expected refactor-hint message, got: %v", err)
	}
}

// TestParseBareYieldRejected confirms a valueless `yield` is a parse error. It
// contributed no element and did not skip the iteration — a silent no-op — so
// both the explicit `yield;` and same-line `yield }` forms are rejected with a
// message that points at the guarded-yield filter idiom.
func TestParseBareYieldRejected(t *testing.T) {
	for _, src := range []string{
		"fn Main() Solid { var x = for i [0:<3] { yield; }; return Cube(s: 1 mm) }",
		"fn Main() Solid { var x = for i [0:<3] { yield }; return Cube(s: 1 mm) }",
	} {
		err := mustParseError(t, "bare yield", src)
		if !strings.Contains(err.Error(), "requires a value") {
			t.Fatalf("expected actionable 'requires a value' message, got: %v", err)
		}
	}
	// The guarded filter idiom the docs recommend must still parse.
	ok := "fn Main() Solid { var e = for i [0:<10] { if i % 2 == 0 { yield i } }; return Cube(s: 1 mm) }"
	if _, err := Parse(ok, "main.fct", SourceUser); err != nil {
		t.Fatalf("guarded yield filter idiom should parse: %v", err)
	}
}
