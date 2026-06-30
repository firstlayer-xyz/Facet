package evaluator

import (
	"testing"

	"facet/pkg/manifold"
)

// TestBindingGivesDistinctIdentity pins the core rule: a solid bound to a
// variable gets its own identity, so two variables are two selectable/colorable
// objects with no user annotation — even when one is derived from the other by
// transforms, which otherwise preserve the kernel's original ID and leave the
// two sharing one face group.
func TestBindingGivesDistinctIdentity(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var a = Cylinder(d: 20 mm, h: 60 mm).Rotate(y: 90 deg)
    var b = a.Rotate(z: 90 deg)
    return a + b
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("a and derived b: want 2 distinct face groups, got %d", got)
	}
}

// TestBindingPreservesSingleColor verifies the per-binding reidentify carries a
// single-part solid's color onto its fresh identity (a plain AsOriginal would
// drop it).
func TestBindingPreservesSingleColor(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var a = Cube(s: 10 mm).Color(hex: "#ff0000")
    return a
}`)
	if got := len(s.FaceMap); got != 1 {
		t.Fatalf("want 1 face group, got %d", got)
	}
	for _, fi := range s.FaceMap {
		if fi.Color == manifold.NoColor {
			t.Fatalf("auto-reidentify at binding dropped the color")
		}
	}
}

// TestBindingDoesNotFlattenMultipart guards that binding a multi-part solid
// leaves its parts (and their colors) intact — the gate skips reidentify for
// solids that already carry distinct per-part identities.
func TestBindingDoesNotFlattenMultipart(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var pair = Cube(s: 10 mm).Color(hex: "#ff0000") + Sphere(r: 6 mm).Move(x: 20 mm).Color(hex: "#0000ff")
    return pair
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("multi-part binding: want 2 parts preserved, got %d", got)
	}
}
