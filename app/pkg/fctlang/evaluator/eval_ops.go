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
	lsf, lIsSolid := lv.(*manifold.SolidFuture)
	rsf, rIsSolid := rv.(*manifold.SolidFuture)
	if lIsSolid && rIsSolid {
		var result *manifold.SolidFuture
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
		e.recordStep(opName, ex.Pos, debugRole{"lhs", lsf}, debugRole{"rhs", rsf}, debugRole{"result", result})
		return result, nil
	solidOpFunc:
	}

	// Sketch boolean operations: +, -, &
	lpf, lIsProfile := lv.(*manifold.SketchFuture)
	rpf, rIsProfile := rv.(*manifold.SketchFuture)
	if lIsProfile && rIsProfile {
		var result *manifold.SketchFuture
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
		e.recordStep(opName, ex.Pos, debugRole{"lhs", lpf}, debugRole{"rhs", rpf}, debugRole{"result", result})
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

	// Length / Length → Number (dimensionless ratio)
	if _, lIsScalar := lv.(length); lIsScalar {
		if _, rIsScalar := rv.(length); rIsScalar && ex.Op == "/" {
			lmm := lv.(length).mm
			rmm := rv.(length).mm
			if rmm == 0 {
				return nil, e.errAt(ex.Pos, "division by zero")
			}
			return lmm / rmm, nil
		}
	}

	// Operator function dispatch — fallback before Length arithmetic
	if fn, ok := e.opFuncs[opFuncKey{op: ex.Op, leftType: typeName(lv), rightType: typeName(rv)}]; ok && len(fn.Params) >= 2 {
		result, err := e.evalFunction(fn, map[string]value{fn.Params[0].Name: lv, fn.Params[1].Name: rv})
		if err != nil {
			return nil, e.wrapErr(ex.Pos, err)
		}
		return result, nil
	}

	// Length arithmetic (promotes Number to Length when mixed)
	lmm, lok := asLength(lv)
	rmm, rok := asLength(rv)
	if !lok || !rok {
		return nil, e.errAt(ex.Pos, "operator %s: incompatible types %s and %s", ex.Op, typeName(lv), typeName(rv))
	}

	switch ex.Op {
	case "+":
		return length{mm: lmm + rmm}, nil
	case "-":
		return length{mm: lmm - rmm}, nil
	case "*":
		return float64(lmm * rmm), nil
	case "/":
		if rmm == 0 {
			return nil, e.errAt(ex.Pos, "division by zero")
		}
		return lmm / rmm, nil
	case "%":
		if rmm == 0 {
			return nil, e.errAt(ex.Pos, "modulo by zero")
		}
		return length{mm: math.Mod(lmm, rmm)}, nil
	default:
		return nil, e.errAt(ex.Pos, "unknown operator %q", ex.Op)
	}
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
		key := opFuncKey{op: fn.Name, leftType: opParamTypeName(fn.Params[0].Type)}
		m[key] = fn
	case 2:
		// Binary operator
		key := opFuncKey{
			op:        fn.Name,
			leftType:  opParamTypeName(fn.Params[0].Type),
			rightType: opParamTypeName(fn.Params[1].Type),
		}
		m[key] = fn
	}
}

// opParamTypeName returns the runtime type name for a parameter type string.
// This maps AST type names to the runtime typeName() values used for dispatch.
func opParamTypeName(t string) string {
	// Most type names match directly. Handle the common cases.
	switch t {
	case "Solid", "Sketch", "Length", "Angle", "Number", "Bool", "String",
		"Vec2", "Vec3":
		return t
	default:
		return t // struct types and others pass through
	}
}
