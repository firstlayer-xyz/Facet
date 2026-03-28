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
