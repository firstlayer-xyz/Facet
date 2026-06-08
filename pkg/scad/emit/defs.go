package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// emitModuleDef translates `module name(params) { body }` into a Facet fn.
// Parameters are classified as scalar Number or vector []Number (§classify);
// the body is rendered by emitGeomBody. Constructs not yet handled surface as
// errors via the body translation rather than being silently dropped.
func (e *Emitter) emitModuleDef(m *ast.ModuleDef) string {
	e.paramRenames = reassignedParamRenames(m.Params, m.Body)
	params := make([]string, 0, len(m.Params))
	if e.animUse[m.Name] {
		params = append(params, animTimeVar+" Number")
	}
	localFn := ""
	for _, p := range m.Params {
		if isResolutionVar(p.Name) {
			// A resolution variable carried as a parameter ($fn=50) becomes a
			// renamed Number parameter and, for $fn, the module-local segment
			// count that scopes curved primitives in the body.
			decl := resolutionParamName(p.Name) + " Number"
			if p.Default != nil {
				decl += " = " + e.expr(p.Default, kNumber)
			}
			params = append(params, decl)
			if p.Name == "$fn" {
				localFn = resolutionParamName(p.Name)
			}
			continue
		}
		// A parameter the body reassigns is renamed; the reassignment then
		// becomes a const of the original name (Facet has no shadowing).
		name := p.Name
		if r, ok := e.paramRenames[p.Name]; ok {
			name = r
		}
		decl := name + " " + e.paramType(m.Name, p)
		// A $t-bearing default can't live on a Facet parameter (defaults can't
		// reference scad_t); it is injected at call sites instead, so the
		// parameter is emitted without a default. See injectedAnimDefaults.
		if p.Default != nil && !exprHasAnimTime(p.Default) {
			decl += " = " + e.expr(p.Default, kNumber)
		}
		params = append(params, decl)
	}

	cu := e.childUse[m.Name]
	if cu.uses {
		params = append(params, "children []"+cu.childElemType())
	}

	var w writer
	w.writef("fn %s(%s) %s {\n", m.Name, strings.Join(params, ", "), e.topReturnType(geometryStmts(m.Body)))
	e.curChild2D = cu.is2D
	e.localFn = localFn
	e.buildScope(m.Name, m.Params, assignBinds(m.Body))
	e.emitGeomBody(&w, m.Body, "module '"+m.Name+"'", m.Pos())
	e.scope = nil
	e.localFn = ""
	e.paramRenames = nil
	w.write("}")
	return w.str()
}

// reassignedParamRenames returns, for each parameter the body reassigns at the
// top level (OpenSCAD shadowing, e.g. `radius = radius/4`), a fresh name for the
// parameter. Facet has no shadowing, so the parameter is renamed and the
// reassignment becomes a const of the original name. Nil when none are.
func reassignedParamRenames(params []ast.Param, body []ast.Stmt) map[string]string {
	isParam := map[string]bool{}
	for _, p := range params {
		isParam[p.Name] = true
	}
	var renames map[string]string
	for _, s := range body {
		a, ok := s.(*ast.Assign)
		if !ok || !isParam[a.Name] {
			continue
		}
		if renames == nil {
			renames = map[string]string{}
		}
		renames[a.Name] = a.Name + "_arg"
	}
	return renames
}

// geometryStmts returns the geometry-producing (non-assignment) statements of a
// body, dropping the local `name = value` assignments that become consts.
func geometryStmts(body []ast.Stmt) []ast.Stmt {
	var geom []ast.Stmt
	for _, s := range body {
		if _, ok := s.(*ast.Assign); !ok {
			geom = append(geom, s)
		}
	}
	return geom
}

// emitGeomBody renders a module body (or one branch of a conditional) as the
// inside of a Facet fn: leading local assignments become `const` declarations
// and the remaining geometry is returned. A lone `if` becomes a return-bearing
// if/else chain (writeGeomIf). Conditional geometry combined with other
// geometry has no single-expression Facet form, so it is rejected. ctx names the
// enclosing definition for error messages.
func (e *Emitter) emitGeomBody(w *writer, body []ast.Stmt, ctx string, pos ast.Pos) {
	for _, s := range body {
		if a, ok := s.(*ast.Assign); ok {
			// A reassigned parameter's const references the (renamed) parameter
			// on its right-hand side; everywhere else the name is this const.
			if _, isReassign := e.paramRenames[a.Name]; isReassign {
				e.renamingParamRHS = true
				rhs := e.expr(a.Value, kNumber)
				e.renamingParamRHS = false
				w.writef("\tconst %s = %s\n", a.Name, rhs)
			} else {
				w.writef("\tconst %s = %s\n", a.Name, e.expr(a.Value, kNumber))
			}
		}
	}
	geom := geometryStmts(body)
	if len(geom) == 1 {
		if ifs, ok := geom[0].(*ast.If); ok {
			e.writeGeomIf(w, ifs, ctx)
			return
		}
	}
	for _, s := range geom {
		if ifs, ok := s.(*ast.If); ok {
			e.errf(ifs.Pos(), "%s: conditional geometry combined with other geometry is not supported", ctx)
			return
		}
	}
	g := e.unionStmts(geom)
	if g == "" {
		e.errf(pos, "%s produces no geometry", ctx)
		return
	}
	w.writef("\treturn %s\n", g)
}

// writeGeomIf renders a geometry `if` as a return-bearing Facet conditional:
// the then- and else-branches each return their own geometry, and a lone nested
// `if` in the else position folds into `else if`. A geometry `if` without an
// `else` would leave a path that returns nothing, so it is rejected.
func (e *Emitter) writeGeomIf(w *writer, n *ast.If, ctx string) {
	w.writef("\tif %s {\n", e.cond(n.Cond))
	e.emitGeomBody(w, n.Then, ctx, n.Pos())
	w.write("\t}")
	if len(n.Else) == 0 {
		e.errf(n.Pos(), "%s: a geometry `if` without `else` does not produce geometry on every path", ctx)
		w.write("\n")
		return
	}
	if len(n.Else) == 1 {
		if elif, ok := n.Else[0].(*ast.If); ok {
			w.write(" else ")
			e.writeGeomIf(w, elif, ctx)
			return
		}
	}
	w.write(" else {\n")
	e.emitGeomBody(w, n.Else, ctx, n.Pos())
	w.write("\t}\n")
}

// cond renders an `if`/ternary condition as a Facet Bool. A boolean-shaped
// expression (comparison/logical/!/bool) passes through. Otherwise OpenSCAD
// truthiness is applied by the value's inferred type: a number is nonzero, an
// array/string is non-empty, a Bool is itself. A type we cannot infer is an
// error rather than a guess.
func (e *Emitter) cond(x ast.Expr) string {
	if isBooleanShaped(x) {
		return e.expr(x, kNumber)
	}
	switch e.inferType(x) {
	case "Bool":
		return e.expr(x, kNumber)
	case "Number":
		return e.operand(x, kNumber) + " != 0"
	case "[]Number":
		return "Size(of: " + e.expr(x, kNumber) + ") > 0"
	case "String":
		return e.expr(x, kNumber) + ` != ""`
	}
	return e.errf(x.Pos(), "cannot determine the truthiness of this condition (unknown type)")
}

// isBooleanShaped reports whether an expression already evaluates to a Bool: a
// comparison, a logical `&&`/`||`, a `!` negation, or a boolean literal.
func isBooleanShaped(x ast.Expr) bool {
	switch n := x.(type) {
	case *ast.Bool:
		return true
	case *ast.Unary:
		return n.Op == "!"
	case *ast.Binary:
		switch n.Op {
		case "<", ">", "<=", ">=", "==", "!=", "&&", "||":
			return true
		}
	}
	return false
}

// emitFunctionDef translates `function name(params) = expr;` into a Facet fn.
// A top-level `let(...)` chain in the body becomes const declarations; the
// remaining expression is returned. Parameters are classified scalar/vector and
// the return type from the body shape. A whole-body ternary becomes an
// if/else-if/else chain where every branch returns (Facet's conditional form);
// a ternary buried inside a larger expression still errors (needs lowering).
func (e *Emitter) emitFunctionDef(d *ast.FunctionDef) string {
	params := make([]string, 0, len(d.Params))
	for _, p := range d.Params {
		decl := p.Name + " " + e.paramType(d.Name, p)
		if p.Default != nil {
			decl += " = " + e.expr(p.Default, kNumber)
		}
		params = append(params, decl)
	}

	// Collect the let-chain bindings first (without emitting), so the scope is
	// known before any binding RHS or the body — both may contain conditions.
	body := d.Body
	var binds []scopeBind
	for {
		l, ok := body.(*ast.Let)
		if !ok {
			break
		}
		for _, b := range l.Binds {
			binds = append(binds, scopeBind{b.Name, b.Value})
		}
		body = l.Body
	}
	e.buildScope(d.Name, d.Params, binds)

	var w writer
	w.writef("fn %s(%s) %s {\n", d.Name, strings.Join(params, ", "), e.classifyFuncReturn(body, map[string]bool{d.Name: true}))
	for _, b := range binds {
		w.writef("\tconst %s = %s\n", b.name, e.expr(b.value, kNumber))
	}
	// A whole-body ternary is just the return value now that Facet has a ternary
	// expression (expr handles *ast.Ternary); no if/else-return lowering needed.
	w.writef("\treturn %s\n", e.expr(body, kNumber))
	w.write("}")
	e.scope = nil
	return w.str()
}

// userModuleCall emits a call to a user-defined module, mapping positional
// arguments to the definition's parameter names (named args pass through).
func (e *Emitter) userModuleCall(n *ast.ModuleCall, sym *symbol) string {
	args := e.mapArgs(n.Name, n.Args, paramNames(sym.params))
	add := func(arg string) {
		if args != "" {
			args += ", "
		}
		args += arg
	}
	if e.animUse[n.Name] {
		add(animTimeVar + ": " + animTimeVar)
	}
	// Supply $t-bearing defaults the call omits, evaluated here in the caller's
	// scope where scad_t is live (Facet defaults can't reference it). A default
	// that references another parameter can't be injected — that parameter isn't
	// in the caller's scope — so it is a hard error, not a broken reference.
	for _, p := range e.injectedAnimDefaults(n, sym) {
		if defaultRefsParam(p, sym.params) {
			e.errf(n.Pos(), "%s: the $t default for parameter %q references another parameter, so it cannot be injected at the call site", n.Name, p.Name)
			continue
		}
		add(p.Name + ": " + e.expr(p.Default, kNumber))
	}
	if cu := e.childUse[n.Name]; cu.uses {
		add("children: " + e.childrenArray(n.Children, cu))
	} else if len(n.Children) > 0 {
		return e.errf(n.Pos(), "%s: passing children to a module that does not use children()", n.Name)
	}
	return n.Name + "(" + args + ")"
}
