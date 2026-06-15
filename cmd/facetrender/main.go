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
	"unsafe"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
)

// facetMesh evaluates the source's Main entry to a merged display mesh, or nil
// if it fails to load, type-check, evaluate, or produces no solids.
func facetMesh(source string) *manifold.DisplayMesh {
	ctx := context.Background()
	prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", nil)
	if err != nil {
		return nil
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		return nil
	}
	result, err := evaluator.Eval(ctx, prog, "model.fct", nil, "Main")
	if err != nil {
		return nil
	}
	solids, err := result.StaticSolids(0)
	if err != nil || len(solids) == 0 {
		return nil
	}
	return manifold.MergeExtractExpandedMeshes(solids, 40)
}

// FacetRenderMesh evaluates source's Main and returns the expanded (non-indexed)
// triangle positions as a malloc'd float32 buffer: 9 floats per triangle (three
// xyz verts), suitable for flat-shaded rendering. *outFloats receives the float
// count. Returns NULL on any failure (parse/check/eval error or empty mesh). The
// caller owns the buffer and must release it with FacetFree.
//
//export FacetRenderMesh
func FacetRenderMesh(csrc *C.char, outFloats *C.int) *C.float {
	*outFloats = 0
	dm := facetMesh(C.GoString(csrc))
	if dm == nil || len(dm.ExpandedRaw) == 0 {
		return nil
	}
	n := len(dm.ExpandedRaw) // bytes (float32 LE)
	buf := C.malloc(C.size_t(n))
	if buf == nil {
		return nil
	}
	C.memcpy(buf, unsafe.Pointer(&dm.ExpandedRaw[0]), C.size_t(n))
	*outFloats = C.int(n / 4)
	return (*C.float)(buf)
}

// FacetFree releases a buffer returned by FacetRenderMesh.
//
//export FacetFree
func FacetFree(p *C.float) {
	C.free(unsafe.Pointer(p))
}

func main() {}
