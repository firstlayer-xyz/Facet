package emit

import "facet/pkg/scad/ast"

// childUse records how a module consumes the geometry passed to it as children.
// OpenSCAD's children()/children(i) become indexing into a `children` array
// parameter, so a module that uses children gains that parameter.
type childUse struct {
	uses bool // body references children(...)
	is2D bool // the children are 2D (Sketch) rather than 3D (Solid)
}

// childElemType is the Facet element type of a module's children array.
func (c childUse) childElemType() string {
	if c.is2D {
		return "Sketch"
	}
	return "Solid"
}

// analyzeChildren determines, for each user module, whether it consumes
// children() and (from its call sites) whether those children are 2D. The
// dimensionality is needed because a 2D children array is []Sketch while a 3D
// one is []Solid, and the two are not interchangeable at the call boundary.
func (e *Emitter) analyzeChildren(f *ast.File) map[string]childUse {
	info := map[string]childUse{}
	for name, sym := range e.syms {
		if !sym.isFunc && bodyUsesChildren(sym.moduleBody) {
			info[name] = childUse{uses: true}
		}
	}
	if len(info) == 0 {
		return info
	}
	bodies := [][]ast.Stmt{f.Stmts}
	for _, sym := range e.syms {
		if !sym.isFunc {
			bodies = append(bodies, sym.moduleBody)
		}
	}
	for name := range info {
		info[name] = childUse{uses: true, is2D: e.callSiteChildIs2D(name, bodies)}
	}
	return info
}

// callSiteChildIs2D reports whether the children passed to module `name` at its
// call sites are 2D. The first call site that supplies children decides; with
// no children-bearing call site it defaults to 3D (Solid).
func (e *Emitter) callSiteChildIs2D(name string, bodies [][]ast.Stmt) bool {
	is2D, decided := false, false
	for _, body := range bodies {
		eachStmt(body, func(s ast.Stmt) {
			if decided {
				return
			}
			if mc, ok := s.(*ast.ModuleCall); ok && mc.Name == name && len(mc.Children) > 0 {
				is2D = e.firstGeomIs2D(mc.Children, map[string]bool{})
				decided = true
			}
		})
		if decided {
			break
		}
	}
	return is2D
}

// bodyUsesChildren reports whether any statement in a body references children().
func bodyUsesChildren(body []ast.Stmt) bool {
	used := false
	eachStmt(body, func(s ast.Stmt) {
		if mc, ok := s.(*ast.ModuleCall); ok && mc.Name == "children" {
			used = true
		}
	})
	return used
}

// eachStmt visits every statement in a body, recursing into module-call
// children, for-loop children, and both branches of an if.
func eachStmt(stmts []ast.Stmt, visit func(ast.Stmt)) {
	for _, s := range stmts {
		visit(s)
		switch n := s.(type) {
		case *ast.ModuleCall:
			eachStmt(n.Children, visit)
		case *ast.For:
			eachStmt(n.Children, visit)
		case *ast.If:
			eachStmt(n.Then, visit)
			eachStmt(n.Else, visit)
		}
	}
}

// childrenRef emits a children() reference inside a module body: children() is
// the union of all passed children; children(i) indexes the children array.
func (e *Emitter) childrenRef(n *ast.ModuleCall) string {
	switch len(n.Args) {
	case 0:
		return "Union(arr: children)"
	case 1:
		if n.Args[0].Name != "" {
			return e.errf(n.Pos(), "children() index must be positional")
		}
		return "children[" + e.expr(n.Args[0].Value, kNumber) + "]"
	default:
		return e.errf(n.Pos(), "children() with multiple indices is not supported")
	}
}

// childrenArray renders a call's children block as the array bound to a
// children parameter. An empty block yields a typed empty array so its element
// type is unambiguous.
func (e *Emitter) childrenArray(children []ast.Stmt, cu childUse) string {
	parts := e.childParts(children)
	if len(parts) == 0 {
		return "[]" + cu.childElemType() + "[]"
	}
	return childPartsArray(parts)
}
