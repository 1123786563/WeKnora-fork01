// Issue #82 R-4 re-verification — browser leg 4 (ACTIVE face, AC1 tail).
// After settle -> webhook -> finalize, the checkout surface shows 权益已生效
// and the billing plan row shows 已生效. Screenshot: 06-active-*.png.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { fileURLToPath } from 'node:url';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = fileURLToPath(new URL('.', import.meta.url));
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }
const EMAIL = process.env.FLOW82_EMAIL_A ?? '';
const PASSWORD = process.env.FLOW82_PASSWORD_A ?? '';
if (!EMAIL || !PASSWORD) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_PASSWORD_A');
  process.exit(2);
}

const results = [];
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};

const browser = await chromium.launch();
let page;
try {
  page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });
  await page.waitForSelector('text=权益已生效', { timeout: 45000 });
  const body = await page.locator('main').innerText();
  note('checkout-active-face', body.includes('权益已生效'), 'checkout shows 权益已生效');
  note('no-awaiting-leftover', !body.includes('待付款') && !body.includes('已付款，权益处理中'),
    'no awaiting/paid faces left on the active order');
  await page.screenshot({ path: `${EV}06-checkout-active.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('text=已生效', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note('billing-active-face', billing.includes('已生效'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已生效')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}07-billing-active.png`, fullPage: true });
} catch (err) {
  // (A-03) Infrastructure failure (login refused, a selector timeout — the exact
  // failure shapes a re-verification must capture) lands as an explicit FAIL
  // note: the RESULT summary still prints and the exit code is non-zero, so
  // downstream readers can tell an assertion failure from an infrastructure
  // one instead of seeing no evidence line at all.
  note('script-error', false, String(err));
  if (page) {
    try { await page.screenshot({ path: `${EV}07-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
  }
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
