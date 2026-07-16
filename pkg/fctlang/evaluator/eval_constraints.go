package evaluator

import (
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Constraint validation and overrides
// ---------------------------------------------------------------------------

// convertOverrides converts raw JSON override values into typed evaluator values
// by inspecting global variable and entry point function parameter constraints.
// An override for a current variable/parameter whose value can't be converted to
// the expected type is a hard error rather than being silently dropped — a bad
// override would otherwise vanish and the default render, masking the mismatch.
// (Overrides for variables/parameters that no longer exist are simply ignored:
// this only iterates the current globals/entry params, never the raw keyset.)
func convertOverrides(prog loader.Program, currentKey string, raw map[string]interface{}, entryPoint string) (map[string]value, error) {
	result := make(map[string]value, len(raw))

	// Convert global overrides
	src := prog.Sources[currentKey]
	if src == nil {
		return result, nil
	}
	for _, g := range src.Globals() {
		if g.Constraint == nil {
			continue
		}
		rv, ok := raw[g.Name]
		if !ok {
			continue
		}
		v, ok := convertOneOverride(g.Constraint, g.Value, rv)
		if !ok {
			return nil, fmt.Errorf("override for %q (%v) could not be converted to its expected type", g.Name, rv)
		}
		result[g.Name] = v
	}

	// Convert entry point function parameter overrides
	for _, fn := range src.Functions() {
		if fn.Name != entryPoint {
			continue
		}
		for _, p := range fn.Params {
			rv, ok := raw[p.Name]
			if !ok {
				continue
			}
			var v value
			if p.Constraint != nil {
				v, ok = convertOneOverride(p.Constraint, p.Default, rv)
			} else {
				v, ok = convertByType(p.Type, rv)
			}
			if !ok {
				return nil, fmt.Errorf("override for parameter %q (%v) is not a valid %s", p.Name, rv, p.Type)
			}
			result[p.Name] = v
		}
		break
	}

	return result, nil
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
			return fmt.Errorf("%s: constraint range bounds must evaluate to numbers", name)
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
			return fmt.Errorf("%s: constraint uses an unrecognized unit %q", name, c.Unit)
		}

		if c.Range.IsExclusive() {
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
				return fmt.Errorf("%s: constraint range bounds must both be Length", name)
			}
			var valMM float64
			if s, ok := val.(length); ok {
				valMM = s.mm
			} else if n, ok := val.(float64); ok {
				valMM = n
			} else {
				return fmt.Errorf("%s: expected Length, got %s", name, typeName(val))
			}
			if c.IsExclusive() {
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
			return fmt.Errorf("%s: constraint range bounds must evaluate to numbers", name)
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
			if !ok {
				return fmt.Errorf("%s: constraint step must evaluate to a number", name)
			}
			if stepF == 0 {
				return fmt.Errorf("%s: constraint step must be non-zero", name)
			}
			// Check value is on a step boundary (with tolerance)
			offset := valF - startF
			remainder := math.Mod(offset, stepF)
			if math.Abs(remainder) > 1e-9 && math.Abs(remainder-stepF) > 1e-9 {
				return fmt.Errorf("%s: value %g is not on step boundary (step=%g)", name, valF, stepF)
			}
		}

		if c.IsExclusive() {
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

// valuesEqual checks if two runtime values are equal. It is the single
// structural-equality predicate behind IndexOf/IndicesOf, `in`-set
// constraints, and Optional comparison — and it must agree with the `==`
// operator: Length↔Number compares like evalCompare does (5 mm == 5), structs
// compare field-wise, arrays element-wise.
func valuesEqual(a, b value) bool {
	a, b = unwrap(a), unwrap(b)
	switch av := a.(type) {
	case float64:
		switch bv := b.(type) {
		case float64:
			return floatEqual(av, bv)
		case length:
			// Mirror evalCompare: a bare Number compares against a Length's mm.
			return floatEqual(av, bv.mm)
		}
	case length:
		switch bv := b.(type) {
		case length:
			return floatEqual(av.mm, bv.mm)
		case float64:
			return floatEqual(av.mm, bv)
		}
	case array:
		if bv, ok := b.(array); ok {
			if len(av.elems) != len(bv.elems) {
				return false
			}
			for i := range av.elems {
				if !valuesEqual(av.elems[i], bv.elems[i]) {
					return false
				}
			}
			return true
		}
	case *structVal:
		if bv, ok := b.(*structVal); ok {
			if av.typeName != bv.typeName || len(av.fields) != len(bv.fields) {
				return false
			}
			for k, fv := range av.fields {
				bf, ok := bv.fields[k]
				if !ok || !valuesEqual(fv, bf) {
					return false
				}
			}
			return true
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
	case *optionalVal:
		if bv, ok := b.(*optionalVal); ok {
			// Two Optionals are equal if both absent, or both present
			// with equal inner values.
			if !av.present && !bv.present {
				return true
			}
			if av.present && bv.present {
				return valuesEqual(av.inner, bv.inner)
			}
			return false
		}
	}
	return false
}
