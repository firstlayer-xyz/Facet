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
		if tk.Kind == token.Ident && tk.Text == "$fn" {
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

// `include <path>` lexes the angle-bracket file reference as a single Path
// token whose Text is the inner path, so the parser can read it directly.
func TestLex_IncludePath(t *testing.T) {
	toks := Lex("include <BOSL2/std.scad>\n")
	if toks[0].Kind != token.Include {
		t.Fatalf("toks[0] = %v, want Include", toks[0].Kind)
	}
	if toks[1].Kind != token.Path {
		t.Fatalf("toks[1] = %v, want Path", toks[1].Kind)
	}
	if toks[1].Text != "BOSL2/std.scad" {
		t.Fatalf("path text = %q, want %q", toks[1].Text, "BOSL2/std.scad")
	}
}

// `use <path>` is lexed the same way as include.
func TestLex_UsePath(t *testing.T) {
	toks := Lex("use <foo/bar.scad>\n")
	if toks[0].Kind != token.Use || toks[1].Kind != token.Path {
		t.Fatalf("kinds = %v %v, want Use Path", toks[0].Kind, toks[1].Kind)
	}
	if toks[1].Text != "foo/bar.scad" {
		t.Fatalf("path text = %q, want %q", toks[1].Text, "foo/bar.scad")
	}
}

// A `<` that is NOT preceded by use/include is still the less-than operator —
// path mode is context-sensitive and must not swallow comparisons.
func TestLex_LessThanStaysOperator(t *testing.T) {
	toks := Lex("a < b")
	want := []token.Kind{token.Ident, token.Lt, token.Ident, token.EOF}
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
