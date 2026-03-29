package parser

import "fmt"

// SourceKind describes the origin of a parsed source file.
type SourceKind int

const (
	SourceUser    SourceKind = iota // user-written source file
	SourceStdLib                    // standard library (facet/std)
	SourceLibrary                   // external library (git-fetched)
	SourceCached                    // cached library (previously fetched)
	SourceExample                   // built-in example (read-only)
)

// Source is the root AST node representing a single parsed source file.
type Source struct {
	Kind             SourceKind // origin of this source
	Path             string     // disk path to the .fct file; zero after Parse, set by loader
	Text             string     // raw source code; zero after Parse
	Declarations     []Decl
	TrailingComments []Comment // comments after last declaration
}

// Parse parses a facet source string and returns the AST.
// The path is the disk path to the source file (used for error reporting and source tracking).
// The kind describes the origin of the source (SourceUser, SourceStdLib, etc.).
func Parse(source, path string, kind SourceKind) (*Source, error) {
	p := &parser{lex: newLexer(source)}
	if err := p.next(); err != nil {
		return nil, err
	}
	src, err := p.parseProgram()
	if err != nil {
		return nil, err
	}
	src.Path = path
	src.Kind = kind
	src.Text = source
	return src, nil
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
