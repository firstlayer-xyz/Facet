//go:build !js

package manifold

import (
	"math"
	"testing"
)

// CreateCylinder with radius_low=0 and radius_high>0 builds a cone with the
// apex at z=0 and the base at z=height. Manifold's underlying Cylinder
// returns an empty mesh for this orientation, so the C++ binding builds the
// symmetric (top-tip) cone and reflects Z about height/2. Verify the result
// has the expected cone volume (1/3·π·r²·h).
func TestCreateCylinderApexAtBottom(t *testing.T) {
	const r = 5.0
	const h = 10.0
	const segments = 256

	solid, err := CreateCylinder(h, 0, r, segments)
	if err != nil {
		t.Fatalf("CreateCylinder: %v", err)
	}
	want := math.Pi * r * r * h / 3
	got := solid.Volume()
	// A 256-segment regular polygon underestimates a true circle by a small
	// fraction; 1% tolerance is comfortable.
	if math.Abs(got-want)/want > 0.01 {
		t.Fatalf("apex-at-bottom cone volume = %v, want ≈ %v (a volume of 0 means the r_low=0 workaround failed)", got, want)
	}
}

// The opposite orientation (radius_low>0, radius_high=0, apex at top) was
// already supported by Manifold; this test pins it so a future refactor of
// the binding can't regress it.
func TestCreateCylinderApexAtTop(t *testing.T) {
	const r = 5.0
	const h = 10.0
	const segments = 256

	solid, err := CreateCylinder(h, r, 0, segments)
	if err != nil {
		t.Fatalf("CreateCylinder: %v", err)
	}
	want := math.Pi * r * r * h / 3
	got := solid.Volume()
	if math.Abs(got-want)/want > 0.01 {
		t.Fatalf("apex-at-top cone volume = %v, want ≈ %v", got, want)
	}
}
