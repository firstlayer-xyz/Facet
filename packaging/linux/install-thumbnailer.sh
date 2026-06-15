#!/bin/sh
# Installs (or removes) the Facet (.fct) thumbnailer + MIME type for freedesktop
# file managers — GNOME Files/Nautilus, Caja, Nemo, Thunar (tumbler). After this,
# .fct files get a rendered 3D thumbnail in icon/list views. The thumbnailer
# shells out to `facetc`, which must be on PATH.
#
#   sudo ./install-thumbnailer.sh           # system-wide  (/usr/share)
#   ./install-thumbnailer.sh                # current user (~/.local/share)
#   ./install-thumbnailer.sh --uninstall    # remove
set -eu

here=$(cd "$(dirname "$0")" && pwd)

if [ "$(id -u)" = 0 ]; then
    datadir=/usr/share
else
    datadir="${XDG_DATA_HOME:-$HOME/.local/share}"
fi
mime="$datadir/mime/packages/facet.xml"
thumb="$datadir/thumbnailers/facet.thumbnailer"

if [ "${1:-}" = "--uninstall" ]; then
    rm -f "$mime" "$thumb"
    update-mime-database "$datadir/mime"
    echo "Removed Facet thumbnailer + MIME type from $datadir."
    exit 0
fi

if ! command -v facetc >/dev/null 2>&1; then
    echo "warning: 'facetc' is not on PATH — install it (e.g. into /usr/local/bin)" >&2
    echo "         so the thumbnailer can run it." >&2
fi

install -Dm644 "$here/facet.xml"         "$mime"
install -Dm644 "$here/facet.thumbnailer" "$thumb"
update-mime-database "$datadir/mime"

echo "Installed the Facet thumbnailer + MIME type into $datadir."
echo "Restart your file manager to pick it up (e.g. 'nautilus -q')."
