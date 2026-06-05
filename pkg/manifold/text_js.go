//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

// CreateText renders a 2D text sketch from the bytes of a font file via the JS
// bridge (_mf_text → Emscripten FreeType). Mirrors the native CreateText: the
// embedded default font (or, later, an uploaded font) is supplied as bytes
// because the browser has no filesystem to read a font path from.
func CreateText(fontData []byte, text string, sizeMM float64, halign, valign string) (*Sketch, error) {
	if len(fontData) == 0 {
		return nil, fmt.Errorf("text(): no font data")
	}
	fontArr := js.Global().Get("Uint8Array").New(len(fontData))
	js.CopyBytesToJS(fontArr, fontData)
	id := js.Global().Call("_mf_text", fontArr, text, sizeMM, halign, valign).Int()
	if id == 0 {
		return nil, fmt.Errorf("text(): failed to render — invalid font or unknown halign=%q / valign=%q", halign, valign)
	}
	return newSketch(id), nil
}
