package parser

import "fmt"

// Parse parses a facet source string and returns the AST.
func Parse(source string) (*Source, error) {
	p := &parser{lex: newLexer(source)}
	if err := p.next(); err != nil {
		return nil, err
	}
	return p.parseProgram()
}

const maxParseDepth = 256

type parser struct {
	lex        *lexer
	cur        Token
	inForYield int // nesting depth; > 0 means yield is valid in nested blocks
	depth      int // expression recursion depth
}

func (p *parser) next() error {
	tok, err := p.lex.Next()
	if err != nil {
		return err
	}
	p.cur = tok
	return nil
}

func (p *parser) errorf(format string, args ...interface{}) error {
	return &SourceError{Line: p.cur.Line, Col: p.cur.Col, Message: fmt.Sprintf(format, args...)}
}

func (p *parser) expect(typ TokenType) (Token, error) {
	if p.cur.Type != typ {
		return Token{}, p.errorf("expected %s, got %q", tokenName(typ), p.cur.Text)
	}
	tok := p.cur
	if err := p.next(); err != nil {
		return Token{}, err
	}
	return tok, nil
}

// consumeOptionalSemi consumes a semicolon token if one is present (explicit or
// ASI-synthesised), and does nothing otherwise.  Use this at the end of
// statement parsers so that trailing semicolons remain optional.
func (p *parser) consumeOptionalSemi() error {
	if p.cur.Type == TokenSemicolon {
		return p.next()
	}
	return nil
}

// rejectUnderscoreIdent returns an error if the identifier starts with '_'.
// Identifiers starting with '_' are reserved for internal builtins.
func (p *parser) rejectUnderscoreIdent(tok Token, kind string) error {
	if len(tok.Text) > 0 && tok.Text[0] == '_' {
		return p.errorf("identifiers starting with '_' are reserved (in %s %q)", kind, tok.Text)
	}
	return nil
}
