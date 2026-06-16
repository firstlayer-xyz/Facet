package manifold

import (
	"fmt"
	"math"
)

// Shared argument validation, used by both the native (cgo) and wasm builds so
// the two can never drift. Pure Go — no cgo, no syscall/js — so it lives in a
// no-build-tag file and both Create*/transform paths call the same checks.

// requireSolids panics if any operand is a nil *Solid. A nil reaching a kernel
// op (Union/Difference/...) is always a caller bug (a forgotten error check, or
// a slice with a nil entry); a clear panic at the entry point beats a nil-deref
// deep inside the cgo/js wrapper or, worse, a SIGSEGV in C++.
func requireSolids(op string, solids ...*Solid) {
	for i, s := range solids {
		if s == nil {
			panic(fmt.Sprintf("manifold.%s: solid argument %d is nil", op, i))
		}
	}
}

// requireSketches is requireSolids for *Sketch operands — a nil Sketch reaching
// a 2D op is a caller bug; panic with a clear message rather than letting the
// nil handle fault inside the wrapper.
func requireSketches(op string, sketches ...*Sketch) {
	for i, sk := range sketches {
		if sk == nil {
			panic(fmt.Sprintf("manifold.%s: sketch argument %d is nil", op, i))
		}
	}
}

// minNormalLengthSq is the smallest squared length a mirror normal may have and
// still normalize stably; below it the plane is degenerate. It catches normals
// that pass an "any component is non-zero" test but normalize to numerical noise
// on the C side.
const minNormalLengthSq = 1e-300

// validateScaleFactor rejects zero, NaN, and infinite scale factors. Zero
// collapses a dimension; NaN/Inf invalidate the entire transform matrix and
// would otherwise surface as a confusing kernel-side error much later.
func validateScaleFactor(name string, v float64) error {
	if v == 0 {
		return fmt.Errorf("Scale: factor %s must be non-zero", name)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("Scale: factor %s must be finite, got %g", name, v)
	}
	return nil
}

// validateMirrorNormal3 rejects a degenerate 3D mirror normal (too short to
// normalize stably, or NaN).
func validateMirrorNormal3(nx, ny, nz float64) error {
	if lenSq := nx*nx + ny*ny + nz*nz; lenSq < minNormalLengthSq || math.IsNaN(lenSq) {
		return fmt.Errorf("Mirror: normal vector is degenerate (got %g, %g, %g)", nx, ny, nz)
	}
	return nil
}

// validateMirrorNormal2 rejects a degenerate 2D mirror axis.
func validateMirrorNormal2(ax, ay float64) error {
	if lenSq := ax*ax + ay*ay; lenSq < minNormalLengthSq || math.IsNaN(lenSq) {
		return fmt.Errorf("Mirror: axis vector is degenerate (got %g, %g)", ax, ay)
	}
	return nil
}

func validateCubeDims(x, y, z float64) error {
	if x <= 0 || y <= 0 || z <= 0 {
		return fmt.Errorf("Cube: all dimensions must be positive, got (%.4g, %.4g, %.4g)", x, y, z)
	}
	return nil
}

func validateSphereRadius(radius float64) error {
	if radius <= 0 {
		return fmt.Errorf("Sphere: radius must be positive, got %.4g", radius)
	}
	return nil
}

func validateCylinder(height, radiusLow, radiusHigh float64) error {
	if height <= 0 {
		return fmt.Errorf("Cylinder: height must be positive, got %.4g", height)
	}
	if radiusLow < 0 || radiusHigh < 0 {
		return fmt.Errorf("Cylinder: radii must be non-negative, got (%.4g, %.4g)", radiusLow, radiusHigh)
	}
	if radiusLow == 0 && radiusHigh == 0 {
		return fmt.Errorf("Cylinder: at least one radius must be positive")
	}
	return nil
}

func validateSquareDims(x, y float64) error {
	if x <= 0 || y <= 0 {
		return fmt.Errorf("Square: dimensions must be positive, got (%.4g, %.4g)", x, y)
	}
	return nil
}

func validateCircleRadius(radius float64) error {
	if radius <= 0 {
		return fmt.Errorf("Circle: radius must be positive, got %.4g", radius)
	}
	return nil
}

func validatePolygonRings(outer []Point2D, holes [][]Point2D) error {
	if len(outer) < 3 {
		return fmt.Errorf("Polygon outline requires at least 3 points, got %d", len(outer))
	}
	for i, h := range holes {
		if len(h) < 3 {
			return fmt.Errorf("Polygon hole %d requires at least 3 points, got %d", i, len(h))
		}
	}
	return nil
}

// validateExtrudeHeight rejects a non-positive extrude height. The kernel
// returns an empty manifold for height <= 0 rather than erroring, so we catch it
// here for an actionable message.
func validateExtrudeHeight(height float64) error {
	if height <= 0 {
		return fmt.Errorf("Extrude: height must be positive, got %.4g", height)
	}
	return nil
}

func validateSweepPath(n int) error {
	if n < 2 {
		return fmt.Errorf("Sweep: path must have at least 2 points, got %d", n)
	}
	return nil
}

func validateLoft(nSketches, nHeights int) error {
	if nSketches < 2 {
		return fmt.Errorf("Loft: need at least 2 sketches, got %d", nSketches)
	}
	if nSketches != nHeights {
		return fmt.Errorf("Loft: sketches (%d) and heights (%d) must be same length", nSketches, nHeights)
	}
	return nil
}
