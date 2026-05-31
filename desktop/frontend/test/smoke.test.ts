import { test, expect } from './harness';

test('page loads with mock and reaches editor', async ({ mockedPage: page }) => {
  await page.goto('/');
  await expect(page.locator('#app')).toBeVisible();
  // Editor renders inside #editor-panel; wait for Monaco to attach.
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Mock was reachable: GetDefaultSource was called during boot.
  const calls = await page.evaluate(() => (window as any).__mockCalls.map((c: any) => c.name));
  expect(calls).toContain('GetDefaultSource');
});
