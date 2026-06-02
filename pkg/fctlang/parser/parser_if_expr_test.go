package parser_test

import (
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// parseFirstVarValue parses src as a single-function program and returns
// the first var's initializer expression — handy for asserting the
// shape of an expression embedded in a var declaration.
func parseFirstVarValue(t *testing.T, src string) parser.Expr {
	t.Helper()
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Functions()) == 0 {
		t.Fatal("no functions parsed")
	}
	for _, s := range prog.Functions()[0].Body {
		if v, ok := s.(*parser.VarStmt); ok {
			return v.Value
		}
	}
	t.Fatal("no var statement in function body")
	return nil
}

// TestParseIfExpressionBasic confirms the simplest if-expression form
// parses into an IfExpr with both arms populated.
func TestParseIfExpressionBasic(t *testing.T) {
	src := `fn Main() Number { var c = if 1 > 0 { 1 } else { -1 }; return c }`
	v := parseFirstVarValue(t, src)
	ife, ok := v.(*parser.IfExpr)
	if !ok {
		t.Fatalf("expected *parser.IfExpr, got %T", v)
	}
	if ife.Then == nil {
		t.Error("then arm not populated")
	}
	if ife.Else == nil {
		t.Error("else arm not populated")
	}
	if len(ife.ElseIfs) != 0 {
		t.Errorf("expected no else-ifs, got %d", len(ife.ElseIfs))
	}
}

// TestParseIfExpressionElseIfChain confirms an else-if chain parses
// into the right number of ElseIfExprClause entries plus a final else.
func TestParseIfExpressionElseIfChain(t *testing.T) {
	src := `fn Main() Number {
		var c = if 1 > 0 { 1 } else if 1 < 0 { -1 } else if 1 == 0 { 0 } else { 99 };
		return c
	}`
	v := parseFirstVarValue(t, src)
	ife, ok := v.(*parser.IfExpr)
	if !ok {
		t.Fatalf("expected *parser.IfExpr, got %T", v)
	}
	if len(ife.ElseIfs) != 2 {
		t.Errorf("expected 2 else-ifs, got %d", len(ife.ElseIfs))
	}
	if ife.Else == nil {
		t.Error("final else arm missing")
	}
}

// TestParseIfExpressionRejectsMissingElse pins the parse-time guard:
// without an else arm the expression has no value, so we error out
// before reaching the checker.
func TestParseIfExpressionRejectsMissingElse(t *testing.T) {
	src := `fn Main() Number { var c = if 1 > 0 { 1 }; return c }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected parse error for if expression without else")
	}
	if !strings.Contains(err.Error(), "else") {
		t.Errorf("error should mention else; got: %v", err)
	}
}

// TestParseIfExpressionInStructField confirms an if-expression nests
// inside a struct literal field initializer without parse confusion
// — the outer struct braces and the if-arm braces don't collide.
func TestParseIfExpressionInStructField(t *testing.T) {
	src := `fn Main() Solid {
		return Cube(s: Vec3{x: if 1 > 0 { 10 mm } else { 5 mm }, y: 1 mm, z: 1 mm})
	}`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("if expression inside struct field should parse cleanly; got: %v", err)
	}
}

// TestParseIfStmtStillWorks regression-guards the existing statement
// form — `if` at statement position must still parse as an IfStmt
// even though `if` is now also recognized at expression position.
func TestParseIfStmtStillWorks(t *testing.T) {
	src := `fn Main() Number {
		if 1 > 0 { return 1 }
		return 0
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := prog.Functions()[0]
	if _, ok := fn.Body[0].(*parser.IfStmt); !ok {
		t.Fatalf("expected first body stmt to be *parser.IfStmt, got %T", fn.Body[0])
	}
}
