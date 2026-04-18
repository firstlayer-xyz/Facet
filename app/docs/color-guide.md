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
    var knob = Cylinder(radius: 3 mm, height: 8 mm)
        .MoveZ(z: 5 mm)
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

Internally, `.Color()` calls `AsOriginal()` on the Manifold, which assigns a unique `originalID` to all faces of the solid. The hex color is stored in a `ColorMap` keyed by that ID. After boolean operations, Manifold tracks face provenance through `runOriginalID` — each output triangle knows which input solid it came from. The viewport, 3MF exporter, and OBJ exporter all read `runOriginalID` to look up the correct color from the `ColorMap`.

Faces without an explicit color get the default theme color in the viewport, or no color attribute in exported files.
