import { Page, expect } from '@playwright/test';

// Switches to an existing tab by its visible label. Throws if the tab does not exist.
//
// Tab DOM structure (from src/app.ts renderTabs):
//   #tab-bar > .tab-scroll > .tab[.active] > .tab-label + .tab-close
//
// The .tab element's full text includes the close button "×", so assertions
// target the inner .tab-label span rather than the .tab element directly.
export async function switchTab(page: Page, label: string) {
  await page.locator('#tab-bar .tab', { hasText: label }).click();
  await expect(
    page.locator('#tab-bar .tab.active .tab-label')
  ).toHaveText(label);
}

// Returns the visible label of the currently active tab.
export async function activeTabLabel(page: Page): Promise<string> {
  return (await page.locator('#tab-bar .tab.active .tab-label').innerText()).trim();
}
