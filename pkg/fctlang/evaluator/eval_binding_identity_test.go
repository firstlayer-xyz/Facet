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

// TestBindingReusePreservesAlpha guards that reidentifying a REUSED translucent
// solid keeps its alpha. Collapsing to one face must carry color AND alpha —
// dropping alpha silently turned the copy fully opaque in the viewport and 3MF
// export.
func TestBindingReusePreservesAlpha(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var proto = Cube(s: 10 mm).Color(c: Color(r: 1, g: 0, b: 0, a: 0.5))
    var a = proto.Move(x: 30 mm)
    return a
}`)
	if got := len(s.FaceMap); got != 1 {
		t.Fatalf("want 1 face group, got %d", got)
	}
	for id, fi := range s.FaceMap {
		if fi.Color != 0xFF0000 || fi.Alpha != 128 {
			t.Fatalf("reuse dropped translucency: face %d = color %#06x alpha %d, want 0xff0000 alpha 128", id, fi.Color, fi.Alpha)
		}
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

// TestTopLevelBindingGivesDistinctIdentity extends the per-binding identity rule
// to MODULE scope: `var a = proto` / `var b = proto.Rotate(...)` at the top level
// must each get their own identity, exactly as local bindings do — otherwise a
// top-level assembly collapses to one selectable object.
func TestTopLevelBindingGivesDistinctIdentity(t *testing.T) {
	s := evalSolid(t, `var proto = Cylinder(d: 20 mm, h: 60 mm)
var a = proto
var b = proto.Rotate(z: 90 deg).Move(x: 30 mm)
fn Main() Solid { return a + b }`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("top-level a and b: want 2 distinct face groups, got %d", got)
	}
}

// TestContainerBindingGivesDistinctIdentity guards that a solid reused inside an
// ARRAY binding is re-originaled per element: `[proto, proto.Move(...)]` must
// yield two selectable parts, not one shared identity.
func TestContainerBindingGivesDistinctIdentity(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var proto = Cube(s: 10 mm)
    var parts = [proto, proto.Move(x: 20 mm)]
    return parts[0] + parts[1]
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("array-reused parts: want 2 distinct face groups, got %d", got)
	}
}

// TestStructBindingGivesDistinctIdentity is the struct counterpart: a solid
// reused across struct fields gets its own identity per field.
func TestStructBindingGivesDistinctIdentity(t *testing.T) {
	s := evalSolid(t, `type Pair {
    l Solid
    r Solid
}
fn Main() Solid {
    var proto = Cube(s: 10 mm)
    var p = Pair{l: proto, r: proto.Move(x: 20 mm)}
    return p.l + p.r
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("struct-reused parts: want 2 distinct face groups, got %d", got)
	}
}

// TestContainerFreshPartsUnchanged guards the other direction: an array of
// FRESH per-element geometry (no reuse of a scoped solid) passes through with
// its parts intact — the recurse must not spuriously touch or flatten them.
func TestContainerFreshPartsUnchanged(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    var parts = [Cube(s: 10 mm), Cube(s: 10 mm).Move(x: 20 mm)]
    return parts[0] + parts[1]
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("fresh array parts: want 2 distinct face groups, got %d", got)
	}
}
