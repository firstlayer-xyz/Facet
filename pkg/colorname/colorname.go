// Package colorname maps common CSS color names to their "#RRGGBB" hex value.
// The table is shared between the SCAD transpiler (literal `color("red")`
// resolved at transpile time) and the Facet runtime (runtime resolution
// inside `Color(hex: name)` when a name reaches the builtin).
package colorname

import "strings"

// hexByName maps a lowercased CSS color name to its "#RRGGBB" hex value.
var hexByName = map[string]string{
	"black":   "#000000",
	"white":   "#FFFFFF",
	"red":     "#FF0000",
	"green":   "#008000",
	"lime":    "#00FF00",
	"blue":    "#0000FF",
	"yellow":  "#FFFF00",
	"cyan":    "#00FFFF",
	"aqua":    "#00FFFF",
	"magenta": "#FF00FF",
	"fuchsia": "#FF00FF",
	"gray":    "#808080",
	"grey":    "#808080",
	"silver":  "#C0C0C0",
	"maroon":  "#800000",
	"olive":   "#808000",
	"purple":  "#800080",
	"teal":    "#008080",
	"navy":    "#000080",
	"orange":  "#FFA500",
	"pink":    "#FFC0CB",
	"brown":   "#A52A2A",
	// extended CSS3 keywords commonly used by OpenSCAD/BOSL2 models
	"lightgray":  "#D3D3D3",
	"lightgrey":  "#D3D3D3",
	"darkgray":   "#A9A9A9",
	"darkgrey":   "#A9A9A9",
	"dimgray":    "#696969",
	"dimgrey":    "#696969",
	"gainsboro":  "#DCDCDC",
	"whitesmoke": "#F5F5F5",
	"gold":       "#FFD700",
	"skyblue":    "#87CEEB",
	"lightblue":  "#ADD8E6",
	"darkblue":   "#00008B",
	"lightgreen": "#90EE90",
	"darkgreen":  "#006400",
	"indigo":     "#4B0082",
	"violet":     "#EE82EE",
	"tan":        "#D2B48C",
	"beige":      "#F5F5DC",
	"ivory":      "#FFFFF0",
	"khaki":      "#F0E68C",
	"salmon":     "#FA8072",
	"coral":      "#FF7F50",
	"turquoise":  "#40E0D0",
	"crimson":    "#DC143C",
}

// Hex returns the "#RRGGBB" hex string for a CSS color name. The second
// result is false for unknown names.
func Hex(name string) (string, bool) {
	v, ok := hexByName[strings.ToLower(name)]
	return v, ok
}
