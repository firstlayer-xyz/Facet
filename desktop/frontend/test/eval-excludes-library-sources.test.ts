import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';
import type { EvalRequestBody } from './mocks/eval-mock';

// Regression for: opening a library in a view-only tab (by navigating into it —
// face-click or Go to Declaration) poisoned the next eval. The editor round-
// tripped the library's text back as a user source; the loader then re-resolved
// it standalone and its relative import (`lib "../threads"`) failed with
// "relative imports are not allowed from this source" — intermittently, and it
// wiped the tabs' read-only state. The fix: never send resolved-library sources
// (stdlib / local-lib / cached-remote) in the eval payload; the loader owns them.
//
// The library model is present here as a restored tab (main stays the active
// tab throughout) rather than via a runtime face-click. That keeps the test off
// Monaco's model-switch path — switching models logs a benign "Canceled" the
// strict harness would fail on — while exercising the exact code under test:
// projectSources() filtering editor.getAllSources() by the eval store's kinds.

const MAIN_KEY = '/project/main.fct';
const MAIN_TEXT = 'fn Main() Solid { return HexNut() }\n';

// A remote library keyed by its virtual "git+..." backing, exactly as the eval
// response reports it. Its `lib "../threads"` relative import is what used to
// blow up when the source was re-sent as a user root.
const LIB_KEY = 'git+github.com/firstlayer-xyz/facetlibs@77cec59/fasteners/fasteners.fct';
const LIB_TEXT = 'var T = lib "../threads"\nfn HexNut() Solid { return Cube() }\n';

const SOURCE_USER = 0;
const SOURCE_CACHED = 3;

test('a view-only library tab is excluded from the eval request sources', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  const bodies: EvalRequestBody[] = [];

  await setEvalHandler(body => {
    bodies.push(body);
    // Report the user's file as a normal source and the library as cached-
    // remote, so the eval store learns the library's kind. entryPoints keep the
    // app on the render path rather than looping check-only.
    return {
      errors: [],
      entryPoints: [
        { name: 'Main', signature: 'Main() Solid', params: [], libPath: MAIN_KEY, libVar: '', doc: '' },
      ],
      symbols: [],
      posMap: [],
      sources: {
        [body.key]: { text: body.sources[body.key] ?? MAIN_TEXT, kind: SOURCE_USER },
        [LIB_KEY]: { text: LIB_TEXT, kind: SOURCE_CACHED },
      },
    };
  });

  // Restore two tabs at boot: the user's file (active) and the library as a
  // view-only tab. Both are set BEFORE navigation so boot reads them. The
  // harness's own init script seeds __mockOverrides = {}; Object.assign keeps
  // those defaults and layers ours on top.
  await page.addInitScript(({ mainKey, mainText, libKey, libText }) => {
    Object.assign((window as any).__mockOverrides, {
      GetSettings: () => JSON.stringify({
        savedTabs: [
          { path: mainKey, label: 'main', cursor: null },
          { path: libKey, label: 'fasteners', cursor: null },
        ],
        activeTab: mainKey,
      }),
      OpenRecentFile: (path: string) => ({
        path,
        source: path === libKey ? libText : mainText,
      }),
    });
  }, { mainKey: MAIN_KEY, mainText: MAIN_TEXT, libKey: LIB_KEY, libText: LIB_TEXT });

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // The boot eval populates the library's kind in the eval store; the library
  // model is loaded (restored tab) but never displayed.
  await expect.poll(() => bodies.length).toBeGreaterThan(0);

  // Edit the active (user) file — no model switch — to fire a fresh eval that
  // runs while the library model is present in the editor.
  const beforeEdit = bodies.length;
  await setEditorText(page, 'fn Main() Solid { return HexNut() } // edited\n');
  await expect.poll(() => bodies.length).toBeGreaterThan(beforeEdit);

  // The library source must NOT ride along in the payload — only the user file.
  const last = bodies[bodies.length - 1];
  expect(Object.keys(last.sources)).not.toContain(LIB_KEY);
  expect(Object.keys(last.sources)).toContain(MAIN_KEY);
});
