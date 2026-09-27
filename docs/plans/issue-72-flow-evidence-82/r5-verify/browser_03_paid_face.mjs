// r5-verify leg 03 (#82 Task 17 / AC1 middle state): after the TRUSTED
// signed notify confirmed payment (delivered by the operator between legs),
// the checkout face must show 已付款，权益处理中 — and NOT 已生效 — while the
// authority subscription is still settling; the billing row shows
// 已付款待激活. Same order identity (A-12). Screenshot rv5-04.
import { login, note, runLeg, evidencePath, assertOrderIdentity } from '../_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = evidencePath('r5-verify');
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_03_paid_face.mjs <orderId>'); process.exit(2); }
if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW');
  process.exit(2);
}

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });
  await page.waitForSelector('text=已付款，权益处理中', { timeout: 45000 });
  const body = await page.locator('main').innerText();
  assertOrderIdentity(results, body, ORDER, 'paid-face-same-order');
  note(results, 'paid-face-presentation', body.includes('已付款，权益处理中'),
    'checkout shows 已付款，权益处理中 (paid awaiting activation)');
  note(results, 'paid-face-no-false-effective', !body.includes('已生效'),
    'no 已生效 claim while the authority subscription is still settling');

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 45000 });
  // (A-04) the plan row renders before the purchase suffix — wait for the
  // POSITIVE paid face first, then assert on the settled DOM.
  await page.waitForSelector('text=已付款待激活', { timeout: 60000 });
  const billing = await page.locator('main').innerText();
  note(results, 'billing-paid-awaiting-activation', billing.includes('已付款待激活'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已付款待激活')) ?? '(absent)'}`);
  note(results, 'billing-no-false-effective', !billing.includes('已生效'),
    'billing plan row does not claim 已生效');
  await page.screenshot({ path: `${EV}rv5-04-paid-awaiting-activation.png`, fullPage: true });
});
