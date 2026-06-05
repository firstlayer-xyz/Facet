// End-to-end test for browser-side animation playback.
//
// Verifies two things the web preview needs to play an Animation entry:
//   1. facetParse flags an Animation-returning entry point as `animated`, so
//      the UI knows to offer a Play control.
//   2. facetFrame(source, entry, overrides, timeMs) renders a single frame from
//      a retained session, and the geometry genuinely varies with the time
//      argument (proving the frame closure is re-run per call, not cached).
//
// The growing-cube source makes the assertion deterministic: volume = side³,
// so side(0)=10mm → 1000mm³ and side(10)=20mm → 8000mm³.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const SRC = `
fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }};
}
`;

function decodeHeader(u8) {
  const dv = new DataView(u8.buffer, u8.byteOffset, u8.byteLength);
  const headerLen = dv.getUint32(0, true);
  const headerJSON = new TextDecoder().decode(u8.subarray(4, 4 + headerLen));
  return JSON.parse(headerJSON);
}

runTest('animation', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetParse === 'function',
    null,
    { timeout: 60_000 },
  );

  const result = await page.evaluate(async ({ src, decodeSrc }) => {
    // eslint-disable-next-line no-new-func
    const decode = new Function('u8', `return (${decodeSrc})(u8)`);

    if (typeof window.facetFrame !== 'function') {
      return { stage: 'frame', error: 'window.facetFrame is not defined' };
    }

    const parseRes = await window.facetParse(src);
    if (!parseRes?.ok) {
      return { stage: 'parse', error: parseRes?.error || 'parse failed' };
    }
    const eps = JSON.parse(parseRes.entryPoints || '[]');
    const main = eps.find((e) => e.name === 'Main');

    const f0 = await window.facetFrame(src, 'Main', '{}', 0);
    const f10 = await window.facetFrame(src, 'Main', '{}', 10);
    if (!(f0 instanceof Uint8Array) || !(f10 instanceof Uint8Array)) {
      return { stage: 'frame', error: 'facetFrame did not return Uint8Array' };
    }
    const h0 = decode(f0);
    const h10 = decode(f10);
    return {
      stage: 'done',
      animated: main?.animated,
      err0: h0.errors,
      err10: h10.errors,
      vol0: h0.stats?.volume,
      vol10: h10.stats?.volume,
    };
  }, { src: SRC, decodeSrc: decodeHeader.toString() });

  if (result.stage !== 'done') {
    throw new Error(`failed at ${result.stage}: ${result.error}`);
  }
  if (!result.animated) {
    throw new Error('parse did not flag Main as animated');
  }
  if (result.err0?.length || result.err10?.length) {
    throw new Error(`frame returned errors: ${JSON.stringify(result.err0 || result.err10)}`);
  }
  if (Math.abs(result.vol0 - 1000) > 1) {
    throw new Error(`frame(0) volume = ${result.vol0}, want ~1000`);
  }
  if (Math.abs(result.vol10 - 8000) > 1) {
    throw new Error(`frame(10) volume = ${result.vol10}, want ~8000`);
  }
  console.log(
    `  animated=${result.animated}, volume frame(0)=${result.vol0.toFixed(0)}mm³ ` +
    `frame(10)=${result.vol10.toFixed(0)}mm³ — geometry varies with time`,
  );
}, chromium);

// UI wiring: loading an Animation example through the page's own example
// selector must reveal the Play button (animated flag → updatePlayUI), and
// switching to a static example must hide it again. Guards the entry-point →
// playback-control chain a wasm-only test can't see. A DOM visibility check, so
// it's deterministic — no canvas pixels or playback timing involved.
runTest('animation-ui', async ({ page }) => {
  await page.waitForFunction(
    () => typeof window.facetExamples === 'function'
       && document.getElementById('example-select')?.options.length > 1,
    null,
    { timeout: 60_000 },
  );

  const loadExample = (name) => page.evaluate((n) => {
    const sel = document.getElementById('example-select');
    sel.value = n;
    sel.dispatchEvent(new Event('change'));
  }, name);

  const playVisible = () => {
    const b = document.getElementById('play-btn');
    return !!b && b.style.display !== 'none';
  };

  // Animation entry → Play button visible.
  await loadExample('animation_cube.fct');
  await page.waitForFunction(playVisible, null, { timeout: 30_000 });
  const label = await page.evaluate(() => document.getElementById('play-btn').textContent);
  if (!/play/i.test(label)) {
    throw new Error(`play button label = "${label}", expected "Play"`);
  }

  // Static entry → Play button hidden again.
  await loadExample('Color Cube.fct');
  await page.waitForFunction(() => {
    const b = document.getElementById('play-btn');
    return !!b && b.style.display === 'none';
  }, null, { timeout: 30_000 });

  console.log('  play button shows for animation_cube.fct, hides for Color Cube.fct');
}, chromium);
