package parser_test

import (
	"encoding/json"
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// The AST-JSON marshaller must serialize every node type. A ternary was missing
// a case and fell through to the {"type":"Unknown"} placeholder (silently
// dropping it); optional chaining (?.) didn't record its flag.
func TestASTJSONSerializesTernaryAndOptionalChain(t *testing.T) {
	src, err := parser.Parse(
		"fn F() Number {\n    return cond ? a : b\n}\n"+
			"fn G() Number {\n    return obj?.field ?? 0\n}\n",
		"p.fct", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `"TernaryExpr"`) {
		t.Errorf("ternary not serialized:\n%s", s)
	}
	if strings.Contains(s, `"Unknown"`) {
		t.Errorf("AST contains an Unknown placeholder (a node type is unhandled):\n%s", s)
	}
	if !strings.Contains(s, `"optional":true`) {
		t.Errorf("optional-chain (?.) flag not serialized:\n%s", s)
	}
}
