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

func tokenName(t TokenType) string {
	switch t {
	case TokenIdent:
		return "identifier"
	case TokenNumber:
		return "number"
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBrace:
		return "{"
	case TokenRBrace:
		return "}"
	case TokenComma:
		return ","
	case TokenSemicolon:
		return ";"
	case TokenEquals:
		return "="
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenStar:
		return "*"
	case TokenSlash:
		return "/"
	case TokenMod:
		return "%"
	case TokenCaret:
		return "^"
	case TokenDot:
		return "."
	case TokenLBracket:
		return "["
	case TokenRBracket:
		return "]"
	case TokenLess:
		return "<"
	case TokenGreater:
		return ">"
	case TokenLessEq:
		return "<="
	case TokenGreaterEq:
		return ">="
	case TokenEqEq:
		return "=="
	case TokenBangEq:
		return "!="
	case TokenAmp:
		return "&"
	case TokenAmpEq:
		return "&="
	case TokenAmpAmp:
		return "&&"
	case TokenPipePipe:
		return "||"
	case TokenReturn:
		return "return"
	case TokenVar:
		return "var"
	case TokenFor:
		return "for"
	case TokenYield:
		return "yield"
	case TokenFold:
		return "fold"
	case TokenAssert:
		return "assert"
	case TokenIf:
		return "if"
	case TokenElse:
		return "else"
	case TokenTrue:
		return "true"
	case TokenFalse:
		return "false"
	case TokenLib:
		return "lib"
	case TokenTypeKw:
		return "type"
	case TokenString:
		return "string"
	case TokenRawString:
		return "raw string"
	case TokenBang:
		return "!"
	case TokenFn:
		return "fn"
	case TokenConst:
		return "const"
	case TokenWhere:
		return "where"
	case TokenColon:
		return ":"
	case TokenPlusEq:
		return "+="
	case TokenMinusEq:
		return "-="
	case TokenStarEq:
		return "*="
	case TokenSlashEq:
		return "/="
	case TokenModEq:
		return "%="
	case TokenCaretEq:
		return "^="
	case TokenEOF:
		return "EOF"
	default:
		return "unknown"
	}
}
