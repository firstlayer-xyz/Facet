// Verifies the bundled-examples wasm exports and the default model:
//   - facetExamples() returns a non-empty name list with Tutorial.fct first
//   - facetExample('Tutorial.fct') returns source that parses + evaluates to a
//     non-empty mesh (the model the page loads by default on boot).
// Catches regressions in the examples embed (facet/share/examples), the
// JS exports, and the default-on-load path.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

runTest('examples', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetExamples === 'function'
       && typeof window.facetExample === 'function'
       && typeof window.facetParse === 'function'
       && typeof window.facetEval === 'function',
    null,
    { timeout: 60_000 },
  );

  const result = await page.evaluate(async () => {
    const names = JSON.parse(window.facetExamples());
    // Load the default (Tutorial) the same way the page does on boot.
    const src = window.facetExample('Tutorial.fct');
    if (src && typeof src === 'object' && src.error) {
      return { names, stage: 'example', error: src.error };
    }
    const parseRes = await window.facetParse(src);
    if (!parseRes?.ok) return { names, stage: 'parse', error: parseRes?.error };
    const entry = JSON.parse(parseRes.entryPoints || '[]')[0]?.name;
    if (!entry) return { names, stage: 'entry', error: 'no entry point' };
    const evalRes = await window.facetEval(src, entry, '{}');
    if (!(evalRes instanceof Uint8Array)) {
      return { names, stage: 'eval', error: `expected Uint8Array, got ${typeof evalRes}` };
    }
    const dv = new DataView(evalRes.buffer, evalRes.byteOffset, evalRes.byteLength);
    const headerLen = dv.getUint32(0, true);
    const header = JSON.parse(new TextDecoder().decode(evalRes.subarray(4, 4 + headerLen)));
    const verts = (header.mesh?.vertexCount || 0) + (header.mesh?.expandedCount || 0);
    return { names, stage: 'done', entry, verts, errors: header.errors };
  });

  const { names } = result;
  if (!Array.isArray(names) || names.length === 0) {
    throw new Error(`facetExamples returned no names: ${JSON.stringify(names)}`);
  }
  if (names[0] !== 'Tutorial.fct') {
    throw new Error(`expected Tutorial.fct first, got ${names[0]}`);
  }
  if (!names.every(n => n.endsWith('.fct'))) {
    throw new Error(`non-.fct entries in list: ${JSON.stringify(names)}`);
  }
  if (result.stage !== 'done') {
    throw new Error(`default model failed at ${result.stage}: ${result.error}`);
  }
  if (result.errors && result.errors.length > 0) {
    throw new Error(`default eval carries errors: ${JSON.stringify(result.errors)}`);
  }
  if (result.verts === 0) {
    throw new Error('default model (Tutorial.fct) produced no vertices');
  }
  console.log(`  ${names.length} examples; default Tutorial.fct → entry ${result.entry}, ${result.verts} verts`);
}, chromium);
