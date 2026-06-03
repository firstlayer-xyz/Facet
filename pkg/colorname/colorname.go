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
	}
	return "", false
}
