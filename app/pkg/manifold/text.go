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

func createText(fontPath string, text string, sizeMM float64) (*Sketch, error) {
	cPath := C.CString(fontPath)
	defer C.free(unsafe.Pointer(cPath))
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	ptr := C.facet_text_to_cross_section(cPath, cText, C.double(sizeMM))
	if ptr == nil {
		return nil, fmt.Errorf("failed to load font %q", fontPath)
	}
	return newSketch(ptr), nil
}

// CreateText creates a 2D text sketch from a font file.
func CreateText(fontPath, text string, sizeMM float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		return createText(fontPath, text, sizeMM)
	})
}
