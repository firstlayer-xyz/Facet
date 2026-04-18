# Language Reference

Comprehensive reference for the Facet programming language.

## Grammar (EBNF)

**Semicolons are optional.** The lexer inserts them automatically on newlines after line-terminating tokens (`IDENT`, `NUMBER`, `STRING`, `` ` ``, `)`, `]`, `}`, `true`, `false`, `yield`). Insertion is suppressed when the next line starts with a continuation character (`)`, `]`, `.`, `&`, `|`, `^`, `+`, `-`, `*`, `/`, `%`), allowing multi-line expressions and method chains.

```ebnf
program     = { var_decl | const_decl | type_decl | function } ;

var_decl    = "var" IDENT "=" expr [ "where" constraint ] ";" ;
const_decl  = "const" IDENT "=" expr ";" ;
constraint  = "[" "]"
            | "[" expr { "," expr } "]"
            | "[" expr ":" [ "<" | ">" | "<=" | ">=" ] expr [ ":" expr ] "]"
              [ UNIT | ANGLE_UNIT ] ;

type_decl   = "type" IDENT "{" { IDENT type [ "=" expr ] ";" } "}" ;

function    = "fn" IDENT "(" [ params ] ")" [ type ] block
            | "fn" IDENT "." IDENT "(" [ params ] ")" [ type ] block ;
params      = paramGroup { "," paramGroup } ;
paramGroup  = nameSpec { "," nameSpec } type ;
nameSpec    = IDENT [ "=" expr ] [ "where" constraint ] ;

type        = "Solid" | "Length" | "Angle" | "Sketch" | "Number"
            | "Vec2" | "Vec3"
            | "Bool" | "String" | "var" | IDENT
            | "[]" type
            | "fn" "(" [ type { "," type } ] ")" [ type ] ;

block       = "{" { statement } "}" ;
statement   = "return" expr ";"
            | "yield" [ expr ] ";"
            | ( "var" | "const" ) IDENT "=" expr [ "where" constraint ] ";"
            | IDENT { "." IDENT } assignOp expr ";"
            | "assert" expr [ "," STRING ] ";"
            | expr ";"
            | (ifStmt | forYield | foldExpr) [ ";" ] ;
assignOp    = "=" | "+=" | "-=" | "*=" | "/=" | "%=" | "&=" | "|=" | "^=" ;

expr        = orExpr ;
orExpr      = andExpr { "||" andExpr } ;
andExpr     = compareExpr { "&&" compareExpr } ;
compareExpr = addExpr [ ("<" | ">" | "<=" | ">=" | "==" | "!=") addExpr ] ;
addExpr     = mulExpr { ("+" | "-" | "|") mulExpr } ;
mulExpr     = postfix { ("*" | "/" | "%" | "&" | "^") postfix } ;
postfix     = primary { "." IDENT "(" [ args ] ")" | "." IDENT
                       | "[" expr "]" | UNIT | ANGLE_UNIT } ;
primary     = ("-" | "!") postfix
            | call | structLit | IDENT | NUMBER | BOOL | STRING
            | libExpr | "(" expr ")" | array | forYield
            | foldExpr | lambda ;
lambda      = "fn" "(" [ params ] ")" [ type ] block ;
libExpr     = "lib" STRING ;
call        = IDENT "(" [ args ] ")" ;
structLit   = [ IDENT ] "{" [ fieldInits ] "}" ;
fieldInits  = IDENT ":" expr { "," IDENT ":" expr }
            | expr { "," expr } ;
args        = namedArg { "," namedArg }
            | expr { "," expr } ;
namedArg    = IDENT ":" expr ;
array       = "[" [ expr { "," expr } [ "," ] ] "]"
            | "[" expr ":" [ "<" | ">" | "<=" | ">=" ] expr [ ":" expr ] "]" ;
forClause   = IDENT "," IDENT expr | IDENT expr ;
forYield    = "for" forClause { "," forClause } "{" { statement } "}" ;
foldExpr    = "fold" IDENT "," IDENT expr "{" { statement } "}" ;
ifStmt      = "if" expr block { "else" "if" expr block } [ "else" block ] ;

NUMBER      = DIGITS [ "." DIGITS ] [ "/" DIGITS ] ;
BOOL        = "true" | "false" ;
STRING      = '"' { character } '"'
            | '`' { character } '`' ;
ANGLE_UNIT  = "deg" | "rad" | "grad" | "turn" | ... ;
UNIT        = "mm" | "cm" | "m" | "km" | "in" | "ft" | "yd" | "mi" | ... ;
```

Hand-written recursive-descent parser.

## Quick Reference

| Goal | Code |
|------|------|
| Simple box | `Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 5 mm})` |
| Uniform cube | `Cube(s: 10 mm)` |
| Sphere | `Sphere(radius: 8 mm)` |
| Cylinder | `Cylinder(bottom: 5 mm, top: 5 mm, height: 20 mm)` |
| Extrude a profile | `Circle(radius: 5 mm).Extrude(height: 20 mm)` |
| Revolve a profile | `Circle(radius: 3 mm).Move(x: 10 mm, y: 0 mm).Revolve()` |
| Drill a hole | `box - Cylinder(radius: 3 mm, height: 30 mm)` |
| Move a solid | `.Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})` |
| Rotate a solid | `.Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})` |
| Mirror across YZ plane | `.Mirror(nx: 1, ny: 0, nz: 0, offset: 0 mm)` |
| Repeat in a line | `.LinearPattern(count: 4, spacing: Vec3{x: 10 mm})` |
| Repeat in a ring | `.CircularPattern(count: 6)` |
| Fillet a profile | `sketch.Fillet(radius: 2 mm).Extrude(height: 5 mm)` |
| Interactive slider | `r Length = 5 mm where [1:20] mm` (on fn params) |
| Dropdown selector | `s String = "m3" where ["m3", "m4", "m5"]` (on fn params) |
| Load a library | `var T = lib "facet/threads"` |
| Get bounding box | `solid.Bounds().Width()` |
| Convex hull | `Hull(arr: [a, b, c])` |

**Minimal program:**

```
fn MyCube() {
    return Cube(s: 10 mm)
}
```

Any function starting with a capital letter is an entry point. `Main` is a common convention but not required.

**Program with interactive parameters:**

```
fn Bracket(size Length = 10 mm where [1:50] mm) Solid {
    return Cube(s: size)
}
```

## Coordinate System

Facet uses a **Z-up** right-handed coordinate system:

| Axis | Direction | Spatial meaning |
|------|-----------|-----------------|
| **X** | Left / Right | Width |
| **Y** | Front / Back | Depth |
| **Z** | Down / Up | Height (vertical) |

Key conventions:

- `Cube` spans from the **origin** to (x, y, z) — it is NOT centered
- `Cylinder` extends along the **Z axis**
- `Extrude` goes upward along **+Z**
- `Revolve` rotates around the **Y axis**
- `CircularPattern` repeats around the **Z axis**
- `Rotate`, `Scale`, and `Mirror` always require an explicit pivot or offset — there is no implicit default

## Types

| Type | Description |
|------|-------------|
| `Solid` | A solid 3D manifold geometry |
| `Sketch` | A 2D cross-section shape |
| `Length` | A dimensional value with a unit (stored internally as mm) |
| `Angle` | An angular value (stored internally as degrees) |
| `Number` | A plain numeric value (no unit) |
| `Vec2` | A 2D vector struct (`x`, `y` Length) |
| `Vec3` | A 3D vector struct (`x`, `y`, `z` Length) |
| `Bool` | A boolean value (`true` or `false`) |
| `[]Type` | A typed array (e.g. `[]Solid`, `[]Vec2`, `[]Number`) |
| `String` | A text string |
| `Library` | A loaded library namespace |
| `Box` | A 3D bounding box with `min` and `max` Vec3 |

## Literals

### Number Literals

- Integers: `10`, `42`
- Floats: `3.14`, `0.5`
- Ratios: `1/2`, `3/4` — no spaces around `/`, parsed as a single literal (not division). This matters with units: `1/2 mm` = `0.5 mm`, but `1 / 2 mm` = `Number ÷ Length` (type error).

### Length Literals

A number followed by a distance unit. All lengths are stored as millimeters internally. Bare numbers default to millimeters where a `Length` is expected. Unit suffixes work on any expression: `x mm`, `(1 + 2) mm`, `Foo() deg`.

**Common units:** `mm`, `cm`, `m`, `km`, `um` (micron), `in`, `ft`, `yd`, `mi`, `thou`. Many more supported (metric prefixes, historical, astronomical, fun units). All stored as mm internally.

**Unit precedence:** Unit suffixes bind as postfix (tighter than all binary operators). `2 * 3 mm` means `2 * (3 mm)` = `6 mm`. `2 mm * 3 mm` = `Number` (Length × Length gives a dimensionless area ratio).

### Angle Literals

A number followed by an angle unit. All angles stored as degrees internally. Angles are unbounded — they accumulate without wrapping (e.g. `350 deg + 20 deg` = `370 deg`, not `10 deg`).

**Common units:** `deg`, `rad`, `grad`, `turn` (360°). Also supported: `arcmin`, `arcsec`, `mrad`, `mil`. All stored as degrees internally.

### Boolean Literals

`true`, `false`

### String Literals

Enclosed in double quotes: `"hello"`. Supports escape sequences:

| Escape | Meaning |
|--------|---------|
| `\n` | Newline |
| `\t` | Tab |
| `\\` | Literal backslash |
| `\"` | Literal double quote |
| `\uXXXX` | Unicode codepoint (4 hex digits) |

Unknown escape sequences are an error. Use backtick raw strings for regex patterns or other content with literal backslashes: `` `m(\d+)x([\d.]+)` ``.

Concatenation with `+`: `"hello" + " world"`.

**String methods:** `.SubStr(start, length)`, `.HasPrefix(prefix)`, `.HasSuffix(suffix)`, `.Split(delimiter)`, `.Match(pattern)`, `.ToUpper()`, `.ToLower()`, `.Trim()`, `.Replace(old, new)`, `.IndexOf(substr)`, `.Contains(substr)`, `.Length()`.

## Constants

| Name | Unicode | Value |
|------|---------|-------|
| `PI` | `π` | 3.14159265... |
| `TAU` | `τ` | 6.28318530... |
| `E` | | 2.71828182... |
| `Vec3{}` | | `Vec3{x: 0 mm, y: 0 mm, z: 0 mm}` — pivot for rotations, scales, and mirrors |

## Variables

### Declaration

```
var name = expr;
```

Variables infer their type from the initializer. They can be global (top-level) or local (inside functions).

### Reassignment

```
name = expr;
name += expr;
name -= expr;
name *= expr;
name /= expr;
name %= expr;
name &= expr;
name |= expr;
name ^= expr;
```

Compound assignment desugars to `name = name op expr`.

### Constrained Parameters

Function parameters with `where` clauses become interactive controls in the parameters panel. This is the preferred way to expose configurable values:

```
fn Main(
    w Length = 10 mm where [1:100] mm,        # Length range slider
    a Angle = 45 deg where [0:360] deg,       # Angle range slider
    n Number = 50 where [0:100],              # Number range slider
    s Number = 5 where [0:100:5],             # Stepped slider
    e Number = 5 where [0:<100],              # Exclusive upper bound
    t String = "m3" where ["m3", "m4", "m5"], # Dropdown
    x Number = 42 where [],                    # Free-form input
) Solid { ... }
```

**Range modifiers:** `<` (exclusive up), `>` (exclusive down), `>=` (inclusive up), `<=` (inclusive down). Default is inclusive.

**Step:** Optional third value. Defaults to `1` (ascending) or `-1` (descending).

Constrained parameters on entry point functions appear as interactive controls in the parameters panel.

## Operators

### Arithmetic

| Expression | Result | Description |
|-----------|--------|-------------|
| `Length + Length` | `Length` | Addition |
| `Length - Length` | `Length` | Subtraction |
| `Length * Number` | `Length` | Scaling |
| `Number * Length` | `Length` | Scaling |
| `Length * Length` | `Number` | Area (dimensionless) |
| `Length / Number` | `Length` | Division |
| `Length / Length` | `Number` | Ratio |
| `Length % Length` | `Length` | Modulo |
| `Number op Number` | `Number` | All arithmetic ops |
| `Angle + Angle` | `Angle` | Addition |
| `Angle - Angle` | `Angle` | Subtraction |
| `Angle * Number` | `Angle` | Scaling |
| `Number * Angle` | `Angle` | Scaling |
| `Angle / Number` | `Angle` | Division |
| `Angle / Angle` | `Number` | Ratio |
| `String + String` | `String` | Concatenation |

Vec2 and Vec3 arithmetic (`+`, `-`, `*`, `/`, unary `-`) is defined in the stdlib.

### Boolean (Geometry)

| Expression | Result | Description |
|-----------|--------|-------------|
| `Solid + Solid` | `Solid` | Union |
| `Solid - Solid` | `Solid` | Difference |
| `Solid & Solid` | `Solid` | Intersection |
| `Solid \| Solid` | `Solid` | Insert (cut + seat) |
| `Solid ^ Solid` | `Solid` | Exclude (symmetric difference) |
| `Sketch + Sketch` | `Sketch` | Union |
| `Sketch - Sketch` | `Sketch` | Difference |
| `Sketch & Sketch` | `Sketch` | Intersection |

`&` and `^` bind at the same precedence as `*`, `/`, `%` (tighter than `+`, `-`, `|`).

### Comparison

`<`, `>`, `<=`, `>=`, `==`, `!=` — work on `Length`, `Number`, `Angle`, `String`, `Vec2`, `Vec3`. Numeric comparisons use epsilon.

### Logical

`&&` (short-circuit AND), `||` (short-circuit OR), `!` (NOT).

### Unary

`-` (negate `Number`, `Length`, `Angle`), `!` (negate `Bool`).

### Precedence (highest first)

1. Postfix (`.method()`, `[index]`, unit application)
2. Unary (`-`, `!`)
3. Multiplicative (`*`, `/`, `%`, `&`, `^`)
4. Additive (`+`, `-`, `|`)
5. Comparison (`<`, `>`, `<=`, `>=`, `==`, `!=`)
6. Logical AND (`&&`)
7. Logical OR (`||`)

## Expressions

### If/Else

```
if cond { body } else if cond { body } else { body }
```

`if` is a statement, not an expression. `return` inside a branch exits the enclosing function:

```
fn Bigger(a, b Length) Length {
    if a > b { return a }
    return b
}
```

Assignments inside `if` branches propagate to the enclosing scope. Variables declared with `var` inside a branch are block-local:

```
var x = 10 mm
if true { x = 20 mm }   # x is now 20 mm
if true { var y = 5 mm } # y does not exist here
```

### Array Literals

```
[1, 2, 3]             # inferred as []Number
[a, b, c]             # type inferred from elements
[]                    # empty array
[]Vec3[{...}, {...}]  # explicit type (required for anonymous struct elements)
```

Array element types are inferred from the elements when all elements share the same type. If elements have mixed types, the checker errors. Use the explicit `[]Type[...]` syntax when elements are anonymous struct literals `{...}` whose type can't be inferred.

Arrays are truthy when non-empty and falsy when empty.

### Range Expressions

```
[start:end]           # inclusive
[start:<end]          # exclusive end
[start:end:step]      # with step
[6:0]                 # descending (auto step -1)
```

### For-Yield

```
for var range { body }
for i, val array { body }           # enumerate
for i range, j range { body }       # cartesian product
```

Collects values into an Array. Use `yield expr;` to produce a value. Use `yield;` (bare, no value) to skip an iteration — nothing is added to the result array for that iteration. This is the idiomatic way to filter:

```
var evens = for i [0:<10] {
    if i % 2 != 0 { yield; }
    yield i;
};
```

### Fold

```
fold acc, elem array { body }
```

Reduces an array. First element is the initial accumulator. Empty array is a runtime error. The body uses `yield` to set the next accumulator value. (In fold, `yield` updates the accumulator rather than appending to a result array.)

### Array Indexing

```
arr[0]              # first element
arr[-1]             # last element
arr[-2]             # second to last
```

Zero-based integer indices. Negative indices wrap from the end (`-1` = last). Out-of-range is a runtime error.

### Array Slicing

```
arr[1:3]            # elements at index 1, 2 (exclusive end)
arr[:3]             # first 3 elements
arr[3:]             # from index 3 to end
arr[-3:]            # last 3 elements
```

Returns a new array. Out-of-range bounds are clamped (no error).

## Functions

```
fn Name(param1 Type, param2 Type) ReturnType {
    body
}
```

- **Uppercase** names (`fn Main`, `fn Rocket`) are entry points — shown in the run menu, must be fully constrained.
- **lowercase** names (`fn helper`, `fn nosecone`) are private helpers — not shown in the run menu.
- Return type is optional (inferred).
- Parameter types are optional for polymorphic functions.
- Consecutive parameters of the same type can be grouped: `fn helper(x, y, z Length)`.
- Functions require an explicit `return` statement to return a value. Functions without `return` are void.
- `return` always exits the enclosing function. For-yield bodies use `yield`. Fold bodies use `yield`.
- Optional parameters: `param Type = default` (default after the type).
- Default values and constraints are not allowed on grouped parameters.

### Overloading

Multiple functions with the same name but different parameter types or counts are allowed. The evaluator picks the first overload whose parameter types match after coercion.

### Generic Parameters (`var`)

Use `var` as a parameter type to accept any type, inferred from the caller:

```
fn Size(a []var) Number { return _size(a) }
```

- `var` matches any single value. `[]var` matches any typed array.
- Consecutive `var` params form a **var group** — all args in the group must resolve to the same concrete type.
- Return type `var` resolves to the first var group's concrete type.
- When overloading, concrete type matches are preferred over `var` matches.

### First-Class Functions (Lambdas)

Functions are values. A `fn` expression creates a closure that captures variables from the enclosing scope (deep copy):

```
var move = fn(s Solid) Solid { return s.Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm}) }
```

Function types are written as `fn(Type1, Type2) ReturnType`.

## Methods

```
fn Type.Method(param1 Type) ReturnType {
    self.DoSomething()   # self refers to the receiver
}
```

Methods are called with dot notation: `obj.Method(args)`.

## Structs

```
type Name {
    field1 Type
    field2 Type = default   # optional default value
}
```

### Instantiation

```
var x = Name { field1: value1, field2: value2 };  # named fields
var y = Name { value1, value2 };                   # positional (by declaration order)
var z = Name {};                                    # all defaults
```

### Anonymous Struct Literals

Omit the type name — the struct is coerced to the expected parameter type:

```
Cube(s: {20 mm, 10 mm, 5 mm})     # coerced to Vec3
solid.Move(v: {5 mm, 0 mm, 0 mm})    # coerced to Vec3
```

Zero values: `Number` → `0`, `Length` → `0 mm`, `Angle` → `0 deg`, `Bool` → `false`, `String` → `""`.

### Field Access and Assignment

```
x.field1
x.field1 = newValue
```

## Libraries

```
var Lib = lib "github.com/user/repo/path@ref";
Lib.Function(args)
```

**Library directories:**
- **macOS**: `~/Library/Application Support/Facet/libraries/`
- **Linux**: `$XDG_DATA_HOME/facet/libraries/` (fallback `~/.local/share/facet/libraries/`)
- **Windows**: `%APPDATA%\Facet\libraries\`

Each library is a directory containing a single `.fct` file named after the directory.

## Assert

```
assert condition;
assert condition, "message";
```

## Comments

```
# This is a comment
// This is also a comment
```

## Entry Points

Any function whose name starts with a capital letter is an **entry point** — it appears in the run menu and can be previewed independently. Entry points must be **fully constrained**: every parameter needs a default value and a `where` clause.

```
fn Main(size Length = 10 mm where [1:50] mm) Solid { ... }   # entry point
fn Variant(n Number = 3 where [1:10]) Solid { ... }          # also an entry point
fn helper(x Length) Solid { ... }                              # lowercase = private helper
```

`Main` is a common convention but not required — any uppercase function is an entry point. Entry points must return `Solid`, `[]Solid`, or `PolyMesh`.

## Reserved Identifiers

- `_`-prefixed names are reserved for internal builtins
- `self` is reserved inside method bodies

## Stdlib

The standard library (`std.fct`) is auto-included and provides all user-facing functions and methods. It wraps internal `_snake_case` builtins in `CamelCase` APIs. The stdlib is self-documenting — see the **API Reference** panel in the app for full signatures and examples.

Key categories: 3D constructors (`Cube`, `Sphere`, `Cylinder`), 2D constructors (`Square`, `Circle`, `Polygon`), transforms (`Move`, `Rotate`, `Scale`, `Mirror`), boolean ops (`Union`, `Difference`, `Intersection`, `Hull`), patterns (`LinearPattern`, `CircularPattern`), alignment (`AlignLeft`, `AlignCenter`, `StackOn`), measurement (`Bounds`, `Volume`, `SurfaceArea`), math (`Sin`, `Cos`, `Min`, `Max`, `Sqrt`, `Lerp`), text (`Text`), mesh ops (`Mesh`, `PolyMesh`, `Warp`, `LevelSet`).
