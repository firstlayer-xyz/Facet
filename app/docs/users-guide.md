# User's Guide

Welcome to Facet — a desktop CAD application where you write code to describe 3D models. This guide will walk you through the language from your first cube to parametric assemblies.

## Quick Reference

Common patterns you can copy directly into your program:

| Goal | Code |
|------|------|
| Simple box | `Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})` |
| Uniform cube | `Cube(s: 10 mm)` |
| Sphere | `Sphere(radius: 8 mm)` |
| Cylinder | `Cylinder(bottom: 5 mm, top: 5 mm, height: 20 mm)` |
| Extrude a profile | `Circle(radius: 5 mm).Extrude(height: 20 mm)` |
| Revolve a profile | `Circle(radius: 3 mm).Move(x: 10 mm, y: 0 mm).Revolve()` |
| Sweep along a path | `Circle(radius: 2 mm).Sweep(path: path)` |
| Loft between profiles | `Loft(profiles: [...], heights: [0 mm, 30 mm])` |
| Drill a hole | `box - Cylinder(radius: 3 mm, height: 30 mm)` |
| Move a shape | `.Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})` |
| Rotate a shape | `.Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})` |
| Mirror across YZ plane | `.Mirror(nx: 1, ny: 0, nz: 0, offset: 0 mm)` |
| Repeat in a line | `.LinearPattern(count: 4, spacing: Vec3{x: 10 mm})` |
| Repeat in a ring | `.CircularPattern(count: 6)` |
| Fillet a profile | `sketch.Fillet(radius: 2 mm).Extrude(height: 5 mm)` |
| Arc of points | `Arc(center: Vec2{x: 0 mm, y: 0 mm}, radius: 10 mm, startAngle: 0 deg, endAngle: 90 deg, segments: 16)` |
| Interactive slider | `r Length = 5 mm where [1:20] mm` (on fn params) |
| Dropdown selector | `s String = "m3" where ["m3", "m4", "m5"]` (on fn params) |
| Load a library | `var T = lib "facet/threads"` |
| Get bounding box | `solid.Bounds().Width()` or `solid.Width()` |
| Convex hull | `Hull(arr: [a, b, c])` |

## Your First Model

Write a function that returns a `Solid` and give it a name starting with a capital letter:

```
fn Main() {
    return Cube(s: 20 mm)
}
```

This creates a 20mm cube. Any function whose name starts with a capital letter is an **entry point** — it appears in the run menu and can be previewed independently. `Main` is a common convention, but you can name it anything: `Bracket`, `Gear`, `Housing`.

Functions require an explicit `return` statement to return a value.

Click **Run** (or press Ctrl+Enter / Cmd+Enter) to see the result in the 3D viewport.

## Primitive Shapes

### 3D Primitives

Facet provides three basic 3D shapes:

```
# Axis-aligned box: width (X), depth (Y), height (Z)
Cube(s: Vec3{x: 30 mm, y: 10 mm, z: 20 mm})
Cube(x: 30 mm, y: 10 mm, z: 20 mm)    # shorthand
Cube(s: 10 mm)                       # uniform cube

# Sphere by radius
Sphere(radius: 15 mm)

# Cylinder: bottom radius, top radius, height
Cylinder(bottom: 10 mm, top: 10 mm, height: 20 mm)   # straight cylinder
Cylinder(bottom: 10 mm, top: 5 mm, height: 20 mm)    # cone (different radii)
Cylinder(bottom: 10 mm, top: 0 mm, height: 20 mm)    # pointed cone
Cylinder(radius: 10 mm, height: 20 mm)                # shorthand (equal radii)
```

`Cube` spans from the origin to (x, y, z) — it is NOT centered. `Sphere` and `Circle` are centered at the origin. `Cylinder` has its base at Z=0.

### 2D Primitives (Sketches)

2D shapes are called **Sketches**. They live in the XY plane and can be extruded into 3D solids:

```
Square(x: 20 mm, y: 10 mm)        # rectangle
Square(s: 15 mm)                # uniform square
Circle(radius: 15 mm)             # circle by radius
Polygon(points: [                  # polygon from points
    Vec2{x: 0 mm, y: 0 mm},
    Vec2{x: 20 mm, y: 0 mm},
    Vec2{x: 10 mm, y: 15 mm},
])
```

You can also generate arc-shaped point sequences with `Arc`:

```
# 90-degree arc with 16 segments
var pts = Arc(center: Vec2{x: 0 mm, y: 0 mm}, radius: 10 mm, startAngle: 0 deg, endAngle: 90 deg, segments: 16)
```

See the Drawing Guide for more on 2D shapes and sketch operations.

## Units

### Length Units

Numbers followed by a unit keyword become `Length` values. All lengths are stored as millimeters internally.

**Common units:**

| Unit | Alias | Equivalent |
|------|-------|------------|
| `mm` | `millimeter` | 1 mm |
| `cm` | `centimeter` | 10 mm |
| `m` | `meter` | 1,000 mm |
| `in` | `inch` | 25.4 mm |
| `ft` | `foot` | 304.8 mm |
| `yd` | `yard` | 914.4 mm |

Bare numbers (without a unit) default to millimeters:

```
var x = 10;       # Number (unitless)
var y = 10 mm;    # Length (10 mm)
```

Bare numbers are accepted where a `Length` is expected, so `Cube({10, 10, 10})` works and means 10mm x 10mm x 10mm.

### Angle Units

| Unit | Alias | Equivalent |
|------|-------|------------|
| `deg` | `degree` | 1 degree |
| `rad` | `radian` | 180/pi degrees |
| `turn` | `rev` | 360 degrees |
| `grad` | `gon` | 0.9 degrees |

Angles accumulate without wrapping — `360 deg + 90 deg` is `450 deg`, which is important for helical extrusions.

### Arithmetic with Units

```
# Length arithmetic
var total = 10 mm + 2 cm;         # 30 mm
var half = total / 2;              # 15 mm (Length / Number -> Length)
var ratio = 10 mm / 5 mm;         # 2 (Length / Length -> Number)

# Angle arithmetic
var a = 90 deg + 1/4 turn;        # 180 deg
```

## Transformations

Every Solid has methods to move, rotate, scale, and mirror it. Transformations return a new shape — the original is unchanged.

### Move

```
# Move 10mm in X, 5mm in Y, 0mm in Z
Cube(s: 10 mm).Move(v: Vec3{x: 10 mm, y: 5 mm, z: 0 mm})
```

You can also pass a `Vec3`. Subtracting two points gives a vector, so this is useful for computing displacements:

```
var a = Vec3{x: 10 mm, y: 20 mm, z: 30 mm}
var b = Vec3{x: 3 mm, y: 5 mm, z: 10 mm}
Cube(s: 10 mm).Move(v: a - b)    # move by the displacement from b to a
```

Vectors support scaling and arithmetic:

```
var v = Vec3{x: 1 mm, y: 2 mm, z: 3 mm}
Cube(s: 10 mm).Move(v: v * 2)    # double the displacement
```

All axes default to 0, so you can move along a single axis:

```
Sphere(radius: 5 mm).Move(z: 10 mm)    # lift 10mm off the ground
```

### Rotate

Rotation is specified in degrees per axis, with an explicit pivot point (applied as Rx * Ry * Rz):

```
Cube(x: 20 mm, y: 10 mm, z: 5 mm).Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})   # 45 degrees around Z
```

`Rotate`, `Scale`, and `Mirror` always require an explicit pivot or offset — there is no implicit default.

### Scale

Non-uniform scaling by factor per axis, with a pivot point:

```
Cube(s: 10 mm).Scale(x: 2, y: 1, z: 0.5, around: Vec3{})   # stretch X, squash Z
```

### Mirror

Mirror across a plane defined by its normal vector, with an offset:

```
Cube(s: 10 mm).Mirror(nx: 1, ny: 0, nz: 0, offset: 0 mm)   # mirror across YZ plane
```

### Chaining Transformations

Methods can be chained since each returns a new Solid:

```
Cube(s: 10 mm)
    .Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})
    .Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})
    .Scale(x: 1, y: 1, z: 2, around: Vec3{})
```

## Boolean Operations

Combine shapes using boolean operators:

| Operator | Operation | Description |
|----------|-----------|-------------|
| `+` | Union | Merge two shapes together |
| `-` | Difference | Subtract right from left |
| `&` | Intersection | Keep only the overlap |
| `\|` | Insert | Cut hole for right, then seat it |
| `^` | Exclude | Symmetric difference (remove overlap) |

For arrays of shapes, use `Union(arr)`, `Difference(arr)`, or `Intersection(arr)`.

```
fn Main() {
    var cube = Cube(s: 20 mm)
    var sphere = Sphere(radius: 13 mm)

    # Union: merge both shapes
    var merged = cube + sphere

    # Difference: carve sphere from cube
    var carved = cube - sphere

    # Intersection: keep only the overlap
    var overlap = cube & sphere

    return carved
}
```

### Compound Assignment

Use `+=`, `-=`, `&=`, `|=`, and `^=` to modify a variable in place:

```
fn Main() {
    var shape = Cube(s: 20 mm)
    shape -= Sphere(radius: 13 mm)                        # carve out sphere
    shape += Cylinder(radius: 3 mm, height: 30 mm)        # add cylinder
    return shape
}
```

## 2D to 3D

### Extrude

Push a 2D Sketch upward along Z to create a Solid:

```
fn Main() {
    return Circle(radius: 10 mm).Extrude(height: 20 mm)
}
```

### Twisted and Tapered Extrusion

The advanced form: `Extrude(height, slices, twist, scaleX, scaleY)`:

```
fn Main() {
    # Twist a square 90 degrees over 30mm, tapering to half size
    return Square(x: 20 mm, y: 20 mm).Extrude(height: 30 mm, slices: 64, twist: 90 deg, scaleX: 0.5, scaleY: 0.5)
}
```

- `slices` — number of cross-section layers (more = smoother twist)
- `twist` — total rotation from bottom to top
- `scaleX`, `scaleY` — scale factors at the top (1 = no taper)

### Revolve

Spin a 2D Sketch around the Y axis:

```
fn Main() {
    # Full 360-degree revolution
    var a = Square(x: 5 mm, y: 20 mm).Move(x: 10 mm, y: 0 mm).Revolve()

    # Partial revolution (270 degrees)
    return Circle(radius: 5 mm).Move(x: 15 mm, y: 0 mm).Revolve(angle: 270 deg)
}
```

### Sweep

Extrude a 2D Sketch along a 3D path:

```
fn Main() {
    var path = [
        Vec3{x: 0 mm, y: 0 mm, z: 0 mm},
        Vec3{x: 10 mm, y: 0 mm, z: 5 mm},
        Vec3{x: 20 mm, y: 0 mm, z: 0 mm},
    ]
    return Circle(radius: 2 mm).Sweep(path: path)
}
```

The path is an array of `Vec3` with at least 2 points. Use `Arc3d` to generate curved paths:

```
fn Main() {
    var path = Arc3d(center: Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, radius: 20 mm, startAngle: 0 deg, endAngle: 180 deg, segments: 32)
    return Circle(radius: 3 mm).Sweep(path: path)
}
```

### Loft

Blend between two or more cross-sections at different heights:

```
fn Main() {
    return Loft(
        profiles: [Square(x: 20 mm, y: 20 mm), Circle(radius: 10 mm)],
        heights: [0 mm, 50 mm]
    )
}
```

The first array is the profiles (Sketches), the second is the heights (Lengths). Both arrays must be the same length, with at least 2 entries.

A three-section loft:

```
fn Main() {
    return Loft(
        profiles: [Circle(radius: 15 mm), Square(x: 10 mm, y: 10 mm), Circle(radius: 5 mm)],
        heights: [0 mm, 30 mm, 60 mm]
    )
}
```

## Variables and Parameters

### Basic Variables

Declare variables with `var`:

```
var size = 10 mm
var count = 5
var angle = 45 deg

fn Main() {
    return Cube(s: size)
}
```

Variables can be reassigned:

```
fn Main() {
    var x = 10 mm
    x = 20 mm
    x += 5 mm    # now 25 mm
    return Cube(s: x)
}
```

### Constrained Parameters

Add a constraint to a function parameter and it becomes an interactive control in the parameters panel. This is the preferred way to make your model configurable:

```
fn Main(
    # Number slider from 0 to 100
    count Number = 10 where [0:100],

    # Length slider from 1mm to 50mm
    size Length = 10 mm where [1:50] mm,

    # Angle slider from 0 to 360 degrees
    twist Angle = 0 deg where [0:360] deg,

    # Stepped slider (increments of 5)
    grid Number = 10 where [0:100:5],

    # Dropdown menu
    thread String = "m8" where ["m3", "m4", "m5", "m6", "m8", "m10"],
) Solid {
    ...
}
```

When you adjust a slider, Facet re-runs your program with the new value — giving you live parametric control over your model.

## Arrays and Loops

### Array Literals

Array element types are inferred from the elements:

```
var sizes = [5 mm, 10 mm, 15 mm, 20 mm]   # inferred as []Length
var nums = [1, 2, 3]                        # inferred as []Number
var first = sizes[0]                        # 5 mm (0-indexed)
var last = sizes[-1]                        # 20 mm (negative = from end)
var sub = sizes[1:3]                        # [10 mm, 15 mm] (slice)
var count = Size(a: sizes)                  # 4
```

If elements have mixed types, you'll get an error — use explicit typed arrays:

```
var points = []Vec3[{x: 1 mm, y: 0 mm, z: 0 mm}, {x: 0 mm, y: 1 mm, z: 0 mm}]
```

The explicit `[]Type[...]` syntax is required when elements are anonymous struct literals `{...}`, since the type can't be inferred from the fields alone.

### Range Expressions

```
var nums = [0:5];                 # [0, 1, 2, 3, 4, 5] (inclusive)
var excl = [0:<5];                # [0, 1, 2, 3, 4] (exclusive end)
var stepped = [0:10:2];           # [0, 2, 4, 6, 8, 10]
var down = [5:0];                 # [5, 4, 3, 2, 1, 0]
```

### For-Yield (Map/Collect)

Iterate and collect results into a new array:

```
# Create a row of spheres
fn Main() {
    var spheres = for i [0:<5] {
        yield Sphere(radius: 5 mm).Move(v: Vec3{x: i * 15 mm, y: 0 mm, z: 0 mm})
    }
    return fold a, b spheres { yield a + b }
}
```

Use `yield` to specify what value to collect from each iteration:

```
# Filter: only even indices
var evens = for i [0:<10] {
    if i % 2 != 0 { yield; }  # skip odd
    yield i;
};
```

### Enumerate

Access both the index and value:

```
var sizes = [5 mm, 10 mm, 15 mm]
var cubes = for i, size sizes {
    yield Cube(s: size).Move(v: Vec3{x: i * 25 mm, y: 0 mm, z: 0 mm})
}
```

### Cartesian Product (Multi-Variable For)

Nest multiple iterators to create grids:

```
fn Main() {
    var grid = for i [0:<3], j [0:<3] {
        yield Sphere(radius: 3 mm).Move(v: Vec3{x: i * 10 mm, y: j * 10 mm, z: 0 mm})
    }
    return fold a, b grid { yield a + b }
}
```

### Fold (Reduce)

Reduce an array to a single value. The first element is the initial accumulator:

```
# Sum numbers
var total = fold a, b [1, 2, 3, 4] { yield a + b };   # 10

# Union all solids in an array
var combined = fold a, b parts { yield a + b };
```

## Functions

### Defining Functions

```
fn RoundedBox(w, h, d, r Length) Solid {
    var box = Cube(s: Vec3{x: w, y: h, z: d})
    var sphere = Sphere(radius: r)
    return box & sphere
}

fn Main() {
    return RoundedBox(w: 20 mm, h: 20 mm, d: 20 mm, r: 12 mm)
}
```

Return types are optional — Facet can infer them:

```
fn RoundedBox(w, h, d, r Length) {
    return Cube(s: Vec3{x: w, y: h, z: d}) & Sphere(radius: r)
}
```

### Optional Parameters

Parameters with default values can be omitted at the call site. The type comes after the name, then the default: `name Type = default`.

```
fn Peg(r Length = 3 mm, h Length = 10 mm) Solid {
    return Cylinder(radius: r, height: h)
}

fn Main() {
    return Peg()              # uses defaults: r=3mm, h=10mm
}
```

### Recursion

```
fn Factorial(n Number) Number {
    if n <= 1 { return 1 } else { return n * Factorial(n: n - 1) }
}
```

## First-Class Functions

Functions are values. You can create an anonymous function (lambda) with `fn`:

```
var move = fn(s Solid) Solid { return s.Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm}) }
var result = move(Cube(s: 10 mm))
```

Lambdas capture variables from the enclosing scope:

```
fn Main() {
    var dx = 25 mm
    var shift = fn(s Solid) Solid { return s.Move(v: Vec3{x: dx, y: 0 mm, z: 0 mm}) }
    return shift(Sphere(radius: 10 mm))
}
```

Pass functions as arguments using `fn(Type) ReturnType` as the parameter type:

```
fn Apply(s Solid, f fn(Solid) Solid) Solid { return f(s) }

fn Main() {
    return Apply(s: Sphere(radius: 10 mm), f: fn(s Solid) Solid { return s.Move(x: 15 mm) })
}
```

### Warp

`.Warp()` deforms every vertex of a solid by a function that maps `Vec3 → Vec3`:

```
fn Main() {
    # Twist a cylinder by displacing vertices based on Z height
    return Cylinder(radius: 5 mm, height: 30 mm)
        .Refine(n: 3)
        .Warp(f: fn(v Vec3) Vec3 {
            var angle = Number(a: v.z) * 3 deg
            return Vec3{
                x: v.x * Cos(a: angle) - v.y * Sin(a: angle),
                y: v.x * Sin(a: angle) + v.y * Cos(a: angle),
                z: v.z
            }
        })
}
```

### LevelSet

`LevelSet` creates a solid from a signed-distance field (SDF). Points where the function returns `≤ 0` are inside the solid:

```
fn Main() {
    var bounds = Cube(s: 40 mm).Bounds()
    return LevelSet(
        f: fn(v Vec3) Number {
            # Sphere SDF of radius 15mm
            var x = Number(a: v.x)
            var y = Number(a: v.y)
            var z = Number(a: v.z)
            return Sqrt(n: x*x + y*y + z*z) - 15
        },
        bounds: bounds,
        edgeLength: 1 mm
    )
}
```

## Structs

Group related values into a named type:

```
type Dims {
    w Length
    h Length
    d Length
}

fn Main() {
    var box = Dims { w: 20 mm, h: 10 mm, d: 5 mm }
    return Cube(s: Vec3{x: box.w, y: box.h, z: box.d})
}
```

### Field Defaults

```
type Config {
    count Number = 6
    spacing Length = 10 mm
    rotation Angle = 0 deg
}

# Only override what you need
var cfg = Config { count: 12 };
```

### Methods on Structs

Define methods using `fn Type.Method()` syntax. Use `self` to access the receiver:

```
type Dims {
    w Length
    h Length
    d Length
}

fn Dims.ToCube() Solid {
    return Cube(s: Vec3{x: self.w, y: self.h, z: self.d})
}

fn Main() {
    var d = Dims { w: 20 mm, h: 10 mm, d: 5 mm }
    return d.ToCube()
}
```

### Anonymous Struct Literals

When calling a function that expects a struct, you can pass the fields directly without naming the type:

```
fn MakeBox(dims Dims) Solid {
    return Cube(s: Vec3{x: dims.w, y: dims.h, z: dims.d})
}

fn Main() {
    return MakeBox(dims: { w: 20 mm, h: 10 mm, d: 5 mm })
}
```

## Libraries

Libraries package reusable functions into separate namespaces.

### Loading a Library

```
var T = lib "facet/threads";

fn Main() {
    return T.Thread(size: "m8").Outside(length: 20 mm)
}
```

### Built-in Libraries

Facet ships with several built-in libraries:

| Library | Description |
|---------|-------------|
| `facet/threads` | ISO metric, UTS, and NPT thread generation |
| `facet/gears` | Involute spur gear profiles |
| `facet/knurling` | Diamond crosshatch grip texture |
| `facet/fasteners` | Hex bolts, socket head cap screws, nuts, standoffs |
| `facet/lego` | Lego-compatible brick geometry |
| `facet/keys` | House key models (Kwikset, Schlage) |
| `facet/fuzzy` | Fuzzy skin texture via vertex displacement |

### Remote Libraries

You can also load libraries from Git repositories:

```
var F = lib "github.com/user/repo/libname@main";
```

## If/Else

`if` is a statement, not an expression. Use it for conditional logic:

```
var size = 10 mm
if count > 10 { size = 5 mm }
```

Chained:

```
var size = 20 mm
if label == "small" {
    size = 5 mm
} else if label == "medium" {
    size = 10 mm
}
```

`return` inside an `if` branch exits the enclosing function:

```
fn Clamp(v, min, max Length) Length {
    if v < min { return min }
    if v > max { return max }
    return v
}
```

Assignments inside `if` branches propagate to the enclosing scope. Variables declared with `var` inside a branch are block-local.

## Bounding Boxes

Query the size and position of any Solid or Sketch:

```
fn Main() {
    var s = Cube(x: 20 mm, y: 10 mm, z: 30 mm)
    var bb = s.Bounds()

    var w = bb.Width()        # 20 mm (X extent)
    var d = bb.Depth()        # 10 mm (Y extent)
    var h = bb.Height()       # 30 mm (Z extent)
    var c = bb.Center()       # Vec3 at (10 mm, 5 mm, 15 mm)

    return s
}
```

`Box` also has `min` and `max` fields (both `Vec3`). Methods: `Width()`, `Height()`, `Depth()`, `Center()`, `ContainsPoint()`, `ContainsBox()`, `Overlaps()`, `Union()`.

Because querying dimensions is so common, `Width()`, `Height()`, `Depth()`, and `Center()` are also available directly on `Solid` and `Sketch` as shortcuts for `.Bounds().Width()` etc. (`Height()` is only available on `Solid`, not `Sketch`):

```
var s = Cube(x: 20 mm, y: 10 mm, z: 30 mm)
s.Width()     # 20 mm  (X extent, same as s.Bounds().Width())
s.Depth()     # 10 mm  (Y extent)
s.Height()    # 30 mm  (Z extent)
s.Center()    # Vec3 at (10 mm, 5 mm, 15 mm)
```

## Patterns

Repeat shapes along a line or around an axis:

```
fn Main() {
    var peg = Cylinder(radius: 3 mm, height: 10 mm)

    # 5 copies spaced 15mm apart along X
    var row = peg.LinearPattern(count: 5, spacing: Vec3{x: 15 mm})

    # 8 copies evenly around Z axis (full 360 degrees)
    var ring = peg.CircularPattern(count: 8)

    # 6 copies over 180 degrees around Z axis
    var arc = peg.CircularPattern(count: 6, span: 180 deg)

    return ring
}
```

## Relative Positioning (Align)

Instead of computing exact coordinates, use Align methods to position one solid relative to another. This is the idiomatic way to assemble multi-part models — you describe *relationships* ("on top of", "centered on", "flush left with") rather than absolute positions.

### StackOn

Place a solid on top of another, centered in X/Y. The bottom face of `self` lands on the top face of `with`:

```
fn Main() {
    var base   = Cube(s: Vec3{x: 40 mm, y: 40 mm, z: 10 mm})
    var column = Cylinder(radius: 8 mm, height: 30 mm).StackOn(with: base)
    var cap    = Sphere(radius: 10 mm).StackOn(with: column)
    return base + column + cap
}
```

An optional `nudge` creates a gap or overlap:

```
var lid = Cube(x: 42 mm, y: 42 mm, z: 3 mm).StackOn(with: base, nudge: -1 mm)   # 1 mm press-fit overlap
```

### AlignCenter

Center one solid relative to another on any combination of axes. By default all three axes are aligned; pass `x: false`, `y: false`, or `z: false` to skip an axis. A common pattern is centering in X and Y while leaving Z unchanged to place a feature on top of a base:

```
fn Main() {
    var base  = Cube(x: 60 mm, y: 40 mm, z: 8 mm)
    var boss  = Cylinder(radius: 5 mm, height: 12 mm).AlignCenter(with: base, z: false).StackOn(with: base)
    var hole  = Cylinder(radius: 3 mm, height: 25 mm).AlignCenter(with: base, z: false)
    return base + boss - hole
}
```

`AlignCenter` accepts optional `nudgeX`, `nudgeY`, `nudgeZ` offsets applied after alignment:

```
# Place the boss 10 mm to the right of center
var boss = Cylinder(radius: 5 mm, height: 12 mm)
    .AlignCenter(with: base, z: false, nudgeX: 10 mm)
    .StackOn(with: base)
```

You can also align to an absolute position instead of another solid:

```
var centered = Cylinder(radius: 5 mm, height: 10 mm).AlignCenter(pos: Vec3{}, z: false)   # center on the Z axis, leave Z alone
```

### AlignLeft / Right / Front / Back / Bottom / Top

These align the named face of `self` to the same face of `with`. They're useful for flush-mounting parts or creating assemblies where components share an edge:

```
fn Main() {
    var body  = Cube(x: 80 mm, y: 40 mm, z: 30 mm)

    # A mounting flange: same height as body, flush left, sitting on the floor
    var flange = Cube(x: 10 mm, y: 60 mm, z: 30 mm)
        .AlignLeft(with: body)          # left face flush with body's left face
        .AlignBottom(with: body)        # bottom face flush with body's bottom face
        .AlignCenter(with: body, x: false, z: false)  # centered in Y

    return body + flange
}
```

An optional `nudge` offsets after alignment. For Left/Front/Bottom, positive nudge moves **outward** (away from the solid). For Right/Back/Top, positive nudge also moves outward:

```
# Shelf with a 5 mm gap from the left face, sitting 20 mm below the top
var shelf = Cube(x: 60 mm, y: 30 mm, z: 2 mm)
    .AlignLeft(with: cabinet, nudge: 5 mm)     # 5 mm further left (outward)
    .AlignTop(with: cabinet, nudge: -20 mm)    # 20 mm downward (inward from top)
```

Each also has an absolute-position overload:

```
var grounded = shape.AlignBottom(pos: 0 mm)   # sit on the Z=0 plane
```

| Method | Aligns |
|--------|--------|
| `.AlignLeft(with: other)` | `self.min.x` → `other.min.x` |
| `.AlignRight(with: other)` | `self.max.x` → `other.max.x` |
| `.AlignFront(with: other)` | `self.min.y` → `other.min.y` |
| `.AlignBack(with: other)` | `self.max.y` → `other.max.y` |
| `.AlignBottom(with: other)` | `self.min.z` → `other.min.z` |
| `.AlignTop(with: other)` | `self.max.z` → `other.max.z` |

## Text

Create 2D text outlines, then extrude:

```
fn Main() {
    return Text(text: "Hello", s: 10 mm).Extrude(height: 2 mm)
}
```

Use a custom font:

```
fn Main() {
    return Text(text: "Custom", s: 10 mm, font: "/path/to/font.ttf").Extrude(height: 2 mm)
}
```

## Importing Meshes

Load external STL or OBJ files:

```
fn Main() {
    return LoadMesh(path: "/path/to/model.stl")
}
```

The imported mesh becomes a Solid that you can transform and combine with other shapes.

## Math Functions

| Function | Description |
|----------|-------------|
| `Sin(a: Angle)` -> `Number` | Sine |
| `Cos(a: Angle)` -> `Number` | Cosine |
| `Tan(a: Angle)` -> `Number` | Tangent |
| `Asin(n: Number)` -> `Angle` | Arcsine |
| `Acos(n: Number)` -> `Angle` | Arccosine |
| `Atan2(y:, x: Number)` -> `Angle` | Two-argument arctangent |
| `Sqrt(n: Number)` -> `Number` | Square root |
| `Pow(base:, exp: Number)` -> `Number` | Exponentiation |
| `Abs(a:)` -> same | Absolute value (Number, Length, Angle) |
| `Min(a:, b:)` -> same | Minimum (Number, Length, Angle) |
| `Max(a:, b:)` -> same | Maximum (Number, Length, Angle) |
| `Floor(n: Number)` -> `Number` | Floor |
| `Ceil(n: Number)` -> `Number` | Ceiling |
| `Round(n: Number)` -> `Number` | Round to nearest |
| `Lerp(a:, b:, t: Number)` -> same | Linear interpolation (Number, Length, Angle) |

Constants: `PI`, `TAU` (2 * PI), `E` (Euler's number). Declare constants with `const`.

## Assert

Validate conditions at runtime:

```
fn Main() {
    var count = 5
    assert count >= 2, "need at least 2"
    assert count < 100
    return Cube(s: 10 mm)
}
```

## Entry Points

Any function whose name starts with a capital letter is an **entry point**. Entry points appear in the run menu and can be previewed independently. A file can have multiple entry points:

```
fn Bracket(width Length = 20 mm where [10:50] mm) {
    return Cube(s: Vec3{x: width, y: 10 mm, z: 5 mm})
}

fn Mount(height Length = 15 mm where [5:30] mm) {
    return Cylinder(radius: 5 mm, height: height)
}
```

Both `Bracket` and `Mount` appear in the run menu. Switch between them to preview different parts.

**Rules for entry points:**
- Name must start with a capital letter (`Main`, `Bracket`, `Gear`)
- Every parameter must have a default value (so it can run without user input)
- Parameters with `where` constraints become interactive sliders/dropdowns
- Must return `Solid`, `[]Solid`, or `PolyMesh`
- Lowercase functions (`helper`, `makeHole`) are private helpers — not shown in the run menu

**`Main` is not special** — it's just a conventional name. If your file has only one entry point, `Main` is a fine choice. If your file defines a reusable part, name the entry point after the part: `fn HexBolt(...)`, `fn Bearing(...)`.

## Tips and Patterns

### Returning Multiple Solids

Entry points can return an array of Solids — they'll be arranged in a grid:

```
fn Main() {
    return [Cube(s: 10 mm), Sphere(radius: 8 mm), Cylinder(radius: 5 mm, height: 10 mm)]
}
```

### Union an Array of Solids

```
var parts = for i [0:<5] {
    yield Sphere(radius: 3 mm).Move(v: Vec3{x: i * 8 mm, y: 0 mm, z: 0 mm})
}
var combined = Union(arr: parts)
```

`Difference(arr)` and `Intersection(arr)` work the same way.

### Centering After Construction

Use `Bounds().Center()` to find the center, then translate:

```
fn Center(s Solid) {
    var c = s.Bounds().Center()
    return s.Move(v: Vec3{x: -c.x, y: -c.y, z: -c.z})
}
```

### Convex Hull

Wrap a set of shapes in their convex hull:

```
var shapes = [
    Sphere(radius: 5 mm),
    Sphere(radius: 5 mm).Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm}),
    Sphere(radius: 5 mm).Move(v: Vec3{x: 10 mm, y: 15 mm, z: 0 mm}),
]
var hull = Hull(arr: shapes)
```

### Comments

```
# This is a comment
var size = 10 mm; # inline comment
```

### Reserved Identifiers

Names starting with `_` are reserved for internal use. The identifier `self` is reserved inside method bodies.

## Common Pitfalls

### `Cube` spans from the origin

`Cube` spans from `(0, 0, 0)` to `(x, y, z)` — it is **not** centered. `Sphere` is centered at the origin. `Cylinder` has its base at Z=0 and top at Z=height.

### Angles accumulate — they don't wrap

Angle arithmetic never wraps to `[0, 360)`:

```
var a = 350 deg + 20 deg;   # 370 deg, not 10 deg
var b = 24 * 360 deg;       # 8640 deg — intentional for thread twist
```

### `Mirror`, `Rotate`, and `Scale` always require a pivot

`Rotate`, `Scale`, and `Mirror` always require an explicit pivot or offset argument — there is no implicit default. Use `Vec3{}` for rotations and scales around the origin:

```
solid.Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})
solid.Scale(x: 2, y: 1, z: 1, around: Vec3{})
solid.Mirror(nx: 1, ny: 0, nz: 0, offset: 0 mm)
```

### `Extrude` always goes in +Z

The sketch extrudes upward from Z=0. If you need the solid to start at a different Z, translate after extruding.

### `Revolve` rotates around the Y axis

Sketch profiles for `Revolve` must sit to the **right of the Y axis** (`x > 0`):

```
# Correct torus — sketch offset from Y axis:
Circle(radius: 3 mm).Move(x: 10 mm, y: 0 mm).Revolve()
```

### `1 / 2 mm` is a type error — use `1/2 mm`

No spaces around `/` in ratio literals when combined with a unit:

```
var half = 1/2 mm;    # correct: 0.5 mm
var bad  = 1 / 2 mm;  # error: Number / Length
```

### `var` declarations inside `if` are block-local

Assignments to existing variables inside `if` branches propagate to the enclosing scope. But `var` declarations are block-local:

```
var x = 10 mm
if true { x = 20 mm }     # x is now 20 mm — assignment propagates
if true { var y = 5 mm }   # y does not exist outside the if block
```

### Boolean ops require matching types

You cannot union a `Solid` with a `Sketch` — extrude the sketch first:

```
solid + sketch.Extrude(height: 10 mm)   # correct
solid + sketch                           # runtime error: type mismatch
```
