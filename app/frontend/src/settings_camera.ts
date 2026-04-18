import {
  settingsRow,
  settingsHelp,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

export function buildCameraPage(ctx: SettingsPageContext): PageResult {
  const { draft } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  page.appendChild(settingsHelp('Select the camera used for head-tracking parallax.'));

  const select = document.createElement('select');
  select.id = 'settings-camera-device';
  select.style.minWidth = '200px';

  // Default option
  const defaultOpt = document.createElement('option');
  defaultOpt.value = '';
  defaultOpt.textContent = 'Default';
  select.appendChild(defaultOpt);

  // Enumerate video devices
  navigator.mediaDevices.enumerateDevices().then((devices) => {
    const videoDevices = devices.filter(d => d.kind === 'videoinput');
    for (const device of videoDevices) {
      const opt = document.createElement('option');
      opt.value = device.deviceId;
      opt.textContent = device.label || `Camera ${select.options.length}`;
      if (device.deviceId === draft.camera.deviceId) opt.selected = true;
      select.appendChild(opt);
    }
    // If saved device not found, keep "Default" selected
    if (!draft.camera.deviceId) {
      select.value = '';
    }
  }).catch(() => {
    const errOpt = document.createElement('option');
    errOpt.textContent = 'Failed to list devices';
    errOpt.disabled = true;
    select.appendChild(errOpt);
  });

  select.addEventListener('change', () => {
    draft.camera.deviceId = select.value;
  });

  page.appendChild(settingsRow('Device', select));

  // Y-offset slider
  const offsetWrap = document.createElement('div');
  offsetWrap.style.display = 'flex';
  offsetWrap.style.alignItems = 'center';
  offsetWrap.style.gap = '8px';

  const offsetSlider = document.createElement('input');
  offsetSlider.type = 'range';
  offsetSlider.min = '0';
  offsetSlider.max = '0.6';
  offsetSlider.step = '0.05';
  offsetSlider.value = String(draft.camera.yOffset);
  offsetSlider.style.width = '120px';

  const offsetValue = document.createElement('span');
  offsetValue.style.color = '#888';
  offsetValue.style.fontSize = '12px';
  offsetValue.style.fontFamily = 'monospace';
  offsetValue.style.minWidth = '32px';
  offsetValue.textContent = draft.camera.yOffset.toFixed(2);

  offsetSlider.addEventListener('input', () => {
    draft.camera.yOffset = parseFloat(offsetSlider.value);
    offsetValue.textContent = draft.camera.yOffset.toFixed(2);
  });

  offsetWrap.appendChild(offsetSlider);
  offsetWrap.appendChild(offsetValue);
  page.appendChild(settingsRow('Vertical Offset', offsetWrap));

  page.appendChild(settingsHelp(
    'Shifts the neutral point down to compensate for webcam position above the screen.',
    'hint',
  ));

  return { el: page };
}
