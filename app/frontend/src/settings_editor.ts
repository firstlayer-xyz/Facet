import type { SettingsPageContext, PageResult } from './settings_appearance';

export function buildEditorPage(ctx: SettingsPageContext): PageResult {
  const { draft, onSave } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  // Word wrap toggle
  const wrapRow = document.createElement('div');
  wrapRow.className = 'settings-checkbox-row';

  const checkbox = document.createElement('input');
  checkbox.type = 'checkbox';
  checkbox.id = 'settings-word-wrap';
  checkbox.checked = draft.editor.wordWrap;
  checkbox.addEventListener('change', () => {
    draft.editor.wordWrap = checkbox.checked;
  });

  const wrapLabel = document.createElement('label');
  wrapLabel.htmlFor = 'settings-word-wrap';
  wrapLabel.textContent = 'Word wrap';

  wrapRow.appendChild(checkbox);
  wrapRow.appendChild(wrapLabel);
  page.appendChild(wrapRow);

  // Format on save toggle
  const formatRow = document.createElement('div');
  formatRow.className = 'settings-checkbox-row';

  const formatCheckbox = document.createElement('input');
  formatCheckbox.type = 'checkbox';
  formatCheckbox.id = 'settings-format-on-save';
  formatCheckbox.checked = draft.editor.formatOnSave;
  formatCheckbox.addEventListener('change', () => {
    draft.editor.formatOnSave = formatCheckbox.checked;
  });

  const formatLabel = document.createElement('label');
  formatLabel.htmlFor = 'settings-format-on-save';
  formatLabel.textContent = 'Format on save';

  formatRow.appendChild(formatCheckbox);
  formatRow.appendChild(formatLabel);
  page.appendChild(formatRow);

  // Highlight mode segmented control
  const highlightRow = document.createElement('div');
  highlightRow.className = 'settings-color-row';

  const highlightLabel = document.createElement('label');
  highlightLabel.textContent = 'Highlight';

  const highlightGroup = document.createElement('div');
  highlightGroup.className = 'segmented-control';

  const highlightOptions: { value: 'mouse' | 'cursor' | 'off'; label: string }[] = [
    { value: 'mouse', label: 'Mouse' },
    { value: 'cursor', label: 'Cursor' },
    { value: 'off', label: 'Off' },
  ];

  for (const opt of highlightOptions) {
    const btn = document.createElement('button');
    btn.className = 'segmented-btn';
    btn.textContent = opt.label;
    if (draft.editor.highlight === opt.value) btn.classList.add('active');
    btn.addEventListener('click', () => {
      draft.editor.highlight = opt.value;
      highlightGroup.querySelectorAll('.segmented-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
    });
    highlightGroup.appendChild(btn);
  }

  highlightRow.appendChild(highlightLabel);
  highlightRow.appendChild(highlightGroup);
  page.appendChild(highlightRow);

  // --- Bed section ---
  const divider = document.createElement('hr');
  divider.className = 'settings-divider';
  page.appendChild(divider);

  // Bed plane selector
  const planeRow = document.createElement('div');
  planeRow.className = 'settings-color-row';

  const planeLabel = document.createElement('label');
  planeLabel.textContent = 'Bed';

  const planeSelect = document.createElement('select');
  for (const plane of ['XZ', 'XY', 'YZ']) {
    const opt = document.createElement('option');
    opt.value = plane;
    opt.textContent = plane;
    if (plane === draft.appearance.bed) opt.selected = true;
    planeSelect.appendChild(opt);
  }
  planeSelect.addEventListener('change', () => {
    draft.appearance.bed = planeSelect.value;
    onSave(structuredClone(draft));
  });

  planeRow.appendChild(planeLabel);
  planeRow.appendChild(planeSelect);
  page.appendChild(planeRow);

  // Bed size input
  const sizeRow = document.createElement('div');
  sizeRow.className = 'settings-color-row';

  const sizeLabel = document.createElement('label');
  sizeLabel.textContent = 'Bed Size (mm)';

  const sizeInput = document.createElement('input');
  sizeInput.type = 'number';
  sizeInput.min = '10';
  sizeInput.max = '2000';
  sizeInput.step = '1';
  sizeInput.value = String(draft.appearance.gridSize);
  sizeInput.addEventListener('change', () => {
    draft.appearance.gridSize = Math.max(10, Math.min(2000, parseInt(sizeInput.value, 10) || 250));
    onSave(structuredClone(draft));
  });

  sizeRow.appendChild(sizeLabel);
  sizeRow.appendChild(sizeInput);
  page.appendChild(sizeRow);

  // Bed spacing input
  const spacingRow = document.createElement('div');
  spacingRow.className = 'settings-color-row';

  const spacingLabel = document.createElement('label');
  spacingLabel.textContent = 'Grid Spacing (mm)';

  const spacingInput = document.createElement('input');
  spacingInput.type = 'number';
  spacingInput.min = '1';
  spacingInput.max = '100';
  spacingInput.step = '1';
  spacingInput.value = String(draft.appearance.gridSpacing);
  spacingInput.addEventListener('change', () => {
    draft.appearance.gridSpacing = Math.max(1, Math.min(100, parseInt(spacingInput.value, 10) || 10));
    onSave(structuredClone(draft));
  });

  spacingRow.appendChild(spacingLabel);
  spacingRow.appendChild(spacingInput);
  page.appendChild(spacingRow);

  return { el: page };
}
