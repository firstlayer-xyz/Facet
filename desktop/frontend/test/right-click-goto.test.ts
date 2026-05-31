import { test, expect } from './harness';
import { setEditorText, rightClickAt } from './helpers/editor';
import * as fs from 'node:fs';
import * as path from 'node:path';

const definitionFoo = JSON.parse(
  fs.readFileSync(
    path.join(__dirname, 'mocks/fixtures/definition-foo.json'),
    'utf8'
  )
).value;

test('right-click "Go to Declaration" jumps to the mocked declaration', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // The Facet editor's goto path (src/editor.ts findDecl) reads the
  // `references` map populated by the eval response — it does NOT issue a
  // separate fetch. Returning a references entry keyed on the call site is
  // enough to drive the "Go to Declaration" action menu item.
  await setEvalHandler(() => definitionFoo);

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Source: a function `foo` defined on line 1 (col 4 = where `foo` starts
  // after `fn `), called on line 2 col 1. Wait for the eval response to land
  // before right-clicking: the references map (which drives the
  // `facet.hasDeclaration` precondition on "Go to Declaration") is only
  // populated after an eval-response cycle.
  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn foo() { cube(10) }\nfoo()\n');
  await evalDone;

  // Right-click on `foo` at the call site (line 2, any col inside the word).
  // onMouseDown in editor.ts moves the caret to the click position before the
  // context menu evaluates `facet.hasDeclaration`.
  await rightClickAt(page, 2, 2);

  // Monaco renders the context menu under `.monaco-menu`; the action label is
  // "Go to Declaration" (registered with id facet.goToDeclaration). Use exact
  // text to avoid matching unrelated menu items like "Go to References".
  const menu = page.locator('.monaco-menu');
  await expect(menu).toBeVisible({ timeout: 2_000 });
  await menu.locator('text="Go to Declaration"').click();

  // After the action, the editor's caret should sit on the declaration.
  await expect.poll(() => page.evaluate(() => {
    const ed = (window as any).monaco.editor.getEditors()[0];
    const pos = ed.getPosition();
    return { lineNumber: pos.lineNumber, column: pos.column };
  })).toEqual({
    lineNumber: definitionFoo.references[':2:1'].line,
    column: definitionFoo.references[':2:1'].col,
  });
});
