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
import { on, type AutomationInvokePayload } from './events';
import type { Viewer } from './viewer';
import { Recorder, blobToDataURL } from './recorder';

type RecordMode = 'canvas' | 'page';

export interface AutomationDeps {
  viewer: Viewer;
}

type CommandFn = (params: any) => Promise<unknown>;

const registry = new Map<string, CommandFn>();

/** Register a command. Grows as demos need more of the GUI driven. */
export function registerCommand(name: string, run: CommandFn): void {
  if (registry.has(name)) throw new Error(`automation: duplicate command ${name}`);
  registry.set(name, run);
}

export function initAutomation(deps: AutomationDeps): void {
  registerViewerCommands(deps.viewer);
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
      await StartWindowCapture();
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
