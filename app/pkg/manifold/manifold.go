package manifold

/*
#cgo CFLAGS: -I${SRCDIR}/cxx/include
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"runtime"
	"unsafe"
)

// newSolid wraps a C ManifoldManifold pointer and registers a finalizer to free it.
func newSolid(ptr *C.ManifoldManifold) *Solid {
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

// DisplayMesh holds mesh data pre-encoded as base64 for efficient transfer to the frontend.
// Created by extracting directly from C buffers without intermediate Go typed slices.
type DisplayMesh struct {
	vertB64        string
	idxB64         string
	faceGroupB64   string            // per-triangle face group IDs (optional)
	faceColorMap   map[string]string // faceGroupID → hex color (optional)
	VertexCount    int
	IndexCount     int
	FaceGroupCount int // number of face group entries (= IndexCount/3)
}

// MarshalJSON serializes a DisplayMesh for the frontend.
func (m *DisplayMesh) MarshalJSON() ([]byte, error) {
	type wireFormat struct {
		Vertices    string            `json:"vertices"`
		Indices     string            `json:"indices"`
		FaceGroups  string            `json:"faceGroups,omitempty"`
		FaceColors  map[string]string `json:"faceColors,omitempty"`
		VertexCount int               `json:"vertexCount"`
		IndexCount  int               `json:"indexCount"`
	}
	return json.Marshal(wireFormat{
		Vertices:    m.vertB64,
		Indices:     m.idxB64,
		FaceGroups:  m.faceGroupB64,
		FaceColors:  m.faceColorMap,
		VertexCount: m.VertexCount,
		IndexCount:  m.IndexCount,
	})
}

// NoColor is the sentinel value for FaceInfo.Color indicating no color is assigned.
const NoColor uint32 = 0xFFFFFFFF

// FaceInfo holds per-face metadata keyed by Manifold originalID.
// Color is 0xRRGGBB; NoColor (0xFFFFFFFF) means no color assigned.
type FaceInfo struct {
	Color uint32
}

// Solid wraps a C ManifoldManifold pointer for use in boolean operations.
type Solid struct {
	ptr     *C.ManifoldManifold
	memSize uint64               // cached ExternalMemSize, set once at creation
	FaceMap map[uint32]FaceInfo  // originalID → face metadata (nil if empty)
}

// Sketch wraps a C ManifoldCrossSection pointer for 2D shapes.
type Sketch struct {
	ptr     *C.ManifoldCrossSection
	memSize uint64 // cached ExternalMemSize, set once at creation
}

// ToMesh extracts the triangle mesh from a Solid.
func (s *Solid) ToMesh() *Mesh {
	m := extractMesh(s.ptr)
	runtime.KeepAlive(s)
	return m
}

// ToMesh converts a Sketch to a renderable mesh via thin extrusion.
func (p *Sketch) ToMesh() *Mesh {
	solid := extrude(p, 0.001, 0, 0, 1, 1)
	m := extractMesh(solid.ptr)
	runtime.KeepAlive(solid)
	return m
}

// ToDisplayMesh extracts a DisplayMesh from a Solid, encoding directly from C memory.
func (s *Solid) ToDisplayMesh() *DisplayMesh {
	m := extractDisplayMesh(s.ptr, s.FaceMap)
	runtime.KeepAlive(s)
	return m
}

// ToDisplayMesh converts a Sketch to a DisplayMesh via thin extrusion.
func (p *Sketch) ToDisplayMesh() *DisplayMesh {
	solid := extrude(p, 0.001, 0, 0, 1, 1)
	m := extractDisplayMesh(solid.ptr, nil)
	runtime.KeepAlive(solid)
	return m
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

// setColor sets the color on all existing FaceMap entries.
// Face IDs are auto-assigned by C at creation time; no new IDs are created here.
func setColor(s *Solid, r, g, b float64) *Solid {
	color := uint32(int(r*255+0.5)<<16 | int(g*255+0.5)<<8 | int(b*255+0.5))
	for id, fi := range s.FaceMap {
		fi.Color = color
		s.FaceMap[id] = fi
	}
	return s
}

// ---------------------------------------------------------------------------
// 3D Primitives (unexported sync helpers)
// ---------------------------------------------------------------------------

func createCube(x, y, z float64) *Solid {
	ptr := C.facet_cube(C.double(x), C.double(y), C.double(z))
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func createSphere(radius float64, segments int) *Solid {
	ptr := C.facet_sphere(C.double(radius), C.int(segments))
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func createCylinder(height, radiusLow, radiusHigh float64, segments int) *Solid {
	ptr := C.facet_cylinder(C.double(height), C.double(radiusLow), C.double(radiusHigh), C.int(segments))
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

// ---------------------------------------------------------------------------
// 2D Primitives (unexported sync helpers)
// ---------------------------------------------------------------------------

func createSquare(x, y float64) *Sketch {
	ptr := C.facet_square(C.double(x), C.double(y))
	return newSketch(ptr)
}

func createCircle(radius float64, segments int) *Sketch {
	ptr := C.facet_circle(C.double(radius), C.int(segments))
	return newSketch(ptr)
}

// Point2D represents a 2D point for polygon construction.
type Point2D struct {
	X, Y float64
}

// Point3D represents a 3D point for hull construction.
type Point3D struct {
	X, Y, Z float64
}

func createPolygon(points []Point2D) *Sketch {
	n := len(points)
	// Ensure CCW winding (positive signed area) so extrusion normals face +Z.
	// Shoelace formula: positive = CCW, negative = CW.
	var area2 float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area2 += points[i].X*points[j].Y - points[j].X*points[i].Y
	}
	if area2 < 0 {
		// CW winding — reverse to CCW
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}
	coords := make([]C.double, n*2)
	for i, p := range points {
		coords[i*2] = C.double(p.X)
		coords[i*2+1] = C.double(p.Y)
	}
	ptr := C.facet_polygon(&coords[0], C.size_t(n))
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 3D Boolean Operations (unexported sync helpers)
// ---------------------------------------------------------------------------

func union(a, b *Solid) *Solid {
	ptr := C.facet_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func difference(a, b *Solid) *Solid {
	ptr := C.facet_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func intersection(a, b *Solid) *Solid {
	ptr := C.facet_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func insert(a, b *Solid) *Solid {
	ptr := C.facet_insert(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func decomposeSolid(s *Solid) []*Solid {
	var outArr **C.ManifoldManifold
	n := int(C.facet_decompose(s.ptr, &outArr))
	runtime.KeepAlive(s)
	if n == 0 {
		return nil
	}
	cSlice := unsafe.Slice(outArr, n)
	result := make([]*Solid, n)
	for i, ptr := range cSlice {
		result[i] = newSolid(ptr)
		result[i].FaceMap = s.withFaceMap()
	}
	C.free(unsafe.Pointer(outArr))
	return result
}

// ---------------------------------------------------------------------------
// 2D Boolean Operations (unexported sync helpers)
// ---------------------------------------------------------------------------

func sketchUnion(a, b *Sketch) *Sketch {
	ptr := C.facet_cs_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

func sketchDifference(a, b *Sketch) *Sketch {
	ptr := C.facet_cs_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

func sketchIntersection(a, b *Sketch) *Sketch {
	ptr := C.facet_cs_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 2D → 3D (unexported sync helpers)
// ---------------------------------------------------------------------------

func extrude(p *Sketch, height float64, slices int, twist, scaleX, scaleY float64) *Solid {
	ptr := C.facet_extrude(p.ptr, C.double(height), C.int(slices), C.double(twist), C.double(scaleX), C.double(scaleY))
	runtime.KeepAlive(p)
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func revolve(p *Sketch, segments int, degrees float64) *Solid {
	ptr := C.facet_revolve(p.ptr, C.int(segments), C.double(degrees))
	runtime.KeepAlive(p)
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func sweep(p *Sketch, path []Point3D) *Solid {
	flat := make([]C.double, len(path)*3)
	for i, pt := range path {
		flat[i*3], flat[i*3+1], flat[i*3+2] = C.double(pt.X), C.double(pt.Y), C.double(pt.Z)
	}
	ptr := C.facet_sweep(p.ptr, &flat[0], C.size_t(len(path)))
	runtime.KeepAlive(p)
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

// ---------------------------------------------------------------------------
// 3D → 2D (unexported sync helpers)
// ---------------------------------------------------------------------------

func slice(s *Solid, height float64) *Sketch {
	ptr := C.facet_slice(s.ptr, C.double(height))
	runtime.KeepAlive(s)
	return newSketch(ptr)
}

func project(s *Solid) *Sketch {
	ptr := C.facet_project(s.ptr)
	runtime.KeepAlive(s)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 3D Transforms (unexported sync helpers)
// ---------------------------------------------------------------------------

func translate(s *Solid, x, y, z float64) *Solid {
	ptr := C.facet_translate(s.ptr, C.double(x), C.double(y), C.double(z))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func rotate(s *Solid, x, y, z float64) *Solid {
	ptr := C.facet_rotate(s.ptr, C.double(x), C.double(y), C.double(z))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func scale(s *Solid, x, y, z float64) *Solid {
	ptr := C.facet_scale(s.ptr, C.double(x), C.double(y), C.double(z))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func mirror(s *Solid, nx, ny, nz float64) *Solid {
	ptr := C.facet_mirror(s.ptr, C.double(nx), C.double(ny), C.double(nz))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// scaleLocal scales a solid pivoting at its bounding box min corner.
func scaleLocal(s *Solid, x, y, z float64) *Solid {
	ptr := C.facet_scale_local(s.ptr, C.double(x), C.double(y), C.double(z))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// rotateLocal rotates a solid around its bounding box center.
func rotateLocal(s *Solid, x, y, z float64) *Solid {
	ptr := C.facet_rotate_local(s.ptr, C.double(x), C.double(y), C.double(z))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// mirrorLocal mirrors a solid across a plane through its bounding box center.
func mirrorLocal(s *Solid, nx, ny, nz float64) *Solid {
	ptr := C.facet_mirror_local(s.ptr, C.double(nx), C.double(ny), C.double(nz))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// scaleAt scales a solid by (x, y, z) around pivot point (ox, oy, oz).
func scaleAt(s *Solid, x, y, z, ox, oy, oz float64) *Solid {
	return translate(scale(translate(s, -ox, -oy, -oz), x, y, z), ox, oy, oz)
}

// mirrorAt mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin. The normal is normalized internally.
func mirrorAt(s *Solid, nx, ny, nz, offset float64) *Solid {
	ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if ln == 0 {
		return s
	}
	dx, dy, dz := nx/ln*offset, ny/ln*offset, nz/ln*offset
	return translate(mirror(translate(s, -dx, -dy, -dz), nx, ny, nz), dx, dy, dz)
}

// sketchScaleAt scales a sketch by (x, y) around pivot point (px, py).
func sketchScaleAt(p *Sketch, x, y, px, py float64) *Sketch {
	return sketchTranslate(sketchScale(sketchTranslate(p, -px, -py), x, y), px, py)
}

// sketchMirrorAt mirrors a sketch across the axis (ax, ay) at signed distance
// offset from the world origin. The axis is normalized internally.
func sketchMirrorAt(p *Sketch, ax, ay, offset float64) *Sketch {
	ln := math.Sqrt(ax*ax + ay*ay)
	if ln == 0 {
		return p
	}
	dx, dy := ax/ln*offset, ay/ln*offset
	return sketchTranslate(sketchMirror(sketchTranslate(p, -dx, -dy), ax, ay), dx, dy)
}

// rotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func rotateAt(s *Solid, rx, ry, rz, ox, oy, oz float64) *Solid {
	ptr := C.facet_rotate_at(s.ptr, C.double(rx), C.double(ry), C.double(rz), C.double(ox), C.double(oy), C.double(oz))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// ---------------------------------------------------------------------------
// 2D Transforms (unexported sync helpers)
// ---------------------------------------------------------------------------

func sketchTranslate(p *Sketch, x, y float64) *Sketch {
	ptr := C.facet_cs_translate(p.ptr, C.double(x), C.double(y))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchRotate(p *Sketch, degrees float64) *Sketch {
	ptr := C.facet_cs_rotate(p.ptr, C.double(degrees))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchScale(p *Sketch, x, y float64) *Sketch {
	ptr := C.facet_cs_scale(p.ptr, C.double(x), C.double(y))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchMirror(p *Sketch, ax, ay float64) *Sketch {
	ptr := C.facet_cs_mirror(p.ptr, C.double(ax), C.double(ay))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// sketchRotateLocal rotates a sketch around its bounding box center.
func sketchRotateLocal(p *Sketch, degrees float64) *Sketch {
	ptr := C.facet_cs_rotate_local(p.ptr, C.double(degrees))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// sketchMirrorLocal mirrors a sketch across an axis through its bounding box center.
func sketchMirrorLocal(p *Sketch, ax, ay float64) *Sketch {
	ptr := C.facet_cs_mirror_local(p.ptr, C.double(ax), C.double(ay))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 3D Operations (unexported sync helpers)
// ---------------------------------------------------------------------------

func hull(s *Solid) *Solid {
	ptr := C.facet_hull(s.ptr)
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	origID := uint32(C.facet_original_id(r.ptr))
	runtime.KeepAlive(r)
	// Hull creates new geometry; carry over any color from the input.
	fi := FaceInfo{Color: NoColor}
	for _, v := range s.FaceMap {
		if v.Color != NoColor {
			fi.Color = v.Color
			break
		}
	}
	r.FaceMap = map[uint32]FaceInfo{origID: fi}
	return r
}

func batchHull(solids []*Solid) *Solid {
	ptrs := make([]*C.ManifoldManifold, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	ptr := C.facet_batch_hull(&ptrs[0], C.size_t(len(solids)))
	runtime.KeepAlive(solids)
	r := newSolid(ptr)
	origID := uint32(C.facet_original_id(r.ptr))
	runtime.KeepAlive(r)
	// Hull creates new geometry; carry over any color from inputs.
	fi := FaceInfo{Color: NoColor}
	for _, s := range solids {
		for _, v := range s.FaceMap {
			if v.Color != NoColor {
				fi.Color = v.Color
				break
			}
		}
		if fi.Color != NoColor {
			break
		}
	}
	r.FaceMap = map[uint32]FaceInfo{origID: fi}
	return r
}

func hullPoints(points []Point3D) *Solid {
	n := len(points)
	coords := make([]C.double, n*3)
	for i, p := range points {
		coords[i*3] = C.double(p.X)
		coords[i*3+1] = C.double(p.Y)
		coords[i*3+2] = C.double(p.Z)
	}
	ptr := C.facet_hull_points(&coords[0], C.size_t(n))
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func trimByPlane(s *Solid, nx, ny, nz, offset float64) *Solid {
	ptr := C.facet_trim_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func smoothOut(s *Solid, minSharpAngle, minSmoothness float64) *Solid {
	ptr := C.facet_smooth_out(s.ptr, C.double(minSharpAngle), C.double(minSmoothness))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func refine(s *Solid, n int) *Solid {
	ptr := C.facet_refine(s.ptr, C.int(n))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func simplify(s *Solid, tolerance float64) *Solid {
	ptr := C.facet_simplify(s.ptr, C.double(tolerance))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

func refineToLength(s *Solid, length float64) *Solid {
	ptr := C.facet_refine_to_length(s.ptr, C.double(length))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// Genus returns the topological genus of the solid (0 = sphere-like, 1 = torus-like, etc.).
func (s *Solid) Genus() int {
	result := C.facet_genus(s.ptr)
	runtime.KeepAlive(s)
	return int(result)
}

// MinGap returns the minimum distance between two solids, searching up to searchLength.
func (s *Solid) MinGap(other *Solid, searchLength float64) float64 {
	result := C.facet_min_gap(s.ptr, other.ptr, C.double(searchLength))
	runtime.KeepAlive(s)
	runtime.KeepAlive(other)
	return float64(result)
}

func splitSolid(m, cutter *Solid) (*Solid, *Solid) {
	pair := C.facet_split(m.ptr, cutter.ptr)
	runtime.KeepAlive(m)
	runtime.KeepAlive(cutter)
	fm := mergeFaceMaps(m.FaceMap, cutter.FaceMap)
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return first, second
}

func splitByPlane(s *Solid, nx, ny, nz, offset float64) (*Solid, *Solid) {
	pair := C.facet_split_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset))
	runtime.KeepAlive(s)
	fm := s.withFaceMap()
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return first, second
}

func composeSolids(solids []*Solid) *Solid {
	ptrs := make([]*C.ManifoldManifold, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	ptr := C.facet_compose((**C.ManifoldManifold)(unsafe.Pointer(&ptrs[0])), C.int(len(solids)))
	for _, s := range solids {
		runtime.KeepAlive(s)
	}
	r := newSolid(ptr)
	for _, s := range solids {
		r.FaceMap = mergeFaceMaps(r.FaceMap, s.FaceMap)
	}
	return r
}

// ---------------------------------------------------------------------------
// 2D Operations (unexported sync helpers)
// ---------------------------------------------------------------------------

func sketchHull(p *Sketch) *Sketch {
	ptr := C.facet_cs_hull(p.ptr)
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchBatchHull(sketches []*Sketch) *Sketch {
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, p := range sketches {
		ptrs[i] = p.ptr
	}
	ptr := C.facet_cs_batch_hull(&ptrs[0], C.size_t(len(sketches)))
	runtime.KeepAlive(sketches)
	return newSketch(ptr)
}

func loft(sketches []*Sketch, heights []float64) *Solid {
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, s := range sketches {
		ptrs[i] = s.ptr
	}
	hs := make([]C.double, len(heights))
	for i, h := range heights {
		hs[i] = C.double(h)
	}
	ptr := C.facet_loft(&ptrs[0], C.size_t(len(sketches)), &hs[0], C.size_t(len(heights)))
	runtime.KeepAlive(sketches)
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}

func sketchOffset(p *Sketch, delta float64, segments int) *Sketch {
	ptr := C.facet_cs_offset(p.ptr, C.double(delta), C.int(segments))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// Info / Measurement (methods on resolved types)
// ---------------------------------------------------------------------------

// BoundingBox returns the axis-aligned bounding box of a solid as
// (minX, minY, minZ, maxX, maxY, maxZ) in mm.
func (s *Solid) BoundingBox() (minX, minY, minZ, maxX, maxY, maxZ float64) {
	var cMinX, cMinY, cMinZ, cMaxX, cMaxY, cMaxZ C.double
	C.facet_bounding_box(s.ptr, &cMinX, &cMinY, &cMinZ, &cMaxX, &cMaxY, &cMaxZ)
	runtime.KeepAlive(s)
	return float64(cMinX), float64(cMinY), float64(cMinZ),
		float64(cMaxX), float64(cMaxY), float64(cMaxZ)
}

// Volume returns the volume of a solid in mm³.
func (s *Solid) Volume() float64 {
	v := float64(C.facet_volume(s.ptr))
	runtime.KeepAlive(s)
	return v
}

// SurfaceArea returns the surface area of a solid in mm².
func (s *Solid) SurfaceArea() float64 {
	v := float64(C.facet_surface_area(s.ptr))
	runtime.KeepAlive(s)
	return v
}

// NumComponents returns the number of disconnected pieces in a solid.
func (s *Solid) NumComponents() int {
	n := int(C.facet_num_components(s.ptr))
	runtime.KeepAlive(s)
	return n
}

// BoundingBox returns the 2D axis-aligned bounding box of a sketch as
// (minX, minY, maxX, maxY) in mm.
func (p *Sketch) BoundingBox() (minX, minY, maxX, maxY float64) {
	var cMinX, cMinY, cMaxX, cMaxY C.double
	C.facet_cs_bounds(p.ptr, &cMinX, &cMinY, &cMaxX, &cMaxY)
	runtime.KeepAlive(p)
	return float64(cMinX), float64(cMinY), float64(cMaxX), float64(cMaxY)
}

// Area returns the area of a sketch in mm².
func (p *Sketch) Area() float64 {
	v := float64(C.facet_cs_area(p.ptr))
	runtime.KeepAlive(p)
	return v
}

// ---------------------------------------------------------------------------
// Mesh Extraction
// ---------------------------------------------------------------------------

// ExtractMeshShared extracts a mesh with shared vertices from a Solid,
// suitable for querying geometry data (vertex positions + triangle indices).
func ExtractMeshShared(s *Solid) *Mesh {
	m := extractMesh(s.ptr)
	runtime.KeepAlive(s)
	return m
}

// MergeMeshes combines multiple meshes into one for viewport rendering.
func MergeMeshes(meshes []*Mesh) *Mesh {
	if len(meshes) == 1 {
		return meshes[0]
	}
	var totalVerts, totalIndices int
	for _, m := range meshes {
		totalVerts += len(m.Vertices)
		totalIndices += len(m.Indices)
	}
	merged := &Mesh{
		Vertices: make([]float32, 0, totalVerts),
		Indices:  make([]uint32, 0, totalIndices),
	}
	var vertOffset uint32
	for _, m := range meshes {
		merged.Vertices = append(merged.Vertices, m.Vertices...)
		for _, idx := range m.Indices {
			merged.Indices = append(merged.Indices, idx+vertOffset)
		}
		vertOffset += uint32(len(m.Vertices) / 3)
	}
	return merged
}

// extractMesh converts a ManifoldManifold into a Go Mesh with shared vertices and indices.
// Normals are omitted; the frontend uses flatShading (GPU-computed face normals).
func extractMesh(m *C.ManifoldManifold) *Mesh {
	var cVerts *C.float
	var cNumVerts C.int
	var cIndices *C.uint32_t
	var cNumTris C.int

	C.facet_extract_mesh(m, &cVerts, &cNumVerts, &cIndices, &cNumTris)

	numVert := int(cNumVerts)
	numTri := int(cNumTris)
	if numVert == 0 || numTri == 0 {
		return &Mesh{}
	}
	defer C.free(unsafe.Pointer(cVerts))
	defer C.free(unsafe.Pointer(cIndices))

	vertices := make([]float32, numVert*3)
	copy(vertices, unsafe.Slice((*float32)(unsafe.Pointer(cVerts)), numVert*3))

	indices := make([]uint32, numTri*3)
	copy(indices, unsafe.Slice((*uint32)(unsafe.Pointer(cIndices)), numTri*3))

	return &Mesh{
		Vertices: vertices,
		Indices:  indices,
	}
}

// colorFromFaceInfo converts a FaceInfo int32 color to a hex string.
func colorFromFaceInfo(fi FaceInfo) string {
	c := fi.Color
	return fmt.Sprintf("#%02X%02X%02X", (c>>16)&0xFF, (c>>8)&0xFF, c&0xFF)
}

// extractDisplayMesh base64-encodes mesh data directly from C buffers,
// skipping intermediate Go typed slices. faceMap is used to build
// faceGroupID → hex color and faceGroupID → source position lookups.
func extractDisplayMesh(m *C.ManifoldManifold, faceMap map[uint32]FaceInfo) *DisplayMesh {
	var cVerts *C.float
	var cNumVerts, cNumProp C.int
	var cIndices *C.uint32_t
	var cNumTris C.int
	var cFaceIDs *C.uint32_t
	var cNumFaceIDs C.int

	C.facet_extract_display_mesh(m, &cVerts, &cNumVerts, &cNumProp,
		&cIndices, &cNumTris, &cFaceIDs, &cNumFaceIDs)

	numVert := int(cNumVerts)
	numTri := int(cNumTris)
	numProp := int(cNumProp)

	if numVert == 0 || numTri == 0 {
		return &DisplayMesh{}
	}
	defer C.free(unsafe.Pointer(cVerts))
	defer C.free(unsafe.Pointer(cIndices))

	// Base64 encode vertex positions (xyz only)
	var vertB64 string
	if numProp == 3 {
		vertB64 = base64.StdEncoding.EncodeToString(
			unsafe.Slice((*byte)(unsafe.Pointer(cVerts)), numVert*3*4))
	} else {
		tmp := make([]byte, numVert*12)
		src := unsafe.Pointer(cVerts)
		stride := uintptr(numProp) * 4
		for i := 0; i < numVert; i++ {
			copy(tmp[i*12:], unsafe.Slice((*byte)(unsafe.Add(src, uintptr(i)*stride)), 12))
		}
		vertB64 = base64.StdEncoding.EncodeToString(tmp)
	}

	// Base64 encode indices directly from C buffer
	triLen := numTri * 3
	idxB64 := base64.StdEncoding.EncodeToString(
		unsafe.Slice((*byte)(unsafe.Pointer(cIndices)), triLen*4))

	// Extract face group IDs and build face color/source maps
	var fgB64 string
	var fgCount int
	var fcMap map[string]string
	nFaceIDs := int(cNumFaceIDs)
	if nFaceIDs > 0 {
		defer C.free(unsafe.Pointer(cFaceIDs))
		fgB64 = base64.StdEncoding.EncodeToString(
			unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), nFaceIDs*4))
		fgCount = nFaceIDs

		faceIDs := unsafe.Slice((*uint32)(unsafe.Pointer(cFaceIDs)), nFaceIDs)
		seen := make(map[uint32]bool)
		for _, fid := range faceIDs {
			if seen[fid] {
				continue
			}
			seen[fid] = true
			if fi, ok := faceMap[fid]; ok && fi.Color != NoColor {
				if fcMap == nil {
					fcMap = make(map[string]string)
				}
				fcMap[fmt.Sprintf("%d", fid)] = colorFromFaceInfo(fi)
			}
		}
	}

	return &DisplayMesh{
		vertB64:        vertB64,
		idxB64:         idxB64,
		faceGroupB64:   fgB64,
		faceColorMap:   fcMap,
		VertexCount:    numVert,
		IndexCount:     triLen,
		FaceGroupCount: fgCount,
	}
}

// MergeDisplayMeshes combines multiple display meshes into one.
// Deprecated: prefer MergeExtractDisplayMeshes when source Solids are available.
func MergeDisplayMeshes(meshes []*DisplayMesh) *DisplayMesh {
	if len(meshes) == 1 {
		return meshes[0]
	}
	totalVerts := 0
	totalIdx := 0
	for _, m := range meshes {
		totalVerts += m.VertexCount
		totalIdx += m.IndexCount
	}

	// Decode, merge, re-encode
	vertBuf := make([]byte, 0, totalVerts*12)
	idxBuf := make([]uint32, 0, totalIdx)
	var fgBuf []uint32
	hasFaceGroups := false
	var vertOffset uint32

	// Check if any mesh has face groups
	for _, m := range meshes {
		if m.faceGroupB64 != "" {
			hasFaceGroups = true
			break
		}
	}
	if hasFaceGroups {
		fgBuf = make([]uint32, 0, totalIdx/3)
	}

	for _, m := range meshes {
		vb, err := base64.StdEncoding.DecodeString(m.vertB64)
		if err != nil {
			log.Printf("MergeDisplayMeshes: skipping mesh with malformed vertex data: %v", err)
			continue
		}
		ib, err := base64.StdEncoding.DecodeString(m.idxB64)
		if err != nil {
			log.Printf("MergeDisplayMeshes: skipping mesh with malformed index data: %v", err)
			continue
		}
		vertBuf = append(vertBuf, vb...)
		n := len(ib) / 4
		if n > 0 {
			src := unsafe.Slice((*uint32)(unsafe.Pointer(&ib[0])), n)
			for _, idx := range src {
				idxBuf = append(idxBuf, idx+vertOffset)
			}
		}

		// Merge face groups (IDs are globally unique via AsOriginal, no offset needed)
		if hasFaceGroups {
			if m.faceGroupB64 != "" {
				fb, err := base64.StdEncoding.DecodeString(m.faceGroupB64)
				if err != nil {
					log.Printf("MergeDisplayMeshes: malformed face group data: %v", err)
				} else if fn := len(fb) / 4; fn > 0 {
					src := unsafe.Slice((*uint32)(unsafe.Pointer(&fb[0])), fn)
					fgBuf = append(fgBuf, src...)
				}
			} else {
				// No face groups in this mesh — assign zero (unknown)
				numTris := n / 3
				for i := 0; i < numTris; i++ {
					fgBuf = append(fgBuf, 0)
				}
			}
		}

		vertOffset += uint32(m.VertexCount)
	}

	var idxBytes []byte
	if len(idxBuf) > 0 {
		idxBytes = make([]byte, len(idxBuf)*4)
		copy(idxBytes, unsafe.Slice((*byte)(unsafe.Pointer(&idxBuf[0])), len(idxBuf)*4))
	}

	var fgB64 string
	var fgCount int
	if len(fgBuf) > 0 {
		fgBytes := make([]byte, len(fgBuf)*4)
		copy(fgBytes, unsafe.Slice((*byte)(unsafe.Pointer(&fgBuf[0])), len(fgBuf)*4))
		fgB64 = base64.StdEncoding.EncodeToString(fgBytes)
		fgCount = len(fgBuf)
	}

	return &DisplayMesh{
		vertB64:        base64.StdEncoding.EncodeToString(vertBuf),
		idxB64:         base64.StdEncoding.EncodeToString(idxBytes),
		faceGroupB64:   fgB64,
		VertexCount:    len(vertBuf) / 12, // 12 bytes per vertex (3 floats × 4 bytes)
		IndexCount:     len(idxBuf),
		FaceGroupCount: fgCount,
	}
}

// MergeExtractDisplayMeshes extracts and merges display meshes from multiple Solids
// in a single C++ call, avoiding per-solid extraction and decode/offset/re-encode overhead.
func MergeExtractDisplayMeshes(solids []*Solid) *DisplayMesh {
	if len(solids) == 0 {
		return &DisplayMesh{}
	}

	// Merge all FaceMaps from constituent solids
	var merged map[uint32]FaceInfo
	for _, s := range solids {
		merged = mergeFaceMaps(merged, s.FaceMap)
	}

	if len(solids) == 1 {
		return extractDisplayMesh(solids[0].ptr, merged)
	}

	ptrs := make([]*C.ManifoldManifold, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}

	var cVerts *C.float
	var cNumVerts, cNumProp C.int
	var cIndices *C.uint32_t
	var cNumTris C.int
	var cFaceIDs *C.uint32_t
	var cNumFaceIDs C.int

	C.facet_merge_extract_display_mesh(&ptrs[0], C.size_t(len(solids)),
		&cVerts, &cNumVerts, &cNumProp,
		&cIndices, &cNumTris, &cFaceIDs, &cNumFaceIDs)
	runtime.KeepAlive(solids)

	numVert := int(cNumVerts)
	numTri := int(cNumTris)

	if numVert == 0 || numTri == 0 {
		return &DisplayMesh{}
	}
	defer C.free(unsafe.Pointer(cVerts))
	defer C.free(unsafe.Pointer(cIndices))

	numProp := int(cNumProp)
	src := unsafe.Pointer(cVerts)
	stride := uintptr(numProp) * 4

	// Extract XYZ positions (first 3 floats per vertex)
	var vertB64 string
	if numProp == 3 {
		vertB64 = base64.StdEncoding.EncodeToString(
			unsafe.Slice((*byte)(src), numVert*3*4))
	} else {
		vertTmp := make([]byte, numVert*12)
		for i := 0; i < numVert; i++ {
			off := unsafe.Add(src, uintptr(i)*stride)
			copy(vertTmp[i*12:], unsafe.Slice((*byte)(off), 12))
		}
		vertB64 = base64.StdEncoding.EncodeToString(vertTmp)
	}

	triLen := numTri * 3
	idxB64 := base64.StdEncoding.EncodeToString(
		unsafe.Slice((*byte)(unsafe.Pointer(cIndices)), triLen*4))

	var fgB64 string
	var fgCount int
	var fcMap map[string]string
	nFaceIDs := int(cNumFaceIDs)
	if nFaceIDs > 0 {
		defer C.free(unsafe.Pointer(cFaceIDs))
		fgB64 = base64.StdEncoding.EncodeToString(
			unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), nFaceIDs*4))
		fgCount = nFaceIDs

		faceIDs := unsafe.Slice((*uint32)(unsafe.Pointer(cFaceIDs)), nFaceIDs)
		seen := make(map[uint32]bool)
		for _, fid := range faceIDs {
			if seen[fid] {
				continue
			}
			seen[fid] = true
			if fi, ok := merged[fid]; ok && fi.Color != NoColor {
				if fcMap == nil {
					fcMap = make(map[string]string)
				}
				fcMap[fmt.Sprintf("%d", fid)] = colorFromFaceInfo(fi)
			}
		}
	}

	return &DisplayMesh{
		vertB64:        vertB64,
		idxB64:         idxB64,
		faceGroupB64:   fgB64,
		faceColorMap:   fcMap,
		VertexCount:    numVert,
		IndexCount:     triLen,
		FaceGroupCount: fgCount,
	}
}

// BuildDisplayMesh creates a DisplayMesh from Go-typed arrays with optional face group IDs.
// verts is flat float32 xyz, indices is flat uint32 triangle indices,
// faceGroups (optional) is per-triangle face group IDs.
func BuildDisplayMesh(verts []float32, indices []uint32, faceGroups []uint32) *DisplayMesh {
	if len(verts) == 0 || len(indices) == 0 {
		return &DisplayMesh{}
	}

	numVert := len(verts) / 3

	// Encode vertices
	vertBytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	vertB64 := base64.StdEncoding.EncodeToString(vertBytes)

	// Encode indices
	idxBytes := unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), len(indices)*4)
	idxB64 := base64.StdEncoding.EncodeToString(idxBytes)

	// Encode face groups if present
	var fgB64 string
	var fgCount int
	if len(faceGroups) > 0 {
		fgBytes := unsafe.Slice((*byte)(unsafe.Pointer(&faceGroups[0])), len(faceGroups)*4)
		fgB64 = base64.StdEncoding.EncodeToString(fgBytes)
		fgCount = len(faceGroups)
	}

	return &DisplayMesh{
		vertB64:        vertB64,
		idxB64:         idxB64,
		faceGroupB64:   fgB64,
		VertexCount:    numVert,
		IndexCount:     len(indices),
		FaceGroupCount: fgCount,
	}
}

// ---------------------------------------------------------------------------
// Public Future Creator Functions
// ---------------------------------------------------------------------------

// CreateCube creates a box with the given dimensions.
func CreateCube(x, y, z float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		return createCube(x, y, z), nil
	})
}

// CreateSphere creates a sphere with the given radius and segment count.
func CreateSphere(radius float64, segments int) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		return createSphere(radius, segments), nil
	})
}

// CreateCylinder creates a cylinder (or cone if radii differ).
func CreateCylinder(height, radiusLow, radiusHigh float64, segments int) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		return createCylinder(height, radiusLow, radiusHigh, segments), nil
	})
}

// CreateSquare creates a 2D rectangle.
func CreateSquare(x, y float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		return createSquare(x, y), nil
	})
}

// CreateCircle creates a 2D circle.
func CreateCircle(radius float64, segments int) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		return createCircle(radius, segments), nil
	})
}

// CreatePolygon creates a 2D sketch from a slice of points.
func CreatePolygon(points []Point2D) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		if len(points) < 3 {
			return nil, fmt.Errorf("Polygon requires at least 3 points, got %d", len(points))
		}
		return createPolygon(points), nil
	})
}

// ---------------------------------------------------------------------------
// Public Future Batch Functions
// ---------------------------------------------------------------------------

// BatchHull computes the convex hull of multiple solid futures together.
func BatchHull(futures []*SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		if len(futures) == 0 {
			return nil, fmt.Errorf("BatchHull requires at least 1 solid")
		}
		solids := make([]*Solid, len(futures))
		for i, f := range futures {
			s, err := f.Resolve()
			if err != nil {
				return nil, err
			}
			solids[i] = s
		}
		return batchHull(solids), nil
	})
}

// SketchBatchHull computes the convex hull of multiple sketch futures together.
func SketchBatchHull(futures []*SketchFuture) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		if len(futures) == 0 {
			return nil, fmt.Errorf("SketchBatchHull requires at least 1 sketch")
		}
		sketches := make([]*Sketch, len(futures))
		for i, f := range futures {
			s, err := f.Resolve()
			if err != nil {
				return nil, err
			}
			sketches[i] = s
		}
		return sketchBatchHull(sketches), nil
	})
}

// Loft creates a solid by blending between cross-sections at different heights.
func Loft(futures []*SketchFuture, heights []float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		if len(futures) < 2 {
			return nil, fmt.Errorf("Loft requires at least 2 cross-sections, got %d", len(futures))
		}
		if len(heights) == 0 {
			return nil, fmt.Errorf("Loft requires at least 1 height")
		}
		sketches := make([]*Sketch, len(futures))
		for i, f := range futures {
			s, err := f.Resolve()
			if err != nil {
				return nil, err
			}
			sketches[i] = s
		}
		return loft(sketches, heights), nil
	})
}

// HullPoints computes the convex hull of a set of 3D points.
func HullPoints(points []Point3D) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		if len(points) == 0 {
			return nil, fmt.Errorf("HullPoints requires at least 1 point")
		}
		return hullPoints(points), nil
	})
}

// DecomposeSolid splits a solid into its disconnected connected components.
// The future is resolved eagerly because the count is not known until computed.
func DecomposeSolid(sf *SolidFuture) ([]*SolidFuture, error) {
	s, err := sf.Resolve()
	if err != nil {
		return nil, err
	}
	solids := decomposeSolid(s)
	futures := make([]*SolidFuture, len(solids))
	for i, sol := range solids {
		futures[i] = ImmediateSolid(sol)
	}
	return futures, nil
}
