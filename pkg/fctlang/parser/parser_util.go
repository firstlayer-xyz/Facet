package parser

// parseWhereConstraint parses an optional "where [constraint]" clause.
// Returns nil if the current token is not 'where'.
func (p *parser) parseWhereConstraint() (Expr, error) {
	if p.cur.Type != TokenWhere {
		return nil, nil
	}
	if err := p.next(); err != nil { // consume 'where'
		return nil, err
	}
	if p.cur.Type != TokenLBracket {
		return nil, p.errorf("expected '[' after 'where', got %q", p.cur.Text)
	}
	return p.parseConstraint()
}

// tokenNames maps each token type to its display string for expect() errors.
var tokenNames = map[TokenType]string{
	TokenIdent: "identifier", TokenNumber: "number",
	TokenString: "string", TokenRawString: "raw string",
	TokenLParen: "(", TokenRParen: ")", TokenLBrace: "{", TokenRBrace: "}",
	TokenLBracket: "[", TokenRBracket: "]", TokenComma: ",", TokenSemicolon: ";",
	TokenColon: ":", TokenDot: ".", TokenEquals: "=",
	TokenPlus: "+", TokenMinus: "-", TokenStar: "*", TokenSlash: "/",
	TokenMod: "%", TokenCaret: "^", TokenBang: "!",
	TokenLess: "<", TokenGreater: ">", TokenLessEq: "<=", TokenGreaterEq: ">=",
	TokenEqEq: "==", TokenBangEq: "!=",
	TokenAmp: "&", TokenAmpEq: "&=", TokenAmpAmp: "&&",
	TokenPipe: "|", TokenPipeEq: "|=", TokenPipePipe: "||",
	TokenPlusEq: "+=", TokenMinusEq: "-=", TokenStarEq: "*=",
	TokenSlashEq: "/=", TokenModEq: "%=", TokenCaretEq: "^=",
	TokenReturn: "return", TokenVar: "var", TokenFor: "for", TokenYield: "yield",
	TokenFold: "fold", TokenAssert: "assert", TokenIf: "if", TokenElse: "else",
	TokenTrue: "true", TokenFalse: "false", TokenLib: "lib", TokenTypeKw: "type",
	TokenFn: "fn", TokenConst: "const", TokenWhere: "where",
	TokenEOF: "EOF",
}

func tokenName(t TokenType) string {
	if n, ok := tokenNames[t]; ok {
		return n
	}
	return "unknown"
}
