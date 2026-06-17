#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
THIRD_PARTY="$PROJECT_ROOT/third_party"
MANIFOLD_DIR="$THIRD_PARTY/manifold"
. "$SCRIPT_DIR/_third-party-versions.sh"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 4)"

# --- Resolve target ---
detect_host_target() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$os" in
    mingw*|msys*|cygwin*) os="windows" ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
  esac
  echo "${os}-${arch}"
}

TARGET="${1:-$(detect_host_target)}"

# Map target to zig triple and cmake vars
case "$TARGET" in
  darwin-arm64)
    ZIG_TRIPLE="aarch64-macos"
    CMAKE_SYSTEM_NAME="Darwin"
    CMAKE_SYSTEM_PROCESSOR="aarch64"
    ;;
  darwin-amd64)
    ZIG_TRIPLE="x86_64-macos"
    CMAKE_SYSTEM_NAME="Darwin"
    CMAKE_SYSTEM_PROCESSOR="x86_64"
    ;;
  linux-amd64)
    ZIG_TRIPLE="x86_64-linux-gnu"
    CMAKE_SYSTEM_NAME="Linux"
    CMAKE_SYSTEM_PROCESSOR="x86_64"
    ;;
  linux-arm64)
    ZIG_TRIPLE="aarch64-linux-gnu"
    CMAKE_SYSTEM_NAME="Linux"
    CMAKE_SYSTEM_PROCESSOR="aarch64"
    ;;
  windows-amd64)
    ZIG_TRIPLE="x86_64-windows-gnu"
    CMAKE_SYSTEM_NAME="Windows"
    CMAKE_SYSTEM_PROCESSOR="x86_64"
    ;;
  windows-arm64)
    ZIG_TRIPLE="aarch64-windows-gnu"
    CMAKE_SYSTEM_NAME="Windows"
    CMAKE_SYSTEM_PROCESSOR="aarch64"
    ;;
  *)
    echo "Unknown target: $TARGET" >&2
    echo "Supported: darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64, windows-arm64" >&2
    exit 1
    ;;
esac

# ISA baseline for x86_64 targets. windows-latest and ubuntu-latest runners
# are CPU-heterogeneous: a lib compiled with the build runner's native ISA
# may emit instructions (AVX2, BMI, etc.) that crash with
# STATUS_ILLEGAL_INSTRUCTION on a different runner from the same pool.
# `nehalem` is the oldest CPU that supports SSE4.2 (~2008, equivalent to
# x86-64-v2 from gcc's naming): negligible perf loss vs native code and
# guaranteed portable across the github-hosted runner fleet. We pick a
# concrete CPU name rather than `x86-64-v2` because zig 0.14.1's bundled
# clang accepts underscored microarchitecture levels (x86_64_v2) but not
# the hyphenated form, and concrete CPU names sidestep that quirk.
case "$TARGET" in
  linux-amd64|windows-amd64|darwin-amd64)
    ISA_BASELINE_FLAGS="-march=nehalem"
    ;;
  *)
    ISA_BASELINE_FLAGS=""
    ;;
esac

echo "Building for target: $TARGET (zig triple: $ZIG_TRIPLE)"

# Force Ninja generator so the zig toolchain file is respected (the default
# Visual Studio generator on Windows ignores CMAKE_C_COMPILER/CMAKE_CXX_COMPILER).
CMAKE_GENERATOR_FLAG=()
if command -v ninja >/dev/null 2>&1; then
  CMAKE_GENERATOR_FLAG=(-G Ninja)
elif command -v make >/dev/null 2>&1; then
  CMAKE_GENERATOR_FLAG=(-G "Unix Makefiles")
fi

# --- zig wrapper scripts (cmake needs a single executable, not "zig cc") ---
WRAPPER_DIR="$THIRD_PARTY/.zig-wrappers-${TARGET}"
mkdir -p "$WRAPPER_DIR"

if [[ "$TARGET" == windows-* ]]; then
  # Windows: create .cmd wrappers that CMake can execute
  cat > "$WRAPPER_DIR/cc.cmd" << ZIGEOF
@echo off
zig cc --target=${ZIG_TRIPLE} %*
ZIGEOF
  cat > "$WRAPPER_DIR/c++.cmd" << ZIGEOF
@echo off
zig c++ --target=${ZIG_TRIPLE} %*
ZIGEOF
  cat > "$WRAPPER_DIR/ar.cmd" << 'ZIGEOF'
@echo off
zig ar %*
ZIGEOF
  cat > "$WRAPPER_DIR/ranlib.cmd" << 'ZIGEOF'
@echo off
zig ranlib %*
ZIGEOF
else
  cat > "$WRAPPER_DIR/cc" << ZIGEOF
#!/bin/sh
exec zig cc --target=${ZIG_TRIPLE} "\$@"
ZIGEOF
  cat > "$WRAPPER_DIR/c++" << ZIGEOF
#!/bin/sh
exec zig c++ --target=${ZIG_TRIPLE} "\$@"
ZIGEOF
  cat > "$WRAPPER_DIR/ar" << 'ZIGEOF'
#!/bin/sh
exec zig ar "$@"
ZIGEOF
  cat > "$WRAPPER_DIR/ranlib" << 'ZIGEOF'
#!/bin/sh
exec zig ranlib "$@"
ZIGEOF
  chmod +x "$WRAPPER_DIR/cc" "$WRAPPER_DIR/c++" "$WRAPPER_DIR/ar" "$WRAPPER_DIR/ranlib"
fi

# Dummy RC compiler for Windows cross-compilation (FreeType tries enable_language(RC))
if [[ "$TARGET" == windows-* ]]; then
  # Walks args looking for /fo, -fo, or -o; creates an empty file at the
  # path that follows. Mirrors the Linux dummy below — without the file-
  # creation step the linker fails with "No such file or directory" on
  # the .res output FreeType expects.
  cat > "$WRAPPER_DIR/rc.cmd" << 'ZIGEOF'
@echo off
setlocal EnableDelayedExpansion
set "PREV="
:loop
if "%~1"=="" goto end
if /I "!PREV!"=="/fo" type nul > "%~1"
if /I "!PREV!"=="-fo" type nul > "%~1"
if /I "!PREV!"=="-o" type nul > "%~1"
set "PREV=%~1"
shift
goto loop
:end
exit /b 0
ZIGEOF
else
  cat > "$WRAPPER_DIR/rc" << 'ZIGEOF'
#!/bin/sh
# Dummy RC compiler for cross-compilation.
prev=""
for arg in "$@"; do
  case "$prev" in
    /fo|-fo|-o) touch "$arg" 2>/dev/null ;;
  esac
  prev="$arg"
done
exit 0
ZIGEOF
  chmod +x "$WRAPPER_DIR/rc"
fi

# --- cmake toolchain file ---
# Picking the compiler:
#
# Linux/macOS native (host == target): use the system compiler. Routing
# native builds through `zig ar` breaks on recent zig versions (0.16 on
# macOS fails with "unable to open ...: No such file or directory"), and
# system-built artifacts match what dev machines produce anyway.
#
# macOS cross (e.g. darwin-amd64 host → darwin-arm64): also use the system
# compiler. Apple's clang handles cross-arch via -arch <name> natively
# from any macOS host, and using zig instead introduces a libc++ ABI
# mismatch — zig bundles a newer libc++ (LLVM 19+) than the one Apple
# ships in Ventura's SDK. Symbols like std::__hash_memory exist in zig's
# libc++ but not Apple's, so any final link against system libc++ fails.
#
# Linux cross / Windows native+cross: zig toolchain wrappers. Required for
# Linux→other-arch and for Windows because cgo on Windows needs a clang/
# gcc-style compiler and CMake on Windows can't parse `CC="zig cc"` as a
# compiler+arg combo.
HOST_TARGET="$(detect_host_target)"
USE_TOOLCHAIN=true
if [[ "$HOST_TARGET" == "$TARGET" ]] && [[ "$TARGET" != windows-* ]]; then
  USE_TOOLCHAIN=false
elif [[ "$TARGET" == darwin-* ]]; then
  # macOS cross-compile via Apple's clang (-arch flag) — see comment above.
  USE_TOOLCHAIN=false
fi

# darwin-* targets: tell CMake the target arch so Apple's clang emits the
# right slice (CMAKE_OSX_ARCHITECTURES) AND so any CMakeLists logic that
# branches on CMAKE_SYSTEM_PROCESSOR sees the target processor instead of
# the host's (CMAKE_SYSTEM_PROCESSOR). Used whether building native or
# cross — harmless when native, required when cross.
DARWIN_OSX_ARCH=()
case "$TARGET" in
  darwin-arm64)
    DARWIN_OSX_ARCH=(-DCMAKE_OSX_ARCHITECTURES=arm64 -DCMAKE_SYSTEM_PROCESSOR=arm64)
    ;;
  darwin-amd64)
    DARWIN_OSX_ARCH=(-DCMAKE_OSX_ARCHITECTURES=x86_64 -DCMAKE_SYSTEM_PROCESSOR=x86_64)
    ;;
esac

TOOLCHAIN_FILE="$WRAPPER_DIR/toolchain.cmake"
TOOLCHAIN_FLAG=()
if $USE_TOOLCHAIN; then
  RC_LINE=""
  if [[ "$TARGET" == windows-* ]]; then
    # Windows: use zig as the compiler executable directly (no .cmd
    # wrapper). The .cmd wrappers caused "Access is denied" when ninja
    # invoked them via CreateProcess — Windows' batch-file dispatch
    # doesn't always cooperate with non-shell tools. CMAKE_<LANG>_
    # COMPILER_ARG1 lets us pass "cc"/"c++" as the first arg to zig so
    # `zig cc <other-flags>` ends up running. ar/ranlib/rc are still
    # invoked via .cmd wrappers — those don't need to be exec'd by
    # ninja in the hot path.
    cat > "$TOOLCHAIN_FILE" << CMAKEEOF
set(CMAKE_SYSTEM_NAME ${CMAKE_SYSTEM_NAME})
set(CMAKE_SYSTEM_PROCESSOR ${CMAKE_SYSTEM_PROCESSOR})
set(CMAKE_C_COMPILER "zig")
set(CMAKE_C_COMPILER_ARG1 "cc")
set(CMAKE_CXX_COMPILER "zig")
set(CMAKE_CXX_COMPILER_ARG1 "c++")
set(CMAKE_C_FLAGS_INIT "--target=${ZIG_TRIPLE}")
set(CMAKE_CXX_FLAGS_INIT "--target=${ZIG_TRIPLE}")
set(CMAKE_AR "${WRAPPER_DIR}/ar.cmd")
set(CMAKE_RANLIB "${WRAPPER_DIR}/ranlib.cmd")
set(CMAKE_RC_COMPILER "${WRAPPER_DIR}/rc.cmd")
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
CMAKEEOF
  else
    # Linux cross-compile (e.g. linux-amd64 host → linux-arm64 target):
    # use the shell-script wrappers in $WRAPPER_DIR. These work fine on
    # Unix because the kernel honors shebang lines.
    cat > "$TOOLCHAIN_FILE" << CMAKEEOF
set(CMAKE_SYSTEM_NAME ${CMAKE_SYSTEM_NAME})
set(CMAKE_SYSTEM_PROCESSOR ${CMAKE_SYSTEM_PROCESSOR})
set(CMAKE_C_COMPILER "${WRAPPER_DIR}/cc")
set(CMAKE_CXX_COMPILER "${WRAPPER_DIR}/c++")
set(CMAKE_AR "${WRAPPER_DIR}/ar")
set(CMAKE_RANLIB "${WRAPPER_DIR}/ranlib")
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
CMAKEEOF
  fi
  TOOLCHAIN_FLAG=(-DCMAKE_TOOLCHAIN_FILE="$TOOLCHAIN_FILE")
fi

# Per-target build/install directories
MANIFOLD_BUILD_DIR="$MANIFOLD_DIR/build-${TARGET}"

# --- Build freetype from source if needed ---
FREETYPE_DIR="$THIRD_PARTY/freetype"
FREETYPE_BUILD_DIR="$FREETYPE_DIR/build-${TARGET}"
FREETYPE_INSTALL_DIR="$FREETYPE_DIR/install-${TARGET}"

if [ ! -f "$FREETYPE_INSTALL_DIR/lib/libfreetype.a" ]; then
  echo "Building freetype for ${TARGET}..."
  if [ ! -f "$FREETYPE_DIR/CMakeLists.txt" ]; then
    rm -rf "$FREETYPE_DIR"
    git clone --depth 1 --branch "$FREETYPE_VERSION" \
      https://github.com/freetype/freetype.git "$FREETYPE_DIR"
  fi
  mkdir -p "$FREETYPE_BUILD_DIR" && cd "$FREETYPE_BUILD_DIR"
  cmake "$FREETYPE_DIR" \
    "${CMAKE_GENERATOR_FLAG[@]}" \
    "${DARWIN_OSX_ARCH[@]}" \
    -DCMAKE_BUILD_TYPE=Release \
    ${TOOLCHAIN_FLAG[@]+"${TOOLCHAIN_FLAG[@]}"} \
    -DBUILD_SHARED_LIBS=OFF \
    -DFT_DISABLE_BZIP2=ON \
    -DFT_DISABLE_BROTLI=ON \
    -DFT_DISABLE_HARFBUZZ=ON \
    -DFT_DISABLE_PNG=ON \
    -DFT_DISABLE_ZLIB=ON \
    -DCMAKE_C_FLAGS="$ISA_BASELINE_FLAGS" \
    -DCMAKE_CXX_FLAGS="$ISA_BASELINE_FLAGS" \
    -DCMAKE_INSTALL_PREFIX="$FREETYPE_INSTALL_DIR"
  cmake --build . --config Release -j "$JOBS"
  cmake --install . --config Release
  echo "freetype build complete."
fi

# --- Clone manifold if needed ---
if [ ! -f "$MANIFOLD_DIR/CMakeLists.txt" ]; then
  rm -rf "$MANIFOLD_DIR"
  git clone --depth 1 --branch "$MANIFOLD_VERSION" \
    https://github.com/elalish/manifold.git "$MANIFOLD_DIR"
fi

# --- Build manifold ---
# Skip if libmanifold.a already exists (e.g. CI cache restore). Matches the
# freetype early-exit pattern above. Without this, re-entering the
# script after a cache restore (e.g. `make test` running `make manifold`
# as a dep) re-invokes cmake against a stale CMakeCache.txt whose
# absolute CMAKE_CXX_COMPILER path points at a previous run's zig install
# (mlugg/setup-zig puts zig under $RUNNER_TEMP/<run-id>/, so the path
# changes per CI run).
if [ ! -f "$MANIFOLD_BUILD_DIR/src/libmanifold.a" ]; then
  echo "Building manifold for ${TARGET}..."
  mkdir -p "$MANIFOLD_BUILD_DIR" && cd "$MANIFOLD_BUILD_DIR"
  cmake "$MANIFOLD_DIR" \
    "${CMAKE_GENERATOR_FLAG[@]}" \
    "${DARWIN_OSX_ARCH[@]}" \
    -DCMAKE_BUILD_TYPE=Release \
    ${TOOLCHAIN_FLAG[@]+"${TOOLCHAIN_FLAG[@]}"} \
    -DBUILD_SHARED_LIBS=OFF \
    -DMANIFOLD_CBIND=ON \
    -DMANIFOLD_TEST=OFF \
    -DMANIFOLD_PYBIND=OFF \
    -DMANIFOLD_EXPORT=OFF \
    -DMANIFOLD_DOWNLOADS=ON \
    -DMANIFOLD_PAR=ON \
    -DMANIFOLD_USE_BUILTIN_TBB=ON \
    -DCMAKE_C_FLAGS="$ISA_BASELINE_FLAGS" \
    -DCMAKE_CXX_FLAGS="$ISA_BASELINE_FLAGS"
  cmake --build . --config Release -j "$JOBS"
else
  echo "manifold already built for ${TARGET} (libmanifold.a present) — skipping cmake+build."
fi

# --- Copy TBB libs to a known location ---
TBB_INSTALL_DIR="$MANIFOLD_BUILD_DIR/tbb"
mkdir -p "$TBB_INSTALL_DIR"
TBB_SRC_DIR=$(find "$MANIFOLD_BUILD_DIR" -maxdepth 1 -type d -name '*_release' -o -name '*_debug' | head -1)
if [ -n "$TBB_SRC_DIR" ]; then
  cp "$TBB_SRC_DIR"/libtbb*.a "$TBB_INSTALL_DIR/"
  # Normalize versioned TBB name (e.g. libtbb12.a on Windows) to libtbb.a
  if [ ! -f "$TBB_INSTALL_DIR/libtbb.a" ] && ls "$TBB_INSTALL_DIR"/libtbb[0-9]*.a >/dev/null 2>&1; then
    cp "$(ls "$TBB_INSTALL_DIR"/libtbb[0-9]*.a | head -1)" "$TBB_INSTALL_DIR/libtbb.a"
  fi
  echo "TBB libraries copied to $TBB_INSTALL_DIR"
fi

# --- Build facet_cxx ---
FACET_CXX_DIR="$PROJECT_ROOT/pkg/manifold/cxx"
FACET_CXX_BUILD_DIR="$FACET_CXX_DIR/build-${TARGET}"
echo "Building facet_cxx for ${TARGET}..."
# Always start from a clean build dir. cmake caches CMAKE_SYSTEM_PROCESSOR
# in its initial config and won't update it on a re-run, even when -D is
# passed — so a stale build dir from a different host or prior cross-
# compile attempt resolves _ARCH to the wrong value (the cached one) and
# downstream include paths point at install-<wrong-arch>.
rm -rf "$FACET_CXX_BUILD_DIR"
mkdir -p "$FACET_CXX_BUILD_DIR" && cd "$FACET_CXX_BUILD_DIR"
cmake "$FACET_CXX_DIR" \
  "${CMAKE_GENERATOR_FLAG[@]}" \
  "${DARWIN_OSX_ARCH[@]}" \
  -DCMAKE_BUILD_TYPE=Release \
  ${TOOLCHAIN_FLAG[@]+"${TOOLCHAIN_FLAG[@]}"} \
  -DBUILD_SHARED_LIBS=OFF \
  -DCMAKE_C_FLAGS="$ISA_BASELINE_FLAGS" \
  -DCMAKE_CXX_FLAGS="$ISA_BASELINE_FLAGS"
cmake --build . --config Release -j "$JOBS"
echo "facet_cxx build complete."

echo "Manifold build complete for ${TARGET}."
echo "Libraries:"
find "$MANIFOLD_BUILD_DIR" "$FACET_CXX_BUILD_DIR" -name '*.a' -o -name '*.lib' | head -20
