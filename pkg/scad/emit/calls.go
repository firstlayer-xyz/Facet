package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// mathBuiltin maps an OpenSCAD math function to its Facet equivalent and the
// ordered Facet parameter names (Facet requires named arguments at call sites).
type mathBuiltin struct {
	facet  string
	params []string
}

var mathBuiltins = map[string]mathBuiltin{
	"sqrt":  {"Sqrt", []string{"n"}},
	"abs":   {"Abs", []string{"a"}},
	"pow":   {"Pow", []string{"base", "exp"}},
	"floor": {"Floor", []string{"n"}},
	"ceil":  {"Ceil", []string{"n"}},
	"round": {"Round", []string{"n"}},
	"len":   {"Size", []string{"of"}},
}

// trigFacet maps OpenSCAD's degree-taking trig functions to their Facet
// equivalents, which take an Angle and return a Number.
var trigFacet = map[string]string{"sin": "Sin", "cos": "Cos", "tan": "Tan"}

// call translates a SCAD function-call expression: a user-defined function, a
// math built-in, or one of the special-shaped built-ins (atan, norm). An
// unknown call is an error, never a silent placeholder.
func (e *Emitter) call(n *ast.Call) string {
	if sym, ok := e.syms[n.Name]; ok && sym.isFunc {
		return n.Name + "(" + e.mapArgs(n.Name, n.Args, paramNames(sym.params)) + ")"
	}
	// Trig bridges OpenSCAD's degree-numbers to Facet's Angle type: Sin/Cos/Tan
	// take an Angle (convert the argument); Asin/Acos/Atan/Atan2 return an Angle,
	// converted back to a degree-number so it flows through numeric arithmetic.
	if facet, ok := trigFacet[n.Name]; ok {
		if a, ok := soleArg(n); ok {
			return facet + "(a: " + e.expr(a, kAngle) + ")"
		}
	}
	switch n.Name {
	case "asin", "acos":
		if a, ok := soleArg(n); ok {
			// asin -> Asin, acos -> Acos.
			return "Number(from: A" + n.Name[1:] + "(n: " + e.expr(a, kNumber) + "))"
		}
	case "exp": // exp(x) == e^x; Facet has no exp(), so Pow(base: E, exp: x).
		if a, ok := soleArg(n); ok {
			return "Pow(base: E, exp: " + e.expr(a, kNumber) + ")"
		}
	case "sign": // sign(x) -> -1 / 0 / +1 (Facet has no sign(); a pure ternary).
		if a, ok := soleArg(n); ok {
			x := e.operand(a, kNumber)
			return "(" + x + " > 0 ? 1 : (" + x + " < 0 ? -1 : 0))"
		}
	case "atan": // atan(x) == Atan2(y: x, x: 1)
		if a, ok := soleArg(n); ok {
			return "Number(from: Atan2(y: " + e.expr(a, kNumber) + ", x: 1))"
		}
	case "atan2":
		if len(n.Args) == 2 {
			return "Number(from: Atan2(" + e.mapArgs("atan2", n.Args, []string{"y", "x"}) + "))"
		}
	case "min", "max":
		// OpenSCAD min/max take either a single list — min([a,b,c]) — or two-or-more
		// scalars — min(a, b, c). Facet's Min/Max are strictly binary, so a list
		// reduces with fold and multiple scalars left-fold into nested calls.
		fn := "Min"
		if n.Name == "max" {
			fn = "Max"
		}
		if len(n.Args) == 1 && n.Args[0].Name == "" {
			// Parenthesized so it stays a single operand in a surrounding expression
			// (e.g. min([…]) + 4).
			return "(fold acc, x " + e.operand(n.Args[0].Value, kNumber) +
				" { yield " + fn + "(a: acc, b: x) })"
		}
		if len(n.Args) >= 2 {
			out := e.expr(n.Args[0].Value, kNumber)
			for _, a := range n.Args[1:] {
				if a.Name != "" {
					return e.errf(n.Pos(), "%s takes positional arguments", n.Name)
				}
				out = fn + "(a: " + out + ", b: " + e.expr(a.Value, kNumber) + ")"
			}
			return out
		}
		return e.errf(n.Pos(), "%s expects a list or two-or-more values", n.Name)
	case "norm": // norm(v) == Sqrt(Dot(v, v)) (Facet has no vector-length built-in)
		if len(n.Args) == 1 && n.Args[0].Name == "" {
			v := e.expr(n.Args[0].Value, kNumber)
			return "Sqrt(n: Dot(a: " + v + ", b: " + v + "))"
		}
	case "concat": // concat(a, b, ...) of lists == Facet a + b + ... (list +)
		if len(n.Args) >= 1 {
			parts := make([]string, len(n.Args))
			for i, a := range n.Args {
				parts[i] = e.operand(a.Value, kNumber)
			}
			return strings.Join(parts, " + ")
		}
	case "str":
		// str(a, b, …) concatenates the string forms of its arguments. Facet's
		// String(a:) converts any value to its text; string concatenation is `+`.
		// A bare string literal is already a String, so it needs no conversion.
		if len(n.Args) == 0 {
			return `""`
		}
		parts := make([]string, len(n.Args))
		for i, a := range n.Args {
			if _, isStr := a.Value.(*ast.Str); isStr {
				parts[i] = e.expr(a.Value, kNumber)
			} else {
				parts[i] = "String(a: " + e.expr(a.Value, kNumber) + ")"
			}
		}
		return strings.Join(parts, " + ")
	case "search":
		// search(match_value, vector) returns the LIST of indices where the value
		// occurs, so it maps to IndicesOf(arr: vector, value: match_value) (the
		// arguments swap). Using the list form (not scalar IndexOf) keeps the
		// idiomatic len(search(...)) / search(...)[0] working. The
		// num_returns_per_match / index_col_num forms have no equivalent and are
		// rejected rather than mistranslated.
		if len(n.Args) == 2 && n.Args[0].Name == "" && n.Args[1].Name == "" {
			return "IndicesOf(arr: " + e.expr(n.Args[1].Value, kNumber) +
				", value: " + e.expr(n.Args[0].Value, kNumber) + ")"
		}
		return e.errf(n.Pos(), "only the two-argument search(value, list) form is supported")
	case "lookup":
		// lookup(key, table) linearly interpolates value over a sorted
		// [[key, value], …] table (clamped at the ends), via the injected
		// scad_lookup helper.
		if len(n.Args) == 2 && n.Args[0].Name == "" && n.Args[1].Name == "" {
			e.usesLookup = true
			return "scad_lookup(key: " + e.expr(n.Args[0].Value, kNumber) +
				", table: " + e.expr(n.Args[1].Value, kNumber) + ")"
		}
		return e.errf(n.Pos(), "lookup expects (key, table)")
	}
	if b, ok := mathBuiltins[n.Name]; ok {
		return b.facet + "(" + e.mapArgs(n.Name, n.Args, b.params) + ")"
	}
	return e.errf(n.Pos(), "function '%s'", n.Name)
}

// mapArgs renders a call's arguments as Facet named arguments, binding each
// positional argument to the corresponding parameter name. Excess positional
// arguments are an error.
func (e *Emitter) mapArgs(callName string, args []ast.Arg, names []string) string {
	parts := make([]string, 0, len(args))
	pos := 0
	for _, a := range args {
		name := a.Name
		if name == "" {
			if pos >= len(names) {
				return e.errf(args[0].Value.Pos(), "%s: too many positional arguments", callName)
			}
			name = names[pos]
			pos++
		}
		parts = append(parts, name+": "+e.expr(a.Value, kNumber))
	}
	return strings.Join(parts, ", ")
}

// soleArg returns the value of a call's single argument (positional or named),
// or false when the call does not have exactly one argument.
func soleArg(n *ast.Call) (ast.Expr, bool) {
	if len(n.Args) == 1 {
		return n.Args[0].Value, true
	}
	return nil, false
}

// paramNames returns the ordered parameter names of a definition.
func paramNames(params []ast.Param) []string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return names
}
