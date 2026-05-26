package parser

import "testing"

func TestLexerRawString(t *testing.T) {
	src := "`hello world`"
	l := newLexer(src)
	tok, err := l.Next()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	if tok.Type != TokenRawString {
		t.Fatalf("expected TokenRawString, got %v", tok.Type)
	}
	if tok.Text != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", tok.Text)
	}
}

func TestLexerRawStringUnterminated(t *testing.T) {
	src := "`unterminated"
	l := newLexer(src)
	_, err := l.Next()
	if err == nil {
		t.Fatal("expected error for unterminated raw string")
	}
}

func TestLexKeywords(t *testing.T) {
	l := newLexer("for yield fold")
	expected := []struct {
		typ  TokenType
		text string
	}{
		{TokenFor, "for"},
		{TokenYield, "yield"},
		{TokenFold, "fold"},
	}
	for _, want := range expected {
		tok, err := l.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Type != want.typ || tok.Text != want.text {
			t.Errorf("got %v %q, want %v %q", tok.Type, tok.Text, want.typ, want.text)
		}
	}
}

func TestLexComparisonTokens(t *testing.T) {
	l := newLexer("< > <= >= == !=")
	expected := []struct {
		typ  TokenType
		text string
	}{
		{TokenLess, "<"},
		{TokenGreater, ">"},
		{TokenLessEq, "<="},
		{TokenGreaterEq, ">="},
		{TokenEqEq, "=="},
		{TokenBangEq, "!="},
	}
	for _, want := range expected {
		tok, err := l.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Type != want.typ || tok.Text != want.text {
			t.Errorf("got %v %q, want %v %q", tok.Type, tok.Text, want.typ, want.text)
		}
	}
}

func TestLexLogicalTokens(t *testing.T) {
	l := newLexer("&& || true false if else")
	expected := []struct {
		typ  TokenType
		text string
	}{
		{TokenAmpAmp, "&&"},
		{TokenPipePipe, "||"},
		{TokenTrue, "true"},
		{TokenFalse, "false"},
		{TokenIf, "if"},
		{TokenElse, "else"},
	}
	for _, want := range expected {
		tok, err := l.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Type != want.typ || tok.Text != want.text {
			t.Errorf("got %v %q, want %v %q", tok.Type, tok.Text, want.typ, want.text)
		}
	}
}

func TestLexStringLiteral(t *testing.T) {
	l := newLexer(`"hello world"`)
	tok, err := l.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Type != TokenString {
		t.Errorf("type = %v, want TokenString", tok.Type)
	}
	if tok.Text != "hello world" {
		t.Errorf("text = %q, want %q", tok.Text, "hello world")
	}
}

func TestLexStringEscapeSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"newline", `"a\nb"`, "a\nb"},
		{"tab", `"a\tb"`, "a\tb"},
		{"escaped quote", `"a\"b"`, `a"b`},
		{"escaped backslash", `"a\\b"`, `a\b`},
		{"multiple escapes", `"\\n\n"`, "\\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer(tt.input)
			tok, err := l.Next()
			if err != nil {
				t.Fatalf("lex error: %v", err)
			}
			if tok.Type != TokenString {
				t.Fatalf("expected TokenString, got %v", tok.Type)
			}
			if tok.Text != tt.want {
				t.Errorf("got %q, want %q", tok.Text, tt.want)
			}
		})
	}
}

func TestLexStringUnknownEscapeError(t *testing.T) {
	l := newLexer(`"a\db"`)
	_, err := l.Next()
	if err == nil {
		t.Fatal("expected error for unknown escape sequence")
	}
}

func TestLexStringUnicodeEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic", `"\u0041"`, "A"},
		{"emoji-ish", `"\u00E9"`, "\u00E9"},
		{"in context", `"caf\u00E9"`, "caf\u00E9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer(tt.input)
			tok, err := l.Next()
			if err != nil {
				t.Fatalf("lex error: %v", err)
			}
			if tok.Text != tt.want {
				t.Errorf("got %q, want %q", tok.Text, tt.want)
			}
		})
	}
}

func TestLexStringUnicodeEscapeInvalid(t *testing.T) {
	l := newLexer(`"\u00GG"`)
	_, err := l.Next()
	if err == nil {
		t.Fatal("expected error for invalid hex digit in unicode escape")
	}
}

func TestLexLibKeyword(t *testing.T) {
	l := newLexer("lib")
	tok, err := l.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Type != TokenLib {
		t.Errorf("type = %v, want TokenLib", tok.Type)
	}
	if tok.Text != "lib" {
		t.Errorf("text = %q, want %q", tok.Text, "lib")
	}
}

func TestLexAssertKeyword(t *testing.T) {
	l := newLexer("assert")
	tok, err := l.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Type != TokenAssert {
		t.Errorf("type = %v, want TokenAssert", tok.Type)
	}
	if tok.Text != "assert" {
		t.Errorf("text = %q, want %q", tok.Text, "assert")
	}
}

func TestLexBangToken(t *testing.T) {
	l := newLexer("! !=")
	tok, err := l.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Type != TokenBang || tok.Text != "!" {
		t.Errorf("got %v %q, want TokenBang '!'", tok.Type, tok.Text)
	}
	tok, err = l.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Type != TokenBangEq || tok.Text != "!=" {
		t.Errorf("got %v %q, want TokenBangEq '!='", tok.Type, tok.Text)
	}
}
