package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// anchorVecs maps BOSL2 anchor-direction constants to unit direction vectors.
// A direction component is -1, 0, or +1 along each axis; combinations like
// TOP+RIGHT are formed by vector addition (see anchorVec).
var anchorVecs = map[string][3]int{
	"RIGHT": {1, 0, 0}, "LEFT": {-1, 0, 0},
	"BACK": {0, 1, 0}, "FWD": {0, -1, 0}, "FRONT": {0, -1, 0},
	"UP": {0, 0, 1}, "DOWN": {0, 0, -1},
	"TOP": {0, 0, 1}, "BOTTOM": {0, 0, -1}, "BOT": {0, 0, -1},
	"CENTER": {0, 0, 0}, "CTR": {0, 0, 0},
}

// anchorVec resolves a BOSL2 anchor expression to a direction vector at
// transpile time. It accepts an anchor constant (UP/TOP/…), a sum of anchors
// (TOP+RIGHT), or a literal 3-vector of ±1/0 components. ok is false otherwise.
func anchorVec(x ast.Expr) ([3]int, bool) {
	switch n := x.(type) {
	case *ast.Ident:
		v, ok := anchorVecs[n.Name]
		return v, ok
	case *ast.Binary:
		if n.Op != "+" {
			return [3]int{}, false
		}
		a, ok1 := anchorVec(n.L)
		b, ok2 := anchorVec(n.R)
		if !ok1 || !ok2 {
			return [3]int{}, false
		}
		return [3]int{a[0] + b[0], a[1] + b[1], a[2] + b[2]}, true
	case *ast.Vector:
		if len(n.Elems) != 3 {
			return [3]int{}, false
		}
		var out [3]int
		for i, el := range n.Elems {
			f, ok := numLitValue(el)
			if !ok {
				return [3]int{}, false
			}
			out[i] = int(f)
		}
		return out, true
	}
	return [3]int{}, false
}

// boxGeom is the attachment geometry of a centered box: each axis spans
// ±half[i], so an anchor in direction d sits at d[i]·half[i]. half holds the
// rendered half-extent Length expressions ("20 mm / 2").
type boxGeom struct {
	half [3]string
}

// newBoxGeom builds a boxGeom from rendered full-size Length components.
func newBoxGeom(x, y, z string) boxGeom {
	return boxGeom{half: [3]string{x + " / 2", y + " / 2", z + " / 2"}}
}

// anchorPoint renders the (x, y, z) Length offsets of an anchor direction on the
// box. A zero component renders "" so callers can drop it from a Move.
func (g boxGeom) anchorPoint(dir [3]int) (x, y, z string) {
	axis := func(i int) string {
		switch dir[i] {
		case 1:
			return g.half[i]
		case -1:
			return "-(" + g.half[i] + ")"
		default:
			return ""
		}
	}
	return axis(0), axis(1), axis(2)
}

// bosl2AttachGuard blocks a shape that has position/attach children but is not a
// supported attachment parent (only cuboid/cyl route children through
// withAttachments). Without this, such children would be silently dropped — the
// guard turns that into a located error instead (no fallbacks).
func (e *Emitter) bosl2AttachGuard(n *ast.ModuleCall) (string, bool) {
	if n.Name == "cuboid" || n.Name == "cyl" {
		return "", false
	}
	for _, c := range n.Children {
		if mc, ok := c.(*ast.ModuleCall); ok && (mc.Name == "position" || mc.Name == "attach") {
			return e.errf(mc.Pos(), "%s: attachments (position/attach) are only supported on cuboid and cyl", mc.Name), true
		}
	}
	return "", false
}

// withAttachments emits a parent shape unioned with its attachment children.
// `position`/`attach` children are placed relative to the parent geometry `g`;
// any other child is plain geometry placed at the parent origin (anchor CENTER)
// and unioned as-is. With no children the base is returned unchanged.
func (e *Emitter) withAttachments(base string, g boxGeom, children []ast.Stmt) string {
	if len(children) == 0 {
		return base
	}
	parts := []string{base}
	for _, c := range children {
		if mc, ok := c.(*ast.ModuleCall); ok {
			switch mc.Name {
			case "position":
				parts = append(parts, e.bosl2Position(mc, g))
				continue
			case "attach":
				parts = append(parts, e.bosl2Attach(mc, g))
				continue
			}
		}
		parts = append(parts, e.stmt(c))
	}
	return unionParts(parts)
}

// bosl2Position emits `position(anchor) child`: the child's origin is moved to
// the parent's anchor point. No reorientation (that is attach's role).
func (e *Emitter) bosl2Position(n *ast.ModuleCall, g boxGeom) string {
	e.rejectExtraArgs(n, 1)
	a, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "position without an anchor")
	}
	dir, ok := anchorVec(a)
	if !ok {
		return e.errf(n.Pos(), "position: unsupported anchor expression")
	}
	x, y, z := g.anchorPoint(dir)
	return e.childExpr(n) + moveSuffix(x, y, z)
}

// bosl2Attach emits `attach(parent_anchor[, child_anchor]) child`: the child's
// child_anchor point is moved onto the parent's parent_anchor point. v1 supports
// only the no-reorientation case — child_anchor anti-parallel to parent_anchor
// (e.g. attach(TOP, BOTTOM)), and the single-anchor shorthand attach(TOP)/attach(UP)
// — so the move is a pure translation. Cases needing the child rotated, and
// children whose size isn't known, are located errors (no fallbacks).
func (e *Emitter) bosl2Attach(n *ast.ModuleCall, parent boxGeom) string {
	pa, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "attach without an anchor")
	}
	pdir, ok := anchorVec(pa)
	if !ok {
		return e.errf(n.Pos(), "attach: unsupported parent anchor")
	}

	cdir := negDir(pdir)
	if ca, has := arg(n, "", 1); has {
		cdir, ok = anchorVec(ca)
		if !ok {
			return e.errf(n.Pos(), "attach: unsupported child anchor")
		}
		if cdir != negDir(pdir) {
			return e.errf(n.Pos(), "attach: reorienting the child (non-opposite anchors) is not yet supported")
		}
	} else if pdir != ([3]int{0, 0, 1}) {
		return e.errf(n.Pos(), "attach: single-anchor attach is only supported for TOP/UP")
	}

	mc, ok := singleChildCall(n)
	if !ok {
		return e.errf(n.Pos(), "attach: child must be a single shape")
	}
	child, ok := e.childBoxGeom(mc)
	if !ok {
		return e.errf(n.Pos(), "attach: child '%s' has no known size", mc.Name)
	}

	x := combineOffset(pdir[0], parent.half[0], cdir[0], child.half[0])
	y := combineOffset(pdir[1], parent.half[1], cdir[1], child.half[1])
	z := combineOffset(pdir[2], parent.half[2], cdir[2], child.half[2])
	return e.childExpr(n) + moveSuffix(x, y, z)
}

// negDir returns the opposite direction vector.
func negDir(d [3]int) [3]int { return [3]int{-d[0], -d[1], -d[2]} }

// singleChildCall returns the single child geometry call of an attachment node,
// or ok=false when there is not exactly one child module call.
func singleChildCall(n *ast.ModuleCall) (*ast.ModuleCall, bool) {
	if len(n.Children) != 1 {
		return nil, false
	}
	mc, ok := n.Children[0].(*ast.ModuleCall)
	return mc, ok
}

// childBoxGeom returns the axis-aligned attachment geometry of a BOSL2 child
// primitive (its bounding half-extents), used to find the child's anchor point.
// ok is false for shapes whose size we can't determine.
func (e *Emitter) childBoxGeom(mc *ast.ModuleCall) (boxGeom, bool) {
	switch mc.Name {
	case "cuboid":
		size, ok := arg(mc, "size", 0)
		if !ok {
			return boxGeom{}, false
		}
		x, y, z := e.boxSizeComponents(size)
		return newBoxGeom(x, y, z), true
	case "cyl":
		h, ok := cylHeightArg(mc)
		if !ok {
			return boxGeom{}, false
		}
		r, ok := e.radiusHalf(mc, 1)
		if !ok {
			return boxGeom{}, false
		}
		return boxGeom{half: [3]string{r, r, e.expr(h, kLength) + " / 2"}}, true
	case "sphere":
		r, ok := e.radiusHalf(mc, 0)
		if !ok {
			return boxGeom{}, false
		}
		return boxGeom{half: [3]string{r, r, r}}, true
	}
	return boxGeom{}, false
}

// radiusHalf renders a primitive's radial half-extent (its r, or d/2) as a
// Length expression, preferring named r/d then the positional arg at posIdx.
func (e *Emitter) radiusHalf(n *ast.ModuleCall, posIdx int) (string, bool) {
	key, val, ok := e.radiusArg(n, posIdx)
	if !ok {
		return "", false
	}
	if key == "d" {
		return val + " / 2", true
	}
	return val, true
}

// combineOffset renders the per-axis translation that moves the child's anchor
// point onto the parent's: parentSign·parentHalf − childSign·childHalf. A zero
// result renders "" so moveSuffix can drop the axis.
func combineOffset(parentSign int, parentHalf string, childSign int, childHalf string) string {
	var b strings.Builder
	switch parentSign {
	case 1:
		b.WriteString(parentHalf)
	case -1:
		b.WriteString("-(" + parentHalf + ")")
	}
	// Subtract childSign·childHalf, i.e. add (−childSign)·childHalf.
	switch add := -childSign; {
	case add == 0:
	case b.Len() == 0 && add == 1:
		b.WriteString(childHalf)
	case b.Len() == 0 && add == -1:
		b.WriteString("-(" + childHalf + ")")
	case add == 1:
		b.WriteString(" + " + childHalf)
	case add == -1:
		b.WriteString(" - " + childHalf)
	}
	return b.String()
}

// moveSuffix renders a `.Move(...)` for the non-empty axis offsets, or "" when
// every offset is empty (the anchor is CENTER — no move needed).
func moveSuffix(x, y, z string) string {
	var parts []string
	for _, p := range []struct{ name, val string }{{"x", x}, {"y", y}, {"z", z}} {
		if p.val != "" {
			parts = append(parts, p.name+": "+p.val)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ".Move(" + strings.Join(parts, ", ") + ")"
}
