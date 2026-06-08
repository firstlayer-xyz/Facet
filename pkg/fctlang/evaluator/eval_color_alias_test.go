package evaluator

import (
	"context"
	"testing"

	"facet/pkg/manifold"
)

// Color is the one Solid op that mutated in place, and a *Solid is shared by
// assignment (copyValue doesn't copy Solids), so coloring used to leak onto the
// original. Color a derived binding and return the original — it must stay
// uncolored.
func TestEvalColorDoesNotMutateReceiver(t *testing.T) {
	src := `fn Main() Solid {
    var x = Cube(s: 10 mm);
    var y = x.Color(hex: "#ff0000");
    return x;
}`
	prog := parseTestProg(t, src)
	result, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	x := result.Solids[0]
	for id, fi := range x.FaceMap {
		if fi.Color != manifold.NoColor {
			t.Errorf("returned solid's face %d is colored (%#x) — color leaked from the y binding", id, fi.Color)
		}
	}
}
