import type { SettingsPageContext, PageResult } from './settings_appearance';
import { DetectSlicers } from '../wailsjs/go/main/App';

export function buildSlicerPage(ctx: SettingsPageContext): PageResult {
  const { draft } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  const desc = document.createElement('div');
  desc.style.color = '#888';
  desc.style.fontSize = '13px';
  desc.style.marginBottom = '16px';
  desc.textContent = 'Choose which slicer to use when sending models. Detected slicers are listed below.';
  page.appendChild(desc);

  // Default slicer dropdown
  const row = document.createElement('div');
  row.className = 'settings-color-row';

  const label = document.createElement('label');
  label.textContent = 'Default Slicer';

  const select = document.createElement('select');
  select.style.minWidth = '200px';

  const autoOpt = document.createElement('option');
  autoOpt.value = '';
  autoOpt.textContent = 'Auto (first detected)';
  select.appendChild(autoOpt);

  // Populate from detected slicers
  DetectSlicers().then((slicers) => {
    if (!slicers || slicers.length === 0) {
      const noneOpt = document.createElement('option');
      noneOpt.textContent = 'No slicers detected';
      noneOpt.disabled = true;
      select.appendChild(noneOpt);
      return;
    }
    for (const s of slicers) {
      const opt = document.createElement('option');
      opt.value = s.id;
      opt.textContent = s.name;
      if (s.id === draft.slicer.defaultSlicer) opt.selected = true;
      select.appendChild(opt);
    }
    if (!draft.slicer.defaultSlicer) select.value = '';
  });

  select.addEventListener('change', () => {
    draft.slicer.defaultSlicer = select.value;
  });

  row.appendChild(label);
  row.appendChild(select);
  page.appendChild(row);

  const hint = document.createElement('div');
  hint.style.color = '#666';
  hint.style.fontSize = '11px';
  hint.style.marginTop = '4px';
  hint.textContent = 'Supported: BambuStudio, OrcaSlicer, PrusaSlicer, UltiMaker Cura, AnycubicSlicer, Anycubic Photon Workshop. Install to /Applications on macOS.';
  page.appendChild(hint);

  return { el: page };
}
