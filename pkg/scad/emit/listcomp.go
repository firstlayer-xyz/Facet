package emit

import (
	"fmt"
	"strings"

	"facet/pkg/scad/ast"
)

// emitListComp emits an OpenSCAD list comprehension `[...]` — values interspersed
// with for/if/let/each clauses — as the concatenation of its elements' lists.
// Each element emits a list-valued Facet expression (a value is a singleton, each
// is its list, a scalar for is a `for { yield }` comprehension, a list-producing
// for is flattened with fold), and the elements are joined with `+`.
func (e *Emitter) emitListComp(n *ast.ListComp) string {
	parts := make([]string, len(n.Elems))
	for i, el := range n.Elems {
		parts[i] = e.compElemList(el)
	}
	return strings.Join(parts, " + ")
}

// compElemList emits one comprehension element as a list-valued Facet expression.
func (e *Emitter) compElemList(el ast.CompElem) string {
	switch n := el.(type) {
	case *ast.ValueElem:
		return "[" + e.expr(n.X, kNumber) + "]"
	case *ast.EachElem:
		// `each L` flattens L's elements into the surrounding list — L already is
		// the list, so it contributes directly.
		return e.expr(n.X, kNumber)
	case *ast.IfElem:
		els := "[]"
		if n.Else != nil {
			els = e.compElemList(n.Else)
		}
		return "(" + e.cond(n.Cond) + " ? " + e.compElemList(n.Then) + " : " + els + ")"
	case *ast.LetElem:
		return e.withLetBinds(n.Binds, func() string { return e.compElemList(n.Body) })
	case *ast.ForElem:
		clauses := e.forClauses(n.Iters)
		if compElemScalar(n.Body) {
			// Body yields 0..1 value per iteration → a direct comprehension.
			return "for " + clauses + " { " + e.compElemYield(n.Body) + " }"
		}
		// Body yields a list per iteration (each/nested-for/if-with-list) → produce
		// the list-of-lists and flatten it with fold.
		acc, elem := e.freshLoopVar(), e.freshLoopVar()
		inner := "for " + clauses + " { yield " + e.compElemList(n.Body) + " }"
		return "fold " + acc + ", " + elem + " (" + inner + ") { yield " + acc + " + " + elem + " }"
	}
	panic(fmt.Sprintf("emit: unhandled list-comprehension element %T", el))
}

// walkCompElem visits a comprehension element tree: name() for each bound name
// (for-iterator variables and let-binding names) and expr() for each contained
// expression. Shared by the analysis passes that previously walked ListComp's
// fixed iter/body shape.
func walkCompElem(el ast.CompElem, name func(string), expr func(ast.Expr)) {
	switch n := el.(type) {
	case *ast.ValueElem:
		expr(n.X)
	case *ast.EachElem:
		expr(n.X)
	case *ast.IfElem:
		expr(n.Cond)
		walkCompElem(n.Then, name, expr)
		if n.Else != nil {
			walkCompElem(n.Else, name, expr)
		}
	case *ast.LetElem:
		for _, b := range n.Binds {
			name(b.Name)
			expr(b.Value)
		}
		walkCompElem(n.Body, name, expr)
	case *ast.ForElem:
		for _, it := range n.Iters {
			name(it.Var)
			expr(it.Range)
		}
		walkCompElem(n.Body, name, expr)
	}
}

// forClauses renders a for clause's iterators as `var iter[, var iter…]`.
func (e *Emitter) forClauses(iters []ast.ForIter) string {
	cs := make([]string, len(iters))
	for i, it := range iters {
		cs[i] = it.Var + " " + e.expr(it.Range, kNumber)
	}
	return strings.Join(cs, ", ")
}

// compElemScalar reports whether a for-body element yields exactly zero-or-one
// value per iteration (so it maps to a `for { [if c] yield v }` comprehension).
// each and nested for produce a list per iteration and need fold-flatten instead.
func compElemScalar(el ast.CompElem) bool {
	switch n := el.(type) {
	case *ast.ValueElem:
		return true
	case *ast.IfElem:
		return compElemScalar(n.Then) && (n.Else == nil || compElemScalar(n.Else))
	case *ast.LetElem:
		return compElemScalar(n.Body)
	case *ast.EachElem, *ast.ForElem:
		return false
	}
	return false
}

// compElemYield emits the for-body statement(s) that yield a single (or filtered)
// element: `yield v`, `if c { … } [else { … }]`, or a let wrapping those. Only
// called when compElemScalar(el) is true.
func (e *Emitter) compElemYield(el ast.CompElem) string {
	switch n := el.(type) {
	case *ast.ValueElem:
		return "yield " + e.expr(n.X, kNumber)
	case *ast.IfElem:
		out := "if " + e.cond(n.Cond) + " { " + e.compElemYield(n.Then) + " }"
		if n.Else != nil {
			out += " else { " + e.compElemYield(n.Else) + " }"
		}
		return out
	case *ast.LetElem:
		return e.withLetBinds(n.Binds, func() string { return e.compElemYield(n.Body) })
	}
	panic(fmt.Sprintf("emit: compElemYield on non-scalar element %T", el))
}
