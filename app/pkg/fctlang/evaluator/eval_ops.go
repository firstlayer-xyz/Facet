package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"math"

	"facet/app/pkg/manifold"
)

func (e *evaluator) evalUnary(ex *parser.UnaryExpr, locals map[string]value) (value, error) {
	v, err := e.evalExpr(ex.Operand, locals)
	if err != nil {
		return nil, err
	}
	switch ex.Op {
	case "-":
		switch val := v.(type) {
		case float64:
			return -val, nil
		case length:
			return length{mm: -val.mm}, nil
		case angle:
			return angle{deg: -val.deg}, nil
		default:
			// Try operator function dispatch for unary minus
			if fn, ok := e.opFuncs[opFuncKey{op: "-", leftType: typeName(v)}]; ok {
				result, err := e.evalFunction(fn, map[string]value{fn.Params[0].Name: v})
				if err != nil {
					return nil, e.wrapErr(ex.Pos, err)
				}
				return result, nil
			}
			return nil, e.errAt(ex.Pos, "unary minus not supported on %s", typeName(v))
		}
	case "!":
		b, ok := v.(bool)
		if !ok {
			return nil, e.errAt(ex.Pos, "operator ! requires Bool, got %s", typeName(v))
		}
		return !b, nil
	default:
		return nil, e.errAt(ex.Pos, "unknown unary operator %q", ex.Op)
	}
}

func (e *evaluator) evalBinary(ex *parser.BinaryExpr, locals map[string]value) (value, error) {
	// Short-circuit logical operators
	if ex.Op == "&&" || ex.Op == "||" {
		lv, err := e.evalExpr(ex.Left, locals)
		if err != nil {
			return nil, err
		}
		lb, ok := lv.(bool)
		if !ok {
			return nil, e.errAt(ex.Pos, "operator %s: left operand must be Bool, got %s", ex.Op, typeName(lv))
		}
		if ex.Op == "&&" && !lb {
			return false, nil
		}
		if ex.Op == "||" && lb {
			return true, nil
		}
		rv, err := e.evalExpr(ex.Right, locals)
		if err != nil {
			return nil, err
		}
		rb, ok := rv.(bool)
		if !ok {
			return nil, e.errAt(ex.Pos, "operator %s: right operand must be Bool, got %s", ex.Op, typeName(rv))
		}
		return rb, nil
	}

	lv, err := e.evalExpr(ex.Left, locals)
	if err != nil {
		return nil, err
	}
	rv, err := e.evalExpr(ex.Right, locals)
	if err != nil {
		return nil, err
	}

	// Comparison operators
	if ex.Op == "<" || ex.Op == ">" || ex.Op == "<=" || ex.Op == ">=" || ex.Op == "==" || ex.Op == "!=" {
		return e.evalCompare(ex.Op, lv, rv, ex.Pos)
	}

	// Array concatenation / append
	if larr, lok := lv.(array); lok && ex.Op == "+" {
		if rarr, rok := rv.(array); rok {
			// array + array → concatenated array
			result := make([]value, len(larr.elems)+len(rarr.elems))
			copy(result, larr.elems)
			copy(result[len(larr.elems):], rarr.elems)
			et := larr.elemType
			if et != rarr.elemType {
				et = inferElemType(result)
			}
			return array{elems: result, elemType: et}, nil
		}
		// array + element → append
		result := make([]value, len(larr.elems)+1)
		copy(result, larr.elems)
		result[len(larr.elems)] = rv
		et := larr.elemType
		if et != "" && typeName(rv) != et {
			et = ""
		}
		return array{elems: result, elemType: et}, nil
	}

	// String concatenation
	if ls, lok := lv.(string); lok {
		if rs, rok := rv.(string); rok {
			if ex.Op == "+" {
				return ls + rs, nil
			}
			return nil, e.errAt(ex.Pos, "operator %s not supported on String values", ex.Op)
		}
	}

	// Solid boolean operations: +, -, &
	lsf, lIsSolid := lv.(*manifold.Solid)
	rsf, rIsSolid := rv.(*manifold.Solid)
	if lIsSolid && rIsSolid {
		var result *manifold.Solid
		var opName string
		switch ex.Op {
		case "+":
			result = lsf.Union(rsf)
			opName = "Union"
		case "-":
			result = lsf.Difference(rsf)
			opName = "Difference"
		case "&":
			result = lsf.Intersection(rsf)
			opName = "Intersection"
		default:
			// Fall through to operator function dispatch (e.g. fn %(Solid,Solid))
			goto solidOpFunc
		}
		e.trackSolid(ex.Pos, result)
		e.recordStep(opName, ex.Pos, debugEntry{"lhs", lsf}, debugEntry{"rhs", rsf}, debugEntry{"result", result})
		return result, nil
	solidOpFunc:
	}

	// Sketch boolean operations: +, -, &
	lpf, lIsProfile := lv.(*manifold.Sketch)
	rpf, rIsProfile := rv.(*manifold.Sketch)
	if lIsProfile && rIsProfile {
		var result *manifold.Sketch
		var opName string
		switch ex.Op {
		case "+":
			result = lpf.Union(rpf)
			opName = "Union"
		case "-":
			result = lpf.Difference(rpf)
			opName = "Difference"
		case "&":
			result = lpf.Intersection(rpf)
			opName = "Intersection"
		default:
			return nil, e.errAt(ex.Pos, "operator %s not supported on Sketch values", ex.Op)
		}
		e.recordStep(opName, ex.Pos, debugEntry{"lhs", lpf}, debugEntry{"rhs", rpf}, debugEntry{"result", result})
		return result, nil
	}

	// Angle arithmetic
	la, lIsAngle := lv.(angle)
	ra, rIsAngle := rv.(angle)
	if lIsAngle && rIsAngle {
		switch ex.Op {
		case "+":
			return angle{deg: la.deg + ra.deg}, nil
		case "-":
			return angle{deg: la.deg - ra.deg}, nil
		case "/":
			if ra.deg == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return la.deg / ra.deg, nil
		default:
			return nil, e.errAt(ex.Pos, "operator %s not supported on Angle values", ex.Op)
		}
	}
	// Number * Angle or Angle * Number → Angle
	if lIsAngle {
		if rn, ok := rv.(float64); ok {
			switch ex.Op {
			case "*":
				return angle{deg: la.deg * rn}, nil
			case "/":
				if rn == 0 {
					return nil, e.errAt(ex.Pos, "division by zero")
				}
				return angle{deg: la.deg / rn}, nil
			default:
				return nil, e.errAt(ex.Pos, "operator %s: incompatible types Angle and Number", ex.Op)
			}
		}
	}
	if rIsAngle {
		if ln, ok := lv.(float64); ok && ex.Op == "*" {
			return angle{deg: ln * ra.deg}, nil
		}
	}

	// Number arithmetic (both float64 → stays Number)
	if ln, lok := lv.(float64); lok {
		if rn, rok := rv.(float64); rok {
			switch ex.Op {
			case "+":
				return ln + rn, nil
			case "-":
				return ln - rn, nil
			case "*":
				return ln * rn, nil
			case "/":
				if rn == 0 {
					return nil, e.errAt(ex.Pos, "division by zero")
				}
				return ln / rn, nil
			case "%":
				if rn == 0 {
					return nil, e.errAt(ex.Pos, "modulo by zero")
				}
				return math.Mod(ln, rn), nil
			default:
				return nil, e.errAt(ex.Pos, "unknown operator %q", ex.Op)
			}
		}
	}

	// Length arithmetic — strict units, no silent promotion between Length and
	// Number. To cross the dimensionless/Length boundary, use Number(from: x) to
	// convert Length→Number explicitly; Number→Length still auto-coerces at
	// value-category boundaries (var decls, arguments, struct fields, returns).
	lLen, lIsLen := lv.(length)
	rLen, rIsLen := rv.(length)
	lNum, lIsNum := lv.(float64)
	rNum, rIsNum := rv.(float64)

	// Length op Length
	if lIsLen && rIsLen {
		switch ex.Op {
		case "+":
			return length{mm: lLen.mm + rLen.mm}, nil
		case "-":
			return length{mm: lLen.mm - rLen.mm}, nil
		case "/":
			if rLen.mm == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return lLen.mm / rLen.mm, nil // dimensionless Number (ratio)
		case "*":
			return nil, e.errAt(ex.Pos, "operator *: Length * Length is not supported (no Area type)")
		case "%":
			return nil, e.errAt(ex.Pos, "operator %%: Length %% Length is not supported")
		}
	}

	// Length op Number — scalar */÷ always work; +/-/% require the Number
	// side to be a bare numeric literal (which is "untyped" and coerces to
	// Length), so `5 mm + 3` is Length = 8 mm but `5 mm + n` for a Number
	// variable `n` is a dimension error.
	if lIsLen && rIsNum {
		switch ex.Op {
		case "*":
			return length{mm: lLen.mm * rNum}, nil
		case "/":
			if rNum == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return length{mm: lLen.mm / rNum}, nil
		case "+", "-", "%":
			if parser.IsNumericLiteral(ex.Right) {
				rLenPromoted := length{mm: rNum}
				switch ex.Op {
				case "+":
					return length{mm: lLen.mm + rLenPromoted.mm}, nil
				case "-":
					return length{mm: lLen.mm - rLenPromoted.mm}, nil
				case "%":
					if rLenPromoted.mm == 0 {
						return nil, e.errAt(ex.Pos, "modulo by zero")
					}
					return length{mm: math.Mod(lLen.mm, rLenPromoted.mm)}, nil
				}
			}
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Length and Number (use Number(from: x) to convert explicitly)", ex.Op)
		default:
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Length and Number (use Number(from: x) to convert explicitly)", ex.Op)
		}
	}

	// Number op Length — * is commutative; + and - accept a bare numeric
	// literal on the left (`3 + 5 mm` → 8 mm).
	if lIsNum && rIsLen {
		switch ex.Op {
		case "*":
			return length{mm: lNum * rLen.mm}, nil
		case "+", "-":
			if parser.IsNumericLiteral(ex.Left) {
				switch ex.Op {
				case "+":
					return length{mm: lNum + rLen.mm}, nil
				case "-":
					return length{mm: lNum - rLen.mm}, nil
				}
			}
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Number and Length (use Number(from: x) to convert explicitly)", ex.Op)
		default:
			return nil, e.errAt(ex.Pos, "operator %s: incompatible types Number and Length (use Number(from: x) to convert explicitly)", ex.Op)
		}
	}

	// Operator function dispatch — user-defined binary operators on other types
	// (e.g. custom Vec/Matrix arithmetic, or user-added Length*Length → Area).
	if fn, ok := e.opFuncs[opFuncKey{op: ex.Op, leftType: typeName(lv), rightType: typeName(rv)}]; ok && len(fn.Params) >= 2 {
		result, err := e.evalFunction(fn, map[string]value{fn.Params[0].Name: lv, fn.Params[1].Name: rv})
		if err != nil {
			return nil, e.wrapErr(ex.Pos, err)
		}
		return result, nil
	}

	return nil, e.errAt(ex.Pos, "operator %s: incompatible types %s and %s", ex.Op, typeName(lv), typeName(rv))
}

// buildOpFuncs creates the operator function dispatch table from stdlib
// and user program operator function declarations.
func buildOpFuncs(prog loader.Program, currentKey string) map[opFuncKey]*parser.Function {
	m := make(map[opFuncKey]*parser.Function)
	// Register stdlib operator functions
	if stdSrc := prog.Std(); stdSrc != nil {
		for _, fn := range stdSrc.Functions() {
			if !fn.IsOperator {
				continue
			}
			registerOpFunc(m, fn)
		}
	}
	// Register current source operator functions (override stdlib)
	if src := prog.Sources[currentKey]; src != nil {
		for _, fn := range src.Functions() {
			if !fn.IsOperator {
				continue
			}
			registerOpFunc(m, fn)
		}
	}
	return m
}

// registerOpFunc registers a single operator function in the dispatch table.
func registerOpFunc(m map[opFuncKey]*parser.Function, fn *parser.Function) {
	switch len(fn.Params) {
	case 1:
		// Unary operator
		key := opFuncKey{op: fn.Name, leftType: fn.Params[0].Type}
		m[key] = fn
	case 2:
		// Binary operator
		key := opFuncKey{
			op:        fn.Name,
			leftType:  fn.Params[0].Type,
			rightType: fn.Params[1].Type,
		}
		m[key] = fn
	}
}

