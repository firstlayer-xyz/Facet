# Language Reference

Comprehensive reference for the Facet programming language.

## Grammar (EBNF)

**Semicolons are optional.** The lexer inserts them automatically on newlines after line-terminating tokens (`IDENT`, `NUMBER`, `STRING`, `` ` ``, `)`, `]`, `true`, `false`, `yield`, `nil`). Note that `}` is intentionally *not* a terminator, so `} else {` can span lines. Insertion is suppressed when the next line starts with a continuation character (`)`, `]`, `.`, `&`, `|`, `^`, `+`, `-`, `*`, `/`, `%`), allowing multi-line expressions and method chains.

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
            | "Bool" | "String" | "Any" | IDENT
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
compareExpr = bitOrExpr [ ("<" | ">" | "<=" | ">=" | "==" | "!=") bitOrExpr ] ;
bitOrExpr   = bitXorExpr { "|" bitXorExpr } ;
bitXorExpr  = bitAndExpr { "^" bitAndExpr } ;
bitAndExpr  = addExpr { "&" addExpr } ;
addExpr     = mulExpr { ("+" | "-") mulExpr } ;
mulExpr     = postfix { ("*" | "/" | "%") postfix } ;
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
fieldInits  = IDENT ":" expr { "," IDENT ":" expr } [ "," ] ;
args        = namedArg { "," namedArg } ;
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
| Sphere | `Sphere(r: 8 mm)` |
| Cylinder | `Cylinder(r: 5 mm, h: 20 mm)` |
| Extrude a profile | `Circle(r: 5 mm).Extrude(z: 20 mm)` |
| Revolve a profile | `Circle(r: 3 mm).Move(x: 10 mm, y: 0 mm).Revolve()` |
| Drill a hole | `box - Cylinder(r: 3 mm, h: 30 mm)` |
| Move a solid | `.Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})` |
| Rotate a solid | `.Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{})` |
| Mirror across YZ plane | `.Mirror(x: 1, y: 0, z: 0, offset: 0 mm)` |
| Repeat in a line | `.LinearPattern(count: 4, gap: 5 mm)` |
| Repeat in a ring | `.CircularPattern(count: 6)` |
| Fillet a profile | `sketch.Fillet(r: 2 mm).Extrude(z: 5 mm)` |
| Interactive slider | `r Length = 5 mm where [1:20] mm` (on fn params) |
| Dropdown selector | `s String = "m3" where ["m3", "m4", "m5"]` (on fn params) |
| Load a library | `var T = lib "github.com/firstlayer-xyz/facetlibs/threads@413a17e"` |
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

- `Rotate`, `Scale`, and `Mirror` default to the world origin / a zero offset when their pivot/offset arg is omitted
- For primitive-specific axis/origin conventions (e.g. where `Cube` or `Cylinder` sits relative to the origin, which axis `Extrude` / `Revolve` / `CircularPattern` use), see the **Stdlib API Reference** section below — it is auto-generated from the stdlib source so it is always current

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
| `T?` | An Optional — either a value of type `T` or absent (`nil`). See **Optional Types** below. |

## Optional Types

An Optional carries either a value of some inner type or no value at all. The shape is the postfix `?` on any type:

```
Number?              // a Number or absent
Length?              // a Length or absent
Vec3?                // a Vec3 or absent
[]Number?            // an array of Number? (the `?` binds tighter than `[]`)
```

Double-optional (`T??`) is rejected at parse time — Optional is always a single layer.

### Constructing an Optional

A bare value implicitly widens to its Optional form at any type boundary that expects `T?`:

```
fn Lookup() Number? {
    return 5             // implicit Some(5) — widens from Number to Number?
}

fn Missing() Number? {
    return nil           // explicit None
}
```

`nil` is the None literal. With no surrounding type context it has type "wild Optional" and widens to any `T?`.

### Working with Optionals

The language provides four sugar operators and an extension to `for-yield`. Together they cover every common pattern; there is no raw "unwrap or crash" form, so the type system forces every caller to handle absence at the boundary. Optionals have no methods — every access goes through one of the forms below.

| Form | Purpose | Example |
|------|---------|---------|
| `opt ?? fallback` | Use the inner value if present, otherwise the fallback | `Lookup() ?? 0` |
| `opt?.field` / `opt?.method()` | Access a field or method through an Optional; the result is `?`-wrapped | `MaybeVec()?.x ?? 0 mm` |
| `opt == nil` / `opt != nil` | Presence check, returns Bool | `if Lookup() != nil { ... }` |
| `if var name = opt { ... }` | Bind-and-narrow: enters the branch only when present, with `name` typed as the inner `T` | `if var i = Lookup() { return i }` |
| `for v opt { yield expr }` | Map / Filter through Optional — the result is `?`-wrapped | `for n m { yield n * 2 }` |

Optional chaining short-circuits: if the receiver is None, the whole `opt?.x.y.z` chain is None and no field access is attempted. `??` short-circuits too — the fallback is only evaluated when the left side is absent.

### Worked Example

```
fn FindPerson(name String) Person? { ... }

fn AgeOf(name String) Number {
    return FindPerson(name: name)?.age ?? 0
}
```

The chain: look up a person, take their age if present, otherwise 0. Each step short-circuits to None if the previous step was None.

## Literals

### Number Literals

- Integers: `10`, `42`
- Floats: `3.14`, `0.5`
- Ratios: `1/2`, `3/4` — no spaces around `/`, parsed as a single literal (not division). This matters with units: `1/2 mm` = `0.5 mm`, but `1 / 2 mm` = `Number ÷ Length` (type error).

### Length Literals

A number followed by a distance unit. All lengths are stored as millimeters internally. Bare numbers default to millimeters where a `Length` is expected. Unit suffixes work on any expression: `x mm`, `(1 + 2) mm`, `Foo() deg`.

**Common units:** `mm`, `cm`, `m`, `km`, `um` (micron), `in`, `ft`, `yd`, `mi`, `thou`. Many more supported (metric prefixes, historical, astronomical, fun units). All stored as mm internally.

**Unit precedence:** Unit suffixes bind as postfix (tighter than all binary operators). `2 * 3 mm` means `2 * (3 mm)` = `6 mm`. `Length × Length` is rejected at compile time — there is no Area type — so use `Length / Number` and friends to keep dimensions consistent.

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
| `Length / Number` | `Length` | Division |
| `Length / Length` | `Number` | Ratio |
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

`|`, `^`, and `&` each have their own precedence level, all looser than `+`/`-`. From tightest to loosest: `* / %` > `+ -` > `&` > `^` > `|`. So `a & b + c` parses as `a & (b + c)`, and `a & b ^ c` parses as `(a & b) ^ c`.

### Comparison

`<`, `>`, `<=`, `>=`, `==`, `!=` — work on `Length`, `Number`, `Angle`, `String`, `Vec2`, `Vec3`. Numeric comparisons use epsilon.

### Logical

`&&` (short-circuit AND), `||` (short-circuit OR), `!` (NOT).

### Optional

- `??` — nullish coalescing. `opt ?? fallback` returns the inner value if `opt` is Some, else evaluates and returns `fallback`. Short-circuits like `&&`/`||`.
- `?.` — optional chaining (postfix on a field/method access). `opt?.field` short-circuits to None if `opt` is absent, else accesses `.field` on the inner value and wraps the result in `?`.

### Conditional (Ternary)

`cond ? a : b` — C-style conditional expression. `cond` must be Bool; `a` and `b` must produce compatible types; the result is that unified type. Only the chosen arm is evaluated. Right-associative: `a ? b : c ? d : e` parses as `a ? b : (c ? d : e)`.

```
var size = big ? 20 mm : 5 mm
return color != nil ? color : Color(hex: "#FFFFFF")
```

### Unary

`-` (negate `Number`, `Length`, `Angle`), `!` (negate `Bool`).

### Precedence (highest first)

1. Postfix (`.method()`, `?.field` / `?.method()`, `[index]`, unit application)
2. Unary (`-`, `!`)
3. Multiplicative (`*`, `/`, `%`)
4. Additive (`+`, `-`)
5. Bitwise AND (`&`)
6. Bitwise XOR (`^`)
7. Bitwise OR (`|`)
8. Comparison (`<`, `>`, `<=`, `>=`, `==`, `!=`)
9. Logical AND (`&&`)
10. Nullish coalescing (`??`) — binds tighter than `||`, so `a || opt ?? d` parses as `a || (opt ?? d)`
11. Logical OR (`||`)
12. Ternary (`? :`) — looser than `||`, so `a || b ? c : d` parses as `(a || b) ? c : d`

Note: `&`, `^`, `|` are three distinct levels, all looser than additive, so e.g. `a + b & c` parses as `((a + b) & c)` and `a | b & c` parses as `(a | (b & c))`.

## Expressions

### If/Else

```
if cond { body } else if cond { body } else { body }
```

`if` is a statement, not an expression. `return` inside a branch exits the enclosing function.

Bind-and-narrow form for Optionals: `if var NAME = opt { body }`. The body runs only when `opt` is Some, with `NAME` bound to the unwrapped inner value (typed as `T`, not `T?`):

```
fn AgeOrZero(name String) Number {
    if var p = FindPerson(name: name) {
        return p.age            // p is Person here, not Person?
    }
    return 0
}
```

The bound name is scoped to the then-branch and does not leak past the closing brace.

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

**For-yield over an Optional.** The same `for v opt { yield expr }` shape works when the source is `T?`. An Optional is conceptually a 0-or-1 element collection, so the result is `U?` instead of `[]U`:

```
fn Maybe() Number? { return 5 }
var doubled = for n Maybe() { yield n * 2 }     // doubled is Number? — Some(10)

fn None() Number? { return nil }
var stillNone = for n None() { yield n * 2 }    // stillNone is Number? — None

// Conditional yield = Filter
var positive = for n Maybe() {
    if n > 0 { yield n }
}                                                // positive is Number? — Some(5)
```

This unifies Map and Filter on Optionals with the existing array machinery — no extra method API.

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

Returns a new array. Out-of-range bounds (after negative indices are normalized from the end) are a runtime error — they are not clamped. A start greater than the end is also an error.

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
- A parameter of Optional type (`param Type?`) is itself optional: when the
  caller omits it, it binds `nil` (None). `param Type? = nil` is equivalent but
  redundant. Read the value with `?? fallback` or `if var`.
- Default values and constraints are not allowed on grouped parameters.
- **Call sites must use named arguments** (`name: value`); a positional argument is a compile error (`…() arguments must be named`). This applies to free functions, methods, library calls, and function-valued variables alike — only internal `_`-prefixed builtins take positional arguments. (Definitions still declare parameters positionally as `name Type`; the rule is about *calls*.)

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
var x = Name { field1: value1, field2: value2 };  # named fields (required)
var z = Name {};                                    # all defaults
```

### Anonymous Struct Literals

Omit the type name when the expected struct type is known from context — the literal is coerced to that type. Fields must be named (positional anonymous literals are not supported):

```
Cube(s: {x: 20 mm, y: 10 mm, z: 5 mm})     # coerced to Vec3
solid.Move(v: {x: 5 mm, y: 0 mm, z: 0 mm}) # coerced to Vec3
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
- **Linux**: `$XDG_CONFIG_HOME/Facet/libraries/` (fallback `~/.config/Facet/libraries/`)
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

`Main` is a common convention but not required — any uppercase function is an entry point. Entry points must return `Solid`, `[]Solid`, `PolyMesh`, or `Animation`.

## Reserved Identifiers

- `_`-prefixed names are reserved for internal builtins
- `self` is reserved inside method bodies

## Stdlib

The standard library (`std.fct`) is auto-included and provides all user-facing functions and methods. It wraps internal `_snake_case` builtins in `CamelCase` APIs.

Full API signatures with doc comments are auto-generated from the stdlib source — see the **Stdlib API Reference** section that follows (assistants) or the **API Reference** panel (app UI). That reference is authoritative; prefer it over anything written here if the two ever disagree.
