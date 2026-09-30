// Issue #82 R-4 re-verification ROUND 2 (post-fix) — browser leg 2 (AC2 sync-return face).
// Signed notify built but NOT delivered; revisit + refresh x3 + reload + one poll
// window must NOT advance the order. Same assertions as round 1.
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
let page;
try {
  page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(45000);
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });

  await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });

  // (The poll re-renders the card every 3s, which keeps Playwright's
  // actionability gate from settling — click at the DOM level instead.)
  // (A-02) Refresh clicks are COUNTED; a failed click is a FAIL note —
  // never the silent .catch(() => {}) that would let the still-awaiting
  // assertions pass vacuously (no refresh ever ran).
  let refreshClicked = 0;
  for (let i = 0; i < 3; i++) {
    try {
      await page.locator('button:has-text("刷新订单状态")').first().evaluate((b) => b.click());
      refreshClicked++;
    } catch (err) {
      note('refresh-click-failed', false,
        `manual refresh #${i + 1} could not be executed (${String(err).split('\n')[0]}) — the refresh precondition did NOT run`);
    }
    await page.waitForTimeout(1200);
  }
  note('refresh-executed-x3', refreshClicked === 3,
    `manual refresh executed ${refreshClicked}/3 times (core precondition of this evidence)`);
  await page.reload();
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });
  await page.waitForTimeout(3500);

  const body = await page.locator('main').innerText();
  // (A-12) Order-identity anchor: the face only counts as evidence if the
  // page presents the TARGET order (a dropped ?order= param would render
  // a NEW quote's order with the same 待付款 face).
  const orderLine = body.split('\n').find((l) => l.includes(ORDER)) ?? '(absent)';
  note('order-identity-target', body.includes(ORDER),
    `page presents the target order ${ORDER}: ${orderLine.trim()}`);
  note('sync-return-still-awaiting', body.includes('待付款（权益未开通）'),
    'after revisit/refresh x3/reload/poll: still 待付款（权益未开通）');
  note('sync-return-no-paid-claim', !body.includes('已付款'),
    'no 已付款 claim anywhere on the sync-return face');
  note('sync-return-no-active-claim', !body.includes('已生效'),
    'no 已生效 claim anywhere on the sync-return face');
  await page.screenshot({ path: `${EV}03-sync-return-no-confirmation.png`, fullPage: true });
} catch (err) {
  // (A-03) Infrastructure failure → explicit FAIL note; the RESULT summary
  // still prints so an assertion failure is distinguishable from a script
  // infrastructure failure.
  note('script-error', false, String(err));
  if (page) {
    try { await page.screenshot({ path: `${EV}03-sync-return-script-error.png`, fullPage: true }); } catch { /* best effort */ }
  }
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
