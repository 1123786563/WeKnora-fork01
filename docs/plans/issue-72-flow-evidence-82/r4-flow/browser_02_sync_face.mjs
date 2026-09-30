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

  // Revisit + manual refresh x3 (sync-return face replay) + one poll window.
  // (The poll re-renders the card every 3s, which keeps Playwright's
  // actionability gate from settling — click at the DOM level instead.)
  // (A-02) Every refresh click is COUNTED and a failed click is a FAIL
  // note, never a silent .catch(() => {}): the manual-refresh replay is
  // this script's core precondition, and swallowing its failure would let
  // the still-awaiting assertions below pass vacuously (no refresh ever
  // ran) — a false-positive evidence line.
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
  await page.waitForTimeout(3500); // let one poll tick land

  const body = await page.locator('main').innerText();
  // (A-12) Order-identity anchor: the face is only this evidence if the
  // page presents the TARGET order. The router feeds CheckoutPage from
  // ?order= — if that wiring ever regressed (param dropped/trimmed empty),
  // the page would open a NEW quote's order that renders the same
  // 待付款 face and every assertion below would pass against the wrong
  // order (the extra POST /purchases this script cannot otherwise see).
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
  // (A-03) A script-infrastructure failure (login refused, a selector
  // timeout — exactly the failure shapes a re-verification must capture)
  // lands as an explicit FAIL note: the RESULT summary JSON still prints
  // and the exit code is non-zero, so downstream readers can tell an
  // ASSERTION failure from an infrastructure one instead of seeing no
  // evidence at all.
  note('script-error', false, String(err));
  if (page) {
    try { await page.screenshot({ path: `${EV}03-sync-return-script-error.png`, fullPage: true }); } catch { /* best effort */ }
  }
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
