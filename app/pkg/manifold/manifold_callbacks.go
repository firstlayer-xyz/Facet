package manifold

/*
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"sync/atomic"
)

// ---------------------------------------------------------------------------
// Warp callback registry
// ---------------------------------------------------------------------------

// warpMu serializes both registry access and callback invocations, so the
// Go evaluator (which is not goroutine-safe) is never called concurrently
// from TBB's worker threads.
var (
	warpMu       sync.Mutex
	warpRegistry = make(map[int]func(x, y, z float64) (float64, float64, float64))
	warpNextID   atomic.Int32
)

func registerWarp(fn func(x, y, z float64) (float64, float64, float64)) int {
	id := int(warpNextID.Add(1))
	warpMu.Lock()
	warpRegistry[id] = fn
	warpMu.Unlock()
	return id
}

func unregisterWarp(id int) {
	warpMu.Lock()
	delete(warpRegistry, id)
	warpMu.Unlock()
}

// ---------------------------------------------------------------------------
// LevelSet callback registry
// ---------------------------------------------------------------------------

var (
	levelSetMu       sync.Mutex
	levelSetRegistry = make(map[int]func(x, y, z float64) float64)
	levelSetNextID   atomic.Int32
)

func registerLevelSet(fn func(x, y, z float64) float64) int {
	id := int(levelSetNextID.Add(1))
	levelSetMu.Lock()
	levelSetRegistry[id] = fn
	levelSetMu.Unlock()
	return id
}

func unregisterLevelSet(id int) {
	levelSetMu.Lock()
	delete(levelSetRegistry, id)
	levelSetMu.Unlock()
}

// ---------------------------------------------------------------------------
// Bridge functions exported to C (called from TBB threads via bindings.cpp)
// ---------------------------------------------------------------------------

// facetWarpBridge is invoked by facet_warp for each vertex.
// warpMu is held for the entire call to serialize evaluator access.
//
//export facetWarpBridge
func facetWarpBridge(id C.int, xp, yp, zp *C.double) {
	warpMu.Lock()
	defer warpMu.Unlock()
	fn := warpRegistry[int(id)]
	if fn == nil {
		return
	}
	nx, ny, nz := fn(float64(*xp), float64(*yp), float64(*zp))
	*xp = C.double(nx)
	*yp = C.double(ny)
	*zp = C.double(nz)
}

// facetLevelSetBridge is invoked by facet_level_set for each sample point.
// levelSetMu is held for the entire call to serialize evaluator access.
//
//export facetLevelSetBridge
func facetLevelSetBridge(id C.int, x, y, z C.double) C.double {
	levelSetMu.Lock()
	defer levelSetMu.Unlock()
	fn := levelSetRegistry[int(id)]
	if fn == nil {
		return 0
	}
	return C.double(fn(float64(x), float64(y), float64(z)))
}
