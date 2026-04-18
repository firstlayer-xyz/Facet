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
	"strings"
	"testing"
	"time"

	"facet/app/stdlib"
)

const testMainKey = "/test/main.fct"

// testStdlibLibs returns a Program with stdlib parsed and seeded.
// Used by test helpers that need stdlib available.
func testStdlibLibs() loader.Program {
	prog := loader.NewProgram()
	stdSrc, err := parser.Parse(stdlib.StdlibSource, "", parser.SourceUser)
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
	s, err := parser.Parse(src, "", parser.SourceUser)
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

// exampleOverrides pins parameters for examples whose Main defaults depend on
// real-world state (clock, locale, etc.).  Without these, the integration test
// becomes non-deterministic and fires on an unrelated assertion inside the
// example's own logic.  Keyed by example file name.
//
// Moon.fct: UtcDate() defaults to today; the example then asserts that the
// moon phase is strictly between (0.03, 0.97) — so the test fires on real-world
// new/full moon days.  1/14/2000 sits ~8 days after the epoch new moon, phase
// ≈ 0.27 (waxing gibbous), comfortably inside the valid range.
var exampleOverrides = map[string]map[string]interface{}{
	"Moon.fct": {"date": "1/14/2000"},
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

			result, err := evaluator.Eval(ctx, prog, testMainKey, exampleOverrides[name], "Main")
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

func TestConstraintViolationOnReassign(t *testing.T) {
	// Reassigning a constrained variable to a value outside the constraint should error.
	src := `
fn Main() {
    var x = 10 mm where [0:100] mm
    x = 200 mm
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`
	prog := parseTestProg(t, src)
	checkErrs := fctchecker.Check(prog).Errors
	for _, ce := range checkErrs {
		t.Errorf("[check] %s", ce.Message)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := evaluator.Eval(ctx, prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected constraint violation error, got nil")
	}
	if !strings.Contains(err.Error(), "constraint") && !strings.Contains(err.Error(), "range") {
		t.Fatalf("expected constraint error, got: %v", err)
	}
}

func TestDisc(t *testing.T) {
	src := `
fn Disc(
    str String = "hello" where [],
) {
    var text = Text(text: str, s: 12 mm).Extrude(z: 1)
        .Color(c: Color(r: 1, g: 1, b: 0))
    var d = Cube(x: text.Width(), y: text.Depth(), z: text.Height())
    return text + d.AlignCenter(with: text)
}
`
	prog := parseTestProg(t, src)
	checkErrs := fctchecker.Check(prog).Errors
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := evaluator.Eval(ctx, prog, testMainKey, nil, "Disc")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Error("expected non-empty solids")
	}
}

func TestMultiFileNoFalseShadow(t *testing.T) {
	// Two unrelated files in the same program. File A has a global "var d".
	// File B has a function with a local "var d". The checker should NOT
	// report a "shadows outer variable" error for file B's local.
	prog := testStdlibLibs()

	srcA, err := parser.Parse(`var d = 2 mm`, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse A: %v", err)
	}
	keyA := "/test/a.fct"
	prog.Sources[keyA] = srcA

	srcB, err := parser.Parse(`
fn Main() {
    var d = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
    return d
}
`, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse B: %v", err)
	}
	keyB := "/test/b.fct"
	prog.Sources[keyB] = srcB

	checkResult := fctchecker.Check(prog)
	for _, ce := range checkResult.Errors {
		if ce.Message == `variable "d" shadows outer variable` {
			t.Errorf("false shadow error: %s at %s:%d:%d", ce.Message, ce.File, ce.Line, ce.Col)
		}
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
    return Cube(s: Vec3{x: a[0] * 1 mm, y: 1 mm, z: 1 mm});
}`,
		},
		{
			name: "nested number array",
			src: `
fn Main() {
    var a = [[1, 2], [3, 4]];
    return Cube(s: Vec3{x: a[0][0] * 1 mm, y: 1 mm, z: 1 mm});
}`,
		},
		{
			name: "for-yield infers solid",
			src: `
fn Main() {
    var cubes = for i [0:<2] {
        yield Cube(s: Vec3{x: (i + 1) mm, y: 1 mm, z: 1 mm});
    };
    return fold a, b cubes { yield a + b; };
}`,
		},
		{
			name: "number-length promotion",
			src: `
fn Main() {
    var a = [1 mm, 2];
    return Cube(s: Vec3{x: a[0], y: a[1] * 1 mm, z: 1 mm});
}`,
		},
		{
			name: "explicit typed array still works",
			src: `
fn Main() {
    var a = []Number[1, 2, 3];
    return Cube(s: Vec3{x: a[0] * 1 mm, y: 1 mm, z: 1 mm});
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
