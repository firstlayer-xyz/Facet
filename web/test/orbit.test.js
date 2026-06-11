// Regression test for the web-preview orbit direction.
//
// The web preview must match the desktop viewer's THREE.OrbitControls
// convention: dragging RIGHT (dx > 0) DECREASES yaw (spins the model the
// conventional way). A bug had `goal.yaw += dx`, inverting horizontal orbit so
// it felt backwards (most noticeably on touch). Vertical is unchanged: dragging
// DOWN (dy > 0) increases pitch.
//
// The viewer's inline script exposes __orbitBy + __camGoal synchronously, so
// this exercises the orbit math without booting WebGL or the wasm bundle.
const { chromium } = require('playwright');
const { runTest } = require('./harness');

runTest('orbit-direction', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.__orbitBy === 'function' && typeof window.__camGoal === 'function',
    null, { timeout: 60_000 });

  // Horizontal: drag right (dx = +50) must DECREASE yaw.
  const horiz = await page.evaluate(() => {
    const before = window.__camGoal().yaw;
    window.__orbitBy(50, 0);
    return { before, after: window.__camGoal().yaw };
  });
  if (!(horiz.after < horiz.before)) {
    throw new Error(`drag-right should DECREASE yaw (OrbitControls convention); before=${horiz.before} after=${horiz.after}`);
  }

  // Vertical: drag down (dy = +50) must INCREASE pitch (unchanged behaviour).
  const vert = await page.evaluate(() => {
    const before = window.__camGoal().pitch;
    window.__orbitBy(0, 50);
    return { before, after: window.__camGoal().pitch };
  });
  if (!(vert.after > vert.before)) {
    throw new Error(`drag-down should INCREASE pitch; before=${vert.before} after=${vert.after}`);
  }

  console.log('  orbit: drag-right decreases yaw, drag-down increases pitch ✓');
}, chromium);
