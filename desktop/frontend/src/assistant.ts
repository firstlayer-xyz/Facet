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
  private streaming = false;
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
  private offs: (() => void)[] = [];
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

    this.modelSelect = document.createElement('select');
    this.modelSelect.title = 'Model';
    this.modelSelect.addEventListener('change', () => this.applyConfigChange());

    this.effortSelect = document.createElement('select');
    this.effortSelect.title = 'Reasoning effort';
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
    this.panel.classList.remove('open');
    if (this.streaming) {
      CancelAssistant();
      this.finishStream();
    }
    this.unregisterEvents();
  }

  toggle(): void {
    if (this.isVisible()) this.hide();
    else this.show();
  }

  isVisible(): boolean {
    return this.panel.classList.contains('open');
  }

  /** Open the panel (if needed) and submit a prompt as if the user typed it and
   *  pressed Send — for automation/demos. No-op while a response streams. */
  submitPrompt(prompt: string): void {
    if (!this.isVisible()) this.show();
    this.input.value = prompt;
    void this.send();
  }

  /** True while a response is streaming — automation polls this to wait for the
   *  agent to finish a round before sending the next. */
  isStreaming(): boolean {
    return this.streaming;
  }

  /** Plain text of the most recent assistant message — lets a driver read what
   *  the AI said (e.g. a clarifying question) and reply, instead of forcing a
   *  no-questions one-shot. Empty if the assistant hasn't spoken yet. */
  lastResponse(): string {
    const msgs = this.messagesDiv.querySelectorAll('.assistant-msg-assistant');
    const last = msgs[msgs.length - 1] as HTMLElement | undefined;
    return last ? (last.textContent ?? '').trim() : '';
  }

  /** The active ask_user_question payload (a multiple-choice card is showing on
   *  camera), or null. A driver reads this to know what the AI asked, then answers
   *  by cursor-clicking an option + Submit — a real interactive Q&A. */
  private pendingQuestion: AssistantQuestionPayload | null = null;
  currentQuestion(): AssistantQuestionPayload | null {
    return this.pendingQuestion;
  }

  private registerEvents(): void {
    if (this.offs.length > 0) return;
    this.offs = [
      on('assistant:token', (token: string) => {
        this.appendToken(token);
      }),
      on('assistant:done', () => {
        this.finishStream();
      }),
      on('assistant:error', (msg: string) => {
        this.showError(msg);
      }),
      // MCP tool-use indicator. Some tools have their own UI affordances
      // (question card, task plan, screenshot flash) — suppress the
      // generic indicator for those so we don't show a spurious
      // "<tool_name>..." line before the real UI lands.
      on('assistant:tool-use', (toolName: string, callNum: number) => {
        if (toolName === 'ask_user_question' || toolName === 'update_task_plan' || toolName === 'screenshot_viewport' || toolName === 'request_permission') return;
        this.showToolUseIndicator(toolName, callNum);
      }),
      // ask_user_question MCP tool — render an interactive multiple-choice
      // card. The backend blocks the model on a channel until the user
      // submits, at which point AnswerAssistantQuestion routes the answer
      // back as the tool's JSON result.
      on('assistant:question', (payload: AssistantQuestionPayload) => {
        this.showQuestion(payload);
      }),
      // request_permission / fetch_url self-gate — render an Allow/Deny card.
      // The backend blocks the tool on a channel until AnswerToolPermission
      // routes the decision back.
      on('assistant:permission-request', (payload: AssistantPermissionRequest) => {
        this.showPermission(payload);
      }),
      // screenshot_viewport MCP tool — capture the live viewport and hand
      // the PNG back to the blocked tool handler. captureScreenshot is
      // optional (the test harness wires no viewer); fail explicitly so
      // the model gets a clear tool error instead of hanging.
      on('assistant:screenshot-request', async (payload: AssistantScreenshotRequest) => {
        if (!payload?.id) return;
        if (!this.captureScreenshot) {
          try {
            await DeliverViewportScreenshot(payload.id, '', 'no viewport available');
          } catch (e) {
            console.warn('DeliverViewportScreenshot failed:', e);
          }
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
      }),
      // update_task_plan MCP tool — render or update the task list. One-way;
      // each call REPLACES the rendered list (the model sends full state).
      on('assistant:task-plan', (payload: AssistantTaskPlanPayload) => {
        this.renderTaskPlan(payload?.tasks ?? []);
      }),
      // MCP-driven code changes — update editor only, Go handles the build.
      // Reject if the active tab is read-only: the backend guards already
      // refuse read-only edits, but the event could be delivered out-of-band
      // (e.g. user switched to a read-only tab mid-run). Silent-overwrite of
      // a read-only file would corrupt its in-memory view.
      on('assistant:replace-code', (code: string) => {
        if (this.getActiveTab().readOnly) return;
        this.onSetEditorSilent(code);
      }),
      // MCP new_file tool — create a fresh editable tab with the given source.
      on('assistant:new-file', (payload: { name: string; code: string }) => {
        if (!payload) return;
        this.onNewFile(payload.name ?? 'Untitled', payload.code ?? '');
      }),
      // Thinking indicator — shown after tool results, before next assistant message
      on('assistant:thinking', (callNum: number) => {
        this.showThinkingIndicator(callNum);
      }),
    ];
  }

  private unregisterEvents(): void {
    for (const off of this.offs) off();
    this.offs = [];
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

  // showIndicator renders the single-line "<prefix><action>... (Ns)" status
  // bubble used for both tool-use and thinking states. It finalizes any
  // in-progress text so it becomes its own bubble, replaces any previous
  // indicator, and ticks the elapsed counter once a second.
  private showIndicator(prefix: string, action: string, startTime: number): void {
    this.finalizeCurrentMessage();
    this.removeToolUseIndicator();
    this.removeThinking();

    this.toolUseDiv = document.createElement('div');
    this.toolUseDiv.className = 'assistant-tool-use';
    this.messagesDiv.appendChild(this.toolUseDiv);

    const updateText = () => {
      if (!this.toolUseDiv) return;
      const elapsed = Math.floor((Date.now() - startTime) / 1000);
      this.toolUseDiv.textContent = `${prefix}${action}... (${elapsed}s)`;
    };
    updateText();
    this.toolUseTimer = setInterval(updateText, 1000);
    this.scrollToBottom();
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
    this.showIndicator(callNum ? `[${callNum}] ` : '', action, this.streamStartTime);
  }

  private showThinkingIndicator(callNum: number): void {
    this.showIndicator(`[${callNum}] `, 'Thinking', Date.now());
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

  // showQuestion renders an interactive card from the ask_user_question payload.
  // Questions are shown ONE AT A TIME — a multi-question ask would otherwise
  // stack into a tall card that scrolls its later questions out of view. Each
  // question gets 2-4 options plus an automatic "Other" free-text entry; Back /
  // Next step through them and Submit (on the last) routes every answer back via
  // AnswerAssistantQuestion. The model is blocked on a channel in the MCP layer
  // until then, at which point the card locks into a read-only summary.
  private showQuestion(payload: AssistantQuestionPayload): void {
    if (!payload?.questions?.length) return;
    this.pendingQuestion = payload;
    this.finalizeCurrentMessage();
    this.removeToolUseIndicator();
    this.removeThinking();

    const questions = payload.questions;
    const total = questions.length;

    const card = document.createElement('div');
    card.className = 'assistant-question-card';

    // Per-question UI state. `selected` is a Set so multiSelect questions can
    // toggle multiple labels; single-select questions just keep one.
    const state = questions.map(() => ({
      selected: new Set<string>(),
      otherText: '',
      otherActive: false,
    }));

    const progress = document.createElement('div');
    progress.className = 'assistant-question-progress';

    const body = document.createElement('div');
    body.className = 'assistant-question-body';

    const footer = document.createElement('div');
    footer.className = 'assistant-question-footer';
    const backBtn = document.createElement('button');
    backBtn.type = 'button';
    backBtn.className = 'assistant-question-back';
    backBtn.textContent = 'Back';
    const errMsg = document.createElement('span');
    errMsg.className = 'assistant-question-error';
    const nextBtn = document.createElement('button');
    nextBtn.type = 'button';
    nextBtn.className = 'assistant-question-submit';
    footer.append(backBtn, errMsg, nextBtn);

    card.append(progress, body, footer);
    this.messagesDiv.appendChild(card);

    let current = 0;
    const isLast = () => current === total - 1;

    const invalidReason = (qi: number): string | null => {
      if (state[qi].selected.size === 0) return 'Pick an option for';
      if (state[qi].selected.has('Other') && !state[qi].otherText.trim()) return 'Type a custom answer for';
      return null;
    };

    const submitAll = async () => {
      this.pendingQuestion = null; // answered — no longer pending for a driver
      const answers: Record<string, string> = {};
      const notes: Record<string, string> = {};
      questions.forEach((q, qi) => {
        // Replace 'Other' with the user's free text (so the model sees the actual
        // choice) and stash the original text in notes for reference.
        const finalLabels = Array.from(state[qi].selected).map(l => (l === 'Other' ? state[qi].otherText.trim() : l));
        answers[q.question] = finalLabels.join(', ');
        if (state[qi].otherActive && state[qi].otherText.trim()) notes[q.question] = state[qi].otherText.trim();
      });
      card.classList.add('answered');
      card.querySelectorAll('button, textarea').forEach(el => ((el as HTMLButtonElement | HTMLTextAreaElement).disabled = true));
      nextBtn.textContent = 'Sent';
      this.showThinking();
      try {
        await AnswerAssistantQuestion(payload.id, answers, notes);
      } catch (err: any) {
        this.showError(`Failed to send answer: ${err?.message || err}`);
      }
    };

    const advance = () => {
      const reason = invalidReason(current);
      if (reason) {
        const q = questions[current];
        errMsg.textContent = `${reason}: ${q.header || q.question}`;
        return;
      }
      errMsg.textContent = '';
      if (isLast()) { void submitAll(); return; }
      current++;
      renderCurrent();
    };

    const renderCurrent = () => {
      const q = questions[current];
      const st = state[current];
      body.innerHTML = '';
      errMsg.textContent = '';
      progress.textContent = total > 1 ? `Question ${current + 1} of ${total}` : '';
      progress.style.display = total > 1 ? 'block' : 'none';

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
      otherInput.value = st.otherText;
      otherInput.style.display = st.otherActive ? 'block' : 'none';
      otherInput.addEventListener('input', () => { st.otherText = otherInput.value; });

      const opts = document.createElement('div');
      opts.className = 'assistant-question-options';
      // Always append a synthetic "Other" so the user can supply free text even
      // when none of the model's options fit (matches the built-in convention).
      const allOptions: AssistantQuestionOption[] = [...q.options, { label: 'Other', description: 'Provide custom text' }];

      allOptions.forEach((opt) => {
        const optBtn = document.createElement('button');
        optBtn.type = 'button';
        optBtn.className = 'assistant-question-option';
        if (st.selected.has(opt.label)) optBtn.classList.add('selected');

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
            if (st.selected.has(opt.label)) { st.selected.delete(opt.label); optBtn.classList.remove('selected'); }
            else { st.selected.add(opt.label); optBtn.classList.add('selected'); }
          } else {
            st.selected.clear();
            st.selected.add(opt.label);
            opts.querySelectorAll('.assistant-question-option').forEach(b => b.classList.remove('selected'));
            optBtn.classList.add('selected');
          }
          if (isOther) {
            st.otherActive = st.selected.has('Other');
            otherInput.style.display = st.otherActive ? 'block' : 'none';
            if (st.otherActive) otherInput.focus();
          }
          errMsg.textContent = '';
        });

        opts.appendChild(optBtn);
      });

      block.appendChild(opts);
      block.appendChild(otherInput);
      body.appendChild(block);

      backBtn.style.display = current > 0 ? 'inline-block' : 'none';
      nextBtn.textContent = isLast() ? 'Submit' : 'Next';
      this.scrollToBottom();
    };

    backBtn.addEventListener('click', () => {
      if (current === 0) return;
      current--;
      renderCurrent();
    });
    nextBtn.addEventListener('click', advance);

    renderCurrent();
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

    if (!this.currentStreamDiv) {
      this.removeThinking();
      this.currentStreamDiv = this.addMessageDiv('assistant', '');
    }
    this.currentStreamText += token;
    this.currentStreamDiv.innerHTML = this.renderMarkdown(this.currentStreamText) + '<span class="streaming-cursor"></span>';
    this.scrollToBottom();
  }

  /** Finalize the current assistant message bubble so the next text starts a new one. */
  private finalizeCurrentMessage(): void {
    if (this.currentStreamDiv && this.currentStreamText) {
      this.currentStreamDiv.innerHTML = this.renderMarkdown(this.currentStreamText);
      this.addApplyButtons(this.currentStreamDiv);
    }
    this.currentStreamDiv = null;
    this.currentStreamText = '';
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
    const makeCopyBtn = (text: string, failLabel: string): HTMLButtonElement => {
      const copyBtn = document.createElement('button');
      copyBtn.className = 'assistant-copy-btn';
      copyBtn.textContent = 'Copy';
      copyBtn.addEventListener('click', () => {
        navigator.clipboard.writeText(text).then(() => {
          copyBtn.textContent = 'Copied';
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        }).catch(() => {
          copyBtn.textContent = failLabel;
          setTimeout(() => { copyBtn.textContent = 'Copy'; }, 1500);
        });
      });
      return copyBtn;
    };

    const makeApplyBtn = (apply: () => void): HTMLButtonElement => {
      const applyBtn = document.createElement('button');
      applyBtn.className = 'assistant-apply-btn';
      applyBtn.textContent = 'Apply';
      applyBtn.addEventListener('click', () => {
        apply();
        applyBtn.textContent = 'Applied';
        applyBtn.disabled = true;
        setTimeout(() => { applyBtn.textContent = 'Apply'; applyBtn.disabled = false; }, 1500);
      });
      return applyBtn;
    };

    // Handle regular fenced code blocks
    const codeBlocks = div.querySelectorAll('pre code');
    codeBlocks.forEach((block) => {
      const pre = block.parentElement;
      if (!pre || pre.closest('.assistant-edit-block')) return;
      const code = block.textContent || '';

      const btnGroup = document.createElement('div');
      btnGroup.className = 'assistant-code-btns';

      btnGroup.appendChild(makeCopyBtn(code, 'Error'));

      // Only show "Apply" for complete programs (contains Main function)
      if (/\bMain\s*\(/.test(code)) {
        btnGroup.appendChild(makeApplyBtn(() => this.onApplyCode(code)));
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

      btnGroup.appendChild(makeCopyBtn(replaceWith, 'Failed'));
      btnGroup.appendChild(makeApplyBtn(() => this.onApplyCode(replaceWith, searchFor)));

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
