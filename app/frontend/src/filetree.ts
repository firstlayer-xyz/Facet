// filetree.ts — File tree panel showing the main file and its library dependencies.

/** Parse lib "..." import strings from Facet source. */
export function parseLibImports(source: string): string[] {
  const seen = new Set<string>();
  const re = /\blib\s+"([^"]+)"/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) seen.add(m[1]);
  return [...seen];
}

/**
 * Find the key in sources that corresponds to a library import path.
 *
 * The backend stores sources under two keys per library:
 *   1. The import path itself (le.Path), e.g. "facet/gears" or "github.com/user/repo@v1.0/sub"
 *   2. The full filesystem path, e.g. "/cache/.../sub/sub.fct" (absent for embedded libs)
 *
 * We try the import path first (covers embedded libs), then fall back to
 * path-segment matching that strips the @ref component (covers remote libs).
 */
export function findLibPath(libId: string, sources: Record<string, { text: string }>): string | undefined {
  // 1. Direct lookup by import path (embedded + remote libs, backend always adds this key)
  if (libId in sources) return libId;

  // 2. Filesystem path matching — strip @ref and match path segments in order.
  //    e.g. "github.com/user/repo@v1.0/shapes" → parts ["github.com","user","repo","shapes"]
  //    file path: "/cache/github.com/user/repo/v1.0/shapes/shapes.fct" → matches
  const withoutRef = libId.replace(/@[^/]+/, '');
  const libParts = withoutRef.split('/').filter(Boolean);
  const baseName = libParts[libParts.length - 1];

  for (const path of Object.keys(sources)) {
    // Only consider real filesystem paths (not import paths / builtins)
    if (!path.startsWith('/') && !path.match(/^[A-Za-z]:\\/)) continue;
    const normalized = path.replace(/\\/g, '/');
    const withoutExt = normalized.replace(/\.[^./]+$/, '');
    const lastSlash = withoutExt.lastIndexOf('/');
    if (lastSlash === -1) continue;
    const fileName = withoutExt.slice(lastSlash + 1);
    if (fileName !== baseName) continue;

    // All lib parts should appear as path segments in order (ref ignored)
    const parentPath = withoutExt.slice(0, lastSlash);
    let searchFrom = 0;
    let allFound = true;
    for (const part of libParts) {
      const idx = parentPath.indexOf('/' + part + '/', searchFrom);
      if (idx !== -1) {
        searchFrom = idx + part.length + 1;
        continue;
      }
      if (parentPath.endsWith('/' + part)) break;
      allFound = false;
      break;
    }
    if (allFound) return path;
  }
  return undefined;
}

interface FileTreeCallbacks {
  getActiveLabel(): string;
  getActiveTab(): string;
  getSources(): Record<string, { text: string }>;
  openTab(path: string, source: string): void;
}

export class FileTree {
  private panel: HTMLElement;
  private libs: string[] = [];
  private visible = false;
  readonly callbacks: FileTreeCallbacks;

  constructor(callbacks: FileTreeCallbacks) {
    this.callbacks = callbacks;
    this.panel = document.createElement('div');
    this.panel.id = 'file-tree-panel';
    this.panel.style.display = 'none';
  }

  get element(): HTMLElement { return this.panel; }

  update(source: string, activeTab: string) {
    this.libs = parseLibImports(source);
    this.render(activeTab);
  }

  setActiveTab(activeTab: string) { this.render(activeTab); }

  private render(activeTab: string) {
    this.panel.innerHTML = '';
    if (!activeTab) return; // no tabs open

    const activeLabel = this.callbacks.getActiveLabel();
    const libSources = this.callbacks.getSources();

    // Active file row
    const mainRow = document.createElement('div');
    mainRow.className = 'ft-row ft-main ft-active';
    mainRow.appendChild(makeFileIcon());
    const mainSpan = document.createElement('span');
    mainSpan.textContent = activeLabel;
    mainRow.appendChild(mainSpan);
    this.panel.appendChild(mainRow);

    // Recursively render lib rows
    this.renderLibs(this.libs, libSources, activeTab, 1);
  }

  private renderLibs(
    libs: string[],
    libSources: Record<string, { text: string }>,
    activeTab: string,
    depth: number,
    visited = new Set<string>(),
  ) {
    for (let i = 0; i < libs.length; i++) {
      const libId = libs[i];
      const isLast = i === libs.length - 1;
      const libPath = findLibPath(libId, libSources);
      const libSource = libPath ? libSources[libPath]?.text : undefined;

      // Skip libs we've already rendered (circular imports)
      if (libPath && visited.has(libPath)) continue;

      const isActive = !!libPath && activeTab === libPath;

      const row = document.createElement('div');
      row.className =
        'ft-row ft-lib' +
        (isActive ? ' ft-active' : '') +
        (libSource ? '' : ' ft-unavailable');
      row.style.paddingLeft = `${depth * 14}px`;

      const connector = document.createElement('span');
      connector.className = 'ft-connector';
      connector.textContent = isLast ? '└─ ' : '├─ ';
      row.appendChild(connector);
      row.appendChild(makeFileIcon());

      const label = document.createElement('span');
      label.textContent = libId;
      row.appendChild(label);

      if (libSource && libPath) {
        row.style.cursor = 'pointer';
        row.addEventListener('click', () => this.callbacks.openTab(libPath!, libSource!));
      } else {
        row.title = 'Run or debug to load this library';
      }
      this.panel.appendChild(row);

      // Recurse into this lib's own imports
      if (libSource && libPath) {
        visited.add(libPath);
        const childLibs = parseLibImports(libSource);
        if (childLibs.length > 0) {
          this.renderLibs(childLibs, libSources, activeTab, depth + 1, visited);
        }
      }
    }
  }

  toggle(): boolean {
    this.visible = !this.visible;
    this.panel.style.display = this.visible ? 'flex' : 'none';
    return this.visible;
  }

  isVisible(): boolean { return this.visible; }
}

function makeFileIcon(): SVGElement {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('class', 'ft-icon');
  svg.setAttribute('width', '11');
  svg.setAttribute('height', '11');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('fill', 'none');
  svg.setAttribute('stroke', 'currentColor');
  svg.setAttribute('stroke-width', '2');
  const p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  p.setAttribute('d', 'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z');
  const poly = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
  poly.setAttribute('points', '14 2 14 8 20 8');
  svg.appendChild(p);
  svg.appendChild(poly);
  return svg;
}
