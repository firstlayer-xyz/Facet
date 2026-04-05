import type { CustomTheme } from './themes';
import { PatchSettings, GetSettings } from '../wailsjs/go/main/App';
import type { SettingsPageContext, PageResult } from './settings_appearance';
import { buildAppearancePage } from './settings_appearance';
import { buildEditorPage } from './settings_editor';
import { buildLibrariesPage } from './settings_libraries';
import { buildAssistantPage } from './settings_assistant';
import { buildCameraPage } from './settings_camera';
import { buildMemoryPage } from './settings_memory';
import { buildDebugPage } from './settings_debug';
import { buildSlicerPage } from './settings_slicer';

export interface AppSettings {
  appearance: {
    bed: string;
    gridSize: number;
    gridSpacing: number;
    darkMode: 'light' | 'dark' | 'auto';
    uiTheme: string;
    themeOverrides: Record<string, string | number>;
    customThemes: CustomTheme[];
  };
  editor: {
    wordWrap: boolean;
    formatOnSave: boolean;
    highlight: 'mouse' | 'cursor' | 'off';
  };
  libraries: LibraryEntry[];
  assistant: {
    cli: string;
    model: string;
    systemPrompt: string;
    maxTurns: number;
  };
  camera: {
    deviceId: string;
    yOffset: number; // shift neutral point down to account for webcam placement (0..1)
  };
  librarySettings: {
    autoPull: boolean;
  };
  slicer: {
    defaultSlicer: string; // slicer ID or '' for auto-detect
  };
}

export interface LibraryEntry {
  url: string;
  ref: string;
}

const STORAGE_KEY = 'facet-settings';

const defaults: AppSettings = {
  appearance: {
    bed: 'XY',
    gridSize: 250,
    gridSpacing: 10,
    darkMode: 'auto',
    uiTheme: 'facet-orange',
    themeOverrides: {},
    customThemes: [],
  },
  editor: {
    wordWrap: true,
    formatOnSave: true,
    highlight: 'cursor' as const,
  },
  libraries: [],
  assistant: {
    cli: '',
    model: '',
    systemPrompt: '',
    maxTurns: 10,
  },
  camera: {
    deviceId: '',
    yOffset: 0.3,
  },
  librarySettings: {
    autoPull: false,
  },
  slicer: {
    defaultSlicer: '',
  },
};

function mergeWithDefaults(parsed: any): AppSettings {
  // Migrate old modules[] to libraries[]
  let libraries: LibraryEntry[] = [];
  if (Array.isArray(parsed.libraries)) {
    libraries = parsed.libraries;
  } else if (Array.isArray(parsed.modules)) {
    libraries = parsed.modules.map((url: string) => ({ url, ref: 'main' }));
  }
  // Migrate old autoTheme boolean to darkMode
  const appearance = { ...defaults.appearance, ...parsed.appearance };
  if (parsed.appearance?.autoTheme !== undefined && !parsed.appearance?.darkMode) {
    appearance.darkMode = parsed.appearance.autoTheme ? 'auto' : 'light';
    delete (appearance as any).autoTheme;
  }
  // Migrate old theme IDs with explicit light/dark to base names
  const themeRenames: Record<string, string> = {
    'github-dark': 'github', 'solarized-dark': 'solarized', 'solarized-light': 'solarized',
    'tomorrow-night': 'tomorrow', 'tomorrow': 'tomorrow',
    'cobalt': 'cobalt', 'dracula': 'dracula', 'monokai': 'monokai',
    'night-owl': 'night-owl', 'nord': 'nord',
  };
  if (appearance.uiTheme in themeRenames) {
    appearance.uiTheme = themeRenames[appearance.uiTheme];
  }
  // Only pick known keys from each section — don't preserve stale fields
  const pick = <T extends Record<string, any>>(def: T, src: any): T => {
    if (!src || typeof src !== 'object') return { ...def };
    const result = { ...def };
    for (const key of Object.keys(def)) {
      if (key in src) (result as any)[key] = src[key];
    }
    return result;
  };
  return {
    ...parsed, // preserve Go-owned keys (lastFile, recentFiles, savedTabs, etc.)
    appearance,
    editor: pick(defaults.editor, parsed.editor),
    libraries,
    assistant: pick(defaults.assistant, parsed.assistant),
    camera: pick(defaults.camera, parsed.camera),
    librarySettings: pick(defaults.librarySettings, parsed.librarySettings),
    slicer: pick(defaults.slicer, parsed.slicer),
  };
}

export async function loadSettings(): Promise<AppSettings> {
  // One-time migration from localStorage to Go backend
  try {
    const oldRaw = localStorage.getItem(STORAGE_KEY);
    if (oldRaw) {
      await PatchSettings(oldRaw);
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch { /* ignore */ }

  try {
    const json = await GetSettings();
    const parsed = JSON.parse(json);
    return mergeWithDefaults(parsed);
  } catch {
    return structuredClone(defaults);
  }
}

/** Patch specific settings keys. Only provided keys are updated on disk. */
export function patchSettings(partial: Record<string, any>): void {
  PatchSettings(JSON.stringify(partial));
}

/** Save all frontend-owned settings sections. */
export function saveSettings(s: AppSettings): void {
  patchSettings({
    appearance: s.appearance,
    editor: s.editor,
    libraries: s.libraries,
    assistant: s.assistant,
    camera: s.camera,
    librarySettings: s.librarySettings,
    slicer: s.slicer,
  });
}

// Inline SVG icons
const paletteIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="13.5" cy="6.5" r="2"/><circle cx="17.5" cy="10.5" r="2"/><circle cx="8.5" cy="7.5" r="2"/><circle cx="6.5" cy="12" r="2"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.9 0 1.5-.7 1.5-1.5 0-.4-.1-.7-.4-1-.3-.3-.4-.7-.4-1 0-.8.7-1.5 1.5-1.5H16c3.3 0 6-2.7 6-6 0-5.5-4.5-9-10-9z"/></svg>`;
const packageIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16.5 9.4l-9-5.2M21 16V8a2 2 0 00-1-1.7l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.7l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/><polyline points="3.3 7 12 12 20.7 7"/><line x1="12" y1="22" x2="12" y2="12"/></svg>`;
const editorIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`;
const assistantIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a4 4 0 014 4v1h2a2 2 0 012 2v10a2 2 0 01-2 2H6a2 2 0 01-2-2V9a2 2 0 012-2h2V6a4 4 0 014-4z"/><circle cx="9" cy="13" r="1"/><circle cx="15" cy="13" r="1"/><path d="M9 17h6"/></svg>`;
const cameraIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/><circle cx="12" cy="13" r="4"/></svg>`;
const memoryIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/></svg>`;
const slicerIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2"/><polyline points="12 2 2 7 12 12 22 7 12 2"/><line x1="2" y1="12" x2="12" y2="17"/><line x1="22" y1="12" x2="12" y2="17"/></svg>`;
const debugIcon = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>`;

export function createSettingsPanel(
  current: AppSettings,
  onSave: (s: AppSettings) => void,
): HTMLElement {
  const draft = structuredClone(current);
  const ctx = { draft, onSave };

  // Root overlay (backdrop)
  const overlay = document.createElement('div');
  overlay.id = 'settings-overlay';

  // Dialog box
  const dialog = document.createElement('div');
  dialog.id = 'settings-dialog';
  overlay.appendChild(dialog);

  // Sidebar
  const sidebar = document.createElement('div');
  sidebar.id = 'settings-sidebar';

  function sidebarButton(icon: string, title: string): HTMLButtonElement {
    const btn = document.createElement('button');
    btn.className = 'settings-sidebar-btn';
    btn.innerHTML = icon;
    btn.title = title;
    sidebar.appendChild(btn);
    return btn;
  }

  const appearanceBtn = sidebarButton(paletteIcon, 'Appearance');
  const editorBtn = sidebarButton(editorIcon, 'Editor');
  const librariesBtn = sidebarButton(packageIcon, 'Libraries');
  const assistantSettingsBtn = sidebarButton(assistantIcon, 'Assistant');
  const slicerSettingsBtn = sidebarButton(slicerIcon, 'Slicer');
  const cameraBtn = sidebarButton(cameraIcon, 'Camera');
  const memoryBtn = sidebarButton(memoryIcon, 'Memory');
  const debugSettingsBtn = sidebarButton(debugIcon, 'Log');

  // Content area
  const content = document.createElement('div');
  content.id = 'settings-content';

  // Drag handle (title bar)
  const dragHandle = document.createElement('div');
  dragHandle.className = 'settings-drag-handle';

  // Close button (left side of title bar)
  const closeBtn = document.createElement('button');
  closeBtn.id = 'settings-close-btn';
  closeBtn.innerHTML = '&times;';
  closeBtn.title = 'Close settings';
  dragHandle.appendChild(closeBtn);

  // Title in drag handle (centered)
  const titleSpan = document.createElement('span');
  titleSpan.className = 'settings-title';
  titleSpan.textContent = 'Appearance';
  dragHandle.appendChild(titleSpan);

  // Spacer to balance close button for centered title
  const spacer = document.createElement('div');
  spacer.style.width = '28px';
  spacer.style.flexShrink = '0';
  dragHandle.appendChild(spacer);

  // Drag logic
  let dragging = false;
  let dragStartX = 0;
  let dragStartY = 0;
  let dialogStartX = 0;
  let dialogStartY = 0;

  dragHandle.addEventListener('mousedown', (e) => {
    if (e.target === closeBtn) return;
    dragging = true;
    dragStartX = e.clientX;
    dragStartY = e.clientY;
    const rect = dialog.getBoundingClientRect();
    dialogStartX = rect.left;
    dialogStartY = rect.top;
    // Switch from centered to absolute positioning
    dialog.style.position = 'fixed';
    dialog.style.left = rect.left + 'px';
    dialog.style.top = rect.top + 'px';
    dialog.style.margin = '0';
    overlay.style.alignItems = 'stretch';
    overlay.style.justifyContent = 'stretch';
    dragHandle.style.cursor = 'grabbing';
    e.preventDefault();
  });

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);

  function onMouseMove(e: MouseEvent) {
    if (!dragging) return;
    dialog.style.left = (dialogStartX + e.clientX - dragStartX) + 'px';
    dialog.style.top = (dialogStartY + e.clientY - dragStartY) + 'px';
  }

  function onMouseUp() {
    if (!dragging) return;
    dragging = false;
    dragHandle.style.cursor = '';
  }

  // Body wrapper (sidebar + content in a row)
  const body = document.createElement('div');
  body.className = 'settings-body';
  body.appendChild(sidebar);
  body.appendChild(content);

  dialog.appendChild(dragHandle);
  dialog.appendChild(body);

  // Active page cleanup callback
  let pageCleanup: (() => void) | null = null;

  function close() {
    if (pageCleanup) {
      pageCleanup();
      pageCleanup = null;
    }
    document.removeEventListener('keydown', onKeyDown);
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
    onSave(draft);
    overlay.remove();
  }

  // Close on backdrop click (outside dialog)
  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) close();
  });

  // Close on Escape
  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }
  document.addEventListener('keydown', onKeyDown);

  closeBtn.addEventListener('click', close);

  // Page switching
  const pages: Record<string, { title: string; btn: HTMLButtonElement; build: (ctx: SettingsPageContext) => PageResult }> = {
    appearance: { title: 'Appearance', btn: appearanceBtn, build: buildAppearancePage },
    editor: { title: 'Editor', btn: editorBtn, build: buildEditorPage },
    libraries: { title: 'Libraries', btn: librariesBtn, build: buildLibrariesPage },
    assistant: { title: 'Assistant', btn: assistantSettingsBtn, build: buildAssistantPage },
    slicer: { title: 'Slicer', btn: slicerSettingsBtn, build: buildSlicerPage },
    camera: { title: 'Camera', btn: cameraBtn, build: buildCameraPage },
    memory: { title: 'Memory', btn: memoryBtn, build: buildMemoryPage },
    debug: { title: 'Log', btn: debugSettingsBtn, build: buildDebugPage },
  };

  function showPage(name: string) {
    const page = pages[name];
    if (!page) return;
    if (pageCleanup) {
      pageCleanup();
      pageCleanup = null;
    }
    content.innerHTML = '';
    titleSpan.textContent = page.title;
    for (const p of Object.values(pages)) p.btn.classList.remove('active');
    page.btn.classList.add('active');
    const result = page.build(ctx);
    content.appendChild(result.el);
    pageCleanup = result.cleanup || null;
  }

  for (const [name, page] of Object.entries(pages)) {
    page.btn.addEventListener('click', () => showPage(name));
  }

  // Default to appearance
  showPage('appearance');

  return overlay;
}
