// Issue #82 flow verification: browser driver (real Playwright chromium).
// Drives the REAL vite dev server (FLOW82_WEB -> backend proxy):
//   leg A (tenant 1, wechat channel via loopback stub): login -> checkout
//   (auto purchase) -> assert awaiting-payment + quote details -> billing
//   page awaiting-payment row. Screenshots land in FLOW82_EVIDENCE_DIR
//   (default: this evidence dir), prefixed by FLOW82_EVIDENCE_TAG.
//
// Run from anywhere:  node browser_flow_82.mjs
import { fileURLToPath } from 'node:url';
import { LOGIN, login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
// (OCR r2) Every credential is env-REQUIRED (no source-code fallback) and
// the expected CNY face is REQUIRED too — a hardcoded ¥99.00 default would
// silently keep asserting the OLD price after a seed/plan change.
envRequired('FLOW82_EMAIL', 'FLOW82_PASSWORD', 'FLOW82_EXPECT_CNY');
const { FLOW82_EMAIL: EMAIL, FLOW82_PASSWORD: PASSWORD, FLOW82_EXPECT_CNY: EXPECT_CNY } = process.env;
// (OCR r2) Evidence output is parameterized: FLOW82_EVIDENCE_DIR (a
// subdirectory of this evidence dir, e.g. reverify2) + FLOW82_EVIDENCE_TAG
// (the screenshot prefix, e.g. rv2) — parallel verification rounds no
// longer overwrite each other's artifacts.
const EV = evidencePath(process.env.FLOW82_EVIDENCE_DIR ?? '');
const TAG = process.env.FLOW82_EVIDENCE_TAG ?? '';

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(LOGIN.timeoutMs);
  await login(page, WEB, EMAIL, PASSWORD);
  note(results, 'login', true, `logged in as ${EMAIL}, landed ${new URL(page.url()).pathname}`);

  // --- checkout: auto purchase (channel via loopback stub) ---
  await page.goto(`${WEB}/platform/billing/checkout`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  // The page auto-creates the order; wait for the awaiting-payment status.
  await page.waitForSelector('text=待付款', { timeout: 60000 });
  const body = await page.locator('main').innerText();
  const hasQuote = body.includes('报价明细') && body.includes(EXPECT_CNY);
  note(results, 'checkout-awaiting-payment', body.includes('待付款（权益未开通）') && body.includes('等待付款'),
    'order status: 等待付款 + 待付款（权益未开通）');
  note(results, 'checkout-quote-details', hasQuote, `quote block: 报价明细 + ${EXPECT_CNY} line`);
  const orderLine = body.split('\n').find((l) => l.includes('ord_')) ?? '';
  note(results, 'checkout-order-id', /ord_[0-9a-f]+/.test(orderLine), orderLine.trim());
  await page.screenshot({ path: `${EV}${TAG}01-checkout-awaiting-payment.png`, fullPage: true });

  // --- billing page: plan row awaiting payment ---
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  await page.waitForSelector('text=待付款', { timeout: 30000 });
  const billingBody = await page.locator('main').innerText();
  note(results, 'billing-awaiting-payment', billingBody.includes('待付款（权益未开放）'),
    `billing plan row: ${billingBody.split('\n').find((l) => l.includes('待付款')) ?? '(absent)'}`);
  await page.screenshot({ path: `${EV}${TAG}02-billing-awaiting-payment.png`, fullPage: true });
});
