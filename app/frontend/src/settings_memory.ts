import { GetMemoryLimit, MemStats, RunGC, SetMemoryLimit } from '../wailsjs/go/main/App';
import {
  styleButton,
  styleInput,
  settingsHelp,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';
import { reportError } from './toast';

export function buildMemoryPage(ctx: SettingsPageContext): PageResult {
  const page = document.createElement('div');
  page.className = 'settings-page';

  // --- Memory Stats ---
  const statsPre = document.createElement('pre');
  statsPre.style.background = 'var(--ui-bg-dark)';
  statsPre.style.color = 'var(--ui-text-dim)';
  statsPre.style.border = '1px solid var(--ui-border)';
  statsPre.style.borderRadius = '4px';
  statsPre.style.padding = '8px';
  statsPre.style.fontSize = '12px';
  statsPre.style.fontFamily = 'monospace';
  statsPre.style.whiteSpace = 'pre';
  statsPre.style.overflow = 'auto';
  statsPre.style.maxHeight = '300px';
  statsPre.style.margin = '0';
  statsPre.style.marginBottom = '8px';
  statsPre.textContent = 'Loading...';

  function fmtMB(bytes: number): string {
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  }

  async function refreshStats() {
    const lines: string[] = [];
    try {
      const go: Record<string, number> = await MemStats();
      lines.push('── Go Runtime ──');
      lines.push(`  Heap in use:   ${fmtMB(go.heapAlloc)}`);
      lines.push(`  Heap from OS:  ${fmtMB(go.heapSys)}`);
      lines.push(`  Heap idle:     ${fmtMB(go.heapIdle)}`);
      lines.push(`  Heap released: ${fmtMB(go.heapReleased)}`);
      lines.push(`  Total from OS: ${fmtMB(go.sys)}`);
      lines.push(`  External (C):  ${fmtMB(go.externalMemory)}`);
      lines.push(`  GC cycles:     ${go.numGC}`);
    } catch (err) {
      lines.push('Go stats: unavailable');
      reportError('MemStats', err);
    }

    const perf = (performance as any).memory;
    if (perf) {
      lines.push('');
      lines.push('── JS / WebView ──');
      lines.push(`  JS heap used:  ${fmtMB(perf.usedJSHeapSize)}`);
      lines.push(`  JS heap total: ${fmtMB(perf.totalJSHeapSize)}`);
      lines.push(`  JS heap limit: ${fmtMB(perf.jsHeapSizeLimit)}`);
    }

    statsPre.textContent = lines.join('\n');
  }

  refreshStats();
  const intervalId = setInterval(refreshStats, 1000);
  page.appendChild(statsPre);

  // Action buttons row
  const actionsRow = document.createElement('div');
  actionsRow.style.display = 'flex';
  actionsRow.style.gap = '8px';
  actionsRow.style.marginBottom = '20px';

  const refreshBtn = document.createElement('button');
  refreshBtn.textContent = 'Refresh';
  styleButton(refreshBtn);
  refreshBtn.addEventListener('click', refreshStats);

  const gcBtn = document.createElement('button');
  gcBtn.textContent = 'Run GC';
  styleButton(gcBtn);
  gcBtn.addEventListener('click', async () => {
    RunGC();
    gcBtn.textContent = 'Done';
    setTimeout(() => { gcBtn.textContent = 'Run GC'; }, 1000);
    setTimeout(refreshStats, 500);
  });

  actionsRow.appendChild(refreshBtn);
  actionsRow.appendChild(gcBtn);
  page.appendChild(actionsRow);

  // --- Memory limit ---
  const limitLabel = document.createElement('label');
  limitLabel.textContent = 'Memory Limit (GB)';
  limitLabel.style.display = 'block';
  limitLabel.style.marginBottom = '6px';
  limitLabel.style.color = 'var(--ui-text)';
  limitLabel.style.fontSize = '14px';
  page.appendChild(limitLabel);

  const limitRow = document.createElement('div');
  limitRow.style.display = 'flex';
  limitRow.style.gap = '8px';
  limitRow.style.alignItems = 'center';

  const memInput = document.createElement('input');
  memInput.type = 'number';
  memInput.min = '0';
  memInput.style.width = '100px';
  styleInput(memInput);
  memInput.style.background = 'var(--ui-bg-dark)';
  memInput.style.fontFamily = 'monospace';

  const setBtn = document.createElement('button');
  setBtn.textContent = 'Set';
  styleButton(setBtn);

  const defaultBtn = document.createElement('button');
  defaultBtn.textContent = 'No Limit';
  styleButton(defaultBtn);

  limitRow.appendChild(memInput);
  limitRow.appendChild(setBtn);
  limitRow.appendChild(defaultBtn);
  page.appendChild(limitRow);

  page.appendChild(settingsHelp(
    'Go runtime soft memory limit. 0 = default (8 GB). Persisted across restarts.',
    'hint',
  ));

  // Load current value from backend
  GetMemoryLimit().then((gb: number) => {
    memInput.value = String(gb);
  }).catch((err) => {
    memInput.value = '0';
    reportError('GetMemoryLimit', err);
  });

  setBtn.addEventListener('click', () => {
    const gb = parseInt(memInput.value, 10);
    if (!isNaN(gb) && gb >= 0) {
      SetMemoryLimit(gb);
    }
  });

  defaultBtn.addEventListener('click', () => {
    memInput.value = '0';
    SetMemoryLimit(0);
  });

  return { el: page, cleanup: () => clearInterval(intervalId) };
}
