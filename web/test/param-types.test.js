// Verifies the parameter UI renders the right control per type and does not
// crash on non-numeric params. Spiral Text's Main has a String param (message)
// plus numeric params (height, twist); the old UI made a slider for every
// param and called v.toFixed → "v.toFixed is not a function" on the String.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

runTest('param-types', async ({ page }) => {
  const pageErrors = [];
  page.on('pageerror', e => pageErrors.push(e.message));

  await page.waitForFunction(
    () => document.getElementById('example-select')?.options.length > 1,
    null,
    { timeout: 60_000 },
  );

  // Drive the app's real flow: selecting an example → loadSource → doParse →
  // rebuildParams. Spiral Text exercises String + numeric params together.
  await page.selectOption('#example-select', 'Spiral Text.fct');
  await page.waitForTimeout(2500);

  const info = await page.evaluate(() => ({
    textInputs: document.querySelectorAll('#params-list input[type=text]').length,
    sliders:    document.querySelectorAll('#params-list input[type=range]').length,
    errorBox:   document.getElementById('error-box')?.textContent || '',
  }));
  console.log(`  Spiral Text params → ${info.textInputs} text input(s), ${info.sliders} slider(s)`);

  const toFixed = pageErrors.find(e => /toFixed/.test(e)) || (/toFixed/.test(info.errorBox) ? info.errorBox : null);
  if (toFixed) throw new Error(`param UI crashed on a non-numeric param: ${toFixed}`);
  if (info.textInputs < 1) throw new Error('expected a text input for the String param "message"');
  if (info.sliders < 1)    throw new Error('expected slider(s) for the numeric params (height, twist)');
  // Note: Spiral Text's eval still fails (Text() is a wasm stub); that error in
  // the box is expected and unrelated to the param-UI crash we're guarding.
}, chromium);
