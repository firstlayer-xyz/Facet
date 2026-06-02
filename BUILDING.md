# Building Facet from Source

## Prerequisites

- [Wails CLI v2.12.0](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0` (matches `go.mod` and CI)
- CMake + Ninja
- Node.js / npm
- A C/C++ compiler (Xcode Command Line Tools on macOS, gcc/g++ on Linux)
- For web (wasm) builds only: [emsdk](https://emscripten.org/) on `PATH` and `wasm-opt` from [binaryen](https://github.com/WebAssembly/binaryen)

## Custom Go Toolchain

Facet requires a patched Go runtime that exposes `runtime.ExternalAlloc` and `runtime.ExternalFree`. These let the garbage collector account for memory allocated by C/C++ code (Manifold), preventing the GC from under-collecting when large C allocations are in play.

The toolchain is built automatically on first `make` and installed to `.go-toolchain/` (gitignored). It does not affect your system Go installation.

## Third-Party Sources

`scripts/build-manifold.sh` clones pinned versions of manifold, assimp, and freetype into `third_party/` on demand. There are no git submodules — nothing to init beyond `git clone`.

## Development

```bash
make dev
```

On first run this will:
1. Clone and build the custom Go toolchain into `.go-toolchain/`
2. Build the Manifold C++ geometry library into `pkg/manifold/cxx/build-<os>-<arch>/`
3. Launch the Wails dev server with hot reload

Subsequent runs skip steps 1-2 if already built.

## Building

```bash
make build
```

The app binary is produced at `desktop/build/bin/Facet.app` (macOS); `make run` builds then opens it.

## Make Targets

| Command | Description |
|---------|-------------|
| `make dev` | Wails dev server with hot reload |
| `make build` | Build the desktop app (`desktop/build/bin/Facet.app`) |
| `make run` | Build + open the desktop app |
| `make cli` | Build the `facetc` CLI compiler (`build/bin/facetc`) |
| `make wasm` | Build the wasm bundle (`build/bin/facet.wasm` + JS shims, copied to `web/`) |
| `make wasm-cxx` | Build just the C++ geometry layer to wasm (Emscripten) |
| `make serve-web` | Static dev server for the wasm bundle in `web/` |
| `make test` | Run the Go test suite |
| `make test-race` | Run Go tests with `-race` |
| `make test-desktop` | Run the desktop frontend Playwright suite (mocked Wails harness, no Wails build needed) |
| `make test-web` | Run the Playwright browser test suite for the wasm preview (requires `make wasm` first) |
| `make check-shims` | Verify the wasm JS bridge stays in sync with the cgo build |
| `make clean` | Remove all build artifacts (toolchain, third-party builds, `desktop/frontend/dist`) |

## Frontend Type Check

```bash
cd desktop/frontend && npx tsc --noEmit
```
