// EvalStore — single owner of the most recent eval result; callers read
// via current() and write via set().
//
// The editor syncs via evalStore.subscribe(); the viewer is still pushed
// explicitly via viewer.applyEvalResult().

import type { EvalResult } from './eval-client';

type Listener = () => void;

class EvalStore {
  private _current: EvalResult | null = null;
  private _listeners = new Set<Listener>();

  /** The most recent eval result, or null when there hasn't been one
   *  yet (or after a debug-exit / no-tabs reset). Callers reach into
   *  this for sources, references, declarations, debug steps, etc. */
  current(): EvalResult | null {
    return this._current;
  }

  /** Replace the current result. Pass null to clear (debug exit, last
   *  tab closed). Notifies subscribers. */
  set(result: EvalResult | null): void {
    if (this._current === result) return;
    // Tab kinds (read-only, library-backed) live in result.sources, but a failed
    // eval — e.g. a syntax error mid-typing — returns a header with no sources
    // map. Those kinds describe the open tabs, not this eval, so carry them
    // forward instead of letting a transient error erase them; otherwise a
    // view-only library tab momentarily reads as an editable user root and gets
    // re-sent to eval. Errors are still surfaced via result.errors.
    if (result && !result.sources && this._current?.sources) {
      result.sources = this._current.sources;
    }
    this._current = result;
    this.notify();
  }

  /**
   * Subscribe to result changes. Cb fires on every set(), including
   * set(null). Returns an unsubscribe function. Mirrors TabStore's
   * subscription API.
   */
  subscribe(cb: Listener): () => void {
    this._listeners.add(cb);
    return () => { this._listeners.delete(cb); };
  }

  private notify(): void {
    for (const cb of this._listeners) cb();
  }
}

export const evalStore = new EvalStore();
