package emit

import "facet/pkg/scad/ast"

// animTime threads OpenSCAD's $t animation clock through the transpiled program.
//
// In OpenSCAD $t is a dynamically-scoped global that varies per frame; Facet
// globals are computed once, so $t cannot be a module-level const if it is to
// animate. Instead the transpiler turns a $t-using program into a Facet
// Animation whose frame lambda derives scad_t from the wall-clock time, and
// passes scad_t down to every definition that needs it:
//
//   - A definition whose BODY references $t (directly, or via calling another
//     such definition) gains a leading `scad_t Number` parameter; call sites
//     pass `scad_t: scad_t`. This mirrors how children() is threaded.
//   - A $t-bearing parameter DEFAULT (e.g. `module spin(a = $t*360)`) is left
//     off the Facet parameter (Facet defaults cannot reference other params)
//     and instead INJECTED at each call site that omits the argument, evaluated
//     in the caller's scope where scad_t is live — matching OpenSCAD, where a
//     default's $t resolves in the caller's context.
//
// scadAnimPeriodMs is the wall-clock period over which scad_t cycles 0..1,
// reproducing OpenSCAD's normalized clock. OpenSCAD has no fixed wall-clock
// period (it derives $t from FPS×steps in the GUI), so the transpiler picks a
// sensible loop; the emitted frame body is easy to retune.
const scadAnimPeriodMs = 4000

// eachExprName walks x, calling ident for every variable identifier referenced
// (including special vars like $t) and bind for every name a Let or
// comprehension introduces, with the binding node's position. Call names and
// member field names are skipped — they are not variable references. It is the
// single expression walker the animation analysis and reserved-name check build
// on.
func eachExprName(x ast.Expr, ident func(name string), bind func(name string, pos ast.Pos)) {
	switch n := x.(type) {
	case *ast.Ident:
		ident(n.Name)
	case *ast.Vector:
		for _, el := range n.Elems {
			eachExprName(el, ident, bind)
		}
	case *ast.ListComp:
		for _, el := range n.Elems {
			walkCompElem(el, func(nm string) { bind(nm, n.Pos()) }, func(x ast.Expr) { eachExprName(x, ident, bind) })
		}
	case *ast.Range:
		eachExprName(n.Start, ident, bind)
		eachExprName(n.End, ident, bind)
		if n.Step != nil {
			eachExprName(n.Step, ident, bind)
		}
	case *ast.Binary:
		eachExprName(n.L, ident, bind)
		eachExprName(n.R, ident, bind)
	case *ast.Unary:
		eachExprName(n.X, ident, bind)
	case *ast.Ternary:
		eachExprName(n.Cond, ident, bind)
		eachExprName(n.Then, ident, bind)
		eachExprName(n.Else, ident, bind)
	case *ast.Call:
		for _, a := range n.Args {
			eachExprName(a.Value, ident, bind)
		}
	case *ast.Index:
		eachExprName(n.X, ident, bind)
		eachExprName(n.Index, ident, bind)
	case *ast.Member:
		eachExprName(n.X, ident, bind)
	case *ast.Let:
		for _, b := range n.Binds {
			bind(b.Name, n.Pos())
			eachExprName(b.Value, ident, bind)
		}
		eachExprName(n.Body, ident, bind)
	}
}

// eachExprIdent calls visit for every variable identifier referenced in x
// (including special vars like $t), ignoring names introduced by Let and
// comprehension bindings. It is the walker the animation analysis builds on.
func eachExprIdent(x ast.Expr, visit func(name string)) {
	eachExprName(x, visit, func(string, ast.Pos) {})
}

// animTimeName is the identifier set matching OpenSCAD's $t clock, tested
// against an expression via exprRefsAny.
var animTimeName = map[string]bool{"$t": true}

// exprHasAnimTime reports whether an expression references $t.
func exprHasAnimTime(x ast.Expr) bool {
	return exprRefsAny(x, animTimeName)
}

// exprRefsAny reports whether an expression references any identifier in names.
func exprRefsAny(x ast.Expr, names map[string]bool) bool {
	found := false
	eachExprIdent(x, func(name string) {
		if names[name] {
			found = true
		}
	})
	return found
}

func argsHaveAnimTime(args []ast.Arg) bool {
	for _, a := range args {
		if exprHasAnimTime(a.Value) {
			return true
		}
	}
	return false
}

// paramsHaveAnimTimeDefault reports whether any parameter default references $t.
func paramsHaveAnimTimeDefault(params []ast.Param) bool {
	for _, p := range params {
		if p.Default != nil && exprHasAnimTime(p.Default) {
			return true
		}
	}
	return false
}

// stmtBodyHasAnimTime reports whether a statement's own expressions reference
// $t — the geometry/assignment expressions of a definition body, NOT parameter
// defaults (those are handled at call sites). Nested bodies (for/if/children
// blocks) are included.
func stmtBodyHasAnimTime(s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.Assign:
		return exprHasAnimTime(n.Value)
	case *ast.ModuleCall:
		// echo/assert are dropped wholesale (see isDroppedBuiltin), so a $t in
		// their args has no geometric effect and must not flag the enclosing
		// definition as needing scad_t — otherwise scad_t is threaded but never
		// derived (emission never sees the $t), leaving an undefined reference.
		if isDroppedBuiltin(n.Name) {
			return false
		}
		if argsHaveAnimTime(n.Args) {
			return true
		}
		return bodyHasAnimTime(n.Children)
	case *ast.For:
		for _, it := range n.Iters {
			if exprHasAnimTime(it.Range) {
				return true
			}
		}
		return bodyHasAnimTime(n.Children)
	case *ast.If:
		if exprHasAnimTime(n.Cond) {
			return true
		}
		return bodyHasAnimTime(n.Then) || bodyHasAnimTime(n.Else)
	}
	return false
}

func bodyHasAnimTime(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if stmtBodyHasAnimTime(s) {
			return true
		}
	}
	return false
}

// analyzeAnimTime returns the set of user definitions that need a `scad_t`
// parameter: those whose body references $t, transitively closed over the call
// graph (a definition that calls a scad_t-needing definition, or that omits an
// argument whose callee default references $t, itself needs scad_t to supply it).
func (e *Emitter) analyzeAnimTime() map[string]bool {
	need := map[string]bool{}
	for name, sym := range e.syms {
		if bodyHasAnimTime(e.defBody(sym)) {
			need[name] = true
		}
	}
	// Fixpoint: a definition that, in its body, makes a call requiring scad_t
	// (the callee needs it, or a $t-bearing default must be injected) also needs
	// scad_t to pass it down.
	for changed := true; changed; {
		changed = false
		for name, sym := range e.syms {
			if need[name] {
				continue
			}
			if e.bodyCallNeedsAnimTime(e.defBody(sym), need) {
				need[name] = true
				changed = true
			}
		}
	}
	return need
}

// defBody returns the statement body of a definition symbol (function bodies are
// a single expression, carried as a synthetic return — they have no $t-bearing
// child statements, so an empty body is returned for them; their $t use is
// captured via callee analysis on the expression side).
func (e *Emitter) defBody(sym *symbol) []ast.Stmt {
	if sym.isFunc {
		return nil
	}
	return sym.moduleBody
}

// injectedAnimDefaults returns the callee parameters whose default references $t
// and which this call omits. Such defaults can't live on the Facet parameter (a
// default can't reference scad_t), so they are injected at the call site,
// evaluated in the caller's scope where scad_t is live — matching OpenSCAD,
// where a default's $t resolves in the caller's context.
func (e *Emitter) injectedAnimDefaults(mc *ast.ModuleCall, sym *symbol) []ast.Param {
	supplied := map[string]bool{}
	pos := 0
	for _, a := range mc.Args {
		if a.Name != "" {
			supplied[a.Name] = true
			continue
		}
		if pos < len(sym.params) {
			supplied[sym.params[pos].Name] = true
		}
		pos++
	}
	var out []ast.Param
	for _, p := range sym.params {
		if p.Default != nil && exprHasAnimTime(p.Default) && !supplied[p.Name] {
			out = append(out, p)
		}
	}
	return out
}

// defaultRefsParam reports whether parameter p's default references another
// parameter of the same module. Such a default cannot be injected at the call
// site — the other parameter is bound in the callee, not the caller's scope — so
// the caller has no value to substitute.
func defaultRefsParam(p ast.Param, params []ast.Param) bool {
	others := map[string]bool{}
	for _, q := range params {
		if q.Name != p.Name {
			others[q.Name] = true
		}
	}
	return exprRefsAny(p.Default, others)
}

// bodyCallNeedsAnimTime reports whether any module call in a body targets a
// definition that needs scad_t, or omits an argument whose callee default
// references $t (so the default must be injected with scad_t at the call site).
func (e *Emitter) bodyCallNeedsAnimTime(stmts []ast.Stmt, need map[string]bool) bool {
	found := false
	eachStmt(stmts, func(s ast.Stmt) {
		if found {
			return
		}
		mc, ok := s.(*ast.ModuleCall)
		if !ok {
			return
		}
		if need[mc.Name] {
			found = true
			return
		}
		if sym, ok := e.syms[mc.Name]; ok && len(e.injectedAnimDefaults(mc, sym)) > 0 {
			found = true
		}
	})
	return found
}
