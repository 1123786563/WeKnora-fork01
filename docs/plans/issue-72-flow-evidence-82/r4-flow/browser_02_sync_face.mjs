// Issue #82 R-4 re-verification — browser leg 2 (AC2 sync-return face).
// The signed notify body EXISTS at this point (built by alipay_sandbox_notify.py,
// not yet delivered). Re-visiting the checkout surface, hammering the manual
// refresh button and letting the poll tick must NOT advance the order: still
// 待付款（权益未开通）, and no 已付款 / 已生效 claim anywhere.
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { fileURLToPath } from 'node:url';

const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const EV = fileURLToPath(new URL('.', import.meta.url));
const ORDER = process.argv[2];
if (!ORDER) { console.error('usage: node browser_02_sync_face.mjs <orderId>'); process.exit(2); }
const EMAIL = process.env.FLOW82_EMAIL_A ?? '';
const PASSWORD = process.env.FLOW82_PASSWORD_A ?? '';
if (!EMAIL || !PASSWORD) {
  console.error('missing required env: FLOW82_EMAIL_A / FLOW82_PASSWORD_A');
  process.exit(2);
}

const results = [];
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};

const browser = await chromium.launch();
try {
  const page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });

  // Revisit + manual refresh x3 (sync-return face replay) + one poll window.
  // (The poll re-renders the card every 3s, which keeps Playwright's
  // actionability gate from settling — click at the DOM level instead.)
  for (let i = 0; i < 3; i++) {
    await page.locator('button:has-text("刷新订单状态")').first()
      .evaluate((b) => b.click()).catch(() => {});
    await page.waitForTimeout(1200);
  }
  await page.reload();
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });
  await page.waitForTimeout(3500); // let one poll tick land

  const body = await page.locator('main').innerText();
  note('sync-return-still-awaiting', body.includes('待付款（权益未开通）'),
    'after revisit/refresh x3/reload/poll: still 待付款（权益未开通）');
  note('sync-return-no-paid-claim', !body.includes('已付款'),
    'no 已付款 claim anywhere on the sync-return face');
  note('sync-return-no-active-claim', !body.includes('已生效'),
    'no 已生效 claim anywhere on the sync-return face');
  await page.screenshot({ path: `${EV}03-sync-return-no-confirmation.png`, fullPage: true });
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
