package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// BOSL2's deferred-CSG layer: diff()/intersect() combine their descendants by
// $tag rather than unioning them. Because tags are lexical (source-level tag()
// wrappers), the partition is resolved at transpile time — no runtime tag query:
//
//	diff(remove, keep)      -> (∪untagged − ∪remove)     ∪ keep
//	intersect(intersect, keep) -> (∪untagged ∩ ∪intersect) ∪ keep
//
// `keep` is unioned back LAST so it survives the subtraction/intersection. A
// scope is walked (collect) into role buckets: tags propagate down, affine
// transforms distribute over the buckets, and an attachable parent splits so its
// tagged attach-children surface to the scope — so a remove cuts every untagged
// shape in the scope, not just its own parent (cross-parent removal).

// tagRole is the CSG role a (possibly tagged) child plays in a diff()/intersect().
type tagRole int

const (
	roleUntagged  tagRole = iota
	roleRemove            // subtracted (diff)
	roleIntersect         // intersected (intersect)
	roleKeep              // unioned back last
)

// peelTags strips a scope child's tag()/tag_this()/force_tag() wrappers and
// returns the inner statement plus its effective tag (the innermost set wins, as
// in BOSL2 where $tag is the nearest enclosing tag). A tag wrapping a group rather
// than a single call is returned whole, so e.stmt unions the group under that tag.
func peelTags(c ast.Stmt) (ast.Stmt, string) {
	tag := ""
	for {
		mc, ok := c.(*ast.ModuleCall)
		if !ok || (mc.Name != "tag" && mc.Name != "tag_this" && mc.Name != "force_tag") {
			return c, tag
		}
		if v := tagValue(mc); v != "" {
			tag = v
		}
		inner, ok := singleChildCall(mc)
		if !ok {
			return c, tag
		}
		c = inner
	}
}

// isCSGScope reports whether a module name opens a BOSL2 deferred-CSG scope.
func isCSGScope(name string) bool {
	switch name {
	case "diff", "intersect", "hide", "show_only":
		return true
	}
	return false
}

// containsCSGScope reports whether a statement subtree contains a diff/intersect/
// hide/show_only call.
func containsCSGScope(s ast.Stmt) bool {
	mc, ok := s.(*ast.ModuleCall)
	if !ok {
		return false
	}
	if isCSGScope(mc.Name) {
		return true
	}
	for _, c := range mc.Children {
		if containsCSGScope(c) {
			return true
		}
	}
	return false
}

// rejectNestedCSG errors if a CSG scope contains another diff/intersect/hide/
// show_only in its children. BOSL2's nested-tag resolution does NOT compose as
// simple nesting (verified vs OpenSCAD: a diff nested in a diff drops the outer
// remove), so rather than emit wrong geometry we reject it. Returns false (and
// records the error) when nesting is found.
func (e *Emitter) rejectNestedCSG(n *ast.ModuleCall) bool {
	for _, c := range n.Children {
		if containsCSGScope(c) {
			e.errf(n.Pos(), "%s: a nested diff/intersect/hide/show_only is not supported (BOSL2's nested-tag semantics don't compose)", n.Name)
			return false
		}
	}
	return true
}

// collect walks one scope child and appends its world-space geometry to buckets by
// CSG role. This is what makes removes/keeps cut/survive ACROSS parents: a tagged
// piece anywhere in the scope surfaces up to the scope's bucket rather than being
// baked into its own parent. suffix is the trailing transform chain accumulated
// from enclosing affine transforms; role is the tag role inherited from above.
func (e *Emitter) collect(stmt ast.Stmt, suffix string, role tagRole, cfg map[string]tagRole, buckets map[tagRole][]string) {
	mc, ok := stmt.(*ast.ModuleCall)
	if !ok {
		e.bucketOpaque(stmt, suffix, role, cfg, buckets)
		return
	}
	// A tag wrapper sets the role for its subtree.
	if mc.Name == "tag" || mc.Name == "tag_this" || mc.Name == "force_tag" {
		if r, ok := cfg[tagValue(mc)]; ok {
			role = r
		}
		for _, c := range mc.Children {
			e.collect(c, suffix, role, cfg, buckets)
		}
		return
	}
	// An affine transform distributes over the tag-partition: prepend its method
	// suffix and recurse into its children (Move/Rotate/Scale/Mirror/Color/Trim all
	// distribute over a union, so applying the suffix per piece is exact).
	if sfx, ok := e.transformSuffix(mc); ok {
		for _, c := range mc.Children {
			e.collect(c, sfx+suffix, role, cfg, buckets)
		}
		return
	}
	// An attachable parent splits: base (parent + same-role children) plus its
	// differently-tagged children surfaced as standalone placed solids.
	if isAttachableParent(mc) {
		e.collectAttachable(mc, suffix, role, cfg, buckets)
		return
	}
	e.bucketOpaque(mc, suffix, role, cfg, buckets)
}

// bucketOpaque emits a node that the walker doesn't descend (a leaf shape, a
// distributor, a hull, an extrude, a user module) whole, bucketed by its uniform
// role. A subtree with MIXED roles inside such a container can't be partitioned —
// that's a located error, never a silent miscompile.
func (e *Emitter) bucketOpaque(stmt ast.Stmt, suffix string, role tagRole, cfg map[string]tagRole, buckets map[tagRole][]string) {
	set := map[tagRole]bool{}
	collectRoles(stmt, role, cfg, set)
	if len(set) > 1 {
		name := "construct"
		if mc, ok := stmt.(*ast.ModuleCall); ok {
			name = mc.Name
		}
		e.errf(stmt.Pos(), "%s: mixed tags inside it cannot be partitioned in a diff/intersect scope", name)
		return
	}
	r := role
	for rr := range set {
		r = rr
	}
	if x := e.stmt(stmt); x != "" {
		buckets[r] = append(buckets[r], x+suffix)
	}
}

// collectRoles gathers the CSG roles of every geometry leaf in a subtree (tags set
// the role descending). A single role means the subtree is uniform.
func collectRoles(stmt ast.Stmt, role tagRole, cfg map[string]tagRole, set map[tagRole]bool) {
	mc, ok := stmt.(*ast.ModuleCall)
	if !ok {
		set[role] = true
		return
	}
	if mc.Name == "tag" || mc.Name == "tag_this" || mc.Name == "force_tag" {
		if r, ok := cfg[tagValue(mc)]; ok {
			role = r
		}
	}
	if len(mc.Children) == 0 {
		set[role] = true
		return
	}
	for _, c := range mc.Children {
		collectRoles(c, role, cfg, set)
	}
}

// isAttachableParent reports whether a call is a BOSL2 attachment parent
// (cuboid/cyl) carrying attachment children — the case collectAttachable splits.
func isAttachableParent(mc *ast.ModuleCall) bool {
	return len(mc.Children) > 0 && (mc.Name == "cuboid" || mc.Name == "cyl")
}

// collectAttachable splits an attachable parent within a CSG scope: the parent and
// its same-role children form the base solid, while differently-tagged children
// are surfaced as standalone placed solids (b2_parent(size).<method>Placed(...))
// so the enclosing diff/intersect applies them across the whole scope.
func (e *Emitter) collectAttachable(mc *ast.ModuleCall, suffix string, role tagRole, cfg map[string]tagRole, buckets map[tagRole][]string) {
	parent, ok := e.bosl2PrimitiveB2(mc)
	if !ok {
		e.errf(mc.Pos(), "%s cannot carry attachments", mc.Name)
		return
	}
	e.usesBosl2Runtime = true
	base := parent
	type placed struct {
		role tagRole
		expr string
	}
	var surfaced []placed
	for _, c := range mc.Children {
		spec, ok := e.b2LinkSpec(c, cfg, role)
		if !ok {
			continue
		}
		if spec.role == role {
			base += spec.union()
		} else {
			surfaced = append(surfaced, placed{spec.role, parent + spec.placed()})
		}
	}
	buckets[role] = append(buckets[role], base+".Solid()"+suffix)
	for _, s := range surfaced {
		buckets[s.role] = append(buckets[s.role], s.expr+suffix)
	}
}

// transformSuffix returns the trailing method chain an affine transform appends
// after its child (e.g. ".Move(x: 8 mm)" for right(8)), extracted by running the
// transform's own emitter over a sentinel child — so the walker reuses every
// existing transform emitter with no duplicated suffix logic. ok is false for a
// call that isn't a simple distributing child→suffix transform.
func (e *Emitter) transformSuffix(n *ast.ModuleCall) (string, bool) {
	if len(n.Children) == 0 || !isAffineWrapper(n.Name) {
		return "", false
	}
	const sentinel = "\x00C\x00"
	save := e.probeChild
	e.probeChild = sentinel
	out := e.moduleCall(n)
	e.probeChild = save
	rest, found := strings.CutPrefix(out, sentinel)
	if !found || strings.Contains(rest, sentinel) {
		return "", false
	}
	return rest, true
}

// isAffineWrapper lists the transforms that emit `childExpr + a method suffix`
// where the method distributes over a union (so it can be lifted onto each tagged
// piece). resize is excluded — it scales to a bounding box and does not distribute.
func isAffineWrapper(name string) bool {
	switch name {
	case "translate", "rotate", "scale", "mirror", "color":
		return true
	case "up", "down", "left", "right", "back", "fwd", "move",
		"xrot", "yrot", "zrot", "rot", "xscale", "yscale", "zscale", "recolor",
		"xflip", "yflip", "zflip",
		"top_half", "bottom_half", "left_half", "right_half", "front_half", "back_half", "half_of":
		return true
	}
	return false
}

// csgPartition emits a diff()/intersect() scope: walk the children into role
// buckets, then `base op core` (subtract/intersect) with `keep` unioned back last.
// op is "-" (diff) or "&" (intersect).
func (e *Emitter) csgPartition(n *ast.ModuleCall, cfg map[string]tagRole, op string) string {
	if !e.rejectNestedCSG(n) {
		return ""
	}
	buckets := map[tagRole][]string{}
	for _, c := range n.Children {
		e.collect(c, "", roleUntagged, cfg, buckets)
	}
	untagged := buckets[roleUntagged]
	if len(untagged) == 0 {
		return e.errf(n.Pos(), "%s has no base (untagged) geometry", n.Name)
	}
	core := buckets[roleRemove]
	if op == "&" {
		core = buckets[roleIntersect]
		if len(core) == 0 {
			return e.errf(n.Pos(), "intersect() has no intersect-tagged geometry to intersect with")
		}
	}
	out := parenthesizeIfOperator(unionParts(untagged))
	if len(core) > 0 {
		out += " " + op + " " + parenthesizeIfOperator(unionParts(core))
	}
	if kept := buckets[roleKeep]; len(kept) > 0 {
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
	return e.csgPartition(n, map[string]tagRole{remove: roleRemove, keep: roleKeep}, "-")
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
	return e.csgPartition(n, map[string]tagRole{isect: roleIntersect, keep: roleKeep}, "&")
}

