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

// ── Bottom-left HUD tool cluster ──
const hudTools = document.createElement('div');
hudTools.id = 'hud-tools';
hudTools.className = 'hud';

/** Create a HUD button (16×16 icon) and append it to the HUD cluster. */
function makeHudBtn(id: string, title: string, svgInner: string): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.id = id;
  btn.className = 'hud-btn';
  btn.title = title;
  btn.innerHTML = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${svgInner}</svg>`;
  hudTools.appendChild(btn);
  return btn;
}

export const centerBtn = makeHudBtn('center-btn', 'Center model', '<circle cx="12" cy="12" r="3"/><line x1="12" y1="2" x2="12" y2="6"/><line x1="12" y1="18" x2="12" y2="22"/><line x1="2" y1="12" x2="6" y2="12"/><line x1="18" y1="12" x2="22" y2="12"/>');

export const autoRotateBtn = makeHudBtn('auto-rotate-btn', 'Auto-center & rotate', '<path d="M21.5 2v6h-6"/><path d="M21.34 15.57a10 10 0 11-.57-8.38L21.5 8"/>');

export const headTrackBtn = makeHudBtn('head-track-btn', 'Head tracking parallax', '<path d="M9 3H5a2 2 0 00-2 2v4"/><path d="M15 3h4a2 2 0 012 2v4"/><path d="M9 21H5a2 2 0 01-2-2v-4"/><path d="M15 21h4a2 2 0 002-2v-4"/><circle cx="12" cy="10" r="3"/><path d="M12 13c-2.5 0-4 1.5-4 3"/>');

const hudDivider = document.createElement('div');
hudDivider.className = 'hud-divider';
hudTools.appendChild(hudDivider);

// Measure tool — toggle click-to-place dimension mode (M).
export const measureBtn = makeHudBtn('measure-btn', 'Measure (M) — click two points to place a dimension', '<path d="M3 16l13-13 5 5-13 13z"/><path d="M7 12l2 2"/><path d="M10 9l2 2"/><path d="M13 6l2 2"/>');

// Extents — place an overall bounding-box dimension.
export const extentsBtn = makeHudBtn('extents-btn', 'Show extents — overall size of the model', '<rect x="3" y="3" width="18" height="18" rx="1"/><path d="M3 8h18"/><path d="M3 16h18"/><path d="M8 3v18"/><path d="M16 3v18"/>');

// Clear dimensions — discard all placed measurements.
export const clearDimsBtn = makeHudBtn('clear-dims-btn', 'Clear all dimensions', '<path d="M3 6h18"/><path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>');

canvasContainer.appendChild(hudTools);

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

export const newBtn = makeBtn('new-btn', 'New (\u2318N)', 'NEW', '<path d="M13 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V9z"/><polyline points="13 2 13 9 20 9"/>');
fileGroup.appendChild(newBtn);

export const openBtn = makeBtn('open-btn', 'Open (\u2318O)', 'OPEN', '<path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>');
fileGroup.appendChild(openBtn);

export const saveBtn = makeBtn('save-btn', 'Save (\u2318S)', 'SAVE', '<path d="M19 21H5a2 2 0 01-2-2V5a2 2 0 012-2h11l5 5v11a2 2 0 01-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/>');
fileGroup.appendChild(saveBtn);

export const shareBtn = makeBtn('share-btn', 'Share to web preview', 'SHARE', '<path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/>');
fileGroup.appendChild(shareBtn);

export const exportBtn = makeBtn('export-btn', 'Export', 'EXP', '<path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>');
export const slicerBtn = makeBtn('slicer-btn', 'Send to Slicer', 'SLICE', '<rect x="4" y="14" width="16" height="8" rx="1"/><line x1="6" y1="18" x2="18" y2="18"/><line x1="6" y1="16" x2="18" y2="16"/><path d="M10 14V6h4v8"/><path d="M8 6h8"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="10" y1="3" x2="14" y2="3"/>');

toolbar.appendChild(fileGroup);

// ── Panels group ──
const panelsGroup = makeGroup();

export const codeBtn = makeBtn('code-btn', 'Toggle code editor', 'CODE', '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>');
codeBtn.classList.add('active'); // editor visible by default
panelsGroup.appendChild(codeBtn);

export const fullCodeBtn = makeBtn('fullcode-btn', 'Toggle preview', 'PREV', '<rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>');
fullCodeBtn.classList.add('active'); // preview visible by default
panelsGroup.appendChild(fullCodeBtn);

export const docsBtn = makeBtn('docs-btn', 'Docs', 'DOCS', '<path d="M2 3h6a4 4 0 014 4v14a3 3 0 00-3-3H2z"/><path d="M22 3h-6a4 4 0 00-4 4v14a3 3 0 013-3h7z"/>');
panelsGroup.appendChild(docsBtn);

export const assistantBtn = makeBtn('assistant-btn', 'AI Assistant', 'ASST', '<path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>');
panelsGroup.appendChild(assistantBtn);

toolbar.appendChild(panelsGroup);

// ── Bottom group (pushed to bottom) ──
const bottomGroup = makeGroup();
bottomGroup.style.marginTop = 'auto';

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

// Debug strip — top-of-viewport banner shown while debug is active
export const debugBar = document.createElement('div');
debugBar.id = 'debug-strip';
debugBar.style.display = 'none';

const dbgBadge = document.createElement('div');
dbgBadge.className = 'dbg-badge';
const dbgDot = document.createElement('span');
dbgDot.className = 'dbg-dot';
dbgDot.textContent = '●';
const dbgBadgeLabel = document.createElement('span');
dbgBadgeLabel.className = 'dbg-badge-label';
dbgBadgeLabel.textContent = 'DEBUG';
dbgBadge.appendChild(dbgDot);
dbgBadge.appendChild(dbgBadgeLabel);
debugBar.appendChild(dbgBadge);

const dbgControls = document.createElement('div');
dbgControls.className = 'dbg-controls';

/** Create a debug-strip control button and append it to the debug controls. */
function makeDbgBtn(cls: string, title: string, text: string): HTMLButtonElement {
  const btn = document.createElement('button');
  btn.className = cls;
  btn.title = title;
  btn.textContent = text;
  dbgControls.appendChild(btn);
  return btn;
}

export const debugRestartBtn = makeDbgBtn('dbg-ctrl-btn', 'Restart debug session (⇧⌘R)', '↺');

export const debugPrevBtn = makeDbgBtn('dbg-ctrl-btn', 'Step back (⇧F10)', '‹');

export const debugContinueBtn = makeDbgBtn('dbg-ctrl-btn primary', 'Continue to next breakpoint, or end (F5)', '▶');

export const debugNextBtn = makeDbgBtn('dbg-ctrl-btn', 'Step forward (F10)', '›');

export const debugStopBtn = makeDbgBtn('dbg-ctrl-btn stop', 'Stop debug (⇧F5)', '■');

debugBar.appendChild(dbgControls);

const dbgScrubber = document.createElement('div');
dbgScrubber.className = 'dbg-scrubber';

export const debugLabel = document.createElement('div');
debugLabel.className = 'dbg-meta';
dbgScrubber.appendChild(debugLabel);

export const debugSlider = document.createElement('input');
debugSlider.type = 'range';
debugSlider.id = 'debug-slider-strip';
debugSlider.min = '0';
debugSlider.max = '0';
debugSlider.value = '0';
dbgScrubber.appendChild(debugSlider);

debugBar.appendChild(dbgScrubber);

// Stats bar (bottom of viewport, shows model info after run)
export const statsBar = document.createElement('div');
statsBar.id = 'stats-bar';
statsBar.className = 'hud';
statsBar.style.display = 'none';

export const compilingOverlay = document.createElement('div');
compilingOverlay.id = 'compiling-overlay';
compilingOverlay.innerHTML = `<div class="compiling-spinner"></div>`;
compilingOverlay.style.display = 'none';

// View pane — collapsible panel top-right of viewport
export const vpPane = document.createElement('div');
vpPane.id = 'vp-pane';

const vpHd = document.createElement('div');
vpHd.className = 'vp-pane-hd';

const vpLabel = document.createElement('span');
vpLabel.className = 'vp-pane-label';
vpLabel.textContent = 'VIEW';
vpHd.appendChild(vpLabel);

export const vpPaneSummary = document.createElement('span');
vpPaneSummary.className = 'vp-pane-summary';
vpPaneSummary.textContent = '3D · Persp · ISO';
vpHd.appendChild(vpPaneSummary);

const vpChevron = document.createElement('span');
vpChevron.className = 'vp-pane-chevron';
vpChevron.textContent = '▾';
vpHd.appendChild(vpChevron);

vpPane.appendChild(vpHd);
vpHd.addEventListener('click', () => vpPane.classList.toggle('open'));

const vpBody = document.createElement('div');
vpBody.className = 'vp-pane-body';

// ── Shade row ──
const shadeRow = document.createElement('div');
shadeRow.className = 'vp-pane-row';
const shadeRowLabel = document.createElement('span');
shadeRowLabel.className = 'vp-pane-row-label';
shadeRowLabel.textContent = 'SHADE';
shadeRow.appendChild(shadeRowLabel);
const shadeSeg = document.createElement('div');
shadeSeg.className = 'vp-seg';
for (const [id, label] of [['3d', '3D'], ['wireframe', 'Wire']] as const) {
  const btn = document.createElement('button');
  btn.className = 'vp-seg-btn' + (id === '3d' ? ' active' : '');
  btn.dataset.renderId = id;
  btn.textContent = label;
  shadeSeg.appendChild(btn);
}
shadeRow.appendChild(shadeSeg);

export const hiddenLinesBtn = document.createElement('button');
hiddenLinesBtn.className = 'vp-seg-btn vp-hidden-btn';
hiddenLinesBtn.textContent = 'Hidden';
hiddenLinesBtn.title = 'Show hidden lines (dashed)';
hiddenLinesBtn.style.display = 'none';
shadeRow.appendChild(hiddenLinesBtn);

vpBody.appendChild(shadeRow);

// ── Cam row ──
const camRow = document.createElement('div');
camRow.className = 'vp-pane-row';
const camRowLabel = document.createElement('span');
camRowLabel.className = 'vp-pane-row-label';
camRowLabel.textContent = 'CAM';
camRow.appendChild(camRowLabel);
const camSeg = document.createElement('div');
camSeg.className = 'vp-seg';
for (const [id, label] of [['persp', 'Persp'], ['ortho', 'Ortho']] as const) {
  const btn = document.createElement('button');
  btn.className = 'vp-seg-btn' + (id === 'persp' ? ' active' : '');
  btn.dataset.projId = id;
  btn.textContent = label;
  camSeg.appendChild(btn);
}
camRow.appendChild(camSeg);
vpBody.appendChild(camRow);

// ── Orient row ──
const orientRow = document.createElement('div');
orientRow.className = 'vp-pane-row';
const orientRowLabel = document.createElement('span');
orientRowLabel.className = 'vp-pane-row-label';
orientRowLabel.textContent = 'ORIENT';
orientRow.appendChild(orientRowLabel);
const orientGrid = document.createElement('div');
orientGrid.className = 'vp-orient-grid';
for (const [id, label] of [
  ['iso', 'ISO'], ['top', 'Top'], ['bot', 'Bot'], ['home', 'Home'],
  ['front', 'Front'], ['back', 'Back'], ['left', 'Left'], ['right', 'Right'],
] as const) {
  const btn = document.createElement('button');
  btn.className = 'vp-orient-btn' + (id === 'iso' ? ' active' : '');
  btn.dataset.viewpoint = id;
  btn.textContent = label;
  orientGrid.appendChild(btn);
}
orientRow.appendChild(orientGrid);
vpBody.appendChild(orientRow);

vpPane.appendChild(vpBody);

// Preview selector — floating pill at top-left of canvas showing which file is previewed.
export const previewSelector = document.createElement('div');
previewSelector.id = 'preview-selector';
previewSelector.className = 'hud';

export const previewFileBtn = document.createElement('button');
previewFileBtn.id = 'preview-file-btn';
previewFileBtn.innerHTML = `<span id="preview-file-lbl">main</span><span class="preview-file-arr">▾</span>`;
previewSelector.appendChild(previewFileBtn);

export const previewFileMenu = document.createElement('div');
previewFileMenu.id = 'preview-file-menu';
previewSelector.appendChild(previewFileMenu);

export const runBtn = document.createElement('button');
runBtn.id = 'run-btn';
runBtn.title = 'Run';
runBtn.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`;
previewSelector.appendChild(runBtn);

export const debugBtn = document.createElement('button');
debugBtn.id = 'debug-btn';
debugBtn.title = 'Debug';
debugBtn.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 012 2v2"/><path d="M6 4H4a2 2 0 00-2 2v2"/><path d="M2 14h4"/><path d="M18 14h4"/><path d="M12 14v8"/><path d="M2 10h4"/><path d="M18 10h4"/><rect x="6" y="6" width="12" height="12" rx="2"/></svg>`;
previewSelector.appendChild(debugBtn);

previewSelector.appendChild(slicerBtn);
previewSelector.appendChild(exportBtn);

canvasContainer.style.position = 'relative';
canvasContainer.appendChild(previewSelector);
canvasContainer.appendChild(debugBar);
canvasContainer.appendChild(vpPane);
canvasContainer.appendChild(statsBar);
canvasContainer.appendChild(compilingOverlay);

// Top-level overlay container for the right-edge drawers (docs + assistant).
// Lives directly under #app so it overlays editor + canvas via absolute
// position rather than competing with them for horizontal flex space.
//
// Children, left to right: docs-resizer, docs slot, assistant-resizer,
// assistant slot. Left-most in this stack sits furthest from #app's
// right edge — so docs ends up inside of assistant when both are open.
export const drawerStack = document.createElement('div');
drawerStack.id = 'drawer-stack';

// Docs resizer (left edge of the docs slot). Visibility toggled via
// `.open` class in lockstep with the docs slot.
export const docsResizer = document.createElement('div');
docsResizer.id = 'docs-resizer';
drawerStack.appendChild(docsResizer);

// Assistant resizer (left edge of the assistant panel). The id is
// `assistant-resizer`; the export name `panelResizer` is kept so the
// existing resize-handler import in main.ts continues to work.
export const panelResizer = document.createElement('div');
panelResizer.id = 'assistant-resizer';
drawerStack.appendChild(panelResizer);

// DocsPanel + AssistantPanel append their own panels into drawerStack
// at runtime (in show() and the constructor respectively). The drawer
// stack uses CSS `order:` to keep arrangement consistent regardless of
// when each panel was inserted:
//   docs-resizer (1) | docs-panel (2) | assistant-resizer (3) | assistant-panel (4)
// — so when both are open, docs sits inside of assistant from the
// right edge of the app.
app.appendChild(drawerStack);

export { app };
