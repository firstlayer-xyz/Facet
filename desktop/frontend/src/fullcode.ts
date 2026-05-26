// fullcode.ts — Full-code view mode (editor fills screen, viewport shrinks to floating preview)

import type { Viewer } from './viewer';

interface FullCodeDeps {
  viewer: Viewer;
  editorPanel: HTMLElement;
  viewportPanel: HTMLElement;
  canvasContainer: HTMLElement;
  divider: HTMLElement;
  panelResizer: HTMLElement;
  app: HTMLElement;
  fullCodeBtn: HTMLElement;
  autoRotateBtn: HTMLElement;
}

let active = false;
let autoRotating = false;
let dragMove: ((e: MouseEvent) => void) | null = null;
let dragUp: (() => void) | null = null;
let deps: FullCodeDeps;

export function initFullCode(d: FullCodeDeps) {
  deps = d;
  deps.fullCodeBtn.addEventListener('click', () => {
    if (active) exit(); else enter();
  });
}

export function toggleFullCode() {
  if (active) exit(); else enter();
}

function enter() {
  const { viewer, editorPanel, viewportPanel, canvasContainer, divider, app, fullCodeBtn, autoRotateBtn } = deps;
  active = true;
  fullCodeBtn.classList.remove('active');

  // Collapse viewport, expand editor
  divider.style.display = 'none';
  viewportPanel.style.display = 'none';
  editorPanel.style.flex = '1';

  // Lift assistant panel so it can float over the editor
  const assistantEl = document.getElementById('assistant-panel');
  if (assistantEl) { app.appendChild(assistantEl); assistantEl.classList.add('fullcode-float'); }

  // Create floating preview anchored to the bottom-right of the editor panel
  const preview = document.createElement('div');
  preview.id = 'mini-preview';
  editorPanel.style.overflow = 'visible';
  editorPanel.appendChild(preview);

  const dragBar = document.createElement('div');
  dragBar.id = 'mini-preview-drag';
  preview.appendChild(dragBar);
  preview.appendChild(canvasContainer);

  // Zoom button to restore split view
  const zoomBtn = document.createElement('button');
  zoomBtn.id = 'mini-preview-zoom';
  zoomBtn.title = 'Restore split view';
  zoomBtn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>`;
  zoomBtn.addEventListener('click', (e) => { e.stopPropagation(); exit(); });
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
  dragMove = (e: MouseEvent) => {
    if (!dragging) return;
    const parent = preview.parentElement!;
    const parentRect = parent.getBoundingClientRect();
    let x = e.clientX - parentRect.left - dragOffsetX;
    let y = e.clientY - parentRect.top - dragOffsetY;
    x = Math.max(0, Math.min(x, parentRect.width - preview.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - preview.offsetHeight));
    preview.style.left = x + 'px';
    preview.style.top = y + 'px';
    preview.style.right = 'auto';
    preview.style.bottom = 'auto';
  };
  dragUp = () => {
    if (dragging) {
      dragging = false;
      dragBar.style.cursor = '';
    }
  };
  document.addEventListener('mousemove', dragMove);
  document.addEventListener('mouseup', dragUp);

  requestAnimationFrame(() => viewer.resize());

  // Start auto-rotate (track whether it was already on)
  autoRotating = viewer.isAutoRotating();
  if (!autoRotating) {
    viewer.toggleAutoRotate();
  }
  autoRotateBtn.classList.add('active');
}

function exit() {
  const { viewer, editorPanel, viewportPanel, canvasContainer, divider, panelResizer, fullCodeBtn, autoRotateBtn } = deps;
  if (dragMove) { document.removeEventListener('mousemove', dragMove); dragMove = null; }
  if (dragUp) { document.removeEventListener('mouseup', dragUp); dragUp = null; }

  active = false;
  fullCodeBtn.classList.add('active');

  // Move canvas back
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

  requestAnimationFrame(() => viewer.resize());

  // Stop auto-rotate if we started it
  if (!autoRotating && viewer.isAutoRotating()) {
    viewer.toggleAutoRotate();
    autoRotateBtn.classList.remove('active');
  }
}
