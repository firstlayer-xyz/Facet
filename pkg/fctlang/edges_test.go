//go:build !js

package fctlang

import (
	"context"
	"testing"

	"facet/pkg/manifold"
)

// Cube's `edges` selects which edges a fillet/chamfer applies to. Phase 1 supports
// all edges, no edges, and a single full axis group (EdgesAlongX/Y/Z); any other
// selection errors rather than producing wrong geometry.
func TestCubeEdgeSelection(t *testing.T) {
	render := func(body string) (*manifold.Mesh, error) {
		prog := parseTestProg(t, "fn Main() Solid {\n"+body+"\n}")
		resolveTestProg(t, prog, t.TempDir(), nil)
		return evalMerged(context.Background(), prog, nil)
	}

	// Supported: each axis group (fillet and chamfer), EDGES_NONE (a rounding
	// with no selected edges → plain box), and a selector that resolves to all
	// edges (exercises EDGES_ALL/.Except/.Eq).
	for _, body := range []string{
		"return Cube(x: 10 mm, y: 20 mm, z: 30 mm, fillet: 2 mm, edges: EdgesAlongZ())",
		"return Cube(x: 10 mm, y: 20 mm, z: 30 mm, fillet: 2 mm, edges: EdgesAlongY())",
		"return Cube(x: 10 mm, y: 20 mm, z: 30 mm, chamfer: 2 mm, edges: EdgesAlongX())",
		"return Cube(s: 10 mm, fillet: 2 mm, edges: EDGES_NONE)",
		"return Cube(s: 10 mm, fillet: 2 mm, edges: EDGES_ALL.Except(o: EDGES_NONE))",
	} {
		mesh, err := render(body)
		if err != nil {
			t.Fatalf("%q should render, got: %v", body, err)
		}
		if mesh == nil || len(mesh.Vertices) == 0 {
			t.Fatalf("%q produced an empty mesh", body)
		}
	}

	// Unsupported: a face group is four edges that meet at corners (corner
	// blending is not implemented yet) — it must error, never approximate.
	if _, err := render("return Cube(s: 10 mm, fillet: 2 mm, edges: FrontEdges())"); err == nil {
		t.Fatal("a face-group edge selection should error (unsupported), got none")
	}
}
