# Color Guide

Facet supports per-face colors on solids. Colors survive boolean operations (union, difference, intersection) and are preserved through export to 3MF, OBJ, and the viewport.

## Applying Colors

Use `.Color()` on any solid. Two forms are available:

```facet
# Hex string (recommended)
var red_box = Cube(s: 10 mm).Color(hex: "#FF0000")

# RGB floats (0-1 per channel)
var blue_box = Cube(s: 10 mm).Color(c: Color(r: 0, g: 0, b: 1))
```

Hex strings support `#RGB`, `#RRGGBB`, and `#RRGGBBAA` formats.

## Multi-Color Models

Each `.Color()` call marks the entire solid with that color. To create multi-color models, color individual parts before combining them:

```facet
fn Main() {
    var base = Cube(x: 20 mm, y: 20 mm, z: 5 mm).Color(hex: "#333333")
    var knob = Cylinder(r: 3 mm, h: 8 mm)
        .Move(z: 5 mm)
        .Color(hex: "#FF0000")
    return base + knob
}
```

Colors are tracked per-face through boolean operations. When two colored solids are unioned, each resulting face retains the color of the solid it came from.

## Export Formats

| Format | Color Support |
|--------|---------------|
| 3MF    | Per-face color via colorgroup resources. Compatible with BambuStudio and PrusaSlicer. |
| OBJ    | Per-face color via `.mtl` material file with `usemtl`/`Kd` directives. |
| STL    | No color support (format limitation). |

## How It Works

Internally, `.Color()` does not create new face IDs — the unique `originalID` for each face is assigned at solid construction (in C++, via `AsOriginal()`, when the primitive, hull, or level-set Manifold is created). `.Color()` simply applies an identity transform to produce a fresh Manifold with a copied face map, then writes the chosen color into each existing entry. The color is stored in the solid's `FaceMap` (`map[uint32]FaceInfo`) keyed by that ID, where each `FaceInfo` holds a packed `0xRRGGBB` `Color` (uint32) plus an 8-bit `Alpha` — not a hex string. After boolean operations, Manifold tracks face provenance through `runOriginalID` — each output triangle knows which input solid it came from. At mesh-extract time these IDs are resolved back to per-face hex from the `FaceMap`: the viewport path builds a `FaceColorMap` (a per-face hex map), while the 3MF and OBJ exporters derive per-triangle hex via `runTriangleHex` — both reading color from the same `FaceMap`.

Faces without an explicit color get the default theme color in the viewport, or no color attribute in exported files.
