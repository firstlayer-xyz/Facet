//go:build js

package manifold

import (
	"fmt"
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
	// warpPanic stashes a panic from the per-vertex callback (guarded by warpMu).
	// A Go panic must not unwind through the C++ Warp call — that crosses the FFI
	// boundary and is undefined in wasm — so the dispatcher recovers, records the
	// panic here, and returns the vertex unchanged; Warp re-raises it on the Go
	// side after the C++ call returns cleanly. Mirrors the native barrier.
	warpPanic any
)

// takeWarpPanic returns and clears any stashed callback panic.
func takeWarpPanic() any {
	warpMu.Lock()
	defer warpMu.Unlock()
	p := warpPanic
	warpPanic = nil
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

// init exposes the dispatcher to JS so the EM_JS-imported facetWarpBridge
// (in bindings.cpp under FACET_WASM) can route per-vertex callbacks back
// into Go.
func init() {
	js.Global().Set("_facetWarpDispatch", js.FuncOf(func(this js.Value, args []js.Value) (ret interface{}) {
		id := args[0].Int()
		x := args[1].Float()
		y := args[2].Float()
		z := args[3].Float()
		// Stash any panic and return the vertex unchanged so it never unwinds
		// through the C++ Warp call; Warp re-raises it afterward.
		defer func() {
			if r := recover(); r != nil {
				warpMu.Lock()
				warpPanic = r
				warpMu.Unlock()
				ret = []interface{}{x, y, z}
			}
		}()
		warpMu.Lock()
		fn := warpRegistry[id]
		warpMu.Unlock()
		if fn == nil {
			// The cxx side referenced an id past unregisterWarp — an impossible
			// state. Returning the vertex unchanged would silently warp the model
			// with a hole; fail loud so the lifecycle bug surfaces.
			panic(fmt.Sprintf("warp callback %d not registered", id))
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
	if p := takeWarpPanic(); p != nil {
		panic(p) // re-raise a callback panic on the Go side, off the C++ stack
	}
	r := newSolid(newID)
	if r == nil {
		return nil
	}
	r.FaceMap = s.withFaceMap()
	return r
}
