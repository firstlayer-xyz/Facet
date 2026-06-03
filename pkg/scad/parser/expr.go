package parser

import (
	"fmt"

	"facet/pkg/scad/ast"
	"facet/pkg/scad/token"
)

var binPrec = map[token.Kind]int{
	token.Or: 1, token.And: 2,
	token.EqEq: 3, token.NeEq: 3,
	token.Lt: 4, token.Gt: 4, token.Le: 4, token.Ge: 4,
	token.Plus: 5, token.Minus: 5,
	token.Star: 6, token.Slash: 6, token.Percent: 6,
}

var binOp = map[token.Kind]string{
	token.Or: "||", token.And: "&&", token.EqEq: "==", token.NeEq: "!=",
	token.Lt: "<", token.Gt: ">", token.Le: "<=", token.Ge: ">=",
	token.Plus: "+", token.Minus: "-", token.Star: "*", token.Slash: "/", token.Percent: "%",
}

func (p *parser) parseExpr() (ast.Expr, error) { return p.parseTernary() }

func (p *parser) parseTernary() (ast.Expr, error) {
	cond, err := p.parseBinary(1)
	if err != nil {
		return nil, err
	}
	if p.at(token.Question) {
		q := p.advance()
		then, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return nil, err
		}
		els, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.Ternary{Cond: cond, Then: then, Else: els, P: curPos(q)}, nil
	}
	return cond, nil
}

func (p *parser) parseBinary(minPrec int) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		prec, ok := binPrec[p.cur().Kind]
		if !ok || prec < minPrec {
			return left, nil
		}
		opTok := p.advance()
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &ast.Binary{Op: binOp[opTok.Kind], L: left, R: right, P: curPos(opTok)}
	}
}

func (p *parser) parseUnary() (ast.Expr, error) {
	if p.at(token.Minus) || p.at(token.Bang) {
		op := p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		s := "-"
		if op.Kind == token.Bang {
			s = "!"
		}
		return &ast.Unary{Op: s, X: x, P: curPos(op)}, nil
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (ast.Expr, error) {
	x, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Kind {
		case token.LBracket:
			lb := p.advance()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return nil, err
			}
			x = &ast.Index{X: x, Index: idx, P: curPos(lb)}
		case token.Dot:
			p.advance()
			name, err := p.expect(token.Ident)
			if err != nil {
				return nil, err
			}
			x = &ast.Member{X: x, Name: name.Text, P: curPos(name)}
		default:
			return x, nil
		}
	}
}

func (p *parser) parsePrimary() (ast.Expr, error) {
	t := p.cur()
	switch t.Kind {
	case token.Number:
		p.advance()
		return &ast.Num{Text: t.Text, P: curPos(t)}, nil
	case token.String:
		p.advance()
		return &ast.Str{Value: t.Text, P: curPos(t)}, nil
	case token.True, token.False:
		p.advance()
		return &ast.Bool{Val: t.Kind == token.True, P: curPos(t)}, nil
	case token.Undef:
		p.advance()
		return &ast.Undef{P: curPos(t)}, nil
	case token.Ident:
		p.advance()
		if p.at(token.LParen) { // function call
			args, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			return &ast.Call{Name: t.Text, Args: args, P: curPos(t)}, nil
		}
		return &ast.Ident{Name: t.Text, SpecialVar: t.SpecialVar, P: curPos(t)}, nil
	case token.LParen:
		p.advance()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(token.RParen)
		return e, err
	case token.LBracket:
		return p.parseBracket()
	case token.Let:
		return p.parseLetExpr()
	}
	return nil, p.errf("unexpected token %q in expression", t.Text)
}

// parseBracket parses either a list comprehension `[for (var = iter, …) body]`,
// a vector `[a, b, …]`, or a range `[a:b]` / `[a:s:b]`.
func (p *parser) parseBracket() (ast.Expr, error) {
	lb := p.advance() // [
	if p.at(token.RBracket) {
		p.advance()
		return &ast.Vector{P: curPos(lb)}, nil
	}
	if p.at(token.For) {
		return p.parseListComp(lb)
	}
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.at(token.Colon) { // range
		p.advance()
		second, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		rng := &ast.Range{Start: first, End: second, P: curPos(lb)}
		if p.at(token.Colon) { // [start:step:end] — OpenSCAD step in MIDDLE
			p.advance()
			third, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			rng.Step = second
			rng.End = third
		}
		if _, err := p.expect(token.RBracket); err != nil {
			return nil, err
		}
		return rng, nil
	}
	// vector
	elems := []ast.Expr{first}
	for p.at(token.Comma) {
		p.advance()
		if p.at(token.RBracket) { // trailing comma
			break
		}
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, e)
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	return &ast.Vector{Elems: elems, P: curPos(lb)}, nil
}

// parseListComp parses `[for (var = iter, …) body]`. The opening `[` is
// already consumed; `lb` is its token for position tracking.
func (p *parser) parseListComp(lb token.Token) (ast.Expr, error) {
	p.advance() // consume `for`
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
		if !p.at(token.Comma) {
			break
		}
		p.advance()
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	return &ast.ListComp{Iters: iters, Body: body, P: curPos(lb)}, nil
}

func (p *parser) parseLetExpr() (ast.Expr, error) {
	lt := p.advance() // let
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	binds, err := p.parseLetBinds()
	if err != nil {
		return nil, err
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.Let{Binds: binds, Body: body, P: curPos(lt)}, nil
}

// parseLetBinds parses `name = expr (, name = expr)* )`.
func (p *parser) parseLetBinds() ([]ast.Assign, error) {
	var binds []ast.Assign
	for {
		name, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Assign); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		binds = append(binds, ast.Assign{Name: name.Text, Value: val, P: curPos(name)})
		if p.at(token.Comma) {
			p.advance()
			continue
		}
		break
	}
	_, err := p.expect(token.RParen)
	return binds, err
}

// parseArgs parses `( (name=)?expr , ... )`.
func (p *parser) parseArgs() ([]ast.Arg, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var args []ast.Arg
	for !p.at(token.RParen) {
		if p.at(token.EOF) {
			return nil, p.errf("unexpected end of input in argument list")
		}
		var name string
		// named arg: IDENT '=' (but not '==')
		// Guard the lookahead: p.pos+1 is always valid because the lexer ends
		// with EOF, so there is always at least one token after the current one
		// while pos < len-1. The advance() method never goes past len-1.
		if p.at(token.Ident) && p.pos+1 < len(p.toks) && p.toks[p.pos+1].Kind == token.Assign {
			name = p.advance().Text
			p.advance() // =
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, ast.Arg{Name: name, Value: val})
		// Require a comma or the closing paren; otherwise the loop would not
		// make progress and would hang on malformed input.
		if p.at(token.Comma) {
			p.advance()
		} else if !p.at(token.RParen) {
			return nil, p.errf("expected ',' or ')' in argument list, got %q", p.cur().Text)
		}
	}
	_, err := p.expect(token.RParen)
	return args, err
}

func (p *parser) errf(format string, a ...any) error {
	return fmt.Errorf("line %d:%d: "+format, append([]any{p.cur().Line, p.cur().Col}, a...)...)
}
