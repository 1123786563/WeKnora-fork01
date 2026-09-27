// Issue #82 flow verification leg B: browser sync-face no-confirmation probe.
// Logs in as tenant B, opens checkout?order=<alipay order> (the closest
// surface to Alipay's synchronous return: an unsigned customer-side
// reload), and asserts the order STAYS awaiting-payment — the sync face
// never advances the flow (AC2). Screenshot: 03-sync-return-no-confirmation.png.
import { login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
const EV = evidencePath(process.env.FLOW82_EVIDENCE_DIR ?? '');
const TAG = process.env.FLOW82_EVIDENCE_TAG ?? '';
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_sync_face_82.mjs <orderId>'); process.exit(2); }
// (OCR r4/r2) credentials env-REQUIRED with the _B fallback; the password
// rides the PASSWORD variable — never a source-code literal.
if (!process.env.FLOW82_EMAIL_B && !process.env.FLOW82_EMAIL) {
  console.error('missing required env: FLOW82_EMAIL(_B)');
  process.exit(2);
}
if (!process.env.FLOW82_PASSWORD_B && !process.env.FLOW82_PASSWORD) {
  console.error('missing required env: FLOW82_PASSWORD(_B)');
  process.exit(2);
}
const EMAIL = process.env.FLOW82_EMAIL_B ?? process.env.FLOW82_EMAIL;
const PASSWORD = process.env.FLOW82_PASSWORD_B ?? process.env.FLOW82_PASSWORD;

await runLeg(async (results, browser) => {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(30000);
  await login(page, WEB, EMAIL, PASSWORD);

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  await page.waitForSelector('text=待付款', { timeout: 30000 });
  const body = await page.locator('main').innerText();
  note(results, 'sync-face-no-advance', body.includes('等待付款') && body.includes('待付款（权益未开通）'),
    'checkout replay of the SAME order still reads 等待付款/待付款（权益未开通）— no confirmation from the sync face');
  note(results, 'sync-face-no-benefits', !body.includes('已生效') && !body.includes('已付款'),
    'no 已生效/已付款 text anywhere on the page');
  await page.screenshot({ path: `${EV}${TAG}03-sync-return-no-confirmation.png`, fullPage: true });
});
