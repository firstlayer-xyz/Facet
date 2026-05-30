import { test, expect } from '@playwright/test';

test('page loads with no console errors', async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });
  page.on('pageerror', err => consoleErrors.push(err.message));

  await page.goto('/');
  // App boots and creates the #app container with at least one child.
  await expect(page.locator('#app')).toBeVisible();
  expect(consoleErrors, consoleErrors.join('\n')).toEqual([]);
});
