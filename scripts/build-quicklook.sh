#!/bin/bash
# Builds the macOS Quick Look extensions (interactive 3D preview + thumbnail) and
# injects them into the already-built Facet.app. Each extension links a c-archive
# of the Facet evaluator + geometry kernel (cmd/facetrender) so it can render a
# .fct in-process — a Quick Look extension is sandboxed and cannot spawn facetc.
#
# macOS-only (a no-op elsewhere). Run after `wails build` and `make manifold`.
# Signs ad-hoc by default; set CODESIGN_IDENTITY (and the release flow notarizes).
set -euo pipefail

if [ "$(uname -s)" != "Darwin" ]; then
  echo "build-quicklook: not macOS — skipping"; exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO="$ROOT/.go-toolchain/bin/go"
APP="${FACET_APP:-$ROOT/desktop/build/bin/Facet.app}"
QL="$ROOT/desktop/quicklook"
IDENTITY="${CODESIGN_IDENTITY:--}"      # default: ad-hoc
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

[ -d "$APP" ] || { echo "build-quicklook: $APP not found — build the app first" >&2; exit 1; }

case "$(uname -m)" in
  arm64)  ARCH=arm64;  SWIFT_TGT=arm64-apple-macosx12.0  ;;
  x86_64) ARCH=amd64;  SWIFT_TGT=x86_64-apple-macosx12.0 ;;
  *) echo "build-quicklook: unsupported arch $(uname -m)" >&2; exit 1 ;;
esac
TARGET="darwin-$ARCH"
SDK="$(xcrun -sdk macosx --show-sdk-path)"
# A secure timestamp is required for notarization, but needs the network and a
# real identity; skip it for ad-hoc local builds.
[ "$IDENTITY" = "-" ] && TS=--timestamp=none || TS=--timestamp

echo "build-quicklook: building c-archive (evaluator + kernel)…"
CGO_ENABLED=1 "$GO" build -C "$ROOT" -buildmode=c-archive -o "$WORK/libfacetrender.a" ./cmd/facetrender

LIBS=(
  -L"$ROOT/pkg/manifold/cxx/build-$TARGET"
  -L"$ROOT/third_party/manifold/build-$TARGET/src"
  -L"$ROOT/third_party/manifold/build-$TARGET/_deps/clipper2-build"
  -L"$ROOT/third_party/manifold/build-$TARGET/tbb"
  -L"$ROOT/third_party/freetype/install-$TARGET/lib"
  -L/opt/homebrew/lib
  -lfacet_cxx -lmanifold -lClipper2 -ltbb -lfreetype -lm -lc++
  -framework CoreFoundation -framework Security -lresolv
)

build_ext() { # appex  exe  info.plist  principal.swift
  local appex="$1" exe="$2" plist="$3" main="$4"
  echo "build-quicklook: ${appex}…"
  swiftc -target "$SWIFT_TGT" -sdk "$SDK" -module-name "$exe" \
    -import-objc-header "$WORK/libfacetrender.h" \
    -framework Cocoa -framework Quartz -framework SceneKit \
    -framework QuickLookThumbnailing -framework Metal \
    -Xlinker -e -Xlinker _NSExtensionMain -emit-executable -o "$WORK/$exe" \
    "$QL/FacetMesh.swift" "$QL/$main" "$WORK/libfacetrender.a" "${LIBS[@]}"
  local b="$WORK/$appex.appex"
  mkdir -p "$b/Contents/MacOS"
  cp "$plist" "$b/Contents/Info.plist"
  cp "$WORK/$exe" "$b/Contents/MacOS/$exe"
  codesign --force --sign "$IDENTITY" --entitlements "$QL/extension.entitlements" --options runtime $TS "$b"
  mkdir -p "$APP/Contents/PlugIns"
  rm -rf "$APP/Contents/PlugIns/$appex.appex"
  cp -R "$b" "$APP/Contents/PlugIns/"
}

build_ext FacetQuickLook FacetQuickLook "$QL/Preview-Info.plist"   PreviewViewController.swift
build_ext FacetThumbnail FacetThumbnail "$QL/Thumbnail-Info.plist" ThumbnailProvider.swift

echo "build-quicklook: re-signing app bundle…"
codesign --force --sign "$IDENTITY" --options runtime $TS "$APP"
echo "build-quicklook: done — preview + thumbnail injected into $APP"
