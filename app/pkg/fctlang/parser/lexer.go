package parser

import (
	"fmt"
	"unicode"
)

// TokenType identifies the kind of token.
type TokenType int

const (
	TokenIdent     TokenType = iota // identifier
	TokenNumber                     // integer or float literal
	TokenLParen                     // (
	TokenRParen                     // )
	TokenLBrace                     // {
	TokenRBrace                     // }
	TokenComma                      // ,
	TokenSemicolon                  // ;
	TokenEquals                     // =
	TokenPlus                       // +
	TokenMinus                      // -
	TokenStar                       // *
	TokenSlash                      // /
	TokenMod                    // %
	TokenCaret                  // ^
	TokenDot                    // .
	TokenLBracket               // [
	TokenRBracket               // ]
	TokenLess                   // <
	TokenGreater                // >
	TokenLessEq                 // <=
	TokenGreaterEq              // >=
	TokenEqEq                   // ==
	TokenBang                   // !
	TokenBangEq                 // !=
	TokenAmp                    // &
	TokenAmpEq                  // &=
	TokenAmpAmp                 // &&
	TokenPipe                   // |
	TokenPipeEq                 // |=
	TokenPipePipe               // ||
	TokenReturn                     // return keyword
	TokenVar                        // var keyword
	TokenFor                        // for keyword
	TokenYield                      // yield keyword
	TokenFold                       // fold keyword
	TokenAssert                     // assert keyword
	TokenIf                         // if keyword
	TokenElse                       // else keyword
	TokenTrue                       // true keyword
	TokenFalse                      // false keyword
	TokenLib                        // lib keyword
	TokenTypeKw                     // type keyword
	TokenFn                         // fn keyword
	TokenConst                      // const keyword
	TokenWhere                      // where keyword
	TokenString                     // string literal
	TokenRawString                  // raw string literal (backtick)
	TokenColon                      // :
	TokenPlusEq                     // +=
	TokenMinusEq                    // -=
	TokenStarEq                     // *=
	TokenSlashEq                    // /=
	TokenModEq                      // %=
	TokenCaretEq                    // ^=
	TokenEOF
	TokenReserved // reserved keyword (cannot be used as identifier)
)

// Token is a single lexical token with position information.
type Token struct {
	Type TokenType
	Text string
	Line int
	Col  int
}

// lexer is a hand-written scanner for the facet language.
type lexer struct {
	src             []rune
	pos             int
	line            int
	col             int
	prevType        TokenType // type of the last token returned by Next
	pendingSemi     bool      // true when ASI has queued a synthetic semicolon
	pendingComments []Comment // comments collected since last drain
}

// lexerState is a snapshot of the lexer for lookahead/restore operations.
// Includes pendingComments to ensure backtracking restores comment state.
type lexerState struct {
	pos             int
	line            int
	col             int
	prevType        TokenType
	pendingSemi     bool
	pendingComments []Comment
}

// snapshot captures the lexer's current position for later restore.
func (l *lexer) snapshot() lexerState {
	// Deep copy pendingComments so restore doesn't alias the live slice.
	comments := make([]Comment, len(l.pendingComments))
	copy(comments, l.pendingComments)
	return lexerState{
		pos:             l.pos,
		line:            l.line,
		col:             l.col,
		prevType:        l.prevType,
		pendingSemi:     l.pendingSemi,
		pendingComments: comments,
	}
}

// restore resets the lexer to a previously captured snapshot.
func (l *lexer) restore(s lexerState) {
	l.pos = s.pos
	l.line = s.line
	l.col = s.col
	l.prevType = s.prevType
	l.pendingSemi = s.pendingSemi
	l.pendingComments = s.pendingComments
}

// drainComments returns all pending comments and clears the buffer.
func (l *lexer) drainComments() []Comment {
	if len(l.pendingComments) == 0 {
		return nil
	}
	comments := l.pendingComments
	l.pendingComments = nil
	return comments
}

// drainCommentsOnLine drains only pending comments that are on the given line.
// Comments on other lines remain in pendingComments.
func (l *lexer) drainCommentsOnLine(line int) []Comment {
	var taken, kept []Comment
	for _, c := range l.pendingComments {
		if c.Pos.Line == line {
			taken = append(taken, c)
		} else {
			kept = append(kept, c)
		}
	}
	l.pendingComments = kept
	return taken
}

// isLineTerminator reports whether a token type triggers automatic semicolon
// insertion when followed by a newline (Go-style ASI).
// TokenRBrace is intentionally excluded so that "} else {" on a new line works.
func isLineTerminator(t TokenType) bool {
	switch t {
	case TokenIdent, TokenNumber, TokenString, TokenRawString,
		TokenRParen, TokenRBracket,
		TokenTrue, TokenFalse, TokenYield:
		return true
	}
	return false
}

func newLexer(source string) *lexer {
	return &lexer{
		src:      []rune(source),
		pos:      0,
		line:     1,
		col:      1,
		prevType: TokenSemicolon, // treat start-of-file as already semicoloned
	}
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) advance() rune {
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.src) {
		ch := l.peek()
		// Handle newlines separately: a newline after a line-terminating token
		// queues a synthetic semicolon (Go-style ASI).
		if ch == '\n' {
			if isLineTerminator(l.prevType) {
				l.pendingSemi = true
			}
			l.advance()
			continue
		}
		if unicode.IsSpace(ch) {
			l.advance()
			continue
		}
		// Collect line comments: # (doc) or // (user)
		// Stop before \n so the next iteration handles ASI.
		if ch == '#' || (ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/') {
			isDoc := ch == '#'
			commentLine, commentCol := l.line, l.col
			l.advance()
			if ch == '/' {
				l.advance() // skip second /
			}
			start := l.pos
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
			text := string(l.src[start:l.pos])
			// Strip leading space
			if len(text) > 0 && text[0] == ' ' {
				text = text[1:]
			}
			l.pendingComments = append(l.pendingComments, Comment{
				Text:  text,
				Pos:   Pos{commentLine, commentCol},
				IsDoc: isDoc,
			})
			continue
		}
		break
	}
}

// Next returns the next token, performing automatic semicolon insertion (ASI).
func (l *lexer) Next() (Token, error) {
	tok, err := l.nextRaw()
	if err == nil {
		l.prevType = tok.Type
	}
	return tok, err
}

// nextRaw is the inner lexer, returning real tokens plus synthetic semicolons.
func (l *lexer) nextRaw() (Token, error) {
	l.skipWhitespaceAndComments()

	// Suppress ASI when the next real character unambiguously continues the
	// current expression rather than starting a new statement:
	//   )  ]             closing delimiter of a multi-line call/array
	//   .                method-chain continuation on the next line
	//   &  |             && / || binary operator on the next line
	//   +  -  *  /  %  ^  arithmetic continuation on the next line
	if l.pendingSemi && l.pos < len(l.src) {
		switch l.src[l.pos] {
		case ')', ']', '.', '&', '|', '+', '-', '*', '/', '%', '^':
			l.pendingSemi = false
		}
	}

	// Emit a synthetic semicolon queued by ASI on a newline.
	if l.pendingSemi {
		l.pendingSemi = false
		return Token{Type: TokenSemicolon, Text: ";", Line: l.line, Col: l.col}, nil
	}

	if l.pos >= len(l.src) {
		// Insert a final synthetic semicolon if the file ends without a newline.
		if isLineTerminator(l.prevType) {
			return Token{Type: TokenSemicolon, Text: ";", Line: l.line, Col: l.col}, nil
		}
		return Token{Type: TokenEOF, Line: l.line, Col: l.col}, nil
	}

	line, col := l.line, l.col
	ch := l.peek()

	switch ch {
	case '(':
		l.advance()
		return Token{Type: TokenLParen, Text: "(", Line: line, Col: col}, nil
	case ')':
		l.advance()
		return Token{Type: TokenRParen, Text: ")", Line: line, Col: col}, nil
	case '{':
		l.advance()
		return Token{Type: TokenLBrace, Text: "{", Line: line, Col: col}, nil
	case '}':
		l.advance()
		return Token{Type: TokenRBrace, Text: "}", Line: line, Col: col}, nil
	case ',':
		l.advance()
		return Token{Type: TokenComma, Text: ",", Line: line, Col: col}, nil
	case ';':
		l.advance()
		return Token{Type: TokenSemicolon, Text: ";", Line: line, Col: col}, nil
	case ':':
		l.advance()
		return Token{Type: TokenColon, Text: ":", Line: line, Col: col}, nil
	case '=':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenEqEq, Text: "==", Line: line, Col: col}, nil
		}
		return Token{Type: TokenEquals, Text: "=", Line: line, Col: col}, nil
	case '<':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenLessEq, Text: "<=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenLess, Text: "<", Line: line, Col: col}, nil
	case '>':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenGreaterEq, Text: ">=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenGreater, Text: ">", Line: line, Col: col}, nil
	case '!':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenBangEq, Text: "!=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenBang, Text: "!", Line: line, Col: col}, nil
	case '&':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '&' {
			l.advance()
			return Token{Type: TokenAmpAmp, Text: "&&", Line: line, Col: col}, nil
		}
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenAmpEq, Text: "&=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenAmp, Text: "&", Line: line, Col: col}, nil
	case '|':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '|' {
			l.advance()
			return Token{Type: TokenPipePipe, Text: "||", Line: line, Col: col}, nil
		}
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenPipeEq, Text: "|=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenPipe, Text: "|", Line: line, Col: col}, nil
	case '+':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenPlusEq, Text: "+=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenPlus, Text: "+", Line: line, Col: col}, nil
	case '-':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenMinusEq, Text: "-=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenMinus, Text: "-", Line: line, Col: col}, nil
	case '*':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenStarEq, Text: "*=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenStar, Text: "*", Line: line, Col: col}, nil
	case '/':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenSlashEq, Text: "/=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenSlash, Text: "/", Line: line, Col: col}, nil
	case '%':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenModEq, Text: "%=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenMod, Text: "%", Line: line, Col: col}, nil
	case '^':
		l.advance()
		if l.pos < len(l.src) && l.peek() == '=' {
			l.advance()
			return Token{Type: TokenCaretEq, Text: "^=", Line: line, Col: col}, nil
		}
		return Token{Type: TokenCaret, Text: "^", Line: line, Col: col}, nil
	case '.':
		l.advance()
		return Token{Type: TokenDot, Text: ".", Line: line, Col: col}, nil
	case '[':
		l.advance()
		return Token{Type: TokenLBracket, Text: "[", Line: line, Col: col}, nil
	case ']':
		l.advance()
		return Token{Type: TokenRBracket, Text: "]", Line: line, Col: col}, nil
	}

	// String literal: "..." with escape sequences
	if ch == '"' {
		l.advance() // consume opening quote
		var buf []rune
		for l.pos < len(l.src) && l.peek() != '"' {
			if l.peek() == '\n' {
				return Token{}, &SourceError{Line: line, Col: col, Message: "unterminated string literal"}
			}
			if l.peek() == '\\' {
				l.advance() // consume backslash
				if l.pos >= len(l.src) {
					return Token{}, &SourceError{Line: line, Col: col, Message: "unterminated string literal"}
				}
				switch l.peek() {
				case 'n':
					buf = append(buf, '\n')
				case 't':
					buf = append(buf, '\t')
				case '\\':
					buf = append(buf, '\\')
				case '"':
					buf = append(buf, '"')
				case 'u':
					// Unicode escape: \uXXXX (4 hex digits)
					l.advance() // consume 'u'
					if l.pos+4 > len(l.src) {
						return Token{}, &SourceError{Line: l.line, Col: l.col, Message: "incomplete unicode escape \\u (need 4 hex digits)"}
					}
					hex := string(l.src[l.pos : l.pos+4])
					var codepoint int
					for _, h := range hex {
						codepoint <<= 4
						switch {
						case h >= '0' && h <= '9':
							codepoint |= int(h - '0')
						case h >= 'a' && h <= 'f':
							codepoint |= int(h-'a') + 10
						case h >= 'A' && h <= 'F':
							codepoint |= int(h-'A') + 10
						default:
							return Token{}, &SourceError{Line: l.line, Col: l.col, Message: fmt.Sprintf("invalid hex digit %q in unicode escape", h)}
						}
					}
					buf = append(buf, rune(codepoint))
					l.pos += 4
					l.col += 4
					continue
				default:
					return Token{}, &SourceError{Line: l.line, Col: l.col, Message: fmt.Sprintf("unknown escape sequence \\%c", l.peek())}
				}
				l.advance()
				continue
			}
			buf = append(buf, l.peek())
			l.advance()
		}
		if l.pos >= len(l.src) {
			return Token{}, &SourceError{Line: line, Col: col, Message: "unterminated string literal"}
		}
		l.advance() // consume closing quote
		return Token{Type: TokenString, Text: string(buf), Line: line, Col: col}, nil
	}

	// Raw string literal: `...` (no escape processing, allows newlines)
	if ch == '`' {
		l.advance() // consume opening backtick
		start := l.pos
		for l.pos < len(l.src) && l.peek() != '`' {
			l.advance()
		}
		if l.pos >= len(l.src) {
			return Token{}, &SourceError{Line: line, Col: col, Message: "unterminated raw string literal"}
		}
		text := string(l.src[start:l.pos])
		l.advance() // consume closing backtick
		return Token{Type: TokenRawString, Text: text, Line: line, Col: col}, nil
	}

	// Number literal: integer, float, or ratio (e.g. 42, 3.14, 1/2)
	if unicode.IsDigit(ch) {
		start := l.pos
		for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
			l.advance()
		}
		if l.pos < len(l.src) && l.peek() == '.' &&
			l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
			l.advance() // decimal point
			for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
				l.advance()
			}
		}
		// Ratio: digits / digits (e.g. 1/2, 3/4)
		if l.pos < len(l.src) && l.peek() == '/' &&
			l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
			l.advance() // consume '/'
			for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
				l.advance()
			}
		}
		text := string(l.src[start:l.pos])
		return Token{Type: TokenNumber, Text: text, Line: line, Col: col}, nil
	}

	// Identifier or keyword
	if ch == '_' || unicode.IsLetter(ch) {
		start := l.pos
		for l.pos < len(l.src) {
			c := l.peek()
			if c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c) {
				l.advance()
			} else {
				break
			}
		}
		text := string(l.src[start:l.pos])
		typ := TokenIdent
		switch text {
		case "return":
			typ = TokenReturn
		case "var":
			typ = TokenVar
		case "for":
			typ = TokenFor
		case "yield":
			typ = TokenYield
		case "fold":
			typ = TokenFold
		case "assert":
			typ = TokenAssert
		case "if":
			typ = TokenIf
		case "else":
			typ = TokenElse
		case "true":
			typ = TokenTrue
		case "false":
			typ = TokenFalse
		case "lib":
			typ = TokenLib
		case "type":
			typ = TokenTypeKw
		case "fn":
			typ = TokenFn
		case "const":
			typ = TokenConst
		case "where":
			typ = TokenWhere
		case "while", "break", "continue", "match", "case", "import", "export", "map":
			return Token{Type: TokenReserved, Text: text, Line: line, Col: col},
				&SourceError{Line: line, Col: col, Message: fmt.Sprintf("%q is a reserved keyword and cannot be used as an identifier", text)}
		}
		return Token{Type: typ, Text: text, Line: line, Col: col}, nil
	}

	return Token{}, &SourceError{Line: line, Col: col, Message: fmt.Sprintf("unexpected character %q", ch)}
}
