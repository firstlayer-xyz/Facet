import { test, expect } from './harness';
import * as fs from 'node:fs';
import * as path from 'node:path';

const docGuides = JSON.parse(
  fs.readFileSync(
    path.join(__dirname, 'mocks/fixtures/doc-guides.json'),
    'utf8'
  )
).value;

test('clicking a doc guide opens it in the docs panel, not the function-preview bar', async ({
  mockedPage: page,
}) => {
  await page.goto('/');

  // Stub GetDocGuides with our fixture. The docs panel fetches this lazily
  // on first open, so wiring the override before clicking the Docs button is
  // sufficient.
  await page.evaluate(guides => {
    (window as any).__mockOverrides.GetDocGuides = () => guides;
  }, docGuides);

  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Open the docs panel via the toolbar Docs button.
  await page.locator('#docs-btn').click();
  const docsPanel = page.locator('#docs-panel');
  await expect(docsPanel).toBeVisible();

  // Without a `users-guide` slug, DocsPanel falls back to renderGuideList,
  // which renders clickable `.guide-card` elements for each guide. Click the
  // first one ("Getting Started") to drill into it.
  await docsPanel.locator('.guide-card', { hasText: 'Getting Started' }).click();

  // After click, renderGuideSingle renders an <h1 class="guide-title"> with
  // the guide's H1 heading.
  await expect(docsPanel.locator('h1.guide-title', { hasText: 'Getting Started' }))
    .toBeVisible();

  // The floating function-preview bar (FunctionPreview in src/function-preview.ts,
  // DOM id `fn-preview-bar`) must NOT pop open as a side-effect of doc-link
  // routing. This catches "doc clicks open in the wrong window".
  await expect(page.locator('#fn-preview-bar')).toBeHidden();
});
