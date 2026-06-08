//go:build cgo

package scad

import (
	"context"
	"os"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
)

// These tests close the gap that the golden/type-check tests leave: they actually
// EVALUATE the transpiled BOSL2 to geometry and assert on real rendered output
// (bounds, volume, topology), so a wrong anchor, a flipped reorientation, a
// non-manifold primitive, or a silently dropped child can't pass unnoticed. They
// link the manifold kernel, so the whole file is CGO-only.

// renderBosl2Solid transpiles a BOSL2 .scad source, evaluates the resulting
// Facet, and returns the union of all static solids it produced.
func renderBosl2Solid(t *testing.T, scadSrc string) *manifold.Solid {
	t.Helper()
	res, err := Transpile(scadSrc, "part.scad")
	if err != nil {
		t.Fatalf("transpile: %v", err)
	}
	ctx := context.Background()
	const key = "<transpiled>"
	prog, err := loader.Load(ctx, res.Facet, key, parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("load transpiled Facet: %v\n%s", err, res.Facet)
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		t.Fatalf("transpiled Facet failed type-check: %v\n%s", errs[0], res.Facet)
	}
	result, err := evaluator.Eval(ctx, prog, key, nil, "Main")
	if err != nil {
		t.Fatalf("eval transpiled Facet: %v\n%s", err, res.Facet)
	}
	solids, err := result.StaticSolids(0)
	if err != nil {
		t.Fatalf("extract solids: %v", err)
	}
	if len(solids) == 0 {
		t.Fatalf("no solids produced:\n%s", res.Facet)
	}
	out := solids[0]
	for _, s := range solids[1:] {
		out = out.Union(s)
	}
	return out
}

func extents(s *manifold.Solid) (dx, dy, dz float64) {
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	return maxX - minX, maxY - minY, maxZ - minZ
}

func near(got, want, tol float64) bool { return got-want <= tol && want-got <= tol }

// Every 3D BOSL2 corpus file renders to a valid, non-empty, manifold solid:
// positive volume, at least one component, and finite non-degenerate bounds.
// This catches non-manifold output, empty geometry, and NaN/Inf coordinates
// across the whole vocabulary — none of which the text goldens can see.
func TestBOSL2Render_CorpusProducesValidSolids(t *testing.T) {
	// The 2D corpus files (bosl2_2d, bosl2_star) render to Sketches, not Solids.
	for _, name := range []string{
		"bosl2_attachment", "bosl2_copies", "bosl2_diff", "bosl2_distributors",
		"bosl2_oriented", "bosl2_primitives", "bosl2_prismoid", "bosl2_radial",
		"bosl2_torus", "bosl2_tube", "bosl2_wedge",
	} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("testdata/" + name + ".scad")
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			s := renderBosl2Solid(t, string(src))
			if v := s.Volume(); !(v > 0) {
				t.Errorf("volume = %v, want > 0", v)
			}
			if n := s.NumComponents(); n < 1 {
				t.Errorf("components = %d, want >= 1", n)
			}
			dx, dy, dz := extents(s)
			if !(dx > 0 && dy > 0 && dz > 0) || dx > 1e6 || dy > 1e6 || dz > 1e6 {
				t.Errorf("degenerate/unbounded extents: %v x %v x %v", dx, dy, dz)
			}
		})
	}
}

// A tube is hollow: it has a tunnel through it (genus 1), and its outer diameter
// sets the bounds. A silently-solid tube would be genus 0.
func TestBOSL2Render_TubeIsHollow(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ntube(h=6, od=20, id=14);\n")
	if g := s.Genus(); g != 1 {
		t.Errorf("genus = %d, want 1 (a hollow ring has one tunnel)", g)
	}
	dx, _, dz := extents(s)
	if !near(dx, 20, 1.0) {
		t.Errorf("x-extent = %v, want ~20 (od)", dx)
	}
	if !near(dz, 6, 0.5) {
		t.Errorf("z-extent = %v, want ~6 (h)", dz)
	}
}

// diff() with tag("remove") really subtracts: a drilled, pocketed plate has less
// volume than the solid plate, and the through-hole leaves a tunnel (genus >= 1).
func TestBOSL2Render_DiffSubtracts(t *testing.T) {
	src := "include <BOSL2/std.scad>\n" +
		"diff() cuboid([40, 24, 10]) {\n" +
		"    tag(\"remove\") cyl(h=30, d=8);\n" +
		"    position(TOP) tag(\"remove\") cyl(h=6, d=16);\n" +
		"}\n"
	s := renderBosl2Solid(t, src)
	if v := s.Volume(); !(v > 0 && v < 40*24*10) {
		t.Errorf("volume = %v, want 0 < v < 9600 (material removed)", v)
	}
	if g := s.Genus(); g < 1 {
		t.Errorf("genus = %d, want >= 1 (the through-hole is a tunnel)", g)
	}
}

// grid_copies makes n[0]*n[1] separate copies at the right spacing: a 3x2 grid of
// non-touching pegs is 6 components spanning the expected extents.
func TestBOSL2Render_GridCopiesExtent(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ngrid_copies(spacing=[12, 12], n=[3, 2]) cyl(h=6, r=2);\n")
	if n := s.NumComponents(); n != 6 {
		t.Errorf("components = %d, want 6 (3x2 pegs)", n)
	}
	dx, dy, dz := extents(s)
	if !near(dx, 28, 1.0) { // cols at -12,0,12, each radius 2 -> -14..14
		t.Errorf("x-extent = %v, want ~28", dx)
	}
	if !near(dy, 16, 1.0) { // rows at -6,6, each radius 2 -> -8..8
		t.Errorf("y-extent = %v, want ~16", dy)
	}
	if !near(dz, 6, 0.5) {
		t.Errorf("z-extent = %v, want ~6", dz)
	}
}

// attach(TOP) stacks the child on the parent's top face: a 12-tall cube with a
// 16-tall cylinder on top spans 28 in Z, as one connected component.
func TestBOSL2Render_AttachStacksOnTop(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([30, 30, 12]) attach(TOP) cyl(h=16, r=4);\n")
	if n := s.NumComponents(); n != 1 {
		t.Errorf("components = %d, want 1 (child touches parent)", n)
	}
	_, _, dz := extents(s)
	if !near(dz, 28, 1.0) { // cube z -6..6, cyl on top to +22
		t.Errorf("z-extent = %v, want ~28 (12 cube + 16 cyl)", dz)
	}
}

// A wedge is half its bounding box (a box sliced corner to corner) and must be a
// solid with OUTWARD-facing normals — i.e. positive volume, not an inside-out
// mesh. [20,12,8] -> 0.5 * 20*12*8 = 960.
func TestBOSL2Render_WedgeVolume(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nwedge([20, 12, 8], center=true);\n")
	if v := s.Volume(); !near(v, 960, 1.0) {
		t.Errorf("wedge volume = %v, want ~960 (half the bounding box, normals outward)", v)
	}
}

// attach(RIGHT) reorients the child so its axis points along +X: a cylinder
// attached to the right face of a 20-cube extends the X span to ~30. A wrong
// reorientation would leave it pointing up (z), not out (x).
func TestBOSL2Render_AttachReorientsToRight(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(RIGHT) cyl(h=10, r=4);\n")
	dx, _, dz := extents(s)
	if !near(dx, 30, 1.0) { // cube x -10..10, cyl out to +20
		t.Errorf("x-extent = %v, want ~30 (cyl points +X)", dx)
	}
	if !near(dz, 20, 1.0) { // unchanged by the reorientation
		t.Errorf("z-extent = %v, want ~20 (cube only)", dz)
	}
}

// cuboid(rounding=R) rounds the edges: the bounding box is unchanged but corner
// material is removed (less volume than the plain box), and it stays a simple
// solid (one component, genus 0).
func TestBOSL2Render_CuboidRounding(t *testing.T) {
	plain := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]);\n")
	rounded := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20], rounding=3);\n")
	pdx, pdy, pdz := extents(plain)
	rdx, rdy, rdz := extents(rounded)
	if !near(rdx, pdx, 0.5) || !near(rdy, pdy, 0.5) || !near(rdz, pdz, 0.5) {
		t.Errorf("rounded bbox %v x %v x %v should match plain %v x %v x %v", rdx, rdy, rdz, pdx, pdy, pdz)
	}
	if !(rounded.Volume() < plain.Volume()) {
		t.Errorf("rounded volume %v should be < plain %v (corners removed)", rounded.Volume(), plain.Volume())
	}
	if rounded.NumComponents() != 1 || rounded.Genus() != 0 {
		t.Errorf("rounded cuboid should be a simple solid: comps=%d genus=%d", rounded.NumComponents(), rounded.Genus())
	}
}

// cyl(rounding=R) rounds both rims: same height/bounds, less volume, simple solid.
func TestBOSL2Render_CylRounding(t *testing.T) {
	plain := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncyl(h=20, r=10);\n")
	rounded := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncyl(h=20, r=10, rounding=3);\n")
	_, _, pdz := extents(plain)
	_, _, rdz := extents(rounded)
	if !near(rdz, pdz, 0.5) {
		t.Errorf("rounded cyl height %v should match plain %v", rdz, pdz)
	}
	if !(rounded.Volume() < plain.Volume()) {
		t.Errorf("rounded volume %v should be < plain %v (rims rounded)", rounded.Volume(), plain.Volume())
	}
	if rounded.NumComponents() != 1 || rounded.Genus() != 0 {
		t.Errorf("rounded cyl should be a simple solid: comps=%d genus=%d", rounded.NumComponents(), rounded.Genus())
	}
}

// A single-anchor attach to a combined edge anchor (TOP+RIGHT) reorients the
// child out along that diagonal: a cylinder attached to the top-right edge of a
// 20-cube sticks out past the cube in BOTH +X and +Z (a pure-axis orientation
// would extend only one).
func TestBOSL2Render_AttachCombinedAnchor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(TOP+RIGHT) cyl(h=10, r=2);\n")
	if v := s.Volume(); !(v > 0) {
		t.Errorf("volume = %v, want > 0", v)
	}
	// The cyl meets the cube along the edge line, so the two may register as
	// separate components — that's fine; what matters is the diagonal orientation.
	_, _, _, maxX, _, maxZ := s.BoundingBox()
	if maxX < 14 || maxZ < 14 { // cube reaches 10; the cyl points out the (1,0,1) diagonal
		t.Errorf("cyl should stick out top-right: maxX=%v maxZ=%v (cube reaches 10)", maxX, maxZ)
	}
}
