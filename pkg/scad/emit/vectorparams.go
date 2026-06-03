package emit

import "facet/pkg/scad/ast"

// vectorParamSet records, per user definition, which parameters are vectors
// ([]Number) rather than scalars (Number).
type vectorParamSet map[string]map[string]bool

// has reports whether parameter name of definition def is classified as a vector.
func (s vectorParamSet) has(def, name string) bool {
	m, ok := s[def]
	return ok && m[name]
}

// classifyVectorParams classifies every user definition's parameters as vector
// or scalar. It seeds from intrinsic signals — a vector default or in-body
// indexing/member access — then propagates across call sites to a fixpoint: a
// parameter passed unchanged to a callee parameter that is a vector is itself a
// vector. This recovers vector types for "pass-through" parameters that are
// never indexed in their own body (e.g. a value forwarded to a helper).
func classifyVectorParams(syms symtab) vectorParamSet {
	vec := vectorParamSet{}
	for name, sym := range syms {
		set := map[string]bool{}
		for _, p := range sym.params {
			if paramIsVectorIntrinsic(p, sym) {
				set[p.Name] = true
			}
		}
		vec[name] = set
	}

	for changed := true; changed; {
		changed = false
		for defName, sym := range syms {
			visitCalls(sym, func(callee, argName string, pos int, arg ast.Expr) {
				id, ok := arg.(*ast.Ident)
				if !ok {
					return
				}
				cs, ok := syms[callee]
				if !ok {
					return
				}
				p := boundParam(cs, argName, pos)
				if p == "" || !vec.has(callee, p) || vec.has(defName, id.Name) {
					return
				}
				if isParam(sym, id.Name) {
					vec[defName][id.Name] = true
					changed = true
				}
			})
		}
	}
	return vec
}

// classifyNestedParams classifies parameters that are nested arrays (a list of
// lists, e.g. a list of points). These exceed the binary scalar/vector model,
// so the emitter types them `Any` (dynamic) rather than mistyping them
// []Number. It records the result in e.nested.
//
// It is a fixpoint that interleaves local-value inference with parameter
// classification: each round it builds every definition's scope (parameter +
// local types, via buildScope) from the classification so far, then propagates
// nesting across call sites. Building scopes inside the loop is what lets a
// parameter inherit nesting from a *local* value, not just from another
// parameter.
//
// Seeds: a parameter double-indexed in its body (p[i][j]) or with a nested
// vector default ([[…]]). Propagation rules for each call g(args) in f:
//   - a parameter passed to polyhedron's points/faces is nested ([][]Number);
//   - an indexed base (base[i]) passed to a vector parameter makes base nested
//     (its element is itself a list) — the indexed value over-approximates to
//     Any, so it marks only the base, never the callee parameter;
//   - a bare parameter forwarded to a nested parameter is nested;
//   - any non-indexed argument that infers as Any (e.g. an accumulator built by
//     concat of a nested local) makes the callee parameter nested.
func (e *Emitter) classifyNestedParams() {
	e.nested = vectorParamSet{}
	for name, sym := range e.syms {
		set := map[string]bool{}
		for _, p := range sym.params {
			if paramIsNestedIntrinsic(p, sym) {
				set[p.Name] = true
			}
		}
		e.nested[name] = set
	}

	for changed := true; changed; {
		changed = false
		for defName, sym := range e.syms {
			// Build this definition's scope so argument types (including locals)
			// resolve against the classification so far.
			e.buildScope(defName, sym.params, defBinds(sym))
			visitCalls(sym, func(callee, argName string, pos int, arg ast.Expr) {
				// polyhedron(points, faces): both are lists of lists ([][]Number),
				// so a parameter passed there is nested. polyhedron is a built-in
				// (not in syms), so handle it before the user-function rules.
				if callee == "polyhedron" {
					if id, ok := arg.(*ast.Ident); ok && isParam(sym, id.Name) && !e.nested.has(defName, id.Name) &&
						(argName == "points" || argName == "faces" || argName == "triangles" || pos == 0 || pos == 1) {
						e.nested[defName][id.Name] = true
						changed = true
					}
					return
				}
				cs, ok := e.syms[callee]
				if !ok {
					return
				}
				q := boundParam(cs, argName, pos)
				if q == "" {
					return
				}
				if idx, ok := arg.(*ast.Index); ok {
					// base[i] passed to a vector parameter => base is nested. The
					// indexed value itself over-approximates to Any, so it is not
					// used to mark the callee parameter.
					if base, ok := idx.X.(*ast.Ident); ok &&
						isParam(sym, base.Name) && e.vecParams.has(callee, q) && !e.nested.has(defName, base.Name) {
						e.nested[defName][base.Name] = true
						changed = true
					}
					return
				}
				// bare p forwarded to a nested parameter => p is nested.
				if id, ok := arg.(*ast.Ident); ok &&
					isParam(sym, id.Name) && e.nested.has(callee, q) && !e.nested.has(defName, id.Name) {
					e.nested[defName][id.Name] = true
					changed = true
				}
				// A non-indexed argument that infers as Any makes the callee
				// parameter nested — local value flow into the callee.
				if e.inferType(arg) == "Any" && !e.nested.has(callee, q) {
					e.nested[callee][q] = true
					changed = true
				}
			})
		}
	}
	e.scope = nil
}

// defBinds returns a definition's local bindings for scope construction: a
// module body's top-level assignments, or a function's let-chain bindings.
func defBinds(sym *symbol) []scopeBind {
	if !sym.isFunc {
		return assignBinds(sym.moduleBody)
	}
	var binds []scopeBind
	for body := sym.funcBody; ; {
		l, ok := body.(*ast.Let)
		if !ok {
			break
		}
		for _, b := range l.Binds {
			binds = append(binds, scopeBind{b.Name, b.Value})
		}
		body = l.Body
	}
	return binds
}

// paramIsNestedIntrinsic reports whether a parameter is a nested array from its
// own definition alone: a nested vector-literal default ([[…]]) or being
// double-indexed (p[i][j]) in the body.
func paramIsNestedIntrinsic(p ast.Param, sym *symbol) bool {
	if v, ok := p.Default.(*ast.Vector); ok {
		for _, el := range v.Elems {
			if _, ok := el.(*ast.Vector); ok {
				return true
			}
		}
	}
	if sym.isFunc {
		return exprDoubleIndexes(p.Name, sym.funcBody)
	}
	for _, s := range sym.moduleBody {
		if stmtDoubleIndexes(p.Name, s) {
			return true
		}
	}
	return false
}

// exprDoubleIndexes reports whether `name` is indexed twice over (name[i][j])
// anywhere in x — a direct signal that it is a nested array.
func exprDoubleIndexes(name string, x ast.Expr) bool {
	switch n := x.(type) {
	case *ast.Index:
		if inner, ok := n.X.(*ast.Index); ok {
			if id, ok := inner.X.(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
		return exprDoubleIndexes(name, n.X) || exprDoubleIndexes(name, n.Index)
	case *ast.Binary:
		return exprDoubleIndexes(name, n.L) || exprDoubleIndexes(name, n.R)
	case *ast.Unary:
		return exprDoubleIndexes(name, n.X)
	case *ast.Member:
		return exprDoubleIndexes(name, n.X)
	case *ast.Ternary:
		return exprDoubleIndexes(name, n.Cond) || exprDoubleIndexes(name, n.Then) || exprDoubleIndexes(name, n.Else)
	case *ast.Call:
		for _, a := range n.Args {
			if exprDoubleIndexes(name, a.Value) {
				return true
			}
		}
	case *ast.Vector:
		for _, el := range n.Elems {
			if exprDoubleIndexes(name, el) {
				return true
			}
		}
	case *ast.Range:
		if exprDoubleIndexes(name, n.Start) || exprDoubleIndexes(name, n.End) {
			return true
		}
		return n.Step != nil && exprDoubleIndexes(name, n.Step)
	case *ast.Let:
		for _, b := range n.Binds {
			if exprDoubleIndexes(name, b.Value) {
				return true
			}
		}
		return exprDoubleIndexes(name, n.Body)
	}
	return false
}

// stmtDoubleIndexes is the statement-level companion to exprDoubleIndexes.
func stmtDoubleIndexes(name string, s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.ModuleCall:
		for _, a := range n.Args {
			if exprDoubleIndexes(name, a.Value) {
				return true
			}
		}
		for _, c := range n.Children {
			if stmtDoubleIndexes(name, c) {
				return true
			}
		}
	case *ast.Assign:
		return exprDoubleIndexes(name, n.Value)
	case *ast.For:
		for _, it := range n.Iters {
			if exprDoubleIndexes(name, it.Range) {
				return true
			}
		}
		for _, c := range n.Children {
			if stmtDoubleIndexes(name, c) {
				return true
			}
		}
	case *ast.If:
		if exprDoubleIndexes(name, n.Cond) {
			return true
		}
		for _, c := range n.Then {
			if stmtDoubleIndexes(name, c) {
				return true
			}
		}
		for _, c := range n.Else {
			if stmtDoubleIndexes(name, c) {
				return true
			}
		}
	}
	return false
}

// paramIsVectorIntrinsic reports whether a parameter is a vector from its own
// definition alone: a vector-literal default, or indexing/member access in the
// body.
func paramIsVectorIntrinsic(p ast.Param, sym *symbol) bool {
	if _, ok := p.Default.(*ast.Vector); ok {
		return true
	}
	if sym.isFunc {
		return exprUsesAsVector(p.Name, sym.funcBody)
	}
	return childrenUseAsVector(p.Name, sym.moduleBody)
}

// boundParam returns the callee parameter name an argument binds to: the named
// argument's name, or the positional parameter at pos. It returns "" when the
// binding does not resolve to a declared parameter.
func boundParam(cs *symbol, argName string, pos int) string {
	if argName != "" {
		if isParam(cs, argName) {
			return argName
		}
		return ""
	}
	if pos >= 0 && pos < len(cs.params) {
		return cs.params[pos].Name
	}
	return ""
}

// isParam reports whether name is one of the definition's parameters.
func isParam(sym *symbol, name string) bool {
	for _, p := range sym.params {
		if p.Name == name {
			return true
		}
	}
	return false
}

// callVisitor receives one call-site argument binding: the callee name, the
// argument's name ("" if positional), its positional index (-1 if named), and
// the argument expression.
type callVisitor func(callee, argName string, pos int, arg ast.Expr)

// visitCalls invokes fn for every call/module-call argument binding in a
// definition's body, recursing through nested expressions and children.
func visitCalls(sym *symbol, fn callVisitor) {
	if sym.isFunc {
		walkExprCalls(sym.funcBody, fn)
		return
	}
	for _, s := range sym.moduleBody {
		walkStmtCalls(s, fn)
	}
}

// bindCallArgs invokes fn for each argument of a call, tracking the positional
// index of unnamed arguments.
func bindCallArgs(callee string, args []ast.Arg, fn callVisitor) {
	pos := 0
	for _, a := range args {
		if a.Name == "" {
			fn(callee, "", pos, a.Value)
			pos++
		} else {
			fn(callee, a.Name, -1, a.Value)
		}
	}
}

// walkExprCalls visits every call within an expression: it binds the call's own
// arguments via bindCallArgs and recurses into all sub-expressions.
func walkExprCalls(x ast.Expr, fn callVisitor) {
	switch n := x.(type) {
	case *ast.Call:
		bindCallArgs(n.Name, n.Args, fn)
		for _, a := range n.Args {
			walkExprCalls(a.Value, fn)
		}
	case *ast.Binary:
		walkExprCalls(n.L, fn)
		walkExprCalls(n.R, fn)
	case *ast.Unary:
		walkExprCalls(n.X, fn)
	case *ast.Index:
		walkExprCalls(n.X, fn)
		walkExprCalls(n.Index, fn)
	case *ast.Member:
		walkExprCalls(n.X, fn)
	case *ast.Ternary:
		walkExprCalls(n.Cond, fn)
		walkExprCalls(n.Then, fn)
		walkExprCalls(n.Else, fn)
	case *ast.Vector:
		for _, el := range n.Elems {
			walkExprCalls(el, fn)
		}
	case *ast.Range:
		walkExprCalls(n.Start, fn)
		walkExprCalls(n.End, fn)
		if n.Step != nil {
			walkExprCalls(n.Step, fn)
		}
	case *ast.Let:
		for _, b := range n.Binds {
			walkExprCalls(b.Value, fn)
		}
		walkExprCalls(n.Body, fn)
	}
}

// walkStmtCalls visits every call within a statement, recursing through nested
// expressions, children, and control-flow branches.
func walkStmtCalls(s ast.Stmt, fn callVisitor) {
	switch n := s.(type) {
	case *ast.ModuleCall:
		bindCallArgs(n.Name, n.Args, fn)
		for _, a := range n.Args {
			walkExprCalls(a.Value, fn)
		}
		for _, c := range n.Children {
			walkStmtCalls(c, fn)
		}
	case *ast.Assign:
		walkExprCalls(n.Value, fn)
	case *ast.For:
		for _, it := range n.Iters {
			walkExprCalls(it.Range, fn)
		}
		for _, c := range n.Children {
			walkStmtCalls(c, fn)
		}
	case *ast.If:
		walkExprCalls(n.Cond, fn)
		for _, c := range n.Then {
			walkStmtCalls(c, fn)
		}
		for _, c := range n.Else {
			walkStmtCalls(c, fn)
		}
	}
}
