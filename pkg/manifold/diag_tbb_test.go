//go:build !js

package manifold

import "testing"

// TestDiagTBBConcurrency prints what Manifold was built with, what TBB
// sees at runtime, and — crucially — whether tbb::parallel_for actually
// spawns multiple worker threads on each platform. The Profile Linux vs
// Profile Windows comparison showed Windows running ~1 core during
// pkg/fctlang tests, but Go's pprof on Windows can't see TBB-owned
// threads, so the profile alone is ambiguous. These probes settle it.
//
// The test intentionally fails so its `t.Logf` output prints under plain
// `go test` (without `-v`). Remove after PR #14 collects the numbers.
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

	// A reasonably sized parallel_for: large enough that TBB should
	// chunk it across cores, small enough that the test stays under
	// a couple hundred ms on a single core.
	const n = 2_000_000

	threads := diagTBBParallelForThreads(n)
	t.Logf("tbb::parallel_for unique worker threads (n=%d): %d", n, threads)

	// Run the timed probe twice — first call may include thread-pool
	// init cost, second is steady-state.
	first := diagTBBParallelForMicros(n)
	second := diagTBBParallelForMicros(n)
	t.Logf("tbb::parallel_for wall time (n=%d): first=%.0fus second=%.0fus",
		n, first, second)

	t.Fail()
}
