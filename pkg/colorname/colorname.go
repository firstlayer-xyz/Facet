// Package colorname maps common CSS color names to their "#RRGGBB" hex value.
// The table is shared between the SCAD transpiler (literal `color("red")`
// resolved at transpile time) and the Facet runtime (runtime resolution
// inside `Color(hex: name)` when a name reaches the builtin).
package colorname

import "strings"

// Hex returns the "#RRGGBB" hex string for a CSS color name. The second
// result is false for unknown names.
func Hex(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "black":
		return "#000000", true
	case "white":
		return "#FFFFFF", true
	case "red":
		return "#FF0000", true
	case "green":
		return "#008000", true
	case "lime":
		return "#00FF00", true
	case "blue":
		return "#0000FF", true
	case "yellow":
		return "#FFFF00", true
	case "cyan", "aqua":
		return "#00FFFF", true
	case "magenta", "fuchsia":
		return "#FF00FF", true
	case "gray", "grey":
		return "#808080", true
	case "silver":
		return "#C0C0C0", true
	case "maroon":
		return "#800000", true
	case "olive":
		return "#808000", true
	case "purple":
		return "#800080", true
	case "teal":
		return "#008080", true
	case "navy":
		return "#000080", true
	case "orange":
		return "#FFA500", true
	case "pink":
		return "#FFC0CB", true
	case "brown":
		return "#A52A2A", true
	// extended CSS3 keywords commonly used by OpenSCAD/BOSL2 models
	case "lightgray", "lightgrey":
		return "#D3D3D3", true
	case "darkgray", "darkgrey":
		return "#A9A9A9", true
	case "dimgray", "dimgrey":
		return "#696969", true
	case "gainsboro":
		return "#DCDCDC", true
	case "whitesmoke":
		return "#F5F5F5", true
	case "gold":
		return "#FFD700", true
	case "skyblue":
		return "#87CEEB", true
	case "lightblue":
		return "#ADD8E6", true
	case "darkblue":
		return "#00008B", true
	case "lightgreen":
		return "#90EE90", true
	case "darkgreen":
		return "#006400", true
	case "indigo":
		return "#4B0082", true
	case "violet":
		return "#EE82EE", true
	case "tan":
		return "#D2B48C", true
	case "beige":
		return "#F5F5DC", true
	case "ivory":
		return "#FFFFF0", true
	case "khaki":
		return "#F0E68C", true
	case "salmon":
		return "#FA8072", true
	case "coral":
		return "#FF7F50", true
	case "turquoise":
		return "#40E0D0", true
	case "crimson":
		return "#DC143C", true
	}
	return "", false
}
