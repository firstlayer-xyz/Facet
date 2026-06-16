// Command facetrender builds a c-archive that evaluates Facet source to a
// renderable triangle mesh, for embedding in the macOS Quick Look extension.
// A Quick Look extension is sandboxed and cannot spawn a subprocess, so it must
// call the evaluator + geometry kernel in-process — this exposes them as plain C.
package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"unsafe"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/pkg/meshpreview"

	"github.com/firstlayer-xyz/meshio"
)

// facetMesh evaluates the source's Main entry to a merged display mesh, or an
// error describing the first load/type-check/evaluation failure (or that it
// produced no solids).
func facetMesh(source string) (*manifold.DisplayMesh, error) {
	ctx := context.Background()
	prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", nil)
	if err != nil {
		return nil, err
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		return nil, &errs[0]
	}
	result, err := evaluator.Eval(ctx, prog, "model.fct", nil, "Main")
	if err != nil {
		return nil, err
	}
	solids, err := result.StaticSolids(0)
	if err != nil {
		return nil, err
	}
	if len(solids) == 0 {
		return nil, fmt.Errorf("model produced no solids")
	}
	return manifold.MergeExtractExpandedMeshes(solids, 40), nil
}

// previewBuffers loads a file into renderer inputs: expanded positions (9 floats
// per triangle) and a parallel per-expanded-vertex RGB color buffer (nil when
// the geometry carries no color). Mesh files (.stl/.obj/.3mf) are read as raw
// triangles; everything else is treated as Facet source and evaluated. Returns
// the load/compile error so callers can report why a file produced no geometry.
func previewBuffers(path string) (positions []float32, colors []byte, err error) {
	if meshio.CanRead(path) {
		return meshpreview.LoadColored(path)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	dm, err := facetMesh(string(src))
	if err != nil {
		return nil, nil, err
	}
	if len(dm.ExpandedRaw) == 0 {
		return nil, nil, fmt.Errorf("model produced no geometry")
	}
	return dm.ExpandedPositions(), dm.ExpandedColors(), nil
}

// FacetRenderFile loads the file at cpath (Facet source or a .stl/.obj/.3mf
// mesh) and returns expanded triangle positions as a malloc'd float32 buffer:
// 9 floats per triangle. *outFloats receives the float count. When the geometry
// has color, *outColors receives a malloc'd per-expanded-vertex RGB buffer (3
// bytes per vertex, so *outColorBytes == *outFloats); otherwise *outColors is
// NULL. Returns NULL on any failure (load/eval error or empty mesh). The caller
// owns both buffers: release positions with FacetFree and colors with
// FacetFreeBytes.
//
//export FacetRenderFile
func FacetRenderFile(cpath *C.char, outFloats *C.int, outColors **C.uchar, outColorBytes *C.int) *C.float {
	*outFloats = 0
	*outColors = nil
	*outColorBytes = 0

	positions, colors, err := previewBuffers(C.GoString(cpath))
	if err != nil || len(positions) == 0 {
		return nil
	}

	pn := len(positions) * 4
	pbuf := C.malloc(C.size_t(pn))
	if pbuf == nil {
		return nil
	}
	C.memcpy(pbuf, unsafe.Pointer(&positions[0]), C.size_t(pn))
	*outFloats = C.int(len(positions))

	if len(colors) > 0 {
		cbuf := C.malloc(C.size_t(len(colors)))
		if cbuf != nil {
			C.memcpy(cbuf, unsafe.Pointer(&colors[0]), C.size_t(len(colors)))
			*outColors = (*C.uchar)(cbuf)
			*outColorBytes = C.int(len(colors))
		}
	}
	return (*C.float)(pbuf)
}

// FacetRenderError loads the file at cpath exactly as FacetRenderFile does and
// returns a malloc'd C string describing why it produced no geometry, or NULL on
// success. The Quick Look preview calls this when FacetRenderFile returns NULL so
// it can show the compile/load error instead of a blank pane. The caller owns the
// string and must release it with FacetFreeString.
//
//export FacetRenderError
func FacetRenderError(cpath *C.char) *C.char {
	_, _, err := previewBuffers(C.GoString(cpath))
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}

// FacetFree releases a positions buffer returned by FacetRenderFile.
//
//export FacetFree
func FacetFree(p *C.float) {
	C.free(unsafe.Pointer(p))
}

// FacetFreeString releases a string returned by FacetRenderError.
//
//export FacetFreeString
func FacetFreeString(p *C.char) {
	C.free(unsafe.Pointer(p))
}

// FacetFreeBytes releases a color buffer returned by FacetRenderFile.
//
//export FacetFreeBytes
func FacetFreeBytes(p *C.uchar) {
	C.free(unsafe.Pointer(p))
}

func main() {}
