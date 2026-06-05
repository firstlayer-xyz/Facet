package manifold

import _ "embed"

//go:embed fonts/Hack-Regular.ttf
var defaultFontData []byte

// DefaultFontData returns the bytes of the embedded default font (Hack), used
// when a Text call doesn't specify a custom font. CreateText takes font bytes
// directly, so this is shared by the native and wasm builds (the latter has no
// filesystem to write a temp font file to).
func DefaultFontData() []byte {
	return defaultFontData
}
