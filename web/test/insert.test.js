// Verifies the `|` (Insert) operator uses the real C++ facet_insert in wasm,
// not the old Difference().Union() fallback. With the fallback, `wall | pipe`
// was *identical* to `wall - pipe + pipe` (it leaves the bore plug). The real
// facet_insert removes the plug, so the volumes must differ.

const { chromium } = require('playwright');
const { runTest } = require('./harness');

const SRC = `
fn pipe() Solid {
  return Cylinder(r: 9 mm, h: 40 mm).AlignCenter(pos: Vec3{})
       - Cylinder(r: 6 mm, h: 40 mm).AlignCenter(pos: Vec3{})
}
fn wall() Solid {
  return Cube(s: Vec3{x: 60 mm, y: 60 mm, z: 20 mm}).AlignCenter(pos: Vec3{})
}
fn Insert()   Solid { return wall() | pipe() }       # real facet_insert
fn Fallback() Solid { return wall() - pipe() + pipe() }  # old Difference().Union()
`;

runTest('insert', async ({ page }) => {
  await page.waitForFunction(() => typeof window.facetEval === 'function', null, { timeout: 60_000 });

  const volume = (entry) => page.evaluate(async ({ src, entry }) => {
    const ev = await window.facetEval(src, entry, '{}');
    if (!(ev instanceof Uint8Array)) return { error: `eval returned ${typeof ev}` };
    const dv = new DataView(ev.buffer, ev.byteOffset, ev.byteLength);
    const headerLen = dv.getUint32(0, true);
    const h = JSON.parse(new TextDecoder().decode(ev.subarray(4, 4 + headerLen)));
    if (h.errors && h.errors.length) return { error: JSON.stringify(h.errors[0]) };
    return { vol: h.stats?.volume };
  }, { src: SRC, entry });

  const ins = await volume('Insert');
  const fb  = await volume('Fallback');
  if (ins.error) throw new Error(`Insert eval failed: ${ins.error}`);
  if (fb.error)  throw new Error(`Fallback eval failed: ${fb.error}`);
  console.log(`  Insert vol=${ins.vol.toFixed(1)} mm³  Fallback(diff+union) vol=${fb.vol.toFixed(1)} mm³`);

  if (Math.abs(ins.vol - fb.vol) < 1) {
    throw new Error(`Insert (${ins.vol.toFixed(1)}) equals the Difference+Union fallback — facet_insert not wired into the wasm bridge`);
  }
}, chromium);
