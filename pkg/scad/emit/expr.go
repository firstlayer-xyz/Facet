package emit

import (
	"fmt"
	"strconv"
	"strings"

	"facet/pkg/scad/ast"
)

// expr emits a SCAD expression as Facet, tagging bare numeric literals with the
// requested kind (mm / deg / bare Number).
func (e *Emitter) expr(x ast.Expr, k kind) string {
	if k == kAngle {
		// OpenSCAD angles are plain numbers in degrees; Facet has a distinct
		// Angle type. Bridge at the boundary: a literal becomes `<n> deg`, any
		// other expression is multiplied by `1 deg` (Number * Angle = Angle).
		if num, ok := x.(*ast.Num); ok {
			return num.Text + " deg"
		}
		if u, ok := x.(*ast.Unary); ok && u.Op == "-" {
			if num, ok := u.X.(*ast.Num); ok {
				return "-" + num.Text + " deg"
			}
		}
		return e.operand(x, kNumber) + " * 1 deg"
	}
	switch n := x.(type) {
	case *ast.Num:
		if k == kLength {
			return n.Text + " mm"
		}
		return n.Text
	case *ast.Str:
		return strconv.Quote(n.Value)
	case *ast.Bool:
		if n.Val {
			return "true"
		}
		return "false"
	case *ast.Ident:
		// A let()-bound name resolves to its (already-emitted) value expression;
		// OpenSCAD has no runtime let, so the binding is inlined at the use site.
		if v, ok := e.letScope[n.Name]; ok {
			return v
		}
		if strings.HasPrefix(n.Name, "$") {
			switch n.Name {
			case "$t":
				// $t is OpenSCAD's animation clock (0..1). It maps to scad_t, which
				// File derives per frame inside a Facet Animation and threads through
				// $t-using definitions as a parameter (see anim.go).
				e.usesAnimTime = true
				return animTimeVar
			case "$children":
				// The number of children passed to the enclosing module.
				return "Size(of: children)"
			case "$fn", "$fa", "$fs":
				// A reference to a resolution variable carried as a module
				// parameter (renamed scad_fn/scad_fa/scad_fs; see emitModuleDef).
				return resolutionParamName(n.Name)
			}
			return e.errf(n.Pos(), "special variable %q is not supported", n.Name)
		}
		if e.renamingParamRHS {
			if r, ok := e.paramRenames[n.Name]; ok {
				return r // the original parameter, referenced from its reassignment
			}
		}
		// An identifier is a Number-domain value (parameter/const); it coerces
		// into a surrounding Length/Angle parameter without a unit suffix.
		return n.Name
	case *ast.Call:
		return e.call(n)
	case *ast.Range:
		// OpenSCAD ranges are inclusive: [start:end] / [start:step:end] (step in
		// the middle). Facet is also inclusive but takes the step last.
		if n.Step != nil {
			return "[" + e.expr(n.Start, kNumber) + ":" + e.expr(n.End, kNumber) + ":" + e.expr(n.Step, kNumber) + "]"
		}
		return "[" + e.expr(n.Start, kNumber) + ":" + e.expr(n.End, kNumber) + "]"
	case *ast.ListComp:
		return e.emitListComp(n)
	case *ast.Let:
		return e.emitLet(n, k)
	case *ast.Vector:
		// A SCAD vector is a []Number list. Geometry boundaries that need a
		// Vec2/Vec3 extract components or wrap with a scad_v* helper separately.
		parts := make([]string, len(n.Elems))
		for i, el := range n.Elems {
			parts[i] = e.expr(el, kNumber)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *ast.Index:
		return e.expr(n.X, kNumber) + "[" + e.expr(n.Index, kNumber) + "]"
	case *ast.Member:
		// OpenSCAD's .x/.y/.z on a vector are list indices in the []Number model.
		switch n.Name {
		case "x":
			return e.expr(n.X, kNumber) + "[0]"
		case "y":
			return e.expr(n.X, kNumber) + "[1]"
		case "z":
			return e.expr(n.X, kNumber) + "[2]"
		}
		return e.errf(n.Pos(), "member access .%s", n.Name)
	case *ast.Unary:
		return n.Op + e.operand(n.X, k)
	case *ast.Binary:
		// OpenSCAD arithmetic is unitless: render operands without a unit and
		// let the result coerce into the surrounding Length/Angle parameter.
		// Propagating the unit would wrongly tag an inner literal (e.g.
		// `base / 2` → `base / 2 mm`, i.e. base / (2 mm)).
		return fmt.Sprintf("%s %s %s", e.operand(n.L, kNumber), n.Op, e.operand(n.R, kNumber))
	case *ast.Ternary:
		// OpenSCAD's `cond ? then : else` maps directly to Facet's ternary. The
		// condition must be Bool (Facet enforces it; cond() rejects a non-boolean
		// one). The arms carry the surrounding kind so unit literals tag, and are
		// parenthesized when nested so the right-associative grouping is explicit.
		return e.cond(n.Cond) + " ? " + e.operand(n.Then, k) + " : " + e.operand(n.Else, k)
	}
	return e.errf(x.Pos(), "expr %T", x)
}

// operand renders x for use as an operand of a binary/unary expression,
// parenthesizing it when it is itself a binary or ternary so the original
// grouping is preserved (e.g. (a-b)*(a-b) must not flatten to a-b*a-b).
func (e *Emitter) operand(x ast.Expr, k kind) string {
	s := e.expr(x, k)
	switch x.(type) {
	case *ast.Binary, *ast.Ternary:
		return "(" + s + ")"
	}
	return s
}

// emitLet inlines an OpenSCAD `let(name = value, …) body`. Facet has no let
// expression, so the bindings are substituted into the body (see withLetBinds).
func (e *Emitter) emitLet(n *ast.Let, k kind) string {
	return e.withLetBinds(n.Binds, func() string { return e.expr(n.Body, k) })
}

// withLetBinds pushes let bindings onto e.letScope (each value emitted with the
// earlier bindings already in scope — bindings are sequential and pure), runs fn
// with them active, then restores the prior scope so they shadow only within the
// let. Used by both let-expressions and let list-comprehension elements.
func (e *Emitter) withLetBinds(binds []ast.Assign, fn func() string) string {
	if e.letScope == nil {
		e.letScope = map[string]string{}
	}
	prev := make(map[string]*string, len(binds))
	for _, b := range binds {
		if _, seen := prev[b.Name]; !seen {
			if old, ok := e.letScope[b.Name]; ok {
				v := old
				prev[b.Name] = &v
			} else {
				prev[b.Name] = nil
			}
		}
		e.letScope[b.Name] = "(" + e.expr(b.Value, kNumber) + ")"
	}
	out := fn()
	for name, old := range prev {
		if old == nil {
			delete(e.letScope, name)
		} else {
			e.letScope[name] = *old
		}
	}
	return out
}
