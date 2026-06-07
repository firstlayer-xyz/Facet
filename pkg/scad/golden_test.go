package scad

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

var update = flag.Bool("update", false, "regenerate golden .fct files")

// goldenCases lists testdata stems. Add a stem per fixture pair.
var goldenCases = []string{
	"cube",
	"sphere",
	"cylinder",
	"cylinder_center",
	"cylinder_cone",
	"circle",
	"circle_d",
	"square",
	"square_center",
	"polygon",
	"text",
	"translate",
	"rotate",
	"transform_chain",
	"multi_child",
	"scale",
	"mirror",
	"color_name",
	"color_rgb",
	"translate_2d",
	"difference",
	"union_many",
	"intersection",
	"difference_nested",
	"hull",
	"linear_extrude",
	"linear_extrude_twist",
	"rotate_extrude",
	"projection",
	"offset",
	"icosphere",
	"bosl2_primitives",
	"bosl2_attachment",
	"bosl2_distributors",
	"bosl2_tube",
	"bosl2_radial",
	"bosl2_copies",
	"bosl2_oriented",
	"bosl2_diff",
	"bosl2_2d",
	"bosl2_torus",
	"bosl2_prismoid",
}

func TestGolden(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			scadPath := filepath.Join("testdata", name+".scad")
			fctPath := filepath.Join("testdata", name+".fct")
			srcBytes, err := os.ReadFile(scadPath)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Transpile(string(srcBytes), scadPath)
			if err != nil {
				t.Fatalf("transpile: %v", err)
			}
			assertTypeChecks(t, res.Facet)
			if *update {
				if err := os.WriteFile(fctPath, []byte(res.Facet), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(fctPath)
			if err != nil {
				t.Fatal(err)
			}
			if res.Facet != string(want) {
				t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, res.Facet, want)
			}
		})
	}
}

// assertTypeChecks loads the Facet source with the stdlib and runs the checker.
func assertTypeChecks(t *testing.T, facetSrc string) {
	t.Helper()
	prog, err := loader.Load(context.Background(), facetSrc, "<transpiled>", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("load transpiled Facet: %v\n%s", err, facetSrc)
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		t.Fatalf("transpiled Facet failed type-check: %v\n%s", errs[0], facetSrc)
	}
}
