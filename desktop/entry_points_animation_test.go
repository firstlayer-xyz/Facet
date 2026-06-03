package main

import (
	"context"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// TestGetEntryPointsMarksAnimation verifies that an Animation-returning entry
// appears in the entry-point list with Animated=true.
func TestGetEntryPointsMarksAnimation(t *testing.T) {
	src := `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: 10 mm) }}
}
`
	prog, err := loader.Load(context.Background(), src, "main.fct", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		t.Fatalf("check: %v", checked.Errors)
	}
	eps := getEntryPoints(checked.Prog, checked.InferredReturnTypes)
	var main *EntryPoint
	for i := range eps {
		if eps[i].Name == "Main" {
			main = &eps[i]
		}
	}
	if main == nil {
		t.Fatal("Main not found as an entry point")
	}
	if !main.Animated {
		t.Fatal("expected Main.Animated == true for an Animation entry")
	}
}
