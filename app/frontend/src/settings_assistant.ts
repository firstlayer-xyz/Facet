import type { SettingsPageContext, PageResult } from './settings_appearance';
import { DetectAssistantCLIs, GetDefaultSystemPrompt } from '../wailsjs/go/main/App';

interface CLIInfoFE {
  id: string;
  name: string;
  models: string[];
  defaultModel: string;
}

const CUSTOM_VALUE = '__custom__';

export function buildAssistantPage(ctx: SettingsPageContext): PageResult {
  const { draft } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  const loading = document.createElement('div');
  loading.style.color = '#888';
  loading.textContent = 'Detecting CLIs...';
  page.appendChild(loading);

  // Load detected CLIs and default prompt, then build the form
  Promise.all([
    DetectAssistantCLIs() as Promise<CLIInfoFE[]>,
    GetDefaultSystemPrompt() as Promise<string>,
  ]).then(([clis, defaultPrompt]) => {
    loading.remove();

    if (!clis || clis.length === 0) {
      const msg = document.createElement('div');
      msg.style.color = '#888';
      msg.textContent = 'No AI CLIs detected. Install claude, ollama, aichat, llm, or chatgpt.';
      page.appendChild(msg);
      return;
    }

    // CLI dropdown
    const cliRow = document.createElement('div');
    cliRow.className = 'settings-color-row';

    const cliLabel = document.createElement('label');
    cliLabel.textContent = 'CLI';

    const cliSelect = document.createElement('select');
    cliSelect.id = 'settings-assistant-cli';
    for (const cli of clis) {
      const opt = document.createElement('option');
      opt.value = cli.id;
      opt.textContent = cli.name;
      if (cli.id === draft.assistant.cli) opt.selected = true;
      cliSelect.appendChild(opt);
    }
    // If no saved CLI or saved CLI not available, select first detected
    if (!draft.assistant.cli || !clis.find(c => c.id === draft.assistant.cli)) {
      cliSelect.value = clis[0].id;
      draft.assistant.cli = clis[0].id;
    }

    cliRow.appendChild(cliLabel);
    cliRow.appendChild(cliSelect);
    page.appendChild(cliRow);

    // Model dropdown
    const modelRow = document.createElement('div');
    modelRow.className = 'settings-color-row';

    const modelLabel = document.createElement('label');
    modelLabel.textContent = 'Model';

    const modelSelect = document.createElement('select');
    modelSelect.id = 'settings-assistant-model';
    modelSelect.style.flex = '1';

    // Custom model text input (hidden by default)
    const customInput = document.createElement('input');
    customInput.type = 'text';
    customInput.style.flex = '1';
    customInput.style.marginLeft = '8px';
    customInput.style.display = 'none';
    customInput.placeholder = 'Enter model name';

    function populateModelSelect() {
      const cli = clis.find(c => c.id === cliSelect.value);
      const models = cli?.models ?? [];
      modelSelect.innerHTML = '';

      for (const m of models) {
        const opt = document.createElement('option');
        opt.value = m;
        opt.textContent = m;
        modelSelect.appendChild(opt);
      }

      // Always add a "Custom..." option
      const customOpt = document.createElement('option');
      customOpt.value = CUSTOM_VALUE;
      customOpt.textContent = 'Custom...';
      modelSelect.appendChild(customOpt);

      // Select the current model
      const current = draft.assistant.model;
      if (current && models.includes(current)) {
        modelSelect.value = current;
        customInput.style.display = 'none';
        customInput.value = '';
      } else if (current) {
        // Current model isn't in the list — show custom input
        modelSelect.value = CUSTOM_VALUE;
        customInput.style.display = '';
        customInput.value = current;
      } else if (cli?.defaultModel) {
        modelSelect.value = cli.defaultModel;
        draft.assistant.model = cli.defaultModel;
        customInput.style.display = 'none';
      }
    }

    populateModelSelect();

    modelSelect.addEventListener('change', () => {
      if (modelSelect.value === CUSTOM_VALUE) {
        customInput.style.display = '';
        customInput.focus();
        draft.assistant.model = customInput.value.trim();
      } else {
        customInput.style.display = 'none';
        customInput.value = '';
        draft.assistant.model = modelSelect.value;
      }
    });

    customInput.addEventListener('input', () => {
      draft.assistant.model = customInput.value.trim();
    });

    modelRow.appendChild(modelLabel);
    modelRow.appendChild(modelSelect);
    modelRow.appendChild(customInput);
    page.appendChild(modelRow);

    // On CLI change: repopulate models and select default
    cliSelect.addEventListener('change', () => {
      draft.assistant.cli = cliSelect.value;
      const cli = clis.find(c => c.id === cliSelect.value);
      draft.assistant.model = cli?.defaultModel ?? '';
      populateModelSelect();
    });

    // Max turns setting
    const turnsRow = document.createElement('div');
    turnsRow.className = 'settings-color-row';

    const turnsLabel = document.createElement('label');
    turnsLabel.textContent = 'Max Turns';
    turnsLabel.title = 'Maximum tool-use iterations per message (Claude only)';

    const turnsInput = document.createElement('input');
    turnsInput.type = 'number';
    turnsInput.min = '1';
    turnsInput.max = '50';
    turnsInput.step = '1';
    turnsInput.style.width = '60px';
    turnsInput.value = String(draft.assistant.maxTurns || 10);
    turnsInput.addEventListener('input', () => {
      draft.assistant.maxTurns = parseInt(turnsInput.value, 10) || 10;
    });

    turnsRow.appendChild(turnsLabel);
    turnsRow.appendChild(turnsInput);
    page.appendChild(turnsRow);

    // System prompt textarea
    const promptLabel = document.createElement('label');
    promptLabel.textContent = 'System Prompt';
    promptLabel.style.display = 'block';
    promptLabel.style.marginTop = '12px';
    promptLabel.style.marginBottom = '4px';
    page.appendChild(promptLabel);

    const promptTextarea = document.createElement('textarea');
    promptTextarea.id = 'settings-assistant-prompt';
    promptTextarea.rows = 10;
    promptTextarea.style.width = '100%';
    promptTextarea.style.resize = 'vertical';
    promptTextarea.style.fontFamily = 'monospace';
    promptTextarea.style.fontSize = '12px';
    promptTextarea.style.background = '#1a1a2e';
    promptTextarea.style.color = '#ccc';
    promptTextarea.style.border = '1px solid #444';
    promptTextarea.style.borderRadius = '4px';
    promptTextarea.style.padding = '8px';
    promptTextarea.value = draft.assistant.systemPrompt || defaultPrompt;
    promptTextarea.addEventListener('input', () => {
      draft.assistant.systemPrompt = promptTextarea.value;
    });
    page.appendChild(promptTextarea);

    // Reset prompt button
    const resetBtn = document.createElement('button');
    resetBtn.textContent = 'Reset to Default';
    resetBtn.style.marginTop = '8px';
    resetBtn.addEventListener('click', () => {
      promptTextarea.value = defaultPrompt;
      draft.assistant.systemPrompt = '';
    });
    page.appendChild(resetBtn);
  }).catch(() => {
    loading.textContent = 'Failed to detect CLIs';
  });

  return { el: page };
}
