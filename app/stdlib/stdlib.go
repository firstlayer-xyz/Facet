package stdlib

import "embed"

//go:embed libraries
var Libraries embed.FS

//go:embed libraries/facet/std/std.fct
var StdlibSource string
