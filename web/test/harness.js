// Shared playwright harness for the wasm browser tests.
//
// Each test file calls `runTest(name, body)`, where body is given a
// {browser, context, page} bundle and is expected to throw on failure.
// The harness handles browser launch, navigation to the wasm page,
// console + page-error logging, and exit codes.

const URL = process.env.FACET_WEB_URL || 'http://localhost:8000/';

async function runTest(name, body, chromium) {
  const browser = await chromium.launch({
    headless: true,
  });
  const context = await browser.newContext();
  const page = await context.newPage();

  // Forward browser console + uncaught errors so failures are debuggable.
  page.on('console', msg => {
    const t = msg.type();
    if (t === 'error' || t === 'warning') {
      console.log(`  [${t}] ${msg.text()}`);
    }
  });
  page.on('pageerror', err => console.log(`  [pageerror] ${err}`));

  console.log(`▶ ${name}`);
  let failed = false;
  try {
    await page.goto(URL, { waitUntil: 'load' });
    await body({ browser, context, page });
    console.log(`✓ ${name}`);
  } catch (e) {
    console.error(`✗ ${name}: ${e.message}`);
    failed = true;
  } finally {
    await browser.close();
  }
  if (failed) process.exit(1);
}

module.exports = { runTest };
