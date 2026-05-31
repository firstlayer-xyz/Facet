import { test as base, Page } from '@playwright/test';
import { installEvalRoute, loadFixture, EvalHandler } from './mocks/eval-mock';

// Defaults: minimum a fresh boot needs. Each test can override individual
// methods via page.evaluate(o => Object.assign(window.__mockOverrides, o), {...}).
// Keys are method NAMES on the Wails-generated App module. The wailsjs/ tree
// is gitignored and only exists locally after `wails dev` or `wails build`, so
// we cannot statically import it here (CI would fail tsc). At runtime, the
// Proxy in the addInitScript below returns undefined for unknown methods, so
// a missed default surfaces as a method returning undefined rather than a
// throw — usually loud enough for the test to fail on its own.
const DEFAULT_FIXTURES: Record<string, unknown> = {
  GetDefaultSource: loadFixture('default-source'),
  GetSettings: '{}',
  GetHTTPAuth: { port: 65432, token: 'mock-token' },  // page.route below intercepts before fetch lands
  ListLibraries: [],
  ListLocalLibraries: [],
  DetectSlicers: [],
  GetDocCatalog: [],
  GetDocGuides: [],
  GetExample: '',
  GetMemoryLimit: 0,
  GetLibraryDir: '/mock/libs',
  GetLogDir: '/mock/logs',
  ConfirmDiscard: true,
};

export const test = base.extend<{ mockedPage: Page; setEvalHandler: (handler: EvalHandler) => Promise<void> }>({
  mockedPage: async ({ page }, use) => {
    await page.addInitScript((defaults) => {
      // Runs in the page context BEFORE any app code. `defaults` is the
      // second arg to addInitScript, structured-cloned across the boundary.

      (window as any).__mockCalls = [];
      (window as any).__mockOverrides = {};

      const make = (name: string) => async (...args: unknown[]) => {
        const override = (window as any).__mockOverrides[name];
        const result = override
          ? await override(...args)
          : (defaults as Record<string, unknown>)[name];
        (window as any).__mockCalls.push({ name, args, returned: result });
        return result;
      };

      // Proxy returns a stub for any property accessed — a NEW Wails method
      // (one we forgot to fixture) just returns undefined instead of throwing.
      // The tsc-time WailsApp import is the real drift guard.
      const App = new Proxy({}, { get: (_, prop) => make(String(prop)) });

      (window as any).go = { main: { App } };

      // Wails runtime stubs — covers all window.runtime methods the app calls.
      // EventsOff/EventsOffAll are used as cleanup in settings_debug.ts and
      // must be present to avoid TypeError at teardown. The generated
      // runtime.js wrappers reach into window.runtime[name], so any name the
      // app actually imports needs an entry here.
      (window as any).runtime = {
        EventsOn: () => {},
        EventsOnMultiple: () => {},
        EventsOnce: () => {},
        EventsOff: () => {},
        EventsOffAll: () => {},
        ClipboardSetText: async () => {},
        ClipboardGetText: async () => '',
        WindowToggleMaximise: () => {},
        BrowserOpenURL: () => {},
      };
    }, DEFAULT_FIXTURES);

    // Intercept eval HTTP calls — the app POSTs to http://127.0.0.1:<port>/eval
    // during boot. Default response is the eval-cube fixture so the app's
    // post-eval code path doesn't fault.
    await installEvalRoute(page, () => loadFixture('eval-cube'));

    // Forward console errors as test failures.
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('pageerror', err => consoleErrors.push(err.message));

    await use(page);

    if (consoleErrors.length) {
      throw new Error(`Console errors during test:\n${consoleErrors.join('\n')}`);
    }
  },
  setEvalHandler: async ({ page }, use) => {
    await use(async (handler: EvalHandler) => {
      await page.unroute('**/eval');
      await installEvalRoute(page, handler);
    });
  },
});

export { expect } from '@playwright/test';
