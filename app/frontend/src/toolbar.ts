// toolbar.ts — DOM creation for toolbar buttons and layout scaffolding.

// Build split layout DOM
const app = document.getElementById('app')!;

export const editorPanel = document.createElement('div');
editorPanel.id = 'editor-panel';

export const divider = document.createElement('div');
divider.id = 'divider';

export const viewportPanel = document.createElement('div');
viewportPanel.id = 'viewport-panel';

export const canvasContainer = document.createElement('div');
canvasContainer.id = 'canvas-container';
viewportPanel.appendChild(canvasContainer);

export const centerBtn = document.createElement('button');
centerBtn.id = 'center-btn';
centerBtn.title = 'Center model';
centerBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><line x1="12" y1="2" x2="12" y2="6"/><line x1="12" y1="18" x2="12" y2="22"/><line x1="2" y1="12" x2="6" y2="12"/><line x1="18" y1="12" x2="22" y2="12"/></svg>`;
canvasContainer.appendChild(centerBtn);

export const autoRotateBtn = document.createElement('button');
autoRotateBtn.id = 'auto-rotate-btn';
autoRotateBtn.title = 'Auto-center & rotate';
autoRotateBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.5 2v6h-6"/><path d="M21.34 15.57a10 10 0 11-.57-8.38L21.5 8"/></svg>`;
canvasContainer.appendChild(autoRotateBtn);

export const headTrackBtn = document.createElement('button');
headTrackBtn.id = 'head-track-btn';
headTrackBtn.title = 'Head tracking parallax';
headTrackBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 3H5a2 2 0 00-2 2v4"/><path d="M15 3h4a2 2 0 012 2v4"/><path d="M9 21H5a2 2 0 01-2-2v-4"/><path d="M15 21h4a2 2 0 002-2v-4"/><circle cx="12" cy="10" r="3"/><path d="M12 13c-2.5 0-4 1.5-4 3"/></svg>`;
canvasContainer.appendChild(headTrackBtn);

// Measure tool — toggle click-to-place dimension mode (M).
export const measureBtn = document.createElement('button');
measureBtn.id = 'measure-btn';
measureBtn.title = 'Measure (M) — click two points to place a dimension';
measureBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 16l13-13 5 5-13 13z"/><path d="M7 12l2 2"/><path d="M10 9l2 2"/><path d="M13 6l2 2"/></svg>`;
canvasContainer.appendChild(measureBtn);

// Extents — place an overall bounding-box dimension.
export const extentsBtn = document.createElement('button');
extentsBtn.id = 'extents-btn';
extentsBtn.title = 'Show extents — overall size of the model';
extentsBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="1"/><path d="M3 8h18"/><path d="M3 16h18"/><path d="M8 3v18"/><path d="M16 3v18"/></svg>`;
canvasContainer.appendChild(extentsBtn);

// Clear dimensions — discard all placed measurements.
export const clearDimsBtn = document.createElement('button');
clearDimsBtn.id = 'clear-dims-btn';
clearDimsBtn.title = 'Clear all dimensions';
clearDimsBtn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/></svg>`;
canvasContainer.appendChild(clearDimsBtn);

app.appendChild(editorPanel);
app.appendChild(divider);
app.appendChild(viewportPanel);

// ---------------------------------------------------------------------------
// Toolbar — grouped buttons
// ---------------------------------------------------------------------------

function makeGroup(): HTMLDivElement {
  const g = document.createElement('div');
  g.className = 'toolbar-group';
  return g;
}

/** Create a toolbar button with an icon and a short text label below it. */
function makeBtn(id: string, title: string, label: string, svgInner: string): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.id = id;
  btn.title = title;
  btn.innerHTML = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${svgInner}</svg><span class="toolbar-btn-label">${label}</span>`;
  return btn;
}

export const toolbar = document.createElement('div');
toolbar.id = 'toolbar';

// ── File group ──
const fileGroup = makeGroup();

export const openBtn = makeBtn('open-btn', 'Open (\u2318O)', 'OPEN', '<path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>');
fileGroup.appendChild(openBtn);

export const saveBtn = makeBtn('save-btn', 'Save (\u2318S)', 'SAVE', '<path d="M19 21H5a2 2 0 01-2-2V5a2 2 0 012-2h11l5 5v11a2 2 0 01-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/>');
fileGroup.appendChild(saveBtn);

export const exportBtn = makeBtn('export-btn', 'Export', 'EXP', '<path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>');
fileGroup.appendChild(exportBtn);

export const slicerBtn = makeBtn('slicer-btn', 'Send to Slicer', 'SLICE', '<rect x="4" y="14" width="16" height="8" rx="1"/><line x1="6" y1="18" x2="18" y2="18"/><line x1="6" y1="16" x2="18" y2="16"/><path d="M10 14V6h4v8"/><path d="M8 6h8"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="10" y1="3" x2="14" y2="3"/>');
fileGroup.appendChild(slicerBtn);

toolbar.appendChild(fileGroup);

// ── Panels group ──
const panelsGroup = makeGroup();

export const fullCodeBtn = makeBtn('fullcode-btn', 'Preview', 'VIEW', '<rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>');
fullCodeBtn.classList.add('active'); // preview visible by default
panelsGroup.appendChild(fullCodeBtn);

export const filesBtn = makeBtn('files-btn', 'Files', 'FILES', '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>');
panelsGroup.appendChild(filesBtn);

export const assistantBtn = makeBtn('assistant-btn', 'AI Assistant', 'ASST', '<path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>');
panelsGroup.appendChild(assistantBtn);

toolbar.appendChild(panelsGroup);

// ── Bottom group (pushed to bottom) ──
const bottomGroup = makeGroup();
bottomGroup.style.marginTop = 'auto';

export const docsBtn = makeBtn('docs-btn', 'Docs', 'DOCS', '<path d="M2 3h6a4 4 0 014 4v14a3 3 0 00-3-3H2z"/><path d="M22 3h-6a4 4 0 00-4 4v14a3 3 0 013-3h7z"/>');
bottomGroup.appendChild(docsBtn);

export const settingsBtn = makeBtn('settings-btn', 'Settings', 'SET', '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>');
bottomGroup.appendChild(settingsBtn);

toolbar.appendChild(bottomGroup);

app.insertBefore(toolbar, editorPanel);

export const errorDiv = document.createElement('div');
errorDiv.id = 'error-bar';
errorDiv.style.display = 'none';

// Tab bar
export const tabBar = document.createElement('div');
tabBar.id = 'tab-bar';
editorPanel.appendChild(tabBar);

// Error bar (between tabs and editor)
editorPanel.appendChild(errorDiv);

// Debug bar (step controls overlay at bottom of viewport)
export const debugBar = document.createElement('div');
debugBar.id = 'debug-bar';
debugBar.style.display = 'none';

export const debugPrevBtn = document.createElement('button');
debugPrevBtn.textContent = '<';
debugBar.appendChild(debugPrevBtn);

export const debugNextBtn = document.createElement('button');
debugNextBtn.textContent = '>';
debugBar.appendChild(debugNextBtn);

export const debugSlider = document.createElement('input');
debugSlider.type = 'range';
debugSlider.min = '0';
debugSlider.max = '0';
debugSlider.value = '0';
debugBar.appendChild(debugSlider);

export const debugLabel = document.createElement('span');
debugLabel.id = 'debug-label';
debugBar.appendChild(debugLabel);

// Stats bar (bottom of viewport, shows model info after run)
export const statsBar = document.createElement('div');
statsBar.id = 'stats-bar';
statsBar.style.display = 'none';

export const compilingOverlay = document.createElement('div');
compilingOverlay.id = 'compiling-overlay';
compilingOverlay.innerHTML = `<div class="compiling-spinner"></div>`;
compilingOverlay.style.display = 'none';

// Render mode bar — always visible at top-center of canvas.
// [3D | Wire]  |  [ISO] [Top] [Front] [Right] [Left]  |  [Persp | Ortho]
export const viewpointBar = document.createElement('div');
viewpointBar.id = 'viewpoint-bar';

// ── Render-mode segmented control ──
const vpSeg = document.createElement('div');
vpSeg.className = 'vp-seg';
for (const [id, label] of [['3d', '3D'], ['wireframe', 'Wire']] as const) {
  const btn = document.createElement('button');
  btn.className = 'vp-seg-btn' + (id === '3d' ? ' active' : '');
  btn.dataset.renderId = id;
  btn.textContent = label;
  vpSeg.appendChild(btn);
}
viewpointBar.appendChild(vpSeg);

// ── Hidden lines toggle (only visible in Wire mode) ──
export const hiddenLinesBtn = document.createElement('button');
hiddenLinesBtn.className = 'vp-seg-btn vp-hidden-btn';
hiddenLinesBtn.textContent = 'Hidden';
hiddenLinesBtn.title = 'Show hidden lines (dashed)';
hiddenLinesBtn.style.display = 'none';
viewpointBar.appendChild(hiddenLinesBtn);

// ── Separator ──
const vpBarSep1 = document.createElement('div');
vpBarSep1.className = 'vp-bar-sep';
viewpointBar.appendChild(vpBarSep1);

// ── Camera preset dropdown ──
const vpPresetWrap = document.createElement('div');
vpPresetWrap.className = 'vp-cam-wrap';

export const vpPresetBtn = document.createElement('button');
vpPresetBtn.id = 'vp-preset-btn';
vpPresetBtn.className = 'vp-cam-btn';
vpPresetBtn.innerHTML = `<span id="vp-preset-lbl">ISO</span><span class="vp-cam-arr">▾</span>`;
vpPresetWrap.appendChild(vpPresetBtn);

export const vpPresetMenu = document.createElement('div');
vpPresetMenu.id = 'vp-preset-menu';
vpPresetMenu.className = 'vp-cam-menu';

for (const [id, label] of [['iso', 'Isometric'], ['top', 'Top'], ['front', 'Front'], ['right', 'Right'], ['left', 'Left']] as const) {
  const item = document.createElement('div');
  item.className = 'vp-cam-item' + (id === 'iso' ? ' on' : '');
  item.dataset.viewpoint = id;
  item.innerHTML = `<span class="vp-cam-chk">${id === 'iso' ? '✓' : ''}</span>${label}`;
  vpPresetMenu.appendChild(item);
}
vpPresetWrap.appendChild(vpPresetMenu);
viewpointBar.appendChild(vpPresetWrap);

// ── Separator ──
const vpBarSep2 = document.createElement('div');
vpBarSep2.className = 'vp-bar-sep';
viewpointBar.appendChild(vpBarSep2);

// ── Projection toggle ──
const vpProjSeg = document.createElement('div');
vpProjSeg.className = 'vp-seg';
for (const [id, label] of [['persp', 'Persp'], ['ortho', 'Ortho']] as const) {
  const btn = document.createElement('button');
  btn.className = 'vp-seg-btn' + (id === 'persp' ? ' active' : '');
  btn.dataset.projId = id;
  btn.textContent = label;
  vpProjSeg.appendChild(btn);
}
viewpointBar.appendChild(vpProjSeg);

// Preview selector — floating pill at top-left of canvas showing which file is previewed.
export const previewSelector = document.createElement('div');
previewSelector.id = 'preview-selector';

export const previewLockBtn = document.createElement('button');
previewLockBtn.id = 'preview-lock-btn';
previewLockBtn.title = 'Lock preview (prevent auto-switching)';
previewLockBtn.innerHTML = `<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0110 0v4"/></svg>`;
previewSelector.appendChild(previewLockBtn);

export const previewFileBtn = document.createElement('button');
previewFileBtn.id = 'preview-file-btn';
previewFileBtn.innerHTML = `<span id="preview-file-lbl">main</span><span class="preview-file-arr">▾</span>`;
previewSelector.appendChild(previewFileBtn);

export const previewFileMenu = document.createElement('div');
previewFileMenu.id = 'preview-file-menu';
previewSelector.appendChild(previewFileMenu);

export const debugBtn = document.createElement('button');
debugBtn.id = 'debug-btn';
debugBtn.title = 'Debug';
debugBtn.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 012 2v2"/><path d="M6 4H4a2 2 0 00-2 2v2"/><path d="M2 14h4"/><path d="M18 14h4"/><path d="M12 14v8"/><path d="M2 10h4"/><path d="M18 10h4"/><rect x="6" y="6" width="12" height="12" rx="2"/></svg>`;
previewSelector.appendChild(debugBtn);

export const runBtn = document.createElement('button');
runBtn.id = 'run-btn';
runBtn.title = 'Run';
runBtn.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`;
previewSelector.appendChild(runBtn);

canvasContainer.style.position = 'relative';
canvasContainer.appendChild(previewSelector);
canvasContainer.appendChild(viewpointBar);
canvasContainer.appendChild(debugBar);
canvasContainer.appendChild(statsBar);
canvasContainer.appendChild(compilingOverlay);

// Resizer between canvas and the right panels (params / assistant)
export const panelResizer = document.createElement('div');
panelResizer.id = 'panel-resizer';
panelResizer.style.display = 'none';
viewportPanel.appendChild(panelResizer);

export { app };
