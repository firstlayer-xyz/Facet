package parser

import "strings"

// parsePrimary → "-" postfix | "!" postfix | number [unit] | ident [ "(" [args] ")" ] | "(" expr ")" | block
func (p *parser) parsePrimary() (Expr, error) {
	// Unary minus: -expr (binds looser than postfix so -self.field == -(self.field))
	// Boolean NOT: !expr
	if p.cur.Type == TokenMinus || p.cur.Type == TokenBang {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		operand, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{
			Op:      op,
			Operand: operand,
			Pos:     Pos{line, col},
		}, nil
	}

	// Lambda expression: fn(params) ReturnType { body }
	if p.cur.Type == TokenFn {
		return p.parseLambda()
	}

	if p.cur.Type == TokenNumber {
		raw := p.cur.Text
		val, err := parseNumberText(raw)
		if err != nil {
			return nil, p.errorf("invalid number %q", raw)
		}
		if err := p.next(); err != nil {
			return nil, err
		}
		return &NumberLit{Value: val, Raw: raw}, nil
	}

	if p.cur.Type == TokenIdent {
		name := p.cur.Text
		nameLine, nameCol := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		// If followed by '(', this is a call
		if p.cur.Type == TokenLParen {
			if err := p.next(); err != nil { // consume '('
				return nil, err
			}
			args, err := p.parseCallArgs()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
			if strings.HasPrefix(name, "_") {
				return &BuiltinCallExpr{Name: name, Args: args, Pos: Pos{nameLine, nameCol}}, nil
			}
			return &CallExpr{Name: name, Args: args, Pos: Pos{nameLine, nameCol}}, nil
		}
		// If followed by '{', this might be a struct literal.
		// Disambiguate by peeking ahead for the IDENT : pattern or empty {}.
		if p.cur.Type == TokenLBrace {
			if p.isStructLitStart() || p.isEmptyBrace() {
				return p.parseStructLit(name, nameLine, nameCol)
			}
		}
		return &IdentExpr{Name: name, Pos: Pos{nameLine, nameCol}}, nil
	}

	// Parenthesized expression: "(" expr ")"
	if p.cur.Type == TokenLParen {
		if err := p.next(); err != nil {
			return nil, err
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil
	}

	// Anonymous struct literal: "{ IDENT : expr, ... }"
	if p.cur.Type == TokenLBrace {
		if p.isStructLitStart() {
			line, col := p.cur.Line, p.cur.Col
			return p.parseStructLit("", line, col)
		}
		return nil, p.errorf("unexpected '{'")
	}

	// Array literal: "[" [ expr { "," expr } ] "]"
	if p.cur.Type == TokenLBracket {
		return p.parseArrayLit()
	}

	// For expression: "for" IDENT expr "{" body "}"
	if p.cur.Type == TokenFor {
		return p.parseForExpr()
	}

	// Fold expression: "fold" accVar "," elemVar expr "{" body "}"
	if p.cur.Type == TokenFold {
		return p.parseFoldExpr()
	}

	// Boolean literals
	if p.cur.Type == TokenTrue {
		if err := p.next(); err != nil {
			return nil, err
		}
		return &BoolLit{Value: true}, nil
	}
	if p.cur.Type == TokenFalse {
		if err := p.next(); err != nil {
			return nil, err
		}
		return &BoolLit{Value: false}, nil
	}

	// String literal (quoted or raw backtick)
	if p.cur.Type == TokenString || p.cur.Type == TokenRawString {
		val := p.cur.Text
		if err := p.next(); err != nil {
			return nil, err
		}
		return &StringLit{Value: val}, nil
	}

	// Lib expression: lib "path"
	if p.cur.Type == TokenLib {
		libLine, libCol := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil { // consume 'lib'
			return nil, err
		}
		pathTok, err := p.expect(TokenString)
		if err != nil {
			return nil, err
		}
		return &LibExpr{Path: pathTok.Text, Pos: Pos{libLine, libCol}}, nil
	}

	return nil, p.errorf("expected expression, got %q", p.cur.Text)
}

// parseCallArgs parses function call arguments. All arguments use named form
// (name: expr). Internal _-prefixed builtins may use bare expressions.
// Named arguments are wrapped in *NamedArg nodes.
func (p *parser) parseCallArgs() ([]Expr, error) {
	var args []Expr
	if p.cur.Type == TokenRParen {
		return args, nil
	}
	for {
		// Try to parse named argument: IDENT ":"
		if p.cur.Type == TokenIdent {
			snap := p.lex.snapshot()
			savedCur := p.cur
			nameTok := p.cur
			if err := p.next(); err != nil {
				return nil, err
			}
			if p.cur.Type == TokenColon {
				// Named argument
				if err := p.next(); err != nil { // consume ':'
					return nil, err
				}
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, &NamedArg{
					Name:  nameTok.Text,
					Value: val,
					Pos:   Pos{nameTok.Line, nameTok.Col},
				})
				if p.cur.Type != TokenComma {
					break
				}
				if err := p.next(); err != nil { // consume ','
					return nil, err
				}
				continue
			}
			// Not a named arg — restore and parse as expression
			p.lex.restore(snap)
			p.cur = savedCur
		}
		// Bare argument (internal builtins)
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.cur.Type != TokenComma {
			break
		}
		if err := p.next(); err != nil { // consume ','
			return nil, err
		}
	}
	return args, nil
}
