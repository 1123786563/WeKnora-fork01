// Issue #82 R-4 re-verification — browser leg 1 (checkout order, alipay rail).
// Real headless chromium against the real vite dev server (:5194 -> :8093).
// Tenant A (settle-r4-a@verify.local) logs in, opens the checkout page,
// explicitly selects the Alipay radio (default) and lets the page create the
// payment-gated purchase. Asserts:
//   - the POST /purchases wire body carries provider == "alipay"
//   - the awaiting-payment face: 等待付款 / 待付款（权益未开通） + 前往支付 link
//   - the frozen quote block (报价明细 + ¥99.00) and the order id (ord_*)
//   - the billing plan row shows 待付款（权益未开放）
// Leaves order-id / checkout-url in r4-flow/order-info.json for the later legs.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { fileURLToPath } from 'node:url';
import { writeFileSync } from 'node:fs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = fileURLToPath(new URL('.', import.meta.url));
const EMAIL = process.env.FLOW82_EMAIL_A ?? '';
const PASSWORD = process.env.FLOW82_PASSWORD_A ?? '';
if (!EMAIL || !PASSWORD) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_PASSWORD_A');
  process.exit(2);
}
const EXPECT_CNY = process.env.FLOW82_EXPECT_CNY ?? '¥99.00';

const results = [];
const purchases = []; // captured POST /purchases wire bodies
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};

const browser = await chromium.launch();
// (OCR84-R1-19) 基础设施失败（登录被拒/选择器超时/导航错误）必须可区分地落
// note + 截图并仍打印 RESULT 汇总——裸栈退出会吞掉 RESULT 行，违背取证脚本
// 「断言失败与基础设施失败可区分」的自身纪律（browser_02/04 已实现该契约）。
let page;
try {
  page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().includes('/api/v1/commercial/purchases')) {
      try { purchases.push(JSON.parse(req.postData() ?? '{}')); } catch { purchases.push({ unparseable: true }); }
    }
  });

  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
  note('login', true, `logged in as ${EMAIL}`);

  await page.goto(`${WEB}/platform/billing/checkout`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });

  // (审查 H1) channel radio: assert Alipay is the default AND explicitly click it.
  await page.waitForSelector('input[name="payment-channel"][value="alipay"]', { timeout: 45000 });
  const alipayCheckedByDefault = await page.isChecked('input[name="payment-channel"][value="alipay"]');
  note('channel-default-alipay', alipayCheckedByDefault, 'alipay radio checked by default');
  await page.check('input[name="payment-channel"][value="alipay"]');
  note('channel-explicit-alipay-select', true, 'explicitly selected the 支付宝 radio');

  // Awaiting-payment face (three-state 1/3).
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 60000 });
  await page.waitForSelector('a:has-text("前往支付")', { timeout: 45000 });
  const body = await page.locator('main').innerText();
  note('awaiting-payment-face', body.includes('待付款（权益未开通）') && body.includes('等待付款'),
    'order status shows 等待付款 + 待付款（权益未开通）');
  note('pay-link-present', body.includes('前往支付'), 'channel checkout link rendered (前往支付)');
  note('quote-block', body.includes('报价明细') && body.includes(EXPECT_CNY),
    `quote block 报价明细 + ${EXPECT_CNY}`);
  const orderLine = body.split('\n').find((l) => l.includes('ord_')) ?? '';
  const orderId = (orderLine.match(/ord_[0-9a-f]+/) ?? [])[0] ?? '';
  note('order-id-visible', /^ord_[0-9a-f]+$/.test(orderId), orderId);
  const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');

  // Wire assertion: the purchase submit body carried provider == alipay.
  // (OCR84-R1-30) 删除恒假的 window.__r4PurchasesSeen 死等待（全仓库无任何赋值
  // 点，1 秒必超时又被 catch 静默吞掉，伪装出「已等待抓包完成」的同步）——
  // page.on('request') 在导航/断言之前已同步捕获 POST /purchases 体，此处直接
  // 消费 purchases 数组。
  const providerOk = purchases.length >= 1 && purchases.every((p) => p.provider === 'alipay');
  note('purchase-wire-provider-alipay', providerOk,
    `POST /purchases bodies: ${JSON.stringify(purchases.map((p) => ({ provider: p.provider, quote_id: p.quote_id?.slice(0, 12) + '…' })))}`);
  await page.screenshot({ path: `${EV}01-checkout-awaiting-payment.png`, fullPage: true });

  // Billing page plan row (three-state 1/3 on the billing surface).
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('text=待付款（权益未开放）', { timeout: 45000 });
  const billingBody = await page.locator('main').innerText();
  note('billing-awaiting-payment', billingBody.includes('待付款（权益未开放）') && !billingBody.includes('已生效'),
    `billing plan row: ${billingBody.split('\n').find((l) => l.includes('待付款')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}02-billing-awaiting-payment.png`, fullPage: true });

  writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
  console.log('ORDER ' + orderId);
} catch (error) {
  note('script-error', false, String(error?.stack ?? error));
  try { if (page) await page.screenshot({ path: `${EV}01-script-error.png`, fullPage: true }); } catch { /* page may be unusable */ }
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
