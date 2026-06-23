// End-to-end test: a #code=<base64url(version ++ brotli)> URL hash — the format
// the desktop app's Share button produces (see pkg/sharelink) — loads and
// renders on boot instead of the default example. A corrupt payload and a
// decompression bomb each surface a visible decode error rather than silently
// falling through to the default example. The wasm engine decodes via
// facetDecodeShare; this test exercises that path with an independent brotli
// implementation (Node's RFC-7932 zlib), confirming format compatibility.

const zlib = require('zlib');
const { chromium } = require('playwright');
const { runTest } = require('./harness');

const URL = process.env.FACET_WEB_URL || 'http://localhost:8000/';

// Must match pkg/sharelink.FormatBrotli.
const SHARE_FORMAT_BROTLI = 0x01;

const SRC = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`;

function encodeShareHash(source) {
  const compressed = zlib.brotliCompressSync(Buffer.from(source, 'utf8'), {
    params: { [zlib.constants.BROTLI_PARAM_QUALITY]: 11 },
  });
  return Buffer.concat([Buffer.from([SHARE_FORMAT_BROTLI]), compressed]).toString('base64url');
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

  // Decompression bomb: a payload inflating past the 4 MiB cap must error,
  // not balloon in the visitor's browser.
  await page.goto(URL + '#code=' + encodeShareHash('//'.padEnd(8 << 20, 'x')));
  await page.reload();
  await page.waitForFunction(
    () => document.getElementById('error-box').textContent.includes('Failed to decode shared model'),
    null, { timeout: 60_000 },
  );

  console.log('  shared source rendered; corrupt payload + bomb errored visibly');
}, chromium);
