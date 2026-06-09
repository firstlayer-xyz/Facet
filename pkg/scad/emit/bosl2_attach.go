package emit

import (
	"fmt"
	"math"
	"strconv"
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

// anchorLit renders a direction vector as a B2Anchor literal for the runtime.
func anchorLit(d [3]int) string {
	return fmt.Sprintf("B2Anchor{x: %d, y: %d, z: %d}", d[0], d[1], d[2])
}

// anchorMode selects the per-geometry math BOSL2's _find_anchor uses to place an
// anchor on a shape. size in anchorOffset is always the full bounding-box extent
// per axis.
type anchorMode int

const (
	anchorBox     anchorMode = iota // bounding box: v·size/2 (cuboid, rect, prismoid/trapezoid at a fixed level)
	anchorSphere                     // sphere surface: r·unit(v), shift ÷ |v|
	anchorCyl                        // cylinder: radial r·unit(xy) (÷ |xy|), axial ±h/2
	anchorEllipse                    // 2D ellipse: where the ray along v meets the perimeter
)

// anchorOffset returns the trailing .Move that shifts a centered shape so the
// BOSL2 anchor v lands on the origin, per mode. size is the full per-axis extent;
// for anchorEllipse size[0]/size[1] are the x/y semi-axes (radii). Returns "" for
// CENTER.
func anchorOffset(v [3]int, size [3]string, mode anchorMode) string {
	if mode == anchorEllipse {
		return ellipseAnchorOffset(v, size[0], size[1])
	}
	// Per-axis shift coeff is -v[i] / (2·mag): mag normalizes the radial group so
	// v/mag is a unit direction (sphere = all three axes, cyl = the x/y pair),
	// while a box axis keeps mag = 1 (the bounding-box corner).
	mag := func(i int) float64 {
		switch mode {
		case anchorSphere:
			return math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]))
		case anchorCyl:
			if i < 2 {
				return math.Hypot(float64(v[0]), float64(v[1]))
			}
			return 1
		default:
			return 1
		}
	}
	axes := [3]string{"x", "y", "z"}
	var parts []string
	for i := 0; i < 3; i++ {
		if v[i] == 0 {
			continue
		}
		coeff := strconv.FormatFloat(-float64(v[i])/(2*mag(i)), 'g', -1, 64)
		parts = append(parts, fmt.Sprintf("%s: %s * (%s)", axes[i], coeff, size[i]))
	}
	if len(parts) == 0 {
		return ""
	}
	return ".Move(" + strings.Join(parts, ", ") + ")"
}

// ellipseAnchorOffset shifts a centered ellipse (semi-axes rx, ry) so its BOSL2
// anchor lands on the origin. BOSL2 anchors an ellipse where the ray in the anchor
// direction meets the perimeter: a cardinal anchor is the semi-axis endpoint
// (±rx or ±ry); a ±1/±1 diagonal lands at |x|=|y|=t with t = rx / sqrt(1 + (rx/ry)^2)
// (= rx·ry/sqrt(rx^2+ry^2)). Length*Length has no Facet type, so t is written with
// the dimensionless ratio rx/ry only.
func ellipseAnchorOffset(v [3]int, rx, ry string) string {
	switch {
	case v[0] == 0 && v[1] == 0:
		return ""
	case v[1] == 0: // cardinal x
		return ".Move(x: " + signedExpr(-v[0], rx) + ")"
	case v[0] == 0: // cardinal y
		return ".Move(y: " + signedExpr(-v[1], ry) + ")"
	default: // diagonal: both ±1
		ratio := "Number(from: " + rx + ") / Number(from: " + ry + ")"
		t := "(" + rx + ") / Sqrt(n: 1 + (" + ratio + ") * (" + ratio + "))"
		return ".Move(x: " + signedExpr(-v[0], t) + ", y: " + signedExpr(-v[1], t) + ")"
	}
}

// signedExpr prefixes expr with a minus when k is negative (k is ±1).
func signedExpr(k int, expr string) string {
	if k < 0 {
		return "-(" + expr + ")"
	}
	return "(" + expr + ")"
}

// applyAnchor appends the .Move that places a centered `shape` by its BOSL2
// anchor= argument (the anchor point lands on the origin), using mode's geometry.
// size is the full bounding-box extent per axis; an "" axis (a 2D shape's z) must
// not be anchored. twoD rejects an off-plane (TOP/BOTTOM) anchor. Without anchor=
// the shape is returned unchanged.
func (e *Emitter) applyAnchor(n *ast.ModuleCall, shape string, size [3]string, twoD bool, mode anchorMode) string {
	return e.applyAnchorFn(n, shape, twoD, mode, [3]int{}, func([3]int) [3]string { return size })
}

// applyAnchorFn is applyAnchor for shapes whose extent depends on the anchor
// itself — a prismoid/trapezoid samples its tapered width at the anchored level —
// so the size is supplied as sizeFn(v) once v is known. def is the shape's BOSL2
// DEFAULT anchor, applied when the call has no anchor= (most shapes default
// CENTER = [3]int{}; prismoid/rect_tube default BOTTOM, so they sit on the plate).
func (e *Emitter) applyAnchorFn(n *ast.ModuleCall, shape string, twoD bool, mode anchorMode, def [3]int, sizeFn func([3]int) [3]string) string {
	v := def
	if a, has := arg(n, "anchor", -1); has {
		var ok bool
		if v, ok = anchorVec(a); !ok {
			return e.errf(n.Pos(), "%s: unsupported anchor (use a named anchor or a ±1/0 vector)", n.Name)
		}
		if twoD && v[2] != 0 {
			return e.errf(n.Pos(), "%s: anchor must be in-plane (no TOP/BOTTOM on a 2D shape)", n.Name)
		}
	}
	return shape + anchorOffset(v, sizeFn(v), mode)
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

// applySpin appends a Rotate(z: spin) when the call carries a spin= angle —
// BOSL2's spin rotates the shape about its Z axis. BOSL2 applies it after anchor
// placement and before orient, so callers wrap the anchored shape with applySpin
// inside applyOrient.
func (e *Emitter) applySpin(n *ast.ModuleCall, shape string) string {
	s, has := arg(n, "spin", -1)
	if !has {
		return shape
	}
	return shape + ".Rotate(z: " + e.expr(s, kAngle) + ")"
}

// hasAttachArgs reports whether a call carries any BOSL2 attachable-transform
// argument (anchor/spin/orient). In a BOSL2 file the core cube/sphere/cylinder
// take the attachable path only when one is present; otherwise they keep their
// plain OpenSCAD origin.
func hasAttachArgs(n *ast.ModuleCall) bool {
	for _, name := range []string{"anchor", "spin", "orient"} {
		if _, has := arg(n, name, -1); has {
			return true
		}
	}
	return false
}

// bosl2CoreLeaf places a CENTERED core primitive (cube/sphere/cylinder — in a
// BOSL2 file these are BOSL2's attachable overrides of the OpenSCAD built-ins) by
// its BOSL2 anchor, then spin, then orient (BOSL2 order). `def` is the anchor used
// when none is given — the one that reproduces the primitive's OpenSCAD origin
// (cube ALLNEG/CENTER, cylinder BOTTOM/CENTER, sphere CENTER); an explicit anchor=
// overrides it. mode selects the _find_anchor geometry (anchorBox for the cube,
// anchorSphere for the sphere; the cylinder uses anchorCyl via cylinderAnchored).
func (e *Emitter) bosl2CoreLeaf(n *ast.ModuleCall, centered string, size [3]string, def [3]int, mode anchorMode) string {
	v := def
	if a, has := arg(n, "anchor", -1); has {
		var ok bool
		if v, ok = anchorVec(a); !ok {
			return e.errf(n.Pos(), "%s: unsupported anchor (use a named anchor or a ±1/0 vector)", n.Name)
		}
	}
	return e.applyOrient(n, e.applySpin(n, centered+anchorOffset(v, size, mode)))
}

// ngonAnchorMove appends the trailing .Move that places a regular `sides`-gon
// (built centered, with a vertex on +X and circumradius rExpr) so its BOSL2 anchor
// point lands on the origin. BOSL2 anchors a 2D polygon by its path (extent=false),
// so a vector anchor lands where the ray from the center crosses the polygon
// perimeter — computed on the unit polygon (which scales linearly with rExpr) and
// scaled by rExpr. An out-of-plane (z) anchor errors; ok=false means an error was
// recorded.
func (e *Emitter) ngonAnchorMove(n *ast.ModuleCall, sides int, rExpr string) (string, bool) {
	a, has := arg(n, "anchor", -1)
	if !has {
		return "", true
	}
	v, ok := anchorVec(a)
	if !ok {
		return e.errf(n.Pos(), "%s: unsupported anchor (use a named anchor or a ±1/0 vector)", n.Name), false
	}
	if v[2] != 0 {
		return e.errf(n.Pos(), "%s: anchor must be in-plane (no TOP/BOTTOM on a 2D shape)", n.Name), false
	}
	if v[0] == 0 && v[1] == 0 {
		return "", true // CENTER
	}
	ux, uy := ngonPerimeterPoint(sides, float64(v[0]), float64(v[1]))
	var parts []string
	if cx := coeffStr(-ux); cx != "0" {
		parts = append(parts, "x: "+cx+" * ("+rExpr+")")
	}
	if cy := coeffStr(-uy); cy != "0" {
		parts = append(parts, "y: "+cy+" * ("+rExpr+")")
	}
	if len(parts) == 0 {
		return "", true
	}
	return ".Move(" + strings.Join(parts, ", ") + ")", true
}

// ngonCenterMove returns the trailing .Move that recenters a corner-anchored
// regular `sides`-gon (built with a vertex on +X at circumradius rExpr, bbox min
// at the origin) on its construction origin — the circumcircle center, which is
// where BOSL2 anchors it (anchor=CENTER). Bounding-center only coincides with
// the construction center for even `sides`; for odd `sides` the polygon isn't
// centrally symmetric, so the offset (the construction bbox's min corner, a fixed
// coefficient of rExpr set by the vertex angles) must be moved out explicitly. A
// star reuses this: its extent is set by its outer vertices, which sit at the
// same angles as a `sides`-gon's.
func ngonCenterMove(sides int, rExpr string) string {
	minCos, minSin := math.Inf(1), math.Inf(1)
	for k := 0; k < sides; k++ {
		a := 2 * math.Pi * float64(k) / float64(sides)
		minCos = math.Min(minCos, math.Cos(a))
		minSin = math.Min(minSin, math.Sin(a))
	}
	// The construction origin sits at (-minCos*r, -minSin*r) in the corner-anchored
	// frame, so moving by (minCos*r, minSin*r) brings it back to (0,0).
	var parts []string
	if cx := coeffStr(minCos); cx != "0" {
		parts = append(parts, "x: "+cx+" * ("+rExpr+")")
	}
	if cy := coeffStr(minSin); cy != "0" {
		parts = append(parts, "y: "+cy+" * ("+rExpr+")")
	}
	if len(parts) == 0 {
		return ""
	}
	return ".Move(" + strings.Join(parts, ", ") + ")"
}

// ngonPerimeterPoint returns the point (on the unit-circumradius regular n-gon
// with a vertex on +X) where the ray from the center toward (dx,dy) crosses the
// perimeter. The boundary distance along angle φ is inradius/cos(φ−ψ), with ψ the
// nearest edge-normal angle (edge normals sit at the odd multiples of π/n, the
// bisectors between vertices at the even multiples).
func ngonPerimeterPoint(sides int, dx, dy float64) (float64, float64) {
	phi := math.Atan2(dy, dx)
	half := math.Pi / float64(sides)
	k := math.Round((phi/half - 1) / 2)
	psi := (2*k + 1) * half
	rho := math.Cos(half) / math.Cos(phi-psi)
	return rho * math.Cos(phi), rho * math.Sin(phi)
}

// coeffStr formats a numeric Move coefficient, rounding away floating-point dust
// so a vertex-aligned anchor emits a clean "1"/"-1" and an on-axis anchor drops
// its zero component.
func coeffStr(x float64) string {
	x = math.Round(x*1e9) / 1e9
	if x == 0 {
		return "0"
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
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


// attachSpec is one resolved attachment link. Every B2 placement op has a
// `.<method>(args)` form (unions the placed child into the parent's chain) and a
// `.<method>Placed(args)` twin (the placed child standalone, in world space).
// union() builds the former; placed() the latter — used when a tag surfaces the
// child out to an enclosing diff/intersect scope so it can cut other parents too.
// role is the child's CSG role (from its tags, inheriting the parent's otherwise).
type attachSpec struct {
	role   tagRole
	method string
	args   string
}

func (s attachSpec) union() string  { return "." + s.method + "(" + s.args + ")" }
func (s attachSpec) placed() string { return "." + s.method + "Placed(" + s.args + ")" }

// b2Link emits one attachment-chain link for the parent's own union chain (the
// non-scope path; tags are inert here, so everything unions).
func (e *Emitter) b2Link(c ast.Stmt) string {
	spec, ok := e.b2LinkSpec(c, nil, roleUntagged)
	if !ok {
		return ""
	}
	return spec.union()
}

// b2LinkSpec resolves one attachment child of an attachable parent into an
// attachSpec. cfg maps tag names to CSG roles; parentRole is inherited by an
// untagged child. The role combines the outer tag (on the link) and the inner
// child's own tag (innermost wins), matching BOSL2's $tag inheritance.
func (e *Emitter) b2LinkSpec(c ast.Stmt, cfg map[string]tagRole, parentRole tagRole) (attachSpec, bool) {
	raw, ok := c.(*ast.ModuleCall)
	if !ok {
		e.errf(c.Pos(), "attachment child must be a shape")
		return attachSpec{}, false
	}
	inner, outer := peelTags(raw)
	role := parentRole
	if r, ok := cfg[outer]; ok {
		role = r
	}
	mc, ok := inner.(*ast.ModuleCall)
	if !ok {
		e.errf(raw.Pos(), "tag without a child shape")
		return attachSpec{}, false
	}
	switch mc.Name {
	case "position":
		return e.b2PositionSpec(mc, cfg, role)
	case "attach":
		return e.b2AttachSpec(mc, cfg, role)
	case "align":
		return e.b2AlignSpec(mc, cfg, role)
	default:
		child, ok := e.b2ChildPrimitive(mc)
		if !ok {
			e.errf(mc.Pos(), "%s: not an attachable shape", mc.Name)
			return attachSpec{}, false
		}
		return attachSpec{role: role, method: "position", args: "a: B2Anchor{x: 0, y: 0, z: 0}, child: " + child}, true
	}
}

// b2PositionSpec resolves a position(anchor) child.
func (e *Emitter) b2PositionSpec(n *ast.ModuleCall, cfg map[string]tagRole, role tagRole) (attachSpec, bool) {
	a, ok := arg(n, "", 0)
	if !ok {
		e.errf(n.Pos(), "position without an anchor")
		return attachSpec{}, false
	}
	dir, ok := anchorVec(a)
	if !ok {
		e.errf(n.Pos(), "position: unsupported anchor expression")
		return attachSpec{}, false
	}
	child, crole, ok := e.b2ChildSpec(n, cfg, role)
	if !ok {
		e.errf(n.Pos(), "position: child is not an attachable shape")
		return attachSpec{}, false
	}
	return attachSpec{role: crole, method: "position", args: "a: " + anchorLit(dir) + ", child: " + child}, true
}

// b2AttachSpec resolves an attach(pa[, ca][, overlap]) child. The two-anchor form
// mates the child's ca anchor onto the parent's pa (rotating ca to face opposite
// pa); the single-anchor form reorients the child to point out the pa face.
func (e *Emitter) b2AttachSpec(n *ast.ModuleCall, cfg map[string]tagRole, role tagRole) (attachSpec, bool) {
	e.rejectExtraArgs(n, 3, "overlap")
	pa, ok := arg(n, "", 0)
	if !ok {
		e.errf(n.Pos(), "attach without an anchor")
		return attachSpec{}, false
	}
	pdir, ok := anchorVec(pa)
	if !ok {
		e.errf(n.Pos(), "attach: unsupported parent anchor")
		return attachSpec{}, false
	}
	child, crole, ok := e.b2ChildSpec(n, cfg, role)
	if !ok {
		e.errf(n.Pos(), "attach: child is not an attachable shape")
		return attachSpec{}, false
	}
	overlap := "0 mm"
	if o, ok := arg(n, "overlap", 2); ok {
		overlap = e.expr(o, kLength)
	}
	if ca, has := arg(n, "", 1); has {
		cdir, ok := anchorVec(ca)
		if !ok {
			e.errf(n.Pos(), "attach: unsupported child anchor")
			return attachSpec{}, false
		}
		return attachSpec{role: crole, method: "attach",
			args: "pa: " + anchorLit(pdir) + ", ca: " + anchorLit(cdir) + ", child: " + child + ", overlap: " + overlap}, true
	}
	return attachSpec{role: crole, method: "attachReorient",
		args: "pa: " + anchorLit(pdir) + ", child: " + child + ", overlap: " + overlap}, true
}

// b2AlignSpec resolves an align(anchor, [inside=]) child. inside=true seats the
// child inside the parent and, per BOSL2, defaults its tag to "remove" — so under
// a subtractive (diff) scope an untagged inside child becomes a remove.
func (e *Emitter) b2AlignSpec(n *ast.ModuleCall, cfg map[string]tagRole, role tagRole) (attachSpec, bool) {
	e.rejectExtraArgs(n, 1, "inside")
	a, ok := arg(n, "", 0)
	if !ok {
		e.errf(n.Pos(), "align without an anchor")
		return attachSpec{}, false
	}
	dir, ok := anchorVec(a)
	if !ok {
		e.errf(n.Pos(), "align: unsupported anchor expression")
		return attachSpec{}, false
	}
	push := "1"
	inside := false
	if ins, has := arg(n, "inside", -1); has {
		b, isBool := ins.(*ast.Bool)
		if !isBool {
			e.errf(n.Pos(), "align: inside must be a literal true or false")
			return attachSpec{}, false
		}
		inside = b.Val
		if inside {
			push = "-1"
		}
	}
	child, crole, ok := e.b2ChildSpec(n, cfg, role)
	if !ok {
		e.errf(n.Pos(), "align: child is not an attachable shape")
		return attachSpec{}, false
	}
	// inside=true defaults an otherwise-untagged child to the scope's remove role.
	if inside && crole == roleUntagged {
		if r, ok := cfg["remove"]; ok {
			crole = r
		}
	}
	return attachSpec{role: crole, method: "align", args: "a: " + anchorLit(dir) + ", child: " + child + ", dir: " + push}, true
}

// b2ChildSpec resolves the single geometry child of a position/attach/align node:
// the emitted B2, plus its CSG role (its own tag, innermost-wins, else the role
// inherited from the link). ok is false if it is not an attachable shape.
func (e *Emitter) b2ChildSpec(n *ast.ModuleCall, cfg map[string]tagRole, fallback tagRole) (string, tagRole, bool) {
	raw, ok := singleChildCall(n)
	if !ok {
		return "", 0, false
	}
	inner, tag := peelTags(raw)
	role := fallback
	if r, ok := cfg[tag]; ok {
		role = r
	}
	mc, ok := inner.(*ast.ModuleCall)
	if !ok {
		return "", 0, false
	}
	child, ok := e.b2ChildPrimitive(mc)
	return child, role, ok
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
