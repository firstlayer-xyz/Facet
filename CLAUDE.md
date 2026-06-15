# CLAUDE.md

## Build & Test

- **Build:** `make build` (never use raw `go build`)
- **Test:** `make test` (never use raw `go test`)
- **Go toolchain:** Always use the custom Go toolchain at `.go-toolchain/bin/go`. Never use the system `go` binary. The project requires Go 1.25+ with custom patches (e.g., `runtime.ExternalAlloc`).
- **Frontend type check:** `cd desktop/frontend && npx tsc --noEmit`
- **Dev mode:** `make dev`

## Engineering Principles

### Quality Over Speed

We are not in a hurry. We want good solutions with proper engineering discipline. Take the time to understand the problem fully before writing code. Root-cause issues — don't just find the simplest workaround and move on.

### Refactor Constantly

Actively look for opportunities to improve code as you work. If you see something that could be better — naming, structure, duplication, unclear boundaries — fix it. Don't leave messes for later.

- Large changes are OK. It does not matter if a change "will touch a lot of files."
- Refactoring is a good thing, not something to avoid.
- If a file is doing too much, split it. If two files do the same thing, merge them.
- Remove dead code. Unused functions, unreachable branches, unnecessary exports, stale imports — delete them. Dead code is not "safe to keep around"; it's misleading and adds maintenance burden.

### DRY (Don't Repeat Yourself)

Never duplicate logic. If the same code exists in two places, extract it to a shared location. When fixing duplicated code, move it — don't update one copy and hope the other stays in sync.

**Caveat:** A little repetition is better than a little dependency. Don't create a shared utility that ties two otherwise-independent modules together just to save three lines of code. If the shared code would create a coupling that doesn't make architectural sense, it's better to repeat it.

### Separation of Concerns

Each module, file, and function should have one clear responsibility. If you can't describe what a unit does in one sentence, it's doing too much. Break it apart.

### No Fallbacks — Fix the Root Cause

Implementing fallbacks is an error. If something fails, understand why and fix the underlying problem. Don't add `try/catch` wrappers, default values, or `|| ''` fallbacks that mask real bugs. The correct fix is the one that makes the error impossible, not the one that hides it.

### Symptoms vs. Problems

The user often describes a symptom because they don't understand the underlying problem. Don't cure the symptom — find the problem. Ask clarifying questions if needed. The visible bug is usually a consequence of a deeper architectural issue. Fix the architecture, and the symptom goes away on its own.

### Error Handling

Errors should be specific, actionable, and caught as early as possible (prefer compile-time over runtime). Never silently swallow errors. If a condition shouldn't happen, make it a hard error — don't code around it defensively.

## Commit Practices

- Never auto-commit. Only commit when explicitly asked.
- Never use `git add -A` without excluding `.claude/`.

## Project Structure

- `desktop/` — Wails desktop app (Go backend at the repo root, frontend under `desktop/frontend/`)
- `pkg/fctlang/` — The Facet language: parser, checker, evaluator, formatter
- `pkg/manifold/` — Go bindings to the C++ geometry layer (`pkg/manifold/cxx/`)
- `desktop/frontend/src/` — TypeScript frontend (vanilla, no framework)
- `share/stdlib/` — Standard library (`std.fct`) embedded into the binary
- `share/examples/` — Bundled example `.fct` files
- `web/` — Browser preview (wasm bundle + Playwright tests under `web/test/`)
- `cmd/` — Standalone Go entry points (e.g., `cmd/facetc`)
- `third_party/` — Pinned upstream sources for freetype and manifold (cloned on demand by `scripts/build-manifold.sh`; gitignored)
- `facetlibs/` — External Facet libraries (gitignored; tests fetch via the loader's git cache, no manual clone needed)
- `docs/` — Project documentation
- `scripts/` — Build, release, and dev-server scripts
- `.github/` — Workflows (`ci.yml`, `release.yml`, `guards.yml`, `profile-windows.yml`) and `CODEOWNERS`
