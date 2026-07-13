package evaluator

import "testing"

// TestMultipartSurvivesArrayIndex is the pipe-connector regression: a multi-part
// solid (a distinct object, NOT a copy of another) placed in an array and read
// back by index must keep its per-part face groups. Otherwise a click on any
// face resolves to the same single group and face-click can't tell the hub from
// a plug from a label. Before the fix, container round-trip collapsed the part
// to one group.
func TestMultipartSurvivesArrayIndex(t *testing.T) {
	s := evalSolid(t, `fn Main() []Solid {
    var part = Cylinder(d: 20 mm, h: 40 mm) + Cylinder(d: 10 mm, h: 60 mm).Move(x: 30 mm)
    var parts = [part]
    return [parts[0]]
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("multi-part solid via array index: want 2 face groups preserved, got %d", got)
	}
}

// TestMultipartSurvivesForYield is the same regression via `for p parts { yield p }`,
// the other export path the pipe program uses.
func TestMultipartSurvivesForYield(t *testing.T) {
	s := evalSolid(t, `fn Main() []Solid {
    var part = Cylinder(d: 20 mm, h: 40 mm) + Cylinder(d: 10 mm, h: 60 mm).Move(x: 30 mm)
    var parts = [part]
    return for p parts { yield p }
}`)
	if got := len(s.FaceMap); got != 2 {
		t.Fatalf("multi-part solid via for-yield: want 2 face groups preserved, got %d", got)
	}
}
