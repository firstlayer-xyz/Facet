# Linux file-manager thumbnails for `.fct`, `.stl`, `.obj`, `.3mf`

These files give Facet model files and common mesh formats a rendered 3D thumbnail
in freedesktop file managers (GNOME Files/Nautilus, Caja, Nemo, Thunar via tumbler):

- **`facet.xml`** — `shared-mime-info` definitions registering MIME types for
  `*.fct` (`application/x-facet`), `*.stl` (`model/stl`), `*.obj` (`model/obj`),
  and `*.3mf` (`application/vnd.ms-package.3dmanufacturing-3dmodel+xml`).
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

Restart your file manager (`nautilus -q`) and `.fct`/`.stl`/`.obj`/`.3mf` files
show a 3D thumbnail.

A distro package (`.deb`/`.rpm`) would instead ship `facet.xml` in
`/usr/share/mime/packages/` and `facet.thumbnailer` in
`/usr/share/thumbnailers/`, running `update-mime-database` in its post-install
hook.
