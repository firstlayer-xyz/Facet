import type { CustomTheme } from './themes';
import { PALETTE_FIELDS, UI_THEMES, resolveThemePalette, resolveUiTheme, getBaseThemeId } from './themes';
import {
  styleButton,
  styleInput,
  settingsRow,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

export function buildAppearancePage(ctx: SettingsPageContext): PageResult {
  const { draft, onSave } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  // Dark mode 3-way toggle
  const dmGroup = document.createElement('div');
  dmGroup.className = 'segmented-control';

  const dmOptions: { value: 'light' | 'auto' | 'dark'; label: string }[] = [
    { value: 'light', label: 'Light' },
    { value: 'auto', label: 'Auto' },
    { value: 'dark', label: 'Dark' },
  ];

  for (const opt of dmOptions) {
    const btn = document.createElement('button');
    btn.className = 'segmented-btn';
    btn.textContent = opt.label;
    if (draft.appearance.darkMode === opt.value) btn.classList.add('active');
    btn.addEventListener('click', () => {
      draft.appearance.darkMode = opt.value;
      dmGroup.querySelectorAll('.segmented-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      onSave(structuredClone(draft));
      rebuildPaletteEditors();
      updateDeleteBtn();
    });
    dmGroup.appendChild(btn);
  }

  page.appendChild(settingsRow('Appearance', dmGroup));

  /** Resolve the UI theme ID accounting for darkMode switch. */
  function currentTheme(): string {
    return resolveUiTheme(draft.appearance.uiTheme, draft.appearance.darkMode);
  }

  // Theme selector (Facet + custom themes only)
  const themeSelect = document.createElement('select');
  themeSelect.id = 'settings-theme';

  function populateThemeSelect() {
    themeSelect.innerHTML = '';
    // Custom themes first
    for (const ct of draft.appearance.customThemes) {
      const opt = document.createElement('option');
      opt.value = ct.id;
      opt.textContent = ct.label;
      if (ct.id === draft.appearance.uiTheme) opt.selected = true;
      themeSelect.appendChild(opt);
    }
    if (draft.appearance.customThemes.length > 0) {
      const sep = document.createElement('option');
      sep.disabled = true;
      sep.textContent = '────────────';
      themeSelect.appendChild(sep);
    }
    // Built-in Facet options (light/dark follows the switch)
    const facetVariants = [
      { id: 'facet-orange', label: 'Facet - Orange' },
      { id: 'facet-green', label: 'Facet - Green' },
      { id: 'facet-digital-blue', label: 'Facet - Digital Blue' },
    ];
    for (const v of facetVariants) {
      const opt = document.createElement('option');
      opt.value = v.id;
      opt.textContent = v.label;
      if (draft.appearance.uiTheme === v.id) opt.selected = true;
      themeSelect.appendChild(opt);
    }
    // Other built-in themes
    for (const t of UI_THEMES) {
      const opt = document.createElement('option');
      opt.value = t.id;
      opt.textContent = t.label;
      if (t.id === draft.appearance.uiTheme) opt.selected = true;
      themeSelect.appendChild(opt);
    }
  }
  populateThemeSelect();

  themeSelect.addEventListener('change', () => {
    draft.appearance.uiTheme = themeSelect.value;
    // Switching to any built-in theme clears overrides
    const isCustom = draft.appearance.customThemes.some(t => t.id === themeSelect.value);
    if (!isCustom) {
      draft.appearance.themeOverrides = {};
    }
    onSave(structuredClone(draft));
    rebuildPaletteEditors();
    updateDeleteBtn();
  });

  page.appendChild(settingsRow('Theme', themeSelect));

  // Palette editors container
  const paletteContainer = document.createElement('div');
  page.appendChild(paletteContainer);

  // Action buttons row (below palette)
  const actionsRow = document.createElement('div');
  actionsRow.style.display = 'flex';
  actionsRow.style.gap = '8px';
  actionsRow.style.marginTop = '12px';
  actionsRow.style.paddingTop = '8px';
  actionsRow.style.borderTop = '1px solid var(--ui-border)';

  const saveThemeBtn = document.createElement('button');
  saveThemeBtn.textContent = 'Save as Theme';
  styleButton(saveThemeBtn);
  saveThemeBtn.addEventListener('click', () => {
    // Show inline name input (window.prompt() doesn't work in WKWebView)
    if (actionsRow.querySelector('.theme-name-input')) return; // already open
    const inputWrap = document.createElement('div');
    inputWrap.className = 'theme-name-input';
    inputWrap.style.display = 'flex';
    inputWrap.style.gap = '4px';
    inputWrap.style.marginTop = '8px';

    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.placeholder = 'Theme name';
    nameInput.style.flex = '1';
    styleInput(nameInput);
    nameInput.style.outline = 'none';

    const okBtn = document.createElement('button');
    okBtn.textContent = 'OK';
    styleButton(okBtn, 'primary');

    const cancelBtn = document.createElement('button');
    cancelBtn.textContent = 'Cancel';
    styleButton(cancelBtn);

    function commit() {
      const name = nameInput.value.trim();
      if (!name) return;
      inputWrap.remove();

      const effective = currentTheme();
      const resolved = resolveThemePalette(effective, draft.appearance.themeOverrides, draft.appearance.customThemes);
      const baseId = getBaseThemeId(effective, draft.appearance.customThemes);
      const paletteSnapshot: Record<string, string | number> = {};
      for (const field of PALETTE_FIELDS) {
        paletteSnapshot[field.key] = resolved[field.key] as string | number;
      }
      const custom: CustomTheme = {
        id: 'custom-' + Date.now(),
        label: name,
        base: baseId,
        palette: paletteSnapshot,
      };
      draft.appearance.customThemes.push(custom);
      draft.appearance.uiTheme = custom.id;
      draft.appearance.themeOverrides = {};
      populateThemeSelect();
      themeSelect.value = custom.id;
      onSave(structuredClone(draft));
      rebuildPaletteEditors();
      updateDeleteBtn();
    }

    okBtn.addEventListener('click', commit);
    cancelBtn.addEventListener('click', () => inputWrap.remove());
    nameInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') commit();
      if (e.key === 'Escape') inputWrap.remove();
    });

    inputWrap.appendChild(nameInput);
    inputWrap.appendChild(okBtn);
    inputWrap.appendChild(cancelBtn);
    actionsRow.after(inputWrap);
    nameInput.focus();
  });

  const deleteThemeBtn = document.createElement('button');
  deleteThemeBtn.textContent = 'Delete';
  styleButton(deleteThemeBtn);
  deleteThemeBtn.style.background = 'var(--ui-error-bg)';
  deleteThemeBtn.style.color = '#f85149';
  deleteThemeBtn.addEventListener('click', () => {
    const idx = draft.appearance.customThemes.findIndex((t: CustomTheme) => t.id === currentTheme());
    if (idx < 0) return;
    draft.appearance.customThemes.splice(idx, 1);
    draft.appearance.uiTheme = 'facet-orange';
    draft.appearance.themeOverrides = {};
    populateThemeSelect();
    themeSelect.value = 'facet-orange';
    onSave(structuredClone(draft));
    rebuildPaletteEditors();
    updateDeleteBtn();
  });

  const resetBtn = document.createElement('button');
  resetBtn.textContent = 'Reset Overrides';
  styleButton(resetBtn);
  resetBtn.addEventListener('click', () => {
    draft.appearance.themeOverrides = {};
    onSave(structuredClone(draft));
    rebuildPaletteEditors();
  });

  function updateDeleteBtn() {
    const isCustom = draft.appearance.customThemes.some((t: CustomTheme) => t.id === currentTheme());
    deleteThemeBtn.style.display = isCustom ? '' : 'none';
    resetBtn.style.display = isCustom ? 'none' : '';
  }
  updateDeleteBtn();

  actionsRow.appendChild(saveThemeBtn);
  actionsRow.appendChild(deleteThemeBtn);
  actionsRow.appendChild(resetBtn);
  page.appendChild(actionsRow);

  function rebuildPaletteEditors() {
    paletteContainer.innerHTML = '';
    const effective = currentTheme();
    const resolved = resolveThemePalette(effective, draft.appearance.themeOverrides, draft.appearance.customThemes);
    const isCustom = draft.appearance.customThemes.some((t: CustomTheme) => t.id === effective);

    let grid: HTMLDivElement | null = null;
    let currentSection = '';

    for (const field of PALETTE_FIELDS) {
      if (field.section !== currentSection) {
        currentSection = field.section;
        const sectionHeader = document.createElement('div');
        sectionHeader.className = 'palette-section-header';
        sectionHeader.textContent = field.section;
        paletteContainer.appendChild(sectionHeader);
        grid = document.createElement('div');
        grid.className = 'palette-grid';
        paletteContainer.appendChild(grid);
      }

      const cell = document.createElement('div');
      cell.className = 'palette-cell';

      const label = document.createElement('label');
      label.textContent = field.label;
      cell.appendChild(label);

      const value = resolved[field.key];

      if (field.type === 'color') {
        const input = document.createElement('input');
        input.type = 'color';
        input.value = String(value);
        input.addEventListener('input', () => {
          if (isCustom) {
            const ct = draft.appearance.customThemes.find((t: CustomTheme) => t.id === effective);
            if (ct) ct.palette[field.key] = input.value;
          } else {
            draft.appearance.themeOverrides[field.key] = input.value;
          }
          onSave(structuredClone(draft));
        });
        cell.appendChild(input);
      } else if (field.type === 'color-alpha') {
        const rgbaStr = String(value);
        const rgbaMatch = rgbaStr.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+))?\s*\)/);
        let hex = '#000000';
        let alpha = 1;
        if (rgbaMatch) {
          const r = parseInt(rgbaMatch[1]), g = parseInt(rgbaMatch[2]), b = parseInt(rgbaMatch[3]);
          hex = '#' + [r, g, b].map(c => c.toString(16).padStart(2, '0')).join('');
          alpha = rgbaMatch[4] !== undefined ? parseFloat(rgbaMatch[4]) : 1;
        }

        const wrap = document.createElement('div');
        wrap.style.display = 'flex';
        wrap.style.alignItems = 'center';
        wrap.style.gap = '4px';

        const colorInput = document.createElement('input');
        colorInput.type = 'color';
        colorInput.value = hex;

        const alphaInput = document.createElement('input');
        alphaInput.type = 'number';
        alphaInput.min = '0';
        alphaInput.max = '1';
        alphaInput.step = '0.05';
        alphaInput.value = String(alpha);
        alphaInput.className = 'palette-number-input';

        function updateColorAlpha() {
          const h = colorInput.value;
          const r = parseInt(h.slice(1, 3), 16);
          const g = parseInt(h.slice(3, 5), 16);
          const b = parseInt(h.slice(5, 7), 16);
          const a = parseFloat(alphaInput.value);
          const rgba = `rgba(${r}, ${g}, ${b}, ${a})`;
          if (isCustom) {
            const ct = draft.appearance.customThemes.find((t: CustomTheme) => t.id === effective);
            if (ct) ct.palette[field.key] = rgba;
          } else {
            draft.appearance.themeOverrides[field.key] = rgba;
          }
          onSave(structuredClone(draft));
        }

        colorInput.addEventListener('input', updateColorAlpha);
        alphaInput.addEventListener('change', updateColorAlpha);

        wrap.appendChild(colorInput);
        wrap.appendChild(alphaInput);
        cell.appendChild(wrap);
      } else {
        const input = document.createElement('input');
        input.type = 'number';
        input.value = String(value);
        input.step = String(field.step ?? 0.1);
        if (field.min !== undefined) input.min = String(field.min);
        if (field.max !== undefined) input.max = String(field.max);
        input.className = 'palette-number-input';
        input.addEventListener('change', () => {
          const num = parseFloat(input.value);
          if (isNaN(num)) return;
          if (isCustom) {
            const ct = draft.appearance.customThemes.find((t: CustomTheme) => t.id === effective);
            if (ct) ct.palette[field.key] = num;
          } else {
            draft.appearance.themeOverrides[field.key] = num;
          }
          onSave(structuredClone(draft));
        });
        cell.appendChild(input);
      }

      grid!.appendChild(cell);
    }
  }
  rebuildPaletteEditors();

  return { el: page };
}
