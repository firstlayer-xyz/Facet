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

// Define the built-in light theme — white bg, dark text, orange accent.
monaco.editor.defineTheme('facet-orange-light', {
  base: 'vs',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: '7c3aed' },
    { token: 'keyword.other.unit', foreground: '7c3aed' },
    { token: 'support.type', foreground: '0d7377' },
    { token: 'constant.numeric', foreground: 'e45500' },
    { token: 'constant.numeric.float', foreground: 'e45500' },
    { token: 'constant.language', foreground: 'e45500' },
    { token: 'string.quoted.double', foreground: '2a7e3b' },
    { token: 'string.quoted.other', foreground: '2a7e3b' },
    { token: 'comment.line', foreground: '999999' },
    { token: 'keyword.operator', foreground: '0d7377' },
    { token: 'variable.other', foreground: '3a3a3a' },
    { token: 'punctuation.delimiter', foreground: '3a3a3a' },
  ],
  colors: {
    'editor.background': '#ffffff',
    'editor.foreground': '#3a3a3a',
    'editorWidget.background': '#f7f7f7',
    'editorWidget.border': '#d0d0d0',
    'editorSuggestWidget.background': '#f7f7f7',
    'editorSuggestWidget.foreground': '#3a3a3a',
    'editorSuggestWidget.border': '#d0d0d0',
    'editorSuggestWidget.selectedBackground': '#e8e8e8',
    'editorSuggestWidget.selectedForeground': '#1a1a1a',
    'editorSuggestWidget.highlightForeground': '#e45500',
    'editorSuggestWidget.focusHighlightForeground': '#e45500',
    'editorHoverWidget.background': '#f7f7f7',
    'editorHoverWidget.foreground': '#3a3a3a',
    'editorHoverWidget.border': '#d0d0d0',
    'editor.lineHighlightBackground': '#f5f5f5',
    'editorLineNumber.foreground': '#b0b0b0',
    'editorCursor.foreground': '#ff6d00',
    'editor.selectionBackground': '#ffe0c0',
  },
});

// Define the built-in dark theme matching the app palette.
// Token names use standard TextMate scope names so this theme stays compatible
// with the same Monarch tokenizer used by the VS Code themes.
monaco.editor.defineTheme('facet-orange-dark', {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: 'c49cff' },
    { token: 'keyword.other.unit', foreground: 'c49cff' },
    { token: 'support.type', foreground: '4dd4d8' },
    { token: 'constant.numeric', foreground: 'ffaa44' },
    { token: 'constant.numeric.float', foreground: 'ffaa44' },
    { token: 'constant.language', foreground: 'ffaa44' },
    { token: 'string.quoted.double', foreground: '7ecc8d' },
    { token: 'string.quoted.other', foreground: '7ecc8d' },
    { token: 'comment.line', foreground: '666666' },
    { token: 'keyword.operator', foreground: '4dd4d8' },
    { token: 'variable.other', foreground: 'cccccc' },
    { token: 'punctuation.delimiter', foreground: 'cccccc' },
  ],
  colors: {
    'editor.background': '#1a1a1a',
    'editor.foreground': '#cccccc',
    'editorWidget.background': '#252525',
    'editorWidget.border': '#3a3a3a',
    'editorSuggestWidget.background': '#252525',
    'editorSuggestWidget.foreground': '#cccccc',
    'editorSuggestWidget.border': '#3a3a3a',
    'editorSuggestWidget.selectedBackground': '#3a3a3a',
    'editorSuggestWidget.selectedForeground': '#eeeeee',
    'editorSuggestWidget.highlightForeground': '#ffaa44',
    'editorSuggestWidget.focusHighlightForeground': '#ffaa44',
    'editorHoverWidget.background': '#252525',
    'editorHoverWidget.foreground': '#cccccc',
    'editorHoverWidget.border': '#3a3a3a',
    'editor.lineHighlightBackground': '#252525',
    'editorLineNumber.foreground': '#555555',
    'editorCursor.foreground': '#ff6d00',
    'editor.selectionBackground': '#553a1a',
  },
});

monaco.editor.defineTheme('facet-green-light', {
  base: 'vs',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: '7c3aed' },
    { token: 'keyword.other.unit', foreground: '7c3aed' },
    { token: 'support.type', foreground: '0d7377' },
    { token: 'constant.numeric', foreground: '1e8a3e' },
    { token: 'constant.numeric.float', foreground: '1e8a3e' },
    { token: 'constant.language', foreground: '1e8a3e' },
    { token: 'string.quoted.double', foreground: '2a7e3b' },
    { token: 'string.quoted.other', foreground: '2a7e3b' },
    { token: 'comment.line', foreground: '999999' },
    { token: 'keyword.operator', foreground: '0d7377' },
    { token: 'variable.other', foreground: '3a3a3a' },
    { token: 'punctuation.delimiter', foreground: '3a3a3a' },
  ],
  colors: {
    'editor.background': '#ffffff',
    'editor.foreground': '#3a3a3a',
    'editorWidget.background': '#f7f7f7',
    'editorWidget.border': '#d0d0d0',
    'editorSuggestWidget.background': '#f7f7f7',
    'editorSuggestWidget.foreground': '#3a3a3a',
    'editorSuggestWidget.border': '#d0d0d0',
    'editorSuggestWidget.selectedBackground': '#e8e8e8',
    'editorSuggestWidget.selectedForeground': '#1a1a1a',
    'editorSuggestWidget.highlightForeground': '#1e8a3e',
    'editorSuggestWidget.focusHighlightForeground': '#1e8a3e',
    'editorHoverWidget.background': '#f7f7f7',
    'editorHoverWidget.foreground': '#3a3a3a',
    'editorHoverWidget.border': '#d0d0d0',
    'editor.lineHighlightBackground': '#f5f5f5',
    'editorLineNumber.foreground': '#b0b0b0',
    'editorCursor.foreground': '#1e8a3e',
    'editor.selectionBackground': '#c8f0d4',
  },
});

monaco.editor.defineTheme('facet-green-dark', {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: 'c49cff' },
    { token: 'keyword.other.unit', foreground: 'c49cff' },
    { token: 'support.type', foreground: '4dd4d8' },
    { token: 'constant.numeric', foreground: '52c878' },
    { token: 'constant.numeric.float', foreground: '52c878' },
    { token: 'constant.language', foreground: '52c878' },
    { token: 'string.quoted.double', foreground: '7ecc8d' },
    { token: 'string.quoted.other', foreground: '7ecc8d' },
    { token: 'comment.line', foreground: '666666' },
    { token: 'keyword.operator', foreground: '4dd4d8' },
    { token: 'variable.other', foreground: 'cccccc' },
    { token: 'punctuation.delimiter', foreground: 'cccccc' },
  ],
  colors: {
    'editor.background': '#1a1a1a',
    'editor.foreground': '#cccccc',
    'editorWidget.background': '#252525',
    'editorWidget.border': '#3a3a3a',
    'editorSuggestWidget.background': '#252525',
    'editorSuggestWidget.foreground': '#cccccc',
    'editorSuggestWidget.border': '#3a3a3a',
    'editorSuggestWidget.selectedBackground': '#3a3a3a',
    'editorSuggestWidget.selectedForeground': '#eeeeee',
    'editorSuggestWidget.highlightForeground': '#52c878',
    'editorSuggestWidget.focusHighlightForeground': '#52c878',
    'editorHoverWidget.background': '#252525',
    'editorHoverWidget.foreground': '#cccccc',
    'editorHoverWidget.border': '#3a3a3a',
    'editor.lineHighlightBackground': '#252525',
    'editorLineNumber.foreground': '#555555',
    'editorCursor.foreground': '#2eb84e',
    'editor.selectionBackground': '#1a3a22',
  },
});

monaco.editor.defineTheme('facet-digital-blue-light', {
  base: 'vs',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: '7c3aed' },
    { token: 'keyword.other.unit', foreground: '7c3aed' },
    { token: 'support.type', foreground: '0d7377' },
    { token: 'constant.numeric', foreground: '0060c8' },
    { token: 'constant.numeric.float', foreground: '0060c8' },
    { token: 'constant.language', foreground: '0060c8' },
    { token: 'string.quoted.double', foreground: '2a7e3b' },
    { token: 'string.quoted.other', foreground: '2a7e3b' },
    { token: 'comment.line', foreground: '999999' },
    { token: 'keyword.operator', foreground: '0d7377' },
    { token: 'variable.other', foreground: '3a3a3a' },
    { token: 'punctuation.delimiter', foreground: '3a3a3a' },
  ],
  colors: {
    'editor.background': '#ffffff',
    'editor.foreground': '#3a3a3a',
    'editorWidget.background': '#f7f7f7',
    'editorWidget.border': '#d0d0d0',
    'editorSuggestWidget.background': '#f7f7f7',
    'editorSuggestWidget.foreground': '#3a3a3a',
    'editorSuggestWidget.border': '#d0d0d0',
    'editorSuggestWidget.selectedBackground': '#e8e8e8',
    'editorSuggestWidget.selectedForeground': '#1a1a1a',
    'editorSuggestWidget.highlightForeground': '#0060c8',
    'editorSuggestWidget.focusHighlightForeground': '#0060c8',
    'editorHoverWidget.background': '#f7f7f7',
    'editorHoverWidget.foreground': '#3a3a3a',
    'editorHoverWidget.border': '#d0d0d0',
    'editor.lineHighlightBackground': '#f5f5f5',
    'editorLineNumber.foreground': '#b0b0b0',
    'editorCursor.foreground': '#0060c8',
    'editor.selectionBackground': '#c8d8f8',
  },
});

monaco.editor.defineTheme('facet-digital-blue-dark', {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: 'keyword.control', foreground: 'c49cff' },
    { token: 'keyword.other.unit', foreground: 'c49cff' },
    { token: 'support.type', foreground: '4dd4d8' },
    { token: 'constant.numeric', foreground: '5aa8ff' },
    { token: 'constant.numeric.float', foreground: '5aa8ff' },
    { token: 'constant.language', foreground: '5aa8ff' },
    { token: 'string.quoted.double', foreground: '7ecc8d' },
    { token: 'string.quoted.other', foreground: '7ecc8d' },
    { token: 'comment.line', foreground: '666666' },
    { token: 'keyword.operator', foreground: '4dd4d8' },
    { token: 'variable.other', foreground: 'cccccc' },
    { token: 'punctuation.delimiter', foreground: 'cccccc' },
  ],
  colors: {
    'editor.background': '#1a1a1a',
    'editor.foreground': '#cccccc',
    'editorWidget.background': '#252525',
    'editorWidget.border': '#3a3a3a',
    'editorSuggestWidget.background': '#252525',
    'editorSuggestWidget.foreground': '#cccccc',
    'editorSuggestWidget.border': '#3a3a3a',
    'editorSuggestWidget.selectedBackground': '#3a3a3a',
    'editorSuggestWidget.selectedForeground': '#eeeeee',
    'editorSuggestWidget.highlightForeground': '#5aa8ff',
    'editorSuggestWidget.focusHighlightForeground': '#5aa8ff',
    'editorHoverWidget.background': '#252525',
    'editorHoverWidget.foreground': '#cccccc',
    'editorHoverWidget.border': '#3a3a3a',
    'editor.lineHighlightBackground': '#252525',
    'editorLineNumber.foreground': '#555555',
    'editorCursor.foreground': '#3d96ff',
    'editor.selectionBackground': '#1a2a4a',
  },
});

export interface DocEntry {
  name: string;
  signature: string;
  doc: string;
  kind: string;
  library: string;
}

export interface CheckError {
  file: string;
  line: number;
  col: number;
  endCol: number;
  message: string;
}

export interface EditorHandle {
  getContent(): string;
  getModelContent(fileKey: string): string;
  highlightError(line: number): void;
  clearError(): void;
  setMarkers(errors: CheckError[]): void;
  clearMarkers(): void;
  highlightDebugLine(line: number): void;
  clearDebugLine(): void;
  setContent(text: string): void;
  setContentSilent(text: string): void;
  switchModel(fileKey: string, content: string): void;
  resetMainModel(key: string, content: string): void;
  disposeModel(fileKey: string): void;
  setReadOnly(ro: boolean): void;
  setWordWrap(on: boolean): void;
  setTheme(name: string): void;
  updateDocIndex(entries: DocEntry[]): void;
  updateVarTypes(types: Record<string, Record<string, string>>): void;
  setCurrentSource(sourceKey: string): void;
  updateDeclarations(decls: Record<string, { line: number; col: number; file?: string }>, sources?: Record<string, string>): void;
  getCursorPosition(): { lineNumber: number; column: number };
  onCursorChange(cb: (line: number, col: number) => void): void;
  onMouseMove(cb: (line: number, col: number) => void): void;
  onMouseLeave(cb: () => void): void;
  revealLine(line: number, col?: number): void;
  undo(): void;
  redo(): void;
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
  let declarations: Record<string, { line: number; col: number; file?: string; kind?: string; returnType?: string }> = {};
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

  // Register command for hover "→ Open in Docs" links and context menu
  let openDocsCmdId: string | null = null;
  if (onOpenDocs) {
    openDocsCmdId = ed.addCommand(0, (_ctx, name: string) => {
      onOpenDocs(name);
    }) ?? null;

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

  // resolveChainType returns the type of the expression represented by the
  // given text fragment. Handles arbitrary chain depth by recursing.
  // e.g. "Cube(1,1,1).Translate(1,0,0)" → "Solid"
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

    // Identifier: look up variable type
    const identMatch = text.match(/([A-Za-z_]\w*)$/);
    if (identMatch) {
      return (allVarTypes[currentSourceKey] ?? {})[identMatch[1]] || null;
    }

    return null;
  }

  // Go to Declaration — deterministic context-based dispatch
  function findDecl(editor: monaco.editor.ICodeEditor): { line: number; col: number; file?: string } | null {
    const pos = editor.getPosition();
    if (!pos) return null;
    const mdl = editor.getModel();
    if (!mdl) return null;
    const word = mdl.getWordAtPosition(pos);
    if (!word) return null;
    const name = word.word;
    const lineContent = mdl.getLineContent(pos.lineNumber);
    const charAfter = lineContent[word.endColumn - 1] || '';
    const isCallSite = charAfter === '(';
    const charBefore = word.startColumn > 1 ? lineContent[word.startColumn - 2] : '';

    if (charBefore === '.') {
      // Dotted context — resolve the receiver type, then look up Type.Name
      const textBefore = lineContent.slice(0, word.startColumn - 2);
      const receiverType = resolveChainType(textBefore);
      if (receiverType) {
        const decl = declarations[receiverType + '.' + name];
        if (decl) return decl;
      }

      // Direct key lookup for library vars: "K.Knurl", "F.Thumbscrew"
      const receiverMatch = textBefore.match(/([A-Za-z_]\w*)$/);
      if (receiverMatch) {
        const qualKey = receiverMatch[1] + '.' + name;
        const decl = declarations[qualKey];
        const structDecl = declarations['struct:' + qualKey];
        if (decl && structDecl) {
          return isCallSite ? decl : structDecl;
        }
        if (decl || structDecl) return (decl || structDecl)!;
      }

      return null; // no match — don't guess
    }

    // Bare name — not after a dot
    const decl = declarations[name];
    const structDecl = declarations['struct:' + name];
    if (decl && structDecl) {
      return isCallSite ? decl : structDecl;
    }
    return decl || structDecl || null;
  }

  // Context key: true when cursor is on a word with a known declaration
  const hasDeclKey = ed.createContextKey<boolean>('facet.hasDeclaration', false);
  ed.onDidChangeCursorPosition(() => {
    hasDeclKey.set(findDecl(ed) !== null);
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
        // Cross-file: open the library file in a tab
        const source = fileSources[decl.file] || '';
        onGoToFile(decl.file, source, decl.line, decl.col);
      } else {
        // Same file: navigate within the editor
        ed.setPosition({ lineNumber: decl.line, column: decl.col });
        ed.revealLineInCenter(decl.line);
        ed.focus();
      }
    },
  });

  // "Open Library" context menu — detect lib "path" on current line
  function getLibPathAtCursor(editor: monaco.editor.ICodeEditor): string | null {
    const pos = editor.getPosition();
    if (!pos) return null;
    const line = editor.getModel()?.getLineContent(pos.lineNumber) ?? '';
    const match = line.match(/lib\s+"([^"]+)"/);
    if (!match) return null;
    return match[1];
  }

  const hasLibPathKey = ed.createContextKey<boolean>('facet.hasLibPath', false);
  ed.onDidChangeCursorPosition(() => {
    hasLibPathKey.set(getLibPathAtCursor(ed) !== null);
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
      const source = fileSources[libPath] || '';
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
    id: 'facet.toggleComment',
    label: 'Toggle Line Comment',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Slash],
    run(editor) {
      editor.trigger('keyboard', 'editor.action.commentLine', null);
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

  ed.addAction({
    id: 'facet.indentLines',
    label: 'Indent Lines',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.BracketRight],
    run(editor) {
      editor.trigger('keyboard', 'editor.action.indentLines', null);
    },
  });
  ed.addAction({
    id: 'facet.outdentLines',
    label: 'Outdent Lines',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.BracketLeft],
    run(editor) {
      editor.trigger('keyboard', 'editor.action.outdentLines', null);
    },
  });
  ed.addAction({
    id: 'facet.undo',
    label: 'Undo',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyZ],
    run(editor) {
      editor.trigger('keyboard', 'undo', null);
    },
  });
  ed.addAction({
    id: 'facet.redo',
    label: 'Redo',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyZ],
    run(editor) {
      editor.trigger('keyboard', 'redo', null);
    },
  });
  ed.addAction({
    id: 'facet.selectAll',
    label: 'Select All',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyA],
    run(editor) {
      editor.trigger('keyboard', 'editor.action.selectAll', null);
    },
  });

  // Clear decorations and markers on content change
  ed.onDidChangeModelContent(() => {
    errorCollection.clear();
    debugCollection.clear();
    const m = ed.getModel();
    if (m) monaco.editor.setModelMarkers(m, 'facet-check', []);
    if (!suppressChange && onChange) {
      onChange();
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
        // a simple variable name lookup for the common case.
        const textBeforeDot = lineContent.slice(0, word.startColumn - 2);
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
        ListLibraries().catch(() => []),
        ListLocalLibraries().catch(() => []),
      ]);

      const paths: string[] = [];
      for (const lib of remote) {
        const fullPath = lib.id + (lib.ref ? '@' + lib.ref : '');
        paths.push(fullPath);
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

  return {
    getContent(): string {
      return ed.getModel()!.getValue();
    },

    getModelContent(fileKey: string): string {
      const m = models.get(fileKey);
      return m ? m.getValue() : '';
    },

    highlightError(line: number) {
      if (line < 1 || line > ed.getModel()!.getLineCount()) return;
      errorCollection.set([{
        range: new monaco.Range(line, 1, line, 1),
        options: {
          isWholeLine: true,
          className: 'monaco-error-line',
        },
      }]);
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
      if (line < 1 || line > ed.getModel()!.getLineCount()) return;
      debugCollection.set([{
        range: new monaco.Range(line, 1, line, 1),
        options: {
          isWholeLine: true,
          className: 'monaco-debug-line',
        },
      }]);
      ed.revealLineInCenter(line);
    },

    clearDebugLine() {
      debugCollection.clear();
    },

    setContent(text: string) {
      const fullRange = ed.getModel()!.getFullModelRange();
      ed.executeEdits('setContent', [{
        range: fullRange,
        text,
      }]);
    },

    setContentSilent(text: string) {
      suppressChange = true;
      try {
        const fullRange = ed.getModel()!.getFullModelRange();
        ed.executeEdits('setContentSilent', [{
          range: fullRange,
          text,
        }]);
      } finally {
        suppressChange = false;
      }
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

    resetMainModel(key: string, content: string) {
      const old = models.get(mainKey);
      mainKey = key;
      currentSourceKey = key;
      const m = monaco.editor.createModel(content, 'facet');
      models.set(key, m);
      ed.setModel(m);
      old?.dispose();
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

    updateDeclarations(decls: Record<string, { line: number; col: number; file?: string; kind?: string }>, sources?: Record<string, string>) {
      declarations = decls;
      fileSources = sources || {};
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

    undo() {
      ed.trigger('toolbar', 'undo', null);
    },

    redo() {
      ed.trigger('toolbar', 'redo', null);
    },
  };
}
