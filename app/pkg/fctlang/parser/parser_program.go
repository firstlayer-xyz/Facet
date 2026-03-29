package parser

// drainTrailingComment drains any pending comment on the given line and
// appends it to the statement's Comments slice. This handles end-of-line
// comments like: var x = 10 mm // explanation
// drainTrailingComments drains pending comments on the given line and marks them as trailing.
func (p *parser) drainTrailingComments(line int) []Comment {
	trailing := p.lex.drainCommentsOnLine(line)
	for i := range trailing {
		trailing[i].IsTrailing = true
	}
	return trailing
}

func (p *parser) drainTrailingComment(s Stmt, line int) {
	trailing := p.drainTrailingComments(line)
	if len(trailing) == 0 {
		return
	}
	switch s := s.(type) {
	case *VarStmt:
		s.Comments = append(s.Comments, trailing...)
	case *ReturnStmt:
		s.Comments = append(s.Comments, trailing...)
	case *YieldStmt:
		s.Comments = append(s.Comments, trailing...)
	case *AssignStmt:
		s.Comments = append(s.Comments, trailing...)
	case *FieldAssignStmt:
		s.Comments = append(s.Comments, trailing...)
	case *ExprStmt:
		s.Comments = append(s.Comments, trailing...)
	case *AssertStmt:
		s.Comments = append(s.Comments, trailing...)
	case *IfStmt:
		s.Comments = append(s.Comments, trailing...)
	}
}

// parseProgram → { var_decl | function }
func (p *parser) parseProgram() (*Source, error) {
	prog := &Source{}
	for p.cur.Type != TokenEOF {
		// Drain comments before each top-level declaration
		comments := p.lex.drainComments()

		// Global variable/constant: var/const name = expr;
		if p.cur.Type == TokenVar || p.cur.Type == TokenConst {
			isConst := p.cur.Type == TokenConst
			if err := p.next(); err != nil {
				return nil, err
			}
			name, err := p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			if err := p.rejectUnderscoreIdent(name, "global variable"); err != nil {
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
			v := &VarStmt{Name: name.Text, Value: expr, IsConst: isConst, Constraint: constraint, Pos: Pos{name.Line, name.Col}, Comments: comments}
			v.Comments = append(v.Comments, p.drainTrailingComments(name.Line)...)
			prog.Declarations = append(prog.Declarations, v)
			continue
		}

		// Struct declaration: type Name { ... }
		if p.cur.Type == TokenTypeKw {
			sd, err := p.parseStructDecl()
			if err != nil {
				return nil, err
			}
			sd.Comments = append(comments, sd.Comments...)
			prog.Declarations = append(prog.Declarations, sd)
			continue
		}

		if p.cur.Type != TokenFn {
			return nil, p.errorf("expected fn, var, const, or type at top level, got %q", p.cur.Text)
		}

		fn, err := p.parseFunction()
		if err != nil {
			return nil, err
		}
		fn.Comments = append(comments, fn.Comments...)
		prog.Declarations = append(prog.Declarations, fn)
	}
	// Any remaining comments after all declarations
	prog.TrailingComments = p.lex.drainComments()
	return prog, nil
}

// parseStructDecl → "type" IDENT "{" { Name Type ";" } "}"
func (p *parser) parseStructDecl() (*StructDecl, error) {
	line, col := p.cur.Line, p.cur.Col
	if _, err := p.expect(TokenTypeKw); err != nil {
		return nil, err
	}
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if err := p.rejectUnderscoreIdent(name, "type"); err != nil {
		return nil, err
	}
	lbrace, err := p.expect(TokenLBrace)
	if err != nil {
		return nil, err
	}
	trailingComments := p.drainTrailingComments(lbrace.Line)
	var fields []*StructField
	for p.cur.Type != TokenRBrace {
		if p.cur.Type == TokenEOF {
			return nil, p.errorf("unexpected end of input in struct %q", name.Text)
		}
		// Drain comments before each field
		fieldComments := p.lex.drainComments()
		// Field syntax: name type [= default]
		fieldName, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if p.cur.Type == TokenVar {
			return nil, p.errorf("type fields cannot use var type")
		}
		typeStr, err := p.parseTypeStr()
		if err != nil {
			return nil, err
		}
		var fieldDefault Expr
		if p.cur.Type == TokenEquals {
			if err := p.next(); err != nil { // consume '='
				return nil, err
			}
			fieldDefault, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		var fieldConstraint Expr
		if p.cur.Type == TokenWhere {
			if err := p.next(); err != nil { // consume 'where'
				return nil, err
			}
			fieldConstraint, err = p.parseConstraint()
			if err != nil {
				return nil, err
			}
		}
		if err := p.consumeOptionalSemi(); err != nil {
			return nil, err
		}
		sf := &StructField{Type: typeStr, Name: fieldName.Text, Default: fieldDefault, Constraint: fieldConstraint, Comments: fieldComments}
		sf.Comments = append(sf.Comments, p.drainTrailingComments(fieldName.Line)...)
		fields = append(fields, sf)
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &StructDecl{Name: name.Text, Fields: fields, Pos: Pos{line, col}, Comments: trailingComments}, nil
}
