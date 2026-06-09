// End-to-end test: a #code=<base64url(deflate-raw)> URL hash — as produced by
// the desktop app's Share button — loads and renders on boot instead of the
// default example, and a corrupt payload surfaces a visible decode error
// rather than silently falling through to the default example.

const zlib = require('zlib');
const { chromium } = require('playwright');
const { runTest } = require('./harness');

const URL = process.env.FACET_WEB_URL || 'http://localhost:8000/';

const SRC = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`;

function encodeShareHash(source) {
  return zlib.deflateRawSync(Buffer.from(source, 'utf8')).toString('base64url');
}

runTest('share-hash', async ({ page }) => {
  // Valid payload: boot must render the shared source, not the default example.
  // goto with only a hash change is a same-document navigation, so reload to
  // re-run the boot sequence with the hash present.
  await page.goto(URL + '#code=' + encodeShareHash(SRC));
  await page.reload();
  await page.waitForFunction(
    () => document.getElementById('filename').textContent === 'shared.fct',
    null, { timeout: 60_000 },
  );
  await page.waitForFunction(
    () => /tris|triangles/.test(document.getElementById('status').textContent),
    null, { timeout: 60_000 },
  );
  const source = await page.evaluate(() => currentSource);
  if (source !== SRC) {
    throw new Error(`currentSource mismatch: ${JSON.stringify(source)}`);
  }

  // Corrupt payload: visible error, no model, and no silent fall-through to
  // the default example.
  await page.goto(URL + '#code=not!valid!base64@@@');
  await page.reload();
  await page.waitForFunction(
    () => document.getElementById('error-box').textContent.includes('Failed to decode shared model'),
    null, { timeout: 60_000 },
  );
  const leaked = await page.evaluate(() => currentSource);
  if (leaked !== '') {
    throw new Error(`corrupt hash still loaded a source: ${JSON.stringify(leaked)}`);
  }

  console.log('  shared source rendered; corrupt payload errored visibly');
}, chromium);
