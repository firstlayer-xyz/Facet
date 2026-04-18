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

	"github.com/firstlayer-xyz/meshio"
)

// unionAll reduces a non-empty slice of Solids into one via pairwise union.
func unionAll(solids []*Solid) *Solid {
	result := solids[0]
	for i := 1; i < len(solids); i++ {
		result = result.Union(solids[i])
	}
	return result
}

// ExportMesh exports a single Solid to a file via Assimp. The format is
// auto-detected from the file extension (OBJ, GLB, etc.). For 3MF and STL,
// prefer Export3MF or ExportSTL which support per-face color.
func ExportMesh(s *Solid, path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cErr := C.facet_export_mesh(s.ptr, cPath)
	runtime.KeepAlive(s)
	if cErr != nil {
		msg := C.GoString(cErr)
		C.facet_free_string(cErr)
		return fmt.Errorf("ExportMesh %s: %s", path, msg)
	}
	return nil
}

// ExportMeshes unions multiple Solids and exports to a file.
func ExportMeshes(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	return ExportMesh(unionAll(solids), path)
}

// RunMesh holds extracted triangle mesh data with run information
// for mapping originalIDs back to face colors.
type RunMesh struct {
	Vertices       []float32 // flat xyz positions
	Indices        []uint32  // triangle indices
	RunOriginalID  []uint32  // originalID per run
	RunIndex       []uint32  // start triVerts index per run (len = NumRuns+1)
}

// extractRunMesh extracts mesh data with run information from a Solid.
// This is used by the 3MF exporter to map per-face colors via originalID.
func extractRunMesh(s *Solid) *RunMesh {
	var cVerts *C.float
	var cNumVerts C.int
	var cIndices *C.uint32_t
	var cNumTris C.int
	var cRunOrigID *C.uint32_t
	var cRunIndex *C.uint32_t
	var cNumRuns C.int

	C.facet_extract_mesh_with_runs(s.ptr,
		&cVerts, &cNumVerts,
		&cIndices, &cNumTris,
		&cRunOrigID, &cRunIndex, &cNumRuns)
	runtime.KeepAlive(s)

	nv := int(cNumVerts)
	nt := int(cNumTris)
	nr := int(cNumRuns)

	if nv == 0 || nt == 0 {
		return &RunMesh{}
	}
	defer C.free(unsafe.Pointer(cVerts))
	defer C.free(unsafe.Pointer(cIndices))

	vertices := make([]float32, nv*3)
	copy(vertices, unsafe.Slice((*float32)(unsafe.Pointer(cVerts)), nv*3))

	indices := make([]uint32, nt*3)
	copy(indices, unsafe.Slice((*uint32)(unsafe.Pointer(cIndices)), nt*3))

	var runOrigID []uint32
	var runIndex []uint32
	if nr > 0 {
		defer C.free(unsafe.Pointer(cRunOrigID))
		defer C.free(unsafe.Pointer(cRunIndex))

		runOrigID = make([]uint32, nr)
		copy(runOrigID, unsafe.Slice((*uint32)(unsafe.Pointer(cRunOrigID)), nr))

		// runIndex has nr+1 entries
		riLen := nr + 1
		runIndex = make([]uint32, riLen)
		copy(runIndex, unsafe.Slice((*uint32)(unsafe.Pointer(cRunIndex)), riLen))
	}

	return &RunMesh{
		Vertices:      vertices,
		Indices:       indices,
		RunOriginalID: runOrigID,
		RunIndex:      runIndex,
	}
}

// buildFaceColors constructs per-face color assignments from run-based mesh data
// and a FaceMap. defaultHex is the color used for faces whose originalID has no
// color in the FaceMap (e.g. "#C0C0C0" for 3MF, "" for OBJ). Returns nil if
// the FaceMap contains no colors or there are no runs.
func buildFaceColors(rm *RunMesh, faceMap map[uint32]FaceInfo, defaultHex string) []meshio.FaceColor {
	hasColors := false
	for _, fi := range faceMap {
		if fi.Color != NoColor {
			hasColors = true
			break
		}
	}
	if !hasColors || len(rm.RunOriginalID) == 0 {
		return nil
	}
	numTris := len(rm.Indices) / 3
	faceColors := make([]meshio.FaceColor, numTris)
	for run := 0; run < len(rm.RunOriginalID); run++ {
		origID := rm.RunOriginalID[run]
		hex := defaultHex
		if fi, ok := faceMap[origID]; ok && fi.Color != NoColor {
			hex = colorFromFaceInfo(fi)
		}
		startTri := int(rm.RunIndex[run]) / 3
		endTri := int(rm.RunIndex[run+1]) / 3
		for t := startTri; t < endTri; t++ {
			faceColors[t] = meshio.FaceColor{Hex: hex}
		}
	}
	return faceColors
}

// Export3MF exports a single Solid to a 3MF file using faceID-based colors.
// The mesh geometry is written untouched — no vertex splitting or merging.
// Colors are derived from the Solid's FaceMap via Manifold's originalID run tracking.
func Export3MF(s *Solid, path string) error {
	rm := extractRunMesh(s)
	if len(rm.Vertices) == 0 {
		return fmt.Errorf("export failed: empty mesh")
	}

	// When a FaceMap with colors exists, every face must get a color — slicers like
	// OrcaSlicer/PrusaSlicer ignore colors entirely if any triangle lacks one.
	faceColors := buildFaceColors(rm, s.FaceMap, "#C0C0C0")

	m := &meshio.Mesh{
		Vertices:   rm.Vertices,
		Indices:    rm.Indices,
		FaceColors: faceColors,
	}
	return m.Write3MF(path)
}

// Export3MFMulti unions multiple Solids and exports to 3MF with faceID-based colors.
func Export3MFMulti(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	return Export3MF(unionAll(solids), path)
}

// ExportSTL exports a single Solid to a binary STL file.
func ExportSTL(s *Solid, path string) error {
	rm := extractRunMesh(s)
	if len(rm.Vertices) == 0 {
		return fmt.Errorf("export failed: empty mesh")
	}
	m := &meshio.Mesh{
		Vertices: rm.Vertices,
		Indices:  rm.Indices,
	}
	return m.WriteSTL(path)
}

// ExportSTLMulti unions multiple Solids and exports to STL.
func ExportSTLMulti(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	return ExportSTL(unionAll(solids), path)
}

// ExportOBJ exports a single Solid to a Wavefront OBJ file with per-face colors.
func ExportOBJ(s *Solid, path string) error {
	rm := extractRunMesh(s)
	if len(rm.Vertices) == 0 {
		return fmt.Errorf("export failed: empty mesh")
	}

	faceColors := buildFaceColors(rm, s.FaceMap, "")

	m := &meshio.Mesh{
		Vertices:   rm.Vertices,
		Indices:    rm.Indices,
		FaceColors: faceColors,
	}
	return m.WriteOBJ(path)
}

// ExportOBJMulti unions multiple Solids and exports to OBJ with per-face colors.
func ExportOBJMulti(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	return ExportOBJ(unionAll(solids), path)
}
