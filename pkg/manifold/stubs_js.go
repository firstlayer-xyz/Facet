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

// Text stub.
func CreateText(fontPath, text string, sizeMM float64) (*Sketch, error) {
	return nil, fmt.Errorf("CreateText: not implemented in WASM mode")
}

// DefaultFontPath stub — no filesystem temp files in WASM.
func DefaultFontPath() string {
	return ""
}

// Import/Export stubs — file I/O not available in browser.
func ImportMesh(path string) (*Solid, error) {
	return nil, fmt.Errorf("ImportMesh: not available in WASM mode")
}

func CreateSolidFromMesh(vertices []float32, indices []uint32) (*Solid, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: empty vertex or index data")
	}
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
	return newSolidWithOrigin(id), nil
}

func ExportMesh(s *Solid, path string) error {
	return fmt.Errorf("ExportMesh: not available in WASM mode (use browser download)")
}

func ExportMeshes(solids []*Solid, path string) error {
	return fmt.Errorf("ExportMeshes: not available in WASM mode (use browser download)")
}

func Export3MF(s *Solid, path string) error {
	return fmt.Errorf("Export3MF: not available in WASM mode (use browser download)")
}

func Export3MFMulti(solids []*Solid, path string) error {
	return fmt.Errorf("Export3MFMulti: not available in WASM mode (use browser download)")
}

func ExportSTL(s *Solid, path string) error {
	return fmt.Errorf("ExportSTL: not available in WASM mode (use browser download)")
}

func ExportSTLMulti(solids []*Solid, path string) error {
	return fmt.Errorf("ExportSTLMulti: not available in WASM mode (use browser download)")
}

func ExportOBJ(s *Solid, path string) error {
	return fmt.Errorf("ExportOBJ: not available in WASM mode (use browser download)")
}

func ExportOBJMulti(solids []*Solid, path string) error {
	return fmt.Errorf("ExportOBJMulti: not available in WASM mode (use browser download)")
}
