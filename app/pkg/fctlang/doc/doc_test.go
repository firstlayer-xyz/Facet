package doc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryAutocompleteNamespaceMatch(t *testing.T) {
	// Create a fake git cache directory with a library file.
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "github.com", "firstlayer-xyz", "facetlibs", "main", "fasteners")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	libSource := `
type Knurl { count Number; depth Length; angle Angle; }
fn Knurl(count Number, depth Length, angle Angle) Knurl {
    return Knurl { count: count, depth: depth, angle: angle };
}
fn Knurl.Apply(s Solid) Solid { return s; }
`
	if err := os.WriteFile(filepath.Join(libDir, "fasteners.fct"), []byte(libSource), 0o644); err != nil {
		t.Fatal(err)
	}

	// The namespace for "github.com/firstlayer-xyz/facetlibs/fasteners@main"
	// is computed by checker.libPathToNamespace; here we use the known result
	// to verify that BuildLibDocEntries produces matching library fields.
	ns := "github.com/firstlayer-xyz/facetlibs/main/fasteners"

	docEntries := BuildLibDocEntries(tmpDir)

	// Verify doc entries include the expected namespace
	libNamespaces := make(map[string]bool)
	for _, e := range docEntries {
		if e.Library != "" {
			libNamespaces[e.Library] = true
		}
	}
	if !libNamespaces[ns] {
		t.Errorf("no doc entries with library=%q", ns)
	}

	// Verify top-level exports exist (for "F." autocomplete)
	var topLevel []string
	for _, e := range docEntries {
		if e.Library == ns && !strings.Contains(e.Name, ".") {
			topLevel = append(topLevel, e.Name)
		}
	}
	if len(topLevel) == 0 {
		t.Error("no top-level library exports found")
	}
	if !sliceContains(topLevel, "Knurl") {
		t.Errorf("expected Knurl in top-level exports, got %v", topLevel)
	}

	// Verify method entries exist (for "knurl." autocomplete after type resolution)
	var methods []string
	for _, e := range docEntries {
		if e.Library == ns && strings.HasPrefix(e.Name, "Knurl.") {
			methods = append(methods, e.Name)
		}
	}
	if len(methods) == 0 {
		t.Error("no Knurl.* method entries found")
	}

	// Verify field entries exist
	var fields []string
	for _, e := range docEntries {
		if e.Library == ns && e.Kind == "field" {
			fields = append(fields, e.Name)
		}
	}
	if len(fields) == 0 {
		t.Error("no struct field entries found")
	}
}

func sliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
