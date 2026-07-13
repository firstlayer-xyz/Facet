import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression for: exiting debug left the canvas blank for a MULTI-solid model.
// The debug "final mesh" was stored as a single mesh (null when there were
// several), so multi-solid models were dropped both while debugging and on exit.
test('exiting debug restores a multi-solid model to the canvas', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  const triBytes = (ox: number) => {
    const v = new Float32Array([ox, 0, 0, ox + 10, 0, 0, ox, 10, 0]);
    const i = new Uint32Array([0, 1, 2]);
    return {
      v: Buffer.from(v.buffer, v.byteOffset, v.byteLength),
      i: Buffer.from(i.buffer, i.byteOffset, i.byteLength),
    };
  };
  const m1 = triBytes(0);
  const m2 = triBytes(40);
  const VS = m1.v.length; // 36
  const IS = m1.i.length; // 12
  const twoMeshBinary = Buffer.concat([m1.v, m1.i, m2.v, m2.i]);
  const oneMeshBinary = Buffer.concat([m1.v, m1.i]);
  const meshMeta = (voff: number) => ({
    vertexCount: 3, indexCount: 3, faceGroupCount: 0,
    vertices: { offset: voff, size: VS },
    indices: { offset: voff + VS, size: IS },
  });
  const mainEntry = (key: string) => ({ name: 'Main', signature: 'Main() []Solid', libPath: key, libVar: '', doc: '', params: [] });

  await setEvalHandler((body) => {
    if (body.debug) {
      // Debug eval: two final meshes (multi-solid), routed via debugSteps.
      return {
        header: {
          errors: [], entryPoints: [mainEntry(body.key)], symbols: [], posMap: [],
          debugSteps: [], debugFinal: [meshMeta(0), meshMeta(VS + IS)],
        },
        binary: twoMeshBinary,
      };
    }
    // Normal eval: a single mesh + the Main entry so it's picked and runnable.
    return {
      header: {
        errors: [], entryPoints: [mainEntry(body.key)], symbols: [], posMap: [],
        mesh: meshMeta(0),
        stats: { triangles: 1, vertices: 3, volume: 0, surfaceArea: 0, bboxMin: [0, 0, 0], bboxMax: [10, 10, 0] },
      },
      binary: oneMeshBinary,
    };
  });

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({ timeout: 10_000 });

  // Normal render first (picks Main, shows the model).
  const firstEval = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn Main() []Solid { return [Cube(s: 10 mm)] }\n');
  await firstEval;
  await expect.poll(() => page.evaluate(() => (window as any).viewer?.meshCount?.() ?? -1), { timeout: 3_000 }).toBeGreaterThan(0);

  // Enter debug — this runs a debug eval returning two final meshes.
  const debugEval = page.waitForResponse(r => r.url().endsWith('/eval') && r.request().postDataJSON()?.debug === true);
  await page.locator('#debug-btn').click();
  await debugEval;
  await expect.poll(() => page.evaluate(() => (window as any).viewer?.meshCount?.() ?? -1), { timeout: 3_000 }).toBeGreaterThan(0);

  // Exit debug via the debug strip's Stop button (#debug-btn is hidden in debug
  // mode). The multi-solid model must come back, not vanish.
  await page.locator('.dbg-ctrl-btn.stop').click();
  await expect.poll(() => page.evaluate(() => (window as any).viewer?.meshCount?.() ?? -1), { timeout: 3_000 }).toBeGreaterThan(0);
});
