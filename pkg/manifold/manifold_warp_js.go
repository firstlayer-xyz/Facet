//go:build js

package manifold

import (
	"sync"
	"sync/atomic"
	"syscall/js"
)

// Warp callback registry — keyed by int id so the C++ side (which can't
// hold Go pointers) can refer to a callback by integer.
//
// Manifold::Warp itself is sequential (ExecutionPolicy::Seq in
// third_party/manifold/src/impl.cpp), so we don't need to worry about
// concurrent dispatch from TBB workers — the lock here just guards
// register/unregister against re-entrant Warp calls.
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

// init exposes the dispatcher to JS so the EM_JS-imported facetWarpBridge
// (in bindings.cpp under FACET_WASM) can route per-vertex callbacks back
// into Go.
func init() {
	js.Global().Set("_facetWarpDispatch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		id := args[0].Int()
		x := args[1].Float()
		y := args[2].Float()
		z := args[3].Float()
		warpMu.Lock()
		fn := warpRegistry[id]
		warpMu.Unlock()
		if fn == nil {
			// Defensive — this would mean the cxx side held an id past unregister.
			return []interface{}{x, y, z}
		}
		nx, ny, nz := fn(x, y, z)
		return []interface{}{nx, ny, nz}
	}))
}

// Warp deforms each vertex of a solid using a per-vertex callback.
// Mirrors the native CGO signature in manifold_warp.go.
func (s *Solid) Warp(fn func(x, y, z float64) (float64, float64, float64)) *Solid {
	id := registerWarp(fn)
	defer unregisterWarp(id)
	newID := js.Global().Call("_mf_warp", s.id, id).Int()
	r := newSolid(newID)
	r.FaceMap = s.withFaceMap()
	return r
}
