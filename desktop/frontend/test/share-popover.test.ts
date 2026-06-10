import { test, expect } from './harness';

// A 1x1 transparent PNG: the popover needs a syntactically valid image data
// URL, not a scannable QR — QR content correctness is covered by the Go tests.
const TINY_PNG =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';

async function bootWithShareLink(page: any, qrpng: string, url: string) {
  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });
  await page.evaluate(
    ({ qrpng, url }: { qrpng: string; url: string }) => {
      (window as any).__mockOverrides.BuildShareLink = async () => ({ url, qrpng });
      (window as any).__openedURLs = [];
      (window as any).runtime.BrowserOpenURL = (u: string) => {
        (window as any).__openedURLs.push(u);
      };
    },
    { qrpng, url },
  );
}

test('share button shows a QR popover; clicking the QR opens the browser', async ({
  mockedPage: page,
}) => {
  const url = 'https://example.test/#code=abc';
  await bootWithShareLink(page, TINY_PNG, url);

  await page.click('#share-btn');
  await expect(page.locator('#share-popover .share-qr')).toBeVisible();

  // Re-clicking the button toggles the popover closed.
  await page.click('#share-btn');
  await expect(page.locator('#share-popover')).toHaveCount(0);

  // Escape dismisses.
  await page.click('#share-btn');
  await expect(page.locator('#share-popover')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.locator('#share-popover')).toHaveCount(0);

  // Clicking the QR opens the browser with the link and closes the popover.
  await page.click('#share-btn');
  await page.click('#share-popover .share-qr');
  await expect(page.locator('#share-popover')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).__openedURLs)).toEqual([url]);
});

test('share popover falls back to an explicit open button when the QR is absent', async ({
  mockedPage: page,
}) => {
  const url = 'https://example.test/#code=too-big-for-qr';
  await bootWithShareLink(page, '', url);

  await page.click('#share-btn');
  await expect(page.locator('#share-popover')).toBeVisible();
  await expect(page.locator('#share-popover .share-qr')).toHaveCount(0);

  await page.click('#share-popover .share-open-btn');
  await expect(page.locator('#share-popover')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).__openedURLs)).toEqual([url]);
});
