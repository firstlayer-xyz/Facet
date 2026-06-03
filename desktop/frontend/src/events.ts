// Typed wrapper over Wails' EventsOn. Replaces every direct
// `EventsOn('some-string', ...)` call site with `on('some-string',
// ...)`, where the listener callback signature is constrained by
// FacetEventMap. Renaming or removing an event becomes a compile
// error at every listener instead of a silent runtime miss — the
// previous stringly-typed model let typos and stale handlers drift
// undetected.
//
// Frontend only listens; the Go backend is the sole emitter via
// wailsRuntime.EventsEmit. Add new events here as the backend
// introduces them. Each tuple matches Wails' positional argument
// delivery — `EventsEmit(ctx, "name", a, b)` lands as `(a, b) => …`.

import { EventsOn } from '../wailsjs/runtime/runtime';

export interface AssistantQuestionOption {
  label: string;
  description?: string;
}

export interface AssistantQuestion {
  question: string;
  header: string;
  options: AssistantQuestionOption[];
  multiSelect?: boolean;
}

export interface AssistantQuestionPayload {
  id: string;
  questions: AssistantQuestion[];
}

export interface AssistantScreenshotRequest {
  id: string;
  azimuth?: number;
  elevation?: number;
  distance?: number;
  target?: { x: number; y: number; z: number };
}

export interface AssistantPermissionRequest {
  id: string;
  toolName: string;
  summary: string;
}

export type AssistantTaskStatus = 'pending' | 'in_progress' | 'completed';

export interface AssistantTaskItem {
  content: string;
  status: AssistantTaskStatus;
}

export interface AssistantTaskPlanPayload {
  tasks: AssistantTaskItem[];
}

export interface AssistantNewFilePayload {
  name: string;
  code: string;
}

/**
 * The single source of truth for every Wails event the frontend
 * listens to. Tuple shape mirrors the positional args Wails emits.
 * Empty tuple = event carries no payload (notification only).
 */
export interface FacetEventMap {
  // App lifecycle
  'app:before-close': [];

  // Log forwarding
  'log:stderr': [line: string];

  // Assistant streaming + tools
  'assistant:token': [token: string];
  'assistant:done': [];
  'assistant:error': [msg: string];
  'assistant:tool-use': [toolName: string, callNum: number];
  'assistant:thinking': [callNum: number];
  'assistant:question': [payload: AssistantQuestionPayload];
  'assistant:screenshot-request': [payload: AssistantScreenshotRequest];
  'assistant:permission-request': [payload: AssistantPermissionRequest];
  'assistant:task-plan': [payload: AssistantTaskPlanPayload];
  'assistant:replace-code': [code: string];
  'assistant:new-file': [payload: AssistantNewFilePayload];

  // Native menu actions
  'menu:new': [];
  'menu:open': [];
  'menu:open-recent': [path: string];
  'menu:open-demo': [name: string];
  'menu:open-library': [dir: string];
  'menu:new-library': [];
  'menu:close-tab': [];
  'menu:save': [];
  'menu:save-as': [];
  'menu:export': [format: string];
  'menu:run': [];
  'menu:debug': [];
  'menu:fullcode': [];
  'menu:toggle-grid': [];
  'menu:toggle-axes': [];
  'menu:docs': [];
  'menu:assistant': [];
  'menu:slicer': [];
  'menu:slicer-id': [id: string];
  'menu:settings': [];
}

export type FacetEventName = keyof FacetEventMap;

/**
 * Register a listener for a Wails event. Returns the unsubscribe
 * function Wails hands back. The generic constrains the callback's
 * arguments to whatever FacetEventMap declares for that name —
 * renaming an event in one place without updating the other now
 * fails to compile.
 */
export function on<K extends FacetEventName>(
  name: K,
  cb: (...args: FacetEventMap[K]) => void,
): () => void {
  return EventsOn(name, cb as (...args: unknown[]) => void);
}
