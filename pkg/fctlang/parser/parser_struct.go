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
//
// exprStart is the start of the whole literal expression (used as Pos for
// diagnostics). typeNamePos is the position of the type-name token itself;
// for unqualified `Thread{...}` it equals exprStart, but for qualified
// `T.Thread{...}` it points at `Thread`, not at `T`.
func (p *parser) parseStructLit(typeName string, exprStart, typeNamePos Pos) (Expr, error) {
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
			fields = append(fields, &StructFieldInit{
				Name:  fieldName.Text,
				Value: val,
				Pos:   Pos{fieldName.Line, fieldName.Col},
			})
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
			if p.cur.Type == TokenRBrace {
				break // trailing comma
			}
		}
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &StructLitExpr{TypeName: typeName, Fields: fields, Pos: exprStart, TypeNamePos: typeNamePos}, nil
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
				// Bare { ... } → struct literal of the array's element type.
				// No type-name token in source for this element; TypeNamePos
				// is the same as Pos (the `{`).
				bracePos := Pos{p.cur.Line, p.cur.Col}
				if p.isStructLitStart() || p.isEmptyBrace() {
					elem, err = p.parseStructLit(typeName, bracePos, bracePos)
				} else {
					// A `{` here is neither a `field: value` struct literal nor an
					// empty `{}`. parseExpr would only reach parsePrimary and reject
					// it with a context-free "unexpected '{'" — name the element type
					// instead of routing through a bogus expression parse.
					return nil, p.errorf("expected 'field: value' struct literal for []%s element", typeName)
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
