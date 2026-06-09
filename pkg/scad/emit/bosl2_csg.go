package emit

import "facet/pkg/scad/ast"

// BOSL2's deferred-CSG layer: diff()/intersect() combine their descendants by
// $tag rather than unioning them. Because tags are lexical (source-level tag()
// wrappers), the partition is resolved at transpile time — no runtime tag query:
//
//	diff(remove, keep)      -> (∪untagged − ∪remove)     ∪ keep
//	intersect(intersect, keep) -> (∪untagged ∩ ∪intersect) ∪ keep
//
// `keep` is unioned back LAST so it survives the subtraction/intersection. The
// attachable-parent case (a tag("remove") attach-child of an untagged shape) is
// still handled inside that shape's attachment chain via e.inDiff; here we only
// partition the scope's own children, and the two compose.

// tagRole is the CSG role a (possibly tagged) child plays in a diff()/intersect().
type tagRole int

const (
	roleUntagged  tagRole = iota
	roleRemove            // subtracted (diff)
	roleIntersect         // intersected (intersect)
	roleKeep              // unioned back last
)

// isTagCall reports whether name is a BOSL2 tag wrapper.
func isTagCall(name string) bool {
	return name == "tag" || name == "tag_this" || name == "force_tag"
}

// collectTagged appends each leaf of a diff()/intersect() scope child to the
// untagged/core/kept lists by its effective CSG role. It descends through
// tag()/tag_this()/force_tag() wrappers — INCLUDING a tag wrapping a group of
// children — so an inner tag overrides an outer one (BOSL2 propagates a tag to all
// descendants until re-tagged); e.g. tag("remove"){ R; tag("keep") K; } subtracts
// R but keeps K. role carries the nearest enclosing recognized tag. A non-tag node
// is a leaf, emitted whole under the current role (so an attachable parent still
// resolves its own tag("remove") attach-children via e.inDiff). A tag nested below
// a transform/union is the one remaining gap (see the package doc).
func (e *Emitter) collectTagged(c ast.Stmt, role tagRole, cfg map[string]tagRole, untagged, core, kept *[]string) {
	if mc, ok := c.(*ast.ModuleCall); ok && isTagCall(mc.Name) {
		if r, ok := cfg[tagValue(mc)]; ok {
			role = r
		}
		for _, ch := range mc.Children {
			e.collectTagged(ch, role, cfg, untagged, core, kept)
		}
		return
	}
	x := e.stmt(c)
	if x == "" {
		return
	}
	switch role {
	case roleRemove, roleIntersect:
		*core = append(*core, x)
	case roleKeep:
		*kept = append(*kept, x)
	default:
		*untagged = append(*untagged, x)
	}
}

// csgPartition emits a diff()/intersect() scope: partition the children by tag,
// then `base op core` (subtract/intersect) with `keep` unioned back last. op is
// "-" (diff) or "&" (intersect). subtractive sets e.inDiff while emitting so an
// untagged attachable parent still subtracts its own tag("remove") attach-children.
func (e *Emitter) csgPartition(n *ast.ModuleCall, cfg map[string]tagRole, op string, subtractive bool) string {
	prev := e.inDiff
	e.inDiff = subtractive
	var untagged, core, kept []string
	for _, c := range n.Children {
		e.collectTagged(c, roleUntagged, cfg, &untagged, &core, &kept)
	}
	e.inDiff = prev

	if len(untagged) == 0 {
		return e.errf(n.Pos(), "%s has no base (untagged) geometry", n.Name)
	}
	if op == "&" && len(core) == 0 {
		return e.errf(n.Pos(), "intersect() has no intersect-tagged geometry to intersect with")
	}
	out := parenthesizeIfOperator(unionParts(untagged))
	if len(core) > 0 {
		out += " " + op + " " + parenthesizeIfOperator(unionParts(core))
	}
	if len(kept) > 0 {
		out = "(" + out + ") + " + parenthesizeIfOperator(unionParts(kept))
	}
	return out
}

// csgTag reads an optional tag-name argument (a string literal, named or
// positional) for diff()/intersect(), defaulting to def. A non-string is a
// located error (no silent fallback).
func (e *Emitter) csgTag(n *ast.ModuleCall, name string, idx int, def string) string {
	v, has := arg(n, name, idx)
	if !has {
		return def
	}
	s, ok := v.(*ast.Str)
	if !ok {
		e.errf(n.Pos(), "%s: %s tag must be a string literal", n.Name, name)
		return def
	}
	return s.Value
}

// bosl2Diff emits BOSL2's diff(remove="remove", keep="keep"): the remove-tagged
// geometry is subtracted from the untagged geometry, then keep-tagged geometry is
// unioned back. Custom remove/keep tag names are supported (positional or named).
func (e *Emitter) bosl2Diff(n *ast.ModuleCall) string {
	remove := e.csgTag(n, "remove", 0, "remove")
	keep := e.csgTag(n, "keep", 1, "keep")
	e.rejectExtraArgs(n, 2, "remove", "keep")
	if remove == keep {
		return e.errf(n.Pos(), "diff(): the remove and keep tags must differ")
	}
	return e.csgPartition(n, map[string]tagRole{remove: roleRemove, keep: roleKeep}, "-", true)
}

// bosl2Intersect emits BOSL2's intersect(intersect="intersect", keep="keep"): the
// intersect-tagged geometry is intersected with the untagged geometry, then
// keep-tagged geometry is unioned back. Custom tag names are supported.
func (e *Emitter) bosl2Intersect(n *ast.ModuleCall) string {
	isect := e.csgTag(n, "intersect", 0, "intersect")
	keep := e.csgTag(n, "keep", 1, "keep")
	e.rejectExtraArgs(n, 2, "intersect", "keep")
	if isect == keep {
		return e.errf(n.Pos(), "intersect(): the intersect and keep tags must differ")
	}
	return e.csgPartition(n, map[string]tagRole{isect: roleIntersect, keep: roleKeep}, "&", false)
}
