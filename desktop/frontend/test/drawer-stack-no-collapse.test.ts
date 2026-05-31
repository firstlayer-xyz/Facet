import { test, expect } from './harness';

// Regression test: dragging the editor/canvas divider all the way to the
// right used to squeeze #canvas-container to zero width when one or more
// drawers were open, because drawers and canvas competed for the same
// flex horizontal space. The drawer-stack overlay design removes that
// competition — drawers are absolute-positioned over #app and have no
// claim on viewport-panel's width.
test('canvas keeps non-zero width when divider dragged fully right with drawers open', async ({
  mockedPage: page,
}) => {
  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Open both drawers — the original bug only manifested when drawers
  // were widening the right-side flex content.
  await page.locator('#docs-btn').click();
  await expect(page.locator('#docs-panel')).toBeVisible();
  await page.locator('#assistant-btn').click();
  await expect(page.locator('#assistant-panel')).toBeVisible();

  // Drag the editor/canvas divider as far right as the UI allows.
  // The divider's drag is clamped to 90% in main.ts, so the canvas
  // would get the remaining ~10% of #app — but with the old layout
  // and two drawers totaling ~880px on a typical viewport, that
  // budget went negative and canvas collapsed to 0.
  const divider = page.locator('#divider');
  const dividerBox = await divider.boundingBox();
  if (!dividerBox) throw new Error('divider has no bounding box');
  const appBox = await page.locator('#app').boundingBox();
  if (!appBox) throw new Error('#app has no bounding box');

  const startX = dividerBox.x + dividerBox.width / 2;
  const y = dividerBox.y + dividerBox.height / 2;
  const endX = appBox.x + appBox.width - 10;  // as far right as it can go

  await page.mouse.move(startX, y);
  await page.mouse.down();
  await page.mouse.move(endX, y, { steps: 10 });
  await page.mouse.up();

  // Canvas must still be visible with positive width. The old layout
  // would return 0 here; the new overlay layout guarantees > 0.
  const canvasWidth = await page.locator('#canvas-container').evaluate(el => {
    return el.getBoundingClientRect().width;
  });
  expect(canvasWidth).toBeGreaterThan(0);
});
