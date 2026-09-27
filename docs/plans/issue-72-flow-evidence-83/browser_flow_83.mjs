// Issue #83 flow verification, act 1 (browser main chain, real Playwright).
// Drives the REAL vite dev server (http://localhost:5196 -> proxy :8095):
//   login(tenant A) -> checkout (default alipay auto-purchase) -> SWITCH the
//   channel radio to wechat -> the explicit "改用微信支付重新发起支付" entry
//   (which exercises the #83 channel-switch wiring: close old alipay order,
//   open the wechat order) -> assert awaiting-payment with a weixin:// link
//   -> stub mark SUCCESS + signed notify -> poll to paid_awaiting_activation
//   -> settle rail (backend drain drives the REAL Stripe PI) + webhook
//   stand-in delivery -> poll to active ("权益已生效").
// Screenshots + the purchase state progression land in this evidence dir.
//
// Run: node browser_flow_83.mjs   (env: FLOW83_EMAIL/FLOW83_PASSWORD required,
//      FLOW83_WEB default http://localhost:5196)
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const WEB = process.env.FLOW83_WEB ?? 'http://localhost:5196';
const STUB = process.env.FLOW83_STUB ?? 'http://127.0.0.1:8296';
const BACKEND = process.env.FLOW83_BACKEND ?? 'http://127.0.0.1:8095';
const EV = fileURLToPath(new URL('.', import.meta.url));
const EMAIL = process.env.FLOW83_EMAIL ?? '';
const PASSWORD = process.env.FLOW83_PASSWORD ?? '';
if (!EMAIL || !PASSWORD) {
  console.error('missing required env: FLOW83_EMAIL / FLOW83_PASSWORD (no source-code fallback)');
  process.exit(2);
}

const results = [];
const progression = [];
const note = (step, ok, detail) => {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
};
const sleep = (ms) => new Promise((r) => { setTimeout(r, ms); });

// Fresh accounts meet two guide dialogs (new-user guide, contextual kb
// guide); their modal backdrops intercept every click. Dismiss whatever
// guide is open (both use the .wk-guide__close button class).
async function dismissGuides(page) {
  for (let i = 0; i < 3; i += 1) {
    const btns = page.locator('.wk-guide__close:visible');
    const n = await btns.count().catch(() => 0);
    if (n === 0) {
      // Nothing visible right now — Esc still clears a just-mounted dialog
      // whose close button is animating in.
      await page.keyboard.press('Escape').catch(() => {});
      await sleep(300);
      continue;
    }
    for (let j = 0; j < n; j += 1) {
      await btns.first().click({ timeout: 3000 }).catch(() => {});
      await sleep(200);
    }
    await page.keyboard.press('Escape').catch(() => {});
    await sleep(300);
  }
}

async function stubOrders() {
  const res = await fetch(`${STUB}/stub/orders`);
  return res.json();
}
async function stubMark(outTradeNo, transactionId) {
  const res = await fetch(`${STUB}/stub/mark`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ out_trade_no: outTradeNo, state: 'SUCCESS', transaction_id: transactionId }),
  });
  return res.json();
}
async function stubNotify(outTradeNo) {
  const res = await fetch(`${STUB}/stub/notify`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ out_trade_no: outTradeNo }),
  });
  const body = await res.json();
  return res.status === 200 && body.ok === true;
}
const TOKEN = process.env.FLOW83_TOKEN ?? '';
async function purchaseState() {
  if (!TOKEN) return { data: { state: 'env-missing-token' } };
  const res = await fetch(`${BACKEND}/api/v1/commercial/purchase`, {
    headers: { Authorization: `Bearer ${TOKEN}` },
  });
  return await res.json();
}

const browser = await chromium.launch();
let page;
try {
  page = await (await browser.newContext()).newPage();
  page.setDefaultTimeout(30000);

  // --- login (tenant A owner) ---
  await page.goto(`${WEB}/login`);
  await page.fill('#auth-email', EMAIL);
  await page.fill('#auth-password', PASSWORD);
  await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });
  note('login', true, `logged in as ${EMAIL}`);
  await dismissGuides(page);

  // --- checkout: the default-alipay order lands first (auto purchase) ---
  await page.goto(`${WEB}/platform/billing/checkout`);
  await page.waitForSelector('main', { timeout: 30000 });
  await dismissGuides(page);
  // The shared Lago stack occasionally answers a short busy window (503
  // unreachable, e.g. while processing an earlier webhook); the page's own
  // retry entry re-drives the same idempotent purchase — click it until the
  // awaiting-payment face lands.
  for (let attempt = 0; attempt < 4; attempt += 1) {
    const shown = await Promise.race([
      page.waitForSelector('text=待付款', { timeout: 20000 }).then(() => 'awaiting'),
      page.locator('button', { hasText: '重试（同一报价服务端幂等' }).waitFor({ timeout: 20000 }).then(() => 'retry'),
    ]).catch(() => 'timeout');
    if (shown === 'awaiting') break;
    if (shown === 'retry') {
      await dismissGuides(page);
      await page.locator('button', { hasText: '重试（同一报价服务端幂等' }).click();
      continue;
    }
    if (attempt === 3) note('checkout-awaiting-payment', false, 'order never landed (see backend log)');
  }
  // The new-user guide mounts lazily after the page settles — dismiss again
  // right before the interactions its backdrop would intercept.
  await dismissGuides(page);
  const alipayChecked = await page.locator('input[name="payment-channel"][value="alipay"]').isChecked();
  const wechatRadio = page.locator('input[name="payment-channel"][value="wechat"]');
  const radioVisible = await wechatRadio.count() === 1;
  note('channel-radio', alipayChecked && radioVisible, `alipay default=${alipayChecked}, wechat radio present=${radioVisible}`);

  // --- switch the channel: the explicit re-checkout entry (#83 wiring) ---
  await dismissGuides(page);
  await wechatRadio.check();
  const switchBtn = page.locator('button', { hasText: '改用微信支付重新发起支付' });
  await switchBtn.waitFor({ timeout: 10000 });
  await switchBtn.click();
  // The new order opens on the wechat channel: awaiting payment + weixin:// link.
  await page.waitForSelector('a[href^="weixin://wxpay/"]', { timeout: 60000 });
  await page.waitForSelector('text=待付款（权益未开通）', { timeout: 30000 });
  const payHref = await page.locator('a[href^="weixin://wxpay/"]').first().getAttribute('href');
  note('wechat-awaiting-payment', payHref.startsWith('weixin://wxpay/'), `checkout link ${payHref}`);
  const body = await page.locator('main').innerText();
  const orderLine = body.split('\n').find((l) => l.includes('ord_')) ?? '';
  const orderId = (orderLine.match(/ord_[0-9a-f]+/) ?? [''])[0];
  note('wechat-order-id', orderId !== '', orderId);
  await page.screenshot({ path: `${EV}01-checkout-wechat-awaiting-payment.png`, fullPage: true });

  // The stub saw exactly this wechat native order at the 99.00 CNY face.
  const orders = await stubOrders();
  const keys = Object.keys(orders);
  const liveOrder = keys.length >= 1 ? keys[keys.length - 1] : '';
  note('stub-native-order', liveOrder !== '' && orders[liveOrder].total === 9900,
    `stub NATIVE out_trade_no=${liveOrder} total=${orders[liveOrder]?.total} state=${orders[liveOrder]?.state}`);

  // --- simulated real payment: mark SUCCESS + push the SIGNED notify ---
  // The channel transaction id is UNIQUE per payment (a real WeChat
  // transaction id never repeats) — a per-run suffix keeps every round's
  // fact distinct like the real channel.
  const txnID = `wx_txn_83_main_${Date.now().toString(36)}`;
  await stubMark(liveOrder, txnID);
  const pushed = await stubNotify(liveOrder);
  note('stub-signed-notify', pushed, `TRANSACTION.SUCCESS pushed for ${liveOrder} (stub asserted WeKnora 200 {"code":"SUCCESS"})`);

  // --- paid_awaiting_activation face ---
  await page.waitForSelector('text=已付款，权益处理中', { timeout: 60000 });
  note('paid-awaiting-activation', true, 'checkout shows 已付款，权益处理中');
  let snap = await purchaseState();
  progression.push(`after-callback: ${JSON.stringify(snap.data?.state ?? snap.state ?? snap)}`);
  await page.screenshot({ path: `${EV}02-checkout-paid-awaiting-activation.png`, fullPage: true });
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  await dismissGuides(page);
  // BillingPage's middle-state copy is 「已付款待激活」(BillingPage.tsx:118);
  // the checkout page's is 「已付款，权益处理中」 — different faces, both
  // asserted precisely.
  await page.waitForSelector('text=已付款待激活', { timeout: 30000 });
  note('billing-paid-awaiting', true, 'billing plan row shows 已付款待激活');
  await page.screenshot({ path: `${EV}03-billing-paid-awaiting.png`, fullPage: true });

  // --- settle rail + webhook finalize: poll purchase to active ---
  const deliver = `${EV}../issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py`;
  const ORG = '305eddac-1bbd-47a3-af15-219f1d39a27d'; // 82flow stack seed org (read-only)
  const TENANT = process.env.FLOW83_TENANT ?? '17';
  const lagoCustomer = () => {
    try {
      return execFileSync('docker', [
        'exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
        "select ppc.provider_customer_id from payment_provider_customers ppc " +
        `join customers c on c.id=ppc.customer_id where c.external_id='weknora-tenant-${TENANT}'`,
      ], { encoding: 'utf8' }).trim();
    } catch { return ''; }
  };
  let active = false;
  let attempts = 0;
  let lastDeliver = 'not-delivered';
  while (!active && attempts < 24) {
    attempts += 1;
    const cus = lagoCustomer();
    if (cus) {
      try {
        const out = execFileSync('python3', [deliver, '--customer', cus,
          '--base', 'http://127.0.0.1:48889', '--org', ORG,
          '--code', 'weknora-stripe'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] });
        lastDeliver = out.trim().split('\n').pop();
      } catch { lastDeliver = `deliver-pending (attempt ${attempts}, cus=${cus})`; }
    } else {
      lastDeliver = `customer-binding-pending (attempt ${attempts})`;
    }
    snap = await purchaseState();
    const state = snap.data?.state ?? snap.state;
    progression.push(`poll-${attempts}: ${state} (webhook: ${lastDeliver})`);
    if (state === 'active') { active = true; break; }
    await sleep(10000);
  }
  note('settle-webhook-active', active, `purchase reached active after ${attempts} polls; last webhook: ${lastDeliver}`);

  // --- the active face, browser side (revisit WITH the order id so the
  // page projects the existing order instead of submitting a fresh quote
  // that the now-active purchase would refuse with purchase_not_awaiting) ---
  await page.goto(`${WEB}/platform/billing/checkout?order=${orderId}`);
  await page.waitForSelector('main', { timeout: 30000 });
  await dismissGuides(page);
  await page.waitForSelector('text=权益已生效', { timeout: 60000 });
  note('checkout-active-face', true, 'checkout shows 权益已生效');
  const finalBody = await page.locator('main').innerText();
  note('no-residue', !finalBody.includes('待付款') && !finalBody.includes('已付款，权益处理中'),
    'no awaiting/paid residue on the active face');
  await page.screenshot({ path: `${EV}04-checkout-active.png`, fullPage: true });
  await page.goto(`${WEB}/platform/billing`);
  await page.waitForSelector('main', { timeout: 30000 });
  await dismissGuides(page);
  await page.waitForSelector('text=已生效', { timeout: 30000 });
  await page.screenshot({ path: `${EV}05-billing-active.png`, fullPage: true });
  note('billing-active-face', true, 'billing shows 已生效');

  const fs = await import('node:fs');
  fs.writeFileSync(`${EV}purchase-state-progression.txt`,
    `#83 act1 purchase state progression (${new Date().toISOString()})\n` +
    progression.join('\n') + '\n');
} finally {
  await browser.close();
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
