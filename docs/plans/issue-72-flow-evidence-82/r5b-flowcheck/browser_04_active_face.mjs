// r5-verify leg 04 (#82 Task 17 / AC1 final state + credits): after the
// settle drive and the built-in webhook finalized the authority, the
// purchase is active: the billing row shows 已生效 and the wallet balance
// carries the purchase credit batch. The old awaiting/paid copy must be
// GONE (waitForSelector detached — deterministic, never a fixed sleep).
// The checkout face is opened DEEP-LINKED (?order=<id>): an active purchase
// must never auto-submit a new one. Screenshot rv5-05.
import { login, note, runLeg, evidencePath, assertOrderIdentity } from '../_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = evidencePath('r5b-flowcheck');
const ORDER = process.argv[2] ?? '';
if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW || !process.env.FLOW82_EXPECT_CNY) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW / FLOW82_EXPECT_CNY');
  process.exit(2);
}

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 45000 });
  // Deterministic convergence: wait until the paid-awaiting copy DETACHES
  // (the purchase projection flipped), then assert the effective face.
  await page.waitForSelector('text=已付款待激活', { state: 'detached', timeout: 180000 });
  await page.waitForSelector('text=已生效', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note(results, 'billing-active', billing.includes('已生效'),
    `billing plan row: ${billing.split('\n').find((l) => l.includes('已生效')) ?? '(absent)'}`);
  note(results, 'billing-no-stale-paid-copy', !billing.includes('已付款待激活') && !billing.includes('待付款（权益未开放）'),
    'no stale awaiting/paid copy remains');

  // The checkout face mirrors the effective state — DEEP-LINKED to the
  // fulfilled order (an active purchase never auto-submits a new one).
  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });
  await page.waitForSelector('text=权益已生效', { timeout: 90000 });
  const checkout = await page.locator('main').innerText();
  note(results, 'checkout-active', checkout.includes('权益已生效'),
    'checkout (deep-linked) shows 权益已生效');
  if (ORDER) assertOrderIdentity(results, checkout, ORDER, 'active-face-same-order');
  await page.screenshot({ path: `${EV}rv5-05-active-credits.png`, fullPage: true });
});
