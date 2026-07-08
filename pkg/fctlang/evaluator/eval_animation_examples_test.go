package evaluator

import (
	"context"
	"os"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// TestAnimationExamplesRenderAFrame loads each bundled animation example,
// type-checks it, evaluates it as an Animation, renders one frame, and asserts
// non-empty geometry.
func TestAnimationExamplesRenderAFrame(t *testing.T) {
	examples := []string{
		"../../../share/examples/Animation Cube.fct",
		"../../../share/examples/Animation Clock.fct",
	}

	for _, rel := range examples {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			src, err := os.ReadFile(rel)
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}

			prog, err := loader.Load(context.Background(), string(src), rel, parser.SourceUser, "", nil)
			if err != nil {
				t.Fatalf("%s load: %v", rel, err)
			}

			if errs := checker.Check(prog).Errors; len(errs) > 0 {
				t.Fatalf("%s check: %v", rel, errs)
			}

			res, err := Eval(context.Background(), prog, rel, nil, "Main")
			if err != nil {
				t.Fatalf("%s eval: %v", rel, err)
			}
			if res.Animation == nil {
				t.Fatalf("%s: expected an Animation entry", rel)
			}

			solid, err := res.Animation.Frame(context.Background(), 1700000000000)
			if err != nil {
				t.Fatalf("%s Frame: %v", rel, err)
			}
			if solid.Volume() <= 0 {
				t.Fatalf("%s: frame produced empty geometry (volume=%.4f)", rel, solid.Volume())
			}
		})
	}
}
