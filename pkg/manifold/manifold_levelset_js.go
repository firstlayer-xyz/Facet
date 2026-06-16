//go:build js

package manifold

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall/js"
)

// LevelSet callback registry — keyed by int id so the C++ side (which can't
// hold Go pointers) can refer to a callback by integer. Mirrors the Warp
// callback registry in manifold_warp_js.go; the two are intentionally
// duplicated for clarity rather than shared via generics, since they
// operate on different signatures and the registry is small.
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

// init exposes the dispatcher to JS so the EM_JS-imported facetLevelSetBridge
// (in bindings.cpp under FACET_WASM) can route per-sample SDF evaluations
// back into Go. Like Warp, Manifold's LevelSet is sequential per-cell, so
// we don't need to worry about cross-thread JS calls.
func init() {
	js.Global().Set("_facetLevelSetDispatch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		id := args[0].Int()
		x := args[1].Float()
		y := args[2].Float()
		z := args[3].Float()
		levelSetMu.Lock()
		fn := levelSetRegistry[id]
		levelSetMu.Unlock()
		if fn == nil {
			// The cxx side referenced an id past unregisterLevelSet — an
			// impossible state. Returning 0 (a surface-crossing value) would
			// fabricate geometry; fail loud so the lifecycle bug surfaces.
			panic(fmt.Sprintf("level-set callback %d not registered", id))
		}
		return fn(x, y, z)
	}))
}

// LevelSet creates a solid from a signed-distance-field (SDF) callback.
// Points where sdf(p) <= 0 form the interior; the surface is at sdf(p) = 0.
// bounds defines the region to sample; edgeLen controls mesh resolution.
func LevelSet(fn func(x, y, z float64) float64, minX, minY, minZ, maxX, maxY, maxZ, edgeLen float64) *Solid {
	id := registerLevelSet(fn)
	defer unregisterLevelSet(id)
	newID := js.Global().Call("_mf_level_set", id, minX, minY, minZ, maxX, maxY, maxZ, edgeLen).Int()
	s := newSolid(newID)
	origID := uint32(js.Global().Call("_mf_original_id", newID).Int())
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}
