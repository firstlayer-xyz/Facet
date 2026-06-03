// Package parser builds an OpenSCAD AST from source.
package parser

import (
	"fmt"

	"facet/pkg/scad/ast"
	"facet/pkg/scad/token"
)

type parser struct {
	toks []token.Token
	pos  int
}

func (p *parser) cur() token.Token     { return p.toks[p.pos] }
func (p *parser) at(k token.Kind) bool { return p.cur().Kind == k }

func (p *parser) advance() token.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) expect(k token.Kind) (token.Token, error) {
	if !p.at(k) {
		return token.Token{}, fmt.Errorf("line %d:%d: expected token %d, got %d (%q)",
			p.cur().Line, p.cur().Col, k, p.cur().Kind, p.cur().Text)
	}
	return p.advance(), nil
}

func curPos(t token.Token) ast.Pos { return ast.Pos{Line: t.Line, Col: t.Col} }
