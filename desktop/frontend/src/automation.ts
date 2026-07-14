// Remote GUI automation. Listens for automation:invoke events (emitted by the
// Go AutomationController behind --automation), dispatches to a named command
// registry, and acks the result back via App.AutomationResult. Each command
// resolves only when its GUI effect has actually completed, so external driver
// scripts need no sleeps.
//
// The registry is the single vocabulary shared by both front doors: the
// /control HTTP route and the gui_* MCP tools both drive these same commands.

import {
  AutomationResult,
  SaveRecording,
  SaveImage,
  StartWindowCapture,
  StopWindowCapture,
  CaptureWindowImage,
} from '../wailsjs/go/main/App';
import { WindowSetSize } from '../wailsjs/runtime/runtime';
import { on, type AutomationInvokePayload } from './events';
import type { Viewer } from './viewer';
import { evalStore } from './eval-store';
import { setPlaying } from './playback';
import { UI_THEMES } from './themes';
import { Recorder, blobToDataURL } from './recorder';

export type DarkMode = 'light' | 'dark' | 'auto';

type RecordMode = 'canvas' | 'page';

/** The editor operations automation drives — a subset of EditorHandle plus a
 *  build trigger. Kept as an interface so automation.ts stays decoupled from
 *  Monaco (main.ts wires these to the real editor). */
export interface EditorControl {
  insertAtCursor(text: string): void;
  moveCursorAfter(find: string): boolean;
  selectRange(find: string): boolean;
  deleteSelection(): void;
  setContentSilent(text: string): void;
  getContent(): string;
  /** Trigger a build of the current editor source. */
  build(): void;
}

export interface AutomationDeps {
  viewer: Viewer;
  editor: EditorControl;
  /** Drive the auto-rotate Toggle (keeps the toolbar button in sync too). */
  setAutoRotate: (on: boolean) => void;
  /** Apply a UI theme (and optional light/dark mode) live. */
  setTheme: (name: string, dark?: DarkMode) => void;
  /** Set an entry-point parameter; false if the current entry lacks it. */
  setParam: (name: string, value: number | boolean | string) => boolean;
  /** Drive the AI assistant drawer. */
  assistant: {
    open: () => void;
    send: (prompt: string) => void;
    isStreaming: () => boolean;
  };
  /** Show/hide the code editor panel (e.g. hidden for the AI demo). */
  setCodeVisible: (visible: boolean) => void;
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

type CommandFn = (params: any) => Promise<unknown>;

const registry = new Map<string, CommandFn>();

/** Register a command. Grows as demos need more of the GUI driven. */
export function registerCommand(name: string, run: CommandFn): void {
  if (registry.has(name)) throw new Error(`automation: duplicate command ${name}`);
  registry.set(name, run);
}

export function initAutomation(deps: AutomationDeps): void {
  registerWindowCommands();
  registerUICommands(deps.setCodeVisible);
  registerThemeCommands(deps.setTheme);
  registerParamCommands(deps.setParam);
  registerAssistantCommands(deps.assistant);
  registerEditorCommands(deps.editor);
  registerViewerCommands(deps.viewer, deps.setAutoRotate);
  registerAnimationCommands();
  registerScreenshotCommands(deps.viewer);
  registerRecordCommands(deps.viewer.getCanvas());

  on('automation:invoke', async (payload: AutomationInvokePayload) => {
    const cmd = registry.get(payload.name);
    if (!cmd) {
      await AutomationResult(payload.id, '', `unknown command: ${payload.name}`);
      return;
    }
    try {
      const value = await cmd((payload.params ?? {}) as any);
      await AutomationResult(payload.id, JSON.stringify(value ?? null), '');
    } catch (e) {
      await AutomationResult(payload.id, '', e instanceof Error ? e.message : String(e));
    }
  });
}

function registerUICommands(setCodeVisible: (visible: boolean) => void): void {
  // Click any UI element by CSS selector (e.g. "#assistant-btn", "#export-btn").
  // Routes through the element's own click handler, so it drives real behavior
  // (open the assistant drawer, export, share, toggle a panel, …). Errors if the
  // selector matches nothing — no silent no-op.
  registerCommand('ui.click', async (p) => {
    const selector = String(p.selector ?? '');
    if (!selector) throw new Error('ui.click: missing selector');
    const el = document.querySelector<HTMLElement>(selector);
    if (!el) throw new Error(`ui.click: no element for selector "${selector}"`);
    el.click();
    return null;
  });

  // Show/hide a named panel idempotently (unlike ui.click, which toggles). Only
  // "code" today — hide the editor to focus a demo on the assistant + viewer.
  registerCommand('ui.setPanel', async (p) => {
    const panel = String(p.panel ?? '');
    const visible = p.visible !== false;
    if (panel === 'code') setCodeVisible(visible);
    else throw new Error(`ui.setPanel: unknown panel "${panel}" (supported: code)`);
    return null;
  });
}

function registerAssistantCommands(assistant: { open: () => void; send: (prompt: string) => void; isStreaming: () => boolean }): void {
  // Open the AI assistant drawer.
  registerCommand('assistant.open', async () => {
    assistant.open();
    return null;
  });
  // Open the drawer (if needed) and submit a prompt — the assistant streams a
  // reply and writes code. Fire-and-return; poll assistant.status to wait for
  // the round to finish (responses can far exceed the per-command timeout).
  registerCommand('assistant.send', async (p) => {
    const prompt = String(p.prompt ?? p.message ?? '');
    if (!prompt) throw new Error('assistant.send: missing prompt');
    assistant.send(prompt);
    return null;
  });
  // Report whether the assistant is mid-response — drivers poll this to do
  // multi-round conversations, waiting for the agent between prompts.
  registerCommand('assistant.status', async () => ({ streaming: assistant.isStreaming() }));
}

function registerParamCommands(setParam: (name: string, value: number | boolean | string) => boolean): void {
  // Set an entry-point parameter (slider/toggle/enum). With durationMs, ramps
  // from the current value to `value` for a visible slider-drag effect.
  registerCommand('params.set', async (p) => {
    const name = String(p.name ?? '');
    const value = p.value;
    const duration = Number(p.durationMs);
    if (typeof value === 'number' && duration > 0) {
      const start = Number(p.from);
      const from = Number.isFinite(start) ? start : value;
      const t0 = performance.now();
      await new Promise<void>((resolve, reject) => {
        const tick = () => {
          const t = Math.min(1, (performance.now() - t0) / duration);
          const v = from + (value - from) * t;
          if (!setParam(name, v)) {
            reject(new Error(`params.set: no parameter "${name}"`));
            return;
          }
          if (t < 1) requestAnimationFrame(tick);
          else resolve();
        };
        requestAnimationFrame(tick);
      });
    } else if (!setParam(name, value)) {
      throw new Error(`params.set: no parameter "${name}"`);
    }
    return null;
  });
}

function registerThemeCommands(setTheme: (name: string, dark?: DarkMode) => void): void {
  // Apply a UI theme by id (e.g. "dracula", "nord", "facet-orange"), optionally
  // with a light/dark mode. Errors on an unknown theme — no silent fallback.
  registerCommand('theme.set', async (p) => {
    const name = String(p.name ?? '');
    const known = UI_THEMES.some((t) => t.id === name) || name.startsWith('custom-');
    if (!known) {
      throw new Error(`theme.set: unknown theme "${name}" (options: ${UI_THEMES.map((t) => t.id).join(', ')})`);
    }
    const dark: DarkMode | undefined =
      p.dark === 'light' || p.dark === 'dark' || p.dark === 'auto' ? p.dark : undefined;
    setTheme(name, dark);
    return null;
  });
}

function registerScreenshotCommands(viewer: Viewer): void {
  // Capture a still PNG. 'page' (default) grabs the whole window natively;
  // 'canvas' grabs just the 3D viewer. Returns the saved path.
  registerCommand('screenshot', async (p) => {
    const name = typeof p.name === 'string' ? p.name : '';
    let path: string;
    if (p.mode === 'canvas') {
      path = await SaveImage(viewer.captureScreenshot(), name);
    } else {
      path = await CaptureWindowImage(name);
    }
    return { path };
  });
}

function registerWindowCommands(): void {
  // Resize the app window (points). This lays out the whole UI at the target
  // size, so both the WebGL canvas (canvas recording) and the captured window
  // (page recording) follow — the clean way to fix a demo's frame.
  registerCommand('window.setSize', async (p) => {
    const w = Math.round(Number(p.width));
    const h = Math.round(Number(p.height));
    if (!(w > 0 && h > 0)) throw new Error('window.setSize needs positive width and height');
    WindowSetSize(w, h);
    return null;
  });
}

function registerEditorCommands(editor: EditorControl): void {
  // Build the current editor source and resolve once it has rendered (the next
  // evalStore update). Typing suppresses auto-run, so nothing is in flight and
  // the update we wake on is this build's — demo scripts need no sleeps.
  const buildAndWait = () =>
    new Promise<void>((resolve) => {
      const unsub = evalStore.subscribe(() => {
        unsub();
        resolve();
      });
      editor.build();
    });

  const typeAtCursor = async (text: string, cps: number) => {
    const delay = 1000 / (cps > 0 ? cps : 40);
    for (const ch of text) {
      editor.insertAtCursor(ch);
      await sleep(delay);
    }
  };

  // Instantly load full source and build (blank-slate seeding, not typed).
  registerCommand('editor.loadCode', async (p) => {
    editor.setContentSilent(String(p.code ?? ''));
    await buildAndWait();
    return null;
  });

  // Type text at the cursor char-by-char (scroll-follows), then build unless
  // build:false. Insertion is at the cursor, so it composes with moveTo.
  registerCommand('editor.type', async (p) => {
    await typeAtCursor(String(p.code ?? p.text ?? ''), Number(p.cps));
    if (p.build !== false) await buildAndWait();
    return null;
  });

  // Move the cursor just after the first occurrence of `find`.
  registerCommand('editor.moveTo', async (p) => {
    const find = String(p.find ?? '');
    if (!editor.moveCursorAfter(find)) throw new Error(`moveTo: not found: ${find}`);
    return null;
  });

  // Find text, select it (briefly visible), delete it, and type the replacement
  // in its place — a human-style edit. Then build unless build:false.
  registerCommand('editor.replace', async (p) => {
    const find = String(p.find ?? '');
    if (!editor.selectRange(find)) throw new Error(`replace: not found: ${find}`);
    await sleep(300); // let the highlighted selection register on camera
    editor.deleteSelection();
    await typeAtCursor(String(p.code ?? p.text ?? ''), Number(p.cps));
    if (p.build !== false) await buildAndWait();
    return null;
  });

  // Explicit build + wait (for use after a build:false type/replace).
  registerCommand('editor.build', async () => {
    await buildAndWait();
    return null;
  });

  // Read back the active editor's source (state inspection for demo scripts).
  registerCommand('editor.getCode', async () => ({ code: editor.getContent() }));
}

function registerViewerCommands(viewer: Viewer, setAutoRotate: (on: boolean) => void): void {
  registerCommand('viewer.setCamera', async (p) => {
    viewer.applyCameraPose({
      azimuth: Number(p.azimuth ?? 0),
      elevation: Number(p.elevation ?? 0),
      distance: p.distance != null ? Number(p.distance) : undefined,
      target: p.target,
    });
    return null;
  });

  // Frame the model to fit the viewport — handy before recording.
  registerCommand('viewer.frameAll', async () => {
    viewer.fitToView();
    return null;
  });

  // Click a face at normalized canvas coords (default centre) as if the user
  // clicked it: highlights it and navigates the editor to the op that made it.
  // Repeated calls at the same spot cycle through the ops (face-click nav demo).
  registerCommand('viewer.clickFace', async (p) => {
    const x = p.x != null ? Number(p.x) : 0.5;
    const y = p.y != null ? Number(p.y) : 0.5;
    viewer.pickFaceAt(x, y);
    return null;
  });

  // Enable/disable the "auto-center & rotate" turntable via its Toggle, so the
  // viewer AND the toolbar button update together.
  registerCommand('viewer.autoRotate', async (p) => {
    setAutoRotate(p.on !== false); // default on
    return null;
  });
}

function registerAnimationCommands(): void {
  // Play/pause the wall-clock animation loop (only meaningful for an Animation
  // entry point; a no-op otherwise).
  registerCommand('animation.play', async () => {
    setPlaying(true);
    return null;
  });
  registerCommand('animation.pause', async () => {
    setPlaying(false);
    return null;
  });
}

function registerRecordCommands(canvas: HTMLCanvasElement): void {
  const recorder = new Recorder(canvas);
  // Track the active surface + name so record.stop routes to the matching stop
  // path (and canvas mode can name the file it saves at stop). 'canvas' records
  // the WebGL viewer here in the WebView; 'page' records the whole window
  // natively (ScreenCaptureKit) on the Go side.
  let active: RecordMode | null = null;
  let activeName = '';

  registerCommand('record.start', async (p) => {
    if (active) throw new Error(`already recording (${active})`);
    const mode: RecordMode = p.mode === 'page' ? 'page' : 'canvas';
    // Optional label → filename prefix, so recordings can be organized.
    activeName = typeof p.name === 'string' ? p.name : '';
    if (mode === 'page') {
      // width/height (px) set the page video's output size; 0 = window's
      // native size. For canvas mode, size follows the window — use
      // window.setSize first.
      const w = p.width != null ? Number(p.width) : 0;
      const h = p.height != null ? Number(p.height) : 0;
      await StartWindowCapture(w, h, activeName);
    } else {
      recorder.start({ fps: p.fps != null ? Number(p.fps) : undefined });
    }
    active = mode;
    return null;
  });

  registerCommand('record.stop', async () => {
    if (!active) throw new Error('not recording');
    let path: string;
    if (active === 'page') {
      path = await StopWindowCapture();
    } else {
      const blob = await recorder.stop();
      path = await SaveRecording(await blobToDataURL(blob), activeName);
    }
    active = null;
    return { path };
  });
}
