package parser

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
