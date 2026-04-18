package main

import (
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"math"
	"sort"

	"facet/app/pkg/manifold"
)

// SourceEntry describes a parsed source file with its text and origin.
type SourceEntry struct {
	Text       string            `json:"text"`
	Kind       parser.SourceKind `json:"kind"`
	ImportPath string            `json:"importPath,omitempty"` // e.g. "facet/gears"; empty for non-library sources
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

func isValidEntryPoint(fn *parser.Function, inferredReturnTypes map[string]string) bool {
	if fn.ReceiverType != "" {
		return false
	}
	if len(fn.Name) == 0 || fn.Name[0] < 'A' || fn.Name[0] > 'Z' {
		return false
	}
	inferred := inferredReturnTypes[fn.Name]
	if fn.Name != "Main" && fn.ReturnType != "Solid" && inferred != "Solid" {
		return false
	}
	for _, p := range fn.Params {
		if p.Default == nil {
			return false
		}
	}
	return true
}

func getEntryPoints(prog loader.Program, inferredReturnTypes map[string]string) []EntryPoint {
	var out []EntryPoint

	collect := func(fn *parser.Function, libPath, libVar string) {
		if !isValidEntryPoint(fn, inferredReturnTypes) {
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
		for _, fn := range src.Functions() {
			collect(fn, srcKey, "")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return entryPointLess(out[i].Name, out[j].Name)
	})

	return out
}

// entryPointLess orders entry points by name, with "Main" pinned to the
// front.  Pulled out so strict weak ordering is unit-testable: when both
// sides are equal (including two "Main"s from different sources) the result
// must be false, which a naive `if a == "Main" { return true }` violates.
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
	switch v := e.(type) {
	case *parser.UnitExpr:
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

func solidBounds(solids []*manifold.Solid) (globalMin, globalMax [3]float64) {
	if len(solids) > 0 {
		globalMin = [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
		globalMax = [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	}
	for _, s := range solids {
		mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
		globalMin[0] = math.Min(globalMin[0], mnX)
		globalMin[1] = math.Min(globalMin[1], mnY)
		globalMin[2] = math.Min(globalMin[2], mnZ)
		globalMax[0] = math.Max(globalMax[0], mxX)
		globalMax[1] = math.Max(globalMax[1], mxY)
		globalMax[2] = math.Max(globalMax[2], mxZ)
	}
	globalMin[0] = sanitizeBBox(globalMin[0])
	globalMin[1] = sanitizeBBox(globalMin[1])
	globalMin[2] = sanitizeBBox(globalMin[2])
	globalMax[0] = sanitizeBBox(globalMax[0])
	globalMax[1] = sanitizeBBox(globalMax[1])
	globalMax[2] = sanitizeBBox(globalMax[2])
	return
}

func sanitizeBBox(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
