package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Constraint validation and overrides
// ---------------------------------------------------------------------------

// convertOverrides converts raw JSON override values into typed evaluator values
// by inspecting global variable and entry point function parameter constraints.
func convertOverrides(prog loader.Program, currentKey string, raw map[string]interface{}, entryPoint string) map[string]value {
	result := make(map[string]value, len(raw))

	// Convert global overrides
	src := prog.Sources[currentKey]
	if src == nil {
		return result
	}
	for _, g := range src.Globals {
		if g.Constraint == nil {
			continue
		}
		rv, ok := raw[g.Name]
		if !ok {
			continue
		}
		if v, ok := convertOneOverride(g.Constraint, g.Value, rv); ok {
			result[g.Name] = v
		}
	}

	// Convert entry point function parameter overrides
	for _, fn := range src.Functions {
		if fn.Name != entryPoint {
			continue
		}
		for _, p := range fn.Params {
			rv, ok := raw[p.Name]
			if !ok {
				continue
			}
			if p.Constraint != nil {
				if v, ok := convertOneOverride(p.Constraint, p.Default, rv); ok {
					result[p.Name] = v
				}
			} else {
				if v, ok := convertByType(p.Type, rv); ok {
					result[p.Name] = v
				}
			}
		}
		break
	}

	return result
}

// convertByType converts a raw JSON value to a typed value based on the parameter's declared type.
// Length values are assumed to be in mm; Angle values in degrees.
func convertByType(typeName string, rv interface{}) (value, bool) {
	switch typeName {
	case "Length":
		if n, ok := toFloat64(rv); ok {
			return length{mm: n}, true
		}
	case "Angle":
		if n, ok := toFloat64(rv); ok {
			return angle{deg: n}, true
		}
	default:
		switch v := rv.(type) {
		case float64:
			return v, true
		case bool:
			return v, true
		case string:
			return v, true
		}
	}
	return nil, false
}

// convertOneOverride converts a single raw JSON value using the constraint and default expression.
func convertOneOverride(constraint parser.Expr, defaultExpr parser.Expr, rv interface{}) (value, bool) {
	switch c := constraint.(type) {
	case *parser.ConstrainedRange:
		if n, ok := toFloat64(rv); ok {
			if _, isAngle := parser.AngleFactors[c.Unit]; isAngle {
				factor := parser.AngleFactors[c.Unit]
				return angle{deg: n * factor}, true
			} else if factor, isUnit := parser.UnitFactors[c.Unit]; isUnit {
				return length{mm: n * factor}, true
			}
			return n, true
		}
	case *parser.RangeExpr:
		if n, ok := toFloat64(rv); ok {
			if v, ok := defaultExpr.(*parser.UnitExpr); ok {
				if v.IsAngle {
					return angle{deg: n * v.Factor}, true
				}
				return length{mm: n * v.Factor}, true
			}
			return n, true
		}
	case *parser.ArrayLitExpr:
		if s, ok := rv.(string); ok {
			return s, true
		} else if n, ok := toFloat64(rv); ok {
			return n, true
		} else if b, ok := rv.(bool); ok {
			return b, true
		}
	}
	return nil, false
}

// toFloat64 converts a JSON number (which Go's json.Unmarshal decodes as float64) to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// validateConstraint validates that a runtime value satisfies a constraint expression.
// name is used only for error messages. constraint must be a *ConstrainedRange, *RangeExpr, or *ArrayLitExpr.
func (e *evaluator) validateConstraint(name string, constraint parser.Expr, val value, locals map[string]value) error {
	val = unwrap(val)
	switch c := constraint.(type) {
	case *parser.ConstrainedRange:
		// Evaluate the range bounds as plain numbers, then apply unit
		sv, err := e.evalExpr(c.Range.Start, locals)
		if err != nil {
			return err
		}
		ev, err := e.evalExpr(c.Range.End, locals)
		if err != nil {
			return err
		}
		startN, ok1 := asNumber(sv)
		endN, ok2 := asNumber(ev)
		if !ok1 || !ok2 {
			return nil // can't validate non-numeric bounds
		}

		// Extract the numeric value from val in the same unit space
		var valN float64
		if _, isAngle := parser.AngleFactors[c.Unit]; isAngle {
			factor := parser.AngleFactors[c.Unit]
			startN *= factor
			endN *= factor
			if a, ok := val.(angle); ok {
				valN = a.deg
			} else if n, ok := val.(float64); ok {
				valN = n
			} else {
				return fmt.Errorf("%s: expected Angle, got %s", name, typeName(val))
			}
		} else if factor, isUnit := parser.UnitFactors[c.Unit]; isUnit {
			startN *= factor
			endN *= factor
			if s, ok := val.(length); ok {
				valN = s.mm
			} else if n, ok := val.(float64); ok {
				valN = n
			} else {
				return fmt.Errorf("%s: expected Length, got %s", name, typeName(val))
			}
		} else {
			return nil // unknown unit, skip validation
		}

		if c.Range.Exclusive {
			if valN < startN || valN >= endN {
				return fmt.Errorf("%s: value out of range [%g:%g) %s", name, asNumber2(sv), asNumber2(ev), c.Unit)
			}
		} else {
			if valN < startN || valN > endN {
				return fmt.Errorf("%s: value out of range [%g:%g] %s", name, asNumber2(sv), asNumber2(ev), c.Unit)
			}
		}

	case *parser.RangeExpr:
		sv, err := e.evalExpr(c.Start, locals)
		if err != nil {
			return err
		}
		ev, err := e.evalExpr(c.End, locals)
		if err != nil {
			return err
		}

		// Handle typed ranges (Length bounds)
		if ss, ok := sv.(length); ok {
			es, eok := ev.(length)
			if !eok {
				return nil
			}
			var valMM float64
			if s, ok := val.(length); ok {
				valMM = s.mm
			} else if n, ok := val.(float64); ok {
				valMM = n
			} else {
				return fmt.Errorf("%s: expected Length, got %s", name, typeName(val))
			}
			if c.Exclusive {
				if valMM < ss.mm || valMM >= es.mm {
					return fmt.Errorf("%s: value out of range", name)
				}
			} else {
				if valMM < ss.mm || valMM > es.mm {
					return fmt.Errorf("%s: value out of range", name)
				}
			}
			return nil
		}

		// Plain number range
		startF, ok1 := asNumber(sv)
		endF, ok2 := asNumber(ev)
		if !ok1 || !ok2 {
			return nil
		}
		valF, ok := asNumber(val)
		if !ok {
			return fmt.Errorf("%s: expected Number, got %s", name, typeName(val))
		}

		// Validate step if present
		if c.Step != nil {
			stepV, err := e.evalExpr(c.Step, locals)
			if err != nil {
				return err
			}
			stepF, ok := asNumber(stepV)
			if !ok || stepF == 0 {
				return nil // can't validate
			}
			// Check value is on a step boundary (with tolerance)
			offset := valF - startF
			remainder := math.Mod(offset, stepF)
			if math.Abs(remainder) > 1e-9 && math.Abs(remainder-stepF) > 1e-9 {
				return fmt.Errorf("%s: value %g is not on step boundary (step=%g)", name, valF, stepF)
			}
		}

		if c.Exclusive {
			if valF < startF || valF >= endF {
				return fmt.Errorf("%s: value %g out of range [%g:%g)", name, valF, startF, endF)
			}
		} else {
			if valF < startF || valF > endF {
				return fmt.Errorf("%s: value %g out of range [%g:%g]", name, valF, startF, endF)
			}
		}

	case *parser.ArrayLitExpr:
		if len(c.Elems) == 0 {
			return nil // free-form: any value allowed
		}
		// Enum: check value is in the list
		for _, elem := range c.Elems {
			ev, err := e.evalExpr(elem, locals)
			if err != nil {
				return err
			}
			if valuesEqual(val, ev) {
				return nil
			}
		}
		return fmt.Errorf("%s: value not in allowed set", name)
	}
	return nil
}

// asNumber2 extracts a number for display (always returns something).
func asNumber2(v value) float64 {
	if n, ok := asNumber(v); ok {
		return n
	}
	return 0
}

// valuesEqual checks if two runtime values are equal.
func valuesEqual(a, b value) bool {
	a, b = unwrap(a), unwrap(b)
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return floatEqual(av, bv)
		}
	case length:
		if bv, ok := b.(length); ok {
			return floatEqual(av.mm, bv.mm)
		}
	case angle:
		if bv, ok := b.(angle); ok {
			return floatEqual(av.deg, bv.deg)
		}
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	}
	return false
}
