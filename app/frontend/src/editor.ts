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
import 'monaco-editor/esm/vs/editor/contrib/wordHighlighter/browser/wordHighlighter';
import 'monaco-editor/esm/vs/editor/contrib/clipboard/browser/clipboard';
import 'monaco-editor/esm/vs/editor/contrib/links/browser/links';
import 'monaco-editor/esm/vs/editor/contrib/comment/browser/comment';

import 'monaco-editor/esm/vs/editor/contrib/parameterHints/browser/parameterHints';
import { registerFacetLanguage } from './facet-language';
import { registerThemes } from './themes';
import { ListLibraries, ListLocalLibraries } from '../wailsjs/go/main/App';
import type { DocEntry } from './docs';
import type { DeclLocation } from './eval-client';

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
  'editor.lineHighlightBackground': '#f5f5f5',
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
  'editor.lineHighlightBackground': '#252525',
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
  updateDocIndex(entries: DocEntry[]): void;
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
  onOpenDocs?: (name: string) => void,
  onGoToFile?: (file: string, source: string, line: number, col: number) => void,
  initialFileKey?: string,
): EditorHandle {
  let docEntries: DocEntry[] = [];
  let allVarTypes: Record<string, Record<string, string>> = {};
  let mainKey = initialFileKey || '';
  let currentSourceKey = mainKey;
  let declarations: Record<string, DeclLocation> = {};
  // references maps "file:line:col" of a referring token to the declaration it
  // resolves to. Built by the checker on the backend, refreshed on every eval.
  // Used by findDecl (goto-definition) and the hover provider.
  let references: Record<string, DeclLocation> = {};
  let fileSources: Record<string, string> = {};
  let suppressChange = false;

  const models = new Map<string, monaco.editor.ITextModel>();
  const mainModel = monaco.editor.createModel(initialDoc, 'facet');
  models.set(mainKey, mainModel);

  const ed = monaco.editor.create(parent, {
    model: mainModel,
    theme: 'facet-orange-light',
    automaticLayout: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: true,
    fontSize: 14,
    tabSize: 4,
    insertSpaces: true,
    lineNumbers: 'on',
    renderLineHighlight: 'line',
    overviewRulerLanes: 0,
    hideCursorInOverviewRuler: true,
    fixedOverflowWidgets: true,
    wordWrap: 'on',
    scrollbar: {
      vertical: 'auto',
      horizontal: 'hidden',
    },
  });

  const errorCollection = ed.createDecorationsCollection();
  const debugCollection = ed.createDecorationsCollection();

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
        const word = ed.getModel()?.getWordAtPosition(pos);
        if (!word) return;
        const wordText = word.word;
        const lineContent = ed.getModel()?.getLineContent(pos.lineNumber) ?? '';
        const charBefore = word.startColumn > 1 ? lineContent[word.startColumn - 2] : '';
        const match = charBefore === '.'
          ? (docEntries.find(e => e.name.endsWith('.' + wordText)) ?? docEntries.find(e => e.name === wordText))
          : docEntries.find(e => e.name === wordText);
        if (match) onOpenDocs(match.name);
      },
    });
  }

  // effectiveTextBefore returns the text before a given 1-based column on
  // `line`. If the text before the column is whitespace-only (e.g. the line
  // starts with a `.` chain continuation), walk backwards to the previous
  // non-empty line and return its trimmed-right content instead — so callers
  // can resolve the receiver expression that lives on an earlier line.
  function effectiveTextBefore(
    model: monaco.editor.ITextModel,
    lineNumber: number,
    column: number,
  ): string {
    const raw = model.getLineContent(lineNumber).slice(0, column - 1);
    if (raw.trim().length > 0) return raw;
    for (let ln = lineNumber - 1; ln >= 1; ln--) {
      const prev = model.getLineContent(ln).trimEnd();
      if (prev.length > 0) return prev;
    }
    return raw;
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

    const file = currentSourceKey === mainKey ? '' : currentSourceKey;
    const key = `${file}:${pos.lineNumber}:${word.startColumn}`;
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

  // Clear decorations and markers on content change
  ed.onDidChangeModelContent(() => {
    if (!suppressChange) {
      errorCollection.clear();
      debugCollection.clear();
      const m = ed.getModel();
      if (m) monaco.editor.setModelMarkers(m, 'facet-check', []);
      if (onChange) onChange();
    }
  });

  // Completion provider
  monaco.languages.registerCompletionItemProvider('facet', {
    triggerCharacters: ['.'],
    provideCompletionItems(_model, position) {
      const word = _model.getWordUntilPosition(position);
      const lineContent = _model.getLineContent(position.lineNumber);
      const charBefore = word.startColumn > 1 ? lineContent[word.startColumn - 2] : '';
      const isDot = charBefore === '.';

      const range = new monaco.Range(
        position.lineNumber, word.startColumn,
        position.lineNumber, word.endColumn,
      );

      let suggestions: monaco.languages.CompletionItem[];

      if (isDot) {
        const prefix = word.word.toLowerCase();

        // Extract receiver type before the dot — use resolveChainType for
        // complex expressions (e.g. arr[0].x, foo().bar.) and fall back to
        // a simple variable name lookup for the common case. When the dot
        // starts a continuation line, grab the receiver from the preceding
        // non-empty line.
        const textBeforeDot = effectiveTextBefore(_model, position.lineNumber, word.startColumn - 1);
        const receiverMatch = textBeforeDot.match(/([A-Za-z_]\w*)$/);
        const receiverName = receiverMatch ? receiverMatch[1] : '';
        const receiverType = resolveChainType(textBeforeDot) || (receiverName && (allVarTypes[currentSourceKey] ?? {})[receiverName]) || '';

        if (receiverType.startsWith('Library:')) {
          // Library namespace: show the library's exported functions and types
          const libNs = receiverType.slice('Library:'.length);
          suggestions = docEntries
            .filter(e => e.library === libNs && !e.name.includes('.') && e.name.toLowerCase().startsWith(prefix))
            .map(e => ({
              label: e.name,
              kind: mapKindToCompletionItemKind(e.kind),
              detail: e.signature || undefined,
              documentation: e.doc || undefined,
              insertText: e.name,
              range,
            }));
        } else {
          const methodSuggestions = docEntries
            .filter(e => {
              const dotIdx = e.name.indexOf('.');
              if (dotIdx < 0) return false;
              if (!e.name.slice(dotIdx + 1).toLowerCase().startsWith(prefix)) return false;
              // If we know the receiver type, filter to matching entries
              if (receiverType) {
                return e.name.slice(0, dotIdx) === receiverType;
              }
              return true;
            })
            .map(e => ({
              label: e.name.slice(e.name.indexOf('.') + 1),
              kind: mapKindToCompletionItemKind(e.kind),
              detail: e.signature || undefined,
              documentation: e.doc || undefined,
              insertText: e.name.slice(e.name.indexOf('.') + 1),
              range,
            }));

          // Deduplicate
          const seen = new Set<string>();
          suggestions = methodSuggestions.filter(s => {
            const lbl = s.label as string;
            if (seen.has(lbl)) return false;
            seen.add(lbl);
            return true;
          });
        }
      } else {
        suggestions = docEntries
          .filter(e => !e.name.includes('.'))
          .map(e => ({
            label: e.name,
            kind: mapKindToCompletionItemKind(e.kind),
            detail: e.signature || undefined,
            documentation: e.doc || undefined,
            insertText: e.name,
            range,
          }));
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

      // Fetch installed libraries
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

      // Range covers the text typed so far after the opening quote
      const startCol = position.column - typed.length;
      const range = new monaco.Range(
        position.lineNumber, startCol,
        position.lineNumber, position.column,
      );

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

  // Signature help (parameter hints) — shows active parameter when typing inside ()
  monaco.languages.registerSignatureHelpProvider('facet', {
    signatureHelpTriggerCharacters: ['(', ','],
    provideSignatureHelp(_model, position) {
      const lineContent = _model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);

      // Walk backwards to find the matching open paren and count commas
      let depth = 0;
      let commas = 0;
      let parenStart = -1;
      for (let i = textBefore.length - 1; i >= 0; i--) {
        const ch = textBefore[i];
        if (ch === ')' || ch === ']') depth++;
        else if (ch === '(' || ch === '[') {
          if (depth > 0) { depth--; continue; }
          if (ch === '(') { parenStart = i; break; }
          break; // '[' at top level — not a function call
        } else if (ch === ',' && depth === 0) commas++;
      }
      if (parenStart < 0) return null;

      // Extract the function name before the paren
      const before = textBefore.slice(0, parenStart);
      const nameMatch = before.match(/([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)$/);
      if (!nameMatch) return null;
      const callName = nameMatch[1];

      // Look up in doc entries — try dotted name first, then bare name
      const parts = callName.split('.');
      const bareName = parts[parts.length - 1];
      let entry = docEntries.find(e => e.name === callName && e.signature);
      if (!entry && parts.length > 1) {
        // Try receiver lookup: T.Func → look for Type.Func using varTypes
        const receiverType = (allVarTypes[currentSourceKey] ?? {})[parts[0]] || parts[0];
        entry = docEntries.find(e => e.name === receiverType + '.' + bareName && e.signature);
      }
      if (!entry) {
        entry = docEntries.find(e => e.name === bareName && e.signature);
      }
      if (!entry || !entry.signature) return null;

      // Parse signature: "fn Name(Type param, ...) ReturnType" or "fn Name(param, ...)"
      const sigMatch = entry.signature.match(/\(([^)]*)\)/);
      if (!sigMatch) return null;
      const paramStr = sigMatch[1].trim();
      if (!paramStr) return null;

      const params = paramStr.split(',').map(p => p.trim());
      const parameters: monaco.languages.ParameterInformation[] = params.map(p => ({
        label: p,
        documentation: '',
      }));

      return {
        value: {
          signatures: [{
            label: entry.signature,
            documentation: entry.doc || '',
            parameters,
          }],
          activeSignature: 0,
          activeParameter: Math.min(commas, parameters.length - 1),
        },
        dispose() {},
      };
    },
  });

  // Hover tooltips — show signature and doc for functions, methods, types, and keywords.
  //
  // Primary path: the checker's references map points at the exact declaration
  // for the token under the cursor. We then find the matching DocEntry by
  // looking up the same declaration in the declarations map and comparing
  // positions — so Number-the-type and Number-the-function never get confused.
  //
  // Fallback path: if no reference exists at this position (typically because
  // the cursor is on a keyword — keywords aren't AST nodes and aren't recorded
  // in references), fall back to a by-name match preferring type/keyword
  // entries so type annotations like `x Number` still show the type.
  monaco.languages.registerHoverProvider('facet', {
    provideHover(model, position) {
      const wordInfo = model.getWordAtPosition(position);
      if (!wordInfo) return null;
      const word = wordInfo.word;

      const file = currentSourceKey === mainKey ? '' : currentSourceKey;
      const key = `${file}:${position.lineNumber}:${wordInfo.startColumn}`;
      const ref = references[key];

      let entry: DocEntry | undefined;
      if (ref) {
        // Decl-identity match: find the DocEntry whose own declaration sits at
        // the same (file, line, col) the reference points to.
        const refFile = ref.file ?? '';
        entry = docEntries.find(e => {
          const decl = declarations[e.name];
          return decl && decl.line === ref.line && decl.col === ref.col && (decl.file ?? '') === refFile;
        });
      } else {
        // No reference — typically a keyword. Prefer type/keyword entries so
        // hovering `Number` in `x Number` doesn't surface the function.
        const matches = docEntries.filter(e => e.name === word);
        entry = matches.find(e => e.kind === 'type' || e.kind === 'keyword')
          ?? matches.find(e => e.kind === 'function')
          ?? matches[0];
      }

      // For local bindings (param, var, const, field access on a local) the
      // checker has a ref but no DocEntry. Synthesize a minimal tooltip from
      // the reference's ReturnType so the user still sees the declared type.
      if (!entry && ref?.returnType) {
        return {
          range: new monaco.Range(
            position.lineNumber, wordInfo.startColumn,
            position.lineNumber, wordInfo.endColumn,
          ),
          contents: [{ value: '```facet\n' + word + ' ' + ref.returnType + '\n```' }],
        };
      }

      if (!entry) return null;

      const parts: string[] = [];
      if (entry.signature) {
        parts.push('```facet\n' + entry.signature + '\n```');
      }
      if (entry.doc) {
        parts.push(entry.doc);
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
      const activeFile = currentSourceKey === mainKey ? '' : currentSourceKey;
      const markers: monaco.editor.IMarkerData[] = errors
        .filter(e => e.line > 0 && e.line <= lineCount && (e.file ?? '') === activeFile)
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
      if (ed.getModel() === m) {
        const main = models.get(mainKey);
        if (main) ed.setModel(main);
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

    updateDocIndex(entries: DocEntry[]) {
      docEntries = entries;
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
  };
}
