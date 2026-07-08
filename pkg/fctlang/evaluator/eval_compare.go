package evaluator

import (
	"facet/pkg/fctlang/parser"
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
		// Bool vs Array (and Array vs Bool below) used to coerce the array
		// to a "truthy" Bool via len > 0. That implicit conversion was a
		// footgun — `true == [1, 2, 3]` would silently succeed — and is
		// also rejected by the checker for typed code, so the runtime path
		// was effectively a backdoor for `Any`-typed values. Removed: cross-
		// type Bool/Array comparisons fall through to the incompatible-type
		// error like every other unrelated pair.
	case array:
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
	case *optionalVal:
		// Two Nones are equal regardless of inner type; two Somes recurse
		// on their inner values. An Optional also compares against a DEFINITE
		// value (the checker accepts `opt == 5` since T? is compatible with
		// T): None is never equal to a definite, Some(x) compares x.
		if op == "==" || op == "!=" {
			eq := false
			if r, rok := rv.(*optionalVal); rok {
				switch {
				case !l.present && !r.present:
					eq = true
				case l.present && r.present:
					eq = valuesEqual(l.inner, r.inner)
				}
			} else {
				eq = l.present && valuesEqual(l.inner, rv)
			}
			if op == "==" {
				return eq, nil
			}
			return !eq, nil
		}
	}
	// Mirror: definite == Optional (the Optional case above only fires when
	// the LEFT side is Optional).
	if r, rok := rv.(*optionalVal); rok && (op == "==" || op == "!=") {
		eq := r.present && valuesEqual(lv, r.inner)
		if op == "==" {
			return eq, nil
		}
		return !eq, nil
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

	// All six comparisons share floatEqual's epsilon so trichotomy holds:
	// for any pair, exactly one of <, ==, > is true, and a == b implies both
	// a <= b and a >= b. With exact IEEE ordering next to epsilon equality,
	// 0.1 + 0.2 was simultaneously == 0.3 and > 0.3 — an `if x == limit` and
	// an `if x > limit` branch could both fire on the same value.
	eq := floatEqual(lf, rf)
	switch op {
	case "<":
		return lf < rf && !eq, nil
	case ">":
		return lf > rf && !eq, nil
	case "<=":
		return lf < rf || eq, nil
	case ">=":
		return lf > rf || eq, nil
	case "==":
		return eq, nil
	case "!=":
		return !eq, nil
	default:
		return nil, e.errAt(pos, "unknown comparison operator %q", op)
	}
}

// floatEqual compares two float64 values using absolute epsilon for
// near-zero values and relative epsilon otherwise. This handles both
// unit-conversion drift (e.g. 1 fathom == 6 ft) and trig results
// near zero (e.g. Cos(Acos(0)) == 0).
//
// 1e-12 is the threshold for both the absolute (near-zero) and the relative
// case. It's tight enough to flag a genuine mismatch in any realistic CAD
// model (mm precision is 12-15 significant digits below a 1-metre scale),
// and loose enough to absorb the standard double-precision IEEE-754 rounding
// from unit conversions and trig roundtrips. If accumulated trig error from
// a long fold ever pushes past this, the right fix is usually to keep one
// canonical unit (Angle / Length) rather than to widen the threshold.
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
