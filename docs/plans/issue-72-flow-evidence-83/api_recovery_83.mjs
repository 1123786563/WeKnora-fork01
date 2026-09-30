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
  const reg = await api('POST', '/api/v1/auth/register', null, { username: `${label}-${run}`, email, password: PW });
  if (reg.status >= 500) {
    // (C-04) A 5xx register is not silent: carrying undefined forward would
    // drown the root cause under a cascade of unrelated FAILs.
    throw new Error(`registerAndLogin(${label}): register answered HTTP ${reg.status} for ${email}`);
  }
  const { json } = await api('POST', '/api/v1/auth/login', null, { email, password: PW });
  // (C-04) The login answer is VALIDATED before anything downstream runs:
  // an undefined token/tenant would send "Bearer undefined" on every later
  // call and interpolate undefined into SQL — a misleading cascade.
  if (!json?.token || !json?.active_tenant?.id) {
    throw new Error(`registerAndLogin(${label}): login for ${email} missing token/active_tenant (register HTTP ${reg.status}, login fields: token=${Boolean(json?.token)} tenant=${json?.active_tenant?.id})`);
  }
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
  // (C-04) An exhausted budget is TERMINAL for the act — returning null
  // used to let an empty order id flow into stubMark (marking a leftover
  // order SUCCESS!) and into id='' SQL.
  throw new Error(`purchaseUntilLanded(${provider}): all 4 attempts failed to land a channel order (see the act log above)`);
}
async function stubOrders() { return (await (await fetch(`${STUB}/stub/orders`)).json()); }
// (C-03) The stub key order is INSERTION order — "the last key" guessed
// this round's order with NO filtering, so a leftover order from an earlier
// round (the stub survives reruns), a retry's sibling channel order, or a
// concurrent order would get marked SUCCESS and receive the signed notify.
// Filter to the genuinely-pending 9900 face and take the LAST of those.
async function newestPendingOrder() {
  const orders = await stubOrders();
  const pending = Object.entries(orders)
    .filter(([, v]) => v?.state === 'NOTPAY' && v?.total === 9900)
    .map(([k]) => k);
  return pending[pending.length - 1] ?? '';
}
async function stubMark(id, transactionId) {
  const res = await fetch(`${STUB}/stub/mark`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ out_trade_no: id, state: 'SUCCESS', transaction_id: transactionId }) });
  return res.json();
}
async function stubNotify(id) {
  const res = await fetch(`${STUB}/stub/notify`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ out_trade_no: id }) });
  return { status: res.status, body: await res.json() };
}
// FLOW83_DB (default data/issue83-flow.db) + FLOW83_TENANT parameterize the
// round: reruns land on a fresh sqlite db and a fresh protagonist tenant, so
// no counts may be hardcoded (v2 rerun-friendliness fix).
const DB_PATH = process.env.FLOW83_DB ?? 'data/issue83-flow.db';
// (C-83/C-01) seed-login-f.json is REDACTED on disk since the credential
// redline fix — the tenant identity still rides it, but the API token comes
// from the env like every other leg of this round.
const MAIN_TENANT_JSON = JSON.parse(readFileSync(`${EV}seed-login-f.json`, 'utf8'));
const TOKEN_MAIN = process.env.FLOW83_TOKEN ?? '';
if (!TOKEN_MAIN) {
  console.error('missing required env: FLOW83_TOKEN (the main-chain protagonist login token; seed-login-f.json is redacted on disk)');
  process.exit(2);
}
const TEN = String(process.env.FLOW83_TENANT ?? MAIN_TENANT_JSON.active_tenant?.id ?? '');
// (C-02) EVERY value that reaches a SQL string passes a whitelist first —
// a failed shape check writes the act log and terminates: a broken or
// always-false query mints misleading PASS/FAIL evidence.
// Lago-side provider bindings are Stripe customer ids (cus_ + alnum), not
// UUIDs — the whitelist matches the CLOSED id shape either side can carry.
const LAGO_ID_RE = /^(?:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|cus_[A-Za-z0-9]{10,30})$/;
function sqlShape(name, value, re) {
  if (!re.test(value)) {
    // (OCR84-R1-22) process.exit(2) 立即终止进程，绕过末尾的 writeFileSync 循环
    // 与 RESULT 汇总——此前累积的全部证据文件（含本行的 sql-shape-failure.txt）
    // 全部丢失。改为 throw 交给顶层 catch：证据落盘 + RESULT 打印 + 非零退出。
    files['sql-shape-failure.txt'] = `refusing to interpolate ${name} into SQL: ${JSON.stringify(value)}\n`;
    throw new Error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
  }
  return value;
}
const tenantShape = (t) => sqlShape('tenant id', String(t), /^\d+$/);
const orderShape = (o) => sqlShape('order id', String(o), /^ord_[0-9a-f]+$/);
const merchantOrderShape = (m) => sqlShape('merchant order id', String(m), /^mo_[0-9a-f]+$/);
const uuidShape = (u) => sqlShape('lago provider binding id', String(u), LAGO_ID_RE);
if (!/^\d+$/.test(TEN)) {
  console.error(`bad tenant id for SQL interpolation: ${TEN}`);
  process.exit(2);
}
function sql(query) {
  return execFileSync('sqlite3', [DB_PATH, query], { encoding: 'utf8', cwd: fileURLToPath(new URL('../../..', import.meta.url)) }).trim();
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
  // (OCR84-R1-26) C-02 不变量闭合：进入 docker psql 查询字符串的每个值都先过
  // 白名单——act2/act4a 的 driveToActive 传入 u.tenant（registerAndLogin 只校验
  // 字段存在），此路径此前是唯一未校验的 TEN 入口。
  const t = tenantShape(tenant);
  try {
    return execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
      `select ppc.provider_customer_id from payment_provider_customers ppc join customers c on c.id=ppc.customer_id where c.external_id='weknora-tenant-${t}'`],
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

// (C-04) One top-level guard over every act: an aborted act still
// writes its accumulated log files and prints the RESULT summary — a
// transport/shape failure must leave diagnosable evidence, never a
// half-written run.
try {
  // ---- act 2: the missed-notify recovery ----
  {
    const act = 'act2-recovery';
    const u = await registerAndLogin('rec-a');
    const log = [`tenant=${u.tenant} (${u.email})`];
    // (OCR84-R1-22) 本幕落盘兜底：块内任何 throw 不得丢失累积的诊断日志——
    // finally 先落盘再重抛，顶层 catch 写 files + RESULT。
    try {
      // (OCR84-R1-27) act2 紧跟 act1 的 settle+webhook，正是共享 Lago 栈 503
      // busy window 最易触发的位置——五幕中唯一没有落地保障的一手 purchase 改
      // 用 purchaseUntilLanded（与 acts 4a/4b 同款）；失败时抛错终止本幕，不再
      // 以 orderID='' 继续命中跨轮残留订单。
      const p1 = await purchaseUntilLanded(u.token, 'wechat', log);
      const orderID = orderShape(p1.order?.id ?? '');
      note(act, 'wechat-order-landed', (p1.order?.checkout_url ?? '').startsWith('weixin://'), `order=${orderID}`);
      const mo = merchantOrderShape(await newestPendingOrder());
      log.push(`stub NATIVE out_trade_no=${mo} (NOTPAY/9900-filtered)`);
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
    } finally {
      if (!files['recovery-no-notify.txt']) files['recovery-no-notify.txt'] = log.join('\n');
    }
  }

  // ---- act 3: duplicate notify idempotence (act 1's order) ----
  {
    const act = 'act3-duplicate';
    const log = [];
    // (OCR84-R1-22) 本幕落盘兜底：任何 throw 先落盘再重抛。
    try {
      const tokenF = TOKEN_MAIN;
      const orders = await stubOrders();
      // (OCR84-R1-25) stub 跨轮存活（C-03），插入序 FIRST 命中的是旧轮的
      // wx_txn_83_main_ 订单——notify 重放会作用于错误订单、后续计数断言却针对
      // 本轮 TEN。与 newestPendingOrder 同形：过滤后取最后一个（本轮最新）。
      const mo = Object.keys(orders).filter((k) => orders[k].total === 9900 && (orders[k].transaction_id ?? '').startsWith('wx_txn_83_main_')).pop() ?? '';
      log.push(`tenant=${TEN} out_trade_no=${mo} state=${orders[mo]?.state} txn=${orders[mo]?.transaction_id}`);
      const first = await stubNotify(merchantOrderShape(mo));
      const second = await stubNotify(merchantOrderShape(mo));
      log.push(`replay1: stub ok=${first.body.ok} detail=${first.body.detail}`);
      log.push(`replay2: stub ok=${second.body.ok} detail=${second.body.detail}`);
      note(act, 'both-ack-success', first.body.ok === true && second.body.ok === true, 'both replays answered WeKnora 200 {"code":"SUCCESS"}');
      // The protagonist legitimately holds TWO order rows: the retired alipay order
      // (closed by the channel switch, pending+channel_failed) and the paid
      // wechat order — the exactly-once assertions are on the FULFILLED count
      // and the single-channel-transaction bookkeeping, not the raw row count.
      const fulfilledOrder = sql(`select id from commercial_orders where tenant_id=${TEN} and state='fulfilled'`);
      // (C-02) A malformed answer TERMINATES the act — noting it and continuing
      // used to interpolate the same broken value into two more queries.
      orderShape(fulfilledOrder);
      const counts = {
        orders: sql(`select count(*) from commercial_orders where tenant_id=${TEN}`),
        fulfilled: sql(`select count(*) from commercial_orders where tenant_id=${TEN} and state='fulfilled'`),
        outboxFulfill: sql(`select count(*) from commercial_outbox_events where kind='fulfill' and event_key like '%${fulfilledOrder}'`),
        attempts: sql(`select count(*) from commercial_payment_attempts where order_id='${fulfilledOrder}' and state='succeeded'`),
      };
      log.push(`fulfilled order=${fulfilledOrder} counts: ${JSON.stringify(counts)}`);
      note(act, 'counts-unchanged', counts.fulfilled === '1' && counts.outboxFulfill === '1' && counts.attempts === '1',
        'fulfilled/outbox-fulfill/succeeded-attempt all exactly 1 (plus the retired pre-switch alipay row)');
    // v1.53 provider payments carry no invoice_id — resolve the tenant's
    // provider customer binding (uuid) and count through it.
    const cus20 = uuidShape(lagoCustomer(TEN));
    const lagoPayments = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
      `select count(*) from payments p join payment_provider_customers ppc on ppc.id=p.payment_provider_customer_id where ppc.provider_customer_id='${cus20}' and p.status='succeeded'`], { encoding: 'utf8' }).trim();
    log.push(`lago payments succeeded (tenant ${TEN} purchase): ${lagoPayments}`);
    note(act, 'lago-payment-one', lagoPayments === '1', `lago succeeded payments = ${lagoPayments}`);
    files['duplicate-notify-idempotent.txt'] = log.join('\n');
    } finally {
      if (!files['duplicate-notify-idempotent.txt']) files['duplicate-notify-idempotent.txt'] = log.join('\n');
    }
  }

  // ---- act 4a: close hits an in-flight payment (ORDER_PAID race) ----
  {
    const act = 'act4a-paid-race';
    const u = await registerAndLogin('race-a');
    const log = [`tenant=${u.tenant} (${u.email})`];
    // (OCR84-R1-22) 本幕落盘兜底：任何 throw 先落盘再重抛。
    try {
    const first = await purchaseUntilLanded(u.token, 'wechat', log);
    const orderID = orderShape(first.order?.id ?? '');
    note(act, 'wechat-order-landed', true, `order=${orderID}`);
    const mo = merchantOrderShape(await newestPendingOrder());
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
    } finally {
      if (!files['close-race-paid.txt']) files['close-race-paid.txt'] = log.join('\n');
    }
  }

  // ---- act 4b: clean channel switch + late success on the closed order ----
  {
    const act = 'act4b-switch';
    const u = await registerAndLogin('sw-a');
    const log = [`tenant=${u.tenant} (${u.email})`];
    // (OCR84-R1-22) 本幕落盘兜底：任何 throw 先落盘再重抛。
    try {
    const first = await purchaseUntilLanded(u.token, 'wechat', log);
    const orderID = orderShape(first.order?.id ?? '');
    const mo = merchantOrderShape(await newestPendingOrder());
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
    const payable = sql(`select count(*) from commercial_orders where tenant_id=${tenantShape(u.tenant)} and state='pending' and channel_failed=0 and checkout_url<>''`);
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
    } finally {
      if (!files['close-race-switch.txt']) files['close-race-switch.txt'] = log.join('\n');
    }
  }

  // ---- act 5: the Lago four objects for act 1 (protagonist tenant) + product face ----
  {
    const act = 'act5-four-objects';
    const log = [];
    // (OCR84-R1-22) 本幕落盘兜底：任何 throw 先落盘再重抛。
    try {
    const lago = (q) => execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc', q], { encoding: 'utf8' }).trim();
    const lagoAPI = async (path) => (await (await fetch(`http://127.0.0.1:48889${path}`, { headers: { Authorization: `Bearer ${LAGO_KEY}` } })).json());
    const sub = lago(`select status from subscriptions where external_id='weknora-tenant-${TEN}-purchase'`);
    log.push(`subscription weknora-tenant-${TEN}-purchase status=${sub} (Lago enum 1=active)`);
    note(act, 'subscription-active', sub === '1', 'the purchase subscription is active (exactly one)');
    // The invoice face reads through the API (string enums beat DB ints).
    const invAPI = await lagoAPI(`/api/v1/invoices?external_subscription_id=weknora-tenant-${TEN}-purchase`);
    const inv = invAPI.invoices?.[0] ?? {};
    log.push(`invoice via API: status=${inv.status} payment_status=${inv.payment_status} fees_amount=${inv.fees_amount_cents} total=${inv.total_amount_cents}`);
    note(act, 'invoice-finalized-fee', inv.status === 'finalized' && inv.payment_status === 'succeeded' && Number(inv.fees_amount_cents) > 0,
      `gating invoice finalized+succeeded (fee face ${inv.fees_amount_cents}, the proration shape 82 already documented)`);
    const fees = lago(`select count(*), coalesce(max(amount_cents),0) from fees where subscription_id=(select id from subscriptions where external_id='weknora-tenant-${TEN}-purchase') and fee_type=2`);
    const [feeN, feeMax] = fees.split('|');
    log.push(`subscription fees = ${feeN} (max amount ${feeMax} fen)`);
    note(act, 'fee-exactly-one', feeN === '1', 'exactly one subscription fee on the gating invoice');
    // Lago's provider payments do not carry invoice_id in v1.53 — key them by
    // the tenant's provider customer binding instead.
    const cus = uuidShape(lagoCustomer(TEN));
    const pays = lago(`select count(*) from payments p join payment_provider_customers ppc on ppc.id=p.payment_provider_customer_id where ppc.provider_customer_id='${cus}' and p.status='succeeded'`);
    log.push(`succeeded provider payments (cus=${cus}) = ${pays}`);
    note(act, 'payment-one', pays === '1', `succeeded payments = ${pays}`);
    const walletsAny = lago(`select w.name, wt.status, wt.amount from wallet_transactions wt join wallets w on w.id=wt.wallet_id join customers c on c.id=w.customer_id where c.external_id='weknora-tenant-${TEN}' and w.name like '%purchase%'`);
    log.push(`purchase wallets: ${walletsAny.replace(/\n/g, ' ; ')}`);
    note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
    const tokenF = TOKEN_MAIN;
    const acct = await api('GET', '/api/v1/commercial/account', tokenF);
    files['account-after-wechat.json'] = JSON.stringify(acct.json, null, 2);
    const credits = JSON.stringify(acct.json?.data?.benefits?.credits ?? {});
    const features = JSON.stringify(acct.json?.data?.benefits?.features ?? {});
    log.push(`product credits=${credits}`);
    log.push(`product features=${features}`);
    note(act, 'product-credits-feature', credits.includes('10900000'), `credits face carries the 1.0 base + 9.9 purchase balance: ${credits.slice(0, 80)}`);
    note(act, 'advanced-models', features.includes('"advanced_models":true'), `features face: ${features}`);
    files['lago-four-objects-wechat.txt'] = log.join('\n');
    } finally {
      if (!files['lago-four-objects-wechat.txt']) files['lago-four-objects-wechat.txt'] = log.join('\n');
    }
  }

} catch (err) {
  note('script', 'uncaught-flow-error', false, String(err));
}

for (const [name, content] of Object.entries(files)) {
  writeFileSync(`${EV}${name}`, content + '\n');
}
console.log('RESULT ' + JSON.stringify(results));
process.exit(results.every((r) => r.ok) ? 0 : 1);
