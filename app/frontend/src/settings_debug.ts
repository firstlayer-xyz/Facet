import type { PageResult } from './settings_appearance';
import { GetLogDir, GetStderrLog, RevealInFileManager } from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

export function buildDebugPage(): PageResult {
  const page = document.createElement('div');
  page.className = 'settings-page';
  page.style.display = 'flex';
  page.style.flexDirection = 'column';
  page.style.height = '100%';

  // Header row with Show button
  const headerRow = document.createElement('div');
  headerRow.style.display = 'flex';
  headerRow.style.alignItems = 'center';
  headerRow.style.gap = '8px';
  headerRow.style.marginBottom = '8px';
  headerRow.style.flexShrink = '0';

  const showBtn = document.createElement('button');
  showBtn.textContent = 'Show';
  showBtn.title = 'Open logs folder';
  showBtn.style.padding = '4px 12px';
  showBtn.style.border = 'none';
  showBtn.style.borderRadius = '4px';
  showBtn.style.background = 'var(--ui-border)';
  showBtn.style.color = 'var(--ui-text)';
  showBtn.style.cursor = 'pointer';
  showBtn.style.fontSize = '13px';
  showBtn.addEventListener('click', async () => {
    const dir = await GetLogDir();
    RevealInFileManager(dir);
  });
  headerRow.appendChild(showBtn);

  page.appendChild(headerRow);

  // Textarea for log output (selectable, Ctrl+A stays inside)
  const logArea = document.createElement('textarea');
  logArea.className = 'settings-debug-log';
  logArea.readOnly = true;
  logArea.wrap = 'off';
  logArea.spellcheck = false;

  // Load initial buffer
  GetStderrLog().then((text: string) => {
    logArea.value = text;
    logArea.scrollTop = logArea.scrollHeight;
  });

  // Listen for new lines
  const handler = (line: string) => {
    logArea.value += line;
    logArea.scrollTop = logArea.scrollHeight;
  };
  EventsOn('log:stderr', handler);

  page.appendChild(logArea);

  return {
    el: page,
    cleanup: () => { EventsOff('log:stderr'); },
  };
}
