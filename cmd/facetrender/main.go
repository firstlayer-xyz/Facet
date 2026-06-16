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
	"sync"
	"unsafe"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/pkg/meshpreview"

	"github.com/firstlayer-xyz/meshio"
)

// evalMain loads, type-checks, and evaluates source's Main entry, returning the
// result (static Solids or an Animation) or the first load/type-check/eval error.
func evalMain(source string) (*evaluator.EvalResult, error) {
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
	return result, nil
}

// solidsToBuffers merges solids into one display mesh and returns its expanded
// positions (9 floats per triangle) plus a parallel per-expanded-vertex RGB
// color buffer (nil when uncolored), erroring on empty geometry. The 40° edge
// threshold matches the in-app viewport's normal smoothing.
func solidsToBuffers(solids []*manifold.Solid) (positions []float32, colors []byte, err error) {
	dm := manifold.MergeExtractExpandedMeshes(solids, 40)
	if len(dm.ExpandedRaw) == 0 {
		return nil, nil, fmt.Errorf("model produced no geometry")
	}
	return dm.ExpandedPositions(), dm.ExpandedColors(), nil
}

// previewBuffers loads a file into renderer inputs: expanded positions (9 floats
// per triangle) and a parallel per-expanded-vertex RGB color buffer (nil when
// the geometry carries no color). Mesh files (.stl/.obj/.3mf) are read as raw
// triangles; everything else is treated as Facet source and evaluated (a single
// frame at t=0 for an Animation). Returns the load/compile error so callers can
// report why a file produced no geometry.
func previewBuffers(path string) (positions []float32, colors []byte, err error) {
	if meshio.CanRead(path) {
		return meshpreview.LoadColored(path)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	result, err := evalMain(string(src))
	if err != nil {
		return nil, nil, err
	}
	solids, err := result.StaticSolids(0)
	if err != nil {
		return nil, nil, err
	}
	return solidsToBuffers(solids)
}

// emitBuffers copies positions (+ optional per-vertex RGB colors) into malloc'd
// C buffers using the QuickLook ABI: returns the positions buffer (NULL when
// empty), sets *outFloats to the float count, and when colors are present sets
// *outColors / *outColorBytes (with *outColorBytes == *outFloats). The caller
// frees positions with FacetFree and colors with FacetFreeBytes.
func emitBuffers(positions []float32, colors []byte, outFloats *C.int, outColors **C.uchar, outColorBytes *C.int) *C.float {
	*outFloats = 0
	*outColors = nil
	*outColorBytes = 0
	if len(positions) == 0 {
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
	positions, colors, _ := previewBuffers(C.GoString(cpath))
	return emitBuffers(positions, colors, outFloats, outColors, outColorBytes)
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

// FacetFree releases a positions buffer returned by FacetRenderFile or
// FacetAnimationFrame.
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

// FacetFreeBytes releases a color buffer returned by FacetRenderFile or
// FacetAnimationFrame.
//
//export FacetFreeBytes
func FacetFreeBytes(p *C.uchar) {
	C.free(unsafe.Pointer(p))
}

// Animation handle registry. A QuickLook preview opens an animation once and
// pulls frames over the life of the preview; handles are kept here until closed.
var (
	animMu       sync.Mutex
	animNext     int32
	animSessions = map[int32]*evaluator.EvalResult{}
)

// maxAnimSessions bounds the registry so a preview dismissed without calling
// FacetCloseAnimation (the QL host process is long-lived and reused across
// previews) leaks at most this many retained sessions; opening past it evicts
// the oldest (lowest handle).
const maxAnimSessions = 8

// openAnimation evaluates source and, if Main returned an Animation, retains the
// session and returns a non-zero handle. Returns (0, false) for a static model
// or any failure.
func openAnimation(source string) (int32, bool) {
	result, err := evalMain(source)
	if err != nil || result == nil || result.Animation == nil {
		return 0, false
	}
	animMu.Lock()
	defer animMu.Unlock()
	animNext++
	animSessions[animNext] = result
	for len(animSessions) > maxAnimSessions {
		oldest := animNext
		for h := range animSessions {
			if h < oldest {
				oldest = h
			}
		}
		delete(animSessions, oldest)
	}
	return animNext, true
}

// animationFrame renders the animation registered under handle at timeMs (ms),
// returning expanded positions + parallel per-vertex RGB colors (nil colors when
// uncolored). Returns (nil, nil) for an unknown handle or a frame error.
func animationFrame(handle int32, timeMs float64) (positions []float32, colors []byte) {
	animMu.Lock()
	result := animSessions[handle]
	animMu.Unlock()
	if result == nil || result.Animation == nil {
		return nil, nil
	}
	solid, err := result.Animation.Frame(timeMs)
	if err != nil {
		return nil, nil
	}
	positions, colors, err = solidsToBuffers([]*manifold.Solid{solid})
	if err != nil {
		return nil, nil
	}
	return positions, colors
}

// closeAnimation releases the session registered under handle.
func closeAnimation(handle int32) {
	animMu.Lock()
	delete(animSessions, handle)
	animMu.Unlock()
}

// FacetOpenAnimation evaluates the .fct at cpath; if Main returns an Animation it
// returns a non-zero session handle, else 0 (the caller renders statically with
// FacetRenderFile instead). Release the handle with FacetCloseAnimation.
// A static .fct returns 0 here and is then rendered (re-evaluated) via
// FacetRenderFile — preview-only, accepted over caching the static mesh.
//
//export FacetOpenAnimation
func FacetOpenAnimation(cpath *C.char) C.int {
	src, err := os.ReadFile(C.GoString(cpath))
	if err != nil {
		return 0
	}
	h, ok := openAnimation(string(src))
	if !ok {
		return 0
	}
	return C.int(h)
}

// FacetAnimationFrame renders the session's frame at timeMs (ms) into malloc'd
// buffers, same ABI as FacetRenderFile (positions via FacetFree, colors via
// FacetFreeBytes). Returns NULL on a bad handle or frame error.
//
//export FacetAnimationFrame
func FacetAnimationFrame(handle C.int, timeMs C.double, outFloats *C.int, outColors **C.uchar, outColorBytes *C.int) *C.float {
	positions, colors := animationFrame(int32(handle), float64(timeMs))
	return emitBuffers(positions, colors, outFloats, outColors, outColorBytes)
}

// FacetCloseAnimation releases a session opened by FacetOpenAnimation.
//
//export FacetCloseAnimation
func FacetCloseAnimation(handle C.int) {
	closeAnimation(int32(handle))
}

func main() {}
