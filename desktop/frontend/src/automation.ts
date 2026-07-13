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
  StartWindowCapture,
  StopWindowCapture,
} from '../wailsjs/go/main/App';
import { WindowSetSize } from '../wailsjs/runtime/runtime';
import { on, type AutomationInvokePayload } from './events';
import type { Viewer } from './viewer';
import { evalStore } from './eval-store';
import { setPlaying } from './playback';
import { Recorder, blobToDataURL } from './recorder';

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
  registerEditorCommands(deps.editor);
  registerViewerCommands(deps.viewer);
  registerAnimationCommands();
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

function registerViewerCommands(viewer: Viewer): void {
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

  // Enable/disable the existing "auto-center & rotate" turntable. setAutoRotate
  // updates both the viewer and the toolbar button (via onAutoRotateChange).
  registerCommand('viewer.autoRotate', async (p) => {
    viewer.setAutoRotate(p.on !== false); // default on
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
  // Track the active surface so record.stop routes to the matching stop path.
  // 'canvas' records the WebGL viewer here in the WebView; 'page' records the
  // whole window natively (ScreenCaptureKit) on the Go side.
  let active: RecordMode | null = null;

  registerCommand('record.start', async (p) => {
    if (active) throw new Error(`already recording (${active})`);
    const mode: RecordMode = p.mode === 'page' ? 'page' : 'canvas';
    if (mode === 'page') {
      // width/height (px) set the page video's output size; 0 = window's
      // native size. For canvas mode, size follows the window — use
      // window.setSize first.
      const w = p.width != null ? Number(p.width) : 0;
      const h = p.height != null ? Number(p.height) : 0;
      await StartWindowCapture(w, h);
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
      path = await SaveRecording(await blobToDataURL(blob));
    }
    active = null;
    return { path };
  });
}
