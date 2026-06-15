#!/bin/sh
# Cross-compiles the Windows Explorer thumbnail handler for .fct files to a DLL
# using zig (the same toolchain CI already uses for the Windows app build).
# Produces facet_thumbnail.dll; register it with regsvr32 / install.ps1.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
out="${1:-$here/facet_thumbnail.dll}"

zig c++ -target x86_64-windows-gnu -O2 -shared \
    -o "$out" \
    "$here/thumbnail_provider.cpp" "$here/thumbnail_provider.def" \
    -lole32 -loleaut32 -lshlwapi -lwindowscodecs -luuid -lshell32 -ladvapi32 -lgdi32

echo "built $out"
