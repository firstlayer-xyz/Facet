// Verifies that a `.fct` importing `lib "github.com/..."` resolves and renders
// in the browser via the jsDelivr CORS-friendly mirror (go-git's git smart-HTTP
// clone is CORS-blocked, so the wasm build routes remote libs through jsDelivr —
// see loader.Options.RemoteFetch / web/wasm/main.go jsDelivrFetch).
//
// Loads share/examples/Bolt And Nut.fct (which imports
// firstlayer-xyz/facetlibs/fasteners, whose .fct in turn relative-imports
// ../threads) and asserts the whole chain parses + evaluates to a non-empty
// mesh. This exercises: the RemoteFetch hook, the HTTP-backed LibTree,
// transitive/relative imports through that tree, and that the eval Promise
// resolves without a Go deadlock. Requires network (jsDelivr).

const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const { runTest } = require('./harness');

const FCT = path.join(__dirname, '..', '..', 'share', 'examples', 'Bolt And Nut.fct');

runTest('lib-import', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetParse === 'function' && typeof window.facetEval === 'function',
    null,
    { timeout: 60_000 },
  );

  const src = fs.readFileSync(FCT, 'utf8');
  console.log(`  loaded ${path.basename(FCT)} (${src.length} bytes)`);

  const result = await page.evaluate(async (src) => {
    const t0 = performance.now();
    try {
      const parseRes = await window.facetParse(src);
      if (parseRes === undefined || parseRes === null) return { stage: 'parse', panic: true };
      if (!parseRes.ok) return { stage: 'parse', error: parseRes.error };
      const entry = JSON.parse(parseRes.entryPoints || '[]')[0]?.name;
      if (!entry) return { stage: 'entry', error: 'no entry point' };
      const ev = await window.facetEval(src, entry, '{}');
      if (!(ev instanceof Uint8Array)) return { stage: 'eval', error: `not Uint8Array: ${typeof ev}` };
      const dv = new DataView(ev.buffer, ev.byteOffset, ev.byteLength);
      const headerLen = dv.getUint32(0, true);
      const header = JSON.parse(new TextDecoder().decode(ev.subarray(4, 4 + headerLen)));
      return {
        stage: 'done',
        ms: performance.now() - t0,
        entry,
        verts: (header.mesh?.vertexCount || 0) + (header.mesh?.expandedCount || 0),
        errors: header.errors,
      };
    } catch (e) {
      return { stage: 'threw', error: String(e) };
    }
  }, src);

  if (result.panic)  throw new Error('facetParse returned undefined — Go panicked');
  if (result.stage !== 'done') throw new Error(`lib import failed at ${result.stage}: ${result.error}`);
  if (result.errors && result.errors.length > 0) {
    throw new Error(`eval carries errors: ${JSON.stringify(result.errors)}`);
  }
  if (result.verts === 0) throw new Error('imported model produced no geometry');
  console.log(`  lib import via jsDelivr OK — entry ${result.entry}, ${result.verts} verts in ${result.ms.toFixed(0)} ms`);
}, chromium);
