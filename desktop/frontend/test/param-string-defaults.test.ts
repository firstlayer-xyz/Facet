import { test, expect } from './harness';
import { setEditorText } from './helpers/editor';

// Regression for: String parameter defaults not showing in the preview panel.
// A String param with no `where` was filtered out entirely, and a computed
// default (UtcDate(), the Moon example) had no literal value so its input was
// blank. Now: plain strings get a text input, and a computed default shows its
// source (from the backend's defaultSource) as a placeholder.
test('string param defaults render: literal value and computed placeholder', async ({
  mockedPage: page,
  setEvalHandler,
}) => {
  const vertices = new Float32Array([0, 0, 0, 10, 0, 0, 0, 10, 0]);
  const indices = new Uint32Array([0, 1, 2]);
  const meshBinary = Buffer.concat([
    Buffer.from(vertices.buffer, vertices.byteOffset, vertices.byteLength),
    Buffer.from(indices.buffer, indices.byteOffset, indices.byteLength),
  ]);
  await setEvalHandler((body) => ({
    header: {
      errors: [],
      entryPoints: [
        {
          name: 'Main',
          signature: 'Main() Solid',
          libPath: body.key, // the preview panel filters entry points by libPath === active tab
          libVar: '',
          doc: '',
          params: [
            // no `where` — previously filtered out; must now show its literal value
            { name: 'nowhere', type: 'String', hasDefault: true, default: 'world' },
            // computed default — no literal value, source carried for the placeholder
            { name: 'computed', type: 'String', hasDefault: true, default: null, defaultSource: 'UtcDate()', constraint: { kind: 'free' } },
          ],
        },
      ],
      symbols: [],
      posMap: [],
      mesh: {
        vertexCount: 3, indexCount: 3, faceGroupCount: 0,
        vertices: { offset: 0, size: vertices.byteLength },
        indices: { offset: vertices.byteLength, size: indices.byteLength },
      },
      stats: { triangles: 1, vertices: 3, volume: 0, surfaceArea: 0, bboxMin: [0, 0, 0], bboxMax: [10, 10, 0] },
    },
    binary: meshBinary,
  }));

  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({ timeout: 10_000 });

  const evalLanded = page.waitForResponse(r => r.url().endsWith('/eval') && r.ok());
  await setEditorText(page, 'fn Main() Solid { return Cube(s: 10 mm) }\n');
  await evalLanded;

  // The preview panel should now hold two text inputs, one per string param.
  const inputs = page.locator('#fn-preview-bar .fn-preview-input');
  await expect(inputs).toHaveCount(2, { timeout: 3_000 });

  // Read them back as {value, placeholder}.
  const state = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll('#fn-preview-bar .fn-preview-input')) as HTMLInputElement[];
    return els.map(e => ({ value: e.value, placeholder: e.placeholder }));
  });

  // nowhere: literal default shown as the value; no placeholder.
  expect(state).toContainEqual({ value: 'world', placeholder: '' });
  // computed: empty value, source shown as placeholder.
  expect(state).toContainEqual({ value: '', placeholder: 'UtcDate()' });
});
