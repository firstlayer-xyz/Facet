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
	solids, err := result.StaticSolids(context.Background(), 0)
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

// attach(TOP) (1-arg) CENTERS the child on the parent's top face, matching BOSL2:
// a 12-tall cube (z -6..6) with a 16-tall cyl centered on z=6 puts the cyl at
// z -2..14, so the whole spans 20 in Z, one connected component. (Sitting the cyl
// fully on top is the 2-arg attach(TOP, BOTTOM).)
func TestBOSL2Render_AttachStacksOnTop(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([30, 30, 12]) attach(TOP) cyl(h=16, r=4);\n")
	if n := s.NumComponents(); n != 1 {
		t.Errorf("components = %d, want 1 (child overlaps parent)", n)
	}
	_, _, _, _, _, maxZ := s.BoundingBox()
	if !near(maxZ, 14, 1.0) { // cyl centered on the top face z=6, reaches z=14
		t.Errorf("max Z = %v, want ~14 (cyl centered on the top face, BOSL2 1-arg attach)", maxZ)
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

// attach(RIGHT) reorients the child so its axis points along +X AND centers it on
// the right face (BOSL2 1-arg attach): a cyl (h=10) centered on the right face of
// a 20-cube (x=10) reaches x=15, so the X span is ~25. A wrong reorientation would
// leave it pointing up (z), not out (x).
func TestBOSL2Render_AttachReorientsToRight(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(RIGHT) cyl(h=10, r=4);\n")
	dx, _, dz := extents(s)
	if !near(dx, 25, 1.0) { // cube x -10..10, cyl centered on x=10 reaches x=15
		t.Errorf("x-extent = %v, want ~25 (cyl centered on the right face, points +X)", dx)
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

// orient= reorients a primitive's +Z (UP) to the given direction. orient=RIGHT
// points UP along +X, so a 10×4×2 box (long axis X, short axis Z) swaps those:
// short axis (2) ends up on X, long axis (10) on Z.
func TestBOSL2Render_CuboidOrient(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([10, 4, 2], orient=RIGHT);\n")
	dx, dy, dz := extents(s)
	if !near(dx, 2, 0.5) {
		t.Errorf("x-extent = %v, want ~2 (was the z dimension)", dx)
	}
	if !near(dy, 4, 0.5) {
		t.Errorf("y-extent = %v, want ~4 (unchanged)", dy)
	}
	if !near(dz, 10, 0.5) {
		t.Errorf("z-extent = %v, want ~10 (was the x dimension)", dz)
	}
}

// attach(RIGHT, BOTTOM) is a NON-opposite two-anchor attach: the cylinder's
// BOTTOM mates onto the cube's RIGHT face, rotating the cyl so its axis ends up
// along +X. So it reaches out to ~x=20 (cube right face at 10 + cyl length 10) —
// a wrong/absent rotation would leave it pointing up (z).
func TestBOSL2Render_AttachNonOpposite(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(RIGHT, BOTTOM) cyl(h=10, r=2);\n")
	if v := s.Volume(); !(v > 0) {
		t.Errorf("volume = %v, want > 0", v)
	}
	_, _, _, maxX, _, maxZ := s.BoundingBox()
	if !near(maxX, 20, 1.0) {
		t.Errorf("maxX = %v, want ~20 (cyl axis along +X out the right face)", maxX)
	}
	if !near(maxZ, 10, 1.0) { // cube only; the cyl lies along X, not Z
		t.Errorf("maxZ = %v, want ~10 (cube top; cyl doesn't add height)", maxZ)
	}
}

// A trapezoid extruded to a solid has the expected size and area: h=10, w1=20,
// w2=10 -> trapezoid area (20+10)/2*10 = 150; extruded 5 -> volume 750. Bounds
// span w1=20 in X, h=10 in Y.
func TestBOSL2Render_Trapezoid(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=5) trapezoid(h=10, w1=20, w2=10);\n")
	if v := s.Volume(); !near(v, 750, 1.0) {
		t.Errorf("volume = %v, want ~750 (area 150 x height 5)", v)
	}
	dx, dy, dz := extents(s)
	if !near(dx, 20, 0.5) {
		t.Errorf("x-extent = %v, want ~20 (w1)", dx)
	}
	if !near(dy, 10, 0.5) {
		t.Errorf("y-extent = %v, want ~10 (h)", dy)
	}
	if !near(dz, 5, 0.5) {
		t.Errorf("z-extent = %v, want ~5", dz)
	}
}

// cuboid(chamfer=C) bevels every edge: same bounding box as the plain box, less
// volume (corners/edges cut off), still a simple solid.
func TestBOSL2Render_CuboidChamfer(t *testing.T) {
	plain := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20]);\n")
	cham := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 20], chamfer=3);\n")
	pdx, pdy, pdz := extents(plain)
	cdx, cdy, cdz := extents(cham)
	if !near(cdx, pdx, 0.5) || !near(cdy, pdy, 0.5) || !near(cdz, pdz, 0.5) {
		t.Errorf("chamfered bbox %vx%vx%v should match plain %vx%vx%v", cdx, cdy, cdz, pdx, pdy, pdz)
	}
	if !(cham.Volume() < plain.Volume()) {
		t.Errorf("chamfered volume %v should be < plain %v (edges beveled)", cham.Volume(), plain.Volume())
	}
	if c, g := cham.NumComponents(), cham.Genus(); c != 1 || g != 0 {
		t.Errorf("chamfered cuboid: comps=%d genus=%d, want 1/0", c, g)
	}
}

// top_half() keeps the +Z half of a centered cube: a 10-cube (z:-5..5) becomes
// z:0..5 with half the volume.
func TestBOSL2Render_TopHalf(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ntop_half() cuboid([10, 10, 10]);\n")
	if v := s.Volume(); !near(v, 500, 1.0) {
		t.Errorf("volume = %v, want ~500 (half of 1000)", v)
	}
	_, _, minZ, _, _, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.2) || !near(maxZ, 5, 0.2) {
		t.Errorf("z range [%v, %v], want [0, 5] (top half)", minZ, maxZ)
	}
}

// An ellipse (rx=10, ry=5) extruded is sized rx x ry per axis with area ~pi*rx*ry.
func TestBOSL2Render_Ellipse(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=4) ellipse(r=[10, 5]);\n")
	dx, dy, dz := extents(s)
	if !near(dx, 20, 0.6) {
		t.Errorf("x-extent = %v, want ~20 (2*rx)", dx)
	}
	if !near(dy, 10, 0.6) {
		t.Errorf("y-extent = %v, want ~10 (2*ry)", dy)
	}
	if !near(dz, 4, 0.2) {
		t.Errorf("z-extent = %v, want ~4", dz)
	}
	if v := s.Volume(); v < 600 || v > 632 { // pi*10*5*4 ~ 628, less with faceting
		t.Errorf("volume = %v, want ~628 (pi*rx*ry*h)", v)
	}
}

// xflip() reflects an off-center cube to the opposite side: a 4-cube centered at
// x=10 (x:8..12) lands at x:-12..-8 with its volume unchanged.
func TestBOSL2Render_Xflip(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nxflip() right(10) cuboid([4, 4, 4]);\n")
	if v := s.Volume(); !near(v, 64, 0.5) {
		t.Errorf("volume = %v, want 64 (4^3, mirror preserves volume)", v)
	}
	minX, _, _, maxX, _, _ := s.BoundingBox()
	if !near(minX, -12, 0.1) || !near(maxX, -8, 0.1) {
		t.Errorf("x range [%v, %v], want [-12, -8] (mirrored from [8, 12])", minX, maxX)
	}
}

// xflip(x=d) mirrors about the plane x=d: a 2-cube at x:10..12 reflected about
// x=10 lands at x:8..10.
func TestBOSL2Render_FlipOffset(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nxflip(x=10) right(11) cuboid([2, 2, 2]);\n")
	minX, _, _, maxX, _, _ := s.BoundingBox()
	if !near(minX, 8, 0.1) || !near(maxX, 10, 0.1) {
		t.Errorf("x range [%v, %v], want [8, 10] (mirrored about x=10)", minX, maxX)
	}
}

// recolor is cosmetic: it must pass the geometry through unchanged (and render
// without error). A recolored 10-cube is still a 1000-volume cube.
func TestBOSL2Render_Recolor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nrecolor(\"blue\") cuboid([10, 10, 10]);\n")
	if v := s.Volume(); !near(v, 1000, 0.5) {
		t.Errorf("volume = %v, want 1000 (recolor preserves geometry)", v)
	}
}

// xdistribute(10) of three size-2 cubes makes three separate components spanning
// x:-11..11, total volume 3*8=24.
func TestBOSL2Render_Distribute(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nxdistribute(10) { cuboid(2); cuboid(2); cuboid(2); }\n")
	if c := s.NumComponents(); c != 3 {
		t.Errorf("components = %d, want 3 (separated cubes)", c)
	}
	if v := s.Volume(); !near(v, 24, 0.5) {
		t.Errorf("volume = %v, want 24 (3 * 2^3)", v)
	}
	dx, _, _ := extents(s)
	if !near(dx, 22, 0.1) {
		t.Errorf("x-extent = %v, want 22 (-11..11)", dx)
	}
}

// cuboid(anchor=BOTTOM) puts the box on the plate: a [10,20,30] box spans z:0..30,
// still centered in x (-5..5) and y (-10..10), volume 6000.
func TestBOSL2Render_CuboidAnchorBottom(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([10, 20, 30], anchor=BOTTOM);\n")
	if v := s.Volume(); !near(v, 6000, 1.0) {
		t.Errorf("volume = %v, want 6000", v)
	}
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.1) || !near(maxZ, 30, 0.1) {
		t.Errorf("z range [%v, %v], want [0, 30] (BOTTOM on plate)", minZ, maxZ)
	}
	if !near(minX, -5, 0.1) || !near(maxX, 5, 0.1) || !near(minY, -10, 0.1) || !near(maxY, 10, 0.1) {
		t.Errorf("xy box [%v,%v]-[%v,%v], want centered [-5,-10]-[5,10]", minX, minY, maxX, maxY)
	}
}

// cyl(anchor=BOTTOM) sits the cylinder on the plate: a h=10 r=4 cyl spans z:0..10,
// centered in x/y (-4..4).
func TestBOSL2Render_CylAnchorBottom(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncyl(h=10, r=4, anchor=BOTTOM);\n")
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.1) || !near(maxZ, 10, 0.1) {
		t.Errorf("z range [%v, %v], want [0, 10] (BOTTOM on plate)", minZ, maxZ)
	}
	if !near(minX, -4, 0.1) || !near(maxX, 4, 0.1) {
		t.Errorf("x range [%v, %v], want [-4, 4] (still centered in x)", minX, maxX)
	}
}

// spheroid(anchor=BOTTOM) rests the sphere on the plate: an r=5 sphere spans
// z:0..10, centered in x (-5..5).
func TestBOSL2Render_SpheroidAnchorBottom(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nspheroid(r=5, anchor=BOTTOM);\n")
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.15) || !near(maxZ, 10, 0.15) {
		t.Errorf("z range [%v, %v], want [0, 10] (BOTTOM on plate)", minZ, maxZ)
	}
	if !near(minX, -5, 0.15) || !near(maxX, 5, 0.15) {
		t.Errorf("x range [%v, %v], want [-5, 5] (still centered)", minX, maxX)
	}
}

// spin=90 about Z swaps a non-square box's footprint: [20,10,4] becomes 10 wide
// (x) and 20 deep (y), volume unchanged.
func TestBOSL2Render_Spin(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 10, 4], spin=90);\n")
	dx, dy, dz := extents(s)
	if !near(dx, 10, 0.1) || !near(dy, 20, 0.1) {
		t.Errorf("footprint = %v x %v, want 10 x 20 (spun 90)", dx, dy)
	}
	if !near(dz, 4, 0.1) {
		t.Errorf("z-extent = %v, want 4", dz)
	}
	if v := s.Volume(); !near(v, 800, 0.5) {
		t.Errorf("volume = %v, want 800 (spin preserves volume)", v)
	}
}

// align(TOP) stacks the child on the parent's top face: a [4,4,8] child on a
// [20,20,10] parent (z:-5..5) sits at z:5..13, so the union reaches z:13 with
// volume 4000+128=4128 (they meet on a shared face, no overlap).
func TestBOSL2Render_AlignTop(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) align(TOP) cuboid([4, 4, 8]);\n")
	_, _, minZ, _, _, maxZ := s.BoundingBox()
	if !near(minZ, -5, 0.1) || !near(maxZ, 13, 0.1) {
		t.Errorf("z range [%v, %v], want [-5, 13] (child stacked on top)", minZ, maxZ)
	}
	if v := s.Volume(); !near(v, 4128, 1.0) {
		t.Errorf("volume = %v, want 4128 (4000 + 128)", v)
	}
}

// align(inside=true) under diff() carves a blind pocket from the top face: a
// [4,4,4] cutter seated inside removes 64 from the [20,20,10] block (4000->3936),
// leaving the bounding box unchanged.
func TestBOSL2Render_AlignInsideDiff(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ndiff() cuboid([20, 20, 10]) { align(TOP, inside=true) cuboid([4, 4, 4]); }\n")
	if v := s.Volume(); !near(v, 3936, 1.0) {
		t.Errorf("volume = %v, want 3936 (4000 - 64 pocket)", v)
	}
	_, _, minZ, _, _, maxZ := s.BoundingBox()
	if !near(minZ, -5, 0.1) || !near(maxZ, 5, 0.1) {
		t.Errorf("z range [%v, %v], want [-5, 5] (pocket is internal)", minZ, maxZ)
	}
}

// rect(anchor=RIGHT) puts the rectangle's right edge on the origin: a [20,10]
// rect extruded spans x:-20..0, y:-5..5, volume 20*10*4=800.
func TestBOSL2Render_RectAnchor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=4) rect([20, 10], anchor=RIGHT);\n")
	minX, minY, _, maxX, maxY, _ := s.BoundingBox()
	if !near(minX, -20, 0.1) || !near(maxX, 0, 0.1) {
		t.Errorf("x range [%v, %v], want [-20, 0] (right edge on origin)", minX, maxX)
	}
	if !near(minY, -5, 0.1) || !near(maxY, 5, 0.1) {
		t.Errorf("y range [%v, %v], want [-5, 5] (still centered in y)", minY, maxY)
	}
	if v := s.Volume(); !near(v, 800, 0.5) {
		t.Errorf("volume = %v, want 800", v)
	}
}

// tube(anchor=BOTTOM) sits the hollow cylinder on the plate: h=10 or=8 ir=4 spans
// z:0..10, x:-8..8, with a hollow core (volume = pi*(8^2-4^2)*10 ~ 1508).
func TestBOSL2Render_TubeAnchor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ntube(h=10, or=8, ir=4, anchor=BOTTOM);\n")
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.1) || !near(maxZ, 10, 0.1) {
		t.Errorf("z range [%v, %v], want [0, 10] (BOTTOM on plate)", minZ, maxZ)
	}
	if !near(minX, -8, 0.1) || !near(maxX, 8, 0.1) {
		t.Errorf("x range [%v, %v], want [-8, 8]", minX, maxX)
	}
	if v := s.Volume(); v < 1400 || v > 1520 { // pi*(64-16)*10 ~ 1508, less with faceting
		t.Errorf("volume = %v, want ~1508 (hollow)", v)
	}
}

// prismoid(anchor=BOTTOM) sits the tapered box on the plate: a 20->10 prismoid of
// height 15 spans z:0..15, base centered (x:-10..10 from the 20-wide bottom).
func TestBOSL2Render_PrismoidAnchorBottom(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15, anchor=BOTTOM);\n")
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(minZ, 0, 0.1) || !near(maxZ, 15, 0.1) {
		t.Errorf("z range [%v, %v], want [0, 15] (BOTTOM on plate)", minZ, maxZ)
	}
	if !near(minX, -10, 0.1) || !near(maxX, 10, 0.1) {
		t.Errorf("x range [%v, %v], want [-10, 10] (20-wide base, centered)", minX, maxX)
	}
}

// ellipse(anchor=RIGHT) puts the right vertex on the origin: an rx=10,ry=5
// ellipse extruded spans x:-20..0, y:-5..5.
func TestBOSL2Render_EllipseAnchor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=3) ellipse(r=[10, 5], anchor=RIGHT);\n")
	minX, minY, _, maxX, maxY, _ := s.BoundingBox()
	if !near(minX, -20, 0.6) || !near(maxX, 0, 0.6) {
		t.Errorf("x range [%v, %v], want [-20, 0] (right edge on origin)", minX, maxX)
	}
	if !near(minY, -5, 0.6) || !near(maxY, 5, 0.6) {
		t.Errorf("y range [%v, %v], want [-5, 5] (still centered in y)", minY, maxY)
	}
}

// trapezoid(anchor=FWD) puts the front edge on the origin: h=10 trapezoid spans
// y:0..10, x within +/- max(w1,w2)/2.
func TestBOSL2Render_TrapezoidAnchor(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=3) trapezoid(h=10, w1=20, w2=8, anchor=FWD);\n")
	_, minY, _, _, maxY, _ := s.BoundingBox()
	if !near(minY, 0, 0.1) || !near(maxY, 10, 0.1) {
		t.Errorf("y range [%v, %v], want [0, 10] (front edge on origin)", minY, maxY)
	}
}

// xrot(90, cp=[0,0,10]) turns a 2-cube about the point (0,0,10): the cube's
// center (0,0,0) is 10 below the pivot, so +90deg about X swings it to (0,10,10).
// The cube lands at y:9..11, z:9..11.
func TestBOSL2Render_AxisRotPivot(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nxrot(90, cp=[0, 0, 10]) cuboid([2, 2, 2]);\n")
	_, minY, minZ, _, maxY, maxZ := s.BoundingBox()
	if !near(minY, 9, 0.1) || !near(maxY, 11, 0.1) {
		t.Errorf("y range [%v, %v], want [9, 11] (swung up by the pivot)", minY, maxY)
	}
	if !near(minZ, 9, 0.1) || !near(maxZ, 11, 0.1) {
		t.Errorf("z range [%v, %v], want [9, 11]", minZ, maxZ)
	}
	if v := s.Volume(); !near(v, 8, 0.2) {
		t.Errorf("volume = %v, want 8 (rotation preserves volume)", v)
	}
}

// prismoid(anchor=RIGHT) anchors on the slanted face center, NOT the bounding box:
// BOSL2 places RIGHT at x=lerp(size1.x/2, size2.x/2, 0.5)=(10+5)/2=7.5 for a
// 20->10 taper, so the shape (base x:-10..10) shifts -7.5 to x:-17.5..2.5.
func TestBOSL2Render_PrismoidAnchorRight(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15, anchor=RIGHT);\n")
	minX, _, _, maxX, _, _ := s.BoundingBox()
	if !near(minX, -17.5, 0.1) || !near(maxX, 2.5, 0.1) {
		t.Errorf("x range [%v, %v], want [-17.5, 2.5] (slant-face center at x=7.5 on origin)", minX, maxX)
	}
}

// prismoid(anchor=RIGHT+TOP) samples the TOP rectangle (size2): x=size2.x/2=5,
// z=+h/2=7.5. So the shape shifts (-5,_,-7.5): x:-15..5, z:-15..0.
func TestBOSL2Render_PrismoidAnchorTop(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15, anchor=RIGHT+TOP);\n")
	_, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(maxX, 5, 0.1) {
		t.Errorf("maxX = %v, want 5 (top-rectangle right edge at x=5 on origin)", maxX)
	}
	if !near(minZ, -15, 0.1) || !near(maxZ, 0, 0.1) {
		t.Errorf("z range [%v, %v], want [-15, 0] (top on origin)", minZ, maxZ)
	}
}

// trapezoid(anchor=RIGHT) anchors on the slant-face center: BOSL2 places RIGHT at
// x=lerp(w1/2, w2/2, 0.5)=(10+4)/2=7 for w1=20,w2=8, so the shape (base x:-10..10)
// shifts -7 to x:-17..3.
func TestBOSL2Render_TrapezoidAnchorRight(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=3) trapezoid(h=10, w1=20, w2=8, anchor=RIGHT);\n")
	minX, _, _, maxX, _, _ := s.BoundingBox()
	if !near(minX, -17, 0.1) || !near(maxX, 3, 0.1) {
		t.Errorf("x range [%v, %v], want [-17, 3] (slant-face center at x=7 on origin)", minX, maxX)
	}
}

// A combined edge anchor (TOP+RIGHT) on a tube: the radial part [1,0] has
// magnitude 1, so X is the full radius (8); Z is half-height (5). Shifts to
// x:-16..0, z:-10..0.
func TestBOSL2Render_TubeAnchorCorner(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ntube(h=10, or=8, ir=4, anchor=TOP+RIGHT);\n")
	minX, _, minZ, maxX, _, maxZ := s.BoundingBox()
	if !near(minX, -16, 0.1) || !near(maxX, 0, 0.1) {
		t.Errorf("x range [%v, %v], want [-16, 0] (right edge on origin)", minX, maxX)
	}
	if !near(minZ, -10, 0.1) || !near(maxZ, 0, 0.1) {
		t.Errorf("z range [%v, %v], want [-10, 0] (top on origin)", minZ, maxZ)
	}
}

// cyl(anchor=RIGHT+BACK) is a radial diagonal: BOSL2 lands it on the cylinder
// wall at r·unit([1,1]) = (10/sqrt2, 10/sqrt2) ~ (7.07, 7.07), NOT the bbox corner
// (10,10). The shape (x,y:-10..10) shifts -7.07 to maxX=maxY ~ 2.93.
func TestBOSL2Render_CylAnchorRadial(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\ncyl(r=10, h=20, anchor=RIGHT+BACK);\n")
	_, _, _, maxX, maxY, _ := s.BoundingBox()
	if !near(maxX, 2.93, 0.2) || !near(maxY, 2.93, 0.2) {
		t.Errorf("maxX,maxY = %v,%v, want ~2.93 (wall point, not bbox corner at 0)", maxX, maxY)
	}
}

// spheroid(anchor=RIGHT+UP) lands on the sphere surface at r·unit([1,0,1]) =
// (10/sqrt2, 0, 10/sqrt2) ~ (7.07, 0, 7.07), NOT the bbox corner. The sphere
// (x,z:-10..10) shifts -7.07 to maxX=maxZ ~ 2.93.
func TestBOSL2Render_SpheroidAnchorDiagonal(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nspheroid(r=10, anchor=RIGHT+UP);\n")
	_, _, _, maxX, _, maxZ := s.BoundingBox()
	if !near(maxX, 2.93, 0.3) || !near(maxZ, 2.93, 0.3) {
		t.Errorf("maxX,maxZ = %v,%v, want ~2.93 (surface point, not bbox corner at 0)", maxX, maxZ)
	}
}

// ellipse(anchor=RIGHT+BACK) lands the diagonal on the perimeter where the ray
// along [1,1] meets the ellipse: for rx=10,ry=5 that is (rx·ry/sqrt(rx^2+ry^2)) in
// each axis = (4.472, 4.472), per BOSL2's solve_ellipse (NOT the scaled-circle
// point (7.07,3.54), and NOT the bbox corner). The shape shifts -4.472, so
// maxX = 10-4.472 = 5.53 and maxY = 5-4.472 = 0.53.
func TestBOSL2Render_EllipseAnchorDiagonal(t *testing.T) {
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nlinear_extrude(height=3) ellipse(r=[10, 5], anchor=RIGHT+BACK);\n")
	_, _, _, maxX, maxY, _ := s.BoundingBox()
	if !near(maxX, 5.53, 0.3) {
		t.Errorf("maxX = %v, want ~5.53 (ray-perimeter anchor at x=4.47)", maxX)
	}
	if !near(maxY, 0.53, 0.3) {
		t.Errorf("maxY = %v, want ~0.53 (ray-perimeter anchor at y=4.47)", maxY)
	}
}

// Anchor-placement regression guards with values verified against real
// OpenSCAD+BOSL2 (see TestBOSL2GroundTruth). These run in CI where openscad is
// absent. The bbox CENTER is the anchor signal (faceting cancels in the midpoint).
func TestBOSL2Render_CoreAnchorPlacement(t *testing.T) {
	center := func(lo, hi float64) float64 { return (lo + hi) / 2 }
	// sphere diagonal anchor lands on the SURFACE: r·unit(1,1,0) = (7.07,7.07,0),
	// so the sphere center sits at (-7.07,-7.07,0) — NOT the bbox corner (-10,-10).
	s := renderBosl2Solid(t, "include <BOSL2/std.scad>\nsphere(r=10, $fn=64, anchor=RIGHT+BACK);\n")
	minX, minY, _, maxX, maxY, _ := s.BoundingBox()
	if !near(center(minX, maxX), -7.071, 0.1) || !near(center(minY, maxY), -7.071, 0.1) {
		t.Errorf("sphere diag center = (%.3f,%.3f), want (-7.07,-7.07) [surface, not bbox]", center(minX, maxX), center(minY, maxY))
	}
	// cylinder diagonal anchor is radial on the rim (7.07,7.07), z centered.
	s = renderBosl2Solid(t, "include <BOSL2/std.scad>\ncylinder(h=20, r=10, $fn=64, anchor=RIGHT+BACK);\n")
	minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
	if !near(center(minX, maxX), -7.071, 0.1) || !near(center(minY, maxY), -7.071, 0.1) {
		t.Errorf("cyl diag center = (%.3f,%.3f), want (-7.07,-7.07) [rim, not bbox]", center(minX, maxX), center(minY, maxY))
	}
	if !near(minZ, -10, 0.05) || !near(maxZ, 10, 0.05) {
		t.Errorf("cyl diag z = [%.3f,%.3f], want [-10,10] (z centered)", minZ, maxZ)
	}
	// cube anchor=BOTTOM sits the box on the plate.
	s = renderBosl2Solid(t, "include <BOSL2/std.scad>\ncube([10,20,30], anchor=BOTTOM);\n")
	_, _, minZ, _, _, maxZ = s.BoundingBox()
	if !near(minZ, 0, 0.02) || !near(maxZ, 30, 0.02) {
		t.Errorf("cube anchor=BOTTOM z = [%.3f,%.3f], want [0,30]", minZ, maxZ)
	}
}
