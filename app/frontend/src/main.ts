import './style.css';
import { createEditor, EditorHandle } from './editor';
import { Viewer } from './viewer';
import { GetDefaultSource, GetExample, DetectSlicers, SetAssistantConfig, CreateLocalLibrary, CreateLibraryFolder, ListLibraryFolders, OpenRecentFile, OpenLibraryDir } from '../wailsjs/go/main/App';
import { EventsOn, ClipboardSetText, ClipboardGetText, WindowToggleMaximise } from '../wailsjs/runtime/runtime';
import { loadSettings, saveSettings, createSettingsPanel, SettingsCorruptError } from './settings';

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
  centerBtn, autoRotateBtn, headTrackBtn, openBtn, saveBtn, settingsBtn, docsBtn, runBtn,
  debugBtn, assistantBtn, exportBtn, slicerBtn, fullCodeBtn, filesBtn, errorDiv, tabBar,
  debugBar, debugPrevBtn, debugNextBtn, debugSlider, debugLabel, statsBar, compilingOverlay,
  viewpointBar, vpPresetBtn, vpPresetMenu, hiddenLinesBtn, panelResizer,
  previewLockBtn, previewFileBtn, previewFileMenu,
} from './toolbar';
import { FileTree } from './filetree';
import { FunctionPreview } from './function-preview';
import type { EntryPoint } from './function-preview';
import type { DrawingViewpoint } from './viewer';

import {
  initApp, setEditor, setInitialFile, setEditorWordWrap, setFormatOnSave, setHighlightMode,
  run, autoRun, toggleRun,
  showDebugStep, showDebugStepPrev, showDebugStepNext,
  openExample, openFile, openRecentFile, saveFile, saveFileAs, newFile, exportMesh, sendToSlicer,
  reeval, toggleDebug, toggleDocs, openDocsToEntry, openLibraryFile, openLibraryTab,
  switchToTab,
  getSources, getActiveTabValue, getActiveLabel, addRestoredTab, renderTabs,
  isPreviewLocked, setPreviewLocked, isDebugStepping,
  setOnTabChange, setOnSourceChange, setOnDebugFilesChange, setOnDebugExit, setOnEntryPoints,
  setEntryOverrides, refreshEditorUI, showError,
} from './app';
import { resolveThemePalette, resolveUiTheme, resolveEditorTheme, applyUIPalette } from './themes';

let settings: Awaited<ReturnType<typeof loadSettings>>;
let viewer: Viewer;

import { promptNewLibrary, showSlicerPicker } from './dialogs';
import { initFullCode, toggleFullCode } from './fullcode';

function buildViewerAppearance(palette: ReturnType<typeof resolveThemePalette>, appearance: typeof settings.appearance) {
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
  viewer.applySettings(buildViewerAppearance(palette, settings.appearance));

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
}

// Docs panel
const docsPanel = new DocsPanel(canvasContainer, handleDocsToggle);

// Assistant panel
let editorRef: EditorHandle | null = null;
const assistantPanel = new AssistantPanel(
  viewportPanel,
  () => editorRef?.getContent() ?? '',
  () => errorDiv.textContent ?? '',
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
  viewer = new Viewer(canvasContainer, buildViewerAppearance(_initPalette, settings.appearance));

  // Initialize app module with all dependencies
  initApp({
    viewer,
    editor: null!, // set below after async creation
    docsPanel,
    errorDiv,
    debugBar,
    debugSlider,
    debugLabel,
    centerBtn,
    autoRotateBtn,
    tabBar,
    statsBar,
    compilingOverlay,
  });

  initFullCode({
    viewer, editorPanel, viewportPanel, canvasContainer, divider,
    panelResizer, app, fullCodeBtn, autoRotateBtn,
  });

  // Restore saved tab state or load default tutorial
  const savedTabs = settings.savedTabs;
  const savedActiveTab = settings.activeTab;

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

  // Fall back to default tutorial when no saved tabs could be loaded
  if (loadedTabs.length === 0) {
    const source = await GetDefaultSource();
    loadedTabs.push({ source, path: 'example:Tutorial.fct', readOnly: true, cursor: null });
  }

  // First loaded tab initializes the editor
  const first = loadedTabs[0];
  const editor = createEditor(editorPanel, first.source, autoRun, async (name) => {
    await openDocsToEntry(name);
    docsBtn.classList.add('active');
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
  setInitialFile(first.path, undefined, first.readOnly);
  if (first.cursor) {
    editor.revealLine(first.cursor.lineNumber, first.cursor.column);
  }
  for (let i = 1; i < loadedTabs.length; i++) {
    const tab = loadedTabs[i];
    editor.preloadModel(tab.path, tab.source);
    addRestoredTab(tab.path, tab.cursor);
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

  // ── File tree panel ────────────────────────────────────────────────────────
  const fileTree = new FileTree({
    getActiveLabel,
    getActiveTab: getActiveTabValue,
    getSources,
    openTab(path, source) {
      openLibraryTab(path, source);
    },
  });
  // Insert tree panel between tab-bar and Monaco editor container
  editorPanel.insertBefore(fileTree.element, tabBar.nextSibling);

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

  // Called by run() after GetEntryPoints resolves.
  // Picks the entry point function and returns its name (or null to skip running).
  setOnEntryPoints((fns) => {
    currentEntryPoints = fns;
    previewMenuDirty = true;
    const tab = getActiveTabValue();
    const picked = pickEntryPoint(tab, fns);
    if (!picked) return null;
    const reconciledOverrides = functionPreview.updateUI(picked);
    setEntryOverrides(reconciledOverrides);
    return { name: picked.name, libPath: picked.libPath };
  });

  // ── Preview selector wiring ────────────────────────────────────────────────
  const previewFileLbl = document.getElementById('preview-file-lbl')!;
  let previewMenuDirty = true;

  function updatePreviewLabel(tab: string) {
    if (selectedFnKey !== null) return; // keep showing function name
    if (tab === '') {
      previewFileLbl.textContent = getActiveLabel();
    } else {
      let name = tab.split('/').pop() || tab;
      if (name.endsWith('.fct')) name = name.slice(0, -4);
      previewFileLbl.textContent = name;
    }
  }

  function buildPreviewMenu() {
    previewFileMenu.innerHTML = '';
    const activeTab = getActiveTabValue();

    // Show only entry points matching the active tab
    const visibleFns = currentEntryPoints.filter(f => f.libPath === activeTab);
    if (visibleFns.length > 0) {
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
  }

  // Update tree + preview selector whenever source or active tab changes
  setOnSourceChange((source) => {
    const tab = getActiveTabValue();
    fileTree.update(source, tab);
    if (!isPreviewLocked()) updatePreviewLabel(tab);
  });

  setOnTabChange((tab) => {
    fileTree.setActiveTab(tab);
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
    const tab = getActiveTabValue();
    fileTree.setActiveTab(tab);
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

  // Close all dropdowns on outside click (preview file + viewpoint preset)
  document.addEventListener('click', () => {
    previewFileMenu.classList.remove('show');
    previewFileBtn.classList.remove('open');
    vpPresetMenu.classList.remove('show');
    vpPresetBtn.classList.remove('open');
  });

  // Preview lock toggle
  previewLockBtn.addEventListener('click', () => {
    const locked = !isPreviewLocked();
    setPreviewLocked(locked);
    previewLockBtn.classList.toggle('locked', locked);
    previewLockBtn.title = locked
      ? 'Preview locked — click to unlock (auto-follow active file)'
      : 'Lock preview (prevent auto-switching)';
    // If unlocking, re-run main file
    if (!locked) {
      updatePreviewLabel(getActiveTabValue());
      run();
    }
  });

  // Initial tree render
  fileTree.update(first.source, first.path);
  updatePreviewLabel(first.path);

  // Files button toggle
  filesBtn.addEventListener('click', () => {
    const visible = fileTree.toggle();
    filesBtn.classList.toggle('active', visible);
  });

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

openBtn.addEventListener('click', openFile);
saveBtn.addEventListener('click', saveFile);

settingsBtn.addEventListener('click', () => {
  if (document.getElementById('settings-overlay')) return;
  const panel = createSettingsPanel(settings, (updated) => {
    settings = updated;
    saveSettings(settings);
    applyCurrentTheme();
    setEditorWordWrap(settings.editor.wordWrap);
    setFormatOnSave(settings.editor.formatOnSave);
    setHighlightMode(settings.editor.highlight);
    SetAssistantConfig(settings.assistant);
  });
  app.appendChild(panel);
});

// Center model button
centerBtn.addEventListener('click', () => viewer.fitToView());

// Auto-rotate button
autoRotateBtn.addEventListener('click', () => {
  const active = viewer.toggleAutoRotate();
  autoRotateBtn.classList.toggle('active', active);
});

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
EventsOn('menu:new', () => newFile());
EventsOn('menu:open', () => openFile());
EventsOn('menu:open-recent', (path: string) => openRecentFile(path));
EventsOn('menu:open-demo', async (name: string) => {
  try {
    const source = await GetExample(name);
    openExample(source, name);
  } catch (err) {
    console.error('Failed to load example:', err);
  }
});
EventsOn('menu:open-library', async (dir: string) => {
  try {
    const result = await OpenLibraryDir(dir);
    openExample(result.source, result.path);
  } catch (err) {
    console.error('Failed to open library:', err);
  }
});
EventsOn('menu:new-library', async () => {
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
EventsOn('menu:save', () => saveFile());
EventsOn('menu:save-as', () => saveFileAs());
EventsOn('menu:export', (format: string) => exportMesh(format));

// Run menu
EventsOn('menu:run', () => toggleRun());
EventsOn('menu:debug', handleDebugToggle);

// View menu
EventsOn('menu:fullcode', toggleFullCode);
EventsOn('menu:toggle-grid', () => viewer.toggleGrid());
EventsOn('menu:toggle-axes', () => viewer.toggleAxes());
EventsOn('menu:docs', handleDocsToggle);

// Model menu
EventsOn('menu:assistant', () => assistantBtn.click());
EventsOn('menu:slicer', () => slicerBtn.click());
EventsOn('menu:slicer-id', (id: string) => sendToSlicer(id));

// Window menu
EventsOn('menu:settings', () => settingsBtn.click());

// Manual run / stop button (in preview selector)
runBtn.addEventListener('click', toggleRun);

// Debug toggle
function handleDebugToggle() {
  const active = toggleDebug();
  debugBtn.classList.toggle('active', active);
  if (active) run();
}
debugBtn.addEventListener('click', handleDebugToggle);

// Assistant toggle
assistantBtn.addEventListener('click', () => {
  assistantPanel.toggle();
  const assistantVisible = assistantPanel.isVisible();
  assistantBtn.classList.toggle('active', assistantVisible);
  panelResizer.style.display = assistantVisible ? 'block' : 'none';
});

// Docs toggle
docsBtn.addEventListener('click', handleDocsToggle);

// Viewpoint bar — [3D | Wire] | [ISO ▾] | [Persp | Ortho]
let currentPreset: DrawingViewpoint = 'iso';

function setPreset(vp: DrawingViewpoint) {
  currentPreset = vp;
  const lblEl = document.getElementById('vp-preset-lbl');
  const shortLabel: Record<DrawingViewpoint, string> = { iso: 'ISO', top: 'Top', front: 'Front', right: 'Right', left: 'Left' };
  if (lblEl) lblEl.textContent = shortLabel[vp];
  for (const item of vpPresetMenu.querySelectorAll<HTMLElement>('.vp-cam-item')) {
    const on = item.dataset.viewpoint === vp;
    item.classList.toggle('on', on);
    item.querySelector('.vp-cam-chk')!.textContent = on ? '✓' : '';
  }
  viewer.setViewpoint(vp);
}

// Viewpoint bar — delegated click handler for render mode and projection toggle
viewpointBar.addEventListener('click', (e) => {
  const target = e.target as HTMLElement;
  // Render mode (3D / Wire)
  const renderBtn = target.closest<HTMLButtonElement>('.vp-seg-btn[data-render-id]');
  if (renderBtn) {
    for (const btn of viewpointBar.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-render-id]')) btn.classList.remove('active');
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
    return;
  }
  // Projection toggle (Persp / Ortho)
  const projBtn = target.closest<HTMLButtonElement>('.vp-seg-btn[data-proj-id]');
  if (projBtn) {
    for (const btn of viewpointBar.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-proj-id]')) btn.classList.remove('active');
    projBtn.classList.add('active');
    viewer.setOrthoProjection(projBtn.dataset.projId === 'ortho');
  }
});

hiddenLinesBtn.addEventListener('click', () => {
  const on = !hiddenLinesBtn.classList.contains('active');
  hiddenLinesBtn.classList.toggle('active', on);
  viewer.setHiddenLines(on);
});

// Preset dropdown — toggle
vpPresetBtn.addEventListener('click', (e) => {
  e.stopPropagation();
  const open = vpPresetMenu.classList.toggle('show');
  vpPresetBtn.classList.toggle('open', open);
});

// Preset dropdown — select
vpPresetMenu.addEventListener('click', (e) => {
  const item = (e.target as HTMLElement).closest<HTMLElement>('.vp-cam-item');
  if (!item) return;
  vpPresetMenu.classList.remove('show');
  vpPresetBtn.classList.remove('open');
  setPreset(item.dataset.viewpoint as DrawingViewpoint);
});

// Export (toolbar button + titlebar button both trigger export)
exportBtn.addEventListener('click', () => exportMesh());
document.getElementById('titlebar-export-btn')!.addEventListener('click', () => exportMesh());

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

// Debug bar controls
debugPrevBtn.addEventListener('click', showDebugStepPrev);
debugNextBtn.addEventListener('click', showDebugStepNext);
debugSlider.addEventListener('input', () => showDebugStep(parseInt(debugSlider.value, 10)));

// Divider drag logic (editor ↔ viewport)
let dragging = false;

divider.addEventListener('mousedown', (e) => {
  e.preventDefault();
  dragging = true;
  document.body.style.cursor = 'col-resize';
  document.body.style.userSelect = 'none';
});

// Panel resizer drag logic (canvas ↔ right panel)
let panelDragging = false;

panelResizer.addEventListener('mousedown', (e) => {
  e.preventDefault();
  panelDragging = true;
  document.body.style.cursor = 'col-resize';
  document.body.style.userSelect = 'none';
});

window.addEventListener('mousemove', (e) => {
  if (dragging) {
    const appRect = app.getBoundingClientRect();
    const x = e.clientX - appRect.left;
    const pct = (x / appRect.width) * 100;
    const clamped = Math.min(Math.max(pct, 10), 90);
    editorPanel.style.flexBasis = `${clamped}%`;
  }
  if (panelDragging) {
    const vpRect = viewportPanel.getBoundingClientRect();
    const newWidth = vpRect.right - e.clientX;
    const clamped = Math.min(Math.max(newWidth, 200), 700);
    const activePanel = document.getElementById('assistant-panel');
    if (activePanel) activePanel.style.width = `${clamped}px`;
  }
});

window.addEventListener('mouseup', () => {
  if (dragging) {
    dragging = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }
  if (panelDragging) {
    panelDragging = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }
});
