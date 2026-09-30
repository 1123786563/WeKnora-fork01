import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// T30 小程序 持续规则、额度与提醒的可观察行为：真实 transport + AuthCoordinator 装配
// （T24/T26/T28/T32 同款），假后端按 method+pathname 路由。三份冻结合同：
// 规则（T13：POST /rules、GET /rules/receipt、GET /rules/:id——与 Web 同版本读写，
// 修改后下次运行计划重新排程）；额度（T21：GET /usage/estimate 只读预估，超额只阻
// 新收费动作，历史完整可访问）；提醒（T20：POST/GET /reminders 隐私正文冻结模板，
// 退订停推待办可读）。订阅消息走 wx.requestSubscribeMessage（平台原生 API，已记录
// 的原生能力例外）：只携带模板 id，零敏感细节；拒绝/不可用一律回落站内待办，
// 绝不伪造已送达。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: `data:text/javascript,${encodeURIComponent(`import Taro from '${stubURL}';export const useDidHide=()=>{};export const useDidShow=()=>{};export default Taro;`)}`, shortCircuit: true };
    if (specifier === '../components/ui.tsx') return { url: 'data:text/javascript,export%20const%20Screen%3D%22Screen%22%3Bexport%20const%20Card%3D%22Card%22%3Bexport%20const%20Action%3D%22Action%22%3Bexport%20const%20Field%3D%22Field%22%3Bexport%20const%20Notice%3D%22Notice%22%3Bexport%20const%20Badge%3D%22Badge%22%3Bexport%20const%20DataBoundary%3D%22DataBoundary%22%3Bexport%20const%20useData%3D()%3D%3E()%3D%3E()%3D%3Eundefined%3Bexport%20const%20useAction%3D()%3D%3E(%7Bbusy%3Afalse,run%3Afn%3D%3Efn()%7D)%3Bexport%20const%20useSession%3D()%3D%3E(%7BuserId%3A%22u1%22,tenantId%3A%221%22%7D)', shortCircuit: true };
    if (specifier === '@tarojs/components') return { url: 'data:text/javascript,export%20const%20View%3D%22View%22%3Bexport%20const%20Text%3D%22Text%22%3Bexport%20const%20Button%3D%22Button%22%3Bexport%20const%20Input%3D%22Input%22%3Bexport%20const%20Textarea%3D%22Textarea%22%3Bexport%20const%20Image%3D%22Image%22%3Bexport%20const%20ScrollView%3D%22ScrollView%22', shortCircuit: true };
    if (/\.(png|scss|css)$/.test(specifier)) return { url: 'data:text/javascript,export%20default%20%22%22', shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtime = await import('../src/services/runtime.ts');
const career = await import('../src/services/career.ts');
const platform = await import('../src/adapters/career-platform.ts');
const rulesPage = await import('../src/career/rules-usage-reminders.tsx');
const { clearPrivateCache } = await import('../src/platform/storage.ts');

const T = '2026-09-25T08:00:00Z';
const meA = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const open3 = call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } });

// ---- 规则合同 fixture（decodeSetRuleReceipt / decodeRuleView 必须全部通过）----
const estimate = () => ({ triggersPerDay: 1, sourcesPerTrigger: 3, estimatedSearchesPerDay: 3, basis: '按触发间隔与已核验来源数的确定性口径' });
const ruleReceipt = (over = {}) => ({
  kind: 'rule_set', requestId: 'srv-rule', ruleId: 'rule-1', query: '上海 前端开发 实习',
  intervalMinutes: 1440, status: 'enabled', revision: 1, nextDueAt: T, estimate: estimate(), ...over,
});
const ruleView = (over = {}) => ({
  ruleId: 'rule-1', query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled',
  revision: 1, lastPeriod: 1, nextDueAt: T, estimate: estimate(), runs: [], todos: [],
  createdAt: T, updatedAt: T, ...over,
});
// ---- 额度合同 fixture（decodeUsageEstimateView 必须全部通过）----
const usageEstimate = (over = {}) => ({
  kind: 'usage_estimate', operation: 'search_once', costUnits: 1,
  conditions: ['每次找岗（一次性或持续规则的每次触发）消耗 1 个额度单位'],
  periodStart: '2026-09-01T00:00:00Z', periodEnd: '2026-10-01T00:00:00Z',
  limitUnits: 10, reservedUnits: 0, settledUnits: 9, remainingUnits: 1, wouldAdmit: true, ...over,
});
// ---- 提醒合同 fixture（decodeReminderReceipt / decodeReminderList 必须全部通过）----
const reminderView = (over = {}) => ({
  reminderId: 'rem-1', sourceKind: 'progress_event', sourceId: 'evt-1', applicationId: 'app-1', opportunityId: 'opp-1',
  noticeKey: 'progress_updated', notice: '你有新的求职进展，请登录查看。', status: 'open', createdAt: T, ...over,
});
const reminderReceipt = (over = {}) => ({
  kind: 'reminder_set', requestId: 'srv-rem', reminderId: 'rem-1', sourceKind: 'progress_event',
  sourceId: 'evt-1', applicationId: 'app-1', opportunityId: 'opp-1', deduplicated: false, noticeKey: 'progress_updated',
  notice: '你有新的求职进展，请登录查看。', status: 'open', revision: 3, createdAt: T, ...over,
});
const confirmedAct = (key, value, revision) => ({
  kind: 'confirmed', requestId: 'srv-act', revision,
  fact: { key, value, revision, source: { kind: 'user', label: '微信小程序' }, confirmation: { userId: 'u1', confirmedAt: T }, confirmedAt: T },
});

function backend(routes) {
  stub.use(call => {
    const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
    const path = new URL(call.options.url).pathname;
    let fn = routes[`${method} ${path}`];
    if (fn === undefined) {
      const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
      if (prefix) fn = routes[prefix];
    }
    if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
    fn(call);
  });
}
async function freshLogin(extraRoutes = {}) {
  stub.reset();
  backend({
    'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: meA() }),
    'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: { code: 0, msg: 'success', data: { protocol_minimum: 1, protocol_maximum: 5 } } }),
    'GET /api/v1/career/open': open3,
    ...extraRoutes,
  });
  await runtime.auth.login('u@example.test', 'pw');
  career.resetCareerDesk();
}
const careerCall = suffix => stub.state.calls.filter(c => new URL(c.options.url).pathname.startsWith('/api/v1/career') && (!suffix || new URL(c.options.url).pathname.includes(suffix)));
const errorCode = error => error?.code;
const ruleWrites = () => stub.state.calls.filter(c => /\/rules$/.test(new URL(c.options.url).pathname) && (c.options.method ?? 'GET') === 'POST');

// ---- A 组：规则与 Web 同版本读写 + 下次运行计划 + 三 seam ----

test('A1: saving a rule posts the frozen T13 contract and persists the rule reference for the scope', async () => {
  await freshLogin({
    'POST /api/v1/career/rules': call => stub.succeed(call, { data: ruleReceipt() }),
  });
  await career.loadCareer();
  const receipt = await platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 });
  assert.equal(receipt.kind, 'rule_set');
  assert.equal(receipt.ruleId, 'rule-1');
  assert.equal(receipt.nextDueAt, T, 'the receipt carries the next-run plan');
  const body = ruleWrites()[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['expectedRevision', 'intervalMinutes', 'query', 'requestId', 'status'], 'the body must match the frozen SetRuleInput (DisallowUnknownFields)');
  assert.equal(body.expectedRevision, 3, 'the rule CAS houses against the profile head revision — same domain as the web RulePage');
  assert.equal(ruleWrites()[0].options.header.Authorization ?? ruleWrites()[0].options.header.authorization, 'Bearer t1');
  assert.equal(platform.readStoredRuleId(), 'rule-1', 'the saved rule id is kept so a later entry re-opens the same rule');
});

test('A2: the stored rule id re-opens the same version — a cross-client edit changes the plan the miniprogram reads', async () => {
  await freshLogin({
    'POST /api/v1/career/rules': call => stub.succeed(call, { data: ruleReceipt() }),
    'GET /api/v1/career/rules/rule-1': call => stub.succeed(call, { data: ruleView() }),
  });
  await platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 });
  const first = await platform.readRule(platform.readStoredRuleId());
  assert.equal(first.revision, 1);
  assert.equal(first.status, 'enabled');
  // Web（或另一端）把间隔改成 60 分钟并停用：服务端 revision 2，下次运行计划重排。
  const afterWebEdit = ruleView({ intervalMinutes: 60, status: 'paused', revision: 2, nextDueAt: undefined, updatedAt: T });
  stub.use(call => { if (new URL(call.options.url).pathname === '/api/v1/career/rules/rule-1') stub.succeed(call, { data: afterWebEdit }); });
  const second = await platform.readRule(platform.readStoredRuleId());
  assert.equal(second.revision, 2, 'same version as the web edit — one rule, one history');
  assert.equal(second.intervalMinutes, 60);
  assert.equal(second.status, 'paused');
  assert.equal(second.nextDueAt, undefined, 'paused cancels the next scheduled run — the plan follows the new conditions');
});

test('A3: a rule revision conflict is definite and typed — no recovery intent (seam: 冲突回执)', async () => {
  await freshLogin({
    'POST /api/v1/career/rules': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 7 } } }),
  });
  await career.loadCareer();
  await assert.rejects(platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 }), error => error.code === 'revision_conflict' && error.currentRevision === 7);
  assert.equal(platform.pendingRuleWrite(), null, 'a definite conflict leaves no pending intent');
  assert.equal(platform.readStoredRuleId(), undefined, 'nothing is persisted for a refused write');
});

test('A4: an unknown rule outcome recovers through the original request id and revision (seam: 未知对账)', async () => {
  let postFails = true; let serverRevision = 3;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/rules': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: ruleReceipt() }); },
    'GET /api/v1/career/rules/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 }), error => error.code === 'outcome_unknown');
  const pending = platform.pendingRuleWrite();
  assert.ok(pending, 'the unresolved rule write is kept for recovery');
  assert.equal(pending.requestId, ruleWrites()[0].options.data.requestId);
  assert.equal(pending.input.expectedRevision, 3, 'the intent stores the original revision for a safe replay');
  serverRevision = 9; // meanwhile the profile moved on — a different domain must not leak into the replay
  await career.refreshCareer();
  postFails = false;
  const receipt = await platform.retryPendingRule();
  assert.equal(receipt.ruleId, 'rule-1');
  const bodies = ruleWrites().map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe replay');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the replay keeps the original request id — no second rule write');
  assert.equal(bodies[1].expectedRevision, 3, 'the replay replays the original revision, never the moved-on profile head');
  assert.equal(platform.pendingRuleWrite(), null, 'a successful replay clears the intent');
  assert.equal(platform.readStoredRuleId(), 'rule-1');
});

test('A4b: a reconciled rule receipt clears the intent without a second write', async () => {
  await freshLogin({
    'POST /api/v1/career/rules': call => stub.fail(call, 'request:fail timeout'),
    'GET /api/v1/career/rules/receipt': call => stub.succeed(call, { data: ruleReceipt({ requestId: new URL(call.options.url).searchParams.get('requestId') }) }),
  });
  await career.loadCareer();
  await assert.rejects(platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 }), error => error.code === 'outcome_unknown');
  const pending = platform.pendingRuleWrite();
  const receipt = await platform.reconcilePendingRule();
  assert.equal(receipt.requestId, pending.requestId, 'the durable receipt answers the original request');
  assert.equal(ruleWrites().length, 1, 'reconciliation is read-only — no second POST');
  assert.equal(platform.pendingRuleWrite(), null);
  assert.equal(platform.readStoredRuleId(), 'rule-1', 'the reconciled rule id becomes the stored reference');
  assert.equal(new URL(careerCall('/rules/receipt')[0].options.url).searchParams.get('requestId'), pending.requestId);
});

test('A5: a scope change invalidates an in-flight rule write — nothing is applied or persisted (seam: 空间切换)', async () => {
  await freshLogin({
    'POST /api/v1/career/rules': () => {/* hangs until the test answers */},
  });
  await career.loadCareer();
  const pending = platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 });
  await runtime.auth.logout(); // logout invalidates the scope
  stub.succeed(stub.lastCall('request'), { data: ruleReceipt() });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(platform.pendingRuleWrite(), null, 'a stale response must not persist a recovery intent for the wrong scope');
  assert.equal(platform.readStoredRuleId(), undefined, 'a stale response must not persist a rule reference for the wrong scope');
});

// OCR high-12：原请求未落地（对账 404）且修订已前进时，原样重放必收确定性
// revision_conflict（服务端幂等回放先于 CAS）——此时可断定原写入从未落地，必须清
// intent 解除保存封锁，而不是把用户锁死在“保存永久禁用、唯一清除路径是登出”。
test('A6: a definite revision conflict on the replay proves the write never landed — the dead-end intent is cleared', async () => {
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/rules': call => {
      if (postFails) stub.fail(call, 'request:fail timeout');
      else stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 9 } } });
    },
    'GET /api/v1/career/rules/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(platform.saveRule({ query: '上海 前端开发 实习', intervalMinutes: 1440, status: 'enabled', expectedRevision: 3 }), error => error.code === 'outcome_unknown');
  assert.ok(platform.pendingRuleWrite(), 'the unknown write is kept for recovery');
  postFails = false;
  await assert.rejects(platform.retryPendingRule(), error => error.code === 'revision_conflict_abandoned', 'a definite conflict on replay closes the dead-end recovery with an honest typed code');
  assert.equal(platform.pendingRuleWrite(), null, 'the intent is cleared — the save button is unblocked without logging out');
  // 显式放弃出口（其它确定失败卡死时的最后出口）：只清本端 intent，零网络写。
  postFails = true;
  await assert.rejects(platform.saveRule({ query: '重存规则', intervalMinutes: 60, status: 'paused', expectedRevision: 3 }), error => error.code === 'outcome_unknown');
  assert.ok(platform.pendingRuleWrite());
  const writesBeforeAbandon = ruleWrites().length;
  platform.abandonPendingRuleWrite();
  assert.equal(platform.pendingRuleWrite(), null, 'the explicit abandon escape clears the intent');
  assert.equal(ruleWrites().length, writesBeforeAbandon, 'abandon performs no network write — server facts are untouched');
});

// ---- B 组：额度（执行前预估 + 超额只阻新收费 + 历史完整可访问 + 重复不二扣）----

test('B1: the usage estimate is read from the frozen endpoint and displayed verbatim', async () => {
  await freshLogin({
    'GET /api/v1/career/usage/estimate': call => stub.succeed(call, { data: usageEstimate() }),
  });
  const estimateView = await platform.fetchUsageEstimate();
  assert.equal(estimateView.kind, 'usage_estimate');
  assert.equal(estimateView.operation, 'search_once');
  assert.equal(estimateView.costUnits, 1);
  assert.deepEqual(estimateView.conditions, ['每次找岗（一次性或持续规则的每次触发）消耗 1 个额度单位'], 'conditions are the backend literals — never recomputed');
  assert.equal(estimateView.remainingUnits, 1);
  assert.equal(estimateView.wouldAdmit, true);
  const call = careerCall('/usage/estimate')[0];
  assert.equal(new URL(call.options.url).searchParams.get('operation'), 'search_once');
  assert.equal((call.options.method ?? 'GET'), 'GET', 'the estimate is a free read-only projection');
});

test('B2: an exhausted window blocks only the new charged act — every stored archive, application and timeline stays readable (关键断言)', async () => {
  const application = () => ({
    applicationId: 'app-1', requestId: 'srv-app', linkState: 'ready', taskId: 'task-1', qualified: true,
    pinnedEvidence: { opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', profileRevision: 3, evaluationStatus: 'eligible', batchIdentity: '批次A' },
  });
  const progress = () => ({
    applicationId: 'app-1', revision: 1, stage: 'interview',
    events: [{ eventId: 'evt-1', seq: 1, kind: 'progress_appended', eventType: 'interview', occurredAt: T, source: { kind: 'manual' }, confirmer: 'u1', corrected: false, requestId: 'srv-pr', createdAt: T }],
  });
  await freshLogin({
    'GET /api/v1/career/usage/estimate': call => stub.succeed(call, { data: usageEstimate({ settledUnits: 10, remainingUnits: 0, wouldAdmit: false }) }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { statusCode: 429, data: { error: { code: 'search_quota_refused', message: '搜索额度不足：本期额度已耗尽，新的收费找岗已被阻止' } } }),
    'GET /api/v1/career/applications/app-1': call => stub.succeed(call, { data: application() }),
    'GET /api/v1/career/applications/app-1/progress': call => stub.succeed(call, { data: progress() }),
  });
  const estimateView = await platform.fetchUsageEstimate();
  assert.equal(estimateView.wouldAdmit, false, 'the window is exhausted');
  // 新收费动作被拒（typed 429）：
  const refused = await career.searchOnce('上海 前端开发 实习').catch(error => error);
  assert.equal(errorCode(refused), 'search_quota_refused', 'the charged act is refused with the typed code');
  assert.equal(refused.status, 429);
  assert.equal(career.pendingSearch(), null, 'a definite refusal leaves no recovery intent');
  // 历史（档案/申请/时间线）完整可访问：
  const view = await career.loadCareer();
  assert.equal(view.revision, 3, 'the stored archive stays readable');
  const app = await career.getApplication('app-1');
  assert.equal(app.applicationId, 'app-1', 'the stored application stays readable');
  const timeline = await career.applicationProgress('app-1');
  assert.equal(timeline.events.length, 1, 'the stored timeline stays readable');
});

test('B3: a repeated charged request replays the same request id — presented once, never double charged', async () => {
  let postFails = true; const served = [];
  const searchReceipt = () => ({
    kind: 'search_once', requestId: 'srv-search', searchId: 'search-1', status: 'completed',
    query: '上海 前端开发 实习', coverage: { sources: [] }, scopeNotes: [],
    results: [], checkedAt: T,
  });
  await freshLogin({
    'POST /api/v1/career/searches': call => {
      served.push(call.options.data.requestId);
      if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: searchReceipt() });
    },
    'GET /api/v1/career/searches/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('上海 前端开发 实习'), error => error.code === 'outcome_unknown');
  postFails = false;
  const outcome = await career.retryPendingSearch();
  assert.equal(outcome.status, 'completed');
  assert.equal(served.length, 2);
  assert.equal(served[1], served[0], 'the replay reuses the original request id — the idempotent server charges once, never twice');
  assert.equal(career.pendingSearch(), null);
});

// ---- C 组：订阅消息（wx.requestSubscribeMessage 原生例外）+ 站内待办（T20 合同）----

test('C1: requesting the subscription passes only the template ids — zero job detail in the native payload', async () => {
  const seen = [];
  const outcome = await platform.requestReminderSubscription({
    templateIds: ['TMPL-REMIND-001'],
    invoke: options => { seen.push(options); options.success({ 'TMPL-REMIND-001': 'accept', errMsg: 'requestSubscribeMessage:ok' }); },
  });
  assert.equal(outcome.status, 'accepted');
  assert.equal(seen.length, 1, 'the native API was invoked exactly once');
  // 发往微信的原生载荷只有模板 id（success/fail 是本端回调，不上 wire）：
  const wirePayload = (({ success: _s, fail: _f, ...rest }) => rest)(seen[0]);
  assert.deepEqual(Object.keys(wirePayload).sort(), ['tmplIds'], 'the wire payload carries nothing but the template ids');
  assert.deepEqual(seen[0].tmplIds, ['TMPL-REMIND-001']);
  assert.ok(!JSON.stringify(wirePayload).includes('上海'), 'no job query text ever reaches the native subscribe payload');
  assert.notEqual(outcome.delivered, true, 'the client never claims a delivered push — delivery is the server\'s later fact');
});

test('C7: accepted native authorization persists subscribed preference at the captured profile revision', async () => {
  const stamp = { origin: 'https://api.example.test', userId: 'u1', tenantId: '1', generation: 3 };
  const markers = new Set(); const writes = []; let prompts = 0;
  const result = await rulesPage.runPushSubscribeFlow({ stamp, isCurrent: () => true, hasPendingWrite: () => false, hasInvalidReceipt: () => false,
    marker: s => markers.has(s.userId), setMarker: s => markers.add(s.userId), clearMarker: s => markers.delete(s.userId),
    requestAuthorization: async () => { prompts++; return { status: 'accepted', templateIds: ['TMPL-1'], delivered: false }; }, revision: () => 9,
    persist: async revision => writes.push(revision),
  });
  assert.equal(result.status, 'saved'); assert.equal(prompts, 1); assert.deepEqual(writes, [9]); assert.equal(markers.size, 0);
});

test('C8: rejected or unavailable native authorization returns before any preference write', async () => {
  for (const status of ['rejected', 'unavailable']) {
    let writes = 0; let markers = 0;
    const result = await rulesPage.runPushSubscribeFlow({ stamp: { origin: 'x', userId: 'u', tenantId: 't', generation: 1 },
      isCurrent: () => true, hasPendingWrite: () => false, hasInvalidReceipt: () => false, marker: () => false, setMarker: () => markers++, clearMarker: () => {},
      requestAuthorization: async () => ({ status, templateIds: [], delivered: false }), revision: () => 3, persist: async () => writes++,
    });
    assert.equal(result.status, status); assert.equal(writes, 0); assert.equal(markers, 0);
  }
});

test('M1: account A authorization resolving after switch to B creates no B marker or write', async () => {
  let current = true; let resolvePrompt; const writes = []; const markers = new Set();
  const flow = rulesPage.runPushSubscribeFlow({
    stamp: { origin: 'https://api.example.test', userId: 'u1', tenantId: '1', generation: 1 },
    isCurrent: () => current, hasPendingWrite: () => false, hasInvalidReceipt: () => false, marker: () => false,
    setMarker: stamp => markers.add(`${stamp.userId}:${stamp.tenantId}`), clearMarker: stamp => markers.delete(`${stamp.userId}:${stamp.tenantId}`),
    requestAuthorization: () => new Promise(resolve => { resolvePrompt = resolve; }), revision: () => 3, persist: async revision => writes.push(revision),
  });
  current = false;
  resolvePrompt({ status: 'accepted', templateIds: ['TMPL-1'], delivered: false });
  assert.equal((await flow).status, 'scope_changed');
  assert.deepEqual(writes, []);
  assert.deepEqual([...markers], []);
});

test('M2: opt-out clears accepted-but-unsynced authorization so resubscribe prompts again', async () => {
  const stamp = { origin: 'https://api.example.test', userId: 'u1', tenantId: '1', generation: 2 };
  const markers = new Set(); let prompts = 0; let writes = 0;
  const clear = captured => markers.delete(`${captured.userId}:${captured.tenantId}`);
  const deps = { stamp, isCurrent: () => true, hasPendingWrite: () => false, hasInvalidReceipt: () => false,
    marker: captured => markers.has(`${captured.userId}:${captured.tenantId}`), setMarker: captured => markers.add(`${captured.userId}:${captured.tenantId}`), clearMarker: clear,
    requestAuthorization: async () => { prompts++; return { status: 'accepted', templateIds: ['TMPL-1'], delivered: false }; }, revision: () => undefined, persist: async () => { writes++; },
  };
  assert.equal((await rulesPage.runPushSubscribeFlow(deps)).status, 'missing_revision');
  assert.equal(markers.has('u1:1'), true);
  assert.equal(rulesPage.clearPushAuthorizationAfterOptOut(stamp, () => true, clear), true);
  deps.revision = () => 4;
  const flow = await rulesPage.runPushSubscribeFlow(deps);
  assert.equal(flow.status, 'saved'); assert.equal(prompts, 2); assert.equal(writes, 1); assert.equal(markers.size, 0);
});

test('M3: malformed successful preference receipt is definite and does not persist an unknown intent', async () => {
  await freshLogin({
    'POST /api/v1/career/act': call => stub.succeed(call, { data: { kind: 'confirmed', requestId: call.options.data.requestId } }),
  });
  await assert.rejects(rulesPage.persistPushPreference(3), error => error.code === 'contract_violation');
  assert.equal(stub.state.storage.keys().some(key => key.includes('push-subscription')), false);
  assert.equal(stub.state.storage.keys().some(key => key.includes('push-receipt-invalid')), true);
  let prompts = 0;
  const blocked = await rulesPage.runPushSubscribeFlow({ stamp: runtime.auth.scope.capture(), isCurrent: () => true, hasPendingWrite: () => false,
    hasInvalidReceipt: () => true, marker: () => true, setMarker() {}, clearMarker() {}, requestAuthorization: async () => { prompts++; return { status: 'accepted', templateIds: [], delivered: false }; },
    revision: () => 3, persist: async () => { throw new Error('must not write before checking the current profile fact'); },
  });
  assert.equal(blocked.status, 'invalid_receipt'); assert.equal(prompts, 0);
});

test('T16-1: invalid receipt blocks the sync persistence seam until a verified profile refresh clears it', async () => {
  let writeCount = 0;
  await freshLogin({
    'POST /api/v1/career/act': call => { writeCount++; stub.succeed(call, { data: writeCount === 1 ? { kind: 'confirmed', requestId: call.options.data.requestId } : confirmedAct('notifications.push', 'subscribed', 4) }); },
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 4, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: 4, facts: [], proposals: [] } }),
  });
  await assert.rejects(rulesPage.persistPushPreference(3), error => error.code === 'contract_violation');
  const writes = () => careerCall('/act').filter(call => (call.options.method ?? 'GET') === 'POST');
  assert.equal(writes().length, 1);
  await assert.rejects(rulesPage.persistPushPreference(3), error => error.code === 'invalid_receipt');
  assert.equal(writes().length, 1, 'sync cannot submit a second write while the successful receipt is invalid');
  const refreshed = await career.refreshCareer();
  assert.equal(refreshed.revision, 4, 'the profile refresh supplies a verified current profile');
  rulesPage.clearPushReceiptInvalidAfterProfileRefresh(runtime.auth.scope.capture());
  await rulesPage.persistPushPreference(4);
  assert.equal(writes().length, 2, 'a verified profile refresh reopens the sync path');
});

test('T16-2: opt-out invalidates old authorization before a malformed committed-success receipt', async () => {
  await freshLogin({
    'POST /api/v1/career/act': call => stub.succeed(call, { data: { kind: 'confirmed', requestId: call.options.data.requestId } }),
  });
  const stamp = runtime.auth.scope.capture();
  let marker = true;
  await assert.rejects(rulesPage.runPushOptOut(stamp, captured => runtime.auth.scope.isCurrent(captured), () => { marker = false; }, () => platform.setPushSubscription('unsubscribed', 3)), error => error.code === 'contract_violation');
  assert.equal(marker, false, 'the previous accepted authorization cannot survive an uncertain opt-out response');
  assert.equal(rulesPage.hasInvalidPushReceipt(stamp), true, 'an undecodable successful response requires profile fact recovery');
  let prompts = 0;
  const result = await rulesPage.runPushSubscribeFlow({ stamp, isCurrent: captured => runtime.auth.scope.isCurrent(captured), hasPendingWrite: () => false, hasInvalidReceipt: captured => rulesPage.hasInvalidPushReceipt(captured),
    marker: () => marker, setMarker: () => { marker = true; }, clearMarker: () => { marker = false; }, requestAuthorization: async () => { prompts++; return { status: 'accepted', templateIds: ['TMPL-1'], delivered: false }; },
    revision: () => undefined, persist: async () => {},
  });
  assert.equal(result.status, 'invalid_receipt');
  assert.equal(prompts, 0, 'profile fact recovery must happen before another authorization or write');
  assert.equal(careerCall('/act').filter(call => (call.options.method ?? 'GET') === 'POST').length, 1, 'the malformed success was not retried');
});

test('T19-1: pending subscribed retry is blocked until profile reconciliation clears invalid receipt', async () => {
  let subscribedPosts = 0;
  await freshLogin({
    'POST /api/v1/career/act': call => {
      if (call.options.data.value === 'unsubscribed') stub.succeed(call, { data: { kind: 'confirmed', requestId: call.options.data.requestId } });
      else if (++subscribedPosts === 1) stub.fail(call, 'request:fail timeout');
      else stub.succeed(call, { data: confirmedAct('notifications.push', 'subscribed', 4) });
    },
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 4, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: 4, facts: [], proposals: [] } }),
  });
  await assert.rejects(rulesPage.persistPushPreference(3), error => error.code === 'outcome_unknown');
  const stamp = runtime.auth.scope.capture();
  await assert.rejects(rulesPage.runPushOptOut(stamp, captured => runtime.auth.scope.isCurrent(captured), () => {}, () => platform.setPushSubscription('unsubscribed', 3)), error => error.code === 'contract_violation');
  assert.equal(rulesPage.hasInvalidPushReceipt(stamp), true);
  await assert.rejects(rulesPage.retryPushPreference(), error => error.code === 'invalid_receipt');
  assert.equal(subscribedPosts, 1, 'retry cannot issue a second subscribed POST while profile truth is unknown');
  const refreshed = await career.refreshCareer();
  assert.equal(refreshed.revision, 4);
  rulesPage.clearPushReceiptInvalidAfterProfileRefresh(stamp);
  await rulesPage.retryPushPreference();
  assert.equal(subscribedPosts, 2, 'the original request can be safely retried after profile reconciliation');
});

test('C2: a rejected subscription keeps the in-station todos readable — no fake delivery', async () => {
  await freshLogin({
    'GET /api/v1/career/reminders': call => stub.succeed(call, { data: { reminders: [reminderView()] } }),
  });
  const outcome = await platform.requestReminderSubscription({
    templateIds: ['TMPL-REMIND-001'],
    invoke: options => options.success({ 'TMPL-REMIND-001': 'reject', errMsg: 'requestSubscribeMessage:ok' }),
  });
  assert.equal(outcome.status, 'rejected');
  assert.notEqual(outcome.delivered, true, 'a rejection must never be presented as delivered');
  const todos = await platform.fetchReminders();
  assert.equal(todos.reminders.length, 1, 'the in-station inbox stays the readable surface after a rejection');
});

test('C3: an unavailable native API (simulator or unconfigured templates) reports the honest limitation', async () => {
  const previousWx = globalThis.wx;
  globalThis.wx = { env: previousWx?.env }; // no requestSubscribeMessage — a simulator without the capability
  try {
    const outcome = await platform.requestReminderSubscription({ templateIds: ['TMPL-REMIND-001'] });
    assert.equal(outcome.status, 'unavailable');
    assert.equal(outcome.reason, 'api_unavailable', 'the reason is stated — not papered over as success');
    assert.notEqual(outcome.delivered, true, 'an unavailable capability must never be presented as delivered');
  } finally { globalThis.wx = previousWx; }
  const noTemplates = await platform.requestReminderSubscription({ invoke: options => options.success({ any: 'accept' }) });
  assert.equal(noTemplates.status, 'unavailable');
  assert.equal(noTemplates.reason, 'no_templates', 'honest refusal when no template id is configured for this build');
});

test('C4: the inbox renders the frozen privacy notices verbatim — no company, job or interview detail', async () => {
  await freshLogin({
    'GET /api/v1/career/reminders': call => stub.succeed(call, { data: { reminders: [reminderView(), reminderView({ reminderId: 'rem-2', sourceKind: 'discovery', sourceId: 'run-1', applicationId: undefined, opportunityId: undefined, noticeKey: 'discovery_found', notice: '持续找岗有新发现，请登录查看。' })] } }),
  });
  const todos = await platform.fetchReminders();
  const bodies = todos.reminders.map(item => item.notice).sort();
  assert.deepEqual(bodies, ['你有新的求职进展，请登录查看。', '持续找岗有新发现，请登录查看。'], 'the notice bodies are exactly the frozen backend template literals');
  for (const word of ['公司', '岗位', '面试', '前端']) {
    assert.ok(!bodies.some(body => body.includes(word)), `the push body must not carry "${word}" detail`);
  }
});

test('C5: registering a todo posts the frozen T20 contract and surfaces the deduplicated receipt with the response-only push report', async () => {
  await freshLogin({
    'POST /api/v1/career/reminders': call => stub.succeed(call, { data: reminderReceipt({ deduplicated: true, push: { attempted: false, delivered: false, reason: 'unsubscribed' } }) }),
  });
  await career.loadCareer();
  const receipt = await platform.createReminder({ sourceKind: 'progress_event', sourceId: 'evt-1', expectedRevision: 3 });
  assert.equal(receipt.kind, 'reminder_set');
  assert.equal(receipt.deduplicated, true, 'one source event holds exactly one todo — no second row');
  assert.deepEqual(receipt.push, { attempted: false, delivered: false, reason: 'unsubscribed' }, 'the push report is response-only: unsubscribed never attempts, never delivers');
  const call = careerCall('/reminders').find(c => (c.options.method ?? 'GET') === 'POST');
  assert.deepEqual(Object.keys(call.options.data).sort(), ['expectedRevision', 'requestId', 'sourceId', 'sourceKind'], 'the body must match the frozen SetReminderInput');
  assert.equal(call.options.data.expectedRevision, 3);
});

test('C6: opting out of push stops the pushes while the todos stay readable', async () => {
  await freshLogin({
    'POST /api/v1/career/act': call => stub.succeed(call, { data: confirmedAct('notifications.push', 'unsubscribed', 4) }),
    'GET /api/v1/career/reminders': call => stub.succeed(call, { data: { reminders: [reminderView()] } }),
  });
  await career.loadCareer();
  const receipt = await platform.setPushSubscription('unsubscribed', 3);
  assert.equal(receipt.kind, 'confirmed');
  assert.equal(receipt.fact.key, 'notifications.push');
  assert.equal(receipt.fact.value, 'unsubscribed');
  const actCall = careerCall('/act').find(c => (c.options.method ?? 'GET') === 'POST');
  assert.equal(actCall.options.data.action, 'confirm');
  assert.equal(actCall.options.data.key, 'notifications.push');
  assert.equal(actCall.options.data.value, 'unsubscribed');
  assert.equal(actCall.options.data.expectedRevision, 3);
  const todos = await platform.fetchReminders();
  assert.equal(todos.reminders.length, 1, 'unsubscribed stops the pushes — the in-station todos remain readable');
});
