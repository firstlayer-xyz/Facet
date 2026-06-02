// Verifies the eval response carries a per-vertex color buffer and that a
// colored model produces the expected distinct face colors. Guards the Go
// color-buffer packing (web/wasm/main.go) and the binary format the WebGL
// renderer consumes. (Headless WebGL pixel readback is flaky, so this asserts
// the color DATA the renderer uploads, not the rendered pixels.)

const { chromium } = require('playwright');
const { runTest } = require('./harness');

runTest('colors', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetExample === 'function'
       && typeof window.facetParse === 'function'
       && typeof window.facetEval === 'function',
    null,
    { timeout: 60_000 },
  );

  const r = await page.evaluate(async () => {
    const src = window.facetExample('Color Cube.fct');
    if (src && typeof src === 'object' && src.error) return { stage: 'example', error: src.error };
    const parseRes = await window.facetParse(src);
    if (!parseRes?.ok) return { stage: 'parse', error: parseRes?.error };
    const entry = JSON.parse(parseRes.entryPoints || '[]')[0]?.name;
    const evalRes = await window.facetEval(src, entry, '{}');
    if (!(evalRes instanceof Uint8Array)) return { stage: 'eval', error: `not Uint8Array: ${typeof evalRes}` };

    const dv = new DataView(evalRes.buffer, evalRes.byteOffset, evalRes.byteLength);
    const headerLen = dv.getUint32(0, true);
    const header = JSON.parse(new TextDecoder().decode(evalRes.subarray(4, 4 + headerLen)));
    const mesh = header.mesh;
    if (!mesh) return { stage: 'mesh', error: 'no mesh' };
    if (!mesh.colors) return { stage: 'colors', error: 'no colors blob in mesh', meshKeys: Object.keys(mesh) };

    const bin = evalRes.subarray(4 + headerLen);                       // data section
    const col = bin.slice(mesh.colors.offset, mesh.colors.offset + mesh.colors.size);
    const set = new Set();
    for (let i = 0; i + 2 < col.length; i += 3) {
      set.add((col[i] << 16) | (col[i + 1] << 8) | col[i + 2]);
    }
    return {
      stage: 'done',
      expandedCount: mesh.expandedCount,
      colorBytes: mesh.colors.size,
      distinct: set.size,
      hasRed: set.has(0xFF0000),       // bottom pyramid
      hasPurple: set.has(0x9933FF),    // right pyramid
      faceColorEntries: mesh.faceColors ? Object.keys(mesh.faceColors).length : 0,
    };
  });

  if (r.stage !== 'done') {
    throw new Error(`failed at ${r.stage}: ${r.error}${r.meshKeys ? ' keys=' + JSON.stringify(r.meshKeys) : ''}`);
  }
  if (r.colorBytes !== r.expandedCount * 3) {
    throw new Error(`color buffer ${r.colorBytes} bytes != expandedCount*3 (${r.expandedCount * 3})`);
  }
  if (r.distinct < 6) {
    throw new Error(`expected >= 6 distinct face colors for Color Cube, got ${r.distinct}`);
  }
  if (!r.hasRed)    throw new Error('expected red #FF0000 in the color buffer');
  if (!r.hasPurple) throw new Error('expected purple #9933FF in the color buffer');
  console.log(`  Color Cube: ${r.colorBytes} color bytes (${r.expandedCount} verts), ${r.distinct} distinct colors`);
}, chromium);
