import {
  ClearLibCache, ForkLibrary, GetLibraryDir, InstallLibrary, ListLibraries, ListLocalLibraries,
  ListLibraryFolders, PullAllLibraries, RevealInFileManager, UpdateLibrary,
} from '../wailsjs/go/main/App';
import {
  settingsCheckboxRow,
  settingsSectionHeader,
  settingsMessage,
  asyncButton,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';
import { reportError } from './toast';

interface LibraryInfo {
  id: string;
  name: string;
  ref: string;
  path: string;
}

export function buildLibrariesPage(ctx: SettingsPageContext): PageResult {
  const { draft } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  page.appendChild(settingsCheckboxRow(
    'settings-auto-pull',
    'Auto-pull on launch',
    draft.librarySettings.autoPull,
    v => { draft.librarySettings.autoPull = v; },
  ));

  // --- Local Libraries section ---
  page.appendChild(settingsSectionHeader('Local'));

  const localList = document.createElement('div');
  localList.id = 'settings-local-lib-list';
  page.appendChild(localList);

  // --- Cached Libraries section ---
  const cachedHeaderRow = document.createElement('div');
  cachedHeaderRow.style.display = 'flex';
  cachedHeaderRow.style.alignItems = 'center';
  cachedHeaderRow.style.gap = '8px';
  cachedHeaderRow.style.margin = '12px 0 6px';

  const cachedHeader = document.createElement('h3');
  cachedHeader.textContent = 'Cached';
  cachedHeader.style.margin = '0';
  cachedHeaderRow.appendChild(cachedHeader);

  const pullAllBtn = document.createElement('button');
  pullAllBtn.className = 'settings-module-remove';
  pullAllBtn.textContent = 'Pull All';
  pullAllBtn.addEventListener('click', () => asyncButton(pullAllBtn, 'Pulling...', 'PullAllLibraries', async () => {
    await PullAllLibraries();
    loadAndRender();
  }));
  cachedHeaderRow.appendChild(pullAllBtn);

  const clearCacheBtn = document.createElement('button');
  clearCacheBtn.className = 'settings-module-remove';
  clearCacheBtn.textContent = 'Clear Cache';
  clearCacheBtn.addEventListener('click', () => asyncButton(clearCacheBtn, 'Clearing...', 'ClearLibCache', async () => {
    await ClearLibCache();
    loadAndRender();
  }));
  cachedHeaderRow.appendChild(clearCacheBtn);

  page.appendChild(cachedHeaderRow);

  const cachedList = document.createElement('div');
  cachedList.id = 'settings-module-list';
  page.appendChild(cachedList);

  let loadAndRenderPending = false;
  async function loadAndRender() {
    if (loadAndRenderPending) return;
    loadAndRenderPending = true;
    try { await loadAndRenderInner(); } finally { loadAndRenderPending = false; }
  }
  async function loadAndRenderInner() {
    // Render local libraries grouped by folder
    localList.replaceChildren(settingsMessage('Loading...'));
    try {
      const [folders, locals]: [string[], LibraryInfo[]] = await Promise.all([
        ListLibraryFolders(),
        ListLocalLibraries(),
      ]);
      localList.innerHTML = '';

      // Group libraries by folder (first segment of id)
      const byFolder = new Map<string, LibraryInfo[]>();
      for (const f of (folders || [])) byFolder.set(f, []);
      for (const lib of (locals || [])) {
        const folder = lib.id.split('/')[0];
        if (!byFolder.has(folder)) byFolder.set(folder, []);
        byFolder.get(folder)!.push(lib);
      }

      if (byFolder.size === 0) {
        localList.replaceChildren(settingsMessage('No local libraries'));
      } else {
        for (const [folder, libs] of byFolder) {
          const folderEl = document.createElement('div');
          folderEl.style.marginBottom = '8px';

          const folderHeader = document.createElement('div');
          folderHeader.style.cssText = 'display:flex;align-items:center;gap:6px;margin-bottom:2px';
          const folderName = document.createElement('span');
          folderName.style.cssText = 'font-weight:600;color:var(--ui-text-secondary,#aaa)';
          folderName.textContent = folder + '/';
          folderHeader.appendChild(folderName);
          const folderRevealBtn = document.createElement('button');
          folderRevealBtn.className = 'settings-module-remove';
          folderRevealBtn.textContent = 'Reveal';
          folderRevealBtn.title = 'Open folder in file manager';
          folderRevealBtn.addEventListener('click', async () => {
            const dir = await GetLibraryDir();
            RevealInFileManager(dir + '/' + folder);
          });
          folderHeader.appendChild(folderRevealBtn);
          folderEl.appendChild(folderHeader);

          if (libs.length === 0) {
            const empty = document.createElement('div');
            empty.style.cssText = 'color:#666;font-size:12px;padding-left:12px';
            empty.textContent = '(empty)';
            folderEl.appendChild(empty);
          }
          for (const lib of libs) {
            const row = document.createElement('div');
            row.className = 'settings-module-row';
            row.style.paddingLeft = '12px';

            const info = document.createElement('span');
            info.className = 'settings-module-url';
            info.textContent = lib.id.split('/').slice(1).join('/');

            const actions = document.createElement('span');
            const revealBtn = document.createElement('button');
            revealBtn.className = 'settings-module-remove';
            revealBtn.textContent = 'Show';
            revealBtn.title = 'Reveal in file manager';
            revealBtn.addEventListener('click', () => RevealInFileManager(lib.path));
            actions.appendChild(revealBtn);

            row.appendChild(info);
            row.appendChild(actions);
            folderEl.appendChild(row);
          }
          localList.appendChild(folderEl);
        }
      }
    } catch (err) {
      localList.replaceChildren(settingsMessage('Failed to load local libraries'));
      reportError('ListLocalLibraries', err);
    }

    // Render cached libraries
    cachedList.replaceChildren(settingsMessage('Loading...'));
    try {
      const libs: LibraryInfo[] = await ListLibraries();
      cachedList.innerHTML = '';
      if (!libs || libs.length === 0) {
        cachedList.replaceChildren(settingsMessage('No cached libraries'));
      } else {
        for (const lib of libs) {
          const row = document.createElement('div');
          row.className = 'settings-module-row';

          const info = document.createElement('span');
          info.className = 'settings-module-url';
          info.textContent = `${lib.name || lib.id} @ ${lib.ref}`;

          const actions = document.createElement('span');

          const revealBtn = document.createElement('button');
          revealBtn.className = 'settings-module-remove';
          revealBtn.textContent = 'Show';
          revealBtn.title = 'Reveal in file manager';
          revealBtn.addEventListener('click', () => {
            RevealInFileManager(lib.path);
          });

          const updateBtn = document.createElement('button');
          updateBtn.className = 'settings-module-remove';
          updateBtn.textContent = 'Update';
          updateBtn.addEventListener('click', () => asyncButton(updateBtn, '...', 'UpdateLibrary', async () => {
            await UpdateLibrary(lib.id, lib.ref);
          }));

          const forkBtn = document.createElement('button');
          forkBtn.className = 'settings-module-remove';
          forkBtn.textContent = 'Fork';
          forkBtn.title = 'Copy to local libraries for editing';
          forkBtn.addEventListener('click', () => asyncButton(forkBtn, '...', 'ForkLibrary', async () => {
            await ForkLibrary(lib.id, lib.ref);
            loadAndRender();
          }));

          actions.appendChild(revealBtn);
          actions.appendChild(updateBtn);
          actions.appendChild(forkBtn);
          row.appendChild(info);
          row.appendChild(actions);
          cachedList.appendChild(row);
        }
      }
    } catch (err) {
      cachedList.replaceChildren(settingsMessage('Failed to load cached libraries'));
      reportError('ListLibraries', err);
    }
  }

  loadAndRender();

  // Clone form
  const addRow = document.createElement('div');
  addRow.className = 'settings-module-add';

  const urlInput = document.createElement('input');
  urlInput.type = 'text';
  urlInput.placeholder = 'github.com/user/repo';
  urlInput.style.flex = '1';

  const refInput = document.createElement('input');
  refInput.type = 'text';
  refInput.placeholder = 'main';
  refInput.style.width = '80px';
  refInput.value = 'main';

  const addBtn = document.createElement('button');
  addBtn.textContent = 'Clone';
  addBtn.addEventListener('click', async () => {
    const url = urlInput.value.trim();
    const ref = refInput.value.trim() || 'main';
    if (!url) return;
    addBtn.disabled = true;
    addBtn.textContent = 'Cloning...';
    try {
      await InstallLibrary(url, ref);
      urlInput.value = '';
      addBtn.textContent = 'Clone';
      addBtn.disabled = false;
      loadAndRender();
    } catch (err) {
      reportError('InstallLibrary', err);
      addBtn.textContent = 'Error';
      addBtn.disabled = false;
    }
  });

  addRow.appendChild(urlInput);
  addRow.appendChild(refInput);
  addRow.appendChild(addBtn);
  page.appendChild(addRow);

  return { el: page };
}
