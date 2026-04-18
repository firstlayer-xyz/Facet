/**
 * Transient notification toasts. Errors appear in the bottom-right corner and
 * auto-dismiss after a few seconds; clicking a toast dismisses it immediately.
 *
 * Use `reportError(where, err)` at catch sites that would otherwise silently
 * swallow failures — it logs to the console (preserving the stack) AND shows
 * the user a visible message.
 */

const CONTAINER_ID = 'toast-container';
const ERROR_DISMISS_MS = 5000;
const INFO_DISMISS_MS = 3000;

function ensureContainer(): HTMLDivElement {
  let el = document.getElementById(CONTAINER_ID) as HTMLDivElement | null;
  if (el) return el;
  el = document.createElement('div');
  el.id = CONTAINER_ID;
  el.style.position = 'fixed';
  el.style.bottom = '16px';
  el.style.right = '16px';
  el.style.zIndex = '10000';
  el.style.display = 'flex';
  el.style.flexDirection = 'column';
  el.style.gap = '8px';
  el.style.pointerEvents = 'none';
  document.body.appendChild(el);
  return el;
}

function showToast(text: string, variant: 'error' | 'info'): void {
  const container = ensureContainer();
  const toast = document.createElement('div');
  toast.className = `toast toast-${variant}`;
  toast.textContent = text;
  toast.style.pointerEvents = 'auto';
  toast.style.cursor = 'pointer';
  toast.style.padding = '8px 12px';
  toast.style.borderRadius = '4px';
  toast.style.fontSize = '13px';
  toast.style.maxWidth = '420px';
  toast.style.boxShadow = '0 2px 8px rgba(0, 0, 0, 0.35)';
  if (variant === 'error') {
    toast.style.background = 'var(--ui-error-bg, #5a1a1a)';
    toast.style.color = 'var(--ui-error-fg, #f85149)';
    toast.style.border = '1px solid #f85149';
  } else {
    toast.style.background = 'var(--ui-bg-dark, #222)';
    toast.style.color = 'var(--ui-text, #ccc)';
    toast.style.border = '1px solid var(--ui-border, #444)';
  }

  const dismiss = () => toast.remove();
  toast.addEventListener('click', dismiss);
  const delay = variant === 'error' ? ERROR_DISMISS_MS : INFO_DISMISS_MS;
  setTimeout(dismiss, delay);

  container.appendChild(toast);
}

/**
 * Report an error to the user and the console. Use at catch sites that
 * would otherwise silently swallow failures. `where` is a short label like
 * "AddRecentFile" or "load libraries" — it prefixes the toast and console
 * message so the user can tell which operation failed.
 */
export function reportError(where: string, err: unknown): void {
  const msg = err instanceof Error ? err.message : String(err);
  console.error(`[${where}]`, err);
  showToast(`${where}: ${msg}`, 'error');
}

/** Show a transient info toast (no console log). */
export function reportInfo(text: string): void {
  showToast(text, 'info');
}
