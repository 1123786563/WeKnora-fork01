// r5-verify leg 02 (#82 Task 17 / AC2): the synchronous return face can
// NEVER confirm payment. Re-visits checkout?order=<id> (the closest surface
// to Alipay's sync return: an unsigned customer-side reload), refreshes the
// order through the page's own refresh entry (deterministic waitForResponse,
// never a fixed sleep), and asserts the order STAYS awaiting-payment with no
// 已付款/已生效 text anywhere. Screenshot rv5-03.
import { login, note, runLeg, evidencePath, assertOrderIdentity } from '../_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = evidencePath('r5-verify');
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_02_sync_face.mjs <orderId>'); process.exit(2); }
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
  await page.waitForSelector('text=待付款', { timeout: 45000 });
  let body = await page.locator('main').innerText();
  assertOrderIdentity(results, body, ORDER, 'sync-face-same-order');
  note(results, 'sync-face-no-advance', body.includes('等待付款') && body.includes('待付款（权益未开通）'),
    'the sync-return surface still reads 等待付款/待付款（权益未开通）');
  note(results, 'sync-face-no-benefits', !body.includes('已生效') && !body.includes('已付款'),
    'no 已生效/已付款 text anywhere on the page');

  // Manual refresh through the page's own entry — deterministically wait
  // for the order read to settle, then re-assert the SAME face.
  await Promise.all([
    page.waitForResponse((r) => r.url().includes(`/commercial/orders/${ORDER}`), { timeout: 45000 }),
    page.getByRole('button', { name: /刷新订单状态/ }).click(),
  ]);
  body = await page.locator('main').innerText();
  note(results, 'sync-face-refresh-still-awaiting', body.includes('等待付款') && body.includes('待付款（权益未开通）'),
    'after an explicit refresh the order is STILL awaiting payment');
  note(results, 'sync-face-refresh-no-benefits', !body.includes('已生效') && !body.includes('已付款'),
    'refresh surface shows no 已生效/已付款 either');
  await page.screenshot({ path: `${EV}rv5-03-sync-return-no-confirmation.png`, fullPage: true });
});
