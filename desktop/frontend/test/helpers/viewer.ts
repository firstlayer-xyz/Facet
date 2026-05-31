import { Page } from '@playwright/test';

// Returns the currently selected entry-point name from the preview selector.
//
// The preview selector DOM (from src/toolbar.ts):
//   #preview-selector > #preview-file-btn > #preview-file-lbl + .preview-file-arr
//
// The plan's original draft called `.inputValue()` on `#preview-selector`, but
// that element is a <div>, not an <input>. The selected function name lives in
// the <span id="preview-file-lbl"> inside the dropdown button.
export async function getActiveEntryPoint(page: Page): Promise<string> {
  return (await page.locator('#preview-file-lbl').innerText()).trim();
}
