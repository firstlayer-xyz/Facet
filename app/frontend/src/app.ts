// app.ts — Run/debug orchestration logic.

import { ConfirmDiscard, OpenFile, OpenRecentFile, AddRecentFile, SaveFile, ExportMesh, SendToSlicer, GetDocGuides, SetWindowTitle, FormatCode, CreateScratchFile, IsScratchFile, SetDirtyState } from '../wailsjs/go/main/App';
import type { EntryPoint } from './function-preview';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { Viewer } from './viewer';
import type { DecodedMesh, DebugStepData } from './viewer';
import type { EditorHandle } from './editor';
import { DocsPanel } from './docs';
import { patchSettings } from './settings';
import { evalRequest, cancelEval } from './eval-client';
import type { EvalResponse } from './eval-client';
import { decodeBinaryMesh } from './mesh-decode';
import type { BinaryMeshMeta } from './mesh-decode';

interface SourceEntry {
  text: string;
  kind: number;
  importPath?: string;
}

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

// Dependencies injected via initApp()
let viewer: Viewer;
let editor: EditorHandle;
let docsPanel: DocsPanel;

// DOM elements injected via initApp()
let errorDiv: HTMLElement;
let debugBar: HTMLElement;
let debugSlider: HTMLInputElement;
let debugLabel: HTMLElement;
let centerBtn: HTMLElement;
let autoRotateBtn: HTMLElement;
let tabBar: HTMLElement;
let statsBar: HTMLElement;
let compilingOverlay: HTMLElement;

// Debug mode state
let debugMode = false;
let debugStepping = false; // true while showDebugStep is switching tabs
let debugEntryTab = ''; // tab that was active when debug started
let debugFinalMesh: DecodedMesh | null = null;
let debugBinary: ArrayBuffer | null = null;
let debugStepIndex = 0;
interface TabState {
  path: string;       // resolved filesystem path
  dirty: boolean;
  cursor: { lineNumber: number; column: number } | null;
  label: string;
  pickedEntry: { name: string; libPath: string } | null;
}
let tabs: Record<string, TabState> = {};
let tabOrder: string[] = []; // explicit ordering for drag reorder
let activeTab = '';

/** Get or create a tab. Use `ensureTab` when you only need the side effect. */
function getTab(key: string): TabState {
  if (!tabs[key]) {
    tabs[key] = { path: key, dirty: false, cursor: null, label: tabLabel(key), pickedEntry: null };
    if (!tabOrder.includes(key)) tabOrder.push(key);
  }
  return tabs[key];
}
const ensureTab = getTab;

function addTab(key: string, state: TabState) {
  tabs[key] = state;
  if (!tabOrder.includes(key)) tabOrder.push(key);
}

function removeTab(key: string) {
  delete tabs[key];
  tabOrder = tabOrder.filter(k => k !== key);
}

function isReadOnly(path: string): boolean {
  return isReadOnlyKind(lastResult?.sources?.[path]?.kind ?? SOURCE_USER);
}

function isDirty(): boolean {
  return tabs[activeTab]?.dirty ?? false;
}

// Cached result from last run:result event
let lastResult: any = null;

// Preview lock — when true, switching editor tabs does not auto-run the new tab's source
let previewLocked = false;

// Callbacks for external UI components
let onTabChangeCb: ((tab: string) => void) | null = null;
let onSourceChangeCb: ((source: string) => void) | null = null;
let onDebugFilesChangeCb: (() => void) | null = null;
let onDebugExitCb: (() => void) | null = null;
// Returns the picked entry point name (or null to skip running).
let onEntryPointsCb: ((fns: EntryPoint[]) => { name: string; libPath: string } | null) | null = null;


// Entry point overrides (slider values for constrained function params).
// The entry point name itself is NOT stored here — it flows through function
// parameters to Run()/Debug(), so it's impossible to eval without one.
let entryOverrides: Record<string, any> = {};
/** Set the file path on startup (no discard prompt, no re-persist). */
export function setInitialFile(path: string, label?: string, readOnly?: boolean) {
  addTab(path, { path, dirty: false, cursor: null, label: label || tabLabel(path), pickedEntry: null });
  activeTab = path;
  editor.setCurrentSource(path);
  editor.setReadOnly(readOnly ?? isReadOnly(path));
  updateWindowTitle();
  renderTabs();
}

/** Register a tab restored from saved state without switching the editor or triggering a run.
 *  The Monaco model should already be pre-created via editor.switchModel(). */
export function addRestoredTab(path: string, cursor: { lineNumber: number; column: number } | null) {
  addTab(path, { path, dirty: false, cursor, label: tabLabel(path), pickedEntry: null });
}

function anyDirty(): boolean {
  return Object.values(tabs).some(t => t.dirty);
}

function markDirty() {
  const tab = tabs[activeTab];
  if (tab && !tab.dirty) {
    tab.dirty = true;
    updateWindowTitle();
    SetDirtyState(true);
  }
}
function markClean() {
  const tab = tabs[activeTab];
  if (tab && tab.dirty) {
    tab.dirty = false;
    updateWindowTitle();
    SetDirtyState(anyDirty());
  }
}
interface SavedTab {
  path: string;
  label: string;
  cursor: { lineNumber: number; column: number } | null;
}

function persistOpenTabs() {
  const sources = lastResult?.sources ?? {};
  // Save cursor for active tab before persisting
  if (activeTab && tabs[activeTab]) {
    tabs[activeTab].cursor = editor.getCursorPosition();
  }
  // Persist all tabs except stdlib (kind=1) and cached libs (kind=3)
  const savedTabs: SavedTab[] = [];
  for (const path of tabOrder) {
    const tab = tabs[path];
    if (!tab) continue;
    const kind = sources[path]?.kind ?? SOURCE_USER;
    if (isEphemeralKind(kind)) continue;
    savedTabs.push({ path: tab.path, label: tab.label, cursor: tab.cursor });
  }
  patchSettings({ savedTabs, activeTab });
}

function updateWindowTitle() {
  const tab = tabs[activeTab];
  const name = tab ? tab.label || tabLabel(activeTab) : 'Untitled';
  const prefix = isDirty() ? '\u25cf ' : '';
  SetWindowTitle(`${prefix}${name} \u2014 Facet`);
}

// Run state — driven by Go-side events ("run:start" / "run:idle")
type RunState = 'idle' | 'running';
let runState: RunState = 'idle';

const PLAY_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`;
const STOP_ICON = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12"/></svg>`;

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
  centerBtn: HTMLElement;
  autoRotateBtn: HTMLElement;
  tabBar: HTMLElement;
  statsBar: HTMLElement;
  compilingOverlay: HTMLElement;
}

export function initApp(deps: AppDeps) {
  viewer = deps.viewer;
  editor = deps.editor;
  docsPanel = deps.docsPanel;
  errorDiv = deps.errorDiv;
  debugBar = deps.debugBar;
  debugSlider = deps.debugSlider;
  debugLabel = deps.debugLabel;
  centerBtn = deps.centerBtn;
  autoRotateBtn = deps.autoRotateBtn;
  tabBar = deps.tabBar;
  statsBar = deps.statsBar;
  compilingOverlay = deps.compilingOverlay;

  // Persist tabs when app is about to close
  EventsOn('app:before-close', () => persistOpenTabs());

}

function setSpinner(active: boolean) {
  compilingOverlay.style.display = active ? 'flex' : 'none';
}

/** Allow main.ts to set the editor after async creation */
export function setEditor(ed: EditorHandle) {
  editor = ed;

  // Wire mouse hover to face highlighting (debounced to one frame)
  let highlightRAF = 0;
  editor.onMouseMove((line: number, col: number) => {
    if (highlightMode !== 'mouse') return;
    if (highlightRAF) cancelAnimationFrame(highlightRAF);
    highlightRAF = requestAnimationFrame(() => {
      viewer.highlightAtPos(activeTab, line, col);
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
      viewer.highlightAtPos(activeTab, line, col);
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

function showStats(stats: { triangles: number; vertices: number; volume: number; surfaceArea: number }, time: number) {
  statsBar.innerHTML =
    `${formatTime(time)}  \u00B7  ${stats.triangles.toLocaleString()} tris<br>` +
    `${formatVolume(stats.volume)}  \u00B7  ${formatArea(stats.surfaceArea)}`;
  statsBar.style.display = 'block';
}

function hideStats() {
  statsBar.style.display = 'none';
}

function setDebugBarVisible(visible: boolean) {
  debugBar.style.display = visible ? 'flex' : 'none';
  centerBtn.classList.toggle('above-debug-bar', visible);
  autoRotateBtn.classList.toggle('above-debug-bar', visible);
  statsBar.classList.toggle('above-debug-bar', visible);
  const fnBar = document.getElementById('fn-preview-bar');
  if (fnBar) fnBar.classList.toggle('above-debug-bar', visible);
  const htBtn = document.getElementById('head-track-btn');
  if (htBtn) htBtn.classList.toggle('above-debug-bar', visible);
}


function syncTabsWithSources(data: any) {
  if (!data.sources) return;
  let tabsClosed = false;
  for (const path of Object.keys(tabs)) {
    if (path === activeTab) continue;
    if (!data.sources[path]) {
      editor.disposeModel(path);
      removeTab(path);
      tabsClosed = true;
    }
  }
  if (tabsClosed) renderTabs();
}

function pushEditorData(data: any) {
  if (data.docIndex) editor.updateDocIndex(data.docIndex);
  if (data.varTypes && Object.keys(data.varTypes).length > 0) editor.updateVarTypes(data.varTypes);
  if (data.declarations?.decls) editor.updateDeclarations(data.declarations.decls);
  if (data.sources) {
    const textSources: Record<string, string> = {};
    for (const [k, v] of Object.entries(data.sources as Record<string, SourceEntry>)) {
      textSources[k] = v.text;
    }
    editor.updateFileSources(textSources);
    onSourceChangeCb?.(editor.getContent());
  }
}

function handleCheckOnly(_data: any, errors: any[], fns: EntryPoint[]) {
  viewer.setPosMap([]);
  if (errors.length > 0) {
    const e = errors[0];
    hideStats();
    const prefix = e.file ? `[${e.file}${e.line > 0 ? ':' + e.line : ''}] ` : '';
    showError(prefix + e.message);
    if (e.line > 0 && !e.file) {
      errorDiv.style.cursor = 'pointer';
      editor.highlightError(e.line);
      errorDiv.onclick = () => editor.revealLine(e.line, e.col || 1);
    } else if (e.line > 0 && e.file) {
      errorDiv.style.cursor = 'pointer';
      errorDiv.onclick = () => {
        ensureTab(e.file);
        switchToTab(e.file);
        editor.highlightError(e.line);
        editor.revealLine(e.line, e.col || 1);
      };
    }
    return;
  }
  const picked = onEntryPointsCb?.(fns);
  if (picked) {
    getTab(activeTab).pickedEntry = picked;
    runViaHTTP(); // re-run with the picked entry point
  }
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
  const openTabs = tabOrder;

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
    tab.className = 'tab' + (activeTab === path ? ' active' : '');
    tab.title = path;

    const sourceKind = lastResult?.sources?.[path]?.kind ?? SOURCE_USER;
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
      const fromIdx = tabOrder.indexOf(draggedPath);
      const toIdx = tabOrder.indexOf(path);
      if (fromIdx < 0 || toIdx < 0) return;
      tabOrder.splice(fromIdx, 1);
      tabOrder.splice(toIdx, 0, draggedPath);
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
  if (activeTab === file) {
    // Switch to another open tab
    const remaining = tabOrder.filter(k => k !== file);
    if (remaining.length > 0) {
      switchToTab(remaining[0]);
    } else {
      activeTab = '';
    }
  }
  // Cancel any in-flight eval if the active tab is being closed
  if (file === activeTab) cancelEval();

  editor.disposeModel(file);
  removeTab(file);

  // Exit debug mode if the entry-point tab was closed
  if (debugMode && file === debugEntryTab) {
    debugMode = false;
    setDebugBarVisible(false);
    editor.clearDebugLine();
    viewer.clearMeshes();
    viewer.setPosMap([]);
    hideStats();
    debugFinalMesh = null;
    debugEntryTab = '';
    lastResult = null;
    onDebugExitCb?.();
  }

  renderTabs();

  // Clear viewport if no tabs remain
  if (Object.keys(tabs).length === 0) {
    viewer.clearMeshes();
    viewer.setPosMap([]);
    hideStats();
    clearError();
    setDebugBarVisible(false);
  }
  // Notify file tree / preview of the tab change
  onTabChangeCb?.(activeTab);
}

export function switchToTab(file: string) {
  if (file === activeTab) return;

  // Save cursor position for the tab we're leaving
  getTab(activeTab).cursor = editor.getCursorPosition();

  activeTab = file;
  editor.setCurrentSource(file);

  // Switch editor model — source comes from sources map or editor's own model cache
  const source = lastResult?.sources?.[file]?.text ?? '';
  editor.switchModel(file, source);
  editor.setReadOnly(isReadOnly(file));

  // Restore cursor position
  const saved = getTab(file).cursor;
  if (saved) {
    editor.revealLine(saved.lineNumber, saved.column);
  }

  renderTabs();

  // Update declarations from cached data so Go to Declaration works
  if (lastResult?.declarations?.decls) {
    editor.updateDeclarations(lastResult.declarations.decls);
  }

  // Re-highlight current debug step line if it belongs to this tab
  const steps = lastResult?.debugSteps ?? [];
  if (steps.length > 0 && debugStepIndex < steps.length) {
    const step = steps[debugStepIndex];
    if ((step.file ?? '') === activeTab && step.line > 0) {
      editor.highlightDebugLine(step.line);
    } else {
      editor.clearDebugLine();
    }
  }

  // Notify external UI (file tree, preview selector) of the tab change
  onTabChangeCb?.(activeTab);
}

export function showDebugStepPrev() {
  showDebugStep(debugStepIndex - 1);
}

export function showDebugStepNext() {
  showDebugStep(debugStepIndex + 1);
}

export async function showDebugStep(index: number) {
  const debugSteps = lastResult?.debugSteps ?? [];
  if (index < 0 || index >= debugSteps.length) return;
  debugStepIndex = index;
  debugSlider.value = String(index);
  debugLabel.textContent = `Step ${index + 1}/${debugSteps.length}: ${debugSteps[index].op}`;

  // Display the step — per-step mesh lazy loading is not yet implemented for HTTP eval.
  const step = debugSteps[index];
  if (step.op === 'Final' && debugFinalMesh) {
    viewer.clearMeshes();
    viewer.loadDecodedMesh(debugFinalMesh);
  } else if (step.meshes && step.meshes.length > 0 && debugBinary) {
    viewer.loadDebugStep(step, debugBinary);
  }
  viewer.centerOnBed();
  viewer.fitToView();

  const stepFile = debugSteps[index].file ?? '';
  if (stepFile && stepFile !== activeTab) {
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

/** Trigger evaluation with the given entry point. */
export function reeval(entry: string, libPath?: string) {
  if (debounceTimer) { clearTimeout(debounceTimer); debounceTimer = null; }
  getTab(activeTab).pickedEntry = { name: entry, libPath: libPath || '' };
  runViaHTTP();
}

export function toggleRun() {
  if (runState === 'running') {
    cancelEval();
  } else {
    run();
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

async function runViaHTTP() {
  const sources = editor.getAllSources();
  const picked = tabs[activeTab]?.pickedEntry ?? null;
  setRunState('running');
  const t0 = performance.now();
  try {
    const resp = await evalRequest({
      sources,
      key: activeTab,
      entry: picked?.name,
      overrides: entryOverrides,
      debug: debugMode,
    });
    resp.header.time = (performance.now() - t0) / 1000;
    handleHTTPResult(resp);
  } catch (e: any) {
    if (e instanceof DOMException && e.name === 'AbortError') return;
    console.error('eval request failed:', e);
  } finally {
    setRunState('idle');
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

  lastResult = data;
  syncTabsWithSources(data);
  pushEditorData(data);
  renderTabs(); // refresh tab icons (star, book, lock) from updated source kinds

  const fns = (data.entryPoints ?? []) as EntryPoint[];

  // Check-only response (no mesh, no debug)
  if (!data.mesh && !data.debugFinal) {
    handleCheckOnly(data, errors, fns);
    return;
  }

  const newPicked = onEntryPointsCb?.(fns);
  if (newPicked) getTab(activeTab).pickedEntry = newPicked;

  if (data.debugSteps) {
    handleDebugHTTPResult(data, resp.binary);
  } else {
    handleEvalHTTPResult(data, resp.binary);
  }
}

function handleEvalHTTPResult(data: any, binary: ArrayBuffer) {
  setDebugBarVisible(false);
  viewer.clearMeshes();
  if (data.mesh) {
    const decoded = decodeBinaryMesh(binary, data.mesh as BinaryMeshMeta);
    viewer.loadDecodedMesh(decoded);
  }
  const excludeFiles = new Set<string>();
  if (data.sources) {
    for (const [path, entry] of Object.entries(data.sources as Record<string, SourceEntry>)) {
      if (entry.kind === SOURCE_STDLIB) excludeFiles.add(path);
    }
  }
  viewer.setPosMap(data.posMap ?? [], excludeFiles);
  viewer.centerOnBed();
  viewer.fitToView();
  showStats(data.stats, data.time);
}

function handleDebugHTTPResult(data: any, binary: ArrayBuffer) {
  setDebugBarVisible(false);
  debugBinary = binary;
  const steps = (data.debugSteps ?? []) as unknown as DebugStepData[];
  const finalMetas = (data.debugFinal ?? []) as BinaryMeshMeta[];

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
function openTab(key: string, source: string, label?: string, readOnly?: boolean) {
  if (tabs[key]) {
    switchToTab(key);
    return;
  }
  addTab(key, { path: key, dirty: false, cursor: null, label: label || tabLabel(key), pickedEntry: null });
  activeTab = key;
  editor.setCurrentSource(key);
  editor.switchModel(key, source);
  editor.setReadOnly(readOnly ?? isReadOnly(key));
  markClean();
  updateWindowTitle();
  renderTabs();
  run();
}

export function openExample(source: string, name: string) {
  openTab('example:' + name, source, name.replace(/\.fct$/, ''), true);
}

export async function openFile() {
  const result = await OpenFile();
  if (!result) return;
  openTab(result.path, result.source);
  AddRecentFile(result.path).catch(() => {});
}

export async function openRecentFile(path: string) {
  if (tabs[path]) { switchToTab(path); return; }
  let result: Record<string, string>;
  try {
    result = await OpenRecentFile(path);
  } catch {
    return; // file may no longer exist
  }
  openTab(result.path, result.source);
  AddRecentFile(result.path).catch(() => {});
}

async function formatSource(source: string): Promise<string> {
  if (!formatOnSave || isReadOnly(activeTab)) return source;
  try {
    return await FormatCode(source);
  } catch (e) {
    console.warn('FormatCode failed:', e);
    return source;
  }
}

/** Core save logic. Pass forceDialog=true for Save As. */
async function doSave(forceDialog: boolean) {
  const source = await formatSource(editor.getContent());
  editor.setContentSilent(source);
  const tab = getTab(activeTab);
  const savePath = forceDialog ? '' : (await IsScratchFile(tab.path) ? '' : tab.path);
  const path = await SaveFile(source, savePath);
  if (!path) return;
  if (path !== tab.path) {
    const oldKey = activeTab;
    // Create a model under the new path and switch to it before disposing the old one
    editor.switchModel(path, source);
    editor.setCurrentSource(path);
    editor.disposeModel(oldKey);
    removeTab(oldKey);
    addTab(path, { path, dirty: false, cursor: tab.cursor, label: tabLabel(path), pickedEntry: null });
    activeTab = path;
  }
  if (lastResult?.sources?.[activeTab]) {
    lastResult.sources[activeTab].text = source;
  }
  markClean();
  updateWindowTitle();
  renderTabs();
  AddRecentFile(path).catch(() => {});
}

export function saveFile() { return doSave(false); }
export function saveFileAs() { return doSave(true); }

export async function newFile() {
  const key = await CreateScratchFile('Untitled-' + Date.now());
  openTab(key, '', 'Untitled', false);
  patchSettings({ activeTab: key });
}

export function showError(err: unknown) {
  const msg = (err as any)?.message || String(err);
  errorDiv.textContent = msg;
  errorDiv.style.display = 'block';
}

function clearError() {
  errorDiv.style.display = 'none';
  errorDiv.textContent = '';
  errorDiv.onclick = null;
  errorDiv.style.cursor = '';
}

function currentEvalParams() {
  const picked = tabs[activeTab]?.pickedEntry;
  return {
    sources: editor.getAllSources(),
    key: activeTab,
    entry: picked?.name ?? '',
    overrides: entryOverrides,
  };
}

export async function exportMesh(format: string = '3mf') {
  try {
    const p = currentEvalParams();
    await ExportMesh(format, p.sources, p.key, p.entry, p.overrides);
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
    debugEntryTab = activeTab;
  } else {
    setDebugBarVisible(false);
    editor.clearDebugLine();

    // Restore the final mesh with normal materials instead of re-evaluating
    if (debugFinalMesh) {
      viewer.clearMeshes();
      viewer.loadDecodedMesh(debugFinalMesh);
      viewer.centerOnBed();
      viewer.fitToView();
    }
    lastResult = null;
    debugFinalMesh = null;

    // Jump back to the tab that had the entry point
    if (debugEntryTab && debugEntryTab !== activeTab && tabs[debugEntryTab]) {
      switchToTab(debugEntryTab);
    }
    debugEntryTab = '';
  }
  return debugMode;
}

async function openDocs(): Promise<void> {
  if (docsPanel.isVisible()) return;
  const guides = await GetDocGuides().catch(() => [] as any[]);
  docsPanel.show(lastResult?.docIndex ?? [], guides);
  viewer.setVisible(false);
  setDebugBarVisible(false);
}

export async function toggleDocs() {
  if (!docsPanel.isVisible()) {
    await openDocs();
  } else {
    docsPanel.hide();
    viewer.setVisible(true);
    if (debugMode && (lastResult?.debugSteps ?? []).length > 0) {
      setDebugBarVisible(true);
    }
  }
  return docsPanel.isVisible();
}

export async function openDocsToEntry(name: string): Promise<void> {
  await openDocs();
  docsPanel.focusEntry(name);
}

// ── State accessors for external UI (file tree, preview selector) ──────────

export function getSources(): Record<string, SourceEntry> { return lastResult?.sources ?? {}; }
export function getActiveTabValue(): string { return activeTab; }
export function getActiveLabel(): string {
  const tab = tabs[activeTab];
  return tab ? tab.label || tabLabel(activeTab) : 'Untitled';
}

export function isPreviewLocked(): boolean { return previewLocked; }
export function isDebugStepping(): boolean { return debugStepping; }
export function setPreviewLocked(locked: boolean) {
  previewLocked = locked;
}

export function setOnTabChange(cb: (tab: string) => void) { onTabChangeCb = cb; }
export function setOnSourceChange(cb: (source: string) => void) { onSourceChangeCb = cb; }
export function setOnDebugFilesChange(cb: () => void) { onDebugFilesChangeCb = cb; }
export function setOnDebugExit(cb: () => void) { onDebugExitCb = cb; }
export function setOnEntryPoints(cb: (fns: EntryPoint[]) => { name: string; libPath: string } | null) { onEntryPointsCb = cb; }
export function setEntryOverrides(overrides: Record<string, any>) { entryOverrides = overrides; }



/** Open a library tab without navigating to a specific line.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export function openLibraryTab(file: string, source: string) {
  const sources = lastResult?.sources ?? {};
  // If file is an import path (e.g. "facet/gears"), resolve to disk path
  if (!sources[file]) {
    for (const [diskPath, entry] of Object.entries(sources)) {
      if ((entry as SourceEntry).importPath === file) {
        file = diskPath;
        break;
      }
    }
  }
  if (sources[file] && !source) {
    source = sources[file].text;
  }
  ensureTab(file);
  switchToTab(file);
}

/** Open a library file in a read-only tab and navigate to the given position.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export function openLibraryFile(file: string, source: string, line: number, col: number) {
  openLibraryTab(file, source);
  editor.revealLine(line, col);
}
