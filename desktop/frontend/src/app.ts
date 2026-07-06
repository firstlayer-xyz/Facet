// app.ts — Run/debug orchestration logic.

import { ConfirmDiscard, OpenFile, OpenRecentFile, AddRecentFile, SaveFile, ExportMesh, SendToSlicer, BuildShareLink, GetDocCatalog, GetDocGuides, SetWindowTitle, FormatCode, CreateScratchFile, IsScratchFile, SetDirtyState } from '../wailsjs/go/main/App';
import type { main } from '../wailsjs/go/models';
import { showSharePopover } from './dialogs';
import type { EntryPoint } from './function-preview';
import { on } from './events';
import { Viewer } from './viewer';
import type { DecodedMesh } from './viewer';
import type { EditorHandle } from './editor';
import { DocsPanel } from './docs';
import { patchSettings } from './settings';
import type { SavedTab } from './settings';
import { reportError } from './toast';
import { evalRequest, cancelEval, formatSourceErrors } from './eval-client';
import type { EvalResponse, EvalResult, SourceEntry, SourceError } from './eval-client';
import { decodeBinaryMesh } from './mesh-decode';
import type { BinaryMeshMeta } from './mesh-decode';
import { initPlayback, onRenderTick, isPlaying, setPlaying } from './playback';

// Source kind constants (mirrors parser.SourceKind in Go)
const SOURCE_USER = 0;
const SOURCE_STDLIB = 1;
const SOURCE_LIBRARY = 2;
const SOURCE_CACHED = 3;
const SOURCE_EXAMPLE = 4;

/** Read-only source kinds — not editable by the user. */
function isReadOnlyKind(kind: number): boolean {
  return kind === SOURCE_STDLIB || kind === SOURCE_CACHED || kind === SOURCE_EXAMPLE;
}

/** Source kinds excluded from tab persistence (ephemeral). */
function isEphemeralKind(kind: number): boolean {
  return kind === SOURCE_STDLIB || kind === SOURCE_CACHED;
}

/** Files excluded from face-click → source navigation (stdlib). Shared by the
 *  static render and animation playback so face-click behaves identically. */
function stdlibExcludeFiles(sources: Record<string, SourceEntry> | undefined): Set<string> {
  const ex = new Set<string>();
  if (sources) {
    for (const [path, entry] of Object.entries(sources)) {
      if (entry.kind === SOURCE_STDLIB) ex.add(path);
    }
  }
  return ex;
}

// Dependencies injected via initApp()
let viewer: Viewer;
let editor: EditorHandle;
let docsPanel: DocsPanel;

// DOM elements injected via initApp()
let errorDiv: HTMLElement;
let debugBar: HTMLElement;
let debugSlider: HTMLInputElement;
let debugLabel: HTMLElement;
let tabBar: HTMLElement;
let statsBar: HTMLElement;
let compilingOverlay: HTMLElement;
let onDebugBarChangeCb: ((visible: boolean) => void) | null = null;

// Debug mode state
let debugMode = false;
let debugStepping = false; // true while showDebugStep is switching tabs
let debugEntryTab = ''; // tab that was active when debug started
let debugFinalMesh: DecodedMesh | null = null;
let debugBinary: ArrayBuffer | null = null;
let debugStepIndex = 0;

// Breakpoint state — keyed by file path, values are line numbers
const breakpoints = new Map<string, Set<number>>();
const validBreakpointLines = new Map<string, Set<number>>();
// Tab state lives in TabStore (see ./tabs.ts). Helper wrappers below
// keep the per-call-site call patterns short — `getTab` materialises
// an absent tab the same way the prior in-file map did.
import { tabStore } from './tabs';
import type { TabState } from './tabs';
import { evalStore } from './eval-store';

/** Get a tab, creating an empty one if absent. */
function getTab(key: string): TabState {
  const existing = tabStore.get(key);
  if (existing) return existing;
  const t: TabState = { path: key, dirty: false, cursor: null, label: tabLabel(key), pickedEntry: null, entryOverrides: {} };
  tabStore.add(t);
  return t;
}
function addTab(state: TabState) {
  tabStore.add(state);
}

function removeTab(key: string) {
  tabStore.remove(key);
}

function isReadOnly(path: string): boolean {
  return isReadOnlyKind(evalStore.current()?.sources?.[path]?.kind ?? SOURCE_USER);
}

function isDirty(): boolean {
  return tabStore.activeState()?.dirty ?? false;
}

// The latest eval result lives in EvalStore (./eval-store.ts).
// app.ts reads it via evalStore.current() and writes via
// evalStore.set(). Direct `lastResult` references were removed when
// the store was introduced.

// Callbacks for external UI components.
// onTabChange went away — subscribe to tabStore.onActiveChange()
// directly from main.ts instead.
let onSourceChangeCb: ((source: string) => void) | null = null;
let onDebugFilesChangeCb: (() => void) | null = null;
let onDebugExitCb: (() => void) | null = null;
// Returns the picked entry point name (or null to skip running).
let onEntryPointsCb: ((fns: EntryPoint[]) => { name: string; libPath: string } | null) | null = null;


// Entry-point overrides (slider values for constrained function params)
// live on each tab — see TabState.entryOverrides. setEntryOverrides
// below routes writes to the active tab; reads pull from
// tabStore.activeState() at eval time.
/** Set the file path on startup (no discard prompt, no re-persist). */
export function setInitialFile(path: string, label?: string, readOnly?: boolean) {
  addTab({ path, dirty: false, cursor: null, label: label || tabLabel(path), pickedEntry: null, entryOverrides: {} });
  tabStore.setActive(path);
  editor.setCurrentSource(path);
  editor.setReadOnly(readOnly ?? isReadOnly(path));
  updateWindowTitle();
  renderTabs();
}

/** Register a tab restored from saved state without switching the editor or triggering a run.
 *  The Monaco model should already be pre-created via editor.switchModel(). */
export function addRestoredTab(path: string, cursor: { lineNumber: number; column: number } | null) {
  addTab({ path, dirty: false, cursor, label: tabLabel(path), pickedEntry: null, entryOverrides: {} });
}

function anyDirty(): boolean {
  return tabStore.anyDirty();
}

function markDirty() {
  const active = tabStore.active();
  if (active && tabStore.markDirty(active)) {
    updateWindowTitle();
    SetDirtyState(true);
  }
}
function markClean() {
  const active = tabStore.active();
  if (active && tabStore.markClean(active)) {
    updateWindowTitle();
    SetDirtyState(anyDirty());
  }
}
function persistOpenTabs() {
  const sources = evalStore.current()?.sources ?? {};
  // Save cursor for active tab before persisting
  const active = tabStore.active();
  if (active) {
    tabStore.setCursor(active, editor.getCursorPosition());
  }
  // Persist all tabs except stdlib (kind=1) and cached libs (kind=3)
  const savedTabs: SavedTab[] = [];
  for (const tab of tabStore.ordered()) {
    const kind = sources[tab.path]?.kind ?? SOURCE_USER;
    if (isEphemeralKind(kind)) continue;
    savedTabs.push({ path: tab.path, label: tab.label, cursor: tab.cursor });
  }
  patchSettings({ savedTabs, activeTab: tabStore.active() });
}

function updateWindowTitle() {
  const active = tabStore.active();
  const tab = tabStore.activeState();
  const name = tab ? tab.label || tabLabel(active) : 'Untitled';
  const prefix = isDirty() ? '\u25cf ' : '';
  SetWindowTitle(`${prefix}${name} \u2014 Facet`);
  syncTitlebarFilename(name, isDirty());
}

function syncTitlebarFilename(name: string, dirty: boolean) {
  const el = document.getElementById('titlebar-filename');
  if (!el) return;
  // textContent (not innerHTML) \u2014 a filename like `<img src=x onerror=\u2026>.fct`
  // must not inject script into the webview, which can call bound Go methods.
  el.textContent = name;
  if (dirty) {
    const dot = document.createElement('span');
    dot.className = 'titlebar-dirty-dot';
    dot.textContent = '\u25cf';
    const label = document.createElement('span');
    label.className = 'titlebar-dirty-label';
    label.textContent = 'modified';
    el.append(' ', dot, ' ', label);
  }
}

// Run state — driven by Go-side events ("run:start" / "run:idle")
type RunState = 'idle' | 'running';
let runState: RunState = 'idle';

const PLAY_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`;
const STOP_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12"/></svg>`;
const PAUSE_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>`;

function setRunState(state: RunState) {
  runState = state;
  const btn = document.getElementById('run-btn')!;
  if (state === 'running') {
    btn.innerHTML = STOP_ICON;
    btn.title = 'Stop';
    btn.classList.add('running');
    setSpinner(true);
  } else {
    btn.innerHTML = PLAY_ICON;
    btn.title = 'Run';
    btn.classList.remove('running');
    // Defer spinner hide so Three.js has a chance to paint the new mesh
    requestAnimationFrame(() => setSpinner(false));
  }
}

export interface AppDeps {
  viewer: Viewer;
  editor: EditorHandle;
  docsPanel: DocsPanel;
  errorDiv: HTMLElement;
  debugBar: HTMLElement;
  debugSlider: HTMLInputElement;
  debugLabel: HTMLElement;
  tabBar: HTMLElement;
  statsBar: HTMLElement;
  compilingOverlay: HTMLElement;
  onEvalStatus?: (state: 'idle' | 'ready' | 'error', ms?: number) => void;
  onDebugBarChange?: (visible: boolean) => void;
}

let onEvalStatusCb: AppDeps['onEvalStatus'];

export function initApp(deps: AppDeps) {
  viewer = deps.viewer;
  editor = deps.editor;
  docsPanel = deps.docsPanel;
  errorDiv = deps.errorDiv;
  debugBar = deps.debugBar;
  debugSlider = deps.debugSlider;
  debugLabel = deps.debugLabel;
  tabBar = deps.tabBar;
  statsBar = deps.statsBar;
  compilingOverlay = deps.compilingOverlay;
  onEvalStatusCb = deps.onEvalStatus;
  onDebugBarChangeCb = deps.onDebugBarChange ?? null;

  // Persist tabs when app is about to close
  on('app:before-close', () => persistOpenTabs());

  // Wire the frame playback loop.
  initPlayback({
    getSources: () => editor.getAllSources(),
    applyFrame: (binary, header) => {
      const decoded = header.mesh ? decodeBinaryMesh(binary, header.mesh) : null;
      // excludeFiles is stable across frames (same program), so derive it from
      // the current eval result rather than the per-frame header.
      const excludeFiles = stdlibExcludeFiles(evalStore.current()?.sources);
      viewer.applyEvalResult(decoded, header.posMap ?? [], { autofit: false, excludeFiles });
    },
    onStateChange: () => updateRunButtonForPlayback(),
  });
  viewer.onFrame(() => onRenderTick());

}

function setSpinner(active: boolean) {
  compilingOverlay.style.display = active ? 'flex' : 'none';
}

/** Allow main.ts to set the editor after async creation */
export function setEditor(ed: EditorHandle) {
  editor = ed;

  // The editor's symbol table / types / declarations / references /
  // sources track whatever the latest eval produced. Subscribing here
  // means a single evalStore.set(data) automatically replaces every
  // dependent piece in the editor — handleHTTPResult no longer has to
  // remember which updateXxx calls to fire after each eval. Null
  // results (debug exit, last tab closed) leave the existing data in
  // place rather than clearing it, so hover/completion stay populated
  // on the editor until the next real result lands.
  evalStore.subscribe(() => {
    const r = evalStore.current();
    if (!r) return;
    if (r.symbols) editor.updateSymbols(r.symbols);
    if (r.varTypes && Object.keys(r.varTypes).length > 0) editor.updateVarTypes(r.varTypes);
    if (r.declarations?.decls) editor.updateDeclarations(r.declarations.decls);
    if (r.references) editor.updateReferences(r.references);
    if (r.sources) {
      const textSources: Record<string, string> = {};
      for (const [k, v] of Object.entries(r.sources)) textSources[k] = v.text;
      editor.updateFileSources(textSources);
      onSourceChangeCb?.(editor.getContent());
    }
  });

  // Sync breakpoints when the editor shifts line numbers due to code edits
  ed.onBreakpointChange((file, lines) => {
    breakpoints.set(file, lines);
  });

  // Jump to first debug step at the clicked line
  ed.onJumpToLine((file, line) => {
    const steps = evalStore.current()?.debugSteps ?? [];
    const idx = steps.findIndex(s => (s.file || debugEntryTab) === file && s.line === line);
    if (idx >= 0) showDebugStep(idx);
  });

  // Wire mouse hover to face highlighting (debounced to one frame)
  let highlightRAF = 0;
  editor.onMouseMove((line: number, col: number) => {
    if (highlightMode !== 'mouse') return;
    if (highlightRAF) cancelAnimationFrame(highlightRAF);
    highlightRAF = requestAnimationFrame(() => {
      viewer.highlightAtPos(tabStore.active(), line, col);
    });
  });
  editor.onMouseLeave(() => {
    if (highlightMode !== 'mouse') return;
    if (highlightRAF) cancelAnimationFrame(highlightRAF);
    viewer.clearHighlight();
  });
  editor.onCursorChange((line: number, col: number) => {
    if (highlightMode !== 'cursor') return;
    if (highlightRAF) cancelAnimationFrame(highlightRAF);
    highlightRAF = requestAnimationFrame(() => {
      viewer.highlightAtPos(tabStore.active(), line, col);
    });
  });
}

/** Forward word-wrap setting to the editor (if initialized) */
export function setEditorWordWrap(on: boolean) {
  if (editor) editor.setWordWrap(on);
}

let highlightMode: 'mouse' | 'cursor' | 'off' = 'cursor';

export function setHighlightMode(mode: 'mouse' | 'cursor' | 'off') {
  highlightMode = mode;
  if (mode === 'off' && viewer) viewer.clearHighlight();
}

let formatOnSave = true;

export function setFormatOnSave(on: boolean) {
  formatOnSave = on;
}

function formatVolume(mm3: number): string {
  if (mm3 >= 1e9) return (mm3 / 1e9).toFixed(2) + ' L';
  if (mm3 >= 1e3) return (mm3 / 1e3).toFixed(2) + ' cm\u00B3';
  return mm3.toFixed(2) + ' mm\u00B3';
}

function formatArea(mm2: number): string {
  if (mm2 >= 1e6) return (mm2 / 1e6).toFixed(2) + ' m\u00B2';
  if (mm2 >= 1e2) return (mm2 / 1e2).toFixed(2) + ' cm\u00B2';
  return mm2.toFixed(2) + ' mm\u00B2';
}

function formatTime(seconds: number): string {
  if (seconds >= 1) return seconds.toFixed(2) + 's';
  return (seconds * 1000).toFixed(0) + 'ms';
}

function statsRow(label: string, value: string): string {
  return `<div class="stats-row"><span class="stats-label">${label}</span><span class="stats-value">${value}</span></div>`;
}

function showStats(stats: { triangles: number; vertices: number; volume: number; surfaceArea: number }, time: number) {
  const ms = Math.round(time * 1000);
  statsBar.innerHTML =
    statsRow('TRIS', stats.triangles.toLocaleString()) +
    statsRow('VOLUME', formatVolume(stats.volume)) +
    statsRow('AREA', formatArea(stats.surfaceArea));
  statsBar.style.display = 'block';
  onEvalStatusCb?.('ready', ms);
}

function hideStats() {
  statsBar.style.display = 'none';
  onEvalStatusCb?.('idle');
}

function setDebugBarVisible(visible: boolean) {
  debugBar.style.display = visible ? 'flex' : 'none';
  onDebugBarChangeCb?.(visible);
}


function syncTabsWithSources(data: EvalResult) {
  if (!data.sources) return;
  let tabsClosed = false;
  for (const path of tabStore.order()) {
    if (tabStore.isActive(path)) continue;
    if (!data.sources[path]) {
      editor.disposeModel(path);
      removeTab(path);
      tabsClosed = true;
    }
  }
  if (tabsClosed) renderTabs();
}

function handleCheckOnly(errors: SourceError[], fns: EntryPoint[]) {
  viewer.setPosMap([]);

  // Sync entry points first — even when there are errors — so callers like
  // closeTab / setOnTabChange can pick a valid entry from the remaining
  // tabs and re-trigger an eval. Skipping this on the error path was the
  // root cause of "closing a broken tab leaves the error stuck on screen."
  const picked = onEntryPointsCb?.(fns);

  if (errors.length > 0) {
    const e = errors[0];
    hideStats();
    const prefix = e.file ? `[${e.file}${e.line > 0 ? ':' + e.line : ''}] ` : '';
    showError(prefix + e.message);
    // hasErrorSelection is true when the user mouse-up'd after dragging
    // to select text inside the error bar — without this guard the
    // onclick fires (mouseup IS the click), navigates to the source
    // line, and the editor's focus shift wipes the selection before
    // the user can copy. Treat the click as a no-op in that case.
    const hasErrorSelection = (): boolean => {
      const sel = window.getSelection();
      return !!sel && sel.toString().length > 0 && errorDiv.contains(sel.anchorNode);
    };
    if (e.line > 0 && !e.file) {
      errorDiv.style.cursor = 'pointer';
      editor.highlightError(e.line);
      errorDiv.onclick = () => {
        if (hasErrorSelection()) return;
        editor.revealLine(e.line, e.col || 1);
      };
    } else if (e.line > 0 && e.file) {
      errorDiv.style.cursor = 'pointer';
      errorDiv.onclick = () => {
        if (hasErrorSelection()) return;
        getTab(e.file);
        switchToTab(e.file);
        editor.highlightError(e.line);
        editor.revealLine(e.line, e.col || 1);
      };
    }
    return;
  }
  if (picked) {
    tabStore.setPickedEntry(tabStore.active(), picked);
    runViaHTTP(); // re-run with the picked entry point
    return;
  }
  // Reached when the eval landed with neither errors nor a runnable
  // entry — the active file has no entry function (fresh scratch,
  // types-only library, just-closed-the-last-tab auto-scratch). Reset
  // the viewer so a stale mesh from a previous file doesn't keep
  // rendering in an empty context. The errors-present branch above
  // intentionally keeps the previous mesh so the user can still see
  // their last good render while fixing the syntax problem.
  viewer.reset();
  hideStats();
}


/** Shorten a path to just the filename, for tab display. */
function tabLabel(path: string): string {
  if (!path) return 'Untitled';
  if (path.startsWith('example:')) {
    const name = path.slice('example:'.length);
    return name.endsWith('.fct') ? name.slice(0, -4) : name;
  }
  const parts = path.split('/');
  const name = parts[parts.length - 1] || path;
  return name.endsWith('.fct') ? name.slice(0, -4) : name;
}

export function renderTabs() {
  onDebugFilesChangeCb?.();
  persistOpenTabs();
  tabBar.innerHTML = '';
  const openTabs = tabStore.order();

  const leftArrow = document.createElement('button');
  leftArrow.className = 'tab-arrow';
  leftArrow.textContent = '\u2039';
  tabBar.appendChild(leftArrow);

  const scrollContainer = document.createElement('div');
  scrollContainer.className = 'tab-scroll';
  tabBar.appendChild(scrollContainer);

  const rightArrow = document.createElement('button');
  rightArrow.className = 'tab-arrow';
  rightArrow.textContent = '\u203A';
  tabBar.appendChild(rightArrow);

  for (const path of openTabs) {
    const tab = document.createElement('div');
    tab.className = 'tab' + (tabStore.isActive(path) ? ' active' : '');
    tab.title = path;

    const sourceKind = evalStore.current()?.sources?.[path]?.kind ?? SOURCE_USER;
    if (sourceKind === SOURCE_EXAMPLE) {
      // Example — star icon
      const star = document.createElement('span');
      star.className = 'tab-book';
      star.innerHTML = '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>';
      star.title = 'Example';
      tab.appendChild(star);
    } else if (sourceKind === SOURCE_LIBRARY) {
      // Library — book icon
      const book = document.createElement('span');
      book.className = 'tab-book';
      book.innerHTML = '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M2 3h6a4 4 0 014 4v14a3 3 0 00-3-3H2z"/><path d="M22 3h-6a4 4 0 00-4 4v14a3 3 0 013-3h7z"/></svg>';
      book.title = 'Library';
      tab.appendChild(book);
    } else if (sourceKind === SOURCE_STDLIB || sourceKind === SOURCE_CACHED) {
      // StdLib or Cached — lock icon
      const lock = document.createElement('span');
      lock.className = 'tab-lock';
      lock.innerHTML = '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0110 0v4"/></svg>';
      lock.title = 'Read-only';
      tab.appendChild(lock);
    } else if (getTab(path).dirty) {
      // User file with unsaved changes
      const dot = document.createElement('span');
      dot.className = 'tab-dirty';
      dot.textContent = '\u25cf';
      dot.title = 'Unsaved changes';
      tab.appendChild(dot);
    }

    const label = document.createElement('span');
    label.className = 'tab-label';
    label.textContent = getTab(path).label || tabLabel(path);
    tab.appendChild(label);

    const closeBtn = document.createElement('span');
    closeBtn.className = 'tab-close';
    closeBtn.textContent = '\u00d7';
    closeBtn.title = 'Close';
    closeBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      closeTab(path);
    });
    tab.appendChild(closeBtn);

    tab.addEventListener('click', () => switchToTab(path));

    // Drag to reorder
    tab.draggable = true;
    tab.dataset.tabPath = path;
    tab.addEventListener('dragstart', (e) => {
      e.dataTransfer!.effectAllowed = 'move';
      e.dataTransfer!.setData('text/plain', path);
      tab.classList.add('dragging');
    });
    tab.addEventListener('dragend', () => {
      tab.classList.remove('dragging');
    });
    tab.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.dataTransfer!.dropEffect = 'move';
      tab.classList.add('drag-over');
    });
    tab.addEventListener('dragleave', () => {
      tab.classList.remove('drag-over');
    });
    tab.addEventListener('drop', (e) => {
      e.preventDefault();
      tab.classList.remove('drag-over');
      const draggedPath = e.dataTransfer!.getData('text/plain');
      if (draggedPath === path) return;
      const current = [...tabStore.order()];
      const fromIdx = current.indexOf(draggedPath);
      const toIdx = current.indexOf(path);
      if (fromIdx < 0 || toIdx < 0) return;
      current.splice(fromIdx, 1);
      current.splice(toIdx, 0, draggedPath);
      tabStore.setOrder(current);
      renderTabs();
    });

    scrollContainer.appendChild(tab);
  }

  const scrollAmt = 120;
  leftArrow.addEventListener('click', () => {
    scrollContainer.scrollBy({ left: -scrollAmt, behavior: 'smooth' });
  });
  rightArrow.addEventListener('click', () => {
    scrollContainer.scrollBy({ left: scrollAmt, behavior: 'smooth' });
  });

  // Hide arrows if all tabs fit
  function updateArrows() {
    const overflows = scrollContainer.scrollWidth > scrollContainer.clientWidth;
    leftArrow.style.display = overflows ? '' : 'none';
    rightArrow.style.display = overflows ? '' : 'none';
  }
  updateArrows();
  // Re-check after layout settles
  requestAnimationFrame(updateArrows);
}

async function closeTab(file: string) {
  // Prompt if tab has unsaved changes
  if (getTab(file).dirty) {
    const ok = await ConfirmDiscard();
    if (!ok) return;
  }
  // Capture this BEFORE switchToTab reassigns the active tab —
  // otherwise the cancelEval check below is unreachable and the
  // in-flight eval leaks.
  const wasActive = tabStore.isActive(file);
  if (wasActive) {
    // The editor must always have a non-disposed model to display.
    // Switch to another open tab if there is one; otherwise create
    // an empty scratch tab and switch to that before disposing.
    // disposeModel below will throw if called while the editor is
    // still showing this file's model.
    const remaining = tabStore.order().filter(k => k !== file);
    if (remaining.length > 0) {
      switchToTab(remaining[0], true);
    } else {
      // Unique name (like newFile) so the replacement scratch can never be the
      // path we're closing — otherwise switchToTab would treat it as already
      // active and disposeModel below would throw on the still-displayed model.
      const scratch = await CreateScratchFile('Untitled-' + Date.now());
      addTab({ path: scratch, dirty: false, cursor: null, label: 'Untitled', pickedEntry: null, entryOverrides: {} });
      switchToTab(scratch, true);
    }
  }
  // Cancel any in-flight eval if the active tab is being closed
  if (wasActive) cancelEval();

  editor.disposeModel(file);
  removeTab(file);

  // Exit debug mode if the entry-point tab was closed
  if (debugMode && file === debugEntryTab) {
    debugMode = false;
    setDebugBarVisible(false);
    editor.clearDebugLine();
    viewer.reset();
    hideStats();
    debugFinalMesh = null;
    debugEntryTab = '';
    evalStore.set(null);
    onDebugExitCb?.();
  }

  renderTabs();

  // Clear viewport if no tabs remain
  if (tabStore.size() === 0) {
    viewer.reset();
    hideStats();
    clearError();
    setDebugBarVisible(false);
  } else {
    // Always re-run after closing a tab so stale render state and errors from
    // the removed file are cleared — regardless of whether an entry point was
    // found for the active tab.
    run();
  }
  // TabStore.onActiveChange subscribers (in main.ts) fire on the
  // setActive call in the switchToTab branch above — no manual
  // notification needed here.
}

// force bypasses the debug-mode guard: closing the active tab must switch the
// editor off the model it's about to dispose, even mid-debug. The guard only
// exists to block user-initiated navigation while stepping.
export function switchToTab(file: string, force = false) {
  if (tabStore.isActive(file)) return;
  if (!force && debugMode && !debugStepping) return;

  // Save cursor position for the tab we're leaving
  const prev = tabStore.active();
  if (prev) tabStore.setCursor(prev, editor.getCursorPosition());

  tabStore.setActive(file);
  editor.setCurrentSource(file);

  // Switch editor model — source comes from sources map or editor's own model cache
  const source = evalStore.current()?.sources?.[file]?.text ?? '';
  editor.switchModel(file, source);
  editor.setReadOnly(isReadOnly(file));

  // Restore cursor position
  const saved = getTab(file).cursor;
  if (saved) {
    editor.revealLine(saved.lineNumber, saved.column);
  }

  renderTabs();

  // Update declarations from cached data so Go to Declaration works
  const cached = evalStore.current();
  if (cached?.declarations?.decls) {
    editor.updateDeclarations(cached.declarations.decls);
  }
  if (cached?.references) {
    editor.updateReferences(cached.references);
  }

  // Re-highlight current debug step line if it belongs to this tab
  const steps = evalStore.current()?.debugSteps ?? [];
  if (steps.length > 0 && debugStepIndex < steps.length) {
    const step = steps[debugStepIndex];
    if ((step.file ?? '') === tabStore.active() && step.line > 0) {
      editor.highlightDebugLine(step.line);
    } else {
      editor.clearDebugLine();
    }
  }

  // tabStore.setActive above wakes tabStore.onActiveChange listeners
  // — file tree / preview selector subscribe from main.ts.
}

export function showDebugStepPrev() {
  showDebugStep(debugStepIndex - 1);
}

export function showDebugStepNext() {
  showDebugStep(debugStepIndex + 1);
}

export async function showDebugStep(index: number) {
  const debugSteps = evalStore.current()?.debugSteps ?? [];
  if (index < 0 || index >= debugSteps.length) return;
  debugStepIndex = index;
  debugSlider.value = String(index);
  const pct = debugSteps.length > 1 ? (index / (debugSteps.length - 1)) * 100 : 100;
  debugSlider.style.setProperty('--fill', `${pct}%`);
  const step = debugSteps[index];
  const lineInfo = step.line > 0 ? `  ·  line ${step.line}` : '';
  debugLabel.textContent = `Step ${index + 1}/${debugSteps.length}  ·  ${step.op}${lineInfo}`;

  // Display the step — per-step mesh lazy loading is not yet implemented for HTTP eval.
  if (step.op === 'Final' && debugFinalMesh) {
    viewer.clearMeshes();
    viewer.loadDecodedMesh(debugFinalMesh);
  } else if (step.meshes && step.meshes.length > 0 && debugBinary) {
    viewer.loadDebugStep(step, debugBinary);
  }
  viewer.fitToView();

  const stepFile = debugSteps[index].file ?? '';
  if (stepFile && stepFile !== tabStore.active()) {
    debugStepping = true;
    switchToTab(stepFile);
    debugStepping = false;
  }

  if (debugSteps[index].line > 0) {
    editor.highlightDebugLine(debugSteps[index].line);
  } else {
    editor.clearDebugLine();
  }
}

/** Navigate to the next step that hits a breakpoint, or jump to the last step if none. */
export function continueDebug() {
  const steps = evalStore.current()?.debugSteps ?? [];
  const hasBreakpoints = [...breakpoints.values()].some(s => s.size > 0);
  if (!hasBreakpoints) {
    showDebugStep(steps.length - 1);
    return;
  }
  for (let i = debugStepIndex + 1; i < steps.length; i++) {
    const step = steps[i];
    const file = step.file || debugEntryTab;
    const bps = breakpoints.get(file);
    if (bps?.has(step.line)) {
      showDebugStep(i);
      return;
    }
  }
  showDebugStep(steps.length - 1);
}

/** Trigger evaluation with the given entry point. */
export function reeval(entry: string, libPath?: string) {
  if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null; }
  tabStore.setPickedEntry(tabStore.active(), { name: entry, libPath: libPath || '' });
  runViaHTTP();
}

export function toggleRun() {
  if (activeEntryIsAnimated()) {
    setPlaying(!isPlaying());
    return;
  }
  if (runState === 'running') {
    cancelEval();
  } else {
    run();
  }
}

// True when the active entry point returns an Animation (the run button then
// toggles live playback instead of a one-shot eval).
function activeEntryIsAnimated(): boolean {
  const name = tabStore.activeState()?.pickedEntry?.name;
  if (!name) return false;
  return (evalStore.current()?.entryPoints ?? []).some(e => e.name === name && e.animated);
}

// Reflect playback state on the run button: pause icon while playing, play icon when stopped.
function updateRunButtonForPlayback() {
  const btn = document.getElementById('run-btn');
  if (!btn) return;
  if (isPlaying()) {
    btn.innerHTML = PAUSE_ICON;
    btn.title = 'Pause';
    btn.classList.add('running');
  } else {
    btn.innerHTML = PLAY_ICON;
    btn.title = 'Run';
    btn.classList.remove('running');
  }
}

let debounceTimer: number | null = null;
const DEBOUNCE_MS = 500;

// Auto-run guard: debounces keystroke changes and sends eval via HTTP.
// Called by editor onChange.
export function autoRun() {
  if (debugMode) return;
  markDirty();
  onSourceChangeCb?.(editor.getContent());
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    debounceTimer = null;
    runViaHTTP();
  }, DEBOUNCE_MS);
}

// Send current sources via HTTP for parsing, checking, and auto-eval.
export function run() {
  editor.clearDebugLine();
  runViaHTTP();
}

/** Refresh editor UI — triggers a check-only run via HTTP. */
export function refreshEditorUI() {
  runViaHTTP();
}

// Monotonically increasing token: starting a run supersedes any older in-flight
// one (which gets aborted). Only the latest run resets the run state, so a
// superseded run's finally doesn't flip the UI to idle while the newer run is
// still running.
let runGeneration = 0;

async function runViaHTTP() {
  const gen = ++runGeneration;
  const sources = editor.getAllSources();
  const active = tabStore.active();
  const picked = tabStore.activeState()?.pickedEntry ?? null;
  setRunState('running');
  const t0 = performance.now();
  try {
    const resp = await evalRequest({
      sources,
      key: active,
      entry: picked?.name,
      overrides: tabStore.activeState()?.entryOverrides ?? {},
      debug: debugMode,
    });
    resp.header.time = (performance.now() - t0) / 1000;
    handleHTTPResult(resp);
  } catch (e: any) {
    if (e instanceof DOMException && e.name === 'AbortError') return;
    // Surface the failure (server down, auth, network) instead of leaving the
    // UI silently idle with nothing rendered.
    showError(e);
  } finally {
    // Only the most recent run clears the state; a superseded run must not flip
    // the UI to idle while its successor is still running.
    if (gen === runGeneration) {
      setRunState('idle');
    }
  }
}

function handleHTTPResult(resp: EvalResponse) {
  const data = resp.header;

  // --- Check data (always present) ---
  editor.clearError();
  editor.clearMarkers();
  clearError();

  const errors = data.errors ?? [];
  if (errors.length > 0) editor.setMarkers(errors);

  // The editor's evalStore.subscribe (wired in setEditor) reacts to
  // this set() — symbols / varTypes / declarations / references /
  // sources all sync without an explicit push from here.
  evalStore.set(data);
  syncTabsWithSources(data);
  renderTabs(); // refresh tab icons (star, book, lock) from updated source kinds

  const fns = data.entryPoints ?? [];

  // Check-only response (no mesh, no debug)
  if (!data.mesh && !data.debugFinal) {
    handleCheckOnly(errors, fns);
    return;
  }

  const newPicked = onEntryPointsCb?.(fns);
  if (newPicked) tabStore.setPickedEntry(tabStore.active(), newPicked);

  if (data.debugSteps) {
    handleDebugHTTPResult(data, resp.binary);
  } else {
    handleEvalHTTPResult(data, resp.binary);
  }
}

function handleEvalHTTPResult(data: EvalResult, binary: ArrayBuffer) {
  setDebugBarVisible(false);
  const excludeFiles = stdlibExcludeFiles(data.sources);
  const decoded = data.mesh ? decodeBinaryMesh(binary, data.mesh) : null;
  viewer.applyEvalResult(decoded, data.posMap ?? [], { excludeFiles });
  if (data.stats && data.time !== undefined) showStats(data.stats, data.time);
}

function computeValidLines(steps: NonNullable<EvalResult['debugSteps']>) {
  validBreakpointLines.clear();
  for (const step of steps) {
    if (step.line <= 0) continue;
    const file = step.file || debugEntryTab;
    if (!validBreakpointLines.has(file)) validBreakpointLines.set(file, new Set());
    validBreakpointLines.get(file)!.add(step.line);
  }
  for (const [file, lines] of validBreakpointLines) {
    editor.setValidBreakpointLines(file, lines);
  }
  // Remove stale breakpoints (lines that no longer produce steps)
  for (const [file, bps] of breakpoints) {
    const valid = validBreakpointLines.get(file);
    for (const line of [...bps]) {
      if (!valid?.has(line)) bps.delete(line);
    }
    editor.syncBreakpoints(file, bps);
  }
}

function handleDebugHTTPResult(data: EvalResult, binary: ArrayBuffer) {
  setDebugBarVisible(false);
  debugBinary = binary;
  const steps = data.debugSteps ?? [];
  const finalMetas = data.debugFinal ?? [];

  // Decode final meshes from binary
  if (finalMetas.length > 0) {
    const finalMeshes: DecodedMesh[] = finalMetas.map(meta => decodeBinaryMesh(binary, meta));
    // Add a "Final" step with the decoded meshes (re-use existing debug step UI)
    steps.push({
      op: 'Final',
      meshes: [], // per-step meshes not available in binary response
      line: 0, col: 0, file: '',
    });
    data.debugSteps = steps;
    debugFinalMesh = finalMeshes.length === 1 ? finalMeshes[0] : null; // TODO: merge if multiple
  } else {
    data.debugSteps = steps;
    debugFinalMesh = null;
  }

  computeValidLines(steps);
  renderTabs();

  viewer.clearMeshes();
  if (debugFinalMesh) viewer.loadDecodedMesh(debugFinalMesh);
  if (steps.length > 0) {
    debugSlider.max = String(steps.length - 1);
    showDebugStep(0);
    setDebugBarVisible(true);
  }
}

/** Open a tab with the given key and source, switching to it if already open. */
function openTab(key: string, source: string, label?: string, readOnly?: boolean,
                 entry?: string, overrides?: Record<string, unknown>) {
  if (tabStore.has(key)) {
    switchToTab(key);
    return;
  }
  addTab({ path: key, dirty: false, cursor: null, label: label || tabLabel(key), pickedEntry: null, entryOverrides: {} });
  tabStore.setActive(key);
  editor.setCurrentSource(key);
  editor.switchModel(key, source);
  editor.setReadOnly(readOnly ?? isReadOnly(key));
  if (entry) tabStore.setPickedEntry(key, { name: entry, libPath: '' });
  if (overrides) tabStore.setEntryOverrides(key, overrides);
  markClean();
  updateWindowTitle();
  renderTabs();
  run();
}

export function openExample(source: string, name: string) {
  openTab('example:' + name, source, name.replace(/\.fct$/, ''), true);
}

/** Strips the directory and a known mesh/source extension to derive a tab label. */
function importBaseName(path: string): string {
  const base = path.split(/[\\/]/).pop() || 'Imported';
  return base.replace(/\.(fct|3mf|stl|obj)$/i, '') || 'Imported';
}

/**
 * Routes an opened file into the editor. Imported files (recovered 3MF projects
 * and generated mesh wrappers) become a new unsaved scratch project so Save
 * prompts for a .fct and never overwrites the mesh. A .fct opens in place.
 */
async function openOpenedFile(result: main.OpenedFile) {
  const overrides = result.overrides as Record<string, unknown> | undefined;
  if (result.imported) {
    const label = importBaseName(result.path);
    const key = await CreateScratchFile(label + '-' + Date.now());
    openTab(key, result.source, label, false, result.entry, overrides);
  } else {
    openTab(result.path, result.source, undefined, undefined, result.entry, overrides);
  }
  AddRecentFile(result.path).catch(err => reportError('AddRecentFile', err));
}

export async function openFile() {
  try {
    const result = await OpenFile();
    if (!result) return;
    await openOpenedFile(result);
  } catch (err) {
    reportError('OpenFile', err);
  }
}

export async function openRecentFile(path: string) {
  if (tabStore.has(path)) { switchToTab(path); return; }
  let result: main.OpenedFile | null;
  try {
    result = await OpenRecentFile(path);
  } catch (err) {
    reportError('OpenRecentFile', err); // e.g. a 3MF with a corrupt Facet part
    return;
  }
  if (!result) return;
  await openOpenedFile(result);
}

async function formatSource(source: string): Promise<string> {
  if (!formatOnSave || isReadOnly(tabStore.active())) return source;
  try {
    return await FormatCode(source);
  } catch (e) {
    // Save proceeds with the unformatted source, but surface the failure so the
    // user knows formatting was skipped instead of it happening silently.
    reportError('FormatCode', e);
    return source;
  }
}

/** Core save logic. Pass forceDialog=true for Save As. */
async function doSave(forceDialog: boolean) {
  try {
    await doSaveInner(forceDialog);
  } catch (err) {
    // Surface the failure (disk full, permission denied, …) instead of leaving
    // the promise to reject silently. markClean() runs only on success below, so
    // the document correctly stays dirty.
    reportError('SaveFile', err);
  }
}

async function doSaveInner(forceDialog: boolean) {
  const source = await formatSource(editor.getContent());
  editor.setContentSilent(source);
  const active = tabStore.active();
  const tab = getTab(active);
  const savePath = forceDialog ? '' : (await IsScratchFile(tab.path) ? '' : tab.path);
  const path = await SaveFile(source, savePath);
  if (!path) return;
  if (path !== tab.path) {
    const oldKey = active;
    // Create a model under the new path and switch to it before disposing the old one
    editor.switchModel(path, source);
    editor.setCurrentSource(path);
    editor.disposeModel(oldKey);
    removeTab(oldKey);
    addTab({ path, dirty: false, cursor: tab.cursor, label: tabLabel(path), pickedEntry: null, entryOverrides: {} });
    tabStore.setActive(path);
  }
  // Patch the cached source text so subsequent reads see the saved
  // version without waiting for the next eval to refresh it.
  const finalActive = tabStore.active();
  const cached = evalStore.current();
  const cachedSource = cached?.sources?.[finalActive];
  if (cachedSource) cachedSource.text = source;
  markClean();
  updateWindowTitle();
  renderTabs();
  AddRecentFile(path).catch(err => reportError('AddRecentFile', err));
}

export function saveFile() { return doSave(false); }
export function saveFileAs() { return doSave(true); }

export async function newFile() {
  try {
    const key = await CreateScratchFile('Untitled-' + Date.now());
    openTab(key, '', 'Untitled', false);
    patchSettings({ activeTab: key });
  } catch (err) {
    reportError('NewFile', err);
  }
}

export function showError(err: unknown) {
  const msg = (err as any)?.message || String(err);
  errorDiv.textContent = msg;
  errorDiv.style.display = 'block';
  onEvalStatusCb?.('error');
}

function clearError() {
  errorDiv.style.display = 'none';
  errorDiv.textContent = '';
  errorDiv.onclick = null;
  errorDiv.style.cursor = '';
}

// currentEvalErrorsText returns every error from the most recent evaluation,
// one located line each. The AI assistant sends this so it sees the full list,
// not just the first error the on-screen bar displays. '' when there are none.
export function currentEvalErrorsText(): string {
  return formatSourceErrors(evalStore.current()?.errors ?? []);
}

function currentEvalParams() {
  const state = tabStore.activeState();
  return {
    sources: editor.getAllSources(),
    key: tabStore.active(),
    entry: state?.pickedEntry?.name ?? '',
    overrides: state?.entryOverrides ?? {},
  };
}

export async function exportMesh(format: string = '3mf', embedSource: boolean = true) {
  try {
    const p = currentEvalParams();
    await ExportMesh(format, p.sources, p.key, p.entry, p.overrides, embedSource);
  } catch (err) {
    showError(err);
  }
}

// shareToWeb shows a QR popover for the active tab's buffer; scanning it with
// a phone or clicking it opens the hosted web preview with the source
// rendered (transported in the URL hash).
export async function shareToWeb(anchorEl: HTMLElement) {
  try {
    const link = await BuildShareLink(editor.getContent());
    showSharePopover(link, anchorEl);
  } catch (err) {
    showError(err);
  }
}

export async function sendToSlicer(id: string) {
  try {
    const p = currentEvalParams();
    await SendToSlicer(id, p.sources, p.key, p.entry, p.overrides);
  } catch (err) {
    showError(err);
  }
}

export function toggleDebug() {
  debugMode = !debugMode;
  if (debugMode) {
    debugEntryTab = tabStore.active();
    editor.setBreakpointMode(true);
  } else {
    editor.setBreakpointMode(false);
    validBreakpointLines.clear();
    setDebugBarVisible(false);
    editor.clearDebugLine();

    // Restore the final mesh with normal materials instead of re-evaluating
    if (debugFinalMesh) {
      viewer.clearMeshes();
      viewer.loadDecodedMesh(debugFinalMesh);
      viewer.fitToView();
    }
    evalStore.set(null);
    debugFinalMesh = null;

    // Jump back to the tab that had the entry point
    if (debugEntryTab && !tabStore.isActive(debugEntryTab) && tabStore.has(debugEntryTab)) {
      switchToTab(debugEntryTab);
    }
    debugEntryTab = '';
  }
  return debugMode;
}

async function openDocs(): Promise<void> {
  if (docsPanel.isVisible()) return;
  // The Docs panel browses the FULL catalog (stdlib + all installed
  // libraries) regardless of whether the current source imports them —
  // that's how the user discovers libraries to import in the first
  // place. The /eval response's `symbols` table is scoped to what the
  // loader actually resolved (the editor's source of truth) and would
  // omit libraries the user is browsing; use GetDocCatalog instead.
  const [entries, guides] = await Promise.all([
    GetDocCatalog().catch(err => { reportError('GetDocCatalog', err); return []; }),
    GetDocGuides().catch(err => { reportError('GetDocGuides', err); return []; }),
  ]);
  docsPanel.show(entries, guides);
  setDebugBarVisible(false);
}

export async function toggleDocs() {
  if (!docsPanel.isVisible()) {
    await openDocs();
  } else {
    docsPanel.hide();
    if (debugMode && (evalStore.current()?.debugSteps ?? []).length > 0) {
      setDebugBarVisible(true);
    }
  }
  return docsPanel.isVisible();
}

export async function openDocsToEntry(name: string, library?: string): Promise<void> {
  await openDocs();
  docsPanel.focusEntry(name, library);
}

// ── State accessors for external UI (file tree, preview selector) ──────────

export function getSources(): Record<string, SourceEntry> { return evalStore.current()?.sources ?? {}; }
export function getActiveTabValue(): string { return tabStore.active(); }
export function isActiveTabReadOnly(): boolean { return isReadOnly(tabStore.active()); }
export function getActiveLabel(): string {
  const active = tabStore.active();
  const tab = tabStore.activeState();
  return tab ? tab.label || tabLabel(active) : 'Untitled';
}

/**
 * Create a new editable scratch tab from the assistant and load it with the
 * given source. Used by the new_file MCP tool. Returns the tab key so the
 * caller can confirm placement.
 */
export async function assistantCreateFile(name: string, source: string): Promise<string> {
  // Strip any path separators defensively — scratch files are bare names.
  const safeName = name.replace(/[\/\\]/g, '_');
  const base = safeName.replace(/\.fct$/i, '');
  const key = await CreateScratchFile(base + '-' + Date.now());
  openTab(key, source, base, false);
  patchSettings({ activeTab: key });
  return key;
}

export function isDebugStepping(): boolean { return debugStepping; }

export function setOnSourceChange(cb: (source: string) => void) { onSourceChangeCb = cb; }
export function setOnDebugFilesChange(cb: () => void) { onDebugFilesChangeCb = cb; }
export function setOnDebugExit(cb: () => void) { onDebugExitCb = cb; }
export function setOnEntryPoints(cb: (fns: EntryPoint[]) => { name: string; libPath: string } | null) { onEntryPointsCb = cb; }
export function setEntryOverrides(overrides: Record<string, unknown>) {
  const active = tabStore.active();
  if (active) tabStore.setEntryOverrides(active, overrides);
}



/** Open a library tab without navigating to a specific line.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export function openLibraryTab(file: string, source: string) {
  const sources = evalStore.current()?.sources ?? {};
  // If file is an import path (e.g. "facet/gears"), resolve to disk path
  if (!sources[file]) {
    for (const [diskPath, entry] of Object.entries(sources)) {
      if (entry.importPath === file) {
        file = diskPath;
        break;
      }
    }
  }
  if (sources[file] && !source) {
    source = sources[file].text;
  }
  getTab(file);
  switchToTab(file);
}

/** Open a library file in a read-only tab and navigate to the given position.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export function openLibraryFile(file: string, source: string, line: number, col: number) {
  openLibraryTab(file, source);
  editor.revealLine(line, col);
}

export function closeActiveTab() {
  const active = tabStore.active();
  if (active) closeTab(active);
}
