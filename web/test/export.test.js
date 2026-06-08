// End-to-end test: facetExport serializes the evaluated model to downloadable
// STL and 3MF bytes through wasm. Verifies the binary STL structure (80-byte
// header + uint32 triangle count + 50 bytes/triangle) and the 3MF zip magic,
// plus that an unsupported format surfaces an error rather than bytes. Guards
// the export path, the manifold.EncodeSolidMesh bridge, and the response shape.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const SRC = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`;

runTest('export', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetParse === 'function' && typeof window.facetExport === 'function',
    null,
    { timeout: 60_000 },
  );

  const result = await page.evaluate(async (src) => {
    const parseRes = await window.facetParse(src);
    if (!parseRes?.ok) {
      return { stage: 'parse', error: parseRes?.error || 'parse failed' };
    }

    const stl = await window.facetExport(src, 'Main', '{}', 'stl', 0);
    if (!(stl instanceof Uint8Array)) {
      return { stage: 'stl', error: `expected Uint8Array, got ${JSON.stringify(stl)}` };
    }
    const stlView = new DataView(stl.buffer, stl.byteOffset, stl.byteLength);
    const triCount = stl.byteLength >= 84 ? stlView.getUint32(80, true) : 0;

    const tmf = await window.facetExport(src, 'Main', '{}', '3mf', 0);
    if (!(tmf instanceof Uint8Array)) {
      return { stage: '3mf', error: `expected Uint8Array, got ${JSON.stringify(tmf)}` };
    }
    const zipMagic = [...tmf.subarray(0, 4)];

    // An unsupported format must error, not return bytes.
    const bad = await window.facetExport(src, 'Main', '{}', 'gcode', 0);
    const badErrored = !(bad instanceof Uint8Array) && !!(bad && bad.error);

    return {
      stage: 'done',
      stlBytes: stl.byteLength,
      triCount,
      tmfBytes: tmf.byteLength,
      zipMagic,
      badErrored,
    };
  }, SRC);

  if (result.stage !== 'done') {
    throw new Error(`failed at ${result.stage}: ${result.error}`);
  }

  // Binary STL: 80-byte header + 4-byte count + 50 bytes per triangle.
  if (result.triCount === 0) {
    throw new Error('STL reported zero triangles');
  }
  const wantStl = 84 + 50 * result.triCount;
  if (result.stlBytes !== wantStl) {
    throw new Error(`STL length ${result.stlBytes} != 84 + 50*${result.triCount} = ${wantStl}`);
  }

  // 3MF is a zip container; it must start with the "PK\x03\x04" local-file magic.
  const PK = [0x50, 0x4b, 0x03, 0x04];
  if (result.zipMagic.join(',') !== PK.join(',')) {
    throw new Error(`3MF does not start with zip magic: [${result.zipMagic.join(', ')}]`);
  }

  if (!result.badErrored) {
    throw new Error('unsupported format did not surface an error');
  }

  console.log(
    `  STL ${result.stlBytes} bytes (${result.triCount} tris), ` +
    `3MF ${result.tmfBytes} bytes (zip magic OK), bad-format errored`,
  );
}, chromium);
