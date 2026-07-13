package loader_test

// This end-to-end test lives in an external package because it imports the
// checker, which imports loader — an internal (package loader) test could not.
// It guards the layer the loader-only test cannot see: LoadMulti returning no
// error is not enough, because a view-only library source echoed into the
// payload used to survive into prog.Sources and only fail later, when the
// checker walked it and flagged its unresolved relative import.

import (
	"context"
	"strings"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/loader"
)

// TestRoundTrippedLibrarySourceDoesNotFailChecker reproduces the full desktop
// symptom: a remote library open as a view-only tab is echoed back keyed by its
// virtual "git+..." backing. LoadMulti must drop it so the checker never sees
// its unresolved "../threads" import and blocks the render.
func TestRoundTrippedLibrarySourceDoesNotFailChecker(t *testing.T) {
	libKey := loader.LibSourceScheme + "github.com/firstlayer-xyz/facetlibs@77cec59/fasteners/fasteners.fct"
	sources := map[string]string{
		"/tmp/project/main.fct": "fn Main() Number { return 1 }\n",
		libKey:                  "var T = lib \"../threads\"\nfn Knurl() Number { return 1 }\n",
	}

	prog, err := loader.LoadMulti(context.Background(), sources, "/tmp/project/main.fct", "", nil)
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}

	res := checker.Check(prog)
	for _, e := range res.Errors {
		if strings.Contains(e.Message, "not resolved") || strings.Contains(e.Message, "../threads") {
			t.Fatalf("checker flagged the round-tripped library source: %s", e.Message)
		}
	}
}
