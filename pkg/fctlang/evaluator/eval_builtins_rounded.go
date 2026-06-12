package evaluator

import (
	"fmt"
	"math"

	"facet/pkg/manifold"
)

// Internal rounding primitives behind the public Cube/Cylinder/Frustum fillet
// and chamfer overloads. Keeping them as builtins (not stdlib functions) lets
// the stdlib expose fillet and chamfer as cleanly separated overloads while the
// shared geometry — parameterised by a single inset `r` and a `bevel` flag — has
// exactly one implementation, hidden from the public API.

// edgeSetMode decodes an EdgeSet struct into a rounding mode: "all", "none", or
// one full axis group ("x"/"y"/"z"). An arbitrary subset returns ok=false (the
// transpiler/stdlib only support those forms, matching the old Facet assert).
func edgeSetMode(v value) (mode string, ok bool) {
	sv, isStruct := unwrap(v).(*structVal)
	if !isStruct || sv.typeName != "EdgeSet" {
		return "", false
	}
	get := func(name string) bool {
		b, _ := unwrap(sv.fields[name]).(bool)
		return b
	}
	count := func(names ...string) int {
		n := 0
		for _, nm := range names {
			if get(nm) {
				n++
			}
		}
		return n
	}
	z := count("frontLeft", "frontRight", "backLeft", "backRight") // edges along Z
	x := count("frontBottom", "frontTop", "backBottom", "backTop") // edges along X
	y := count("leftBottom", "leftTop", "rightBottom", "rightTop") // edges along Y
	switch {
	case z == 4 && x == 4 && y == 4:
		return "all", true
	case z == 0 && x == 0 && y == 0:
		return "none", true
	case z == 4 && x == 0 && y == 0:
		return "z", true
	case x == 4 && z == 0 && y == 0:
		return "x", true
	case y == 4 && z == 0 && x == 0:
		return "y", true
	}
	return "", false
}

// octahedronSolid is the chamfer corner solid: a regular octahedron with its
// bounding box at (0,0,0)..(2r,2r,2r), whose 45° faces form the bevels. Matches
// the stdlib Octahedron(r). Winding is normalised by CreateSolidFromMesh.
func octahedronSolid(r float64) (*manifold.Solid, error) {
	f := func(v float64) float32 { return float32(v) }
	verts := []float32{
		f(r), f(r), 0, f(r), f(r), f(2 * r),
		0, f(r), f(r), f(2 * r), f(r), f(r),
		f(r), 0, f(r), f(r), f(2 * r), f(r),
	}
	idx := []uint32{1, 3, 5, 1, 5, 2, 1, 2, 4, 1, 4, 3, 0, 5, 3, 0, 2, 5, 0, 4, 2, 0, 3, 4}
	return manifold.CreateSolidFromMesh(verts, idx)
}

// builtinCubeRounded implements _cube_rounded(x, y, z, r, bevel, edges): a box
// with its edges rounded (bevel=false) or beveled (bevel=true) by inset `r`,
// restricted to `edges`. Corner-origin, bounding box (0,0,0)..(x,y,z).
func (e *evaluator) builtinCubeRounded(args []value) (value, error) {
	const name = "_cube_rounded"
	if len(args) != 6 {
		return nil, fmt.Errorf("%s() expects 6 arguments, got %d", name, len(args))
	}
	x, err := requireLength(name, 1, args[0])
	if err != nil {
		return nil, err
	}
	y, err := requireLength(name, 2, args[1])
	if err != nil {
		return nil, err
	}
	z, err := requireLength(name, 3, args[2])
	if err != nil {
		return nil, err
	}
	r, err := requireLength(name, 4, args[3])
	if err != nil {
		return nil, err
	}
	bevel, isBool := unwrap(args[4]).(bool)
	if !isBool {
		return nil, fmt.Errorf("%s() argument 5 (bevel) must be a Bool", name)
	}
	mode, ok := edgeSetMode(args[5])
	if !ok {
		return nil, fmt.Errorf("Cube: per-edge rounding currently supports EDGES_ALL, EDGES_NONE, or one full axis group (EdgesAlongX/Y/Z)")
	}

	if r == 0 || mode == "none" {
		return manifold.CreateCube(x, y, z)
	}

	if mode == "all" {
		if 2*r > x || 2*r > y || 2*r > z {
			return nil, fmt.Errorf("Cube: rounding too large for the box dimensions (2*%g must fit in every side)", r)
		}
		// Convex hull of a rounding solid placed at each of the eight inset
		// corners — a sphere (fillet) or an octahedron (chamfer).
		var corner *manifold.Solid
		if bevel {
			corner, err = octahedronSolid(r)
		} else {
			corner, err = manifold.CreateSphere(r, 0)
		}
		if err != nil {
			return nil, err
		}
		d := 2 * r
		places := [][3]float64{
			{0, 0, 0}, {x - d, 0, 0}, {0, y - d, 0}, {x - d, y - d, 0},
			{0, 0, z - d}, {x - d, 0, z - d}, {0, y - d, z - d}, {x - d, y - d, z - d},
		}
		solids := make([]*manifold.Solid, len(places))
		for i, p := range places {
			solids[i] = corner.Translate(p[0], p[1], p[2])
		}
		composed, err := manifold.ComposeSolids(solids)
		if err != nil {
			return nil, err
		}
		return composed.Hull(), nil
	}

	// One full axis group: round/bevel the 2D cross-section perpendicular to the
	// axis, extrude, then orient so the bbox still spans (0,0,0)..(x,y,z).
	// Fillet is a round offset (segments 0); chamfer a single-segment offset
	// whose distance is r·√2 (the 45° face width that cuts back r per edge).
	roundProfile := func(sx, sy float64) *manifold.Sketch {
		sq, _ := manifold.CreateSquare(sx, sy)
		if bevel {
			cham := r * math.Sqrt2
			return sq.Offset(-cham, 1).Offset(cham, 1)
		}
		return sq.Offset(-r, 0).Offset(r, 0)
	}
	switch mode {
	case "z":
		if 2*r > x || 2*r > y {
			return nil, fmt.Errorf("Cube: rounding too large for the Z-edge cross-section")
		}
		return roundProfile(x, y).Extrude(z, 1, 0, 1, 1)
	case "y":
		if 2*r > x || 2*r > z {
			return nil, fmt.Errorf("Cube: rounding too large for the Y-edge cross-section")
		}
		sol, err := roundProfile(x, z).Extrude(y, 1, 0, 1, 1)
		if err != nil {
			return nil, err
		}
		return sol.Rotate(-90, 0, 0).Translate(0, 0, z), nil
	default: // "x"
		if 2*r > y || 2*r > z {
			return nil, fmt.Errorf("Cube: rounding too large for the X-edge cross-section")
		}
		sol, err := roundProfile(z, y).Extrude(x, 1, 0, 1, 1)
		if err != nil {
			return nil, err
		}
		return sol.Rotate(0, 90, 0).Translate(0, 0, z), nil
	}
}

// rimTorus reproduces the stdlib Torus(r_maj, r_min): a circle of radius rMin
// (default resolution) swept around the Z axis at major radius rMaj, with its
// circumferential resolution set by segs.
func rimTorus(rMaj, rMin float64, segs int) (*manifold.Solid, error) {
	c, err := manifold.CreateCircle(rMin, 0)
	if err != nil {
		return nil, err
	}
	return c.Translate(rMaj-rMin, -rMin).Revolve(segs, 360)
}

// builtinFrustumRounded implements _frustum_rounded(r1, r2, h, r, bevel, segments):
// a frustum/cylinder with its top and bottom rims rounded (bevel=false, the hull
// of two rim tori) or beveled (bevel=true, a 45°-clipped revolved profile, which
// requires equal radii). Corner-origin.
func (e *evaluator) builtinFrustumRounded(args []value) (value, error) {
	const name = "_frustum_rounded"
	if len(args) != 6 {
		return nil, fmt.Errorf("%s() expects 6 arguments, got %d", name, len(args))
	}
	r1, err := requireLength(name, 1, args[0])
	if err != nil {
		return nil, err
	}
	r2, err := requireLength(name, 2, args[1])
	if err != nil {
		return nil, err
	}
	h, err := requireLength(name, 3, args[2])
	if err != nil {
		return nil, err
	}
	r, err := requireLength(name, 4, args[3])
	if err != nil {
		return nil, err
	}
	bevel, isBool := unwrap(args[4]).(bool)
	if !isBool {
		return nil, fmt.Errorf("%s() argument 5 (bevel) must be a Bool", name)
	}
	segN, err := requireNumber(name, 6, args[5])
	if err != nil {
		return nil, err
	}
	segs, err := requireCount(name, 6, segN, maxSegments)
	if err != nil {
		return nil, err
	}

	if r == 0 {
		return manifold.CreateCylinder(h, r1, r2, segs)
	}

	if bevel {
		// A flat 45° rim cut: a rectangle with its two outer corners clipped,
		// revolved. A tapered frustum's rim cut is not this simple revolve, so
		// require equal radii rather than emit a wrong shape.
		if r1 != r2 {
			return nil, fmt.Errorf("Frustum: chamfer is only supported on equal-radius cylinders (r1 == r2)")
		}
		if r > r1 {
			return nil, fmt.Errorf("Frustum: chamfer too large for radius")
		}
		if 2*r > h {
			return nil, fmt.Errorf("Frustum: chamfer too large for height")
		}
		prof, err := manifold.CreatePolygon([]manifold.Point2D{
			{X: 0, Y: 0}, {X: r1 - r, Y: 0}, {X: r1, Y: r},
			{X: r1, Y: h - r}, {X: r1 - r, Y: h}, {X: 0, Y: h},
		}, nil)
		if err != nil {
			return nil, err
		}
		sol, err := prof.Revolve(segs, 360)
		if err != nil {
			return nil, err
		}
		return sol.Translate(r1, r1, 0), nil
	}

	// Fillet: hull the two rim tori, shifted so the bbox min corner is at origin.
	if r > r1 || r > r2 {
		return nil, fmt.Errorf("Frustum: fillet too large for a radius")
	}
	if 2*r > h {
		return nil, fmt.Errorf("Frustum: fillet too large for height")
	}
	tb, err := rimTorus(r1-r, r, segs)
	if err != nil {
		return nil, err
	}
	tt, err := rimTorus(r2-r, r, segs)
	if err != nil {
		return nil, err
	}
	composed, err := manifold.ComposeSolids([]*manifold.Solid{
		tb.Translate(0, 0, r), tt.Translate(0, 0, h-r),
	})
	if err != nil {
		return nil, err
	}
	rmax := math.Max(r1, r2)
	return composed.Hull().Translate(rmax, rmax, 0), nil
}
