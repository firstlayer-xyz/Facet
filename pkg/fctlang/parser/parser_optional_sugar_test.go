package parser_test

import (
	"testing"

	"facet/pkg/fctlang/parser"
)

// TestParseNullCoalesceBinary confirms `expr ?? expr` parses as a binary
// expression with operator "??".
func TestParseNullCoalesceBinary(t *testing.T) {
	src := `fn Main() Number { var x = nil; return x ?? 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[1].(*parser.ReturnStmt)
	bin, ok := ret.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if bin.Op != "??" {
		t.Errorf("operator = %q, want ??", bin.Op)
	}
}

// TestParseOptionalChainingField confirms `expr?.field` parses to a
// FieldAccessExpr with Optional=true.
func TestParseOptionalChainingField(t *testing.T) {
	src := `fn Main() { var v = nil; var x = v?.Width; return 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[1].(*parser.VarStmt)
	fa, ok := v.Value.(*parser.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", v.Value)
	}
	if !fa.Optional {
		t.Error("expected Optional=true on ?.field access")
	}
}

// TestParseOptionalChainingMethod confirms `expr?.Method()` parses to a
// MethodCallExpr with Optional=true.
func TestParseOptionalChainingMethod(t *testing.T) {
	src := `fn Main() { var v = nil; var x = v?.Foo(); return 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	v := fn.Body[1].(*parser.VarStmt)
	mc, ok := v.Value.(*parser.MethodCallExpr)
	if !ok {
		t.Fatalf("expected MethodCallExpr, got %T", v.Value)
	}
	if !mc.Optional {
		t.Error("expected Optional=true on ?.method() call")
	}
}

// TestParseIfVarBindForm confirms `if var NAME = expr { ... }` parses to
// an IfStmt with BindVar set.
func TestParseIfVarBindForm(t *testing.T) {
	src := `fn Main() Number {
		var maybe = nil;
		if var x = maybe { return x }
		return 0
	}`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ifs, ok := fn.Body[1].(*parser.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", fn.Body[1])
	}
	if ifs.BindVar != "x" {
		t.Errorf("BindVar = %q, want %q", ifs.BindVar, "x")
	}
}

// TestParseNullCoalesceLowerThanOr confirms `??` binds tighter than `||`:
// `a || b ?? c` parses as `a || (b ?? c)`.
func TestParseNullCoalescePrecedence(t *testing.T) {
	src := `fn Main() Bool { return true || nil ?? false }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	ret := fn.Body[0].(*parser.ReturnStmt)
	or, ok := ret.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected outer BinaryExpr, got %T", ret.Value)
	}
	if or.Op != "||" {
		t.Errorf("outer op = %q, want ||", or.Op)
	}
	inner, ok := or.Right.(*parser.BinaryExpr)
	if !ok || inner.Op != "??" {
		t.Errorf("expected inner ?? expression on the right; got %T", or.Right)
	}
}
