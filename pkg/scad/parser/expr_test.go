package parser

import (
	"testing"

	"facet/pkg/scad/ast"
	"facet/pkg/scad/lexer"
)

// parseExprSrc parses a single expression by driving the parser directly.
func parseExprSrc(t *testing.T, src string) ast.Expr {
	t.Helper()
	p := &parser{toks: lexer.Lex(src)}
	e, err := p.parseExpr()
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return e
}

func TestExpr_Precedence(t *testing.T) {
	e := parseExprSrc(t, "1 + 2 * 3")
	b := e.(*ast.Binary)
	if b.Op != "+" {
		t.Fatalf("top op = %q, want +", b.Op)
	}
	if r, ok := b.R.(*ast.Binary); !ok || r.Op != "*" {
		t.Fatalf("right should be 2*3, got %#v", b.R)
	}
}

func TestExpr_LeftAssociative(t *testing.T) {
	// 1 - 2 - 3 must parse as (1 - 2) - 3, i.e. left operand is itself a Binary.
	b := parseExprSrc(t, "1 - 2 - 3").(*ast.Binary)
	if b.Op != "-" {
		t.Fatalf("top op = %q", b.Op)
	}
	if _, ok := b.L.(*ast.Binary); !ok {
		t.Fatalf("left should be (1-2) Binary, got %#v", b.L)
	}
	if _, ok := b.R.(*ast.Num); !ok {
		t.Fatalf("right should be Num 3, got %#v", b.R)
	}
}

func TestExpr_VectorAndRange(t *testing.T) {
	v := parseExprSrc(t, "[1, 2, 3]").(*ast.Vector)
	if len(v.Elems) != 3 {
		t.Fatalf("vector len = %d", len(v.Elems))
	}
	r := parseExprSrc(t, "[0:2:10]").(*ast.Range)
	if r.Step == nil {
		t.Fatal("expected step in [0:2:10]")
	}
	// OpenSCAD step-in-MIDDLE: [0:2:10] means Start=0, Step=2, End=10.
	if r.Start.(*ast.Num).Text != "0" || r.Step.(*ast.Num).Text != "2" || r.End.(*ast.Num).Text != "10" {
		t.Fatalf("range fields wrong: start=%v step=%v end=%v", r.Start, r.Step, r.End)
	}
	r2 := parseExprSrc(t, "[0:10]").(*ast.Range)
	if r2.Step != nil {
		t.Fatal("expected no step in [0:10]")
	}
}

func TestExpr_TernaryAndCall(t *testing.T) {
	tn := parseExprSrc(t, "a ? b : c").(*ast.Ternary)
	if _, ok := tn.Cond.(*ast.Ident); !ok {
		t.Fatal("ternary cond should be ident")
	}
	c := parseExprSrc(t, "sin(30)").(*ast.Call)
	if c.Name != "sin" || len(c.Args) != 1 {
		t.Fatalf("call = %#v", c)
	}
}

func TestExpr_MemberIndexUnary(t *testing.T) {
	idx := parseExprSrc(t, "v[0]").(*ast.Index)
	if _, ok := idx.X.(*ast.Ident); !ok {
		t.Fatal("index target should be ident")
	}
	m := parseExprSrc(t, "p.x").(*ast.Member)
	if m.Name != "x" {
		t.Fatalf("member = %q", m.Name)
	}
	u := parseExprSrc(t, "-a").(*ast.Unary)
	if u.Op != "-" {
		t.Fatalf("unary = %q", u.Op)
	}
}

func TestExpr_MalformedNeverHangs(t *testing.T) {
	// Malformed input must return (an error is fine) and never hang or panic.
	for _, src := range []string{"sin(30", "f(a b)", "[1,2", "(1+", "a ?", "[0:"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parseExpr(%q) panicked: %v", src, r)
				}
			}()
			p := &parser{toks: lexer.Lex(src)}
			_, _ = p.parseExpr()
		}()
	}
}
