// Package examples ships the bundled .fct examples with the binary.
package examples

import (
	"embed"
	_ "embed"
)

//go:embed *.fct
var FS embed.FS

//go:embed Tutorial.fct
var DefaultSource string
