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
