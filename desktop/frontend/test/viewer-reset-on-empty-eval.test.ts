import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression for: "close last tab → empty scratch tab → previous
// rendered mesh keeps showing in the now-empty context."
//
// When an eval lands check-only AND has no errors AND no entry points
// to run, there's nothing meaningful to render — the viewer should
// clear. Previously, viewer.reset() ran only on the "no tabs remaining"
// path; an auto-created scratch tab (or any other no-entry file) hit
// handleCheckOnly which left the previous mesh in place.

test('viewer resets when an eval has no mesh, no errors, no entry points', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // Two-phase setup: first eval returns a real mesh so the viewer has
  // something to clear; second eval returns the empty-scratch shape.
  let phase: 'mesh' | 'empty' = 'mesh';

  const vertices = new Float32Array([0, 0, 0, 10, 0, 0, 0, 10, 0]);
  const indices = new Uint32Array([0, 1, 2]);
  const meshBinary = Buffer.concat([
    Buffer.from(vertices.buffer, vertices.byteOffset, vertices.byteLength),
    Buffer.from(indices.buffer, indices.byteOffset, indices.byteLength),
  ]);

  await setEvalHandler(() => {
    if (phase === 'mesh') {
      return {
        header: {
          errors: [],
          entryPoints: [
            { name: 'Main', signature: 'Main() Solid', params: [], libPath: '', libVar: '', doc: '' },
          ],
          symbols: [],
          posMap: [],
          mesh: {
            vertexCount: 3,
            indexCount: 3,
            faceGroupCount: 0,
            vertices: { offset: 0, size: vertices.byteLength },
            indices: { offset: vertices.byteLength, size: indices.byteLength },
          },
          stats: {
            triangles: 1,
            vertices: 3,
            volume: 0,
            surfaceArea: 0,
            bboxMin: [0, 0, 0],
            bboxMax: [10, 10, 0],
          },
          time: 0.01,
        },
        binary: meshBinary,
      };
    }
    // Empty-scratch shape: check-only, no errors, no entry points.
    return {
      errors: [],
      entryPoints: [],
      symbols: [],
      posMap: [],
    };
  });

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // First eval: rendered mesh.
  const meshLanded = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn Main() Solid { return Cube(s: 10 mm) }\n');
  await meshLanded;
  await expect.poll(
    () => page.evaluate(() => (window as unknown as { viewer?: { meshCount: () => number } }).viewer?.meshCount() ?? -1),
    { timeout: 3_000 },
  ).toBeGreaterThan(0);

  // Flip the mock to the empty shape and trigger a fresh eval. Editing
  // text triggers autoRun via the editor's onChange.
  phase = 'empty';
  const emptyLanded = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, '// no entry function here\n');
  await emptyLanded;

  // After handleCheckOnly's reset, the mesh count drops to zero. Poll
  // because the reset happens after the response is parsed.
  await expect.poll(
    () => page.evaluate(() => (window as unknown as { viewer?: { meshCount: () => number } }).viewer?.meshCount() ?? -1),
    { timeout: 3_000 },
  ).toBe(0);
});
