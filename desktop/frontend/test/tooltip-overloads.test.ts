import { test, expect } from './harness';
import { setEditorText, hoverAt } from './helpers/editor';

// Regression test for: "hover works, but only shows one overload".
//
// The Go-side doc extractor emits one DocEntry per declaration, so an
// overloaded function appears as N DocEntries with the same name+kind
// and distinct signatures. editor.ts used to call docEntries.find()
// once and render that single entry's signature. Now it collects all
// matching entries and joins their signatures in the tooltip.
test('Monaco hover shows all overloads of an identifier', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // Three overloads of `cube`: one with a single Length, one with three
  // Lengths, one with a Vec3. Distinct signatures, same name+kind.
  const overloads = [
    'fn cube(size Length) Solid',
    'fn cube(x Length, y Length, z Length) Solid',
    'fn cube(size Vec3) Solid',
  ];

  await setEvalHandler(() => ({
    errors: [],
    entryPoints: [],
    docIndex: overloads.map(sig => ({
      name: 'cube',
      signature: sig,
      doc: 'Create a cube.',
      kind: 'function',
      library: 'std',
    })),
    posMap: [],
    references: {},
    declarations: { decls: {} },
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'cube(10)\n');
  await evalDone;

  // Hover on `cube` (line 1, col 2 — inside the word).
  await hoverAt(page, 1, 2);

  const hover = page.locator('.monaco-hover').first();
  await expect(hover).toBeVisible({ timeout: 2_000 });

  // All three overload signatures should appear in the tooltip body.
  const hoverText = await hover.innerText();
  for (const sig of overloads) {
    expect(hoverText, `hover text should contain "${sig}":\n${hoverText}`).toContain(sig);
  }
});
