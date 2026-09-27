// Issue #82 reverify-1: browser leg for the PAID state (tenant D, alipay order).
// After the trusted signed notify confirmed payment, the same checkout surface
// must show the paid-but-not-yet-effective presentation (order-state.ts maps
// payment=paid -> "已付款，权益处理中") and the billing plan row must NOT
// claim 已生效 while the authority subscription stays incomplete (D2 frozen).
// Output dir/tag: FLOW82_EVIDENCE_DIR (default reverify1) + FLOW82_EVIDENCE_TAG.
import { login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
// (OCR r2) Evidence output is parameterized — parallel rounds do not
// overwrite each other's artifacts (default keeps the reverify1 contract).
const EV = evidencePath(process.env.FLOW82_EVIDENCE_DIR ?? 'reverify1');
const TAG = process.env.FLOW82_EVIDENCE_TAG ?? 'rv1-';
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_paid_face_82.mjs <orderId>'); process.exit(2); }
if (!process.env.FLOW82_EMAIL_D && !process.env.FLOW82_EMAIL) {
  console.error('missing required env: FLOW82_EMAIL(_D)');
  process.exit(2);
}
if (!process.env.FLOW82_PASSWORD_D && !process.env.FLOW82_PASSWORD) {
  console.error('missing required env: FLOW82_PASSWORD(_D)');
  process.exit(2);
}
const EMAIL = process.env.FLOW82_EMAIL_D ?? process.env.FLOW82_EMAIL;
const PASSWORD = process.env.FLOW82_PASSWORD_D ?? process.env.FLOW82_PASSWORD;

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(30000);
  await login(page, WEB, EMAIL, PASSWORD);

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  await page.waitForSelector('text=已付款，权益处理中', { timeout: 30000 });
  const body = await page.locator('main').innerText();
  note(results, 'checkout-paid-presentation', body.includes('已付款，权益处理中'),
    'checkout shows 已付款，权益处理中 (order.payment=paid presentation)');
  note(results, 'no-false-effective', !body.includes('已生效'),
    'no 已生效 claim anywhere while authority subscription is incomplete (D2 frozen)');
  note(results, 'no-awaiting-payment-leftover', !body.includes('待付款（权益未开通）'),
    'awaiting-payment status no longer shown for the paid order');
  await page.screenshot({ path: `${EV}${TAG}03-checkout-paid-awaiting-activation.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  // (A-04) The plan <li> (summary request) renders BEFORE the purchase
  // status suffix (a second async purchaseStatus() request) — a structural
  // selector like 'main li' makes the negative 已生效 assertion VACUOUS in
  // that window. Wait for the POSITIVE paid face first (r4-flow/browser_03
  // discipline), then assert the negative on the settled DOM.
  await page.waitForSelector('text=已付款待激活', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note(results, 'billing-paid-awaiting-activation', billing.includes('已付款待激活'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已付款待激活')) ?? '(absent)'}`);
  note(results, 'billing-no-false-effective', !billing.includes('已生效'),
    'billing plan row does not claim 已生效');
  await page.screenshot({ path: `${EV}${TAG}04-billing-after-paid.png`, fullPage: true });
});
