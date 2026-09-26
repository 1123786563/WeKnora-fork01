// Issue #82 R-4 re-verification ROUND 3 (post-fix #2) — browser leg 1 (checkout, alipay).
// Same assertions as rounds 1-2; see ../r4-flow2/browser_01_checkout.mjs.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { fileURLToPath } from 'node:url';
import { writeFileSync } from 'node:fs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = fileURLToPath(new URL('.', import.meta.url));
const EMAIL = process.env.FLOW82_EMAIL_A ?? '';
const PASSWORD = process.env.FLOW82_PASSWORD_A ?? '';
if (!EMAIL || !PASSWORD) { console.error('missing env: FLOW82_EMAIL_A/FLOW82_PASSWORD_A'); process.exit(2); }
const EXPECT_CNY = process.env.FLOW82_EXPECT_CNY ?? '¥99.00';

const results = [];
const purchases = [];
const note = (step, ok, detail) => { results.push({ step, ok, detail }); console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`); };

const browser = await chromium.launch();
try {
  const page = await (await browser.newContext()).newPage();
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
  await page.waitForSelector('input[name="payment-channel"][value="alipay"]', { timeout: 45000 });
  note('channel-default-alipay', await page.isChecked('input[name="payment-channel"][value="alipay"]'), 'alipay radio checked by default');
  await page.check('input[name="payment-channel"][value="alipay"]');
  note('channel-explicit-alipay-select', true, 'explicitly selected the 支付宝 radio');

  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 60000 });
  await page.waitForSelector('a:has-text("前往支付")', { timeout: 45000 });
  const body = await page.locator('main').innerText();
  note('awaiting-payment-face', body.includes('待付款（权益未开通）') && body.includes('等待付款'), 'order status shows 等待付款 + 待付款（权益未开通）');
  note('pay-link-present', body.includes('前往支付'), 'channel checkout link rendered (前往支付)');
  note('quote-block', body.includes('报价明细') && body.includes(EXPECT_CNY), `quote block 报价明细 + ${EXPECT_CNY}`);
  const orderLine = body.split('\n').find((l) => l.includes('ord_')) ?? '';
  const orderId = (orderLine.match(/ord_[0-9a-f]+/) ?? [])[0] ?? '';
  note('order-id-visible', /^ord_[0-9a-f]+$/.test(orderId), orderId);
  const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
  note('purchase-wire-provider-alipay', purchases.length >= 1 && purchases.every((p) => p.provider === 'alipay'),
    `POST /purchases bodies: ${JSON.stringify(purchases.map((p) => ({ provider: p.provider })))}`);
  await page.screenshot({ path: `${EV}01-checkout-awaiting-payment.png`, fullPage: true });

  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('text=待付款（权益未开放）', { timeout: 45000 });
  const billingBody = await page.locator('main').innerText();
  note('billing-awaiting-payment', billingBody.includes('待付款（权益未开放）') && !billingBody.includes('已生效'),
    `billing plan row: ${billingBody.split('\n').find((l) => l.includes('待付款')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}02-billing-awaiting-payment.png`, fullPage: true });

  writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
  console.log('ORDER ' + orderId);
} finally { await browser.close(); }
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
