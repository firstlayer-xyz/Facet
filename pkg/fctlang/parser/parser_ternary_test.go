package parser_test

import (
	"testing"

	"facet/pkg/fctlang/parser"
)

// TestParseTernaryBasic confirms `cond ? a : b` parses as TernaryExpr.
func TestParseTernaryBasic(t *testing.T) {
	src := `fn Main() Number { return true ? 1 : -1 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[0].(*parser.ReturnStmt)
	tx, ok := ret.Value.(*parser.TernaryExpr)
	if !ok {
		t.Fatalf("expected *TernaryExpr, got %T", ret.Value)
	}
	if tx.Cond == nil || tx.Then == nil || tx.Else == nil {
		t.Error("Cond/Then/Else must all be populated")
	}
}

// TestParseTernaryRightAssociative confirms `a ? b : c ? d : e` parses
// as `a ? b : (c ? d : e)` — the conventional right-association.
func TestParseTernaryRightAssociative(t *testing.T) {
	src := `fn Main() Number { return true ? 1 : false ? 2 : 3 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[0].(*parser.ReturnStmt)
	outer, ok := ret.Value.(*parser.TernaryExpr)
	if !ok {
		t.Fatalf("expected outer *TernaryExpr, got %T", ret.Value)
	}
	if _, ok := outer.Else.(*parser.TernaryExpr); !ok {
		t.Errorf("expected the else arm to be a nested *TernaryExpr (right-assoc); got %T", outer.Else)
	}
}

// TestParseTernaryAfterLogicalOr confirms `a || b ? c : d` parses as
// `(a || b) ? c : d` — ternary binds looser than ||.
func TestParseTernaryAfterLogicalOr(t *testing.T) {
	src := `fn Main() Number { return false || true ? 1 : 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[0].(*parser.ReturnStmt)
	tx, ok := ret.Value.(*parser.TernaryExpr)
	if !ok {
		t.Fatalf("expected outer *TernaryExpr, got %T", ret.Value)
	}
	if _, ok := tx.Cond.(*parser.BinaryExpr); !ok {
		t.Errorf("expected the cond to be the || expression; got %T", tx.Cond)
	}
}

// TestParseTernaryInsideNamedArg confirms the inner `:` of `cond ? a : b`
// is not confused with the named-arg `:` when the ternary appears as a
// call argument value.
func TestParseTernaryInsideNamedArg(t *testing.T) {
	src := `fn Main() { var x = Cube(s: true ? 10 mm : 5 mm); return 0 }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("ternary inside named-arg should parse; got: %v", err)
	}
}
