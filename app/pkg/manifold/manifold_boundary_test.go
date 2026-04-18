package manifold

import (
	"strings"
	"testing"
)

// ----------------------------------------------------------------------------
// Boundary validation — empty/degenerate inputs must return errors, not panic.
// ----------------------------------------------------------------------------

func TestComposeSolidsEmpty(t *testing.T) {
	_, err := ComposeSolids(nil)
	if err == nil {
		t.Fatal("expected error for empty ComposeSolids, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestBatchHullEmpty(t *testing.T) {
	_, err := BatchHull(nil)
	if err == nil {
		t.Fatal("expected error for empty BatchHull, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestSketchBatchHullEmpty(t *testing.T) {
	_, err := SketchBatchHull(nil)
	if err == nil {
		t.Fatal("expected error for empty SketchBatchHull, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestLoftEmpty(t *testing.T) {
	_, err := Loft(nil, nil)
	if err == nil {
		t.Fatal("expected error for empty Loft, got nil")
	}
}

func TestLoftLengthMismatch(t *testing.T) {
	sq := CreateSquare(10, 10)
	_, err := Loft([]*Sketch{sq, sq}, []float64{0})
	if err == nil {
		t.Fatal("expected error for mismatched Loft inputs, got nil")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Errorf("error should mention length mismatch: %v", err)
	}
}

func TestLoftTooFewProfiles(t *testing.T) {
	sq := CreateSquare(10, 10)
	_, err := Loft([]*Sketch{sq}, []float64{0})
	if err == nil {
		t.Fatal("expected error for Loft with fewer than 2 profiles, got nil")
	}
}

func TestHullPointsEmpty(t *testing.T) {
	_, err := HullPoints(nil)
	if err == nil {
		t.Fatal("expected error for empty HullPoints, got nil")
	}
}

func TestHullPointsTooFew(t *testing.T) {
	pts := []Point3D{
		{X: 0, Y: 0, Z: 0},
		{X: 10, Y: 0, Z: 0},
		{X: 0, Y: 10, Z: 0},
	}
	_, err := HullPoints(pts)
	if err == nil {
		t.Fatal("expected error for HullPoints with fewer than 4 points, got nil")
	}
	if !strings.Contains(err.Error(), "4") {
		t.Errorf("error should mention 4-point minimum: %v", err)
	}
}

func TestHullPointsMinimal(t *testing.T) {
	// Four non-coplanar points — the minimum for a valid 3D hull.
	pts := []Point3D{
		{X: 0, Y: 0, Z: 0},
		{X: 10, Y: 0, Z: 0},
		{X: 0, Y: 10, Z: 0},
		{X: 0, Y: 0, Z: 10},
	}
	s, err := HullPoints(pts)
	if err != nil {
		t.Fatalf("HullPoints with 4 non-coplanar points: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil Solid")
	}
}

func TestSolidMirrorZeroNormal(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cube.Mirror(0, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for Mirror with zero-length normal, got nil")
	}
	if !strings.Contains(err.Error(), "zero") {
		t.Errorf("error should mention zero: %v", err)
	}
}

func TestSketchMirrorZeroAxis(t *testing.T) {
	sq := CreateSquare(10, 10)
	_, err := sq.Mirror(0, 0, 0)
	if err == nil {
		t.Fatal("expected error for Sketch.Mirror with zero-length axis, got nil")
	}
	if !strings.Contains(err.Error(), "zero") {
		t.Errorf("error should mention zero: %v", err)
	}
}

func TestCreateCylinderNegativeHeight(t *testing.T) {
	_, err := CreateCylinder(-10, 5, 5, 32)
	if err == nil {
		t.Fatal("expected error for CreateCylinder with negative height, got nil")
	}
	if !strings.Contains(err.Error(), "positive") && !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("error should mention positive/non-negative: %v", err)
	}
}

// Scale with a zero factor would collapse a spatial dimension, producing
// a degenerate manifold.  Negative factors are legitimate (mirror-along-axis)
// and must still succeed.

func TestSolidScaleZeroFactorRejected(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	cases := [][3]float64{
		{0, 1, 1},
		{1, 0, 1},
		{1, 1, 0},
	}
	for _, c := range cases {
		_, err := cube.Scale(c[0], c[1], c[2], 0, 0, 0)
		if err == nil {
			t.Errorf("Scale(%v): expected error, got nil", c)
			continue
		}
		if !strings.Contains(err.Error(), "non-zero") {
			t.Errorf("Scale(%v): error should mention non-zero, got: %v", c, err)
		}
	}
}

func TestSolidScaleNegativeFactorAllowed(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Negative factor = mirror along that axis; must succeed.
	s, err := cube.Scale(-1, 1, 1, 0, 0, 0)
	if err != nil {
		t.Fatalf("negative scale should be allowed: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil Solid from negative scale")
	}
}

func TestSketchScaleZeroFactorRejected(t *testing.T) {
	sq := CreateSquare(10, 10)
	cases := [][2]float64{
		{0, 1},
		{1, 0},
	}
	for _, c := range cases {
		_, err := sq.Scale(c[0], c[1], 0, 0)
		if err == nil {
			t.Errorf("Sketch.Scale(%v): expected error, got nil", c)
			continue
		}
		if !strings.Contains(err.Error(), "non-zero") {
			t.Errorf("Sketch.Scale(%v): error should mention non-zero, got: %v", c, err)
		}
	}
}
