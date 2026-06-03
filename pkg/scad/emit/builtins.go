package emit

import (
	"fmt"
	"strconv"
	"strings"

	"facet/pkg/scad/ast"
)

// moduleCall dispatches a SCAD instantiation to its Facet emission.
func (e *Emitter) moduleCall(n *ast.ModuleCall) string {
	if isTransform(n.Name) {
		return e.transform(n)
	}
	if isDimChanger(n.Name) {
		return e.dimChanger(n)
	}
	if isBoolean(n.Name) {
		return e.boolean(n)
	}
	if n.Name == "hull" {
		return e.hull(n)
	}
	if n.Name == "intersection_for" {
		return e.intersectionFor(n)
	}
	if n.Name == "children" {
		return e.childrenRef(n)
	}
	switch n.Name {
	case "cube":
		return e.cube(n)
	case "sphere":
		return e.sphere(n)
	case "cylinder":
		return e.cylinder(n)
	case "circle":
		return e.circle(n)
	case "square":
		return e.square(n)
	case "polygon":
		return e.polygon(n)
	case "text":
		return e.text(n)
	case "polyhedron":
		return e.polyhedron(n)
	}
	if sym, ok := e.syms[n.Name]; ok {
		if sym.isFunc {
			return e.errf(n.Pos(), "%s: calling a function as a module is not supported", n.Name)
		}
		return e.userModuleCall(n, sym)
	}
	return e.errf(n.Pos(), "module '%s'", n.Name)
}

// arg returns the value for parameter `name` or the positional arg at `idx`.
// A named match takes precedence over the positional fallback.
func arg(n *ast.ModuleCall, name string, idx int) (ast.Expr, bool) {
	for _, a := range n.Args {
		if a.Name == name {
			return a.Value, true
		}
	}
	pos := -1
	for _, a := range n.Args {
		if a.Name == "" {
			pos++
			if pos == idx {
				return a.Value, true
			}
		}
	}
	return nil, false
}

// boolArg reports whether a boolean-valued arg is set true, matching the named
// `name` or the positional arg at `idx` (idx < 0 means named-only). OpenSCAD
// allows e.g. cube(size, center) with center as the 2nd positional argument.
func boolArg(n *ast.ModuleCall, name string, idx int) bool {
	if v, ok := arg(n, name, idx); ok {
		b, isB := v.(*ast.Bool)
		return isB && b.Val
	}
	return false
}

// rejectExtraArgs records an error for any argument an emitter does not
// translate: a named argument not in `allowed`, or a positional argument beyond
// `maxPos`. This enforces argument completeness — the transpiler never silently
// drops an argument it doesn't handle. Arguments with no geometric effect (e.g.
// `convexity`) are listed in `allowed` and dropped intentionally by the caller.
func (e *Emitter) rejectExtraArgs(n *ast.ModuleCall, maxPos int, allowed ...string) {
	allow := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allow[a] = true
	}
	pos := 0
	for _, a := range n.Args {
		if a.Name == "" {
			if pos >= maxPos {
				e.errf(n.Pos(), "%s: unexpected positional argument #%d", n.Name, pos+1)
			}
			pos++
			continue
		}
		if !allow[a.Name] {
			e.errf(n.Pos(), "%s: unsupported argument %q", n.Name, a.Name)
		}
	}
}

// is2D reports whether a primitive named `name` yields a Sketch (2D) rather
// than a Solid (3D).
func is2D(name string) bool {
	switch name {
	case "circle", "square", "polygon", "text":
		return true
	}
	return false
}

// radiusArg renders the radius/diameter selector for a primitive that accepts
// r or d. It prefers a named r, then named d, then the positional arg at posIdx
// (treated as r). It returns the keyword ("r"/"d"), the rendered Length, and
// whether a value was found.
func (e *Emitter) radiusArg(n *ast.ModuleCall, posIdx int) (key, val string, ok bool) {
	if v, found := arg(n, "r", -1); found {
		return "r", e.expr(v, kLength), true
	}
	if v, found := arg(n, "d", -1); found {
		return "d", e.expr(v, kLength), true
	}
	if posIdx >= 0 {
		if v, found := arg(n, "", posIdx); found {
			return "r", e.expr(v, kLength), true
		}
	}
	return "", "", false
}

// cube emits `cube(size, center)` → Facet Cube(...) with optional centering.
func (e *Emitter) cube(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "size", "center")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "cube without size")
	}
	var dims string
	if v, isVec := size.(*ast.Vector); isVec && len(v.Elems) == 3 {
		dims = fmt.Sprintf("Cube(x: %s, y: %s, z: %s)",
			e.expr(v.Elems[0], kLength), e.expr(v.Elems[1], kLength), e.expr(v.Elems[2], kLength))
	} else {
		dims = fmt.Sprintf("Cube(s: %s)", e.expr(size, kLength))
	}
	if boolArg(n, "center", 1) {
		dims += ".AlignCenter(pos: Vec3{})"
	}
	return dims
}

// sphere emits Sphere(...).AlignCenter(...). OpenSCAD centers spheres at the
// origin; Facet's Sphere is corner-origin, so recenter onto Vec3{}.
func (e *Emitter) sphere(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "r", "d", "$fn", "$fa", "$fs")
	key, val, ok := e.radiusArg(n, 0)
	if !ok {
		return e.errf(n.Pos(), "sphere without radius")
	}
	rMM, rOK := radiusMM(n, 0)
	return fmt.Sprintf("Sphere(%s: %s%s).AlignCenter(pos: Vec3{})",
		key, val, e.segmentsSuffix(n, rMM, rOK))
}

// cylinder emits Cylinder(...) or Frustum(...) with origin normalization.
// OpenSCAD centers cylinders in X/Y; Z runs 0..h unless center=true.
func (e *Emitter) cylinder(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "h", "r", "r1", "r2", "d", "d1", "d2", "center", "$fn", "$fa", "$fs")
	h, ok := arg(n, "h", 0)
	if !ok {
		return e.errf(n.Pos(), "cylinder without height")
	}
	hStr := e.expr(h, kLength)
	rMM, rOK := cylinderRadiusMM(n)
	segs := e.segmentsSuffix(n, rMM, rOK)

	r1, hasR1 := arg(n, "r1", -1)
	r2, hasR2 := arg(n, "r2", -1)
	d1, hasD1 := arg(n, "d1", -1)
	d2, hasD2 := arg(n, "d2", -1)

	var ctor string
	switch {
	case hasR1 || hasR2:
		ctor = fmt.Sprintf("Frustum(r1: %s, r2: %s, h: %s%s)",
			e.expr(r1, kLength), e.expr(r2, kLength), hStr, segs)
	case hasD1 || hasD2:
		ctor = fmt.Sprintf("Frustum(d1: %s, d2: %s, h: %s%s)",
			e.expr(d1, kLength), e.expr(d2, kLength), hStr, segs)
	default:
		// Per OpenSCAD, positional args are (h, r): h is idx 0, r is idx 1.
		key, val, found := e.radiusArg(n, 1)
		if !found {
			return e.errf(n.Pos(), "cylinder without radius")
		}
		ctor = fmt.Sprintf("Cylinder(%s: %s, h: %s%s)",
			key, val, hStr, segs)
	}

	if boolArg(n, "center", -1) {
		return ctor + ".AlignCenter(pos: Vec3{})"
	}
	return ctor + ".AlignCenter(pos: Vec3{}, z: false)"
}

// circle emits Circle(...).Move(...). OpenSCAD centers circles at the origin;
// Facet's Circle is corner-origin and Sketches have no AlignCenter, so shift
// by the negative radius (half the diameter for the d form).
func (e *Emitter) circle(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "r", "d", "$fn", "$fa", "$fs")
	key, val, ok := e.radiusArg(n, 0)
	if !ok {
		return e.errf(n.Pos(), "circle without radius")
	}
	rMM, rOK := radiusMM(n, 0)
	ctor := fmt.Sprintf("Circle(%s: %s%s)", key, val, e.segmentsSuffix(n, rMM, rOK))
	var off string
	if key == "d" {
		off = "-" + val + " / 2"
	} else {
		off = "-" + val
	}
	return fmt.Sprintf("%s.Move(x: %s, y: %s)", ctor, off, off)
}

// square emits Square(...) with optional centering. OpenSCAD squares are
// corner-origin by default (matching Facet); center=true shifts by half.
func (e *Emitter) square(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "size", "center")
	size, ok := arg(n, "size", 0)
	if !ok {
		return e.errf(n.Pos(), "square without size")
	}
	if v, isVec := size.(*ast.Vector); isVec && len(v.Elems) >= 2 {
		x := e.expr(v.Elems[0], kLength)
		y := e.expr(v.Elems[1], kLength)
		ctor := fmt.Sprintf("Square(x: %s, y: %s)", x, y)
		if boolArg(n, "center", 1) {
			return fmt.Sprintf("%s.Move(x: -%s / 2, y: -%s / 2)", ctor, x, y)
		}
		return ctor
	}
	// Scalar size s -> Square(s: ...) (one-arg overload).
	s := e.expr(size, kLength)
	ctor := fmt.Sprintf("Square(s: %s)", s)
	if boolArg(n, "center", 1) {
		return fmt.Sprintf("%s.Move(x: -%s / 2, y: -%s / 2)", ctor, s, s)
	}
	return ctor
}

// polygon emits Polygon(points: []Vec2[...]). With OpenSCAD's `paths` argument
// (index lists into points), the first path is the outer outline and the rest
// become holes, mapping to Facet's Polygon(points, holes) overload. No centering.
func (e *Emitter) polygon(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "points", "paths", "convexity")
	pts, ok := arg(n, "points", 0)
	if !ok {
		return e.errf(n.Pos(), "polygon without points")
	}
	pv, isVecLit := pts.(*ast.Vector)
	paths, hasPaths := arg(n, "paths", 1)

	// Treat a literal points vector that contains anything other than literal
	// 2-vectors (e.g. a function call returning a Vec2-shaped value, or a
	// variable reference) the same as a fully computed points list — the
	// elements only have a known shape at runtime, so we route through the
	// scad_v2 / scad_v2_path helpers.
	if isVecLit && !allLiteralVec2(pv.Elems) {
		isVecLit = false
	}

	// Case 1: computed (or mixed-literal) points.
	if !isVecLit {
		if !hasPaths {
			e.usesV2 = true
			return "Polygon(points: scad_v2(ps: " + e.expr(pts, kNumber) + "))"
		}
		return e.polygonComputedWithPaths(n, pts, paths)
	}

	points := pv.Elems

	// Case 2: literal points, no paths — the simple case.
	if !hasPaths {
		outer, ok := e.vec2Array(points)
		if !ok {
			return e.errf(n.Pos(), "polygon with non-Vec2 point")
		}
		return "Polygon(points: " + outer + ")"
	}

	// Case 3: literal points + paths — resolve indices at compile time.
	pathsVec, ok := paths.(*ast.Vector)
	if !ok || len(pathsVec.Elems) == 0 {
		return e.errf(n.Pos(), "polygon paths must be a non-empty literal list")
	}
	outerPts, ok := resolvePath(points, pathsVec.Elems[0])
	if !ok {
		return e.errf(n.Pos(), "polygon paths must be literal index lists into points")
	}
	outer, ok := e.vec2Array(outerPts)
	if !ok {
		return e.errf(n.Pos(), "polygon with non-Vec2 point")
	}
	if len(pathsVec.Elems) == 1 {
		return "Polygon(points: " + outer + ")"
	}
	// Outer array is untyped (`[ ... ]`); each hole is a typed `[]Vec2[...]`.
	var holes strings.Builder
	holes.WriteString("[")
	for i, hp := range pathsVec.Elems[1:] {
		holePts, ok := resolvePath(points, hp)
		if !ok {
			return e.errf(n.Pos(), "polygon paths must be literal index lists into points")
		}
		hole, ok := e.vec2Array(holePts)
		if !ok {
			return e.errf(n.Pos(), "polygon with non-Vec2 point")
		}
		if i > 0 {
			holes.WriteString(", ")
		}
		holes.WriteString(hole)
	}
	holes.WriteString("]")
	return "Polygon(points: " + outer + ", holes: " + holes.String() + ")"
}

// allLiteralVec2 reports whether every element is a literal 2-vector — the
// shape vec2Array consumes. False means at least one element is a function
// call, variable reference, or other non-vector expression and the points
// list has to be rendered as a runtime []]Number.
func allLiteralVec2(elems []ast.Expr) bool {
	for _, el := range elems {
		v, ok := el.(*ast.Vector)
		if !ok || len(v.Elems) < 2 {
			return false
		}
	}
	return true
}

// polygonComputedWithPaths emits a polygon whose `points` is a runtime
// expression and whose `paths` is a literal list of literal index lists. The
// emitted code routes each path through scad_v2_path, which indexes the
// runtime points and converts each chosen entry to a Vec2 (mm).
func (e *Emitter) polygonComputedWithPaths(n *ast.ModuleCall, pts ast.Expr, paths ast.Expr) string {
	pathsVec, ok := paths.(*ast.Vector)
	if !ok || len(pathsVec.Elems) == 0 {
		return e.errf(n.Pos(), "polygon paths must be a non-empty literal list")
	}
	ptsExpr := e.expr(pts, kNumber)
	render := func(p ast.Expr) (string, bool) {
		pv, ok := p.(*ast.Vector)
		if !ok {
			return "", false
		}
		var idx strings.Builder
		idx.WriteString("[")
		for i, idxExpr := range pv.Elems {
			if i > 0 {
				idx.WriteString(", ")
			}
			idx.WriteString(e.expr(idxExpr, kNumber))
		}
		idx.WriteString("]")
		return "scad_v2_path(ps: " + ptsExpr + ", indices: " + idx.String() + ")", true
	}
	outer, ok := render(pathsVec.Elems[0])
	if !ok {
		return e.errf(n.Pos(), "polygon paths must be literal index lists into points")
	}
	e.usesV2Path = true
	if len(pathsVec.Elems) == 1 {
		return "Polygon(points: " + outer + ")"
	}
	var holes strings.Builder
	holes.WriteString("[")
	for i, hp := range pathsVec.Elems[1:] {
		hole, ok := render(hp)
		if !ok {
			return e.errf(n.Pos(), "polygon paths must be literal index lists into points")
		}
		if i > 0 {
			holes.WriteString(", ")
		}
		holes.WriteString(hole)
	}
	holes.WriteString("]")
	return "Polygon(points: " + outer + ", holes: " + holes.String() + ")"
}

// polyhedron translates `polyhedron(points, faces)` into a Facet
// `Mesh{vertices, indices}.Solid()`. Points and faces flow through the scad_v3
// and scad_faces helpers, which work for both literal and variable inputs and
// fan-triangulate n-gon faces (Facet's Face is a triangle). The deprecated
// `triangles=` spelling of `faces` is accepted; `convexity` is a render-only
// hint and is dropped.
func (e *Emitter) polyhedron(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 2, "points", "faces", "triangles", "convexity")
	pts, ok := arg(n, "points", 0)
	if !ok {
		return e.errf(n.Pos(), "polyhedron without points")
	}
	faces, ok := arg(n, "faces", 1)
	if !ok {
		faces, ok = arg(n, "triangles", -1) // deprecated OpenSCAD spelling
		if !ok {
			return e.errf(n.Pos(), "polyhedron without faces")
		}
	}
	e.usesV3 = true
	e.usesFaces = true
	return "Mesh{vertices: scad_v3(ps: " + e.expr(pts, kNumber) +
		"), indices: scad_faces(fs: " + e.expr(faces, kNumber) + ")}.Solid()"
}

// vec2Array renders a slice of 2-element vector literals as a Facet
// `[]Vec2[{x:.., y:..}, ...]` array. ok is false if any element is not a
// literal 2-vector.
func (e *Emitter) vec2Array(elems []ast.Expr) (string, bool) {
	var b strings.Builder
	b.WriteString("[]Vec2[")
	for i, el := range elems {
		p, ok := el.(*ast.Vector)
		if !ok || len(p.Elems) < 2 {
			return "", false
		}
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "{x: %s, y: %s}", e.expr(p.Elems[0], kLength), e.expr(p.Elems[1], kLength))
	}
	b.WriteString("]")
	return b.String(), true
}

// resolvePath resolves an OpenSCAD path (a vector of literal integer indices)
// into the referenced point expressions from points. ok is false on a
// non-literal, non-integer, or out-of-range index.
func resolvePath(points []ast.Expr, path ast.Expr) ([]ast.Expr, bool) {
	pv, ok := path.(*ast.Vector)
	if !ok {
		return nil, false
	}
	out := make([]ast.Expr, 0, len(pv.Elems))
	for _, idxExpr := range pv.Elems {
		f, ok := numLitValue(idxExpr)
		if !ok || f < 0 || f != float64(int(f)) || int(f) >= len(points) {
			return nil, false
		}
		out = append(out, points[int(f)])
	}
	return out, true
}

// text emits Text("...", size[, font][, halign][, valign]). OpenSCAD's default
// size is 10; its halign/valign vocabulary matches Facet's. spacing, direction,
// language, and script are not yet translated; they error rather than be
// silently ignored.
func (e *Emitter) text(n *ast.ModuleCall) string {
	e.rejectExtraArgs(n, 1, "text", "size", "font", "halign", "valign")
	t, ok := arg(n, "text", 0)
	if !ok {
		return e.errf(n.Pos(), "text without string")
	}
	s, ok := t.(*ast.Str)
	if !ok {
		return e.errf(n.Pos(), "text with non-literal string")
	}
	sizeStr := "10 mm"
	if sz, found := arg(n, "size", -1); found {
		sizeStr = e.expr(sz, kLength)
	}
	out := fmt.Sprintf("Text(text: %s, s: %s", strconv.Quote(s.Value), sizeStr)
	// font/halign/valign are pass-through string literals with matching names.
	for _, name := range []string{"font", "halign", "valign"} {
		v, found := arg(n, name, -1)
		if !found {
			continue
		}
		sv, isStr := v.(*ast.Str)
		if !isStr {
			return e.errf(n.Pos(), "text %s must be a string literal", name)
		}
		out += fmt.Sprintf(", %s: %s", name, strconv.Quote(sv.Value))
	}
	return out + ")"
}
