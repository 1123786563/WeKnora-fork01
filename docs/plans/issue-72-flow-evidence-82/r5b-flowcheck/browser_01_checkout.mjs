// r5-verify leg 01 (#82 Task 17): browser checkout -> awaiting payment.
// Logs in as the seeded browser protagonist, opens the checkout, asserts
// the Alipay radio is the DEFAULT selection, submits (wire assertion: the
// purchases request body carries provider=='alipay'), and lands on the
// awaiting-payment face: 等待付款 + 待付款（权益未开通） + the frozen quote
// (FLOW82_EXPECT_CNY) + the order id (A-12 anchor). Then the billing page's
// 待付款（权益未开放） row. Screenshots rv5-01/02.
import { login, note, runLeg, evidencePath, assertOrderIdentity } from '../_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = evidencePath('r5b-flowcheck');
if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW || !process.env.FLOW82_EXPECT_CNY) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW / FLOW82_EXPECT_CNY');
  process.exit(2);
}
const EMAIL = process.env.FLOW82_EMAIL_A;
const PASSWORD = process.env.FLOW82_R5_PW;
const EXPECT_CNY = process.env.FLOW82_EXPECT_CNY;

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);

  // Wire assertion face: capture the purchases request body.
  let purchaseProvider = '';
  page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().includes('/commercial/purchases')) {
      try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { /* captured below */ }
    }
  });

  await login(page, WEB, EMAIL, PASSWORD);
  note(results, 'login', true, `logged in as ${EMAIL}`);

  await page.goto(`${WEB}/platform/billing/checkout`);
  await page.waitForSelector('text=订单结算', { timeout: 45000 });
  // The page auto-submits the purchase; await the awaiting-payment face
  // FIRST (the radio rides the ready state).
  await page.waitForSelector('text=待付款', { timeout: 60000 });
  // The Alipay radio is the default selection (#82 main rail).
  const alipayChecked = await page.isChecked('input[value="alipay"]');
  note(results, 'checkout-alipay-default', alipayChecked, 'the alipay radio is the default channel selection');
  note(results, 'checkout-submit-provider', purchaseProvider === 'alipay',
    `purchases request body provider=${JSON.stringify(purchaseProvider)}`);
  const body = await page.locator('main').innerText();
  note(results, 'checkout-awaiting-payment',
    body.includes('等待付款') && body.includes('待付款（权益未开通）'),
    'order status: 等待付款 + 待付款（权益未开通）');
  note(results, 'checkout-quote-frozen', body.includes('报价明细') && body.includes(EXPECT_CNY),
    `quote block: 报价明细 + ${EXPECT_CNY}`);
  const orderID = assertOrderIdentity(results, body, undefined, 'checkout-order-id-anchor');
  await page.screenshot({ path: `${EV}rv5-01-checkout-awaiting-payment.png`, fullPage: true });

  // Billing page: the plan row carries the awaiting-payment suffix.
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('text=待付款', { timeout: 45000 });
  const billing = await page.locator('main').innerText();
  note(results, 'billing-awaiting-payment', billing.includes('待付款（权益未开放）'),
    'billing plan row: 待付款（权益未开放）');
  await page.screenshot({ path: `${EV}rv5-02-billing-awaiting-payment.png`, fullPage: true });

  // The order id is the anchor every later leg must re-observe (A-12).
  console.log(`ANCHOR_ORDER_ID=${orderID}`);
});
