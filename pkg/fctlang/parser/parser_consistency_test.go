package parser_test

import (
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// `if var NAME = …` binds a variable and must reject a reserved `_`-prefixed
// name like every other binding site.
func TestParseIfVarRejectsUnderscore(t *testing.T) {
	_, err := parser.Parse("fn F() Number {\n    if var _x = m() {\n        return 1\n    }\n    return 0\n}\n", "p.fct", parser.SourceUser)
	if err == nil {
		t.Fatal("expected an error for `if var _x` (reserved identifier)")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected a reserved-identifier error, got: %v", err)
	}
}

// A struct field's `where` clause is parsed by the shared parseWhereConstraint,
// so it enforces the `[` after `where` like every other where site.
func TestParseStructFieldWhereUsesSharedParser(t *testing.T) {
	if _, err := parser.Parse("type T {\n    x Number where [x > 0]\n}\nfn F() Number { return 0 }\n", "p.fct", parser.SourceUser); err != nil {
		t.Fatalf("valid struct-field where should parse: %v", err)
	}
	_, err := parser.Parse("type T {\n    x Number where x > 0\n}\nfn F() Number { return 0 }\n", "p.fct", parser.SourceUser)
	if err == nil {
		t.Fatal("expected an error for a struct-field where without [")
	}
	if !strings.Contains(err.Error(), "after 'where'") {
		t.Fatalf("expected the shared where-parser error, got: %v", err)
	}
}
