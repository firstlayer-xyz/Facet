import { test, expect } from './harness';

// The picker's dismiss handlers attach synchronously when it opens, so the
// click that opens it (which lands on the button's svg icon, not the button
// element itself) must not instantly close it — that requires the anchor
// check to use contains(), not target identity.

const SLICERS = [
  { id: 'orca', name: 'OrcaSlicer' },
  { id: 'prusa', name: 'PrusaSlicer' },
];

async function bootWithSlicers(page: any) {
  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });
  await page.evaluate((slicers: unknown) => {
    (window as any).__mockOverrides.DetectSlicers = async () => slicers;
  }, SLICERS);
}

test('slicer picker opens, stays open, and sends the picked slicer', async ({
  mockedPage: page,
}) => {
  await bootWithSlicers(page);

  await page.click('#slicer-btn');
  await expect(page.locator('#slicer-dropdown .slicer-item')).toHaveCount(2);

  await page.click('#slicer-dropdown .slicer-item:has-text("PrusaSlicer")');
  await expect(page.locator('#slicer-dropdown')).toHaveCount(0);

  const sent = await page.evaluate(() =>
    (window as any).__mockCalls
      .filter((c: any) => c.name === 'SendToSlicer')
      .map((c: any) => c.args[0]),
  );
  expect(sent).toEqual(['prusa']);
});

test('slicer picker toggles closed and dismisses on Escape', async ({ mockedPage: page }) => {
  await bootWithSlicers(page);

  // Re-clicking the button toggles the dropdown closed.
  await page.click('#slicer-btn');
  await expect(page.locator('#slicer-dropdown')).toBeVisible();
  await page.click('#slicer-btn');
  await expect(page.locator('#slicer-dropdown')).toHaveCount(0);

  // Escape dismisses.
  await page.click('#slicer-btn');
  await expect(page.locator('#slicer-dropdown')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.locator('#slicer-dropdown')).toHaveCount(0);

  // Nothing was sent — both opens were dismissed.
  const sent = await page.evaluate(() =>
    (window as any).__mockCalls.filter((c: any) => c.name === 'SendToSlicer'),
  );
  expect(sent).toEqual([]);
});
