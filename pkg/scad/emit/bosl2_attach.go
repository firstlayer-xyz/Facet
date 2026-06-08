package emit

import (
	"fmt"

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

// negDir returns the opposite direction vector.
func negDir(d [3]int) [3]int { return [3]int{-d[0], -d[1], -d[2]} }

// anchorLit renders a direction vector as a B2Anchor literal for the runtime.
func anchorLit(d [3]int) string {
	return fmt.Sprintf("B2Anchor{x: %d, y: %d, z: %d}", d[0], d[1], d[2])
}

// anchorVec3Lit renders a direction vector as a Facet Vec3 literal (mm units).
func anchorVec3Lit(d [3]int) string {
	return fmt.Sprintf("Vec3{x: %d mm, y: %d mm, z: %d mm}", d[0], d[1], d[2])
}

// applyOrient appends a Rotate(from: UP, to: orient) when the call carries an
// orient= anchor — BOSL2's orient= points the shape's +Z (UP) axis along that
// direction. The shape must already be centered on the origin (orient rotates it
// in place).
func (e *Emitter) applyOrient(n *ast.ModuleCall, shape string) string {
	o, has := arg(n, "orient", -1)
	if !has {
		return shape
	}
	dir, ok := anchorVec(o)
	if !ok {
		return e.errf(n.Pos(), "%s: unsupported orient anchor", n.Name)
	}
	return shape + ".Rotate(from: Vec3{z: 1 mm}, to: " + anchorVec3Lit(dir) + ")"
}

// bosl2AttachGuard blocks a leaf shape that has children — in BOSL2 a child of a
// shape is an attachment, and only cuboid/cyl carry a known attachment geometry
// (handled by bosl2AttachChain). Any other shape with children (a bare child, or
// a position/attach child) would have those children silently dropped by the
// leaf emitter, so the guard turns that into a located error instead (no
// fallbacks). Transforms, distributors, and attachment containers are not leaf
// shapes, so their children pass through to their own emitters.
func (e *Emitter) bosl2AttachGuard(n *ast.ModuleCall) (string, bool) {
	if len(n.Children) > 0 && isLeafShape(n.Name) {
		return e.errf(n.Pos(), "%s: attachments are only supported on cuboid and cyl", n.Name), true
	}
	return "", false
}

// isLeafShape reports whether a module is a leaf geometry primitive — one that
// never wraps children — as opposed to a transform, distributor, or attachment
// container. cuboid and cyl are deliberately excluded: they ARE attachment
// parents and carry their children as a B2 attachment chain.
func isLeafShape(name string) bool {
	switch name {
	case "cube", "sphere", "cylinder", "circle", "square", "polygon", "text", "polyhedron",
		"tube", "xcyl", "ycyl", "zcyl", "torus", "rect_tube", "rect",
		"prismoid", "wedge", "spheroid", "regular_ngon", "hexagon", "pentagon", "octagon", "star":
		return true
	}
	return false
}

// bosl2AttachChain emits a parent shape with attachment children as a B2 method
// chain into the BOSL2 runtime, extracted to a Solid at the end:
//
//	b2_cuboid(size: …).attach(…).position(…).Solid()
//
// The geometry math (anchor points, placement) lives in the Facet runtime; the
// transpiler only resolves anchors and wires up the calls.
func (e *Emitter) bosl2AttachChain(n *ast.ModuleCall) string {
	parent, ok := e.bosl2PrimitiveB2(n)
	if !ok {
		return e.errf(n.Pos(), "%s cannot carry attachments", n.Name)
	}
	e.usesBosl2Runtime = true
	chain := parent
	for _, c := range n.Children {
		chain += e.b2Link(c)
	}
	return chain + ".Solid()"
}

// tagValue returns the string tag of a tag()/tag_this()/force_tag() call.
func tagValue(mc *ast.ModuleCall) string {
	if v, ok := arg(mc, "tag", 0); ok {
		if s, ok := v.(*ast.Str); ok {
			return s.Value
		}
	}
	return ""
}

// unwrapTags peels any leading tag() wrappers off a call, returning the inner
// geometry call and whether a "remove" tag was seen while inside a diff() (which
// means the geometry should be subtracted). Outside diff(), tags are inert.
func (e *Emitter) unwrapTags(mc *ast.ModuleCall) (*ast.ModuleCall, bool) {
	removed := false
	for mc != nil && (mc.Name == "tag" || mc.Name == "tag_this" || mc.Name == "force_tag") {
		if e.inDiff && tagValue(mc) == "remove" {
			removed = true
		}
		inner, ok := singleChildCall(mc)
		if !ok {
			return nil, removed
		}
		mc = inner
	}
	return mc, removed
}

// b2Link emits one attachment-chain link for a child of an attachment parent:
// `.position(...)` / `.attach(...)`, or — for a plain (non-position/attach)
// child — `.position(...)` at the CENTER anchor (BOSL2 places bare children at
// the parent origin). A leading `tag("remove")` inside diff() turns the union
// into a subtraction (the *Remove variants).
func (e *Emitter) b2Link(c ast.Stmt) string {
	raw, ok := c.(*ast.ModuleCall)
	if !ok {
		return e.errf(c.Pos(), "attachment child must be a shape")
	}
	mc, removed := e.unwrapTags(raw)
	if mc == nil {
		return e.errf(raw.Pos(), "tag without a child shape")
	}
	switch mc.Name {
	case "position":
		return e.b2PositionLink(mc, removed)
	case "attach":
		return e.b2AttachLink(mc, removed)
	default:
		child, ok := e.b2ChildPrimitive(mc)
		if !ok {
			return e.errf(mc.Pos(), "%s: not an attachable shape", mc.Name)
		}
		return "." + pick(removed, "positionRemove", "position") +
			"(a: B2Anchor{x: 0, y: 0, z: 0}, child: " + child + ")"
	}
}

// pick returns r when cond is true, else u — chooses the Remove vs union method.
func pick(cond bool, r, u string) string {
	if cond {
		return r
	}
	return u
}

// b2PositionLink emits `.position(a: <anchor>, child: <B2>)` (or .positionRemove
// when the child is remove-tagged inside a diff()).
func (e *Emitter) b2PositionLink(n *ast.ModuleCall, removedOuter bool) string {
	a, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "position without an anchor")
	}
	dir, ok := anchorVec(a)
	if !ok {
		return e.errf(n.Pos(), "position: unsupported anchor expression")
	}
	child, removedChild, ok := e.b2ChildOf(n)
	if !ok {
		return e.errf(n.Pos(), "position: child is not an attachable shape")
	}
	return "." + pick(removedOuter || removedChild, "positionRemove", "position") +
		"(a: " + anchorLit(dir) + ", child: " + child + ")"
}

// b2AttachLink emits one attach link. The two-anchor form attach(P, C) is the
// no-reorientation case — C must be anti-parallel to P (e.g. attach(TOP, BOTTOM))
// — and emits B2.attach. The single-anchor form attach(P) reorients the child to
// point out the P anchor (any direction, including combined edge/corner anchors)
// and emits B2.attachReorient. Non-opposite two-anchor attach (a general child
// rotation) is still a located error.
func (e *Emitter) b2AttachLink(n *ast.ModuleCall, removedOuter bool) string {
	pa, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "attach without an anchor")
	}
	pdir, ok := anchorVec(pa)
	if !ok {
		return e.errf(n.Pos(), "attach: unsupported parent anchor")
	}
	child, removedChild, ok := e.b2ChildOf(n)
	if !ok {
		return e.errf(n.Pos(), "attach: child is not an attachable shape")
	}
	removed := removedOuter || removedChild

	if ca, has := arg(n, "", 1); has {
		cdir, ok := anchorVec(ca)
		if !ok {
			return e.errf(n.Pos(), "attach: unsupported child anchor")
		}
		if cdir != negDir(pdir) {
			return e.errf(n.Pos(), "attach: reorienting the child (non-opposite anchors) is not yet supported")
		}
		return "." + pick(removed, "attachRemove", "attach") +
			"(pa: " + anchorLit(pdir) + ", ca: " + anchorLit(cdir) + ", child: " + child + ")"
	}

	return "." + pick(removed, "attachReorientRemove", "attachReorient") +
		"(pa: " + anchorLit(pdir) + ", child: " + child + ")"
}

// b2ChildOf emits the single geometry child of a position/attach node as a B2,
// peeling any tag() wrappers. It returns the emitted child, whether a "remove"
// tag (inside diff()) applied to it, and ok=false if it is not an attachable
// shape.
func (e *Emitter) b2ChildOf(n *ast.ModuleCall) (child string, removed bool, ok bool) {
	raw, ok := singleChildCall(n)
	if !ok {
		return "", false, false
	}
	mc, removed := e.unwrapTags(raw)
	if mc == nil {
		return "", removed, false
	}
	child, ok = e.b2ChildPrimitive(mc)
	return child, removed, ok
}

// b2ChildPrimitive emits an attached child shape as a B2 constructor, rejecting a
// child that itself carries attachments. Nested attachment chains aren't
// supported, and bosl2PrimitiveB2 ignores a shape's children, so without this
// the inner attachment would be silently dropped.
func (e *Emitter) b2ChildPrimitive(mc *ast.ModuleCall) (string, bool) {
	if len(mc.Children) > 0 {
		return e.errf(mc.Pos(), "%s: nested attachments are not supported (an attached child cannot itself carry attachments)", mc.Name), true
	}
	return e.bosl2PrimitiveB2(mc)
}

// singleChildCall returns the single child geometry call of an attachment node,
// or ok=false when there is not exactly one child module call.
func singleChildCall(n *ast.ModuleCall) (*ast.ModuleCall, bool) {
	if len(n.Children) != 1 {
		return nil, false
	}
	mc, ok := n.Children[0].(*ast.ModuleCall)
	return mc, ok
}

// bosl2PrimitiveB2 emits a BOSL2 primitive as a B2 constructor call
// (b2_cuboid/b2_cyl/b2_sphere). ok is false for shapes with no attachment
// geometry. Options we don't translate (cones, rounding, chamfer, anchors) are
// located errors via rejectExtraArgs.
func (e *Emitter) bosl2PrimitiveB2(mc *ast.ModuleCall) (string, bool) {
	switch mc.Name {
	case "cuboid":
		e.rejectExtraArgs(mc, 1, "size")
		size, ok := arg(mc, "size", 0)
		if !ok {
			return e.errf(mc.Pos(), "cuboid without size"), true
		}
		x, y, z := e.boxSizeComponents(size)
		return fmt.Sprintf("b2_cuboid(size: Vec3{x: %s, y: %s, z: %s})", x, y, z), true
	case "cyl":
		e.rejectExtraArgs(mc, 2, "h", "l", "height", "r", "d", "$fn", "$fa", "$fs")
		h, ok := cylHeightArg(mc)
		if !ok {
			return e.errf(mc.Pos(), "cyl without height"), true
		}
		r, ok := e.radiusHalf(mc, 1)
		if !ok {
			return e.errf(mc.Pos(), "cyl without radius"), true
		}
		return fmt.Sprintf("b2_cyl(h: %s, r: %s)", e.expr(h, kLength), r), true
	case "sphere":
		e.rejectExtraArgs(mc, 1, "r", "d", "$fn", "$fa", "$fs")
		r, ok := e.radiusHalf(mc, 0)
		if !ok {
			return e.errf(mc.Pos(), "sphere without radius"), true
		}
		return fmt.Sprintf("b2_sphere(r: %s)", r), true
	}
	return "", false
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
