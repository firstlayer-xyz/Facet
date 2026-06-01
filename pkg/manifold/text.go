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

// CreateText creates a 2D text sketch from a font file.
//
// halign: "left", "center", or "right". Empty string means "left".
// valign: "baseline", "top", "center", or "bottom". Empty string means "baseline".
// An unknown value returns an error — alignment is part of the contract,
// not a default-on-mismatch convenience.
func CreateText(fontPath, text string, sizeMM float64, halign, valign string) (*Sketch, error) {
	cPath := C.CString(fontPath)
	defer C.free(unsafe.Pointer(cPath))
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	cHAlign := C.CString(halign)
	defer C.free(unsafe.Pointer(cHAlign))
	cVAlign := C.CString(valign)
	defer C.free(unsafe.Pointer(cVAlign))

	var ret C.FacetSketchRet
	C.facet_text_to_cross_section(cPath, cText, C.double(sizeMM), cHAlign, cVAlign, &ret)
	if ret.ptr == nil {
		return nil, fmt.Errorf("text(): failed to render — invalid font %q or unknown halign=%q / valign=%q", fontPath, halign, valign)
	}
	return newSketch(ret), nil
}
