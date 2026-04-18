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
//   primitives.go   — CreateCube/Sphere/Cylinder/Square/Circle/Polygon
//   booleans.go     — Union/Difference/Intersection/Insert/DecomposeSolid
//   extrusions.go   — Extrude/Revolve/Sweep/Slice/Project/Loft
//   transforms.go   — Translate/Rotate/Scale/Mirror for Solid and Sketch
//   operations.go   — Hull/Trim/Smooth/Refine/Split/Compose/Offset
//   queries.go      — BoundingBox/Volume/SurfaceArea/Area/Genus/MinGap/…
//   mesh_extract.go — Mesh and DisplayMesh extraction/merging
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
	"runtime"
)

// newSolid wraps a C ManifoldPtr pointer and registers a finalizer to free it.
func newSolid(ptr *C.ManifoldPtr) *Solid {
	if ptr == nil {
		return nil
	}
	sz := uint64(C.facet_solid_memory_size(ptr))
	s := &Solid{ptr: ptr, memSize: sz}
	runtime.ExternalAlloc(sz)
	runtime.SetFinalizer(s, func(s *Solid) {
		runtime.ExternalFree(s.memSize)
		C.facet_delete_solid(s.ptr)
	})
	return s
}

// newSketch wraps a C ManifoldCrossSection pointer and registers a finalizer to free it.
func newSketch(ptr *C.ManifoldCrossSection) *Sketch {
	sz := uint64(C.facet_sketch_memory_size(ptr))
	sk := &Sketch{ptr: ptr, memSize: sz}
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

// NoColor is the sentinel value for FaceInfo.Color indicating no color is assigned.
const NoColor uint32 = 0xFFFFFFFF

// FaceInfo holds per-face metadata keyed by Manifold originalID.
// Color is 0xRRGGBB; NoColor (0xFFFFFFFF) means no color assigned.
type FaceInfo struct {
	Color uint32
}

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
// If both maps have the same key, a's value wins (with per-field merge:
// color from a wins if set, source from a wins if set).
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

// SetColor sets a uniform RGB color on all vertices.
// Face IDs are auto-assigned by C at creation time; no new IDs are created here.
func (s *Solid) SetColor(r, g, b float64) *Solid {
	color := uint32(int(r*255+0.5)<<16 | int(g*255+0.5)<<8 | int(b*255+0.5))
	for id, fi := range s.FaceMap {
		fi.Color = color
		s.FaceMap[id] = fi
	}
	return s
}

// newSolidWithOrigin wraps a C ManifoldPtr pointer, registers a finalizer,
// and initializes a single-entry FaceMap using the solid's originalID.
func newSolidWithOrigin(ptr *C.ManifoldPtr) *Solid {
	if ptr == nil {
		return nil
	}
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

// transformSolid wraps a unary C transform that produces a new ManifoldPtr.
// The caller passes the already-evaluated C result pointer; this function keeps
// the source solid alive, wraps the result, and copies the FaceMap.
func transformSolid(s *Solid, ptr *C.ManifoldPtr) *Solid {
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}
