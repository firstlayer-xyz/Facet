package checker

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
)

// inferBinaryOp infers the result type of a binary operation.
func (c *checker) inferBinaryOp(ex *parser.BinaryExpr, env *typeEnv) typeInfo {
	left := c.inferExpr(ex.Left, env)
	right := c.inferExpr(ex.Right, env)

	// If either side is unknown, propagate unknown
	if left.ft == typeUnknown || right.ft == typeUnknown {
		return unknown()
	}

	op := ex.Op

	// Check operator function map first (before hardcoded rules)
	if ret, ok := c.opMap[opMapKey{op: op, leftType: left.displayName(), rightType: right.displayName()}]; ok {
		return ret
	}

	// Logical operators
	if op == "&&" || op == "||" {
		if left.ft != typeBool {
			c.addError(ex.Pos, fmt.Sprintf("operator %s: left operand must be Bool, got %s", op, left.displayName()))
			return unknown()
		}
		if right.ft != typeBool {
			c.addError(ex.Pos, fmt.Sprintf("operator %s: right operand must be Bool, got %s", op, right.displayName()))
			return unknown()
		}
		return simple(typeBool)
	}

	// Comparison operators
	if op == "<" || op == ">" || op == "<=" || op == ">=" || op == "==" || op == "!=" {
		return c.inferComparison(ex, left, right)
	}

	// Array concatenation / append — preserve element type
	if left.ft == typeArray && op == "+" {
		if right.ft == typeArray {
			// Both arrays: merge element types
			if left.elem != nil && right.elem != nil {
				if c.typeCompatible(*left.elem, *right.elem) {
					return left
				}
				if c.typeCompatible(*right.elem, *left.elem) {
					return right
				}
				// Incompatible element types — report error
				c.addError(ex.Pos, fmt.Sprintf("cannot concatenate %s and %s: incompatible element types",
					left.displayName(), right.displayName()))
				return left
			}
			if left.elem != nil {
				return left
			}
			return right
		}
		// Array + non-array (append)
		return left
	}

	// String concatenation
	if left.ft == typeString && right.ft == typeString {
		if op == "+" {
			return simple(typeString)
		}
		c.addError(ex.Pos, fmt.Sprintf("operator %s not supported on String values", op))
		return unknown()
	}

	// Angle arithmetic
	if left.ft == typeAngle && right.ft == typeAngle {
		switch op {
		case "+", "-":
			return simple(typeAngle)
		case "/":
			return simple(typeNumber)
		default:
			c.addError(ex.Pos, fmt.Sprintf("operator %s not supported on Angle values", op))
			return unknown()
		}
	}

	// Angle * Number or Angle / Number → Angle
	if left.ft == typeAngle && right.ft == typeNumber {
		if op == "*" || op == "/" {
			return simple(typeAngle)
		}
		c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types Angle and Number", op))
		return unknown()
	}
	// Number * Angle → Angle
	if left.ft == typeNumber && right.ft == typeAngle {
		if op == "*" {
			return simple(typeAngle)
		}
		c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types Number and Angle", op))
		return unknown()
	}

	// Number op Number → Number
	if left.ft == typeNumber && right.ft == typeNumber {
		switch op {
		case "+", "-", "*", "/", "%":
			return simple(typeNumber)
		default:
			c.addError(ex.Pos, fmt.Sprintf("unknown operator %q", op))
			return unknown()
		}
	}

	// Length / Length → Number
	if left.ft == typeLength && right.ft == typeLength && op == "/" {
		return simple(typeNumber)
	}

	// Length op Length → Length (for +, -, %) or Number (for *)
	if left.ft == typeLength && right.ft == typeLength {
		switch op {
		case "+", "-", "%":
			return simple(typeLength)
		case "*":
			return simple(typeNumber)
		default:
			c.addError(ex.Pos, fmt.Sprintf("unknown operator %q", op))
			return unknown()
		}
	}

	// Length op Number or Number op Length → Length (mixed arithmetic with coercion)
	if (left.ft == typeLength && right.ft == typeNumber) || (left.ft == typeNumber && right.ft == typeLength) {
		switch op {
		case "+", "-", "*", "/", "%":
			return simple(typeLength)
		default:
			c.addError(ex.Pos, fmt.Sprintf("unknown operator %q", op))
			return unknown()
		}
	}

	c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types %s and %s", op, left.displayName(), right.displayName()))
	return unknown()
}

// inferComparison infers the result type of a comparison operation.
func (c *checker) inferComparison(ex *parser.BinaryExpr, left, right typeInfo) typeInfo {
	op := ex.Op

	// Same-type comparisons
	if left.ft == right.ft {
		switch left.ft {
		case typeLength, typeNumber, typeAngle:
			return simple(typeBool)
		case typeBool:
			if op == "==" || op == "!=" {
				return simple(typeBool)
			}
		case typeString:
			return simple(typeBool)
		case typeStruct:
			if op == "==" || op == "!=" {
				// Delegate to opMap for struct comparisons (Vec/Pt == and !=)
				if ret, ok := c.opMap[opMapKey{op: op, leftType: left.displayName(), rightType: right.displayName()}]; ok {
					return ret
				}
			}
		}
	}

	// Length/Number mixed comparison (coercion)
	if (left.ft == typeLength && right.ft == typeNumber) || (left.ft == typeNumber && right.ft == typeLength) {
		return simple(typeBool)
	}

	c.addError(ex.Pos, fmt.Sprintf("operator %s: incompatible types %s and %s", op, left.displayName(), right.displayName()))
	return unknown()
}
