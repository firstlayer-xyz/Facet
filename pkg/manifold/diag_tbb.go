//go:build !js

package manifold

/*
#include "facet_cxx.h"
*/
import "C"

// diagManifoldPar reports how Manifold was compiled: 1 = parallel (TBB),
// -1 = serial, 0 = unknown. Used by TestDiagTBBConcurrency.
func diagManifoldPar() int { return int(C.facet_manifold_par()) }

// diagTBBDefaultConcurrency reports tbb::info::default_concurrency() — the
// number of logical threads TBB sees. -1 if the build linked the serial
// variant of Manifold (no TBB headers compiled in).
func diagTBBDefaultConcurrency() int { return int(C.facet_tbb_default_concurrency()) }
