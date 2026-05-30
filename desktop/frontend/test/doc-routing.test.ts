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
  // Inject the fixture as the GetDocGuides response BEFORE page boot so the
  // docs panel hydrates with our guides on first open. addInitScript runs in
  // page context before app code, so __mockOverrides exists when we read it
  // back later in evaluate().
  await page.addInitScript(g => {
    (window as any).__pendingDocOverride = g;
  }, docGuides);
  await page.goto('/');

  // Wire the override now that __mockOverrides is in scope.
  await page.evaluate(() => {
    (window as any).__mockOverrides.GetDocGuides = () =>
      (window as any).__pendingDocOverride;
  });

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
