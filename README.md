# Facet

Code-driven CAD application for 3D modeling.

Built with **Wails v2** (Go + webview), **Manifold** (C geometry library via cgo), and **Three.js** (3D rendering).

## Prerequisites

- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation) or `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- CMake
- Node.js / npm
- A C/C++ compiler (Xcode Command Line Tools on macOS, gcc/g++ on Linux)
- [Zig](https://ziglang.org/) (for cross-compilation only — `make cross-build`)

A **custom Go toolchain** is required — Facet uses `runtime.ExternalAlloc`/`ExternalFree` from a fork of Go that adds CGo memory pressure tracking. The Makefile handles this automatically.

## Setup

```bash
git submodule update --init --recursive
```

## Development

```bash
make dev
```

On first run this will:
1. Clone and build the custom Go toolchain from `github.com/szatmary/go` (`feature/cgo-memory-pressure` branch) into `.go-toolchain/`
2. Build the Manifold C++ geometry library
3. Launch the Wails dev server with hot reload

Subsequent runs skip steps 1-2 if already built.

## Building

```bash
make build
```

The app binary is produced at `app/build/bin/facet.app` (macOS).

## Other Targets

| Command | Description |
|---------|-------------|
| `make cli` | Build the `facetc` CLI compiler |
| `make mcp` | Build the MCP server |
| `make test` | Run tests |
| `make test-race` | Run tests with race detector |
| `make cross-all` | Cross-compile for all platforms (requires zig) |
| `make clean` | Remove all build artifacts |

## Custom Go Toolchain

Facet requires a patched Go runtime that exposes `runtime.ExternalAlloc` and `runtime.ExternalFree`. These functions let the Go garbage collector account for memory allocated by C/C++ code (Manifold), preventing the GC from under-collecting when large C allocations are in play.

The toolchain is built automatically by `make` via `scripts/setup-go.sh` and installed to `.go-toolchain/` (gitignored). It does not affect your system Go installation.
