//go:build !js

package manifold

import "testing"

// TestDiagTBBConcurrency prints what Manifold was built with and what TBB
// thinks the runtime concurrency is. Used to localize the Linux-vs-Windows
// gap surfaced by .github/workflows/profile-linux.yml — Linux runs ~2.6
// cores, Windows runs ~1, which suggests TBB parallelism isn't active on
// Windows. The test always passes; its output is the artifact.
func TestDiagTBBConcurrency(t *testing.T) {
	par := diagManifoldPar()
	conc := diagTBBDefaultConcurrency()

	parStr := "UNKNOWN"
	switch par {
	case 1:
		parStr = "PARALLEL (TBB)"
	case -1:
		parStr = "SERIAL"
	}

	t.Logf("FACET_MANIFOLD_PAR=%d (%s)", par, parStr)
	t.Logf("tbb::info::default_concurrency()=%d", conc)
}
