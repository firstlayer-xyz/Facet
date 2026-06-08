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

fn b2_cuboid(size Vec3) B2 {
    return B2{solid: Cube(x: size.x, y: size.y, z: size.z).AlignCenter(pos: Vec3{}), size: size}
}

fn b2_cyl(h Length, r Length) B2 {
    return B2{
        solid: Cylinder(r: r, h: h).AlignCenter(pos: Vec3{}),
        size: Vec3{x: r * 2, y: r * 2, z: h},
    }
}

fn b2_sphere(r Length) B2 {
    return B2{solid: Sphere(r: r).AlignCenter(pos: Vec3{}), size: Vec3{x: r * 2, y: r * 2, z: r * 2}}
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

# The child solid placed so its ca anchor lands on this shape's pa anchor (no
# reorientation: the caller guarantees ca is anti-parallel to pa).
fn B2.attachPlaced(pa B2Anchor, ca B2Anchor, child B2) Solid {
    var p = self.anchorPoint(a: pa)
    var c = child.anchorPoint(a: ca)
    return child.solid.Move(v: Vec3{x: p.x - c.x, y: p.y - c.y, z: p.z - c.z})
}
fn B2.attach(pa B2Anchor, ca B2Anchor, child B2) B2 {
    return B2{solid: self.solid + self.attachPlaced(pa: pa, ca: ca, child: child), size: self.size}
}
fn B2.attachRemove(pa B2Anchor, ca B2Anchor, child B2) B2 {
    return B2{solid: self.solid - self.attachPlaced(pa: pa, ca: ca, child: child), size: self.size}
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
# out the pa face, then its base sits on that anchor. The child's mating face
# lies on its rotation axis, so placement is anchorPoint(pa) plus a push of half
# the child's height along the UNIT pa direction (pa may be a combined anchor,
# whose magnitude is √2 or √3, so it must be normalized).
fn B2.attachReorientPlaced(pa B2Anchor, child B2) Solid {
    var oriented = b2_orient_up_to(solid: child.solid, dir: pa)
    var ap = self.anchorPoint(a: pa)
    var m = Sqrt(n: pa.x * pa.x + pa.y * pa.y + pa.z * pa.z)
    var hz = child.size.z / 2
    return oriented.Move(
        v: Vec3{
            x: ap.x + pa.x / m * hz,
            y: ap.y + pa.y / m * hz,
            z: ap.z + pa.z / m * hz
        }
    )
}
fn B2.attachReorient(pa B2Anchor, child B2) B2 {
    return B2{solid: self.solid + self.attachReorientPlaced(pa: pa, child: child), size: self.size}
}
fn B2.attachReorientRemove(pa B2Anchor, child B2) B2 {
    return B2{solid: self.solid - self.attachReorientPlaced(pa: pa, child: child), size: self.size}
}
`
