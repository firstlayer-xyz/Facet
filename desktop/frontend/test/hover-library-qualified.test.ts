import { test, expect } from './harness';
import { setEditorText, hoverAt } from './helpers/editor';

// Regression test for the central bug this refactor fixes: hover on
// `T.Foo` (a library-qualified call) used to silently miss because the
// editor's docEntry-based lookup keyed by a name shape that didn't
// match the checker's declarations map for qualified library calls.
//
// The fix routes hover through resolveSymbolAtCursor, which uses
// resolveReceiverContext: `T`'s varTypes is "Library:<ns>", so the
// lookup filters symbols by `library === ns`. The mocked symbol below
// carries the same library tag the checker would stamp.
test('hover on T.Foo (library-qualified call) shows the library function', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  await setEvalHandler(body => ({
    errors: [],
    entryPoints: [],
    symbols: [
      {
        name: 'HexNut',
        signature: 'fn HexNut(size Length) Solid',
        doc: 'Make a hex nut.',
        kind: 'function',
        library: 'github.com/example/fasteners',
      },
    ],
    // varTypes is keyed by source path. The editor stores
    // currentSourceKey = mainKey = activeTab after pushEditorData, so
    // the key must match the request's `key` field (the active tab).
    varTypes: {
      [body.key]: { T: 'Library:github.com/example/fasteners' },
    },
    references: {},
    declarations: { decls: {} },
    posMap: [],
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'var T = lib "github.com/example/fasteners@main";\nT.HexNut(size: 10)\n');
  await evalDone;

  // Hover on `HexNut` — line 2, col 4 (T=1, .=2, H=3, so col 3-8 is HexNut).
  await hoverAt(page, 2, 4);

  const hover = page.locator('.monaco-hover').first();
  await expect(hover).toBeVisible({ timeout: 2_000 });

  const hoverText = await hover.innerText();
  expect(hoverText).toContain('fn HexNut(size Length) Solid');
  expect(hoverText).toContain('Make a hex nut.');
});

// Pin the local-binding hover fallback: when the cursor sits on a
// param/var/const there is no Symbol entry, but the checker's
// references map carries the declared type. The hover provider
// synthesizes `name Type` so the user still sees something useful.
test('hover on a local variable shows synthesized type from references map', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  await setEvalHandler(() => ({
    errors: [],
    entryPoints: [],
    symbols: [],
    references: {
      // Token `x` at line 2, col 1.
      ':2:1': { line: 1, col: 5, file: '', kind: 'var', returnType: 'Length' },
    },
    declarations: {
      decls: { x: { line: 1, col: 5, file: '', kind: 'var', returnType: 'Length' } },
    },
    posMap: [],
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'var x = 10 mm;\nx\n');
  await evalDone;

  // Hover on `x` at line 2, col 1.
  await hoverAt(page, 2, 1);

  const hover = page.locator('.monaco-hover').first();
  await expect(hover).toBeVisible({ timeout: 2_000 });

  const hoverText = await hover.innerText();
  expect(hoverText).toContain('x Length');
});
