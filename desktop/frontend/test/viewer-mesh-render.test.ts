import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression test for: the model isn't loading in the preview window.
//
// None of the prior tests exercised the mesh-render path — they all
// mocked /eval with `eval-cube.json` which has no real mesh bytes.
// That left the entire decode → loadDecodedMesh → scene-attach pipeline
// uncovered. A regression anywhere in it (viewer.applyEvalResult,
// decodeBinaryMesh, setViewOffset, ResizeObserver wiring, the binary
// wire format) would render an empty viewport with no error and pass
// every other test in the suite.
//
// This test injects a minimal one-triangle mesh through the same
// binary protocol the Go backend produces and asserts the viewer
// actually attached a mesh to the scene after the eval response lands.

test('viewer attaches a mesh to the scene after /eval returns mesh bytes', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  // One triangle in world space. Vertex/index layout matches what
  // the Go side emits in eval_handler.appendMeshBinary:
  //   [vertices float32[]] [indices uint32[]]
  // Float32Array(9) = 36 bytes; Uint32Array(3) = 12 bytes.
  const vertices = new Float32Array([
    0, 0, 0,
    10, 0, 0,
    0, 10, 0,
  ]);
  const indices = new Uint32Array([0, 1, 2]);
  const binary = Buffer.concat([
    Buffer.from(vertices.buffer, vertices.byteOffset, vertices.byteLength),
    Buffer.from(indices.buffer, indices.byteOffset, indices.byteLength),
  ]);

  await setEvalHandler(() => ({
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
    binary,
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  const evalDone = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn Main() Solid { return Cube(s: 10 mm) }\n');
  await evalDone;

  // The frontend handles the response synchronously after fetch
  // resolves, but a render frame may need to flush before
  // applyEvalResult attaches the mesh. Poll the meshCount instead
  // of asserting once.
  await expect.poll(
    () => page.evaluate(() => {
      const v = (window as unknown as { viewer?: { meshCount: () => number } }).viewer;
      return v ? v.meshCount() : -1;
    }),
    { timeout: 3_000 },
  ).toBeGreaterThan(0);
});
