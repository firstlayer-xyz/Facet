# Cheat Sheet

A fast, scannable quick-reference for the standard library — what each thing is
called and the arguments you reach for most. For every overload, the full doc,
and worked examples, see the **API Reference** tab (it's auto-generated from the
stdlib source, so it's always authoritative if the two ever disagree).

**Every argument must be named** — `Circle(r: 5 mm)`, never `Circle(5 mm)`. The
checker rejects positional arguments with `…() arguments must be named`. (Names
also pick the right overload.)

## Quick start

```
fn Main(size Length = 20 mm where [5:50] mm) Solid {
    return Cube(s: size) - Sphere(r: size / 2).AlignCenter(pos: Vec3{})
}
```

- Any **uppercase** function is an entry point (shown in the run menu). It must
  be fully constrained — every parameter needs a default and a `where` clause —
  and must return `Solid`, `[]Solid`, `PolyMesh`, or `Animation`.
- **Units:** write `10 mm`, `2 cm`, `0.5 in`, `90 deg`, `1 rad`. A bare number
  means mm where a `Length` is expected.
- **Z-up, right-handed:** X = left/right (width), Y = front/back (depth),
  Z = down/up (height).
- Every primitive is **corner-anchored** — its bounding-box min corner sits at
  the origin (positive octant). Recenter with `.AlignCenter(pos: Vec3{})`.

## 3D primitives

| Call | Makes |
|------|-------|
| `Cube(s: 20 mm)` | Cube; also `Cube(x:, y:, z:)` or `Cube(s: Vec3{…})` |
| `Sphere(r: 10 mm)` | Sphere; `d:` and `segments:` also accepted |
| `Cylinder(r: 5 mm, h: 20 mm)` | Cylinder; `d:`, `segments:` (6 = hex prism) |
| `Cone(r: 8 mm, h: 20 mm)` | Cone |
| `Frustum(r1: 8 mm, r2: 3 mm, h: 20 mm)` | Truncated cone; `d1:`/`d2:` too |
| `Torus(r_maj: 20 mm, r_min: 4 mm)` | Torus; `d_maj:`/`d_min:` too |
| `Prism(s: 10 mm, h: 20 mm, sides: 6)` | Regular n-gon prism (`sides` ≥ 3) |
| `Prism(a:, b:, c:, h: 10 mm)` | Triangular prism from 3 `Vec2` corners |
| `Wedge(x: 20 mm, y: 12 mm, z: 8 mm)` | Right-triangular ramp |
| `Octahedron(r: 8 mm)` | Octahedron (square bipyramid) |

Rounded/beveled forms live under **Rounding & edges**.

## 2D shapes & sketches

| Call | Makes |
|------|-------|
| `Circle(r: 5 mm)` | Circle; `d:`, `segments:` (6 = hexagon) |
| `Square(s: 20 mm)` | Square; `Square(x:, y:)` for a rectangle |
| `Ngon(n: 6, r: 10 mm)` | Regular polygon on circumradius `r`; `d:` too |
| `Star(n: 5, r: 12 mm, ir: 5 mm)` | Star; outer `r`, inner `ir` (`d:`/`id:` too) |
| `Polygon(points: [a, b, c])` | Polygon from `[]Vec2`; add `holes:` to cut |
| `Text(text: "HI", s: 8 mm)` | Text sketch; `halign:`, `valign:`, `font:` |
| `Arc(center:, r:, startAngle:, endAngle:, segments:)` | `[]Vec2` along an arc |
| `Arc3d(center: Vec3, …)` | `[]Vec3` along an arc |

## Build 3D from 2D

| Call | Makes |
|------|-------|
| `sketch.Extrude(z: 20 mm)` | Extrude up +Z; `twist:`, `taperX:`, `taperY:`, `slices:` |
| `sketch.Revolve(a: 360 deg)` | Revolve around Z; `segments:` |
| `sketch.Sweep(path: [...])` | Sweep along a `[]Vec3` path |
| `Loft(profiles: [...], heights: [...])` | Blend cross-sections at heights |
| `solid.Slice(z: 5 mm)` | Cross-section at Z → `Sketch` |
| `solid.Project()` | Top-down silhouette → `Sketch` |

## Move, rotate, scale

`Solid` methods (each returns a new `Solid`). Unspecified axes default to 0; the
pivot/offset defaults to the origin (`Vec3{}`).

| Call | Does |
|------|------|
| `.Move(x: 5 mm, z: 2 mm)` | Translate; or `.Move(v: Vec3{…})` |
| `.Rotate(z: 45 deg)` | Euler X→Y→Z about `around:` (default origin) |
| `.Rotate(axis: Vec3{…}, angle: 30 deg)` | Rotate about an arbitrary axis |
| `.Rotate(from: Vec3{…}, to: Vec3{…})` | Rotate one direction onto another |
| `.Scale(v: 2)` | Uniform scale; `.Scale(x:, y:, z:)` per-axis; `around:` |
| `.Resize(size: Vec3{…})` | Scale to exact dimensions (0 = keep axis) |
| `.Mirror(x: 1)` | Reflect across a plane; `offset:` from origin |
| `.Trim(z: 1, offset: 5 mm)` | Cut away the negative side of a half-space |

`Sketch` mirrors most of these in 2D: `.Move`, `.Rotate(a:, around:)`,
`.Scale(x:, y:)`, `.Mirror(x:, y:, offset:)`.

## Align, stack & position

| Call | Does |
|------|------|
| `.MoveTo(pos: Vec3{…})` | Move min corner to a point; `.MoveTo(x:, y:, z:)` |
| `.AlignCenter(pos: Vec3{})` | Center the bbox on a point (recenter to origin) |
| `.AlignCenter(with: other)` | Center onto another solid; `x:`/`y:`/`z:`, `nudge*:` |
| `.AlignTop(with: other)` | Flush a face to another's; also Bottom/Left/Right/Front/Back |
| `.AlignTop(pos: 10 mm)` | Put a face at a world coordinate |
| `.StackOnTop(of: base)` | Sit flush on another, auto-centered; `nudge:` for a gap |
| `.StackOnRight(of: base)` | Also OnBottom/OnLeft/OnFront/OnBack |

## Combine: booleans & aggregates

```
a + b      # union
a - b      # difference (b removed from a)
a & b      # intersection
a | b      # insert: cut a hole for b, then seat it
a ^ b      # exclude: symmetric difference
```

`Solid` supports all five; `Sketch` supports `+`, `-`, `&`.

| Call | Does |
|------|------|
| `Union(arr: [...])` | N-ary union (`Solid` or `Sketch`) |
| `Difference(arr: [...])` | Subtract 2…n from the first |
| `Intersection(arr: [...])` | Keep the common overlap |
| `Hull(arr: [...])` | Convex hull of `Solid`/`Sketch`/`Vec3` |
| `a.Insert(part: b)` | Cut + seat a part (insert) |
| `a.Exclude(with: b)` | Symmetric difference (exclude) |
| `EvenOdd(solids: [...])` | Material inside an odd number of solids |
| `Compose(solids: [...])` | Merge known-disjoint solids (faster than Union) |
| `Decompose(s: solid)` | Split into disconnected components → `[]Solid` |
| `solid.Split(cutter: c)` | `[inside, outside]` |
| `solid.SplitByPlane(normal:, offset:)` | `[above, below]` |
| `solid.LinearPattern(count: 4, gap: 5 mm)` | Repeat along `axis:` (default +X); `length:` to fill a span |
| `solid.GridPattern(countX: 4, countY: 3, gap: Vec2{x: 2 mm, y: 2 mm})` | Tile a grid; `width,depth` to fill; `rowOffset` to stagger (0.5 = brick) |
| `solid.HexPattern(countX: 5, countY: 4, gap: 1 mm)` | Honeycomb packing; `width,depth` to fill |
| `solid.CircularPattern(count: 6)` | Repeat around Z; `span:` for an arc, `center:` to orbit a point |
| `Layout(solids: [...])` | Bin-pack onto the XY plane → `[]Solid`; `gap:` |

Build the array first with `for … yield` or `fold`, then union it:

```
return Union(arr: for i [0:<5] { yield Cube(s: 8 mm).Move(x: i * 12 mm) })
```

## Rounding & edges

`fillet` rounds, `chamfer` bevels — they are **separate overloads** (a shape is
rounded *or* beveled, never both).

| Call | Does |
|------|------|
| `Cube(s: 20 mm, fillet: 2 mm)` | Rounded box; `chamfer:` to bevel; `edges:` to limit |
| `Cylinder(r: 5 mm, h: 20 mm, fillet: 1 mm)` | Rounded rims; `chamfer:` too (also `Frustum`) |
| `sketch.Fillet(r: 2 mm)` | Round convex corners of a sketch |
| `sketch.Chamfer(distance: 2 mm)` | Bevel convex corners |
| `sketch.Offset(delta: 1 mm)` | Grow / shrink a sketch (negative shrinks) |
| `solid.Offset(delta: 2 mm)` | Grow / shrink any solid (SDF, approximate) |
| `solid.Round(r: 3 mm)` | Round convex edges (approximate) |
| `solid.Cove(r: 3 mm)` | Fillet concave/inside edges (approximate) |
| `solid.Smooth(minSharpAngle:, minSmoothness:)` | Relax edges; `Refine` first |
| `solid.Refine(n: 2)` | Subdivide every triangle (4× per pass) |

**Edge selection** for `Cube(…, edges:)` — start from a helper, combine with
`.Or()` / `.Except()`:

```
EDGES_ALL  EDGES_NONE
EdgesAlongX()  EdgesAlongY()  EdgesAlongZ()
TopEdges()  BottomEdges()  LeftEdges()  RightEdges()  FrontEdges()  BackEdges()
```

## Color

| Call | Does |
|------|------|
| `solid.Color(hex: "#cc3344")` | Tint all faces; `#RGB`/`#RRGGBB`/`#RRGGBBAA` |
| `solid.Color(r: 0.8, g: 0.2, b: 0.2, a: 1)` | Tint from 0–1 channels |
| `solid.Color(c: myColor)` | Tint from a `Color` value |
| `Color(hex: "#cc3344")` | Build a `Color`; `Color(r:, g:, b:, a:)` too |
| `color.Hex()` | `Color` → `"#RRGGBB"` |

## Measure & inspect

| Call | Returns |
|------|---------|
| `solid.Bounds()` | `Box` (`.min`, `.max`) |
| `solid.Width()` / `.Depth()` / `.Height()` | Extents (X / Y / Z) |
| `solid.Center()` | Bbox center `Vec3` |
| `solid.Left()` … `solid.Top()` | Face coordinates (min/max per axis) |
| `solid.LeftFrontBottom()` … `RightBackTop()` | The 8 corner points |
| `solid.Volume(unit: 1 cm)` | Enclosed volume as a `Number` |
| `solid.SurfaceArea(unit: 1 cm)` | Surface area as a `Number` |
| `solid.Genus()` | Topological genus (0 = sphere-like) |
| `solid.MinGap(with: other, reach: 10 mm)` | Closest distance between two solids |

`Box` has `.Width/.Height/.Depth`, `.Left…Top`, `.Center()`, `.ContainsPoint(p:)`,
`.ContainsBox(other:)`, `.Overlaps(other:)`, `.Union(other:)`.

## Math

| Call | Returns |
|------|---------|
| `Sin(a:)` `Cos(a:)` `Tan(a:)` | `Number` (angle in) |
| `Asin(n:)` `Acos(n:)` | `Angle` |
| `Atan2(y:, x:)` | `Angle` (full circle) |
| `Min(a:, b:)` `Max(a:, b:)` `Abs(a:)` | Same type (Number/Length/Angle) |
| `Sqrt(n:)` `Pow(base:, exp:)` | `Number` |
| `Floor(n:)` `Ceil(n:)` `Round(n:)` | `Number` |
| `Lerp(from:, to:, t:)` | Interpolated value |
| `Number(from: x)` | Raw value (Length→mm, Angle→deg, String→parsed) |
| `String(a: x)` | String form |
| `Size(of: arr)` | Element / character count |
| `IndexOf(arr:, value:)` | `Number?` (first match) |
| `IndicesOf(arr:, value:)` | `[]Number` (all matches) |
| `FindIndex(arr:, pred:)` | `Number?` (first matching a lambda) |
| `UtcDate()` `UtcTime()` | Current UTC date / time string |

## Strings & text

`String` methods: `.SubStr(start:, length:)`, `.HasPrefix(prefix:)`,
`.HasSuffix(suffix:)`, `.Split(delimiter:)`, `.Match(pattern:)` → `[]String?`,
`.ToUpper()`, `.ToLower()`, `.Trim()`, `.Replace(old:, new:)`,
`.IndexOf(substr:)` → `Number?`, `.Contains(substr:)`, `.Length()`.

Concatenate with `+`. Make 2D lettering with `Text(text:, s:)` then `.Extrude(…)`.

## Vectors

| Call | Makes |
|------|-------|
| `Vec3{x: 1 mm, y: 2 mm, z: 3 mm}` | Struct literal (also `Vec2{x:, y:}`) |
| `Vec3(v: 5 mm)` | All components equal; `Vec2(v:)` too |
| `Vec3(xy: v2, z: 1 mm)` | Lift a `Vec2` into 3D |
| `Dot(a:, b:)` | Dot product → `Number` |
| `Cross(a:, b:)` | Cross product → `Vec3` |
| `Length(v: vec)` | Magnitude → `Length` |
| `Normalize(v: vec)` | Unit vector |

Arithmetic: `+`, `-`, `*` (by `Number` or component-wise), `/`, unary `-`.

## Mesh, import & advanced

| Call | Does |
|------|------|
| `LoadMesh(path: "part.stl")` | Import STL/OBJ/3MF as a `Solid` |
| `solid.Mesh()` | → `Mesh` (`.vertices` `[]Vec3`, `.indices` `[]Face`) |
| `Mesh{vertices:, indices:}.Solid()` | Build a solid from raw mesh data |
| `mesh.FaceNormals()` / `.VertexNormals()` | `[]Vec3` normals |
| `solid.PolyMesh()` | → `PolyMesh` (N-gon faces) |
| `solid.Warp(f: fn(Vec3) Vec3 {…})` | Deform every vertex (refine first) |
| `LevelSet(f:, bounds:, edgeLength:)` | Build a solid from an SDF |

`PolyMesh` Conway operators (chainable): `.Dual()`, `.Ambo()`, `.Kis()`,
`.Truncate()`, `.Expand()`, `.Snub()`; plus `.Canonicalize()`,
`.ScaleToRadius(r:)`, `.ScaleUniform(factor:)`, `.Solid()`.

## Constants

| Name | Value |
|------|-------|
| `PI` / `π` | 3.14159… |
| `TAU` / `τ` | 6.28318… |
| `E` | 2.71828… |
| `Vec3{}` | Origin (0,0,0) — the default rotate/scale/mirror pivot |
| `EDGES_ALL` / `EDGES_NONE` | All / no box edges (for `Cube(…, edges:)`) |

## Gotchas

- **Corner-anchored primitives.** A fresh `Sphere`/`Cube`/… sits in the positive
  octant, not centered. Recenter with `.AlignCenter(pos: Vec3{})`, or place it
  with `.Move` / `.MoveTo`.
- **`Vec3{}` is the origin** and the default pivot — there is no `WorldOrigin`.
- **Radius or diameter.** Round primitives take `r:` *or* `d:` (and `Frustum`
  takes `r1:`/`r2:` or `d1:`/`d2:`). Pick one.
- **fillet ≠ chamfer.** They're separate overloads; you can't pass both.
- **Units are dimensioned.** `Length / Length` → `Number`, but `Length * Length`
  is a compile error (no area type). Ratios: `1/2 mm` = `0.5 mm`, while
  `1 / 2 mm` is `Number ÷ Length` (an error).
- **Build collections, then combine.** Use `for … yield` or `fold` to make a
  `[]Solid`, then `Union(arr:)` / `Hull(arr:)` — don't try to mutate an outer
  local from inside a loop body.
- **Arguments must be named.** Every stdlib call needs `name: value` arguments;
  a positional argument is a compile error. Names also select the right overload.
