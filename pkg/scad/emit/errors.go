package emit

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// TranspileError is one OpenSCAD construct the transpiler could not translate.
// The transpiler never substitutes a placeholder for such a construct: it
// records the error and fails the transpile (see scad.Transpile).
type TranspileError struct {
	Feature string // e.g. "module 'minkowski'"
	Line    int
	Col     int
}

// ErrorList formats a set of TranspileErrors (sorted by source position) into a
// single error, or returns nil when the set is empty. Exported for scad.Transpile.
func ErrorList(path string, errs []TranspileError) error {
	if len(errs) == 0 {
		return nil
	}
	sorted := append([]TranspileError(nil), errs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Line != sorted[j].Line {
			return sorted[i].Line < sorted[j].Line
		}
		return sorted[i].Col < sorted[j].Col
	})
	var b strings.Builder
	fmt.Fprintf(&b, "scad: cannot translate %s:", path)
	for _, e := range sorted {
		fmt.Fprintf(&b, "\n  %d:%d: %s", e.Line, e.Col, e.Feature)
	}
	return errors.New(b.String())
}
