import { Page } from '@playwright/test';

// Returns Monaco's editor handle attached to the page. Throws if not ready.
async function editor(page: Page) {
  return await page.evaluateHandle(() => {
    // Monaco exposes editor instances on window.monaco.editor.getEditors().
    const m = (window as any).monaco;
    if (!m) throw new Error('monaco not loaded');
    const editors = m.editor.getEditors();
    if (!editors.length) throw new Error('no editor instances');
    return editors[0];
  });
}

// Replace the entire editor source. Triggers Monaco's `onDidChangeContent`
// just like a user typing would.
export async function setEditorText(page: Page, text: string) {
  const ed = await editor(page);
  await page.evaluate(({ ed, text }) => (ed as any).setValue(text), { ed, text });
}

// Right-click at a (line, column) position, both 1-based, mirroring Monaco's
// own coordinate system.
export async function rightClickAt(page: Page, line: number, col: number) {
  const ed = await editor(page);
  // Get the DOM coordinate from Monaco's coordinate system, then synthesize
  // a contextmenu event at that pixel.
  const xy = await page.evaluate(
    ({ ed, line, col }) => {
      const e = ed as any;
      const pos = { lineNumber: line, column: col };
      e.revealPosition(pos);
      const vc = e.getScrolledVisiblePosition(pos);
      const dom = e.getDomNode().getBoundingClientRect();
      return { x: dom.left + vc.left + 1, y: dom.top + vc.top + 1 };
    },
    { ed, line, col }
  );
  await page.mouse.move(xy.x, xy.y);
  await page.mouse.click(xy.x, xy.y, { button: 'right' });
}

// Hover at a position long enough for Monaco's hover-tooltip delay (~300ms default).
export async function hoverAt(page: Page, line: number, col: number) {
  const ed = await editor(page);
  const xy = await page.evaluate(
    ({ ed, line, col }) => {
      const e = ed as any;
      const pos = { lineNumber: line, column: col };
      e.revealPosition(pos);
      const vc = e.getScrolledVisiblePosition(pos);
      const dom = e.getDomNode().getBoundingClientRect();
      return { x: dom.left + vc.left + 1, y: dom.top + vc.top + 1 };
    },
    { ed, line, col }
  );
  await page.mouse.move(xy.x, xy.y);
  // Monaco's hover delay is ~300ms; give it 600ms to settle and stabilize.
  await page.waitForTimeout(600);
}
