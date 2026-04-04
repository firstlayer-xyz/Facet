import './style.css';
import { createEditor, EditorHandle } from './editor';
import { Viewer } from './viewer';
import { GetDefaultSource, GetExample, DetectSlicers, SetAssistantConfig, CreateLocalLibrary, CreateLibraryFolder, ListLibraryFolders, OpenRecentFile, OpenLibraryDir } from '../wailsjs/go/main/App';
import { EventsOn, ClipboardSetText, ClipboardGetText, WindowToggleMaximise } from '../wailsjs/runtime/runtime';
import { loadSettings, saveSettings, createSettingsPanel } from './settings';

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
  getSources, getActiveTabValue, getActiveLabel, restoreTabCursor,
  isPreviewLocked, setPreviewLocked, isDebugStepping,
  setOnTabChange, setOnSourceChange, setOnDebugFilesChange, setOnEntryPoints,
  setEntryOverrides, refreshEditorUI,
} from './app';
import { resolveThemePalette, resolveUiTheme, resolveEditorTheme, applyUIPalette } from './themes';

let settings: Awaited<ReturnType<typeof loadSettings>>;
let viewer: Viewer;

import { promptNewLibrary, showSlicerPicker } from './dialogs';

/** Resolve UI palette and apply to CSS vars, viewport, and editor theme. */
function applyCurrentTheme(): void {
  // UI palette (from appearance settings)
  const uiId = resolveUiTheme(settings.appearance.uiTheme, settings.appearance.darkMode);
  const palette = resolveThemePalette(uiId, settings.appearance.themeOverrides, settings.appearance.customThemes);
  applyUIPalette(palette);
  viewer.applySettings({
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
    bed: settings.appearance.bed,
    gridSize: settings.appearance.gridSize,
    gridSpacing: settings.appearance.gridSpacing,
  });

  // Editor theme (follows UI theme)
  editorRef?.setTheme(resolveEditorTheme(
    settings.appearance.uiTheme,
    settings.appearance.darkMode,
    settings.appearance.customThemes,
  ));
}

// Docs panel
const docsPanel = new DocsPanel(canvasContainer, async () => {
  const active = await toggleDocs();
  docsBtn.classList.toggle('active', active);
});

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
  () => viewer.captureScreenshot(),
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
  SetAssistantConfig(settings.assistant as any);

  // Initialize 3D viewer with theme-derived viewport colors
  const _initUiId = resolveUiTheme(settings.appearance.uiTheme, settings.appearance.darkMode);
  const _initPalette = resolveThemePalette(_initUiId, settings.appearance.themeOverrides, settings.appearance.customThemes);
  applyUIPalette(_initPalette);
  viewer = new Viewer(canvasContainer, {
    backgroundColor: _initPalette.viewBg,
    meshColor: _initPalette.viewMesh ?? _initPalette.accent,
    meshMetalness: _initPalette.viewMeshMetalness,
    meshRoughness: _initPalette.viewMeshRoughness,
    edgeColor: _initPalette.viewEdgeColor,
    edgeOpacity: _initPalette.viewEdgeOpacity,
    edgeThreshold: _initPalette.viewEdgeThreshold,
    ambientIntensity: _initPalette.viewAmbientIntensity,
    gridMajorColor: _initPalette.viewGridMajor,
    gridMinorColor: _initPalette.viewGridMinor,
    bed: settings.appearance.bed,
    gridSize: settings.appearance.gridSize,
    gridSpacing: settings.appearance.gridSpacing,
  });

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
    debugBtn,
    tabBar,
    statsBar,
    compilingOverlay,
  });

  // Restore saved tab state or load default tutorial
  const savedTabs = (settings as any).savedTabs as { path: string; label: string; cursor: { lineNumber: number; column: number } | null }[] | undefined;
  const savedActiveTab = (settings as any).activeTab as string | undefined;

  let initialSource = '';
  let initialFileKey = 'example:Tutorial.fct';
  let initialReadOnly = true;

  // Find the first tab to load as the initial editor content
  const firstTab = savedTabs?.[0] ?? null;
  if (firstTab) {
    try {
      const result = await OpenRecentFile(firstTab.path);
      if (result?.source) {
        initialSource = result.source;
        initialFileKey = result.path;
        initialReadOnly = false;
      }
    } catch {
      // file no longer exists — fall through to tutorial
    }
  }
  if (!initialSource) {
    initialSource = await GetDefaultSource();
    initialFileKey = 'example:Tutorial.fct';
    initialReadOnly = true;
  }

  const editor = createEditor(editorPanel, initialSource, autoRun, async (name) => {
    await openDocsToEntry(name);
    docsBtn.classList.add('active');
  }, (file, source, line, col) => {
    openLibraryFile(file, source, line, col);
  }, initialFileKey);
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

  // Set initial file and show tab
  const initialLabel = initialFileKey.startsWith('example:')
    ? initialFileKey.replace(/^example:/, '').replace(/\.fct$/, '')
    : undefined;
  setInitialFile(initialFileKey, initialLabel, initialReadOnly);
  if (firstTab?.cursor) {
    editor.revealLine(firstTab.cursor.lineNumber, firstTab.cursor.column);
  }

  // Restore remaining saved tabs
  if (savedTabs && savedTabs.length > 1) {
    for (const saved of savedTabs) {
      if (saved.path === initialFileKey) continue; // already open
      try {
        const result = await OpenRecentFile(saved.path);
        if (result?.source) {
          await openLibraryTab(result.path, result.source);
          // Restore cursor for this tab
          if (saved.cursor) {
            restoreTabCursor(saved.path, saved.cursor);
          }
        }
      } catch {
        // file no longer exists — skip
      }
    }
  }
  // Switch to the previously active tab
  if (savedActiveTab && savedActiveTab !== getActiveTabValue()) {
    switchToTab(savedActiveTab);
    const savedTab = savedTabs?.find(t => t.path === savedActiveTab);
    if (savedTab?.cursor) {
      editor.revealLine(savedTab.cursor.lineNumber, savedTab.cursor.column);
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
            ? '(' + fnParams.map((p: any) => `${p.name}: ${p.type}`).join(', ') + ')'
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

  // Preview file dropdown toggle
  previewFileBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    if (previewMenuDirty) { buildPreviewMenu(); previewMenuDirty = false; }
    const open = previewFileMenu.classList.toggle('show');
    previewFileBtn.classList.toggle('open', open);
  });

  // Close on outside click
  document.addEventListener('click', () => {
    previewFileMenu.classList.remove('show');
    previewFileBtn.classList.remove('open');
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

  // Initial tree render (source is initialSource which was set into editor)
  fileTree.update(initialSource, initialFileKey);
  updatePreviewLabel(initialFileKey);

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
init();

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
    SetAssistantConfig(settings.assistant as any);
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
    errorDiv.textContent = err?.message || 'Camera access denied';
    errorDiv.style.display = 'block';
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
EventsOn('menu:debug', () => { const active = toggleDebug(); debugBtn.classList.toggle('active', active); });

// View menu
EventsOn('menu:fullcode', () => { if (fullCodeActive) exitFullCode(); else enterFullCode(); });
EventsOn('menu:toggle-grid', () => viewer.toggleGrid());
EventsOn('menu:toggle-axes', () => viewer.toggleAxes());
EventsOn('menu:docs', async () => {
  const active = await toggleDocs();
  docsBtn.classList.toggle('active', active);
});

// Model menu
EventsOn('menu:assistant', () => assistantBtn.click());
EventsOn('menu:slicer', () => slicerBtn.click());
EventsOn('menu:slicer-id', (id: string) => sendToSlicer(id));

// Window menu
EventsOn('menu:settings', () => settingsBtn.click());

// Manual run / stop button (in preview selector)
runBtn.addEventListener('click', toggleRun);

// Debug toggle
debugBtn.addEventListener('click', () => {
  const active = toggleDebug();
  debugBtn.classList.toggle('active', active);
  if (active) run();
});

// Assistant toggle
assistantBtn.addEventListener('click', () => {
  assistantPanel.toggle();
  const assistantVisible = assistantPanel.isVisible();
  assistantBtn.classList.toggle('active', assistantVisible);
  panelResizer.style.display = assistantVisible ? 'block' : 'none';
});

// Docs toggle
docsBtn.addEventListener('click', async () => {
  const active = await toggleDocs();
  docsBtn.classList.toggle('active', active);
});

// Full-code view toggle — editor fills the screen, viewport shrinks to a floating preview
let fullCodeActive = false;
let fullCodeAutoRotating = false;
let fullCodeDragMove: ((e: MouseEvent) => void) | null = null;
let fullCodeDragUp: (() => void) | null = null;

function enterFullCode() {
  fullCodeActive = true;
  fullCodeBtn.classList.remove('active'); // preview is now hidden

  // Collapse viewport, expand editor
  divider.style.display = 'none';
  viewportPanel.style.display = 'none';
  editorPanel.style.flex = '1';

  // Lift assistant panel out of the hidden viewport panel so it can
  // still float over the editor when toggled.
  const assistantEl = document.getElementById('assistant-panel');
  if (assistantEl) { app.appendChild(assistantEl); assistantEl.classList.add('fullcode-float'); }

  // Create floating preview anchored to the bottom-right of the editor panel
  const preview = document.createElement('div');
  preview.id = 'mini-preview';
  editorPanel.style.overflow = 'visible';
  editorPanel.appendChild(preview);
  // Drag handle bar at top of preview — separates drag from orbit controls
  const dragBar = document.createElement('div');
  dragBar.id = 'mini-preview-drag';
  preview.appendChild(dragBar);

  preview.appendChild(canvasContainer);

  // Zoom button to restore split view
  const zoomBtn = document.createElement('button');
  zoomBtn.id = 'mini-preview-zoom';
  zoomBtn.title = 'Restore split view';
  zoomBtn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>`;
  zoomBtn.addEventListener('click', (e) => { e.stopPropagation(); exitFullCode(); });
  preview.appendChild(zoomBtn);

  // Make preview draggable via drag bar only
  let dragOffsetX = 0, dragOffsetY = 0, dragging = false;
  dragBar.addEventListener('mousedown', (e) => {
    dragging = true;
    dragOffsetX = e.clientX - preview.getBoundingClientRect().left;
    dragOffsetY = e.clientY - preview.getBoundingClientRect().top;
    dragBar.style.cursor = 'grabbing';
    e.preventDefault();
  });
  fullCodeDragMove = (e: MouseEvent) => {
    if (!dragging) return;
    const parent = preview.parentElement!;
    const parentRect = parent.getBoundingClientRect();
    let x = e.clientX - parentRect.left - dragOffsetX;
    let y = e.clientY - parentRect.top - dragOffsetY;
    // Clamp to parent bounds
    x = Math.max(0, Math.min(x, parentRect.width - preview.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - preview.offsetHeight));
    preview.style.left = x + 'px';
    preview.style.top = y + 'px';
    preview.style.right = 'auto';
    preview.style.bottom = 'auto';
  };
  fullCodeDragUp = () => {
    if (dragging) {
      dragging = false;
      dragBar.style.cursor = '';
    }
  };
  document.addEventListener('mousemove', fullCodeDragMove);
  document.addEventListener('mouseup', fullCodeDragUp);

  // Force renderer to adopt new (smaller) container dimensions
  requestAnimationFrame(() => viewer.resize());

  // Start auto-rotate (track whether it was already on)
  fullCodeAutoRotating = viewer.isAutoRotating();
  if (!fullCodeAutoRotating) {
    viewer.toggleAutoRotate();
  }
  autoRotateBtn.classList.add('active');
}

function exitFullCode() {
  if (fullCodeDragMove) { document.removeEventListener('mousemove', fullCodeDragMove); fullCodeDragMove = null; }
  if (fullCodeDragUp) { document.removeEventListener('mouseup', fullCodeDragUp); fullCodeDragUp = null; }

  fullCodeActive = false;
  fullCodeBtn.classList.add('active'); // preview is visible again

  // Move canvas back before panelResizer
  viewportPanel.insertBefore(canvasContainer, panelResizer);

  document.getElementById('mini-preview')?.remove();

  // Return assistant panel to the viewport panel
  const assistantEl = document.getElementById('assistant-panel');
  if (assistantEl) { assistantEl.classList.remove('fullcode-float'); viewportPanel.insertBefore(assistantEl, panelResizer); }

  // Restore layout
  divider.style.display = '';
  viewportPanel.style.display = '';
  editorPanel.style.flex = '';
  editorPanel.style.overflow = '';

  // Force renderer to fill the restored viewport
  requestAnimationFrame(() => viewer.resize());

  // Stop auto-rotate (it was started by enterFullCode)
  if (!fullCodeAutoRotating && viewer.isAutoRotating()) {
    viewer.toggleAutoRotate();
    autoRotateBtn.classList.remove('active');
  }
}

fullCodeBtn.addEventListener('click', () => {
  if (fullCodeActive) exitFullCode();
  else enterFullCode();
});

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

// Render mode seg (3D / Wire)
viewpointBar.addEventListener('click', (e) => {
  const target = e.target as HTMLElement;
  const renderBtn = target.closest<HTMLButtonElement>('.vp-seg-btn[data-render-id]');
  if (!renderBtn) return;
  for (const btn of viewpointBar.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-render-id]')) btn.classList.remove('active');
  renderBtn.classList.add('active');
  const isWire = renderBtn.dataset.renderId === 'wireframe';
  viewer.setWireframeMode(false);
  viewer.setDrawingMode(isWire);
  // Show/hide hidden lines toggle
  hiddenLinesBtn.style.display = isWire ? '' : 'none';
  if (!isWire) {
    hiddenLinesBtn.classList.remove('active');
    viewer.setHiddenLines(false);
  }
  if (isWire) viewer.setViewpoint(currentPreset);
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

document.addEventListener('click', () => {
  vpPresetMenu.classList.remove('show');
  vpPresetBtn.classList.remove('open');
});

// Projection toggle (Persp / Ortho)
viewpointBar.addEventListener('click', (e) => {
  const projBtn = (e.target as HTMLElement).closest<HTMLButtonElement>('.vp-seg-btn[data-proj-id]');
  if (!projBtn) return;
  for (const btn of viewpointBar.querySelectorAll<HTMLButtonElement>('.vp-seg-btn[data-proj-id]')) btn.classList.remove('active');
  projBtn.classList.add('active');
  viewer.setOrthoProjection(projBtn.dataset.projId === 'ortho');
});

// Export (toolbar button + titlebar button both trigger export)
exportBtn.addEventListener('click', () => exportMesh());
document.getElementById('titlebar-export-btn')!.addEventListener('click', () => exportMesh());

async function pickAndSendToSlicer() {
  const slicers = await DetectSlicers();
  if (!slicers || slicers.length === 0) {
    errorDiv.textContent = 'No slicer found — install BambuStudio, OrcaSlicer, PrusaSlicer, Cura, or AnycubicSlicer';
    errorDiv.style.display = 'block';
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
