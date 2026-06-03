package parser_test

import (
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// TestParseOptionalReturnType confirms `T?` is accepted in return position
// and the type string carries the trailing `?`.
func TestParseOptionalReturnType(t *testing.T) {
	src := `fn Lookup() Number? { return nil }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	if fn.ReturnType != "Number?" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "Number?")
	}
}

// TestParseOptionalParamType confirms `T?` is accepted in parameter position.
func TestParseOptionalParamType(t *testing.T) {
	src := `fn Take(x Number?) { return 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	if fn.Params[0].Type != "Number?" {
		t.Errorf("param type = %q, want %q", fn.Params[0].Type, "Number?")
	}
}

// TestParseOptionalArrayElement confirms `[]T?` parses as array of optionals
// (the `?` binds tighter than `[]`).
func TestParseOptionalArrayElement(t *testing.T) {
	src := `fn MaybeList() []Number? { return [] }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	if fn.ReturnType != "[]Number?" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "[]Number?")
	}
}

// TestParseDoubleOptionalRejected pins the no-nesting rule: `T??` errors.
func TestParseDoubleOptionalRejected(t *testing.T) {
	src := `fn Bad() Number?? { return nil }`
	_, err := parser.Parse(src, "", parser.SourceUser)
	if err == nil {
		t.Fatal("expected parse error for Number??")
	}
	if !strings.Contains(err.Error(), "nested optional") {
		t.Errorf("error should mention nested optional; got: %v", err)
	}
}

// TestParseNilLiteral confirms `nil` parses to a *NilLit.
func TestParseNilLiteral(t *testing.T) {
	src := `fn Main() { var x = nil; return 0 }`
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := prog.Functions()[0]
	v, ok := fn.Body[0].(*parser.VarStmt)
	if !ok {
		t.Fatalf("expected VarStmt, got %T", fn.Body[0])
	}
	if _, ok := v.Value.(*parser.NilLit); !ok {
		t.Errorf("expected NilLit, got %T", v.Value)
	}
}
