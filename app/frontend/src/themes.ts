import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';

// Theme JSON files are bundled locally (copied from monaco-themes).
// Filenames with spaces are valid on disk and handled correctly by Vite/rollup.
import draculaTheme from './themes/Dracula.json';
// @ts-ignore
import draculaLightTheme from './themes/Dracula Light.json';
import monokaiTheme from './themes/Monokai.json';
// @ts-ignore
import monokaiLightTheme from './themes/Monokai Light.json';
import nordTheme from './themes/Nord.json';
// @ts-ignore
import nordLightTheme from './themes/Nord Light.json';
import cobaltTheme from './themes/Cobalt.json';
// @ts-ignore
import cobaltLightTheme from './themes/Cobalt Light.json';
import solarizedDarkTheme from './themes/Solarized-dark.json';
import solarizedLightTheme from './themes/Solarized-light.json';
import tomorrowNightTheme from './themes/Tomorrow-Night.json';
import tomorrowTheme from './themes/Tomorrow.json';
// @ts-ignore — TypeScript may not resolve the space in the filename; Vite handles it fine
import githubDarkTheme from './themes/GitHub Dark.json';
// @ts-ignore
import githubLightTheme from './themes/GitHub Light.json';
// @ts-ignore
import nightOwlTheme from './themes/Night Owl.json';
// @ts-ignore
import nightOwlLightTheme from './themes/Night Owl Light.json';

interface ThemeEntry {
  id: string;
  label: string;
}

/** UI themes for the appearance selector. Each has light + dark palette variants.
 *  The dark mode switch picks which variant is used. */
export const UI_THEMES: ThemeEntry[] = [
  { id: 'cobalt', label: 'Cobalt' },
  { id: 'dracula', label: 'Dracula' },
  { id: 'github', label: 'GitHub' },
  { id: 'monokai', label: 'Monokai' },
  { id: 'night-owl', label: 'Night Owl' },
  { id: 'nord', label: 'Nord' },
  { id: 'solarized', label: 'Solarized' },
  { id: 'tomorrow', label: 'Tomorrow' },
];

type ThemeData = monaco.editor.IStandaloneThemeData;

export interface UIPalette {
  bg: string;
  bgDark: string;
  surface: string;
  border: string;
  borderHover: string;
  text: string;
  textMuted: string;
  textBright: string;
  textDim: string;
  textPlaceholder: string;
  textCode: string;
  panelBg: string;
  panelBorder: string;
  accent: string;
  errorBg: string;
  errorBorder: string;
  msgUserBg: string;
  // 3D viewport
  viewBg: string;
  viewMesh?: string;
  viewMeshMetalness: number;
  viewMeshRoughness: number;
  viewEdgeColor: string;
  viewEdgeOpacity: number;
  viewEdgeThreshold: number;
  viewAmbientIntensity: number;
  viewGridMajor: string;
  viewGridMinor: string;
}

const THEME_PALETTES: Record<string, UIPalette> = {
  // ── Facet - Orange ──
  'facet-orange-light': {
    bg: '#ffffff', bgDark: '#f0f0f0', surface: '#f7f7f7',
    border: '#d0d0d0', borderHover: '#b0b0b0',
    text: '#3a3a3a', textMuted: '#888888', textBright: '#111111',
    textDim: '#aaaaaa', textPlaceholder: '#bbbbbb', textCode: '#e56c30',
    panelBg: 'rgba(240, 240, 240, 0.95)', panelBorder: 'rgba(180, 180, 180, 0.5)',
    accent: '#e56c30', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#ffe8d0',
    viewBg: '#ffffff', viewMeshMetalness: 0.0, viewMeshRoughness: 0.45,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.25, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#cccccc', viewGridMinor: '#e5e5e5',
  },
  'facet-orange-dark': {
    bg: '#0E0E0E', bgDark: '#131313', surface: '#1A1A1A',
    border: '#262626', borderHover: '#484847',
    text: '#ADAAAA', textMuted: '#767575', textBright: '#E8E8E8',
    textDim: '#484847', textPlaceholder: '#262626', textCode: '#FF9154',
    panelBg: 'rgba(26, 26, 26, 0.98)', panelBorder: 'rgba(72, 72, 71, 0.2)',
    accent: '#FF7518', errorBg: '#1c0a0a', errorBorder: '#3d1515',
    msgUserBg: 'rgba(255, 117, 24, 0.08)',
    viewBg: '#000000', viewMeshMetalness: 0.15, viewMeshRoughness: 0.35,
    viewEdgeColor: '#ffffff', viewEdgeOpacity: 0.06, viewEdgeThreshold: 40,
    viewAmbientIntensity: 1.2,
    viewGridMajor: '#1a1a1a', viewGridMinor: '#131313',
  },
  // ── Facet - Green ──
  'facet-green-light': {
    bg: '#ffffff', bgDark: '#f0f0f0', surface: '#f7f7f7',
    border: '#d0d0d0', borderHover: '#b0b0b0',
    text: '#3a3a3a', textMuted: '#888888', textBright: '#111111',
    textDim: '#aaaaaa', textPlaceholder: '#bbbbbb', textCode: '#1e8a3e',
    panelBg: 'rgba(240, 240, 240, 0.95)', panelBorder: 'rgba(180, 180, 180, 0.5)',
    accent: '#1e8a3e', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#d4f0dc',
    viewBg: '#ffffff', viewMeshMetalness: 0.0, viewMeshRoughness: 0.45,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.25, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#cccccc', viewGridMinor: '#e5e5e5',
  },
  'facet-green-dark': {
    bg: '#0E0E0E', bgDark: '#131313', surface: '#1A1A1A',
    border: '#262626', borderHover: '#484847',
    text: '#ADAAAA', textMuted: '#767575', textBright: '#E8E8E8',
    textDim: '#484847', textPlaceholder: '#262626', textCode: '#52c878',
    panelBg: 'rgba(26, 26, 26, 0.98)', panelBorder: 'rgba(72, 72, 71, 0.2)',
    accent: '#2eb84e', errorBg: '#1c0a0a', errorBorder: '#3d1515',
    msgUserBg: 'rgba(46, 184, 78, 0.08)',
    viewBg: '#000000', viewMeshMetalness: 0.15, viewMeshRoughness: 0.35,
    viewEdgeColor: '#ffffff', viewEdgeOpacity: 0.06, viewEdgeThreshold: 40,
    viewAmbientIntensity: 1.2,
    viewGridMajor: '#1a1a1a', viewGridMinor: '#131313',
  },
  // ── Facet - Digital Blue ──
  'facet-digital-blue-light': {
    bg: '#ffffff', bgDark: '#f0f0f0', surface: '#f7f7f7',
    border: '#d0d0d0', borderHover: '#b0b0b0',
    text: '#3a3a3a', textMuted: '#888888', textBright: '#111111',
    textDim: '#aaaaaa', textPlaceholder: '#bbbbbb', textCode: '#0060c8',
    panelBg: 'rgba(240, 240, 240, 0.95)', panelBorder: 'rgba(180, 180, 180, 0.5)',
    accent: '#0060c8', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#d0e4ff',
    viewBg: '#ffffff', viewMeshMetalness: 0.0, viewMeshRoughness: 0.45,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.25, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#cccccc', viewGridMinor: '#e5e5e5',
  },
  'facet-digital-blue-dark': {
    bg: '#0E0E0E', bgDark: '#131313', surface: '#1A1A1A',
    border: '#262626', borderHover: '#484847',
    text: '#ADAAAA', textMuted: '#767575', textBright: '#E8E8E8',
    textDim: '#484847', textPlaceholder: '#262626', textCode: '#5aa8ff',
    panelBg: 'rgba(26, 26, 26, 0.98)', panelBorder: 'rgba(72, 72, 71, 0.2)',
    accent: '#3d96ff', errorBg: '#1c0a0a', errorBorder: '#3d1515',
    msgUserBg: 'rgba(61, 150, 255, 0.08)',
    viewBg: '#000000', viewMeshMetalness: 0.15, viewMeshRoughness: 0.35,
    viewEdgeColor: '#ffffff', viewEdgeOpacity: 0.06, viewEdgeThreshold: 40,
    viewAmbientIntensity: 1.2,
    viewGridMajor: '#1a1a1a', viewGridMinor: '#131313',
  },
  // ── Cobalt ──
  'cobalt-light': {
    bg: '#f0f5fa', bgDark: '#dce6f0', surface: '#e4edf5',
    border: '#b8cce0', borderHover: '#90aac8',
    text: '#002240', textMuted: '#4a6a8a', textBright: '#001530',
    textDim: '#a0b4c8', textPlaceholder: '#b0c0d0', textCode: '#0070a0',
    panelBg: 'rgba(224, 236, 248, 0.95)', panelBorder: 'rgba(140, 170, 200, 0.5)',
    accent: '#ff9d00', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#ffe0b0',
    viewBg: '#f0f5fa', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#b8cce0', viewGridMinor: '#d8e4f0',
  },
  'cobalt-dark': {
    bg: '#002240', bgDark: '#001a33', surface: '#003050',
    border: '#1a3a55', borderHover: '#2a5a80',
    text: '#e8f1ff', textMuted: '#7090b0', textBright: '#ffffff',
    textDim: '#4a6a80', textPlaceholder: '#4a5a70', textCode: '#80d0f0',
    panelBg: 'rgba(0, 34, 64, 0.92)', panelBorder: 'rgba(26, 58, 85, 0.6)',
    accent: '#ff9d00', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#003d70',
    viewBg: '#002240', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#1a3a55', viewGridMinor: '#0d2640',
  },
  // ── Dracula ──
  'dracula-light': {
    bg: '#f8f8f2', bgDark: '#eeeee8', surface: '#f0f0ea',
    border: '#d0d0c8', borderHover: '#b0b0a8',
    text: '#282a36', textMuted: '#6272a4', textBright: '#21222c',
    textDim: '#b0b0b8', textPlaceholder: '#c0c0c8', textCode: '#0d7da8',
    panelBg: 'rgba(240, 240, 234, 0.95)', panelBorder: 'rgba(180, 180, 170, 0.5)',
    accent: '#bd93f9', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#e0d4f8',
    viewBg: '#f8f8f2', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d0d0c8', viewGridMinor: '#e8e8e0',
  },
  'dracula-dark': {
    bg: '#282a36', bgDark: '#21222c', surface: '#323444',
    border: '#44475a', borderHover: '#6272a4',
    text: '#f8f8f2', textMuted: '#6272a4', textBright: '#ffffff',
    textDim: '#555766', textPlaceholder: '#505260', textCode: '#8be9fd',
    panelBg: 'rgba(40, 42, 54, 0.92)', panelBorder: 'rgba(68, 71, 90, 0.6)',
    accent: '#bd93f9', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#2d3a80',
    viewBg: '#282a36', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#44475a', viewGridMinor: '#363844',
  },
  // ── GitHub ──
  'github-light': {
    bg: '#ffffff', bgDark: '#f6f8fa', surface: '#f0f2f5',
    border: '#d0d7de', borderHover: '#afb8c1',
    text: '#24292f', textMuted: '#57606a', textBright: '#0d1117',
    textDim: '#b0b8c0', textPlaceholder: '#c0c8d0', textCode: '#0550ae',
    panelBg: 'rgba(246, 248, 250, 0.95)', panelBorder: 'rgba(208, 215, 222, 0.5)',
    accent: '#0969da', errorBg: '#ffebe9', errorBorder: '#e0a0a0',
    msgUserBg: '#ddf4ff',
    viewBg: '#ffffff', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d0d7de', viewGridMinor: '#e8ecf0',
  },
  'github-dark': {
    bg: '#24292e', bgDark: '#1c2128', surface: '#2d333b',
    border: '#444c56', borderHover: '#6e7681',
    text: '#cdd9e5', textMuted: '#768390', textBright: '#e3e9ef',
    textDim: '#57606a', textPlaceholder: '#50606a', textCode: '#79c0ff',
    panelBg: 'rgba(36, 41, 46, 0.92)', panelBorder: 'rgba(68, 76, 86, 0.6)',
    accent: '#539bf5', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#1a3060',
    viewBg: '#24292e', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#444c56', viewGridMinor: '#363c44',
  },
  // ── Monokai ──
  'monokai-light': {
    bg: '#fafaf5', bgDark: '#f0f0e8', surface: '#f2f2ea',
    border: '#d8d8c8', borderHover: '#b8b8a8',
    text: '#272822', textMuted: '#75715e', textBright: '#1e1e1a',
    textDim: '#b0b0a0', textPlaceholder: '#c0c0b0', textCode: '#7a9e18',
    panelBg: 'rgba(242, 242, 234, 0.95)', panelBorder: 'rgba(180, 178, 166, 0.5)',
    accent: '#ae81ff', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#e8d8f8',
    viewBg: '#fafaf5', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d8d8c8', viewGridMinor: '#eceee0',
  },
  'monokai-dark': {
    bg: '#272822', bgDark: '#1e1e1a', surface: '#3e3d32',
    border: '#49483e', borderHover: '#75715e',
    text: '#f8f8f2', textMuted: '#75715e', textBright: '#ffffff',
    textDim: '#555550', textPlaceholder: '#505050', textCode: '#a6e22e',
    panelBg: 'rgba(39, 40, 34, 0.92)', panelBorder: 'rgba(73, 72, 62, 0.6)',
    accent: '#ae81ff', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#1e3a70',
    viewBg: '#272822', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#49483e', viewGridMinor: '#363630',
  },
  // ── Night Owl ──
  'night-owl-light': {
    bg: '#fbfbfb', bgDark: '#f0f0f0', surface: '#f0f0f0',
    border: '#d9d9d9', borderHover: '#b0b0b0',
    text: '#403f53', textMuted: '#637777', textBright: '#011627',
    textDim: '#b0b0b0', textPlaceholder: '#c0c0c0', textCode: '#4876d6',
    panelBg: 'rgba(240, 240, 240, 0.95)', panelBorder: 'rgba(180, 180, 180, 0.5)',
    accent: '#4876d6', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#d4e4ff',
    viewBg: '#fbfbfb', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d9d9d9', viewGridMinor: '#ececec',
  },
  'night-owl-dark': {
    bg: '#011627', bgDark: '#010e1a', surface: '#0d2137',
    border: '#1b3556', borderHover: '#2b5080',
    text: '#d6deeb', textMuted: '#637777', textBright: '#fffefe',
    textDim: '#3a5060', textPlaceholder: '#365060', textCode: '#82aaff',
    panelBg: 'rgba(1, 22, 39, 0.92)', panelBorder: 'rgba(27, 53, 86, 0.6)',
    accent: '#82aaff', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#1a3666',
    viewBg: '#011627', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#1b3556', viewGridMinor: '#0d2137',
  },
  // ── Nord ──
  'nord-light': {
    bg: '#eceff4', bgDark: '#e5e9f0', surface: '#e5e9f0',
    border: '#d8dee9', borderHover: '#c0c8d8',
    text: '#2e3440', textMuted: '#4c566a', textBright: '#1a2030',
    textDim: '#b0b8c8', textPlaceholder: '#c0c8d0', textCode: '#5e81ac',
    panelBg: 'rgba(229, 233, 240, 0.95)', panelBorder: 'rgba(180, 190, 210, 0.5)',
    accent: '#5e81ac', errorBg: '#fff0f0', errorBorder: '#e0a0a0',
    msgUserBg: '#d0ddf0',
    viewBg: '#eceff4', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d8dee9', viewGridMinor: '#e5e9f0',
  },
  'nord-dark': {
    bg: '#2e3440', bgDark: '#252a35', surface: '#3b4252',
    border: '#434c5e', borderHover: '#4c566a',
    text: '#d8dee9', textMuted: '#637d9d', textBright: '#eceff4',
    textDim: '#434c5e', textPlaceholder: '#3e4660', textCode: '#88c0d0',
    panelBg: 'rgba(46, 52, 64, 0.92)', panelBorder: 'rgba(67, 76, 94, 0.6)',
    accent: '#88c0d0', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#283966',
    viewBg: '#2e3440', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#434c5e', viewGridMinor: '#3b4252',
  },
  // ── Solarized ──
  'solarized-light': {
    bg: '#fdf6e3', bgDark: '#eee8d5', surface: '#f5efdc',
    border: '#d0c9b8', borderHover: '#b0a898',
    text: '#657b83', textMuted: '#93a1a1', textBright: '#073642',
    textDim: '#b8b0a0', textPlaceholder: '#b0a898', textCode: '#268bd2',
    panelBg: 'rgba(238, 232, 213, 0.95)', panelBorder: 'rgba(147, 161, 161, 0.4)',
    accent: '#268bd2', errorBg: '#ffdddd', errorBorder: '#e0a0a0',
    msgUserBg: '#c8d8f0',
    viewBg: '#fdf6e3', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#d0c9b8', viewGridMinor: '#e8e2d0',
  },
  'solarized-dark': {
    bg: '#002b36', bgDark: '#00212b', surface: '#073642',
    border: '#094855', borderHover: '#586e75',
    text: '#839496', textMuted: '#586e75', textBright: '#93a1a1',
    textDim: '#405555', textPlaceholder: '#3a5060', textCode: '#2aa198',
    panelBg: 'rgba(0, 43, 54, 0.92)', panelBorder: 'rgba(7, 54, 66, 0.6)',
    accent: '#268bd2', errorBg: '#3d1a0e', errorBorder: '#5a2510',
    msgUserBg: '#08305a',
    viewBg: '#002b36', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#094855', viewGridMinor: '#073642',
  },
  // ── Tomorrow ──
  'tomorrow-light': {
    bg: '#ffffff', bgDark: '#e8e8e8', surface: '#f2f2f2',
    border: '#dddddd', borderHover: '#bbbbbb',
    text: '#4d4d4c', textMuted: '#8e908c', textBright: '#1a1a1a',
    textDim: '#aaaaaa', textPlaceholder: '#bbbbbb', textCode: '#4271ae',
    panelBg: 'rgba(240, 240, 240, 0.95)', panelBorder: 'rgba(180, 180, 180, 0.5)',
    accent: '#4271ae', errorBg: '#ffeeee', errorBorder: '#e0b0b0',
    msgUserBg: '#d0e4f0',
    viewBg: '#f0f0f0', viewMeshMetalness: 0.0, viewMeshRoughness: 0.4,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.2, viewEdgeThreshold: 40,
    viewAmbientIntensity: 2.0,
    viewGridMajor: '#cccccc', viewGridMinor: '#e0e0e0',
  },
  'tomorrow-dark': {
    bg: '#1d1f21', bgDark: '#161819', surface: '#282a2e',
    border: '#3a3d42', borderHover: '#4c5158',
    text: '#c5c8c6', textMuted: '#636f7e', textBright: '#e0e0e0',
    textDim: '#505560', textPlaceholder: '#505060', textCode: '#81a2be',
    panelBg: 'rgba(29, 31, 33, 0.92)', panelBorder: 'rgba(58, 61, 66, 0.6)',
    accent: '#81a2be', errorBg: '#3d1f1f', errorBorder: '#5a2020',
    msgUserBg: '#1a3060',
    viewBg: '#1d1f21', viewMeshMetalness: 0.1, viewMeshRoughness: 0.6,
    viewEdgeColor: '#000000', viewEdgeOpacity: 0.15, viewEdgeThreshold: 40,
    viewAmbientIntensity: 0.8,
    viewGridMajor: '#3a3d42', viewGridMinor: '#2a2d30',
  },
};

export function registerThemes(): void {
  monaco.editor.defineTheme('cobalt', cobaltTheme as ThemeData);
  monaco.editor.defineTheme('cobalt-light', cobaltLightTheme as ThemeData);
  monaco.editor.defineTheme('dracula', draculaTheme as ThemeData);
  monaco.editor.defineTheme('dracula-light', draculaLightTheme as ThemeData);
  monaco.editor.defineTheme('github-dark', githubDarkTheme as ThemeData);
  monaco.editor.defineTheme('github-light', githubLightTheme as ThemeData);
  monaco.editor.defineTheme('monokai', monokaiTheme as ThemeData);
  monaco.editor.defineTheme('monokai-light', monokaiLightTheme as ThemeData);
  monaco.editor.defineTheme('night-owl', nightOwlTheme as ThemeData);
  monaco.editor.defineTheme('night-owl-light', nightOwlLightTheme as ThemeData);
  monaco.editor.defineTheme('nord', nordTheme as ThemeData);
  monaco.editor.defineTheme('nord-light', nordLightTheme as ThemeData);
  monaco.editor.defineTheme('solarized-dark', solarizedDarkTheme as ThemeData);
  monaco.editor.defineTheme('solarized-light', solarizedLightTheme as ThemeData);
  monaco.editor.defineTheme('tomorrow-night', tomorrowNightTheme as ThemeData);
  monaco.editor.defineTheme('tomorrow', tomorrowTheme as ThemeData);
}

/** Resolve UI theme ID from base uiTheme + darkMode to a palette key.
 *  e.g. ('facet', 'dark') → 'facet-dark', ('dracula', 'light') → 'dracula-light'. */
export function resolveUiTheme(uiTheme: string, darkMode: 'light' | 'dark' | 'auto'): string {
  const suffix = darkMode === 'auto'
    ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    : darkMode;
  // Custom themes (id starts with 'custom-') don't have light/dark variants
  if (uiTheme.startsWith('custom-')) return uiTheme;
  // Built-in themes: append -light or -dark
  return `${uiTheme}-${suffix}`;
}

/** Apply UI palette to CSS custom properties (no Monaco theme change). */
export function applyUIPalette(palette: UIPalette): void {
  const root = document.documentElement;
  root.style.setProperty('--ui-bg', palette.bg);
  root.style.setProperty('--ui-bg-dark', palette.bgDark);
  root.style.setProperty('--ui-surface', palette.surface);
  root.style.setProperty('--ui-border', palette.border);
  root.style.setProperty('--ui-border-hover', palette.borderHover);
  root.style.setProperty('--ui-text', palette.text);
  root.style.setProperty('--ui-text-muted', palette.textMuted);
  root.style.setProperty('--ui-text-bright', palette.textBright);
  root.style.setProperty('--ui-text-dim', palette.textDim);
  root.style.setProperty('--ui-text-placeholder', palette.textPlaceholder);
  root.style.setProperty('--ui-text-code', palette.textCode);
  root.style.setProperty('--ui-panel-bg', palette.panelBg);
  root.style.setProperty('--ui-panel-border', palette.panelBorder);
  root.style.setProperty('--ui-accent', palette.accent);
  root.style.setProperty('--ui-error-bg', palette.errorBg);
  root.style.setProperty('--ui-error-border', palette.errorBorder);
  root.style.setProperty('--ui-msg-user-bg', palette.msgUserBg);
}

interface PaletteField {
  key: keyof UIPalette;
  label: string;
  type: 'color' | 'color-alpha' | 'number';
  section: 'UI' | 'Viewport';
  step?: number;
  min?: number;
  max?: number;
}

export const PALETTE_FIELDS: PaletteField[] = [
  // UI section
  { key: 'bg', label: 'Background', type: 'color', section: 'UI' },
  { key: 'bgDark', label: 'Background Dark', type: 'color', section: 'UI' },
  { key: 'surface', label: 'Surface', type: 'color', section: 'UI' },
  { key: 'border', label: 'Border', type: 'color', section: 'UI' },
  { key: 'borderHover', label: 'Border Hover', type: 'color', section: 'UI' },
  { key: 'text', label: 'Text', type: 'color', section: 'UI' },
  { key: 'textMuted', label: 'Text Muted', type: 'color', section: 'UI' },
  { key: 'textBright', label: 'Text Bright', type: 'color', section: 'UI' },
  { key: 'textDim', label: 'Text Dim', type: 'color', section: 'UI' },
  { key: 'textPlaceholder', label: 'Text Placeholder', type: 'color', section: 'UI' },
  { key: 'textCode', label: 'Text Code', type: 'color', section: 'UI' },
  { key: 'panelBg', label: 'Panel Background', type: 'color-alpha', section: 'UI' },
  { key: 'panelBorder', label: 'Panel Border', type: 'color-alpha', section: 'UI' },
  { key: 'accent', label: 'Accent', type: 'color', section: 'UI' },
  { key: 'errorBg', label: 'Error Background', type: 'color', section: 'UI' },
  { key: 'errorBorder', label: 'Error Border', type: 'color', section: 'UI' },
  { key: 'msgUserBg', label: 'User Message', type: 'color', section: 'UI' },
  // Viewport section
  { key: 'viewBg', label: 'Background', type: 'color', section: 'Viewport' },
  { key: 'viewMesh', label: 'Mesh Color', type: 'color', section: 'Viewport' },
  { key: 'viewMeshMetalness', label: 'Metalness', type: 'number', section: 'Viewport', step: 0.05, min: 0, max: 1 },
  { key: 'viewMeshRoughness', label: 'Roughness', type: 'number', section: 'Viewport', step: 0.05, min: 0, max: 1 },
  { key: 'viewEdgeColor', label: 'Edge Color', type: 'color', section: 'Viewport' },
  { key: 'viewEdgeOpacity', label: 'Edge Opacity', type: 'number', section: 'Viewport', step: 0.05, min: 0, max: 1 },
  { key: 'viewEdgeThreshold', label: 'Edge Threshold', type: 'number', section: 'Viewport', step: 1, min: 1, max: 90 },
  { key: 'viewAmbientIntensity', label: 'Ambient Intensity', type: 'number', section: 'Viewport', step: 0.1, min: 0, max: 5 },
  { key: 'viewGridMajor', label: 'Grid Major', type: 'color', section: 'Viewport' },
  { key: 'viewGridMinor', label: 'Grid Minor', type: 'color', section: 'Viewport' },
];

export interface CustomTheme {
  id: string;
  label: string;
  base: string;
  palette: Record<string, string | number>;
}

export function resolveThemePalette(
  themeId: string,
  overrides: Record<string, string | number>,
  customThemes: CustomTheme[],
): UIPalette {
  const custom = customThemes.find(t => t.id === themeId);
  if (custom) {
    const base = THEME_PALETTES[custom.base] ?? THEME_PALETTES['facet-orange-light'];
    return { ...base, ...custom.palette } as UIPalette;
  }
  const base = THEME_PALETTES[themeId] ?? THEME_PALETTES['facet-orange-light'];
  if (!overrides || Object.keys(overrides).length === 0) return base;
  return { ...base, ...overrides } as UIPalette;
}

export function getBaseThemeId(themeId: string, customThemes: CustomTheme[]): string {
  const custom = customThemes.find(t => t.id === themeId);
  return custom ? custom.base : themeId;
}

/** Map from UI theme base name to [light editor theme, dark editor theme]. */
const EDITOR_THEME_MAP: Record<string, [string, string]> = {
  'facet-orange':       ['facet-orange-light',       'facet-orange-dark'],
  'facet-green':        ['facet-green-light',        'facet-green-dark'],
  'facet-digital-blue': ['facet-digital-blue-light', 'facet-digital-blue-dark'],
  'cobalt':    ['cobalt-light',     'cobalt'],
  'dracula':   ['dracula-light',    'dracula'],
  'github':    ['github-light',     'github-dark'],
  'monokai':   ['monokai-light',    'monokai'],
  'night-owl': ['night-owl-light',  'night-owl'],
  'nord':      ['nord-light',       'nord'],
  'solarized': ['solarized-light',  'solarized-dark'],
  'tomorrow':  ['tomorrow',         'tomorrow-night'],
};

/** Derive the Monaco editor theme from the current UI theme + dark mode setting. */
export function resolveEditorTheme(
  uiTheme: string,
  darkMode: 'light' | 'dark' | 'auto',
  customThemes: CustomTheme[],
): string {
  const isDark = darkMode === 'auto'
    ? window.matchMedia('(prefers-color-scheme: dark)').matches
    : darkMode === 'dark';

  // Custom theme — extract base UI theme name from the palette base field
  if (uiTheme.startsWith('custom-')) {
    const custom = customThemes.find(t => t.id === uiTheme);
    if (custom) {
      // custom.base is e.g. 'nord-dark' or 'solarized-light' — strip suffix
      const baseName = custom.base.replace(/-(light|dark)$/, '');
      const mapping = EDITOR_THEME_MAP[baseName];
      if (mapping) return isDark ? mapping[1] : mapping[0];
    }
    return isDark ? 'facet-orange-dark' : 'facet-orange-light';
  }

  const mapping = EDITOR_THEME_MAP[uiTheme];
  if (mapping) return isDark ? mapping[1] : mapping[0];
  return isDark ? 'facet-orange-dark' : 'facet-orange-light';
}

