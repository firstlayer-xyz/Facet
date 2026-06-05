//go:build !js

package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// CreateText creates a 2D text sketch from the bytes of a font file.
//
// halign: "left", "center", or "right". Empty string means "left".
// valign: "baseline", "top", "center", or "bottom". Empty string means "baseline".
// An unknown value returns an error — alignment is part of the contract,
// not a default-on-mismatch convenience.
func CreateText(fontData []byte, text string, sizeMM float64, halign, valign string) (*Sketch, error) {
	if len(fontData) == 0 {
		return nil, fmt.Errorf("text(): no font data")
	}
	// Copy into C memory: FreeType (FT_New_Memory_Face) reads the buffer during
	// the call, kept entirely C-side to stay clear of cgo's pass-a-Go-pointer
	// rules.
	cFont := C.CBytes(fontData)
	defer C.free(cFont)
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	cHAlign := C.CString(halign)
	defer C.free(unsafe.Pointer(cHAlign))
	cVAlign := C.CString(valign)
	defer C.free(unsafe.Pointer(cVAlign))

	var ret C.FacetSketchRet
	C.facet_text_to_cross_section(
		(*C.char)(cFont), C.size_t(len(fontData)),
		cText, C.double(sizeMM), cHAlign, cVAlign, &ret)
	if ret.ptr == nil {
		return nil, fmt.Errorf("text(): failed to render — invalid font or unknown halign=%q / valign=%q", halign, valign)
	}
	return newSketch(ret), nil
}
