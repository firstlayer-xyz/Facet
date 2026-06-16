package parser

// parseExpr → ternaryExpr
func (p *parser) parseExpr() (Expr, error) {
	p.depth++
	if p.depth > maxParseDepth {
		return nil, p.errorf("expression too deeply nested (limit %d)", maxParseDepth)
	}
	defer func() { p.depth-- }()
	return p.parseTernaryExpr()
}

// parseTernaryExpr → orExpr [ "?" expr ":" expr ]
// The conditional expression `cond ? a : b`. cond must be Bool, both arms
// must produce compatible types. Right-associative: `a ? b : c ? d : e`
// parses as `a ? b : (c ? d : e)`. Each arm is parsed via parseTernaryExpr
// so a nested ternary can re-enter on the right.
func (p *parser) parseTernaryExpr() (Expr, error) {
	cond, err := p.parseOrExpr()
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

// parseOrExpr → nullCoalesceExpr { "||" nullCoalesceExpr }
func (p *parser) parseOrExpr() (Expr, error) {
	left, err := p.parseNullCoalesceExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenPipePipe {
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseNullCoalesceExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: "||", Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseNullCoalesceExpr → andExpr { "??" andExpr }
// Precedence sits between `||` (looser) and `&&` (tighter): `cond || opt ?? def`
// parses as `cond || (opt ?? def)`, while `a && b ?? c` parses as `(a && b) ?? c`.
// Mixing `??` with `&&` on an optional therefore needs explicit parens —
// `a && (b ?? c)`.
func (p *parser) parseNullCoalesceExpr() (Expr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenQuestionQuestion {
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: "??", Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseAndExpr → compareExpr { "&&" compareExpr }
func (p *parser) parseAndExpr() (Expr, error) {
	left, err := p.parseCompareExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenAmpAmp {
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseCompareExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: "&&", Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseCompareExpr → bitwiseOrExpr [ cmpOp bitwiseOrExpr ]
func (p *parser) parseCompareExpr() (Expr, error) {
	left, err := p.parseBitwiseOrExpr()
	if err != nil {
		return nil, err
	}
	if p.cur.Type == TokenLess || p.cur.Type == TokenGreater ||
		p.cur.Type == TokenLessEq || p.cur.Type == TokenGreaterEq ||
		p.cur.Type == TokenEqEq || p.cur.Type == TokenBangEq {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseBitwiseOrExpr()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}, nil
	}
	return left, nil
}

// parseBitwiseOrExpr → bitwiseXorExpr { "|" bitwiseXorExpr }
func (p *parser) parseBitwiseOrExpr() (Expr, error) {
	left, err := p.parseBitwiseXorExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenPipe {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseBitwiseXorExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseBitwiseXorExpr → bitwiseAndExpr { "^" bitwiseAndExpr }
func (p *parser) parseBitwiseXorExpr() (Expr, error) {
	left, err := p.parseBitwiseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenCaret {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseBitwiseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseBitwiseAndExpr → addExpr { "&" addExpr }
func (p *parser) parseBitwiseAndExpr() (Expr, error) {
	left, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenAmp {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseAddExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseAddExpr → parseMulExpr { ("+" | "-") parseMulExpr }
func (p *parser) parseAddExpr() (Expr, error) {
	left, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenPlus || p.cur.Type == TokenMinus {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parseMulExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
	}
	return left, nil
}

// parseMulExpr → parsePostfix { ("*" | "/" | "%") parsePostfix }
func (p *parser) parseMulExpr() (Expr, error) {
	left, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenStar || p.cur.Type == TokenSlash || p.cur.Type == TokenMod {
		op := p.cur.Text
		line, col := p.cur.Line, p.cur.Col
		if err := p.next(); err != nil {
			return nil, err
		}
		right, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right, Pos: Pos{line, col}}
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

// parsePostfix → parsePrimary { "." IDENT "(" [ args ] ")" | "[" expr "]" | UNIT }
func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.cur.Type == TokenDot || p.cur.Type == TokenQuestionDot || p.cur.Type == TokenLBracket || p.isUnitSuffix() {
		// Unit suffix: 5 mm, (1/2) mm, Foo() deg
		if p.isUnitSuffix() {
			line, col := p.cur.Line, p.cur.Col
			unit := p.cur.Text
			if factor, ok := AngleFactors[unit]; ok {
				if err := p.next(); err != nil {
					return nil, err
				}
				expr = &UnitExpr{Expr: expr, Unit: unit, Factor: factor, IsAngle: true, Pos: Pos{line, col}}
				continue
			}
			if factor, ok := UnitFactors[unit]; ok {
				if err := p.next(); err != nil {
					return nil, err
				}
				expr = &UnitExpr{Expr: expr, Unit: unit, Factor: factor, IsAngle: false, Pos: Pos{line, col}}
				continue
			}
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
	// Check for qualified struct literal: T.Thread { field: val }
	// Pos points at T (start of the whole expression) for diagnostics;
	// TypeNamePos points at Thread (the field token after the dot) so
	// references resolve there.
	if p.cur.Type == TokenLBrace {
		if fa, ok := expr.(*FieldAccessExpr); ok {
			if id, ok := fa.Receiver.(*IdentExpr); ok {
				qualName := id.Name + "." + fa.Field
				if p.isStructLitStart() || p.isEmptyBrace() {
					return p.parseStructLit(qualName, id.Pos, fa.Pos)
				}
			}
		}
	}
	return expr, nil
}
