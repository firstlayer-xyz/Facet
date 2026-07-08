//go:build js

package manifold

import (
	"encoding/binary"
	"math"
	"syscall/js"
)

// ExtractPolyMesh mirrors the native CGO impl in polymesh_extract.go but
// routes through the JS bridge — _mf_extract_polymesh returns three typed
// arrays (verts, face indices, face sizes) which we walk to assemble the
// PolyMesh's variable-length face list.
func ExtractPolyMesh(s *Solid) *PolyMesh {
	r := js.Global().Call("_mf_extract_polymesh", s.id)
	if r.IsNull() || r.IsUndefined() {
		return &PolyMesh{}
	}

	vertsBytes := typedArrayToBytes(r.Get("vertices"))      // float64
	faceIdxBytes := typedArrayToBytes(r.Get("faceIndices")) // int32
	faceSizesBytes := typedArrayToBytes(r.Get("faceSizes")) // int32

	nV := len(vertsBytes) / 8
	nF := len(faceSizesBytes) / 4
	if nV == 0 || nF == 0 {
		return &PolyMesh{}
	}

	vertices := make([]float64, nV)
	for i := 0; i < nV; i++ {
		vertices[i] = math.Float64frombits(binary.LittleEndian.Uint64(vertsBytes[i*8:]))
	}

	totalIdx := len(faceIdxBytes) / 4
	faces := make([][]int, nF)
	offset := 0
	for fi := 0; fi < nF; fi++ {
		sz := int(int32(binary.LittleEndian.Uint32(faceSizesBytes[fi*4:])))
		if offset+sz > totalIdx {
			break
		}
		face := make([]int, sz)
		for j := 0; j < sz; j++ {
			face[j] = int(int32(binary.LittleEndian.Uint32(faceIdxBytes[(offset+j)*4:])))
		}
		faces[fi] = face
		offset += sz
	}

	return &PolyMesh{Vertices: vertices, Faces: faces}
}
