# Building Facet from Source

## Prerequisites

- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation) or `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- CMake + Ninja
- Node.js / npm
- A C/C++ compiler (Xcode Command Line Tools on macOS, gcc/g++ on Linux)

## Custom Go Toolchain

Facet requires a patched Go runtime that exposes `runtime.ExternalAlloc` and `runtime.ExternalFree`. These let the garbage collector account for memory allocated by C/C++ code (Manifold), preventing the GC from under-collecting when large C allocations are in play.

The toolchain is built automatically on first `make` and installed to `.go-toolchain/` (gitignored). It does not affect your system Go installation.

## Setup

```bash
git submodule update --init --recursive
```

## Development

```bash
make dev
```

On first run this will:
1. Clone and build the custom Go toolchain into `.go-toolchain/`
2. Build the Manifold C++ geometry library
3. Launch the Wails dev server with hot reload

Subsequent runs skip steps 1-2 if already built.

## Building

```bash
make build
```

The app binary is produced at `app/build/bin/Facet.app` (macOS).

## Other Targets

| Command | Description |
|---------|-------------|
| `make dev` | Dev server with hot reload |
| `make build` | Build the desktop app |
| `make cli` | Build the `facetc` CLI compiler |
| `make test` | Run tests |
| `make test-race` | Run tests with race detector |
| `make clean` | Remove all build artifacts |

## Frontend Type Check

```bash
cd app/frontend && npx tsc --noEmit
```
