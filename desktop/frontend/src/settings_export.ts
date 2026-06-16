import {
  settingsCheckboxRow,
  settingsHelp,
  type SettingsPageContext,
  type PageResult,
} from './settings_ui';

export function buildExportSettingsPage(ctx: SettingsPageContext): PageResult {
  const { draft, onSave } = ctx;
  const page = document.createElement('div');
  page.className = 'settings-page';

  page.appendChild(settingsCheckboxRow(
    'settings-embed-source-3mf',
    'Embed Facet source in exported 3MF',
    draft.export.embedSourceIn3mf,
    v => {
      draft.export.embedSourceIn3mf = v;
      onSave(structuredClone(draft));
    },
  ));

  page.appendChild(settingsHelp(
    'When on, exported .3mf files include the entry-point .fct source so Facet can reopen them as editable projects.',
    'hint',
  ));

  return { el: page };
}
