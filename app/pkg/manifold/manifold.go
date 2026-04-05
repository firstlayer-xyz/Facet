package manifold

/*
#cgo CFLAGS: -I${SRCDIR}/cxx/include
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"math"
	"runtime"
	"unsafe"
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
	solid, err := p.Extrude(0.001, 0, 0, 1, 1)
	if err != nil {
		return nil
	}
	m := extractMesh(solid.ptr)
	runtime.KeepAlive(solid)
	return m
}

// ToDisplayMesh extracts a DisplayMesh from a Solid with expanded positions and edge lines.
func (s *Solid) ToDisplayMesh() *DisplayMesh {
	m := extractDisplayMesh(s.ptr, s.FaceMap)
	appendExpandedData(m, s.ptr, 40)
	runtime.KeepAlive(s)
	return m
}

// ToDisplayMesh converts a Sketch to a DisplayMesh via thin extrusion.
func (p *Sketch) ToDisplayMesh() *DisplayMesh {
	solid, err := p.Extrude(0.001, 0, 0, 1, 1)
	if err != nil {
		return nil
	}
	m := extractDisplayMesh(solid.ptr, nil)
	appendExpandedData(m, solid.ptr, 40)
	runtime.KeepAlive(solid)
	return m
}

// appendExpandedData adds pre-expanded positions and edge lines to an existing DisplayMesh.
func appendExpandedData(dm *DisplayMesh, ptr *C.ManifoldPtr, edgeThresholdDeg float32) {
	var cPositions *C.float
	var cNumPositions C.int
	var cFaceIDs *C.uint32_t
	var cNumFaceIDs C.int
	var cEdgeLines *C.float
	var cNumEdges C.int

	C.facet_extract_expanded_mesh(ptr,
		&cPositions, &cNumPositions,
		&cFaceIDs, &cNumFaceIDs,
		&cEdgeLines, &cNumEdges,
		C.float(edgeThresholdDeg))

	numPositions := int(cNumPositions)
	if numPositions > 0 && cPositions != nil {
		defer C.free(unsafe.Pointer(cPositions))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cPositions)), numPositions*3*4)
		dm.ExpandedRaw = make([]byte, len(src))
		copy(dm.ExpandedRaw, src)
		dm.ExpandedCount = numPositions
	}

	numFaceIDs := int(cNumFaceIDs)
	if numFaceIDs > 0 && cFaceIDs != nil {
		defer C.free(unsafe.Pointer(cFaceIDs))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), numFaceIDs*4)
		dm.FaceGroupRaw = make([]byte, len(src))
		copy(dm.FaceGroupRaw, src)
		dm.FaceGroupCount = numFaceIDs
	}

	numEdges := int(cNumEdges)
	if numEdges > 0 && cEdgeLines != nil {
		defer C.free(unsafe.Pointer(cEdgeLines))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cEdgeLines)), numEdges*6*4)
		dm.EdgeLinesRaw = make([]byte, len(src))
		copy(dm.EdgeLinesRaw, src)
		dm.EdgeCount = numEdges
	}
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

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

// CreateCube creates a box with the given dimensions.
func CreateCube(x, y, z float64) (*Solid, error) {
	s := newSolidWithOrigin(C.facet_cube(C.double(x), C.double(y), C.double(z)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cube")
	}
	return s, nil
}

// CreateSphere creates a sphere with the given radius and segment count.
func CreateSphere(radius float64, segments int) (*Solid, error) {
	s := newSolidWithOrigin(C.facet_sphere(C.double(radius), C.int(segments)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create sphere")
	}
	return s, nil
}

// CreateCylinder creates a cylinder (or cone if radii differ).
func CreateCylinder(height, radiusLow, radiusHigh float64, segments int) (*Solid, error) {
	s := newSolidWithOrigin(C.facet_cylinder(C.double(height), C.double(radiusLow), C.double(radiusHigh), C.int(segments)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cylinder")
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

// CreateSquare creates a 2D rectangle.
func CreateSquare(x, y float64) *Sketch {
	ptr := C.facet_square(C.double(x), C.double(y))
	return newSketch(ptr)
}

// CreateCircle creates a 2D circle.
func CreateCircle(radius float64, segments int) *Sketch {
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

// CreatePolygon creates a 2D sketch from a slice of points.
func CreatePolygon(points []Point2D) (*Sketch, error) {
	n := len(points)
	if n < 3 {
		return nil, fmt.Errorf("Polygon requires at least 3 points, got %d", n)
	}
	// Ensure CCW winding (positive signed area) so extrusion normals face +Z.
	// Shoelace formula: positive = CCW, negative = CW.
	var area2 float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area2 += points[i].X*points[j].Y - points[j].X*points[i].Y
	}
	if area2 < 0 {
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
	return newSketch(ptr), nil
}

// ---------------------------------------------------------------------------
// 3D Boolean Operations
// ---------------------------------------------------------------------------

// Union computes the boolean union of two solids.
func (a *Solid) Union(b *Solid) *Solid {
	ptr := C.facet_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Difference computes the boolean difference of two solids.
func (a *Solid) Difference(b *Solid) *Solid {
	ptr := C.facet_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Intersection computes the boolean intersection of two solids.
func (a *Solid) Intersection(b *Solid) *Solid {
	ptr := C.facet_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
func (a *Solid) Insert(b *Solid) *Solid {
	ptr := C.facet_insert(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// DecomposeSolid splits a solid into its disconnected connected components.
func DecomposeSolid(s *Solid) []*Solid {
	var outArr **C.ManifoldPtr
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
// 2D Boolean Operations
// ---------------------------------------------------------------------------

// Union computes the boolean union of two sketches.
func (a *Sketch) Union(b *Sketch) *Sketch {
	ptr := C.facet_cs_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// Difference computes the boolean difference of two sketches.
func (a *Sketch) Difference(b *Sketch) *Sketch {
	ptr := C.facet_cs_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// Intersection computes the boolean intersection of two sketches.
func (a *Sketch) Intersection(b *Sketch) *Sketch {
	ptr := C.facet_cs_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 2D → 3D
// ---------------------------------------------------------------------------

// Extrude extrudes a sketch upward by height.
func (p *Sketch) Extrude(height float64, slices int, twist, scaleX, scaleY float64) (*Solid, error) {
	ptr := C.facet_extrude(p.ptr, C.double(height), C.int(slices), C.double(twist), C.double(scaleX), C.double(scaleY))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to extrude")
	}
	return s, nil
}

// Revolve revolves a sketch around the Y axis.
func (p *Sketch) Revolve(segments int, degrees float64) (*Solid, error) {
	ptr := C.facet_revolve(p.ptr, C.int(segments), C.double(degrees))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to revolve")
	}
	return s, nil
}

// Sweep extrudes a sketch along a 3D path.
func (p *Sketch) Sweep(path []Point3D) (*Solid, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("Sweep requires at least 2 path points, got %d", len(path))
	}
	flat := make([]C.double, len(path)*3)
	for i, pt := range path {
		flat[i*3], flat[i*3+1], flat[i*3+2] = C.double(pt.X), C.double(pt.Y), C.double(pt.Z)
	}
	ptr := C.facet_sweep(p.ptr, &flat[0], C.size_t(len(path)))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to sweep")
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

// Slice takes a cross-section of a solid at the given Z height.
func (s *Solid) Slice(height float64) *Sketch {
	ptr := C.facet_slice(s.ptr, C.double(height))
	runtime.KeepAlive(s)
	return newSketch(ptr)
}

// Project projects a solid onto the XY plane.
func (s *Solid) Project() *Sketch {
	ptr := C.facet_project(s.ptr)
	runtime.KeepAlive(s)
	return newSketch(ptr)
}

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

// Translate moves a solid by (x, y, z).
func (s *Solid) Translate(x, y, z float64) *Solid {
	return transformSolid(s, C.facet_translate(s.ptr, C.double(x), C.double(y), C.double(z)))
}

func scale(s *Solid, x, y, z float64) *Solid {
	return transformSolid(s, C.facet_scale(s.ptr, C.double(x), C.double(y), C.double(z)))
}

func mirror(s *Solid, nx, ny, nz float64) *Solid {
	return transformSolid(s, C.facet_mirror(s.ptr, C.double(nx), C.double(ny), C.double(nz)))
}

// Rotate rotates a solid by (x, y, z) degrees around each axis, pivoting on
// the bounding box center so the solid spins in place.
func (s *Solid) Rotate(x, y, z float64) *Solid {
	return transformSolid(s, C.facet_rotate_local(s.ptr, C.double(x), C.double(y), C.double(z)))
}

// Scale scales a solid by (x, y, z) around pivot point (ox, oy, oz).
func (s *Solid) Scale(x, y, z, ox, oy, oz float64) *Solid {
	return scale(s.Translate(-ox, -oy, -oz), x, y, z).Translate(ox, oy, oz)
}

// Mirror mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin. The normal is normalized internally.
func (s *Solid) Mirror(nx, ny, nz, offset float64) *Solid {
	ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if ln == 0 {
		return s
	}
	dx, dy, dz := nx/ln*offset, ny/ln*offset, nz/ln*offset
	return mirror(s.Translate(-dx, -dy, -dz), nx, ny, nz).Translate(dx, dy, dz)
}

// RotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func (s *Solid) RotateAt(rx, ry, rz, ox, oy, oz float64) *Solid {
	return transformSolid(s, C.facet_rotate_at(s.ptr, C.double(rx), C.double(ry), C.double(rz), C.double(ox), C.double(oy), C.double(oz)))
}

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

// Translate moves a sketch by (x, y).
func (p *Sketch) Translate(x, y float64) *Sketch {
	ptr := C.facet_cs_translate(p.ptr, C.double(x), C.double(y))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// RotateOrigin rotates a sketch by degrees around the world origin (0, 0).
func (p *Sketch) RotateOrigin(degrees float64) *Sketch {
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

// Rotate rotates a sketch by degrees, pivoting on the bounding box center.
func (p *Sketch) Rotate(degrees float64) *Sketch {
	ptr := C.facet_cs_rotate_local(p.ptr, C.double(degrees))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// Scale scales a sketch by (x, y) around pivot point (px, py).
func (p *Sketch) Scale(x, y, px, py float64) *Sketch {
	return sketchScale(p.Translate(-px, -py), x, y).Translate(px, py)
}

// Mirror mirrors a sketch across the axis (ax, ay) at signed distance offset
// from the world origin. The axis is normalized internally.
func (p *Sketch) Mirror(ax, ay, offset float64) *Sketch {
	ln := math.Sqrt(ax*ax + ay*ay)
	if ln == 0 {
		return p
	}
	dx, dy := ax/ln*offset, ay/ln*offset
	return sketchMirror(p.Translate(-dx, -dy), ax, ay).Translate(dx, dy)
}

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a solid.
func (s *Solid) Hull() *Solid {
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

// BatchHull computes the convex hull of multiple solids together.
func BatchHull(solids []*Solid) *Solid {
	ptrs := make([]*C.ManifoldPtr, len(solids))
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

// HullPoints computes the convex hull of a set of 3D points.
func HullPoints(points []Point3D) *Solid {
	n := len(points)
	coords := make([]C.double, n*3)
	for i, p := range points {
		coords[i*3] = C.double(p.X)
		coords[i*3+1] = C.double(p.Y)
		coords[i*3+2] = C.double(p.Z)
	}
	return newSolidWithOrigin(C.facet_hull_points(&coords[0], C.size_t(n)))
}

// TrimByPlane trims a solid by the plane defined by normal and offset.
func (s *Solid) TrimByPlane(nx, ny, nz, offset float64) *Solid {
	return transformSolid(s, C.facet_trim_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset)))
}

// SmoothOut smooths sharp edges of a solid.
func (s *Solid) SmoothOut(minSharpAngle, minSmoothness float64) *Solid {
	return transformSolid(s, C.facet_smooth_out(s.ptr, C.double(minSharpAngle), C.double(minSmoothness)))
}

// Refine subdivides the mesh of a solid n times.
func (s *Solid) Refine(n int) *Solid {
	return transformSolid(s, C.facet_refine(s.ptr, C.int(n)))
}

// Simplify reduces the triangle count of a solid by merging edges shorter than tolerance.
func (s *Solid) Simplify(tolerance float64) *Solid {
	return transformSolid(s, C.facet_simplify(s.ptr, C.double(tolerance)))
}

// RefineToLength subdivides edges longer than the given length.
func (s *Solid) RefineToLength(length float64) *Solid {
	return transformSolid(s, C.facet_refine_to_length(s.ptr, C.double(length)))
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

// SplitSolid splits m by cutter, returning [inside, outside].
func SplitSolid(m, cutter *Solid) [2]*Solid {
	pair := C.facet_split(m.ptr, cutter.ptr)
	runtime.KeepAlive(m)
	runtime.KeepAlive(cutter)
	fm := mergeFaceMaps(m.FaceMap, cutter.FaceMap)
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return [2]*Solid{first, second}
}

// SplitSolidByPlane splits a solid by an infinite plane, returning [above, below].
func SplitSolidByPlane(s *Solid, nx, ny, nz, offset float64) [2]*Solid {
	pair := C.facet_split_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset))
	runtime.KeepAlive(s)
	fm := s.withFaceMap()
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return [2]*Solid{first, second}
}

// ComposeSolids assembles non-overlapping solids into one without boolean operations.
func ComposeSolids(solids []*Solid) *Solid {
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	ptr := C.facet_compose((**C.ManifoldPtr)(unsafe.Pointer(&ptrs[0])), C.int(len(solids)))
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
// 2D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a sketch.
func (p *Sketch) Hull() *Sketch {
	ptr := C.facet_cs_hull(p.ptr)
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// SketchBatchHull computes the convex hull of multiple sketches together.
func SketchBatchHull(sketches []*Sketch) *Sketch {
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, p := range sketches {
		ptrs[i] = p.ptr
	}
	ptr := C.facet_cs_batch_hull(&ptrs[0], C.size_t(len(sketches)))
	runtime.KeepAlive(sketches)
	return newSketch(ptr)
}

// Loft creates a solid by blending between cross-sections at different heights.
func Loft(sketches []*Sketch, heights []float64) *Solid {
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
	return newSolidWithOrigin(ptr)
}

// Offset offsets a sketch's edges by delta with round join.
func (p *Sketch) Offset(delta float64, segments int) *Sketch {
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

// extractMesh converts a ManifoldPtr into a Go Mesh with shared vertices and indices.
// Normals are omitted; the frontend uses flatShading (GPU-computed face normals).
func extractMesh(m *C.ManifoldPtr) *Mesh {
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

// extractDisplayMesh copies mesh data directly from C buffers as raw bytes,
// skipping intermediate Go typed slices. faceMap is used to build
// faceGroupID → hex color and faceGroupID → source position lookups.
// buildFaceColorMap constructs a faceID→hex color map from a slice of face IDs
// and a FaceInfo map. Only face IDs with a non-NoColor entry are included.
func buildFaceColorMap(faceIDs []uint32, faceMap map[uint32]FaceInfo) map[string]string {
	var fcMap map[string]string
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
	return fcMap
}

func extractDisplayMesh(m *C.ManifoldPtr, faceMap map[uint32]FaceInfo) *DisplayMesh {
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

	// Copy vertex positions (xyz only) as raw bytes
	var vertRaw []byte
	if numProp == 3 {
		cBytes := unsafe.Slice((*byte)(unsafe.Pointer(cVerts)), numVert*3*4)
		vertRaw = make([]byte, len(cBytes))
		copy(vertRaw, cBytes)
	} else {
		vertRaw = make([]byte, numVert*12)
		src := unsafe.Pointer(cVerts)
		stride := uintptr(numProp) * 4
		for i := 0; i < numVert; i++ {
			copy(vertRaw[i*12:], unsafe.Slice((*byte)(unsafe.Add(src, uintptr(i)*stride)), 12))
		}
	}

	// Copy indices directly from C buffer
	triLen := numTri * 3
	idxSrc := unsafe.Slice((*byte)(unsafe.Pointer(cIndices)), triLen*4)
	idxRaw := make([]byte, len(idxSrc))
	copy(idxRaw, idxSrc)

	// Extract face group IDs and build face color/source maps
	var fgRaw []byte
	var fgCount int
	var fcMap map[string]string
	nFaceIDs := int(cNumFaceIDs)
	if nFaceIDs > 0 {
		defer C.free(unsafe.Pointer(cFaceIDs))
		fgSrc := unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), nFaceIDs*4)
		fgRaw = make([]byte, len(fgSrc))
		copy(fgRaw, fgSrc)
		fgCount = nFaceIDs

		faceIDs := unsafe.Slice((*uint32)(unsafe.Pointer(cFaceIDs)), nFaceIDs)
		fcMap = buildFaceColorMap(faceIDs, faceMap)
	}

	return &DisplayMesh{
		VertRaw:        vertRaw,
		IdxRaw:         idxRaw,
		FaceGroupRaw:   fgRaw,
		FaceColorMap:   fcMap,
		VertexCount:    numVert,
		IndexCount:     triLen,
		FaceGroupCount: fgCount,
	}
}

// mergeDisplayMeshes combines multiple display meshes into one.
// Deprecated: prefer MergeExtractDisplayMeshes when source Solids are available.
func mergeDisplayMeshes(meshes []*DisplayMesh) *DisplayMesh {
	if len(meshes) == 1 {
		return meshes[0]
	}
	totalVerts := 0
	totalIdx := 0
	for _, m := range meshes {
		totalVerts += m.VertexCount
		totalIdx += m.IndexCount
	}

	// Merge raw byte buffers
	vertBuf := make([]byte, 0, totalVerts*12)
	idxBuf := make([]uint32, 0, totalIdx)
	var fgBuf []uint32
	hasFaceGroups := false
	var vertOffset uint32

	// Check if any mesh has face groups
	for _, m := range meshes {
		if len(m.FaceGroupRaw) > 0 {
			hasFaceGroups = true
			break
		}
	}
	if hasFaceGroups {
		fgBuf = make([]uint32, 0, totalIdx/3)
	}

	for _, m := range meshes {
		vertBuf = append(vertBuf, m.VertRaw...)
		n := len(m.IdxRaw) / 4
		if n > 0 {
			src := unsafe.Slice((*uint32)(unsafe.Pointer(&m.IdxRaw[0])), n)
			for _, idx := range src {
				idxBuf = append(idxBuf, idx+vertOffset)
			}
		}

		// Merge face groups (IDs are globally unique via AsOriginal, no offset needed)
		if hasFaceGroups {
			if len(m.FaceGroupRaw) > 0 {
				fn := len(m.FaceGroupRaw) / 4
				src := unsafe.Slice((*uint32)(unsafe.Pointer(&m.FaceGroupRaw[0])), fn)
				fgBuf = append(fgBuf, src...)
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

	var idxRaw []byte
	if len(idxBuf) > 0 {
		idxRaw = make([]byte, len(idxBuf)*4)
		copy(idxRaw, unsafe.Slice((*byte)(unsafe.Pointer(&idxBuf[0])), len(idxBuf)*4))
	}

	var fgRaw []byte
	var fgCount int
	if len(fgBuf) > 0 {
		fgRaw = make([]byte, len(fgBuf)*4)
		copy(fgRaw, unsafe.Slice((*byte)(unsafe.Pointer(&fgBuf[0])), len(fgBuf)*4))
		fgCount = len(fgBuf)
	}

	return &DisplayMesh{
		VertRaw:        vertBuf,
		IdxRaw:         idxRaw,
		FaceGroupRaw:   fgRaw,
		VertexCount:    len(vertBuf) / 12,
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

	ptrs := make([]*C.ManifoldPtr, len(solids))
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

	// Extract XYZ positions (first 3 floats per vertex) as raw bytes
	var vertRaw []byte
	if numProp == 3 {
		cBytes := unsafe.Slice((*byte)(src), numVert*3*4)
		vertRaw = make([]byte, len(cBytes))
		copy(vertRaw, cBytes)
	} else {
		vertRaw = make([]byte, numVert*12)
		for i := 0; i < numVert; i++ {
			off := unsafe.Add(src, uintptr(i)*stride)
			copy(vertRaw[i*12:], unsafe.Slice((*byte)(off), 12))
		}
	}

	triLen := numTri * 3
	idxSrc := unsafe.Slice((*byte)(unsafe.Pointer(cIndices)), triLen*4)
	idxRaw := make([]byte, len(idxSrc))
	copy(idxRaw, idxSrc)

	var fgRaw []byte
	var fgCount int
	var fcMap map[string]string
	nFaceIDs := int(cNumFaceIDs)
	if nFaceIDs > 0 {
		defer C.free(unsafe.Pointer(cFaceIDs))
		fgSrc := unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), nFaceIDs*4)
		fgRaw = make([]byte, len(fgSrc))
		copy(fgRaw, fgSrc)
		fgCount = nFaceIDs

		faceIDs := unsafe.Slice((*uint32)(unsafe.Pointer(cFaceIDs)), nFaceIDs)
		fcMap = buildFaceColorMap(faceIDs, merged)
	}

	return &DisplayMesh{
		VertRaw:        vertRaw,
		IdxRaw:         idxRaw,
		FaceGroupRaw:   fgRaw,
		FaceColorMap:   fcMap,
		VertexCount:    numVert,
		IndexCount:     triLen,
		FaceGroupCount: fgCount,
	}
}

// MergeExtractExpandedMeshes extracts expanded (non-indexed) display meshes with
// pre-computed edge lines. The frontend can use the buffers directly without
// calling toNonIndexed() or EdgesGeometry().
func MergeExtractExpandedMeshes(solids []*Solid, edgeThresholdDeg float32) *DisplayMesh {
	if len(solids) == 0 {
		return &DisplayMesh{}
	}

	// Extract the standard indexed mesh (needed for face colors, posMap, etc.)
	dm := MergeExtractDisplayMeshes(solids)

	// Extract the expanded + edge data
	var cPositions *C.float
	var cNumPositions C.int
	var cFaceIDs *C.uint32_t
	var cNumFaceIDs C.int
	var cEdgeLines *C.float
	var cNumEdges C.int

	if len(solids) == 1 {
		C.facet_extract_expanded_mesh(solids[0].ptr,
			&cPositions, &cNumPositions,
			&cFaceIDs, &cNumFaceIDs,
			&cEdgeLines, &cNumEdges,
			C.float(edgeThresholdDeg))
	} else {
		ptrs := make([]*C.ManifoldPtr, len(solids))
		for i, s := range solids {
			ptrs[i] = s.ptr
		}
		C.facet_merge_extract_expanded_mesh(&ptrs[0], C.size_t(len(solids)),
			&cPositions, &cNumPositions,
			&cFaceIDs, &cNumFaceIDs,
			&cEdgeLines, &cNumEdges,
			C.float(edgeThresholdDeg))
	}
	runtime.KeepAlive(solids)

	numPositions := int(cNumPositions)
	if numPositions > 0 && cPositions != nil {
		defer C.free(unsafe.Pointer(cPositions))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cPositions)), numPositions*3*4)
		dm.ExpandedRaw = make([]byte, len(src))
		copy(dm.ExpandedRaw, src)
		dm.ExpandedCount = numPositions
	}

	numFaceIDs := int(cNumFaceIDs)
	if numFaceIDs > 0 && cFaceIDs != nil {
		defer C.free(unsafe.Pointer(cFaceIDs))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cFaceIDs)), numFaceIDs*4)
		dm.FaceGroupRaw = make([]byte, len(src))
		copy(dm.FaceGroupRaw, src)
		dm.FaceGroupCount = numFaceIDs
	}

	numEdges := int(cNumEdges)
	if numEdges > 0 && cEdgeLines != nil {
		defer C.free(unsafe.Pointer(cEdgeLines))
		src := unsafe.Slice((*byte)(unsafe.Pointer(cEdgeLines)), numEdges*6*4)
		dm.EdgeLinesRaw = make([]byte, len(src))
		copy(dm.EdgeLinesRaw, src)
		dm.EdgeCount = numEdges
	}

	return dm
}

// buildDisplayMesh creates a DisplayMesh from Go-typed arrays with optional face group IDs.
// verts is flat float32 xyz, indices is flat uint32 triangle indices,
// faceGroups (optional) is per-triangle face group IDs.
func buildDisplayMesh(verts []float32, indices []uint32, faceGroups []uint32) *DisplayMesh {
	if len(verts) == 0 || len(indices) == 0 {
		return &DisplayMesh{}
	}

	numVert := len(verts) / 3

	// Copy vertices as raw bytes
	vertSrc := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	vertRaw := make([]byte, len(vertSrc))
	copy(vertRaw, vertSrc)

	// Copy indices as raw bytes
	idxSrc := unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), len(indices)*4)
	idxRaw := make([]byte, len(idxSrc))
	copy(idxRaw, idxSrc)

	// Copy face groups if present
	var fgRaw []byte
	var fgCount int
	if len(faceGroups) > 0 {
		fgSrc := unsafe.Slice((*byte)(unsafe.Pointer(&faceGroups[0])), len(faceGroups)*4)
		fgRaw = make([]byte, len(fgSrc))
		copy(fgRaw, fgSrc)
		fgCount = len(faceGroups)
	}

	return &DisplayMesh{
		VertRaw:        vertRaw,
		IdxRaw:         idxRaw,
		FaceGroupRaw:   fgRaw,
		VertexCount:    numVert,
		IndexCount:     len(indices),
		FaceGroupCount: fgCount,
	}
}


