// Issue #83 flow verification, acts 2-5 (API legs).
//   act 2  recovery: missed notify -> GET /orders/:id query-recovery pays.
//   act 3  duplicate notify: two replays -> 200 both, counts unchanged.
//   act 4  close races: paid-race (ORDER_PAID -> query decides, answers the
//          PAID wechat order, zero alipay creates) + clean switch (CLOSE 204,
//          old order channel-failed, new alipay order opens) + late success
//          on the closed order (idempotent, no second fulfillment).
//   act 5  the Lago four-object check for act 1's main chain (tenant 20) and
//          the product face (credits + features).
// Evidence files land beside this script.
//
// Run: node api_recovery_83.mjs  (env: FLOW83_PW required; FLOW83_BACKEND
//      default http://127.0.0.1:8095, FLOW83_STUB default http://127.0.0.1:8296)
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';

const BACKEND = process.env.FLOW83_BACKEND ?? 'http://127.0.0.1:8095';
const STUB = process.env.FLOW83_STUB ?? 'http://127.0.0.1:8296';
const EV = fileURLToPath(new URL('.', import.meta.url));
const PW = process.env.FLOW83_PW ?? '';
if (!PW) { console.error('missing required env: FLOW83_PW'); process.exit(2); }
const LAGO_KEY = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
  'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();

const results = [];
const note = (act, step, ok, detail) => {
  results.push({ act, step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | [${act}] ${step} | ${detail}`);
};
const sleep = (ms) => new Promise((r) => { setTimeout(r, ms); });
const files = {};

async function api(method, path, token, body) {
  const res = await fetch(`${BACKEND}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await res.json().catch(() => ({}));
  return { status: res.status, json };
}
async function registerAndLogin(label) {
  // Fresh identity per RUN (a rerun would otherwise replay an already-bought
  // tenant and every fresh-order assertion would face consumed state).
  const run = process.env.FLOW83_RUN ?? Date.now().toString(36);
  const email = `issue83-${label}-${run}@verify.local`;
  await api('POST', '/api/v1/auth/register', null, { username: `${label}-${run}`, email, password: PW });
  const { json } = await api('POST', '/api/v1/auth/login', null, { email, password: PW });
  return { token: json.token, tenant: json.active_tenant.id, email };
}
async function quote(token) {
  const { json } = await api('POST', '/api/v1/commercial/quotes', token,
    { plan_key: 'pro', plan_version: 1, subscription_version: 0 });
  return json.data.id;
}
async function purchase(token, quoteID, provider) {
  return api('POST', '/api/v1/commercial/purchases', token, { quote_id: quoteID, provider });
}
// The shared Lago stack answers short busy windows (503 unreachable) right
// after a settle/webhook burst; a fresh quote retries the same purchase —
// the channel order must LAND before the act's assertions read it.
async function purchaseUntilLanded(token, provider, log) {
  for (let i = 0; i < 4; i += 1) {
    const q = await quote(token);
    const res = await purchase(token, q, provider);
    const order = res.json?.data?.order;
    log.push(`purchase(${provider}) try${i}: HTTP ${res.status} order=${order?.id ?? '-'} url=${(order?.checkout_url ?? '').slice(0, 46)}`);
    if (res.status === 201 && order?.id) return { res, order };
    await sleep(4000);
  }
  return { res: null, order: null };
}
async function stubOrders() { return (await (await fetch(`${STUB}/stub/orders`)).json()); }
async function newestPendingOrder() {
  const orders = await stubOrders();
  const keys = Object.keys(orders);
  return keys[keys.length - 1] ?? '';
}
async function stubMark(id, transactionId) {
  const res = await fetch(`${STUB}/stub/mark`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ out_trade_no: id, state: 'SUCCESS', transaction_id: transactionId }) });
  return res.json();
}
async function stubNotify(id) {
  const res = await fetch(`${STUB}/stub/notify`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ out_trade_no: id }) });
  return { status: res.status, body: await res.json() };
}
function sql(query) {
  return execFileSync('sqlite3', ['data/issue83-flow.db', query], { encoding: 'utf8', cwd: fileURLToPath(new URL('../../..', import.meta.url)) }).trim();
}
function alipayLog() {
  try { return readFileSync(`${tmpdir()}/issue83-alipay-stub.log`, 'utf8'); } catch { return ''; }
}
function wechatLog() {
  try { return readFileSync(`${tmpdir()}/issue83-wechat-stub.log`, 'utf8'); } catch { return ''; }
}
function deliverWebhook(customer, org) {
  const deliver = `${EV}../issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py`;
  try {
    const out = execFileSync('python3', [deliver, '--customer', customer,
      '--base', 'http://127.0.0.1:48889', '--org', org, '--code', 'weknora-stripe'],
      { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], env: { ...process.env } });
    return out.trim().split('\n').pop();
  } catch { return 'deliver-pending'; }
}
function lagoCustomer(tenant) {
  try {
    return execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
      `select ppc.provider_customer_id from payment_provider_customers ppc join customers c on c.id=ppc.customer_id where c.external_id='weknora-tenant-${tenant}'`],
      { encoding: 'utf8' }).trim();
  } catch { return ''; }
}
async function driveToActive(token, tenant, log) {
  const ORG = '305eddac-1bbd-47a3-af15-219f1d39a27d';
  for (let i = 0; i < 18; i += 1) {
    const cus = lagoCustomer(tenant);
    if (cus) deliverWebhook(cus, ORG);
    const { json } = await api('GET', '/api/v1/commercial/purchase', token);
    const state = json.data?.state;
    log.push(`poll-${i}: ${state}`);
    if (state === 'active') return true;
    await sleep(8000);
  }
  return false;
}

// ---- act 2: the missed-notify recovery ----
{
  const act = 'act2-recovery';
  const u = await registerAndLogin('rec-a');
  const log = [`tenant=${u.tenant} (${u.email})`];
  const q1 = await quote(u.token);
  const p1 = await purchase(u.token, q1, 'wechat');
  const orderID = p1.json?.data?.order?.id ?? '';
  log.push(`purchase: HTTP ${p1.status} order=${orderID} url=${p1.json?.data?.order?.checkout_url ?? ''}`);
  note(act, 'wechat-order', p1.status === 201 && (p1.json?.data?.order?.checkout_url ?? '').startsWith('weixin://'), `order=${orderID}`);
  const mo = await newestPendingOrder();
  log.push(`stub NATIVE out_trade_no=${mo}`);
  // mark SUCCESS but NEVER push the notify (the missed-notification case)
  await stubMark(mo, `wx_txn_83_rec_${u.tenant}`);
  log.push('stub marked SUCCESS — NO notify pushed (missed notification)');
  const before = await api('GET', `/api/v1/commercial/orders/${orderID}`, u.token);
  log.push(`GET /orders/:id (recovery query): state=${before.json?.data?.state}`);
  note(act, 'query-recovery-pays', before.json?.data?.state === 'paid', `order state after recovery=${before.json?.data?.state}`);
  const mid = await api('GET', '/api/v1/commercial/purchase', u.token);
  log.push(`purchase state: ${mid.json?.data?.state}`);
  note(act, 'paid-awaiting', mid.json?.data?.state === 'paid_awaiting_activation', mid.json?.data?.state);
  const active = await driveToActive(u.token, u.tenant, log);
  note(act, 'recovered-active', active, 'purchase reached active after the missed-notify recovery');
  log.push(`active=${active}`);
  files['recovery-no-notify.txt'] = log.join('\n');
}

// ---- act 3: duplicate notify idempotence (act 1's order) ----
{
  const act = 'act3-duplicate';
  const log = [];
  const tokenF = JSON.parse(readFileSync(`${EV}seed-login-f.json`, 'utf8')).token;
  const orders = await stubOrders();
  const mo = Object.keys(orders).find((k) => orders[k].total === 9900 && (orders[k].transaction_id ?? '').startsWith('wx_txn_83_main_')) ?? '';
  log.push(`out_trade_no=${mo} state=${orders[mo]?.state} txn=${orders[mo]?.transaction_id}`);
  const first = await stubNotify(mo);
  const second = await stubNotify(mo);
  log.push(`replay1: stub ok=${first.body.ok} detail=${first.body.detail}`);
  log.push(`replay2: stub ok=${second.body.ok} detail=${second.body.detail}`);
  note(act, 'both-ack-success', first.body.ok === true && second.body.ok === true, 'both replays answered WeKnora 200 {"code":"SUCCESS"}');
  // tenant 20 legitimately holds TWO order rows: the retired alipay order
  // (closed by the channel switch, pending+channel_failed) and the paid
  // wechat order — the exactly-once assertions are on the FULFILLED count
  // and the single-channel-transaction bookkeeping, not the raw row count.
  const counts = {
    orders: sql("select count(*) from commercial_orders where tenant_id=20"),
    fulfilled: sql("select count(*) from commercial_orders where tenant_id=20 and state='fulfilled'"),
    outboxFulfill: sql("select count(*) from commercial_outbox_events where kind='fulfill' and event_key like '%ord_a33ff14916c09989'"),
    attempts: sql("select count(*) from commercial_payment_attempts where order_id='ord_a33ff14916c09989' and state='succeeded'"),
  };
  log.push(`counts: ${JSON.stringify(counts)}`);
  note(act, 'counts-unchanged', counts.fulfilled === '1' && counts.outboxFulfill === '1' && counts.attempts === '1',
    'fulfilled/outbox-fulfill/succeeded-attempt all exactly 1 (plus the retired pre-switch alipay row)');
  // v1.53 provider payments carry no invoice_id — resolve the tenant's
  // provider customer binding (uuid) and count through it.
  const cus20 = lagoCustomer(20);
  const lagoPayments = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
    `select count(*) from payments p join payment_provider_customers ppc on ppc.id=p.payment_provider_customer_id where ppc.provider_customer_id='${cus20}' and p.status='succeeded'`], { encoding: 'utf8' }).trim();
  log.push(`lago payments succeeded (tenant 20 purchase): ${lagoPayments}`);
  note(act, 'lago-payment-one', lagoPayments === '1', `lago succeeded payments = ${lagoPayments}`);
  files['duplicate-notify-idempotent.txt'] = log.join('\n');
}

// ---- act 4a: close hits an in-flight payment (ORDER_PAID race) ----
{
  const act = 'act4a-paid-race';
  const u = await registerAndLogin('race-a');
  const log = [`tenant=${u.tenant} (${u.email})`];
  const first = await purchaseUntilLanded(u.token, 'wechat', log);
  const orderID = first.order?.id ?? '';
  note(act, 'wechat-order-landed', Boolean(orderID), `order=${orderID}`);
  const mo = await newestPendingOrder();
  log.push(`wechat order=${orderID} out_trade_no=${mo}`);
  await stubMark(mo, `wx_txn_83_race_${u.tenant}`); // in-flight payment
  const precreateBefore = (alipayLog().match(/PRECREATE/g) ?? []).length;
  const second = await purchaseUntilLanded(u.token, 'alipay', log);
  const answered = second.order;
  log.push(`switch purchase: HTTP ${second.res?.status} order=${answered?.id} state=${answered?.payment ?? second.res?.json?.data?.state}`);
  note(act, 'answers-paid-old-order', second.res?.status === 201 && answered?.id === orderID && answered?.payment === 'paid',
    `answered the OLD wechat order ${answered?.id} payment=${answered?.payment}`);
  const precreateAfter = (alipayLog().match(/PRECREATE/g) ?? []).length;
  log.push(`alipay PRECREATE calls: before=${precreateBefore} after=${precreateAfter}`);
  note(act, 'zero-alipay-creates', precreateAfter === precreateBefore, 'the paid race never opened an alipay order');
  const wl = wechatLog().split('\n').filter((l) => l.includes(mo)).slice(-4);
  log.push(`wechat stub tail: ${wl.join(' | ')}`);
  note(act, 'close-orderpaid-then-query', wl.some((l) => l.includes('ORDER_PAID')) && wl.some((l) => l.includes('QUERY')),
    'CLOSE answered ORDER_PAID, the decisive QUERY ran');
  const outbox = sql(`select count(*) from commercial_outbox_events where kind='fulfill' and event_key like '%${orderID}'`);
  note(act, 'one-fulfill', outbox === '1', `fulfill events for the raced order = ${outbox}`);
  const active = await driveToActive(u.token, u.tenant, log);
  note(act, 'race-active', active, 'the raced payment still reached active (exactly-once fulfillment)');
  files['close-race-paid.txt'] = log.join('\n');
}

// ---- act 4b: clean channel switch + late success on the closed order ----
{
  const act = 'act4b-switch';
  const u = await registerAndLogin('sw-a');
  const log = [`tenant=${u.tenant} (${u.email})`];
  const first = await purchaseUntilLanded(u.token, 'wechat', log);
  const orderID = first.order?.id ?? '';
  const mo = await newestPendingOrder();
  log.push(`wechat order=${orderID} out_trade_no=${mo} (kept NOTPAY)`);
  const second = await purchaseUntilLanded(u.token, 'alipay', log);
  const newOrder = second.order;
  log.push(`switch purchase: HTTP ${second.res?.status} order=${newOrder?.id} url=${newOrder?.checkout_url}`);
  note(act, 'new-alipay-order', second.res?.status === 201 && (newOrder?.checkout_url ?? '').startsWith('https://qr.alipay.com/'),
    `new order carries an alipay qr link`);
  const wl = wechatLog().split('\n').filter((l) => l.includes(mo)).slice(-3);
  log.push(`wechat stub tail: ${wl.join(' | ')}`);
  note(act, 'wechat-close-204', wl.some((l) => l.includes('CLOSE') && l.includes('204')), 'the old wechat order was closed (204)');
  const oldRow = sql(`select channel_failed from commercial_orders where id='${orderID}'`);
  const attState = sql(`select state from commercial_payment_attempts where order_id='${orderID}'`);
  log.push(`old order channel_failed=${oldRow} attempt state=${attState}`);
  note(act, 'old-retired', oldRow === '1' && attState === 'closed', 'the old order is channel-failed and its attempt closed');
  const payable = sql(`select count(*) from commercial_orders where tenant_id=${u.tenant} and state='pending' and channel_failed=0 and checkout_url<>''`);
  note(act, 'one-payable-entry', payable === '1', `payable pending orders = ${payable} (the new one only)`);
  // defensive: a LATE success notify on the closed order must land idempotently
  await stubMark(mo, `wx_txn_83_late_${u.tenant}`);
  const late = await stubNotify(mo);
  log.push(`late notify ok=${late.body.ok}`);
  note(act, 'late-success-accepted', late.body.ok === true, 'the late success on the closed order answered 200 (fact retained)');
  const outboxFulfill = sql(`select count(*) from commercial_outbox_events where kind='fulfill' and event_key like '%${orderID}'`);
  const overPaid = sql(`select count(*) from commercial_outbox_events where kind='over_payment'`);
  log.push(`fulfill for closed order=${outboxFulfill} over_payment total=${overPaid}`);
  note(act, 'no-second-fulfillment', outboxFulfill === '1', 'the late success never minted a second fulfillment right');
  files['close-race-switch.txt'] = log.join('\n');
}

// ---- act 5: the Lago four objects for act 1 (tenant 20) + product face ----
{
  const act = 'act5-four-objects';
  const log = [];
  const lago = (q) => execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc', q], { encoding: 'utf8' }).trim();
  const lagoAPI = async (path) => (await (await fetch(`http://127.0.0.1:48889${path}`, { headers: { Authorization: `Bearer ${LAGO_KEY}` } })).json());
  const sub = lago("select status from subscriptions where external_id='weknora-tenant-20-purchase'");
  log.push(`subscription weknora-tenant-20-purchase status=${sub} (Lago enum 1=active)`);
  note(act, 'subscription-active', sub === '1', 'the purchase subscription is active (exactly one)');
  // The invoice face reads through the API (string enums beat DB ints).
  const invAPI = await lagoAPI('/api/v1/invoices?external_subscription_id=weknora-tenant-20-purchase');
  const inv = invAPI.invoices?.[0] ?? {};
  log.push(`invoice via API: status=${inv.status} payment_status=${inv.payment_status} fees_amount=${inv.fees_amount_cents} total=${inv.total_amount_cents}`);
  note(act, 'invoice-finalized-fee', inv.status === 'finalized' && inv.payment_status === 'succeeded' && Number(inv.fees_amount_cents) > 0,
    `gating invoice finalized+succeeded (fee face ${inv.fees_amount_cents}, the proration shape 82 already documented)`);
  const fees = lago("select count(*), coalesce(max(amount_cents),0) from fees where subscription_id=(select id from subscriptions where external_id='weknora-tenant-20-purchase') and fee_type=2");
  const [feeN, feeMax] = fees.split('|');
  log.push(`subscription fees = ${feeN} (max amount ${feeMax} fen)`);
  note(act, 'fee-exactly-one', feeN === '1', 'exactly one subscription fee on the gating invoice');
  // Lago's provider payments do not carry invoice_id in v1.53 — key them by
  // the tenant's provider customer binding instead.
  const cus = lagoCustomer(20);
  const pays = lago(`select count(*) from payments p join payment_provider_customers ppc on ppc.id=p.payment_provider_customer_id where ppc.provider_customer_id='${cus}' and p.status='succeeded'`);
  log.push(`succeeded provider payments (cus=${cus}) = ${pays}`);
  note(act, 'payment-one', pays === '1', `succeeded payments = ${pays}`);
  const walletsAny = lago("select w.name, wt.status, wt.amount from wallet_transactions wt join wallets w on w.id=wt.wallet_id join customers c on c.id=w.customer_id where c.external_id='weknora-tenant-20' and w.name like '%purchase%'");
  log.push(`purchase wallets: ${walletsAny.replace(/\n/g, ' ; ')}`);
  note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
  const tokenF = JSON.parse(readFileSync(`${EV}seed-login-f.json`, 'utf8')).token;
  const acct = await api('GET', '/api/v1/commercial/account', tokenF);
  files['account-after-wechat.json'] = JSON.stringify(acct.json, null, 2);
  const credits = JSON.stringify(acct.json?.data?.benefits?.credits ?? {});
  const features = JSON.stringify(acct.json?.data?.benefits?.features ?? {});
  log.push(`product credits=${credits}`);
  log.push(`product features=${features}`);
  note(act, 'product-credits-feature', credits.includes('10900000'), `credits face carries the 1.0 base + 9.9 purchase balance: ${credits.slice(0, 80)}`);
  note(act, 'advanced-models', features.includes('"advanced_models":true'), `features face: ${features}`);
  files['lago-four-objects-wechat.txt'] = log.join('\n');
}

for (const [name, content] of Object.entries(files)) {
  writeFileSync(`${EV}${name}`, content + '\n');
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
