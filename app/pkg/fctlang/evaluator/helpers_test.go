package evaluator

import (
	"context"
	fctchecker "facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/stdlib"
	"math"
	"testing"

	"facet/app/pkg/manifold"
)

// evalMerged is a test helper that calls Eval, extracts render meshes, and merges them.
func evalMerged(ctx context.Context, prog loader.Program, overrides map[string]interface{}) (*manifold.Mesh, error) {
	result, err := Eval(ctx, prog, testMainKey, overrides, "Main")
	if err != nil {
		return nil, err
	}
	meshes := make([]*manifold.Mesh, len(result.Solids))
	for i, s := range result.Solids {
		meshes[i] = s.ToMesh()
	}
	return manifold.MergeMeshes(meshes), nil
}

// meshBounds returns the axis-aligned bounding box of a mesh as (minX, minY, minZ, maxX, maxY, maxZ).
func meshBounds(m *manifold.Mesh) (float32, float32, float32, float32, float32, float32) {
	if len(m.Vertices) < 3 {
		return 0, 0, 0, 0, 0, 0
	}
	minX, minY, minZ := float32(math.MaxFloat32), float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY, maxZ := float32(-math.MaxFloat32), float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for i := 0; i < len(m.Vertices); i += 3 {
		x, y, z := m.Vertices[i], m.Vertices[i+1], m.Vertices[i+2]
		if x < minX { minX = x }
		if y < minY { minY = y }
		if z < minZ { minZ = z }
		if x > maxX { maxX = x }
		if y > maxY { maxY = y }
		if z > maxZ { maxZ = z }
	}
	return minX, minY, minZ, maxX, maxY, maxZ
}

// assertMeshSize checks that the mesh bounding box matches expected dimensions within tolerance.
func assertMeshSize(t *testing.T, m *manifold.Mesh, wantW, wantH, wantD float32, tol float32) {
	t.Helper()
	minX, minY, minZ, maxX, maxY, maxZ := meshBounds(m)
	gotW := maxX - minX
	gotH := maxY - minY
	gotD := maxZ - minZ
	if abs32(gotW-wantW) > tol {
		t.Errorf("width: got %.2f, want %.2f (tol %.2f)", gotW, wantW, tol)
	}
	if abs32(gotH-wantH) > tol {
		t.Errorf("height: got %.2f, want %.2f (tol %.2f)", gotH, wantH, tol)
	}
	if abs32(gotD-wantD) > tol {
		t.Errorf("depth: got %.2f, want %.2f (tol %.2f)", gotD, wantD, tol)
	}
}

func abs32(x float32) float32 {
	if x < 0 { return -x }
	return x
}

const testMainKey = "/test/main.fct"

// testStdlibLibs returns a Program with stdlib parsed and seeded.
func testStdlibLibs() loader.Program {
	prog := loader.NewProgram()
	stdSrc, err := parser.Parse(stdlib.StdlibSource)
	if err != nil {
		panic("stdlib parse error: " + err.Error())
	}
	stdSrc.Path = loader.StdlibPath
	stdSrc.Text = stdlib.StdlibSource
	prog.Sources[loader.StdlibPath] = stdSrc
	return prog
}

// parseTestProg is a test helper that parses source and wraps it in a loader.Program
// with stdlib seeded into Libs so the evaluator can find it.
func parseTestProg(t *testing.T, src string) loader.Program {
	t.Helper()
	s, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	prog := testStdlibLibs()
	prog.Sources[testMainKey] = s
	return prog
}

// resolveTestProg resolves libraries in a test program.
func resolveTestProg(t *testing.T, prog loader.Program, libDir string, opts *loader.Options) {
	t.Helper()
	if err := loader.ResolveLibraries(context.Background(), prog, testMainKey, libDir, opts); err != nil {
		t.Fatalf("ResolveLibraries: %v", err)
	}
}

// checkSource is a test helper that parses, wraps in a Program with stdlib, and runs the checker.
func checkSource(t *testing.T, src string) []parser.SourceError {
	t.Helper()
	prog := parseTestProg(t, src)
	return fctchecker.Check(prog).Errors
}

// expectNoErrors is a test helper that runs the checker and fails if there are errors.
func expectNoErrors(t *testing.T, src string) {
	t.Helper()
	errs := checkSource(t, src)
	for _, e := range errs {
		t.Errorf("unexpected error: %d:%d: %s", e.Line, e.Col, e.Message)
	}
}
