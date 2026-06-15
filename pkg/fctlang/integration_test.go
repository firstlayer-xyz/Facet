//go:build !js

package fctlang

import (
	"context"
	fctchecker "facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"facet/share/stdlib"
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

// guiEdgeThresholdDeg mirrors the edge threshold the desktop /eval handler
// passes to MergeExtractExpandedMeshes (desktop/eval_handler.go). Keeping the
// same value here means the test renders each example exactly as the GUI does.
const guiEdgeThresholdDeg = 40

// assertRendersInGUI builds the display mesh the desktop /eval handler hands to
// the 3D viewer (manifold.MergeExtractExpandedMeshes) and fails if it is empty.
// Evaluating to a non-empty solid is not enough: a solid can extract to a
// degenerate mesh with no triangles, which evaluates fine but draws nothing on
// screen. This is the assertion that an example actually renders in the GUI.
func assertRendersInGUI(t *testing.T, name string, solids []*manifold.Solid) {
	t.Helper()
	dm := manifold.MergeExtractExpandedMeshes(solids, guiEdgeThresholdDeg)
	if dm.VertexCount == 0 || dm.IndexCount == 0 || dm.ExpandedCount == 0 {
		t.Errorf("%s: empty display mesh (verts=%d indices=%d expanded=%d) — renders blank in the GUI",
			name, dm.VertexCount, dm.IndexCount, dm.ExpandedCount)
		return
	}
	t.Logf("%s: display mesh verts=%d tris=%d", name, dm.VertexCount, dm.IndexCount/3)
}

func TestAllExamples(t *testing.T) {
	// Use a temp dir — the loader falls back to embedded stdlib automatically
	libDir := t.TempDir()

	examplesDir := filepath.Join("..", "..", "share", "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("reading examples dir: %v", err)
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
			resolveTestProg(t, prog, libDir, nil)

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
			switch {
			case result.Animation != nil:
				// Animation examples produce no static solids; render one frame
				// and verify it yields a display mesh the GUI could draw — the
				// desktop /eval handler renders the initial frame the same way.
				solid, err := result.Animation.Frame(1700000000000)
				if err != nil {
					t.Fatalf("%s: Animation.Frame: %v", name, err)
				}
				assertRendersInGUI(t, name, []*manifold.Solid{solid})
			case len(result.Solids) > 0:
				assertRendersInGUI(t, name, result.Solids)
			default:
				t.Errorf("%s: expected non-empty solids or an Animation", name)
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

// TestReferencesRoundTrip_BoltAndNut loads the real example file and confirms
// that the multi-line method chain (`var knurl = F\n    .Knurl(...)`) — the
// original motivator for the AST-driven references work — resolves correctly
// through the full loader → checker pipeline.  Line 14 hosts `.Knurl(...)`;
// its reference target must point at a library file (non-empty `File`).
func TestReferencesRoundTrip_BoltAndNut(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "share", "examples", "Bolt And Nut.fct"))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	prog := parseTestProg(t, string(src))

	resolveTestProg(t, prog, t.TempDir(), nil)

	res := fctchecker.Check(prog)
	for _, ce := range res.Errors {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}

	// Line 14 is "        .Knurl(count: knurl_count, ...)": 8 spaces, then `.`,
	// so the K of Knurl sits at col 10.  Pin the method-token position
	// directly — a looser `strings.Contains(k, ":14:")` check would also be
	// satisfied by the named-arg refs (count/depth/angle) that also live on
	// line 14 and also point at library params, which would silently hide a
	// regression of the method-call ref itself (the original motivating bug).
	got, ok := res.References[testMainKey+":14:10"]
	if !ok {
		t.Fatalf("no reference recorded for .Knurl at %s:14:10; refs=%v", testMainKey, res.References)
	}
	if got.Kind != "fn" {
		t.Errorf("Knurl ref Kind = %q, want fn", got.Kind)
	}
	if got.File == "" {
		t.Error("Knurl ref File is empty; expected library source path")
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
