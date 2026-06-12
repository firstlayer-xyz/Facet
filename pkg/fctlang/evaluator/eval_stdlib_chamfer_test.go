package evaluator

import (
	"context"
	"math"
	"strings"
	"testing"

	"facet/pkg/manifold"
)

// evalSolid evaluates a Main returning a Solid and returns the (unioned) result,
// so tests can assert on real geometry (volume, topology), not just bounds.
func evalSolid(t *testing.T, src string) *manifold.Solid {
	t.Helper()
	prog := parseTestProg(t, src)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatalf("no solids produced")
	}
	s := result.Solids[0]
	for _, x := range result.Solids[1:] {
		s = s.Union(x)
	}
	return s
}

// evalErr evaluates src expecting an error, returning its message.
func evalErr(t *testing.T, src string) string {
	t.Helper()
	prog := parseTestProg(t, src)
	if _, err := Eval(context.Background(), prog, testMainKey, nil, "Main"); err != nil {
		return err.Error()
	}
	t.Fatalf("expected an error, got none")
	return ""
}

// An octahedron spans 0..2r with volume (4/3)r^3, oriented OUTWARD (positive
// volume — the hand-wound Mesh faces must point out).
func TestEvalOctahedron(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Octahedron(r: 3 mm); }`)
	if v := s.Volume(); v < 35 || v > 37 {
		t.Errorf("octahedron volume = %v, want ~36 ((4/3)*27, outward normals)", v)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("octahedron: comps=%d genus=%d, want 1/0", c, g)
	}
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if minX < -0.1 || minY < -0.1 || minZ < -0.1 || maxX < 5.9 || maxX > 6.1 {
		t.Errorf("octahedron bounds (%v,%v,%v)-(%v,%v,%v), want (0,0,0)-(6,6,6)", minX, minY, minZ, maxX, maxY, maxZ)
	}
}

// A chamfered cube keeps the bounding box but bevels every edge: less volume than
// the plain box, and a simple solid.
func TestEvalChamferedCube(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Cube(s: 20 mm, chamfer: 3 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if minX < -0.1 || minY < -0.1 || minZ < -0.1 {
		t.Errorf("not corner-origin: min (%v,%v,%v)", minX, minY, minZ)
	}
	if maxX < 19.5 || maxX > 20.5 || maxY < 19.5 || maxY > 20.5 || maxZ < 19.5 || maxZ > 20.5 {
		t.Errorf("chamfered cube bbox max (%v,%v,%v), want ~(20,20,20)", maxX, maxY, maxZ)
	}
	if v := s.Volume(); v <= 0 || v >= 8000 {
		t.Errorf("chamfered cube volume = %v, want 0 < v < 8000 (edges beveled)", v)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("chamfered cube: comps=%d genus=%d, want 1/0", c, g)
	}
}

// chamfer: 0 mm leaves the plain cube unchanged.
func TestEvalCubeChamferZeroUnchanged(t *testing.T) {
	plain := evalSolid(t, `fn Main() Solid { return Cube(s: 10 mm); }`)
	zero := evalSolid(t, `fn Main() Solid { return Cube(s: 10 mm, chamfer: 0 mm); }`)
	if math.Abs(plain.Volume()-zero.Volume()) > 0.001 {
		t.Errorf("chamfer:0 volume %v != plain %v", zero.Volume(), plain.Volume())
	}
}

// A chamfered cylinder keeps the plain outer bounds but bevels both rims.
func TestEvalChamferedCylinder(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Cylinder(r: 10 mm, h: 20 mm, chamfer: 3 mm); }`)
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if minX < -0.5 || minZ < -0.5 {
		t.Errorf("not corner-origin: x=%v z=%v", minX, minZ)
	}
	if maxX < 19.5 || maxX > 20.5 {
		t.Errorf("x-extent max %v, want ~20 (radius 10)", maxX)
	}
	if maxZ < 19.5 || maxZ > 20.5 {
		t.Errorf("z-extent max %v, want ~20 (height)", maxZ)
	}
	if v := s.Volume(); v <= 0 || v >= 6284 { // plain cylinder pi*100*20 ~ 6283
		t.Errorf("chamfered cyl volume %v, want 0 < v < 6283 (rims beveled)", v)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("chamfered cyl: comps=%d genus=%d, want 1/0", c, g)
	}
}

// fillet and chamfer are SEPARATE overloads — an edge is rounded or beveled, not
// both. Passing both has no matching overload (a compile-time rejection, rather
// than the old runtime "use one, not both" assert).
func TestEvalCubeFilletAndChamferErrors(t *testing.T) {
	if msg := evalErr(t, `fn Main() Solid { return Cube(s: 20 mm, fillet: 2 mm, chamfer: 2 mm); }`); !strings.Contains(msg, "no matching overload") {
		t.Errorf("expected a no-matching-overload error for fillet+chamfer, got: %s", msg)
	}
}

// A tapered frustum's rim cut isn't the simple revolve, so chamfer requires
// equal radii rather than emitting a wrong shape.
func TestEvalFrustumChamferTaperedErrors(t *testing.T) {
	if msg := evalErr(t, `fn Main() Solid { return Frustum(r1: 10 mm, r2: 5 mm, h: 20 mm, chamfer: 2 mm); }`); !strings.Contains(msg, "equal-radius") {
		t.Errorf("expected an equal-radius error, got: %s", msg)
	}
}
