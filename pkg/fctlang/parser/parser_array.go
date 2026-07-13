package parser

// parseArrayLit → "[" [ expr { "," expr } ] "]"
//
//	| "[" expr ":" ["<"|">"|"<="|">="] expr [ ":" expr ] "]"
func (p *parser) parseArrayLit() (Expr, error) {
	bracketLine, bracketCol := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenLBracket); err != nil {
		return nil, err
	}
	// []Name[...] — typed array literal, or [] — empty untyped array
	if p.cur.Type == TokenRBracket {
		if _, err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
		// Check for typed array: []Name[...]
		if p.cur.Type == TokenIdent {
			typeName := p.cur.Text
			tLine, tCol := bracketLine, bracketCol
			if err := p.next(); err != nil { // consume type name
				return nil, err
			}
			if p.cur.Type == TokenLBracket {
				return p.parseTypedArrayLit(typeName, tLine, tCol)
			}
			// Not followed by [ — syntax error
			return nil, p.errorf("expected '[' after type name in typed array literal")
		}
		// Bare [] — empty untyped array
		return &ArrayLitExpr{Elems: nil, Pos: Pos{bracketLine, bracketCol}}, nil
	}
	// Parse first element
	firstElemLine := p.cur.Line
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// Check for ":" → range syntax [start:end] or [start:end:step]
	if p.cur.Type == TokenColon {
		if err := p.next(); err != nil { // consume ':'
			return nil, err
		}
		// Check for bound modifier: <, >, <=, >=. Record the literal spelling; it
		// is the single source of truth — the formatter reproduces it verbatim
		// and RangeExpr.IsExclusive derives exclusivity from it.
		bound := ""
		if p.cur.Type == TokenLess || p.cur.Type == TokenGreater ||
			p.cur.Type == TokenLessEq || p.cur.Type == TokenGreaterEq {
			bound = p.cur.Text
			if err := p.next(); err != nil { // consume the modifier
				return nil, err
			}
		}
		// Parse end expression
		end, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		// Check for step: ":" expr
		var step Expr
		if p.cur.Type == TokenColon {
			if err := p.next(); err != nil { // consume ':'
				return nil, err
			}
			step, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		if _, err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
		return &RangeExpr{Start: first, End: end, Step: step, Bound: bound, Pos: Pos{bracketLine, bracketCol}}, nil
	}
	// Normal array literal. Preserve interior comments: after each comma, the
	// previous element's trailing end-of-line comment and the standalone comment
	// lines before the next element are attached to those elements (via a parallel
	// side-list) so the formatter keeps them in place instead of leaking them to
	// the next statement. Mirrors parseCallArgs / parseStructLit.
	elems := []Expr{first}
	prevElemLine := firstElemLine
	ec := newElemComments()
	for p.cur.Type == TokenComma {
		if err := p.next(); err != nil { // consume ','
			return nil, err
		}
		ec.attach(len(elems)-1, p.lex.drainCommentsOnLine(prevElemLine), true) // prev elem trailing
		leading := p.lex.drainComments()
		if p.cur.Type == TokenRBracket {
			// Trailing comma with dangling comments before ']'. Keep them (as
			// trailing on the last element) rather than leaking them forward.
			ec.attach(len(elems)-1, leading, true)
			break
		}
		elemLine := p.cur.Line
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
		ec.attach(len(elems)-1, leading, false) // this elem's leading
		prevElemLine = elemLine
	}
	ec.attach(len(elems)-1, p.lex.drainCommentsOnLine(prevElemLine), true) // last elem trailing (before ']')
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ArrayLitExpr{Elems: elems, Pos: Pos{bracketLine, bracketCol}, ElemComments: ec.list(len(elems))}, nil
}

// parseConstraint parses the optional constraint after a var value expression.
// Returns *RangeExpr, *ConstrainedRange, or *ArrayLitExpr.
func (p *parser) parseConstraint() (Expr, error) {
	constraint, err := p.parseArrayLit() // returns *RangeExpr or *ArrayLitExpr
	if err != nil {
		return nil, err
	}
	// Check for unit suffix: [1:100] mm or [0:360] deg
	if rng, ok := constraint.(*RangeExpr); ok && p.isUnitSuffix() {
		unit := p.cur.Text
		if err := p.next(); err != nil {
			return nil, err
		}
		return &ConstrainedRange{Range: rng, Unit: unit}, nil
	}
	return constraint, nil
}
