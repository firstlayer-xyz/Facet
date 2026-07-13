package loader

import (
	"context"
	"strings"
	"testing"
)

// TestLoadMultiIgnoresResolvedLibrarySourcesAsRoots reproduces the desktop
// round-trip bug: when the editor has a remote library open as a view-only tab,
// its text comes back in the sources map keyed by the library's virtual
// "git+..." backing. That key is not a user project root — its relative imports
// ("../threads") only mean something inside the git tree it came from, which is
// not available here. LoadMulti must not try to resolve it as a root; doing so
// hits the "relative imports are not allowed from this source" degrade because a
// virtual key is not an absolute disk path.
func TestLoadMultiIgnoresResolvedLibrarySourcesAsRoots(t *testing.T) {
	libKey := LibSourceScheme + "github.com/firstlayer-xyz/facetlibs@77cec59/fasteners/fasteners.fct"
	sources := map[string]string{
		// A real user project root (absolute path so its own relative imports
		// would be allowed — it just doesn't have any).
		"/tmp/project/main.fct": "fn Main() { return 1 }\n",
		// The view-only library tab the editor round-tripped back to us.
		libKey: "var T = lib \"../threads\"\nfn Knurl() { return 1 }\n",
	}

	prog, err := LoadMulti(context.Background(), sources, "/tmp/project/main.fct", "", nil)
	if err != nil {
		t.Fatalf("LoadMulti must not resolve a view-only library source as a root, got: %v", err)
	}
	// The virtual key must not enter prog.Sources at all: if it did, the checker
	// would later walk it and flag its unresolved "../threads" import, blocking
	// the render even though LoadMulti itself returned no error.
	if _, ok := prog.Sources[libKey]; ok {
		t.Errorf("view-only library source %q must not be added to prog.Sources", libKey)
	}
}

// TestLoadMultiRejectsRelativeImportInUserRoot is the flip side: a genuine user
// source (absolute key) that has an unresolvable relative import must still
// surface an error — the guard above only skips virtual-library keys, not real
// user roots.
func TestLoadMultiRejectsRelativeImportInUserRoot(t *testing.T) {
	sources := map[string]string{
		// Non-absolute (scratch/virtual) user key: relative imports are
		// genuinely unresolvable and must error, as they always have.
		"scratch:main": "var T = lib \"./nope\"\nfn Main() { return 1 }\n",
	}
	_, err := LoadMulti(context.Background(), sources, "scratch:main", "", nil)
	if err == nil {
		t.Fatal("expected an error for a relative import from a non-absolute user root")
	}
	if !strings.Contains(err.Error(), "not allowed") && !strings.Contains(err.Error(), "could not resolve") {
		t.Errorf("expected a relative-import error, got: %v", err)
	}
}
