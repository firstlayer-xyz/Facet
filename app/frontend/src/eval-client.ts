// eval-client.ts — HTTP client for the /eval endpoint with AbortController cancellation.

import { GetHTTPAuth } from '../wailsjs/go/main/App';
import type { EntryPoint } from './function-preview';
import type { DocEntry } from './docs';
import type { BinaryMeshMeta, DebugStepData } from './mesh-decode';
import type { PosEntry } from './viewer';

let currentController: AbortController | null = null;

interface EvalAuth {
  url: string;
  token: string;
}

let cachedAuth: EvalAuth | null = null;

async function getEvalAuth(): Promise<EvalAuth> {
  if (!cachedAuth) {
    const auth = await GetHTTPAuth();
    cachedAuth = {
      url: `http://127.0.0.1:${auth.port}/eval`,
      token: auth.token,
    };
  }
  return cachedAuth;
}

export interface EvalRequest {
  sources: Record<string, string>;
  key: string;
  entry?: string;
  overrides?: Record<string, any>;
  debug?: boolean;
}

/** A source-level error with location (mirrors parser.SourceError on the Go side). */
export interface SourceError {
  file: string;
  line: number;
  col: number;
  endCol: number;
  message: string;
  /** Library source text (for error navigation). */
  source?: string;
}

/** One loaded source file in the eval response (mirrors main.SourceEntry on the Go side). */
export interface SourceEntry {
  text: string;
  kind: number;
  importPath?: string;
}

/** Location of a declaration for "Go to Definition" (mirrors checker.DeclLocation on the Go side). */
export interface DeclLocation {
  line: number;
  col: number;
  /** Empty = main/current file; set for library declarations. */
  file?: string;
  /** Declaration kind: "fn", "type", "const", "var", "param", "field". */
  kind?: string;
  /** Declared return type name (for functions) or parameter/field type. */
  returnType?: string;
}

export interface Declarations {
  decls: Record<string, DeclLocation>;
}

/** Model statistics (mirrors evaluator.ModelStats on the Go side). */
export interface ModelStats {
  triangles: number;
  vertices: number;
  volume: number;
  surfaceArea: number;
  bboxMin: [number, number, number];
  bboxMax: [number, number, number];
}

/** Parsed eval response header (mirrors main.evalResponseHeader on the Go side). */
export interface EvalResult {
  // Check data
  errors?: SourceError[];
  sources?: Record<string, SourceEntry>;
  varTypes?: Record<string, Record<string, string>>;
  declarations?: Declarations;
  /** References map: "file:line:col" of a referring token → declaration location. */
  references?: Record<string, DeclLocation>;
  entryPoints?: EntryPoint[];
  docIndex?: DocEntry[];

  // Eval data
  mesh?: BinaryMeshMeta;
  stats?: ModelStats;
  time?: number;
  posMap?: PosEntry[];

  // Debug data
  debugFinal?: BinaryMeshMeta[];
  debugSteps?: DebugStepData[];
}

export interface EvalResponse {
  header: EvalResult;
  binary: ArrayBuffer;
}

/** Send an eval request, cancelling any in-flight request. */
export async function evalRequest(req: EvalRequest): Promise<EvalResponse> {
  if (currentController) currentController.abort();
  currentController = new AbortController();

  const auth = await getEvalAuth();
  const resp = await fetch(auth.url, {
    method: 'POST',
    body: JSON.stringify(req),
    signal: currentController.signal,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${auth.token}`,
    },
  });

  // An error response (401, 500, …) is text, not our binary framing.
  // Parsing the first 4 bytes as a header length yields garbage and
  // confuses the caller into thinking the eval "succeeded" with an
  // unreadable payload.  Surface the status + body instead.
  if (!resp.ok) {
    const body = await resp.text();
    throw new Error(`eval HTTP ${resp.status}: ${body.trim()}`);
  }

  const buf = await resp.arrayBuffer();
  const view = new DataView(buf);
  const headerLen = view.getUint32(0, true);
  const headerJSON = new TextDecoder().decode(new Uint8Array(buf, 4, headerLen));
  const header = JSON.parse(headerJSON) as EvalResult;
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
