# Facet — Main Branch Code Review

**Date:** 2026-04-16
**Scope:** Full `main` branch, ~45K LOC (36K Go + 9K TS across 134 source files)
**Method:** Four parallel reviewers, one per major subsystem (`app/pkg/fctlang/`, `app/pkg/manifold/`, `app/frontend/src/`, top-level `app/*.go` + `app/pkg/docgen/`)

---

## Cross-Cutting Themes

These same issues recur across all four subsystems — they are architectural patterns, not isolated bugs.

### 1. Pervasive fallback pattern (contradicts CLAUDE.md "no fallbacks")

The exact anti-pattern the engineering principles call out is the default idiom in several places:

- **app shell**: `loadConfig` returns `appConfig{}` on any error (`app/app.go:174`) — silently blows away user settings on malformed JSON. `_ = err` patterns in `app_library.go` hide permission errors.
- **fctlang**: anonymous struct coercion silently stamps a type name when declaration is missing (`app/pkg/fctlang/evaluator/eval_struct.go:240-248`); slice expressions clamp out-of-range indices (`app/pkg/fctlang/evaluator/eval_expr.go:256-265`); `asNumber` silently strips `length.mm` so units are advisory rather than enforced.
- **manifold**: `Mirror`/`Scale` return the receiver unchanged on zero-length axis (`app/pkg/manifold/manifold.go:535-542`); `Hull` color heuristic picks "first non-NoColor" and drops the rest; warp callback registry silently returns zero when id is missing — corrupting geometry instead of erroring.
- **frontend**: `app/frontend/src/settings.ts:137-150` two silent catches that reset settings to defaults; `data.errors ?? []`, `data.entryPoints ?? []`; `parseFloat(val) || 0`; `resolveThemePalette` falls back to `facet-orange-light` for unknown names.

### 2. Errors swallowed across the cgo / Wails / HTTP boundaries

- **manifold**: `facet_export_mesh`, `facet_import_mesh`, `facet_warp`, `facet_level_set` all return `void` or `nullptr` — the Go layer stat-checks the file to see if export "worked" (`app/pkg/manifold/export.go:29-44`). `ImportMesh` conflates "file missing", "corrupt", "unsupported format", and "no vertices" into one misleading error (`app/pkg/manifold/import.go:17-34`).
- **app shell**: `_, _ :=` / `, _ :=` throughout `app_library.go`, `http_server.go`, `app.go`.
- **frontend**: `catch { btn.textContent = 'Error' }` idiom across `settings_libraries.ts`; `AddRecentFile(...).catch(() => {})`; `GetDocGuides().catch(() => [])`.

### 3. Duplication across parallel pipelines

- **fctlang**: three near-identical statement dispatch switches in `app/pkg/fctlang/evaluator/eval_func.go:78-175`, `app/pkg/fctlang/evaluator/eval_controlflow.go:23-143`, `app/pkg/fctlang/evaluator/eval_controlflow.go:196-286`. Checker and evaluator each build their own symbol tables (`structDecls`, `stdMethods`, `opMap`).
- **manifold**: four extraction functions (`app/pkg/manifold/manifold.go:938-1248`) copy C buffers with nearly identical loops. Expanded-mesh edge/normal computation duplicated in `bindings.cpp:995-1311`. Three cgo platform files copy the same link flag list verbatim.
- **frontend**: 10+ `settings_*.ts` modules each hand-roll `div.settings-color-row > label + input` with inline styles. This is the single biggest DRY win available.

### 4. God objects / files

- **Go**: `App` struct (`app/app.go:112-131`) mixes 5 concerns (Wails ctx, config, assistant, MCP, eval, stderr). Every exported method is JS-callable. `manifold.go` is 1,290 lines across 11 responsibilities.
- **TS**: `viewer.ts` (1416 LOC) combines camera, drawing, grid, headtrack, raycaster, overlays, keyboard controls. `app.ts`, `editor.ts`, `main.ts` all exceed 700 LOC with multiple concerns.

### 5. Checker/Evaluator divergence in fctlang

The checker enforces one set of type rules, the evaluator implements a slightly different set (and mutates via `coerceToType`). The `Length`/`Number` semantics disagreement is the clearest symptom — a function declaring `Number` happily accepts `Length` at runtime. Entry-point "must return Solid" is only checked at runtime.

---

## Critical Items

### Security: Localhost HTTP exposure

`app/http_server.go` runs an **unauthenticated MCP server on a random localhost port with `Access-Control-Allow-Origin: *`**. Any local process (or any webpage in the user's browser via port scan) can call `replace_code` to rewrite the editor and trigger auto-run. This is the single highest-severity item in the entire review. Also: `check_syntax` hand-rolls JSON with `fmt.Sprintf("%q", ...)` which is Go-syntax, not JSON-syntax — malformed for certain inputs.

### Correctness bugs (real, not speculative)

- **`app/frontend/src/app.ts:531-578`** `closeTab`: `cancelEval()` is unreachable because `activeTab` is reassigned before the check at line 547. Closing the tab whose eval is running doesn't cancel the eval.
- **`app/pkg/fctlang/evaluator/eval_types.go:304-318`** `valueEqual` on arrays always returns `false` — `arr == arr` is `false`, breaks the `changed` tracker in `coerceToType`.
- **`app/pkg/manifold/manifold.go:731-745`** `ComposeSolids` panics on empty slice (same for `HullPoints`, `BatchHull`, `Loft`). `CreateCylinder` allows negative `height`.
- **`app/pkg/manifold/manifold_callbacks.go:72-83`** warp callback silently returns zero on missing id — corrupts geometry instead of failing.
- **`app/pkg/fctlang/evaluator/eval_expr.go:256-265`** slice clamps `arr[5:2]` and `arr[-99:2]` to empty instead of erroring; accepts non-integer indices.

### Code-supply / drift risks

- `cgo_{darwin,linux,windows}.go` are parallel copies of the link-flag list — adding a lib to one and forgetting the others fails silently on one platform.
- `SettingsPageContext` / `PageResult` types exported from `app/frontend/src/settings_appearance.ts` (a page implementation) and imported by every other settings page — wrong module ownership.
- README for fctlang documents wrong function signatures (`app/pkg/fctlang/README.md:44,61`).

---

## Top Recommended Actions (ranked)

| # | Action | Area | Why now | Status |
|---|--------|------|---------|--------|
| 1 | Lock down MCP/HTTP server — per-run bearer token, origin validation, remove `CORS: *` | app shell | Security; any webpage can rewrite the editor | ✅ PR #29 |
| 2 | Fix `closeTab` cancellation (`app.ts:531-578`) and `valueEqual` array bug (`eval_types.go:304-318`) | frontend + fctlang | Real correctness bugs, small diffs | ✅ PR #24 |
| 3 | Decide `Length`/`Number` semantics — checker and evaluator must agree | fctlang | Silent unit-stripping is a class of bugs, not one bug | ✅ PR #23, #26 |
| 4 | Surface errors across the cgo boundary — `facet_export_mesh` et al. return `int`/`char*` error | manifold | Exports can silently fail; stat-based detection is hiding bugs | ✅ PR #36 (Assimp) + #30 (warp hard-panic) |
| 5 | Unify the three statement-dispatch switches in the evaluator | fctlang | Every bug fix currently has to land in three places | ✅ PR #35 |
| 6 | Introduce `settings_ui.ts` primitives and migrate one settings page as a pilot | frontend | Biggest DRY/SoC win; unblocks inline-style cleanup | ✅ PR #39 |
| 7 | Consolidate extraction/copy helpers in `manifold.go`; split the 1,290-line file | manifold | Near-identical copies invite drift | ✅ PR #38 |
| 8 | Audit and surface swallowed errors — add a `reportError(where, err)` helper frontend-side; triage `, _ :=` sites backend-side | all | CLAUDE.md lists this as a principle; the code violates it pervasively | ✅ PR #37 (backend) + #39 (frontend toast) |
| 9 | Split `App` struct into services (`AssistantService`, `EvalService`, `LogCapture`, `MCPService`) | app shell | Current trajectory adds a field per feature; eventually untestable | **open** |
| 10 | Type the Wails binding surface end-to-end — remove `any` casts (`lastResult`, `savedTabs`, `AssistantConfig`) | frontend | Compiler loses the ability to catch shape drift | ✅ PR #40 |

Also cleared from Critical Items: boundary validation (`ComposeSolids`/`HullPoints`/`BatchHull`/`Loft` empty slice, `CreateCylinder` negative height) — PR #27; anonymous-struct silent coercion + slice clamping — PR #25; entry-point return-type gate moved to checker — PR #34; `Mirror`/`Scale` zero-length axis + cgo link-flag dedup — PR #31; CORS preflight hotfix — PR #33.

---

## Full Per-Subsystem Reports

---

### Subsystem 1: `app/pkg/fctlang/` — Language Implementation

**Scope:** parser, checker, evaluator, formatter, loader, doc, integration tests.

#### Strengths

- Clean pipeline separation: `parser → loader → checker → evaluator` with a lean root `README.md` describing the contract.
- The AST in `parser/ast.go:8` is well-documented, with `Decl` as a sum type and declared interfaces for `Stmt`/`Expr`.
- `parser.CollectCandidates` (`parser/ast.go:34`) factors out the shared arity-filtering logic used by checker and evaluator.
- Recursion/range bounds are enforced (`parser/parser.go:43` `maxParseDepth`, `evaluator/eval_func.go:177-178` `maxCallDepth`/`maxRangeSize`).
- Context cancellation is threaded through the evaluator (`eval_expr.go:139`, `eval_controlflow.go:33`).
- Test coverage is substantial (~13k lines across 12+ test files), including operator precedence, ASI, methods, overloads, errors, and libraries.
- `checker.Check` (`checker/checker.go:42`) runs multi-pass (register → globals → struct defaults → dup-detect → infer returns → validate), which is the right shape.

#### Critical

1. **`valueEqual` is broken for arrays and self-comparison** (`evaluator/eval_types.go:304-318`).
   `arr == arr` returns `false` even when both sides are the exact same array. The `coerceToType` `changed` tracker uses this to decide whether to rebuild. Either define array equality (deep compare) or make it a hard error at the checker level.

2. **Slice expression silently clamps out-of-range indices** (`evaluator/eval_expr.go:256-265`).
   `arr[5:2]` and `arr[-99:2]` return empty arrays instead of producing positional runtime errors. Go panics here for a reason. Also `sn`/`en` are computed without the `NaN`/non-integer validation that `IndexExpr` applies.

3. **`Length` arithmetic semantics are inconsistent and lossy** (`evaluator/eval_ops.go:238-285`).
   - `Length / Length` → `Number` (documented).
   - `Length * Length` falls into the generic `asNumber` promotion and returns a dimensionless `Number`.
   - `asNumber` (`eval_types.go:352-362`) silently strips `length.mm`, so `5 mm + 3 (Number) → length{mm:8}` without a type error.
   - The checker's `typeCompatible` coerces only `Number → Length/Angle`, but the evaluator goes the other way silently. The two layers disagree on what's legal.

4. **`*functionVal` reports as `"ast.Function"` to users** (`evaluator/eval_types.go:88`). Go implementation detail leaking into user-facing error messages.

5. **Anonymous struct coercion silently succeeds when the target declaration cannot be found** (`evaluator/eval_struct.go:240-248`). Struct gets stamped `typeName = "Foo"` and accepted as valid; errors only show later when a field access or method call fails. Should be a hard error at coercion time.

6. **Entry-point `must-return-Solid` gate lives in evaluator** (`evaluator/eval_run.go:106-108` and `:136-156`). Users get a runtime-only error for what is a static constraint.

#### Important

7. **Massive duplication of statement dispatch.** Three switches handle the same cases (VarStmt, AssignStmt, FieldAssignStmt, IfStmt, AssertStmt, ExprStmt, YieldStmt): `evaluator/eval_func.go:78-175` (`execBody`), `evaluator/eval_controlflow.go:23-143` (`evalBlock`), `evaluator/eval_controlflow.go:196-286` (`evalForBody`). VarStmt's constraint-wrap logic is written three times; AssignStmt's getConstraint/re-validate logic three times. A single `dispatchStmt(stmt, locals, policy)` would collapse this.

8. **Stdlib-globals evaluation is duplicated.** `run()` (`eval_run.go:39-51`) and `evalLibExpr` (`eval_lib.go:56-64`) both re-evaluate every stdlib global per invocation and per library. Compute once in the checker pass and pass through.

9. **Checker and evaluator each build their own struct-decls map and op dispatch table.** `checker.initChecker` (`checker/checker.go:488-528`) and `evaluator.run` (`eval_run.go:14-34`) build nearly identical loops for `structDecls`, `stdMethods`, op tables.

10. **Evaluator and checker traverse the AST with slightly different rules.** The checker's `coerceArgs` path does not exist — the checker enforces `typeCompatible` without mutating, while the evaluator's `coerceToType` / `coerceAnonymousStruct` rewrites values and can succeed where the checker failed (or vice versa).

11. **`functionVal` captures every local by shallow copy, including `*structVal`** (`eval_expr.go:287-297`). `copyValue` does a deep copy for structs but not for arrays. Arrays captured in a lambda alias the outer array. Closure capture semantics are poorly specified.

12. **`evalExpr`'s `default` branch uses `fmt.Errorf` without position** (`eval_expr.go:299-300`). Every other branch uses `e.errAt(pos, ...)`. Same pattern in `eval_controlflow.go:139,280,282`, `eval_func.go:171`, `eval_call.go:18`, `eval_call.go:251`.

13. **`evalBlock` propagates assignments to the enclosing map** (`eval_controlflow.go:113-116`) but `execBody` and `callFunctionVal` do not. Correct Go-like semantics, but the three paths arrived at it via different routes.

14. **Dead/redundant code candidates:**
    - `evaluator/eval_struct.go:149-154` `resolveFieldDefault` called from only one site.
    - `DebugResult.Final` (`evaluator/eval.go:125`) — comment says "populated by callers (not the evaluator)". SoC violation.
    - `checker.initChecker` filters out operators but other loops don't.

15. **Error types are inconsistent.** `parser.SourceError` vs evaluator `source_error.go` vs `fmt.Errorf(...)` sprinkled throughout. Route all evaluator errors through a single `e.errAt` / `e.wrapErr`.

16. **Precedence is undocumented and non-standard.** `parser/parser_expr.go` defines `||`, `&&`, comparison, `|`, `^`, `&`, `+`/`-`, `*`/`/`/`%`. Neither Go (`&` same level as `*`) nor C (`|`/`&` lower than comparison). For a Solid DSL where `+`/`-`/`&` are Boolean ops, `a & b + c` parses as `(a+c) & b`.

17. **Checker/README drift.** README (line 44) says `Eval(ctx, prog, overrides, entryPoint)`; actual is `Eval(ctx, prog, currentKey, overrides, entryPoint)`. README (line 61) says `Format(source string) string`; actual is `Format(src *parser.Source) string`.

18. **`inferReturnTypes` skips overloaded functions entirely** (`checker.go:206-208`). Return types for overloaded user functions are not pre-inferred; subsequent validation uses `unknown`.

#### Minor

- `parser/ast.go:79-84` — `Decl` interface methods written out three times.
- `evaluator/eval.go:18-28` — `constVal`/`constrainedVal` nesting rules fragile.
- `evaluator/eval_run.go:171-191` — PosMap construction overcomplicated; simpler `map[posKey][]uint32` would do.
- `evaluator/eval_dispatch.go:60-68` — "bad-name" handling has duplicated inner loop.
- `checker/checker.go:339-341` — empty `else if` branch with comment-only body.
- `parser/parser_expr.go:183-190` — `isUnitSuffix` does double map lookup.
- `evaluator/eval_compare.go:42-58` — implicit bool ↔ array truthiness in `==`/`!=` is surprising for a statically-typed language.
- `evaluator/eval_expr.go:200-205` — index `NaN`/non-integer validation exists; slice lacks it.
- `evaluator/eval.go:27` — `name string` field on `constrainedVal` stored for error messages only.
- `evaluator/eval_lib.go:37-49` — sub-evaluator built by hand, duplicating `newEvaluator` init.

#### Recommended next steps (prioritized)

1. Fix the `valueEqual` array bug and tighten slice/index runtime errors (Critical #1, #2).
2. Unify statement dispatch into one implementation (Important #7).
3. Decide and enforce Length/Number semantics (Critical #3, Important #10).
4. Remove the anonymous-struct fallback and the entry-point runtime gate (Critical #5, #6).
5. Consolidate program-wide symbol tables (Important #8, #9).

---

### Subsystem 2: `app/pkg/manifold/` — 3D Kernel & Mesh I/O

**Scope:** cgo wrapper for Manifold C++ kernel, mesh import/export, text/font rendering.

#### Strengths

- Clear file layout separating Go FFI, callback registry, import/export, polymesh logic, text, platform link flags.
- Defensive cgo lifetime hygiene: every transform/boolean pairs the C call with `runtime.KeepAlive(a/b)`, and `newSolid` installs a finalizer.
- Primitives validate inputs before calling into C (recent "zero-dim" hang fix enforced at a clean boundary).
- Callback bridge well-documented about the mutex contract — serialization of TBB worker threads into the non-reentrant Go evaluator is correct.
- Tests exist for main shapes: display mesh, platonic/Conway operators, import round-trips, external memory accounting.

#### Critical

1. **Warp/LevelSet callback can be freed while TBB threads still hold the id.**
   `manifold_warp.go:13-21`: `Warp` registers an id, calls C, unregisters on defer. Bridge (`manifold_callbacks.go:72-83`) silently returns on missing id — producing corrupt geometry with no error. Same pattern in `facetLevelSetBridge` returns `0`, which can flood an entire SDF with the zero iso-surface. Either the id is guaranteed present (delete the nil branch and hard-error), or there's a real race (fix the lifetime).

2. **`Mirror`/`Scale` return the input on zero-length axis** (`manifold.go:535-542`). Silent fallback; a zero mirror plane is a programmer error and should produce `error` like `CreateCube`, not an identity transform.

3. **Platform-specific link flags drift — 3rd-party paths hard-coded per-OS.** `cgo_{darwin,linux,windows}.go` each list `-lfacet_cxx -lmanifold -lClipper2 -ltbb -lassimp ...` with only `-lc++` vs `-lstdc++` differing. `/opt/homebrew/lib` hard-coded on darwin. Build drift risk.

4. **`CreateCylinder` allows negative height** (`manifold.go:280-295`). Rejects `height == 0` and `radius < 0` but not negative height.

5. **`HullPoints` does not validate n < 4** (`manifold.go:654-663`). With <4 points the C hull call on a degenerate set is undefined.

6. **`ComposeSolids` with empty slice panics** (`manifold.go:731-745`). Dereferences `&ptrs[0]` with no length check. `BatchHull`, `SketchBatchHull`, `Loft` have the same pattern.

7. **`ImportMesh` conflates "file does not exist" and "file has no vertices"** (`import.go:17-34`). Error on `ptr == nil` says "no vertices found in %s", but C++ returns an empty `MeshGL` for any failure. The underlying Assimp error is discarded at `bindings.cpp:862-866`.

8. **`ExportMesh` silently succeeds when C fails** (`export.go:29-44`). `C.facet_export_mesh` returns `void`. Go learns of failure only by stat'ing the file. C binding should return an error code.

#### Important

1. **Heavy duplication in extracted-mesh code paths.** `extractDisplayMesh` (`manifold.go:938-1005`), `MergeExtractDisplayMeshes` (`:1092-1180`), `MergeExtractExpandedMeshes`/`appendExpandedData` (`:131-171`, `:1185-1248`) all do the same C-buffer-to-byte-slice-with-stride copy. On the C++ side, the expanded-mesh edge/normal lambda is duplicated in `bindings.cpp:995-1127` vs `:1164-1311`. ~150 lines copy-pasted.

2. **Dead / obsolete code:**
   - `mergeDisplayMeshes` (`manifold.go:1007-1088`) marked deprecated but still used by tests.
   - `facet_cs_empty` (`bindings.cpp:88-90`) never called from Go.
   - `facet_rotate` (old API) never called from Go.
   - `facet_scale_local` / `facet_mirror_local` declared but never called — Go implements via translate-op-translate. Two implementations in two languages.
   - `facet_as_original` exported but unused.
   - `Mesh.Normals` field never populated.
   - `DisplayMesh.FaceGroupRaw` assigned in both indexed and expanded paths with different meanings.

3. **`mergeFaceMaps` is asymmetric but used symmetrically** (`manifold.go:188-191`). Doc says "a's value wins"; `ComposeSolids` loops `r.FaceMap = mergeFaceMaps(r.FaceMap, s.FaceMap)`, so any face-ID collision between solids 1 and 3 is silently resolved by solid 1.

4. **`Hull`/`BatchHull` face-color heuristic is lossy** (`manifold.go:607-651`). Picks the first non-`NoColor` color and applies uniformly. For multi-color hull, silently drops 2..N colors. Fallback hiding the real problem.

5. **`SetColor` mutates the receiver but looks like a chainable setter** (`manifold.go:219-226`). All other unary methods return new `*Solid`.

6. **`unionAll` is O(n) pairwise left-fold** (`export.go:18-24`). Slow for large collections. Balanced reduction or `facet_compose` would be better.

7. **`ExportSTL` → `WriteSTL` ignores `FaceColors`, but builds them anyway** (`export.go:181-191`). `extractRunMesh` does too much for the STL path.

8. **`DefaultFontPath` leaks the temp file and silently returns `""` on failure** (`text_font.go:18-39`). Writes embedded TTF to temp, never cleans up. On error returns `""` which causes misleading `failed to load font ""` downstream.

9. **`text.cpp:107-190` per-call FreeType init/teardown + UTF-8 decoder.** Re-initialises FreeType on every `CreateText` call. `FT_New_Face(path)` re-reads and re-parses TTF every call. Hand-rolled UTF-8 decoder silently skips invalid bytes.

10. **`CreatePolygon` mutates the caller's slice in place** (`manifold.go:336-340`). Surprising side-effect for a pure constructor.

11. **`memSize` is cached but based on a heuristic** (`bindings.cpp:39-45`). Estimates memory as `nv*(24+np*8) + nt*108`. Constants undocumented. No reality check against C allocator.

#### Minor

- `manifold.go:220` — color packing assumes `[0,1]`; no clamp.
- `manifold.go` is 1300+ lines across 11 responsibilities; split into `primitives.go`, `booleans.go`, `transforms.go`, `extraction.go`, `hull.go`.
- Lowercase `scale`/`mirror` helpers confusingly named next to exported `Scale`.
- `polymesh.go:82-86` — `faceCentroid` guards `n == 0` but callers assume non-empty.
- `polymesh.go:229-231` — `newDodecahedron = icosahedron.Dual()` computed every call.
- `polymesh_extract.go:54-56` — silently truncates malformed C output.
- Three places duplicate "wrap pointer + set single-entry origin FaceMap" logic.
- `manifold.go:1010` — early-return returns caller's mesh pointer; mutations alias input.
- `polymesh.go:267-340` — `orderFaceRing` is subtle, >70 lines, should be own file.
- `bindings.cpp:44` — magic numbers without comment.
- `polymesh.cpp:117-124` — ordered `std::map` where `unordered_map` is usual.
- `text_font.go:26,33` — three parallel `log.Printf` calls.
- `Mesh.FaceColorMap map[string]string` wasteful; should be `uint32 -> uint24` unless JSON-concession.

#### Recommended next steps (prioritized)

1. Fix callback lifetime or remove the silent-nil fallback (Critical #1).
2. Remove dead C functions, Go fields, deprecated paths.
3. Consolidate the three extraction paths in `manifold.go` and their C++ counterparts.
4. Split `manifold.go` into files with one responsibility each.
5. Surface C-side errors across the cgo boundary.

---

### Subsystem 3: `app/frontend/src/` — TypeScript Frontend

**Scope:** ~30 TS modules, ~9K LOC. Vanilla TypeScript — no framework. Wails bindings to Go.

#### Strengths

- `eval-client.ts` (57 LOC): focused, typed, uses `AbortController` correctly, no fallbacks, no `any`. Model citizen.
- `filetree.ts` (190 LOC): callback-based interface, clean SoC.
- `dialogs.ts` (174 LOC): self-contained, promise-based modals with proper cleanup.
- `function-preview.ts` (347 LOC): deliberate DOM reconciliation; well-documented intent.
- `headtrack.ts` (265 LOC): well-isolated MediaPipe wrapper, clean lifecycle.
- `toolbar.ts` (270 LOC): pure DOM building, no state leaks.
- `settings.ts` explicit `pick()` migration and one-time localStorage migration pattern is exactly right.
- `assistant.ts`: clear class, streaming logic readable, `applyEdit` SEARCH/REPLACE helper well-factored.
- Monaco integration (`editor.ts`): tree-shaken imports, custom language grammar isolated.

#### Critical

1. **`closeTab` never cancels in-flight eval on the active tab** — `app.ts:531-578`.
   Guard at line 547 is unreachable:
   ```
   537  if (activeTab === file) {
   541      switchToTab(remaining[0]);   // reassigns activeTab
   543      activeTab = '';              // or clears it
   545  }
   547  if (file === activeTab) cancelEval();   // always false
   ```
   Capture `const wasActive = activeTab === file;` at the top, then branch on `wasActive`.

2. **Silent `catch` on settings load falls back to defaults** — `settings.ts:137-150`. Broken backend or malformed payload silently becomes "reset to defaults." Users lose configuration with no log, no dialog.

3. **`lastResult: any` leaks untyped data through the whole eval pipeline** — `app.ts:102` and downstream. `data.errors ?? []` / `data.entryPoints ?? []` fallbacks mean any shape change on the Go side goes undetected. Define `EvalResult` interface.

#### Important

4. **Wholesale error-swallowing pattern.** Not occasional; the default idiom. Sites:
   - `settings_libraries.ts:14` — `asyncButton`: `} catch { btn.textContent = 'Error'; }`
   - `settings_libraries.ts:177, 231` — `innerHTML = 'Failed to load...'`
   - `app.ts:837, 849, 886` — `AddRecentFile(...).catch(() => {})`
   - `app.ts:968` — `GetDocGuides().catch(() => [] as any[])`
   - `main.ts:538` — `folders = []` on library list failure
   - `assistant.ts:148` — `} catch { /* ignore detection errors */ }`
   - `editor.ts:617-618` — Monaco completion silently degrades
   - `headtrack.ts:171` — `detectForVideo` silent catch

5. **Pervasive `?? default` / `|| default` fallback values.**
   - `viewer.ts:143-159` — every appearance setting is `appearance?.field ?? hardcodedDefault`. Duplicates defaults.
   - `themes.ts:490, 493` — `resolveThemePalette` falls back to `'facet-orange-light'` for unknown theme.
   - `function-preview.ts:207` — `parseFloat(numInput.value) || 0` treats `NaN` as 0.
   - Multiple sites: `data.errors ?? []`, `data.entryPoints ?? []`, `stats?.x ?? …`.

6. **Heavy duplication across `settings_*.ts`.** 10+ page modules each hand-construct:
   - `div.settings-color-row > label + input/select` (20+ repetitions)
   - Inline styles: `style.color = '#888'`, `style.fontSize = '13px'`, `style.marginLeft = '8px'`
   - Hex/rgba color parsing (duplicated)

   Expand `settings_ui.ts` to own `settingsRow`, `settingsSelect`, `settingsNumberInput`, `settingsColorSwatch`. Biggest SoC/DRY win available.

7. **Architectural smell: shared types exported from a page module** — `settings_appearance.ts`. `SettingsPageContext` and `PageResult` imported by every other `settings_*.ts`. Belong in `settings_ui.ts` or a new `settings_types.ts`.

8. **God files:**
   - `viewer.ts` (1416 LOC) — split off `viewer-grid.ts`, `viewer-drawing.ts`, `viewer-parallax.ts`.
   - `app.ts` (1041 LOC) — tab system extracts cleanly into `tabs.ts`.
   - `editor.ts` (888 LOC) — themes and completion providers could be separate files.
   - `main.ts` (754 LOC) — divider/resizer logic could move to `layout.ts`.

9. **`any` casts bypass Wails bindings.**
   - `main.ts:135, 489` — `SetAssistantConfig(settings.assistant as any)`
   - `main.ts:165-166` — `(settings as any).savedTabs`
   - `app.ts:102, 470` — `lastResult: any`, `handleEvalHTTPResult(data: any, …)`

10. **Dead code:**
    - `settings.ts:103-107` — `themeRenames` entries like `'tomorrow': 'tomorrow'` are identity mappings.
    - `editor.ts` — `undo()` / `redo()` exported but unused.

#### Minor

- Inline styles everywhere — move to CSS classes. Also makes theme overrides impossible.
- `main.ts:271, 430, 726, 743` — document-level listeners without cleanup.
- `themes.ts` — palette table repeats identical base values across variants.
- Naming: `SettingsPageContext` / `PageResult` not descriptive.
- `fullcode.ts` (137 LOC) module globals mirror `app.ts` pattern.
- `viewer.ts:1093` — `_addHiddenLinesForMesh` duplicates work on toggle.

#### Recommended next steps (prioritized)

1. Fix the `closeTab` bug. Smallest change, biggest correctness win.
2. Introduce `settings_ui.ts` primitives; pilot on `settings_camera.ts`, roll across.
3. Replace the error-swallowing idiom with a `reportError` toast helper.
4. Type the Wails binding surface — delete `any` casts.
5. Relocate `SettingsPageContext` out of `settings_appearance.ts`; delete dead `themeRenames`.

---

### Subsystem 4: App Shell — Top-level Go + `app/pkg/docgen/`

**Scope:** Wails bindings, http server, assistant, eval handler, file manager, menus, slicer, webview, docgen.

#### Strengths

- File organization by concern is mostly reasonable (`app_menu.go`, `app_library.go`, etc.).
- Eval cancellation pattern at `eval_handler.go:254-261` is clean: mutex-guarded `cancelEval`, derive child context, replace before dispatch.
- HTTP server bound to `127.0.0.1:0` and shut down on context cancellation. Good default posture.
- Platform handling via `runtime.GOOS` switches is explicit and readable. Zombie reaping via `go cmd.Wait()` is correct.
- `SourceUser` guard on example names prevents path traversal.
- `appConfig.json.RawMessage` for frontend-owned sections is pragmatic.

#### Critical

1. **MCP server is unauthenticated — any process on the machine can read/mutate editor contents.** `http_server.go:74-192` exposes MCP tools (`get_editor_code`, `edit_code`, `replace_code`, `check_syntax`) over plain HTTP on random localhost port with `Stateless: true`. Any local process (including browser tabs via DNS rebinding against `127.0.0.1`) can hit the endpoint. `setCORS` at `eval_handler.go:227` sends `Access-Control-Allow-Origin: *`. `replace_code` + auto-run = malicious page can rewrite editor. Port is guessable via scan.

2. **`check_syntax` MCP tool embeds untrusted input as a Go literal and POSTs back to self.** `http_server.go:160`: `fmt.Sprintf(`{"source":%q}`, source)` — `%q` produces Go-syntax quoted string, NOT JSON. Fragile hand-rolled encoder where `json.Marshal` exists. Also self-loopback call — could invoke `handleCheck` directly.

3. **`SendAssistantMessage` passes system prompt via `--system-prompt` argv.** `assistant.go:571`. On some platforms argv is readable by other users (`ps`). Long prompts may hit `ARG_MAX`.

4. **Startup race on `a.ctx` in `initStderrCapture`.** `app.go:463-468` and `:457` guard `if a.ctx != nil` suggesting someone was worried. `os.Stderr = w` at `:442` is a global mutation performed once and never reversed. Double-call would leak the pipe.

5. **`handleEval` mutex window.** `eval_handler.go:258`: `ctx` from `r.Context()`. Mutex released at `:260` before dispatch, so concurrent evals can interleave. Audit whether `loader.LoadMulti` / `evaluator.Eval` assume single-threaded access.

#### Important

6. **`App` is a God object.** Mixes Wails context, config mutex, assistant state (3 fields), MCP state (2 fields), eval mutex+cancel, stderr ring buffer, log file handle. Split into `AssistantService`, `HTTPService`, `EvalService`, `LogCapture`.

7. **Wails binding surface is very broad.** Every exported `App` method is JS-callable. Includes destructive operations: `ClearLibCache`, `InstallLibrary` (git clones arbitrary URLs), `SaveFile` (arbitrary path), `SetMemoryLimit`, `RunGC`, `SendAssistantMessage`. Any future XSS in webview = arbitrary file write.

8. **`loadConfig`/`saveConfig` are lock-free but called from methods holding `configMu`.** `app.go:171-198`. Invariant "config accesses serialized via `a.configMu`" not enforced by types. Should be inside `loadConfig/saveConfig` or methods on a `ConfigStore`.

9. **Errors silently swallowed:**
   - `app.go:174` — `loadConfig` returns `appConfig{}` on any file read error. User-data-loss bug waiting to happen.
   - `app.go:218` — `existing, _ := os.ReadFile(configPath())`
   - `app.go:562` — `dir, _ := scratchDir()`
   - `app_library.go:115-130, 158, 239, 311-313` — `ReadDir` / `RemoveAll` errors ignored
   - `http_server.go:168` — `body, _ := io.ReadAll(resp.Body)`

10. **Duplication: per-CLI arg building in `runGenericCLIStream`** (`assistant.go:740-775`). Hand-builds args for 4 CLIs with near-identical shape. Could be a table.

11. **Duplication in docgen deduplication.** `pkg/docgen/docgen.go:54-68` and `eval_handler.go:357-373` both dedupe with same `name+"|"+library` key. Extract `doc.DedupeEntries`.

12. **`buildDocIndex` duplicates work on every eval.** `eval_handler.go:376-390`. Cache; invalidate in `rebuildSystemPrompt` (already the canonical library-changed hook).

13. **`parseAutoPull` reimplements partial JSON decoding** (`app.go:294-305`). Manual unmarshal of frontend-owned settings blob; silently stops working on rename.

14. **`entry_points.go`: only iterates main program, never libraries.** `entry_points.go:106-110`: `LibPath`/`LibVar` always `""`. Either dead parameter or half-finished feature.

15. **`getEntryPoints` sort mis-handles two `Main`s** (`entry_points.go:112-121`). `less(Main, Main) == true` violates strict weak ordering.

16. **Windows webview support is missing.** `webview_darwin.go` and `webview_linux.go` grant media capture permissions; no `webview_windows.go`. Given `knownCLIs` and slicer defs include Windows paths, Windows is a target.

17. **Subpar error propagation in `ExportMesh`.** `app_export.go:44-50`: user-cancelled sentinel returned as nil. Dedicated sentinel or `(ok bool, err error)` would be clearer.

18. **`InstallLibrary` constructs clone URL from trimmed input without validation** (`app_library.go:27-41`). Relies on git to sanitize. Also silently upgrades `http://` → `https://`.

#### Minor

- `app.go:124-125` — whitespace inconsistency in `App` struct.
- Log file name is `YYYY-MM-DD.log` based on wall clock at startup; doesn't rotate.
- `app.go:335` — `return result != "Yes"` — comment vs code readability.
- `assistant.go:44-51` — `knownCLIs` static; `queryModels` has no `claude` case.
- `assistant.go:562-565` — `MaxTurns` default 10 duplicated in comment.
- `eval_handler.go:68-72` — alignment in struct tags off.
- `eval_handler.go:445` — `fmt.Errorf("%s", checked.Errors[0].Message)` loses all but first error.
- `eval_handler.go:447-449` vs `:146` — same condition, two meanings.
- `slicer.go:83` — `filepath.WalkDir /Applications` on every `DetectSlicers` call is slow.
- `pkg/docgen/docgen.go:78` — `goldmark.New()` per call.
- `pkg/docgen/template.go` — CSS inlined as Go string; should be `//go:embed docs.css`.

#### Recommended next steps (prioritized)

1. Harden localhost HTTP server. Drop `CORS: *`, add origin/host allowlist, per-run bearer token.
2. Split `App` into services.
3. Audit error-swallowing; triage `_ = err` and `, _ :=` sites.
4. Fix `check_syntax` self-loop; use `json.Marshal` and in-process call.
5. Cache `buildDocIndex` / `collectLibDocEntries` with library-changed invalidation.
