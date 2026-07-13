import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression for: zooming (dollying) the perspective camera far out clipped the
// model against the fixed far=1000 plane — an "invisible back wall". The far/near
// planes must now track the camera distance so the model stays inside the frustum
// at any zoom.
test('perspective clip planes bracket the model when dollied far out', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  const vertices = new Float32Array([0, 0, 0, 10, 0, 0, 0, 10, 0]);
  const indices = new Uint32Array([0, 1, 2]);
  const meshBinary = Buffer.concat([
    Buffer.from(vertices.buffer, vertices.byteOffset, vertices.byteLength),
    Buffer.from(indices.buffer, indices.byteOffset, indices.byteLength),
  ]);
  await setEvalHandler(() => ({
    header: {
      errors: [],
      entryPoints: [{ name: 'Main', signature: 'Main() Solid', params: [], libPath: '', libVar: '', doc: '' }],
      symbols: [],
      posMap: [],
      mesh: {
        vertexCount: 3,
        indexCount: 3,
        faceGroupCount: 0,
        vertices: { offset: 0, size: vertices.byteLength },
        indices: { offset: vertices.byteLength, size: indices.byteLength },
      },
      stats: { triangles: 1, vertices: 3, volume: 0, surfaceArea: 0, bboxMin: [0, 0, 0], bboxMax: [10, 10, 0] },
    },
    binary: meshBinary,
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({ timeout: 10_000 });

  const meshLanded = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn Main() Solid { return Cube(s: 10 mm) }\n');
  await meshLanded;
  await expect
    .poll(() => page.evaluate(() => (window as any).viewer?.meshCount?.() ?? -1), { timeout: 3_000 })
    .toBeGreaterThan(0);

  const r = await page.evaluate(() => {
    const v = (window as any).viewer;
    const s = v._getMeshBoundingSphere();
    const cam = v.perspCamera;
    const dist = 5000; // dolly well past the old fixed far=1000
    cam.position.set(s.center.x, s.center.y, s.center.z + dist);
    v._updatePerspectiveClip();
    return { far: cam.far, near: cam.near, dist: cam.position.distanceTo(s.center), radius: s.radius };
  });

  expect(r.radius).toBeGreaterThan(0);
  // The model's far face (dist + radius) sits inside the far plane; its near face
  // (dist - radius) outside the near plane. The old fixed far=1000 would fail here.
  expect(r.far).toBeGreaterThan(r.dist + r.radius);
  expect(r.near).toBeLessThan(r.dist - r.radius);
  expect(r.near).toBeGreaterThan(0);
});
