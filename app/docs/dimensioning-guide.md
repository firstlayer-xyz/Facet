# Dimensioning Guide

This guide covers on-model dimensioning in the 3D viewer. You can measure distances, diameters, and angles directly on your geometry — no code required. Dimensions live in the viewer; they are not part of your Facet source.

## Getting Started

Dimensions are placed by picking points on the model. Two buttons in the viewport toolbar drive the workflow:

- **Measure** — enter placement mode. Click two snap targets to place a dimension. Press again (or hit **Esc**) to exit.
- **Extents** — one-shot: draws the axis-aligned bounding box of the current model with overall width × depth × height labelled.

You can also press **M** to toggle Measure mode from the keyboard.

## Placing a Dimension

1. Press **Measure** (or **M**).
2. Move the cursor over the model. A live preview follows the cursor, showing what would be measured from the point you're hovering.
3. Click to lock in the **first** point. A small marker appears at that snap.
4. Move to a second location. The preview shows the dimension between your locked point and the current hover.
5. Click again to place the dimension. The marker moves to the new point and the chain continues — each subsequent click adds another linear dimension from the previous point and, once three points exist, a corner-angle at the shared vertex.
6. Press **Esc** to end the chain. The placed dimensions stay; the next click will start a fresh chain.
7. Press **Esc** again or click **Measure** again to exit Measure mode entirely.

Measurements render in your current theme's text color, attached to the geometry, with the value shown on a small on-model label that always faces the camera.

## Snap Targets

The viewer tries to snap your pick to geometrically meaningful points rather than the raw raycast hit. Snap candidates are evaluated in priority order and the first one within **12 pixels** of the cursor wins:

1. **Circle center** — if the cursor is near a circular edge loop (detected automatically from the mesh's face-group boundary edges), the pick snaps to the center of that circle. This is how you measure the diameter of a hole, boss, or cylinder.
2. **Vertex** — any of the three corners of the hovered triangle.
3. **Edge endpoint** — the ends of nearby edge segments.
4. **Perpendicular foot** — *only while placing the second point of a measurement.* If you've already locked in a first point and you hover over an edge, the pick snaps to the point on that edge that is perpendicular to your first point. This is how you measure the shortest distance from a feature to an edge. Takes priority over midpoint when both would be valid.
5. **Edge midpoint** — the midpoint of nearby edge segments.
6. **Face centroid** — the fallback: the centroid of the face group under the cursor, with the face's normal attached for angular measurements.

The hover preview reflects whatever the current snap would be, so you can see what you're about to lock in before clicking. A small glyph on the model shows both **what** snap is active and **which** category it belongs to:

| Glyph | Snap |
|-------|------|
| Corner ("L" of two stubs with a dot) | Vertex |
| Single stub with a dot at the end | Edge endpoint |
| Line with a dot in the middle | Edge midpoint |
| Right-angle bracket with a dot at the corner | Perpendicular foot |
| Outlined square with a centered dot | Face centroid |
| Circle with a crosshair | Circle center |

Each glyph is colored from the current theme palette so it reads against any background.

When the snap is edge-based (endpoint or midpoint), the full edge segment is highlighted so you can see **which** edge the snap resolved to — useful when several edges are close together. When the snap is a circle center, the whole circular loop is highlighted.

## Measurement Kinds

The kind of dimension produced depends on what you picked:

**Radial (diameter)** — single click on a circular edge. The snap resolver lands on the circle center and the dimension reports the **diameter** (prefixed with ∅) as a chord drawn through the center in the circle's plane. Use this for holes, bosses, and cylinder ends.

**Linear** — any pair of point-like snaps (vertex, edge endpoint, edge midpoint, or one face centroid paired with a point). The dimension is the straight 3D distance between the two snaps. Use this for feature-to-feature measurements: hole spacing, flange thickness, corner-to-corner.

**Face-to-face** — two face-centroid picks on planar faces whose normals are parallel (within 1°). The dimension reports the perpendicular distance between the two planes — so you can measure wall thickness or slot width by clicking one face and then the opposite face.

**Angular** — two face-centroid picks on planar faces whose normals are **not** parallel. The viewer draws rays along each face's normal from the midpoint between the picks, plus an arc between them, and labels the angle in degrees (0.1° precision).

**Corner angle** — when chained dimensioning produces three-or-more points, the shared vertex between two consecutive dimension segments is annotated with an arc + angle label showing the interior angle between the two dim lines. Angles are shown only where dim lines meet each other, not where a dim line meets an object edge.

**Extents** — the bounding-box shortcut. Not click-placed: just press the **Extents** button and the axis-aligned overall size appears as a dashed box with its dimensions in the label.

Face-centroid picks against **curved** surfaces (cylinders, fillets, spheres) still return a centroid and average normal, but angular and face-to-face measurements refuse to use them — you'll fall back to a linear distance between the centroids.

## Managing Dimensions

- **Clear all** — the trash-icon button next to Measure wipes every placed dimension in one sweep.
- **Esc** — exits Measure mode. Placed dimensions remain; only the in-progress pick (if any) is cancelled.

## Units

Labels show either millimetres or inches depending on the **Measurement Units** setting (Settings → Editor → Measurement). Metric is the default and formats to 3 decimal places, trailing zeros stripped (e.g. `12.5 mm`, `0.125 mm`). Angles are always shown in degrees regardless of the units setting.

When **Imperial** is selected, two additional controls take effect:

- **Imperial Format** — `Fraction` (reduced fractions like `1 1/4"`, `3/8"`, `2"`) or `Decimal` (decimal inches to 3 dp, trailing zeros stripped: `1.25"`, `0.125"`).
- **Fraction Denominator** — the precision for fraction mode. Any power of two from `1/4"` through `1/128"`. Values are rounded to the nearest tick and the fraction is then reduced, so at denominator `1/64"` a true length of 0.5" displays as `1/2"`, not `32/64"`.

Changes to any of these settings take effect immediately — all placed dimensions re-label without being replaced.

## Dimensions Reset on Re-eval

Dimensions are **transient**: every time your code is evaluated and a new mesh is loaded, all placed dimensions are cleared. This is deliberate — face and edge identities are not stable across edits, so a dimension placed on "that hole" before the edit may land on a completely different feature after it. Rather than silently re-anchoring to the wrong thing, the viewer drops dimensions on mesh reload.

If you want to keep a set of measurements, take a screenshot before your next run.

## Tips

- Snap priority is your friend. To measure a hole's diameter, don't try to click two points across the rim — hover near the circular edge and a single click snaps to the center and places a radial dimension.
- For thickness measurements, click the two opposing faces. If the faces are parallel the result is the true perpendicular thickness, not the distance between the centroids.
- For the shortest distance from a feature to an edge, pick the feature first, then hover the edge — the second snap will land on the perpendicular foot, not on the edge's midpoint.
- For the angle between two dimension lines, keep clicking — each chained click adds a dim segment, and the angle between consecutive segments is labelled at the shared vertex.
- For angles between features, click two planar faces; the arc is drawn at a scale proportional to the distance between the picks, so the result stays readable on both small and large models.
- If a pick doesn't land where you expected, the hover preview will show the snap candidate before you commit — move the cursor a few pixels to nudge between vertex, edge, and centroid.
