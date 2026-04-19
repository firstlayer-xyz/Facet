# Architecture Notes

Running notes on non-obvious architectural decisions. Add to this as things come up — don't let the *why* get lost in the code.

## Frontend ↔ Backend Transport

Two channels coexist:

1. **Localhost HTTP server** ([app/mcp_service.go](../app/mcp_service.go), [app/http_server.go](../app/http_server.go)) — bound to `127.0.0.1:<random-port>`, bearer-token authenticated, Host-header validated. Routes:
   - `POST /eval` — source → evaluation result (mesh data, Declarations, References, errors)
   - `POST /check` — source → syntax/type errors only
   - `POST /mcp` — MCP streamable HTTP handler for assistant tools (`get_editor_code`, `edit_code`, `replace_code`, `check_syntax`, `get_documentation`)

2. **Wails bridge** — bound IPC between Go and the WebView. Used for short, low-volume messages: app lifecycle, UI events, file dialogs, and runtime events (`wailsRuntime.EventsEmit`) such as `assistant:replace-code`.

### Why `/eval` is HTTP, not Wails

The Wails bridge serializes everything through a JSON string channel and is noticeably slow for large payloads. Eval results are **big and binary-heavy** — mesh vertex/index/normal buffers can run into megabytes. Putting eval over HTTP lets us stream raw bytes (and use compression) at a speed the Wails bridge cannot match.

**Rule of thumb:**
- **HTTP** for anything with binary payloads, large JSON, or streaming.
- **Wails bridge** for small control-plane messages and events.

When adding a new endpoint, put it on the HTTP mux if payload size is unbounded; use Wails events for sub-kilobyte notifications.

### MCP and the HTTP surface

The same HTTP server hosts `/mcp`, so external MCP clients (Claude Code, etc.) talk to the app through the same authenticated localhost endpoint the frontend uses. This is convenient for future tools that expose compiler/type-system data (e.g., `lookup_symbol`, `resolve_symbol`) — they can reuse the evaluator pipeline's output (Declarations, References) without a second transport.

## Go ↔ C++ Boundary

The geometry kernel is C++ — Manifold, Assimp, FreeType. Everything else is Go. The C++ side exists because those libraries are C++; it is **not** a general-purpose geometry layer we add code to by default.

The boundary is defined in [app/pkg/manifold/cxx/include/facet_cxx.h](../app/pkg/manifold/cxx/include/facet_cxx.h) (one `extern "C"` block) and wrapped Go-side by files in [app/pkg/manifold/](../app/pkg/manifold/) — primitives, booleans, transforms, extrusions, operations, queries, mesh I/O. No caller bypasses that wrapper; all cross-boundary traffic funnels through it.

### Default: Go

When you need to add something new, write it in Go unless it has to call a C++ library. Validation, formatting, error prose, parameter munging, algorithmic code that manipulates data already owned by Go — all Go. Cross the boundary only to invoke Manifold / Assimp / FreeType.

### Rules

1. **C++ wraps libraries, not logic.** A function that doesn't call into libmanifold, libassimp, or FreeType doesn't belong in `cxx/`. Pure math, parameter validation, data transformation, error formatting — Go.

2. **CGO overhead is the metric.** Each cross is ~150–300 ns regardless of what the call does. That's negligible when the C call does real geometry work (μs–ms), and dominant when it doesn't.
   - Composing substantial ops (e.g. `translate → scale → translate` for a pivot transform) is fine — the overhead is a rounding error on the actual work.
   - The patterns that matter: **hot loops** (per-vertex, per-triangle, per-primitive calls) and **trivial payloads** (a call that returns two floats or copies 8 bytes). Batch the former, fold or move the latter.

3. **Arrays cross once per call, one direction.** If you have N things to process, pass all N in one call. If you get N things back, get them in a single buffer. No "for v in verts: call C" patterns — that's the hot-loop trap from rule 2.

4. **Memory ownership is unambiguous.** C++ allocates → C++ frees (via `facet_delete_*` or `facet_free_string`). Go never holds a C pointer across function returns except through opaque handles (`Solid*`, `Sketch*`). Strings allocated in C are copied immediately in Go and freed.

5. **Errors cross as codes; user prose is Go.** C returns an enum or a numeric status; Go formats the sentence the user sees. The one exception is third-party library error strings (Assimp parse errors, FreeType failures) which cross opaquely as C-allocated strings and are freed via `facet_free_string`.

6. **Go→C callbacks only when the library demands it.** `Warp` and `LevelSet` exist because Manifold calls back per vertex / grid cell. Every callback is a trap across the boundary. Don't invent new ones to paper over a missing C function — add the C function instead.

7. **File I/O in C++ only when the library owns the format.** Assimp handles mesh formats (OBJ, STL, 3MF) and FreeType handles fonts; those stay in C++. Facet's own metadata (colors, face IDs, project files, JSON, logs) — Go.

8. **Scalars return by value, not by pointer.** Volume, area, genus, bounding-box components — direct returns. No output parameters for things that fit in a register.

### What *not* to infer from these rules

- Chaining C calls is not banned. Three Manifold ops in a row is just using the API.
- C++ isn't off-limits for custom geometry — Sweep and Loft (in [bindings.cpp](../app/pkg/manifold/cxx/src/bindings.cpp)) contain Facet-specific frame/contour math because it's tightly coupled to building a `Manifold` from vertex/triangle arrays. Moving that to Go would marshal the same data back across the boundary, which isn't automatically cheaper. Revisit only with a measurement.
- These rules describe the boundary, not an aesthetic preference for one language. Pick the side where the library lives.
