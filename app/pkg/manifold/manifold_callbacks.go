package manifold

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
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
// The id is registered by Warp() before calling into C and unregistered
// only after C returns, so the bridge is guaranteed to find it.  A missing
// id indicates a programmer bug (use-after-unregister, stray C invocation,
// corrupted registry) — panic loudly rather than silently returning the
// input unchanged, which would produce subtly-wrong geometry.
//
//export facetWarpBridge
func facetWarpBridge(id C.int, xp, yp, zp *C.double) {
	warpMu.Lock()
	defer warpMu.Unlock()
	fn := lookupWarpLocked(int(id))
	nx, ny, nz := fn(float64(*xp), float64(*yp), float64(*zp))
	*xp = C.double(nx)
	*yp = C.double(ny)
	*zp = C.double(nz)
}

// lookupWarpLocked returns the registered warp callback or panics.
// Caller must hold warpMu.  Split out so tests can exercise the
// "unknown id" panic without building a cgo test file (which Go forbids
// when the package has //export directives).
func lookupWarpLocked(id int) func(x, y, z float64) (float64, float64, float64) {
	fn := warpRegistry[id]
	if fn == nil {
		panic(fmt.Sprintf("facetWarpBridge: no callback registered for id %d", id))
	}
	return fn
}

// facetLevelSetBridge is invoked by facet_level_set for each sample point.
// levelSetMu is held for the entire call to serialize evaluator access.
//
// As with facetWarpBridge, the id is guaranteed present — a nil entry is
// a programmer bug.  Returning 0 silently would flood the entire SDF
// sample grid with the zero iso-surface, producing nonsense geometry
// with no error surface.  Panic instead.
//
//export facetLevelSetBridge
func facetLevelSetBridge(id C.int, x, y, z C.double) C.double {
	levelSetMu.Lock()
	defer levelSetMu.Unlock()
	fn := lookupLevelSetLocked(int(id))
	return C.double(fn(float64(x), float64(y), float64(z)))
}

// lookupLevelSetLocked: see lookupWarpLocked.
func lookupLevelSetLocked(id int) func(x, y, z float64) float64 {
	fn := levelSetRegistry[id]
	if fn == nil {
		panic(fmt.Sprintf("facetLevelSetBridge: no callback registered for id %d", id))
	}
	return fn
}
