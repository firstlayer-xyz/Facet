package evaluator

import (
	"context"
	"slices"
	"testing"
)

// TestPosMapClickResolvesToOwningBinding pins the face-click ordering: for each
// face, the first posMap entry that contains it (what the frontend clicks into,
// via reversePosMap[face][0]) must be the narrowest binding that owns that face
// — not a whole-model entry (the fn return, or an `a + b` union) which covers
// every face. Without most-specific-first ordering, every click lands on the
// whole-model entry and highlights everything, defeating per-variable identity.
func TestPosMapClickResolvesToOwningBinding(t *testing.T) {
	prog := parseTestProg(t, `fn Main() Solid {
    var a = Cylinder(d: 20 mm, h: 60 mm).Rotate(y: 90 deg)
    var b = a.Rotate(z: 90 deg)
    return a + b
}`)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := len(result.Solids[0].FaceMap); got != 2 {
		t.Fatalf("want 2 distinct face groups (a, b), got %d", got)
	}
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
			t.Errorf("clicking face %d lands on a %d-face entry (L%d) — should resolve to the single owning binding, not the whole model",
				id, len(first.FaceIDs), first.Line)
		}
	}
}
