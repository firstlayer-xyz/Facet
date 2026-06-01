import { Page } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';

const FIXTURE_DIR = path.join(__dirname, 'fixtures');

export type EvalRequestBody = {
  sources: Record<string, string>;
  key: string;
  entry?: string;
  overrides?: Record<string, unknown>;
  debug?: boolean;
};

/**
 * Eval mock handler return shape. Plain object = header-only response
 * (the common case for tests that only need the metadata). To inject
 * a binary mesh payload — anything that the frontend's mesh-decode
 * machinery needs to do real work — return `{ header, binary }`. The
 * binary is appended after the framed JSON header, matching the
 * wire format the eval-client expects.
 */
export type EvalHandlerResult =
  | Record<string, unknown>
  | { header: Record<string, unknown>; binary: Buffer | Uint8Array };

export type EvalHandler = (body: EvalRequestBody) => EvalHandlerResult;

export function loadFixture(name: string): Record<string, unknown> {
  const raw = fs.readFileSync(path.join(FIXTURE_DIR, `${name}.json`), 'utf8');
  return JSON.parse(raw).value as Record<string, unknown>;
}

// Frames a JS object plus optional binary into the
// [4-byte LE length][JSON UTF-8][binary] wire format the eval-client
// expects. Matches the parsing at eval-client.ts:136-138.
function frameResponse(header: unknown, binary?: Buffer | Uint8Array): Buffer {
  const headerJSON = JSON.stringify(header);
  const headerBytes = new TextEncoder().encode(headerJSON);
  const binLen = binary ? binary.length : 0;
  const buf = new ArrayBuffer(4 + headerBytes.length + binLen);
  const view = new DataView(buf);
  view.setUint32(0, headerBytes.length, true);
  const u8 = new Uint8Array(buf);
  u8.set(headerBytes, 4);
  if (binary) u8.set(binary, 4 + headerBytes.length);
  return Buffer.from(buf);
}

// Zero-vertex mesh stub. The frontend treats a response without `mesh` or
// `debugFinal` as "check-only" and immediately re-runs with the picked entry —
// real backend then returns a mesh, terminating the cycle. Mocks that omit
// mesh otherwise spin forever on full-eval requests (those with body.entry
// set). Injecting this stub when the handler omits mesh on a full-eval
// request mirrors backend semantics: "evaluator ran, produced no geometry".
// Viewer.loadDecodedMesh short-circuits on empty vertices with a console.warn.
const EMPTY_MESH_META = {
  vertexCount: 0,
  indexCount: 0,
  faceGroupCount: 0,
  vertices: { offset: 0, size: 0 },
  indices: { offset: 0, size: 0 },
};

// Routes any POST to a URL containing /eval through `handler`. The handler
// receives the parsed request body and returns the EvalResult to send back.
export async function installEvalRoute(page: Page, handler: EvalHandler): Promise<void> {
  await page.route('**/eval', async route => {
    const req = route.request();
    if (req.method() !== 'POST') {
      await route.continue();
      return;
    }
    const body = req.postDataJSON() as EvalRequestBody;
    const raw = handler(body);
    // Distinguish framed (header + binary) from header-only by the
    // presence of a `header` key. Tests that need to inject mesh
    // bytes return { header, binary }; everything else returns the
    // header directly.
    let header: Record<string, unknown>;
    let binary: Buffer | Uint8Array | undefined;
    if (raw && typeof raw === 'object' && 'header' in raw && 'binary' in raw) {
      header = (raw as { header: Record<string, unknown> }).header;
      binary = (raw as { binary: Buffer | Uint8Array }).binary;
    } else {
      header = raw as Record<string, unknown>;
    }
    if (body.entry && !header.mesh && !header.debugFinal) {
      header.mesh = EMPTY_MESH_META;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/octet-stream',
      body: frameResponse(header, binary),
    });
  });
}
