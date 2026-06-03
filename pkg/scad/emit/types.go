package emit

import "facet/pkg/scad/ast"

// inferType returns the Facet type of an expression for condition/truthiness
// purposes: "Number", "[]Number", "Bool", "String", or "" when unknown. Bare
// identifiers are resolved in the current definition's scope (parameters +
// locals, built by buildScope); everything else is structural.
func (e *Emitter) inferType(x ast.Expr) string {
	switch n := x.(type) {
	case *ast.Bool:
		return "Bool"
	case *ast.Num:
		return "Number"
	case *ast.Str:
		return "String"
	case *ast.Ident:
		return e.scope[n.Name]
	case *ast.Binary:
		switch n.Op {
		case "<", ">", "<=", ">=", "==", "!=", "&&", "||":
			return "Bool"
		}
		return "Number" // arithmetic
	case *ast.Unary:
		if n.Op == "!" {
			return "Bool"
		}
		return e.inferType(n.X)
	case *ast.Index:
		if e.inferType(n.X) == "Any" {
			return "Any" // indexing a dynamic value stays dynamic
		}
		return "Number" // element of a []Number
	case *ast.Member:
		return "Number" // .x/.y/.z component
	case *ast.Vector:
		return "[]Number"
	case *ast.Ternary:
		return e.inferType(n.Then) // arms share a type
	case *ast.Call:
		return e.inferCallType(n)
	}
	return ""
}

// inferCallType returns the Facet result type of a call expression: a user
// function resolves to its classified return type; the built-ins are mapped by
// the shape the emitter gives them (concat builds a list; len/search and the
// math/trig family yield Numbers — trig and inverse-trig are Number because the
// emitter converts Facet's Angle results back to degree-numbers).
func (e *Emitter) inferCallType(n *ast.Call) string {
	if sym, ok := e.syms[n.Name]; ok && sym.isFunc {
		return e.classifyFuncReturn(sym.funcBody, map[string]bool{n.Name: true})
	}
	switch n.Name {
	case "concat":
		// concat builds a list; if any operand is itself nested (a list of
		// lists / Any), the result is nested too — e.g. an accumulator built by
		// concat(acc, <nested>). Otherwise it is a flat []Number.
		for _, a := range n.Args {
			if e.inferType(a.Value) == "Any" {
				return "Any"
			}
		}
		return "[]Number"
	case "search": // search -> IndicesOf, a []Number list of indices
		return "[]Number"
	case "len", "norm",
		"sin", "cos", "tan", "asin", "acos", "atan", "atan2",
		"sqrt", "abs", "pow", "floor", "ceil", "round", "min", "max":
		return "Number"
	}
	return ""
}

// scopeBind is a local binding (a module-body assignment or a function let) used
// to seed the scope before a definition's body is emitted.
type scopeBind struct {
	name  string
	value ast.Expr
}

// buildScope records, into e.scope, the Facet type of a definition's parameters
// and local bindings so inferType can resolve bare identifiers in conditions.
// Parameters use their classified vector/scalar type (a boolean default makes a
// parameter Bool); locals are inferred from their right-hand sides in source
// order, so a later binding sees the earlier ones.
func (e *Emitter) buildScope(defName string, params []ast.Param, binds []scopeBind) {
	s := make(map[string]string, len(params)+len(binds))
	for _, p := range params {
		switch {
		case e.nested.has(defName, p.Name):
			s[p.Name] = "Any"
		case e.vecParams.has(defName, p.Name):
			s[p.Name] = "[]Number"
		case isBoolLit(p.Default):
			s[p.Name] = "Bool"
		default:
			s[p.Name] = "Number"
		}
	}
	e.scope = s
	for _, b := range binds {
		s[b.name] = e.inferType(b.value)
	}
}

// isBoolLit reports whether x is a boolean literal.
func isBoolLit(x ast.Expr) bool {
	_, ok := x.(*ast.Bool)
	return ok
}

// assignBinds collects the top-level `name = value` assignments of a body as
// scope bindings (module bodies carry locals as assignment statements).
func assignBinds(body []ast.Stmt) []scopeBind {
	var binds []scopeBind
	for _, s := range body {
		if a, ok := s.(*ast.Assign); ok {
			binds = append(binds, scopeBind{a.Name, a.Value})
		}
	}
	return binds
}
