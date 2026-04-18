package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"strings"
	"testing"
)

// coerceAnonymousStruct must return a hard error when the target type has
// no declaration — no silent stamping of a made-up typeName. In production
// this path is blocked upstream by isAccessibleType; this test is
// defense-in-depth.

func TestCoerceAnonymousStructUnknownTypeErrors(t *testing.T) {
	e := &evaluator{
		structDecls:  map[string]*parser.StructDecl{},
		libEvalCache: map[string]map[string]value{},
	}
	sv := &structVal{fields: map[string]value{"x": float64(1)}}
	err := e.coerceAnonymousStruct(sv, "NotDeclaredAnywhere", nil)
	if err == nil {
		t.Fatal("expected error when target struct type is not declared")
	}
	if !strings.Contains(err.Error(), "NotDeclaredAnywhere") {
		t.Errorf("error should name the missing type: %v", err)
	}
	if sv.typeName != "" {
		t.Errorf("typeName should remain empty on coercion failure, got %q", sv.typeName)
	}
}
