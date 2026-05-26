// Smoke test: a no-lib `.fct` source parses through wasm and returns
// entry-point metadata. Verifies the wasm boots, the loader runs against
// in-memory storage, and `window.facetParse` resolves its Promise with
// `ok: true`. Catches regressions in the basic Go-wasm bootstrap and the
// memory-backed loader path.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const SRC = 'fn Main() Solid { return Sphere(r: 10 mm) }';

runTest('parse-simple', async ({ page }) => {
  await page.waitForFunction(() => typeof window.facetParse === 'function', null, {
    timeout: 60_000,
  });

  const result = await page.evaluate(async (src) => {
    const t0 = performance.now();
    const r = await window.facetParse(src);
    return { ms: performance.now() - t0, ok: r?.ok, error: r?.error, entryPoints: r?.entryPoints };
  }, SRC);

  console.log(`  facetParse returned in ${result.ms.toFixed(1)} ms`);
  if (!result.ok) {
    throw new Error(`facetParse ok=false, error=${result.error}`);
  }
  if (!result.entryPoints || JSON.parse(result.entryPoints).length === 0) {
    throw new Error('facetParse returned no entryPoints');
  }
  const eps = JSON.parse(result.entryPoints);
  if (eps[0].name !== 'Main') {
    throw new Error(`expected first entryPoint Main, got ${eps[0]?.name}`);
  }
}, chromium);
