package parser

import (
	"fmt"
	"strings"
)

// parseLambda parses a lambda expression: fn(params) [ReturnType] { body }
// Called when the current token is TokenFn in expression position.
func (p *parser) parseLambda() (*LambdaExpr, error) {
	fnTok, err := p.expect(TokenFn)
	if err != nil {
		return nil, err
	}
	pos := Pos{fnTok.Line, fnTok.Col}

	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	var params []*Param
	if p.cur.Type != TokenRParen {
		var perr error
		params, perr = p.parseParams()
		if perr != nil {
			return nil, perr
		}
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	retType, err := p.parseReturnTypeStr()
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	body, err := p.parseBodyStmts(false, "lambda")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &LambdaExpr{Params: params, ReturnType: retType, Body: body, Pos: pos}, nil
}

// parseFuncTypeStr parses a function type expression like fn(Solid, Length) Solid
// and returns it as a string (e.g. "fn(Solid,Length) Solid").
// Used for function-typed parameter declarations and return types.
func (p *parser) parseFuncTypeStr() (string, error) {
	if _, err := p.expect(TokenFn); err != nil {
		return "", err
	}
	if _, err := p.expect(TokenLParen); err != nil {
		return "", err
	}
	var paramTypes []string
	for p.cur.Type != TokenRParen && p.cur.Type != TokenEOF {
		t, err := p.parseTypeStr()
		if err != nil {
			return "", err
		}
		paramTypes = append(paramTypes, t)
		if p.cur.Type == TokenComma {
			if err := p.next(); err != nil {
				return "", err
			}
		}
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return "", err
	}
	// Optional return type.
	var retType string
	switch p.cur.Type {
	case TokenIdent, TokenVar, TokenFn, TokenLBracket:
		var err error
		retType, err = p.parseTypeStr()
		if err != nil {
			return "", err
		}
	}
	result := "fn(" + strings.Join(paramTypes, ",") + ")"
	if retType != "" {
		result += " " + retType
	}
	return result, nil
}

// parseTypeStr parses a complete type expression.
// Handles: ident, T.Type (qualified), var, []T (prefix array), fn(T...) R,
// and a postfix `?` making the type Optional. The `?` is consumed once after
// the base type, so `Number?` parses but `Number??` errors (double optional
// is meaningless — Bind / Map flatten).
func (p *parser) parseTypeStr() (string, error) {
	t, err := p.parseTypeStrBase()
	if err != nil {
		return "", err
	}
	if p.cur.Type == TokenQuestion {
		if err := p.next(); err != nil { // consume '?'
			return "", err
		}
		if p.cur.Type == TokenQuestion {
			return "", p.errorf("nested optional %q?? is not allowed — Optional is a single layer", t)
		}
		return t + "?", nil
	}
	// A `??` token at type position is the nested-optional spelling
	// (the lexer prefers the longest match, so `Number??` tokenizes as
	// `Number` + `??` rather than `Number` + `?` + `?`). Report the same
	// error as the explicit two-? case.
	if p.cur.Type == TokenQuestionQuestion {
		return "", p.errorf("nested optional %q?? is not allowed — Optional is a single layer", t)
	}
	return t, nil
}

// parseTypeStrBase parses the part of a type that comes before any postfix `?`.
func (p *parser) parseTypeStrBase() (string, error) {
	p.depth++
	if p.depth > maxParseDepth {
		return "", p.errorf("type too deeply nested (limit %d)", maxParseDepth)
	}
	defer func() { p.depth-- }()
	// Prefix array: []T
	if p.cur.Type == TokenLBracket {
		if err := p.next(); err != nil { // consume '['
			return "", err
		}
		if _, err := p.expect(TokenRBracket); err != nil {
			return "", err
		}
		inner, err := p.parseTypeStr()
		if err != nil {
			return "", err
		}
		return "[]" + inner, nil
	}
	if p.cur.Type == TokenFn {
		return p.parseFuncTypeStr()
	}
	if p.cur.Type == TokenVar {
		if err := p.next(); err != nil {
			return "", err
		}
		return "var", nil
	}
	if p.cur.Type == TokenIdent {
		tok, err := p.expect(TokenIdent)
		if err != nil {
			return "", err
		}
		t := tok.Text
		if p.cur.Type == TokenDot {
			if err := p.next(); err != nil {
				return "", err
			}
			qualTok, err := p.expect(TokenIdent)
			if err != nil {
				return "", err
			}
			t = t + "." + qualTok.Text
		}
		return t, nil
	}
	return "", p.errorf("expected type, got %q", p.cur.Text)
}

// parseReturnTypeStr parses an optional return type annotation after a parameter list.
// Returns "" if no return type is present (next token is not a type-starting token).
func (p *parser) parseReturnTypeStr() (string, error) {
	switch p.cur.Type {
	case TokenLBracket, TokenVar, TokenIdent, TokenFn:
		return p.parseTypeStr()
	}
	return "", nil
}

// IsOptionalType reports whether a parsed type string is an optional
// (carries a trailing `?`). The corresponding inner type is what you get
// from OptionalInner.
func IsOptionalType(t string) bool {
	return len(t) > 0 && t[len(t)-1] == '?'
}

// OptionalInner returns the inner type of an optional. Panics if t isn't an
// optional — callers should guard with IsOptionalType.
func OptionalInner(t string) string {
	if !IsOptionalType(t) {
		return t
	}
	return t[:len(t)-1]
}

// isOperatorToken returns true if the token type is an operator that can
// be used as an operator function name (fn +, fn -, fn *, etc.).
func isOperatorToken(t TokenType) bool {
	switch t {
	case TokenPlus, TokenMinus, TokenStar, TokenSlash, TokenMod, TokenCaret, TokenAmp, TokenPipe,
		TokenEqEq, TokenBangEq, TokenLess, TokenGreater, TokenLessEq, TokenGreaterEq:
		return true
	}
	return false
}

// parseFunction → "fn" name "(" [params] ")" [returnType] "{" { statement } "}"
// name → IDENT | IDENT "." IDENT (receiver.method) | operator (for operator functions)
// returnType → IDENT | IDENT "." IDENT | []T | fn(...) R | var
func (p *parser) parseFunction() (*Function, error) {
	fnTok, err := p.expect(TokenFn)
	if err != nil {
		return nil, err
	}

	var receiverType string
	var name Token
	isOperator := false

	// Check for operator function: fn +(params) or fn -(params) etc.
	if isOperatorToken(p.cur.Type) {
		isOperator = true
		name = p.cur
		if err := p.next(); err != nil { // consume operator token
			return nil, err
		}
	} else {
		// Read function/method name: IDENT or IDENT.IDENT
		first, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}

		if p.cur.Type == TokenDot {
			// Receiver.Method
			if err := p.next(); err != nil { // consume '.'
				return nil, err
			}
			name, err = p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			receiverType = first.Text
		} else {
			name = first
		}

		// Reject _-prefixed function/method names in user code
		if err := p.rejectUnderscoreIdent(name, "function"); err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	var params []*Param
	if p.cur.Type != TokenRParen {
		params, err = p.parseParams()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	// Optional trailing return type
	retType, err := p.parseReturnTypeStr()
	if err != nil {
		return nil, err
	}

	lbrace, err := p.expect(TokenLBrace)
	if err != nil {
		return nil, err
	}
	// Capture trailing comments on the opening brace line (e.g. fn Foo() { // comment)
	trailingComments := p.drainTrailingComments(lbrace.Line)

	body, err := p.parseBodyStmts(false, fmt.Sprintf("function %q", name.Text))
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return &Function{
		ReturnType:   retType,
		Name:         name.Text,
		ReceiverType: receiverType,
		IsOperator:   isOperator,
		Params:       params,
		Body:         body,
		Pos:          Pos{fnTok.Line, fnTok.Col},
		Comments:     trailingComments,
	}, nil
}

// parseParams → paramGroup { "," paramGroup }
// paramGroup → name { "," name } typeExpr [ "=" defaultExpr ] [ "where" constraint ]
// Each group has one or more names followed by a shared type.
// Defaults and constraints are only allowed on single-name groups (not grouped params).
//   fn Foo(x, y, z Length)              →  all required
//   fn Foo(radius Length, seg Number = 0)  →  seg has default
// Required params must come before optional params (across the entire signature).
func (p *parser) parseParams() ([]*Param, error) {
	var params []*Param
	for {
		var names []Token

		// Read the first name.
		firstTok, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		names = append(names, firstTok)

		// Greedily read more names: comma + ident pairs.
		// Stop when: no comma, or after consuming comma the next token is not an ident
		// (meaning it's a type token like fn, [, var, or a syntax error).
		for p.cur.Type == TokenComma {
			snap := p.lex.snapshot()
			savedCur := p.cur
			if err := p.next(); err != nil { // tentatively consume ','
				return nil, err
			}
			if p.cur.Type != TokenIdent {
				// Not an ident after comma — this comma is a group separator.
				p.lex.restore(snap)
				p.cur = savedCur
				break
			}
			nameTok, err := p.expect(TokenIdent)
			if err != nil {
				return nil, err
			}
			names = append(names, nameTok)
		}

		// Parse the shared type for all names in this group.
		typeStr, err := p.parseTypeStr()
		if err != nil {
			return nil, err
		}

		// Parse optional default and constraint (only valid for single-name groups).
		var defExpr Expr
		var constraint Expr
		if p.cur.Type == TokenEquals {
			if len(names) > 1 {
				return nil, p.errorf("default values are not allowed on grouped parameters; use separate declarations")
			}
			if err := p.next(); err != nil {
				return nil, err
			}
			defExpr, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		constraint, err = p.parseWhereConstraint()
		if err != nil {
			return nil, err
		}
		if constraint != nil && len(names) > 1 {
			return nil, p.errorf("constraints are not allowed on grouped parameters; use separate declarations")
		}

		// Emit params. Multi-name groups (`a, b var`) share a positive
		// GroupID so the checker can treat them as one type slot; singletons
		// stay at 0.
		groupID := 0
		if len(names) > 1 {
			p.groupSeq++
			groupID = p.groupSeq
		}
		for _, tok := range names {
			if err := p.rejectUnderscoreIdent(tok, "parameter"); err != nil {
				return nil, err
			}
			params = append(params, &Param{Type: typeStr, Name: tok.Text, Default: defExpr, Constraint: constraint, Pos: Pos{Line: tok.Line, Col: tok.Col}, GroupID: groupID})
		}

		if p.cur.Type != TokenComma {
			break
		}
		if err := p.next(); err != nil { // consume group separator comma
			return nil, err
		}
		// Allow trailing comma before closing paren
		if p.cur.Type == TokenRParen {
			break
		}
	}
	return params, nil
}
