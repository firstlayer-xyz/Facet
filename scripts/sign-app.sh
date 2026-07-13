#!/bin/bash
# Codesigns an assembled Facet.app bundle inside-out and is the single place the
# app is signed: build-quicklook.sh (ad-hoc, local/CI) and release.yml (the real
# Developer ID) both call it. It never holds keys — the identity is passed in via
# CODESIGN_IDENTITY ("-" = ad-hoc, the default); the private key lives only where
# the caller runs (locally: none; release: imported from a secret into a keychain).
#
# Inside-out, not a single `codesign --deep`: --deep would force the app's camera
# entitlement onto the sandboxed Quick Look / Thumbnail extensions, replacing
# their own. So: nested code first, then each extension with its entitlements,
# then the app last (with the camera entitlement) so it re-seals the extensions.
#
# Usage: scripts/sign-app.sh [path/to/Facet.app]
#   CODESIGN_IDENTITY  signing identity (default "-", ad-hoc)
#   CODESIGN_KEYCHAIN  keychain to search for the identity (optional)
set -euo pipefail

if [ "$(uname -s)" != "Darwin" ]; then
  echo "sign-app: not macOS — skipping"; exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="${1:-${FACET_APP:-$ROOT/desktop/build/bin/Facet.app}}"
IDENTITY="${CODESIGN_IDENTITY:--}"
QL_ENT="$ROOT/desktop/quicklook/extension.entitlements"
APP_ENT="$ROOT/desktop/build/darwin/entitlements.plist"

[ -d "$APP" ] || { echo "sign-app: $APP not found — build the app first" >&2; exit 1; }

# A secure timestamp needs the network and a real identity; skip it for ad-hoc.
[ "$IDENTITY" = "-" ] && TS=--timestamp=none || TS=--timestamp

# Sign with the configured identity; route through the given keychain if set.
# (Two branches rather than an array so this stays safe under macOS bash 3.2.)
_sign() {
  if [ -n "${CODESIGN_KEYCHAIN:-}" ]; then
    codesign --force --sign "$IDENTITY" --options runtime "$TS" --keychain "$CODESIGN_KEYCHAIN" "$@"
  else
    codesign --force --sign "$IDENTITY" --options runtime "$TS" "$@"
  fi
}

echo "sign-app: signing $APP (identity: $IDENTITY)…"
_sign --deep "$APP"                                   # nested frameworks/dylibs first
for ext in "$APP"/Contents/PlugIns/*.appex; do        # extensions with their own entitlements
  [ -e "$ext" ] || continue
  _sign --entitlements "$QL_ENT" "$ext"
done
_sign --entitlements "$APP_ENT" "$APP"                # app last: camera entitlement, re-seals extensions
echo "sign-app: done"
