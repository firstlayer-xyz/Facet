//go:build js

package manifold

import (
	"runtime"
	"syscall/js"
)

// Solid wraps a JS-side manifold object by integer ID.
type Solid struct {
	id      int
	memSize uint64 // C++ footprint registered with the GC via runtime.ExternalAlloc
	FaceMap map[uint32]FaceInfo
}

// Sketch wraps a JS-side CrossSection object by integer ID.
type Sketch struct {
	id      int
	memSize uint64
}

// Mesh holds extracted triangle mesh data with Go typed slices.
// FaceInfo, NoColor, and clamp01 live in face_color.go (no build tag) so
// both the native and wasm builds share the same definition.

// newSolid wraps a JS-side manifold handle. The wasm geometry lives in the
// separate facet_cxx module's linear memory, invisible to Go's heap accounting,
// so each object queries its size at creation and reports it via
// runtime.ExternalAlloc; that lets the GC pace collection against the real
// off-heap growth and free C++ memory promptly (otherwise sustained per-frame
// solid creation during animation playback never triggers a GC and native
// memory climbs without bound).
func newSolid(id int) *Solid {
	// id 0 is a null handle: every C++ op ends catch(...){ facetClear } which
	// nulls the pointer, and web/index.html reports that as id 0. Return nil like
	// native newSolid, so a failed op yields nil rather than a live wrapper whose
	// later js→C++ calls would dereference a null ManifoldPtr.
	if id == 0 {
		return nil
	}
	s := &Solid{id: id}
	s.memSize = uint64(js.Global().Call("_mf_solid_size", id).Float())
	runtime.ExternalAlloc(s.memSize)
	runtime.SetFinalizer(s, func(s *Solid) {
		runtime.ExternalFree(s.memSize)
		js.Global().Call("_mf_solid_free", s.id)
	})
	return s
}

func newSketch(id int) *Sketch {
	if id == 0 {
		return nil
	}
	sk := &Sketch{id: id}
	sk.memSize = uint64(js.Global().Call("_mf_sketch_size", id).Float())
	runtime.ExternalAlloc(sk.memSize)
	runtime.SetFinalizer(sk, func(sk *Sketch) {
		runtime.ExternalFree(sk.memSize)
		js.Global().Call("_mf_sketch_free", sk.id)
	})
	return sk
}

func newSolidWithOrigin(id int) *Solid {
	s := newSolid(id)
	if s == nil {
		return nil
	}
	// The nil check must precede _mf_original_id: on a null handle that would be a
	// js→C++ call on a null ManifoldPtr (an untrapped near-0 memory read).
	seedOriginFaceMap(s, js.Global().Call("_mf_original_id", id).Int())
	return s
}

// SetColor returns a copy of the solid colored uniformly. Non-destructive — see
// the native build's SetColor for why the receiver must not be mutated in place.
// An identity transform clones the geometry with a copied FaceMap, then colors it.
func (s *Solid) SetColor(r, g, b, a float64) *Solid {
	out := s.Translate(0, 0, 0)
	color, alpha := encodeColor(r, g, b, a)
	for id, fi := range out.FaceMap {
		fi.Color = color
		fi.Alpha = alpha
		out.FaceMap[id] = fi
	}
	return out
}

// seedHullResult wraps a kernel op whose FaceMap collapses to a single original,
// carrying face onto it. A null handle yields nil (like newSolidWithOrigin); the
// nil check must precede _mf_original_id so it is never called on a null handle.
func seedHullResult(id int, face FaceInfo) *Solid {
	r := newSolid(id)
	if r == nil {
		return nil
	}
	seedHullFaceMap(r, js.Global().Call("_mf_original_id", id).Int(), face)
	return r
}

// transformSolid wraps the result of a unary solid transform, carrying over the FaceMap.
func transformSolid(s *Solid, id int) *Solid {
	if id == 0 {
		return nil // kernel failed — nil, not a silently-empty solid (matches native)
	}
	r := newSolid(id)
	r.FaceMap = s.withFaceMap()
	return r
}

// solidIDArray marshals the solids' JS handles into a JS Array for bridge calls.
func solidIDArray(solids []*Solid) js.Value {
	arr := js.Global().Get("Array").New()
	for _, s := range solids {
		arr.Call("push", s.id)
	}
	return arr
}

// sketchIDArray marshals the sketches' JS handles into a JS Array for bridge calls.
func sketchIDArray(sketches []*Sketch) js.Value {
	arr := js.Global().Get("Array").New()
	for _, p := range sketches {
		arr.Call("push", p.id)
	}
	return arr
}

// typedArrayToBytes copies a JS TypedArray's bytes into a Go slice.
func typedArrayToBytes(val js.Value) []byte {
	buf := val.Get("buffer")
	byteOffset := val.Get("byteOffset").Int()
	byteLength := val.Get("byteLength").Int()
	if byteLength == 0 {
		return nil
	}
	u8 := js.Global().Get("Uint8Array").New(buf, byteOffset, byteLength)
	result := make([]byte, byteLength)
	js.CopyBytesToGo(result, u8)
	return result
}
