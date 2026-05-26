import {
  settingsRow,
  settingsCheckboxRow,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

export function buildEditorPage(ctx: SettingsPageContext): PageResult {
  const { draft, onSave } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  page.appendChild(settingsCheckboxRow(
    'settings-word-wrap',
    'Word wrap',
    draft.editor.wordWrap,
    v => { draft.editor.wordWrap = v; },
  ));

  page.appendChild(settingsCheckboxRow(
    'settings-format-on-save',
    'Format on save',
    draft.editor.formatOnSave,
    v => { draft.editor.formatOnSave = v; },
  ));

  // Highlight mode segmented control
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

  page.appendChild(settingsRow('Highlight', highlightGroup));

  // --- Bed section ---
  const divider = document.createElement('hr');
  divider.className = 'settings-divider';
  page.appendChild(divider);

  // Bed plane selector
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
  page.appendChild(settingsRow('Bed', planeSelect));

  // Bed size input
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
  page.appendChild(settingsRow('Bed Size (mm)', sizeInput));

  // Bed spacing input
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
  page.appendChild(settingsRow('Grid Spacing (mm)', spacingInput));

  // --- Measurement section ---
  const measureDivider = document.createElement('hr');
  measureDivider.className = 'settings-divider';
  page.appendChild(measureDivider);

  // Units selector: metric (mm) or imperial (inches).
  const unitsSelect = document.createElement('select');
  const unitsOptions: { value: 'metric' | 'imperial'; label: string }[] = [
    { value: 'metric',   label: 'Metric (mm)' },
    { value: 'imperial', label: 'Imperial (inches)' },
  ];
  for (const opt of unitsOptions) {
    const o = document.createElement('option');
    o.value = opt.value;
    o.textContent = opt.label;
    if (opt.value === draft.measurement.units) o.selected = true;
    unitsSelect.appendChild(o);
  }
  unitsSelect.addEventListener('change', () => {
    draft.measurement.units = unitsSelect.value as 'metric' | 'imperial';
    updateImperialControlsEnabled();
    onSave(structuredClone(draft));
  });
  page.appendChild(settingsRow('Measurement Units', unitsSelect));

  // Imperial display format: reduced fraction or decimal inches.
  const imperialFormatSelect = document.createElement('select');
  const imperialFormatOptions: { value: 'fraction' | 'decimal'; label: string }[] = [
    { value: 'fraction', label: 'Fraction' },
    { value: 'decimal',  label: 'Decimal' },
  ];
  for (const opt of imperialFormatOptions) {
    const o = document.createElement('option');
    o.value = opt.value;
    o.textContent = opt.label;
    if (opt.value === draft.measurement.imperialFormat) o.selected = true;
    imperialFormatSelect.appendChild(o);
  }
  imperialFormatSelect.addEventListener('change', () => {
    draft.measurement.imperialFormat = imperialFormatSelect.value as 'fraction' | 'decimal';
    updateImperialControlsEnabled();
    onSave(structuredClone(draft));
  });
  page.appendChild(settingsRow('Imperial Format', imperialFormatSelect));

  // Imperial fraction denominator: powers of 2 from 4 through 128.
  const denominatorSelect = document.createElement('select');
  const denominatorOptions: (4 | 8 | 16 | 32 | 64 | 128)[] = [4, 8, 16, 32, 64, 128];
  for (const d of denominatorOptions) {
    const o = document.createElement('option');
    o.value = String(d);
    o.textContent = `1/${d}"`;
    if (d === draft.measurement.imperialDenominator) o.selected = true;
    denominatorSelect.appendChild(o);
  }
  denominatorSelect.addEventListener('change', () => {
    const v = parseInt(denominatorSelect.value, 10) as 4 | 8 | 16 | 32 | 64 | 128;
    draft.measurement.imperialDenominator = v;
    onSave(structuredClone(draft));
  });
  page.appendChild(settingsRow('Fraction Denominator', denominatorSelect));

  // Grey out imperial-specific controls when they don't apply — denominator
  // is only meaningful in imperial + fraction; format selector is only
  // meaningful in imperial.
  function updateImperialControlsEnabled() {
    const imperial = draft.measurement.units === 'imperial';
    imperialFormatSelect.disabled = !imperial;
    denominatorSelect.disabled = !imperial || draft.measurement.imperialFormat !== 'fraction';
  }
  updateImperialControlsEnabled();

  return { el: page };
}
