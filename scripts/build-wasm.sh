#!/usr/bin/env bash
# Build the facet_cxx geometry layer for browser wasm via Emscripten.
#
# Output (override with $OUT_DIR):
#   build/bin/facet_cxx.js    — JS loader (Module factory)
#   build/bin/facet_cxx.wasm  — wasm module
#
# Requires:
#   emsdk active in PATH (emcc, emcmake)
#
# Builds against Manifold's first-class Emscripten support. -DFACET_WASM swaps
# the warp/level-set callbacks for JS-host bridges; mesh I/O is handled JS-side
# (no native file I/O in the browser). FreeType is linked through Emscripten's
# port (-sUSE_FREETYPE) so text.cpp renders glyphs identically to the native
# build, from in-memory font bytes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MANIFOLD_DIR="$PROJECT_ROOT/third_party/manifold"
CXX_DIR="$PROJECT_ROOT/pkg/manifold/cxx"
. "$SCRIPT_DIR/_third-party-versions.sh"
OUT_DIR="${OUT_DIR:-$PROJECT_ROOT/build/bin}"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 4)"

if ! command -v emcc >/dev/null 2>&1; then
  echo "error: emcc not on PATH. Activate emsdk first."
  exit 1
fi

# --- Clone manifold if needed ---
# third_party/manifold is gitignored, not a submodule. build-manifold.sh
# clones it lazily for the desktop build; do the same here so a fresh
# checkout can `make wasm` without first running `make manifold`.
if [ ! -f "$MANIFOLD_DIR/CMakeLists.txt" ]; then
  echo "Cloning manifold ${MANIFOLD_VERSION}..."
  rm -rf "$MANIFOLD_DIR"
  git clone --depth 1 --branch "$MANIFOLD_VERSION" \
    https://github.com/elalish/manifold.git "$MANIFOLD_DIR"
fi

# --- Build Manifold (wasm static lib) ---
# MANIFOLD_PAR=OFF builds Manifold's serial algorithms. The web target is
# single-threaded on purpose: a multithreaded build needs SharedArrayBuffer,
# which requires a cross-origin-isolated context (COOP: same-origin + COEP:
# require-corp). Static hosting (GitHub Pages) cannot send those headers, so
# we stay single-threaded here.
MANIFOLD_BUILD_DIR="$MANIFOLD_DIR/build-wasm"
echo "Building manifold for wasm (single-threaded)..."
mkdir -p "$MANIFOLD_BUILD_DIR" && cd "$MANIFOLD_BUILD_DIR"
emcmake cmake "$MANIFOLD_DIR" \
  -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INTERPROCEDURAL_OPTIMIZATION=ON \
  -DBUILD_SHARED_LIBS=OFF \
  -DMANIFOLD_CBIND=ON \
  -DMANIFOLD_TEST=OFF \
  -DMANIFOLD_PYBIND=OFF \
  -DMANIFOLD_EXPORT=OFF \
  -DMANIFOLD_DOWNLOADS=ON \
  -DMANIFOLD_JSBIND=OFF \
  -DMANIFOLD_PAR=OFF
emmake ninja -j "$JOBS"

# --- Link facet_cxx as a wasm module ---
mkdir -p "$OUT_DIR"
echo "Linking facet_cxx.wasm..."

# Pick out manifold's public headers + the linked archives. emcmake produced
# .a files alongside its build dirs.
MANIFOLD_INC="$MANIFOLD_DIR/include"
SRC_DIR="$MANIFOLD_DIR/src"
CLIPPER_INC="$MANIFOLD_BUILD_DIR/_deps/clipper2-src/CPP/Clipper2Lib/include"

LIBS=(
  "$MANIFOLD_BUILD_DIR/src/libmanifold.a"
  "$MANIFOLD_BUILD_DIR/_deps/clipper2-build/libClipper2.a"
)

# Exported C functions: read from facet_cxx.h with a regex over the
# `<type> facet_<name>(...)` declarations, prepend the underscore Emscripten
# wants. (-sEXPORTED_FUNCTIONS expects names in linker form.)
EXPORTS_RAW=$(grep -oE ' facet_[a-z_]+\(' "$CXX_DIR/include/facet_cxx.h" | tr -d ' (' | sort -u | sed 's/^/_/' | paste -sd, -)
# Always retain malloc/free/realloc so JS can build/free buffers across the boundary.
EXPORTS="${EXPORTS_RAW},_malloc,_free,_realloc"
echo "Exporting $(echo "$EXPORTS_RAW" | tr ',' '\n' | wc -l | tr -d ' ') C functions"

emcc \
  "$CXX_DIR/src/bindings.cpp" \
  "$CXX_DIR/src/bindings_booleans.cpp" \
  "$CXX_DIR/src/bindings_transforms.cpp" \
  "$CXX_DIR/src/bindings_extrude.cpp" \
  "$CXX_DIR/src/bindings_ops.cpp" \
  "$CXX_DIR/src/bindings_queries.cpp" \
  "$CXX_DIR/src/bindings_extract.cpp" \
  "$CXX_DIR/src/bindings_callbacks.cpp" \
  "$CXX_DIR/src/polymesh.cpp" \
  "$CXX_DIR/src/text.cpp" \
  "${LIBS[@]}" \
  -DFACET_WASM \
  -I "$CXX_DIR/include" \
  -I "$MANIFOLD_INC" \
  -I "$SRC_DIR" \
  -I "$CLIPPER_INC" \
  -std=c++17 \
  -Oz \
  -flto \
  -fexceptions \
  -sUSE_FREETYPE=1 \
  -sALLOW_MEMORY_GROWTH=1 \
  -sMAXIMUM_MEMORY=4294967296 \
  -sINITIAL_MEMORY=32MB \
  -sMODULARIZE=1 \
  -sEXPORT_NAME=createFacetCxx \
  -sENVIRONMENT=web,node \
  -sFILESYSTEM=0 \
  -sIGNORE_MISSING_MAIN=1 \
  -sEXPORTED_FUNCTIONS="$EXPORTS" \
  -sEXPORTED_RUNTIME_METHODS=ccall,cwrap,UTF8ToString,stringToUTF8,getValue,setValue,HEAP8,HEAPU8,HEAP16,HEAPU16,HEAP32,HEAPU32,HEAPF32,HEAPF64 \
  -o "$OUT_DIR/facet_cxx.js"

# Post-link size pass — wasm-opt finds opportunities Emscripten misses.
# --enable-threads: the FreeType port emits atomic instructions (refcount
# guards); wasm-opt must accept them. The module's memory is NOT shared (no
# -sUSE_PTHREADS), so these atomics run single-threaded on regular memory and
# need no SharedArrayBuffer / COOP+COEP headers — the static-hosting story is
# unchanged.
echo "Running wasm-opt -Oz on facet_cxx.wasm..."
wasm-opt -Oz \
  --enable-bulk-memory \
  --enable-sign-ext \
  --enable-nontrapping-float-to-int \
  --enable-mutable-globals \
  --enable-threads \
  --strip-debug --strip-producers \
  -o "$OUT_DIR/facet_cxx.wasm" "$OUT_DIR/facet_cxx.wasm"

echo "Wrote $OUT_DIR/facet_cxx.{js,wasm}"
ls -lh "$OUT_DIR/facet_cxx.js" "$OUT_DIR/facet_cxx.wasm"
