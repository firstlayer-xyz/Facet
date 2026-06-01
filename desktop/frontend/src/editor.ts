// Import only the core editor API — skips 90+ bundled language grammars
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';

// Import editor features that are NOT included in the tree-shaken API import.
// Without these, registerHoverProvider / registerCompletionItemProvider have
// no UI to render into.
import 'monaco-editor/esm/vs/editor/contrib/hover/browser/hoverContribution';
import 'monaco-editor/esm/vs/editor/contrib/suggest/browser/suggestController';
import 'monaco-editor/esm/vs/editor/contrib/snippet/browser/snippetController2';
import 'monaco-editor/esm/vs/editor/contrib/contextmenu/browser/contextmenu';
import 'monaco-editor/esm/vs/editor/contrib/find/browser/findController';
import 'monaco-editor/esm/vs/editor/contrib/bracketMatching/browser/bracketMatching';
import 'monaco-editor/esm/vs/editor/contrib/folding/browser/folding';
import 'monaco-editor/esm/vs/editor/contrib/multicursor/browser/multicursor';
import 'monaco-editor/esm/vs/editor/contrib/wordOperations/browser/wordOperations';
import 'monaco-editor/esm/vs/editor/contrib/wordHighlighter/browser/wordHighlighter';
import 'monaco-editor/esm/vs/editor/contrib/clipboard/browser/clipboard';
import 'monaco-editor/esm/vs/editor/contrib/links/browser/links';
import 'monaco-editor/esm/vs/editor/contrib/comment/browser/comment';

import 'monaco-editor/esm/vs/editor/contrib/parameterHints/browser/parameterHints';
import { registerFacetLanguage } from './facet-language';
import { registerThemes } from './themes';
import { ListLibraries, ListLocalLibraries, ListLibraryModules } from '../wailsjs/go/main/App';
import type { DeclLocation, FacetSymbol } from './eval-client';

// Monaco worker setup — only the base editor worker is needed
self.MonacoEnvironment = {
  getWorker() {
    return new editorWorker();
  },
};

// Register the Facet language once
registerFacetLanguage();

// Register all bundled VS Code themes
registerThemes();

// Base colors shared by all light/dark theme variants.
// Only accent-specific colors (highlight, cursor, selection) differ per theme.
const lightBaseColors: Record<string, string> = {
  'editor.background': '#ffffff',
  'editor.foreground': '#3a3a3a',
  'editorWidget.background': '#f7f7f7',
  'editorWidget.border': '#d0d0d0',
  'editorSuggestWidget.background': '#f7f7f7',
  'editorSuggestWidget.foreground': '#3a3a3a',
  'editorSuggestWidget.border': '#d0d0d0',
  'editorSuggestWidget.selectedBackground': '#e8e8e8',
  'editorSuggestWidget.selectedForeground': '#1a1a1a',
  'editorHoverWidget.background': '#f7f7f7',
  'editorHoverWidget.foreground': '#3a3a3a',
  'editorHoverWidget.border': '#d0d0d0',
  'editorLineNumber.foreground': '#b0b0b0',
};

const darkBaseColors: Record<string, string> = {
  'editor.background': '#1a1a1a',
  'editor.foreground': '#cccccc',
  'editorWidget.background': '#252525',
  'editorWidget.border': '#3a3a3a',
  'editorSuggestWidget.background': '#252525',
  'editorSuggestWidget.foreground': '#cccccc',
  'editorSuggestWidget.border': '#3a3a3a',
  'editorSuggestWidget.selectedBackground': '#3a3a3a',
  'editorSuggestWidget.selectedForeground': '#eeeeee',
  'editorHoverWidget.background': '#252525',
  'editorHoverWidget.foreground': '#cccccc',
  'editorHoverWidget.border': '#3a3a3a',
  'editorLineNumber.foreground': '#555555',
};

interface AccentColors {
  /** Token foreground for constant.numeric / constant.language */
  light: string;
  dark: string;
  /** Cursor color (light variant) */
  cursorLight: string;
  /** Cursor color (dark variant) */
  cursorDark: string;
  /** Selection background (light variant) */
  selectionLight: string;
  /** Selection background (dark variant) */
  selectionDark: string;
}

// Token color definitions — each entry maps to { light, dark } foreground colors.
// Accent-colored tokens use null and are filled per-theme.
const TOKEN_COLORS: [string, string | null, string | null][] = [
  ['keyword.control',       '7c3aed', 'c49cff'],
  ['keyword.other.unit',    '7c3aed', 'c49cff'],
  ['support.type',          '0d7377', '4dd4d8'],
  ['constant.numeric',       null,     null   ],
  ['constant.numeric.float', null,     null   ],
  ['constant.language',      null,     null   ],
  ['string.quoted.double',  '2a7e3b', '7ecc8d'],
  ['string.quoted.other',   '2a7e3b', '7ecc8d'],
  ['comment.line',          '999999', '666666'],
  ['keyword.operator',      '0d7377', '4dd4d8'],
  ['variable.other',        '3a3a3a', 'cccccc'],
  ['punctuation.delimiter', '3a3a3a', 'cccccc'],
];

function defineAccentThemes(name: string, accent: AccentColors): void {
  function buildRules(variant: 'light' | 'dark'): monaco.editor.ITokenThemeRule[] {
    const accentColor = variant === 'light' ? accent.light : accent.dark;
    return TOKEN_COLORS.map(([token, light, dark]) => ({
      token,
      foreground: (variant === 'light' ? light : dark) ?? accentColor,
    }));
  }
  const lightRules = buildRules('light');
  const darkRules = buildRules('dark');

  // Active-line wash: accent at ~8% (light) / ~6% (dark) opacity, as 8-char hex
  const lineWashLight = `#${accent.light}14`;
  const lineWashDark  = `#${accent.dark}0F`;

  monaco.editor.defineTheme(`facet-${name}-light`, {
    base: 'vs',
    inherit: true,
    rules: lightRules,
    colors: {
      ...lightBaseColors,
      'editorSuggestWidget.highlightForeground': '#' + accent.light,
      'editorSuggestWidget.focusHighlightForeground': '#' + accent.light,
      'editorCursor.foreground': accent.cursorLight,
      'editor.selectionBackground': accent.selectionLight,
      'editor.lineHighlightBackground': lineWashLight,
      'editor.lineHighlightBorder': '#00000000',
      'editorGutter.activeLineBackground': lineWashLight,
      'editorLineNumber.activeForeground': `#${accent.light}`,
    },
  });

  monaco.editor.defineTheme(`facet-${name}-dark`, {
    base: 'vs-dark',
    inherit: true,
    rules: darkRules,
    colors: {
      ...darkBaseColors,
      'editorSuggestWidget.highlightForeground': '#' + accent.dark,
      'editorSuggestWidget.focusHighlightForeground': '#' + accent.dark,
      'editorCursor.foreground': accent.cursorDark,
      'editor.selectionBackground': accent.selectionDark,
      'editor.lineHighlightBackground': lineWashDark,
      'editor.lineHighlightBorder': '#00000000',
      'editorGutter.activeLineBackground': lineWashDark,
      'editorLineNumber.activeForeground': `#${accent.dark}`,
    },
  });
}

defineAccentThemes('orange', {
  light: 'e45500', dark: 'ffaa44',
  cursorLight: '#ff6d00', cursorDark: '#ff6d00',
  selectionLight: '#ffe0c0', selectionDark: '#553a1a',
});
defineAccentThemes('green', {
  light: '1e8a3e', dark: '52c878',
  cursorLight: '#1e8a3e', cursorDark: '#2eb84e',
  selectionLight: '#c8f0d4', selectionDark: '#1a3a22',
});
defineAccentThemes('digital-blue', {
  light: '0060c8', dark: '5aa8ff',
  cursorLight: '#0060c8', cursorDark: '#3d96ff',
  selectionLight: '#c8d8f8', selectionDark: '#1a2a4a',
});


interface CheckError {
  file: string;
  line: number;
  col: number;
  endCol: number;
  message: string;
}

export interface EditorHandle {
  getContent(): string;
  getAllSources(): Record<string, string>;
  highlightError(line: number): void;
  clearError(): void;
  setMarkers(errors: CheckError[]): void;
  clearMarkers(): void;
  highlightDebugLine(line: number): void;
  clearDebugLine(): void;
  setContent(text: string): void;
  setContentSilent(text: string): void;
  switchModel(fileKey: string, content: string): void;
  preloadModel(fileKey: string, content: string): void;
  disposeModel(fileKey: string): void;
  setReadOnly(ro: boolean): void;
  setWordWrap(on: boolean): void;
  setTheme(name: string): void;
  updateSymbols(symbols: FacetSymbol[]): void;
  updateVarTypes(types: Record<string, Record<string, string>>): void;
  setCurrentSource(sourceKey: string): void;
  updateDeclarations(decls: Record<string, DeclLocation>): void;
  updateReferences(refs: Record<string, DeclLocation>): void;
  updateFileSources(sources: Record<string, string>): void;
  getCursorPosition(): { lineNumber: number; column: number };
  onCursorChange(cb: (line: number, col: number) => void): void;
  onMouseMove(cb: (line: number, col: number) => void): void;
  onMouseLeave(cb: () => void): void;
  revealLine(line: number, col?: number): void;
  // Breakpoints
  setBreakpointMode(enabled: boolean): void;
  setValidBreakpointLines(file: string, lines: Set<number>): void;
  syncBreakpoints(file: string, lines: Set<number>): void;
  onBreakpointChange(cb: (file: string, lines: Set<number>) => void): void;
  onJumpToLine(cb: (file: string, line: number) => void): void;
}

function mapKindToCompletionItemKind(kind: string): monaco.languages.CompletionItemKind {
  switch (kind) {
    case 'function': return monaco.languages.CompletionItemKind.Function;
    case 'method': return monaco.languages.CompletionItemKind.Method;
    case 'class': return monaco.languages.CompletionItemKind.Class;
    case 'keyword': return monaco.languages.CompletionItemKind.Keyword;
    case 'field': return monaco.languages.CompletionItemKind.Field;
    default: return monaco.languages.CompletionItemKind.Function;
  }
}

export function createEditor(
  parent: HTMLElement,
  initialDoc: string,
  onChange?: () => void,
  onOpenDocs?: (name: string, library?: string) => void,
  onGoToFile?: (file: string, source: string, line: number, col: number) => void,
  initialFileKey?: string,
): EditorHandle {
  let symbols: FacetSymbol[] = [];
  let allVarTypes: Record<string, Record<string, string>> = {};
  // currentSourceKey is the tab the editor is currently displaying.
  // All references-map and varTypes lookups key directly off this; the
  // backend stamps the same source-key shape so no normalization is
  // needed. Tabs are peers — there is no privileged "main".
  let currentSourceKey = initialFileKey || '';
  let declarations: Record<string, DeclLocation> = {};
  // references maps "<srcKey>:line:col" of a referring token to the
  // declaration it resolves to. Built by the checker on the backend,
  // refreshed on every eval. Used by findDecl (goto-definition) and
  // the hover provider.
  let references: Record<string, DeclLocation> = {};
  let fileSources: Record<string, string> = {};
  let suppressChange = false;

  // Breakpoint state
  let breakpointClickEnabled = false;
  const breakpointsPerFile = new Map<string, Set<number>>();
  const validLinesPerFile = new Map<string, Set<number>>();
  let onBreakpointChangeCb: ((file: string, lines: Set<number>) => void) | null = null;
  let onJumpToLineCb: ((file: string, line: number) => void) | null = null;

  const models = new Map<string, monaco.editor.ITextModel>();
  const initialModel = monaco.editor.createModel(initialDoc, 'facet');
  models.set(currentSourceKey, initialModel);

  const ed = monaco.editor.create(parent, {
    model: initialModel,
    theme: 'facet-orange-light',
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: true,
    fontSize: 14,
    tabSize: 4,
    insertSpaces: true,
    lineNumbers: 'on',
    renderLineHighlight: 'all',
    overviewRulerLanes: 0,
    hideCursorInOverviewRuler: true,
    fixedOverflowWidgets: true,
    wordWrap: 'on',
    glyphMargin: false,
    lineDecorationsWidth: 6,
    lineNumbersMinChars: 4,
    scrollbar: {
      vertical: 'auto',
      horizontal: 'hidden',
    },
  });

  // Expose monaco and the editor instance for integration tests. Monaco is
  // imported as an ESM module and is otherwise unreachable from page-context
  // scripts (Playwright `page.evaluate`); without this, the test suite cannot
  // drive the editor (setValue, getPosition, getScrolledVisiblePosition, etc.).
  // The cost is a single property on window that points at code already in the
  // bundle — no additional payload.
  (window as unknown as { monaco: typeof monaco }).monaco = monaco;

  const errorCollection = ed.createDecorationsCollection();
  const debugCollection = ed.createDecorationsCollection();
  const gutterCollection = ed.createDecorationsCollection();

  function refreshBreakpointDecorations() {
    const bps = breakpointsPerFile.get(currentSourceKey) ?? new Set<number>();
    const valid = validLinesPerFile.get(currentSourceKey) ?? new Set<number>();
    const model = ed.getModel();
    if (!model) return;
    const lineCount = model.getLineCount();

    const decorations: monaco.editor.IModelDeltaDecoration[] = [];
    if (breakpointClickEnabled) {
      for (const line of valid) {
        if (line >= 1 && line <= lineCount) {
          const isBp = bps.has(line);
          // Left lane: ○/● breakpoint indicator
          decorations.push({
            range: new monaco.Range(line, 1, line, 1),
            options: {
              isWholeLine: isBp,
              className: isBp ? 'monaco-breakpoint-line' : undefined,
              glyphMarginClassName: isBp ? 'monaco-bp-active' : 'monaco-bp-hint',
              glyphMargin: { position: monaco.editor.GlyphMarginLane.Left },
            },
          });
          // Right lane: ▶ jump button
          decorations.push({
            range: new monaco.Range(line, 1, line, 1),
            options: {
              glyphMarginClassName: 'monaco-step-jump',
              glyphMargin: { position: monaco.editor.GlyphMarginLane.Right },
            },
          });
        }
      }
    }
    gutterCollection.set(decorations);
  }

  function setLineDecoration(collection: monaco.editor.IEditorDecorationsCollection, line: number, className: string) {
    if (line < 1 || line > ed.getModel()!.getLineCount()) return;
    collection.set([{ range: new monaco.Range(line, 1, line, 1), options: { isWholeLine: true, className } }]);
  }

  function replaceContent(text: string, silent: boolean) {
    if (silent) suppressChange = true;
    try {
      const fullRange = ed.getModel()!.getFullModelRange();
      ed.executeEdits(silent ? 'setContentSilent' : 'setContent', [{ range: fullRange, text }]);
    } finally {
      if (silent) suppressChange = false;
    }
  }

  // Register command for hover "→ Open in Docs" links and context menu
  if (onOpenDocs) {
    ed.addCommand(0, (_ctx, name: string) => {
      onOpenDocs(name);
    });

    ed.addAction({
      id: 'facet.openDocs',
      label: 'Open Documentation',
      contextMenuGroupId: 'z_docs',
      contextMenuOrder: 1,
      run(ed) {
        const pos = ed.getPosition();
        if (!pos) return;
        const model = ed.getModel();
        if (!model) return;
        const word = model.getWordAtPosition(pos);
        if (!word) return;
        const matched = resolveSymbolAtCursor(model, pos, word);
        const sym = matched[0];
        if (!sym) return;
        // The Docs panel keys entries by DocEntry name shape: methods
        // are "Receiver.Method"; everything else is bare. The library
        // tag is passed alongside so the panel can disambiguate name
        // collisions between stdlib and an imported library.
        const docName = sym.receiver ? `${sym.receiver}.${sym.name}` : sym.name;
        onOpenDocs(docName, sym.library || undefined);
      },
    });
  }

  // effectiveTextBefore returns the text before a given 1-based column on
  // `line`. If the text before the column is whitespace-only (e.g. the line
  // starts with a `.` chain continuation), walk backwards to the previous
  // non-empty line and return its trimmed-right content instead — so callers
  // can resolve the receiver expression that lives on an earlier line.
  // Comment-only lines are skipped so a `// foo` between expressions doesn't
  // become a fake receiver.
  function effectiveTextBefore(
    model: monaco.editor.ITextModel,
    lineNumber: number,
    column: number,
  ): string {
    const raw = model.getLineContent(lineNumber).slice(0, column - 1);
    if (raw.trim().length > 0) return raw;
    for (let ln = lineNumber - 1; ln >= 1; ln--) {
      const prev = model.getLineContent(ln).trimEnd();
      if (prev.length === 0) continue;
      if (prev.trimStart().startsWith('//')) continue;
      return prev;
    }
    return raw;
  }

  // buildCodeMask marks the positions in `line` that are part of code
  // (true) vs inside a string literal or line comment (false). Used by
  // walkBackToCall so a `(`/`,`/`)` inside `"..."` or after `//` does
  // not confuse the call-site detector.
  function buildCodeMask(line: string): boolean[] {
    const mask = new Array(line.length).fill(true);
    let i = 0;
    while (i < line.length) {
      const ch = line[i];
      if (ch === '/' && line[i + 1] === '/') {
        for (let j = i; j < line.length; j++) mask[j] = false;
        return mask;
      }
      if (ch === '"') {
        mask[i] = false;
        i++;
        while (i < line.length) {
          mask[i] = false;
          if (line[i] === '\\' && i + 1 < line.length) {
            mask[i + 1] = false;
            i += 2;
            continue;
          }
          if (line[i] === '"') { i++; break; }
          i++;
        }
        continue;
      }
      i++;
    }
    return mask;
  }

  // splitParams splits a `(a Type, b Type = default, ...)`-body string
  // into individual parameter specs. Naively splitting on `,` breaks any
  // default value that itself contains a comma (e.g. `= Vec3(1, 2, 3)`),
  // so this respects nested paren/bracket/brace depth.
  function splitParams(paramStr: string): string[] {
    const out: string[] = [];
    let depth = 0;
    let start = 0;
    for (let i = 0; i < paramStr.length; i++) {
      const ch = paramStr[i];
      if (ch === '(' || ch === '[' || ch === '{') depth++;
      else if (ch === ')' || ch === ']' || ch === '}') depth--;
      else if (ch === ',' && depth === 0) {
        const seg = paramStr.slice(start, i).trim();
        if (seg) out.push(seg);
        start = i + 1;
      }
    }
    const last = paramStr.slice(start).trim();
    if (last) out.push(last);
    return out;
  }

  // extractParamBody returns the contents between the outer parens of a
  // signature like `fn cube(size Length = 10 mm) Solid`. The naive
  // `/\(([^)]*)\)/` regex stops at the first `)`, so any default value
  // containing a paren cuts the param list short — this walks balanced
  // parens to find the actual closer.
  function extractParamBody(signature: string): string | null {
    const open = signature.indexOf('(');
    if (open < 0) return null;
    let depth = 0;
    for (let i = open; i < signature.length; i++) {
      const ch = signature[i];
      if (ch === '(') depth++;
      else if (ch === ')') {
        depth--;
        if (depth === 0) return signature.slice(open + 1, i);
      }
    }
    return null;
  }

  // resolveChainType returns the type of the expression represented by the
  // given text fragment. Handles arbitrary chain depth by recursing.
  // e.g. "Cube(1,1,1).Move(1,0,0)" → "Solid"
  function resolveChainType(text: string): string | null {
    text = text.trimEnd();

    // Index expression: arr[0] → strip [] from the receiver's type
    if (text.endsWith(']')) {
      let depth = 0;
      let i = text.length - 1;
      for (; i >= 0; i--) {
        if (text[i] === ']') depth++;
        else if (text[i] === '[') { depth--; if (depth === 0) break; }
      }
      if (i <= 0) return null;
      const receiverType = resolveChainType(text.slice(0, i));
      if (receiverType && receiverType.endsWith('[]')) {
        return receiverType.slice(0, -2);
      }
      return null;
    }

    if (text.endsWith(')')) {
      // Call expression: walk back past balanced parens to find the function name
      let depth = 0;
      let i = text.length - 1;
      for (; i >= 0; i--) {
        if (text[i] === ')') depth++;
        else if (text[i] === '(') { depth--; if (depth === 0) break; }
      }
      if (i <= 0) return null;
      const beforeParen = text.slice(0, i);

      // Method call: beforeParen ends with "expr.MethodName"
      const dotMatch = beforeParen.match(/^(.*)\.\s*([A-Za-z_]\w*)$/);
      if (dotMatch) {
        const receiverType = resolveChainType(dotMatch[1]);
        if (receiverType) {
          const decl = declarations[receiverType + '.' + dotMatch[2]];
          if (decl?.returnType) return decl.returnType;
        }
        // Fall through — may be a qualified function name like "F.HexNut"
      }

      // Qualified function call: e.g. "F.HexNut"
      const qualMatch = beforeParen.match(/([A-Za-z_]\w*\.[A-Za-z_]\w*)$/);
      if (qualMatch) {
        const decl = declarations[qualMatch[1]];
        if (decl?.returnType) return decl.returnType;
      }

      // Bare function call: e.g. "Cube"
      const bareMatch = beforeParen.match(/([A-Za-z_]\w*)$/);
      if (bareMatch) {
        const decl = declarations[bareMatch[1]];
        if (decl?.returnType) return decl.returnType;
      }

      return null;
    }

    // Field access chain: receiverExpr.field — look up <receiverType>.<field>
    // and return the field's declared type. Recurse on the receiver expression.
    const fieldMatch = text.match(/^(.*)\.\s*([A-Za-z_]\w*)$/);
    if (fieldMatch) {
      const receiverType = resolveChainType(fieldMatch[1]);
      if (receiverType) {
        const decl = declarations[receiverType + '.' + fieldMatch[2]];
        if (decl?.returnType) return decl.returnType;
      }
      // Could also be a qualified reference like "F.SomeConst" — no type info.
      return null;
    }

    // Identifier: look up variable type
    const identMatch = text.match(/([A-Za-z_]\w*)$/);
    if (identMatch) {
      return (allVarTypes[currentSourceKey] ?? {})[identMatch[1]] || null;
    }

    return null;
  }

  // findDecl resolves the declaration at the cursor via the references map
  // built by the checker. The map is keyed by "file:line:col" of the
  // referring token; currentSourceKey is normalized to "" for the main file
  // to match the backend's DeclLocation.File convention.
  //
  // The checker pre-resolves every identifier, call, method, field access,
  // struct-lit name, and named arg at type-check time — so there is no
  // client-side chain walking, receiver-type inference, or local-scope scan.
  // Multi-line method chains (e.g. `var x = F\n    .Knurl(...)`) just work
  // because the map keys on the actual token position from the AST.
  function findDecl(editor: monaco.editor.ICodeEditor): DeclLocation | null {
    const pos = editor.getPosition();
    if (!pos) return null;
    const mdl = editor.getModel();
    if (!mdl) return null;
    const word = mdl.getWordAtPosition(pos);
    if (!word) return null;

    const key = `${currentSourceKey}:${pos.lineNumber}:${word.startColumn}`;
    return references[key] ?? null;
  }

  // "Open Library" context menu — detect lib "path" on current line
  function getLibPathAtCursor(editor: monaco.editor.ICodeEditor): string | null {
    const pos = editor.getPosition();
    if (!pos) return null;
    const line = editor.getModel()?.getLineContent(pos.lineNumber) ?? '';
    const match = line.match(/lib\s+"([^"]+)"/);
    if (!match) return null;
    return match[1];
  }

  // Context keys updated on every cursor move (single listener for both)
  const hasDeclKey = ed.createContextKey<boolean>('facet.hasDeclaration', false);
  const hasLibPathKey = ed.createContextKey<boolean>('facet.hasLibPath', false);
  ed.onDidChangeCursorPosition(() => {
    hasDeclKey.set(findDecl(ed) !== null);
    hasLibPathKey.set(getLibPathAtCursor(ed) !== null);
  });

  // Right-click normally leaves the caret where it was, which means the
  // `facet.hasDeclaration` context key (updated only on cursor move) would
  // reflect the stale caret position instead of the token under the mouse.
  // Move the caret to the click position first so the precondition is
  // evaluated against the thing the user actually clicked — and the
  // `Go to Declaration` action fires at that token when invoked.
  ed.onMouseDown(e => {
    if (e.event.rightButton && e.target.position) {
      ed.setPosition(e.target.position);
    }

    // Gutter interactions in debug mode
    if (breakpointClickEnabled && !e.event.rightButton && e.target.position) {
      const t = e.target.type;
      const line = e.target.position.lineNumber;
      const valid = validLinesPerFile.get(currentSourceKey);

      if (t === monaco.editor.MouseTargetType.GUTTER_GLYPH_MARGIN && valid?.has(line)) {
        const layout = ed.getLayoutInfo();
        const edRect = ed.getDomNode()!.getBoundingClientRect();
        const relX = e.event.posx - edRect.left - layout.glyphMarginLeft;
        if (relX < layout.glyphMarginWidth / 2) {
          // left lane (○/●) → toggle breakpoint
          const bps = breakpointsPerFile.get(currentSourceKey) ?? new Set<number>();
          if (bps.has(line)) bps.delete(line); else bps.add(line);
          breakpointsPerFile.set(currentSourceKey, bps);
          refreshBreakpointDecorations();
          onBreakpointChangeCb?.(currentSourceKey, new Set(bps));
        } else {
          // right lane (▶) → jump to first debug step
          onJumpToLineCb?.(currentSourceKey, line);
        }
      }
    }
  });

  ed.addAction({
    id: 'facet.goToDeclaration',
    label: 'Go to Declaration',
    contextMenuGroupId: 'navigation',
    contextMenuOrder: 1,
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.F12],
    precondition: 'facet.hasDeclaration',
    run(ed) {
      const decl = findDecl(ed);
      if (!decl) return;
      if (decl.file && onGoToFile) {
        const source = fileSources[decl.file] ?? '';
        onGoToFile(decl.file, source, decl.line, decl.col);
      } else {
        ed.setPosition({ lineNumber: decl.line, column: decl.col });
        ed.revealLineInCenter(decl.line);
        ed.focus();
      }
    },
  });

  ed.addAction({
    id: 'facet.openLibrary',
    label: 'Open Library',
    contextMenuGroupId: 'navigation',
    contextMenuOrder: 2,
    precondition: 'facet.hasLibPath',
    run(ed) {
      const libPath = getLibPathAtCursor(ed);
      if (!libPath || !onGoToFile) return;
      const source = fileSources[libPath] ?? '';
      onGoToFile(libPath, source, 1, 1);
    },
  });

  // WKWebView clipboard: override Monaco's copy/cut/paste to use Wails native clipboard
  ed.addAction({
    id: 'facet.clipboardCopy',
    label: 'Copy',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyC],
    run(editor) {
      const sel = editor.getSelection();
      if (!sel || sel.isEmpty()) return;
      const text = editor.getModel()!.getValueInRange(sel);
      navigator.clipboard.writeText(text);
    },
  });
  ed.addAction({
    id: 'facet.clipboardCut',
    label: 'Cut',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyX],
    run(editor) {
      const sel = editor.getSelection();
      if (!sel || sel.isEmpty()) return;
      const text = editor.getModel()!.getValueInRange(sel);
      navigator.clipboard.writeText(text);
      editor.executeEdits('cut', [{ range: sel, text: '' }]);
    },
  });
  ed.addAction({
    id: 'facet.clipboardPaste',
    label: 'Paste',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyV],
    run(editor) {
      navigator.clipboard.readText().then((text) => {
        const sel = editor.getSelection();
        if (sel) {
          editor.executeEdits('paste', [{ range: sel, text }]);
        }
      });
    },
  });

  // Word navigation — registered explicitly because Monaco standalone (ESM) does
  // not always bind these when using selective contribution imports.
  ed.addAction({
    id: 'facet.cursorWordStartLeft',
    label: 'Move Word Left',
    keybindings: [monaco.KeyMod.Alt | monaco.KeyCode.LeftArrow],
    run: (ed) => ed.trigger('keyboard', 'cursorWordStartLeft', {}),
  });
  ed.addAction({
    id: 'facet.cursorWordEndRight',
    label: 'Move Word Right',
    keybindings: [monaco.KeyMod.Alt | monaco.KeyCode.RightArrow],
    run: (ed) => ed.trigger('keyboard', 'cursorWordEndRight', {}),
  });
  ed.addAction({
    id: 'facet.cursorWordStartLeftSelect',
    label: 'Select Word Left',
    keybindings: [monaco.KeyMod.Shift | monaco.KeyMod.Alt | monaco.KeyCode.LeftArrow],
    run: (ed) => ed.trigger('keyboard', 'cursorWordStartLeftSelect', {}),
  });
  ed.addAction({
    id: 'facet.cursorWordEndRightSelect',
    label: 'Select Word Right',
    keybindings: [monaco.KeyMod.Shift | monaco.KeyMod.Alt | monaco.KeyCode.RightArrow],
    run: (ed) => ed.trigger('keyboard', 'cursorWordEndRightSelect', {}),
  });

  // Multi-cursor / selection shortcuts — registered explicitly because Monaco
  // standalone doesn't always bind these by default.
  ed.addAction({
    id: 'facet.addSelectionToNextFindMatch',
    label: 'Add Selection to Next Find Match',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyD],
    run: (ed) => ed.trigger('keyboard', 'editor.action.addSelectionToNextFindMatch', {}),
  });
  ed.addAction({
    id: 'facet.selectHighlights',
    label: 'Select All Occurrences of Selection',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyL],
    run: (ed) => ed.trigger('keyboard', 'editor.action.selectHighlights', {}),
  });
  ed.addAction({
    id: 'facet.insertCursorAtEndOfEachLineSelected',
    label: 'Add Cursor to Line Ends',
    keybindings: [monaco.KeyMod.Alt | monaco.KeyMod.Shift | monaco.KeyCode.KeyI],
    run: (ed) => ed.trigger('keyboard', 'editor.action.insertCursorAtEndOfEachLineSelected', {}),
  });

  ed.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Equal, () => {
    const size = ed.getOption(monaco.editor.EditorOption.fontSize);
    ed.updateOptions({ fontSize: Math.min(size + 1, 32) });
  });
  ed.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Minus, () => {
    const size = ed.getOption(monaco.editor.EditorOption.fontSize);
    ed.updateOptions({ fontSize: Math.max(size - 1, 8) });
  });

  // Clear decorations and markers on content change; shift breakpoints when lines are added/removed
  ed.onDidChangeModelContent((e) => {
    if (!suppressChange) {
      errorCollection.clear();
      debugCollection.clear();
      const m = ed.getModel();
      if (m) monaco.editor.setModelMarkers(m, 'facet-check', []);

      const bps = breakpointsPerFile.get(currentSourceKey);
      if (bps && bps.size > 0) {
        const newBps = new Set<number>();
        for (const bp of bps) {
          let delta = 0;
          for (const change of e.changes) {
            if (bp > change.range.endLineNumber) {
              delta += (change.text.match(/\n/g) ?? []).length
                     - (change.range.endLineNumber - change.range.startLineNumber);
            }
          }
          const newLine = bp + delta;
          if (newLine > 0) newBps.add(newLine);
        }
        breakpointsPerFile.set(currentSourceKey, newBps);
        if (onBreakpointChangeCb) {
          // Only notify if something actually changed
          const changed = newBps.size !== bps.size || [...newBps].some(l => !bps.has(l));
          if (changed) onBreakpointChangeCb(currentSourceKey, new Set(newBps));
        }
        refreshBreakpointDecorations();
      }

      if (onChange) onChange();
    }
  });

  // A dot-receiver resolves to either a library alias or a concrete
  // type — completion, sig-help, and hover all need the same answer,
  // so it lives in one helper. Returns null when the text doesn't
  // resolve (treated as "no context" by callers).
  type ReceiverContext =
    | { kind: 'library'; namespace: string }
    | { kind: 'type'; name: string };

  function resolveReceiverContext(text: string): ReceiverContext | null {
    const t = resolveChainType(text);
    if (t) {
      if (t.startsWith('Library:')) return { kind: 'library', namespace: t.slice('Library:'.length) };
      return { kind: 'type', name: t };
    }
    // resolveChainType handles complex chains; the simple-identifier
    // case (`T` is just a name) is covered by varTypes.
    const m = text.match(/([A-Za-z_]\w*)\s*$/);
    if (m) {
      const name = m[1];
      const v = (allVarTypes[currentSourceKey] ?? {})[name];
      if (v) {
        if (v.startsWith('Library:')) return { kind: 'library', namespace: v.slice('Library:'.length) };
        return { kind: 'type', name: v };
      }
      // The receiver text may be a TYPE name itself — `Solid.Move(...)`
      // or `Vec3.Add(...)` constructor-style member access. There's no
      // varTypes entry for a type-as-receiver, but the symbol table
      // says it's a type, which is enough to filter methods by it.
      if (symbols.some(s => s.name === name && s.kind === 'type')) {
        return { kind: 'type', name };
      }
    }
    return null;
  }

  // findReceiverMembers filters symbols by a (kind: 'library' | 'type')
  // context. Used by every provider that needs the members of a dot
  // receiver so the filter predicate cannot drift between them.
  function findReceiverMembers(ctx: ReceiverContext): FacetSymbol[] {
    if (ctx.kind === 'library') {
      // Library exports: things declared in the library, no receiver.
      return symbols.filter(s => s.library === ctx.namespace && !s.receiver);
    }
    // Instance members: methods and fields on the named type.
    return symbols.filter(s => s.receiver === ctx.name);
  }

  // findCallSymbols returns the symbols that match a call expression's
  // function name. callText is what sits before the open paren (or
  // before the dot for member access). All providers route call-site
  // lookup through this so completion, signature help, hover, and
  // param-name completion agree on identity. When the receiver cannot
  // resolve, returns empty — the noisy bare-name fallback would mix
  // methods from arbitrary receivers into hint popups.
  function findCallSymbols(callText: string): FacetSymbol[] {
    const dotIdx = callText.lastIndexOf('.');
    if (dotIdx < 0) {
      // Bare name — must be a top-level entry (no library, no receiver).
      return symbols.filter(s => s.name === callText && !s.library && !s.receiver);
    }
    const receiverText = callText.slice(0, dotIdx);
    const name = callText.slice(dotIdx + 1);
    const ctx = resolveReceiverContext(receiverText);
    if (!ctx) return [];
    return findReceiverMembers(ctx).filter(s => s.name === name);
  }

  // resolveSymbolAtCursor returns the symbols matching the token at the
  // given position. Shared by hover and the Open Documentation action
  // so they always agree on identity. After a dot, the receiver is
  // resolved through resolveReceiverContext / findReceiverMembers;
  // otherwise the in-scope (library === "") matches are preferred over
  // library-imported ones so a user-defined name shadows an imported
  // one with the same identifier.
  function resolveSymbolAtCursor(
    model: monaco.editor.ITextModel,
    position: monaco.Position,
    wordInfo: monaco.editor.IWordAtPosition,
  ): FacetSymbol[] {
    const word = wordInfo.word;
    const lineContent = model.getLineContent(position.lineNumber);
    const charBefore = wordInfo.startColumn > 1 ? lineContent[wordInfo.startColumn - 2] : '';

    if (charBefore === '.') {
      const receiverText = effectiveTextBefore(model, position.lineNumber, wordInfo.startColumn - 1);
      const ctx = resolveReceiverContext(receiverText);
      if (!ctx) return [];
      return findReceiverMembers(ctx).filter(s => s.name === word);
    }

    const byName = symbols.filter(s => s.name === word);
    const local = byName.filter(s => !s.library);
    const pool = local.length > 0 ? local : byName;
    const preferred = pool.find(s => s.kind === 'type' || s.kind === 'keyword')
      ?? pool.find(s => s.kind === 'function')
      ?? pool[0];
    if (!preferred) return [];
    // Keep the full overload set for the preferred name/kind so the
    // caller can show every signature.
    return pool.filter(s => s.kind === preferred.kind);
  }

  function symbolToCompletion(
    s: FacetSymbol,
    range: monaco.Range,
  ): monaco.languages.CompletionItem {
    return {
      label: s.name,
      kind: mapKindToCompletionItemKind(s.kind),
      detail: s.signature || undefined,
      documentation: s.doc || undefined,
      insertText: s.name,
      range,
    };
  }

  // Completion provider — top-level (no dot) and dot-qualified.
  // The data source is the checker's symbol table, filtered by
  // (library, receiver). For dot completion the receiver is resolved
  // by resolveReceiverContext, so the namespace match cannot disagree
  // with the checker's view of which library the alias points at.
  monaco.languages.registerCompletionItemProvider('facet', {
    triggerCharacters: ['.'],
    provideCompletionItems(_model, position) {
      const word = _model.getWordUntilPosition(position);
      const lineContent = _model.getLineContent(position.lineNumber);
      const charBefore = word.startColumn > 1 ? lineContent[word.startColumn - 2] : '';
      const isDot = charBefore === '.';
      const prefix = word.word.toLowerCase();

      const range = new monaco.Range(
        position.lineNumber, word.startColumn,
        position.lineNumber, word.endColumn,
      );

      if (!isDot) {
        // Top-level: only entries that are in scope without a
        // qualifier. Fields and methods need a receiver; library
        // symbols need an alias.
        const matches = symbols.filter(s =>
          !s.library && !s.receiver && s.kind !== 'field' &&
          s.name.toLowerCase().startsWith(prefix)
        );
        return { suggestions: matches.map(s => symbolToCompletion(s, range)) };
      }

      // Dot-qualified. The receiver fragment may span the previous
      // line when the dot starts a chain continuation.
      const textBeforeDot = effectiveTextBefore(_model, position.lineNumber, word.startColumn - 1);
      const ctx = resolveReceiverContext(textBeforeDot);

      const matches: FacetSymbol[] = ctx
        ? findReceiverMembers(ctx).filter(s => s.name.toLowerCase().startsWith(prefix))
        : [];
      // Same (name, kind) can appear in multiple overloads. Collapse
      // for the suggestion list — sig-help shows the overload set
      // once the user picks one.
      const seen = new Set<string>();
      const suggestions: monaco.languages.CompletionItem[] = [];
      for (const s of matches) {
        const key = s.name + '|' + s.kind;
        if (seen.has(key)) continue;
        seen.add(key);
        suggestions.push(symbolToCompletion(s, range));
      }
      return { suggestions };
    },
  });

  // Library path completion — triggers inside `lib "..."` strings
  monaco.languages.registerCompletionItemProvider('facet', {
    triggerCharacters: ['"', '/'],
    async provideCompletionItems(_model, position) {
      const lineContent = _model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);

      // Check if we're inside a lib "..." string
      const libMatch = textBefore.match(/lib\s+"([^"]*)$/);
      if (!libMatch) return { suggestions: [] };

      const typed = libMatch[1];

      // Range covers the text typed so far after the opening quote.
      const startCol = position.column - typed.length;
      const range = new monaco.Range(
        position.lineNumber, startCol,
        position.lineNumber, position.column,
      );

      // If the typed path is `host/user/repo/` (or any deeper subpath
      // ending in a slash), the user is asking for what's *inside* that
      // repo's tree. Look up modules via ListLibraryModules — the bare
      // clone may already have them. This is the path that would have
      // steered the user to `fasteners`, `gears`, etc. instead of
      // landing on the bare `facetlibs@main` import that doesn't exist.
      if (typed.endsWith('/')) {
        const trimmed = typed.replace(/\/+$/, '');
        // Strip any @ref the user already typed before the slash.
        const repoID = trimmed.split('@')[0];
        // ListLibraryModules only knows about `host/user/repo`. A deeper
        // subpath has no module-listing endpoint — bail to empty.
        const segments = repoID.split('/').filter(Boolean);
        if (segments.length === 3) {
          const modules = await ListLibraryModules(repoID).catch(
            e => { console.warn('ListLibraryModules failed:', e); return [] as string[]; }
          );
          if (modules.length > 0) {
            return {
              suggestions: modules.map(m => ({
                label: m,
                kind: monaco.languages.CompletionItemKind.Folder,
                insertText: m,
                range: new monaco.Range(
                  position.lineNumber, position.column,
                  position.lineNumber, position.column,
                ),
              })),
            };
          }
        }
      }

      // Otherwise the user is typing a repo path. Suggest from cached
      // remote repos + local libs, matching whatever prefix was typed.
      const [remote, local] = await Promise.all([
        ListLibraries().catch(e => { console.warn('ListLibraries failed:', e); return []; }),
        ListLocalLibraries().catch(e => { console.warn('ListLocalLibraries failed:', e); return []; }),
      ]);

      const paths: string[] = [];
      for (const lib of remote) {
        // Cached repos no longer expose refs in the settings model; the user
        // types @ref themselves in the lib statement.
        paths.push(lib.id);
      }
      for (const lib of local) {
        paths.push(lib.id);
      }

      const suggestions: monaco.languages.CompletionItem[] = paths
        .filter(p => p.toLowerCase().startsWith(typed.toLowerCase()))
        .map(p => ({
          label: p,
          kind: monaco.languages.CompletionItemKind.Module,
          insertText: p,
          range,
        }));

      return { suggestions };
    },
  });

  // walkBackToCall scans backwards from `before` for the enclosing
  // open paren of a function call, counting commas at depth 0 so
  // signature help and param-name completion agree on what arg slot
  // the cursor is in. Returns null when the cursor is not inside a
  // call (e.g., inside an array literal). String literals and line
  // comments are skipped via buildCodeMask so a `(` in `"look (here"`
  // doesn't get matched as the call's opener.
  function walkBackToCall(before: string): { parenStart: number; commas: number } | null {
    const mask = buildCodeMask(before);
    let depth = 0;
    let commas = 0;
    for (let i = before.length - 1; i >= 0; i--) {
      if (!mask[i]) continue;
      const ch = before[i];
      if (ch === ')' || ch === ']') depth++;
      else if (ch === '(' || ch === '[') {
        if (depth > 0) { depth--; continue; }
        if (ch === '(') return { parenStart: i, commas };
        return null; // '[' at top level — not a function call
      } else if (ch === ',' && depth === 0) commas++;
    }
    return null;
  }

  // Signature help — finds the call expression at the cursor, looks
  // up its overloads through the symbol table, and renders all of
  // them so the user can pick by signature.
  monaco.languages.registerSignatureHelpProvider('facet', {
    signatureHelpTriggerCharacters: ['(', ','],
    provideSignatureHelp(_model, position) {
      const lineContent = _model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);

      const call = walkBackToCall(textBefore);
      if (!call) return null;

      const before = textBefore.slice(0, call.parenStart);
      const nameMatch = before.match(/([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)$/);
      if (!nameMatch) return null;
      const callName = nameMatch[1];

      // Collect ALL overloads of the matched symbols. The checker
      // emits one Symbol per declaration; dedup by (library, signature)
      // so two libraries that happen to mirror the same shape don't
      // silently collapse into one entry.
      const matches = findCallSymbols(callName).filter(s => s.signature);
      if (matches.length === 0) return null;

      const seenSigs = new Set<string>();
      const signatures: monaco.languages.SignatureInformation[] = [];
      for (const s of matches) {
        if (!s.signature) continue;
        const key = (s.library || '') + '|' + s.signature;
        if (seenSigs.has(key)) continue;
        seenSigs.add(key);
        const paramStr = extractParamBody(s.signature);
        if (paramStr === null) continue;
        const params = splitParams(paramStr);
        signatures.push({
          label: s.signature,
          documentation: s.doc || '',
          parameters: params.map(p => ({ label: p, documentation: '' })),
        });
      }
      if (signatures.length === 0) return null;

      return {
        value: {
          signatures,
          activeSignature: 0,
          activeParameter: Math.max(0, call.commas),
        },
        dispose() {},
      };
    },
  });

  // Parameter-name completion. Facet requires named arguments at call
  // sites, so when the cursor is inside `(...)` of a function call we
  // suggest the parameter names with `name: ` insertion. Names already
  // used at the call site are filtered out. Triggers on `(` and `,` so
  // the popup appears automatically as the user opens or continues a
  // call. Shares findCallSymbols with sig-help so both agree on which
  // overloads contribute parameter names.
  monaco.languages.registerCompletionItemProvider('facet', {
    triggerCharacters: ['(', ','],
    provideCompletionItems(_model, position) {
      const lineContent = _model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);

      const call = walkBackToCall(textBefore);
      if (!call) return { suggestions: [] };

      // We must be at the START of an argument slot — the previous
      // non-whitespace char is `(` or `,`. Anything else means we're
      // mid-expression and the top-level completion provider owns
      // the popup.
      let cursor = textBefore.length - 1;
      while (cursor >= 0 && /\s/.test(textBefore[cursor])) cursor--;
      while (cursor >= 0 && /\w/.test(textBefore[cursor])) cursor--;
      while (cursor >= 0 && /\s/.test(textBefore[cursor])) cursor--;
      const slotOpener = cursor >= 0 ? textBefore[cursor] : '';
      if (slotOpener !== '(' && slotOpener !== ',') return { suggestions: [] };

      const before = textBefore.slice(0, call.parenStart);
      const nameMatch = before.match(/([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)$/);
      if (!nameMatch) return { suggestions: [] };
      const callName = nameMatch[1];

      // Collect param names across all matching overloads. Facet's
      // FormatSignature on the Go side emits `name Type` (optionally
      // followed by ` = default`), so the param name is the FIRST
      // identifier in each spec. Splitting on `,` respects nested
      // parens via splitParams so a default like `Vec3(1, 2, 3)`
      // doesn't fragment.
      const paramNames: string[] = [];
      const seenNames = new Set<string>();
      for (const s of findCallSymbols(callName)) {
        if (!s.signature) continue;
        const paramStr = extractParamBody(s.signature);
        if (!paramStr) continue;
        for (const p of splitParams(paramStr)) {
          const idMatch = p.match(/^\s*([A-Za-z_]\w*)/);
          if (!idMatch) continue;
          const pname = idMatch[1];
          if (seenNames.has(pname)) continue;
          seenNames.add(pname);
          paramNames.push(pname);
        }
      }
      if (paramNames.length === 0) return { suggestions: [] };

      // Filter out param names already used at this call site.
      const argsRegion = textBefore.slice(call.parenStart + 1);
      const used = new Set<string>();
      for (const m of argsRegion.matchAll(/([A-Za-z_]\w*)\s*:/g)) {
        used.add(m[1]);
      }
      const available = paramNames.filter(n => !used.has(n));
      if (available.length === 0) return { suggestions: [] };

      const word = _model.getWordUntilPosition(position);
      const range = new monaco.Range(
        position.lineNumber, word.startColumn,
        position.lineNumber, word.endColumn,
      );

      const suggestions: monaco.languages.CompletionItem[] = available.map(name => ({
        label: name + ':',
        kind: monaco.languages.CompletionItemKind.Property,
        insertText: name + ': ',
        // High sort priority so param names come before random top-level
        // names. Monaco sorts ascending lexicographically.
        sortText: '0_' + name,
        range,
      }));

      return { suggestions };
    },
  });

  // Hover tooltips — show signature and doc for functions, methods,
  // types, fields, and keywords. Identity resolution is shared with
  // the Open Documentation action via resolveSymbolAtCursor, so the
  // two cannot disagree about which symbol the cursor points at.
  //
  // For local bindings (params, vars) there is no Symbol, but the
  // checker has stamped a reference whose returnType is the declared
  // type. That feeds a synthesized tooltip so the user still sees a
  // useful type annotation.
  monaco.languages.registerHoverProvider('facet', {
    provideHover(model, position) {
      const wordInfo = model.getWordAtPosition(position);
      if (!wordInfo) return null;
      const word = wordInfo.word;

      const matches = resolveSymbolAtCursor(model, position, wordInfo);

      // No symbol match — try the local-binding fallback via the
      // checker's references map. Params, vars, and field accesses
      // on local values land here.
      if (matches.length === 0) {
        const refKey = `${currentSourceKey}:${position.lineNumber}:${wordInfo.startColumn}`;
        const ref = references[refKey];
        if (ref?.returnType) {
          return {
            range: new monaco.Range(
              position.lineNumber, wordInfo.startColumn,
              position.lineNumber, wordInfo.endColumn,
            ),
            contents: [{ value: '```facet\n' + word + ' ' + ref.returnType + '\n```' }],
          };
        }
        return null;
      }

      // Collect overload signatures (deduped per library so two libs
      // exporting the same signature don't silently collapse).
      // Documentation comes from the first symbol that has one —
      // overloads typically share a comment block.
      const overloads: string[] = [];
      const seenSigs = new Set<string>();
      let docText = '';
      for (const s of matches) {
        if (s.signature) {
          const key = (s.library || '') + '|' + s.signature;
          if (!seenSigs.has(key)) {
            seenSigs.add(key);
            overloads.push(s.signature);
          }
        }
        if (!docText && s.doc) docText = s.doc;
      }

      const parts: string[] = [];
      if (overloads.length > 0) {
        parts.push('```facet\n' + overloads.join('\n') + '\n```');
      }
      if (docText) {
        parts.push(docText);
      }
      if (parts.length === 0) return null;

      return {
        range: new monaco.Range(
          position.lineNumber, wordInfo.startColumn,
          position.lineNumber, wordInfo.endColumn,
        ),
        contents: parts.map(value => ({ value })),
      };
    },
  });

  return {
    getContent(): string {
      return ed.getModel()!.getValue();
    },

    getAllSources(): Record<string, string> {
      const result: Record<string, string> = {};
      for (const [key, model] of models) {
        result[key] = model.getValue();
      }
      return result;
    },

    highlightError(line: number) {
      setLineDecoration(errorCollection, line, 'monaco-error-line');
    },

    clearError() {
      errorCollection.clear();
    },

    setMarkers(errors: CheckError[]) {
      const m = ed.getModel()!;
      const lineCount = m.getLineCount();
      const markers: monaco.editor.IMarkerData[] = errors
        .filter(e => e.line > 0 && e.line <= lineCount && (e.file ?? '') === currentSourceKey)
        .map(e => {
          const lineLength = m.getLineLength(e.line);
          const startCol = e.col > 0 ? e.col : 1;
          const endCol = e.endCol > 0 ? e.endCol + 1 : lineLength + 1;
          return {
            severity: monaco.MarkerSeverity.Error,
            message: e.message,
            startLineNumber: e.line,
            startColumn: startCol,
            endLineNumber: e.line,
            endColumn: endCol,
          };
        });
      monaco.editor.setModelMarkers(m, 'facet-check', markers);
    },

    clearMarkers() {
      monaco.editor.setModelMarkers(ed.getModel()!, 'facet-check', []);
    },

    highlightDebugLine(line: number) {
      setLineDecoration(debugCollection, line, 'monaco-debug-line');
      if (line >= 1 && line <= ed.getModel()!.getLineCount()) {
        ed.revealLineInCenter(line);
      }
    },

    clearDebugLine() {
      debugCollection.clear();
    },

    setContent(text: string) {
      replaceContent(text, false);
    },

    setContentSilent(text: string) {
      replaceContent(text, true);
    },

    switchModel(fileKey: string, content: string) {
      let m = models.get(fileKey);
      if (!m) {
        m = monaco.editor.createModel(content, 'facet');
        models.set(fileKey, m);
      }
      if (ed.getModel() !== m) {
        suppressChange = true;
        try {
          ed.setModel(m);
        } finally {
          suppressChange = false;
        }
        refreshBreakpointDecorations();
      }
    },

    preloadModel(fileKey: string, content: string) {
      if (!models.has(fileKey)) {
        models.set(fileKey, monaco.editor.createModel(content, 'facet'));
      }
    },

    disposeModel(fileKey: string) {
      const m = models.get(fileKey);
      if (!m) return;
      // The caller must switch the editor to another model before
      // calling this — disposing the displayed model leaves the
      // editor on a disposed handle. Throwing surfaces the caller bug
      // immediately instead of letting the editor enter a broken
      // state that crashes later from somewhere unrelated.
      if (ed.getModel() === m) {
        throw new Error(`disposeModel(${fileKey}): cannot dispose the displayed model; switchModel first`);
      }
      m.dispose();
      models.delete(fileKey);
    },

    setReadOnly(ro: boolean) {
      ed.updateOptions({ readOnly: ro });
    },

    setWordWrap(on: boolean) {
      ed.updateOptions({ wordWrap: on ? 'on' : 'off' });
    },

    setTheme(name: string) {
      monaco.editor.setTheme(name);
    },

    updateSymbols(syms: FacetSymbol[]) {
      symbols = syms;
    },

    updateVarTypes(types: Record<string, Record<string, string>>) {
      allVarTypes = types;
    },

    setCurrentSource(sourceKey: string) {
      currentSourceKey = sourceKey;
    },

    updateDeclarations(decls: Record<string, DeclLocation>) {
      declarations = decls;
    },

    updateReferences(refs: Record<string, DeclLocation>) {
      references = refs;
    },

    updateFileSources(sources: Record<string, string>) {
      fileSources = sources;
    },

    getCursorPosition(): { lineNumber: number; column: number } {
      const pos = ed.getPosition();
      return pos ? { lineNumber: pos.lineNumber, column: pos.column } : { lineNumber: 1, column: 1 };
    },

    onCursorChange(cb: (line: number, col: number) => void) {
      ed.onDidChangeCursorPosition((e) => {
        cb(e.position.lineNumber, e.position.column);
      });
    },

    onMouseMove(cb: (line: number, col: number) => void) {
      ed.onMouseMove((e) => {
        if (e.target.position) {
          cb(e.target.position.lineNumber, e.target.position.column);
        }
      });
    },

    onMouseLeave(cb: () => void) {
      ed.getDomNode()?.addEventListener('mouseleave', cb);
    },

    revealLine(line: number, col?: number) {
      if (line < 1 || line > ed.getModel()!.getLineCount()) return;
      ed.setPosition({ lineNumber: line, column: col ?? 1 });
      ed.revealLineInCenter(line);
      ed.focus();
    },

    setBreakpointMode(enabled: boolean) {
      breakpointClickEnabled = enabled;
      const editorDom = ed.getDomNode();
      if (!enabled) {
        gutterCollection.clear();
        editorDom?.classList.remove('facet-debug-mode');
        ed.updateOptions({ glyphMargin: false, lineNumbersMinChars: 4 });
      } else {
        editorDom?.classList.add('facet-debug-mode');
        ed.updateOptions({ glyphMargin: true, lineNumbersMinChars: 1 });
        refreshBreakpointDecorations();
      }
    },

    setValidBreakpointLines(file: string, lines: Set<number>) {
      validLinesPerFile.set(file, lines);
      if (file === currentSourceKey) refreshBreakpointDecorations();
    },

    syncBreakpoints(file: string, lines: Set<number>) {
      breakpointsPerFile.set(file, new Set(lines));
      if (file === currentSourceKey) refreshBreakpointDecorations();
    },

    onBreakpointChange(cb: (file: string, lines: Set<number>) => void) {
      onBreakpointChangeCb = cb;
    },

    onJumpToLine(cb: (file: string, line: number) => void) {
      onJumpToLineCb = cb;
    },
  };
}
