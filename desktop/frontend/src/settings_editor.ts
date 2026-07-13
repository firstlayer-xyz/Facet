import {
  settingsRow,
  settingsCheckboxRow,
  segmentedControl,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

/** Build a <select> from options, marking `current` selected and wiring change. */
function makeSelect(
  options: { value: string; label: string }[],
  current: string,
  onChange: (v: string) => void,
): HTMLSelectElement {
  const select = document.createElement('select');
  for (const opt of options) {
    const o = document.createElement('option');
    o.value = opt.value;
    o.textContent = opt.label;
    if (opt.value === current) o.selected = true;
    select.appendChild(o);
  }
  select.addEventListener('change', () => onChange(select.value));
  return select;
}

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
  const highlightOptions: { value: 'mouse' | 'cursor' | 'off'; label: string }[] = [
    { value: 'mouse', label: 'Mouse' },
    { value: 'cursor', label: 'Cursor' },
    { value: 'off', label: 'Off' },
  ];
  const highlightGroup = segmentedControl(highlightOptions, draft.editor.highlight, v => {
    draft.editor.highlight = v;
  });

  page.appendChild(settingsRow('Highlight', highlightGroup));

  // --- Bed section ---
  const divider = document.createElement('hr');
  divider.className = 'settings-divider';
  page.appendChild(divider);

  // Bed plane selector
  const planeSelect = makeSelect(
    ['XZ', 'XY', 'YZ'].map(p => ({ value: p, label: p })),
    draft.appearance.bed,
    v => {
      draft.appearance.bed = v;
      onSave(structuredClone(draft));
    },
  );
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
  const unitsOptions: { value: 'metric' | 'imperial'; label: string }[] = [
    { value: 'metric',   label: 'Metric (mm)' },
    { value: 'imperial', label: 'Imperial (inches)' },
  ];
  const unitsSelect = makeSelect(unitsOptions, draft.measurement.units, v => {
    draft.measurement.units = v as 'metric' | 'imperial';
    updateImperialControlsEnabled();
    onSave(structuredClone(draft));
  });
  page.appendChild(settingsRow('Measurement Units', unitsSelect));

  // Imperial display format: reduced fraction or decimal inches.
  const imperialFormatOptions: { value: 'fraction' | 'decimal'; label: string }[] = [
    { value: 'fraction', label: 'Fraction' },
    { value: 'decimal',  label: 'Decimal' },
  ];
  const imperialFormatSelect = makeSelect(imperialFormatOptions, draft.measurement.imperialFormat, v => {
    draft.measurement.imperialFormat = v as 'fraction' | 'decimal';
    updateImperialControlsEnabled();
    onSave(structuredClone(draft));
  });
  page.appendChild(settingsRow('Imperial Format', imperialFormatSelect));

  // Imperial fraction denominator: powers of 2 from 4 through 128.
  const denominatorOptions: (4 | 8 | 16 | 32 | 64 | 128)[] = [4, 8, 16, 32, 64, 128];
  const denominatorSelect = makeSelect(
    denominatorOptions.map(d => ({ value: String(d), label: `1/${d}"` })),
    String(draft.measurement.imperialDenominator),
    v => {
      draft.measurement.imperialDenominator = parseInt(v, 10) as 4 | 8 | 16 | 32 | 64 | 128;
      onSave(structuredClone(draft));
    },
  );
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
