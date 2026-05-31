import { test, expect } from './harness';
import { setEditorText, rightClickAt } from './helpers/editor';

// Regression test for: "right-click 'Open Documentation' opens inside the
// floating mini-preview window when fullcode (View) mode is active."
//
// Root cause was DocsPanel being constructed with #canvas-container as
// its parent. Fullcode mode reparents #canvas-container into a floating
// #mini-preview, so the docs panel got dragged inside that small window.
// Fixed by making docs a sibling drawer of canvas — fullcode.ts now
// lifts it out alongside the assistant panel.
test('docs panel opens in its own drawer, not inside #mini-preview, when fullcode is active', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // Eval response must include `docIndex` so editor.docEntries gets
  // populated — the right-click "Open Documentation" action looks up the
  // clicked identifier in that list before calling onOpenDocs.
  await setEvalHandler(() => ({
    errors: [],
    entryPoints: [],
    docIndex: [
      {
        name: 'foo',
        signature: 'foo()',
        doc: 'My foo function',
        kind: 'fn',
        library: 'main',
      },
    ],
    posMap: [],
    references: {},
    declarations: { decls: {} },
  }));

  await page.goto('/');

  // No guides — keeps GetDocGuides response trivial; docs panel falls back
  // to the API view, which is what openDocsToEntry focuses anyway. Must
  // come after goto so __mockOverrides exists in the page context.
  await page.evaluate(() => {
    (window as any).__mockOverrides.GetDocGuides = () => [];
  });

  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn foo() { cube(10) }\nfoo()\n');
  await evalDone;

  // Toggle fullcode mode. The #fullcode-btn (label "VIEW") collapses the
  // viewport and moves the canvas into a floating #mini-preview. Drawers
  // live in #drawer-stack permanently, so fullcode doesn't touch them.
  await page.locator('#fullcode-btn').click();
  await expect(page.locator('#mini-preview')).toBeVisible();

  // Trigger the same Monaco action the right-click "Open Documentation"
  // menu item invokes. Direct .trigger() bypasses context-menu rendering
  // (which is flaky to drive from Playwright in fullcode mode — overlay
  // z-indexes interact in ways that swallow clicks). The action callback
  // in src/editor.ts is the same code path either way; this test isn't
  // about menu rendering.
  await page.evaluate(() => {
    const ed = (window as any).monaco.editor.getEditors()[0];
    ed.setPosition({ lineNumber: 2, column: 2 });
    ed.trigger('test', 'facet.openDocs', null);
  });

  const docsPanel = page.locator('#docs-panel');
  await expect(docsPanel).toBeVisible();

  // The bug: docs would render INSIDE the floating mini-preview because
  // it shared the canvas's DOM parent. Assert the opposite — docs lives
  // outside that floating window.
  const inMiniPreview = await page.evaluate(() => {
    const docs = document.getElementById('docs-panel');
    const mini = document.getElementById('mini-preview');
    return docs != null && mini != null && mini.contains(docs);
  });
  expect(inMiniPreview).toBe(false);

  // Docs always lives under #drawer-stack regardless of mode — fullcode no
  // longer reparents drawers (they're overlay-positioned via the stack).
  const parentId = await docsPanel.evaluate(el => el.parentElement?.id);
  expect(parentId).toBe('drawer-stack');
});
