// Package entrypoints derives the runnable entry points of a Facet program —
// the capitalized, fully-constrained functions a viewer can render — together
// with their parameter metadata for the preview panel. It is shared by the
// desktop app and the browser (wasm) preview so both detect entries (including
// Animation entries) and present parameters identically.
package entrypoints

import (
	"sort"

	"facet/pkg/fctlang/doc"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// ParamConstraint describes the allowed values for a parameter.
type ParamConstraint struct {
	Kind      string        `json:"kind"`                // "range", "enum", or "free"
	Min       interface{}   `json:"min,omitempty"`       // numeric min (in display units)
	Max       interface{}   `json:"max,omitempty"`       // numeric max (in display units)
	Step      interface{}   `json:"step,omitempty"`      // step size
	Exclusive bool          `json:"exclusive,omitempty"` // true if upper bound is exclusive
	Values    []interface{} `json:"values,omitempty"`    // for enum constraints
}

// ParamEntry describes a single function parameter for the preview panel.
type ParamEntry struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	HasDefault bool             `json:"hasDefault"`
	Default    interface{}      `json:"default"`
	Unit       string           `json:"unit,omitempty"`
	Constraint *ParamConstraint `json:"constraint,omitempty"`
}

// EntryPoint describes a runnable entry point function — a capitalized,
// fully-constrained function that returns Solid or Animation.
type EntryPoint struct {
	Name      string       `json:"name"`
	Signature string       `json:"signature"`
	Params    []ParamEntry `json:"params"`
	LibPath   string       `json:"libPath"` // "" = main file; "facet/gears" etc. = library
	LibVar    string       `json:"libVar"`  // variable name the lib is bound to in source
	Doc       string       `json:"doc"`
	Animated  bool         `json:"animated,omitempty"` // true when the entry returns Animation
}

// IsValid reports whether fn is a runnable entry point: a capitalized,
// receiver-less function whose every parameter has a default, returning a
// renderable type. "Main" is exempt from the return-type check so the default
// entry is always offered (a wrongly-typed Main then surfaces as an eval error
// rather than silently vanishing from the list).
func IsValid(fn *parser.Function, inferredReturnTypes map[string]string) bool {
	if fn.ReceiverType != "" {
		return false
	}
	if len(fn.Name) == 0 || fn.Name[0] < 'A' || fn.Name[0] > 'Z' {
		return false
	}
	inferred := inferredReturnTypes[fn.Name]
	isAnim := fn.ReturnType == "Animation" || inferred == "Animation"
	if fn.Name != "Main" && fn.ReturnType != "Solid" && inferred != "Solid" && !isAnim {
		return false
	}
	for _, p := range fn.Params {
		if p.Default == nil {
			return false
		}
	}
	return true
}

// Build returns every entry point in prog, with parameter defaults and
// constraint bounds converted to display units (e.g. cm rather than canonical
// mm) so a preview UI can show and round-trip them directly. Entries are sorted
// with "Main" first, then alphabetically.
func Build(prog loader.Program, inferredReturnTypes map[string]string) []EntryPoint {
	var out []EntryPoint

	collect := func(fn *parser.Function, libPath, libVar string) {
		if !IsValid(fn, inferredReturnTypes) {
			return
		}
		params := make([]ParamEntry, 0, len(fn.Params))
		for _, p := range fn.Params {
			pe := ParamEntry{
				Name:       p.Name,
				Type:       p.Type,
				HasDefault: p.Default != nil,
			}
			if p.Default != nil {
				pe.Default, _ = literalValue(p.Default)
			}
			pe.Constraint = extractParamConstraint(p.Constraint)
			if pe.Constraint != nil && pe.Unit == "" {
				pe.Unit = constraintUnit(p.Constraint)
			}
			if pe.Unit == "" {
				pe.Unit = paramDefaultUnit(p.Default)
			}
			// RangeExpr bounds with explicit units (e.g. [10 cm:30 cm]) are
			// converted to canonical mm/deg by literalValue. Convert them back
			// to display units so the slider shows the right numbers.
			if _, isRangeExpr := p.Constraint.(*parser.RangeExpr); isRangeExpr && pe.Unit != "" {
				f := displayUnitFactor(pe.Unit)
				if f != 1 && pe.Constraint != nil {
					pe.Constraint.Min = divideIfFloat(pe.Constraint.Min, f)
					pe.Constraint.Max = divideIfFloat(pe.Constraint.Max, f)
					pe.Constraint.Step = divideIfFloat(pe.Constraint.Step, f)
				}
			}
			// Default values from UnitExpr are also canonical — convert to display units.
			if pe.Unit != "" {
				f := displayUnitFactor(pe.Unit)
				if f != 1 {
					pe.Default = divideIfFloat(pe.Default, f)
				}
			}
			params = append(params, pe)
		}
		out = append(out, EntryPoint{
			Name:      fn.Name,
			Signature: doc.FormatSignature(fn),
			Params:    params,
			LibPath:   libPath,
			LibVar:    libVar,
			Doc:       parser.DocComment(fn.Comments),
			Animated:  fn.ReturnType == "Animation" || inferredReturnTypes[fn.Name] == "Animation",
		})
	}

	for srcKey, src := range prog.Sources {
		for _, fn := range src.Functions() {
			collect(fn, srcKey, "")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return entryPointLess(out[i].Name, out[j].Name)
	})

	return out
}

// entryPointLess orders entry points by name, with "Main" pinned to the front.
// Pulled out so strict weak ordering is unit-testable: when both sides are
// equal (including two "Main"s from different sources) the result must be
// false, which a naive `if a == "Main" { return true }` violates.
func entryPointLess(a, b string) bool {
	if a == b {
		return false
	}
	if a == "Main" {
		return true
	}
	if b == "Main" {
		return false
	}
	return a < b
}

func extractParamConstraint(c parser.Expr) *ParamConstraint {
	switch c := c.(type) {
	case *parser.ConstrainedRange:
		pc := &ParamConstraint{Kind: "range", Exclusive: c.Range.Exclusive}
		if min, ok := literalNumber(c.Range.Start); ok {
			pc.Min = min
		}
		if max, ok := literalNumber(c.Range.End); ok {
			pc.Max = max
		}
		if c.Range.Step != nil {
			if step, ok := literalNumber(c.Range.Step); ok {
				pc.Step = step
			}
		}
		return pc
	case *parser.RangeExpr:
		pc := &ParamConstraint{Kind: "range", Exclusive: c.Exclusive}
		if min, ok := literalValue(c.Start); ok {
			pc.Min = min
		}
		if max, ok := literalValue(c.End); ok {
			pc.Max = max
		}
		if c.Step != nil {
			if step, ok := literalValue(c.Step); ok {
				pc.Step = step
			}
		}
		return pc
	case *parser.ArrayLitExpr:
		if len(c.Elems) == 0 {
			return &ParamConstraint{Kind: "free"}
		}
		pc := &ParamConstraint{Kind: "enum"}
		for _, elem := range c.Elems {
			if v, ok := literalValue(elem); ok {
				pc.Values = append(pc.Values, v)
			}
		}
		return pc
	}
	return nil
}

func constraintUnit(c parser.Expr) string {
	switch c := c.(type) {
	case *parser.ConstrainedRange:
		return c.Unit
	case *parser.RangeExpr:
		if u := exprUnit(c.End); u != "" {
			return u
		}
		return exprUnit(c.Start)
	}
	return ""
}

func paramDefaultUnit(e parser.Expr) string {
	if e == nil {
		return ""
	}
	if v, ok := e.(*parser.UnitExpr); ok {
		return v.Unit
	}
	return ""
}

func literalNumber(e parser.Expr) (float64, bool) {
	switch v := e.(type) {
	case *parser.NumberLit:
		return v.Value, true
	case *parser.UnitExpr:
		if num, ok := v.Expr.(*parser.NumberLit); ok {
			return num.Value * v.Factor, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func literalValue(e parser.Expr) (interface{}, bool) {
	switch v := e.(type) {
	case *parser.NumberLit:
		return v.Value, true
	case *parser.UnitExpr:
		if num, ok := v.Expr.(*parser.NumberLit); ok {
			return num.Value * v.Factor, true
		}
		return nil, false
	case *parser.StringLit:
		return v.Value, true
	case *parser.BoolLit:
		return v.Value, true
	default:
		return nil, false
	}
}

func exprUnit(e parser.Expr) string {
	if u, ok := e.(*parser.UnitExpr); ok {
		return u.Unit
	}
	return ""
}

// displayUnitFactor returns the canonical-unit factor for a display unit name
// (e.g. "cm" → 10 because 1 cm = 10 mm). Returns 1 for unknown units.
func displayUnitFactor(unit string) float64 {
	if f, ok := parser.UnitFactors[unit]; ok {
		return f
	}
	if f, ok := parser.AngleFactors[unit]; ok {
		return f
	}
	return 1
}

// divideIfFloat divides v by f if v is a float64; otherwise returns v unchanged.
func divideIfFloat(v interface{}, f float64) interface{} {
	if n, ok := v.(float64); ok {
		return n / f
	}
	return v
}
