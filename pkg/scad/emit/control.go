package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// forStmt translates `for (i = range) child` into a unioned Facet for-yield:
// `Union(arr: for i <range> { yield <child> })`. Multiple iterators become
// multiple for-yield clauses (a Cartesian product, as in OpenSCAD).
func (e *Emitter) forStmt(n *ast.For) string {
	return e.geomComprehension(n.Pos(), "Union", n.Iters, n.Children)
}

// intersectionFor translates `intersection_for(i = range, …) child` into
// `Intersection(arr: for i <range> { yield <child> })`. OpenSCAD parses it as a
// module call whose named arguments are the loop iterators.
func (e *Emitter) intersectionFor(n *ast.ModuleCall) string {
	iters := make([]ast.ForIter, 0, len(n.Args))
	for _, a := range n.Args {
		if a.Name == "" {
			return e.errf(n.Pos(), "intersection_for expects named iterators (i = range)")
		}
		iters = append(iters, ast.ForIter{Var: a.Name, Range: a.Value})
	}
	return e.geomComprehension(n.Pos(), "Intersection", iters, n.Children)
}

// geomComprehension builds `<combiner>(arr: for <clauses> { yield <children> })`
// where the children are unioned into a single geometry per iteration. Local
// assignments in the loop body (`for(i=…){ x = f(i); cube(x); }`) are OpenSCAD
// block-scoped bindings; they are inlined into the geometry (like let), since
// Facet has no statement bindings inside a for-yield.
func (e *Emitter) geomComprehension(p ast.Pos, combiner string, iters []ast.ForIter, children []ast.Stmt) string {
	clauses := e.forClauses(iters)
	child := e.unionStmts(geometryStmts(children))
	if child == "" {
		return e.errf(p, "loop body produces no geometry")
	}
	// Local assignments in the loop body are OpenSCAD block-scoped bindings; emit
	// them as Facet consts inside the for-yield body (as emitGeomBody does for a
	// module body), in source order, so the geometry can reference them.
	var binds strings.Builder
	for _, s := range children {
		if a, ok := s.(*ast.Assign); ok {
			binds.WriteString("const " + a.Name + " = " + e.expr(a.Value, kNumber) + "\n")
		}
	}
	inner := "for " + clauses + " {\n" + binds.String() + "yield " + child + "\n}"
	return e.combineGeom(combiner, inner)
}

// combineGeom wraps a runtime-length arr expression in the combiner. For Union
// it routes through the scad_union helper (and likewise scad_intersection) so a
// single-iteration loop / single child — a one-element list — doesn't trip the
// stdlib's >=2-element requirement. Other combiners pass through unchanged.
func (e *Emitter) combineGeom(combiner, inner string) string {
	switch combiner {
	case "Union":
		e.usesUnion = true
		return "scad_union(arr: " + inner + ")"
	case "Intersection":
		e.usesIntersection = true
		return "scad_intersection(arr: " + inner + ")"
	}
	return combiner + "(arr: " + inner + ")"
}
