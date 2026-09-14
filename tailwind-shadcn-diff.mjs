// Pixel-level screenshot comparison for migration acceptance.
// Usage: node tailwind-shadcn-diff.mjs <baselineDir> <afterDir> [threshold=0.1]
// Loads PNG pairs into a chromium canvas and reports the fraction of
// pixels differing beyond the channel threshold, plus bounding-box stats.
import { chromium } from 'playwright-core';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const baseDir = process.argv[2] ?? 'artifacts/tailwind-shadcn/baseline';
const afterDir = process.argv[3] ?? 'artifacts/tailwind-shadcn/after-foundation';
const threshold = Number(process.argv[4] ?? '0.1');
const names = readdirSync(baseDir).filter((f) => f.endsWith('.png') && statSync(join(baseDir, f)).isFile());

const browser = await chromium.launch();
const page = await browser.newPage();
await page.goto('about:blank');

async function diffPair(name) {
  const b64a = readFileSync(join(baseDir, name)).toString('base64');
  const b64b = readFileSync(join(afterDir, name)).toString('base64');
  return page.evaluate(async ([a64, b64, thr, label]) => {
    const load = (b64) => new Promise((res, rej) => { const img = new Image(); img.onload = () => res(img); img.onerror = rej; img.src = 'data:image/png;base64,' + b64; });
    const [ia, ib] = await Promise.all([load(a64), load(b64)]);
    const w = Math.min(ia.width, ib.width); const h = Math.min(ia.height, ib.height);
    const ca = document.createElement('canvas'); ca.width = w; ca.height = h;
    const cb = document.createElement('canvas'); cb.width = w; cb.height = h;
    ca.getContext('2d').drawImage(ia, 0, 0); cb.getContext('2d').drawImage(ib, 0, 0);
    const da = ca.getContext('2d').getImageData(0, 0, w, h).data;
    const db = cb.getContext('2d').getImageData(0, 0, w, h).data;
    let diff = 0; let minX = w, minY = h, maxX = -1, maxY = -1;
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
      const i = (y * w + x) * 4;
      if (Math.abs(da[i] - db[i]) > thr * 255 || Math.abs(da[i+1] - db[i+1]) > thr * 255 || Math.abs(da[i+2] - db[i+2]) > thr * 255) {
        diff++; if (x < minX) minX = x; if (x > maxX) maxX = x; if (y < minY) minY = y; if (y > maxY) maxY = y;
      }
    }
    const total = w * h;
    return { name: label, w, h, iaW: ia.width, iaH: ia.height, ibW: ib.width, ibH: ib.height, diffPct: (diff / total * 100).toFixed(3), bbox: maxX < 0 ? 'none' : (maxX - minX + 1) + 'x' + (maxY - minY + 1) + '@' + minX + ',' + minY };
  }, [b64a, b64b, threshold, name]);
}

const rows = [];
for (const name of names) {
  try { rows.push(await diffPair(name)); } catch (e) { rows.push({ name, error: String(e).slice(0, 120) }); }
}
await browser.close();
rows.sort((a, b) => Number(b.diffPct ?? 100) - Number(a.diffPct ?? 100));
for (const r of rows) {
  if (r.error) { console.log(r.name.padEnd(32), 'ERROR', r.error); continue; }
  const sizeNote = (r.iaW !== r.w || r.iaH !== r.h || r.ibW !== r.w || r.ibH !== r.h) ? ' SIZE-DIFF' : '';
  console.log(r.name.padEnd(32), (r.diffPct + '%').padStart(9), 'diff bbox:', r.bbox.padEnd(16), sizeNote);
}
