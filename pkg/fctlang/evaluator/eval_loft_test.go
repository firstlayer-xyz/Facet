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
