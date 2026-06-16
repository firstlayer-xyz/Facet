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

// ExternalMemSize returns the approximate C++ heap memory used by this Solid.
func (s *Solid) ExternalMemSize() int {
	return int(s.memSize)
}

// ExternalMemSize returns the approximate C++ heap memory used by this Sketch.
func (sk *Sketch) ExternalMemSize() int {
	return int(sk.memSize)
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
	color, alpha := encodeColor(r, g, b, a)
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
	seedOriginFaceMap(s, int(ret.original_id))
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
