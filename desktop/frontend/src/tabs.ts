// TabStore — single owner of tab state. Replaces three module-level
// variables in app.ts (`tabs`, `tabOrder`, `activeTab`) plus the
// scattered mutation sites that touched them. Tabs are peers; the
// store maintains the invariant that `active()` either points at an
// existing tab or is the empty string (no privileged "main" tab).
//
// Mutations go through methods so a subscriber notification can fire
// once per change — UI components subscribe instead of being commanded
// to re-render from every call site that happened to mutate.

export interface TabState {
  /** Resolved filesystem path; also the key under which this tab is stored. */
  path: string;
  dirty: boolean;
  cursor: { lineNumber: number; column: number } | null;
  label: string;
  pickedEntry: { name: string; libPath: string } | null;
  /**
   * Function-preview slider values for this tab's picked entry point.
   * Lives on the tab so switching away and back restores the sliders
   * you left rather than the last value any tab happened to set.
   */
  entryOverrides: Record<string, unknown>;
}

type Listener = () => void;

class TabStore {
  private _tabs: Record<string, TabState> = {};
  private _order: string[] = [];
  private _active = '';
  private _listeners = new Set<Listener>();

  // ── Read accessors ────────────────────────────────────────────────

  has(path: string): boolean {
    return this._tabs[path] !== undefined;
  }

  get(path: string): TabState | undefined {
    return this._tabs[path];
  }

  /** Returns the active tab key, or '' if no tab is active. */
  active(): string {
    return this._active;
  }

  /** Returns the active tab's state, or undefined when nothing is active. */
  activeState(): TabState | undefined {
    return this._tabs[this._active];
  }

  isActive(path: string): boolean {
    return this._active !== '' && this._active === path;
  }

  /** Tab keys in their current display order. */
  order(): readonly string[] {
    return this._order;
  }

  /** Tab states in display order. Missing tabs (race during remove) are filtered out. */
  ordered(): TabState[] {
    const out: TabState[] = [];
    for (const k of this._order) {
      const t = this._tabs[k];
      if (t) out.push(t);
    }
    return out;
  }

  /** Total tab count. */
  size(): number {
    return this._order.length;
  }

  /** True when at least one tab is marked dirty. */
  anyDirty(): boolean {
    for (const k of this._order) {
      if (this._tabs[k]?.dirty) return true;
    }
    return false;
  }

  // ── Mutations ─────────────────────────────────────────────────────

  /** Add a new tab. Idempotent on path: a second call replaces the state. */
  add(state: TabState): void {
    const isNew = this._tabs[state.path] === undefined;
    this._tabs[state.path] = state;
    if (isNew) this._order.push(state.path);
    this.notify();
  }

  /** Remove a tab. Clears `active` if the removed tab was active. */
  remove(path: string): void {
    if (this._tabs[path] === undefined) return;
    delete this._tabs[path];
    this._order = this._order.filter(k => k !== path);
    if (this._active === path) this._active = '';
    this.notify();
  }

  /**
   * Set the active tab. Path must exist in the store, or be the empty
   * string (no active tab — only valid when zero tabs are open).
   */
  setActive(path: string): void {
    if (path !== '' && this._tabs[path] === undefined) {
      throw new Error(`TabStore.setActive: no tab "${path}"`);
    }
    if (this._active === path) return;
    this._active = path;
    this.notify();
  }

  /** Replace the order array (drag-reorder, restore-from-session). */
  setOrder(paths: string[]): void {
    // Drop any path the store doesn't have; preserves invariant
    // that order only references known tabs.
    this._order = paths.filter(p => this._tabs[p] !== undefined);
    // Append any known tabs missing from the new order so they don't disappear.
    for (const k of Object.keys(this._tabs)) {
      if (!this._order.includes(k)) this._order.push(k);
    }
    this.notify();
  }

  markDirty(path: string): boolean {
    const t = this._tabs[path];
    if (!t || t.dirty) return false;
    t.dirty = true;
    this.notify();
    return true;
  }

  markClean(path: string): boolean {
    const t = this._tabs[path];
    if (!t || !t.dirty) return false;
    t.dirty = false;
    this.notify();
    return true;
  }

  setCursor(path: string, cursor: TabState['cursor']): void {
    const t = this._tabs[path];
    if (t) t.cursor = cursor;
    // Cursor changes are too frequent to broadcast — they would
    // re-render the tab bar on every keystroke. Subscribers that
    // need cursor changes can read getCursorPosition() from the
    // editor directly.
  }

  /**
   * Replace the entry-override slider values for `path`. Stored
   * per-tab so each tab keeps its own picked-function sliders across
   * tab switches. Doesn't notify — overrides change on every slider
   * tick during dragging, and re-rendering the tab bar on every tick
   * burns frames. Run/eval reads the latest value via activeState().
   */
  setEntryOverrides(path: string, overrides: Record<string, unknown>): void {
    const t = this._tabs[path];
    if (t) t.entryOverrides = overrides;
  }

  setPickedEntry(path: string, entry: TabState['pickedEntry']): void {
    const t = this._tabs[path];
    if (!t) return;
    t.pickedEntry = entry;
    this.notify();
  }

  // ── Subscriptions ─────────────────────────────────────────────────

  /**
   * Subscribe to any change in tab state. Callback fires once per
   * mutation; subscribers should debounce/batch if they re-render
   * expensively. Returns an unsubscribe function.
   */
  subscribe(cb: Listener): () => void {
    this._listeners.add(cb);
    return () => { this._listeners.delete(cb); };
  }

  /**
   * Subscribe to active-tab transitions only. Callback receives the
   * new active key (or '' when no tab is active). Filters out
   * mutations that don't move active — dirty toggles, picked-entry
   * updates, label changes — so consumers that only care about
   * "what's showing now" don't wake on every store mutation.
   */
  onActiveChange(cb: (active: string) => void): () => void {
    let prev = this._active;
    const adapter = () => {
      const cur = this._active;
      if (cur !== prev) {
        prev = cur;
        cb(cur);
      }
    };
    return this.subscribe(adapter);
  }

  private notify(): void {
    for (const cb of this._listeners) cb();
  }
}

export const tabStore = new TabStore();
