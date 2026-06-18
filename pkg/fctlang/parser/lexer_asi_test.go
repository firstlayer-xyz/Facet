package parser_test

import (
	"testing"

	"facet/pkg/fctlang/parser"
)

// A `??` or `?.` at the start of a continuation line continues the expression;
// ASI must not insert a semicolon before it (which made the operator start a new
// statement and produced "expected expression, got ??"). A bare ternary `?` is
// deliberately not treated as a continuation.
func TestLexerASIContinuesNullishAndOptionalChain(t *testing.T) {
	cases := []struct{ name, src string }{
		{"nullish", "fn F() Number {\n    var x = a\n        ?? b\n    return x\n}\n"},
		{"optional_chain", "fn F() Number {\n    var x = a\n        ?.field\n    return x\n}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := parser.Parse(c.src, "p.fct", parser.SourceUser)
			if err != nil {
				t.Fatalf("multi-line continuation should parse, got: %v", err)
			}
			fn := src.Functions()[0]
			if len(fn.Body) != 2 {
				t.Fatalf("expected 2 statements (var + return) — the continuation merged into the var; got %d", len(fn.Body))
			}
		})
	}
}
