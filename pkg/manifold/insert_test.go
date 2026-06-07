//go:build !js

package manifold

import (
	"math"
	"strings"
	"testing"
)

// Insert classifies the disconnected pieces of (a - b) into the outer shell
// (kept) and trapped inner plugs (discarded). The classifier must use exact
// boolean containment against b's convex hull, not an axis-aligned bounding
// box: a rotated b has a bloated AABB that wrongly swallows legitimate
// material sitting beside it.

// TestInsertKeepsPieceWithinBBoxButOutsideHull builds a base solid with a
// piece that lies inside the inserted part's axis-aligned bounding box but
// well outside its (rotated) convex hull. The bbox classifier deletes that
// piece; the hull classifier keeps it.
func TestInsertKeepsPieceWithinBBoxButOutsideHull(t *testing.T) {
	// P1: a large cube far from b — outside b's AABB, kept either way.
	p1, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	p1 = p1.Translate(40, 0, 0) // [40,50] x [0,10] x [0,10]

	// b: a thin bar centred at the origin, rotated 45° about z. Its convex
	// hull is the slim diagonal bar; its AABB is a fat ~15.6-wide square.
	b, err := CreateCube(2, 20, 8)
	if err != nil {
		t.Fatal(err)
	}
	b = b.Translate(-1, -10, -4) // centre at origin: [-1,1] x [-10,10] x [-4,4]
	b = b.Rotate(0, 0, 45)       // bar now runs along the y = -x diagonal

	// P2: a small cube parked in the *off-diagonal* corner of b's AABB —
	// inside the AABB (|5| < 7.78) but ~7 units off the bar (outside the hull).
	p2, err := CreateCube(2, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	p2 = p2.Translate(4, 4, -1) // centre ~ (5, 5, 0)

	a := p1.Union(p2) // disconnected: two islands, b touches neither

	result, err := a.Insert(b)
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	// b is disjoint from both islands, so a correct result is three separate
	// components: P1, P2, and b. The bbox classifier drops P2, leaving two.
	parts := DecomposeSolid(result)
	if len(parts) != 3 {
		t.Errorf("component count = %d, want 3 (P2 wrongly discarded by bbox test?)", len(parts))
	}

	// Cross-check by volume: P1(1000) + P2(8) + b(2*20*8=320) = 1328.
	if vol := result.Volume(); math.Abs(vol-1328) > 1.0 {
		t.Errorf("volume = %.4g, want ~1328 (missing P2's 8?)", vol)
	}
}

// TestInsertRemovesEnclosedPlug is the core feature guard: a genuine trapped
// plug — a piece of the base fully inside b's hull — must be discarded.
func TestInsertRemovesEnclosedPlug(t *testing.T) {
	a, err := CreateCube(20, 20, 20) // [0,20]^3, volume 8000
	if err != nil {
		t.Fatal(err)
	}

	// b: a square tube (box frame) standing in the centre of the cube and
	// protruding top and bottom so it cuts all the way through. Subtracting it
	// severs the cube's central [7,13]^2 column into a free-floating plug.
	outer, err := CreateCube(12, 12, 24)
	if err != nil {
		t.Fatal(err)
	}
	outer = outer.Translate(4, 4, -2) // [4,16] x [4,16] x [-2,22]
	inner, err := CreateCube(6, 6, 28)
	if err != nil {
		t.Fatal(err)
	}
	inner = inner.Translate(7, 7, -4) // [7,13] x [7,13] x [-4,24]
	b := outer.Difference(inner)      // hollow square tube

	result, err := a.Insert(b)
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	// Plug removed:  outer-shell(5120) + tube(2592) = 7712.
	// Plug kept:     + the 6*6*20 = 720 core = 8432.
	if vol := result.Volume(); math.Abs(vol-7712) > 1.0 {
		t.Errorf("volume = %.4g, want ~7712 (plug not removed?)", vol)
	}
}

// TestInsertErrorsWhenEveryPieceEnclosed covers the degenerate case the old
// code masked with a silent fallback: if *every* piece of (a - b) lies within
// b's hull, there is no outer shell to keep, so seating b would discard the
// entire base. That is not a recoverable result — Insert must report it.
func TestInsertErrorsWhenEveryPieceEnclosed(t *testing.T) {
	// b: a hollow container. Its convex hull is the solid [0,20]^3 cube.
	outer, err := CreateCube(20, 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	cavity, err := CreateCube(16, 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	cavity = cavity.Translate(2, 2, 2) // [2,18]^3
	b := outer.Difference(cavity)      // walls 2 thick, hollow [2,18]^3 cavity

	// a: two small cubes floating inside the cavity, disjoint from the walls
	// and each other. Both lie within b's hull, so both classify as plugs.
	c1, err := CreateCube(2, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	c1 = c1.Translate(5, 5, 5)
	c2, err := CreateCube(2, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	c2 = c2.Translate(11, 11, 11)
	a := c1.Union(c2)

	result, err := a.Insert(b)
	if err == nil {
		t.Fatalf("Insert succeeded, want error (every piece enclosed); got result %v", result)
	}
	if !strings.Contains(err.Error(), "Insert") {
		t.Errorf("error %q should identify the Insert operation", err.Error())
	}
}
