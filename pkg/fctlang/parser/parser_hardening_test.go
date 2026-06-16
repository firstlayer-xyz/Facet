package parser_test

import (
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// A long unary chain must error at the depth limit, not overflow the
// (unrecoverable) goroutine stack. parsePrimary's unary branch recurses through
// parsePostfix back into parsePrimary, bypassing parseExpr's depth guard, so it
// needs its own.
func TestParseDeepUnaryRejected(t *testing.T) {
	src := "fn M() Number { return " + strings.Repeat("-", 5000) + "1 }"
	if _, err := parser.Parse(src, "", parser.SourceUser); err == nil {
		t.Fatal("expected a depth-limit error for a deep unary chain, got nil")
	}
}

// A \u escape naming a UTF-16 surrogate half is not a valid Unicode scalar and
// must be rejected, not silently mangled to U+FFFD.
func TestLexSurrogateEscapeRejected(t *testing.T) {
	src := `fn M() Number { var s = "\uD800" return 1 }`
	if _, err := parser.Parse(src, "", parser.SourceUser); err == nil {
		t.Fatal(`expected an error for surrogate escape \uD800, got nil`)
	}
}

// Pin `??` precedence: it sits between `||` (looser) and `&&` (tighter), so
// `a && b ?? c` parses as `(a && b) ?? c`.
func TestNullCoalescePrecedence(t *testing.T) {
	prog, err := parser.Parse("fn M() Bool { return a && b ?? c }", "", parser.SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	ret, ok := prog.Declarations[0].(*parser.Function).Body[0].(*parser.ReturnStmt)
	if !ok {
		t.Fatalf("body[0] is %T, want *ReturnStmt", prog.Declarations[0].(*parser.Function).Body[0])
	}
	top, ok := ret.Value.(*parser.BinaryExpr)
	if !ok || top.Op != "??" {
		t.Fatalf("top op = %#v, want (a && b) ?? c", ret.Value)
	}
	left, ok := top.Left.(*parser.BinaryExpr)
	if !ok || left.Op != "&&" {
		t.Fatalf("??-left = %#v, want an a && b node (so (a && b) ?? c)", top.Left)
	}
}
