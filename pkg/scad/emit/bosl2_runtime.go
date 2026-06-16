package emit

// bosl2Runtime is the Facet source of the BOSL2 attachment runtime, injected as
// a preamble whenever a transpiled program uses BOSL2 attachments (see
// e.usesBosl2Runtime). It implements BOSL2's anchor/position/attach model in
// Facet rather than as Go string-building: B2 wraps a Solid with its centered
// bounding `size`, and the parent flows in as the method receiver (Facet is
// lexically scoped — this stands in for BOSL2's dynamic $parent_geom). Anchors
// are unitless directions, so they need their own type (Vec3 holds Lengths).
const bosl2Runtime = `# --- BOSL2 attachment runtime (emitted by the OpenSCAD transpiler) ---

# A unitless anchor direction; each component is -1, 0, or +1 (sums may exceed 1).
type B2Anchor {
    x Number
    y Number
    z Number
}

# An attachable shape: its solid plus the centered bounding extents used to
# resolve anchor points.
type B2 {
    solid Solid
    size  Vec3
}

fn b2_cuboid(
    size Vec3,
    fillet Length = 0 mm,
    chamfer Length = 0 mm,
    edges EdgeSet = EDGES_ALL,
) B2 {
    # Cube keeps fillet and chamfer as separate overloads, so dispatch to the
    # matching rounding builtin (at most one is non-zero — BOSL2 cuboid is
    # rounded OR chamfered, never both).
    var box = chamfer > 0 mm ?
        _cube_rounded(size.x, size.y, size.z, chamfer, true, edges) :
        fillet > 0 mm ?
            _cube_rounded(size.x, size.y, size.z, fillet, false, edges) :
            _cube(size.x, size.y, size.z)
    return B2{solid: box.AlignCenter(pos: Vec3{}), size: size}
}

fn b2_cyl(h Length, r Length, segments Number = 0) B2 {
    # segments == 0 means no $fn/$fa/$fs was set, so fall back to Facet's default
    # faceting (matching the non-attach path, which omits the segments argument).
    var cyl = segments > 0 ? Cylinder(r: r, h: h, segments: segments) : Cylinder(r: r, h: h)
    return B2{
        solid: cyl.AlignCenter(pos: Vec3{}),
        size: Vec3{x: r * 2, y: r * 2, z: h},
    }
}

fn b2_sphere(r Length, segments Number = 0) B2 {
    var sph = segments > 0 ? Sphere(r: r, segments: segments) : Sphere(r: r)
    return B2{solid: sph.AlignCenter(pos: Vec3{}), size: Vec3{x: r * 2, y: r * 2, z: r * 2}}
}

# The point on this shape's bounding box in anchor direction a.
fn B2.anchorPoint(a B2Anchor) Vec3 {
    return Vec3{x: self.size.x * a.x / 2, y: self.size.y * a.y / 2, z: self.size.z * a.z / 2}
}

# Final solid of an attachment chain.
fn B2.Solid() Solid {
    return self.solid
}

# The child solid placed at this shape's anchor point a (origin on the anchor).
fn B2.positionPlaced(a B2Anchor, child B2) Solid {
    return child.solid.Move(v: self.anchorPoint(a: a))
}

# Places child at this shape's anchor a; the Remove form subtracts it instead
# (BOSL2 diff() with a "remove" tag).
fn B2.position(a B2Anchor, child B2) B2 {
    return B2{solid: self.solid + self.positionPlaced(a: a, child: child), size: self.size}
}
fn B2.positionRemove(a B2Anchor, child B2) B2 {
    return B2{solid: self.solid - self.positionPlaced(a: a, child: child), size: self.size}
}

# The child solid placed so its ca anchor lands on this shape's pa anchor, rotated
# so the child's ca face points opposite pa (into this shape). The ca anchor is
# moved to the origin first, rotated there, then moved onto pa — so only the solid
# is rotated, never a point. When ca is anti-parallel to pa the rotation is the
# identity, leaving a pure translation. overlap pushes the child that far INTO this
# shape along the inward pa normal (BOSL2's overlap=).
fn B2.attachPlaced(pa B2Anchor, ca B2Anchor, child B2, overlap Length) Solid {
    var c = child.anchorPoint(a: ca)
    var p = self.anchorPoint(a: pa)
    var m = Sqrt(n: pa.x * pa.x + pa.y * pa.y + pa.z * pa.z)
    return child.solid.Move(v: -c)
        .Rotate(
            from: Vec3{x: ca.x * 1 mm, y: ca.y * 1 mm, z: ca.z * 1 mm},
            to: Vec3{x: pa.x * -1 mm, y: pa.y * -1 mm, z: pa.z * -1 mm}
        )
        .Move(v: Vec3{x: p.x - pa.x / m * overlap, y: p.y - pa.y / m * overlap, z: p.z - pa.z / m * overlap})
}
fn B2.attach(pa B2Anchor, ca B2Anchor, child B2, overlap Length) B2 {
    return B2{solid: self.solid + self.attachPlaced(pa: pa, ca: ca, child: child, overlap: overlap), size: self.size}
}
fn B2.attachRemove(pa B2Anchor, ca B2Anchor, child B2, overlap Length) B2 {
    return B2{solid: self.solid - self.attachPlaced(pa: pa, ca: ca, child: child, overlap: overlap), size: self.size}
}

# Rotates a solid so its +Z (UP) axis points along the anchor direction dir —
# any direction, including combined edge/corner anchors (e.g. TOP+RIGHT). The
# shortest-arc from/to rotation reduces to the same result as the per-axis euler
# turns for the six face anchors.
fn b2_orient_up_to(solid Solid, dir B2Anchor) Solid {
    return solid.Rotate(
        from: Vec3{x: 0 mm, y: 0 mm, z: 1 mm},
        to: Vec3{x: dir.x * 1 mm, y: dir.y * 1 mm, z: dir.z * 1 mm}
    )
}

# The child solid for a single-anchor attach: reoriented so its +Z axis points
# out the pa face, then CENTERED on that anchor point (BOSL2's 1-arg attach(P)
# aligns the child's CENTER to the parent anchor, not its base). overlap shifts it
# that far INTO this shape along the inward pa normal (pa may be a combined anchor,
# whose magnitude is √2 or √3, so the direction is normalized).
fn B2.attachReorientPlaced(pa B2Anchor, child B2, overlap Length) Solid {
    var oriented = b2_orient_up_to(solid: child.solid, dir: pa)
    var ap = self.anchorPoint(a: pa)
    var m = Sqrt(n: pa.x * pa.x + pa.y * pa.y + pa.z * pa.z)
    return oriented.Move(
        v: Vec3{
            x: ap.x - pa.x / m * overlap,
            y: ap.y - pa.y / m * overlap,
            z: ap.z - pa.z / m * overlap
        }
    )
}
fn B2.attachReorient(pa B2Anchor, child B2, overlap Length) B2 {
    return B2{solid: self.solid + self.attachReorientPlaced(pa: pa, child: child, overlap: overlap), size: self.size}
}
fn B2.attachReorientRemove(pa B2Anchor, child B2, overlap Length) B2 {
    return B2{solid: self.solid - self.attachReorientPlaced(pa: pa, child: child, overlap: overlap), size: self.size}
}

# The child placed flush against this shape's anchor face a: at the anchor point,
# pushed by half the child's size on each anchored axis so its near face sits on
# the face. Unlike attach, the child keeps its orientation and aligns by bounding
# box. dir is +1 to seat it outside the parent (the default) or -1 to seat it
# inside (BOSL2 align(inside=true), used under diff() to carve a pocket).
fn B2.alignPlaced(a B2Anchor, child B2, dir Number) Solid {
    var p = self.anchorPoint(a: a)
    return child.solid.Move(
        v: Vec3{
            x: p.x + a.x * dir * child.size.x / 2,
            y: p.y + a.y * dir * child.size.y / 2,
            z: p.z + a.z * dir * child.size.z / 2
        }
    )
}
fn B2.align(a B2Anchor, child B2, dir Number) B2 {
    return B2{solid: self.solid + self.alignPlaced(a: a, child: child, dir: dir), size: self.size}
}
fn B2.alignRemove(a B2Anchor, child B2, dir Number) B2 {
    return B2{solid: self.solid - self.alignPlaced(a: a, child: child, dir: dir), size: self.size}
}
`
