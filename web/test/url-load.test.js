// Tests the ?fct=<url> share-load feature: the web preview fetches a .fct from
// a URL given in the page's query string and renders it instead of the default
// example. Covers success, a failed fetch (error shown, no silent default), and
// the no-param regression. Fetches are mocked with page.route so the test is
// hermetic (no real network, no CORS).

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const BASE = process.env.FACET_WEB_URL || 'http://localhost:8000/';
const FIXTURE_SRC = '# url-load-fixture-marker\nfn Main() Solid { return Sphere(r: 5 mm) }\n';

(async () => {
  // 1. ?fct= fetches and loads the referenced design (not the default).
  await runTest('url-load: ?fct= loads the referenced design', async ({ page }) => {
    await page.route('**/shared-fixture.fct', route =>
      route.fulfill({ status: 200, contentType: 'text/plain', body: FIXTURE_SRC }));

    await page.goto(BASE + '?fct=' + encodeURIComponent('https://example.com/shared-fixture.fct'),
      { waitUntil: 'load' });

    await page.waitForFunction(
      () => document.getElementById('filename')?.textContent === 'shared-fixture.fct',
      null, { timeout: 60_000 });

    const errText = await page.evaluate(() => document.getElementById('error-box').textContent || '');
    if (/couldn't load/i.test(errText)) {
      throw new Error(`unexpected load error shown: ${errText}`);
    }
  }, chromium);

  // 2. A failed fetch surfaces a persistent error and does NOT silently load the
  //    default example (which would clear the error).
  await runTest('url-load: failed ?fct= shows error, no silent default', async ({ page }) => {
    await page.route('**/missing.fct', route =>
      route.fulfill({ status: 404, contentType: 'text/plain', body: 'not found' }));

    await page.goto(BASE + '?fct=' + encodeURIComponent('https://example.com/missing.fct'),
      { waitUntil: 'load' });

    // Error is shown and mentions the failed load.
    await page.waitForFunction(() => {
      const e = document.getElementById('error-box');
      return e && e.style.display === 'block' && /couldn't load/i.test(e.textContent);
    }, null, { timeout: 60_000 });

    // Default Tutorial was NOT substituted (that would have cleared the error).
    const name = await page.evaluate(() => document.getElementById('filename').textContent || '');
    if (name === 'Tutorial.fct') {
      throw new Error('default example was loaded on failure — it should not be, the error must persist');
    }
  }, chromium);

  // 3. No ?fct= → unchanged default-example behavior (regression guard).
  await runTest('url-load: no ?fct= loads the default example', async ({ page }) => {
    await page.goto(BASE, { waitUntil: 'load' });
    await page.waitForFunction(
      () => document.getElementById('filename')?.textContent === 'Tutorial.fct',
      null, { timeout: 60_000 });
  }, chromium);
})();
