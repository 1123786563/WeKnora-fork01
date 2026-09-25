// Issue #82 flow verification: browser driver (real Playwright chromium).
// Drives the REAL vite dev server (http://localhost:5192 -> proxy :8092):
//   leg A (tenant 1, wechat channel via loopback stub): login -> checkout
//   (auto purchase) -> assert awaiting-payment + quote details -> billing
//   page awaiting-payment row. Screenshots land in this evidence dir.
//
// Run from anywhere:  node browser_flow_82.mjs
import { createRequire } from 'node:module';
const require = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json');
const { chromium } = require('@playwright/test');

const WEB = 'http://localhost:5192';
const EV = new URL('.', import.meta.url).pathname;
const EMAIL = process.env.FLOW82_EMAIL ?? 'issue82-flow-a@verify.local';
const PASSWORD = process.env.FLOW82_PASSWORD ?? 'issue82-Flow-Pw-a';

const results = [];
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};

const browser = await chromium.launch();
try {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(30000);

  // --- login (tenant A owner) ---
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });
  note('login', true, `logged in as ${EMAIL}, landed ${new URL(page.url()).pathname}`);

  // --- checkout: auto purchase (wechat channel via loopback stub) ---
  await page.goto(`${WEB}/platform/billing/checkout`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  // The page auto-creates the order; wait for the awaiting-payment status.
  await page.waitForSelector('text=待付款', { timeout: 60000 });
  const body = await page.locator('main').innerText();
  const hasQuote = body.includes('报价明细') && body.includes('¥99.00');
  // CheckoutPage 的 Status 文案是「待付款（权益未开通）」（权益未开**通**，
  // CheckoutPage.tsx:176）；BillingPage 的套餐行文案是「待付款（权益未开
  // 放）」（BillingPage.tsx:117）——两处词汇不同，断言各自精确匹配。
  note('checkout-awaiting-payment', body.includes('待付款（权益未开通）') && body.includes('等待付款'),
    `order status: 等待付款 + 待付款（权益未开通）`);
  note('checkout-quote-details', hasQuote, `quote block: 报价明细 + ¥99.00 line`);
  const orderLine = body.split('\n').find((l) => l.includes('ord_')) ?? '';
  note('checkout-order-id', /ord_[0-9a-f]+/.test(orderLine), orderLine.trim());
  await page.screenshot({ path: `${EV}01-checkout-awaiting-payment.png`, fullPage: true });

  // --- billing page: plan row awaiting payment ---
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  await page.waitForTimeout(1500); // purchase status fetch settles
  const billingBody = await page.locator('main').innerText();
  note('billing-awaiting-payment', billingBody.includes('待付款（权益未开放）'),
    `billing plan row: ${billingBody.split('\n').find((l) => l.includes('待付款')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}02-billing-awaiting-payment.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
