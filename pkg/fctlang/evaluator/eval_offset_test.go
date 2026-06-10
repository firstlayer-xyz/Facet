package evaluator

import (
	"context"
	"testing"
)

// Offset(+2) on a cube must grow its volume; the result must mesh non-empty.
func TestStdlibSolidOffsetGrows(t *testing.T) {
	src := `
fn Main() {
    var base = Cube(s: 20 mm).Volume();
    var off  = Cube(s: 20 mm).Offset(delta: 2 mm).Volume();
    var ratio = off / base;
    assert ratio > 1.0, "Offset(+2) should grow the cube";
    return Cube(s: 20 mm).Offset(delta: 2 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh")
	}
}

// Round(cube, r) ~ the exact Cube(fillet: r): opening rounds the 12 convex
// edges by r, like the analytic hull-of-spheres. Volumes agree within 15%
// at coarse resolution (1 mm grid); coarse meshing widens the tolerance.
func TestStdlibRoundMatchesExactFillet(t *testing.T) {
	src := `
fn Main() {
    var approx = Cube(s: 20 mm).Round(r: 3 mm, resolution: 1 mm).Volume();
    var exact  = Cube(s: 20 mm, fillet: 3 mm).Volume();
    var ratio = approx / exact;
    assert ratio > 0.85 && ratio < 1.15, "Round should match exact fillet within 15%";
    return Cube(s: 20 mm).Round(r: 3 mm, resolution: 1 mm);
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// Cove fills a concave (interior) edge, so an L-shape gains volume.
func TestStdlibCoveAddsMaterialAtConcaveEdge(t *testing.T) {
	src := `
fn Main() {
    var l     = Cube(x: 20 mm, y: 20 mm, z: 8 mm) + Cube(x: 8 mm, y: 20 mm, z: 20 mm);
    var coved = l.Cove(r: 3 mm, resolution: 2 mm);
    var ratio = coved.Volume() / l.Volume();
    assert ratio > 1.0, "Cove should add material at the concave edge";
    return coved;
}
`
	prog := parseTestProg(t, src)
	if _, err := evalMerged(context.Background(), prog, nil); err != nil {
		t.Fatalf("eval error: %v", err)
	}
}
