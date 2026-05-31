import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';
import { getActiveEntryPoint } from './helpers/viewer';

// Parse `fn Foo() ...` declarations out of source text and return the names.
// Used by the eval mock to emit a synthetic entry-point list per source.
function extractFnNames(text: string): string[] {
  const out: string[] = [];
  const re = /\bfn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) out.push(m[1]);
  return out;
}

test('switching tabs preserves each tab\'s entry-point selection without error', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // Stub CreateScratchFile to return a deterministic second-tab key. The key
  // becomes the tab's libPath, which the eval handler below uses to emit
  // entry-points scoped to each tab.
  await page.goto('/');
  await page.evaluate(() => {
    (window as any).__mockOverrides.CreateScratchFile = () => 'scratch-2';
  });

  // The eval mock emits one entry-point per `fn` declared in each source, with
  // libPath set to that source's tab key. The frontend filters entry-points by
  // libPath === activeTab, so this is what makes per-tab entry-points work.
  await setEvalHandler(body => {
    const entryPoints = [];
    for (const [key, text] of Object.entries(body.sources)) {
      for (const name of extractFnNames(text)) {
        entryPoints.push({
          name,
          signature: `${name}() Solid`,
          params: [],
          libPath: key,
          libVar: '',
          doc: '',
        });
      }
    }
    return { errors: [], entryPoints, symbols: [], posMap: [] };
  });

  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Tab 1: source declaring a single entry point.
  await setEditorText(page, 'fn OnlyMain() { cube(10) }\n');
  await expect.poll(() => getActiveEntryPoint(page)).toBe('OnlyMain');

  // Open tab 2 via the toolbar New button.
  await page.locator('#new-btn').click();

  // Tab 2: source declaring two entry points. The selector should land on the
  // first one (Main) by pickEntryPoint's "no prior selection" branch.
  await setEditorText(page, 'fn Main() { cube(10) }\nfn Demo() { sphere(5) }\n');
  await expect.poll(() => getActiveEntryPoint(page)).toBe('Main');

  // Switch back to tab 1 — entry point must be restored, no console error.
  await page.locator('#tab-bar .tab').first().click();
  await expect.poll(() => getActiveEntryPoint(page)).toBe('OnlyMain');

  // Re-enter tab 2 — second pass through the bug-prone transition.
  await page.locator('#tab-bar .tab').nth(1).click();
  await expect.poll(() => getActiveEntryPoint(page)).toBe('Main');

  // The harness fixture asserts no console errors at teardown, covering the
  // "error when switching tabs" symptom.
});
