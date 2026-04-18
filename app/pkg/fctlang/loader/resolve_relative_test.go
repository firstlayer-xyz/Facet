package loader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"facet/app/pkg/fctlang/parser"
)

// writeLib writes a .fct file at <root>/<name>/<name>.fct with the given body.
func writeLib(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name+".fct")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestRelativeImportResolves exercises the full resolver path for a main
// source that imports a sibling library via "./sibling". The main source
// lives inside a tempdir we pick as both parent dir and containment root.
func TestRelativeImportResolves(t *testing.T) {
	root := t.TempDir()
	writeLib(t, root, "sidekick", "var X = 42\n")
	mainPath := filepath.Join(root, "main.fct")
	mainSrc := `var sidekick = lib "./sidekick"
var y = sidekick.X
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	prog, err := Load(context.Background(), mainSrc, mainPath, parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The relative import should resolve to the sibling's .fct disk path and
	// appear in both Imports (keyed by the canonical resolved path) and
	// Sources (keyed by the same disk path).
	expectKey := filepath.Join(root, "sidekick", "sidekick.fct")
	diskPath, ok := prog.Imports[expectKey]
	if !ok {
		t.Fatalf("Imports missing key %q; got keys: %v", expectKey, keysOf(prog.Imports))
	}
	if diskPath != expectKey {
		t.Errorf("expected Imports[%q]=%q, got %q", expectKey, expectKey, diskPath)
	}
	if _, ok := prog.Sources[expectKey]; !ok {
		t.Errorf("Sources missing %q", expectKey)
	}
}

// TestRelativeImportTwoSourcesNoCollision verifies that two main-style
// sources in different directories each importing "./sidekick" resolve to
// distinct libraries — the canonical Resolved key disambiguates them.
func TestRelativeImportTwoSourcesNoCollision(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeLib(t, rootA, "sidekick", "var X = 1\n")
	writeLib(t, rootB, "sidekick", "var X = 2\n")

	mainA := filepath.Join(rootA, "main.fct")
	mainB := filepath.Join(rootB, "main.fct")
	srcA := `var s = lib "./sidekick"` + "\n"
	srcB := `var s = lib "./sidekick"` + "\n"
	for _, w := range []struct{ path, body string }{{mainA, srcA}, {mainB, srcB}} {
		if err := os.WriteFile(w.path, []byte(w.body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	progA, err := Load(context.Background(), srcA, mainA, parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("Load A: %v", err)
	}
	progB, err := Load(context.Background(), srcB, mainB, parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("Load B: %v", err)
	}

	keyA := filepath.Join(rootA, "sidekick", "sidekick.fct")
	keyB := filepath.Join(rootB, "sidekick", "sidekick.fct")
	if _, ok := progA.Sources[keyA]; !ok {
		t.Errorf("progA.Sources missing %q", keyA)
	}
	if _, ok := progB.Sources[keyB]; !ok {
		t.Errorf("progB.Sources missing %q", keyB)
	}
	if keyA == keyB {
		t.Fatalf("test setup bug: tmp roots collided (%q)", keyA)
	}
}

// TestRelativeImportContainmentEscape ensures that a "../" that would leave
// the containment root is rejected, not silently followed.
func TestRelativeImportContainmentEscape(t *testing.T) {
	root := t.TempDir()
	// Set up: <root>/inner/main.fct and a victim dir at <root>/outside/.
	innerDir := filepath.Join(root, "inner")
	if err := os.MkdirAll(innerDir, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	writeLib(t, root, "outside", "var X = 99\n")
	mainPath := filepath.Join(innerDir, "main.fct")
	// Main tries to escape its own directory by going up.
	mainSrc := `var x = lib "../outside"` + "\n"
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	_, err := Load(context.Background(), mainSrc, mainPath, parser.SourceUser, "", nil)
	if err == nil {
		t.Fatal("expected containment-escape error, got nil")
	}
	if !strings.Contains(err.Error(), "escapes containment") && !strings.Contains(err.Error(), "could not resolve") {
		t.Errorf("expected containment error, got: %v", err)
	}
}

// TestRelativeImportFromNonAbsoluteKey verifies that a source loaded under a
// virtual/non-absolute key cannot use relative imports (there's no meaningful
// dir to resolve against).
func TestRelativeImportFromNonAbsoluteKey(t *testing.T) {
	mainSrc := `var x = lib "./foo"` + "\n"
	_, err := Load(context.Background(), mainSrc, "scratch:main", parser.SourceUser, "", nil)
	if err == nil {
		t.Fatal("expected error for relative import from non-absolute key")
	}
	if !strings.Contains(err.Error(), "not allowed") && !strings.Contains(err.Error(), "could not resolve") {
		t.Errorf("expected disallowed error, got: %v", err)
	}
}

// TestRelativeImportCousinUncleWithinRepo verifies that siblings, uncles, and
// cousins all resolve correctly when the importing source lives inside a
// library whose containment root spans the whole "repo" (as it does for
// remote libs loaded from the bare-clone git tree or an installedLibs override).
//
// Tree under repoRoot:
//
//	familyA/
//	  brother/brother.fct   -- imports "../sister" (sibling) + "../../familyB/cousin" + "../../familyB"
//	  sister/sister.fct
//	familyB/
//	  familyB.fct           -- the "uncle" (parent of cousin, also a lib itself)
//	  cousin/cousin.fct
func TestRelativeImportCousinUncleWithinRepo(t *testing.T) {
	repoRoot := t.TempDir()

	// Write libs. Each lib is <dir>/<name>/<name>.fct. familyB is special —
	// it's a lib at the top level of its own subtree (the "uncle").
	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// sister (sibling of brother)
	mustWrite(filepath.Join(repoRoot, "familyA", "sister", "sister.fct"),
		`var S = 1`+"\n")
	// cousin (lives under familyB — a cousin to brother via the grandparent)
	mustWrite(filepath.Join(repoRoot, "familyB", "cousin", "cousin.fct"),
		`var C = 2`+"\n")
	// familyB itself as a library (the "uncle")
	mustWrite(filepath.Join(repoRoot, "familyB", "familyB.fct"),
		`var U = 3`+"\n")
	// brother imports all three kin.
	mustWrite(filepath.Join(repoRoot, "familyA", "brother", "brother.fct"),
		`var sister = lib "../sister"
var cousin = lib "../../familyB/cousin"
var uncle  = lib "../../familyB"
var total = sister.S + cousin.C + uncle.U
`)

	// Main source, outside the "repo", imports brother via a fake remote
	// path whose resolution is redirected by installedLibs. This is the
	// shape the real-world intra-facetlibs imports rely on: brother is
	// loaded with its containment root set to the whole repo, so its
	// "../" and "../../" imports resolve within that root.
	mainDir := t.TempDir()
	mainPath := filepath.Join(mainDir, "main.fct")
	mainSrc := `var brother = lib "github.com/fake/repo/familyA/brother@x"
var got = brother.total
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	opts := &Options{
		InstalledLibs: map[string]string{"github.com/fake/repo": repoRoot},
	}
	prog, err := Load(context.Background(), mainSrc, mainPath, parser.SourceUser, "", opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	expects := []string{
		filepath.Join(repoRoot, "familyA", "sister", "sister.fct"),
		filepath.Join(repoRoot, "familyB", "cousin", "cousin.fct"),
		filepath.Join(repoRoot, "familyB", "familyB.fct"),
	}
	for _, e := range expects {
		if _, ok := prog.Sources[e]; !ok {
			t.Errorf("Sources missing %q; got keys: %v", e, keysOf(prog.Imports))
		}
		if _, ok := prog.Imports[e]; !ok {
			t.Errorf("Imports missing key %q; got keys: %v", e, keysOf(prog.Imports))
		}
	}
}

// TestRelativeImportCousinEscapesRoot ensures a cousin-style "../../" from a
// lib still can't traverse above its containment root. Brother's root is the
// override dir; "../../../outside" must be rejected.
func TestRelativeImportCousinEscapesRoot(t *testing.T) {
	repoRoot := t.TempDir()
	outsideParent := filepath.Dir(repoRoot) // one level above the override root

	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	// Decoy "outside" lib the escape attempt targets — we want to confirm
	// the loader doesn't follow the traversal even though a real lib exists.
	mustWrite(filepath.Join(outsideParent, "outside", "outside.fct"), `var X = 0`+"\n")
	// Brother tries to escape upward past the override root.
	mustWrite(filepath.Join(repoRoot, "familyA", "brother", "brother.fct"),
		`var sneaky = lib "../../../outside"`+"\n")

	mainDir := t.TempDir()
	mainPath := filepath.Join(mainDir, "main.fct")
	mainSrc := `var brother = lib "github.com/fake/repo/familyA/brother@x"` + "\n"
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	opts := &Options{
		InstalledLibs: map[string]string{"github.com/fake/repo": repoRoot},
	}
	_, err := Load(context.Background(), mainSrc, mainPath, parser.SourceUser, "", opts)
	if err == nil {
		t.Fatal("expected containment-escape error from cousin-style traversal, got nil")
	}
	if !strings.Contains(err.Error(), "escapes containment") && !strings.Contains(err.Error(), "could not resolve") {
		t.Errorf("expected containment error, got: %v", err)
	}
}

func keysOf(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
