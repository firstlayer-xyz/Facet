package fctlang

import (
	"context"
	fctchecker "facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
	"os"
	"path/filepath"
	"testing"
	"time"

	"facet/app/stdlib"
)

const testMainKey = "/test/main.fct"

// testStdlibLibs returns a Program with stdlib parsed and seeded.
// Used by test helpers that need stdlib available.
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
// with stdlib seeded into Libs so the checker and evaluator can find it.
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

// evalMerged is a test helper that calls evaluator.Eval, extracts render meshes, and merges them.
func evalMerged(ctx context.Context, prog loader.Program, overrides map[string]interface{}) (*manifold.Mesh, error) {
	result, err := evaluator.Eval(ctx, prog, testMainKey, overrides, "Main")
	if err != nil {
		return nil, err
	}
	meshes := make([]*manifold.Mesh, len(result.Solids))
	for i, s := range result.Solids {
		meshes[i] = s.ToMesh()
	}
	return manifold.MergeMeshes(meshes), nil
}

func TestAllExamples(t *testing.T) {
	// Use a temp dir — the loader falls back to embedded stdlib automatically
	libDir := t.TempDir()

	examplesDir := filepath.Join("..", "..", "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("reading examples dir: %v", err)
	}

	localFacetlibs, _ := filepath.Abs(filepath.Join("..", "..", "..", "facetlibs"))
	resolveOpts := &loader.Options{
		InstalledLibs: map[string]string{
			"github.com/firstlayer-xyz/facetlibs": localFacetlibs,
		},
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".fct" {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(examplesDir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}

			prog := parseTestProg(t, string(src))
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}

			// Resolve libraries
			resolveTestProg(t, prog, libDir, resolveOpts)

			// Check for type errors — must be zero
			checkErrs := fctchecker.Check(prog).Errors
			for _, ce := range checkErrs {
				t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
			}

			// Evaluate with a timeout
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := evaluator.Eval(ctx, prog, testMainKey, nil, "Main")
			if err != nil {
				t.Fatalf("eval %s: %v", name, err)
			}
			if len(result.Solids) == 0 {
				t.Errorf("%s: expected non-empty solids", name)
			} else {
				t.Logf("%s: %d solids", name, len(result.Solids))
			}
		})
	}
}

func TestInferredArrayTypes(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "number array",
			src: `
fn Main() {
    var a = [1, 2, 3];
    return Cube(size: Vec3{x: a[0] * 1 mm, y: 1 mm, z: 1 mm});
}`,
		},
		{
			name: "nested number array",
			src: `
fn Main() {
    var a = [[1, 2], [3, 4]];
    return Cube(size: Vec3{x: a[0][0] * 1 mm, y: 1 mm, z: 1 mm});
}`,
		},
		{
			name: "for-yield infers solid",
			src: `
fn Main() {
    var cubes = for i [0:<2] {
        yield Cube(size: Vec3{x: (i + 1) mm, y: 1 mm, z: 1 mm});
    };
    return fold a, b cubes { yield a + b; };
}`,
		},
		{
			name: "number-length promotion",
			src: `
fn Main() {
    var a = [1 mm, 2];
    return Cube(size: Vec3{x: a[0], y: a[1] * 1 mm, z: 1 mm});
}`,
		},
		{
			name: "explicit typed array still works",
			src: `
fn Main() {
    var a = []Number[1, 2, 3];
    return Cube(size: Vec3{x: a[0] * 1 mm, y: 1 mm, z: 1 mm});
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := parseTestProg(t, tt.src)
			mesh, err := evalMerged(context.Background(), prog, nil)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if mesh == nil || len(mesh.Vertices) == 0 {
				t.Error("expected non-empty mesh")
			}
		})
	}
}
