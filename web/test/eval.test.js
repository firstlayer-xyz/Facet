// End-to-end test: parse + eval a no-lib `.fct` source through wasm and
// verify the response is a non-empty Uint8Array in the expected binary
// shape (4-byte little-endian header length + JSON header + binary data
// payload). Catches regressions in the eval path, the C++ wasm bridge,
// and the response packing.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const SRC = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`;

runTest('eval', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetParse === 'function' && typeof window.facetEval === 'function',
    null,
    { timeout: 60_000 },
  );

  const result = await page.evaluate(async (src) => {
    const parseRes = await window.facetParse(src);
    if (!parseRes?.ok) {
      return { stage: 'parse', error: parseRes?.error || 'parse failed' };
    }
    const t0 = performance.now();
    const evalRes = await window.facetEval(src, 'Main', '{}');
    const ms = performance.now() - t0;
    if (!(evalRes instanceof Uint8Array)) {
      return { stage: 'eval', error: `expected Uint8Array, got ${typeof evalRes}` };
    }
    // Decode the 4-byte LE header length and the JSON header.
    const dv = new DataView(evalRes.buffer, evalRes.byteOffset, evalRes.byteLength);
    const headerLen = dv.getUint32(0, true);
    const headerJSON = new TextDecoder().decode(evalRes.subarray(4, 4 + headerLen));
    const header = JSON.parse(headerJSON);
    return {
      stage: 'done',
      ms,
      totalBytes: evalRes.byteLength,
      headerLen,
      mesh: header.mesh,
      errors: header.errors,
    };
  }, SRC);

  if (result.stage !== 'done') {
    throw new Error(`failed at ${result.stage}: ${result.error}`);
  }
  if (result.errors && result.errors.length > 0) {
    throw new Error(`eval header carries errors: ${JSON.stringify(result.errors)}`);
  }
  if (!result.mesh) {
    throw new Error('eval returned no mesh');
  }
  // The wasm path populates meshMeta.expandedCount (per-face exploded
  // verts) and/or vertexCount (raw shared-vertex count). Accept either.
  const verts = result.mesh.vertexCount || 0;
  const expanded = result.mesh.expandedCount || 0;
  if (verts === 0 && expanded === 0) {
    throw new Error(`mesh produced no vertices (vertexCount=0, expandedCount=0): ${JSON.stringify(result.mesh)}`);
  }
  console.log(
    `  eval returned ${result.totalBytes} bytes (header ${result.headerLen}B), ` +
    `vertexCount=${verts}, expandedCount=${expanded} in ${result.ms.toFixed(1)} ms`,
  );
}, chromium);
