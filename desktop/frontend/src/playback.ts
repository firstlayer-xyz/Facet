// playback.ts — Wall-clock playback state and render-tick handler for
// animated Facet entry points.
//
// Registered with viewer.onFrame so it fires on every render tick
// (~60 Hz). Each tick issues one /frame request if playing and no
// request is already in flight (frame-dropping).
// Playback dependencies (getSources, applyFrame) are supplied by the host app.

import { frameRequest, frameInFlight, cancelFrame } from './frame-client';
import type { EvalResult } from './frame-client';
import { tabStore } from './tabs';

let playing = false;
let onStateChange: ((playing: boolean) => void) | null = null;
let applyFrame: ((binary: ArrayBuffer, header: EvalResult) => void) | null = null;
let getSources: (() => Record<string, string>) | null = null;
// Sources snapshot taken when playback starts. The user is not editing during
// playback, so the payload is identical every frame; snapshotting once avoids
// re-serializing every editor model ~60x/second. Null while paused.
let sourcesSnapshot: Record<string, string> | null = null;

export interface PlaybackOpts {
  /** Called with the decoded binary + header to swap the mesh. */
  applyFrame: (binary: ArrayBuffer, header: EvalResult) => void;
  /** Returns the current editor sources for the /frame payload. */
  getSources: () => Record<string, string>;
  /** Optional callback fired whenever the playing state changes. */
  onStateChange?: (playing: boolean) => void;
}

/** Wire up the playback module.  Must be called once before use. */
export function initPlayback(opts: PlaybackOpts): void {
  applyFrame = opts.applyFrame;
  getSources = opts.getSources;
  onStateChange = opts.onStateChange ?? null;
}

export function isPlaying(): boolean {
  return playing;
}

export function setPlaying(p: boolean): void {
  if (playing === p) return;
  playing = p;
  if (playing) {
    // Snapshot the editor sources once; every frame this run reuses them.
    sourcesSnapshot = getSources?.() ?? {};
  } else {
    // Pausing: abort any in-flight /frame so a late response can't apply a
    // stale mesh after the user stopped, and the server is freed immediately.
    cancelFrame();
    sourcesSnapshot = null;
  }
  onStateChange?.(playing);
}

/**
 * Registered with viewer.onFrame; called on every render tick (~60 Hz).
 * Issues a /frame for the current wall-clock time, dropping the tick
 * while a previous request is still in flight.
 */
export function onRenderTick(): void {
  if (!playing || frameInFlight()) return;
  // UTC epoch ms; a model applies any timezone offset itself.
  void issueFrame(Date.now());
}

async function issueFrame(timeMs: number): Promise<void> {
  const key = tabStore.active();
  const state = tabStore.activeState();
  const picked = state?.pickedEntry;
  if (!key || !picked?.name || !applyFrame || !sourcesSnapshot) return;

  try {
    const resp = await frameRequest({
      sources: sourcesSnapshot,
      key,
      entry: picked.name,
      overrides: state?.entryOverrides ?? {},
      timeMs,
    });
    // Any source or eval errors pause playback so the error state is visible.
    if (resp.header.errors && resp.header.errors.length > 0) {
      setPlaying(false);
      return;
    }
    applyFrame(resp.binary, resp.header);
  } catch (e: unknown) {
    if (e instanceof Error && e.name === 'AbortError') return;
    setPlaying(false);
    console.error('frame request failed:', e);
  }
}
