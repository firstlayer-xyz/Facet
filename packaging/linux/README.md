# Linux file-manager thumbnails for `.fct`

These files give Facet model files a rendered 3D thumbnail in freedesktop file
managers (GNOME Files/Nautilus, Caja, Nemo, Thunar via tumbler):

- **`facet.xml`** — a `shared-mime-info` definition registering the
  `application/x-facet` MIME type for `*.fct`.
- **`facet.thumbnailer`** — a freedesktop *thumbnailer* that renders the preview
  by running `facetc %i -o %o -format png -size %s` (see `facetc -o preview.png`,
  the pure-Go headless renderer in `pkg/render`).

## Install

`facetc` must be on `PATH` (e.g. installed into `/usr/local/bin`). Then:

```sh
sudo ./install-thumbnailer.sh     # system-wide  (/usr/share)
./install-thumbnailer.sh          # current user (~/.local/share)
./install-thumbnailer.sh --uninstall
```

Restart your file manager (`nautilus -q`) and `.fct` files show a 3D thumbnail.

A distro package (`.deb`/`.rpm`) would instead ship `facet.xml` in
`/usr/share/mime/packages/` and `facet.thumbnailer` in
`/usr/share/thumbnailers/`, running `update-mime-database` in its post-install
hook.
