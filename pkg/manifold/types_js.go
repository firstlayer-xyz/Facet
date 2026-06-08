//go:build js

package manifold

import (
	"fmt"
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
type Mesh struct {
	Vertices []float32
	Normals  []float32
	Indices  []uint32
}

// DisplayMesh holds mesh data as raw byte slices for efficient binary transfer.
type DisplayMesh struct {
	VertRaw        []byte
	IdxRaw         []byte
	FaceGroupRaw   []byte
	FaceColorMap   map[string]string
	VertexCount    int
	IndexCount     int
	FaceGroupCount int
	ExpandedRaw    []byte
	EdgeLinesRaw   []byte
	ExpandedCount  int
	EdgeCount      int
}

// FaceInfo, NoColor, and clamp01 live in face_color.go (no build tag) so
// both the native and wasm builds share the same definition.

// Point2D represents a 2D point for polygon construction.
type Point2D struct {
	X, Y float64
}

// Point3D represents a 3D point for hull and sweep construction.
type Point3D struct {
	X, Y, Z float64
}

// ExternalMemory reports the external (C/C++ heap) memory footprint in bytes.
type ExternalMemory interface {
	ExternalMemSize() int
}

// ExternalMemSize reports the approximate C++-side footprint registered with
// the Go GC. The wasm geometry lives in the separate facet_cxx module's linear
// memory, invisible to Go's heap accounting, so each object queries its size at
// creation and reports it via runtime.ExternalAlloc; that lets the GC pace
// collection against the real off-heap growth and free C++ memory promptly
// (otherwise sustained per-frame solid creation during animation playback never
// triggers a GC and native memory climbs without bound).
func (s *Solid) ExternalMemSize() int { return int(s.memSize) }

func (sk *Sketch) ExternalMemSize() int { return int(sk.memSize) }

func newSolid(id int) *Solid {
	s := &Solid{id: id}
	if id != 0 { // id 0 is a null handle (e.g. an empty boolean result)
		s.memSize = uint64(js.Global().Call("_mf_solid_size", id).Float())
		runtime.ExternalAlloc(s.memSize)
	}
	runtime.SetFinalizer(s, func(s *Solid) {
		runtime.ExternalFree(s.memSize)
		js.Global().Call("_mf_solid_free", s.id)
	})
	return s
}

func newSketch(id int) *Sketch {
	sk := &Sketch{id: id}
	if id != 0 {
		sk.memSize = uint64(js.Global().Call("_mf_sketch_size", id).Float())
		runtime.ExternalAlloc(sk.memSize)
	}
	runtime.SetFinalizer(sk, func(sk *Sketch) {
		runtime.ExternalFree(sk.memSize)
		js.Global().Call("_mf_sketch_free", sk.id)
	})
	return sk
}

func newSolidWithOrigin(id int) *Solid {
	s := newSolid(id)
	origID := uint32(js.Global().Call("_mf_original_id", id).Int())
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

// mergeFaceMaps returns a new map containing entries from both inputs.
func mergeFaceMaps(a, b map[uint32]FaceInfo) map[uint32]FaceInfo {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	m := make(map[uint32]FaceInfo, len(a)+len(b))
	for k, v := range b {
		m[k] = v
	}
	for k, v := range a {
		m[k] = v
	}
	return m
}

func (s *Solid) withFaceMap() map[uint32]FaceInfo {
	if len(s.FaceMap) == 0 {
		return nil
	}
	m := make(map[uint32]FaceInfo, len(s.FaceMap))
	for k, v := range s.FaceMap {
		m[k] = v
	}
	return m
}

// SetColor returns a copy of the solid colored uniformly. Non-destructive — see
// the native build's SetColor for why the receiver must not be mutated in place.
// An identity transform clones the geometry with a copied FaceMap, then colors it.
func (s *Solid) SetColor(r, g, b, a float64) *Solid {
	out := s.Translate(0, 0, 0)
	color := uint32(int(r*255+0.5)<<16 | int(g*255+0.5)<<8 | int(b*255+0.5))
	alpha := uint8(clamp01(a)*255 + 0.5)
	for id, fi := range out.FaceMap {
		fi.Color = color
		fi.Alpha = alpha
		out.FaceMap[id] = fi
	}
	return out
}

// transformSolid wraps the result of a unary solid transform, carrying over the FaceMap.
func transformSolid(s *Solid, id int) *Solid {
	r := newSolid(id)
	r.FaceMap = s.withFaceMap()
	return r
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

// colorFromFaceInfo converts a FaceInfo int32 color to a hex string.
func colorFromFaceInfo(fi FaceInfo) string {
	c := fi.Color
	return fmt.Sprintf("#%02X%02X%02X", (c>>16)&0xFF, (c>>8)&0xFF, c&0xFF)
}
