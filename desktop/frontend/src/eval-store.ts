// EvalStore — single owner of the most recent eval result. Replaces
// the module-level `lastResult` in app.ts plus the scattered mutation
// sites that touched it. The architecture review flagged `lastResult`
// as the second of three "holders of truth" — every read in app.ts
// either reaches into it or pushes a slice of it into the editor /
// viewer / docs panel. Centralising it sets up future consumers to
// subscribe rather than be pushed to.
//
// This refactor only moves the variable behind a store. Subscribers
// are not yet wired up — that's a follow-up that gradually replaces
// the explicit editor.updateXxx() / viewer.applyEvalResult() pushes
// with EvalStore.subscribe() reactions.

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
