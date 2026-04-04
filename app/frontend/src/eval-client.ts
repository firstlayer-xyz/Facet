// eval-client.ts — HTTP client for the /eval endpoint with AbortController cancellation.

import { GetHTTPPort } from '../wailsjs/go/main/App';

let currentController: AbortController | null = null;
let evalBaseURL: string | null = null;

async function getEvalURL(): Promise<string> {
  if (!evalBaseURL) {
    const port = await GetHTTPPort();
    evalBaseURL = `http://127.0.0.1:${port}/eval`;
  }
  return evalBaseURL;
}

export interface EvalRequest {
  sources: Record<string, string>;
  key: string;
  entry?: string;
  overrides?: Record<string, any>;
  debug?: boolean;
}

export interface EvalResponse {
  header: any;
  binary: ArrayBuffer;
}

/** Send an eval request, cancelling any in-flight request. */
export async function evalRequest(req: EvalRequest): Promise<EvalResponse> {
  if (currentController) currentController.abort();
  currentController = new AbortController();

  const url = await getEvalURL();
  const resp = await fetch(url, {
    method: 'POST',
    body: JSON.stringify(req),
    signal: currentController.signal,
    headers: { 'Content-Type': 'application/json' },
  });

  const buf = await resp.arrayBuffer();
  const view = new DataView(buf);
  const headerLen = view.getUint32(0, true);
  const headerJSON = new TextDecoder().decode(new Uint8Array(buf, 4, headerLen));
  const header = JSON.parse(headerJSON);
  const binary = buf.slice(4 + headerLen);
  return { header, binary };
}

/** Cancel the current in-flight eval request. */
export function cancelEval(): void {
  if (currentController) {
    currentController.abort();
    currentController = null;
  }
}
