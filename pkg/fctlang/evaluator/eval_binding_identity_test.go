package evaluator

import (
	"context"
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

// TestReusedMultipartInterimSharesIdentity documents the interim tradeoff of the
// multi-part guard: reidentify no longer touches multi-part solids, so a 2-part
// `proto` reused for a, b, c keeps its two groups and the three reuses SHARE them
// (pre-per-binding-identity behavior, but only for multi-part). The important
// guarantee here is that `part` is not flattened — every part stays clickable.
// The per-part reidentify follow-up restores per-reuse distinctness (each reuse
// gets its own fresh set of part identities) without collapsing anything.
func TestReusedMultipartInterimSharesIdentity(t *testing.T) {
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
	// a, b, c share proto's two part identities; with the sphere that's 3 groups.
	// (The per-part reidentify follow-up makes this 7 — two fresh parts per reuse
	// plus the sphere — each independently selectable.)
	if got := len(result.Solids[0].FaceMap); got != 3 {
		t.Fatalf("interim reused-multipart: want 3 shared groups, got %d", got)
	}
	// The parts are not flattened: proto's assembly keeps more than one group, so
	// a click still distinguishes the big cylinder from the small one.
	if len(result.Solids[0].FaceMap) < 2 {
		t.Fatalf("multi-part solid was flattened to a single group")
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
