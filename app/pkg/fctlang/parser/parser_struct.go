package parser

// isEmptyBrace peeks ahead (without consuming) to check if the current
// position is an empty brace pair: { }
func (p *parser) isEmptyBrace() bool {
	snap := p.lex.snapshot()
	savedCur := p.cur
	defer func() {
		p.lex.restore(snap)
		p.cur = savedCur
	}()
	// Consume {
	if err := p.next(); err != nil {
		return false
	}
	return p.cur.Type == TokenRBrace
}

// isStructLitStart peeks ahead (without consuming) to check if the current
// position begins a struct literal: { IDENT : ... }
func (p *parser) isStructLitStart() bool {
	snap := p.lex.snapshot()
	savedCur := p.cur
	defer func() {
		p.lex.restore(snap)
		p.cur = savedCur
	}()
	// Consume {
	if err := p.next(); err != nil {
		return false
	}
	if p.cur.Type != TokenIdent {
		return false
	}
	// Consume IDENT
	if err := p.next(); err != nil {
		return false
	}
	return p.cur.Type == TokenColon
}

// parseStructLit → "{" IDENT ":" expr { "," IDENT ":" expr } "}"
// All fields must use named syntax (field: value).
func (p *parser) parseStructLit(typeName string, line, col int) (Expr, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	var fields []*StructFieldInit
	if p.cur.Type != TokenRBrace {
		for {
			fieldName, err := p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenColon); err != nil {
				return nil, err
			}
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			fields = append(fields, &StructFieldInit{Name: fieldName.Text, Value: val})
			// Consume an optional semicolon (ASI may have inserted one on a
			// newline before ',' or '}' in a multi-line struct literal).
			if p.cur.Type == TokenSemicolon {
				if err := p.next(); err != nil {
					return nil, err
				}
			}
			if p.cur.Type != TokenComma {
				break
			}
			if err := p.next(); err != nil { // consume ','
				return nil, err
			}
		}
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &StructLitExpr{TypeName: typeName, Fields: fields, Pos: Pos{line, col}}, nil
}

// isTypedArrayStart peeks ahead to check if the current position begins a
// typed array constructor: Name[ expr, expr, ... ].
// Scans tokens after [, tracking ()[]{}  depth. At depth 0 a comma → true
// (typed array); hitting ] without a comma → false (regular indexing).
func (p *parser) isTypedArrayStart() bool {
	snap := p.lex.snapshot()
	savedCur := p.cur
	defer func() {
		p.lex.restore(snap)
		p.cur = savedCur
	}()
	// Consume [
	if err := p.next(); err != nil {
		return false
	}
	// Empty typed array: Name[]
	if p.cur.Type == TokenRBracket {
		return true
	}
	depth := 0
	for {
		switch p.cur.Type {
		case TokenLParen, TokenLBrace:
			depth++
		case TokenLBracket:
			depth++
		case TokenRParen, TokenRBrace:
			depth--
			if depth < 0 {
				return false
			}
		case TokenRBracket:
			if depth == 0 {
				return false // hit ] without seeing , → indexing
			}
			depth--
			if depth < 0 {
				return false
			}
		case TokenComma:
			if depth == 0 {
				return true
			}
		case TokenEOF:
			return false
		}
		if err := p.next(); err != nil {
			return false
		}
	}
}

// parseTypedArrayLit → "[" elem { "," elem } "]"
// Parses elements for a typed array constructor (TypeName already consumed).
// Bare { ... } elements are parsed as named struct literals of the given type.
func (p *parser) parseTypedArrayLit(typeName string, line, col int) (Expr, error) {
	if _, err := p.expect(TokenLBracket); err != nil {
		return nil, err
	}
	var elems []Expr
	if p.cur.Type != TokenRBracket {
		for {
			var elem Expr
			var err error
			if p.cur.Type == TokenLBrace {
				// Bare { ... } → struct literal of the array's element type
				eLine, eCol := p.cur.Line, p.cur.Col
				if p.isStructLitStart() || p.isEmptyBrace() {
					elem, err = p.parseStructLit(typeName, eLine, eCol)
				} else {
					// Fallback: parse as normal expression (block)
					elem, err = p.parseExpr()
				}
			} else {
				elem, err = p.parseExpr()
			}
			if err != nil {
				return nil, err
			}
			elems = append(elems, elem)
			if p.cur.Type != TokenComma {
				break
			}
			if err := p.next(); err != nil { // consume ','
				return nil, err
			}
			if p.cur.Type == TokenRBracket {
				break // trailing comma
			}
		}
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ArrayLitExpr{TypeName: typeName, Elems: elems, Pos: Pos{line, col}}, nil
}

