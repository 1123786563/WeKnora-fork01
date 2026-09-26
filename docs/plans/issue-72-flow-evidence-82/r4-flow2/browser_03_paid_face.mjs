// Issue #82 R-4 re-verification ROUND 2 (post-fix) — browser leg 3 (paid-awaiting-activation face).
// After the trusted signed notify confirmed payment, the checkout surface shows
// the composed middle state 已付款，权益处理中 without 已生效; billing row 已付款待激活.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { fileURLToPath } from 'node:url';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = fileURLToPath(new URL('.', import.meta.url));
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_03_paid_face.mjs <orderId>'); process.exit(2); }
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
try {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });
  await page.waitForSelector('text=已付款，权益处理中', { timeout: 45000 });
  const body = await page.locator('main').innerText();
  note('checkout-paid-awaiting-activation', body.includes('已付款，权益处理中'),
    'checkout shows the composed middle state 已付款，权益处理中');
  note('no-false-active', !body.includes('已生效'),
    'no 已生效 claim while the authority subscription is incomplete');
  note('no-awaiting-leftover', !body.includes('待付款（权益未开通）'),
    'awaiting-payment face is gone for the paid order');
  await page.screenshot({ path: `${EV}04-paid-awaiting-activation.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('text=已付款待激活', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note('billing-paid-awaiting-activation', billing.includes('已付款待激活') && !billing.includes('已生效'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已付款待激活')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}05-billing-paid-awaiting-activation.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
