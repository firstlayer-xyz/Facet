package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Type → mesh conversions (convenience methods that own runtime.KeepAlive).
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Mesh Extraction (typed slices)
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

// ---------------------------------------------------------------------------
// Face-color helpers
// ---------------------------------------------------------------------------

// colorFromFaceInfo converts a FaceInfo int32 color to a hex string.
func colorFromFaceInfo(fi FaceInfo) string {
	c := fi.Color
	return fmt.Sprintf("#%02X%02X%02X", (c>>16)&0xFF, (c>>8)&0xFF, c&0xFF)
}

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

// ---------------------------------------------------------------------------
// DisplayMesh extraction — indexed format
// ---------------------------------------------------------------------------

// cDisplayMeshBuffers holds raw C pointers returned by facet_extract_display_mesh
// or facet_merge_extract_display_mesh. The two C functions have identical output
// signatures — only the input differs — so both paths populate this struct and
// then call buildDisplayMeshFromC to copy data into Go memory and free C buffers.
type cDisplayMeshBuffers struct {
	verts      *C.float
	numVerts   C.int
	numProp    C.int
	indices    *C.uint32_t
	numTris    C.int
	faceIDs    *C.uint32_t
	numFaceIDs C.int
}

// buildDisplayMeshFromC copies vertex, index, and face-group data from C
// buffers into a new DisplayMesh and frees the C buffers. faceMap provides
// colors for any face IDs present in the mesh. This is the shared tail for
// both single-solid (extractDisplayMesh) and multi-solid
// (MergeExtractDisplayMeshes) extraction.
func buildDisplayMeshFromC(b cDisplayMeshBuffers, faceMap map[uint32]FaceInfo) *DisplayMesh {
	numVert := int(b.numVerts)
	numTri := int(b.numTris)
	if numVert == 0 || numTri == 0 {
		return &DisplayMesh{}
	}
	defer C.free(unsafe.Pointer(b.verts))
	defer C.free(unsafe.Pointer(b.indices))

	vertRaw := copyXYZFromStrided(b.verts, numVert, int(b.numProp))

	triLen := numTri * 3
	idxRaw := copyCBytes(unsafe.Pointer(b.indices), triLen*4)

	var fgRaw []byte
	var fgCount int
	var fcMap map[string]string
	nFaceIDs := int(b.numFaceIDs)
	if nFaceIDs > 0 {
		defer C.free(unsafe.Pointer(b.faceIDs))
		fgRaw = copyCBytes(unsafe.Pointer(b.faceIDs), nFaceIDs*4)
		fgCount = nFaceIDs
		faceIDs := unsafe.Slice((*uint32)(unsafe.Pointer(b.faceIDs)), nFaceIDs)
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

// copyCBytes copies n bytes starting at ptr into a new Go-owned byte slice.
// The caller is responsible for freeing the C buffer after this returns.
func copyCBytes(ptr unsafe.Pointer, n int) []byte {
	if n == 0 || ptr == nil {
		return nil
	}
	dst := make([]byte, n)
	copy(dst, unsafe.Slice((*byte)(ptr), n))
	return dst
}

// copyXYZFromStrided copies XYZ positions (first 3 floats per vertex) from a C
// vertex buffer where each vertex occupies numProp floats. When numProp == 3
// this is a straight memcpy; otherwise the xyz triple is extracted per vertex.
func copyXYZFromStrided(cVerts *C.float, numVert, numProp int) []byte {
	if numProp == 3 {
		return copyCBytes(unsafe.Pointer(cVerts), numVert*12)
	}
	dst := make([]byte, numVert*12)
	src := unsafe.Pointer(cVerts)
	stride := uintptr(numProp) * 4
	for i := 0; i < numVert; i++ {
		copy(dst[i*12:], unsafe.Slice((*byte)(unsafe.Add(src, uintptr(i)*stride)), 12))
	}
	return dst
}

// extractDisplayMesh copies mesh data directly from C buffers as raw bytes,
// skipping intermediate Go typed slices. faceMap is used to build
// faceGroupID → hex color and faceGroupID → source position lookups.
func extractDisplayMesh(m *C.ManifoldPtr, faceMap map[uint32]FaceInfo) *DisplayMesh {
	var b cDisplayMeshBuffers
	C.facet_extract_display_mesh(m, &b.verts, &b.numVerts, &b.numProp,
		&b.indices, &b.numTris, &b.faceIDs, &b.numFaceIDs)
	return buildDisplayMeshFromC(b, faceMap)
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

	var b cDisplayMeshBuffers
	C.facet_merge_extract_display_mesh(&ptrs[0], C.size_t(len(solids)),
		&b.verts, &b.numVerts, &b.numProp,
		&b.indices, &b.numTris, &b.faceIDs, &b.numFaceIDs)
	runtime.KeepAlive(solids)
	return buildDisplayMeshFromC(b, merged)
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

// ---------------------------------------------------------------------------
// DisplayMesh extraction — expanded format (pre-expanded verts + edge lines)
// ---------------------------------------------------------------------------

// cExpandedMeshBuffers holds raw C pointers returned by
// facet_extract_expanded_mesh or facet_merge_extract_expanded_mesh.
// The two C functions share this output shape; both callers populate the
// struct and then hand it to copyExpandedMeshToDisplayMesh.
type cExpandedMeshBuffers struct {
	positions    *C.float
	numPositions C.int
	faceIDs      *C.uint32_t
	numFaceIDs   C.int
	edgeLines    *C.float
	numEdges     C.int
}

// copyExpandedMeshToDisplayMesh copies pre-expanded positions, per-triangle
// face IDs, and edge-line segments from C buffers into the given DisplayMesh,
// freeing the C buffers as it goes.
func copyExpandedMeshToDisplayMesh(dm *DisplayMesh, b cExpandedMeshBuffers) {
	if n := int(b.numPositions); n > 0 && b.positions != nil {
		defer C.free(unsafe.Pointer(b.positions))
		dm.ExpandedRaw = copyCBytes(unsafe.Pointer(b.positions), n*3*4)
		dm.ExpandedCount = n
	}
	if n := int(b.numFaceIDs); n > 0 && b.faceIDs != nil {
		defer C.free(unsafe.Pointer(b.faceIDs))
		dm.FaceGroupRaw = copyCBytes(unsafe.Pointer(b.faceIDs), n*4)
		dm.FaceGroupCount = n
	}
	if n := int(b.numEdges); n > 0 && b.edgeLines != nil {
		defer C.free(unsafe.Pointer(b.edgeLines))
		dm.EdgeLinesRaw = copyCBytes(unsafe.Pointer(b.edgeLines), n*6*4)
		dm.EdgeCount = n
	}
}

// appendExpandedData adds pre-expanded positions and edge lines to an existing DisplayMesh.
func appendExpandedData(dm *DisplayMesh, ptr *C.ManifoldPtr, edgeThresholdDeg float32) {
	var b cExpandedMeshBuffers
	C.facet_extract_expanded_mesh(ptr,
		&b.positions, &b.numPositions,
		&b.faceIDs, &b.numFaceIDs,
		&b.edgeLines, &b.numEdges,
		C.float(edgeThresholdDeg))
	copyExpandedMeshToDisplayMesh(dm, b)
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

	// Extract the expanded + edge data. Single- and multi-solid paths use
	// different C functions but populate identical buffer shapes.
	var b cExpandedMeshBuffers
	if len(solids) == 1 {
		C.facet_extract_expanded_mesh(solids[0].ptr,
			&b.positions, &b.numPositions,
			&b.faceIDs, &b.numFaceIDs,
			&b.edgeLines, &b.numEdges,
			C.float(edgeThresholdDeg))
	} else {
		ptrs := make([]*C.ManifoldPtr, len(solids))
		for i, s := range solids {
			ptrs[i] = s.ptr
		}
		C.facet_merge_extract_expanded_mesh(&ptrs[0], C.size_t(len(solids)),
			&b.positions, &b.numPositions,
			&b.faceIDs, &b.numFaceIDs,
			&b.edgeLines, &b.numEdges,
			C.float(edgeThresholdDeg))
	}
	runtime.KeepAlive(solids)
	copyExpandedMeshToDisplayMesh(dm, b)
	return dm
}
