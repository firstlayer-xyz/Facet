package manifold

// Type declarations shared by both the native (cgo) and wasm builds. Only Solid
// and Sketch are build-specific (they hold a C pointer vs. a wasm handle);
// everything here is plain data or operates only on the common FaceMap field, so
// keeping a single definition stops the two builds from drifting — a drift the
// compiler can't catch across build tags.

// Mesh is an indexed triangle mesh in Go-owned slices.
type Mesh struct {
	Vertices []float32 // flat xyz positions
	Normals  []float32 // flat xyz normals (per-vertex)
	Indices  []uint32  // triangle indices
}

// DisplayMesh holds mesh data as raw byte slices for efficient binary transfer.
// Created by extracting directly from C buffers without intermediate Go typed
// slices.
//
// Which fields are populated depends on the extractor and build, so a consumer
// must match the two:
//   - Indexed fields (VertRaw/IdxRaw/FaceGroupRaw, VertexCount/IndexCount) are
//     filled only by the native indexed extractors (ToDisplayMesh,
//     MergeExtractDisplayMeshes); they are EMPTY in the wasm build.
//   - Expanded fields (ExpandedRaw/EdgeLinesRaw, ExpandedCount/EdgeCount) are
//     filled by the expanded extractors and by every wasm extractor.
// So reading IdxRaw works on desktop but yields empty data on wasm — read
// ExpandedRaw for code that must run in both.
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

// mergeFaceMaps returns a new map containing entries from both inputs. On a
// duplicate key, a's full FaceInfo wins (struct overwrite — not a per-field
// merge).
func mergeFaceMaps(a, b map[uint32]FaceInfo) map[uint32]FaceInfo {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a) == 0 {
		m := make(map[uint32]FaceInfo, len(b))
		for k, v := range b {
			m[k] = v
		}
		return m
	}
	if len(b) == 0 {
		m := make(map[uint32]FaceInfo, len(a))
		for k, v := range a {
			m[k] = v
		}
		return m
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
