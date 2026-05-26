// Regression test: a `.fct` that imports `lib "github.com/..."` makes it
// past the wasm storage layer and through the Promise boundary without
// triggering the Go runtime's deadlock detector.
//
// The test loads share/examples/Bolt And Nut.fct (which imports
// firstlayer-xyz/facetlibs/fasteners) and calls `await
// window.facetParse(src)`. Two separate things are being verified:
//
//   1. The wasm doesn't panic with "all goroutines are asleep -
//      deadlock!" while waiting on the network fetch. This is the
//      regression guarded by the Promise refactor.
//
//   2. The result Promise resolves cleanly — the lib-resolution error
//      surfaces as a structured `{ok: false, error: …}` object rather
//      than a hung Promise or a thrown exception.
//
// As of the time this test was written, the network fetch itself fails
// (GitHub's smart-HTTP `/info/refs` endpoint doesn't return
// Access-Control-Allow-Origin) so the result is `ok: false` with a
// `fetch() failed` error message. Once a CORS-friendly raw-content
// LibTree replaces go-git on the wasm side, this test should still pass
// — flip the assertion to `ok: true` then.

const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const { runTest } = require('./harness');

const FCT = path.join(__dirname, '..', '..', 'share', 'examples', 'Bolt And Nut.fct');

runTest('lib-import', async ({ page }) => {
  await page.waitForFunction(() => typeof window.facetParse === 'function', null, {
    timeout: 60_000,
  });

  const src = fs.readFileSync(FCT, 'utf8');
  console.log(`  loaded ${path.basename(FCT)} (${src.length} bytes)`);

  const result = await page.evaluate(async (src) => {
    const t0 = performance.now();
    try {
      const r = await window.facetParse(src);
      return { resolved: true, ms: performance.now() - t0, result: r };
    } catch (e) {
      return { resolved: false, ms: performance.now() - t0, error: String(e) };
    }
  }, src);

  if (!result.resolved) {
    throw new Error(`Promise rejected (likely Go panic): ${result.error}`);
  }
  console.log(`  facetParse Promise resolved in ${result.ms.toFixed(1)} ms`);

  // A wasm-side panic produces undefined, not a structured result.
  if (result.result === undefined || result.result === null) {
    throw new Error('facetParse returned undefined — Go panicked');
  }

  // We accept either ok:true (lib fetch succeeded — future state once
  // CORS-friendly fetch is in place) or ok:false with a structured
  // error message. Either is fine; the regression we're guarding is the
  // deadlock that produced no result at all.
  if (!result.result.ok) {
    if (typeof result.result.error !== 'string' || result.result.error.length === 0) {
      throw new Error(`ok:false but error message missing: ${JSON.stringify(result.result)}`);
    }
    console.log(`  resolved with structured error: ${result.result.error.split('\n')[0]}`);
  } else {
    console.log(`  lib resolution succeeded`);
  }
}, chromium);
