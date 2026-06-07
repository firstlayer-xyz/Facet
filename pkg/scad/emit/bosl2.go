package emit

import (
	"fmt"
	"regexp"
	"strings"

	"facet/pkg/scad/ast"
)

// bosl2PathRe matches a BOSL2 library reference path, e.g. "BOSL2/std.scad" or
// "BOSL2/shapes/cuboid.scad". BOSL2 is consumed via `include <BOSL2/...>`, so
// the leading "BOSL2/" segment (case-insensitive) is the signal.
var bosl2PathRe = regexp.MustCompile(`(?i)(^|/)BOSL2/`)

// libRef handles an OpenSCAD `include <path>` / `use <path>`. A BOSL2 path is a
// gate: it turns on the BOSL2 vocabulary and is otherwise dropped (the runtime
// is supplied by the emitted preamble, not by resolving the file). Any other
// path cannot be resolved — that is a located error, never a silent drop.
func (e *Emitter) libRef(path string, pos ast.Pos) {
	if bosl2PathRe.MatchString(path) {
		e.bosl2 = true
		return
	}
	e.errf(pos, "cannot resolve <%s>: only the BOSL2 library is supported", path)
}

// bosl2Call dispatches a BOSL2 instantiation to its Facet emission. It returns
// (emission, true) for a recognized BOSL2 name and ("", false) otherwise, so
// unrecognized names fall through to the caller's unknown-module error. Only
// reached when the program included BOSL2 (see moduleCall).
func (e *Emitter) bosl2Call(n *ast.ModuleCall) (string, bool) {
	switch n.Name {
	case "cuboid":
		return e.bosl2Cuboid(n), true
	case "cyl":
		return e.bosl2Cyl(n), true
	case "tube":
		return e.bosl2Tube(n), true
	case "zcyl":
		return e.bosl2OrientedCyl(n, ""), true
	case "xcyl":
		return e.bosl2OrientedCyl(n, "Rotate(y: 90 deg)"), true
	case "ycyl":
		return e.bosl2OrientedCyl(n, "Rotate(x: -90 deg)"), true
	case "torus":
		return e.bosl2Torus(n), true
	case "rect_tube":
		return e.bosl2RectTube(n), true
	case "rect":
		return e.bosl2Rect(n), true
	case "prismoid":
		return e.bosl2Prismoid(n), true
	case "wedge":
		return e.bosl2Wedge(n), true
	case "regular_ngon":
		return e.bosl2RegularNgon(n, "", 1), true
	case "hexagon":
		return e.bosl2RegularNgon(n, "6", 0), true
	case "pentagon":
		return e.bosl2RegularNgon(n, "5", 0), true
	case "octagon":
		return e.bosl2RegularNgon(n, "8", 0), true
	case "star":
		return e.bosl2Star(n), true
	case "position", "attach":
		// Reached only outside a supported parent (top level, or under a
		// transform); inside cuboid/cyl these are handled by withAttachments.
		return e.errf(n.Pos(), "%s requires an attachable parent shape (cuboid/cyl)", n.Name), true
	// single-axis translations (a scalar distance applied to one signed axis)
	case "up":
		return e.bosl2AxisMove(n, "z", +1), true
	case "down":
		return e.bosl2AxisMove(n, "z", -1), true
	case "right":
		return e.bosl2AxisMove(n, "x", +1), true
	case "left":
		return e.bosl2AxisMove(n, "x", -1), true
	case "back":
		return e.bosl2AxisMove(n, "y", +1), true
	case "fwd":
		return e.bosl2AxisMove(n, "y", -1), true
	case "move":
		// BOSL2's vector translation is the same mapping as OpenSCAD translate.
		return e.childExpr(n) + "." + e.translateMethod(n, e.childIs2D(n)), true
	// single-axis rotations
	case "xrot":
		return e.bosl2AxisRot(n, "x"), true
	case "yrot":
		return e.bosl2AxisRot(n, "y"), true
	case "zrot":
		return e.bosl2AxisRot(n, "z"), true
	case "rot":
		return e.bosl2Rot(n), true
	case "xscale":
		return e.bosl2AxisScale(n, "x"), true
	case "yscale":
		return e.bosl2AxisScale(n, "y"), true
	case "zscale":
		return e.bosl2AxisScale(n, "z"), true
	// linear distributors (n copies spaced along one axis, centered)
	case "xcopies":
		return e.bosl2LinearCopies(n, "x"), true
	case "ycopies":
		return e.bosl2LinearCopies(n, "y"), true
	case "zcopies":
		return e.bosl2LinearCopies(n, "z"), true
	case "line_copies", "line_of":
		return e.bosl2LineCopies(n), true
	case "zrot_copies", "rot_copies":
		return e.bosl2RotCopies(n), true
	case "arc_copies", "arc_of":
		return e.bosl2ArcCopies(n), true
	case "grid_copies", "grid2d":
		return e.bosl2GridCopies(n), true
	// mirror-and-keep copies
	case "xflip_copy":
		return e.bosl2FlipCopy(n, "x"), true
	case "yflip_copy":
		return e.bosl2FlipCopy(n, "y"), true
	case "zflip_copy":
		return e.bosl2FlipCopy(n, "z"), true
	case "mirror_copy":
		return e.bosl2MirrorCopy(n), true
	// subtractive modeling
	case "diff":
		return e.bosl2Diff(n), true
	case "tag", "tag_this", "force_tag":
		// Outside an attachable parent (top level / under a transform) a tag is
		// inert — its effect on diff() is resolved where the parent emits its
		// attachment children (see unwrapTags). Emit the child geometry.
		return e.childExpr(n), true
	}
	return "", false
}

// bosl2Diff emits BOSL2's diff(): it renders its child with subtractive tagging
// active, so a `tag("remove")` attachment is subtracted from its parent rather
// than unioned. Only the default remove tag ("remove") is supported.
func (e *Emitter) bosl2Diff(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 0)
	prev := e.inDiff
	e.inDiff = true
	out := e.childExpr(n)
	e.inDiff = prev
	if out == "" {
		return e.errf(n.Pos(), "diff() has no child geometry")
	}
	return out
}

// bosl2FlipCopy emits a BOSL2 single-axis mirror-and-keep (xflip_copy/yflip_copy/
// zflip_copy): the child plus a copy mirrored across the plane normal to `axis`.
// An `offset` shifts the mirror plane (reflection across axis = offset).
func (e *Emitter) bosl2FlipCopy(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 1, "offset")
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	mirror := child + ".Mirror(" + axis + ": 1)"
	if off, ok := arg(n, "offset", 0); ok {
		mirror += ".Move(" + axis + ": 2 * (" + e.expr(off, kLength) + "))"
	}
	return "(" + child + " + " + mirror + ")"
}

// bosl2MirrorCopy emits BOSL2's mirror_copy(v): the child plus a copy mirrored
// across the plane with normal v. Reuses the OpenSCAD mirror mapping.
func (e *Emitter) bosl2MirrorCopy(n *ast.ModuleCall) string {
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "mirror_copy has no child geometry")
	}
	m := e.mirrorMethod(n, e.childIs2D(n))
	if m == "" {
		return child
	}
	return "(" + child + " + " + child + "." + m + ")"
}

// bosl2GridCopies emits BOSL2's grid_copies: an n-by-n (or [nx,ny]) grid of the
// child on the XY plane, centered, as a nested for-comprehension. spacing and n
// accept a scalar (both axes) or a 2-vector (per axis).
func (e *Emitter) bosl2GridCopies(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "spacing", "n", "size")
	cnt, ok := arg(n, "n", 1)
	if !ok {
		return e.errf(n.Pos(), "grid_copies needs a count n")
	}
	sp, ok := arg(n, "spacing", 0)
	if !ok {
		return e.errf(n.Pos(), "grid_copies needs a spacing")
	}
	nx, ny := e.pair2(cnt, kNumber)
	sx, sy := e.pair2(sp, kLength)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "grid_copies has no child geometry")
	}
	vi := e.freshLoopVar()
	vj := e.freshLoopVar()
	ox := "(" + vi + " - (" + nx + " - 1) / 2) * " + sx
	oy := "(" + vj + " - (" + ny + " - 1) / 2) * " + sy
	return "Union(arr: for " + vi + " [0:" + nx + " - 1], " + vj + " [0:" + ny +
		" - 1] { yield " + child + ".Move(x: " + ox + ", y: " + oy + ") })"
}

// bosl2Rot emits BOSL2's general rotation rot(): a scalar spins about Z, a
// vector is an euler rotation. It reuses the OpenSCAD rotate mapping, so the
// axis-angle (v=) and pivot (cp=) forms are located errors, same as rotate.
func (e *Emitter) bosl2Rot(n *ast.ModuleCall) string {
	child := e.childExpr(n)
	m := e.rotateMethod(n, e.childIs2D(n))
	if m == "" {
		return child
	}
	return child + "." + m
}

// bosl2ArcCopies emits BOSL2's arc_copies/arc_of: n copies of the child placed
// on a circular arc of radius r, each moved out and rotated to face outward
// (rot=true). With no sa/ea it spans a full circle (angle = i*360/n); with an
// end angle ea (and optional start sa, default 0) it spans the arc inclusively
// (angle = sa + i*(ea-sa)/(n-1)). The elliptical (rx/ry) and rot=false forms
// are not supported and error.
func (e *Emitter) bosl2ArcCopies(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 0, "n", "r", "d", "sa", "ea")
	count, ok := arg(n, "n", -1)
	if !ok {
		return e.errf(n.Pos(), "arc_copies needs a count n")
	}
	cnt := e.expr(count, kNumber)
	r, ok := e.tubeRadius(n, "r", "d")
	if !ok {
		return e.errf(n.Pos(), "arc_copies needs a radius (r/d)")
	}
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "arc_copies has no child geometry")
	}
	v := e.freshLoopVar()
	var ang string
	if ea, has := arg(n, "ea", -1); has {
		eaStr := e.expr(ea, kNumber)
		saStr := "0"
		if sa, hasSa := arg(n, "sa", -1); hasSa {
			saStr = e.expr(sa, kNumber)
		}
		ang = "(" + saStr + " + " + v + " * (" + eaStr + " - " + saStr + ") / (" + cnt + " - 1)) * 1 deg"
	} else {
		ang = "(" + v + " * 360 / " + cnt + ") * 1 deg"
	}
	return "Union(arr: for " + v + " [0:" + cnt + " - 1] { yield " + child +
		".Move(x: " + r + ").Rotate(z: " + ang + ") })"
}

// bosl2RotCopies emits BOSL2's zrot_copies: n copies of the child spaced evenly
// about Z (copy i at i·360/n degrees). With a radius r, each copy is first moved
// out to that radius, producing a ring. The explicit `rots` angle-list form is
// not supported (n is required).
func (e *Emitter) bosl2RotCopies(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 0, "n", "r")
	count, ok := arg(n, "n", -1)
	if !ok {
		return e.errf(n.Pos(), "zrot_copies needs a count n")
	}
	cnt := e.expr(count, kNumber)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "zrot_copies has no child geometry")
	}
	inner := child
	if r, ok := arg(n, "r", -1); ok {
		inner = child + ".Move(x: " + e.expr(r, kLength) + ")"
	}
	v := e.freshLoopVar()
	angle := "(" + v + " * 360 / " + cnt + ") * 1 deg"
	return "Union(arr: for " + v + " [0:" + cnt + " - 1] { yield " + inner + ".Rotate(z: " + angle + ") })"
}

// bosl2RectTube emits BOSL2's rect_tube — a hollow rectangular tube, centered —
// as an outer box minus an inner one of equal height. The outer footprint is
// `size` ([x,y] or scalar); the inner is `isize`, or `size` minus 2·`wall`.
func (e *Emitter) bosl2RectTube(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "h", "l", "height", "size", "isize", "wall", "$fn", "$fa", "$fs")
	h, ok := cylHeightArg(n)
	if !ok {
		return e.errf(n.Pos(), "rect_tube without height")
	}
	size, ok := arg(n, "size", -1)
	if !ok {
		return e.errf(n.Pos(), "rect_tube without size")
	}
	ox, oy := e.rect2Components(size)
	var ix, iy string
	if isize, ok := arg(n, "isize", -1); ok {
		ix, iy = e.rect2Components(isize)
	} else if wall, ok := arg(n, "wall", -1); ok {
		w := e.expr(wall, kLength)
		ix = ox + " - 2 * (" + w + ")"
		iy = oy + " - 2 * (" + w + ")"
	} else {
		return e.errf(n.Pos(), "rect_tube needs an inner size (isize) or a wall thickness (wall)")
	}
	hStr := e.expr(h, kLength)
	return fmt.Sprintf("(Cube(x: %s, y: %s, z: %s).AlignCenter(pos: Vec3{}) - Cube(x: %s, y: %s, z: %s).AlignCenter(pos: Vec3{}))",
		ox, oy, hStr, ix, iy, hStr)
}

// bosl2Rect emits BOSL2's 2D rect — a rectangle centered on the origin (a
// Sketch). size is [x,y] or a scalar (square).
func (e *Emitter) bosl2Rect(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "size")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "rect without size")
	}
	x, y := e.pair2(size, kLength)
	return e.centeredSquare(x, y)
}

// centeredSquare renders a Square of the given side-length expressions recentered
// on the origin (Facet's Square is corner-origin).
func (e *Emitter) centeredSquare(x, y string) string {
	return fmt.Sprintf("Square(x: %s, y: %s).Move(x: -%s / 2, y: -%s / 2)", x, y, x, y)
}

// bosl2Prismoid emits BOSL2's prismoid — a box that tapers from a bottom
// rectangle (size1) to a top one (size2) over height h — by lofting the two
// centered rectangles and recentering on the origin. `shift` (an off-axis top)
// is not yet supported and errors via rejectExtraArgs.
func (e *Emitter) bosl2Prismoid(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 3, "size1", "size2", "h", "l", "height", "$fn", "$fa", "$fs")
	s1, ok := arg(n, "size1", 0)
	if !ok {
		return e.errf(n.Pos(), "prismoid without size1")
	}
	s2, ok := arg(n, "size2", 1)
	if !ok {
		return e.errf(n.Pos(), "prismoid without size2")
	}
	h, ok := arg(n, "h", -1)
	if !ok {
		h, ok = arg(n, "l", -1)
	}
	if !ok {
		h, ok = arg(n, "height", -1)
	}
	if !ok {
		h, ok = arg(n, "", 2)
	}
	if !ok {
		return e.errf(n.Pos(), "prismoid without height")
	}
	x1, y1 := e.pair2(s1, kLength)
	x2, y2 := e.pair2(s2, kLength)
	return "Loft(profiles: [" + e.centeredSquare(x1, y1) + ", " + e.centeredSquare(x2, y2) +
		"], heights: [0 mm, " + e.expr(h, kLength) + "]).AlignCenter(pos: Vec3{})"
}

// isBosl22D reports whether a BOSL2 shape name yields a 2D Sketch.
func isBosl22D(name string) bool {
	switch name {
	case "rect", "regular_ngon", "hexagon", "pentagon", "octagon", "star":
		return true
	}
	return false
}

// bosl2Star emits BOSL2's 2D star — a centered 2n-point star (Sketch). Points
// alternate the outer radius r (r/or/d/od) and inner radius ir (ir/id) at
// angle 180*i/n, matching BOSL2's vertex placement. The `step` form (star
// polygons), realign, and align_tip are not supported and error.
func (e *Emitter) bosl2Star(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 3, "n", "r", "or", "d", "od", "ir", "id", "$fn", "$fa", "$fs")
	nArg, ok := arg(n, "n", 0)
	if !ok {
		return e.errf(n.Pos(), "star without a point count n")
	}
	nStr := e.expr(nArg, kNumber)
	r, ok := e.ngonRadius(n, 1)
	if !ok {
		return e.errf(n.Pos(), "star without an outer radius (r/or/d/od)")
	}
	ir, iok := e.tubeRadius(n, "ir", "id")
	if !iok {
		if v, ok := arg(n, "", 2); ok {
			ir, iok = e.expr(v, kLength), true
		}
	}
	if !iok {
		return e.errf(n.Pos(), "star without an inner radius (ir/id)")
	}
	v := e.freshLoopVar()
	radius := "(" + v + " % 2 == 1 ? " + ir + " : " + r + ")"
	ang := "(180 * " + v + " / " + nStr + ") * 1 deg"
	return "Polygon(points: for " + v + " [1:2 * " + nStr + "] { yield Vec2{x: " +
		radius + " * Cos(a: " + ang + "), y: " + radius + " * Sin(a: " + ang + ")} })"
}

// bosl2RegularNgon emits a BOSL2 regular polygon (regular_ngon, or the named
// hexagon/pentagon/octagon) as a centered Polygon. Vertices follow BOSL2's
// default orientation: a = 360 - i*360/n from +X, at the circumradius. fixedN is
// the side count for the named shapes ("" reads the n argument); rPos is the
// positional index of the radius. side/ir/realign/rounding are not supported and
// error via rejectExtraArgs.
func (e *Emitter) bosl2RegularNgon(n *ast.ModuleCall, fixedN string, rPos int) string {
	nStr := fixedN
	if fixedN == "" {
		e.rejectExtraArgs(n, 2, "n", "r", "d", "or", "od", "$fn", "$fa", "$fs")
		nArg, ok := arg(n, "n", 0)
		if !ok {
			return e.errf(n.Pos(), "regular_ngon without a side count n")
		}
		nStr = e.expr(nArg, kNumber)
	} else {
		e.rejectExtraArgs(n, 1, "r", "d", "or", "od", "$fn", "$fa", "$fs")
	}
	r, ok := e.ngonRadius(n, rPos)
	if !ok {
		return e.errf(n.Pos(), "%s without a radius (r/d/or/od)", n.Name)
	}
	v := e.freshLoopVar()
	ang := "(360 - " + v + " * 360 / " + nStr + ") * 1 deg"
	return "Polygon(points: for " + v + " [0:" + nStr + " - 1] { yield Vec2{x: " +
		r + " * Cos(a: " + ang + "), y: " + r + " * Sin(a: " + ang + ")} })"
}

// ngonRadius renders a polygon's circumradius from r/or, or d/od (halved), or
// the positional arg at rPos.
func (e *Emitter) ngonRadius(n *ast.ModuleCall, rPos int) (string, bool) {
	for _, name := range []string{"r", "or"} {
		if v, ok := arg(n, name, -1); ok {
			return e.expr(v, kLength), true
		}
	}
	for _, name := range []string{"d", "od"} {
		if v, ok := arg(n, name, -1); ok {
			return e.expr(v, kLength) + " / 2", true
		}
	}
	if rPos >= 0 {
		if v, ok := arg(n, "", rPos); ok {
			return e.expr(v, kLength), true
		}
	}
	return "", false
}

// rect2Components renders a 2D footprint size into (x, y) Length expressions: a
// 2-vector gives per-axis lengths; a scalar repeats across both.
func (e *Emitter) rect2Components(size ast.Expr) (x, y string) {
	return e.pair2(size, kLength)
}

// pair2 renders a scalar-or-2-vector argument into two component expressions of
// kind k: a 2-vector gives its first two elements; a scalar repeats across both.
func (e *Emitter) pair2(v ast.Expr, k kind) (a, b string) {
	if vec, isVec := v.(*ast.Vector); isVec && len(vec.Elems) >= 2 {
		return e.expr(vec.Elems[0], k), e.expr(vec.Elems[1], k)
	}
	s := e.expr(v, k)
	return s, s
}

// bosl2LineCopies emits a BOSL2 line_copies/line_of: n copies of the child
// spaced along a direction vector and centered on the origin (the vector
// generalization of xcopies). Copy i sits at (i - (n-1)/2)·spacing; zero
// spacing components are dropped. Count n defaults to 2.
func (e *Emitter) bosl2LineCopies(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "spacing", "n", "l")
	count := "2"
	if c, ok := arg(n, "n", 1); ok {
		count = e.expr(c, kNumber)
	}
	sp, ok := arg(n, "spacing", 0)
	if !ok {
		return e.errf(n.Pos(), "line_copies needs a spacing vector")
	}
	sx, sy, sz := e.vec3Of(sp)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "line_copies has no child geometry")
	}
	v := e.freshLoopVar()
	factor := "(" + v + " - (" + count + " - 1) / 2)"
	var parts []string
	for _, c := range []struct{ axis, s string }{{"x", sx}, {"y", sy}, {"z", sz}} {
		if c.s != "0 mm" {
			parts = append(parts, c.axis+": "+factor+" * "+c.s)
		}
	}
	if len(parts) == 0 {
		return e.errf(n.Pos(), "line_copies spacing is zero")
	}
	return "Union(arr: for " + v + " [0:" + count + " - 1] { yield " + child +
		".Move(" + strings.Join(parts, ", ") + ") })"
}

// vec3Of renders a 3-vector argument into (x, y, z) Length expressions; missing
// components and a scalar's y/z are "0 mm" (a scalar lies along X).
func (e *Emitter) vec3Of(expr ast.Expr) (x, y, z string) {
	if v, isVec := expr.(*ast.Vector); isVec {
		get := func(i int) string {
			if i < len(v.Elems) {
				return e.expr(v.Elems[i], kLength)
			}
			return "0 mm"
		}
		return get(0), get(1), get(2)
	}
	return e.expr(expr, kLength), "0 mm", "0 mm"
}

// bosl2LinearCopies emits a BOSL2 linear distributor (xcopies/ycopies/zcopies):
// n copies of the child spaced along one axis and centered on the origin. It
// becomes a unioned for-comprehension; copy i sits at (i - (n-1)/2)·spacing.
// Spacing is the `spacing` argument (or positional 0); alternatively a total
// length `l` gives spacing = l/(n-1). The count `n` (positional 1) defaults to 2.
func (e *Emitter) bosl2LinearCopies(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 2, "spacing", "n", "l")
	count := "2"
	if c, ok := arg(n, "n", 1); ok {
		count = e.expr(c, kNumber)
	}
	var spacing string
	if s, ok := arg(n, "spacing", 0); ok {
		spacing = e.expr(s, kLength)
	} else if l, ok := arg(n, "l", -1); ok {
		spacing = "(" + e.expr(l, kLength) + ") / (" + count + " - 1)"
	} else {
		return e.errf(n.Pos(), "%s needs a spacing or a total length l", n.Name)
	}
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	v := e.freshLoopVar()
	offset := "(" + v + " - (" + count + " - 1) / 2) * " + spacing
	return "Union(arr: for " + v + " [0:" + count + " - 1] { yield " + child + ".Move(" + axis + ": " + offset + ") })"
}

// bosl2AxisMove emits a BOSL2 single-axis translation (up/down/left/right/
// back/fwd): a scalar distance applied to one axis with a fixed sign.
func (e *Emitter) bosl2AxisMove(n *ast.ModuleCall, axis string, sign int) string {
	e.rejectExtraArgs(n, 1)
	d, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "%s without distance", n.Name)
	}
	return e.childExpr(n) + ".Move(" + axis + ": " + e.signedLen(d, sign) + ")"
}

// bosl2AxisScale emits a BOSL2 single-axis scale (xscale/yscale/zscale): the
// child scaled by a factor along one axis, with the other axes left at 1.
func (e *Emitter) bosl2AxisScale(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 1)
	f, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "%s without a factor", n.Name)
	}
	fStr := e.expr(f, kNumber)
	sx, sy, sz := "1", "1", "1"
	switch axis {
	case "x":
		sx = fStr
	case "y":
		sy = fStr
	case "z":
		sz = fStr
	}
	child := e.childExpr(n)
	if e.childIs2D(n) {
		return child + ".Scale(x: " + sx + ", y: " + sy + ")"
	}
	return child + ".Scale(x: " + sx + ", y: " + sy + ", z: " + sz + ")"
}

// bosl2AxisRot emits a BOSL2 single-axis rotation (xrot/yrot/zrot): a scalar
// degree angle about one axis.
func (e *Emitter) bosl2AxisRot(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 1)
	a, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "%s without angle", n.Name)
	}
	return e.childExpr(n) + ".Rotate(" + axis + ": " + e.expr(a, kAngle) + ")"
}

// signedLen renders a Length expression negated when sign is negative. A literal
// gets a clean leading minus; any other expression is parenthesized so the unary
// minus binds to the whole value, not just its first operand.
func (e *Emitter) signedLen(d ast.Expr, sign int) string {
	val := e.expr(d, kLength)
	if sign >= 0 {
		return val
	}
	if _, isLit := numLitValue(d); isLit {
		return "-" + val
	}
	return "-(" + val + ")"
}

// bosl2Cuboid emits BOSL2's cuboid, which is centered on the origin by default
// (anchor=CENTER) — unlike OpenSCAD's corner-origin cube. Rounding, chamfering,
// edge selection, and explicit anchors are not yet translated; rejectExtraArgs
// turns any such argument into a located error rather than dropping it.
func (e *Emitter) bosl2Cuboid(n *ast.ModuleCall) string {
	if len(n.Children) > 0 {
		return e.bosl2AttachChain(n)
	}
	e.rejectExtraArgs(n, 1, "size")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "cuboid without size")
	}
	return e.cubeCtor(size) + ".AlignCenter(pos: Vec3{})"
}

// bosl2Wedge emits BOSL2's wedge — a triangular ramp — as its exact VNF (the
// six vertices and eight faces BOSL2 builds, scaled by size/2). BOSL2's default
// anchor is the min corner, so the centered mesh is shifted by size/2 unless
// center=true.
func (e *Emitter) bosl2Wedge(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "size", "center")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "wedge without size")
	}
	x, y, z := e.boxSizeComponents(size)
	hx, hy, hz := x+" / 2", y+" / 2", z+" / 2"
	nx, ny, nz := "-("+x+" / 2)", "-("+y+" / 2)", "-("+z+" / 2)"
	v := func(px, py, pz string) string {
		return fmt.Sprintf("Vec3{x: %s, y: %s, z: %s}", px, py, pz)
	}
	verts := strings.Join([]string{
		v(hx, hy, nz), v(hx, ny, nz), v(hx, ny, hz),
		v(nx, hy, nz), v(nx, ny, nz), v(nx, ny, hz),
	}, ", ")
	faces := "Face{v0: 0, v1: 1, v2: 2}, Face{v0: 3, v1: 5, v2: 4}, " +
		"Face{v0: 0, v1: 3, v2: 1}, Face{v0: 1, v1: 3, v2: 4}, " +
		"Face{v0: 1, v1: 4, v2: 2}, Face{v0: 2, v1: 4, v2: 5}, " +
		"Face{v0: 2, v1: 5, v2: 3}, Face{v0: 0, v1: 2, v2: 3}"
	mesh := "Mesh{vertices: []Vec3[" + verts + "], indices: []Face[" + faces + "]}.Solid()"
	if boolArg(n, "center", 1) {
		return mesh
	}
	return mesh + ".Move(x: " + hx + ", y: " + hy + ", z: " + hz + ")"
}

// boxSizeComponents renders the three side-length Length expressions of an
// OpenSCAD size: a 3-vector gives per-axis lengths; a scalar repeats across all
// three axes. These feed the attachment geometry (see boxGeom).
func (e *Emitter) boxSizeComponents(size ast.Expr) (x, y, z string) {
	if v, isVec := size.(*ast.Vector); isVec && len(v.Elems) == 3 {
		return e.expr(v.Elems[0], kLength), e.expr(v.Elems[1], kLength), e.expr(v.Elems[2], kLength)
	}
	s := e.expr(size, kLength)
	return s, s, s
}

// bosl2Cyl emits BOSL2's cyl, a cylinder centered on the origin in every axis
// (anchor=CENTER) — unlike OpenSCAD's cylinder, which centers only X/Y. `l` and
// `h` are both accepted for the length; `r`/`d` for the radius/diameter. Cones
// (r1/r2/d1/d2), chamfer, rounding, and anchors are not yet translated and
// error via rejectExtraArgs.
func (e *Emitter) bosl2Cyl(n *ast.ModuleCall) string {
	if len(n.Children) > 0 {
		return e.bosl2AttachChain(n)
	}
	s, _ := e.cylCentered(n)
	return s
}

// cylCentered renders a BOSL2 cylinder centered on the origin in every axis (the
// shared body of cyl/xcyl/ycyl/zcyl). r1/r2 (or d1/d2) make a cone/frustum; a
// single r/d makes a plain cylinder. ok is false on a missing height/radius (an
// error is already recorded).
func (e *Emitter) cylCentered(n *ast.ModuleCall) (string, bool) {
	e.rejectExtraArgs(n, 2, "h", "l", "height", "r", "d", "r1", "r2", "d1", "d2", "$fn", "$fa", "$fs")
	h, ok := cylHeightArg(n)
	if !ok {
		return e.errf(n.Pos(), "cyl without height"), false
	}
	hStr := e.expr(h, kLength)
	rMM, rMMok := cylinderRadiusMM(n)
	segs := e.segmentsSuffix(n, rMM, rMMok)

	r1, hasR1 := arg(n, "r1", -1)
	r2, hasR2 := arg(n, "r2", -1)
	d1, hasD1 := arg(n, "d1", -1)
	d2, hasD2 := arg(n, "d2", -1)
	var ctor string
	switch {
	case hasR1 && hasR2:
		ctor = fmt.Sprintf("Frustum(r1: %s, r2: %s, h: %s%s)", e.expr(r1, kLength), e.expr(r2, kLength), hStr, segs)
	case hasD1 && hasD2:
		ctor = fmt.Sprintf("Frustum(d1: %s, d2: %s, h: %s%s)", e.expr(d1, kLength), e.expr(d2, kLength), hStr, segs)
	default:
		key, val, rok := e.radiusArg(n, 1)
		if !rok {
			return e.errf(n.Pos(), "cyl without radius"), false
		}
		ctor = fmt.Sprintf("Cylinder(%s: %s, h: %s%s)", key, val, hStr, segs)
	}
	return ctor + ".AlignCenter(pos: Vec3{})", true
}

// bosl2OrientedCyl renders a BOSL2 axis-oriented cylinder (xcyl/ycyl/zcyl): the
// centered cylinder with its Z axis rotated onto the target axis. zcyl passes ""
// (Z is the default, no rotation).
func (e *Emitter) bosl2OrientedCyl(n *ast.ModuleCall, rotate string) string {
	s, ok := e.cylCentered(n)
	if !ok {
		return s
	}
	if rotate == "" {
		return s
	}
	return s + "." + rotate
}

// bosl2Tube emits BOSL2's tube — a hollow cylinder, centered on the origin —
// as an outer cylinder minus an inner one of equal height. The outer wall comes
// from or/od and the inner bore from ir/id. Wall-thickness forms and rounding
// are not yet translated and error via rejectExtraArgs.
func (e *Emitter) bosl2Tube(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "h", "l", "height", "or", "ir", "od", "id", "$fn", "$fa", "$fs")
	h, ok := cylHeightArg(n)
	if !ok {
		return e.errf(n.Pos(), "tube without height")
	}
	outer, ok := e.tubeRadius(n, "or", "od")
	if !ok {
		return e.errf(n.Pos(), "tube without an outer radius (or/od)")
	}
	inner, ok := e.tubeRadius(n, "ir", "id")
	if !ok {
		return e.errf(n.Pos(), "tube without an inner radius (ir/id)")
	}
	hStr := e.expr(h, kLength)
	return fmt.Sprintf("(Cylinder(r: %s, h: %s).AlignCenter(pos: Vec3{}) - Cylinder(r: %s, h: %s).AlignCenter(pos: Vec3{}))",
		outer, hStr, inner, hStr)
}

// bosl2Torus emits BOSL2's torus by revolving a minor-radius circle, offset to
// the major radius, around Z. Major/minor radii come from r_maj/r_min (or
// d_maj/d_min), or are derived from outer/inner radii or/ir (or od/id):
// r_maj = (or+ir)/2, r_min = (or-ir)/2.
func (e *Emitter) bosl2Torus(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 0, "r_maj", "r_min", "d_maj", "d_min", "or", "ir", "od", "id", "$fn", "$fa", "$fs")
	rmaj, rmin, ok := e.torusRadii(n)
	if !ok {
		return e.errf(n.Pos(), "torus needs major/minor radii (r_maj/r_min, d_maj/d_min, or or/ir)")
	}
	// A circle of r_min, recentered onto (r_maj, 0) in the profile plane, then
	// revolved about Z. Facet's Circle is corner-origin, so the move is
	// (r_maj - r_min, -r_min).
	return fmt.Sprintf("Circle(r: %s).Move(x: %s - %s, y: -%s).Revolve()", rmin, rmaj, rmin, rmin)
}

// torusRadii resolves a torus's (major, minor) radii from r_maj/r_min,
// d_maj/d_min, or the outer/inner pair or/ir (od/id).
func (e *Emitter) torusRadii(n *ast.ModuleCall) (rmaj, rmin string, ok bool) {
	rmaj, mok := e.tubeRadius(n, "r_maj", "d_maj")
	rmin, nok := e.tubeRadius(n, "r_min", "d_min")
	if mok && nok {
		return rmaj, rmin, true
	}
	or, ook := e.tubeRadius(n, "or", "od")
	ir, iok := e.tubeRadius(n, "ir", "id")
	if ook && iok {
		return "(" + or + " + " + ir + ") / 2", "(" + or + " - " + ir + ") / 2", true
	}
	return "", "", false
}

// tubeRadius renders a tube radius from a radius arg (rName) or a diameter arg
// (dName, halved). ok is false if neither is present.
func (e *Emitter) tubeRadius(n *ast.ModuleCall, rName, dName string) (string, bool) {
	if v, ok := arg(n, rName, -1); ok {
		return e.expr(v, kLength), true
	}
	if v, ok := arg(n, dName, -1); ok {
		return e.expr(v, kLength) + " / 2", true
	}
	return "", false
}

// cylHeightArg resolves a BOSL2 cyl's length: named h/l/height, else the first
// positional argument.
func cylHeightArg(n *ast.ModuleCall) (ast.Expr, bool) {
	for _, name := range []string{"h", "l", "height"} {
		if v, ok := arg(n, name, -1); ok {
			return v, true
		}
	}
	return arg(n, "", 0)
}
