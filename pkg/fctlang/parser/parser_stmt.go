package parser

// parseStatement → "return" expr [";"] | "var" ident "=" expr [";"] | "assert" expr [ "," string ] [";"]
func (p *parser) parseStatement(comments []Comment) (Stmt, error) {
	if p.cur.Type == TokenReturn {
		returnLine, returnCol := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.consumeOptionalSemi(); err != nil {
			return nil, err
		}
		return &ReturnStmt{Value: expr, Pos: Pos{returnLine, returnCol}, Comments: comments}, nil
	}

	if p.cur.Type == TokenVar || p.cur.Type == TokenConst {
		isConst := p.cur.Type == TokenConst
		if err := p.next(); err != nil {
			return nil, err
		}
		name, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if err := p.rejectUnderscoreIdent(name, "variable"); err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenEquals); err != nil {
			return nil, err
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		constraint, err := p.parseWhereConstraint()
		if err != nil {
			return nil, err
		}
		if err := p.consumeOptionalSemi(); err != nil {
			return nil, err
		}
		return &VarStmt{Name: name.Text, Value: expr, IsConst: isConst, Constraint: constraint, Pos: Pos{name.Line, name.Col}, Comments: comments}, nil
	}

	if p.cur.Type == TokenAssert {
		stmt, err := p.parseAssertStmt()
		if err != nil {
			return nil, err
		}
		stmt.(*AssertStmt).Comments = comments
		return stmt, nil
	}

	return nil, p.errorf("expected statement, got %q", p.cur.Text)
}

// parseAssertStmt → "assert" expr [ "," expr ] [";"]
//
//	| "assert" expr "where" constraint [";"]
func (p *parser) parseAssertStmt() (Stmt, error) {
	line, col := p.cur.Line, p.cur.Col
	if err := p.next(); err != nil { // consume 'assert'
		return nil, err
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// "assert EXPR where CONSTRAINT" form
	if p.cur.Type == TokenWhere {
		constraint, err := p.parseWhereConstraint()
		if err != nil {
			return nil, err
		}
		if err := p.consumeOptionalSemi(); err != nil {
			return nil, err
		}
		return &AssertStmt{Value: expr, Constraint: constraint, Pos: Pos{line, col}}, nil
	}

	// "assert COND [, MSG]" form
	var msg Expr
	if p.cur.Type == TokenComma {
		if err := p.next(); err != nil { // consume ','
			return nil, err
		}
		msg, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	if err := p.consumeOptionalSemi(); err != nil {
		return nil, err
	}
	return &AssertStmt{Cond: expr, Message: msg, Pos: Pos{line, col}}, nil
}
