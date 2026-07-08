//go:build cgo

package scad

import (
	"context"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// OpenSCAD winds polyhedron faces clockwise-from-outside; the Facet Mesh
// primitive (since the auto-orient kernel fix) orients itself outward, so a
// transpiled polyhedron renders as a solid with OUTWARD normals (positive
// volume) rather than inside-out. The canonical OpenSCAD-docs square pyramid is
// +1333.3 when oriented correctly. This is the user-facing manifestation of the
// CreateSolidFromMesh orientation fix; CGO-only (links the kernel).
func TestPolyhedronRendersOutward(t *testing.T) {
	res, err := Transpile(
		"polyhedron(points=[[10,10,0],[10,-10,0],[-10,-10,0],[-10,10,0],[0,0,10]],"+
			"faces=[[0,1,4],[1,2,4],[2,3,4],[3,0,4],[1,0,3],[2,1,3]]);\n", "part.scad")
	if err != nil {
		t.Fatalf("transpile: %v", err)
	}
	ctx := context.Background()
	const key = "<transpiled>"
	prog, err := loader.Load(ctx, res.Facet, key, parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("load: %v\n%s", err, res.Facet)
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		t.Fatalf("type-check: %v\n%s", errs[0], res.Facet)
	}
	result, err := evaluator.Eval(ctx, prog, key, nil, "Main")
	if err != nil {
		t.Fatalf("eval: %v\n%s", err, res.Facet)
	}
	solids, err := result.StaticSolids(context.Background(), 0)
	if err != nil {
		t.Fatalf("solids: %v", err)
	}
	if v := solids[0].Volume(); v < 1333-1 || v > 1333+1 {
		t.Errorf("pyramid volume = %v, want ~+1333.3 (outward, not inside-out)", v)
	}
}
