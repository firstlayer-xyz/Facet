#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
THIRD_PARTY="$PROJECT_ROOT/app/third_party"
MANIFOLD_DIR="$THIRD_PARTY/manifold"
MANIFOLD_VERSION="v3.3.2"
ASSIMP_DIR="$THIRD_PARTY/assimp"
ASSIMP_VERSION="v5.4.3"
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
  cat > "$WRAPPER_DIR/rc.cmd" << 'ZIGEOF'
@echo off
rem Dummy RC compiler — create empty output for cross-compilation
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
# When building natively (host == target), skip zig wrappers and use system compiler
HOST_TARGET="$(detect_host_target)"
USE_TOOLCHAIN=true
if [[ "$HOST_TARGET" == "$TARGET" ]] && ! command -v zig >/dev/null 2>&1; then
  # No zig available but building for host — use system compiler
  USE_TOOLCHAIN=false
elif [[ "$HOST_TARGET" == "$TARGET" ]] && [[ "$TARGET" == windows-* ]]; then
  # Native Windows build — use system compiler (MSVC/MinGW), zig wrappers don't work
  USE_TOOLCHAIN=false
fi

TOOLCHAIN_FILE="$WRAPPER_DIR/toolchain.cmake"
TOOLCHAIN_FLAG=()
if $USE_TOOLCHAIN; then
  RC_LINE=""
  if [[ "$TARGET" == windows-* ]]; then
    EXT=".cmd"
    RC_LINE="set(CMAKE_RC_COMPILER \"${WRAPPER_DIR}/rc${EXT}\")"
  else
    EXT=""
  fi
  cat > "$TOOLCHAIN_FILE" << CMAKEEOF
set(CMAKE_SYSTEM_NAME ${CMAKE_SYSTEM_NAME})
set(CMAKE_SYSTEM_PROCESSOR ${CMAKE_SYSTEM_PROCESSOR})
set(CMAKE_C_COMPILER "${WRAPPER_DIR}/cc${EXT}")
set(CMAKE_CXX_COMPILER "${WRAPPER_DIR}/c++${EXT}")
set(CMAKE_AR "${WRAPPER_DIR}/ar${EXT}")
set(CMAKE_RANLIB "${WRAPPER_DIR}/ranlib${EXT}")
${RC_LINE}
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
CMAKEEOF
  TOOLCHAIN_FLAG=(-DCMAKE_TOOLCHAIN_FILE="$TOOLCHAIN_FILE")
fi

# Per-target build/install directories
ASSIMP_BUILD_DIR="$ASSIMP_DIR/build-${TARGET}"
ASSIMP_INSTALL_DIR="$ASSIMP_DIR/install-${TARGET}"
MANIFOLD_BUILD_DIR="$MANIFOLD_DIR/build-${TARGET}"

# --- Build assimp from source if needed ---
if [ ! -f "$ASSIMP_INSTALL_DIR/lib/libassimp.a" ]; then
  echo "Building assimp ${ASSIMP_VERSION} for ${TARGET}..."
  if [ ! -f "$ASSIMP_DIR/CMakeLists.txt" ]; then
    rm -rf "$ASSIMP_DIR"
    git clone --depth 1 --branch "$ASSIMP_VERSION" \
      https://github.com/assimp/assimp.git "$ASSIMP_DIR"
  fi
  mkdir -p "$ASSIMP_BUILD_DIR" && cd "$ASSIMP_BUILD_DIR"
  cmake "$ASSIMP_DIR" \
    "${CMAKE_GENERATOR_FLAG[@]}" \
    -DCMAKE_BUILD_TYPE=Release \
    "${TOOLCHAIN_FLAG[@]}" \
    -DBUILD_SHARED_LIBS=OFF \
    -DASSIMP_BUILD_TESTS=OFF \
    -DASSIMP_BUILD_SAMPLES=OFF \
    -DASSIMP_BUILD_ZLIB=ON \
    -DASSIMP_NO_EXPORT=OFF \
    -DASSIMP_WARNINGS_AS_ERRORS=OFF \
    -DCMAKE_C_FLAGS="-Dfdopen=fdopen" \
    -DCMAKE_CXX_FLAGS="-Wno-nontrivial-memcall -Wno-unknown-pragmas" \
    -DCMAKE_INSTALL_PREFIX="$ASSIMP_INSTALL_DIR"
  cmake --build . --config Release -j "$JOBS"
  cmake --install . --config Release
  echo "assimp build complete."
fi

# --- Build freetype from source if needed ---
FREETYPE_DIR="$THIRD_PARTY/freetype"
FREETYPE_VERSION="VER-2-13-3"
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
    -DCMAKE_BUILD_TYPE=Release \
    "${TOOLCHAIN_FLAG[@]}" \
    -DBUILD_SHARED_LIBS=OFF \
    -DFT_DISABLE_BZIP2=ON \
    -DFT_DISABLE_BROTLI=ON \
    -DFT_DISABLE_HARFBUZZ=ON \
    -DFT_DISABLE_PNG=ON \
    -DFT_DISABLE_ZLIB=ON \
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

# --- Apply patches ---
if [ -d "$THIRD_PARTY/patches" ]; then
  for p in "$THIRD_PARTY/patches/"*.patch; do
    [ -f "$p" ] && git -C "$MANIFOLD_DIR" apply --check "$p" 2>/dev/null && \
      git -C "$MANIFOLD_DIR" apply "$p" && echo "Applied patch: $(basename "$p")"
  done
fi

# --- Build manifold ---
echo "Building manifold for ${TARGET}..."
mkdir -p "$MANIFOLD_BUILD_DIR" && cd "$MANIFOLD_BUILD_DIR"
cmake "$MANIFOLD_DIR" \
  "${CMAKE_GENERATOR_FLAG[@]}" \
  -DCMAKE_BUILD_TYPE=Release \
  "${TOOLCHAIN_FLAG[@]}" \
  -DBUILD_SHARED_LIBS=OFF \
  -DMANIFOLD_CBIND=ON \
  -DMANIFOLD_TEST=OFF \
  -DMANIFOLD_PYBIND=OFF \
  -DMANIFOLD_EXPORT=ON \
  -DMANIFOLD_DOWNLOADS=ON \
  -DMANIFOLD_PAR=ON \
  -DMANIFOLD_USE_BUILTIN_TBB=ON \
  -DCMAKE_PREFIX_PATH="$ASSIMP_INSTALL_DIR"
cmake --build . --config Release -j "$JOBS"

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
FACET_CXX_DIR="$PROJECT_ROOT/app/pkg/manifold/cxx"
FACET_CXX_BUILD_DIR="$FACET_CXX_DIR/build-${TARGET}"
echo "Building facet_cxx for ${TARGET}..."
mkdir -p "$FACET_CXX_BUILD_DIR" && cd "$FACET_CXX_BUILD_DIR"
cmake "$FACET_CXX_DIR" \
  "${CMAKE_GENERATOR_FLAG[@]}" \
  -DCMAKE_BUILD_TYPE=Release \
  "${TOOLCHAIN_FLAG[@]}" \
  -DBUILD_SHARED_LIBS=OFF
cmake --build . --config Release -j "$JOBS"
echo "facet_cxx build complete."

echo "Manifold build complete for ${TARGET}."
echo "Libraries:"
find "$MANIFOLD_BUILD_DIR" "$FACET_CXX_BUILD_DIR" -name '*.a' -o -name '*.lib' | head -20
