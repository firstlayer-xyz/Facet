package evaluator

import (
	"context"
	"testing"
)

// Offset(+2) on a cube must grow its volume; the result must mesh non-empty.
func TestStdlibSolidOffsetGrows(t *testing.T) {
	src := `
fn Main() {
    var base = Cube(s: 20 mm).Volume();
    var off  = Cube(s: 20 mm).Offset(delta: 2 mm).Volume();
    var ratio = off / base;
    assert ratio > 1.0, "Offset(+2) should grow the cube";
    return Cube(s: 20 mm).Offset(delta: 2 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh")
	}
}
