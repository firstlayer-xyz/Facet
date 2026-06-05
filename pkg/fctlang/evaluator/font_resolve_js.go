//go:build js

package evaluator

import (
	"fmt"

	"facet/pkg/manifold"
)

// resolveFontBytes returns the embedded default font. The browser preview has
// no filesystem, so a custom font path is a clear error rather than a silent
// failure (font uploads will supply bytes directly in a later change).
func resolveFontBytes(font *string) ([]byte, error) {
	if font == nil {
		return manifold.DefaultFontData(), nil
	}
	return nil, fmt.Errorf("_text(): custom fonts aren't supported in the browser preview yet — only the built-in font (got %q)", *font)
}
