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

- **Entry:** `Eval(ctx, prog, overrides, entryPoint) (*EvalResult, error)`
- **Debug:** `EvalDebug(ctx, prog, overrides, entryPoint) (*DebugResult, error)`
- **Returns:** `*EvalResult` with `Solids []*manifold.Solid` and `ModelStats` (volume, surface area)

Parameters:
- `entryPoint` — **required**, name of the function to evaluate (e.g. `"Main"`, `"MyCube"`)
- `overrides` — map of parameter names to values for slider/UI-driven parameters

The evaluator returns an error if `entryPoint` is empty or if the named function does not exist.

## Supporting packages

### formatter

Parses Facet source code and re-formats it from the AST.

- **Entry:** `Format(source string) string`
- Takes source code as a string, parses it internally, and returns reformatted output. Returns the original unchanged if parsing fails.

### doc

Extracts documentation entries (signatures, descriptions) from source and libraries.

- **Entry:** `BuildDocIndex(source, stdlibSrc) []DocEntry`
- Returns `[]DocEntry` for autocomplete and documentation display.
