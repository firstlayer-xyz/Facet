import './style.css';
import { createEditor, EditorHandle } from './editor';
import { Viewer } from './viewer';
import { initAutomation } from './automation';
import { Toggle, bindToggleButton } from './toggle';
import { Gnomon } from './gnomon';
import { GetDefaultSource, GetExample, DetectSlicers, SetAssistantConfig, CreateLocalLibrary, CreateLibraryFolder, ListLibraryFolders, OpenRecentFile, OpenLibraryDir, AutomationEnabled, CreateScratchFile } from '../wailsjs/go/main/App';
import { ClipboardSetText, ClipboardGetText, WindowToggleMaximise } from '../wailsjs/runtime/runtime';
import { on } from './events';
import { tabStore } from './tabs';
import { loadSettings, saveSettings, createSettingsPanel, SettingsCorruptError } from './settings';

// macOS fullscreen detection — traffic lights disappear in fullscreen so reduce titlebar inset
function syncFullscreen() {
  document.body.classList.toggle('macos-fullscreen', window.innerHeight >= screen.height - 5);
}
syncFullscreen();
window.addEventListener('resize', syncFullscreen);

// WKWebView doesn't support browser clipboard API — override with Wails native clipboard.
Object.defineProperty(navigator, 'clipboard', {
  value: {
    writeText: (text: string): Promise<void> => ClipboardSetText(text).then(() => {}),
    readText: (): Promise<string> => ClipboardGetText(),
  },
  configurable: true,
});
import { DocsPanel } from './docs';
import { AssistantPanel, applyEdit } from './assistant';

import {
  app, editorPanel, divider, viewportPanel, canvasContainer,
  centerBtn, autoRotateBtn, headTrackBtn, newBtn, openBtn, saveBtn, shareBtn, settingsBtn, docsBtn, runBtn,
  debugBtn, assistantBtn, exportBtn, slicerBtn, fullCodeBtn, codeBtn, errorDiv, tabBar,
  debugBar, debugPrevBtn, debugNextBtn, debugSlider, debugLabel, statsBar, compilingOverlay,
  debugRestartBtn, debugContinueBtn, debugStopBtn,
  vpPane, vpPaneSummary, hiddenLinesBtn, panelResizer, docsResizer, previewSelector,
  previewFileBtn, previewFileMenu,
  measureBtn, extentsBtn, clearDimsBtn,
  drawerStack,
} from './toolbar';
import { FunctionPreview } from './function-preview';
import type { EntryPoint } from './function-preview';
import type { DrawingViewpoint } from './viewer';

import {
  initApp, setEditor, setInitialFile, setFormatOnSave, setHighlightMode,
  run, autoRun, toggleRun,
  showDebugStep, showDebugStepPrev, showDebugStepNext, continueDebug,
  openExample, openFile, openRecentFile, saveFile, saveFileAs, newFile, exportMesh, sendToSlicer, shareToWeb,
  reeval, toggleDebug, toggleDocs, openDocsToEntry, openLibraryFile, openLibraryTab,
  switchToTab, closeActiveTab,
  getSources, getActiveTabValue, isActiveTabReadOnly, assistantCreateFile, labelForPath, addRestoredTab, renderTabs,
  isDebugStepping,
  setOnSourceChange, setOnDebugFilesChange, setOnDebugExit, setOnEntryPoints,
  setEntryOverrides, refreshEditorUI, showError, currentEvalErrorsText,
} from './app';
import { resolveThemePalette, resolveUiTheme, resolveEditorTheme, applyUIPalette } from './themes';

let settings: Awaited<ReturnType<typeof loadSettings>>;
let viewer: Viewer;

import { promptNewLibrary, showSlicerPicker } from './dialogs';
import { initFullCode, toggleFullCode, isFullCode } from './fullcode';

function buildViewerAppearance(
  palette: ReturnType<typeof resolveThemePalette>,
  appearance: typeof settings.appearance,
  measurement: typeof settings.measurement,
) {
  return {
    backgroundColor: palette.viewBg,
    meshColor: palette.viewMesh ?? palette.accent,
    meshMetalness: palette.viewMeshMetalness,
    meshRoughness: palette.viewMeshRoughness,
    edgeColor: palette.viewEdgeColor,
    edgeOpacity: palette.viewEdgeOpacity,
    edgeThreshold: palette.viewEdgeThreshold,
    ambientIntensity: palette.viewAmbientIntensity,
    gridMajorColor: palette.viewGridMajor,
    gridMinorColor: palette.viewGridMinor,
    measurementLineColor: palette.text,
    measurementLabelColor: palette.textBright,
    measurementFormat: {
      units: measurement.units,
      imperialFormat: measurement.imperialFormat,
      imperialDenominator: measurement.imperialDenominator,
    },
    bed: appearance.bed,
    gridSize: appearance.gridSize,
    gridSpacing: appearance.gridSpacing,
  };
}

/** Resolve UI palette and apply to CSS vars, viewport, and editor theme. */
function applyCurrentTheme(): void {
  // UI palette (from appearance settings)
  const uiId = resolveUiTheme(settings.appearance.uiTheme, settings.appearance.darkMode);
  const palette = resolveThemePalette(uiId, settings.appearance.themeOverrides, settings.appearance.customThemes);
  applyUIPalette(palette);
  viewer.applySettings(buildViewerAppearance(palette, settings.appearance, settings.measurement));

  // Editor theme (follows UI theme)
  editorRef?.setTheme(resolveEditorTheme(
    settings.appearance.uiTheme,
    settings.appearance.darkMode,
    settings.appearance.customThemes,
  ));
}

async function handleDocsToggle() {
  const active = await toggleDocs();
  docsBtn.classList.toggle('active', active);
  docsResizer.classList.toggle('open', active);
}

// Status bar eval state — set inside initApp once the status elements exist
let applyEvalStatus: ((state: 'idle' | 'ready' | 'error', ms?: number) => void) | undefined;

// Docs panel renders into its drawer-stack slot. The slot is a stable
// DOM home that never gets reparented across mode changes — fullcode
// (the View toggle) doesn't touch it.
const docsPanel = new DocsPanel(drawerStack, handleDocsToggle);

// Assistant panel
let editorRef: EditorHandle | null = null;
const assistantPanel = new AssistantPanel(
  drawerStack,
  () => editorRef?.getContent() ?? '',
  () => currentEvalErrorsText(),
  () => ({ path: getActiveTabValue(), readOnly: isActiveTabReadOnly() }),
  (newCode: string, searchFor?: string) => {
    if (editorRef) {
      if (searchFor !== undefined) {
        const current = editorRef.getContent();
        const result = applyEdit(current, searchFor, newCode);
        if (result !== null) {
          editorRef.setContent(result);
          run();
        }
        // If search text not found, do nothing — don't replace the entire
        // editor with just a replacement fragment. The user can manually
        // click Apply or copy the code.
      } else {
        editorRef.setContent(newCode);
        run();
      }
    }
  },
  (newCode: string) => { editorRef?.setContentSilent(newCode); refreshEditorUI(); },
  (name: string, code: string) => {
    assistantCreateFile(name, code).catch(err => showError(err));
  },
  // Read the persisted assistant config for the panel's model/effort selector.
  () => settings.assistant,
  // Persist a model/effort change and push it to the backend for the next send.
  (cfg) => {
    settings.assistant = cfg;
    saveSettings(settings);
    SetAssistantConfig(settings.assistant);
  },
  // Viewport capture for the screenshot_viewport MCP tool. When the model
  // requests an explicit camera pose, render off-screen so the user's view
  // is untouched; otherwise grab the live canvas (preserveDrawingBuffer is
  // on, so toDataURL is safe without an extra render pass).
  (opts) => {
    if (opts && opts.azimuth !== undefined && opts.elevation !== undefined && opts.distance !== undefined) {
      return viewer.captureScreenshotFromView({
        azimuth: opts.azimuth,
        elevation: opts.elevation,
        distance: opts.distance,
        target: opts.target,
      });
    }
    return viewer.captureScreenshot();
  },
  // onClose: sync toolbar button state when closed via the × button.
  syncAssistantState,
);

// Async init — loadSettings reads from Go backend
async function init() {
  // Load settings first (may migrate from localStorage)
  settings = await loadSettings();

  // Trap localStorage writes — all settings go through Go backend now
  Storage.prototype.setItem = function(key: string, _value: string) {
    console.error(`[settings] localStorage.setItem("${key}") blocked — use Go backend`);
  };
  Storage.prototype.removeItem = function(key: string) {
    console.error(`[settings] localStorage.removeItem("${key}") blocked`);
  };

  // Push assistant config to backend
  SetAssistantConfig(settings.assistant);

  // Initialize 3D viewer with theme-derived viewport colors
  const _initUiId = resolveUiTheme(settings.appearance.uiTheme, settings.appearance.darkMode);
  const _initPalette = resolveThemePalette(_initUiId, settings.appearance.themeOverrides, settings.appearance.customThemes);
  applyUIPalette(_initPalette);
  viewer = new Viewer(canvasContainer, buildViewerAppearance(_initPalette, settings.appearance, settings.measurement));

  // Test hook: expose the viewer to page-context scripts. Same
  // rationale as window.monaco in editor.ts — Playwright tests can
  // assert viewer state (mesh count, etc) without poking private
  // fields. Single property reference, code is in the bundle anyway.
  (window as unknown as { viewer: Viewer }).viewer = viewer;

  // Auto-center & rotate turntable, owned by one Toggle: the toolbar button, the
  // menu (future), and automation all drive the same instance, so they can't
  // desync. bindToggleButton wires the button both ways (click → toggle, state
  // → active class).
  const autoRotate = new Toggle(false, (on) => viewer.setAutoRotate(on));
  bindToggleButton(autoRotateBtn, autoRotate);

  // Remote GUI automation: listen for automation:invoke commands (only emitted
  // when the app runs with --automation). Registering the listener always is
  // harmless — no events arrive without the flag.
  const requireEditor = () => {
    if (!editorRef) throw new Error('editor not ready');
    return editorRef;
  };
  initAutomation({
    viewer,
    setAutoRotate: (on) => autoRotate.set(on),
    editor: {
      insertAtCursor: (t) => requireEditor().insertAtCursor(t),
      moveCursorAfter: (f) => requireEditor().moveCursorAfter(f),
      selectRange: (f) => requireEditor().selectRange(f),
      deleteSelection: () => requireEditor().deleteSelection(),
      setContentSilent: (t) => requireEditor().setContentSilent(t),
      getContent: () => editorRef?.getContent() ?? '',
      build: () => run(),
    },
  });

  // Gnomon — always-visible axis indicator bottom-left of viewport, drag to orbit
  const gnomon = new Gnomon(canvasContainer, (dTheta, dPhi) => viewer.orbitBy(dTheta, dPhi));
  viewer.onFrame((camera) => gnomon.update(camera));

  // Keep the Measure toolbar button in sync with viewer state (Esc / mesh reload
  // can drop out of placing mode without a button click).
  viewer.setOnMeasureModeChange((mode) => {
    measureBtn.classList.toggle('active', mode === 'placing');
    canvasContainer.style.cursor = mode === 'placing' ? 'crosshair' : '';
  });

  // Initialize app module with all dependencies
  // --automation: boot a clean blank slate and don't persist the throwaway
  // session over the user's real tabs. The flag is Go-owned; the frontend just
  // reads it here.
  const automationMode = await AutomationEnabled();

  initApp({
    viewer,
    docsPanel,
    errorDiv,
    debugBar,
    debugSlider,
    debugLabel,
    tabBar,
    statsBar,
    compilingOverlay,
    automationMode,
    onEvalStatus: (state, ms) => applyEvalStatus?.(state, ms),
    onDebugBarChange: (visible) => {
      previewSelector.style.display = visible ? 'none' : '';
      vpPane.classList.toggle('debug-active', visible);
    },
  });

  initFullCode({
    viewer, editorPanel, viewportPanel, canvasContainer, divider,
    app, fullCodeBtn,
  });

  // Under --automation (fetched above) we assume a demo/recording session: boot
  // a clean blank slate instead of restoring the user's last session (app.ts
  // also skips persisting on close, so this throwaway session never clobbers
  // their tabs).
  const savedTabs = automationMode ? [] : settings.savedTabs;
  const savedActiveTab = automationMode ? null : settings.activeTab;

  // Load a saved tab's source — handles both example: and filesystem paths.
  async function loadSavedTab(path: string): Promise<{ source: string; path: string; readOnly: boolean } | null> {
    if (path.startsWith('example:')) {
      const name = path.slice('example:'.length);
      const source = await GetExample(name);
      return { source, path, readOnly: true };
    }
    const result = await OpenRecentFile(path);
    if (result?.source) {
      return { source: result.source, path: result.path, readOnly: false };
    }
    return null;
  }

  // Load all saved tabs, skipping any that fail (file deleted, example removed, etc.)
  type LoadedTab = { source: string; path: string; readOnly: boolean; cursor: { lineNumber: number; column: number } | null };
  const loadedTabs: LoadedTab[] = [];
  for (const saved of savedTabs ?? []) {
    try {
      const loaded = await loadSavedTab(saved.path);
      if (loaded) {
        loadedTabs.push({ ...loaded, cursor: saved.cursor });
      }
    } catch {
      // file no longer exists — skip
    }
  }

  // With no tabs to restore: a blank editable scratch tab under automation (a
  // clean canvas the demo script fills via editor.loadCode), otherwise the
  // default tutorial.
  if (loadedTabs.length === 0) {
    if (automationMode) {
      const key = await CreateScratchFile('Untitled-' + Date.now());
      loadedTabs.push({ source: '', path: key, readOnly: false, cursor: null });
    } else {
      const source = await GetDefaultSource();
      loadedTabs.push({ source, path: 'example:Tutorial.fct', readOnly: true, cursor: null });
    }
  }

  // Wrap Monaco in an explicit flex:1 container so the status bar always
  // renders below it regardless of what class Monaco applies to its root div.
  const monacoContainer = document.createElement('div');
  monacoContainer.className = 'monaco-container';
  editorPanel.appendChild(monacoContainer);

  // First loaded tab initializes the editor
  const first = loadedTabs[0];
  const editor = createEditor(monacoContainer, first.source, autoRun, async (name, library) => {
    await openDocsToEntry(name, library);
    docsBtn.classList.add('active');
    docsResizer.classList.add('open');
  }, (file, source, line, col) => {
    openLibraryFile(file, source, line, col);
  }, first.path);
  editor.setWordWrap(settings.editor.wordWrap);
  editor.setTheme(resolveEditorTheme(
    settings.appearance.uiTheme,
    settings.appearance.darkMode,
    settings.appearance.customThemes,
  ));
  setFormatOnSave(settings.editor.formatOnSave);
  setHighlightMode(settings.editor.highlight);
  editorRef = editor;
  setEditor(editor);

  // Editor status bar
  const editorStatus = document.createElement('div');
  editorStatus.className = 'editor-status';

  const statusDot = document.createElement('span');
  statusDot.className = 'editor-status-dot';

  const statusStateSpan = document.createElement('span');
  statusStateSpan.className = 'editor-status-state';

  const statusMsSpan = document.createElement('span');
  statusMsSpan.className = 'editor-status-ms';

  const statusPos = document.createElement('span');
  statusPos.className = 'editor-status-pos';
  statusPos.textContent = 'Ln 1, Col 1';

  editorStatus.appendChild(statusDot);
  editorStatus.appendChild(statusStateSpan);
  editorStatus.appendChild(statusMsSpan);
  editorStatus.appendChild(statusPos);

  let currentLine = 1, currentCol = 1;
  let evalState: 'idle' | 'ready' | 'error' = 'idle';
  let evalMs: number | null = null;

  function refreshStatus() {
    statusDot.className = `editor-status-dot ${evalState}`;
    statusStateSpan.textContent = evalState !== 'idle' ? evalState : '';
    statusMsSpan.textContent = evalMs !== null && evalState !== 'idle' ? `${evalMs}ms` : '';
    statusPos.textContent = `Ln ${currentLine}, Col ${currentCol}`;
  }

  editor.onCursorChange((line, col) => {
    currentLine = line; currentCol = col;
    refreshStatus();
  });

  applyEvalStatus = (state, ms) => {
    evalState = state;
    evalMs = ms ?? null;
    refreshStatus();
  };

  // Wire face-click → source navigation
  viewer.setOnFaceClick(async (file, line, col) => {
    if (line <= 0 || !file) {
      editor.clearError();
      editor.clearDebugLine();
      return;
    }
    // Ensure the correct tab is open and active
    if (file !== getActiveTabValue()) {
      const sources = getSources();
      const text = sources[file]?.text ?? '';
      await openLibraryTab(file, text);
    }
    requestAnimationFrame(() => {
      editor.revealLine(line, col || 1);
      editor.highlightError(line);
    });
  });

  // Register all loaded tabs
  await setInitialFile(first.path, first.readOnly);
  if (first.cursor) {
    editor.revealLine(first.cursor.lineNumber, first.cursor.column);
  }
  for (let i = 1; i < loadedTabs.length; i++) {
    const tab = loadedTabs[i];
    editor.preloadModel(tab.path, tab.source);
    await addRestoredTab(tab.path, tab.cursor);
  }
  if (loadedTabs.length > 1) renderTabs();

  // Switch to the previously active tab
  if (savedActiveTab && savedActiveTab !== getActiveTabValue()) {
    switchToTab(savedActiveTab);
    const saved = loadedTabs.find(t => t.path === savedActiveTab);
    if (saved?.cursor) {
      editor.revealLine(saved.cursor.lineNumber, saved.cursor.column);
    }
  }

  // ── Function preview panel ─────────────────────────────────────────────────
  let currentEntryPoints: EntryPoint[] = [];
  let selectedFnKey: string | null = null; // "libPath::name" or null

  function fnKey(e: EntryPoint) {
    return `${e.libPath}::${e.name}`;
  }

  /** Return the currently selected entry point, or null. */
  function selectedEntry(): EntryPoint | null {
    if (selectedFnKey === null) return null;
    return currentEntryPoints.find(f => fnKey(f) === selectedFnKey) ?? null;
  }

  let previewMenuDirty = true;
  const previewFileLbl = document.getElementById('preview-file-lbl')!;

  /** Show file name in the preview label, but keep showing fn name once one is picked. */
  function updatePreviewLabel(tab: string) {
    if (selectedFnKey !== null) return;
    previewFileLbl.textContent = labelForPath(tab);
  }

  const functionPreview = new FunctionPreview(canvasContainer, {
    onOverrideChange(overrides) {
      setEntryOverrides(overrides);
      const fn = selectedEntry();
      if (fn) reeval(fn.name, fn.libPath);
    },
  });

  /** Filter visible entry points for a tab, keep previous selection if valid, else pick first. */
  function pickEntryPoint(tab: string, fns: EntryPoint[]): EntryPoint | null {
    const visible = fns.filter(f => f.libPath === tab);
    if (visible.length === 0) {
      selectedFnKey = null;
      setEntryOverrides({});
      functionPreview.updateUI(null);
      updatePreviewLabel(tab);
      return null;
    }
    let picked = visible[0];
    if (selectedFnKey !== null) {
      const still = visible.find(f => fnKey(f) === selectedFnKey);
      if (still) picked = still;
    }
    selectedFnKey = fnKey(picked);
    previewFileLbl.textContent = picked.name;
    return picked;
  }

  function buildPreviewMenu() {
    previewFileMenu.innerHTML = '';
    const activeTab = getActiveTabValue();
    const visibleFns = currentEntryPoints.filter(f => f.libPath === activeTab);
    for (const fn of visibleFns) {
      const item = document.createElement('div');
      const key = fnKey(fn);
      const isActive = selectedFnKey === key;
      item.className = 'pv-item pv-fn-item' + (isActive ? ' pv-active' : '');

      const chk = document.createElement('span');
      chk.className = 'pv-chk';
      chk.textContent = isActive ? '✓' : '';

      const nameSpan = document.createElement('span');
      nameSpan.className = 'pv-fn-name';
      nameSpan.textContent = fn.name;

      const sigSpan = document.createElement('span');
      sigSpan.className = 'pv-fn-sig';
      const fnParams = fn.params || [];
      sigSpan.textContent = fnParams.length > 0
        ? '(' + fnParams.map(p => `${p.name}: ${p.type}`).join(', ') + ')'
        : '()';

      item.appendChild(chk);
      item.appendChild(nameSpan);
      item.appendChild(sigSpan);

      item.addEventListener('click', () => {
        previewFileMenu.classList.remove('show');
        previewFileBtn.classList.remove('open');
        selectedFnKey = key;
        previewFileLbl.textContent = fn.name;
        functionPreview.resetOverrides();
        functionPreview.updateUI(fn);
        setEntryOverrides({});
        reeval(fn.name, fn.libPath);
      });
      previewFileMenu.appendChild(item);
    }
  }

  // Called by run() after GetEntryPoints resolves.
  // Picks the entry point function and returns its name (or null to skip running).
  setOnEntryPoints((fns) => {
    currentEntryPoints = fns;
    previewMenuDirty = true;
    const tab = getActiveTabValue();
    const picked = pickEntryPoint(tab, fns);
    if (previewFileMenu.classList.contains('show')) buildPreviewMenu();
    if (!picked) return null;
    const reconciledOverrides = functionPreview.updateUI(picked);
    setEntryOverrides(reconciledOverrides);
    return { name: picked.name, libPath: picked.libPath };
  });

  setOnSourceChange(() => updatePreviewLabel(getActiveTabValue()));

  tabStore.onActiveChange((tab) => {
    if (isDebugStepping()) return; // don't re-eval while navigating debug steps
    previewMenuDirty = true;
    const picked = pickEntryPoint(tab, currentEntryPoints);
    if (picked) {
      const reconciledOverrides = functionPreview.updateUI(picked);
      setEntryOverrides(reconciledOverrides);
      reeval(picked.name, picked.libPath);
    }
    if (previewFileMenu.classList.contains('show')) buildPreviewMenu();
  });

  setOnDebugFilesChange(() => {
    previewMenuDirty = true;
    if (previewFileMenu.classList.contains('show')) buildPreviewMenu();
  });

  setOnDebugExit(() => {
    debugBtn.classList.remove('active');
  });

  // Preview file dropdown toggle
  previewFileBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    if (previewMenuDirty) { buildPreviewMenu(); previewMenuDirty = false; }
    const open = previewFileMenu.classList.toggle('show');
    previewFileBtn.classList.toggle('open', open);
  });

  // Close dropdown on outside click
  document.addEventListener('click', () => {
    previewFileMenu.classList.remove('show');
    previewFileBtn.classList.remove('open');
  });

  updatePreviewLabel(getActiveTabValue());

  // Append status bar last so it sits at the bottom of the editor panel
  editorPanel.appendChild(editorStatus);

  run();

  // Auto-switch theme when OS light/dark preference changes
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (settings.appearance.darkMode !== 'auto') return;
    applyCurrentTheme();
  });
}
init().catch((err) => {
  if (err instanceof SettingsCorruptError) {
    // Settings file exists but could not be read or parsed. Show a blocking
    // notice so the user can repair or move the file — do NOT let the app
    // continue with defaults, because any subsequent save would overwrite the
    // real file (PatchSettings already refuses, but this reaches the user
    // before they see silent failures across the UI).
    document.body.innerHTML = '';
    const panel = document.createElement('div');
    panel.style.cssText = 'position:fixed;inset:0;display:flex;align-items:center;justify-content:center;background:#1e1e1e;color:#eee;font-family:system-ui,sans-serif;padding:24px;';
    panel.innerHTML = `
      <div style="max-width:640px;">
        <h2 style="margin-top:0;color:#ff6b6b;">Settings file could not be read</h2>
        <p>Facet could not load your settings. To avoid overwriting your real configuration, the app has refused to start.</p>
        <pre style="background:#2a2a2a;padding:12px;border-radius:4px;white-space:pre-wrap;">${err.message.replace(/[<>&]/g, (c) => ({'<':'&lt;','>':'&gt;','&':'&amp;'}[c]!))}</pre>
        <p>Move or delete the settings file, then restart Facet. A fresh file will be created on next save.</p>
      </div>`;
    document.body.appendChild(panel);
    return;
  }
  throw err;
});

// --- Event listeners ---

// Double-click titlebar to maximise/restore
document.getElementById('titlebar-drag')!.addEventListener('dblclick', () => WindowToggleMaximise());

newBtn.addEventListener('click', newFile);
openBtn.addEventListener('click', openFile);
saveBtn.addEventListener('click', saveFile);

settingsBtn.addEventListener('click', () => {
  if (document.getElementById('settings-overlay')) return;
  const panel = createSettingsPanel(settings, (updated) => {
    settings = updated;
    saveSettings(settings);
    applyCurrentTheme();
    editorRef?.setWordWrap(settings.editor.wordWrap);
    setFormatOnSave(settings.editor.formatOnSave);
    setHighlightMode(settings.editor.highlight);
    SetAssistantConfig(settings.assistant);
  });
  app.appendChild(panel);
});

// Center model button
centerBtn.addEventListener('click', () => viewer.fitToView());

// Auto-rotate button is wired in init() via a Toggle (bindToggleButton), so the
// button, menu, and automation all stay in sync — see the autoRotate Toggle.

// Head-tracking parallax button
headTrackBtn.addEventListener('click', async () => {
  try {
    const active = await viewer.toggleHeadTracking(
      settings.camera.deviceId || undefined,
      settings.camera.yOffset,
    );
    headTrackBtn.classList.toggle('active', active);
  } catch (err: any) {
    showError(err?.message || 'Camera access denied');
  }
});

// Native menu event listeners
on('menu:new', () => newFile());
on('menu:open', () => openFile());
on('menu:open-recent', (path: string) => openRecentFile(path));
on('menu:open-demo', async (name: string) => {
  try {
    const source = await GetExample(name);
    openExample(source, name);
  } catch (err) {
    console.error('Failed to load example:', err);
  }
});
on('menu:open-library', async (dir: string) => {
  try {
    const result = await OpenLibraryDir(dir);
    openExample(result.source, result.path);
  } catch (err) {
    console.error('Failed to open library:', err);
  }
});
on('menu:new-library', async () => {
  let folders: string[];
  try { folders = await ListLibraryFolders(); } catch { folders = []; }
  if (!folders) folders = [];

  const result = await promptNewLibrary(folders);
  if (!result) return;

  // Create new folder if needed
  if (result.isNewFolder) {
    try { await CreateLibraryFolder(result.folder); } catch (err: any) {
      alert('Could not create folder: ' + (err?.message ?? String(err)));
      return;
    }
  }

  try {
    const path = await CreateLocalLibrary(result.folder, result.name);
    const file = await OpenRecentFile(path);
    openLibraryTab(path, file.source);
  } catch (err: any) {
    alert('Could not create library: ' + (err?.message ?? String(err)));
  }
});
on('menu:close-tab', () => closeActiveTab());
on('menu:save', () => saveFile());
on('menu:save-as', () => saveFileAs());
on('menu:export', (format: string) => exportMesh(format, settings.export.embedSourceIn3mf));

// Run menu
on('menu:run', () => toggleRun());
on('menu:debug', handleDebugToggle);

// View menu
on('menu:fullcode', toggleFullCode);
on('menu:toggle-grid', () => viewer.toggleGrid());
on('menu:toggle-axes', () => viewer.toggleAxes());
on('menu:docs', handleDocsToggle);

// Model menu
on('menu:assistant', () => assistantBtn.click());
on('menu:slicer', () => slicerBtn.click());
on('menu:slicer-id', (id: string) => sendToSlicer(id));

// Window menu
on('menu:settings', () => settingsBtn.click());

// Manual run / stop button (in preview selector)
runBtn.addEventListener('click', toggleRun);

// Debug toggle
function handleDebugToggle() {
  const active = toggleDebug();
  debugBtn.classList.toggle('active', active);
  if (active) run();
}
debugBtn.addEventListener('click', handleDebugToggle);

// CODE toggle — hides/shows the code editor panel.
codeBtn.addEventListener('click', () => {
  const hiding = !editorPanel.classList.contains('code-hidden');
  editorPanel.classList.toggle('code-hidden', hiding);
  codeBtn.classList.toggle('active', !hiding);
  requestAnimationFrame(() => viewer.resize());
});

// Assistant toggle. AssistantPanel manages `.open` on its own panel;
// the resizer's `.open` is in lockstep.
function syncAssistantState() {
  const assistantVisible = assistantPanel.isVisible();
  assistantBtn.classList.toggle('active', assistantVisible);
  panelResizer.classList.toggle('open', assistantVisible);
}
assistantBtn.addEventListener('click', () => {
  assistantPanel.toggle();
  syncAssistantState();
});

// Docs toggle
docsBtn.addEventListener('click', handleDocsToggle);

// View pane — state tracking
let currentPreset: DrawingViewpoint = 'iso';
let currentShadeLabel = '3D';
let currentCamLabel = 'Persp';
let currentOrientLabel = 'ISO';

function updateVpSummary() {
  vpPaneSummary.textContent = `${currentShadeLabel} · ${currentCamLabel} · ${currentOrientLabel}`;
}

function setPreset(vp: DrawingViewpoint) {
  currentPreset = vp;
  const orientLabels: Record<DrawingViewpoint, string> = {
    iso: 'ISO', top: 'Top', bot: 'Bot', home: 'Home',
    front: 'Front', back: 'Back', right: 'Right', left: 'Left',
  };
  currentOrientLabel = orientLabels[vp];
  for (const btn of vpPane.querySelectorAll<HTMLButtonElement>('.vp-orient-btn')) {
    btn.classList.toggle('active', btn.dataset.viewpoint === vp);
  }
  updateVpSummary();
  viewer.setViewpoint(vp);
}

// View pane — delegated click handler for shade, cam, and orient
vpPane.addEventListener('click', (e) => {
  const target = e.target as HTMLElement;
  // Shade (3D / Wire)
  const renderBtn = target.closest<HTMLButtonElement>('.vp-seg-btn[data-render-id]');
  if (renderBtn) {
    for (const btn of vpPane.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-render-id]')) btn.classList.remove('active');
    renderBtn.classList.add('active');
    const isWire = renderBtn.dataset.renderId === 'wireframe';
    viewer.setWireframeMode(false);
    viewer.setDrawingMode(isWire);
    hiddenLinesBtn.style.display = isWire ? '' : 'none';
    if (!isWire) {
      hiddenLinesBtn.classList.remove('active');
      viewer.setHiddenLines(false);
    }
    if (isWire) viewer.setViewpoint(currentPreset);
    currentShadeLabel = isWire ? 'Wire' : '3D';
    updateVpSummary();
    return;
  }
  // Cam (Persp / Ortho)
  const projBtn = target.closest<HTMLButtonElement>('.vp-seg-btn[data-proj-id]');
  if (projBtn) {
    for (const btn of vpPane.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-proj-id]')) btn.classList.remove('active');
    projBtn.classList.add('active');
    const isOrtho = projBtn.dataset.projId === 'ortho';
    viewer.setOrthoProjection(isOrtho);
    currentCamLabel = isOrtho ? 'Ortho' : 'Persp';
    updateVpSummary();
    return;
  }
  // Orient grid
  const orientBtn = target.closest<HTMLButtonElement>('.vp-orient-btn[data-viewpoint]');
  if (orientBtn) {
    setPreset(orientBtn.dataset.viewpoint as DrawingViewpoint);
  }
});

hiddenLinesBtn.addEventListener('click', () => {
  const on = !hiddenLinesBtn.classList.contains('active');
  hiddenLinesBtn.classList.toggle('active', on);
  viewer.setHiddenLines(on);
});

// Measurement controls
measureBtn.addEventListener('click', () => {
  const on = !measureBtn.classList.contains('active');
  viewer.setMeasureMode(on ? 'placing' : 'off');
});
extentsBtn.addEventListener('click', () => {
  viewer.showExtents();
});
clearDimsBtn.addEventListener('click', () => {
  viewer.clearMeasurements();
});


exportBtn.addEventListener('click', () => exportMesh('3mf', settings.export.embedSourceIn3mf));
shareBtn.addEventListener('click', () => shareToWeb(shareBtn));

async function pickAndSendToSlicer() {
  const slicers = await DetectSlicers();
  if (!slicers || slicers.length === 0) {
    showError('No slicer found — install BambuStudio, OrcaSlicer, PrusaSlicer, Cura, or AnycubicSlicer');
    return;
  }
  if (slicers.length === 1) {
    sendToSlicer(slicers[0].id);
    return;
  }
  const id = await showSlicerPicker(slicers, slicerBtn);
  if (id) sendToSlicer(id);
}

slicerBtn.addEventListener('click', async () => {
  if (settings.slicer.defaultSlicer) {
    sendToSlicer(settings.slicer.defaultSlicer);
    return;
  }
  pickAndSendToSlicer();
});

slicerBtn.addEventListener('contextmenu', (e) => {
  e.preventDefault();
  pickAndSendToSlicer();
});

// Debug strip controls
debugPrevBtn.addEventListener('click', showDebugStepPrev);
debugNextBtn.addEventListener('click', showDebugStepNext);
debugSlider.addEventListener('input', () => showDebugStep(parseInt(debugSlider.value, 10)));
debugContinueBtn.addEventListener('click', continueDebug);
debugRestartBtn.addEventListener('click', () => run());
debugStopBtn.addEventListener('click', handleDebugToggle);

// Shared drag lifecycle for the divider and drawer resizers: a mousedown on the
// handle starts the drag (col-resize cursor, text selection suppressed), window
// mousemove forwards to onMove while dragging, and mouseup ends it.
function makeDragResizer(handle: HTMLElement, onMove: (e: MouseEvent) => void) {
  let dragging = false;
  handle.addEventListener('mousedown', (e) => {
    e.preventDefault();
    dragging = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  });
  window.addEventListener('mousemove', (e) => {
    if (dragging) onMove(e);
  });
  window.addEventListener('mouseup', () => {
    if (!dragging) return;
    dragging = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  });
}

// The main stage must always keep at least this many pixels so the user
// can't drag a drawer over the entire content area.
const MIN_VP = 150;
// Width of the flex-growing main stage that absorbs drawer resizing. In
// fullcode mode the viewport is hidden (canvas floats in the mini-preview)
// and the editor is the flex:1 stage; otherwise it's the viewport panel.
const stageWidth = () =>
  (isFullCode() ? editorPanel : viewportPanel).getBoundingClientRect().width;

// Drawer resize (canvas ↔ right panel): grow/shrink the drawer between min/max,
// but never past the point that would starve the main stage below MIN_VP.
const resizeDrawer = (id: string, min: number, max: number) => (e: MouseEvent) => {
  const el = document.getElementById(id);
  if (!el) return;
  const r = el.getBoundingClientRect();
  const maxAllowed = r.width + Math.max(0, stageWidth() - MIN_VP);
  el.style.width = `${Math.min(Math.max(r.right - e.clientX, min), Math.min(max, maxAllowed))}px`;
};

// Divider drag (editor ↔ viewport): 10% min for the editor; the right side must
// leave at least MIN_VP px for the viewport (drawer-stack is a flex sibling so
// its width counts against available space).
makeDragResizer(divider, (e) => {
  const appRect = app.getBoundingClientRect();
  const x = e.clientX - appRect.left;
  const pct = (x / appRect.width) * 100;
  const drawerW = drawerStack.getBoundingClientRect().width;
  const maxPct = ((appRect.width - drawerW - MIN_VP) / appRect.width) * 100;
  const clamped = Math.min(Math.max(pct, 10), Math.max(10, maxPct));
  editorPanel.style.flexBasis = `${clamped}%`;
});
makeDragResizer(panelResizer, resizeDrawer('assistant-panel', 200, 700));
makeDragResizer(docsResizer, resizeDrawer('docs-panel', 240, 900));
