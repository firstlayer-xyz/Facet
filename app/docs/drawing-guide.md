# Drawing Guide

This guide covers 2D drawing in Facet. Sketches are flat shapes that live in the XY plane — you can combine, transform, and refine them before extruding into 3D solids.

## Basic Shapes

The simplest sketches are built-in primitives.

**Rectangle:**

```
fn Main() {
    return Square(x: 30 mm, y: 20 mm).Extrude(z: 5 mm)
}
```

`Square(x, y)` creates a rectangle with its bounding-box min corner at the origin (extends from (0,0) to (x,y)). `Square(size)` creates a uniform square.

**Circle:**

```
fn Main() {
    return Circle(r: 15 mm).Extrude(z: 5 mm)
}
```

`Circle(radius)` creates a circle whose bounding-box min corner sits at the origin — the circle extends from (0,0) to (2r, 2r), with its center at (r, r). Use `.Move(x: -r, y: -r)` if you want it center-aligned on the origin.

**Low-segment polygon trick:** You can approximate a circle with fewer segments using `Polygon` and trigonometry to create hexagons, octagons, etc. See the Drawing with Points section below.

## Drawing with Points

Use `Vec2` and `Polygon` for custom shapes. A polygon needs at least 3 points:

**Triangle:**

```
fn Main() {
    var tri = Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 20 mm, y: 0 mm},
        Vec2{x: 10 mm, y: 15 mm},
    ])
    return tri.Extrude(z: 5 mm)
}
```

**Star:**

```
fn Main() {
    var pts = for i [0:<10] {
        var angle = i * 36 deg
        var r = 7 mm
        if i % 2 == 0 { r = 15 mm }
        yield Vec2{x: Cos(a: angle) * r, y: Sin(a: angle) * r}
    }
    return Polygon(points: pts).Extrude(z: 3 mm)
}
```

**L-shape:**

```
fn Main() {
    var shape = Polygon(points: [
        Vec2{x: 0 mm, y: 0 mm},
        Vec2{x: 20 mm, y: 0 mm},
        Vec2{x: 20 mm, y: 10 mm},
        Vec2{x: 10 mm, y: 10 mm},
        Vec2{x: 10 mm, y: 30 mm},
        Vec2{x: 0 mm, y: 30 mm},
    ])
    return shape.Extrude(z: 5 mm)
}
```

**Regular hexagon (low-segment polygon):**

```
fn Main() {
    var hex = Polygon(points: for i [0:<6] {
        var angle = i * 60 deg
        yield Vec2{x: Cos(a: angle) * 10 mm, y: Sin(a: angle) * 10 mm}
    })
    return hex.Extrude(z: 5 mm)
}
```

## Arcs

`Arc` generates an array of points along a circular arc. Use it with `Polygon` to create curved profiles.

```
Arc(center: Vec2, r: Length, startAngle: Angle, endAngle: Angle, segments: Number)
```

- `center` — `Vec2`, the center of the arc
- `radius` — `Length`, the arc radius
- `startAngle`, `endAngle` — `Angle`, sweep range
- `segments` — `Number`, how many line segments to use

**Semicircle tab:**

```
fn Main() {
    var origin = Vec2{x: 0 mm, y: 0 mm}
    var arcPts = Arc(center: origin, r: 10 mm, startAngle: 0 deg, endAngle: 180 deg, segments: 16)
    return Polygon(points: arcPts).Extrude(z: 3 mm)
}
```

**Combining arcs with straight segments:**

```
fn Main() {
    var origin = Vec2{x: 0 mm, y: 0 mm}
    var arc = Arc(center: origin, r: 10 mm, startAngle: 0 deg, endAngle: 90 deg, segments: 16)
    var pts = arc + [Vec2{x: 0 mm, y: 10 mm}, Vec2{x: 0 mm, y: 0 mm}]
    return Polygon(points: pts).Extrude(z: 5 mm)
}
```

You can concatenate arrays of points with `+` to mix arcs and straight edges.

**3D arcs:** `Arc3d` works the same way but produces `Vec3` values — useful as paths for `Sweep`:

```
fn Main() {
    var path = Arc3d(center: Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, r: 20 mm, startAngle: 0 deg, endAngle: 270 deg, segments: 32)
    return Circle(r: 2 mm).Sweep(path: path)
}
```

## Text

`Text` creates a sketch from a string. The second argument is the font size:

```
fn Main() {
    return Text(text: "FACET", s: 12 mm).Extrude(z: 2 mm)
}
```

Use a custom `.ttf` or `.otf` font file:

```
fn Main() {
    return Text(text: "Hello", s: 10 mm, font: "/path/to/font.ttf").Extrude(z: 2 mm)
}
```

Text sketches can be combined with other sketches using boolean operations — for example, engraving text into a plate:

```
fn Main() {
    var plate = Square(x: 60 mm, y: 20 mm).Extrude(z: 3 mm)
    var label = Text(text: "TAG", s: 8 mm)
        .Move(x: 5 mm, y: 6 mm)
        .Extrude(z: 3 mm)
    return plate - label
}
```

## Transforming Sketches

Sketches support the same transformation methods as solids, but in 2D.

**Move** — move in X and Y:

```
Circle(r: 5 mm).Move(x: 10 mm, y: 5 mm)
```

**Rotate** — rotate around a pivot point:

```
Square(x: 10 mm, y: 5 mm).Rotate(a: 45 deg, around: Vec2{x: 0 mm, y: 0 mm})
```

**Scale** — non-uniform 2D scaling around a pivot:

```
Circle(r: 10 mm).Scale(x: 2, y: 1, around: Vec2{x: 0 mm, y: 0 mm})    # ellipse
```

**Mirror** — mirror across an axis with an offset:

```
# Mirror across the Y axis
Square(x: 10 mm, y: 10 mm).Move(x: 5 mm, y: 0 mm).MirrorX(offset: 0 mm)
```

**Chaining** — transformations compose left to right:

```
Square(x: 10 mm, y: 5 mm)
    .Move(x: 5 mm, y: 0 mm)
    .Rotate(a: 30 deg, around: Vec2{x: 0 mm, y: 0 mm})
    .Scale(x: 1.5, y: 1, around: Vec2{x: 0 mm, y: 0 mm})
```

## Combining Sketches

Sketch booleans work the same as solid booleans:

| Operator | Description |
|----------|-------------|
| `+` | Union — merge two sketches |
| `-` | Difference — cut one sketch from another |
| `&` | Intersection — keep only the overlap |

**Plate with holes:**

```
fn Main() {
    var plate = Square(x: 40 mm, y: 30 mm)
    var hole = Circle(r: 3 mm)

    # Subtract four corner holes
    plate -= hole.Move(x: 5 mm, y: 5 mm)
    plate -= hole.Move(x: 35 mm, y: 5 mm)
    plate -= hole.Move(x: 5 mm, y: 25 mm)
    plate -= hole.Move(x: 35 mm, y: 25 mm)

    return plate.Extrude(z: 3 mm)
}
```

**Union two overlapping shapes:**

```
fn Main() {
    var body = Square(x: 20 mm, y: 10 mm)
    var tab = Circle(r: 5 mm).Move(x: 20 mm, y: 5 mm)
    return (body + tab).Extrude(z: 5 mm)
}
```

## Refining Edges

After constructing a sketch, you can round or bevel its corners.

**Offset** — grow or shrink a sketch uniformly:

```
fn Main() {
    var s = Square(x: 20 mm, y: 20 mm)
    var outer = s.Offset(delta: 2 mm)     # grow by 2mm
    var inner = s.Offset(delta: -2 mm)    # shrink by 2mm
    return (outer - inner).Extrude(z: 3 mm)
}
```

**Fillet** — round convex corners:

```
fn Main() {
    return Square(x: 20 mm, y: 20 mm).Fillet(r: 3 mm).Extrude(z: 5 mm)
}
```

**Chamfer** — bevel convex corners:

```
fn Main() {
    return Square(x: 20 mm, y: 20 mm).Chamfer(distance: 3 mm).Extrude(z: 5 mm)
}
```

## Patterns

Repeat a sketch in a line or around a center point.

**Linear pattern:**

```
fn Main() {
    var hole = Circle(r: 3 mm)
    var row = hole.LinearPattern(count: 5, spacing: Vec2{x: 10 mm})
    return (Square(x: 55 mm, y: 10 mm) - row.Move(x: 5 mm, y: 5 mm)).Extrude(z: 3 mm)
}
```

**Circular pattern:**

```
fn Main() {
    var petal = Circle(r: 5 mm).Move(x: 10 mm, y: 0 mm)

    # 6 copies evenly around the Z axis (world origin)
    var flower = petal.CircularPattern(count: 6)

    return (Circle(r: 4 mm) + flower).Extrude(z: 2 mm)
}
```

**Partial circular pattern:**

```
fn Main() {
    var dot = Circle(r: 2 mm).Move(x: 12 mm, y: 0 mm)
    var arc = dot.CircularPattern(count: 5, span: 180 deg)
    return arc.Extrude(z: 2 mm)
}
```

## Measuring

**Area** — get the 2D area of a sketch:

```
var a = Circle(r: 10 mm).Area()    # ~314.16 mm^2
```

**Bounding box** — get the extents:

```
var bb = Square(x: 20 mm, y: 10 mm).Bounds()
var w = bb.Width()     # 20 mm
var h = bb.Height()    # 10 mm
```

## From Sketch to Solid

Once you have a sketch, there are four ways to turn it into a 3D solid:

- **Extrude** — push the sketch straight up along Z. See the User's Guide for basic and twisted/tapered extrusions.
- **Revolve** — spin the sketch around the Y axis. The sketch must sit to the right of the Y axis.
- **Sweep** — extrude along a 3D path of `Vec3` values.
- **Loft** — blend between multiple sketches at different heights.

Each of these is covered in detail in the User's Guide under "2D to 3D".

## What's Next

You can combine everything in this guide to build complex 2D profiles — plates with mounting holes, decorative outlines, gaskets, gear blanks, and more. For reusable parts, check out Facet's built-in libraries: threads, gears, fasteners, knurling, and others. Load a library with `var T = lib "facet/threads"` and start building on what others have made.
