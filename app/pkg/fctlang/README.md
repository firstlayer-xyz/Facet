# fctlang

Facet language compiler pipeline. Processes `.fct` source through four stages:

```
Parse → Load → Check → Evaluate
```

## Pipeline

### 1. parser

Lexes and parses Facet source into an AST.

- **Entry:** `Parse(source string) (*Source, error)`
- **Returns:** `*Source` — the root AST node containing `Globals`, `Functions`, and `StructDecls`
- **Key types:** `Source`, `Function`, `Param`, `StructDecl`, `Stmt`, `Expr`, `SourceError`

### 2. loader

Resolves library imports (`use "github.com/org/repo"`) and builds a complete program with all dependencies.

- **Entry:** `Load(ctx, source, libDir, opts) (*Program, error)`
- **Returns:** `*Program` — contains the main `Source` plus all resolved library sources in a `Libs` map
- **Key types:** `Program`, `LibCache`, `Options`

Libraries are fetched from git, cached locally, and parsed. `LibCache` allows reuse across invocations.

### 3. checker

Static type checking, type inference, and declaration extraction. Does not enforce any particular entry point — that is the evaluator's responsibility.

- **Entry:** `Check(prog *Program) *Result`
- **Returns:** `*Result` containing:
  - `Errors` — type errors found during analysis
  - `VarTypes` — variable name to type mappings
  - `InferredReturnTypes` — function name to inferred return type mappings
  - `Declarations` — source locations for go-to-definition support

### 4. evaluator

Evaluates a checked program by calling a named entry point function. The entry point must be a function that returns `Solid`, `[]Solid`, or `PolyMesh`.

- **Entry:** `Eval(ctx, prog, currentKey, overrides, entryPoint) (*EvalResult, error)`
- **Debug:** `EvalDebug(ctx, prog, currentKey, overrides, entryPoint) (*DebugResult, error)`
- **Returns:** `*EvalResult` with `Solids []*manifold.Solid` and `ModelStats` (volume, surface area)

Parameters:
- `currentKey` — source key of the program being evaluated; used to resolve relative library references
- `overrides` — map of parameter names to values for slider/UI-driven parameters
- `entryPoint` — **required**, name of the function to evaluate (e.g. `"Main"`, `"MyCube"`)

The evaluator returns an error if `entryPoint` is empty or if the named function does not exist.

## Supporting packages

### formatter

Re-formats a parsed Facet AST to canonical source text.

- **Entry:** `Format(src *parser.Source) string`
- Takes a `*parser.Source` (parse with `parser.Parse` first) and returns reformatted output.

### doc

Extracts documentation entries (signatures, descriptions) from source and libraries.

- **Entry:** `BuildDocIndex(source, stdlibSrc) []DocEntry`
- Returns `[]DocEntry` for autocomplete and documentation display.

## Type System

Facet has a small set of primitive types — `Number`, `Length`, `Angle`, `Bool`, `String` — plus composite types (structs, arrays, solids, etc.). `Number` and `Length` are distinct: a `Length` carries millimetre units, a `Number` is dimensionless. Mixing them is a compile-time error except where the rules below allow it.

### Arithmetic rules

| Expression | Result |
|---|---|
| `Number + Number`, `-`, `*`, `/`, `%` | `Number` |
| `Length + Length`, `-` | `Length` |
| `Length / Length` | `Number` (dimensionless ratio) |
| `Length * Length` | **Error** (no `Area` type) |
| `Length % Length` | **Error** |
| `Length * Number`, `Number * Length` | `Length` (scale) |
| `Length / Number` | `Length` (scale down) |
| `Length + Number`, `-`, `%` (Number side is a bare literal) | `Length` (literal coerces) |
| `Length + Number`, `-`, `%` (Number side is a variable/expression) | **Error** (dimension mismatch) |
| `Number / Length`, `Number - Length` (Number side non-literal) | **Error** |
| `Angle + Angle`, `-` | `Angle` |
| `Angle / Angle` | `Number` |
| `Angle * Number`, `Number * Angle`, `Angle / Number` | `Angle` |

### Conversions

- **`Number → Length`** auto-coerces at value-category boundaries — variable declarations, function arguments, struct field assignments, and return values. Inside an arithmetic expression it coerces only when the Number side is a bare numeric literal; a committed Number variable does not auto-coerce.
- **`Length → Number`** never coerces silently. Call the stdlib `Number(from: x)` function to strip units explicitly when you need a dimensionless value.
- **`Number → Angle`** also auto-coerces at boundaries (bare `45` passed to an `Angle` parameter means `45 deg`).

### Numeric literals are "untyped"

A bare numeric literal like `3` or `-0.5` has no committed type until context pins it down. This is the same idea as untyped constants in Go. In a mixed expression, a literal can coerce to whichever dimension makes the expression type-check:

```facet
var w = 5 mm + 3          // Length: the literal 3 becomes 3 mm → w = 8 mm
var h = 3 + 5 mm          // Length: literal coerces, same result
var k = 5 mm + -2         // Length: negated literal also counts

var n = 3                 // Number (committed)
var bad = 5 mm + n        // ERROR: n is a Number variable, not a literal
```

### Literal ratios vs. division

Whitespace around `/` matters for length literals:

- `20/2 mm` — single literal, tokenized as `20/2 = 10` with `mm` suffix → `10 mm`.
- `20 / 2 mm` — division expression: `Number / Length`, which is a dimension error.

### Examples

```facet
var w = 5 mm * 2          // Length (scale)
var r = 10 mm / 2 mm      // Number (ratio)
var d = 5 mm + 2          // Length = 7 mm (literal coerces)
var a = 5 mm * 3 mm       // ERROR: Length * Length not supported
var n = Number(from: 5 mm) // explicit strip → 5.0 (Number)
```
