# Windows Explorer thumbnails for `.fct/.3mf/.obj/.stl`

A COM **`IThumbnailProvider`** shell handler that gives Facet model files and
common mesh formats a rendered 3D thumbnail in Explorer's icon/tile/content views.

- **`thumbnail_provider.cpp`** — the handler. On `GetThumbnail` it shells out to
  `facetc <file> -o <tmp> -format png -size <px>` (see `facetc -o preview.png`,
  the pure-Go renderer in `pkg/render`), then decodes the PNG with WIC into the
  `HBITMAP` Explorer wants. A Windows thumbnail handler isn't sandboxed, so
  spawning `facetc` is fine (unlike the macOS Quick Look extension, which embeds
  the kernel).
- **`thumbnail_provider.def`** — exports the COM `Dll*` entry points.
- **`build.sh`** — cross-compiles the DLL with `zig` (the same toolchain CI uses
  for the Windows app): `facet_thumbnail.dll`.
- **`install.ps1`** — copies the DLL + `facetc.exe` to `%ProgramFiles%\Facet`
  and registers it with `regsvr32`; `-Uninstall` to remove.

## Build + install

```sh
sh build.sh                       # → facet_thumbnail.dll (needs zig)
```
```powershell
# elevated PowerShell, with facetc.exe alongside the DLL:
.\install.ps1
```
`.fct`, `.3mf`, `.obj`, and `.stl` files then show a 3D thumbnail (restart
Explorer to refresh).

> **Note:** Windows ships its own `.3mf` (and sometimes `.stl`) thumbnail
> handlers; registering ours under those extensions overrides them per-machine.
> This is intended.

## Status

The handler **compiles and links** cleanly (zig → a valid PE32+ DLL). It has
**not** yet been runtime-verified on Windows — confirming Explorer loads it and
renders the thumbnail needs a Windows session. The render itself is the same
`facetc -o *.png` path used (and verified) on macOS/Linux.
