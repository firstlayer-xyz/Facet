package parser

import (
	"facet/pkg/scad/ast"
	"facet/pkg/scad/lexer"
	"facet/pkg/scad/token"
)

// Parse lexes and parses OpenSCAD source into a *ast.File.
func Parse(src string) (*ast.File, error) {
	p := &parser{toks: lexer.Lex(src)}
	return p.parseFile()
}

func (p *parser) parseFile() (*ast.File, error) {
	f := &ast.File{P: curPos(p.cur())}
	for !p.at(token.EOF) {
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s != nil {
			f.Stmts = append(f.Stmts, s)
		}
	}
	return f, nil
}

func (p *parser) parseStmt() (ast.Stmt, error) {
	switch p.cur().Kind {
	case token.Semi:
		p.advance()
		return nil, nil
	case token.Module:
		return p.parseModuleDef()
	case token.Function:
		return p.parseFunctionDef()
	case token.For:
		return p.parseFor()
	case token.If:
		return p.parseIf()
	case token.Use:
		t := p.advance()
		return &ast.Use{Path: t.Text, P: curPos(t)}, nil
	case token.Include:
		t := p.advance()
		return &ast.Include{Path: t.Text, P: curPos(t)}, nil
	case token.LBrace:
		children, err := p.parseChildBlock()
		if err != nil {
			return nil, err
		}
		return &ast.ModuleCall{Name: "union", Children: children, P: curPos(p.cur())}, nil
	case token.Hash, token.Bang, token.Percent, token.Star:
		mod := p.advance()
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if mc, ok := s.(*ast.ModuleCall); ok {
			mc.Modifier = modByte(mod.Kind)
		}
		return s, nil
	case token.Ident:
		// Lookahead for `name = ...` (assignment) vs a module call. Guarded
		// for consistency with parseArgs; the lexer's trailing EOF token makes
		// p.pos+1 in range here in practice, but spell out the invariant.
		if p.pos+1 < len(p.toks) && p.toks[p.pos+1].Kind == token.Assign {
			return p.parseAssign()
		}
		return p.parseModuleCall()
	}
	return nil, p.errf("unexpected token %q at statement start", p.cur().Text)
}

func modByte(k token.Kind) byte {
	switch k {
	case token.Hash:
		return '#'
	case token.Bang:
		return '!'
	case token.Percent:
		return '%'
	case token.Star:
		return '*'
	}
	return 0
}

func (p *parser) parseAssign() (ast.Stmt, error) {
	name := p.advance() // ident
	p.advance()         // =
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Semi); err != nil {
		return nil, err
	}
	return &ast.Assign{Name: name.Text, Value: val, P: curPos(name)}, nil
}

func (p *parser) parseModuleCall() (ast.Stmt, error) {
	name := p.advance()
	args, err := p.parseArgs()
	if err != nil {
		return nil, err
	}
	mc := &ast.ModuleCall{Name: name.Text, Args: args, P: curPos(name)}
	children, err := p.parseChildren()
	if err != nil {
		return nil, err
	}
	mc.Children = children
	return mc, nil
}

// parseChildren parses what follows a module call: `;`, a single child stmt,
// or a `{ ... }` block.
func (p *parser) parseChildren() ([]ast.Stmt, error) {
	switch p.cur().Kind {
	case token.Semi:
		p.advance()
		return nil, nil
	case token.LBrace:
		return p.parseChildBlock()
	default:
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s == nil {
			return nil, nil
		}
		return []ast.Stmt{s}, nil
	}
}

func (p *parser) parseChildBlock() ([]ast.Stmt, error) {
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	var out []ast.Stmt
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
	_, err := p.expect(token.RBrace)
	return out, err
}

func (p *parser) parseParams() ([]ast.Param, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var params []ast.Param
	for !p.at(token.RParen) {
		name, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		par := ast.Param{Name: name.Text}
		if p.at(token.Assign) {
			p.advance()
			def, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			par.Default = def
		}
		params = append(params, par)
		if p.at(token.Comma) {
			p.advance()
		} else if !p.at(token.RParen) {
			return nil, p.errf("expected ',' or ')' in parameter list, got %q", p.cur().Text)
		}
	}
	_, err := p.expect(token.RParen)
	return params, err
}

func (p *parser) parseModuleDef() (ast.Stmt, error) {
	kw := p.advance()
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	// OpenSCAD accepts either a braced block or a single statement as the
	// module body (`module curve() polygon(...);` is valid). parseChildren
	// already handles both forms for transforms; reuse it here.
	body, err := p.parseChildren()
	if err != nil {
		return nil, err
	}
	return &ast.ModuleDef{Name: name.Text, Params: params, Body: body, P: curPos(kw)}, nil
}

func (p *parser) parseFunctionDef() (ast.Stmt, error) {
	kw := p.advance()
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Semi); err != nil {
		return nil, err
	}
	return &ast.FunctionDef{Name: name.Text, Params: params, Body: body, P: curPos(kw)}, nil
}

func (p *parser) parseFor() (ast.Stmt, error) {
	kw := p.advance()
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var iters []ast.ForIter
	for {
		v, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Assign); err != nil {
			return nil, err
		}
		rng, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		iters = append(iters, ast.ForIter{Var: v.Text, Range: rng})
		if p.at(token.Comma) {
			p.advance()
			continue
		}
		break
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	children, err := p.parseChildren()
	if err != nil {
		return nil, err
	}
	return &ast.For{Iters: iters, Children: children, P: curPos(kw)}, nil
}

func (p *parser) parseIf() (ast.Stmt, error) {
	kw := p.advance()
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	then, err := p.parseChildren()
	if err != nil {
		return nil, err
	}
	iff := &ast.If{Cond: cond, Then: then, P: curPos(kw)}
	if p.at(token.Else) {
		p.advance()
		els, err := p.parseChildren()
		if err != nil {
			return nil, err
		}
		iff.Else = els
	}
	return iff, nil
}
