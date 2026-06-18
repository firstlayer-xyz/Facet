package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// isBoolean reports whether a module named `name` is an OpenSCAD boolean
// combinator (union/difference/intersection) over its children.
func isBoolean(name string) bool {
	switch name {
	case "union", "difference", "intersection":
		return true
	}
	return false
}

// boolean emits a boolean combination of children. union joins with ` + `,
// difference with ` - ` (first child minus the rest), and intersection with
// ` & `. Facet's `+ - &` are left-associative binary operators that bind looser
// than method calls, so a part like `B.Move(...)` combines correctly without
// parens. A part that is itself a top-level boolean expression (e.g. a nested
// union `b + c`) must be parenthesized so precedence stays correct: difference
// of `a` and `union(b, c)` must read `a - (b + c)`, not `a - b + c`.
func (e *Emitter) boolean(n *ast.ModuleCall) string {
	parts := e.childParts(n.Children)
	switch len(parts) {
	case 0:
		return e.errf(n.Pos(), "%s without children", n.Name)
	case 1:
		return parts[0]
	}
	if n.Name == "union" {
		return unionParts(parts)
	}
	op := boolOp(n.Name)
	return foldParts(parts, " "+op+" ")
}

// boolOp maps a boolean module name to its Facet operator.
func boolOp(name string) string {
	switch name {
	case "difference":
		return "-"
	case "intersection":
		return "&"
	}
	panic("boolOp: not a binary boolean: " + name)
}

// hull emits the convex hull of a module's children as `Hull(arr: [a, b, ...])`.
// Hull is overloaded on element type (Solid/Sketch/Vec3), so a homogeneous array
// resolves to the matching overload; mixed-dimensionality children would not
// type-check, but OpenSCAD's hull is likewise defined over uniform geometry.
func (e *Emitter) hull(n *ast.ModuleCall) string {
	parts := e.childParts(n.Children)
	if len(parts) == 0 {
		return e.errf(n.Pos(), "hull without children")
	}
	return "Hull(arr: " + childPartsArray(parts) + ")"
}

// childParts emits each child statement, dropping any that produce no geometry.
func (e *Emitter) childParts(children []ast.Stmt) []string {
	var parts []string
	for _, c := range children {
		if x := e.stmt(c); x != "" {
			parts = append(parts, x)
		}
	}
	return parts
}

// parenthesizeIfOperator wraps a part in parens when it contains a top-level
// boolean operator (` + `, ` - `, ` & `), so it can be safely used as an operand
// in a surrounding boolean expression. Operators nested inside parentheses,
// brackets, or braces are ignored — only a top-level operator forces wrapping.
func parenthesizeIfOperator(part string) string {
	if hasTopLevelBoolOp(part) {
		return "(" + part + ")"
	}
	return part
}

// hasTopLevelBoolOp reports whether `s` contains a ` + `, ` - `, or ` & `
// operator that is not nested inside any (), [], or {} group.
func hasTopLevelBoolOp(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '+', '-', '&':
			if depth == 0 && isSpaced(s, i) {
				return true
			}
		}
	}
	return false
}

// isSpaced reports whether the byte at index i is surrounded by spaces, marking
// it as a binary operator (` + `) rather than a unary sign or part of a token.
func isSpaced(s string, i int) bool {
	return i > 0 && i < len(s)-1 && s[i-1] == ' ' && s[i+1] == ' '
}

// childPartsArray renders childParts as a Facet array literal `[a, b, c]` for
// constructs that take an array of geometry (e.g. Hull).
func childPartsArray(parts []string) string {
	return "[" + strings.Join(parts, ", ") + "]"
}
