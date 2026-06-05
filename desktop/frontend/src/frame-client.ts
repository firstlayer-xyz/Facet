// frame-client.ts — HTTP client for the /frame endpoint.
// Mirrors eval-client.ts exactly (same auth, same binary framing),
// but targets /frame and tracks one outstanding request to support
// frame-dropping: callers check frameInFlight() before issuing a new
// request to avoid building an unbounded queue at the server.

import { getEvalAuth } from './eval-client';
import type { EvalResult } from './eval-client';

export type { EvalResult };

export interface FrameRequest {
  sources: Record<string, string>;
  key: string;
  entry: string;
  overrides?: Record<string, unknown>;
  timeMs: number;
}

export interface FrameResponse {
  header: EvalResult;
  binary: ArrayBuffer;
}

let frameController: AbortController | null = null;

/** True while a /frame request is in flight. */
export function frameInFlight(): boolean {
  return frameController !== null;
}

/** Abort the in-flight /frame request, if any. The pending frameRequest
 *  promise rejects with an AbortError, which the caller treats as a no-op. */
export function cancelFrame(): void {
  frameController?.abort();
}

/** Send a frame request. Only one may be in-flight at a time; the
 *  caller is responsible for checking frameInFlight() before calling. */
export async function frameRequest(req: FrameRequest): Promise<FrameResponse> {
  frameController = new AbortController();
  try {
    const auth = await getEvalAuth();
    const resp = await fetch(`${auth.origin}/frame`, {
      method: 'POST',
      body: JSON.stringify(req),
      signal: frameController.signal,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${auth.token}`,
      },
    });

    if (!resp.ok) {
      const body = await resp.text();
      throw new Error(`frame HTTP ${resp.status}: ${body.trim()}`);
    }

    const buf = await resp.arrayBuffer();
    const view = new DataView(buf);
    const headerLen = view.getUint32(0, true);
    const headerJSON = new TextDecoder().decode(new Uint8Array(buf, 4, headerLen));
    const header = JSON.parse(headerJSON) as EvalResult;
    const binary = buf.slice(4 + headerLen);
    return { header, binary };
  } finally {
    frameController = null;
  }
}
