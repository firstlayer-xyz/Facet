import { test, expect } from './harness';
import { setEditorText, hoverAt } from './helpers/editor';

test('Monaco hover tooltip appears, hides, and reappears', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // Generous per-test cap: the hover-visibility waits below are raised to 10s
  // each to tolerate slow content fetch + fade-in under CI load, which can
  // exceed the 20s default when summed. These are ceilings, not delays — a
  // healthy run finishes in a few seconds.
  test.setTimeout(60_000);

  // The hover provider in src/editor.ts pulls tooltip content from `symbols`
  // (the checker's symbol table) plus `references` (token → declaration).
  // The default eval-cube fixture has neither, so an unaugmented hover
  // returns null and Monaco never shows the widget. Provide a minimal
  // enriched response keyed off the literal source text in the request body.
  await setEvalHandler(body => ({
    errors: [],
    entryPoints: [
      { name: 'cube', signature: 'cube(size: Length) Solid', params: [], libPath: '', libVar: '', doc: 'A 10mm cube.' },
    ],
    symbols: [
      { name: 'cube', signature: 'cube(size: Length) Solid', doc: 'A 10mm cube.', kind: 'function', library: '' },
    ],
    // References are keyed by "<srcKey>:line:col" using the actual
    // request key (the active tab). Tabs are peers — no "" sentinel.
    references: {
      [`${body.key}:1:1`]: { line: 1, col: 1, file: body.key, kind: 'fn', returnType: 'Solid' },
    },
    declarations: {
      decls: { cube: { line: 1, col: 1, file: body.key, kind: 'fn', returnType: 'Solid' } },
    },
    posMap: [],
  }));

  await page.goto('/');
  // Wait for editor to be ready.
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  await setEditorText(page, 'cube(10)\n');

  // Monaco renders its hover widget as `.monaco-hover` when active.
  const hover = page.locator('.monaco-hover').first();

  // Hover on `cube` (line 1, col 2). The hover widget mounts quickly but its
  // content fetch + fade-in can run well past 2s under CI load (the widget
  // sits in the DOM as `.monaco-hover ... hidden` until then), so wait
  // generously — the assertion is "it shows", not "it shows within 2s".
  await hoverAt(page, 1, 2);
  await expect(hover).toBeVisible({ timeout: 10_000 });

  // Move mouse far away — Monaco hides the hover when the pointer leaves.
  await page.mouse.move(0, 0);
  await expect(hover).toBeHidden({ timeout: 5_000 });

  // Re-hover — the recurring breakage pattern is that the second hover never
  // re-shows the tooltip.
  await hoverAt(page, 1, 2);
  await expect(hover).toBeVisible({ timeout: 10_000 });
});
