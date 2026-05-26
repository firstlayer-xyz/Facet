#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/szatmary/go.git"
BRANCH="feature/cgo-memory-pressure"
TOOLCHAIN_DIR="$(cd "$(dirname "$0")/.." && pwd)/.go-toolchain"

# Unset GOROOT so the bootstrap Go detection below works correctly.
# The Makefile exports GOROOT=$(TOOLCHAIN_DIR) which doesn't exist yet
# during first-time setup, causing `go env GOROOT` to return a bad path.
unset GOROOT

# If the toolchain already exists and is on the right branch, skip.
GO_BIN="$TOOLCHAIN_DIR/bin/go"
[ -f "$TOOLCHAIN_DIR/bin/go.exe" ] && GO_BIN="$TOOLCHAIN_DIR/bin/go.exe"

if [ -x "$GO_BIN" ]; then
    current=$(git -C "$TOOLCHAIN_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || true)
    if [ "$current" = "$BRANCH" ]; then
        echo "Custom Go toolchain already built at $TOOLCHAIN_DIR"
        "$GO_BIN" version
        exit 0
    fi
    echo "Toolchain exists but on branch '$current', expected '$BRANCH'. Rebuilding..."
    rm -rf "$TOOLCHAIN_DIR"
fi

# Remove incomplete/corrupt toolchain directory (exists but no working binary)
if [ -d "$TOOLCHAIN_DIR" ] && [ ! -x "$GO_BIN" ]; then
    echo "Incomplete toolchain directory found. Removing..."
    rm -rf "$TOOLCHAIN_DIR"
fi

# Ensure GOROOT_BOOTSTRAP is set to a working Go installation.
if [ -z "${GOROOT_BOOTSTRAP:-}" ]; then
    if command -v go >/dev/null 2>&1; then
        GOROOT_BOOTSTRAP="$(go env GOROOT)"
    fi
fi
if [ -z "${GOROOT_BOOTSTRAP:-}" ] || { [ ! -x "$GOROOT_BOOTSTRAP/bin/go" ] && [ ! -x "$GOROOT_BOOTSTRAP/bin/go.exe" ]; }; then
    echo "No bootstrap Go found. Installing one..."
    BOOTSTRAP_DIR="$(cd "$(dirname "$0")/.." && pwd)/.go-bootstrap"
    if [ ! -x "$BOOTSTRAP_DIR/bin/go" ] && [ ! -x "$BOOTSTRAP_DIR/bin/go.exe" ]; then
        BOOTSTRAP_VERSION="1.24.6"
        UNAME_S="$(uname -s | tr '[:upper:]' '[:lower:]')"
        ARCH="$(uname -m)"
        case "$ARCH" in
            x86_64|amd64) ARCH="amd64" ;;
            arm64|aarch64) ARCH="arm64" ;;
        esac
        # Map uname -s output to Go's release suffix. Git Bash on Windows
        # reports something like "mingw64_nt-10.0-26200" which Go doesn't
        # publish releases for — flatten all Windows shells to "windows".
        # Go uses .zip on Windows and .tar.gz everywhere else.
        case "$UNAME_S" in
            mingw*|msys*|cygwin*|windows*)
                OS="windows"
                EXT="zip"
                ;;
            darwin*)
                OS="darwin"
                EXT="tar.gz"
                ;;
            linux*)
                OS="linux"
                EXT="tar.gz"
                ;;
            *)
                OS="$UNAME_S"
                EXT="tar.gz"
                ;;
        esac
        ARCHIVE="go${BOOTSTRAP_VERSION}.${OS}-${ARCH}.${EXT}"
        echo "Downloading Go $BOOTSTRAP_VERSION for $OS/$ARCH..."
        curl -fsSL "https://go.dev/dl/$ARCHIVE" -o "/tmp/$ARCHIVE"
        mkdir -p "$BOOTSTRAP_DIR"
        if [ "$EXT" = "zip" ]; then
            # Windows: unzip into a temp dir, then move the inner go/* up.
            UNZIP_TMP="/tmp/go-bootstrap-unzip-$$"
            rm -rf "$UNZIP_TMP"
            mkdir -p "$UNZIP_TMP"
            unzip -q "/tmp/$ARCHIVE" -d "$UNZIP_TMP"
            cp -R "$UNZIP_TMP/go/." "$BOOTSTRAP_DIR/"
            rm -rf "$UNZIP_TMP"
        else
            tar -xzf "/tmp/$ARCHIVE" -C "$BOOTSTRAP_DIR" --strip-components=1
        fi
        rm -f "/tmp/$ARCHIVE"
    fi
    export GOROOT_BOOTSTRAP="$BOOTSTRAP_DIR"
fi
# On Git Bash / MSYS2 / Cygwin the bootstrap path is Unix-style
# (/c/Users/...) but make.bat joins it with backslashes when looking up
# bin\go.exe, producing mixed-slash paths Windows can't resolve. Convert
# to a native Windows path so make.bat's concatenation stays consistent.
if command -v cygpath >/dev/null 2>&1; then
    export GOROOT_BOOTSTRAP="$(cygpath -w "$GOROOT_BOOTSTRAP")"
fi
export GOTOOLCHAIN=local
echo "Using bootstrap Go: $GOROOT_BOOTSTRAP"

echo "Cloning $REPO (branch $BRANCH) into $TOOLCHAIN_DIR ..."
git clone --depth 1 --branch "$BRANCH" "$REPO" "$TOOLCHAIN_DIR"

echo "Building Go toolchain ..."
cd "$TOOLCHAIN_DIR/src"
if [ -f make.bat ] && command -v cmd.exe >/dev/null 2>&1; then
    WINPATH=$(cygpath -w "$TOOLCHAIN_DIR/src/make.bat")
    MSYS_NO_PATHCONV=1 cmd.exe /C "$WINPATH"
else
    ./make.bash
fi

echo ""
echo "Custom Go toolchain ready:"
GO_BIN="$TOOLCHAIN_DIR/bin/go"
[ -f "$TOOLCHAIN_DIR/bin/go.exe" ] && GO_BIN="$TOOLCHAIN_DIR/bin/go.exe"
"$GO_BIN" version
