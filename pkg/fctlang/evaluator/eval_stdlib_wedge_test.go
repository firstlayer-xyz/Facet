package evaluator

import (
	"math"
	"testing"
)

// Wedge is a right-triangular ramp filling half of its x*y*z bounding box:
// full height z along the y=0 edge, tapering to 0 at y=z... it is corner-
// anchored (min corner at origin, like Cube). The bounding box alone can't
// tell a ramp from a box, so the volume check (half of x*y*z) is what proves
// it is actually a wedge.
func TestEvalWedge(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid { return Wedge(x: 10 mm, y: 8 mm, z: 6 mm); }`)
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if math.Abs(minX) > 0.02 || math.Abs(minY) > 0.02 || math.Abs(minZ) > 0.02 {
		t.Errorf("wedge not corner-anchored: min (%.3f,%.3f,%.3f), want (0,0,0)", minX, minY, minZ)
	}
	if math.Abs(maxX-10) > 0.02 || math.Abs(maxY-8) > 0.02 || math.Abs(maxZ-6) > 0.02 {
		t.Errorf("wedge bbox max (%.3f,%.3f,%.3f), want (10, 8, 6)", maxX, maxY, maxZ)
	}
	wantVol := 10.0 * 8 * 6 / 2 // 240 — half the box
	if v := s.Volume(); math.Abs(v-wantVol) > 0.5 {
		t.Errorf("wedge volume = %.3f, want %.1f (half of the 10x8x6 box)", v, wantVol)
	}
	if c, g := s.NumComponents(), s.Genus(); c != 1 || g != 0 {
		t.Errorf("wedge: comps=%d genus=%d, want 1/0", c, g)
	}
}
