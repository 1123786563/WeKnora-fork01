// Issue #82 reverify-1: browser leg for the PAID state (tenant D, alipay order).
// After the trusted signed notify confirmed payment, the same checkout surface
// must show the paid-but-not-yet-effective presentation (order-state.ts maps
// payment=paid -> "已付款，权益处理中") and the billing plan row must NOT
// claim 已生效 while the authority subscription stays incomplete (D2 frozen).
// Screenshot: reverify1/rv1-03-checkout-paid-awaiting-activation.png
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');

const WEB = 'http://localhost:5192';
const EV = new URL('.', import.meta.url).pathname + 'reverify1/';
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_paid_face_82.mjs <orderId>'); process.exit(2); }

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
  await page.fill('#auth-email', 'issue82-flow-d@verify.local');
  await page.fill('#auth-password', 'issue82-Flow-Pw-d');
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  await page.waitForTimeout(4000); // one poll tick
  const body = await page.locator('main').innerText();
  note('checkout-paid-presentation', body.includes('已付款，权益处理中'),
    'checkout shows 已付款，权益处理中 (order.payment=paid presentation)');
  note('no-false-effective', !body.includes('权益已生效') && !body.includes('已生效'),
    'no 已生效 claim anywhere while authority subscription is incomplete (D2 frozen)');
  note('no-awaiting-payment-leftover', !body.includes('待付款（权益未开通）'),
    'awaiting-payment status no longer shown for the paid order');
  await page.screenshot({ path: `${EV}rv1-03-checkout-paid-awaiting-activation.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  await page.waitForTimeout(1500);
  const billing = await page.locator('main').innerText();
  note('billing-no-false-effective', !billing.includes('已生效'),
    'billing plan row does not claim 已生效');
  await page.screenshot({ path: `${EV}rv1-04-billing-after-paid.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
