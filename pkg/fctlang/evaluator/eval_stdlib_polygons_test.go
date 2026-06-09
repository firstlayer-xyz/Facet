package evaluator

import (
	"math"
	"testing"
)

// Ngon is a corner-anchored regular polygon (bbox min at origin, like Square /
// Circle). A hexagon of circumradius 10 has a vertex on +X, so its centered
// extent is 2r wide (vertex-to-vertex) and r*sqrt(3) tall (flat-to-flat);
// corner-anchored that is 20 x 17.32 with min at the origin. The extruded
// volume is the regular-hexagon area (3*sqrt(3)/2 * r^2) times the height.
func TestEvalNgonHexagon(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Ngon(n: 6, r: 10 mm).Extrude(z: 2 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if math.Abs(minX) > 0.05 || math.Abs(minY) > 0.05 || math.Abs(minZ) > 0.05 {
		t.Errorf("hexagon not corner-anchored: min (%.3f,%.3f,%.3f), want (0,0,0)", minX, minY, minZ)
	}
	if math.Abs(maxX-20) > 0.05 || math.Abs(maxY-17.3205) > 0.05 || math.Abs(maxZ-2) > 0.05 {
		t.Errorf("hexagon bbox max (%.3f,%.3f,%.3f), want (20, 17.32, 2)", maxX, maxY, maxZ)
	}
	wantVol := 3 * math.Sqrt(3) / 2 * 100 * 2 // 519.6
	if v := s.Volume(); math.Abs(v-wantVol) > 1 {
		t.Errorf("hexagon volume = %.3f, want ~%.1f", v, wantVol)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("hexagon: comps=%d genus=%d, want 1/0", c, g)
	}
}

// The diameter overload halves d to the circumradius, and a 5-gon exercises a
// non-hexagonal vertex count. Pentagon circumradius 10: vertex on +X, others at
// 72-degree steps, so the centered extent is 18.09 x 19.02 (corner-anchored,
// min at origin). Area = (5/2) r^2 sin(72deg), volume that times the height.
func TestEvalNgonPentagonByDiameter(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Ngon(n: 5, d: 20 mm).Extrude(z: 2 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if math.Abs(minX) > 0.05 || math.Abs(minY) > 0.05 || math.Abs(minZ) > 0.05 {
		t.Errorf("pentagon not corner-anchored: min (%.3f,%.3f,%.3f)", minX, minY, minZ)
	}
	if math.Abs(maxX-18.09) > 0.05 || math.Abs(maxY-19.021) > 0.05 || math.Abs(maxZ-2) > 0.05 {
		t.Errorf("pentagon bbox max (%.3f,%.3f,%.3f), want (18.09, 19.02, 2)", maxX, maxY, maxZ)
	}
	wantVol := 2.5 * 100 * math.Sin(72*math.Pi/180) * 2 // ~475.5
	if v := s.Volume(); math.Abs(v-wantVol) > 1 {
		t.Errorf("pentagon volume = %.3f, want ~%.1f", v, wantVol)
	}
}

// Star is a 2n-point polygon alternating outer radius r and inner radius ir, at
// angle 180*i/n (the BOSL2 vertex placement). Corner-anchored like Ngon. For
// n=5, r=10, ir=4 the centered extent is 18.09 x 19.02; volume is the star
// area (n*r*ir*sin(pi/n)) times the height. The dent between points makes it a
// non-convex but still single, hole-free solid.
func TestEvalStarFivePoint(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Star(n: 5, r: 10 mm, ir: 4 mm).Extrude(z: 2 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if math.Abs(minX) > 0.05 || math.Abs(minY) > 0.05 || math.Abs(minZ) > 0.05 {
		t.Errorf("star not corner-anchored: min (%.3f,%.3f,%.3f)", minX, minY, minZ)
	}
	if math.Abs(maxX-18.09) > 0.05 || math.Abs(maxY-19.021) > 0.05 || math.Abs(maxZ-2) > 0.05 {
		t.Errorf("star bbox max (%.3f,%.3f,%.3f), want (18.09, 19.02, 2)", maxX, maxY, maxZ)
	}
	wantVol := 5.0 * 10 * 4 * math.Sin(math.Pi/5) * 2 // ~235.1
	if v := s.Volume(); math.Abs(v-wantVol) > 1 {
		t.Errorf("star volume = %.3f, want ~%.1f", v, wantVol)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("star: comps=%d genus=%d, want 1/0", c, g)
	}
}

// The diameter overload halves d/id; an 8-point star exercises a larger count.
// With outer radius 10 it has outer points on +/-X and +/-Y, so the centered
// extent is exactly 20 x 20 (corner-anchored, min at origin).
func TestEvalStarEightPointByDiameter(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Star(n: 8, d: 20 mm, id: 14 mm).Extrude(z: 2 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if math.Abs(minX) > 0.05 || math.Abs(minY) > 0.05 || math.Abs(minZ) > 0.05 {
		t.Errorf("8-star not corner-anchored: min (%.3f,%.3f,%.3f)", minX, minY, minZ)
	}
	if math.Abs(maxX-20) > 0.05 || math.Abs(maxY-20) > 0.05 || math.Abs(maxZ-2) > 0.05 {
		t.Errorf("8-star bbox max (%.3f,%.3f,%.3f), want (20, 20, 2)", maxX, maxY, maxZ)
	}
	wantVol := 8.0 * 10 * 7 * math.Sin(math.Pi/8) * 2 // ~428.6
	if v := s.Volume(); math.Abs(v-wantVol) > 1 {
		t.Errorf("8-star volume = %.3f, want ~%.1f", v, wantVol)
	}
}
