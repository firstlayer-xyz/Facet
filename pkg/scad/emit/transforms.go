package emit

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"facet/pkg/colorname"
	"facet/pkg/scad/ast"
)

// isTransform reports whether a module named `name` is an OpenSCAD transform
// that wraps its children and preserves their dimensionality (translate, rotate,
// etc.). Dimensionality-changing wrappers (linear_extrude, projection, …) are
// classified separately by isDimChanger.
func isTransform(name string) bool {
	switch name {
	case "translate", "rotate", "scale", "mirror", "color", "resize":
		return true
	}
	return false
}

// isDimChanger reports whether a module named `name` wraps a single child and
// changes its dimensionality: linear_extrude/rotate_extrude turn a Sketch into a
// Solid; projection/offset turn a Solid (offset: a Sketch) into a Sketch.
func isDimChanger(name string) bool {
	switch name {
	case "linear_extrude", "rotate_extrude", "projection", "offset":
		return true
	}
	return false
}

// dimChangerIs2D reports whether a dimensionality-changing wrapper produces a
// Sketch (2D). linear_extrude/rotate_extrude produce Solids (3D); projection and
// offset produce Sketches (2D).
func dimChangerIs2D(name string) bool {
	switch name {
	case "projection", "offset":
		return true
	}
	return false
}

// stmtIs2D reports whether the geometry produced by a statement is a Sketch
// (2D) rather than a Solid (3D). Dimensionality-changing wrappers are classified
// by their result type; dimensionality-preserving transforms inherit the
// dimensionality of their first child; primitives use their own classification.
func (e *Emitter) stmtIs2D(s ast.Stmt, seen map[string]bool) bool {
	switch n := s.(type) {
	case *ast.ModuleCall:
		if n.Name == "children" {
			return e.curChild2D
		}
		if isDimChanger(n.Name) {
			return dimChangerIs2D(n.Name)
		}
		if isTransform(n.Name) {
			if len(n.Children) == 0 {
				return false
			}
			return e.stmtIs2D(n.Children[0], seen)
		}
		// A user module's dimensionality is that of its own body's geometry.
		if sym, ok := e.syms[n.Name]; ok && !sym.isFunc {
			if seen[n.Name] {
				return false // recursion: fall back to 3D (Solid)
			}
			seen[n.Name] = true
			return e.firstGeomIs2D(sym.moduleBody, seen)
		}
		return is2D(n.Name)
	case *ast.If:
		// A conditional's dimensionality is that of its then-branch geometry;
		// both branches must agree for the emitted Facet to type-check.
		return e.firstGeomIs2D(n.Then, seen)
	}
	return false
}

// firstGeomIs2D reports whether the first geometry-producing statement of a body
// is 2D. A body's children share a dimensionality (they are unioned), so the
// first is representative.
func (e *Emitter) firstGeomIs2D(body []ast.Stmt, seen map[string]bool) bool {
	if g := geometryStmts(body); len(g) > 0 {
		return e.stmtIs2D(g[0], seen)
	}
	return false
}

// childExpr emits a transform's children as a single Facet expression. A single
// child is emitted bare; multiple children are unioned and parenthesized so the
// transform method applies to the whole union.
func (e *Emitter) childExpr(n *ast.ModuleCall) string {
	body := e.unionStmts(n.Children)
	if len(n.Children) > 1 {
		return "(" + body + ")"
	}
	return body
}

// transform emits a child geometry followed by the Facet method for the
// transform. The child is emitted first (inner geometry) and the method is
// appended (outer transform), realizing OpenSCAD's outside-in nesting as
// Facet's left-to-right method chain.
func (e *Emitter) transform(n *ast.ModuleCall) string {
	child := e.childExpr(n)
	is2D := false
	if len(n.Children) > 0 {
		is2D = e.stmtIs2D(n.Children[0], map[string]bool{})
	}

	var method string
	switch n.Name {
	case "translate":
		method = e.translateMethod(n, is2D)
	case "rotate":
		method = e.rotateMethod(n, is2D)
	case "scale":
		method = e.scaleMethod(n, is2D)
	case "mirror":
		method = e.mirrorMethod(n, is2D)
	case "color":
		method = e.colorMethod(n, is2D)
	case "resize":
		method = e.resizeMethod(n, is2D)
	}
	if method == "" {
		// An untranslatable variant recorded an error via errf; flow the child
		// through unchanged so one pass can still collect all errors.
		return child
	}
	return child + "." + method
}

// dimChanger emits a dimensionality-changing wrapper (linear_extrude,
// rotate_extrude, projection, offset) as a method on the wrapped child. The
// child is emitted first and the method appended, matching transform's
// inside-out → left-to-right mapping.
func (e *Emitter) dimChanger(n *ast.ModuleCall) string {
	child := e.childExpr(n)
	var method string
	switch n.Name {
	case "linear_extrude":
		method = e.linearExtrudeMethod(n)
	case "rotate_extrude":
		method = e.rotateExtrudeMethod(n)
	case "projection":
		method = e.projectionMethod(n)
	case "offset":
		method = e.offsetMethod(n)
	}
	if method == "" {
		return child
	}
	return child + "." + method
}

// linearExtrudeMethod builds `.Extrude(z: H mm[, twist: T deg][, slices: N]
// [, taperX: S, taperY: S])`, plus a `.AlignCenter` when center=true. OpenSCAD's
// `height` maps to Facet's `z`; `twist` and `slices` pass through (twist sign
// matches: both wind clockwise looking down +Z); `scale` (scalar or [sx,sy])
// maps to taperX/taperY (Facet default 1 = OpenSCAD default scale 1); and
// `center=true` centers the solid on z=0. `convexity` is a render-only hint with
// no geometric effect and is intentionally dropped. Any other argument is an
// error rather than a silent drop.
func (e *Emitter) linearExtrudeMethod(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "height", "twist", "slices", "scale", "center", "convexity", "$fn", "$fa", "$fs")
	h, ok := arg(n, "height", 0)
	if !ok {
		return e.errf(n.Pos(), "linear_extrude without height")
	}
	out := "Extrude(z: " + e.expr(h, kLength)
	twist, hasTwist := arg(n, "twist", -1)
	if hasTwist {
		out += ", twist: " + e.expr(twist, kAngle)
	}
	if s := e.extrudeSlices(n, twist, hasTwist); s != "" {
		out += ", slices: " + s
	}
	if sc, found := arg(n, "scale", -1); found {
		sx, sy := e.scaleFactors(sc)
		out += ", taperX: " + sx + ", taperY: " + sy
	}
	out += ")"
	if boolArg(n, "center", -1) {
		// OpenSCAD center=true centers only the Z extent (the 2D profile's X/Y
		// position is unchanged), so leave x/y and center z.
		out += ".AlignCenter(pos: Vec3{}, x: false, y: false)"
	}
	return out
}

// extrudeSlices returns the Facet `slices` value for a linear extrude: an
// explicit `slices` argument if present; otherwise, for a twisted extrude with
// a known segment count, OpenSCAD's derived count max(1, ceil(fn·|twist|/360)).
// Returns "" when neither applies, leaving Facet's default of a single slice.
func (e *Emitter) extrudeSlices(n *ast.ModuleCall, twist ast.Expr, hasTwist bool) string {
	if s, found := arg(n, "slices", -1); found {
		return e.expr(s, kNumber)
	}
	if !hasTwist {
		return ""
	}
	fn := e.resolutionFn(n)
	if fn == "" {
		return ""
	}
	return "Max(a: 1, b: Ceil(n: " + fn + " * Abs(a: " + e.expr(twist, kNumber) + ") / 360))"
}

// scaleFactors renders an OpenSCAD linear_extrude `scale` as Facet taperX/taperY
// values: a scalar applies to both axes; a 2-vector gives per-axis factors.
func (e *Emitter) scaleFactors(sc ast.Expr) (sx, sy string) {
	if v, isVec := sc.(*ast.Vector); isVec && len(v.Elems) >= 2 {
		return e.expr(v.Elems[0], kNumber), e.expr(v.Elems[1], kNumber)
	}
	s := e.expr(sc, kNumber)
	return s, s
}

// rotateExtrudeMethod builds `.Revolve(a: A deg[, segments: N])`. OpenSCAD's
// `angle` maps to Facet's `a` (default 360°) and `$fn` to `segments` (Facet
// default 0 = auto). Both revolve the profile around the Z axis (verified:
// pkg/manifold/revolve_axis_test.go), so the mapping is faithful. `convexity` is
// a render-only hint with no geometric effect and is dropped.
func (e *Emitter) rotateExtrudeMethod(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "angle", "$fn", "$fa", "$fs", "convexity")
	var parts []string
	if a, found := arg(n, "angle", 0); found {
		parts = append(parts, "a: "+e.expr(a, kAngle))
	}
	if s := e.segmentsAngle(n); s != "" {
		parts = append(parts, "segments: "+s)
	}
	return "Revolve(" + strings.Join(parts, ", ") + ")"
}

// projectionMethod builds `.Project()` (cut=false) or `.Slice(z: 0 mm)`
// (cut=true). projection takes a Solid child and yields a Sketch.
func (e *Emitter) projectionMethod(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "cut")
	if boolArg(n, "cut", 0) {
		return "Slice(z: 0 mm)"
	}
	return "Project()"
}

// offsetMethod builds `.Offset(delta: D mm)`. OpenSCAD's `delta` is a straight
// (mitered) offset; `r` is a rounded offset. Facet's Offset is mitered, so a
// rounded `r` request is approximated as mitered — the visual difference for
// thin offsets (line outlines, fillets at small radii) is usually negligible.
// `chamfer` is a geometric modifier (bevels corners) that is not supported;
// it must error rather than be silently ignored.
func (e *Emitter) offsetMethod(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "delta", "r")
	if d, found := arg(n, "delta", -1); found {
		return "Offset(delta: " + e.expr(d, kLength) + ")"
	}
	if r, found := arg(n, "r", 0); found {
		return "Offset(delta: " + e.expr(r, kLength) + ")"
	}
	e.errf(n.Pos(), "offset without delta or r")
	return ""
}

// vecArg pulls the named arg `name` (or the positional arg at `idx`) as a vector
// literal and renders each component with `k`. Missing components (e.g. a
// 2-element vector) render as the zero literal for `k`. `ok` is false when the
// arg is absent or not a vector.
func (e *Emitter) vecArg(n *ast.ModuleCall, name string, idx int, k kind) (x, y, z string, ok bool) {
	v, found := arg(n, name, idx)
	if !found {
		return "", "", "", false
	}
	vec, isVec := v.(*ast.Vector)
	if !isVec {
		return "", "", "", false
	}
	zero := zeroLit(k)
	x, y, z = zero, zero, zero
	if len(vec.Elems) > 0 {
		x = e.expr(vec.Elems[0], k)
	}
	if len(vec.Elems) > 1 {
		y = e.expr(vec.Elems[1], k)
	}
	if len(vec.Elems) > 2 {
		z = e.expr(vec.Elems[2], k)
	}
	return x, y, z, true
}

// zeroLit is the rendered zero literal for a numeric kind.
func zeroLit(k kind) string {
	switch k {
	case kLength:
		return "0 mm"
	case kAngle:
		return "0 deg"
	default:
		return "0"
	}
}

// isZeroLit reports whether a rendered value is a literal zero ("0", "0 mm",
// "0 deg"). Such components are dropped from method calls for clean output.
func isZeroLit(v string) bool {
	switch v {
	case "0", "0 mm", "0 deg":
		return true
	}
	return false
}

// pair is a method-argument name/value used by joinNonZero.
type pair struct {
	name string
	val  string
}

// joinNonZero renders `name: val` for each pair, dropping any whose value is a
// literal zero. If every pair is zero it keeps the first so the method call
// still receives a valid argument.
func joinNonZero(pairs ...pair) string {
	var kept []string
	for _, p := range pairs {
		if !isZeroLit(p.val) {
			kept = append(kept, p.name+": "+p.val)
		}
	}
	if len(kept) == 0 && len(pairs) > 0 {
		kept = append(kept, pairs[0].name+": "+pairs[0].val)
	}
	return strings.Join(kept, ", ")
}

// translateMethod builds `.Move(...)`. 2D children move in x,y only. Accepts
// both `translate([x,y,z])` and the named `translate(v=[x,y,z])` form. When
// the argument is a runtime expression rather than a Vector literal (e.g.
// `translate(l1 * [cos(a), sin(a), 0])`), each axis indexes into the
// expression — Facet auto-coerces the resulting Number to Length at the
// Move boundary.
func (e *Emitter) translateMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 1, "v")
	if x, y, z, ok := e.vecArg(n, "v", 0, kLength); ok {
		if is2D {
			return "Move(" + joinNonZero(pair{"x", x}, pair{"y", y}) + ")"
		}
		return "Move(" + joinNonZero(pair{"x", x}, pair{"y", y}, pair{"z", z}) + ")"
	}
	v, found := arg(n, "v", 0)
	if !found {
		return ""
	}
	expr := e.expr(v, kNumber)
	if is2D {
		return fmt.Sprintf("Move(x: %s[0] * 1 mm, y: %s[1] * 1 mm)", expr, expr)
	}
	return fmt.Sprintf("Move(x: %s[0] * 1 mm, y: %s[1] * 1 mm, z: %s[2] * 1 mm)", expr, expr, expr)
}

// rotateMethod builds `.Rotate(...)`. Handles both the vector form
// `rotate([x,y,z])` and the scalar form `rotate(a)`. The axis-angle form
// `rotate(a, v=[...])` is not representable and records a diagnostic.
func (e *Emitter) rotateMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 1, "a", "v")
	// Axis-angle form: a scalar angle plus an axis vector `v`.
	if _, hasV := arg(n, "v", -1); hasV {
		e.errf(n.Pos(), "rotate axis-angle form")
		return ""
	}
	x, y, z, isVec := e.vecArg(n, "a", 0, kAngle)
	if isVec {
		if is2D {
			// 2D rotation is a single angle taken from the Z component.
			return "Rotate(a: " + z + ")"
		}
		return "Rotate(" + joinNonZero(pair{"x", x}, pair{"y", y}, pair{"z", z}) + ")"
	}
	// Scalar form: rotate(a) spins about Z.
	a, found := arg(n, "a", 0)
	if !found {
		return ""
	}
	aStr := e.expr(a, kAngle)
	if is2D {
		return "Rotate(a: " + aStr + ")"
	}
	return "Rotate(z: " + aStr + ")"
}

// scaleMethod builds `.Scale(...)`. Scale factors are bare Numbers. Scale has no
// per-axis defaults, so all axes are always emitted. Accepts both `scale([...])`
// and the named `scale(v=[...])` form.
func (e *Emitter) scaleMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 1, "v")
	v, found := arg(n, "v", 0)
	if !found {
		return e.errf(n.Pos(), "scale without a factor")
	}
	x, y, z := e.scaleComponents(v)
	if is2D {
		return fmt.Sprintf("Scale(x: %s, y: %s)", x, y)
	}
	return fmt.Sprintf("Scale(x: %s, y: %s, z: %s)", x, y, z)
}

// scaleComponents resolves an OpenSCAD scale factor to per-axis Facet factors. A
// vector scales per axis, with omitted components defaulting to 1 (no scaling),
// matching OpenSCAD — not 0, which would collapse the axis. A scalar scales every
// axis uniformly; a vector-valued variable is indexed. (A scalar factor used to
// fall through vecArg and be dropped entirely, silently removing the scale.)
func (e *Emitter) scaleComponents(v ast.Expr) (x, y, z string) {
	if vec, ok := v.(*ast.Vector); ok {
		x, y, z = "1", "1", "1"
		if len(vec.Elems) > 0 {
			x = e.expr(vec.Elems[0], kNumber)
		}
		if len(vec.Elems) > 1 {
			y = e.expr(vec.Elems[1], kNumber)
		}
		if len(vec.Elems) > 2 {
			z = e.expr(vec.Elems[2], kNumber)
		}
		return x, y, z
	}
	if e.inferType(v) == "[]Number" {
		b := e.operand(v, kNumber)
		return b + "[0]", b + "[1]", b + "[2]"
	}
	s := e.operand(v, kNumber)
	return s, s, s
}

// mirrorMethod builds `.Mirror(...)`. The vector is the mirror-plane normal
// (bare Numbers). 2D drops the z component. Accepts both `mirror([...])` and the
// named `mirror(v=[...])` form.
func (e *Emitter) mirrorMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 1, "v")
	x, y, z, ok := e.vecArg(n, "v", 0, kNumber)
	if !ok {
		return ""
	}
	if is2D {
		return "Mirror(" + joinNonZero(pair{"x", x}, pair{"y", y}) + ")"
	}
	return "Mirror(" + joinNonZero(pair{"x", x}, pair{"y", y}, pair{"z", z}) + ")"
}

// resizeMethod builds `.Resize(size: Vec3{...})`. Resize is a 3D-only method.
// Accepts both `resize([...])` and the named `resize(newsize=[...])` form.
func (e *Emitter) resizeMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 1, "newsize")
	if is2D {
		e.errf(n.Pos(), "resize on 2D geometry")
		return ""
	}
	x, y, z, ok := e.vecArg(n, "newsize", 0, kLength)
	if !ok {
		return ""
	}
	return fmt.Sprintf("Resize(size: Vec3{x: %s, y: %s, z: %s})", x, y, z)
}

// colorMethod builds `.Color(...)`. Color applies to solids; a 2D child errors.
// Without alpha, a named color and an [r,g,b] vector map to the hex overload
// (Solid.Color(hex)). When alpha is present — a 4th vector component or the
// `alpha` argument — the [0,1] RGBA overload Solid.Color(r,g,b,a) carries the
// opacity (OpenSCAD and Facet both use [0,1] components).
func (e *Emitter) colorMethod(n *ast.ModuleCall, is2D bool) string {
	e.rejectExtraArgs(n, 2, "c", "alpha")
	if is2D {
		return e.errf(n.Pos(), "color on 2D geometry")
	}
	c, found := arg(n, "c", 0)
	if !found {
		return e.errf(n.Pos(), "color without value")
	}
	// An explicit `alpha` argument overrides a vector's 4th component.
	alpha := ""
	if a, ok := arg(n, "alpha", 1); ok {
		alpha = e.expr(a, kNumber)
	}
	switch v := c.(type) {
	case *ast.Str:
		hex, known := cssColorHex(v.Value)
		if !known {
			return e.errf(n.Pos(), "unknown color name '%s'", v.Value)
		}
		if alpha == "" {
			return fmt.Sprintf("Color(hex: %s)", strconv.Quote(hex))
		}
		r, g, b := hexToRGB01(hex)
		return fmt.Sprintf("Color(r: %s, g: %s, b: %s, a: %s)", r, g, b, alpha)
	case *ast.Vector:
		if len(v.Elems) < 3 {
			return e.errf(n.Pos(), "color vector needs at least 3 components")
		}
		if alpha == "" && len(v.Elems) >= 4 {
			alpha = e.expr(v.Elems[3], kNumber)
		}
		if alpha == "" {
			hex, ok := vectorColorHex(v)
			if !ok {
				return e.errf(n.Pos(), "non-literal color components")
			}
			return fmt.Sprintf("Color(hex: %s)", strconv.Quote(hex))
		}
		return fmt.Sprintf("Color(r: %s, g: %s, b: %s, a: %s)",
			e.expr(v.Elems[0], kNumber), e.expr(v.Elems[1], kNumber), e.expr(v.Elems[2], kNumber), alpha)
	default:
		// A runtime color value (a parameter or other expression). We can't
		// resolve CSS color names at transpile time, so we pass the string
		// through to Color(hex:) and let the runtime decide. The user is
		// responsible for ensuring the value is a hex string (`#RRGGBB`) at
		// runtime — SCAD CSS color names won't resolve.
		expr := e.expr(c, kNumber)
		if alpha == "" {
			return fmt.Sprintf("Color(hex: %s)", expr)
		}
		// With an explicit alpha, decompose the hex client-side at runtime is
		// not currently supported. Fall back to the historical error.
		return e.errf(n.Pos(), "non-literal color value with alpha")
	}
}

// hexToRGB01 converts a "#RRGGBB" string to three [0,1] decimal component
// strings, used when an alpha must be carried alongside a named color.
func hexToRGB01(hex string) (r, g, b string) {
	comp := func(s string) string {
		v, _ := strconv.ParseInt(s, 16, 0)
		return strconv.FormatFloat(float64(v)/255.0, 'g', -1, 64)
	}
	return comp(hex[1:3]), comp(hex[3:5]), comp(hex[5:7])
}

// vectorColorHex converts an OpenSCAD [r,g,b] or [r,g,b,a] literal (components
// in [0,1]) to a "#RRGGBB" hex string. The alpha component is accepted but not
// represented in the hex form. Returns false if a component is not a numeric
// literal.
func vectorColorHex(v *ast.Vector) (string, bool) {
	if len(v.Elems) < 3 {
		return "", false
	}
	var rgb [3]int
	for i := 0; i < 3; i++ {
		f, ok := numLitValue(v.Elems[i])
		if !ok {
			return "", false
		}
		rgb[i] = clamp255(f)
	}
	return fmt.Sprintf("#%02X%02X%02X", rgb[0], rgb[1], rgb[2]), true
}

// numLitValue extracts a float from a numeric literal (optionally negated).
func numLitValue(x ast.Expr) (float64, bool) {
	switch n := x.(type) {
	case *ast.Num:
		f, err := strconv.ParseFloat(n.Text, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case *ast.Unary:
		if n.Op == "-" {
			f, ok := numLitValue(n.X)
			return -f, ok
		}
	}
	return 0, false
}

// clamp255 maps a [0,1] color component to a [0,255] byte, clamping out-of-range
// inputs.
func clamp255(f float64) int {
	v := int(math.Round(f * 255))
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// cssColorHex maps a common CSS color name to its "#RRGGBB" hex value via
// the shared colorname table. The second result is false for unknown names
// (the caller emits #000000 + an error).
func cssColorHex(name string) (string, bool) {
	hex, ok := colorname.Hex(name)
	if !ok {
		return "#000000", false
	}
	return hex, true
}
