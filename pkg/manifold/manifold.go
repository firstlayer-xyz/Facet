//go:build !js

// Package manifold exposes a Go-friendly facade over the Manifold C++ library
// for 2D sketches and 3D solids. The C surface is reached through the small
// C glue in cxx/include/facet_cxx.h.
//
// This file holds the core types (Solid, Sketch, Mesh, DisplayMesh, FaceInfo,
// Point2D, Point3D), the private constructors that register finalizers, and
// low-level plumbing shared by every operation (face map helpers, the
// transformSolid wrapper, external-memory accounting). Operations live in
// sibling files organized by concern:
//
//	primitives.go   — CreateCube/Sphere/Cylinder/Square/Circle/Polygon
//	booleans.go     — Union/Difference/Intersection/Insert/DecomposeSolid
//	extrusions.go   — Extrude/Revolve/Sweep/Slice/Project/Loft
//	transforms.go   — Translate/Rotate/Scale/Mirror for Solid and Sketch
//	operations.go   — Hull/Trim/Smooth/Refine/Split/Compose/Offset
//	queries.go      — BoundingBox/Volume/SurfaceArea/Area/Genus/MinGap/…
//	mesh_extract.go — Mesh and DisplayMesh extraction/merging
//
// The cgo #cgo CFLAGS directive below applies package-wide; sibling files
// only need the #include preamble for their own C references.
package manifold

/*
#cgo CFLAGS: -I${SRCDIR}/cxx/include
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
)

// newSolid wraps a FacetSolidRet returned by C and registers a finalizer
// to free the underlying Manifold. Every Solid creator goes through here
// so external-memory accounting is consistent.
func newSolid(ret C.FacetSolidRet) *Solid {
	if ret.ptr == nil {
		return nil
	}
	sz := uint64(ret.size)
	s := &Solid{ptr: ret.ptr, memSize: sz}
	runtime.ExternalAlloc(sz)
	runtime.SetFinalizer(s, func(s *Solid) {
		runtime.ExternalFree(s.memSize)
		C.facet_delete_solid(s.ptr)
	})
	return s
}

// newSketch wraps a FacetSketchRet returned by C and registers a finalizer.
func newSketch(ret C.FacetSketchRet) *Sketch {
	if ret.ptr == nil {
		return nil
	}
	sz := uint64(ret.size)
	sk := &Sketch{ptr: ret.ptr, memSize: sz}
	runtime.ExternalAlloc(sz)
	runtime.SetFinalizer(sk, func(sk *Sketch) {
		runtime.ExternalFree(sk.memSize)
		C.facet_delete_sketch(sk.ptr)
	})
	return sk
}

// Mesh holds extracted triangle mesh data with Go typed slices.
// Used by the language runtime and tests for programmatic access.
type Mesh struct {
	Vertices []float32 // flat xyz positions
	Normals  []float32 // flat xyz normals (per-vertex)
	Indices  []uint32  // triangle indices
}

// DisplayMesh holds mesh data as raw byte slices for efficient binary transfer.
// Created by extracting directly from C buffers without intermediate Go typed slices.
type DisplayMesh struct {
	VertRaw        []byte            // float32 LE vertex positions (xyz, 12 bytes per vertex)
	IdxRaw         []byte            // uint32 LE triangle indices (4 bytes each)
	FaceGroupRaw   []byte            // uint32 LE per-triangle face group IDs (optional)
	FaceColorMap   map[string]string // faceGroupID → hex color (optional)
	VertexCount    int
	IndexCount     int
	FaceGroupCount int // number of face group entries (= IndexCount/3)

	// Expanded format: pre-expanded non-indexed vertices + edge lines (computed on C++ side)
	ExpandedRaw   []byte // float32 LE non-indexed positions (3 floats * 3 verts * numTri)
	EdgeLinesRaw  []byte // float32 LE edge line segments (6 floats per edge)
	ExpandedCount int    // number of expanded vertices (= numTri * 3)
	EdgeCount     int    // number of edge line segments
}

// FaceInfo, NoColor, and clamp01 live in face_color.go (no build tag) so
// both the native and wasm builds share the same definition.

// Solid wraps a C ManifoldPtr pointer for use in boolean operations.
type Solid struct {
	ptr     *C.ManifoldPtr
	memSize uint64              // cached ExternalMemSize, set once at creation
	FaceMap map[uint32]FaceInfo // originalID → face metadata (nil if empty)
}

// Sketch wraps a C ManifoldCrossSection pointer for 2D shapes.
type Sketch struct {
	ptr     *C.ManifoldCrossSection
	memSize uint64 // cached ExternalMemSize, set once at creation
}

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

// ExternalMemSize returns the approximate C++ heap memory used by this Solid.
func (s *Solid) ExternalMemSize() int {
	return int(s.memSize)
}

// ExternalMemSize returns the approximate C++ heap memory used by this Sketch.
func (sk *Sketch) ExternalMemSize() int {
	return int(sk.memSize)
}

// mergeFaceMaps returns a new map containing entries from both inputs.
// On a duplicate key, a's full FaceInfo wins (struct overwrite — not a
// per-field merge).
func mergeFaceMaps(a, b map[uint32]FaceInfo) map[uint32]FaceInfo {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a) == 0 {
		m := make(map[uint32]FaceInfo, len(b))
		for k, v := range b {
			m[k] = v
		}
		return m
	}
	if len(b) == 0 {
		m := make(map[uint32]FaceInfo, len(a))
		for k, v := range a {
			m[k] = v
		}
		return m
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

// requireSolids panics if any argument is nil. A nil Solid reaching a boolean
// or hull op is a caller bug (forgot to check an earlier error return, passed
// a slice with a nil entry); a clear panic at the entry point beats a
// nil-deref deep inside the cgo wrapper or, worse, a SIGSEGV in C++.
func requireSolids(op string, solids ...*Solid) {
	for i, s := range solids {
		if s == nil {
			panic(fmt.Sprintf("manifold.%s: solid argument %d is nil", op, i))
		}
	}
}

// requireSketches is requireSolids for *Sketch operands — a nil Sketch reaching
// a 2D boolean is a caller bug; panic with a clear message rather than letting
// the nil `.ptr` SIGSEGV inside the cgo wrapper.
func requireSketches(op string, sketches ...*Sketch) {
	for i, sk := range sketches {
		if sk == nil {
			panic(fmt.Sprintf("manifold.%s: sketch argument %d is nil", op, i))
		}
	}
}

// withFaceMap returns a copy of the Solid's FaceMap (or nil).
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

// SetColor returns a copy of the solid with a uniform RGBA color on all faces.
// Alpha 1 is fully opaque. Each channel is clamped to [0, 1] before quantization
// so out-of-range inputs can't wrap into a wrong color through int conversion +
// bit shifts. Face IDs are auto-assigned by C at creation time; no new IDs are
// created here.
//
// Non-destructive: a *Solid is shared by language-level assignment (copyValue
// copies structs, not Solids), so coloring must not mutate the receiver's
// FaceMap in place. An identity transform yields a fresh Manifold with a copied
// FaceMap, which is then colored — leaving the receiver untouched, like every
// other Solid op.
func (s *Solid) SetColor(r, g, b, a float64) *Solid {
	out := s.Translate(0, 0, 0)
	ri := uint32(clamp01(r)*255 + 0.5)
	gi := uint32(clamp01(g)*255 + 0.5)
	bi := uint32(clamp01(b)*255 + 0.5)
	color := ri<<16 | gi<<8 | bi
	alpha := uint8(clamp01(a)*255 + 0.5)
	for id, fi := range out.FaceMap {
		fi.Color = color
		fi.Alpha = alpha
		out.FaceMap[id] = fi
	}
	return out
}

// newSolidWithOrigin wraps a FacetSolidRet and seeds a single-entry FaceMap
// from its original_id. The original_id is populated by the same cgo call
// that produced the pointer — no extra crossing.
func newSolidWithOrigin(ret C.FacetSolidRet) *Solid {
	s := newSolid(ret)
	if s == nil {
		return nil
	}
	if ret.original_id >= 0 {
		s.FaceMap = map[uint32]FaceInfo{uint32(ret.original_id): {Color: NoColor}}
	}
	return s
}

// transformSolid wraps a unary C transform that produced a new Manifold.
// Wraps the result and copies the FaceMap so face colors survive the
// transform. Callers must runtime.KeepAlive(s) immediately after the C call
// that produced ret — keeping it alive only here, after the C call has
// returned, would not satisfy the cgo pointer-safety contract.
func transformSolid(s *Solid, ret C.FacetSolidRet) *Solid {
	r := newSolid(ret)
	if r == nil {
		return nil
	}
	r.FaceMap = s.withFaceMap()
	return r
}
