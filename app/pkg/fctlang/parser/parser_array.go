package parser

// parseArrayLit → "[" [ expr { "," expr } ] "]"
//               | "[" expr ":" ["<"|">"|"<="|">="] expr [ ":" expr ] "]"
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
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// Check for ":" → range syntax [start:end] or [start:end:step]
	if p.cur.Type == TokenColon {
		if err := p.next(); err != nil { // consume ':'
			return nil, err
		}
		// Check for bound modifier: <, >, <=, >=
		exclusive := false
		if p.cur.Type == TokenLess || p.cur.Type == TokenGreater {
			exclusive = true
			if err := p.next(); err != nil { // consume '<' or '>'
				return nil, err
			}
		} else if p.cur.Type == TokenLessEq || p.cur.Type == TokenGreaterEq {
			exclusive = false // explicit inclusive
			if err := p.next(); err != nil { // consume '<=' or '>='
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
		return &RangeExpr{Start: first, End: end, Step: step, Exclusive: exclusive, Pos: Pos{bracketLine, bracketCol}}, nil
	}
	// Normal array literal
	elems := []Expr{first}
	for p.cur.Type == TokenComma {
		if err := p.next(); err != nil { // consume ','
			return nil, err
		}
		if p.cur.Type == TokenRBracket {
			break // trailing comma
		}
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ArrayLitExpr{Elems: elems, Pos: Pos{bracketLine, bracketCol}}, nil
}

// parseConstraint parses the optional constraint after a var value expression.
// Returns *RangeExpr, *ConstrainedRange, or *ArrayLitExpr.
func (p *parser) parseConstraint() (Expr, error) {
	constraint, err := p.parseArrayLit() // returns *RangeExpr or *ArrayLitExpr
	if err != nil {
		return nil, err
	}
	// Check for unit suffix: [1:100] mm or [0:360] deg
	if rng, ok := constraint.(*RangeExpr); ok && p.cur.Type == TokenIdent {
		if _, isAngle := AngleFactors[p.cur.Text]; isAngle {
			unit := p.cur.Text
			if err := p.next(); err != nil {
				return nil, err
			}
			return &ConstrainedRange{Range: rng, Unit: unit}, nil
		}
		if _, isUnit := UnitFactors[p.cur.Text]; isUnit {
			unit := p.cur.Text
			if err := p.next(); err != nil {
				return nil, err
			}
			return &ConstrainedRange{Range: rng, Unit: unit}, nil
		}
	}
	return constraint, nil
}
