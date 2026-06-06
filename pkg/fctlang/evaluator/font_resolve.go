//go:build !js

package evaluator

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/pkg/manifold"
)

// resolveFontBytes returns the bytes for a Text font: the embedded default when
// font is nil, otherwise the contents of the file at the given path (absolute,
// or resolved against the working directory).
func resolveFontBytes(font *string) ([]byte, error) {
	if font == nil {
		return manifold.DefaultFontData(), nil
	}
	path := *font
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("_text(): failed to get working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("_text(): failed to read font %q: %w", *font, err)
	}
	return data, nil
}
