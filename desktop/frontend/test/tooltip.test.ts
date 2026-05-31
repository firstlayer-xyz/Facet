import { test, expect } from './harness';
import { setEditorText, hoverAt } from './helpers/editor';

test('Monaco hover tooltip appears, hides, and reappears', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // The hover provider in src/editor.ts pulls tooltip content from `docIndex`
  // (function/keyword docs) plus `references` (token → declaration). The default
  // eval-cube fixture has neither, so an unaugmented hover returns null and
  // Monaco never shows the widget. Provide a minimal enriched response keyed
  // off the literal source text in the request body.
  await setEvalHandler(() => ({
    errors: [],
    entryPoints: [
      { name: 'cube', signature: 'cube(size: Length) Solid', params: [], libPath: '', libVar: '', doc: 'A 10mm cube.' },
    ],
    docIndex: [
      { name: 'cube', signature: 'cube(size: Length) Solid', doc: 'A 10mm cube.', kind: 'function', library: '' },
    ],
    // `cube` at line 1, col 1 (startColumn from Monaco's getWordAtPosition is
    // 1-based). References key format is "file:line:col"; empty file = main.
    references: {
      ':1:1': { line: 1, col: 1, file: '', kind: 'fn', returnType: 'Solid' },
    },
    declarations: {
      decls: { cube: { line: 1, col: 1, file: '', kind: 'fn', returnType: 'Solid' } },
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

  // Hover on `cube` (line 1, col 2).
  await hoverAt(page, 1, 2);
  await expect(hover).toBeVisible({ timeout: 2_000 });

  // Move mouse far away — Monaco hides the hover when the pointer leaves.
  await page.mouse.move(0, 0);
  await expect(hover).toBeHidden({ timeout: 2_000 });

  // Re-hover — the recurring breakage pattern is that the second hover never
  // re-shows the tooltip.
  await hoverAt(page, 1, 2);
  await expect(hover).toBeVisible({ timeout: 2_000 });
});
