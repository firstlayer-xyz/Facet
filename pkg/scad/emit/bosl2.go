package emit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"facet/pkg/scad/ast"
)

// literalNumber returns the value of a numeric literal expression, if x is one.
func literalNumber(x ast.Expr) (float64, bool) {
	n, ok := x.(*ast.Num)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(n.Text, 64)
	return f, err == nil
}

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
	case "spheroid":
		return e.bosl2Spheroid(n), true
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
	case "trapezoid":
		return e.bosl2Trapezoid(n), true
	case "ellipse":
		return e.bosl2Ellipse(n), true
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
	// half-space cuts (keep one side of the plane through the origin)
	case "top_half":
		return e.bosl2Half(n, "z", 1), true
	case "bottom_half":
		return e.bosl2Half(n, "z", -1), true
	case "back_half":
		return e.bosl2Half(n, "y", 1), true
	case "front_half":
		return e.bosl2Half(n, "y", -1), true
	case "right_half":
		return e.bosl2Half(n, "x", 1), true
	case "left_half":
		return e.bosl2Half(n, "x", -1), true
	case "half_of":
		return e.bosl2HalfOf(n), true
	// single-axis mirrors (no copy; optional offset plane)
	case "xflip":
		return e.bosl2Flip(n, "x"), true
	case "yflip":
		return e.bosl2Flip(n, "y"), true
	case "zflip":
		return e.bosl2Flip(n, "z"), true
	case "recolor":
		return e.bosl2Recolor(n), true
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
	// distributors (spread the distinct children along one axis, centered)
	case "xdistribute":
		return e.bosl2Distribute(n, "x"), true
	case "ydistribute":
		return e.bosl2Distribute(n, "y"), true
	case "zdistribute":
		return e.bosl2Distribute(n, "z"), true
	// mirror-and-keep copies
	case "xflip_copy":
		return e.bosl2FlipCopy(n, "x"), true
	case "yflip_copy":
		return e.bosl2FlipCopy(n, "y"), true
	case "zflip_copy":
		return e.bosl2FlipCopy(n, "z"), true
	case "mirror_copy":
		return e.bosl2MirrorCopy(n), true
	// deferred-CSG modeling (tag-partition over the scope's children)
	case "diff":
		return e.bosl2Diff(n), true
	case "intersect":
		return e.bosl2Intersect(n), true
	case "hide":
		return e.bosl2Hide(n), true
	case "show_only":
		return e.bosl2ShowOnly(n), true
	case "tag", "tag_this", "force_tag":
		// Outside a diff/intersect scope a tag is inert (its CSG role is resolved by
		// the scope walker, which peels tags itself). Emit the child geometry.
		return e.childExpr(n), true
	}
	return "", false
}

// bosl2Half emits a BOSL2 named half-space cut (top_half/bottom_half/…): it keeps
// the side of the plane through the origin that the (axis, sign) normal points
// toward, via Solid.Trim (which cuts the negative side of the half-space).
func (e *Emitter) bosl2Half(n *ast.ModuleCall, axis string, sign int) string {
	e.rejectExtraArgs(n, 0)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	return fmt.Sprintf("%s.Trim(%s: %d)", child, axis, sign)
}

// bosl2Flip emits BOSL2's single-axis mirror xflip/yflip/zflip: a reflection
// across the plane perpendicular to that axis. The optional offset (named after
// the axis, e.g. xflip(x=5)) moves the mirror plane; with no offset it is the
// plane through the origin, a plain Solid.Mirror on that axis.
func (e *Emitter) bosl2Flip(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 1, axis)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	if off, ok := arg(n, axis, 0); ok {
		d := e.expr(off, kLength)
		return fmt.Sprintf("%s.Move(%s: -(%s)).Mirror(%s: 1).Move(%s: %s)", child, axis, d, axis, axis, d)
	}
	return fmt.Sprintf("%s.Mirror(%s: 1)", child, axis)
}

// bosl2Recolor emits BOSL2's recolor(c): it paints the child and all of its
// descendants the given color — exactly Facet's Solid.Color, reached through the
// shared OpenSCAD color mapping (CSS names and r,g,b[,a] vectors).
func (e *Emitter) bosl2Recolor(n *ast.ModuleCall) string {
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "recolor has no child geometry")
	}
	method := e.colorMethod(n, false)
	if method == "" {
		// colorMethod recorded the specific error (unknown name, bad vector, …).
		return child
	}
	return child + "." + method
}

// bosl2Distribute spreads a module's distinct children evenly along one axis,
// centered on the origin (BOSL2 xdistribute/ydistribute/zdistribute). Child i of
// n sits at offset (i - (n-1)/2) * spacing, where spacing is the explicit
// `spacing` or `l` total length spread across the n-1 gaps. Unlike the *copies
// distributors (which repeat one child), this places each different child, so it
// expands to a union of individually-moved children rather than a loop.
func (e *Emitter) bosl2Distribute(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 1, "spacing", "l")
	parts := e.childParts(n.Children)
	if len(parts) == 0 {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	cnt := len(parts)
	var spacing string
	if s, ok := arg(n, "spacing", 0); ok {
		spacing = e.expr(s, kLength)
	} else if l, ok := arg(n, "l", -1); ok {
		if cnt < 2 {
			return e.errf(n.Pos(), "%s with a total length l needs at least two children", n.Name)
		}
		spacing = "(" + e.expr(l, kLength) + ") / " + strconv.Itoa(cnt-1)
	} else {
		return e.errf(n.Pos(), "%s needs a spacing or a total length l", n.Name)
	}
	mid := float64(cnt-1) / 2
	moved := make([]string, cnt)
	for i, p := range parts {
		coeff := float64(i) - mid
		if coeff == 0 {
			moved[i] = parenthesizeIfOperator(p)
			continue
		}
		c := strconv.FormatFloat(coeff, 'g', -1, 64)
		moved[i] = fmt.Sprintf("%s.Move(%s: %s * (%s))", parenthesizeIfOperator(p), axis, c, spacing)
	}
	return strings.Join(moved, " + ")
}

// bosl2HalfOf emits BOSL2's general half_of(v): keep the half of the child on the
// side the direction v points toward, via Solid.Trim with v as the plane normal.
func (e *Emitter) bosl2HalfOf(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "v")
	v, ok := arg(n, "v", 0)
	if !ok {
		return e.errf(n.Pos(), "half_of needs a direction v")
	}
	dir, ok := anchorVec(v)
	if !ok {
		return e.errf(n.Pos(), "half_of: unsupported direction (use an anchor or a ±1/0 vector)")
	}
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "half_of has no child geometry")
	}
	return fmt.Sprintf("%s.Trim(x: %d, y: %d, z: %d)", child, dir[0], dir[1], dir[2])
}


// bosl2FlipCopy emits a BOSL2 single-axis mirror-and-keep (xflip_copy/yflip_copy/
// zflip_copy): the child plus a copy mirrored across the plane normal to `axis`.
// An `offset` shifts the mirror plane (reflection across axis = offset).
func (e *Emitter) bosl2FlipCopy(n *ast.ModuleCall, axis string) string {
	e.rejectExtraArgs(n, 2, "offset", axis)
	child := e.childExpr(n)
	if child == "" {
		return e.errf(n.Pos(), "%s has no child geometry", n.Name)
	}
	// BOSL2 offsets the children along +axis by `offset` BEFORE copying (so the
	// original and its mirror end up ±offset apart), then mirrors across the plane
	// at axis=<x/y/z> (default 0).
	kept := child
	if off, ok := arg(n, "offset", 0); ok {
		kept = "(" + child + ").Move(" + axis + ": " + e.expr(off, kLength) + ")"
	}
	mirror := kept + ".Mirror(" + axis + ": 1)"
	if p, ok := arg(n, axis, 1); ok {
		pc := e.expr(p, kLength)
		mirror = kept + ".Move(" + axis + ": -(" + pc + ")).Mirror(" + axis + ": 1).Move(" + axis + ": " + pc + ")"
	}
	return "(" + kept + " + " + mirror + ")"
}

// bosl2MirrorCopy emits BOSL2's mirror_copy(v): the child plus a copy mirrored
// across the plane with normal v. Reuses the OpenSCAD mirror mapping.
func (e *Emitter) bosl2MirrorCopy(n *ast.ModuleCall) string {
	// offset=/cp= (shift along / plane through an arbitrary v) aren't translated;
	// reject them rather than silently dropping. Axis-aligned offsets go through
	// xflip_copy/yflip_copy/zflip_copy.
	e.rejectExtraArgs(n, 1, "v")
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
	// `size` (grid extent) is not implemented here — it must error, not be dropped.
	e.rejectExtraArgs(n, 2, "spacing", "n")
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
	// BOSL2's rot(from=, to=) aligns one direction onto another — no OpenSCAD
	// rotate equivalent, so handle it before delegating to the shared handler
	// (which only knows the a/v forms).
	if from, hasFrom := arg(n, "from", -1); hasFrom {
		to, hasTo := arg(n, "to", -1)
		if !hasTo {
			return e.errf(n.Pos(), "rot(from=) needs a matching to=")
		}
		if child == "" {
			return e.errf(n.Pos(), "rot has no child geometry")
		}
		fx, fy, fz := e.vec3Of(from)
		tx, ty, tz := e.vec3Of(to)
		return child + ".Rotate(from: Vec3{x: " + fx + ", y: " + fy + ", z: " + fz +
			"}, to: Vec3{x: " + tx + ", y: " + ty + ", z: " + tz + "})"
	}
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
		// A partial arc spreads n points across [sa, ea] inclusive: (n-1) gaps.
		// A FULL or wrapping arc (ea ≥ 360, or ea ≤ sa) would put the last point
		// on top of the first; BOSL2 drops the duplicate (n gaps), which this
		// (n-1)-gap formula does not reproduce. Reject the wrapping case rather
		// than emit overlapping copies; omit ea for a full circle.
		eaStr := e.expr(ea, kNumber)
		saStr := "0"
		saVal, saLit := 0.0, true
		if sa, hasSa := arg(n, "sa", -1); hasSa {
			saStr = e.expr(sa, kNumber)
			saVal, saLit = literalNumber(sa)
		}
		if eaVal, eaLit := literalNumber(ea); eaLit && (eaVal >= 360 || (saLit && eaVal <= saVal)) {
			return e.errf(n.Pos(), "arc_copies: a full or wrapping arc (ea ≥ 360, or ea ≤ sa) is not supported; omit ea for a full circle")
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
	e.rejectExtraArgs(n, 1, "h", "l", "height", "size", "isize", "wall", "anchor", "$fn", "$fa", "$fs")
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
	shape := fmt.Sprintf("(Cube(x: %s, y: %s, z: %s).AlignCenter(pos: Vec3{}) - Cube(x: %s, y: %s, z: %s).AlignCenter(pos: Vec3{}))",
		ox, oy, hStr, ix, iy, hStr)
	// Like tube/prismoid, rect_tube's bounding box is [outer, outer, h]; its BOSL2
	// default anchor is BOTTOM, so with no anchor= it sits on the plate.
	return e.applyAnchorFn(n, shape, false, anchorBox, [3]int{0, 0, -1}, func([3]int) [3]string {
		return [3]string{ox, oy, hStr}
	})
}

// bosl2Rect emits BOSL2's 2D rect — a rectangle centered on the origin (a
// Sketch). size is [x,y] or a scalar (square).
func (e *Emitter) bosl2Rect(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "size", "anchor")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "rect without size")
	}
	x, y := e.pair2(size, kLength)
	return e.applyAnchor(n, e.centeredSquare(x, y), [3]string{x, y, ""}, true, anchorBox)
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
	e.rejectExtraArgs(n, 3, "size1", "size2", "h", "l", "height", "anchor", "$fn", "$fa", "$fs")
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
	hStr := e.expr(h, kLength)
	shape := "Loft(profiles: [" + e.centeredSquare(x1, y1) + ", " + e.centeredSquare(x2, y2) +
		"], heights: [0 mm, " + hStr + "]).AlignCenter(pos: Vec3{})"
	// BOSL2 anchors a prismoid on its tapered face: a side anchor samples the
	// width at the anchored z-level (bottom size1, top size2, average between).
	// Its default anchor is BOTTOM, so with no anchor= the base sits on the plate.
	return e.applyAnchorFn(n, shape, false, anchorBox, [3]int{0, 0, -1}, func(v [3]int) [3]string {
		return [3]string{taperWidth(x1, x2, v[2]), taperWidth(y1, y2, v[2]), hStr}
	})
}

// taperWidth samples a prismoid/trapezoid width that runs from w0 at the low end
// to w1 at the high end, at the level the anchor picks along the taper axis: the
// low end (-1), the high end (+1), or their average (0) — matching BOSL2's
// lerp(size/2, size2/2, (axis+1)/2).
func taperWidth(w0, w1 string, level int) string {
	switch {
	case level < 0:
		return w0
	case level > 0:
		return w1
	default:
		return "(" + w0 + " + " + w1 + ") / 2"
	}
}

// isBosl22D reports whether a BOSL2 shape name yields a 2D Sketch.
func isBosl22D(name string) bool {
	switch name {
	case "rect", "regular_ngon", "hexagon", "pentagon", "octagon", "star", "trapezoid", "ellipse":
		return true
	}
	return false
}

// bosl2Star emits BOSL2's 2D star via the stdlib Star primitive. Star takes the
// outer radius r (r/or/d/od) and inner radius ir (ir/id); it builds the 2n-point
// path at angle 180*i/n — the same placement BOSL2 uses. Star is corner-anchored,
// so ngonCenterMove shifts it onto its construction origin (BOSL2's CENTER
// anchor); the star's extent is set by its outer vertices, which sit at a
// `sides`-gon's angles, so it reuses the n-gon offset. A literal point count is
// required. The `step` form (star polygons), realign, and align_tip are not
// supported and error.
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
	sides := 0
	if f, isLit := literalNumber(nArg); isLit {
		sides = int(f)
	}
	if sides < 2 {
		return e.errf(n.Pos(), "star: requires a literal point count of at least 2")
	}
	return "Star(n: " + nStr + ", r: " + r + ", ir: " + ir + ")" + ngonCenterMove(sides, r)
}

// bosl2Ellipse emits BOSL2's 2D ellipse, centered on the origin: a circle of
// radius rx (so the facet count suits the final size), centered, then scaled in y
// by ry/rx. Accepts r=[rx,ry] or d=[dx,dy] (a scalar gives a circle).
func (e *Emitter) bosl2Ellipse(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "r", "d", "anchor", "$fn", "$fa", "$fs")
	var rx, ry string
	if r, ok := arg(n, "r", 0); ok {
		rx, ry = e.pair2(r, kLength)
	} else if d, ok := arg(n, "d", -1); ok {
		dx, dy := e.pair2(d, kLength)
		rx, ry = "("+dx+") / 2", "("+dy+") / 2"
	} else {
		return e.errf(n.Pos(), "ellipse without r or d")
	}
	circ := "Circle(r: " + rx + e.segmentsSuffix(n, 0, false) + ")"
	shape := fmt.Sprintf("%s.Move(x: -(%s), y: -(%s)).Scale(x: 1, y: Number(from: %s) / Number(from: %s))",
		circ, rx, rx, ry, rx)
	return e.applyAnchor(n, shape, [3]string{rx, ry, ""}, true, anchorEllipse)
}

// bosl2Trapezoid emits BOSL2's 2D isosceles trapezoid, centered on the origin:
// height h along Y, bottom width w1 and top width w2 along X — a four-point
// Polygon.
func (e *Emitter) bosl2Trapezoid(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 3, "h", "w1", "w2", "anchor")
	h, ok := arg(n, "h", 0)
	if !ok {
		return e.errf(n.Pos(), "trapezoid without height h")
	}
	w1, ok := arg(n, "w1", 1)
	if !ok {
		return e.errf(n.Pos(), "trapezoid without bottom width w1")
	}
	w2, ok := arg(n, "w2", 2)
	if !ok {
		return e.errf(n.Pos(), "trapezoid without top width w2")
	}
	hs, a, b := e.expr(h, kLength), e.expr(w1, kLength), e.expr(w2, kLength)
	shape := fmt.Sprintf("Polygon(points: [Vec2{x: -(%s) / 2, y: -(%s) / 2}, "+
		"Vec2{x: (%s) / 2, y: -(%s) / 2}, Vec2{x: (%s) / 2, y: (%s) / 2}, "+
		"Vec2{x: -(%s) / 2, y: (%s) / 2}])",
		a, hs, a, hs, b, hs, b, hs)
	// 2D prismoid: the width tapers along the height (y), so a left/right anchor
	// samples it at the anchored y-level — bottom w1 (FWD), top w2 (BACK), average.
	return e.applyAnchorFn(n, shape, true, anchorBox, [3]int{}, func(v [3]int) [3]string {
		return [3]string{taperWidth(a, b, v[1]), hs, ""}
	})
}

// bosl2RegularNgon emits a BOSL2 regular polygon (regular_ngon, or the named
// hexagon/pentagon/octagon) via the stdlib Ngon primitive. Ngon places a vertex
// on +X at the circumradius; a regular polygon has the same vertex set under
// 360/n symmetry, so this is the same shape as BOSL2's default orientation. Ngon
// is corner-anchored, so ngonCenterMove shifts it onto its construction origin
// (BOSL2's CENTER anchor), and a vector anchor= then lands on the perimeter
// (ngonAnchorMove). Both moves need the offset coefficients, so a literal side
// count is required. fixedN is the side count for the named shapes ("" reads the
// n argument); rPos is the positional index of the radius. side/ir/realign/
// rounding are not supported and error via rejectExtraArgs.
func (e *Emitter) bosl2RegularNgon(n *ast.ModuleCall, fixedN string, rPos int) string {
	nStr := fixedN
	sides := 0
	if fixedN == "" {
		e.rejectExtraArgs(n, 2, "n", "r", "d", "or", "od", "anchor", "$fn", "$fa", "$fs")
		nArg, ok := arg(n, "n", 0)
		if !ok {
			return e.errf(n.Pos(), "regular_ngon without a side count n")
		}
		nStr = e.expr(nArg, kNumber)
		if f, isLit := literalNumber(nArg); isLit {
			sides = int(f)
		}
	} else {
		e.rejectExtraArgs(n, 1, "r", "d", "or", "od", "anchor", "$fn", "$fa", "$fs")
		sides, _ = strconv.Atoi(fixedN)
	}
	r, ok := e.ngonRadius(n, rPos)
	if !ok {
		return e.errf(n.Pos(), "%s without a radius (r/d/or/od)", n.Name)
	}
	if sides < 3 {
		return e.errf(n.Pos(), "%s: requires a literal side count of at least 3", n.Name)
	}
	shape := "Ngon(n: " + nStr + ", r: " + r + ")" + ngonCenterMove(sides, r)
	// BOSL2 anchors a 2D polygon by its path: a vector anchor lands on the
	// perimeter where the center-ray crosses it (verified vs OpenSCAD: hexagon
	// RIGHT+BACK -> (6.34,6.34), not the bbox corner).
	if _, has := arg(n, "anchor", -1); has {
		move, ok := e.ngonAnchorMove(n, sides, r)
		if !ok {
			return move
		}
		return shape + move
	}
	return shape
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
	// `l` (total length) is not implemented here — it must error, not be dropped.
	e.rejectExtraArgs(n, 2, "spacing", "n")
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
	e.rejectExtraArgs(n, 1, "cp")
	a, ok := arg(n, "", 0)
	if !ok {
		return e.errf(n.Pos(), "%s without angle", n.Name)
	}
	method := "Rotate(" + axis + ": " + e.expr(a, kAngle) + ")"
	return e.pivotRotate(n, e.childExpr(n), method)
}

// pivotRotate wraps a rotation method around an optional cp= pivot point: the
// child is moved so cp sits at the origin, rotated, then moved back, so the
// rotation turns about cp instead of the origin (BOSL2's cp=). With no cp it is a
// plain trailing rotation.
func (e *Emitter) pivotRotate(n *ast.ModuleCall, child, method string) string {
	cp, has := arg(n, "cp", -1)
	if !has {
		return child + "." + method
	}
	if _, isVec := cp.(*ast.Vector); !isVec {
		return e.errf(n.Pos(), "%s: cp must be a coordinate vector like [x, y, z]", n.Name)
	}
	cx, cy, cz := e.vec3Of(cp)
	p := "Vec3{x: " + cx + ", y: " + cy + ", z: " + cz + "}"
	return child + ".Move(v: -(" + p + "))." + method + ".Move(v: " + p + ")"
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

// bosl2Spheroid emits BOSL2's spheroid (its preferred sphere): a centered Sphere
// from r/d, with anchor= placing it by its anchor point over the [2r,2r,2r]
// bounding box (so spheroid(anchor=BOTTOM) rests on the plate). circum/style
// options still error as extras.
func (e *Emitter) bosl2Spheroid(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "r", "d", "anchor", "spin", "$fn", "$fa", "$fs")
	ctor, radius, ok := e.sphereCtor(n)
	if !ok {
		return e.errf(n.Pos(), "spheroid without radius")
	}
	shape := ctor + ".AlignCenter(pos: Vec3{})"
	dia := "2 * (" + radius + ")"
	shape = e.applyAnchor(n, shape, [3]string{dia, dia, dia}, false, anchorSphere)
	return e.applySpin(n, shape)
}

// bosl2Cuboid emits BOSL2's cuboid, which is centered on the origin by default
// (anchor=CENTER) — unlike OpenSCAD's corner-origin cube. rounding/chamfer round
// or bevel every edge, anchor= shifts the box so that anchor point lands on the
// origin, and orient= reorients it. Per-edge selection (edges=/except=) is not
// translated; rejectExtraArgs turns any such argument into a located error.
func (e *Emitter) bosl2Cuboid(n *ast.ModuleCall) string {
	if len(n.Children) > 0 {
		return e.bosl2AttachChain(n)
	}
	e.rejectExtraArgs(n, 1, "size", "rounding", "chamfer", "edges", "orient", "anchor", "spin", "$fn", "$fa", "$fs")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "cuboid without size")
	}
	fillet, chamfer, edges, good := e.cuboidRoundingArgs(n)
	if !good {
		return ""
	}
	shape := e.cubeCtor(size, fillet, chamfer, edges) + ".AlignCenter(pos: Vec3{})"
	sx, sy, sz := e.cubeSizeComponents(size)
	shape = e.applyAnchor(n, shape, [3]string{sx, sy, sz}, false, anchorBox)
	return e.applyOrient(n, e.applySpin(n, shape))
}

// cuboidRoundingArgs extracts a cuboid's rounding/chamfer/edges as Facet argument
// snippets (each "" when the option is absent), shared by the plain and
// attachable cuboid paths. good is false (with a located error already recorded)
// when the edges selector is one Facet can't express yet.
func (e *Emitter) cuboidRoundingArgs(n *ast.ModuleCall) (fillet, chamfer, edges string, good bool) {
	if r, has := arg(n, "rounding", -1); has {
		fillet = e.expr(r, kLength)
	}
	if c, has := arg(n, "chamfer", -1); has {
		chamfer = e.expr(c, kLength)
	}
	if ea, has := arg(n, "edges", -1); has {
		es, ok := bosl2EdgeSetExpr(ea)
		if !ok {
			e.errf(ea.Pos(), "cuboid: unsupported edges selector (only the axis groups \"X\", \"Y\", \"Z\" are supported)")
			return "", "", "", false
		}
		edges = es
	}
	return fillet, chamfer, edges, true
}

// bosl2EdgeSetExpr maps a BOSL2 cuboid edges= selector to a Facet EdgeSet
// expression. Only the three axis groups (the string "X"/"Y"/"Z", selecting the
// four edges parallel to that axis) are supported so far; individual edges, face
// groups, and except= are not yet expressible (Facet rounds a full axis group or
// all edges), so anything else returns ok=false for a located error.
func bosl2EdgeSetExpr(x ast.Expr) (string, bool) {
	if s, ok := x.(*ast.Str); ok {
		switch s.Value {
		case "X":
			return "EdgesAlongX()", true
		case "Y":
			return "EdgesAlongY()", true
		case "Z":
			return "EdgesAlongZ()", true
		}
	}
	return "", false
}

// bosl2Wedge emits BOSL2's wedge — a triangular ramp — via the stdlib Wedge
// primitive. Wedge is corner-anchored (min corner at origin), which matches
// BOSL2's default wedge anchor, so the bare form maps directly; center=true
// recenters it with AlignCenter.
func (e *Emitter) bosl2Wedge(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "size", "center")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "wedge without size")
	}
	x, y, z := e.boxSizeComponents(size)
	wedge := "Wedge(x: " + x + ", y: " + y + ", z: " + z + ")"
	if boolArg(n, "center", 1) {
		return wedge + ".AlignCenter(pos: Vec3{})"
	}
	return wedge
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
// `h` are both accepted for the length; `r`/`d` for the radius/diameter; r1/r2
// (or d1/d2) make a cone (Frustum); chamfer/rounding bevel/round both rims;
// orient= reorients it; and anchor= shifts it by its anchor point (an x/y anchor
// on a tapered cyl is a located error — its bounding diameter is the larger end).
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
	e.rejectExtraArgs(n, 2, "h", "l", "height", "r", "d", "r1", "r2", "d1", "d2", "rounding", "chamfer", "orient", "anchor", "spin", "$fn", "$fa", "$fs")
	h, ok := cylHeightArg(n)
	if !ok {
		return e.errf(n.Pos(), "cyl without height"), false
	}
	hStr := e.expr(h, kLength)
	rMM, rMMok := cylinderRadiusMM(n)
	segs := e.segmentsSuffix(n, rMM, rMMok)
	// BOSL2 cyl(rounding=R)/chamfer=C round/bevel both rims; Frustum/Cylinder
	// fillet/chamfer do the same.
	edge := ""
	if r, has := arg(n, "rounding", -1); has {
		edge += ", fillet: " + e.expr(r, kLength)
	}
	if c, has := arg(n, "chamfer", -1); has {
		edge += ", chamfer: " + e.expr(c, kLength)
	}

	r1, hasR1 := arg(n, "r1", -1)
	r2, hasR2 := arg(n, "r2", -1)
	d1, hasD1 := arg(n, "d1", -1)
	d2, hasD2 := arg(n, "d2", -1)
	var ctor string
	switch {
	case hasR1 && hasR2:
		ctor = fmt.Sprintf("Frustum(r1: %s, r2: %s, h: %s%s%s)", e.expr(r1, kLength), e.expr(r2, kLength), hStr, segs, edge)
	case hasD1 && hasD2:
		ctor = fmt.Sprintf("Frustum(d1: %s, d2: %s, h: %s%s%s)", e.expr(d1, kLength), e.expr(d2, kLength), hStr, segs, edge)
	default:
		key, val, rok := e.radiusArg(n, 1)
		if !rok {
			return e.errf(n.Pos(), "cyl without radius"), false
		}
		ctor = fmt.Sprintf("Cylinder(%s: %s, h: %s%s%s)", key, val, hStr, segs, edge)
	}
	shape := ctor + ".AlignCenter(pos: Vec3{})"
	if a, has := arg(n, "anchor", -1); has {
		v, vok := anchorVec(a)
		if !vok {
			return e.errf(n.Pos(), "cyl: unsupported anchor (use a named anchor or a ±1/0 vector)"), false
		}
		// The cylinder's bounding box is [diameter, diameter, h]; the diameter is
		// only needed when the anchor leans off the z axis.
		dia := ""
		if v[0] != 0 || v[1] != 0 {
			d, dok := e.cylDiameter(n)
			if !dok {
				return e.errf(n.Pos(), "cyl: an x/y anchor on a tapered cyl is not supported"), false
			}
			dia = d
		}
		shape += anchorOffset(v, [3]string{dia, dia, hStr}, anchorCyl)
	}
	return e.applyOrient(n, e.applySpin(n, shape)), true
}

// cylDiameter returns the x/y bounding diameter of a straight cyl: the explicit
// d, or 2*r. It returns ok=false for a tapered cyl (r1/r2/d1/d2), whose bounding
// diameter is the larger end — not needed for the common z-axis anchors.
func (e *Emitter) cylDiameter(n *ast.ModuleCall) (string, bool) {
	for _, name := range []string{"r1", "r2", "d1", "d2"} {
		if _, has := arg(n, name, -1); has {
			return "", false
		}
	}
	key, val, ok := e.radiusArg(n, 1)
	if !ok {
		return "", false
	}
	if key == "d" {
		return val, true
	}
	return "2 * (" + val + ")", true
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
	e.rejectExtraArgs(n, 1, "h", "l", "height", "or", "ir", "od", "id", "anchor", "$fn", "$fa", "$fs")
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
	shape := fmt.Sprintf("(Cylinder(r: %s, h: %s).AlignCenter(pos: Vec3{}) - Cylinder(r: %s, h: %s).AlignCenter(pos: Vec3{}))",
		outer, hStr, inner, hStr)
	// The tube's bounding box is [outer diameter, outer diameter, h].
	dia := "2 * (" + outer + ")"
	return e.applyAnchor(n, shape, [3]string{dia, dia, hStr}, false, anchorCyl)
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
