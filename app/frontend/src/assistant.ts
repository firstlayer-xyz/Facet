import { EventsOn } from '../wailsjs/runtime/runtime';
import { SendAssistantMessage, CancelAssistant, ClearAssistantHistory, PickImageFile, DetectAssistantCLIs }
  from '../wailsjs/go/main/App';

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
  private onApplyCode: (newCode: string, searchFor?: string) => void;
  private onSetEditorSilent: (newCode: string) => void;
  private captureScreenshot: () => string;
  private offToken: (() => void) | null = null;
  private offDone: (() => void) | null = null;
  private offError: (() => void) | null = null;
  private offToolUse: (() => void) | null = null;
  private offReplaceCode: (() => void) | null = null;
  private offEditCode: (() => void) | null = null;
  private offThinking: (() => void) | null = null;

  constructor(
    container: HTMLElement,
    getEditorCode: () => string,
    getErrors: () => string,
    onApplyCode: (newCode: string, searchFor?: string) => void,
    onSetEditorSilent: (newCode: string) => void,
    captureScreenshot: () => string,
  ) {
    this.container = container;
    this.getEditorCode = getEditorCode;
    this.getErrors = getErrors;
    this.onApplyCode = onApplyCode;
    this.onSetEditorSilent = onSetEditorSilent;
    this.captureScreenshot = captureScreenshot;

    this.panel = document.createElement('div');
    this.panel.id = 'assistant-panel';
    this.panel.style.display = 'none';

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

    this.panel.appendChild(header);

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

  show(): void {
    this.visible = true;
    this.panel.style.display = 'flex';
    this.registerEvents();
    this.input.focus();
    this.checkForCLIs();
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
    const isWin = navigator.platform?.startsWith('Win') || navigator.userAgent.includes('Win');
    let installHtml = `<strong>No AI assistant found</strong><br><br>Install one of the supported CLIs to enable AI assistance:<br><br>`;
    installHtml += `<strong>Claude</strong> (recommended)<br>`;
    if (isMac) {
      installHtml += `<code>brew install claude-code</code><br>or <code>npm install -g @anthropic-ai/claude-code</code>`;
    } else if (isWin) {
      installHtml += `<code>npm install -g @anthropic-ai/claude-code</code>`;
    } else {
      installHtml += `<code>npm install -g @anthropic-ai/claude-code</code>`;
    }
    installHtml += `<br><br><strong>Ollama</strong> (local, free)<br>`;
    if (isMac) {
      installHtml += `<code>brew install ollama</code><br>then <code>ollama pull llama3</code>`;
    } else if (isWin) {
      installHtml += `Download from <strong>ollama.com</strong><br>then <code>ollama pull llama3</code>`;
    } else {
      installHtml += `<code>curl -fsSL https://ollama.com/install.sh | sh</code><br>then <code>ollama pull llama3</code>`;
    }
    installHtml += `<br><br><em>Restart Facet after installing.</em>`;
    this.noCLIBanner.innerHTML = installHtml;
    this.messagesDiv.insertBefore(this.noCLIBanner, this.messagesDiv.firstChild);
  }

  hide(): void {
    this.visible = false;
    this.panel.style.display = 'none';
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
    this.offToken = EventsOn('assistant:token', (token: string) => {
      this.appendToken(token);
    });
    this.offDone = EventsOn('assistant:done', () => {
      this.finishStream();
    });
    this.offError = EventsOn('assistant:error', (msg: string) => {
      this.showError(msg);
    });
    // MCP tool-use indicator
    this.offToolUse = EventsOn('assistant:tool-use', (toolName: string, callNum: number) => {
      this.showToolUseIndicator(toolName, callNum);
    });
    // MCP-driven code changes — update editor only, Go handles the build
    this.offReplaceCode = EventsOn('assistant:replace-code', (code: string) => {
      this.onSetEditorSilent(code);
    });
    this.offEditCode = EventsOn('assistant:edit-code', (data: { search: string; replace: string }) => {
      this.onSetEditorSilent(data.replace);
    });
    // Thinking indicator — shown after tool results, before next assistant message
    this.offThinking = EventsOn('assistant:thinking', (callNum: number) => {
      this.showThinkingIndicator(callNum);
    });
  }

  private unregisterEvents(): void {
    if (this.offToken) { this.offToken(); this.offToken = null; }
    if (this.offDone) { this.offDone(); this.offDone = null; }
    if (this.offError) { this.offError(); this.offError = null; }
    if (this.offToolUse) { this.offToolUse(); this.offToolUse = null; }
    if (this.offReplaceCode) { this.offReplaceCode(); this.offReplaceCode = null; }
    if (this.offEditCode) { this.offEditCode(); this.offEditCode = null; }
    if (this.offThinking) { this.offThinking(); this.offThinking = null; }
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
      await SendAssistantMessage(text, this.getEditorCode(), this.getErrors(), images);
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
    this.streaming = false;
    this.receivedFirstToken = false;
    this.sendBtn.textContent = 'Send';
    this.sendBtn.classList.remove('assistant-send-btn-stop');
    this.removeThinking();
    this.removeToolUseIndicator();
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

  private escapeHtml(text: string): string {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/\n/g, '<br>');
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
        const escapedReplace = seg.replace
          .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        return `<div class="assistant-edit-block" data-search="${encodeURIComponent(seg.search)}" data-replace="${encodeURIComponent(seg.replace)}">` +
          `<div class="edit-block-label">Edit</div>` +
          `<pre><code class="language-facet">${escapedReplace}</code></pre>` +
          `</div>`;
      }
      if (seg.type === 'code') {
        const escapedCode = seg.code
          .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        const cls = seg.lang ? ` class="language-${seg.lang}"` : '';
        return `<pre><code${cls}>${escapedCode}</code></pre>`;
      }
      // Plain text: escape, then apply inline markdown
      let html = seg.content
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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
