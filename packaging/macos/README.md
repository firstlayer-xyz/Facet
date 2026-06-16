# macOS Quick Look thumbnails & previews

macOS support for `.fct/.stl/.obj/.3mf` thumbnails and previews is **not packaged
here** — unlike Windows and Linux, it ships *inside* `Facet.app`. This directory
exists only as a breadcrumb; there is nothing to build or install separately.

## How it works

The thumbnail and interactive-preview extensions are macOS **app extensions**
(`.appex`) bundled in `Facet.app/Contents/PlugIns/`. The OS discovers them
automatically the first time the app runs (LaunchServices registers an app's
extensions) — there is no `regsvr32`/`install.sh` step.

- **Source:** [`desktop/quicklook/`](../../desktop/quicklook/)
  - `ThumbnailProvider.swift` — `QLThumbnailProvider` (icon/gallery thumbnails)
  - `PreviewViewController.swift` — `QLPreviewingController` (drag-to-orbit preview).
    A `.fct` whose `Main` returns an `Animation` *plays back*: a background producer
    pulls frames over wall-clock time (`FacetOpenAnimation`/`FacetAnimationFrame`/
    `FacetCloseAnimation`, the c-archive's session API) and swaps the model's
    geometry on the main thread. Everything else renders as a static turntable.
  - `FacetMesh.swift` — shared loader: calls the in-process evaluator/kernel
    (`FacetRenderFile`, the c-archive from `cmd/facetrender`) and builds the
    SceneKit scene. A Quick Look extension is sandboxed and cannot spawn `facetc`,
    so it links the renderer directly. `geometry()` and `framedScene()` are factored
    out so the static path and the animation per-frame swap build identical geometry.
  - `Thumbnail-Info.plist` / `Preview-Info.plist` — the `.appex` Info.plists; their
    `QLSupportedContentTypes` list the supported UTIs.
  - `extension.entitlements` — read-only file sandbox.
- **Built by:** [`scripts/build-quicklook.sh`](../../scripts/build-quicklook.sh),
  invoked automatically by `make build` after `wails build`. It compiles a
  c-archive of the evaluator + geometry kernel, `swiftc`-links each extension
  against it, and injects `FacetThumbnail.appex` + `FacetQuickLook.appex` into the
  app bundle (ad-hoc signed locally; notarized by the release flow).
- **File-type declarations:** `desktop/build/darwin/Info.plist` — `.fct` is an
  `UTExportedTypeDeclaration` (Facet owns it); the mesh formats are
  `UTImportedTypeDeclarations` (recognized for Quick Look without claiming the
  default-open association, which stays with the user's slicer/CAD app).

## Refreshing during development

After `make build`, the app extensions must be **activated**, which happens when
the app is **launched** — `lsregister` alone registers the app for document/UTI
purposes but does *not* activate the embedded `.appex` extensions:

```sh
open desktop/build/bin/Facet.app   # launch once so PlugInKit registers the extensions, then quit
qlmanage -r && qlmanage -r cache   # flush Quick Look's generator + thumbnail caches
```

Then thumbnails/previews update in Finder (you may need to relaunch Finder, or
re-run the two `qlmanage -r` commands). When iterating on the extension code, a
rebuilt `.appex` is picked up by re-running `pluginkit -a <path>.appex` (bumps
the registration) followed by `qlmanage -r`; a fresh `open` of the app is the
definitive way to re-bless a changed extension binary.

**Gotcha — the running Quick Look host caches the loaded `.appex` binary in
memory.** After a rebuild, `pluginkit -a` + `qlmanage -r` is *not* enough — the
preview keeps running the old code. Force a fresh load:

```sh
killall QuickLookUIService   # drops the cached preview-extension binary
killall FacetQuickLook        # in case the preview process is still alive
```

This is easy to mistake for a logic bug (e.g. "the animation isn't playing") when
the code is actually correct and a stale binary is running.

## Limitation — `.fct` files that import external libraries

The extension runs in a read-only sandbox with **no network** and access to *only*
the previewed file. Inside that sandbox `~`/`os.UserConfigDir` redirect to the
extension's own container, so it can neither fetch a git library nor read the main
app's existing cache at `~/Library/Application Support/Facet/libcache`. A `.fct`
that imports an external library therefore fails to evaluate — the preview shows
the source text and the thumbnail shows the document icon. Stdlib-only/self-contained
`.fct` files and all mesh formats (`.stl/.obj/.3mf`) render fine, since they need no
library fetch.

The planned fix (deferred) is an **App Group shared cache**: the non-sandboxed app
fetches libraries with network access and writes them to a group container the
sandboxed extension can read, passing that path into the c-archive as the loader's
git-cache dir. This works in signed release builds (a real Team ID enables App
Groups); local ad-hoc dev simply falls back to today's behavior.

## Why there's no build/install artifact here

`packaging/windows/` and `packaging/linux/` hold standalone thumbnail handlers
(a COM DLL; a freedesktop `.thumbnailer` + MIME XML) that are built and registered
with the OS *separately* from the app. macOS app extensions are part of the signed
`.app` bundle itself, so there is no separate artifact to package — only the build
step in `scripts/build-quicklook.sh`.
