package parser

import "slices"

// parseExpr → ternaryExpr
func (p *parser) parseExpr() (Expr, error) {
	p.depth++
	if p.depth > maxParseDepth {
		return nil, p.errorf("expression too deeply nested (limit %d)", maxParseDepth)
	}
	defer func() { p.depth-- }()
	return p.parseTernaryExpr()
}

// parseTernaryExpr → binaryExpr [ "?" expr ":" expr ]
// The conditional expression `cond ? a : b`. cond must be Bool, both arms
// must produce compatible types. Right-associative: `a ? b : c ? d : e`
// parses as `a ? b : (c ? d : e)`. Each arm is parsed via parseTernaryExpr
// so a nested ternary can re-enter on the right.
func (p *parser) parseTernaryExpr() (Expr, error) {
	cond, err := p.parseBinaryExpr(0)
	if err != nil {
		return nil, err
	}
	if p.cur.Type != TokenQuestion {
		return cond, nil
	}
	line, col := p.cur.Line, p.cur.Col
	if err := p.next(); err != nil { // consume '?'
		return nil, err
	}
	thenExpr, err := p.parseTernaryExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenColon); err != nil {
		return nil, err
	}
	elseExpr, err := p.parseTernaryExpr()
	if err != nil {
		return nil, err
	}
	return &TernaryExpr{Cond: cond, Then: thenExpr, Else: elseExpr, Pos: Pos{line, col}}, nil
}

// binLevels is the binary-operator precedence ladder, loosest level first.
// Each level is left-associative except the comparison level, which is
// non-associative (parses at most one comparison operator).
// `??` sits between `||` (looser) and `&&` (tighter): `cond || opt ?? def`
// parses as `cond || (opt ?? def)`, while `a && b ?? c` parses as `(a && b) ?? c`.
// Mixing `??` with `&&` on an optional therefore needs explicit parens —
// `a && (b ?? c)`.
var binLevels = []struct {
	tokens   []TokenType
	nonAssoc bool
}{
	{[]TokenType{TokenPipePipe}, false},
	{[]TokenType{TokenQuestionQuestion}, false},
	{[]TokenType{TokenAmpAmp}, false},
	{[]TokenType{TokenLess, TokenGreater, TokenLessEq, TokenGreaterEq, TokenEqEq, TokenBangEq}, true},
	{[]TokenType{TokenPipe}, false},
	{[]TokenType{TokenCaret}, false},
	{[]TokenType{TokenAmp}, false},
	{[]TokenType{TokenPlus, TokenMinus}, false},
	{[]TokenType{TokenStar, TokenSlash, TokenMod}, false},
}

// parseBinaryExpr parses the precedence level at the given binLevels index;
// past the last level it falls through to parsePostfix.
func (p *parser) parseBinaryExpr(level int) (Expr, error) {
	if level == len(binLevels) {
		return p.parsePostfix()
	}
	left, err := p.parseBinaryExpr(level + 1)
	if err != nil {
		return nil, err
	}
	for slices.Contains(binLevels[level].tokens, p.cur.Type) {
		op, line, col := p.cur.Text, p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseBinaryExpr(level + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
		if binLevels[level].nonAssoc {
			break
		}
	}
	return left, nil
}

// isUnitSuffix checks if the current token is a unit identifier that should be
// applied as a postfix unit conversion.
func (p *parser) isUnitSuffix() bool {
	if p.cur.Type != TokenIdent {
		return false
	}
	_, isAngle := AngleFactors[p.cur.Text]
	_, isUnit := UnitFactors[p.cur.Text]
	return isAngle || isUnit
}

// parsePostfix → parsePrimary { "." IDENT "(" [ args ] ")" | "[" expr "]" | UNIT | Lib.Type "{" ... "}" }
func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parsePostfixOn(expr)
}

// parsePostfixOn applies postfix operators to an already-parsed expression,
// looping until none apply. Pulling this out of parsePostfix lets a qualified
// struct literal (`Lib.Type{...}`) feed back through the chain so a method or
// field can follow it (`Lib.Type{...}.Solid()`).
func (p *parser) parsePostfixOn(expr Expr) (Expr, error) {
	for p.cur.Type == TokenDot || p.cur.Type == TokenQuestionDot || p.cur.Type == TokenLBracket || p.isUnitSuffix() {
		// Unit suffix: 5 mm, (1/2) mm, Foo() deg
		if p.isUnitSuffix() {
			line, col := p.cur.Line, p.cur.Col
			unit := p.cur.Text
			factor, isAngle := AngleFactors[unit]
			if !isAngle {
				factor = UnitFactors[unit]
			}
			if err := p.next(); err != nil {
				return nil, err
			}
			expr = &UnitExpr{Expr: expr, Unit: unit, Factor: factor, IsAngle: isAngle, Pos: Pos{line, col}}
			continue
		}
		if p.cur.Type == TokenLBracket {
			line, col := p.cur.Line, p.cur.Col
			if err := p.next(); err != nil { // consume '['
				return nil, err
			}
			// Check for [:end] slice (no start)
			if p.cur.Type == TokenColon {
				if err := p.next(); err != nil { // consume ':'
					return nil, err
				}
				var end Expr
				var err error
				if p.cur.Type != TokenRBracket {
					end, err = p.parseExpr()
					if err != nil {
						return nil, err
					}
				}
				if _, err := p.expect(TokenRBracket); err != nil {
					return nil, err
				}
				expr = &SliceExpr{Receiver: expr, Start: nil, End: end, Pos: Pos{line, col}}
				continue
			}
			index, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			// Check for [start:end] slice
			if p.cur.Type == TokenColon {
				if err := p.next(); err != nil { // consume ':'
					return nil, err
				}
				var end Expr
				if p.cur.Type != TokenRBracket {
					end, err = p.parseExpr()
					if err != nil {
						return nil, err
					}
				}
				if _, err := p.expect(TokenRBracket); err != nil {
					return nil, err
				}
				expr = &SliceExpr{Receiver: expr, Start: index, End: end, Pos: Pos{line, col}}
				continue
			}
			if _, err := p.expect(TokenRBracket); err != nil {
				return nil, err
			}
			expr = &IndexExpr{Receiver: expr, Index: index, Pos: Pos{line, col}}
			continue
		}
		// `?.` parses identically to `.` and sets Optional on the node.
		optional := p.cur.Type == TokenQuestionDot
		if err := p.next(); err != nil { // consume '.' or '?.'
			return nil, err
		}
		methodLine, methodCol := p.cur.Line, p.cur.Col
		method, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		// Field access: no '(' after .IDENT
		if p.cur.Type != TokenLParen {
			expr = &FieldAccessExpr{Receiver: expr, Field: method.Text, Pos: Pos{methodLine, methodCol}, Optional: optional}
			continue
		}
		if _, err := p.expect(TokenLParen); err != nil {
			return nil, err
		}
		args, err := p.parseCallArgs()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		expr = &MethodCallExpr{Receiver: expr, Method: method.Text, Args: args, Pos: Pos{methodLine, methodCol}, Optional: optional}
	}
	// Qualified struct literal: T.Thread { field: val }. After parsing it,
	// continue the chain so `.method()` / `.field` can follow.
	// Pos points at T (start of the whole expression) for diagnostics;
	// TypeNamePos points at Thread (the field token after the dot) so
	// references resolve there.
	if p.cur.Type == TokenLBrace {
		if fa, ok := expr.(*FieldAccessExpr); ok {
			if id, ok := fa.Receiver.(*IdentExpr); ok {
				if p.isStructLitStart() || p.isEmptyBrace() {
					lit, err := p.parseStructLit(id.Name+"."+fa.Field, id.Pos, fa.Pos)
					if err != nil {
						return nil, err
					}
					return p.parsePostfixOn(lit)
				}
			}
		}
	}
	return expr, nil
}
