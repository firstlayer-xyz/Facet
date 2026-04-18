import { DetectSlicers } from '../wailsjs/go/main/App';
import {
  settingsRow,
  settingsHelp,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

export function buildSlicerPage(ctx: SettingsPageContext): PageResult {
  const { draft } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  page.appendChild(settingsHelp(
    'Choose which slicer to use when sending models. Detected slicers are listed below.',
  ));

  // Default slicer dropdown
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

  page.appendChild(settingsRow('Default Slicer', select));

  page.appendChild(settingsHelp(
    'Supported: BambuStudio, OrcaSlicer, PrusaSlicer, UltiMaker Cura, AnycubicSlicer, Anycubic Photon Workshop. Install to /Applications on macOS.',
    'hint',
  ));

  return { el: page };
}
