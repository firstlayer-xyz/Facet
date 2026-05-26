import { GetVersion } from '../wailsjs/go/main/App';
import { BrowserOpenURL } from '../wailsjs/runtime/runtime';
import { styleButton, type SettingsPageContext, type PageResult } from './settings_ui';

const FACET_GITHUB_URL = 'https://github.com/firstlayer-xyz/Facet';

export function buildAboutPage(_ctx: SettingsPageContext): PageResult {
  const page = document.createElement('div');
  page.className = 'settings-page';
  page.style.display = 'flex';
  page.style.flexDirection = 'column';
  page.style.alignItems = 'center';
  page.style.justifyContent = 'center';
  page.style.gap = '12px';
  page.style.padding = '32px 16px';

  const title = document.createElement('div');
  title.textContent = 'Facet';
  title.style.fontSize = '28px';
  title.style.fontWeight = '600';
  page.appendChild(title);

  const tagline = document.createElement('div');
  tagline.textContent = 'Code-driven CAD';
  tagline.style.fontSize = '13px';
  tagline.style.color = '#888';
  page.appendChild(tagline);

  const versionLine = document.createElement('div');
  versionLine.style.fontSize = '13px';
  versionLine.style.color = '#888';
  versionLine.style.marginTop = '8px';
  versionLine.textContent = 'Version: ...';
  GetVersion().then((v) => {
    versionLine.textContent = `Version: ${v}`;
  });
  page.appendChild(versionLine);

  const githubBtn = document.createElement('button');
  githubBtn.textContent = 'View on GitHub';
  styleButton(githubBtn, 'primary');
  githubBtn.style.marginTop = '16px';
  githubBtn.addEventListener('click', () => {
    BrowserOpenURL(FACET_GITHUB_URL);
  });
  page.appendChild(githubBtn);

  return { el: page };
}
