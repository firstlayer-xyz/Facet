// app.ts — Run/debug orchestration logic.

import { Stop, ConfirmDiscard, OpenFile, OpenRecentFile, AddRecentFile, SaveFile, ExportMesh, SendToSlicer, GetDocGuides, GetDebugStepMeshes, SetWindowTitle, IsReadOnlyPath, GetLibraryFilePath, FormatCode, UpdateSource, Run, Debug, ResetRunner, CreateScratchFile, IsScratchFile } from '../wailsjs/go/main/App';
import type { EntryPoint } from './function-preview';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { Viewer, mergeMeshes } from './viewer';
import type { DecodedMesh, DebugStepData, MeshData } from './viewer';
import type { EditorHandle } from './editor';
import { DocsPanel } from './docs';
import { patchSettings } from './settings';

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
let debugBtn: HTMLElement;
let tabBar: HTMLElement;
let statsBar: HTMLElement;
let compilingOverlay: HTMLElement;

// Debug mode state
let debugMode = false;
let debugFinalMesh: DecodedMesh | null = null;
let debugStepIndex = 0;
let debugStepGen = 0;
interface TabState {
  path: string;       // resolved filesystem path
  readOnly: boolean;
  dirty: boolean;
  cursor: { lineNumber: number; column: number } | null;
  label: string;
}
let tabs: Record<string, TabState> = {};
let activeTab = '';

function getTab(key: string): TabState {
  if (!tabs[key]) tabs[key] = { path: key, readOnly: true, dirty: false, cursor: null, label: tabLabel(key) };
  return tabs[key];
}

// Dirty flag for the active main file
let dirty = false;

// Cached result from last run:result event
let lastResult: any = null;

// Preview lock — when true, switching editor tabs does not auto-run the new tab's source
let previewLocked = false;
let lockedTab = ''; // the activeTab captured when lock was engaged

// Callbacks for external UI components
let onTabChangeCb: ((tab: string) => void) | null = null;
let onSourceChangeCb: ((source: string) => void) | null = null;
let onDebugFilesChangeCb: (() => void) | null = null;
// Returns the picked entry point name (or null to skip running).
let onEntryPointsCb: ((fns: EntryPoint[]) => { name: string; libPath: string } | null) | null = null;


// Entry point overrides (slider values for constrained function params).
// The entry point name itself is NOT stored here — it flows through function
// parameters to Run()/Debug(), so it's impossible to eval without one.
let entryOverrides: Record<string, any> = {};

/** Set the active tab to a file path, creating the tab if needed. */
async function setActiveFile(path: string, label?: string) {
  const ro = path ? await IsReadOnlyPath(path) : false;
  tabs[path] = { path, readOnly: ro, dirty: false, cursor: null, label: label || tabLabel(path) };
  activeTab = path;
  editor.setReadOnly(ro);
  patchSettings({ lastFile: path });
}

/** Set the file path on startup (no discard prompt, no re-persist). */
export async function setInitialFile(path: string) {
  const ro = path ? await IsReadOnlyPath(path) : false;
  tabs[path] = { path, readOnly: ro, dirty: false, cursor: null, label: tabLabel(path) };
  activeTab = path;
  editor.setReadOnly(ro);
  updateWindowTitle();
}

function markDirty() {
  if (!dirty) {
    dirty = true;
    const tab = tabs[activeTab];
    if (tab) tab.dirty = true;
    updateWindowTitle();
  }
}
function markClean() {
  if (dirty) {
    dirty = false;
    const tab = tabs[activeTab];
    if (tab) tab.dirty = false;
    updateWindowTitle();
  }
}
function hasUnsavedChanges(): boolean {
  if (dirty) return true;
  return Object.values(tabs).some(t => t.dirty);
}
async function confirmDiscardAsync(): Promise<boolean> {
  if (!hasUnsavedChanges()) return true;
  return ConfirmDiscard();
}
function updateWindowTitle() {
  const tab = tabs[activeTab];
  const name = tab ? tab.label || tabLabel(activeTab) : 'Untitled';
  const prefix = dirty ? '\u25cf ' : '';
  SetWindowTitle(`${prefix}${name} \u2014 Facet`);
}

// Docs state
let docsMode = false;

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
  debugBtn: HTMLElement;
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
  debugBtn = deps.debugBtn;
  tabBar = deps.tabBar;
  statsBar = deps.statsBar;
  compilingOverlay = deps.compilingOverlay;

  // Run state driven by Go-side events
  EventsOn('run:start', () => setRunState('running'));
  EventsOn('run:idle', () => setRunState('idle'));

  // Single result event — carries check data, eval data, or both
  EventsOn('run:result', (data: any) => handleRunResult(data));

}

/** Feedback from a completed run, used by the auto-verify loop. */
export interface RunFeedback {
  success: boolean;
  stats?: {
    triangles: number; vertices: number; volume: number; surfaceArea: number;
    bboxMin: [number, number, number]; bboxMax: [number, number, number];
  };
  time?: number;
  error?: string;
}

type RunFeedbackCallback = (feedback: RunFeedback) => void;
let pendingRunCallback: RunFeedbackCallback | null = null;

/**
 * Register a one-shot listener that fires after the next run completes.
 * The callback receives the run result (success + stats or error).
 * Returns a cancel function.
 */
export function onNextRunComplete(cb: RunFeedbackCallback): () => void {
  pendingRunCallback = cb;
  return () => { pendingRunCallback = null; };
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

function handleRunResult(data: any) {
  // --- Check data (always present) ---
  editor.clearError();
  editor.clearMarkers();
  errorDiv.style.display = 'none';
  errorDiv.textContent = '';
  errorDiv.onclick = null;
  errorDiv.style.cursor = '';

  // Markers from check errors
  const errors = data.errors ?? [];
  if (errors.length > 0) editor.setMarkers(errors);

  lastResult = data;

  // Doc index, var types, declarations — push into editor
  if (data.docIndex) {
    editor.updateDocIndex(data.docIndex);
  }
  if (data.varTypes && Object.keys(data.varTypes).length > 0) editor.updateVarTypes(data.varTypes);
  if (data.declarations) {
    if (data.declarations.decls) {
      editor.updateDeclarations(data.declarations.decls, data.declarations.sources || {});
    }
    if (data.declarations.sources) {
      onSourceChangeCb?.(editor.getContent());
    }
  }

  // Entry points — notify callback, which may trigger a follow-up Run with eval
  const fns = (data.entryPoints ?? []) as EntryPoint[];
  if (!data.mesh && !data.debugFinal) {
    // No eval output — clear stale posMap so highlighting doesn't use old positions
    viewer.setPosMap([]);
    // Either check-only or eval error
    if (errors.length > 0) {
      const e = errors[0];
      hideStats();
      const prefix = e.file
        ? `[${e.file}${e.line > 0 ? ':' + e.line : ''}] `
        : '';
      errorDiv.textContent = prefix + e.message;
      errorDiv.style.display = 'block';
      if (e.line > 0 && !e.file) {
        errorDiv.style.cursor = 'pointer';
        editor.highlightError(e.line);
        errorDiv.onclick = () => editor.revealLine(e.line, e.col || 1);
      } else if (e.line > 0 && e.file) {
        errorDiv.style.cursor = 'pointer';
        errorDiv.onclick = async () => {
          getTab(e.file);
          await switchToTab(e.file);
          editor.highlightError(e.line);
          editor.revealLine(e.line, e.col || 1);
        };
      }
      if (pendingRunCallback) {
        const cb = pendingRunCallback;
        pendingRunCallback = null;
        cb({ success: false, error: prefix + e.message });
      }
      return;
    }
    // Check-only success — let callback pick entry point and dispatch eval
    const picked = onEntryPointsCb?.(fns);
    if (picked) {
      const key = picked.libPath || activeTab;
      if (debugMode) {
        Debug(key, picked.name, entryOverrides);
      } else {
        Run(key, picked.name, entryOverrides);
      }
    }
    return;
  }

  // Update entry points UI without re-dispatching
  onEntryPointsCb?.(fns);

  // --- Debug result ---
  if (data.debugSteps) {
    setDebugBarVisible(false);
    const steps = (data.debugSteps ?? []) as unknown as DebugStepData[];

    const finalMeshes = (data.debugFinal ?? []) as any as MeshData[];
    if (finalMeshes.length > 0) {
      steps.push({
        op: 'Final',
        meshes: finalMeshes.map((m: MeshData) => ({ role: 'result', mesh: m })),
        line: 0, col: 0, file: '',
      });
    }
    // Store augmented steps back so lastResult includes the synthetic Final step
    data.debugSteps = steps;

    renderTabs();
    debugFinalMesh = finalMeshes.length > 0 ? mergeMeshes(finalMeshes) : null;
    viewer.clearMeshes();
    if (debugFinalMesh) viewer.loadDecodedMesh(debugFinalMesh);

    if (steps.length > 0) {
      debugSlider.max = String(steps.length - 1);
      showDebugStep(0);
      setDebugBarVisible(true);
    }
    return;
  }

  // --- Normal eval result ---
  setDebugBarVisible(false);
  viewer.clearMeshes();
  if (data.mesh) viewer.loadMesh(data.mesh as MeshData);
  viewer.setPosMap(data.posMap ?? []);
  viewer.centerOnBed();
  viewer.fitToView();
  showStats(data.stats, data.time);

  if (pendingRunCallback) {
    const cb = pendingRunCallback;
    pendingRunCallback = null;
    requestAnimationFrame(() => requestAnimationFrame(() => {
      cb({ success: true, stats: data.stats, time: data.time });
    }));
  }
}

/** Shorten a path to just the filename, for tab display. */
function tabLabel(path: string): string {
  if (!path) return 'Untitled';
  const parts = path.split('/');
  const name = parts[parts.length - 1] || path;
  return name.endsWith('.fct') ? name.slice(0, -4) : name;
}

export function renderTabs() {
  onDebugFilesChangeCb?.();
  tabBar.innerHTML = '';
  // Only show tabs the user has explicitly opened
  const openTabs = Object.keys(tabs);
  if (openTabs.length === 0) return;

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

    if (getTab(path).readOnly) {
      const lock = document.createElement('span');
      lock.className = 'tab-lock';
      lock.innerHTML = '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0110 0v4"/></svg>';
      lock.title = 'Read-only';
      tab.appendChild(lock);
    } else if (getTab(path).dirty) {
      const dot = document.createElement('span');
      dot.className = 'tab-dirty';
      dot.textContent = '\u25cf';
      dot.title = 'Unsaved changes';
      tab.appendChild(dot);
    } else {
      const book = document.createElement('span');
      book.className = 'tab-book';
      book.innerHTML = '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M2 3h6a4 4 0 014 4v14a3 3 0 00-3-3H2z"/><path d="M22 3h-6a4 4 0 00-4 4v14a3 3 0 013-3h7z"/></svg>';
      book.title = 'Library';
      tab.appendChild(book);
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

function closeTab(file: string) {
  if (activeTab === file) {
    // Switch to another open tab
    const remaining = Object.keys(tabs).filter(k => k !== file);
    if (remaining.length > 0) {
      switchToTab(remaining[0]);
    }
  }
  editor.disposeModel(file);
  delete tabs[file];
  renderTabs();
}

export async function switchToTab(file: string) {
  if (file === activeTab) return;

  // Save cursor position for the tab we're leaving
  getTab(activeTab).cursor = editor.getCursorPosition();

  activeTab = file;
  // Sync module-level dirty flag with the new active tab
  dirty = getTab(file).dirty;
  editor.setCurrentSource(file);

  // Switch editor model — source comes from declarations or editor's own model cache
  editor.setReadOnly(false);
  const source = lastResult?.declarations?.sources?.[file] ?? '';
  editor.switchModel(file, source);

  // Resolve read-only status
  const tab = getTab(file);
  tab.readOnly = await IsReadOnlyPath(file);
  editor.setReadOnly(tab.readOnly);

  // Restore cursor position
  const saved = getTab(file).cursor;
  if (saved) {
    editor.revealLine(saved.lineNumber, saved.column);
  }

  renderTabs();

  // Update declarations from cached data so Go to Declaration works
  if (lastResult?.declarations?.decls) {
    editor.updateDeclarations(lastResult.declarations.decls, lastResult.declarations.sources || {});
  }

  // Re-highlight current debug step line if it belongs to this tab
  const _steps = lastResult?.debugSteps ?? [];
  if (_steps.length > 0 && debugStepIndex < _steps.length) {
    const step = _steps[debugStepIndex];
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

  const gen = ++debugStepGen;

  // Fetch meshes lazily — synthetic Final step already has its mesh
  let stepWithMeshes: DebugStepData;
  const step = debugSteps[index];
  const hasMeshes = step.meshes && step.meshes.length > 0;
  if (hasMeshes) {
    stepWithMeshes = step;
  } else {
    const meshes = await GetDebugStepMeshes(index);
    if (gen !== debugStepGen) return; // stale — user moved on
    stepWithMeshes = { ...step, meshes: meshes as unknown as DebugStepData['meshes'] };
  }
  // Final step: use the same display path as a normal run
  if (step.op === 'Final' && debugFinalMesh) {
    viewer.clearMeshes();
    viewer.loadDecodedMesh(debugFinalMesh);
    viewer.centerOnBed();
    viewer.fitToView();
  } else {
    viewer.loadDebugStep(stepWithMeshes);
  }

  const stepFile = debugSteps[index].file ?? '';
  if (stepFile !== activeTab) {
    switchToTab(stepFile);
  }

  if (debugSteps[index].line > 0) {
    editor.highlightDebugLine(debugSteps[index].line);
  } else {
    editor.clearDebugLine();
  }
}

/** Trigger evaluation with the given entry point. */
export function reeval(entry: string, libPath?: string) {
  const key = libPath || activeTab;
  if (debugMode) {
    Debug(key, entry, entryOverrides);
  } else {
    Run(key, entry, entryOverrides);
  }
}

export function stop() {
  Stop();
}

export function toggleRun() {
  if (runState === 'running') {
    stop();
  } else {
    run();
  }
}

// Auto-run guard: sends source updates to the runner.
// Called by editor onChange.
export function autoRun() {
  if (debugMode) return;
  markDirty();
  onSourceChangeCb?.(editor.getContent());
  run();
}

// Send current source to the runner for parsing, checking, and auto-eval.
export function run() {
  editor.clearDebugLine();
  const source = editor.getContent();
  UpdateSource(activeTab, source);
}

/** Refresh editor UI without dispatching an eval — check-only run. */
export function refreshEditorUI() {
  UpdateSource(activeTab, editor.getContent());
}

/** Cancel running builds, clear all state, and reset the backend. Called when loading new content. */
function resetSession() {
  // Clear backend state: program, entry point, lib cache
  ResetRunner();
  if (debugMode) {
    debugMode = false;
    debugBtn.classList.remove('active');
    setDebugBarVisible(false);
    editor.clearDebugLine();
  }
  // Dispose all editor models for library tabs
  const sources = lastResult?.declarations?.sources ?? {};
  for (const key of Object.keys(sources)) editor.disposeModel(key);
  lastResult = null;
  tabs = {};
  activeTab = '';
  debugFinalMesh = null;
  // Reset overrides and function list
  entryOverrides = {};
  onEntryPointsCb?.([]);
  renderTabs();
}

export async function loadSource(source: string, name?: string) {
  if (!await confirmDiscardAsync()) return;
  resetSession();
  const label = name ? name.replace(/\.fct$/, '') : 'Untitled';
  const key = 'untitled';
  tabs[key] = { path: '', readOnly: false, dirty: false, cursor: null, label };
  activeTab = key;
  editor.resetMainModel(key, source);
  editor.setReadOnly(false);
  markClean();
  updateWindowTitle();
  run();
}

export async function openFile() {
  if (!await confirmDiscardAsync()) return;
  const result = await OpenFile();
  if (!result) return; // cancelled
  resetSession();
  await setActiveFile(result.path);
  editor.resetMainModel(activeTab, result.source);
  markClean();
  updateWindowTitle();
  AddRecentFile(result.path).catch(() => {});
  run();
}

export async function openRecentFile(path: string) {
  if (!await confirmDiscardAsync()) return;
  let result: Record<string, string>;
  try {
    result = await OpenRecentFile(path);
  } catch {
    return; // file may no longer exist
  }
  resetSession();
  await setActiveFile(result.path);
  editor.resetMainModel(activeTab, result.source);
  markClean();
  updateWindowTitle();
  AddRecentFile(result.path).catch(() => {});
  run();
}

async function formatSource(source: string): Promise<string> {
  if (!formatOnSave || tabs[activeTab]?.readOnly) return source;
  try {
    return await FormatCode(source);
  } catch {
    return source;
  }
}

export async function saveFile() {
  const source = await formatSource(editor.getContent());
  editor.setContentSilent(source);
  const tab = getTab(activeTab);
  // Scratch files → show Save As dialog (empty path triggers dialog)
  const savePath = await IsScratchFile(tab.path) ? '' : tab.path;
  const path = await SaveFile(source, savePath);
  if (path) {
    // If saving under a new path (Save As dialog), update the tab
    if (path !== tab.path) {
      delete tabs[activeTab];
      tabs[path] = { path, readOnly: false, dirty: false, cursor: tab.cursor, label: tabLabel(path) };
      activeTab = path;
      editor.setCurrentSource(path);
    }
    tab.dirty = false;
    if (lastResult?.declarations?.sources) {
      lastResult.declarations.sources[activeTab] = source;
    }
    markClean();
    updateWindowTitle();
    renderTabs();
    AddRecentFile(path).catch(() => {});
  }
}

export async function newFile() {
  if (!await confirmDiscardAsync()) return;
  resetSession();
  const key = await CreateScratchFile('Untitled-' + Date.now());
  tabs[key] = { path: key, readOnly: false, dirty: false, cursor: null, label: 'Untitled' };
  activeTab = key;
  editor.resetMainModel(key, '');
  editor.setReadOnly(false);
  viewer.clearMeshes();
  markClean();
  updateWindowTitle();
  patchSettings({ lastFile: key });
}

export async function saveFileAs() {
  const source = await formatSource(editor.getContent());
  editor.setContentSilent(source);
  const path = await SaveFile(source, ''); // force dialog
  if (path) {
    const oldTab = tabs[activeTab];
    delete tabs[activeTab];
    tabs[path] = { path, readOnly: false, dirty: false, cursor: oldTab?.cursor ?? null, label: tabLabel(path) };
    activeTab = path;
    editor.setCurrentSource(path);
    markClean();
    updateWindowTitle();
    renderTabs();
    AddRecentFile(path).catch(() => {});
  }
}

export async function exportMesh(format: string = '3mf') {
  try {
    await ExportMesh(format);
  } catch (err: any) {
    const msg = err?.message || String(err);
    errorDiv.textContent = msg;
    errorDiv.style.display = 'block';
  }
}

export async function sendToSlicer(id: string) {
  try {
    await SendToSlicer(id);
  } catch (err: any) {
    const msg = err?.message || String(err);
    errorDiv.textContent = msg;
    errorDiv.style.display = 'block';
  }
}

export function toggleDebug() {
  debugMode = !debugMode;
  if (!debugMode) {
    setDebugBarVisible(false);
    editor.clearDebugLine();
    // Close library tabs opened during debug, keep the main file tab
    const sources = lastResult?.declarations?.sources ?? {};
    for (const key of Object.keys(sources)) {
      editor.disposeModel(key);
      delete tabs[key];
    }
    renderTabs();

    // Restore the final mesh with normal materials instead of re-evaluating
    if (debugFinalMesh) {
      viewer.clearMeshes();
      viewer.loadDecodedMesh(debugFinalMesh);
      viewer.centerOnBed();
      viewer.fitToView();
    }
    lastResult = null;
    debugFinalMesh = null;
  }
  return debugMode;
}

async function openDocs(): Promise<boolean> {
  if (docsPanel.isVisible()) return true;
  docsMode = true;
  const source = editor.getContent();
  try {
    const guides = await GetDocGuides();
    docsPanel.show(lastResult?.docIndex ?? [], guides);
    viewer.setVisible(false);
    setDebugBarVisible(false);
    return true;
  } catch {
    docsPanel.show([]);
    viewer.setVisible(false);
    setDebugBarVisible(false);
    return true;
  }
}

export async function toggleDocs() {
  docsMode = !docsMode;
  if (docsMode) {
    await openDocs();
  } else {
    docsPanel.hide();
    viewer.setVisible(true);
    if (debugMode && (lastResult?.debugSteps ?? []).length > 0) {
      setDebugBarVisible(true);
    }
  }
  return docsMode;
}

export async function openDocsToEntry(name: string): Promise<void> {
  await openDocs();
  docsPanel.focusEntry(name);
}

// ── State accessors for external UI (file tree, preview selector) ──────────

export function getLibrarySources(): Record<string, string> { return lastResult?.declarations?.sources ?? {}; }
export function getActiveTabValue(): string { return activeTab; }
export function getMainLabel(): string {
  const tab = tabs[activeTab];
  return tab ? tab.label || tabLabel(activeTab) : 'Untitled';
}

export function isPreviewLocked(): boolean { return previewLocked; }
export function setPreviewLocked(locked: boolean) {
  previewLocked = locked;
  if (locked) lockedTab = activeTab;
}

export function setOnTabChange(cb: (tab: string) => void) { onTabChangeCb = cb; }
export function setOnSourceChange(cb: (source: string) => void) { onSourceChangeCb = cb; }
export function setOnDebugFilesChange(cb: () => void) { onDebugFilesChangeCb = cb; }
export function setOnEntryPoints(cb: (fns: EntryPoint[]) => { name: string; libPath: string } | null) { onEntryPointsCb = cb; }
export function getEntryPoints(): EntryPoint[] { return lastResult?.entryPoints ?? []; }

export function setEntryOverrides(overrides: Record<string, any>) { entryOverrides = overrides; }

/** Return the source currently being edited in the main tab. */
export function getMainSource(): string {
  return editor.getModelContent(activeTab);
}

/** Open a library tab without navigating to a specific line.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export async function openLibraryTab(file: string, source: string) {
  const sources = lastResult?.declarations?.sources ?? {};
  if (!sources[file]) {
    const resolved = await GetLibraryFilePath(file);
    if (resolved) file = resolved;
  }
  if (sources[file] && !source) {
    source = sources[file];
  }
  if (source && !sources[file]) {
    sources[file] = source;
  }
  getTab(file);
  await switchToTab(file);
}

/** Open a library file in a read-only tab and navigate to the given position.
 *  file may be an import path or disk path. If import path, resolves to disk path. */
export async function openLibraryFile(file: string, source: string, line: number, col: number) {
  // Resolve import path to disk path if needed
  const sources = lastResult?.declarations?.sources ?? {};
  if (!sources[file]) {
    const resolved = await GetLibraryFilePath(file);
    if (resolved) file = resolved;
  }
  if (sources[file] && !source) {
    source = sources[file];
  }
  if (source && !sources[file]) {
    sources[file] = source;
  }
  getTab(file);
  await switchToTab(file);
  editor.revealLine(line, col);
}
