package emit

import "facet/pkg/scad/ast"

// paramType returns a parameter's Facet type. A Bool literal default
// (`flag = false`) classifies the parameter as Bool. A nested array (a list
// of lists) exceeds the binary scalar/vector model and is typed `Any`
// (dynamic). A flat vector — from a vector default, in-body indexing/member
// access, or a type propagated across call sites — is []Number. A parameter
// forwarded as a String-position argument (e.g. `color(c)`, `text(name)`)
// classifies as String. Everything else is scalar Number.
func (e *Emitter) paramType(defName string, p ast.Param) string {
	switch {
	case e.nested.has(defName, p.Name):
		return "Any"
	case e.vecParams.has(defName, p.Name):
		return "[]Number"
	case isBoolDefault(p.Default):
		return "Bool"
	case e.paramIsString(defName, p.Name):
		return "String"
	default:
		return "Number"
	}
}

// paramIsString reports whether `name` is used as a String argument anywhere
// in defName's body — currently just `color(name)` and `color(c = name)`.
func (e *Emitter) paramIsString(defName, name string) bool {
	sym, ok := e.syms[defName]
	if !ok {
		return false
	}
	return stmtsUseAsStringColor(name, sym.moduleBody)
}

func stmtsUseAsStringColor(name string, stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if stmtUsesAsStringColor(name, s) {
			return true
		}
	}
	return false
}

func stmtUsesAsStringColor(name string, s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.ModuleCall:
		if n.Name == "color" {
			for i, a := range n.Args {
				if a.Name == "c" || (a.Name == "" && i == 0) {
					if id, ok := a.Value.(*ast.Ident); ok && id.Name == name {
						return true
					}
				}
			}
		}
		if stmtsUseAsStringColor(name, n.Children) {
			return true
		}
	case *ast.For:
		return stmtsUseAsStringColor(name, n.Children)
	case *ast.If:
		return stmtsUseAsStringColor(name, n.Then) || stmtsUseAsStringColor(name, n.Else)
	}
	return false
}

// isBoolDefault reports whether a parameter's default is a Bool literal —
// the only signal we use today for classifying a parameter as Bool, since
// OpenSCAD has no separate Bool type annotation. False on a missing default
// (nil) or any other expression shape.
func isBoolDefault(x ast.Expr) bool {
	_, ok := x.(*ast.Bool)
	return ok
}

// exprUsesAsVector reports whether `name` is indexed (`name[…]`) or
// member-accessed (`name.x/.y/.z`) anywhere in x — the signal that the
// identifier holds a vector.
func exprUsesAsVector(name string, x ast.Expr) bool {
	switch n := x.(type) {
	case *ast.Index:
		if id, ok := n.X.(*ast.Ident); ok && id.Name == name {
			return true
		}
		return exprUsesAsVector(name, n.X) || exprUsesAsVector(name, n.Index)
	case *ast.Member:
		if id, ok := n.X.(*ast.Ident); ok && id.Name == name {
			switch n.Name {
			case "x", "y", "z":
				return true
			}
		}
		return exprUsesAsVector(name, n.X)
	case *ast.Binary:
		return exprUsesAsVector(name, n.L) || exprUsesAsVector(name, n.R)
	case *ast.Unary:
		return exprUsesAsVector(name, n.X)
	case *ast.Ternary:
		return exprUsesAsVector(name, n.Cond) || exprUsesAsVector(name, n.Then) || exprUsesAsVector(name, n.Else)
	case *ast.Call:
		return builtinVectorArg(name, n) || argsUseAsVector(name, n.Args)
	case *ast.Vector:
		for _, el := range n.Elems {
			if exprUsesAsVector(name, el) {
				return true
			}
		}
	case *ast.Range:
		if exprUsesAsVector(name, n.Start) || exprUsesAsVector(name, n.End) {
			return true
		}
		return n.Step != nil && exprUsesAsVector(name, n.Step)
	case *ast.Let:
		for _, b := range n.Binds {
			if exprUsesAsVector(name, b.Value) {
				return true
			}
		}
		return exprUsesAsVector(name, n.Body)
	}
	return false
}

// builtinVectorArg reports whether `name` is passed (as a bare identifier) in an
// argument position of a built-in that requires an array there: the list of
// search, the vector of norm, or any argument of concat. Such a parameter is a
// vector even though it is never indexed in this body.
func builtinVectorArg(name string, n *ast.Call) bool {
	isName := func(x ast.Expr) bool {
		id, ok := x.(*ast.Ident)
		return ok && id.Name == name
	}
	switch n.Name {
	case "search": // search(match_value, list): the list is an array
		if a, ok := callPositional(n, 1); ok {
			return isName(a)
		}
	case "norm": // norm(v): v is a vector
		if a, ok := callPositional(n, 0); ok {
			return isName(a)
		}
	case "concat": // concat(a, b, …): every argument is a list
		for _, a := range n.Args {
			if a.Name == "" && isName(a.Value) {
				return true
			}
		}
	}
	return false
}

// callPositional returns the value of the idx-th positional argument of a call.
func callPositional(n *ast.Call, idx int) (ast.Expr, bool) {
	pos := 0
	for _, a := range n.Args {
		if a.Name != "" {
			continue
		}
		if pos == idx {
			return a.Value, true
		}
		pos++
	}
	return nil, false
}

// stmtUsesAsVector is the statement-level companion to exprUsesAsVector.
func stmtUsesAsVector(name string, s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.ModuleCall:
		if argsUseAsVector(name, n.Args) {
			return true
		}
		return childrenUseAsVector(name, n.Children)
	case *ast.Assign:
		return exprUsesAsVector(name, n.Value)
	case *ast.For:
		for _, it := range n.Iters {
			if exprUsesAsVector(name, it.Range) {
				return true
			}
		}
		return childrenUseAsVector(name, n.Children)
	case *ast.If:
		if exprUsesAsVector(name, n.Cond) {
			return true
		}
		return childrenUseAsVector(name, n.Then) || childrenUseAsVector(name, n.Else)
	}
	return false
}

func argsUseAsVector(name string, args []ast.Arg) bool {
	for _, a := range args {
		if exprUsesAsVector(name, a.Value) {
			return true
		}
	}
	return false
}

func childrenUseAsVector(name string, stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if stmtUsesAsVector(name, s) {
			return true
		}
	}
	return false
}

// classifyFuncReturn infers a value function's Facet return type from the shape
// of its body. A vector literal or concat is []Number; a comparison/logical op
// is Bool; otherwise Number. Trig and inverse-trig calls are Number because the
// emitter converts Facet's Angle results back to degree-numbers (see call). A
// call to another user function resolves to that function's return type, so a
// value forwarded through a helper keeps its shape; seen guards recursion.
func (e *Emitter) classifyFuncReturn(body ast.Expr, seen map[string]bool) string {
	switch b := body.(type) {
	case *ast.Vector:
		// A list whose elements are themselves lists is a nested array, beyond
		// the []Number model — type it Any. An element's inferred type of
		// "[]Number" or "Any" means it is itself a list.
		for _, el := range b.Elems {
			if t := e.inferType(el); t == "[]Number" || t == "Any" {
				return "Any"
			}
		}
		return "[]Number"
	case *ast.Binary:
		switch b.Op {
		case "<", ">", "<=", ">=", "==", "!=", "&&", "||":
			return "Bool"
		case "+", "-", "*", "/":
			// Arithmetic broadcasts a vector through: scalar*vector and
			// vector*scalar both produce a vector. If either operand
			// classifies as a vector, so does the result.
			if e.classifyFuncReturn(b.L, seen) == "[]Number" || e.classifyFuncReturn(b.R, seen) == "[]Number" {
				return "[]Number"
			}
		}
	case *ast.Call:
		if b.Name == "concat" {
			return "[]Number"
		}
		if sym, ok := e.syms[b.Name]; ok && sym.isFunc {
			if seen[b.Name] {
				return "Number" // recursion: fall back to scalar
			}
			seen[b.Name] = true
			return e.classifyFuncReturn(sym.funcBody, seen)
		}
	case *ast.Ternary:
		// A ternary's branches share a type; classify from the then-branch.
		return e.classifyFuncReturn(b.Then, seen)
	case *ast.Let:
		// The value is the let body; the bindings only scope locals.
		return e.classifyFuncReturn(b.Body, seen)
	}
	return "Number"
}
