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

# Places child so its origin sits at this shape's anchor point a.
fn B2.position(a B2Anchor, child B2) B2 {
    return B2{solid: self.solid + child.solid.Move(v: self.anchorPoint(a: a)), size: self.size}
}

# Mates child's ca anchor onto this shape's pa anchor (no reorientation: the
# caller guarantees ca is anti-parallel to pa).
fn B2.attach(pa B2Anchor, ca B2Anchor, child B2) B2 {
    var p = self.anchorPoint(a: pa)
    var c = child.anchorPoint(a: ca)
    return B2{solid: self.solid + child.solid.Move(v: Vec3{x: p.x - c.x, y: p.y - c.y, z: p.z - c.z}), size: self.size}
}
`
