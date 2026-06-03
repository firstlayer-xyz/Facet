package lexer

import (
	"slices"
	"testing"

	"facet/pkg/scad/token"
)

func kinds(toks []token.Token) []token.Kind {
	out := make([]token.Kind, len(toks))
	for i, t := range toks {
		out[i] = t.Kind
	}
	return out
}

func TestLex_CubeCall(t *testing.T) {
	toks := Lex("cube(10);")
	want := []token.Kind{token.Ident, token.LParen, token.Number, token.RParen, token.Semi, token.EOF}
	if got := kinds(toks); !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	if toks[0].Text != "cube" || toks[2].Text != "10" {
		t.Fatalf("texts = %q,%q", toks[0].Text, toks[2].Text)
	}
}

func TestLex_SpecialVarAndComments(t *testing.T) {
	toks := Lex("sphere(r=2,$fn=6); // trailing\n/* block */ x=1;")
	var sawFn bool
	for _, tk := range toks {
		if tk.Kind == token.Ident && tk.Text == "$fn" && tk.SpecialVar {
			sawFn = true
		}
	}
	if !sawFn {
		t.Fatalf("expected $fn special-var token; toks=%v", toks)
	}
	for _, tk := range toks {
		if tk.Text == "trailing" || tk.Text == "block" {
			t.Fatalf("comment leaked into tokens: %v", tk)
		}
	}
}

func TestLex_NumbersAndOperators(t *testing.T) {
	toks := Lex("a = 1.5 + .5 - 2e3 <= b && c;")
	want := []token.Kind{
		token.Ident, token.Assign, token.Number, token.Plus, token.Number,
		token.Minus, token.Number, token.Le, token.Ident, token.And, token.Ident,
		token.Semi, token.EOF,
	}
	if got := kinds(toks); !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

func TestLex_MalformedInputNeverPanics(t *testing.T) {
	// The lexer must never panic on malformed input ("never fail hard").
	for _, src := range []string{`"\`, `"unterminated`, `/* unterminated`, `1.2.3`, `1e`, "\x00\x01"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Lex(%q) panicked: %v", src, r)
				}
			}()
			_ = Lex(src)
		}()
	}
}
