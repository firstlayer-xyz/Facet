//go:build js

package manifold

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
)

// createSolidFromMeshWithFaceIDs creates a Manifold Solid from raw vertex, index,
// and per-triangle faceID data. Called by PolyMesh.ToSolid().
func createSolidFromMeshWithFaceIDs(verts []float32, indices, faceIDs []uint32) (*Solid, error) {
	if len(verts) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("createSolidFromMeshWithFaceIDs: empty vertex or index data")
	}
	nVerts := len(verts) / 3
	nTris := len(indices) / 3
	nFaceIDs := len(faceIDs)

	vertArr := js.Global().Get("Float32Array").New(len(verts))
	for i, v := range verts {
		vertArr.SetIndex(i, float64(v))
	}

	idxArr := js.Global().Get("Uint32Array").New(len(indices))
	for i, v := range indices {
		idxArr.SetIndex(i, v)
	}

	faceIDArr := js.Global().Get("Uint32Array").New(nFaceIDs)
	for i, v := range faceIDs {
		faceIDArr.SetIndex(i, v)
	}

	id := js.Global().Call("_mf_solid_from_mesh_with_face_ids", vertArr, nVerts, idxArr, nTris, faceIDArr, nFaceIDs).Int()
	if id == 0 {
		return nil, fmt.Errorf("createSolidFromMeshWithFaceIDs: manifold creation failed")
	}
	return newSolidWithOrigin(id), nil
}

// buildDisplayMesh creates a DisplayMesh from Go-typed arrays with optional face group IDs.
// Called by PolyMesh.ToDisplayMesh().
func buildDisplayMesh(verts []float32, indices []uint32, faceGroups []uint32) *DisplayMesh {
	if len(verts) == 0 || len(indices) == 0 {
		return &DisplayMesh{}
	}

	numVert := len(verts) / 3

	// Encode vertices as raw bytes (little-endian float32)
	vertRaw := make([]byte, len(verts)*4)
	for i, v := range verts {
		binary.LittleEndian.PutUint32(vertRaw[i*4:], math.Float32bits(v))
	}

	// Encode indices as raw bytes
	idxRaw := make([]byte, len(indices)*4)
	for i, v := range indices {
		binary.LittleEndian.PutUint32(idxRaw[i*4:], v)
	}

	// Encode face groups if present
	var fgRaw []byte
	var fgCount int
	if len(faceGroups) > 0 {
		fgRaw = make([]byte, len(faceGroups)*4)
		for i, v := range faceGroups {
			binary.LittleEndian.PutUint32(fgRaw[i*4:], v)
		}
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

// ImportMesh stub — file I/O is not available in the browser. Mesh export in
// the browser goes through facetExport (web/wasm), which serializes to bytes
// via manifold.EncodeSolidMesh and hands them to JS for download.
func ImportMesh(path string) (*Solid, error) {
	return nil, fmt.Errorf("ImportMesh: not available in WASM mode")
}

func CreateSolidFromMesh(vertices []float32, indices []uint32) (*Solid, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: empty vertex or index data")
	}
	indices = orientOutward(vertices, indices)
	nVerts := len(vertices) / 3
	nTris := len(indices) / 3

	vertArr := js.Global().Get("Float32Array").New(len(vertices))
	for i, v := range vertices {
		vertArr.SetIndex(i, float64(v))
	}
	idxArr := js.Global().Get("Uint32Array").New(len(indices))
	for i, v := range indices {
		idxArr.SetIndex(i, v)
	}
	id := js.Global().Call("_mf_solid_from_mesh", vertArr, nVerts, idxArr, nTris).Int()
	if id == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: manifold creation failed")
	}
	s := newSolidWithOrigin(id)
	if s.NumComponents() == 0 {
		// Accepted but empty: the input is not a valid closed 2-manifold (open,
		// self-intersecting, or non-orientable). Error rather than vanish.
		return nil, fmt.Errorf("CreateSolidFromMesh: mesh is not a valid closed manifold (open, self-intersecting, or non-orientable)")
	}
	return s, nil
}
