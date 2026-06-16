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
  - `PreviewViewController.swift` — `QLPreviewingController` (drag-to-orbit preview)
  - `FacetMesh.swift` — shared loader: calls the in-process evaluator/kernel
    (`FacetRenderFile`, the c-archive from `cmd/facetrender`) and builds the
    SceneKit scene. A Quick Look extension is sandboxed and cannot spawn `facetc`,
    so it links the renderer directly.
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

## Why there's no build/install artifact here

`packaging/windows/` and `packaging/linux/` hold standalone thumbnail handlers
(a COM DLL; a freedesktop `.thumbnailer` + MIME XML) that are built and registered
with the OS *separately* from the app. macOS app extensions are part of the signed
`.app` bundle itself, so there is no separate artifact to package — only the build
step in `scripts/build-quicklook.sh`.
