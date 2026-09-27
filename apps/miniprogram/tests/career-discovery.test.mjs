import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// T24 小程序 Career 找岗/建档/分享导入的可观察行为：真实 transport + AuthCoordinator +
// 共享 CareerDesk（career-core）装配，假后端按 method+pathname 路由。career 端点是裸 JSON
// （handler.go 直接 c.JSON(200, receipt)，不带 success envelope），错误统一是
// {error:{code,message[,currentRevision|requestId]}}。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtime = await import('../src/services/runtime.ts');
const career = await import('../src/services/career.ts');
const { errorMessage } = await import('../src/core/errors.ts');

const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const fact = (key, value, revision = 3) => ({ key, value, revision, source: { kind: 'resume' }, confirmation: { userId: 'u1', confirmedAt: '2026-09-25T00:00:00Z' }, confirmedAt: '2026-09-25T00:00:00Z' });
const proposal = (id, key, value, revision = 2) => ({ id, key, value, source: { kind: 'resume' }, status: 'pending', createdAt: '2026-09-25T00:00:00Z', revision });
const confirmedReceipt = (key, value, revision) => ({ kind: 'confirmed', requestId: 'srv-r', revision, fact: fact(key, value, revision) });
const searchReceipt = (over = {}) => ({
  kind: 'search_once', requestId: 'srv-search', searchId: 's-1', status: 'completed', query: 'Go 工程师',
  coverage: { sources: [{ sourceId: 'src-1', label: '示例招聘站', accessMethods: ['http'], cities: ['上海'], available: true }] },
  scopeNotes: [], results: [
    { resultId: 'r-1', sourceId: 'src-1', link: 'https://jobs.example.test/1', checkedAt: '2026-09-25T01:02:03Z', qualification: 'needs_review', uncertainty: 'low_confidence' },
    { resultId: 'r-2', sourceId: 'src-1', link: 'https://jobs.example.test/2', checkedAt: '2026-09-25T01:02:04Z', qualification: 'qualified', uncertainty: '' },
  ], checkedAt: '2026-09-25T01:02:05Z', ...over,
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
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
    ...extraRoutes,
  });
  await runtime.auth.login('u@example.test', 'pw');
  career.resetCareerDesk();
}
const careerCall = suffix => stub.state.calls.filter(c => new URL(c.options.url).pathname.startsWith('/api/v1/career') && (!suffix || new URL(c.options.url).pathname.includes(suffix)));
const errorCode = error => error?.code;
const ambiguous = error => ['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED'].includes(errorCode(error)) || errorCode(error) === 'outcome_unknown' || (error?.status === undefined || error.status >= 500);

test('A1: opening the desk reads the same server-side profile and never mints a second one', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [fact('城市', '上海')], proposals: [proposal('p1', '学历', '本科')] } }),
  });
  const view = await career.loadCareer();
  assert.ok(view, 'view must load');
  assert.equal(view.revision, 3);
  assert.equal(view.facts[0].value, '上海');
  assert.equal(career.needsOnboarding(view), false);
  const open = careerCall('/career/open')[0];
  assert.ok(open, 'open request recorded');
  assert.equal(open.options.header.Authorization, 'Bearer t1', 'identity flows through the authenticated client');
  const paths = stub.paths();
  assert.ok(!paths.some(p => p.includes('/auth/register')), 'the miniprogram never registers a new identity');
  assert.ok(!paths.some(p => /POST .*\/career/.test(p)), 'loading performs no career writes — linkage only');
});

test('A2: an empty profile reports onboarding instead of creating a parallel one', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
  });
  const view = await career.loadCareer();
  assert.equal(career.needsOnboarding(view), true);
  assert.ok(!stub.paths().some(p => /POST .*\/career/.test(p)), 'no parallel profile is created client-side');
});

test('B1: extracted facts join the profile only after an item-by-item confirm_proposal', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [proposal('p1', '学历', '本科')] } }),
    'POST /api/v1/career/act': call => stub.succeed(call, { data: confirmedReceipt('学历', '本科', 4) }),
  });
  await career.loadCareer();
  const receipt = await career.confirmProposal('p1');
  assert.equal(receipt.kind, 'confirmed');
  const body = careerCall('/career/act')[0].options.data;
  assert.equal(body.action, 'confirm_proposal');
  assert.equal(body.proposalId, 'p1');
  assert.equal(body.expectedRevision, 3);
  assert.ok(typeof body.requestId === 'string' && body.requestId.length > 0);
  assert.equal(body.source.kind, 'user', 'source.kind must stay inside the frozen server whitelist (no client parser provenance)');
  const view = career.careerDesk().snapshot;
  assert.ok(view.facts.some(f => f.key === '学历' && f.value === '本科'), 'confirmed fact is visible');
  assert.equal(view.revision, 4);
});

test('B2: unconfirmed proposals stay proposals — nothing silently becomes a fact', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [proposal('p1', '学历', '本科')] } }),
  });
  const view = await career.loadCareer();
  assert.equal(view.facts.length, 0);
  assert.equal(view.proposals.length, 1);
});

test('C1: a revision conflict surfaces the server receipt without corrupting the desk', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [fact('城市', '上海')], proposals: [] } }),
    'POST /api/v1/career/act': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 9 } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.confirmProposal('p1'), error => error.code === 'revision_conflict' && error.currentRevision === 9);
  const desk = career.careerDesk();
  assert.equal(desk.pendingAction, undefined, 'a definitive conflict leaves no pending intent');
  assert.equal(desk.snapshot.revision, 3, 'local view is untouched');
});

test('C2: an ambiguous act outcome is reconciled through the original request id', async () => {
  let actFails = true;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [proposal('p1', '城市', '杭州')] } }),
    'POST /api/v1/career/act': call => { if (actFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: confirmedReceipt('城市', '杭州', 4) }); },
    'GET /api/v1/career/receipt': call => { if (actFails) stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }); else stub.succeed(call, { data: confirmedReceipt('城市', '杭州', 4) }); },
  });
  await career.loadCareer();
  await assert.rejects(career.confirmProposal('p1'), error => error.code === 'outcome_unknown');
  const pending = career.careerDesk().pendingAction;
  assert.ok(pending, 'the unresolved action is kept for recovery');
  const sentRequestId = careerCall('/career/act')[0].options.data.requestId;
  assert.equal(pending.requestId, sentRequestId);
  actFails = false;
  const receipt = await career.reconcilePending();
  assert.equal(receipt.kind, 'confirmed');
  assert.equal(career.careerDesk().pendingAction, undefined, 'reconciliation clears the pending action');
  assert.ok(career.careerDesk().snapshot.facts.some(f => f.key === '城市'), 'recovered fact applied');
  assert.ok(new URL(careerCall('/career/receipt')[0].options.url).searchParams.get('requestId').length > 0);
});

test('C3: a scope change invalidates in-flight career reads — stale responses never land', async () => {
  await freshLogin({
    'GET /api/v1/career/open': () => {/* hangs until the test answers */},
  });
  const pending = career.loadCareer();
  await runtime.auth.clear(); // logout invalidates the scope
  stub.succeed(stub.lastCall('request'), { data: { revision: 3, facts: [fact('城市', '上海')], proposals: [] } });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(career.careerDesk().snapshot, undefined, 'the stale response must not be applied');
});

test('D1: one-shot search posts the frozen searches contract and returns truthful rows', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { data: searchReceipt() }),
  });
  await career.loadCareer();
  const outcome = await career.searchOnce('Go 工程师');
  assert.equal(outcome.status, 'completed');
  assert.equal(outcome.searchId, 's-1');
  const body = careerCall('/searches')[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['expectedRevision', 'query', 'requestId'], 'the body must match the frozen contract (DisallowUnknownFields)');
  assert.equal(body.query, 'Go 工程师');
  assert.equal(body.expectedRevision, 3);
  assert.equal(outcome.rows.length, 2);
  const row = outcome.rows[0];
  assert.equal(row.resultId, 'r-1');
  assert.equal(row.link, 'https://jobs.example.test/1');
  assert.equal(row.checkedAt, '2026-09-25T01:02:03Z');
  assert.equal(row.qualification, 'needs_review');
  assert.equal(row.uncertainty, 'low_confidence');
  assert.equal(outcome.sources[0].label, '示例招聘站');
  assert.equal(outcome.hasNoVettedSources, false);
});

test('D2: a search with no vetted sources reports honestly instead of inventing rows', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { data: searchReceipt({ status: 'completed', coverage: { sources: [] }, scopeNotes: ['当前没有已核验来源'], results: [] }) }),
  });
  await career.loadCareer();
  const outcome = await career.searchOnce('任意岗位');
  assert.equal(outcome.rows.length, 0);
  assert.equal(outcome.sources.length, 0);
  assert.equal(outcome.hasNoVettedSources, true);
  assert.deepEqual(outcome.scopeNotes, ['当前没有已核验来源']);
});

test('D3: a failed search keeps its failure code; quota refusal stays actionable', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { data: searchReceipt({ status: 'failed', failureCode: 'no_vetted_sources', results: [] }) }),
  });
  await career.loadCareer();
  const failed = await career.searchOnce('任意岗位');
  assert.equal(failed.status, 'failed');
  assert.equal(failed.failureCode, 'no_vetted_sources');
});

test('D4: quota refusal surfaces as search_quota_refused without touching the profile', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { statusCode: 429, data: { error: { code: 'search_quota_refused', message: 'quota' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('任意岗位'), error => error.code === 'search_quota_refused');
  assert.equal(career.pendingSearch(), null, 'a definitive refusal leaves no pending intent');
});

test('D5: an unknown search outcome is recovered through the receipt endpoint with the original request id', async () => {
  let posts = 0;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => { posts++; stub.fail(call, 'request:fail network'); },
    'GET /api/v1/career/searches/receipt': call => stub.succeed(call, { data: searchReceipt() }),
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('Go 工程师'), error => error.code === 'outcome_unknown');
  const pending = career.pendingSearch();
  assert.ok(pending, 'pending search intent persisted for recovery');
  assert.equal(pending.query, 'Go 工程师');
  const storedKeys = [...stub.state.storage.keys()].filter(k => k.startsWith('wk:career:'));
  assert.ok(storedKeys.length > 0, 'intent lives in the controlled wk:career: store');
  const recovered = await career.searchReceipt();
  assert.equal(recovered.status, 'completed');
  assert.equal(career.pendingSearch(), null, 'recovery clears the intent');
  const receiptCall = careerCall('/searches/receipt')[0];
  assert.equal(new URL(receiptCall.options.url).searchParams.get('requestId'), pending.requestId, 'recovery replays the original request id');
});

test('D4b: a quota refusal renders the dedicated actionable prompt through the real error chain', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => stub.succeed(call, { statusCode: 429, data: { error: { code: 'search_quota_refused', message: 'quota' } } }),
  });
  await career.loadCareer();
  const refused = await career.searchOnce('任意岗位').catch(error => error);
  assert.equal(refused.code, 'search_quota_refused', 'the typed refusal reaches the app');
  // 页面渲染链：useAction 捕获后经 errorMessage() 成串（searchBusy.error），Notice 呈现。
  // 专属提示必须经这条真实链路可达，而不是死代码。
  const shown = errorMessage(refused);
  assert.ok(shown.includes('搜索额度不足'), 'the dedicated quota prompt is what the UI renders');
  assert.ok(shown.includes('仍可查看既有档案与申请记录'), 'the prompt keeps the quota-exhausted readability guarantee');
});

test('D6: a missing receipt (404) during reconciliation exposes an actionable recovery state', async () => {
  let postFails = true;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => { if (postFails) stub.fail(call, 'request:fail network'); else stub.succeed(call, { data: searchReceipt() }); },
    'GET /api/v1/career/searches/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('Go 工程师'), error => error.code === 'outcome_unknown');
  const pending = career.pendingSearch();
  assert.ok(pending, 'intent persisted for reconciliation');
  // 对账返回 404：UI 必须能把“回执缺失”从其他失败中区分出来，呈现可操作恢复态。
  const missing = await career.searchReceipt().catch(error => error);
  assert.ok(career.isReceiptMissing(missing), 'receipt-missing is distinguishable for the recovery UI');
  assert.equal(career.pendingSearch()?.requestId, pending.requestId, 'a missing receipt keeps the intent — nothing is silently dropped');
  // 恢复动作：同 requestId 安全重发（服务端幂等不重复执行），成功后清除 intent。
  postFails = false;
  const recovered = await career.retryPendingSearch();
  assert.equal(recovered.status, 'completed');
  assert.equal(career.pendingSearch(), null, 'a successful safe resend clears the intent');
  const bodies = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/searches' && (c.options.method ?? 'GET') === 'POST').map(call => call.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[0].requestId, bodies[1].requestId, 'the safe resend replays the original request id');
});

test('D7: a quota-refused safe resend keeps the typed code, the dedicated prompt, and the intent', async () => {
  let postFails = true;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => { if (postFails) stub.fail(call, 'request:fail network'); else stub.succeed(call, { statusCode: 429, data: { error: { code: 'search_quota_refused', message: 'quota' } } }); },
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('Go 工程师'), error => error.code === 'outcome_unknown');
  const pending = career.pendingSearch();
  postFails = false;
  const refused = await career.retryPendingSearch().catch(error => error);
  assert.equal(refused.code, 'search_quota_refused', 'the resend path surfaces the typed refusal');
  assert.ok(errorMessage(refused).includes('搜索额度不足'), 'the dedicated prompt also covers the resend path');
  assert.equal(career.pendingSearch()?.requestId, pending.requestId, 'a refused resend keeps the intent for later recovery');
});

// OCR high-7：作用域在途切换时，旧作用域的搜索失败绝不把恢复意图铸到新作用域键下
// （存储未清也必须零 intent——断言看的是全部 wk:career:search:* 原始键，不是当前作用域读取）。
test('D8: a scope change mid-search mints no recovery intent under any scope (seam: 空间切换)', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': () => {/* hangs until the test answers */},
  });
  await career.loadCareer();
  const pending = career.searchOnce('Go 工程师');
  await runtime.auth.clear(); // 登出使作用域失效（此处刻意不清 wk:career:* 存储）
  stub.succeed(stub.lastCall('request'), { data: searchReceipt() });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(career.pendingSearch(), null, 'the new scope reads no intent');
  assert.ok(
    ![...stub.state.storage.keys()].some(key => key.startsWith('wk:career:search:')),
    'no search intent is minted under ANY scope key — the stamp guard precedes the intent write',
  );
});

// OCR high-8：安全重发必须逐字节重放发送前修订——intent 持久化 expectedRevision，
// 档案修订前进后重放仍是原值（用当前值重发必然 idempotency_conflict）。
test('D9: the safe resend replays the persisted original revision byte-for-byte', async () => {
  let postFails = true; let serverRevision = 0;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/searches': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: searchReceipt() }); },
    'GET /api/v1/career/searches/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.searchOnce('Go 工程师'), error => error.code === 'outcome_unknown');
  const pending = career.pendingSearch();
  assert.equal(pending.expectedRevision, 0, 'the intent persists the revision the original send carried');
  serverRevision = 9; // 期间档案修订前进
  await career.refreshCareer();
  assert.equal(career.careerDesk().snapshot.revision, 9);
  postFails = false;
  const recovered = await career.retryPendingSearch();
  assert.equal(recovered.status, 'completed');
  const bodies = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/searches' && (c.options.method ?? 'GET') === 'POST').map(call => call.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the resend replays the original request id');
  assert.equal(bodies[1].expectedRevision, 0, 'the resend replays the ORIGINAL revision — the moved-on head (9) would hit idempotency_conflict');
  assert.equal(career.pendingSearch(), null);
});

test('E1: share import previews verbatim before any network write, then submits the same payload', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/opportunities/import': call => stub.succeed(call, { data: { kind: 'opportunity_imported', requestId: 'srv-imp', opportunityId: 'opp-1', observationId: 'obs-1', snapshotId: 'snap-1', status: 'needs_review', acquiredAt: '2026-09-25T02:00:00Z' } }),
  });
  await career.loadCareer();
  const jd = 'Go 工程师，要求三年经验，base 上海。';
  const draft = career.prepareSharedImport(jd, '微信分享');
  const before = stub.state.calls.length;
  const receipt = await career.confirmSharedImport(draft);
  assert.ok(stub.state.calls.length > before, 'submission performs the network write');
  assert.equal(receipt.opportunityId, 'opp-1');
  assert.equal(receipt.status, 'needs_review');
  const body = careerCall('/opportunities/import')[0].options.data;
  assert.equal(body.rawText, jd, 'the submitted payload is the reviewed text, not the excerpt');
  assert.equal(body.sourceLabel, '微信分享');
  assert.equal(body.requestId, draft.requestId);
});

test('E2: a missing share payload exposes recovery instead of demo data', async () => {
  assert.throws(() => career.prepareSharedImport('  '), error => error.code === 'share_payload_missing' && error.recoverable === true);
});

test('F1: uploading a resume posts multipart with request id and expected revision', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } }),
    'POST /api/v1/career/sources/upload': call => stub.succeed(call, { data: {
      source: { id: 'src-9', revision: 1, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 2048, digest: 'a'.repeat(64), status: 'ready', createdAt: '2026-09-25T03:00:00Z' },
      receipt: { kind: 'intake_completed', requestId: 'srv-up', revision: 4, proposals: [proposal('p1', '学历', '本科')] },
    } }),
  });
  await career.loadCareer();
  const upload = await career.uploadResume({ uri: 'wxfile://tmp-resume.pdf', name: 'resume.pdf', type: 'application/pdf', size: 2048 });
  assert.equal(upload.source.status, 'ready');
  assert.equal(upload.receipt.proposals[0].key, '学历');
  const call = stub.state.calls.find(c => c.kind === 'uploadFile');
  assert.ok(call, 'native uploadFile used');
  assert.equal(new URL(call.options.url).pathname, '/api/v1/career/sources/upload');
  assert.equal(call.options.name, 'file');
  assert.equal(call.options.formData.requestId.length > 0, true);
  assert.equal(call.options.formData.expectedRevision, '3');
});

test('F2: a failed source keeps its error category visible to the user', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 0, facts: [], proposals: [] } }),
    'POST /api/v1/career/sources/upload': call => stub.succeed(call, { data: {
      source: { id: 'src-x', revision: 1, fileName: 'resume.doc', mimeType: 'application/msword', size: 10, digest: 'b'.repeat(64), status: 'failed', errorCategory: 'unsupported_content', errorMessage: '无法解析', createdAt: '2026-09-25T03:00:00Z' },
    } }),
  });
  await career.loadCareer();
  const upload = await career.uploadResume({ uri: 'wxfile://tmp-resume.doc', name: 'resume.doc', type: 'application/msword', size: 10 });
  assert.equal(upload.source.status, 'failed');
  assert.equal(upload.source.errorCategory, 'unsupported_content');
  assert.equal(upload.receipt, undefined, 'no fabricated intake for a failed source');
});

test('G1: web-side profile changes sync into the miniprogram view', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [fact('城市', '上海')], proposals: [] } }),
    'GET /api/v1/career/changes': call => stub.succeed(call, { data: { revision: 5, changes: [{ revision: 5, kind: 'confirmed', fact: fact('城市', '杭州', 5) }] } }),
  });
  await career.loadCareer();
  const changeSet = await career.syncFromWeb();
  assert.equal(changeSet.revision, 5);
  const view = career.careerDesk().snapshot;
  assert.ok(view.facts.some(f => f.key === '城市' && f.value === '杭州'), 'the web edit is visible on weapp');
  assert.equal(view.revision, 5);
  assert.equal(new URL(careerCall('/career/changes')[0].options.url).searchParams.get('since'), '3');
});

test('H1: a forbidden career scope clears local desk state for recovery', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { statusCode: 403, data: { error: { code: 'forbidden', message: 'career workspace not allowed' } } }),
  });
  await assert.rejects(career.loadCareer(), error => error.code === 'forbidden');
  assert.equal(career.careerDesk().snapshot, undefined, 'private desk state is invalidated');
});

test('OCR2-037: a bare AUTH_REQUIRED on the first search never persists a recovery intent', async () => {
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } }),
  });
  await career.loadCareer();
  await runtime.auth.logout();
  const failed = await career.searchOnce('Go 工程师').catch(error => error);
  assert.equal(failed.message, 'SCOPE_CHANGED');
  assert.match(`${failed.cause?.message ?? ''}`, /AUTH_REQUIRED/);
  assert.equal(career.pendingSearch(), null, 'AUTH_REQUIRED is a definite local failure — no search intent may be persisted');
});
