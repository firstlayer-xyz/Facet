package evaluator

import (
	"context"
	"testing"
)

// A loft of two convex profiles must produce a convex solid — its volume equals
// its own convex hull. Rounded-rect profiles with different corner radii (a
// tapered rounded foot) previously twisted: facet_loft corresponded points by
// arc-length, so a corner on one ring mapped to an edge on the next, denting the
// side wall inward (volume well below the hull).
func TestEvalLoftTaperedRoundedIsConvex(t *testing.T) {
	src := `
fn Main() {
    var lo = Square(x: 30 mm, y: 30 mm).Fillet(r: 2 mm);
    var hi = Square(x: 30 mm, y: 30 mm).Fillet(r: 12 mm);
    var lofted = Loft(profiles: [lo, hi], heights: [0 mm, 10 mm]);
    var hull = Hull(arr: [lofted]);
    assert lofted.Volume() > hull.Volume() * 0.97, "loft of convex profiles must be convex (side wall not twisted)";
    return lofted;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

// A loft of profiles that each carry a HOLE (a CW inner contour) at different
// hole sizes must keep the hole through the taper. This exercises the
// winding-aware branch of the angular resampling (sign = -1 for a CW hole): if
// the hole's winding were mishandled it would self-intersect or fill in. The
// hole is a 5mm→10mm taper through a 30×30×10 block (~9000 mm³ solid), so a
// surviving hole leaves the volume well below the block but above empty.
func TestEvalLoftHoleSurvivesWinding(t *testing.T) {
	src := `
fn Main() {
    var loOuter = Square(x: 30 mm, y: 30 mm);
    var loHole  = Circle(r: 5 mm).Move(x: 15 mm, y: 15 mm);
    var hiOuter = Square(x: 30 mm, y: 30 mm);
    var hiHole  = Circle(r: 10 mm).Move(x: 15 mm, y: 15 mm);
    var lofted = Loft(profiles: [loOuter - loHole, hiOuter - hiHole], heights: [0 mm, 10 mm]);
    assert lofted.Volume() < 8500, "hole must survive the loft (CW winding preserved)";
    assert lofted.Volume() > 5000, "but most of the block remains";
    return lofted;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}
