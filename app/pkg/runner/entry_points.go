package runner

import (
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"sort"
)

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
// fully-constrained function that returns Solid.
type EntryPoint struct {
	Name      string       `json:"name"`
	Signature string       `json:"signature"`
	Params    []ParamEntry `json:"params"`
	LibPath   string       `json:"libPath"` // "" = main file; "facet/gears" etc. = library
	LibVar    string       `json:"libVar"`  // variable name the lib is bound to in source
	Doc       string       `json:"doc"`
}

// ParamConstraint describes the allowed values for a parameter.
type ParamConstraint struct {
	Kind      string        `json:"kind"`               // "range", "enum", or "free"
	Min       interface{}   `json:"min,omitempty"`       // numeric min (in display units)
	Max       interface{}   `json:"max,omitempty"`       // numeric max (in display units)
	Step      interface{}   `json:"step,omitempty"`      // step size
	Exclusive bool          `json:"exclusive,omitempty"` // true if upper bound is exclusive
	Values    []interface{} `json:"values,omitempty"`    // for enum constraints
}

// GetEntryPoints returns all entry points (capitalized, fully-constrained,
// Solid-returning functions) from the main program and resolved libraries.
// inferredReturnTypes provides inferred return types from the checker (fn name -> type name).
// ResolveLibraries must have been called on prog before this.
func GetEntryPoints(prog loader.Program, inferredReturnTypes map[string]string) []EntryPoint {
	var out []EntryPoint

	collect := func(fn *parser.Function, libPath, libVar string) {
		if fn.ReceiverType != "" {
			return
		}
		// Main is always treated as returning Solid; all others require explicit or inferred Solid return.
		inferred := inferredReturnTypes[fn.Name]
		if fn.Name != "Main" && fn.ReturnType != "Solid" && inferred != "Solid" {
			return
		}
		if len(fn.Name) == 0 || fn.Name[0] < 'A' || fn.Name[0] > 'Z' {
			return // only include exported (capital letter) functions
		}
		// Entry points must be fully constrained — every param needs a default.
		fullyConstrained := true
		for _, p := range fn.Params {
			if p.Default == nil {
				fullyConstrained = false
				break
			}
		}
		if !fullyConstrained {
			return
		}
		params := make([]ParamEntry, 0, len(fn.Params)) // never nil -> always [] in JSON
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
			params = append(params, pe)
		}
		out = append(out, EntryPoint{
			Name:      fn.Name,
			Signature: doc.FormatSignature(fn),
			Params:    params,
			LibPath:   libPath,
			LibVar:    libVar,
			Doc:       parser.DocComment(fn.Comments),
		})
	}

	for srcKey, src := range prog.Sources {
		for _, fn := range src.Functions {
			collect(fn, srcKey, "")
		}
	}

	// Sort: Main first, then alphabetical.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Name, out[j].Name
		if a == "Main" {
			return true
		}
		if b == "Main" {
			return false
		}
		return a < b
	})

	return out
}

// extractParamConstraint converts a constraint Expr into a ParamConstraint.
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

// constraintUnit extracts the display unit from a constraint expression.
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

// paramDefaultUnit extracts the display unit from a default-value expression.
func paramDefaultUnit(e parser.Expr) string {
	if e == nil {
		return ""
	}
	switch v := e.(type) {
	case *parser.UnitExpr:
		return v.Unit
	}
	return ""
}

// literalNumber extracts a plain numeric value from a literal expression.
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

// literalValue extracts a JSON-serializable value from a literal expression.
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

// exprUnit returns the unit string if the expression is a UnitExpr, or "".
func exprUnit(e parser.Expr) string {
	if u, ok := e.(*parser.UnitExpr); ok {
		return u.Unit
	}
	return ""
}
