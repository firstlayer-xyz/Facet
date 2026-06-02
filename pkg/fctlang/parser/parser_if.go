package parser

// parseIfExpr → "if" expr "{" expr "}" { "else" "if" expr "{" expr "}" } "else" "{" expr "}"
//
// Each arm is a single expression rather than a body of statements (that
// shape is parseIfStmt). The else arm is required — without it the
// expression has no value when the cond is false.
//
// The trailing else distinguishes this from the statement form at parse
// time: in expression position we always consume the chain greedily and
// require the final else.
func (p *parser) parseIfExpr() (*IfExpr, error) {
	ifLine, ifCol := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenIf); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	thenExpr, err := p.parseBracedExpr("if expression body")
	if err != nil {
		return nil, err
	}

	ifExpr := &IfExpr{Cond: cond, Then: thenExpr, Pos: Pos{ifLine, ifCol}}

	for p.cur.Type == TokenElse {
		if err := p.next(); err != nil { // consume 'else'
			return nil, err
		}
		if p.cur.Type == TokenIf {
			eifLine, eifCol := p.cur.Line, p.cur.Col
			if err := p.next(); err != nil { // consume 'if'
				return nil, err
			}
			eifCond, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			eifBody, err := p.parseBracedExpr("else-if expression body")
			if err != nil {
				return nil, err
			}
			ifExpr.ElseIfs = append(ifExpr.ElseIfs, &ElseIfExprClause{Cond: eifCond, Body: eifBody, Pos: Pos{eifLine, eifCol}})
		} else {
			elseExpr, err := p.parseBracedExpr("else expression body")
			if err != nil {
				return nil, err
			}
			ifExpr.Else = elseExpr
			break
		}
	}

	if ifExpr.Else == nil {
		return nil, p.errorf("if expression requires an else arm — every code path must produce a value")
	}

	return ifExpr, nil
}

// parseBracedExpr parses "{" expr "}" — a single expression wrapped in
// braces, used for each arm of an if-expression. The description appears
// in error messages.
func (p *parser) parseBracedExpr(desc string) (Expr, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// Allow an optional trailing semicolon — ASI may insert one before '}'.
	if p.cur.Type == TokenSemicolon {
		if err := p.next(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, p.errorf("%s must contain a single expression", desc)
	}
	return expr, nil
}

// parseIfStmt → "if" expr "{" body "}" { "else" "if" expr "{" body "}" } [ "else" "{" body "}" ]
func (p *parser) parseIfStmt() (*IfStmt, error) {
	ifLine, ifCol := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenIf); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	thenBody, err := p.parseBodyStmts(false, "if body")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	ifStmt := &IfStmt{Cond: cond, Then: thenBody, Pos: Pos{ifLine, ifCol}}

	// Parse else-if and else clauses
	for p.cur.Type == TokenElse {
		if err := p.next(); err != nil { // consume 'else'
			return nil, err
		}
		if p.cur.Type == TokenIf {
			// else if
			eifLine, eifCol := p.cur.Line, p.cur.Col
			if err := p.next(); err != nil { // consume 'if'
				return nil, err
			}
			eifCond, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenLBrace); err != nil {
				return nil, err
			}
			eifBody, err := p.parseBodyStmts(false, "else-if body")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenRBrace); err != nil {
				return nil, err
			}
			ifStmt.ElseIfs = append(ifStmt.ElseIfs, &ElseIfClause{Cond: eifCond, Body: eifBody, Pos: Pos{eifLine, eifCol}})
		} else {
			// else
			if _, err := p.expect(TokenLBrace); err != nil {
				return nil, err
			}
			elseBody, err := p.parseBodyStmts(false, "else body")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenRBrace); err != nil {
				return nil, err
			}
			ifStmt.Else = elseBody
			break // else is always last
		}
	}

	return ifStmt, nil
}
