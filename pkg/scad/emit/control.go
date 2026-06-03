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
// where the children are unioned into a single geometry per iteration.
func (e *Emitter) geomComprehension(p ast.Pos, combiner string, iters []ast.ForIter, children []ast.Stmt) string {
	clauses := make([]string, 0, len(iters))
	for _, it := range iters {
		clauses = append(clauses, it.Var+" "+e.expr(it.Range, kNumber))
	}
	child := e.unionStmts(children)
	if child == "" {
		return e.errf(p, "loop body produces no geometry")
	}
	return combiner + "(arr: for " + strings.Join(clauses, ", ") + " { yield " + child + " })"
}
