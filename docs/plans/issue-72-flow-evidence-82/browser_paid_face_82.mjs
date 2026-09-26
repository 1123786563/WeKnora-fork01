// Issue #82 reverify-1: browser leg for the PAID state (tenant D, alipay order).
// After the trusted signed notify confirmed payment, the same checkout surface
// must show the paid-but-not-yet-effective presentation (order-state.ts maps
// payment=paid -> "已付款，权益处理中") and the billing plan row must NOT
// claim 已生效 while the authority subscription stays incomplete (D2 frozen).
// Screenshot: reverify1/rv1-03-checkout-paid-awaiting-activation.png
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');

// (OCR r4) origin parameterized (FLOW82_WEB); credentials env-REQUIRED.
const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
const EV = fileURLToPath(new URL('.', import.meta.url)) + 'reverify1/';
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_paid_face_82.mjs <orderId>'); process.exit(2); }
import { fileURLToPath } from 'node:url';
const EMAIL = process.env.FLOW82_EMAIL_D ?? process.env.FLOW82_EMAIL ?? '';
const PASSWORD = process.env.FLOW82_PASSWORD_D ?? process.env.FLOW82_PASSWORD ?? '';
if (!EMAIL || !PASSWORD) {
  console.error('missing required env: FLOW82_EMAIL(_D) / FLOW82_PASSWORD(_D)');
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
  page.setDefaultTimeout(30000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  // (A-01) the env-REQUIRED PASSWORD variable — never a literal credential
  // (the branch redline: no usable credential literals in source/tests).
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  // (OCR r4) waitForTimeout replaced by waiting for the paid face itself.
  await page.waitForSelector('text=已付款，权益处理中', { timeout: 30000 });
  const body = await page.locator('main').innerText();
  note('checkout-paid-presentation', body.includes('已付款，权益处理中'),
    'checkout shows 已付款，权益处理中 (order.payment=paid presentation)');
  note('no-false-effective', !body.includes('已生效'),
    'no 已生效 claim anywhere while authority subscription is incomplete (D2 frozen)');
  note('no-awaiting-payment-leftover', !body.includes('待付款（权益未开通）'),
    'awaiting-payment status no longer shown for the paid order');
  await page.screenshot({ path: `${EV}rv1-03-checkout-paid-awaiting-activation.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  // (A-04) The plan <li> (summary request) renders BEFORE the purchase
  // status suffix (a second async purchaseStatus() request) — a structural
  // selector like 'main li' makes the negative 已生效 assertion VACUOUS in
  // that window. Wait for the POSITIVE paid face first (r4-flow/browser_03
  // discipline), then assert the negative on the settled DOM.
  await page.waitForSelector('text=已付款待激活', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note('billing-paid-awaiting-activation', billing.includes('已付款待激活'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已付款待激活')) ?? '(absent)'}`);
  note('billing-no-false-effective', !billing.includes('已生效'),
    'billing plan row does not claim 已生效');
  await page.screenshot({ path: `${EV}rv1-04-billing-after-paid.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
