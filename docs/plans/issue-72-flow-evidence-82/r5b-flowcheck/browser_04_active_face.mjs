// r5-verify leg 04 (#82 Task 17 / AC1 final state + credits): after the
// settle drive and the built-in webhook finalized the authority, the
// purchase is active: the billing row shows 已生效 and the wallet balance
// carries the purchase credit batch. The old awaiting/paid copy must be
// GONE. The checkout face is opened DEEP-LINKED (?order=<id>): an active
// purchase must never auto-submit a new one. Screenshot rv5-05.
import { login, note, runLeg, evidencePath, assertOrderIdentity } from '../_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = evidencePath('r5b-flowcheck');
// (OCR84-R1-20) orderId 是必需参数：`?? ''` 的静默降级会让 CheckoutPage 走自动
// 建单分支——恰好绕过本腿要验证的「active purchase 深链绝不自动开新单」守卫。
// 与 legs 02/03 对齐：缺参即 usage exit 2。
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }
if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW');
  process.exit(2);
}

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 45000 });
  // (OCR84-R1-33) BillingPage 只在挂载/reloadToken 变化时拉取一次（无轮询）：
  // 等待期间周期性驱动页面自身 Reload 按钮推进投影，detached 判定保持确定，
  // 超时上限保持 180s——纯 detached 等待在投影未翻转时必烧满超时后 leg-error。
  const reload = page.getByRole('button', { name: 'Reload' });
  const deadline = Date.now() + 180_000;
  let detached = false;
  while (Date.now() < deadline) {
    try {
      await page.waitForSelector('text=已付款待激活', { state: 'detached', timeout: 3_000 });
      detached = true;
      break;
    } catch {
      await reload.click().catch(() => { /* button not (yet) rendered — retry next tick */ });
    }
  }
  if (!detached) throw new Error('billing projection never flipped within 180s (paid-awaiting copy stayed)');
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
  // (OCR84-R1-20) A-12 订单身份断言恒执行。
  assertOrderIdentity(results, checkout, ORDER, 'active-face-same-order');
  await page.screenshot({ path: `${EV}rv5-05-active-credits.png`, fullPage: true });
});
