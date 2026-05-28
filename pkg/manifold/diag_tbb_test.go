//go:build !js

package manifold

import "testing"

// TestDiagTBBConcurrency prints what Manifold was built with and what TBB
// thinks the runtime concurrency is. Used to localize the Linux-vs-Windows
// gap surfaced by .github/workflows/profile-linux.yml.
//
// The test intentionally fails so its `t.Logf` output prints under plain
// `go test` (without `-v`). The values are the diagnostic; the failure
// signals "this build needs investigation, not a green badge."
// Remove after PR #14 collects the numbers.
func TestDiagTBBConcurrency(t *testing.T) {
	par := diagManifoldPar()
	conc := diagTBBDefaultConcurrency()

	parStr := "UNKNOWN"
	switch par {
	case 1:
		parStr = "PARALLEL_TBB"
	case -1:
		parStr = "SERIAL"
	}

	t.Logf("FACET_MANIFOLD_PAR=%d (%s)", par, parStr)
	t.Logf("tbb::info::default_concurrency()=%d", conc)
	t.Fail()
}
