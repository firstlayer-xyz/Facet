package evaluator

import (
	"context"
	"testing"
)

// Torus is centered on the origin with outer radius r_maj+r_min and a z-extent
// of 2*r_min.
func TestEvalTorus(t *testing.T) {
	src := `fn Main() Solid { return Torus(r_maj: 20 mm, r_min: 3 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, minZ, maxX, _, maxZ := meshBounds(mesh)
	if got := maxX - minX; got < 45.5 || got > 46.5 {
		t.Errorf("Torus x-extent: got %f, want ~46 (outer radius 23)", got)
	}
	if got := maxZ - minZ; got < 5.5 || got > 6.5 {
		t.Errorf("Torus z-extent: got %f, want ~6 (tube radius 3)", got)
	}
}

// Torus accepts diameters too.
func TestEvalTorusDiameter(t *testing.T) {
	src := `fn Main() Solid { return Torus(d_maj: 40 mm, d_min: 6 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if got := maxX - minX; got < 45.5 || got > 46.5 {
		t.Errorf("Torus(d) x-extent: got %f, want ~46", got)
	}
}

// A rounded cylinder keeps the plain cylinder's outer bounds (radius, height)
// but rounds the rims — so it has more triangles than the plain one.
func TestEvalRoundedCylinder(t *testing.T) {
	src := `fn Main() Solid { return Cylinder(r: 10 mm, h: 20 mm, fillet: 2 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, minZ, maxX, _, maxZ := meshBounds(mesh)
	if got := maxX - minX; got < 19.5 || got > 20.5 {
		t.Errorf("rounded cylinder x-extent: got %f, want ~20 (radius 10)", got)
	}
	if got := maxZ - minZ; got < 19.5 || got > 20.5 {
		t.Errorf("rounded cylinder z-extent: got %f, want ~20 (height)", got)
	}
	// Same origin convention as the plain cylinder: min corner at (0,0,0), so
	// fillet 0->2 must not move the shape.
	if minX < -0.5 || minX > 0.5 {
		t.Errorf("rounded cylinder min X: got %f, want ~0 (corner-origin like plain)", minX)
	}
	if minZ < -0.5 || minZ > 0.5 {
		t.Errorf("rounded cylinder min Z: got %f, want ~0", minZ)
	}

	plain := parseTestProg(t, `fn Main() Solid { return Cylinder(r: 10 mm, h: 20 mm); }`)
	pmesh, err := evalMerged(context.Background(), plain, nil)
	if err != nil {
		t.Fatalf("plain eval error: %v", err)
	}
	if len(mesh.Vertices) <= len(pmesh.Vertices) {
		t.Errorf("rounded cylinder should have more vertices than plain: rounded=%d plain=%d",
			len(mesh.Vertices), len(pmesh.Vertices))
	}
}

// fillet: 0 mm is the plain cylinder verbatim (backward compatible).
func TestEvalCylinderFilletZeroUnchanged(t *testing.T) {
	rounded := parseTestProg(t, `fn Main() Solid { return Cylinder(r: 8 mm, h: 12 mm, fillet: 0 mm); }`)
	rmesh, err := evalMerged(context.Background(), rounded, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	plain := parseTestProg(t, `fn Main() Solid { return Cylinder(r: 8 mm, h: 12 mm); }`)
	pmesh, err := evalMerged(context.Background(), plain, nil)
	if err != nil {
		t.Fatalf("plain eval error: %v", err)
	}
	if len(rmesh.Vertices) != len(pmesh.Vertices) {
		t.Errorf("fillet:0 must equal plain: got %d verts, plain %d", len(rmesh.Vertices), len(pmesh.Vertices))
	}
}

// A rounded frustum renders to a valid solid with the wider base's bounds.
func TestEvalRoundedFrustum(t *testing.T) {
	src := `fn Main() Solid { return Frustum(r1: 10 mm, r2: 5 mm, h: 15 mm, fillet: 1 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty rounded frustum mesh")
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if got := maxX - minX; got < 19.5 || got > 20.5 {
		t.Errorf("rounded frustum x-extent: got %f, want ~20 (base radius 10)", got)
	}
}
