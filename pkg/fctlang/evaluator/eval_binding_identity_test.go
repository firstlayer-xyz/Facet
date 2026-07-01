package evaluator

import (
	"context"
	"slices"
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

// TestBindingReidentifiesReusedMultipartSolid pins the reuse case: one uncolored
// 2-part `proto` is reused for a, b, c and assembled into `part`. Each reuse must
// get its own identity so the three connectors are distinct objects, while the
// assembly `part` is NOT collapsed — a click resolves to the specific connector,
// not the whole part.
func TestBindingReidentifiesReusedMultipartSolid(t *testing.T) {
	prog := parseTestProg(t, `fn Main() Solid {
    var proto = Cylinder(d: 20 mm, h: 40 mm) + Cylinder(d: 10 mm, h: 60 mm)
    var a = proto
    var b = proto.Rotate(z: 90 deg)
    var c = proto.Rotate(x: 45 deg)
    var part = a + b + c + Sphere(d: 8 mm)
    return part
}`)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	// a, b, c each collapse to their own id; proto's ids are reidentified away and
	// don't reach the output. With the sphere that's 4 distinct objects.
	if got := len(result.Solids[0].FaceMap); got != 4 {
		t.Fatalf("proto reused for a/b/c + sphere: want 4 distinct objects, got %d", got)
	}
	// Every face's first posMap entry must own it alone, so a click lands on the
	// specific connector's binding, not the whole `part` assembly.
	for id := range result.Solids[0].FaceMap {
		var first *PosEntry
		for i := range result.PosMap {
			if slices.Contains(result.PosMap[i].FaceIDs, id) {
				first = &result.PosMap[i]
				break
			}
		}
		if first == nil {
			t.Fatalf("face %d has no posMap entry", id)
		}
		if len(first.FaceIDs) != 1 {
			t.Errorf("face %d: first posMap entry owns %d ids (want 1) — click would highlight more than this object", id, len(first.FaceIDs))
		}
	}
}
