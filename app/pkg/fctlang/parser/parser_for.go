package parser

// compoundOp maps compound assignment tokens (+=, -=, etc.) to their
// corresponding binary operator string. Returns ("", false) for non-compound tokens.
func compoundOp(t TokenType) (string, bool) {
	switch t {
	case TokenPlusEq:
		return "+", true
	case TokenMinusEq:
		return "-", true
	case TokenStarEq:
		return "*", true
	case TokenSlashEq:
		return "/", true
	case TokenModEq:
		return "%", true
	case TokenCaretEq:
		return "^", true
	case TokenAmpEq:
		return "&", true
	case TokenPipeEq:
		return "|", true
	default:
		return "", false
	}
}

// parseForExpr → "for" forClause { "," forClause } "{" body "}"
// forClause    → IDENT "," IDENT expr   (enumerate: index, value)
//              | IDENT expr              (regular)
func (p *parser) parseForExpr() (Expr, error) {
	forLine, forCol := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenFor); err != nil {
		return nil, err
	}

	// Parse first clause
	clause, err := p.parseForClause()
	if err != nil {
		return nil, err
	}
	clauses := []*ForClause{clause}

	// Parse additional comma-separated clauses
	for p.cur.Type == TokenComma {
		if err := p.next(); err != nil {
			return nil, err
		}
		c, err := p.parseForClause()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, c)
	}

	// ForYieldExpr
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	p.inForYield++
	body, err := p.parseBodyStmts(true, "for-yield body")
	p.inForYield--
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &ForYieldExpr{Clauses: clauses, Body: body, Pos: Pos{forLine, forCol}}, nil
}

// parseForClause parses a single for clause.
// Enumerate: IDENT "," IDENT expr   → ForClause{Index: i, Var: v, Iter: expr}
// Regular:   IDENT expr             → ForClause{Var: v, Iter: expr}
func (p *parser) parseForClause() (*ForClause, error) {
	clauseLine, clauseCol := p.cur.Line, p.cur.Col
	varName, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if err := p.rejectUnderscoreIdent(varName, "loop variable"); err != nil {
		return nil, err
	}

	// If next token is comma, this is enumerate syntax: index, value expr
	if p.cur.Type == TokenComma {
		if err := p.next(); err != nil {
			return nil, err
		}
		elemVar, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if err := p.rejectUnderscoreIdent(elemVar, "loop variable"); err != nil {
			return nil, err
		}
		iter, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ForClause{Index: varName.Text, Var: elemVar.Text, Iter: iter, Pos: Pos{clauseLine, clauseCol}}, nil
	}

	// Regular clause: value expr
	iter, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ForClause{Var: varName.Text, Iter: iter, Pos: Pos{clauseLine, clauseCol}}, nil
}

// parseFoldExpr → "fold" IDENT "," IDENT expr "{" body "}"
func (p *parser) parseFoldExpr() (Expr, error) {
	foldLine, foldCol := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenFold); err != nil {
		return nil, err
	}
	accVar, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if err := p.rejectUnderscoreIdent(accVar, "fold accumulator variable"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenComma); err != nil {
		return nil, err
	}
	elemVar, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if err := p.rejectUnderscoreIdent(elemVar, "fold element variable"); err != nil {
		return nil, err
	}
	iter, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	p.inForYield++
	body, err := p.parseBodyStmts(false, "fold body")
	p.inForYield--
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &FoldExpr{AccVar: accVar.Text, ElemVar: elemVar.Text, Iter: iter, Body: body, Pos: Pos{foldLine, foldCol}}, nil
}

// parseForStatement handles statements inside for-yield bodies: var, yield, assert
func (p *parser) parseForStatement(comments []Comment) (Stmt, error) {
	if p.cur.Type == TokenYield {
		yieldLine, yieldCol := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		// Bare yield (no value) — skip this iteration. Accept ';' (explicit or
		// ASI-inserted after a newline) or '}' (same-line "yield }").
		if p.cur.Type == TokenSemicolon || p.cur.Type == TokenRBrace {
			if p.cur.Type == TokenSemicolon {
				if err := p.next(); err != nil {
					return nil, err
				}
			}
			return &YieldStmt{Value: nil, Pos: Pos{yieldLine, yieldCol}, Comments: comments}, nil
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.consumeOptionalSemi(); err != nil {
			return nil, err
		}
		return &YieldStmt{Value: expr, Pos: Pos{yieldLine, yieldCol}, Comments: comments}, nil
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

	return nil, p.errorf("expected 'yield' or 'var' in for-yield body, got %q", p.cur.Text)
}

// parseBodyStmts parses statements inside a block body. Bare expressions
// are wrapped as ExprStmt. Explicit return/yield statements are required.
func (p *parser) parseBodyStmts(isForYield bool, context string) ([]Stmt, error) {
	var stmts []Stmt
	for p.cur.Type != TokenRBrace {
		if p.cur.Type == TokenEOF {
			return nil, p.errorf("unexpected end of input in %s", context)
		}

		// Drain comments before each statement
		comments := p.lex.drainComments()

		// Check if this token starts a statement (keyword-driven)
		isStmt := p.cur.Type == TokenVar || p.cur.Type == TokenConst || p.cur.Type == TokenAssert
		// yield is valid in the for-yield body itself, or in nested blocks (if/else)
		// when we're inside a for-yield context
		if isForYield || p.inForYield > 0 {
			isStmt = isStmt || p.cur.Type == TokenYield
		}
		if !isForYield {
			isStmt = isStmt || p.cur.Type == TokenReturn
		} else if p.cur.Type == TokenReturn {
			return nil, p.errorf("return is not allowed inside for-yield; use yield instead")
		}

		if isStmt {
			stmtLine := p.cur.Line
			var stmt Stmt
			var err error
			if isForYield || (p.inForYield > 0 && p.cur.Type == TokenYield) {
				stmt, err = p.parseForStatement(comments)
			} else {
				stmt, err = p.parseStatement(comments)
			}
			if err != nil {
				return nil, err
			}
			p.drainTrailingComment(stmt, stmtLine)
			stmts = append(stmts, stmt)
			continue
		}

		// If statement — parsed at the statement level, not as an expression
		if p.cur.Type == TokenIf {
			ifStmt, err := p.parseIfStmt()
			if err != nil {
				return nil, err
			}
			ifStmt.Comments = comments
			stmts = append(stmts, ifStmt)
			// Consume optional semicolon (ASI may have inserted one after '}')
			if p.cur.Type == TokenSemicolon {
				if err := p.next(); err != nil {
					return nil, err
				}
			}
			continue
		}

		// Not a statement keyword — try as expression
		exprLine, exprCol := p.cur.Line, p.cur.Col
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		// ident = expr ; → assignment
		// ident += expr ; → compound assignment (desugars to ident = ident + expr)
		if ident, ok := expr.(*IdentExpr); ok {
			op, isCompound := compoundOp(p.cur.Type)
			if p.cur.Type == TokenEquals || isCompound {
				if err := p.rejectUnderscoreIdent(Token{Text: ident.Name, Line: ident.Pos.Line, Col: ident.Pos.Col}, "assignment"); err != nil {
					return nil, err
				}
				opLine, opCol := p.cur.Line, p.cur.Col
				if err := p.next(); err != nil { // consume '=' or 'op='
					return nil, err
				}
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if isCompound {
					val = &BinaryExpr{Op: op, Left: ident, Right: val, Pos: Pos{opLine, opCol}}
				}
				if err := p.consumeOptionalSemi(); err != nil {
					return nil, err
				}
				as := &AssignStmt{Name: ident.Name, Value: val, Pos: Pos{ident.Pos.Line, ident.Pos.Col}, Comments: comments}
				p.drainTrailingComment(as, ident.Pos.Line)
				stmts = append(stmts, as)
				continue
			}
		}

		// field.access = expr ; → field assignment
		// field.access += expr ; → compound field assignment
		if fa, ok := expr.(*FieldAccessExpr); ok {
			op, isCompound := compoundOp(p.cur.Type)
			if p.cur.Type == TokenEquals || isCompound {
				if err := p.rejectUnderscoreIdent(Token{Text: fa.Field, Line: fa.Pos.Line, Col: fa.Pos.Col}, "field assignment"); err != nil {
					return nil, err
				}
				opLine, opCol := p.cur.Line, p.cur.Col
				if err := p.next(); err != nil { // consume '=' or 'op='
					return nil, err
				}
				val, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if isCompound {
					val = &BinaryExpr{Op: op, Left: fa, Right: val, Pos: Pos{opLine, opCol}}
				}
				if err := p.consumeOptionalSemi(); err != nil {
					return nil, err
				}
				fas := &FieldAssignStmt{Receiver: fa.Receiver, Field: fa.Field, Value: val, Pos: fa.Pos, Comments: comments}
				p.drainTrailingComment(fas, fa.Pos.Line)
				stmts = append(stmts, fas)
				continue
			}
		}

		if p.cur.Type == TokenRBrace {
			stmts = append(stmts, &ExprStmt{Expr: expr, Pos: Pos{exprLine, exprCol}, Comments: comments})
			break
		}

		// Block-like expressions (if, for, fold, {}) that end with '}' can appear
		// as statements without a trailing semicolon — their result is discarded.
		// Consume an optional semicolon (explicit or ASI-inserted) and continue.
		switch expr.(type) {
		case *ForYieldExpr, *FoldExpr, *StructLitExpr:
			if p.cur.Type == TokenSemicolon {
				if err := p.next(); err != nil {
					return nil, err
				}
			}
			stmts = append(stmts, &ExprStmt{Expr: expr, Pos: Pos{exprLine, exprCol}, Comments: comments})
			continue
		}

		if p.cur.Type == TokenSemicolon {
			if err := p.next(); err != nil {
				return nil, err
			}
			stmts = append(stmts, &ExprStmt{Expr: expr, Pos: Pos{exprLine, exprCol}, Comments: comments})
			if p.cur.Type == TokenRBrace {
				break
			}
			continue
		}

		return nil, p.errorf("expected '}' or ';' after expression, got %q", p.cur.Text)
	}
	return stmts, nil
}
