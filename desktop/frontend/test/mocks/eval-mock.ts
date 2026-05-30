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

export type EvalHandler = (body: EvalRequestBody) => unknown;

export function loadEvalFixture(name: string): unknown {
  const raw = fs.readFileSync(path.join(FIXTURE_DIR, `${name}.json`), 'utf8');
  return JSON.parse(raw).value;
}

// Frames a JS object into the [4-byte LE length][JSON UTF-8] wire format the
// app's eval-client expects. Matches the parsing at eval-client.ts:136-138.
function frameResponse(obj: unknown): Buffer {
  const headerJSON = JSON.stringify(obj);
  const headerBytes = new TextEncoder().encode(headerJSON);
  const buf = new ArrayBuffer(4 + headerBytes.length);
  const view = new DataView(buf);
  view.setUint32(0, headerBytes.length, true);
  new Uint8Array(buf, 4).set(headerBytes);
  return Buffer.from(buf);
}

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
    const result = handler(body);
    await route.fulfill({
      status: 200,
      contentType: 'application/octet-stream',
      body: frameResponse(result),
    });
  });
}
