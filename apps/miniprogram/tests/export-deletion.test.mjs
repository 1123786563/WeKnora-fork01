import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';
import { readFile } from 'node:fs/promises';

// T32 小程序 全空间导出与完整删除的可观察行为：真实 transport + AuthCoordinator 装配
// （T24/T26 同款），假后端按 method+pathname 路由。五路由与 Web ExportDeletionPage
// 同源同版本（同一批 api-client 解码器，packages/api-client/src/career.ts）：导出内联
// 归档+sha256 摘要、删除先呈现边界清单、部分失败保留可恢复状态且绝不称完全删除、
// 删除后旧导出授权 404、本地 storage 缓存清理。
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
const platform = await import('../src/adapters/career-platform.ts');

const T = '2026-09-26T08:00:00Z';
const encoder = new TextEncoder();
const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const open3 = call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } });

// 与服务端冻结结构一一对应的导出归档 fixture（career_export.go CareerExportArchive）：
// 六段包含性——profile 事实、事实历史、岗位与原始快照、申请与进展事件、材料与版本、投递记录。
const exportedFact = { key: 'education.graduation_year', value: '2026', revision: 2, source: { kind: 'user', label: 'live' }, confirmation: { userId: 'u1', confirmedAt: T }, confirmedAt: T };
const archive = {
  profile: { revision: 3, facts: [exportedFact], proposals: [] },
  factHistory: [exportedFact],
  opportunities: [{ opportunityId: 'opp-1', snapshots: [{ snapshotId: 'snap-1', status: 'stored', rawText: '仅限2027届', acquiredAt: T }] }],
  applications: [{ applicationId: 'app-1', opportunityId: 'opp-1', snapshotId: 'snap-1', batchIdentity: '2026秋招A批', progressEvents: [{ eventId: 'evt-1', applicationId: 'app-1', seq: 1, eventType: 'applied', occurredAt: T, source: { kind: 'manual' }, confirmer: 'u1' }] }],
  materials: [{ materialId: 'mat-1', opportunityId: 'opp-1', status: 'confirmed', versions: [{ version: 1, versionBody: '{"sections":[]}', createdAt: T }, { version: 2, versionBody: '{"sections":[]}', createdAt: T }] }],
  submissions: [{ submissionId: 'sub-1', applicationId: 'app-1', channel: 'web', occurredAt: T, versionConfirmed: true, materialId: 'mat-1', exportId: 'exp-1', version: 2, contentDigest: 'c'.repeat(64), confirmer: 'u1', createdAt: T }],
};
const exportReceipt = (over = {}) => ({
  kind: 'career_exported', requestId: 'srv-space-exp', exportId: 'exp-space-1', revision: 3, status: 'complete',
  digest: 'a'.repeat(64), archive, createdAt: T, ...over,
});
const doneSteps = [
  { name: 'revoke_material_exports', status: 'done' },
  { name: 'purge_career_data', status: 'done' },
  { name: 'remove_workbench_tasks', status: 'done' },
  { name: 'finalize', status: 'done' },
];
const partialSteps = [
  { name: 'revoke_material_exports', status: 'done' },
  { name: 'purge_career_data', status: 'failed', detail: 'career_submissions: row locked' },
  { name: 'remove_workbench_tasks', status: 'pending' },
  { name: 'finalize', status: 'pending' },
];
const retention = [
  { holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'retained' },
  { holder: 'career_changes', reason: '仅保留 deletion 事件以驱动客户端缓存失效', status: 'retained' },
];
const deletedReceipt = (over = {}) => ({
  kind: 'career_deleted', requestId: 'srv-del', status: 'deleted', steps: doneSteps, retention,
  revision: 4, startedAt: T, completedAt: T, ...over,
});
const partialReceipt = (over = {}) => ({
  kind: 'career_deleted', requestId: 'srv-del', status: 'partial', steps: partialSteps, retention,
  revision: 3, startedAt: T, ...over,
});
const boundaryView = {
  inSpace: [
    { section: 'profile', description: '已确认的档案事实与待处理提案', count: 1 },
    { section: 'material_exports', description: '材料导出与下载授权', count: 2 },
  ],
  external: [
    { item: 'external_platform_submissions', description: '你在外部招聘平台完成的投递、沟通与账号操作不在本空间控制范围内。', revocable: false },
    { item: 'external_email_copies', description: '已通过邮件或其他渠道发往外部的简历与材料副本无法由本系统收回。', revocable: false },
  ],
  retention,
};

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
    'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: { code: 0, msg: 'success', data: { protocol_minimum: 1, protocol_maximum: 5 } } }),
    'GET /api/v1/career/open': open3,
    ...extraRoutes,
  });
  await runtime.auth.login('u@example.test', 'pw');
  career.resetCareerDesk();
}
const careerCall = suffix => stub.state.calls.filter(c => new URL(c.options.url).pathname.startsWith('/api/v1/career') && (!suffix || new URL(c.options.url).pathname.includes(suffix)));
const errorCode = error => error?.code;
const notFound = call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'career export receipt not found' } } });

test('A1: whole-space export posts the frozen two-field contract and returns the inline archive the web sees', async () => {
  await freshLogin({
    'POST /api/v1/career/exports': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  const receipt = await career.exportWholeSpace();
  assert.equal(receipt.kind, 'career_exported');
  assert.equal(receipt.exportId, 'exp-space-1');
  assert.equal(receipt.status, 'complete');
  assert.match(receipt.digest, /^[a-f0-9]{64}$/);
  const call = careerCall('/exports').find(c => (c.options.method ?? 'GET') === 'POST');
  assert.deepEqual(Object.keys(call.options.data).sort(), ['expectedRevision', 'requestId'], 'body must match the frozen CareerExportInput (DisallowUnknownFields)');
  assert.equal(call.options.data.expectedRevision, 3, 'the pinned desk revision travels in the intent');
  assert.equal(call.options.header.Authorization ?? call.options.header.authorization, 'Bearer t1', 'identity flows through the authenticated client');
});

test('A2: the export archive carries all six segments the web inventory renders', async () => {
  await freshLogin({
    'POST /api/v1/career/exports': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  const { archive: view } = await career.exportWholeSpace();
  assert.equal(view.profile.facts.length, 1, 'confirmed profile facts');
  assert.equal(view.profile.proposals.length, 0, 'pending proposals');
  assert.equal(view.factHistory.length, 1, 'fact history');
  assert.equal(view.opportunities.length, 1);
  assert.equal(view.opportunities[0].snapshots.length, 1, 'original job snapshots ride along');
  assert.equal(view.applications[0].progressEvents.length, 1, 'application progress events');
  assert.equal(view.materials[0].versions.length, 2, 'immutable material versions');
  assert.equal(view.submissions.length, 1, 'submission records');
  assert.equal(view.submissions[0].boundVersion === undefined || view.submissions[0].versionConfirmed === true, true);
});

test('A3: saving the export package writes the complete JSON to a local file — no silent truncation', async () => {
  await freshLogin({
    'POST /api/v1/career/exports': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  const receipt = await career.exportWholeSpace();
  const payload = platform.spaceExportPayload(receipt);
  assert.deepEqual(JSON.parse(payload), JSON.parse(JSON.stringify(receipt)), 'the payload round-trips the whole receipt');
  const record = await platform.saveSpaceExportPackage(payload, receipt.exportId);
  assert.ok(record.filePath.startsWith('wxfile://usr/'), `saved inside USER_DATA_PATH (got ${record.filePath})`);
  assert.ok(record.filePath.includes('career-export'), 'the file name identifies the package');
  const saved = stub.state.fileContents.get(record.filePath);
  assert.equal(saved, payload, 'the saved bytes are the complete payload');
  assert.equal(record.bytes, encoder.encode(payload).length, 'the recorded size equals the full byte length');
  assert.equal(record.digest, platform.sha256Hex(encoder.encode(payload)), 'the local copy digest is verifiable from the exact saved bytes');
  assert.ok(record.savedAt);
});

test('A4: the clipboard equivalent flow carries the full package when local saving is not enough', async () => {
  stub.reset();
  const receipt = exportReceipt();
  const payload = platform.spaceExportPayload(receipt);
  await platform.copySpaceExportToClipboard(payload);
  assert.equal(stub.state.clipboard.at(-1), payload, 'the full untruncated package reaches the clipboard');
});

test('B1: the deletion boundary view decodes in-space sections, external non-revocable items and retention', async () => {
  await freshLogin({
    'GET /api/v1/career/deletions/boundary': call => stub.succeed(call, { data: boundaryView }),
  });
  await career.loadCareer();
  const view = await career.deletionBoundary();
  assert.deepEqual(view.inSpace.map(s => s.section), ['profile', 'material_exports']);
  assert.equal(view.inSpace[0].count, 1);
  assert.ok(view.external.every(item => item.revocable === false), 'external platform data is disclosed as non-revocable');
  assert.equal(view.retention.length, 2);
  assert.equal(view.retention[0].status, 'retained');
});

test('B2: deletion posts the frozen contract and only reports deleted after every step done', async () => {
  await freshLogin({
    'POST /api/v1/career/deletions': call => stub.succeed(call, { data: deletedReceipt() }),
  });
  await career.loadCareer();
  const receipt = await career.deleteWholeSpace();
  assert.equal(receipt.status, 'deleted');
  assert.ok(receipt.completedAt, 'a complete deletion carries the completion time');
  assert.ok(receipt.steps.every(step => step.status === 'done'), 'every step reports done');
  assert.equal(receipt.retention.length, 2, 'the retention disclosure travels with the receipt');
  const call = careerCall('/deletions').find(c => (c.options.method ?? 'GET') === 'POST' && new URL(c.options.url).pathname === '/api/v1/career/deletions');
  assert.deepEqual(Object.keys(call.options.data).sort(), ['expectedRevision', 'requestId'], 'body must match the frozen CareerDeletionInput');
  assert.equal(call.options.data.expectedRevision, 3);
});

test('B3: a partial deletion never claims complete deletion and recovers under the same request id', async () => {
  let posts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => { posts++; if (posts === 1) stub.succeed(call, { data: partialReceipt() }); else stub.succeed(call, { data: deletedReceipt() }); },
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  assert.ok(partial.steps.some(step => step.status === 'failed'), 'the failed step stays visible');
  assert.equal(partial.completedAt, undefined, 'a partial deletion never claims a completion time');
  // 部分失败可恢复：同一 request id 重试（服务端幂等续跑），不是新删除。
  const intent = career.pendingSpaceDeletion();
  assert.ok(intent, 'the deletion intent is kept recoverable');
  const done = await career.retryPendingSpaceDeletion();
  assert.equal(done.status, 'deleted');
  const bodies = careerCall('/deletions').filter(c => (c.options.method ?? 'GET') === 'POST' && new URL(c.options.url).pathname === '/api/v1/career/deletions').map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one partial attempt plus one recovery replay');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the recovery replays the original request id');
  assert.equal(career.pendingSpaceDeletion(), null, 'a complete deletion clears the intent');
});

test('N1: starting another whole-space deletion is refused while a partial request still owns the recovery intent', async () => {
  let posts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => {
      posts++;
      if (posts === 1) stub.succeed(call, { data: partialReceipt() });
      else stub.succeed(call, { data: deletedReceipt() });
    },
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  const original = career.pendingSpaceDeletion();
  assert.ok(original);

  const refused = await career.deleteWholeSpace().catch(error => error);
  assert.equal(errorCode(refused), 'unresolved_action');
  assert.equal(posts, 1, 'no new request is sent while the original partial deletion is unresolved');
  assert.equal(career.pendingSpaceDeletion()?.requestId, original.requestId, 'the only recovery request id is preserved');

  assert.equal(career.abandonPendingSpaceDeletion('different-request-id'), false, 'a stale confirmation cannot clear a different intent');
  assert.equal(career.pendingSpaceDeletion()?.requestId, original.requestId);
  assert.equal(career.abandonPendingSpaceDeletion(original.requestId), true, 'an explicit matching abandon clears the local recovery record');
  assert.equal(career.pendingSpaceDeletion(), null);
  const deleted = await career.deleteWholeSpace();
  assert.equal(deleted.status, 'deleted', 'a fresh request is allowed after an explicit abandon');
  assert.equal(posts, 2);
});

test('F1: concurrent deletion starts reserve one scope slot before the POST settles', async () => {
  let posts = 0;
  let finish;
  await freshLogin({
    'POST /api/v1/career/deletions': call => {
      posts++;
      finish = () => stub.succeed(call, { data: partialReceipt() });
    },
  });
  await career.loadCareer();
  const first = career.deleteWholeSpace();
  const second = await career.deleteWholeSpace().catch(error => error);
  assert.equal(errorCode(second), 'unresolved_action');
  assert.equal(posts, 1, 'the second call is rejected before another transport starts');
  finish();
  const partial = await first;
  assert.equal(partial.status, 'partial');
  assert.ok(career.pendingSpaceDeletion()?.requestId, 'the first request id remains recoverable');
  assert.equal(posts, 1);
});

test('R1: space export abandonment is scope-local and only clears the expected request id', async () => {
  await freshLogin({ 'POST /api/v1/career/exports': call => stub.fail(call, 'request:fail timeout') });
  await career.loadCareer();
  await assert.rejects(career.exportWholeSpace(), error => error.code === 'outcome_unknown');
  const pending = career.pendingSpaceExport();
  assert.ok(pending);
  assert.equal(career.abandonPendingSpaceExport('stale-confirmation'), false);
  assert.equal(career.pendingSpaceExport()?.requestId, pending.requestId);
  assert.equal(career.abandonPendingSpaceExport(pending.requestId), true);
  assert.equal(career.pendingSpaceExport(), null);
});

test('F2: export abandonment is refused during a deferred retry and ambiguous result keeps the id', async () => {
  const { confirmAbandonIntent } = await import('../src/career/export-deletion.gating.ts');
  let attempts = 0;
  let finishRetry;
  await freshLogin({
    'POST /api/v1/career/exports': call => {
      attempts++;
      if (attempts === 1) stub.fail(call, 'request:fail timeout');
      else finishRetry = () => stub.fail(call, 'request:fail timeout');
    },
  });
  await career.loadCareer();
  await assert.rejects(career.exportWholeSpace(), error => error.code === 'outcome_unknown');
  const original = career.pendingSpaceExport();
  assert.ok(original);
  const retry = career.retryPendingSpaceExport().catch(error => error);
  await new Promise(resolve => setImmediate(resolve));
  const result = await confirmAbandonIntent(original.requestId, {
    confirm: async () => true,
    currentRequestId: () => career.pendingSpaceExport()?.requestId,
    isBusy: () => career.spaceExportRecoveryActive(original.requestId),
    abandon: id => career.abandonPendingSpaceExport(id),
  });
  assert.equal(result, 'busy');
  assert.equal(career.abandonPendingSpaceExport(original.requestId), false, 'the service CAS also refuses active recovery');
  finishRetry();
  const ambiguous = await retry;
  assert.equal(errorCode(ambiguous), 'outcome_unknown');
  assert.equal(career.pendingSpaceExport()?.requestId, original.requestId, 'ambiguous retry retains its recoverable request id');
});

test('N9: a deletion response stores its recoverable intent under the scope captured before send', async () => {
  await freshLogin({ 'POST /api/v1/career/deletions': () => {/* answered after switching scope */} });
  await career.loadCareer();
  const { intentKeyFor } = await import('../src/services/career-intent.ts');
  const originalScope = runtime.auth.scope.capture();
  const originalKey = intentKeyFor('spaceDeletion', originalScope);
  const originalRequest = runtime.client.request;
  runtime.client.request = async function(options) {
    const result = await originalRequest.call(runtime.client, options);
    if (options.path === '/api/v1/career/deletions' && options.method === 'POST') {
      runtime.auth.scope.switchTo({ origin: originalScope.origin, userId: 'u2', tenantId: '2' });
    }
    return result;
  };
  try {
    const write = career.deleteWholeSpace();
    stub.succeed(stub.lastCall('request'), { data: partialReceipt() });
    await write;
  } finally { runtime.client.request = originalRequest; }

  assert.ok(stub.state.storage.get(originalKey), 'the late partial receipt writes to the sending scope key');
  assert.equal(career.pendingSpaceDeletion(), null, 'the new scope never sees the old scope deletion intent');
  assert.equal(career.abandonPendingSpaceDeletion(stub.state.storage.get(originalKey).requestId), false,
    'a stale page cannot abandon another active scope’s intent');
  runtime.auth.scope.switchTo({ origin: originalScope.origin, userId: originalScope.userId, tenantId: originalScope.tenantId });
  assert.equal(career.pendingSpaceDeletion()?.requestId, stub.state.storage.get(originalKey).requestId);
});

test('N9: a completed deletion retry removes the original sending-scope key after a scope switch', async () => {
  let posts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => {
      posts++;
      if (posts === 1) stub.succeed(call, { data: partialReceipt() });
      else { /* answered after switching scope */ }
    },
  });
  await career.loadCareer();
  await career.deleteWholeSpace();
  const { intentKeyFor } = await import('../src/services/career-intent.ts');
  const originalScope = runtime.auth.scope.capture();
  const originalKey = intentKeyFor('spaceDeletion', originalScope);
  const requestId = career.pendingSpaceDeletion()?.requestId;
  const originalRequest = runtime.client.request;
  runtime.client.request = async function(options) {
    const result = await originalRequest.call(runtime.client, options);
    if (options.path === '/api/v1/career/deletions' && options.method === 'POST') {
      runtime.auth.scope.switchTo({ origin: originalScope.origin, userId: 'u2', tenantId: '2' });
    }
    return result;
  };
  try {
    const retry = career.retryPendingSpaceDeletion();
    stub.succeed(stub.lastCall('request'), { data: deletedReceipt({ requestId }) });
    await retry;
  } finally { runtime.client.request = originalRequest; }

  assert.equal(stub.state.storage.get(originalKey), undefined, 'the completed retry removes the intent from the sending scope');
  assert.equal(career.pendingSpaceDeletion(), null, 'the new scope remains clear');
});

test('C1: an unknown export outcome is reconciled through the original request id', async () => {
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/exports': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: exportReceipt() }); },
    'GET /api/v1/career/exports/receipt': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  await assert.rejects(career.exportWholeSpace(), error => error.code === 'outcome_unknown');
  const pending = career.pendingSpaceExport();
  assert.ok(pending, 'the unresolved export is kept for recovery');
  const sent = careerCall('/exports').find(c => (c.options.method ?? 'GET') === 'POST').options.data.requestId;
  assert.equal(pending.requestId, sent);
  const receipt = await career.reconcilePendingSpaceExport();
  assert.equal(receipt.exportId, 'exp-space-1');
  assert.equal(career.pendingSpaceExport(), null, 'reconciliation clears the pending intent');
  assert.equal(new URL(careerCall('/exports/receipt')[0].options.url).searchParams.get('requestId'), sent, 'recovery replays the original request id');
});

test('C2: an unknown deletion outcome keeps the intent; a missing receipt is distinguishable and the resend replays the id', async () => {
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/deletions': call => { if (postFails) stub.fail(call, 'request:fail network'); else stub.succeed(call, { data: deletedReceipt() }); },
    'GET /api/v1/career/deletions/receipt': notFound,
  });
  await career.loadCareer();
  await assert.rejects(career.deleteWholeSpace(), error => error.code === 'outcome_unknown');
  const pending = career.pendingSpaceDeletion();
  assert.ok(pending, 'intent persisted for reconciliation');
  const missing = await career.reconcilePendingSpaceDeletion().catch(error => error);
  assert.ok(career.isReceiptMissing(missing), 'a missing receipt is distinguishable for the recovery UI');
  assert.equal(career.pendingSpaceDeletion()?.requestId, pending.requestId, 'a missing receipt keeps the intent');
  postFails = false;
  const receipt = await career.retryPendingSpaceDeletion();
  assert.equal(receipt.status, 'deleted');
  assert.equal(career.pendingSpaceDeletion(), null, 'a successful safe resend clears the intent');
  const bodies = careerCall('/deletions').filter(c => (c.options.method ?? 'GET') === 'POST' && new URL(c.options.url).pathname === '/api/v1/career/deletions').map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the resend replays the original request id');
  assert.equal(bodies[1].expectedRevision, bodies[0].expectedRevision, 'the resend replays the original fingerprint revision');
});

test('D1: after a complete deletion the old export receipt is rejected typed (404)', async () => {
  await freshLogin({
    'POST /api/v1/career/exports': call => stub.succeed(call, { data: exportReceipt() }),
    'POST /api/v1/career/deletions': call => stub.succeed(call, { data: deletedReceipt() }),
    'GET /api/v1/career/exports/receipt': notFound,
  });
  await career.loadCareer();
  const exported = await career.exportWholeSpace();
  await career.deleteWholeSpace();
  const stale = await career.spaceExportReceipt(exported.requestId).catch(error => error);
  assert.equal(errorCode(stale), 'not_found', 'the pre-deletion export receipt is dead after deletion');
  assert.ok(career.isReceiptMissing(stale), 'the UI can distinguish the dead old grant');
});

test('D2: clearing local career caches removes only wk:career storage keys and resets the desk', async () => {
  await freshLogin();
  await career.loadCareer();
  stub.state.storage.set('wk:career:search:u1:1', { requestId: 'r1', query: '岗位' });
  stub.state.storage.set('wk:career:application:u1:1', { requestId: 'r2' });
  stub.state.storage.set('wk:auth:token', 'keep-me');
  stub.state.storage.set('wk:theme', 'light');
  const cleared = career.clearCareerCaches();
  assert.deepEqual([...cleared].sort(), ['wk:career:application:u1:1', 'wk:career:search:u1:1'], 'exactly the career cache keys are cleared');
  assert.equal(stub.state.storage.has('wk:auth:token'), true, 'auth storage is not the career cache');
  assert.equal(stub.state.storage.has('wk:theme'), true, 'unrelated keys survive');
  assert.equal(career.careerDesk().snapshot, undefined, 'the shared desk is reset so stale views cannot come back');
});

test('E1: a scope change discards a stale deletion response and persists no intent', async () => {
  await freshLogin({
    'POST /api/v1/career/deletions': () => {/* hangs until the test answers */},
  });
  await career.loadCareer();
  const pending = career.deleteWholeSpace();
  await runtime.auth.logout(); // logout invalidates the scope
  stub.succeed(stub.lastCall('request'), { data: deletedReceipt() });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(career.pendingSpaceDeletion(), null, 'a stale response must not persist a recovery intent for the wrong scope');
});

test('E2: a cross-tenant boundary read surfaces the typed forbidden', async () => {
  await freshLogin({
    'GET /api/v1/career/deletions/boundary': call => stub.succeed(call, { statusCode: 403, data: { error: { code: 'forbidden', message: 'career workspace not allowed' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.deletionBoundary(), error => error.code === 'forbidden');
});

test('E3: a revision conflict on deletion is typed with the current revision', async () => {
  await freshLogin({
    'POST /api/v1/career/deletions': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 11 } } }),
  });
  await career.loadCareer();
  const refused = await career.deleteWholeSpace().catch(error => error);
  assert.equal(errorCode(refused), 'revision_conflict');
  assert.equal(refused.currentRevision, 11, 'the current revision travels for the reload affordance');
  assert.equal(career.pendingSpaceDeletion(), null, 'a definite refusal leaves no pending intent');
});

// —— 修复轮 M1：主按钮门控对齐 Web ExportDeletionPage（unknown/对账中封锁防重复防竞态）。
// Web 语义（apps/web/src/career/ExportDeletionPage.tsx）：
//   exportBlocked = exportPhase busy || unknown
//   deletionBlocked = deletionPhase busy || unknown || exportBlocked
//   导出按钮 disabled = exportBlocked || revision 未读取
//   删除按钮 disabled = deletionBlocked || revision 未读取 || 无边界 || 未知悉 || 已删除
//   知悉勾选 disabled = deletionBlocked
// 小程序页面把同一组可观察输入交给 lifecycleGating 计算（页面与测试同源同函数）。
const pageGatingFromServiceState = (over = {}) => {
  const { lifecycleGating } = lifecycleGatingModule;
  if (!lifecycleGating) throw new Error('lifecycleGating 模块未创建（M1 行为缺失）');
  return lifecycleGating({
    exportBusy: false,
    exportUnknown: career.pendingSpaceExport() !== null,
    deletionBusy: false,
    deletionUnknown: career.pendingSpaceDeletion() !== null,
    revisionLoaded: true,
    boundaryShown: true,
    acknowledged: true,
    deleted: false,
    ...over,
  });
};
const lifecycleGatingModule = {};

test('M1: an unknown export outcome blocks both main actions and the acknowledgement until reconciled', async () => {
  Object.assign(lifecycleGatingModule, await import('../src/career/export-deletion.gating.ts'));
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/exports': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: exportReceipt() }); },
    'GET /api/v1/career/exports/receipt': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  await assert.rejects(career.exportWholeSpace(), error => error.code === 'outcome_unknown');
  assert.ok(career.pendingSpaceExport(), 'the unknown export intent is persisted');
  // 发起后中断（结果未知）：即使边界/知悉/修订全部就绪，两个主按钮与知悉勾选都封锁。
  const unknown = pageGatingFromServiceState();
  assert.equal(unknown.exportDisabled, true, '发起导出主按钮在结果未知期间禁用（Web exportBlocked）');
  assert.equal(unknown.deletionDisabled, true, '发起完整删除主按钮被导出未决联动封锁（Web deletionBlocked ⊇ exportBlocked）');
  assert.equal(unknown.acknowledgeDisabled, true, '知悉勾选在未决期间禁用（Web ack disabled={deletionBlocked}）');
  // 「重新对账」动作用原请求编号把结果落定；成功后 intent 清除、门控恢复。
  const receipt = await career.reconcilePendingSpaceExport();
  assert.equal(receipt.exportId, 'exp-space-1');
  assert.equal(career.pendingSpaceExport(), null);
  const recovered = pageGatingFromServiceState();
  assert.equal(recovered.exportDisabled, false, '对账成功后导出主按钮恢复可用');
  assert.equal(recovered.deletionDisabled, false, '对账成功后删除主按钮恢复可用');
  assert.equal(recovered.acknowledgeDisabled, false, '对账成功后知悉勾选恢复可用');
});

test('M1: an unknown deletion blocks the delete main action; a reported partial does not (Web unknown ≠ error)', async () => {
  Object.assign(lifecycleGatingModule, await import('../src/career/export-deletion.gating.ts'));
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/deletions': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: partialReceipt() }); },
  });
  await career.loadCareer();
  await assert.rejects(career.deleteWholeSpace(), error => error.code === 'outcome_unknown');
  assert.ok(career.pendingSpaceDeletion(), 'the unknown deletion intent is persisted');
  const unknown = pageGatingFromServiceState();
  assert.equal(unknown.deletionDisabled, true, '删除结果未知期间删除主按钮禁用（Web deletionPhase unknown）');
  assert.equal(unknown.acknowledgeDisabled, true, '删除结果未知期间知悉勾选禁用');
  // 部分失败是已呈报的确定回执（Web phase=error，不按 unknown 封锁）：intent 保留供恢复，
  // 但页面持有 partial 回执时主按钮不因 unknown 被封锁——恢复走原编号重试入口。
  postFails = false;
  const receipt = await career.retryPendingSpaceDeletion();
  assert.equal(receipt.status, 'partial');
  assert.ok(career.pendingSpaceDeletion(), 'a partial deletion keeps the recoverable intent');
  const partial = pageGatingFromServiceState({ deletionUnknown: career.pendingSpaceDeletion() !== null && receipt.status !== 'partial' });
  assert.equal(partial.deletionDisabled, false, 'a reported partial is not an unknown outcome (Web partial keeps the main action open)');
  // deleted 终态：主按钮封锁（不可再次发起）。
  const deleted = pageGatingFromServiceState({ deletionUnknown: false, deleted: true });
  assert.equal(deleted.deletionDisabled, true, 'after a complete deletion the main action stays closed');
});

test('M1: in-flight runs (发起/对账/重试) block the main actions; export busy additionally blocks deletion', async () => {
  Object.assign(lifecycleGatingModule, await import('../src/career/export-deletion.gating.ts'));
  const { lifecycleGating } = lifecycleGatingModule;
  const ready = { exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: false, revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: false };
  const exportBusy = lifecycleGating({ ...ready, exportBusy: true });
  assert.equal(exportBusy.exportDisabled, true, '发起导出进行中禁用主按钮（Web busy）');
  assert.equal(exportBusy.deletionDisabled, true, '导出进行中联动封锁删除（deletionBlocked ⊇ exportBlocked）');
  const reconciling = lifecycleGating({ ...ready, exportBusy: true, exportUnknown: true });
  assert.equal(reconciling.acknowledgeDisabled, true, '对账期间知悉勾选禁用');
  const deletionBusy = lifecycleGating({ ...ready, deletionBusy: true });
  assert.equal(deletionBusy.deletionDisabled, true, '删除发起/对账/重试进行中禁用删除主按钮');
  assert.equal(deletionBusy.acknowledgeDisabled, true, '删除进行中知悉勾选禁用');
  const noRevision = lifecycleGating({ ...ready, revisionLoaded: false });
  assert.equal(noRevision.exportDisabled, true, '修订未读取时导出不可发起');
  assert.equal(noRevision.deletionDisabled, true, '修订未读取时删除不可发起');
  const noBoundary = lifecycleGating({ ...ready, boundaryShown: false });
  assert.equal(noBoundary.deletionDisabled, true, '未呈现边界清单时删除不可发起');
  const notAcked = lifecycleGating({ ...ready, acknowledged: false });
  assert.equal(notAcked.deletionDisabled, true, '未勾选知悉时删除不可发起');
  const idle = lifecycleGating(ready);
  assert.equal(idle.exportDisabled, false, '空闲且修订就绪时导出可发起');
  assert.equal(idle.deletionDisabled, false, '空闲且边界/知悉/修订就绪时删除可发起');
});

// —— 修复轮 2 F1：partial 确定回执后的恢复尝试以未决告终时，不得再按 partial 解锁——
// Web 基准（apps/web/src/career/ExportDeletionPage.tsx:291-293 门控 + 267-268/286 置 unknown）：
// runDeletion(fixed) uncertain → phase='unknown'（封锁）；lookupDeletionReceipt 失败 → phase='unknown'
// （封锁）；definite 拒绝 → attempt 清除 phase='error'（不封锁）。页面用同一组 gating 帮助函数
// 计算 deletionUnknown 输入与"恢复失败置未决"判定（export-deletion.tsx 与本测试同源同函数）。
const fix2Gating = {};
const pageDeletionUnknown = (recoveryUnresolved, inMemoryStatus) => {
  const { deletionOutcomeUnknown } = fix2Gating;
  if (!deletionOutcomeUnknown) throw new Error('deletionOutcomeUnknown 未创建（F1 行为缺失）');
  return deletionOutcomeUnknown({ intentPresent: career.pendingSpaceDeletion() !== null, inMemoryStatus, recoveryUnresolved });
};

test('F1: a partial receipt followed by an ambiguous retry re-blocks the deletion main action until a definite receipt', async () => {
  Object.assign(fix2Gating, await import('../src/career/export-deletion.gating.ts'));
  const { lifecycleGating, deletionRecoveryUnresolvedAfter } = fix2Gating;
  let attempts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => { attempts++; if (attempts === 1) stub.succeed(call, { data: partialReceipt() }); else stub.fail(call, 'request:fail timeout'); },
    'GET /api/v1/career/deletions/receipt': call => stub.succeed(call, { data: partialReceipt() }),
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  assert.ok(career.pendingSpaceDeletion(), 'partial keeps the recoverable intent');
  // 恢复失败前：持有确定 partial 回执，不因 unknown 封锁（Web phase=error）。
  assert.equal(pageDeletionUnknown(false, 'partial'), false);
  // 重试以结果未知告终：intent 保留 + 页面置未决（deletionRecoveryUnresolvedAfter 判定）→
  // 旧 partial 回执不再代表当前结果，主按钮与知悉回到 unknown 封锁。
  const ambiguous = await career.retryPendingSpaceDeletion().catch(error => error);
  assert.equal(errorCode(ambiguous), 'outcome_unknown');
  assert.ok(career.pendingSpaceDeletion(), 'ambiguous retry keeps the intent');
  const unresolvedFlag = deletionRecoveryUnresolvedAfter('retry', ambiguous, career.pendingSpaceDeletion() !== null);
  assert.equal(unresolvedFlag, true, 'ambiguous retry raises the unresolved flag');
  const unresolved = pageDeletionUnknown(unresolvedFlag, 'partial');
  assert.equal(unresolved, true, 'stale partial receipt + unresolved recovery counts as unknown');
  const blocked = lifecycleGating({ exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: unresolved, revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: false });
  assert.equal(blocked.deletionDisabled, true, 'deletion main action blocked');
  assert.equal(blocked.acknowledgeDisabled, true, 'acknowledgement locked again');
  // 对账拿到确定 partial 回执 → 未决清除 → 解锁（Web partial=error 不封锁）。
  const definite = await career.reconcilePendingSpaceDeletion();
  assert.equal(definite.status, 'partial');
  const settled = pageDeletionUnknown(false, definite.status);
  assert.equal(settled, false);
  const recovered = lifecycleGating({ exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: settled, revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: false });
  assert.equal(recovered.deletionDisabled, false, 'a definite receipt settles the unknown and reopens the main action');
});

test('N3/N2 wiring: unknown delete locks, and export/deletion recovery buttons use guarded abandon handlers', async () => {
  const source = await readFile(new URL('../src/career/export-deletion.tsx', import.meta.url), 'utf8');
  assert.match(source, /deletionController\.initialOutcomeUnknown\(\)/,
    'the page routes an outcome_unknown from initial delete through its tested state controller');
  assert.equal((source.match(/confirmAbandonIntent\(/g) ?? []).length, 2, 'both abandon handlers use the testable conditional-confirm seam');
  assert.match(source, /disabled=\{exportRecoveryBusy \|\| exportRecoveryConfirmationBusy\}/,
    'export recovery cannot be cleared while export/recovery requests are running');
  assert.match(source, /disabled=\{deletionRecoveryBusy \|\| deletionRecoveryConfirmationBusy\}/,
    'deletion recovery cannot be cleared while deletion/recovery requests are running');
  assert.match(source, /const expectedRequestId = pendingExport\?\.requestId/);
  assert.match(source, /const expectedRequestId = pendingDeletion\?\.requestId/);
  assert.match(source, /currentRequestId: \(\) => career\.pendingSpaceExport\(\)\?\.requestId/);
  assert.match(source, /currentRequestId: \(\) => career\.pendingSpaceDeletion\(\)\?\.requestId/);
  assert.match(source, /abandon: id => career\.abandonPendingSpaceExport\(id\)/);
  assert.match(source, /abandon: id => career\.abandonPendingSpaceDeletion\(id\)/);
});

test('F3: the page deletion controller drives unknown gating and recovery transitions', async () => {
  const { createDeletionPageController, lifecycleGating } = await import('../src/career/export-deletion.gating.ts');
  const controller = createDeletionPageController();
  const gate = () => lifecycleGating({ exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: controller.isUnknown(true), revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: false });
  controller.accept('partial');
  assert.equal(gate().deletionDisabled, false, 'a definite partial receipt leaves recovery available');
  controller.initialOutcomeUnknown();
  assert.equal(gate().deletionDisabled, true, 'an ambiguous initial deletion disables the main action');
  controller.recoveryFailed('retry', Object.assign(new Error('unknown'), { code: 'outcome_unknown' }), true);
  assert.equal(gate().deletionDisabled, true, 'ambiguous retry remains blocked');
  controller.recoveryFailed('reconcile', new Error('receipt unavailable'), true);
  assert.equal(gate().deletionDisabled, true, 'failed reconcile remains blocked');
  controller.accept('partial');
  assert.equal(gate().deletionDisabled, false, 'a definite partial receipt settles unknown state');
  controller.accept('deleted');
  assert.equal(lifecycleGating({ exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: false, revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: true }).deletionDisabled, true, 'deleted remains terminal');
});

test('N2/R1: conditional abandonment confirms before checking identity and in-flight state, then clears only the captured id', async () => {
  const { confirmAbandonIntent } = await import('../src/career/export-deletion.gating.ts');
  const deferred = () => {
    let resolve;
    const promise = new Promise(done => { resolve = done; });
    return { promise, resolve };
  };
  const run = async ({ confirmed = true, idAfterModalOpens, busyAfterModalOpens = false } = {}) => {
    const modal = deferred();
    let busy = false;
    const cleared = [];
    const pending = { id: 'export-1' };
    const work = confirmAbandonIntent('export-1', {
      confirm: () => modal.promise,
      currentRequestId: () => pending.id,
      isBusy: () => busy,
      abandon: id => { cleared.push(id); return pending.id === id; },
    });
    if (idAfterModalOpens) pending.id = idAfterModalOpens;
    if (busyAfterModalOpens) busy = true;
    modal.resolve(confirmed);
    return { status: await work, cleared };
  };
  assert.deepEqual(await run({ confirmed: false }), { status: 'cancelled', cleared: [] }, 'cancel keeps the intent');
  assert.deepEqual(await run({ idAfterModalOpens: 'export-2' }), { status: 'changed', cleared: [] }, 'an intent that changes while the modal is open is never cleared');
  assert.deepEqual(await run({ busyAfterModalOpens: true }), { status: 'busy', cleared: [] }, 'an operation started while the modal is open keeps the intent');
  assert.deepEqual(await run(), { status: 'abandoned', cleared: ['export-1'] }, 'a confirmed idle operation clears only the captured intent');
});

test('F2: conditional deletion abandonment refuses while a deferred recovery for that id is active', async () => {
  const { confirmAbandonIntent } = await import('../src/career/export-deletion.gating.ts');
  let finishRetry;
  let attempts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => {
      attempts++;
      if (attempts === 1) stub.succeed(call, { data: partialReceipt() });
      else finishRetry = () => stub.fail(call, 'request:fail timeout');
    },
  });
  await career.loadCareer();
  await career.deleteWholeSpace();
  const original = career.pendingSpaceDeletion();
  assert.ok(original);
  const retry = career.retryPendingSpaceDeletion().catch(error => error);
  // Let the request reach the deferred transport before attempting abandon.
  await new Promise(resolve => setImmediate(resolve));
  const result = await confirmAbandonIntent(original.requestId, {
    confirm: async () => true,
    currentRequestId: () => career.pendingSpaceDeletion()?.requestId,
    isBusy: () => career.spaceDeletionRecoveryActive(original.requestId),
    abandon: id => career.abandonPendingSpaceDeletion(id),
  });
  assert.equal(result, 'busy', 'the shared storage/service seam refuses and reports the active recovery');
  finishRetry();
  const ambiguous = await retry;
  assert.equal(errorCode(ambiguous), 'outcome_unknown');
  assert.equal(career.pendingSpaceDeletion()?.requestId, original.requestId, 'ambiguous retry keeps the original intent');
});

test('N1/N3: a partial intent prevents a second delete from reaching the new-request path', async () => {
  let posts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => {
      posts++;
      if (posts === 1) stub.succeed(call, { data: partialReceipt({ requestId: 'srv-old-partial' }) });
      else stub.fail(call, 'request:fail timeout');
    },
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  const oldIntentId = career.pendingSpaceDeletion()?.requestId;
  const refused = await career.deleteWholeSpace().catch(error => error);
  assert.equal(errorCode(refused), 'unresolved_action');
  const pending = career.pendingSpaceDeletion();
  assert.ok(pending, 'the original partial deletion remains recoverable');
  assert.equal(pending.requestId, oldIntentId, 'no new request id overwrites the original recovery target');

  const { deletionOutcomeUnknown, lifecycleGating } = await import('../src/career/export-deletion.gating.ts');
  const unknown = deletionOutcomeUnknown({ intentPresent: true, inMemoryStatus: partial.status, recoveryUnresolved: true });
  assert.equal(unknown, true, 'if an earlier partial view is followed by an ambiguous operation, unresolved still takes precedence');
  const gating = lifecycleGating({ exportBusy: false, exportUnknown: false, deletionBusy: false, deletionUnknown: unknown, revisionLoaded: true, boundaryShown: true, acknowledged: true, deleted: false });
  assert.equal(gating.deletionDisabled, true);
  assert.equal(posts, 1, 'the second attempt is refused before transport');
  assert.equal(career.pendingSpaceDeletion()?.requestId, pending.requestId, 'the unresolved request id remains available for recovery');
});

test('F1: a failed reconcile keeps the block until a definite receipt arrives (Web lookup failure → unknown)', async () => {
  Object.assign(fix2Gating, await import('../src/career/export-deletion.gating.ts'));
  const { deletionRecoveryUnresolvedAfter } = fix2Gating;
  let receiptMissing = true;
  await freshLogin({
    'POST /api/v1/career/deletions': call => stub.succeed(call, { data: partialReceipt() }),
    'GET /api/v1/career/deletions/receipt': call => { if (receiptMissing) notFound(call); else stub.succeed(call, { data: partialReceipt() }); },
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  assert.equal(pageDeletionUnknown(false, 'partial'), false);
  // 对账失败（回执暂不可读）：intent 保留，页面按 Web lookupDeletionReceipt catch → unknown 封锁。
  const refused = await career.reconcilePendingSpaceDeletion().catch(error => error);
  assert.equal(errorCode(refused), 'not_found');
  assert.ok(career.pendingSpaceDeletion(), 'a missing receipt keeps the intent');
  const flag = deletionRecoveryUnresolvedAfter('reconcile', refused, career.pendingSpaceDeletion() !== null);
  assert.equal(flag, true, 'any reconcile failure (receipt unreadable) raises the unresolved flag');
  assert.equal(pageDeletionUnknown(flag, 'partial'), true, 'deletion stays blocked while the receipt cannot be read');
  // 回执可读后：确定 partial → 未决清除 → 解锁。
  receiptMissing = false;
  const definite = await career.reconcilePendingSpaceDeletion();
  assert.equal(definite.status, 'partial');
  assert.equal(pageDeletionUnknown(false, definite.status), false);
});

test('F1: a definite retry refusal and a scope-changed failure do not raise the unresolved block (Web definite → error phase)', async () => {
  Object.assign(fix2Gating, await import('../src/career/export-deletion.gating.ts'));
  const { deletionRecoveryUnresolvedAfter } = fix2Gating;
  let attempts = 0;
  await freshLogin({
    'POST /api/v1/career/deletions': call => { attempts++; if (attempts === 1) stub.succeed(call, { data: partialReceipt() }); else stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 9 } } }); },
  });
  await career.loadCareer();
  const partial = await career.deleteWholeSpace();
  assert.equal(partial.status, 'partial');
  // definite 拒绝（非 ambiguous）：不置未决（Web runDeletion definite 分支清除 attempt 置 error）。
  const refused = await career.retryPendingSpaceDeletion().catch(error => error);
  assert.equal(errorCode(refused), 'revision_conflict');
  assert.equal(deletionRecoveryUnresolvedAfter('retry', refused, career.pendingSpaceDeletion() !== null), false, 'definite refusal is not an unknown outcome');
  assert.equal(pageDeletionUnknown(false, 'partial'), false);
  // SCOPE_CHANGED / intent 已不在当前作用域：不置未决（无恢复入口时不制造封锁死局）。
  assert.equal(deletionRecoveryUnresolvedAfter('retry', Object.assign(new Error('删除结果未知'), { code: 'outcome_unknown' }), false), false);
  assert.equal(deletionRecoveryUnresolvedAfter('reconcile', new Error('request:fail timeout'), false), false);
});

test('OCR2-037: a bare AUTH_REQUIRED on the first deletion never persists a recovery intent', async () => {
  await freshLogin();
  await career.loadCareer();
  await runtime.auth.logout();
  const failed = await career.deleteWholeSpace().catch(error => error);
  assert.equal(failed.message, 'SCOPE_CHANGED');
  assert.match(`${failed.cause?.message ?? ''}`, /AUTH_REQUIRED/);
  assert.equal(career.pendingSpaceDeletion(), null, 'AUTH_REQUIRED is a definite local failure — no deletion intent may be persisted');
});
