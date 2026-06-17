//go:build js

package manifold

import (
	"encoding/binary"
	"math"
	"syscall/js"
)

func (s *Solid) ToMesh() *Mesh {
	r := js.Global().Call("_mf_get_mesh", s.id)
	verts := typedArrayToBytes(r.Get("verts"))
	idxBytes := typedArrayToBytes(r.Get("indices"))
	n := len(verts) / 4
	vertices := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(verts[i*4:])
		vertices[i] = math.Float32frombits(bits)
	}
	ni := len(idxBytes) / 4
	indices := make([]uint32, ni)
	for i := 0; i < ni; i++ {
		indices[i] = binary.LittleEndian.Uint32(idxBytes[i*4:])
	}
	return &Mesh{Vertices: vertices, Indices: indices}
}

func (p *Sketch) ToMesh() *Mesh {
	solid, err := p.Extrude(0.001, 0, 0, 1, 1)
	if err != nil {
		return nil
	}
	return solid.ToMesh()
}

func (s *Solid) ToDisplayMesh() *DisplayMesh {
	return extractDisplayMeshJS(s)
}

func (p *Sketch) ToDisplayMesh() *DisplayMesh {
	solid, err := p.Extrude(0.001, 0, 0, 1, 1)
	if err != nil {
		return nil
	}
	return solid.ToDisplayMesh()
}

func ExtractMeshShared(s *Solid) *Mesh {
	return s.ToMesh()
}

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

func extractDisplayMeshJS(s *Solid) *DisplayMesh {
	return displayMeshFromExpanded(js.Global().Call("_mf_extract_display_mesh", s.id), s.FaceMap)
}

// displayMeshFromExpanded builds a DisplayMesh from a JS {expanded, faceIDs,
// edges} result and the source FaceMap. Shared by the single-solid and
// merge-extract paths so both produce identical meshes and face-color maps.
func displayMeshFromExpanded(r js.Value, faceMap map[uint32]FaceInfo) *DisplayMesh {
	if r.IsNull() || r.IsUndefined() {
		return &DisplayMesh{}
	}

	expandedRaw := typedArrayToBytes(r.Get("expanded"))
	faceIDRaw := typedArrayToBytes(r.Get("faceIDs"))
	edgeRaw := typedArrayToBytes(r.Get("edges"))

	nExpanded := len(expandedRaw) / 12 // 3 floats * 4 bytes
	nTri := nExpanded / 3
	nEdges := len(edgeRaw) / 24 // 6 floats * 4 bytes

	// Decode the face IDs to []uint32 and hand them to the shared
	// buildFaceColorMap so native and wasm produce identical maps (incl.
	// #RRGGBBAA for translucent faces).
	var fcMap map[string]string
	if len(faceMap) > 0 && len(faceIDRaw) > 0 {
		faceIDs := make([]uint32, len(faceIDRaw)/4)
		for i := range faceIDs {
			faceIDs[i] = binary.LittleEndian.Uint32(faceIDRaw[i*4:])
		}
		fcMap = buildFaceColorMap(faceIDs, faceMap)
	}

	return &DisplayMesh{
		ExpandedRaw:    expandedRaw,
		FaceGroupRaw:   faceIDRaw,
		EdgeLinesRaw:   edgeRaw,
		ExpandedCount:  nExpanded,
		FaceGroupCount: nTri,
		EdgeCount:      nEdges,
		FaceColorMap:   fcMap,
	}
}

func MergeExtractDisplayMeshes(solids []*Solid) *DisplayMesh {
	return MergeExtractExpandedMeshes(solids, DefaultDisplayEdgeThresholdDeg)
}

// MergeExtractExpandedMeshes composes the solids and extracts one expanded mesh
// in a single C call (via _mf_merge_extract_expanded_mesh), matching native. The
// previous implementation faked the merge with a boolean Union — which welds
// coincident faces and drops interior geometry — and built a *Solid bypassing
// newSolid, leaking the ExternalAlloc/finalizer accounting.
func MergeExtractExpandedMeshes(solids []*Solid, edgeThresholdDeg float32) *DisplayMesh {
	if len(solids) == 0 {
		return &DisplayMesh{}
	}
	if len(solids) == 1 {
		return extractDisplayMeshJS(solids[0])
	}
	idArr := js.Global().Get("Array").New()
	var faceMap map[uint32]FaceInfo
	for _, s := range solids {
		idArr.Call("push", s.id)
		faceMap = mergeFaceMaps(faceMap, s.FaceMap)
	}
	r := js.Global().Call("_mf_merge_extract_expanded_mesh", idArr, len(solids), edgeThresholdDeg)
	return displayMeshFromExpanded(r, faceMap)
}
