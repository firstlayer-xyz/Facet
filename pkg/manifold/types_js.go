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
	FaceMap map[uint32]FaceInfo
}

// Sketch wraps a JS-side CrossSection object by integer ID.
type Sketch struct {
	id int
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

// ExternalMemSize returns 0 in WASM mode (memory managed by JS GC).
func (s *Solid) ExternalMemSize() int { return 0 }

// ExternalMemSize returns 0 in WASM mode.
func (sk *Sketch) ExternalMemSize() int { return 0 }

func newSolid(id int) *Solid {
	s := &Solid{id: id}
	runtime.SetFinalizer(s, func(s *Solid) {
		js.Global().Call("_mf_solid_free", s.id)
	})
	return s
}

func newSketch(id int) *Sketch {
	sk := &Sketch{id: id}
	runtime.SetFinalizer(sk, func(sk *Sketch) {
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

// SetColor sets a uniform RGBA color on all faces. Alpha 1 is fully opaque.
func (s *Solid) SetColor(r, g, b, a float64) *Solid {
	color := uint32(int(r*255+0.5)<<16 | int(g*255+0.5)<<8 | int(b*255+0.5))
	alpha := uint8(clamp01(a)*255 + 0.5)
	for id, fi := range s.FaceMap {
		fi.Color = color
		fi.Alpha = alpha
		s.FaceMap[id] = fi
	}
	return s
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
