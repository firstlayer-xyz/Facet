package manifold

import (
	_ "embed"
	"os"
	"sync"
)

//go:embed fonts/Hack-Regular.ttf
var defaultFontData []byte

var defaultFontPath string
var defaultFontOnce sync.Once

// DefaultFontPath returns the filesystem path to the embedded default font.
// The font is written to a temp file on first call and cached for the process lifetime.
func DefaultFontPath() string {
	defaultFontOnce.Do(func() {
		f, err := os.CreateTemp("", "facet-font-*.ttf")
		if err != nil {
			return
		}
		if _, err := f.Write(defaultFontData); err != nil {
			f.Close()
			os.Remove(f.Name())
			return
		}
		if err := f.Close(); err != nil {
			os.Remove(f.Name())
			return
		}
		defaultFontPath = f.Name()
	})
	return defaultFontPath
}
