// fullcode.ts — Full-code view mode (editor fills screen, viewport shrinks to floating preview)

import type { Viewer } from './viewer';

interface FullCodeDeps {
  viewer: Viewer;
  editorPanel: HTMLElement;
  viewportPanel: HTMLElement;
  canvasContainer: HTMLElement;
  divider: HTMLElement;
  app: HTMLElement;
  fullCodeBtn: HTMLElement;
}

let active = false;
let listeners: AbortController | null = null;
let deps: FullCodeDeps;

export function initFullCode(d: FullCodeDeps) {
  deps = d;
  deps.fullCodeBtn.addEventListener('click', toggleFullCode);
}

export function toggleFullCode() {
  if (active) exit(); else enter();
}

export function isFullCode(): boolean {
  return active;
}

function enter() {
  const { viewer, editorPanel, viewportPanel, canvasContainer, divider, app, fullCodeBtn } = deps;
  active = true;
  fullCodeBtn.classList.remove('active');

  // Collapse viewport, expand editor
  divider.style.display = 'none';
  viewportPanel.style.display = 'none';
  editorPanel.style.flex = '1';

  // Drawers (assistant, docs) live in #drawer-stack permanently — they
  // are flex siblings of #app's children and don't need to be touched.

  // Create floating preview anchored to bottom-right of the full app area.
  // Parented to #app (position:relative) so it can float anywhere in the window.
  const preview = document.createElement('div');
  preview.id = 'mini-preview';
  app.appendChild(preview);

  const dragBar = document.createElement('div');
  dragBar.id = 'mini-preview-drag';
  preview.appendChild(dragBar);
  preview.appendChild(canvasContainer);

  // Dock button — restores split view. Uses a two-panel split icon so the
  // user reads it as "return to split layout", not "go fullscreen".
  const dockBtn = document.createElement('button');
  dockBtn.id = 'mini-preview-zoom';
  dockBtn.title = 'Restore split view';
  dockBtn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="18" rx="2"/><line x1="9" y1="3" x2="9" y2="21"/></svg>`;
  dockBtn.addEventListener('click', (e) => { e.stopPropagation(); exit(); });
  preview.appendChild(dockBtn);

  // Resize handle — bottom-right corner grip
  const resizeHandle = document.createElement('div');
  resizeHandle.id = 'mini-preview-resize';
  preview.appendChild(resizeHandle);

  // Convert the preview from its default right/bottom CSS anchoring to
  // explicit left/top. Both drag and resize need this so the edges they
  // move stay pinned the way the user expects (otherwise growing width on
  // a right-anchored box extends the *left* edge).
  const pinTopLeft = () => {
    const r = preview.getBoundingClientRect();
    const a = app.getBoundingClientRect();
    preview.style.left = (r.left - a.left) + 'px';
    preview.style.top = (r.top - a.top) + 'px';
    preview.style.right = 'auto';
    preview.style.bottom = 'auto';
  };

  // Coalesce viewer.resize() to one call per animation frame so a resize
  // drag doesn't trigger a WebGL realloc on every mousemove.
  let resizePending = false;
  const scheduleResize = () => {
    if (resizePending) return;
    resizePending = true;
    requestAnimationFrame(() => { resizePending = false; viewer.resize(); });
  };

  listeners = new AbortController();
  const opts = { signal: listeners.signal };

  // ── Drag to reposition ──
  let dragOffsetX = 0, dragOffsetY = 0, dragging = false;
  dragBar.addEventListener('mousedown', (e) => {
    dragging = true;
    pinTopLeft();
    dragOffsetX = e.clientX - preview.getBoundingClientRect().left;
    dragOffsetY = e.clientY - preview.getBoundingClientRect().top;
    dragBar.style.cursor = 'grabbing';
    e.preventDefault();
  });
  const dragMove = (e: MouseEvent) => {
    if (!dragging) return;
    const appRect = app.getBoundingClientRect();
    let x = e.clientX - appRect.left - dragOffsetX;
    let y = e.clientY - appRect.top - dragOffsetY;
    x = Math.max(0, Math.min(x, appRect.width - preview.offsetWidth));
    y = Math.max(0, Math.min(y, appRect.height - preview.offsetHeight));
    preview.style.left = x + 'px';
    preview.style.top = y + 'px';
  };
  const dragUp = () => {
    if (dragging) {
      dragging = false;
      dragBar.style.cursor = '';
    }
  };
  document.addEventListener('mousemove', dragMove, opts);
  document.addEventListener('mouseup', dragUp, opts);

  // ── Resize by dragging the bottom-right handle ──
  let resizing = false;
  let resizeStartX = 0, resizeStartY = 0;
  let resizeStartW = 0, resizeStartH = 0;
  resizeHandle.addEventListener('mousedown', (e) => {
    resizing = true;
    pinTopLeft();
    resizeStartX = e.clientX;
    resizeStartY = e.clientY;
    resizeStartW = preview.offsetWidth;
    resizeStartH = preview.offsetHeight;
    e.preventDefault();
    e.stopPropagation();
  });
  const resizeMove = (e: MouseEvent) => {
    if (!resizing) return;
    const appRect = app.getBoundingClientRect();
    const previewRect = preview.getBoundingClientRect();
    const maxW = appRect.right - previewRect.left;
    const maxH = appRect.bottom - previewRect.top;
    const newW = Math.min(Math.max(resizeStartW + (e.clientX - resizeStartX), 160), maxW);
    const newH = Math.min(Math.max(resizeStartH + (e.clientY - resizeStartY), 120), maxH);
    preview.style.width = newW + 'px';
    preview.style.height = newH + 'px';
    scheduleResize();
  };
  const resizeUp = () => {
    if (resizing) resizing = false;
  };
  document.addEventListener('mousemove', resizeMove, opts);
  document.addEventListener('mouseup', resizeUp, opts);

  requestAnimationFrame(() => viewer.resize());
}

function exit() {
  const { viewer, editorPanel, viewportPanel, canvasContainer, divider, fullCodeBtn } = deps;
  listeners?.abort();
  listeners = null;

  active = false;
  fullCodeBtn.classList.add('active');

  // Move canvas back into the viewport panel.
  viewportPanel.appendChild(canvasContainer);
  document.getElementById('mini-preview')?.remove();

  // Restore layout
  divider.style.display = '';
  viewportPanel.style.display = '';
  editorPanel.style.flex = '';

  requestAnimationFrame(() => viewer.resize());
}
