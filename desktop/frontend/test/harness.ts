import { test as base, Page } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';

// Static drift check: if the Go side adds/removes/renames a Wails method,
// this typeof-import fails to compile and forces a mock update.
type WailsApp = typeof import('../wailsjs/go/main/App');

const FIXTURE_DIR = path.join(__dirname, 'mocks/fixtures');

function loadFixture(name: string): unknown {
  const raw = fs.readFileSync(path.join(FIXTURE_DIR, `${name}.json`), 'utf8');
  return JSON.parse(raw).value;
}

// Defaults: minimum a fresh boot needs. Each test can override individual
// methods via page.evaluate(o => Object.assign(window.__mockOverrides, o), {...}).
// Keys here are method NAMES on WailsApp; the value is whatever that method
// should resolve with by default.
const DEFAULT_FIXTURES: Partial<Record<keyof WailsApp, unknown>> = {
  GetDefaultSource: loadFixture('default-source'),
  GetSettings: '{}',
  GetHTTPAuth: { port: 65432, token: 'mock-token' },  // page.route below intercepts before fetch lands
  ListLibraries: [],
  ListLocalLibraries: [],
  DetectSlicers: [],
  GetDocGuides: [],
  GetExample: '',
  GetMemoryLimit: 0,
  GetLibraryDir: '/mock/libs',
  GetLogDir: '/mock/logs',
  ConfirmDiscard: true,
};

export const test = base.extend<{ mockedPage: Page }>({
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
      // EventsOn in the Wails JS bridge delegates to EventsOnMultiple, so
      // both must be present. EventsOff/EventsOffAll are used as cleanup in
      // settings_debug.ts and must be present to avoid TypeError at teardown.
      (window as any).runtime = {
        EventsOnMultiple: () => {},
        EventsOff: () => {},
        EventsOffAll: () => {},
        ClipboardSetText: async () => {},
        ClipboardGetText: async () => '',
        WindowToggleMaximise: () => {},
      };
    }, DEFAULT_FIXTURES);

    // Intercept eval HTTP calls — the app POSTs to http://127.0.0.1:<port>/eval
    // during boot. We return a minimal valid framed response (4-byte LE header
    // length + header JSON) so the app doesn't throw and log a console error.
    await page.route('**/eval', async (route) => {
      const headerJSON = JSON.stringify({});
      const headerBytes = new TextEncoder().encode(headerJSON);
      const buf = new ArrayBuffer(4 + headerBytes.length);
      const view = new DataView(buf);
      view.setUint32(0, headerBytes.length, true /* little-endian */);
      new Uint8Array(buf, 4).set(headerBytes);
      await route.fulfill({
        status: 200,
        contentType: 'application/octet-stream',
        body: Buffer.from(buf),
      });
    });

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
});

export { expect } from '@playwright/test';
