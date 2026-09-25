// Issue #82 flow verification leg B: browser sync-face no-confirmation probe.
// Logs in as tenant B (issue82-flow-b), opens checkout?order=<alipay order>
// (the closest surface to Alipay's synchronous return: an unsigned customer-
// side reload), and asserts the order STAYS awaiting-payment — the sync face
// never advances the flow (AC2). Screenshot: 03-sync-return-no-confirmation.png.
import { createRequire } from 'node:module';
const require = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json');
const { chromium } = require('@playwright/test');

const WEB = 'http://localhost:5192';
const EV = new URL('.', import.meta.url).pathname;
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_sync_face_82.mjs <orderId>'); process.exit(2); }

const results = [];
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};

const browser = await chromium.launch();
try {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(30000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', 'issue82-flow-b@verify.local');
  await page.fill('#auth-password', 'issue82-Flow-Pw-b');
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=订单结算', { timeout: 30000 });
  await page.waitForTimeout(4000); // let one poll tick land (3s interval)
  const body = await page.locator('main').innerText();
  note('sync-face-no-advance', body.includes('等待付款') && body.includes('待付款（权益未开通）'),
    'checkout replay of the SAME order still reads 等待付款/待付款（权益未开通）— no confirmation from the sync face');
  note('sync-face-no-benefits', !body.includes('已生效') && !body.includes('已付款'),
    'no 已生效/已付款 text anywhere on the page');
  await page.screenshot({ path: `${EV}03-sync-return-no-confirmation.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
