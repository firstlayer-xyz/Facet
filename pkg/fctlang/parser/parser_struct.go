package parser

// isEmptyBrace peeks ahead (without consuming) to check if the current
// position is an empty brace pair: { }
func (p *parser) isEmptyBrace() bool {
	snap := p.snapshot()
	defer p.restore(snap)
	// Consume {
	if err := p.next(); err != nil {
		return false
	}
	return p.cur.Type == TokenRBrace
}

// isStructLitStart peeks ahead (without consuming) to check if the current
// position begins a struct literal: { IDENT : ... }
func (p *parser) isStructLitStart() bool {
	snap := p.snapshot()
	defer p.restore(snap)
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
	prevFieldLine := 0
	attachTrailing := func() {
		if prevFieldLine == 0 || len(fields) == 0 {
			return
		}
		last := fields[len(fields)-1]
		last.Comments = append(last.Comments, p.drainTrailingComments(prevFieldLine)...)
	}
	if p.cur.Type != TokenRBrace {
		for {
			// Attach the previous field's trailing end-of-line comment (scanned
			// past when peeking the next token after its comma), then the standalone
			// comment lines that lead this field. Mirrors parseCallArgs so interior
			// comments stay with their field instead of leaking to the next
			// statement. drainCommentsOnLine removes the trailing ones, so the
			// following drainComments (leading) can't double-count them.
			attachTrailing()
			leading := p.lex.drainComments()

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
				Name:     fieldName.Text,
				Value:    val,
				Pos:      Pos{fieldName.Line, fieldName.Col},
				Comments: leading,
			})
			prevFieldLine = fieldName.Line
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
	// The last field's trailing comment is scanned when peeking '}'.
	attachTrailing()
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
	// Preserve interior comments, as parseArrayLit does — the internal comments of
	// each struct element are already kept by parseStructLit; this covers comments
	// BETWEEN elements.
	ec := newElemComments()
	prevElemLine := 0
	if p.cur.Type != TokenRBracket {
		for {
			if prevElemLine > 0 {
				ec.attach(len(elems)-1, p.lex.drainCommentsOnLine(prevElemLine), true) // prev elem trailing
			}
			leading := p.lex.drainComments()
			if p.cur.Type == TokenRBracket {
				ec.attach(len(elems)-1, leading, true) // dangling before ']' after trailing comma
				break
			}
			elemLine := p.cur.Line
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
			ec.attach(len(elems)-1, leading, false) // this elem's leading
			prevElemLine = elemLine
			if p.cur.Type != TokenComma {
				break
			}
			if err := p.next(); err != nil { // consume ','
				return nil, err
			}
		}
	}
	if prevElemLine > 0 {
		ec.attach(len(elems)-1, p.lex.drainCommentsOnLine(prevElemLine), true) // last elem trailing
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ArrayLitExpr{TypeName: typeName, Elems: elems, Pos: Pos{line, col}, ElemComments: ec.list(len(elems))}, nil
}
