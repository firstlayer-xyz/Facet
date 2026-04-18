import type { AppSettings } from './settings';
import { reportError } from './toast';

// ---------------------------------------------------------------------------
// Types shared by all settings pages
// ---------------------------------------------------------------------------

export interface SettingsPageContext {
  draft: AppSettings;
  onSave: (s: AppSettings) => void;
}

export interface PageResult {
  el: HTMLElement;
  cleanup?: () => void;
}

// ---------------------------------------------------------------------------
// Styling primitives
// ---------------------------------------------------------------------------

/** Apply standard settings button styling. */
export function styleButton(btn: HTMLButtonElement, variant: 'primary' | 'secondary' = 'secondary') {
  btn.style.padding = '4px 12px';
  btn.style.border = 'none';
  btn.style.borderRadius = '4px';
  btn.style.cursor = 'pointer';
  btn.style.fontSize = '13px';
  if (variant === 'primary') {
    btn.style.background = 'var(--ui-accent)';
    btn.style.color = '#fff';
  } else {
    btn.style.background = 'var(--ui-input-bg, #333)';
    btn.style.color = 'var(--ui-text)';
  }
}

/** Apply standard settings text input styling. */
export function styleInput(input: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement) {
  input.style.padding = '4px 8px';
  input.style.border = '1px solid var(--ui-border)';
  input.style.borderRadius = '4px';
  input.style.background = 'var(--ui-bg)';
  input.style.color = 'var(--ui-text)';
  input.style.fontSize = '13px';
}

// ---------------------------------------------------------------------------
// Layout primitives
// ---------------------------------------------------------------------------

/** Label-plus-control row used throughout settings pages. */
export function settingsRow(labelText: string, control: HTMLElement): HTMLDivElement {
  const row = document.createElement('div');
  row.className = 'settings-color-row';
  const label = document.createElement('label');
  label.textContent = labelText;
  row.appendChild(label);
  row.appendChild(control);
  return row;
}

/** Checkbox-plus-label row with proper `htmlFor` wiring. */
export function settingsCheckboxRow(
  id: string,
  labelText: string,
  checked: boolean,
  onChange: (v: boolean) => void,
): HTMLDivElement {
  const row = document.createElement('div');
  row.className = 'settings-checkbox-row';
  const checkbox = document.createElement('input');
  checkbox.type = 'checkbox';
  checkbox.id = id;
  checkbox.checked = checked;
  checkbox.addEventListener('change', () => onChange(checkbox.checked));
  const label = document.createElement('label');
  label.htmlFor = id;
  label.textContent = labelText;
  row.appendChild(checkbox);
  row.appendChild(label);
  return row;
}

/**
 * Help text below or above a control.
 * - `description`: 13px intro text shown at the top of a page (marginBottom 16px).
 * - `hint`:        11px trailing note shown below a control (marginTop 4px).
 */
export function settingsHelp(
  text: string,
  variant: 'description' | 'hint' = 'description',
): HTMLDivElement {
  const el = document.createElement('div');
  el.textContent = text;
  if (variant === 'description') {
    el.style.color = '#888';
    el.style.fontSize = '13px';
    el.style.marginBottom = '16px';
  } else {
    el.style.color = '#666';
    el.style.fontSize = '11px';
    el.style.marginTop = '4px';
  }
  return el;
}

/** Section header used to group related controls on a settings page. */
export function settingsSectionHeader(title: string): HTMLHeadingElement {
  const h = document.createElement('h3');
  h.textContent = title;
  h.style.margin = '12px 0 6px';
  return h;
}

/**
 * Inline message element used inside a list container (e.g. "Loading...",
 * "Failed to load libraries", "No items"). For transient failures that the
 * user should notice, use `reportError` instead.
 */
export function settingsMessage(text: string, tone: 'muted' | 'error' = 'muted'): HTMLDivElement {
  const el = document.createElement('div');
  el.textContent = text;
  el.style.color = tone === 'error' ? '#f85149' : '#888';
  el.style.fontSize = '13px';
  return el;
}

// ---------------------------------------------------------------------------
// Async helpers
// ---------------------------------------------------------------------------

/**
 * Run an async action behind a button with loading/done/error feedback.
 * Disables the button for the duration, restores its label after 2s, and
 * surfaces failures via `reportError(where, err)` rather than swallowing them.
 */
export async function asyncButton(
  btn: HTMLButtonElement,
  loadingText: string,
  where: string,
  action: () => Promise<void>,
): Promise<void> {
  const original = btn.textContent;
  btn.disabled = true;
  btn.textContent = loadingText;
  try {
    await action();
    btn.textContent = 'Done';
  } catch (err) {
    btn.textContent = 'Error';
    reportError(where, err);
  }
  setTimeout(() => {
    btn.textContent = original;
    btn.disabled = false;
  }, 2000);
}
