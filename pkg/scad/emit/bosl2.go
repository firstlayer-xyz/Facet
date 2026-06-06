package emit

import (
	"fmt"
	"regexp"

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
	}
	return "", false
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
	e.rejectExtraArgs(n, 2, "h", "l", "height", "r", "d", "$fn", "$fa", "$fs")
	h, ok := cylHeightArg(n)
	if !ok {
		return e.errf(n.Pos(), "cyl without height")
	}
	key, val, rok := e.radiusArg(n, 1)
	if !rok {
		return e.errf(n.Pos(), "cyl without radius")
	}
	rMM, rMMok := cylinderRadiusMM(n)
	return fmt.Sprintf("Cylinder(%s: %s, h: %s%s).AlignCenter(pos: Vec3{})",
		key, val, e.expr(h, kLength), e.segmentsSuffix(n, rMM, rMMok))
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
