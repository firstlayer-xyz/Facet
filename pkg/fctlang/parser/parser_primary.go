package parser

import "strings"

// parsePrimary → "-" postfix | "!" postfix | number [unit] | ident [ "(" [args] ")" ] | "(" expr ")" | block
func (p *parser) parsePrimary() (Expr, error) {
	// Unary minus: -expr (binds looser than postfix so -self.field == -(self.field))
	// Boolean NOT: !expr
	if p.cur.Type == TokenMinus || p.cur.Type == TokenBang {
		// Guard the unary recursion: parsePrimary → parsePostfix → parsePrimary
		// bypasses parseExpr's depth check, so a long `---!--…` chain would
		// otherwise overflow the (unrecoverable) goroutine stack.
		p.depth++
		if p.depth > maxParseDepth {
			return nil, p.errorf("expression too deeply nested (limit %d)", maxParseDepth)
		}
		defer func() { p.depth-- }()
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
				namePos := Pos{nameLine, nameCol}
				return p.parseStructLit(name, namePos, namePos)
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
	// No type name in source; both positions are the `{`.
	if p.cur.Type == TokenLBrace {
		if p.isStructLitStart() {
			bracePos := Pos{p.cur.Line, p.cur.Col}
			return p.parseStructLit("", bracePos, bracePos)
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

	// Optional None literal: nil
	if p.cur.Type == TokenNil {
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		return &NilLit{Pos: Pos{line, col}}, nil
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
//
// Comments inside the argument list are preserved:
//   - Trailing end-of-line comments (on the same line as the arg) are attached
//     to that arg as IsTrailing comments.
//   - Standalone comment lines before a named arg are attached to it as leading
//     (non-trailing) comments.
func (p *parser) parseCallArgs() ([]Expr, error) {
	var args []Expr
	if p.cur.Type == TokenRParen {
		return args, nil
	}
	prevArgLine := 0
	for {
		// Drain the trailing end-of-line comment from the previous arg. After
		// consuming the comma, skipWhitespaceAndComments has already scanned past
		// the comment into pendingComments. We retroactively attach it.
		if prevArgLine > 0 && len(args) > 0 {
			trailing := p.lex.drainCommentsOnLine(prevArgLine)
			for i := range trailing {
				trailing[i].IsTrailing = true
			}
			if na, ok := args[len(args)-1].(*NamedArg); ok {
				na.Comments = append(na.Comments, trailing...)
			}
		}

		// Try to parse named argument: IDENT ":"
		if p.cur.Type == TokenIdent {
			// Snapshot before lookahead so leading comments remain in pendingComments
			// and are available after we confirm this is a named arg.
			snap := p.lex.snapshot()
			savedCur := p.cur
			nameTok := p.cur
			if err := p.next(); err != nil {
				return nil, err
			}
			if p.cur.Type == TokenColon {
				// Drain leading comments (standalone lines before this arg) now that
				// we know it is a named arg and won't restore the snapshot.
				leadingComments := p.lex.drainComments()
				if err := p.next(); err != nil { // consume ':'
					return nil, err
				}
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, &NamedArg{
					Name:     nameTok.Text,
					Value:    val,
					Pos:      Pos{nameTok.Line, nameTok.Col},
					Comments: leadingComments,
				})
				prevArgLine = nameTok.Line
				if p.cur.Type != TokenComma {
					break
				}
				if err := p.next(); err != nil { // consume ','
					return nil, err
				}
				continue
			}
			// Not a named arg — restore (leading comments stay in pendingComments).
			p.lex.restore(snap)
			p.cur = savedCur
		}
		// Bare argument (internal builtins). Leading comments are not attached
		// because bare Expr nodes have no Comments field.
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		prevArgLine = 0
		if p.cur.Type != TokenComma {
			break
		}
		if err := p.next(); err != nil { // consume ','
			return nil, err
		}
	}
	// Drain trailing comment for the last arg (collected when scanning to ')').
	if prevArgLine > 0 && len(args) > 0 {
		trailing := p.lex.drainCommentsOnLine(prevArgLine)
		for i := range trailing {
			trailing[i].IsTrailing = true
		}
		if na, ok := args[len(args)-1].(*NamedArg); ok {
			na.Comments = append(na.Comments, trailing...)
		}
	}
	return args, nil
}
