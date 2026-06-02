// Smoke-tests EVERY bundled example through the browser wasm. For each example
// it parses + evaluates the first entry point and classifies the outcome. The
// pass bar is robustness, not success: an example may render a mesh OR fail
// cleanly (e.g. a lib-importing example blocked by CORS surfaces a structured
// error). What must NEVER happen is a Go panic (undefined/null/non-Uint8Array
// return) or a thrown/hung evaluation. This is the "test them all" surface
// (importing examples will start rendering once the CORS round lands).

const { chromium } = require('playwright');
const { runTest } = require('./harness');

runTest('examples-smoke', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetExamples === 'function'
       && typeof window.facetExample === 'function'
       && typeof window.facetParse === 'function'
       && typeof window.facetEval === 'function',
    null,
    { timeout: 60_000 },
  );

  const results = await page.evaluate(async () => {
    const names = JSON.parse(window.facetExamples());
    const out = [];
    for (const name of names) {
      try {
        const src = window.facetExample(name);
        if (src && typeof src === 'object' && src.error) {
          out.push({ name, category: 'PANIC', detail: `facetExample error: ${src.error}` });
          continue;
        }
        const parseRes = await window.facetParse(src);
        if (parseRes === undefined || parseRes === null) {
          out.push({ name, category: 'PANIC', detail: 'facetParse returned undefined (Go panic)' });
          continue;
        }
        if (!parseRes.ok) {
          out.push({ name, category: 'parse-error', detail: parseRes.error });
          continue;
        }
        const entry = JSON.parse(parseRes.entryPoints || '[]')[0]?.name;
        if (!entry) { out.push({ name, category: 'no-entry', detail: 'no entry point' }); continue; }

        const evalRes = await window.facetEval(src, entry, '{}');
        if (!(evalRes instanceof Uint8Array)) {
          out.push({ name, category: 'PANIC', detail: `facetEval returned ${typeof evalRes} (Go panic)` });
          continue;
        }
        const dv = new DataView(evalRes.buffer, evalRes.byteOffset, evalRes.byteLength);
        const headerLen = dv.getUint32(0, true);
        const header = JSON.parse(new TextDecoder().decode(evalRes.subarray(4, 4 + headerLen)));
        if (header.errors && header.errors.length > 0) {
          const e0 = header.errors[0];
          const detail = typeof e0 === 'string' ? e0 : (e0?.message || JSON.stringify(e0));
          out.push({ name, category: 'eval-error', detail, entry });
          continue;
        }
        const verts = (header.mesh?.vertexCount || 0) + (header.mesh?.expandedCount || 0);
        out.push({ name, category: verts > 0 ? 'rendered' : 'empty', detail: `${verts} verts`, entry });
      } catch (e) {
        out.push({ name, category: 'THREW', detail: String(e && e.message || e) });
      }
    }
    return out;
  });

  const tag = { rendered: '✓', 'eval-error': '·', 'parse-error': '·', 'no-entry': '·', empty: '?', PANIC: '✗', THREW: '✗' };
  for (const r of results) {
    console.log(`  ${tag[r.category] || '?'} ${r.name} [${r.category}] ${r.detail}`);
  }
  const rendered = results.filter(r => r.category === 'rendered').length;
  const cleanFail = results.filter(r => r.category === 'eval-error' || r.category === 'parse-error' || r.category === 'no-entry').length;
  console.log(`  → ${rendered} rendered, ${cleanFail} clean-fail, ${results.length} total`);

  const bad = results.filter(r => r.category === 'PANIC' || r.category === 'THREW');
  if (bad.length > 0) {
    throw new Error(`examples crashed (panic/throw): ${bad.map(b => `${b.name}: ${b.detail}`).join('; ')}`);
  }
  if (rendered === 0) {
    throw new Error('no example rendered a mesh — eval path is broken');
  }
}, chromium);
