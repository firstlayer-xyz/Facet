//go:build !js

package manifold

/*
#include "facet_cxx.h"
*/
import "C"

// diagManifoldPar reports how Manifold was compiled: 1 = parallel (TBB),
// -1 = serial, 0 = unknown.
func diagManifoldPar() int { return int(C.facet_manifold_par()) }

// diagTBBDefaultConcurrency reports tbb::info::default_concurrency().
// -1 if the build linked the serial Manifold variant.
func diagTBBDefaultConcurrency() int { return int(C.facet_tbb_default_concurrency()) }

// diagTBBParallelForMicros runs a fixed CPU-bound tbb::parallel_for over
// [0, n) and returns the wall time in microseconds. Used to verify
// whether parallel_for actually scales across cores.
func diagTBBParallelForMicros(n int) float64 {
	return float64(C.facet_tbb_parallel_for_us(C.int(n)))
}

// diagTBBParallelForThreads counts unique OS thread IDs that handled at
// least one iteration of a tbb::parallel_for over [0, n). Greater than 1
// means TBB actually spawned workers; equal to 1 means it ran serial
// regardless of what default_concurrency reports.
func diagTBBParallelForThreads(n int) int {
	return int(C.facet_tbb_parallel_for_threads(C.int(n)))
}
