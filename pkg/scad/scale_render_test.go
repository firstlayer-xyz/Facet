//go:build cgo

package scad

import (
	"context"
	"os"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// scale(radius) must actually reach the geometry. The bundled icosphere builds
// its vertices on the UNIT sphere and relies on scale(radius) for sizing; with
// scale(scalar) dropped it rendered at radius ~1. A radius-10 icosphere spans
// ~20 across (recursion adds vertices on the axes at ±radius). Bounds are
// orientation-independent, so this checks sizing regardless of winding. CGO-only.
func TestScaleRendersIcosphereAtRadius(t *testing.T) {
	src, err := os.ReadFile("testdata/icosphere.scad")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	res, err := Transpile(string(src)+"\n", "part.scad")
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
		t.Fatalf("eval: %v", err)
	}
	solids, err := result.StaticSolids(context.Background(), 0)
	if err != nil {
		t.Fatalf("solids: %v", err)
	}
	minX, _, _, maxX, _, _ := solids[0].BoundingBox()
	if dx := maxX - minX; dx < 19 || dx > 20.5 {
		t.Errorf("icosphere x-extent = %v, want ~20 (radius 10 — scale applied)", dx)
	}
}
