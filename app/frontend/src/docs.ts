export interface DocEntry {
  name: string;
  signature: string;
  doc: string;
  kind: string;
  library: string;
}

export interface DocGuide {
  title: string;
  slug: string;
  markdown: string;
}

type DocsView = 'guides' | 'api';

// Group entries by display section
function groupEntries(entries: DocEntry[]): [string, DocEntry[]][] {
  const groups: [string, DocEntry[]][] = [
    ['Types', []],
    ['Functions', []],
    ['Solid Methods', []],
    ['Sketch Methods', []],
    ['Thread Methods', []],
    ['String Methods', []],
    ['Keywords', []],
  ];
  const map = new Map(groups);

  // Track library groups separately so they appear after builtins
  const libGroups: [string, DocEntry[]][] = [];
  const libMap = new Map<string, DocEntry[]>();

  for (const e of entries) {
    if (e.kind === 'type') {
      map.get('Types')!.push(e);
    } else if (e.kind === 'keyword') {
      map.get('Keywords')!.push(e);
    } else if (e.library && e.library !== 'facet/std') {
      // Group by library name
      let bucket = libMap.get(e.library);
      if (!bucket) {
        bucket = [];
        libMap.set(e.library, bucket);
        libGroups.push([e.library, bucket]);
      }
      bucket.push(e);
    } else if (e.kind === 'method') {
      if (e.name.startsWith('Solid.')) {
        map.get('Solid Methods')!.push(e);
      } else if (e.name.startsWith('Sketch.')) {
        map.get('Sketch Methods')!.push(e);
      } else if (e.name.startsWith('String.')) {
        map.get('String Methods')!.push(e);
      } else {
        const dot = e.name.indexOf('.');
        const receiver = dot > 0 ? e.name.substring(0, dot) + ' Methods' : 'Methods';
        let bucket = map.get(receiver);
        if (!bucket) {
          bucket = [];
          map.set(receiver, bucket);
          groups.push([receiver, bucket]);
        }
        bucket.push(e);
      }
    } else if (e.kind === 'function') {
      map.get('Functions')!.push(e);
    }
  }

  // Append library groups after builtins
  const result = groups.filter(([, items]) => items.length > 0);
  for (const [label, items] of libGroups) {
    if (items.length > 0) result.push([label, items]);
  }
  return result;
}

// Split markdown into top-level sections at H1 and H2 headings.
// H3+ remain part of the enclosing section body.
function splitSections(markdown: string): Array<{ heading: string; level: number; body: string }> {
  const sections: Array<{ heading: string; level: number; body: string }> = [];
  let heading = '';
  let level = 0;
  let lines: string[] = [];

  for (const line of markdown.split('\n')) {
    const m = line.match(/^(#{1,2})\s+(.+)$/);
    if (m) {
      if (heading || lines.length > 0) {
        sections.push({ heading, level, body: lines.join('\n').trim() });
      }
      heading = m[2];
      level = m[1].length;
      lines = [];
    } else {
      lines.push(line);
    }
  }
  if (heading || lines.length > 0) {
    sections.push({ heading, level, body: lines.join('\n').trim() });
  }
  return sections;
}

// Extract the first descriptive paragraph as plain text for a card excerpt.
function extractExcerpt(markdown: string, maxLen = 130): string {
  for (const line of markdown.split('\n')) {
    const t = line.trim();
    if (!t || t.startsWith('#') || t.startsWith('```') || t.startsWith('|') || t.startsWith('-')) continue;
    const plain = t
      .replace(/`([^`]+)`/g, '$1')
      .replace(/\*\*([^*]+)\*\*/g, '$1')
      .replace(/\*([^*]+)\*/g, '$1');
    if (plain.length > 20) return plain.length > maxLen ? plain.slice(0, maxLen) + '…' : plain;
  }
  return '';
}

// Render markdown to HTML — supports headings, code blocks, inline code, bold, italic, links, lists, tables, paragraphs
function renderMarkdown(text: string): string {
  // Escape HTML
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Fenced code blocks: ```...```
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_m, _lang, code) => {
    return `<div class="code-block"><button class="copy-btn" title="Copy to clipboard">Copy</button><pre><code>${code.trimEnd()}</code></pre></div>`;
  });

  // Split into lines for block-level processing
  const lines = html.split('\n');
  const result: string[] = [];
  let inTable = false;
  let inList = false;
  let listType: 'ul' | 'ol' = 'ul';

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // Skip lines inside <pre> blocks
    if (line.includes('<pre>')) {
      let j = i;
      while (j < lines.length && !lines[j].includes('</pre>')) j++;
      result.push(lines.slice(i, j + 1).join('\n'));
      i = j;
      continue;
    }

    // Table rows
    if (line.match(/^\|(.+)\|$/)) {
      if (i + 1 < lines.length && lines[i + 1].match(/^\|[\s\-:|]+\|$/)) {
        if (!inTable) {
          result.push('<table>');
          inTable = true;

        }
        const cells = line.split('|').filter(c => c.trim() !== '');
        result.push('<tr>' + cells.map(c => `<th>${inlineMarkdown(c.trim())}</th>`).join('') + '</tr>');
        i++;
        continue;
      } else if (inTable) {
        const cells = line.split('|').filter(c => c.trim() !== '');
        result.push('<tr>' + cells.map(c => `<td>${inlineMarkdown(c.trim())}</td>`).join('') + '</tr>');
        continue;
      }
    } else if (inTable) {
      result.push('</table>');
      inTable = false;
    }

    // Close list if not a list item
    if (inList && !line.match(/^(\s*[-*]\s|  \d+\.\s)/)) {
      result.push(listType === 'ul' ? '</ul>' : '</ol>');
      inList = false;
    }

    // Headings
    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      result.push(`<h${level}>${inlineMarkdown(headingMatch[2])}</h${level}>`);
      continue;
    }

    // Unordered list items
    const ulMatch = line.match(/^\s*[-*]\s+(.+)$/);
    if (ulMatch) {
      if (!inList || listType !== 'ul') {
        if (inList) result.push(listType === 'ul' ? '</ul>' : '</ol>');
        result.push('<ul>');
        inList = true;
        listType = 'ul';
      }
      result.push(`<li>${inlineMarkdown(ulMatch[1])}</li>`);
      continue;
    }

    // Ordered list items
    const olMatch = line.match(/^\s*\d+\.\s+(.+)$/);
    if (olMatch) {
      if (!inList || listType !== 'ol') {
        if (inList) result.push(listType === 'ul' ? '</ul>' : '</ol>');
        result.push('<ol>');
        inList = true;
        listType = 'ol';
      }
      result.push(`<li>${inlineMarkdown(olMatch[1])}</li>`);
      continue;
    }

    // Empty lines = paragraph break
    if (line.trim() === '') {
      result.push('');
      continue;
    }

    // Regular paragraph text
    result.push(`<p>${inlineMarkdown(line)}</p>`);
  }

  if (inTable) result.push('</table>');
  if (inList) result.push(listType === 'ul' ? '</ul>' : '</ol>');

  return result.join('\n');
}

// Attach copy-to-clipboard handlers to all .copy-btn buttons inside el
function attachCopyButtons(el: HTMLElement): void {
  for (const btn of el.querySelectorAll<HTMLButtonElement>('.copy-btn')) {
    btn.addEventListener('click', () => {
      const pre = btn.nextElementSibling as HTMLElement | null;
      const code = pre?.querySelector('code')?.textContent ?? '';
      navigator.clipboard.writeText(code).then(() => {
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
          btn.textContent = 'Copy';
          btn.classList.remove('copied');
        }, 2000);
      }).catch(() => {
        btn.textContent = 'Error';
        setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
      });
    });
  }
}

// Show type-detail tooltip on hover for inline <code> type references
function attachTypeHovers(el: HTMLElement, entries: DocEntry[]): void {
  const typeMap = new Map<string, DocEntry>();
  for (const e of entries) {
    if (e.kind === 'type') typeMap.set(e.name, e);
  }
  if (typeMap.size === 0) return;

  let tooltip: HTMLElement | null = null;

  function showTooltip(anchor: HTMLElement, entry: DocEntry) {
    hideTooltip();
    tooltip = document.createElement('div');
    tooltip.className = 'type-tooltip';

    const name = document.createElement('div');
    name.className = 'type-tooltip-name';
    name.textContent = entry.name;
    tooltip.appendChild(name);

    if (entry.doc) {
      const doc = document.createElement('div');
      doc.className = 'type-tooltip-doc';
      doc.textContent = entry.doc;
      tooltip.appendChild(doc);
    }

    document.body.appendChild(tooltip);

    const rect = anchor.getBoundingClientRect();
    const ttRect = tooltip.getBoundingClientRect();
    let top = rect.bottom + 6;
    let left = rect.left;
    if (left + ttRect.width > window.innerWidth - 8) left = window.innerWidth - ttRect.width - 8;
    if (top + ttRect.height > window.innerHeight - 8) top = rect.top - ttRect.height - 6;
    tooltip.style.top = top + 'px';
    tooltip.style.left = left + 'px';
  }

  function hideTooltip() {
    if (tooltip) { tooltip.remove(); tooltip = null; }
  }

  for (const code of el.querySelectorAll<HTMLElement>('code')) {
    if (code.closest('pre')) continue;
    const entry = typeMap.get(code.textContent?.trim() ?? '');
    if (!entry) continue;
    code.classList.add('type-ref');
    code.addEventListener('mouseenter', () => showTooltip(code, entry));
    code.addEventListener('mouseleave', hideTooltip);
  }
}

// Inline markdown: backticks, bold, italic, links
function isSafeURL(url: string): boolean {
  const trimmed = url.trim().toLowerCase();
  return !trimmed.startsWith('javascript:') && !trimmed.startsWith('data:');
}

function inlineMarkdown(text: string): string {
  return text
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, label, url) =>
      isSafeURL(url) ? `<a href="${url}">${label}</a>` : label);
}

export class DocsPanel {
  private el: HTMLElement | null = null;
  private container: HTMLElement;
  private onClose: (() => void) | null;
  private currentView: DocsView = 'guides';
  private guides: DocGuide[] = [];
  private entries: DocEntry[] = [];
  private currentGuide: DocGuide | null = null;
  private contentEl: HTMLElement | null = null;
  private searchEl: HTMLInputElement | null = null;

  constructor(container: HTMLElement, onClose?: () => void) {
    this.container = container;
    this.onClose = onClose ?? null;
  }

  show(entries: DocEntry[], guides?: DocGuide[]): void {
    this.hide();
    this.entries = entries;
    if (guides) this.guides = guides;
    this.currentView = 'guides';
    this.currentGuide = null;

    const panel = document.createElement('div');
    panel.id = 'docs-panel';

    // Navigation tabs
    const nav = document.createElement('div');
    nav.className = 'docs-nav';

    const guidesTab = document.createElement('button');
    guidesTab.className = 'docs-tab active';
    guidesTab.textContent = 'Guides';
    guidesTab.onclick = () => this.switchView('guides', panel);

    const apiTab = document.createElement('button');
    apiTab.className = 'docs-tab';
    apiTab.textContent = 'API Reference';
    apiTab.onclick = () => this.switchView('api', panel);

    nav.appendChild(guidesTab);
    nav.appendChild(apiTab);

    if (this.onClose) {
      const closeBtn = document.createElement('button');
      closeBtn.className = 'docs-close';
      closeBtn.innerHTML = '&times;';
      closeBtn.title = 'Close docs';
      closeBtn.onclick = () => this.onClose?.();
      nav.appendChild(closeBtn);
    }

    panel.appendChild(nav);

    // Search input
    const search = document.createElement('input');
    search.className = 'docs-search';
    search.type = 'text';
    search.placeholder = 'Search docs...';
    panel.appendChild(search);
    this.searchEl = search;

    // Content container
    const content = document.createElement('div');
    content.className = 'docs-content';
    panel.appendChild(content);
    this.contentEl = content;

    search.addEventListener('input', () => this.rerender());
    this.rerender();

    this.el = panel;
    this.container.appendChild(panel);
    this.container.classList.add('docs-open');
    search.focus();
  }

  private rerender(): void {
    if (!this.contentEl) return;
    this.contentEl.innerHTML = '';
    const filter = this.searchEl?.value ?? '';
    if (this.currentView === 'guides') {
      this.renderGuides(this.contentEl, filter);
    } else {
      this.renderAPI(this.contentEl, filter);
    }
  }

  private switchView(view: DocsView, panel: HTMLElement): void {
    this.currentView = view;
    this.currentGuide = null;
    panel.querySelectorAll('.docs-tab').forEach((tab, i) => {
      tab.classList.toggle('active', (i === 0 && view === 'guides') || (i === 1 && view === 'api'));
    });
    this.rerender();
  }

  private renderGuides(content: HTMLElement, filter: string): void {
    if (this.guides.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'docs-empty';
      empty.textContent = 'No guides available.';
      content.appendChild(empty);
      return;
    }

    // With a filter: show all matching content inline
    if (filter) {
      const lf = filter.toLowerCase();
      for (const guide of this.guides) {
        if (!guide.title.toLowerCase().includes(lf) && !guide.markdown.toLowerCase().includes(lf)) continue;
        const section = document.createElement('div');
        section.className = 'docs-guide';
        section.innerHTML = renderMarkdown(guide.markdown);
        attachCopyButtons(section);
        attachTypeHovers(section, this.entries);
        content.appendChild(section);
      }
      if (content.children.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'docs-empty';
        empty.textContent = 'No matching guides.';
        content.appendChild(empty);
      }
      return;
    }

    // No filter: guide list or single guide
    // Auto-open User's Guide by default
    if (!this.currentGuide && this.guides.length > 0) {
      const usersGuide = this.guides.find(g => g.slug === 'users-guide');
      if (usersGuide) {
        this.currentGuide = usersGuide;
      }
    }
    if (!this.currentGuide) {
      this.renderGuideList(content);
    } else {
      this.renderGuideSingle(content, this.currentGuide);
    }
  }

  private renderGuideList(content: HTMLElement): void {
    for (const guide of this.guides) {
      const card = document.createElement('div');
      card.className = 'guide-card';

      const title = document.createElement('div');
      title.className = 'guide-card-title';
      title.textContent = guide.title;
      card.appendChild(title);

      const excerpt = extractExcerpt(guide.markdown);
      if (excerpt) {
        const desc = document.createElement('div');
        desc.className = 'guide-card-desc';
        desc.textContent = excerpt;
        card.appendChild(desc);
      }

      const open = () => { this.currentGuide = guide; this.rerender(); };
      card.addEventListener('click', open);
      content.appendChild(card);
    }
  }

  private renderGuideSingle(content: HTMLElement, guide: DocGuide): void {
    // Back button
    const back = document.createElement('button');
    back.className = 'guide-back-btn';
    back.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg> Guides`;
    back.addEventListener('click', () => { this.currentGuide = null; this.rerender(); });
    content.appendChild(back);

    const sections = splitSections(guide.markdown);

    for (const { heading, level, body } of sections) {
      if (level === 1) {
        // Guide title
        const h = document.createElement('h1');
        h.className = 'guide-title';
        h.textContent = heading;
        content.appendChild(h);

        if (body) {
          const intro = document.createElement('div');
          intro.className = 'docs-guide';
          intro.innerHTML = renderMarkdown(body);
          attachCopyButtons(intro);
          attachTypeHovers(intro, this.entries);
          content.appendChild(intro);
        }
      } else if (level === 2) {
        const details = document.createElement('details');
        details.className = 'guide-section';

        const summary = document.createElement('summary');
        summary.className = 'guide-section-summary';
        summary.textContent = heading;
        details.appendChild(summary);

        if (body) {
          const bodyEl = document.createElement('div');
          bodyEl.className = 'docs-guide guide-section-body';
          bodyEl.innerHTML = renderMarkdown(body);
          attachCopyButtons(bodyEl);
          attachTypeHovers(bodyEl, this.entries);
          details.appendChild(bodyEl);
        }

        content.appendChild(details);
      }
    }
  }

  private renderAPI(content: HTMLElement, filter: string): void {
    const lf = filter.toLowerCase();
    const filtered = filter
      ? this.entries.filter(e =>
          e.name.toLowerCase().includes(lf) ||
          e.signature.toLowerCase().includes(lf) ||
          e.doc.toLowerCase().includes(lf) ||
          (e.library && e.library.toLowerCase().includes(lf)))
      : this.entries;

    const groups = groupEntries(filtered);
    for (const [label, items] of groups) {
      const section = document.createElement('div');
      section.className = 'docs-section';

      const h2 = document.createElement('h2');
      h2.textContent = label;
      section.appendChild(h2);

      for (const item of items) {
        const card = document.createElement('div');
        card.className = 'docs-entry';
        card.dataset.name = item.name;

        const sig = document.createElement('pre');
        sig.className = 'docs-signature';
        const code = document.createElement('code');
        code.textContent = item.signature || item.name;
        sig.appendChild(code);
        card.appendChild(sig);

        if (item.doc) {
          const body = document.createElement('div');
          body.className = 'docs-body';
          body.innerHTML = renderMarkdown(item.doc);
          attachCopyButtons(body);
          card.appendChild(body);
        }

        if (item.library) {
          const meta = document.createElement('div');
          meta.className = 'docs-meta';
          meta.textContent = item.library;
          card.appendChild(meta);
        }

        section.appendChild(card);
      }

      content.appendChild(section);
    }

    if (groups.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'docs-empty';
      empty.textContent = 'No matching entries.';
      content.appendChild(empty);
    }
  }

  focusEntry(name: string): void {
    // Switch to API view and clear any filter
    this.currentView = 'api';
    this.currentGuide = null;
    if (this.searchEl) this.searchEl.value = '';
    if (this.el) {
      this.el.querySelectorAll('.docs-tab').forEach((tab, i) => {
        tab.classList.toggle('active', i === 1); // API tab is second
      });
    }
    this.rerender();

    if (!this.contentEl) return;
    const target = this.contentEl.querySelector<HTMLElement>(`[data-name="${CSS.escape(name)}"]`);
    if (!target) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    target.classList.add('docs-entry-focused');
    setTimeout(() => target.classList.remove('docs-entry-focused'), 1500);
  }

  hide(): void {
    if (this.el) {
      this.el.remove();
      this.el = null;
      this.contentEl = null;
      this.searchEl = null;
      this.container.classList.remove('docs-open');
    }
  }

  isVisible(): boolean {
    return this.el !== null;
  }
}
