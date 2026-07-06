export interface DocEntry {
  name: string;
  signature: string;
  doc: string;
  kind: string;
  library: string;
  section?: string;
}

export interface DocGuide {
  title: string;
  slug: string;
  markdown: string;
}

type DocsView = string;
type NavItem = { label: string; action: () => void; sectionId?: string };

const USER_LIBRARY = '__user__';
const VIEW_GUIDES = 'guides';
const VIEW_API = 'api';
const VIEW_USER_CODE = 'user-code';

// Returns the last path component of a library namespace, capitalized.
// e.g. "facet/gears" → "Gears", "github.com/user/repo/threads" → "Threads"
function libDisplayName(library: string): string {
  const last = library.split('/').at(-1) ?? library;
  return last.charAt(0).toUpperCase() + last.slice(1);
}

// Returns unique non-stdlib, non-user library names found in entries, in first-seen order.
function distinctLibraries(entries: DocEntry[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const e of entries) {
    if (e.library && e.library !== USER_LIBRARY && !seen.has(e.library)) {
      seen.add(e.library);
      result.push(e.library);
    }
  }
  return result;
}

// Group entries by kind/section only, ignoring library — used for single-source views
// (Your Code tab, library tabs) so entries don't get re-bucketed under a library heading.
function groupEntriesFlat(entries: DocEntry[]): [string, DocEntry[]][] {
  const typesBucket: DocEntry[] = [];
  const keywordsBucket: DocEntry[] = [];
  const fnSectionOrder: string[] = [];
  const fnSections = new Map<string, DocEntry[]>();
  const methodOrder: string[] = [];
  const methodBuckets = new Map<string, DocEntry[]>();

  for (const e of entries) {
    if (e.kind === 'type') {
      typesBucket.push(e);
    } else if (e.kind === 'keyword') {
      keywordsBucket.push(e);
    } else if (e.kind === 'method') {
      const dot = e.name.indexOf('.');
      const receiver = dot > 0 ? e.name.substring(0, dot) + ' Methods' : 'Methods';
      if (!methodBuckets.has(receiver)) {
        methodOrder.push(receiver);
        methodBuckets.set(receiver, []);
      }
      methodBuckets.get(receiver)!.push(e);
    } else if (e.kind === 'function') {
      const key = e.section || 'Functions';
      if (!fnSections.has(key)) {
        fnSectionOrder.push(key);
        fnSections.set(key, []);
      }
      fnSections.get(key)!.push(e);
    }
  }

  const result: [string, DocEntry[]][] = [];
  if (typesBucket.length > 0) result.push(['Types', typesBucket]);
  for (const sec of fnSectionOrder) result.push([sec, fnSections.get(sec)!]);
  for (const recv of methodOrder) result.push([recv, methodBuckets.get(recv)!]);
  if (keywordsBucket.length > 0) result.push(['Keywords', keywordsBucket]);
  return result;
}

// Group entries by display section, preserving source order for stdlib functions.
// Stdlib functions are subdivided by their section field (e.g. "3D Constructors").
// Methods are grouped by receiver type. Library entries get one bucket each.
// User-code entries (library === USER_LIBRARY) appear in a "Your Code" section.
function groupEntries(entries: DocEntry[]): [string, DocEntry[]][] {
  const typesBucket: DocEntry[] = [];
  const keywordsBucket: DocEntry[] = [];
  const userCodeBucket: DocEntry[] = [];

  // Stdlib functions: section → entries, preserving first-seen order
  const fnSectionOrder: string[] = [];
  const fnSections = new Map<string, DocEntry[]>();

  // Methods: receiver → entries
  const methodOrder: string[] = [];
  const methodBuckets = new Map<string, DocEntry[]>();

  // External library entries: library → entries
  const libOrder: string[] = [];
  const libBuckets = new Map<string, DocEntry[]>();

  for (const e of entries) {
    if (e.library === USER_LIBRARY) {
      userCodeBucket.push(e);
    } else if (e.kind === 'type') {
      typesBucket.push(e);
    } else if (e.kind === 'keyword') {
      keywordsBucket.push(e);
    } else if (e.library && e.library !== 'facet/std') {
      if (!libBuckets.has(e.library)) {
        libOrder.push(e.library);
        libBuckets.set(e.library, []);
      }
      libBuckets.get(e.library)!.push(e);
    } else if (e.kind === 'method') {
      const dot = e.name.indexOf('.');
      const receiver = dot > 0 ? e.name.substring(0, dot) + ' Methods' : 'Methods';
      if (!methodBuckets.has(receiver)) {
        methodOrder.push(receiver);
        methodBuckets.set(receiver, []);
      }
      methodBuckets.get(receiver)!.push(e);
    } else if (e.kind === 'function') {
      const key = e.section || 'Functions';
      if (!fnSections.has(key)) {
        fnSectionOrder.push(key);
        fnSections.set(key, []);
      }
      fnSections.get(key)!.push(e);
    }
    // 'field' entries are omitted from the API panel
  }

  const result: [string, DocEntry[]][] = [];
  if (typesBucket.length > 0) result.push(['Types', typesBucket]);
  for (const sec of fnSectionOrder) result.push([sec, fnSections.get(sec)!]);
  for (const recv of methodOrder) result.push([recv, methodBuckets.get(recv)!]);
  if (keywordsBucket.length > 0) result.push(['Keywords', keywordsBucket]);
  if (userCodeBucket.length > 0) result.push(['Your Code', userCodeBucket]);
  for (const lib of libOrder) result.push([lib, libBuckets.get(lib)!]);
  return result;
}

// Split markdown into top-level sections at H1 and H2 headings.
// H3+ remain part of the enclosing section body.
function splitSections(markdown: string): Array<{ heading: string; level: number; body: string }> {
  const sections: Array<{ heading: string; level: number; body: string }> = [];
  let heading = '';
  let level = 0;
  let lines: string[] = [];
  let inFence = false;

  for (const line of markdown.split('\n')) {
    if (line.startsWith('```')) inFence = !inFence;
    const m = !inFence && line.match(/^(#{1,2})\s+(.+)$/);
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

// A URL may go into an href only if it is scheme-relative/relative or uses an
// allowlisted scheme. Denylisting schemes (the old javascript:/data: check) is
// unsafe: vbscript:, blob:, and whitespace-obfuscated variants like
// "java\tscript:" — which browsers resolve by ignoring control characters —
// slip through. Strip control and space characters before testing the scheme so
// obfuscation can't hide it, then require http/https/mailto or no scheme at all.
function isSafeURL(url: string): boolean {
  const cleaned = url.replace(/[\u0000-\u0020]/g, '').toLowerCase();
  if (/^(?:https?|mailto):/.test(cleaned)) return true;
  return !/^[a-z][a-z0-9+.-]*:/.test(cleaned); // relative, absolute-path, or #anchor
}

// Escape a value for interpolation inside a double-quoted attribute. renderMarkdown
// has already entity-escaped & < > in the surrounding text; the quote characters
// it leaves untouched are what an attacker uses to break out of href="...".
function escapeAttr(value: string): string {
  return value.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function inlineMarkdown(text: string): string {
  return text
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, label, url) =>
      isSafeURL(url) ? `<a href="${escapeAttr(url)}">${label}</a>` : label);
}

// Convert a label string to a CSS-safe element id fragment.
function toId(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

// Walk all text nodes inside el and wrap every occurrence of term in <mark>.
// Skips nodes inside <pre> so code block content is not altered.
function highlightText(el: HTMLElement, term: string): void {
  if (!term) return;
  const lf = term.toLowerCase();
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      return node.parentElement?.closest('pre')
        ? NodeFilter.FILTER_REJECT
        : NodeFilter.FILTER_ACCEPT;
    },
  });
  const nodes: Text[] = [];
  let node: Node | null;
  while ((node = walker.nextNode())) nodes.push(node as Text);

  for (const textNode of nodes) {
    const text = textNode.textContent ?? '';
    const lower = text.toLowerCase();
    if (!lower.includes(lf)) continue;

    const frag = document.createDocumentFragment();
    let last = 0;
    let idx = lower.indexOf(lf);
    while (idx !== -1) {
      if (idx > last) frag.appendChild(document.createTextNode(text.slice(last, idx)));
      const mark = document.createElement('mark');
      mark.className = 'search-highlight';
      mark.textContent = text.slice(idx, idx + term.length);
      frag.appendChild(mark);
      last = idx + term.length;
      idx = lower.indexOf(lf, last);
    }
    if (last < text.length) frag.appendChild(document.createTextNode(text.slice(last)));
    textNode.parentNode?.replaceChild(frag, textNode);
  }
}

export class DocsPanel {
  // Permanent panel element — always in the DOM, visibility via .open class.
  private el: HTMLElement;
  private onClose: (() => void) | null;
  private currentView: DocsView = VIEW_GUIDES;
  private guides: DocGuide[] = [];
  private entries: DocEntry[] = [];
  private currentGuide: DocGuide | null = null;
  private contentEl: HTMLElement | null = null;
  private searchEl: HTMLInputElement | null = null;
  private countEl: HTMLElement | null = null;
  private sideNavEl: HTMLElement | null = null;
  private sideNavOpen = true;
  private scrollCleanup: (() => void) | null = null;

  constructor(container: HTMLElement, onClose?: () => void) {
    this.onClose = onClose ?? null;
    this.el = document.createElement('div');
    this.el.id = 'docs-panel';
    container.appendChild(this.el);
  }

  show(entries: DocEntry[], guides?: DocGuide[]): void {
    this.hide();
    this.entries = entries;
    if (guides) this.guides = guides;
    this.currentView = VIEW_GUIDES;
    this.currentGuide = null;

    this.el.innerHTML = '';
    this.el.classList.add('open');

    const panel = this.el;

    // Navigation tabs
    const nav = document.createElement('div');
    nav.className = 'docs-nav';

    const toggleBtn = document.createElement('button');
    toggleBtn.className = 'docs-sidenav-toggle';
    toggleBtn.title = 'Toggle sidebar';
    toggleBtn.innerHTML = `<svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="1" y="1" width="12" height="12" rx="1.5"/><line x1="5" y1="1.5" x2="5" y2="12.5"/></svg>`;
    toggleBtn.onclick = () => this.toggleSideNav();
    nav.appendChild(toggleBtn);

    // Scrollable tab strip with overflow fade indicators
    const tabsWrap = document.createElement('div');
    tabsWrap.className = 'docs-tabs-wrap';

    const tabsScroll = document.createElement('div');
    tabsScroll.className = 'docs-tabs-scroll';

    const updateOverflow = () => {
      const sl = tabsScroll.scrollLeft;
      const max = tabsScroll.scrollWidth - tabsScroll.clientWidth;
      tabsWrap.classList.toggle('scroll-left', sl > 1);
      tabsWrap.classList.toggle('scroll-right', sl < max - 1);
    };
    tabsScroll.addEventListener('scroll', updateOverflow, { passive: true });
    new ResizeObserver(updateOverflow).observe(tabsScroll);

    const addTab = (view: DocsView, label: string) => {
      const tab = document.createElement('button');
      tab.className = 'docs-tab';
      tab.dataset.view = view;
      if (view === this.currentView) tab.classList.add('active');
      tab.textContent = label;
      tab.onclick = () => this.switchView(view);
      tabsScroll.appendChild(tab);
    };

    addTab(VIEW_GUIDES, 'Guides');
    addTab(VIEW_API, 'API Reference');
    if (entries.some(e => e.library === USER_LIBRARY)) {
      addTab(VIEW_USER_CODE, 'Your Code');
    }
    for (const lib of distinctLibraries(entries)) {
      addTab('lib:' + lib, libDisplayName(lib));
    }

    tabsWrap.appendChild(tabsScroll);
    nav.appendChild(tabsWrap);

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

    // Entry count bar (hidden in guides view)
    const count = document.createElement('div');
    count.className = 'docs-count';
    panel.appendChild(count);
    this.countEl = count;

    // Side nav + scrollable content row
    const bodyRow = document.createElement('div');
    bodyRow.className = 'docs-body-row';

    const sideNav = document.createElement('div');
    sideNav.className = 'docs-sidenav';
    if (!this.sideNavOpen) sideNav.classList.add('collapsed');
    bodyRow.appendChild(sideNav);
    this.sideNavEl = sideNav;

    const content = document.createElement('div');
    content.className = 'docs-content';
    bodyRow.appendChild(content);
    this.contentEl = content;

    panel.appendChild(bodyRow);

    search.addEventListener('input', () => this.rerender());
    this.rerender();
    search.focus();
  }

  private toggleSideNav(): void {
    this.sideNavOpen = !this.sideNavOpen;
    if (this.sideNavEl) this.sideNavEl.classList.toggle('collapsed', !this.sideNavOpen);
  }

  private renderSideNav(items: NavItem[]): void {
    if (!this.sideNavEl) return;
    this.sideNavEl.innerHTML = '';
    for (const { label, action, sectionId } of items) {
      const item = document.createElement('div');
      item.className = 'docs-sidenav-item';
      item.textContent = label;
      if (sectionId) item.dataset.sectionId = sectionId;
      item.addEventListener('click', action);
      this.sideNavEl.appendChild(item);
    }
  }

  private setupSectionObserver(): void {
    this.scrollCleanup?.();
    this.scrollCleanup = null;

    if (!this.contentEl || !this.sideNavEl) return;

    const content = this.contentEl;
    const sidenav = this.sideNavEl;

    const updateActive = () => {
      const sections = content.querySelectorAll<HTMLElement>('[id^="guide-section-"], [id^="api-"]');
      if (sections.length === 0) return;

      const contentTop = content.getBoundingClientRect().top;
      let activeId: string | null = null;
      for (const section of sections) {
        if (section.getBoundingClientRect().top - contentTop <= 20) activeId = section.id;
      }
      if (!activeId) activeId = sections[0].id;

      sidenav.querySelectorAll<HTMLElement>('.docs-sidenav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.sectionId === activeId);
      });
    };

    content.addEventListener('scroll', updateActive, { passive: true });
    updateActive();
    this.scrollCleanup = () => content.removeEventListener('scroll', updateActive);
  }

  private rerender(): void {
    if (!this.contentEl) return;
    this.contentEl.innerHTML = '';
    const filter = this.searchEl?.value ?? '';

    if (this.currentView === VIEW_GUIDES) {
      if (this.countEl) this.countEl.textContent = '';
      const navItems = this.renderGuides(this.contentEl, filter);
      this.renderSideNav(navItems);
    } else {
      const lf = filter.toLowerCase();

      // Narrow entry pool for single-source views
      let pool = this.entries;
      let isSourceView = false;
      if (this.currentView === VIEW_USER_CODE) {
        pool = this.entries.filter(e => e.library === USER_LIBRARY);
        isSourceView = true;
      } else if (this.currentView.startsWith('lib:')) {
        const lib = this.currentView.slice(4);
        pool = this.entries.filter(e => e.library === lib);
        isSourceView = true;
      }

      const filtered = filter
        ? pool.filter(e =>
            e.name.toLowerCase().includes(lf) ||
            e.signature.toLowerCase().includes(lf) ||
            e.doc.toLowerCase().includes(lf))
        : pool;
      const groups = isSourceView ? groupEntriesFlat(filtered) : groupEntries(filtered);
      this.renderAPI(this.contentEl, filter, groups, isSourceView);
      this.renderSideNav(groups.map(([label]) => ({
        label,
        sectionId: `api-${toId(label)}`,
        action: () => {
          const target = this.contentEl?.querySelector<HTMLDetailsElement>(`#api-${toId(label)}`);
          if (!target) return;
          target.open = true;
          target.scrollIntoView({ behavior: 'smooth', block: 'start' });
        },
      })));
    }
    this.setupSectionObserver();
  }

  private switchView(view: DocsView): void {
    this.currentView = view;
    this.currentGuide = null;
    this.el?.querySelectorAll<HTMLButtonElement>('.docs-tab').forEach(tab => {
      const active = tab.dataset.view === view;
      tab.classList.toggle('active', active);
      if (active) tab.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
    this.rerender();
  }

  private renderGuides(content: HTMLElement, filter: string): NavItem[] {
    if (this.guides.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'docs-empty';
      empty.textContent = 'No guides available.';
      content.appendChild(empty);
      return [];
    }

    // With a filter: render all matching guides flat with highlights; no side nav
    if (filter) {
      const lf = filter.toLowerCase();
      for (const guide of this.guides) {
        if (!guide.title.toLowerCase().includes(lf) && !guide.markdown.toLowerCase().includes(lf)) continue;
        const section = document.createElement('div');
        section.className = 'docs-guide';
        section.innerHTML = renderMarkdown(guide.markdown);
        attachCopyButtons(section);
        attachTypeHovers(section, this.entries);
        highlightText(section, filter);
        content.appendChild(section);
      }
      if (content.children.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'docs-empty';
        empty.textContent = 'No matching guides.';
        content.appendChild(empty);
      }
      return [];
    }

    // No filter: auto-open User's Guide by default
    if (!this.currentGuide && this.guides.length > 0) {
      const usersGuide = this.guides.find(g => g.slug === 'users-guide');
      if (usersGuide) this.currentGuide = usersGuide;
    }

    if (!this.currentGuide) {
      return this.renderGuideList(content);
    } else {
      return this.renderGuideSingle(content, this.currentGuide);
    }
  }

  private renderGuideList(content: HTMLElement): NavItem[] {
    const navItems: NavItem[] = [];
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
      navItems.push({ label: guide.title, action: open });
    }
    return navItems;
  }

  private renderGuideSingle(content: HTMLElement, guide: DocGuide): NavItem[] {
    const navItems: NavItem[] = [];

    const back = document.createElement('button');
    back.className = 'guide-back-btn';
    back.innerHTML = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg> Guides`;
    back.addEventListener('click', () => { this.currentGuide = null; this.rerender(); });
    content.appendChild(back);

    const sections = splitSections(guide.markdown);

    for (const { heading, level, body } of sections) {
      if (level === 1) {
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
        const id = `guide-section-${toId(heading)}`;
        const details = document.createElement('details');
        details.id = id;
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
        navItems.push({
          label: heading,
          sectionId: id,
          action: () => {
            details.open = true;
            details.scrollIntoView({ behavior: 'smooth', block: 'start' });
          },
        });
      }
    }
    return navItems;
  }

  private renderAPI(content: HTMLElement, filter: string, groups: [string, DocEntry[]][], hideMeta = false): void {
    const totalVisible = groups.reduce((sum, [, items]) => sum + items.length, 0);
    if (this.countEl) {
      this.countEl.textContent = filter
        ? `${totalVisible} result${totalVisible !== 1 ? 's' : ''}`
        : `${totalVisible} entries`;
    }

    for (const [label, items] of groups) {
      const details = document.createElement('details');
      details.id = `api-${toId(label)}`;
      details.className = 'api-section';
      details.open = true;

      const summary = document.createElement('summary');
      summary.className = 'api-section-summary';
      summary.textContent = label;
      details.appendChild(summary);

      const bodyEl = document.createElement('div');
      bodyEl.className = 'api-section-body';

      for (const item of items) {
        const card = document.createElement('div');
        card.className = 'docs-entry';
        card.dataset.name = item.name;
        card.dataset.library = item.library || '';

        const sig = document.createElement('pre');
        sig.className = 'docs-signature';
        const code = document.createElement('code');
        code.textContent = item.signature || item.name;
        sig.appendChild(code);
        card.appendChild(sig);

        if (item.doc) {
          const docEl = document.createElement('div');
          docEl.className = 'docs-body';
          docEl.innerHTML = renderMarkdown(item.doc);
          attachCopyButtons(docEl);
          card.appendChild(docEl);
        }

        if (!hideMeta && item.library && item.library !== USER_LIBRARY) {
          const meta = document.createElement('div');
          meta.className = 'docs-meta';
          meta.textContent = item.library;
          card.appendChild(meta);
        }

        if (filter) highlightText(card, filter);
        bodyEl.appendChild(card);
      }

      details.appendChild(bodyEl);
      content.appendChild(details);
    }

    if (groups.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'docs-empty';
      empty.textContent = 'No matching entries.';
      content.appendChild(empty);
      if (this.countEl) this.countEl.textContent = '0 results';
    }
  }

  focusEntry(name: string, library?: string): void {
    this.currentView = VIEW_API;
    this.currentGuide = null;
    if (this.searchEl) this.searchEl.value = '';
    this.el?.querySelectorAll<HTMLButtonElement>('.docs-tab').forEach(tab => {
      tab.classList.toggle('active', tab.dataset.view === VIEW_API);
    });
    this.rerender();

    if (!this.contentEl) return;
    // When the caller supplied a library, prefer that exact match so a
    // stdlib `Cube` and a library `Cube` don't both render with the
    // same data-name. Without a library, fall back to the first card
    // with the matching name.
    const nameSel = `[data-name="${CSS.escape(name)}"]`;
    const target = (library !== undefined
      ? this.contentEl.querySelector<HTMLElement>(`${nameSel}[data-library="${CSS.escape(library)}"]`)
      : null) ?? this.contentEl.querySelector<HTMLElement>(nameSel);
    if (!target) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    target.classList.add('docs-entry-focused');
    setTimeout(() => target.classList.remove('docs-entry-focused'), 1500);
  }

  hide(): void {
    if (!this.el.classList.contains('open')) return;
    this.scrollCleanup?.();
    this.scrollCleanup = null;
    this.el.classList.remove('open');
    this.contentEl = null;
    this.searchEl = null;
    this.countEl = null;
    this.sideNavEl = null;
  }

  isVisible(): boolean {
    return this.el.classList.contains('open');
  }
}
