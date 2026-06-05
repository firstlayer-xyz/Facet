// Package lexer turns OpenSCAD source into a token stream.
package lexer

import (
	"strings"

	"facet/pkg/scad/token"
)

type lexer struct {
	src       string
	pos       int
	line, col int
	// pathNext is set after emitting a `use`/`include` keyword so the following
	// `<...>` is lexed as a single Path token rather than a `<` operator.
	pathNext bool
}

// singleKind maps single-byte punctuation/operators to their token kinds.
var singleKind = map[byte]token.Kind{
	'(': token.LParen, ')': token.RParen, '{': token.LBrace, '}': token.RBrace,
	'[': token.LBracket, ']': token.RBracket, ';': token.Semi, ',': token.Comma,
	':': token.Colon, '.': token.Dot, '?': token.Question, '#': token.Hash,
	'!': token.Bang, '%': token.Percent, '*': token.Star, '=': token.Assign,
	'+': token.Plus, '-': token.Minus, '/': token.Slash, '<': token.Lt, '>': token.Gt,
}

// Lex scans the whole source and returns tokens terminated by an EOF token.
func Lex(src string) []token.Token {
	l := &lexer{src: src, line: 1, col: 1}
	var out []token.Token
	for {
		tk := l.next()
		out = append(out, tk)
		if tk.Kind == token.EOF {
			return out
		}
	}
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peek2() byte {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func (l *lexer) skipTrivia() {
	for l.pos < len(l.src) {
		c := l.peek()
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			l.advance()
		case c == '/' && l.peek2() == '/':
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
		case c == '/' && l.peek2() == '*':
			l.advance()
			l.advance()
			for l.pos < len(l.src) && !(l.peek() == '*' && l.peek2() == '/') {
				l.advance()
			}
			if l.pos < len(l.src) {
				l.advance() // *
				l.advance() // /
			}
		default:
			return
		}
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentPart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || isDigit(c)
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (l *lexer) next() token.Token {
	wantPath := l.pathNext
	l.pathNext = false
	for {
		l.skipTrivia()
		line, col := l.line, l.col
		if l.pos >= len(l.src) {
			return token.Token{Kind: token.EOF, Line: line, Col: col}
		}
		c := l.peek()

		// A `<...>` file reference immediately after use/include is one Path
		// token (the angle brackets are stripped). Only valid in that position,
		// so a bare `<` elsewhere stays the less-than operator.
		if wantPath && c == '<' {
			l.advance() // <
			start := l.pos
			for l.pos < len(l.src) && l.peek() != '>' && l.peek() != '\n' {
				l.advance()
			}
			text := l.src[start:l.pos]
			if l.peek() == '>' {
				l.advance()
			}
			return token.Token{Kind: token.Path, Text: text, Line: line, Col: col}
		}

		switch {
		case isIdentStart(c):
			start := l.pos
			special := c == '$'
			l.advance() // consume the start byte (may be '$')
			for l.pos < len(l.src) && isIdentPart(l.peek()) {
				l.advance()
			}
			text := l.src[start:l.pos]
			k := token.Lookup(text)
			if special {
				k = token.Ident // $-vars are never keywords
			}
			if k == token.Use || k == token.Include {
				l.pathNext = true
			}
			return token.Token{Kind: k, Text: text, Line: line, Col: col, SpecialVar: special}

		case isDigit(c) || (c == '.' && isDigit(l.peek2())):
			start := l.pos
			for l.pos < len(l.src) && (isDigit(l.peek()) || l.peek() == '.') {
				l.advance()
			}
			if l.peek() == 'e' || l.peek() == 'E' {
				l.advance()
				if l.peek() == '+' || l.peek() == '-' {
					l.advance()
				}
				for l.pos < len(l.src) && isDigit(l.peek()) {
					l.advance()
				}
			}
			return token.Token{Kind: token.Number, Text: l.src[start:l.pos], Line: line, Col: col}

		case c == '"':
			l.advance()
			var sb strings.Builder
			for l.pos < len(l.src) && l.peek() != '"' {
				if l.peek() == '\\' {
					l.advance() // skip the backslash
					if l.pos >= len(l.src) {
						break
					}
				}
				sb.WriteByte(l.advance())
			}
			if l.peek() == '"' {
				l.advance()
			}
			return token.Token{Kind: token.String, Text: sb.String(), Line: line, Col: col}
		}

		// operators / punctuation — match two-char before single-char
		two := ""
		if l.pos+1 < len(l.src) {
			two = l.src[l.pos : l.pos+2]
		}
		switch two {
		case "==":
			l.advance()
			l.advance()
			return token.Token{Kind: token.EqEq, Line: line, Col: col}
		case "!=":
			l.advance()
			l.advance()
			return token.Token{Kind: token.NeEq, Line: line, Col: col}
		case "<=":
			l.advance()
			l.advance()
			return token.Token{Kind: token.Le, Line: line, Col: col}
		case ">=":
			l.advance()
			l.advance()
			return token.Token{Kind: token.Ge, Line: line, Col: col}
		case "&&":
			l.advance()
			l.advance()
			return token.Token{Kind: token.And, Line: line, Col: col}
		case "||":
			l.advance()
			l.advance()
			return token.Token{Kind: token.Or, Line: line, Col: col}
		}

		l.advance()
		if k, ok := singleKind[c]; ok {
			return token.Token{Kind: k, Line: line, Col: col}
		}
		// Unknown byte: skip and continue the loop. The lexer never fails hard; the parser reports errors.
	}
}
