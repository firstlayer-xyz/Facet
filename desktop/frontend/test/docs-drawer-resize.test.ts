import { test, expect } from './harness';

// Regression test for: "the drawer is not resizable. It also can't co-exist
// with the AI assistant."
//
// Docs and assistant live in #drawer-stack as direct children. Each has
// a left-edge resizer (#docs-resizer / #assistant-resizer) that drags
// the drawer's width. CSS `order:` arranges them as docs (inside) →
// assistant (outside) when both are open, regardless of insertion order.
test('docs drawer is resizable and coexists with the assistant', async ({
  mockedPage: page,
}) => {
  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({
    timeout: 10_000,
  });

  // Open docs via toolbar.
  await page.locator('#docs-btn').click();
  const docsPanel = page.locator('#docs-panel');
  await expect(docsPanel).toBeVisible();

  // Resizer should now be visible alongside docs.
  const docsResizer = page.locator('#docs-resizer');
  await expect(docsResizer).toBeVisible();

  // Capture the starting width.
  const startWidth = await docsPanel.evaluate(el => el.getBoundingClientRect().width);

  // Drag the resizer left to widen the drawer by ~120px.
  const handle = await docsResizer.boundingBox();
  if (!handle) throw new Error('docs-resizer has no bounding box');
  const startX = handle.x + handle.width / 2;
  const y = handle.y + handle.height / 2;
  await page.mouse.move(startX, y);
  await page.mouse.down();
  await page.mouse.move(startX - 120, y);
  await page.mouse.up();

  // Width should have grown by roughly the drag distance (allow slop for
  // clamping and sub-pixel rendering).
  const endWidth = await docsPanel.evaluate(el => el.getBoundingClientRect().width);
  expect(endWidth).toBeGreaterThan(startWidth + 80);

  // Open the assistant. Both drawers must remain visible simultaneously.
  await page.locator('#assistant-btn').click();
  const assistantPanel = page.locator('#assistant-panel');
  await expect(assistantPanel).toBeVisible();
  await expect(docsPanel).toBeVisible();

  // Both should be direct children of #drawer-stack — confirms the
  // structural fix that lets the overlay container arrange them
  // side-by-side instead of competing with canvas for flex space.
  const docsParentId = await docsPanel.evaluate(el => el.parentElement?.id);
  const assistantParentId = await assistantPanel.evaluate(el => el.parentElement?.id);
  expect(docsParentId).toBe('drawer-stack');
  expect(assistantParentId).toBe('drawer-stack');
});
