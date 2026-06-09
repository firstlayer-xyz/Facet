//go:build !js

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
	// warpPanic holds a panic that escaped a facetWarpBridge invocation, to be
	// re-raised by Warp once facet_warp has returned. A panic must NOT unwind
	// through the C++ frames that called the bridge (undefined behavior). Guarded
	// by warpMu; callbacks are serialized, so a single slot suffices.
	warpPanic any
)

// takeWarpPanic returns and clears any stashed bridge panic.
func takeWarpPanic() any {
	warpMu.Lock()
	p := warpPanic
	warpPanic = nil
	warpMu.Unlock()
	return p
}

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
	// levelSetPanic: see warpPanic.
	levelSetPanic any
)

// takeLevelSetPanic returns and clears any stashed bridge panic.
func takeLevelSetPanic() any {
	levelSetMu.Lock()
	p := levelSetPanic
	levelSetPanic = nil
	levelSetMu.Unlock()
	return p
}

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
// id (or a panic from the callback) indicates a programmer bug; we still want
// to fail loudly rather than silently produce subtly-wrong geometry, but a Go
// panic must not unwind through the C++ frames that called us (undefined
// behavior). So we recover here, stash the panic, and leave this vertex
// unchanged; Warp() re-raises it on the Go side after facet_warp returns.
//
//export facetWarpBridge
func facetWarpBridge(id C.int, xp, yp, zp *C.double) {
	warpMu.Lock()
	defer warpMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			warpPanic = r
		}
	}()
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
// As with facetWarpBridge, a missing id or a callback panic is a programmer
// bug we want surfaced — but a panic cannot unwind through the C++ caller.
// So we recover, stash it, return 0 for this one sample, and let LevelSet()
// re-raise it on the Go side after facet_level_set returns (the result is
// discarded, so the transient zero sample never reaches geometry).
//
//export facetLevelSetBridge
func facetLevelSetBridge(id C.int, x, y, z C.double) (result C.double) {
	levelSetMu.Lock()
	defer levelSetMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			levelSetPanic = r
			result = 0
		}
	}()
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
