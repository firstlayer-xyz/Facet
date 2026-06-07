import { on } from './events';
import type {
  AssistantQuestionOption,
  AssistantQuestionPayload,
  AssistantScreenshotRequest,
  AssistantPermissionRequest,
  AssistantTaskItem,
  AssistantTaskPlanPayload,
  AssistantTaskStatus,
} from './events';
import { SendAssistantMessage, CancelAssistant, ClearAssistantHistory, PickImageFile, DetectAssistantCLIs, GetAssistantEffortLevels, AnswerAssistantQuestion, DeliverViewportScreenshot, AnswerToolPermission }
  from '../wailsjs/go/main/App';
import type { AppSettings } from './settings';

type AssistantConfig = AppSettings['assistant'];

export class AssistantPanel {
  private container: HTMLElement;
  private panel: HTMLElement;
  private messagesDiv: HTMLElement;
  private input: HTMLTextAreaElement;
  private sendBtn: HTMLButtonElement;
  private attachBtn: HTMLButtonElement;
  private attachedImages: string[] = [];
  private attachBadge: HTMLSpanElement;
  private visible = false;
  private streaming = false;
  private receivedFirstToken = false;
  private currentStreamDiv: HTMLElement | null = null;
  private currentStreamText = '';
  private thinkingDiv: HTMLElement | null = null;
  private toolUseDiv: HTMLElement | null = null;
  private toolUseTimer: ReturnType<typeof setInterval> | null = null;
  private streamStartTime = 0;
  private getEditorCode: () => string;
  private getErrors: () => string;
  private getActiveTab: () => { path: string; readOnly: boolean };
  private onApplyCode: (newCode: string, searchFor?: string) => void;
  private onSetEditorSilent: (newCode: string) => void;
  private onNewFile: (name: string, code: string) => void;
  private offToken: (() => void) | null = null;
  private offDone: (() => void) | null = null;
  private offError: (() => void) | null = null;
  private offToolUse: (() => void) | null = null;
  private offReplaceCode: (() => void) | null = null;
  private offNewFile: (() => void) | null = null;
  private offThinking: (() => void) | null = null;
  private offQuestion: (() => void) | null = null;
  private offScreenshot: (() => void) | null = null;
  private offTaskPlan: (() => void) | null = null;
  private offPermission: (() => void) | null = null;
  private taskPlanDiv: HTMLElement | null = null;
  private captureScreenshot: ((opts?: AssistantScreenshotRequest) => string | null) | null = null;
  private getAssistantConfig: () => AssistantConfig;
  private onAssistantConfigChange: (cfg: AssistantConfig) => void;
  private modelSelect!: HTMLSelectElement;
  private effortSelect!: HTMLSelectElement;

  constructor(
    container: HTMLElement,
    getEditorCode: () => string,
    getErrors: () => string,
    getActiveTab: () => { path: string; readOnly: boolean },
    onApplyCode: (newCode: string, searchFor?: string) => void,
    onSetEditorSilent: (newCode: string) => void,
    onNewFile: (name: string, code: string) => void,
    getAssistantConfig: () => AssistantConfig,
    onAssistantConfigChange: (cfg: AssistantConfig) => void,
    captureScreenshot?: (opts?: AssistantScreenshotRequest) => string | null,
    onClose?: () => void,
  ) {
    this.container = container;
    this.getEditorCode = getEditorCode;
    this.getErrors = getErrors;
    this.getActiveTab = getActiveTab;
    this.onApplyCode = onApplyCode;
    this.onSetEditorSilent = onSetEditorSilent;
    this.onNewFile = onNewFile;
    this.getAssistantConfig = getAssistantConfig;
    this.onAssistantConfigChange = onAssistantConfigChange;
    this.captureScreenshot = captureScreenshot ?? null;

    this.panel = document.createElement('div');
    this.panel.id = 'assistant-panel';
    // Visibility is controlled by the `.open` class via CSS; show()/hide()
    // toggle it. No inline display style here.

    // Header
    const header = document.createElement('div');
    header.className = 'assistant-header';

    const titleArea = document.createElement('div');
    titleArea.className = 'assistant-title-area';

    const title = document.createElement('span');
    title.textContent = 'AI Assistant';
    titleArea.appendChild(title);

    header.appendChild(titleArea);

    const clearBtn = document.createElement('button');
    clearBtn.className = 'assistant-clear-btn';
    clearBtn.textContent = 'Clear';
    clearBtn.addEventListener('click', () => this.clearHistory());
    header.appendChild(clearBtn);

    if (onClose) {
      const closeBtn = document.createElement('button');
      closeBtn.className = 'assistant-close-btn';
      closeBtn.innerHTML = '&times;';
      closeBtn.title = 'Close assistant';
      closeBtn.addEventListener('click', () => { this.hide(); onClose(); });
      header.appendChild(closeBtn);
    }

    this.panel.appendChild(header);

    // Model + effort quick selector. Reads and writes the same persisted
    // assistant config the Settings page uses, so the two stay in sync.
    this.panel.appendChild(this.buildControls());

    // Messages area
    this.messagesDiv = document.createElement('div');
    this.messagesDiv.className = 'assistant-messages';
    this.panel.appendChild(this.messagesDiv);

    // Input area
    const inputArea = document.createElement('div');
    inputArea.className = 'assistant-input-area';

    // Attach image button
    const attachWrap = document.createElement('div');
    attachWrap.className = 'assistant-attach-wrap';

    this.attachBtn = document.createElement('button');
    this.attachBtn.className = 'assistant-attach-btn';
    this.attachBtn.title = 'Attach image';
    this.attachBtn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>`;
    this.attachBtn.addEventListener('click', () => this.pickImage());

    this.attachBadge = document.createElement('span');
    this.attachBadge.className = 'assistant-attach-badge';
    this.attachBadge.style.display = 'none';

    attachWrap.appendChild(this.attachBtn);
    attachWrap.appendChild(this.attachBadge);

    this.input = document.createElement('textarea');
    this.input.className = 'assistant-input';
    this.input.placeholder = 'Ask about your Facet code...';
    this.input.rows = 2;
    this.input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        this.send();
      }
    });

    this.sendBtn = document.createElement('button');
    this.sendBtn.className = 'assistant-send-btn';
    this.sendBtn.textContent = 'Send';
    this.sendBtn.addEventListener('click', () => {
      if (this.streaming) {
        CancelAssistant();
        this.finishStream();
      } else {
        this.send();
      }
    });

    inputArea.appendChild(attachWrap);
    inputArea.appendChild(this.input);
    inputArea.appendChild(this.sendBtn);
    this.panel.appendChild(inputArea);

    container.appendChild(this.panel);
  }

  private buildControls(): HTMLElement {
    const controls = document.createElement('div');
    controls.className = 'assistant-controls';
    controls.style.display = 'flex';
    controls.style.gap = '6px';
    controls.style.alignItems = 'center';
    controls.style.padding = '4px 8px';

    const styleSelect = (sel: HTMLSelectElement) => {
      sel.style.flex = '1';
      sel.style.minWidth = '0';
      sel.style.background = '#1a1a2e';
      sel.style.color = '#ccc';
      sel.style.border = '1px solid #444';
      sel.style.borderRadius = '4px';
      sel.style.padding = '2px 4px';
      sel.style.fontSize = '11px';
    };

    this.modelSelect = document.createElement('select');
    this.modelSelect.title = 'Model';
    styleSelect(this.modelSelect);
    this.modelSelect.addEventListener('change', () => this.applyConfigChange());

    this.effortSelect = document.createElement('select');
    this.effortSelect.title = 'Reasoning effort';
    styleSelect(this.effortSelect);
    this.effortSelect.addEventListener('change', () => this.applyConfigChange());

    controls.appendChild(this.modelSelect);
    controls.appendChild(this.effortSelect);
    // Selected values + model options are filled lazily in show(); `settings`
    // (which getAssistantConfig reads) isn't loaded yet at construction time.
    return controls;
  }

  // syncControlsFromConfig sets the selector to the persisted model + effort.
  // Called from show() rather than the constructor because the settings the
  // callbacks read are loaded after the panel is built.
  private syncControlsFromConfig(): void {
    void this.populateEffortSelect();
    void this.populateModelSelect();
  }

  // populateEffortSelect fills the effort dropdown from the levels the claude
  // CLI advertises in --help (via the backend), plus a leading "Default" that
  // sends no --effort. A configured-but-unadvertised value stays selectable so
  // detection hiccups don't silently drop the user's choice.
  private async populateEffortSelect(): Promise<void> {
    let levels: string[] = [];
    try {
      levels = (await GetAssistantEffortLevels()) ?? [];
    } catch {
      // Detection failed — only "Default" is offered.
    }
    const cfg = this.getAssistantConfig();
    if (cfg.effort && !levels.includes(cfg.effort)) levels = [cfg.effort, ...levels];
    this.effortSelect.innerHTML = '';
    const def = document.createElement('option');
    def.value = '';
    def.textContent = 'Default';
    this.effortSelect.appendChild(def);
    for (const lv of levels) {
      const o = document.createElement('option');
      o.value = lv;
      o.textContent = lv.charAt(0).toUpperCase() + lv.slice(1);
      this.effortSelect.appendChild(o);
    }
    this.effortSelect.value = cfg.effort || '';
  }

  // populateModelSelect fills the model dropdown from the detected CLI's model
  // list, ensuring the currently-configured model (which may be a custom one
  // set in Settings) is present and selected.
  private async populateModelSelect(): Promise<void> {
    const cfg = this.getAssistantConfig();
    let models: string[] = [];
    try {
      const clis = await DetectAssistantCLIs();
      const cli = (clis ?? []).find(c => c.id === (cfg.cli || 'claude')) ?? (clis ?? [])[0];
      models = cli?.models ?? [];
    } catch {
      // Detection failed — fall back to just the configured model below.
    }
    const opts = [...models];
    if (cfg.model && !opts.includes(cfg.model)) opts.unshift(cfg.model);
    this.modelSelect.innerHTML = '';
    for (const m of opts) {
      const o = document.createElement('option');
      o.value = m;
      o.textContent = m;
      this.modelSelect.appendChild(o);
    }
    if (cfg.model) this.modelSelect.value = cfg.model;
  }

  // applyConfigChange writes the selector's model + effort back into the
  // persisted assistant config. The model is only read from the dropdown once
  // it has options, so an early effort change can't clobber it with "".
  private applyConfigChange(): void {
    const current = this.getAssistantConfig();
    const model = this.modelSelect.options.length > 0 ? this.modelSelect.value : current.model;
    this.onAssistantConfigChange({ ...current, model, effort: this.effortSelect.value });
  }

  show(): void {
    this.visible = true;
    this.panel.classList.add('open');
    this.registerEvents();
    this.input.focus();
    this.checkForCLIs();
    this.syncControlsFromConfig();
  }

  private noCLIBanner: HTMLElement | null = null;
  private cliCheckDone = false;

  private async checkForCLIs(): Promise<void> {
    if (this.cliCheckDone) return;
    try {
      const clis = await DetectAssistantCLIs();
      this.cliCheckDone = true;
      if (!clis || clis.length === 0) {
        this.showNoCLIBanner();
      }
    } catch {
      // ignore detection errors
    }
  }

  private showNoCLIBanner(): void {
    if (this.noCLIBanner) return;
    this.noCLIBanner = document.createElement('div');
    this.noCLIBanner.className = 'assistant-no-cli';
    const isMac = navigator.platform?.startsWith('Mac') || navigator.userAgent.includes('Mac');
    let installHtml = `<strong>Currently only Claude is supported.</strong><br><br>Install the Claude CLI to use the AI assistant:<br><br>`;
    if (isMac) {
      installHtml += `<code>brew install claude-code</code><br>or <code>npm install -g @anthropic-ai/claude-code</code>`;
    } else {
      installHtml += `<code>npm install -g @anthropic-ai/claude-code</code>`;
    }
    installHtml += `<br><br><em>Restart Facet after installing.</em>`;
    installHtml += `<br><br>More AI assistants coming soon.`;
    this.noCLIBanner.innerHTML = installHtml;
    this.messagesDiv.insertBefore(this.noCLIBanner, this.messagesDiv.firstChild);
  }

  hide(): void {
    this.visible = false;
    this.panel.classList.remove('open');
    if (this.streaming) {
      CancelAssistant();
      this.finishStream();
    }
    this.unregisterEvents();
  }

  toggle(): void {
    if (this.visible) this.hide();
    else this.show();
  }

  isVisible(): boolean {
    return this.visible;
  }

  private registerEvents(): void {
    if (this.offToken) return;
    this.offToken = on('assistant:token', (token: string) => {
      this.appendToken(token);
    });
    this.offDone = on('assistant:done', () => {
      this.finishStream();
    });
    this.offError = on('assistant:error', (msg: string) => {
      this.showError(msg);
    });
    // MCP tool-use indicator. Some tools have their own UI affordances
    // (question card, task plan, screenshot flash) — suppress the
    // generic indicator for those so we don't show a spurious
    // "<tool_name>..." line before the real UI lands.
    this.offToolUse = on('assistant:tool-use', (toolName: string, callNum: number) => {
      if (toolName === 'ask_user_question' || toolName === 'update_task_plan' || toolName === 'screenshot_viewport' || toolName === 'request_permission') return;
      this.showToolUseIndicator(toolName, callNum);
    });
    // ask_user_question MCP tool — render an interactive multiple-choice
    // card. The backend blocks the model on a channel until the user
    // submits, at which point AnswerAssistantQuestion routes the answer
    // back as the tool's JSON result.
    this.offQuestion = on('assistant:question', (payload: AssistantQuestionPayload) => {
      this.showQuestion(payload);
    });
    // request_permission / fetch_url self-gate — render an Allow/Deny card.
    // The backend blocks the tool on a channel until AnswerToolPermission
    // routes the decision back.
    this.offPermission = on('assistant:permission-request', (payload: AssistantPermissionRequest) => {
      this.showPermission(payload);
    });
    // screenshot_viewport MCP tool — capture the live viewport and hand
    // the PNG back to the blocked tool handler. captureScreenshot is
    // optional (the test harness wires no viewer); fail explicitly so
    // the model gets a clear tool error instead of hanging.
    this.offScreenshot = on('assistant:screenshot-request', async (payload: AssistantScreenshotRequest) => {
      if (!payload?.id) return;
      if (!this.captureScreenshot) {
        try { await DeliverViewportScreenshot(payload.id, '', 'no viewport available'); } catch {}
        return;
      }
      let dataURL: string | null = null;
      let err = '';
      try {
        dataURL = this.captureScreenshot(payload);
      } catch (e: any) {
        err = e?.message || String(e);
      }
      try {
        await DeliverViewportScreenshot(payload.id, dataURL ?? '', err || (dataURL ? '' : 'capture returned no data'));
      } catch (e) {
        console.warn('DeliverViewportScreenshot failed:', e);
      }
    });
    // update_task_plan MCP tool — render or update the task list. One-way;
    // each call REPLACES the rendered list (the model sends full state).
    this.offTaskPlan = on('assistant:task-plan', (payload: AssistantTaskPlanPayload) => {
      this.renderTaskPlan(payload?.tasks ?? []);
    });
    // MCP-driven code changes — update editor only, Go handles the build.
    // Reject if the active tab is read-only: the backend guards already
    // refuse read-only edits, but the event could be delivered out-of-band
    // (e.g. user switched to a read-only tab mid-run). Silent-overwrite of
    // a read-only file would corrupt its in-memory view.
    this.offReplaceCode = on('assistant:replace-code', (code: string) => {
      if (this.getActiveTab().readOnly) return;
      this.onSetEditorSilent(code);
    });
    // MCP new_file tool — create a fresh editable tab with the given source.
    this.offNewFile = on('assistant:new-file', (payload: { name: string; code: string }) => {
      if (!payload) return;
      this.onNewFile(payload.name ?? 'Untitled', payload.code ?? '');
    });
    // Thinking indicator — shown after tool results, before next assistant message
    this.offThinking = on('assistant:thinking', (callNum: number) => {
      this.showThinkingIndicator(callNum);
    });
  }

  private unregisterEvents(): void {
    if (this.offToken) { this.offToken(); this.offToken = null; }
    if (this.offDone) { this.offDone(); this.offDone = null; }
    if (this.offError) { this.offError(); this.offError = null; }
    if (this.offToolUse) { this.offToolUse(); this.offToolUse = null; }
    if (this.offReplaceCode) { this.offReplaceCode(); this.offReplaceCode = null; }
    if (this.offNewFile) { this.offNewFile(); this.offNewFile = null; }
    if (this.offThinking) { this.offThinking(); this.offThinking = null; }
    if (this.offQuestion) { this.offQuestion(); this.offQuestion = null; }
    if (this.offScreenshot) { this.offScreenshot(); this.offScreenshot = null; }
    if (this.offTaskPlan) { this.offTaskPlan(); this.offTaskPlan = null; }
    if (this.offPermission) { this.offPermission(); this.offPermission = null; }
  }

  private async pickImage(): Promise<void> {
    try {
      const path = await PickImageFile();
      if (path) {
        this.attachedImages.push(path);
        this.updateAttachBadge();
      }
    } catch (err) {
      console.error('Failed to pick image:', err);
    }
  }

  private updateAttachBadge(): void {
    const count = this.attachedImages.length;
    if (count > 0) {
      this.attachBadge.textContent = String(count);
      this.attachBadge.style.display = 'flex';
    } else {
      this.attachBadge.style.display = 'none';
    }
  }

  private async send(): Promise<void> {
    const text = this.input.value.trim();
    if (!text || this.streaming) return;

    this.input.value = '';

    // Show user message using textContent (safe from XSS)
    const userDiv = document.createElement('div');
    userDiv.className = 'assistant-msg assistant-msg-user';
    userDiv.style.whiteSpace = 'pre-wrap';
    userDiv.textContent = text;
    if (this.attachedImages.length > 0) {
      const label = document.createElement('div');
      label.className = 'assistant-attached-label';
      const n = this.attachedImages.length;
      label.textContent = `${n} image${n > 1 ? 's' : ''} attached`;
      userDiv.appendChild(label);
    }
    this.messagesDiv.appendChild(userDiv);
    this.scrollToBottom();

    this.streaming = true;
    this.receivedFirstToken = false;
    this.streamStartTime = Date.now();
    this.sendBtn.textContent = 'Stop';
    this.sendBtn.classList.add('assistant-send-btn-stop');
    this.currentStreamText = '';
    this.currentStreamDiv = null;

    this.showThinking();

    const images = [...this.attachedImages];
    this.attachedImages = [];
    this.updateAttachBadge();

    try {
      const tab = this.getActiveTab();
      await SendAssistantMessage(text, this.getEditorCode(), this.getErrors(), tab.path, tab.readOnly, images);
    } catch (err: any) {
      this.showError(err?.message || String(err));
    }
  }

  private thinkingTimer: ReturnType<typeof setInterval> | null = null;

  private showThinking(): void {
    this.removeThinking();
    this.thinkingDiv = document.createElement('div');
    this.thinkingDiv.className = 'assistant-msg assistant-msg-assistant assistant-thinking';
    const updateThinking = () => {
      if (!this.thinkingDiv) return;
      const elapsed = Math.floor((Date.now() - this.streamStartTime) / 1000);
      this.thinkingDiv.innerHTML = `<span class="thinking-dots"><span>.</span><span>.</span><span>.</span></span> <span class="thinking-elapsed">${elapsed}s</span>`;
    };
    updateThinking();
    this.thinkingTimer = setInterval(updateThinking, 1000);
    this.messagesDiv.appendChild(this.thinkingDiv);
    this.scrollToBottom();
  }

  private removeThinking(): void {
    if (this.thinkingTimer) {
      clearInterval(this.thinkingTimer);
      this.thinkingTimer = null;
    }
    if (this.thinkingDiv) {
      this.thinkingDiv.remove();
      this.thinkingDiv = null;
    }
  }

  private showToolUseIndicator(toolName: string, callNum?: number): void {
    const labels: Record<string, string> = {
      'get_editor_code': 'Reading code',
      'edit_code': 'Editing code',
      'replace_code': 'Writing code',
      'get_last_run': 'Checking build results',
      'check_syntax': 'Checking syntax',
      'get_documentation': 'Looking up docs',
    };
    const action = labels[toolName] || toolName;

    // Finalize any in-progress text so it becomes its own bubble
    this.finalizeCurrentMessage();
    // Remove previous indicator and timer
    this.removeToolUseIndicator();
    // Also remove thinking dots — we have a real status now
    this.removeThinking();

    this.toolUseDiv = document.createElement('div');
    this.toolUseDiv.className = 'assistant-tool-use';
    this.messagesDiv.appendChild(this.toolUseDiv);

    const updateText = () => {
      if (!this.toolUseDiv) return;
      const elapsed = Math.floor((Date.now() - this.streamStartTime) / 1000);
      const prefix = callNum ? `[${callNum}] ` : '';
      this.toolUseDiv.textContent = `${prefix}${action}... (${elapsed}s)`;
    };
    updateText();
    this.toolUseTimer = setInterval(() => {
      updateText();
    }, 1000);

    this.scrollToBottom();
  }

  private showThinkingIndicator(callNum: number): void {
    this.finalizeCurrentMessage();
    this.removeToolUseIndicator();
    this.removeThinking();

    this.toolUseDiv = document.createElement('div');
    this.toolUseDiv.className = 'assistant-tool-use';
    this.messagesDiv.appendChild(this.toolUseDiv);

    const startTime = Date.now();
    const updateText = () => {
      if (!this.toolUseDiv) return;
      const elapsed = Math.floor((Date.now() - startTime) / 1000);
      this.toolUseDiv.textContent = `[${callNum}] Thinking... (${elapsed}s)`;
    };
    updateText();
    this.toolUseTimer = setInterval(updateText, 1000);
    this.scrollToBottom();
  }

  private removeToolUseIndicator(): void {
    if (this.toolUseTimer) {
      clearInterval(this.toolUseTimer);
      this.toolUseTimer = null;
    }
    if (this.toolUseDiv) {
      this.toolUseDiv.remove();
      this.toolUseDiv = null;
    }
  }

  // renderTaskPlan updates the task-list card in place. The model
  // sends the FULL list each call, but we diff against the existing
  // DOM rather than wipe-and-rebuild — recreating the card on every
  // update causes a visible flicker, which read as "didn't update in
  // real time" the first time around. Per-item icon/text/class
  // changes are visually obvious thanks to the pulsing in_progress
  // glyph in CSS, so the user can see exactly which step moved.
  private renderTaskPlan(tasks: AssistantTaskItem[]): void {
    if (tasks.length === 0) {
      if (this.taskPlanDiv) {
        this.taskPlanDiv.remove();
        this.taskPlanDiv = null;
      }
      return;
    }

    let card = this.taskPlanDiv;
    let list: HTMLOListElement;
    if (!card || !card.parentElement) {
      card = document.createElement('div');
      card.className = 'assistant-task-plan';
      const heading = document.createElement('div');
      heading.className = 'assistant-task-plan-heading';
      heading.textContent = 'Plan';
      card.appendChild(heading);
      list = document.createElement('ol');
      list.className = 'assistant-task-list';
      card.appendChild(list);
      this.taskPlanDiv = card;
      this.messagesDiv.appendChild(card);
    } else {
      list = card.querySelector('.assistant-task-list') as HTMLOListElement;
    }

    const icons: Record<AssistantTaskStatus, string> = {
      pending: '○',
      in_progress: '▸',
      completed: '✓',
    };

    // Update or create rows in order. Any extras left over from a
    // longer previous list get removed below.
    const existing = list.querySelectorAll<HTMLElement>('.assistant-task');
    for (let i = 0; i < tasks.length; i++) {
      const t = tasks[i];
      let li = existing[i];
      let icon: HTMLElement;
      let txt: HTMLElement;
      if (!li) {
        li = document.createElement('li');
        icon = document.createElement('span');
        icon.className = 'assistant-task-icon';
        txt = document.createElement('span');
        txt.className = 'assistant-task-text';
        li.appendChild(icon);
        li.appendChild(txt);
        list.appendChild(li);
      } else {
        icon = li.children[0] as HTMLElement;
        txt = li.children[1] as HTMLElement;
      }
      const newClass = `assistant-task assistant-task-${t.status}`;
      if (li.className !== newClass) li.className = newClass;
      const newIcon = icons[t.status];
      if (icon.textContent !== newIcon) icon.textContent = newIcon;
      if (txt.textContent !== t.content) txt.textContent = t.content;
    }
    for (let i = tasks.length; i < existing.length; i++) {
      existing[i].remove();
    }
    this.scrollToBottom();
  }

  // showQuestion renders an interactive multiple-choice card from the
  // ask_user_question payload. Each question gets 2-4 options plus an
  // automatic "Other" entry that reveals a free-text input. The model
  // is blocked on a channel in the MCP layer; once Submit is clicked,
  // AnswerAssistantQuestion routes the selections back as the tool's
  // result and the card locks into a read-only summary.
  private showQuestion(payload: AssistantQuestionPayload): void {
    if (!payload?.questions?.length) return;
    this.finalizeCurrentMessage();
    this.removeToolUseIndicator();
    this.removeThinking();

    const card = document.createElement('div');
    card.className = 'assistant-question-card';

    // Per-question UI state. `selected` is a Set so multiSelect questions
    // can toggle multiple labels; single-select questions just keep one.
    const state = payload.questions.map(() => ({
      selected: new Set<string>(),
      otherText: '',
      otherActive: false,
    }));

    const otherInputs: HTMLTextAreaElement[] = [];

    payload.questions.forEach((q, qi) => {
      const block = document.createElement('div');
      block.className = 'assistant-question-block';

      const headerRow = document.createElement('div');
      headerRow.className = 'assistant-question-header';
      if (q.header) {
        const chip = document.createElement('span');
        chip.className = 'assistant-question-chip';
        chip.textContent = q.header;
        headerRow.appendChild(chip);
      }
      const qText = document.createElement('span');
      qText.className = 'assistant-question-text';
      qText.textContent = q.question;
      headerRow.appendChild(qText);
      block.appendChild(headerRow);

      const otherInput = document.createElement('textarea');
      otherInput.className = 'assistant-question-other';
      otherInput.placeholder = 'Your answer...';
      otherInput.rows = 2;
      otherInput.style.display = 'none';
      otherInput.addEventListener('input', () => {
        state[qi].otherText = otherInput.value;
      });
      otherInputs.push(otherInput);

      const opts = document.createElement('div');
      opts.className = 'assistant-question-options';
      // Always append a synthetic "Other" so the user can supply free
      // text even when none of the model's options fit. Matching the
      // built-in AskUserQuestion convention.
      const allOptions: AssistantQuestionOption[] = [
        ...q.options,
        { label: 'Other', description: 'Provide custom text' },
      ];

      allOptions.forEach((opt) => {
        const optBtn = document.createElement('button');
        optBtn.type = 'button';
        optBtn.className = 'assistant-question-option';

        const labelEl = document.createElement('div');
        labelEl.className = 'assistant-question-option-label';
        labelEl.textContent = opt.label;
        optBtn.appendChild(labelEl);

        if (opt.description) {
          const descEl = document.createElement('div');
          descEl.className = 'assistant-question-option-desc';
          descEl.textContent = opt.description;
          optBtn.appendChild(descEl);
        }

        optBtn.addEventListener('click', () => {
          const isOther = opt.label === 'Other';
          if (q.multiSelect) {
            if (state[qi].selected.has(opt.label)) {
              state[qi].selected.delete(opt.label);
              optBtn.classList.remove('selected');
            } else {
              state[qi].selected.add(opt.label);
              optBtn.classList.add('selected');
            }
          } else {
            state[qi].selected.clear();
            state[qi].selected.add(opt.label);
            opts.querySelectorAll('.assistant-question-option').forEach(b => b.classList.remove('selected'));
            optBtn.classList.add('selected');
          }
          if (isOther) {
            state[qi].otherActive = state[qi].selected.has('Other');
            otherInput.style.display = state[qi].otherActive ? 'block' : 'none';
            if (state[qi].otherActive) otherInput.focus();
          }
        });

        opts.appendChild(optBtn);
      });

      block.appendChild(opts);
      block.appendChild(otherInput);
      card.appendChild(block);
    });

    const footer = document.createElement('div');
    footer.className = 'assistant-question-footer';
    const errMsg = document.createElement('span');
    errMsg.className = 'assistant-question-error';
    footer.appendChild(errMsg);
    const submitBtn = document.createElement('button');
    submitBtn.type = 'button';
    submitBtn.className = 'assistant-question-submit';
    submitBtn.textContent = 'Submit';
    footer.appendChild(submitBtn);
    card.appendChild(footer);

    this.messagesDiv.appendChild(card);
    this.scrollToBottom();

    submitBtn.addEventListener('click', async () => {
      // Validate: every question needs at least one selection, and
      // "Other" requires non-empty text. Short-circuit on the first
      // failure with an inline message; don't lock the card.
      for (let qi = 0; qi < payload.questions.length; qi++) {
        if (state[qi].selected.size === 0) {
          errMsg.textContent = `Pick an option for: ${payload.questions[qi].header || payload.questions[qi].question}`;
          return;
        }
        if (state[qi].selected.has('Other') && !state[qi].otherText.trim()) {
          errMsg.textContent = `Type a custom answer for: ${payload.questions[qi].header || payload.questions[qi].question}`;
          otherInputs[qi].focus();
          return;
        }
      }
      errMsg.textContent = '';

      const answers: Record<string, string> = {};
      const notes: Record<string, string> = {};
      payload.questions.forEach((q, qi) => {
        const sel = Array.from(state[qi].selected);
        // Replace 'Other' with the user's free text in the answer
        // string (so the model sees the actual choice), and stash the
        // original text in notes for reference.
        const finalLabels = sel.map(l => l === 'Other' ? state[qi].otherText.trim() : l);
        answers[q.question] = finalLabels.join(', ');
        if (state[qi].otherActive && state[qi].otherText.trim()) {
          notes[q.question] = state[qi].otherText.trim();
        }
      });

      // Lock the card and disable further input.
      card.classList.add('answered');
      card.querySelectorAll('button, textarea').forEach(el => (el as HTMLButtonElement | HTMLTextAreaElement).disabled = true);
      submitBtn.textContent = 'Sent';
      this.showThinking();

      try {
        await AnswerAssistantQuestion(payload.id, answers, notes);
      } catch (err: any) {
        this.showError(`Failed to send answer: ${err?.message || err}`);
      }
    });
  }

  // showPermission renders an Allow/Deny card for a tool the model wants to
  // use. The model is blocked on a channel in the MCP layer; on click,
  // AnswerToolPermission routes the decision back and the card locks.
  private showPermission(payload: AssistantPermissionRequest): void {
    if (!payload?.id) return;
    this.finalizeCurrentMessage();
    this.removeToolUseIndicator();
    this.removeThinking();

    const card = document.createElement('div');
    card.className = 'assistant-question-card';

    const block = document.createElement('div');
    block.className = 'assistant-question-block';
    const headerRow = document.createElement('div');
    headerRow.className = 'assistant-question-header';
    const chip = document.createElement('span');
    chip.className = 'assistant-question-chip';
    chip.textContent = 'Permission';
    headerRow.appendChild(chip);
    const qText = document.createElement('span');
    qText.className = 'assistant-question-text';
    qText.textContent = payload.summary || `Allow tool: ${payload.toolName}?`;
    headerRow.appendChild(qText);
    block.appendChild(headerRow);

    const rememberRow = document.createElement('label');
    rememberRow.className = 'assistant-question-other';
    rememberRow.style.display = 'flex';
    rememberRow.style.alignItems = 'center';
    rememberRow.style.gap = '6px';
    const remember = document.createElement('input');
    remember.type = 'checkbox';
    rememberRow.appendChild(remember);
    const rememberText = document.createElement('span');
    rememberText.textContent = 'Remember for this session';
    rememberRow.appendChild(rememberText);
    block.appendChild(rememberRow);
    card.appendChild(block);

    const footer = document.createElement('div');
    footer.className = 'assistant-question-footer';
    const denyBtn = document.createElement('button');
    denyBtn.type = 'button';
    denyBtn.className = 'assistant-question-submit';
    denyBtn.textContent = 'Deny';
    const allowBtn = document.createElement('button');
    allowBtn.type = 'button';
    allowBtn.className = 'assistant-question-submit';
    allowBtn.textContent = 'Allow';
    footer.appendChild(denyBtn);
    footer.appendChild(allowBtn);
    card.appendChild(footer);

    this.messagesDiv.appendChild(card);
    this.scrollToBottom();

    const decide = async (allow: boolean) => {
      card.classList.add('answered');
      card.querySelectorAll('button, input').forEach(el => (el as HTMLButtonElement | HTMLInputElement).disabled = true);
      allowBtn.textContent = allow ? 'Allowed' : 'Allow';
      denyBtn.textContent = allow ? 'Deny' : 'Denied';
      this.showThinking();
      try {
        await AnswerToolPermission(payload.id, allow, remember.checked);
      } catch (err: any) {
        this.showError(`Failed to send permission decision: ${err?.message || err}`);
      }
    };
    allowBtn.addEventListener('click', () => decide(true));
    denyBtn.addEventListener('click', () => decide(false));
  }

  private appendToken(token: string): void {
    // When we get text tokens, remove any tool-use indicator
    this.removeToolUseIndicator();

    if (!this.receivedFirstToken) {
      this.receivedFirstToken = true;
      this.removeThinking();
      this.currentStreamDiv = this.addMessageDiv('assistant', '');
    }
    this.currentStreamText += token;
    if (this.currentStreamDiv) {
      this.currentStreamDiv.innerHTML = this.renderMarkdown(this.currentStreamText) + '<span class="streaming-cursor"></span>';
      this.scrollToBottom();
    }
  }

  /** Finalize the current assistant message bubble so the next text starts a new one. */
  private finalizeCurrentMessage(): void {
    if (this.currentStreamDiv && this.currentStreamText) {
      this.currentStreamDiv.innerHTML = this.renderMarkdown(this.currentStreamText);
      this.addApplyButtons(this.currentStreamDiv);
    }
    this.currentStreamDiv = null;
    this.currentStreamText = '';
    this.receivedFirstToken = false;
  }

  private finishStream(): void {
    this.streaming = false;
    this.sendBtn.textContent = 'Send';
    this.sendBtn.classList.remove('assistant-send-btn-stop');
    this.removeThinking();
    this.removeToolUseIndicator();
    this.finalizeCurrentMessage();
    this.scrollToBottom();
  }

  private addMessageDiv(role: string, html: string): HTMLElement {
    const div = document.createElement('div');
    div.className = `assistant-msg assistant-msg-${role}`;
    div.innerHTML = html;
    this.messagesDiv.appendChild(div);
    this.scrollToBottom();
    return div;
  }

  private addApplyButtons(div: HTMLElement): void {
    // Handle regular fenced code blocks
    const codeBlocks = div.querySelectorAll('pre code');
    codeBlocks.forEach((block) => {
      const pre = block.parentElement;
      if (!pre || pre.closest('.assistant-edit-block')) return;
      const code = block.textContent || '';

      const btnGroup = document.createElement('div');
      btnGroup.className = 'assistant-code-btns';

      const copyBtn = document.createElement('button');
      copyBtn.className = 'assistant-copy-btn';
      copyBtn.textContent = 'Copy';
      copyBtn.addEventListener('click', () => {
        navigator.clipboard.writeText(code).then(() => {
          copyBtn.textContent = 'Copied';
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        }).catch(() => {
          copyBtn.textContent = 'Error';
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        });
      });
      btnGroup.appendChild(copyBtn);

      // Only show "Apply" for complete programs (contains Main function)
      if (/\bMain\s*\(/.test(code)) {
        const applyBtn = document.createElement('button');
        applyBtn.className = 'assistant-apply-btn';
        applyBtn.textContent = 'Apply';
        applyBtn.addEventListener('click', () => {
          this.onApplyCode(code);
          applyBtn.textContent = 'Applied';
          applyBtn.disabled = true;
          setTimeout(() => { applyBtn.textContent = 'Apply'; applyBtn.disabled = false; }, 1500);
        });
        btnGroup.appendChild(applyBtn);
      }

      pre.appendChild(btnGroup);
    });

    // Handle SEARCH/REPLACE edit blocks
    const editBlocks = div.querySelectorAll('.assistant-edit-block');
    editBlocks.forEach((block) => {
      const htmlBlock = block as HTMLElement;
      const searchFor = decodeURIComponent(htmlBlock.dataset.search || '');
      const replaceWith = decodeURIComponent(htmlBlock.dataset.replace || '');

      const btnGroup = document.createElement('div');
      btnGroup.className = 'assistant-code-btns';

      const copyBtn = document.createElement('button');
      copyBtn.className = 'assistant-copy-btn';
      copyBtn.textContent = 'Copy';
      copyBtn.addEventListener('click', () => {
        navigator.clipboard.writeText(replaceWith).then(() => {
          copyBtn.textContent = 'Copied';
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        }).catch(() => {
          copyBtn.textContent = 'Failed';
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        });
      });
      btnGroup.appendChild(copyBtn);

      const applyBtn = document.createElement('button');
      applyBtn.className = 'assistant-apply-btn';
      applyBtn.textContent = 'Apply';
      applyBtn.addEventListener('click', () => {
        this.onApplyCode(replaceWith, searchFor);
        applyBtn.textContent = 'Applied';
        applyBtn.disabled = true;
        setTimeout(() => { applyBtn.textContent = 'Apply'; applyBtn.disabled = false; }, 1500);
      });
      btnGroup.appendChild(applyBtn);

      htmlBlock.appendChild(btnGroup);
    });
  }

  private showError(msg: string): void {
    this.finishStream();
    const div = document.createElement('div');
    div.className = 'assistant-msg assistant-msg-error';
    div.textContent = 'Error: ' + msg;
    this.messagesDiv.appendChild(div);
    this.scrollToBottom();
  }

  private async clearHistory(): Promise<void> {
    if (this.streaming) {
      CancelAssistant();
      this.finishStream();
    }
    await ClearAssistantHistory();
    this.messagesDiv.innerHTML = '';
  }

  private scrollToBottom(): void {
    this.messagesDiv.scrollTop = this.messagesDiv.scrollHeight;
  }

  private static escapeHtml(text: string): string {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  private renderMarkdown(text: string): string {
    // Parse segments in order: edit blocks, fenced code blocks, then plain text.
    // We work on the raw text so we can do exact matching before HTML-escaping.
    type Segment =
      | { type: 'edit'; search: string; replace: string }
      | { type: 'code'; lang: string; code: string }
      | { type: 'text'; content: string };

    const segments: Segment[] = [];
    const EDIT_RE = /<<<<<<< SEARCH\n([\s\S]*?)\n=======\n([\s\S]*?)\n>>>>>>> REPLACE/g;
    const CODE_RE = /```(\w*)\n([\s\S]*?)```/g;

    // Collect all matches with their positions
    type RawMatch = { index: number; end: number; seg: Segment };
    const matches: RawMatch[] = [];

    let m: RegExpExecArray | null;
    EDIT_RE.lastIndex = 0;
    while ((m = EDIT_RE.exec(text)) !== null) {
      matches.push({ index: m.index, end: m.index + m[0].length, seg: { type: 'edit', search: m[1], replace: m[2] } });
    }
    CODE_RE.lastIndex = 0;
    while ((m = CODE_RE.exec(text)) !== null) {
      matches.push({ index: m.index, end: m.index + m[0].length, seg: { type: 'code', lang: m[1], code: m[2] } });
    }
    matches.sort((a, b) => a.index - b.index);

    // Build segments, interleaving plain text between matches
    // Skip overlapping matches (earlier match wins)
    let pos = 0;
    for (const match of matches) {
      if (match.index < pos) continue; // overlapping — skip
      if (match.index > pos) {
        segments.push({ type: 'text', content: text.slice(pos, match.index) });
      }
      segments.push(match.seg);
      pos = match.end;
    }
    if (pos < text.length) {
      segments.push({ type: 'text', content: text.slice(pos) });
    }

    // Render each segment to HTML
    return segments.map(seg => {
      if (seg.type === 'edit') {
        const esc = AssistantPanel.escapeHtml;
        return `<div class="assistant-edit-block" data-search="${encodeURIComponent(seg.search)}" data-replace="${encodeURIComponent(seg.replace)}">` +
          `<div class="edit-block-label">Edit</div>` +
          `<pre><code class="language-facet">${esc(seg.replace)}</code></pre>` +
          `</div>`;
      }
      if (seg.type === 'code') {
        const cls = seg.lang ? ` class="language-${seg.lang}"` : '';
        return `<pre><code${cls}>${AssistantPanel.escapeHtml(seg.code)}</code></pre>`;
      }
      // Plain text: escape, then apply inline markdown
      let html = AssistantPanel.escapeHtml(seg.content);
      html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
      html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
      html = html.replace(/\n/g, '<br>');
      return html;
    }).join('');
  }
}

/**
 * Apply a SEARCH/REPLACE edit: find `searchFor` in `original` and return the
 * text with it replaced by `replaceWith`. Returns null if not found.
 */
export function applyEdit(original: string, searchFor: string, replaceWith: string): string | null {
  const idx = original.indexOf(searchFor);
  if (idx === -1) return null;
  return original.slice(0, idx) + replaceWith + original.slice(idx + searchFor.length);
}
