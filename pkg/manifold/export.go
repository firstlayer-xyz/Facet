//go:build !js

package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/firstlayer-xyz/meshio"
)

// RunMesh holds extracted triangle mesh data with run information
// for mapping originalIDs back to face colors.
type RunMesh struct {
	Vertices      []float32 // flat xyz positions
	Indices       []uint32  // triangle indices
	RunOriginalID []uint32  // originalID per run
	RunIndex      []uint32  // start triVerts index per run (len = NumRuns+1)
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
	var cNumRunIndex C.int

	C.facet_extract_mesh_with_runs(s.ptr,
		&cVerts, &cNumVerts,
		&cIndices, &cNumTris,
		&cRunOrigID, &cRunIndex, &cNumRuns, &cNumRunIndex)
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

		// Size from the length C reports (normally nr+1), not an assumed nr+1, so a
		// shorter runIndex can't drive an out-of-bounds read of the C buffer.
		riLen := int(cNumRunIndex)
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

// runTriangleHex maps the FaceMap onto one hex color per triangle, using
// Manifold's originalID run tracking. A triangle whose face has no assigned
// color gets "". The result feeds EncodeSolidMesh / faceColorsFromHex, which
// apply the per-format default.
func runTriangleHex(rm *RunMesh, faceMap map[uint32]FaceInfo) []string {
	hex := make([]string, len(rm.Indices)/3)
	for run := 0; run < len(rm.RunOriginalID); run++ {
		fi, ok := faceMap[rm.RunOriginalID[run]]
		if !ok || fi.Color == NoColor {
			continue
		}
		c := colorFromFaceInfo(fi)
		startTri := int(rm.RunIndex[run]) / 3
		endTri := int(rm.RunIndex[run+1]) / 3
		for t := startTri; t < endTri; t++ {
			hex[t] = c
		}
	}
	return hex
}

// Export3MF exports a single Solid to a 3MF file using faceID-based colors.
// The mesh geometry is written untouched — no vertex splitting or merging.
// Colors are derived from the Solid's FaceMap via Manifold's originalID run tracking.
// attachments are extra OPC parts embedded in the package (e.g. the Facet
// project payload); pass nil for none.
func Export3MF(s *Solid, path string, attachments []meshio.Attachment) error {
	rm := extractRunMesh(s)
	return writeRunMesh(rm, path, "3mf", runTriangleHex(rm, s.FaceMap), attachments)
}

// writeRunMesh encodes a run-mesh in the given format — with optional
// per-triangle hex colors and OPC attachments — and writes it to path.
func writeRunMesh(rm *RunMesh, path, format string, hex []string, attachments []meshio.Attachment) error {
	data, err := EncodeSolidMesh(rm.Vertices, rm.Indices, hex, format, attachments)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Export3MFMulti unions multiple Solids and exports to 3MF with faceID-based
// colors and the given OPC attachments (nil for none).
func Export3MFMulti(solids []*Solid, path string, attachments []meshio.Attachment) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	u, err := BatchBoolean(solids, OpUnion)
	if err != nil {
		return err
	}
	return Export3MF(u, path, attachments)
}

// ExportSTL exports a single Solid to a binary STL file.
func ExportSTL(s *Solid, path string) error {
	rm := extractRunMesh(s)
	return writeRunMesh(rm, path, "stl", nil, nil)
}

// ExportSTLMulti unions multiple Solids and exports to STL.
func ExportSTLMulti(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	u, err := BatchBoolean(solids, OpUnion)
	if err != nil {
		return err
	}
	return ExportSTL(u, path)
}

// ExportOBJ exports a single Solid to a Wavefront OBJ file with per-face colors.
func ExportOBJ(s *Solid, path string) error {
	rm := extractRunMesh(s)
	if len(rm.Vertices) == 0 {
		return fmt.Errorf("export failed: empty mesh")
	}

	m := &meshio.Mesh{
		Vertices:   rm.Vertices,
		Indices:    rm.Indices,
		FaceColors: faceColorsFromHex(runTriangleHex(rm, s.FaceMap), len(rm.Indices)/3, ""),
	}
	return m.WriteOBJ(path)
}

// ExportOBJMulti unions multiple Solids and exports to OBJ with per-face colors.
func ExportOBJMulti(solids []*Solid, path string) error {
	if len(solids) == 0 {
		return fmt.Errorf("no solids to export")
	}
	u, err := BatchBoolean(solids, OpUnion)
	if err != nil {
		return err
	}
	return ExportOBJ(u, path)
}
