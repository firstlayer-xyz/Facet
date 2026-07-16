//go:build js

package manifold

import (
	"encoding/binary"
	"fmt"
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
		panic(fmt.Errorf("Sketch.ToMesh: identity-scale extrude failed: %w", err))
	}
	return solid.ToMesh()
}

func (s *Solid) ToDisplayMesh() *DisplayMesh {
	return extractDisplayMeshJS(s)
}

func (p *Sketch) ToDisplayMesh() *DisplayMesh {
	solid, err := p.Extrude(0.001, 0, 0, 1, 1)
	if err != nil {
		panic(fmt.Errorf("Sketch.ToDisplayMesh: identity-scale extrude failed: %w", err))
	}
	return solid.ToDisplayMesh()
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
	var fcMap map[uint32]string
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
// in a single C call (via _mf_merge_extract_expanded_mesh), matching native.
func MergeExtractExpandedMeshes(solids []*Solid, edgeThresholdDeg float32) *DisplayMesh {
	if len(solids) == 0 {
		return &DisplayMesh{}
	}
	if len(solids) == 1 {
		return extractDisplayMeshJS(solids[0])
	}
	r := js.Global().Call("_mf_merge_extract_expanded_mesh", solidIDArray(solids), len(solids), edgeThresholdDeg)
	return displayMeshFromExpanded(r, mergedFaceMaps(solids))
}
