import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression test for: signature help (the popup inside `()`) used to
// show only one overload because the registerSignatureHelpProvider in
// src/editor.ts called docEntries.find() once and returned a single-
// element `signatures` array. Monaco supports multiple signatures with
// ↑/↓ navigation — we now return them all.
test('signature help inside () shows all overloads of the called function', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
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
  await setEditorText(page, 'cube(\n');
  await evalDone;

  // Position the caret right after the `(`.
  await page.evaluate(() => {
    const ed = (window as any).monaco.editor.getEditors()[0];
    ed.setPosition({ lineNumber: 1, column: 6 });
    // Triggering "Parameter Hints" via the editor command surfaces the
    // signature help widget reliably without depending on typing a
    // trigger character (which the test's setEditorText doesn't emit).
    ed.trigger('test', 'editor.action.triggerParameterHints', null);
  });

  // Monaco renders signature help as `.parameter-hints-widget`. The
  // overload indicator "1/3" appears alongside the up/down chevrons
  // when more than one signature is available.
  const widget = page.locator('.parameter-hints-widget').first();
  await expect(widget).toBeVisible({ timeout: 3_000 });

  const widgetText = await widget.innerText();
  // All three signature labels should be reachable. The active one
  // shows first; the others surface as we cycle. As a lighter-weight
  // check, assert the overload counter shows `1/3` (or `1 of 3`) —
  // its presence proves Monaco received all three signatures.
  expect(widgetText, widgetText).toMatch(/\b1\s*[/of]\s*3\b/);
});
