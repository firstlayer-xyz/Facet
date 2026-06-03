package emit

import (
	"fmt"
	"strings"
)

// kind is the Facet type a numeric expression should render as.
type kind int

const (
	kNumber kind = iota // bare Number
	kLength             // append " mm"
	kAngle              // append " deg"
)

// writer accumulates Facet source text.
type writer struct {
	sb strings.Builder
}

func (w *writer) str() string               { return w.sb.String() }
func (w *writer) write(s string)            { w.sb.WriteString(s) }
func (w *writer) writef(f string, a ...any) { fmt.Fprintf(&w.sb, f, a...) }
