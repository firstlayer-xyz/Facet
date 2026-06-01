//go:build !js

package manifold

import (
	"math"
	"testing"
)

// TestRevolveAxis empirically verifies which axis Manifold::Revolve
// rotates around. The Facet docs (std.fct, drawing-guide, users-guide)
// say "around the Y axis", but the manifold C++ math maps the 2D
// sketch's y-coordinate to the world Z. This test settles it.
//
// Sketch: a thin 1mm × 4mm rectangle, translated to x ∈ [5, 6].
// After a full 360° revolve, the result is a hollow torus-like ring.
// The axis of revolution is whichever world axis the ring's "height"
// runs along — call it the AXIAL axis. The other two axes form the
// RADIAL plane and have equal half-extents (the outer radius).
func TestRevolveAxis(t *testing.T) {
	const (
		sketchW    = 1.0
		sketchH    = 4.0
		translateX = 5.0
		segments   = 64
		degrees    = 360.0
	)

	sk := CreateSquare(sketchW, sketchH).Translate(translateX, 0)
	sol, err := sk.Revolve(segments, degrees)
	if err != nil {
		t.Fatalf("Revolve: %v", err)
	}

	minX, minY, minZ, maxX, maxY, maxZ := sol.BoundingBox()
	extentX := maxX - minX
	extentY := maxY - minY
	extentZ := maxZ - minZ

	t.Logf("bounding box: X[%.3f, %.3f] (ext %.3f)", minX, maxX, extentX)
	t.Logf("              Y[%.3f, %.3f] (ext %.3f)", minY, maxY, extentY)
	t.Logf("              Z[%.3f, %.3f] (ext %.3f)", minZ, maxZ, extentZ)

	// The radial axes both span 2 × outerRadius = 2 × (translateX + sketchW) = 12.
	// The axial axis spans the sketch height = 4.
	const expectedRadialExtent = 2.0 * (translateX + sketchW)
	const expectedAxialExtent = sketchH

	close := func(a, b, tol float64) bool { return math.Abs(a-b) < tol }
	const tol = 0.5 // segments=64 leaves a small polygonal undershoot

	axialAxis := ""
	switch {
	case close(extentX, expectedAxialExtent, tol) && close(extentY, expectedRadialExtent, tol) && close(extentZ, expectedRadialExtent, tol):
		axialAxis = "X"
	case close(extentY, expectedAxialExtent, tol) && close(extentX, expectedRadialExtent, tol) && close(extentZ, expectedRadialExtent, tol):
		axialAxis = "Y"
	case close(extentZ, expectedAxialExtent, tol) && close(extentX, expectedRadialExtent, tol) && close(extentY, expectedRadialExtent, tol):
		axialAxis = "Z"
	default:
		t.Fatalf("bounding box doesn't match any single-axis revolve pattern (expected one axis ≈ %.1f, other two ≈ %.1f)",
			expectedAxialExtent, expectedRadialExtent)
	}

	t.Logf("Revolve axis (empirical): %s", axialAxis)
}
