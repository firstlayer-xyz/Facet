// dialogs.ts — Standalone UI dialogs (window.prompt doesn't work in WKWebView).

/** Show a dialog for creating a new library: pick/create folder + name. */
export function promptNewLibrary(folders: string[]): Promise<{folder: string; name: string; isNewFolder: boolean} | null> {
  return new Promise(resolve => {
    const inputCSS = 'width:100%;box-sizing:border-box;padding:6px 8px;border:1px solid var(--ui-border,#444);border-radius:4px;background:var(--ui-bg-alt,#1a1a1a);color:var(--ui-text,#eee);font-size:13px;outline:none';
    const btnCSS = 'padding:4px 14px;border:none;border-radius:4px;cursor:pointer;font-size:13px';

    const overlay = document.createElement('div');
    overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.4);display:flex;align-items:center;justify-content:center;z-index:9999';

    const box = document.createElement('div');
    box.style.cssText = 'background:var(--ui-bg,#222);border:1px solid var(--ui-border,#444);border-radius:8px;padding:16px;min-width:300px;color:var(--ui-text,#eee);font-family:system-ui,sans-serif;font-size:13px';

    const title = document.createElement('div');
    title.textContent = 'New Library';
    title.style.cssText = 'font-weight:600;font-size:14px;margin-bottom:12px';
    box.appendChild(title);

    // Folder row
    const folderLabel = document.createElement('div');
    folderLabel.textContent = 'Folder';
    folderLabel.style.marginBottom = '4px';
    box.appendChild(folderLabel);

    const folderRow = document.createElement('div');
    folderRow.style.cssText = 'display:flex;gap:6px;margin-bottom:10px';

    const sel = document.createElement('select');
    sel.style.cssText = inputCSS + ';flex:1';
    for (const f of folders) {
      const o = document.createElement('option');
      o.value = f; o.textContent = f;
      sel.appendChild(o);
    }
    const newOpt = document.createElement('option');
    newOpt.value = '__new__';
    newOpt.textContent = '+ New Folder...';
    sel.appendChild(newOpt);

    const newFolderInput = document.createElement('input');
    newFolderInput.type = 'text';
    newFolderInput.placeholder = 'folder name';
    newFolderInput.style.cssText = inputCSS + ';flex:1;display:none';

    sel.addEventListener('change', () => {
      if (sel.value === '__new__') {
        sel.style.display = 'none';
        newFolderInput.style.display = '';
        newFolderInput.focus();
      }
    });

    folderRow.append(sel, newFolderInput);
    box.appendChild(folderRow);

    // Name row
    const nameLabel = document.createElement('div');
    nameLabel.textContent = 'Library Name';
    nameLabel.style.marginBottom = '4px';
    box.appendChild(nameLabel);

    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.placeholder = 'my-library';
    nameInput.style.cssText = inputCSS + ';margin-bottom:12px';
    box.appendChild(nameInput);

    // Buttons
    const btns = document.createElement('div');
    btns.style.cssText = 'display:flex;gap:8px;justify-content:flex-end';

    const cancel = document.createElement('button');
    cancel.textContent = 'Cancel';
    cancel.style.cssText = btnCSS + ';background:var(--ui-border,#444);color:var(--ui-text,#eee)';

    const ok = document.createElement('button');
    ok.textContent = 'Create';
    ok.style.cssText = btnCSS + ';background:var(--ui-accent,#4a9eff);color:#fff';

    function done(result: {folder: string; name: string; isNewFolder: boolean} | null) {
      overlay.remove(); resolve(result);
    }
    function stripFct(s: string): string {
      return s.endsWith('.fct') ? s.slice(0, -4) : s;
    }
    function submit() {
      let rawName = stripFct(nameInput.value.trim());
      let folder: string;
      let name: string;
      let isNew: boolean;
      if (rawName.includes('/')) {
        const idx = rawName.indexOf('/');
        folder = rawName.slice(0, idx).trim();
        name = rawName.slice(idx + 1).trim();
        isNew = !folders.includes(folder);
      } else {
        isNew = sel.value === '__new__' || sel.style.display === 'none';
        folder = isNew ? stripFct(newFolderInput.value.trim()) : sel.value;
        name = rawName;
      }
      if (!folder || !name) return;
      done({ folder, name, isNewFolder: isNew });
    }

    cancel.addEventListener('click', () => done(null));
    ok.addEventListener('click', submit);
    nameInput.addEventListener('keydown', e => {
      if (e.key === 'Enter') submit();
      if (e.key === 'Escape') done(null);
    });
    newFolderInput.addEventListener('keydown', e => {
      if (e.key === 'Enter') nameInput.focus();
      if (e.key === 'Escape') done(null);
    });
    overlay.addEventListener('click', e => { if (e.target === overlay) done(null); });

    btns.append(cancel, ok);
    box.appendChild(btns);
    overlay.appendChild(box);
    document.body.appendChild(overlay);

    if (folders.length === 0) {
      sel.style.display = 'none';
      newFolderInput.style.display = '';
      newFolderInput.focus();
    } else {
      nameInput.focus();
    }
  });
}

/** Show a slicer picker dropdown. Returns the selected slicer ID or null. */
export function showSlicerPicker(
  slicers: { id: string; name: string }[],
  anchorEl: HTMLElement,
): Promise<string | null> {
  return new Promise(resolve => {
    // Close existing dropdown if open
    const existing = document.getElementById('slicer-dropdown');
    if (existing) { existing.remove(); resolve(null); return; }

    const dropdown = document.createElement('div');
    dropdown.id = 'slicer-dropdown';

    for (const slicer of slicers) {
      const item = document.createElement('button');
      item.className = 'slicer-item';
      item.textContent = slicer.name;
      item.addEventListener('click', () => {
        dropdown.remove();
        document.removeEventListener('click', closeHandler);
        resolve(slicer.id);
      });
      dropdown.appendChild(item);
    }

    const rect = anchorEl.getBoundingClientRect();
    document.body.appendChild(dropdown);
    const menuH = dropdown.offsetHeight;
    const top = Math.min(rect.top, window.innerHeight - menuH - 8);
    dropdown.style.left = (rect.right + 4) + 'px';
    dropdown.style.top = Math.max(8, top) + 'px';

    const closeHandler = (e: MouseEvent) => {
      if (!dropdown.contains(e.target as Node) && e.target !== anchorEl) {
        dropdown.remove();
        document.removeEventListener('click', closeHandler);
        resolve(null);
      }
    };
    setTimeout(() => document.addEventListener('click', closeHandler), 0);
  });
}
