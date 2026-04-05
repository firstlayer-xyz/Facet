package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"math"
)

func (e *evaluator) evalCompare(op string, lv, rv value, pos parser.Pos) (value, error) {
	// Extract comparable numeric values
	var lf, rf float64
	var ok bool

	switch l := lv.(type) {
	case length:
		r, rok := rv.(length)
		if !rok {
			// also allow bare number vs length
			if rn, rnok := rv.(float64); rnok {
				lf, rf, ok = l.mm, rn, true
			}
		} else {
			lf, rf, ok = l.mm, r.mm, true
		}
	case float64:
		switch r := rv.(type) {
		case float64:
			lf, rf, ok = l, r, true
		case length:
			lf, rf, ok = l, r.mm, true
		}
	case angle:
		if r, rok := rv.(angle); rok {
			lf, rf, ok = l.deg, r.deg, true
		}
	case bool:
		if r, rok := rv.(bool); rok && (op == "==" || op == "!=") {
			if op == "==" {
				return l == r, nil
			}
			return l != r, nil
		}
		// Bool vs Array: empty array is falsy, non-empty is truthy
		if r, rok := rv.(array); rok && (op == "==" || op == "!=") {
			truthy := len(r.elems) > 0
			if op == "==" {
				return l == truthy, nil
			}
			return l != truthy, nil
		}
	case array:
		// Array vs Bool: empty array is falsy, non-empty is truthy
		if r, rok := rv.(bool); rok && (op == "==" || op == "!=") {
			truthy := len(l.elems) > 0
			if op == "==" {
				return truthy == r, nil
			}
			return truthy != r, nil
		}
	case string:
		if r, rok := rv.(string); rok {
			switch op {
			case "==":
				return l == r, nil
			case "!=":
				return l != r, nil
			case "<":
				return l < r, nil
			case ">":
				return l > r, nil
			case "<=":
				return l <= r, nil
			case ">=":
				return l >= r, nil
			}
		}
	}

	if !ok {
		// Try operator function dispatch for comparisons
		if fn, found := e.opFuncs[opFuncKey{op: op, leftType: typeName(lv), rightType: typeName(rv)}]; found {
			result, err := e.evalFunction(fn, map[string]value{fn.Params[0].Name: lv, fn.Params[1].Name: rv})
			if err != nil {
				return nil, e.wrapErr(pos, err)
			}
			return result, nil
		}
		return nil, e.errAt(pos, "operator %s: incompatible types %s and %s", op, typeName(lv), typeName(rv))
	}

	switch op {
	case "<":
		return lf < rf, nil
	case ">":
		return lf > rf, nil
	case "<=":
		return lf <= rf, nil
	case ">=":
		return lf >= rf, nil
	case "==":
		return floatEqual(lf, rf), nil
	case "!=":
		return !floatEqual(lf, rf), nil
	default:
		return nil, e.errAt(pos, "unknown comparison operator %q", op)
	}
}

// floatEqual compares two float64 values using absolute epsilon for
// near-zero values and relative epsilon otherwise. This handles both
// unit-conversion drift (e.g. 1 fathom == 6 ft) and trig results
// near zero (e.g. Cos(Acos(0)) == 0).
func floatEqual(a, b float64) bool {
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	if diff < 1e-12 {
		return true
	}
	largest := math.Max(math.Abs(a), math.Abs(b))
	return diff/largest < 1e-12
}

