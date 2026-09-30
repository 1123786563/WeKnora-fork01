import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';
import { readFile } from 'node:fs/promises';

// T28 小程序 申请进展时间线与按需准备的可观察行为：真实 transport + AuthCoordinator
// 装配（T24/T26/T32 同款），假后端按 method+pathname 路由。进展合同（T17）：append-only、
// 纠错=追加引用事件原事件保留、expectedRevision 域=每应用事件计数（GET progress 的
// revision 字段，首事件=0）。准备合同（T19）：锚定实际投递版，未确认投递=typed 提示态
// （409 preparation_version_unknown，绝不静默改用最新版），草稿物化 materials 域可修订。
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
const preparationPage = await import('../src/career/progress-preparation.tsx');
const { clearPrivateCache } = await import('../src/platform/storage.ts');

const T = '2026-09-25T08:00:00Z';
const meA = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const meB = () => ({ success: true, data: { user: { id: 'u2', username: 'Bo' }, tenant: { id: 2, name: 'SpaceB' }, memberships: [] } });
const open3 = call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } });

// ---- 进展合同 fixture（packages/api-client decodeProgress* 必须全部通过）----
const prEvent = (over = {}) => ({
  eventId: 'evt-1', seq: 1, kind: 'progress_appended', eventType: 'interview', note: '一面定在 10 月 8 日下午',
  occurredAt: T, source: { kind: 'manual' }, confirmer: 'u1', corrected: false, requestId: 'srv-pr', createdAt: T, ...over,
});
const prReceipt = (over = {}) => ({
  kind: 'progress_appended', requestId: 'srv-pr', applicationId: 'app-1', eventId: 'evt-9', seq: 3, revision: 3,
  eventType: 'interview', stage: 'interview', note: '一面改到 10 月 10 日上午', occurredAt: T,
  source: { kind: 'manual' }, confirmer: 'u1', createdAt: T, ...over,
});
const prView = (events, over = {}) => ({ applicationId: 'app-1', revision: events.length, stage: 'interview', events, ...over });

// ---- 准备合同 fixture（decodePreparationReceipt 必须全部通过）----
const prepSection = { heading: '面试要点', content: '项目深挖：准备小程序求职工作台的两个取舍。', claims: [] };
const contentDigest = 'c'.repeat(64);
const prepAnchor = { submissionId: 'sub-1', materialId: 'mat-9', exportId: 'exp-1', version: 2, contentDigest };
const prepReceipt = (over = {}) => ({
  kind: 'preparation_generated', requestId: 'srv-prep', applicationId: 'app-1', preparationId: 'prep-1',
  focus: 'interview_prep', status: 'draft', anchor: prepAnchor, materialId: 'mat-9',
  body: { sections: [prepSection] }, reviewRisks: [],
  sources: {
    submittedVersion: prepAnchor,
    snapshot: { opportunityId: 'opp-1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) },
    factKeys: ['f_grad_year'], profileRevision: 3,
  },
  revision: 3, createdAt: T, ...over,
});
const materialReceipt = (over = {}) => ({ kind: 'material_edited', requestId: 'srv-mat', materialId: 'mat-9', status: 'draft', pinnedEvidence: { opportunityId: 'opp-1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64), profileRevision: 3 }, body: { sections: [prepSection] }, reviewRisks: [], ...over });

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
async function freshLogin(me = meA, extraRoutes = {}) {
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
const progressWrites = () => stub.state.calls.filter(c => /\/progress(\/correct)?$/.test(new URL(c.options.url).pathname) && (c.options.method ?? 'GET') === 'POST');
const logoutLikeRuntime = async () => { await runtime.auth.logout(); clearPrivateCache(); career.resetCareerDesk(); };

// ---- A 组：时间线权威顺序 + 跨端更正并见 ----

test('A1: the timeline renders in the server-authoritative order and a cross-client correction keeps the original traceable', async () => {
  const before = prView([prEvent(), prEvent({ eventId: 'evt-2', seq: 2, eventType: 'assessment', note: '笔试通过', requestId: 'srv-pr-2' })]);
  const after = prView([
    prEvent({ corrected: true }), // 跨端更正后：原文仍在，仅标记 corrected
    prEvent({ eventId: 'evt-2', seq: 2, eventType: 'assessment', note: '笔试通过', requestId: 'srv-pr-2' }),
    prEvent({ eventId: 'evt-3', seq: 3, kind: 'progress_corrected', eventType: 'interview', note: '一面改到 10 月 10 日上午', correctsEventId: 'evt-1', requestId: 'srv-pr-3' }),
  ], { revision: 3 });
  let current = before;
  await freshLogin(meA, {
    'GET /api/v1/career/applications/app-1/progress': call => stub.succeed(call, { data: current }),
  });
  const first = await career.applicationProgress('app-1');
  assert.deepEqual(first.events.map(e => e.eventId), ['evt-1', 'evt-2'], 'server order is authoritative — no client re-sort');
  assert.equal(first.stage, 'interview');
  assert.equal(first.revision, 2, 'revision is the per-application event count');
  // 跨端（Web/API）对该申请已见事件发起更正后，小程序刷新：
  current = after;
  const refreshed = await career.applicationProgress('app-1');
  assert.equal(refreshed.events.length, 3, 'the correction appends; nothing is removed');
  const original = refreshed.events.find(e => e.eventId === 'evt-1');
  assert.ok(original, 'the original event stays visible (原文可追溯)');
  assert.equal(original.corrected, true);
  const correction = refreshed.events.find(e => e.eventId === 'evt-3');
  assert.equal(correction.kind, 'progress_corrected');
  assert.equal(correction.correctsEventId, 'evt-1');
  assert.equal(refreshed.revision, 3);
  assert.equal(careerCall('/progress')[0].options.header.Authorization ?? careerCall('/progress')[0].options.header.authorization, 'Bearer t1');
});

test('A2: appending posts the frozen progress contract with the manual source and the per-application revision', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress': call => stub.succeed(call, { data: prReceipt({ seq: 1, revision: 1 }) }),
  });
  await career.loadCareer();
  const receipt = await career.appendProgressEvent({ applicationId: 'app-1', eventType: 'interview', note: '一面定在 10 月 8 日下午' }, 0);
  assert.equal(receipt.eventId, 'evt-9');
  assert.equal(receipt.kind, 'progress_appended');
  const body = progressWrites()[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['applicationId', 'eventType', 'expectedRevision', 'note', 'requestId', 'source'], 'the body must match the frozen AppendProgressInput (DisallowUnknownFields)');
  assert.deepEqual(body.source, { kind: 'manual' }, 'provenance is pinned to manual entry by the server anyway');
  assert.equal(body.expectedRevision, 0, 'the progress CAS domain is the per-application event count, not the profile revision');
  assert.equal(body.applicationId, 'app-1');
});

test('A3: an in-app correction targets the correction endpoint and carries correctsEventId', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress/correct': call => stub.succeed(call, { data: prReceipt({ kind: 'progress_corrected', correctsEventId: 'evt-1' }) }),
  });
  const receipt = await career.correctProgressEvent({ applicationId: 'app-1', eventType: 'interview', note: '一面改到 10 月 10 日上午', correctsEventId: 'evt-1' }, 1);
  assert.equal(receipt.kind, 'progress_corrected');
  assert.equal(receipt.correctsEventId, 'evt-1');
  assert.equal(new URL(progressWrites()[0].options.url).pathname, '/api/v1/career/applications/app-1/progress/correct');
  assert.equal(progressWrites()[0].options.data.correctsEventId, 'evt-1');
});

test('A4: an unknown progress outcome is reconciled through the original request id (seam: 未知对账)', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress': call => stub.fail(call, 'request:fail timeout'),
    'GET /api/v1/career/progress/receipt': call => stub.succeed(call, { data: prReceipt({ revision: 1 }) }),
  });
  await assert.rejects(career.appendProgressEvent({ applicationId: 'app-1', eventType: 'submitted' }, 0), error => error.code === 'outcome_unknown');
  const pending = career.pendingProgressWrite();
  assert.ok(pending, 'the unresolved progress write is kept for recovery');
  assert.equal(pending.requestId, progressWrites()[0].options.data.requestId);
  assert.equal(pending.expectedRevision, 0, 'the intent stores the original per-application revision for a safe resend');
  const receipt = await career.reconcilePendingProgress();
  assert.equal(receipt.eventId, 'evt-9');
  assert.equal(career.pendingProgressWrite(), null, 'reconciliation clears the intent');
  assert.equal(new URL(careerCall('/progress/receipt')[0].options.url).searchParams.get('requestId'), pending.requestId);
});

test('A5: a progress safe resend replays the original event revision even after the desk advanced', async () => {
  let postFails = true; let serverRevision = 3;
  await freshLogin(meA, {
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/applications/app-1/progress': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: prReceipt() }); },
    'GET /api/v1/career/progress/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.appendProgressEvent({ applicationId: 'app-1', eventType: 'submitted' }, 0), error => error.code === 'outcome_unknown');
  serverRevision = 8; // meanwhile the profile moved on (web-side confirm) — a different domain
  await career.refreshCareer();
  assert.equal(career.careerDesk().snapshot.revision, 8);
  postFails = false;
  const receipt = await career.retryPendingProgress();
  assert.equal(receipt.eventId, 'evt-9');
  const bodies = progressWrites().map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the resend replays the original request id');
  assert.equal(bodies[1].expectedRevision, 0, 'the resend replays the original event revision — never the profile revision');
  assert.equal(career.pendingProgressWrite(), null);
});

test('N8: a legacy progress intent without its per-application revision fails closed', async () => {
  let posts = 0;
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress': call => { posts++; stub.succeed(call, { data: prReceipt() }); },
  });
  const { intentKeyFor } = await import('../src/services/career-intent.ts');
  const key = intentKeyFor('progress', runtime.auth.scope.capture());
  stub.state.storage.set(key, { requestId: 'legacy-progress', input: { applicationId: 'app-1', eventType: 'submitted' } });

  await assert.rejects(career.retryPendingProgress(), error => error.code === 'intent_revision_missing');
  assert.equal(posts, 0, 'progress retry must not substitute the unrelated profile revision');
  assert.equal(career.pendingProgressWrite()?.requestId, 'legacy-progress', 'the unknown intent remains available for deliberate recovery');
});

test('A6: a progress revision conflict is definite and typed — no recovery intent (seam: 冲突回执)', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 2 } } }),
  });
  await assert.rejects(career.appendProgressEvent({ applicationId: 'app-1', eventType: 'submitted' }, 0), error => error.code === 'revision_conflict' && error.currentRevision === 2);
  assert.equal(career.pendingProgressWrite(), null, 'a definite conflict leaves no pending intent');
});

// ---- B 组：按需准备（锚定投递版 / 未知提示态 / 物化修订）----

test('B1: generating posts the frozen preparation contract and returns the submitted-version anchor', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/preparations': call => stub.succeed(call, { data: prepReceipt() }),
  });
  await career.loadCareer();
  const receipt = await career.generatePreparation({ applicationId: 'app-1', focus: 'interview_prep' });
  assert.equal(receipt.status, 'draft');
  assert.equal(receipt.focus, 'interview_prep');
  assert.equal(receipt.anchor.version, 2, 'the anchor is the actually submitted version, frozen at recording time');
  assert.equal(receipt.anchor.submissionId, 'sub-1');
  assert.equal(receipt.materialId, 'mat-9', 'the draft materializes in the material domain');
  assert.deepEqual(receipt.sources.factKeys, ['f_grad_year']);
  assert.equal(receipt.body.sections.length, 1);
  const call = careerCall('/preparations')[0];
  assert.deepEqual(Object.keys(call.options.data).sort(), ['applicationId', 'expectedRevision', 'focus', 'requestId'], 'the body must match the frozen GeneratePreparationInput');
  assert.equal(call.options.data.expectedRevision, 3, 'the preparation CAS houses against the profile head revision');
  assert.equal(call.options.header.Authorization ?? call.options.header.authorization, 'Bearer t1');
});

test('B2: an unconfirmed submitted version answers the typed prompt state — never a silent latest-version fallback', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/preparations': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'preparation_version_unknown', message: 'no confirmed submitted version' } } }),
  });
  const failed = await career.generatePreparation({ applicationId: 'app-1', focus: 'interview_prep' }).catch(error => error);
  assert.equal(errorCode(failed), 'preparation_version_unknown');
  assert.equal(career.isPreparationVersionUnknown(failed), true, 'the UI can render the same prompt semantics as the web panel');
  assert.equal(career.pendingPreparationWrite(), null, 'the typed prompt is a definite answer — no recovery intent');
  assert.equal(careerCall('/materials').length, 0, 'no silent fallback read or write against the latest material version');
});

test('B3: preparations are listed through the frozen list endpoint', async () => {
  await freshLogin(meA, {
    'GET /api/v1/career/applications/app-1/preparations': call => stub.succeed(call, { data: { preparations: [prepReceipt()] } }),
  });
  const list = await career.listPreparations('app-1');
  assert.equal(list.preparations.length, 1);
  assert.equal(list.preparations[0].preparationId, 'prep-1');
  assert.equal(new URL(careerCall('/preparations')[0].options.url).pathname, '/api/v1/career/applications/app-1/preparations');
});

test('B4: revising a preparation draft goes through the material domain and stays anchored', async () => {
  const revised = { sections: [{ heading: '面试要点', content: '项目深挖：准备小程序求职工作台的两个取舍；新增系统设计一节。', claims: [] }] };
  await freshLogin(meA, {
    'POST /api/v1/career/materials': call => stub.succeed(call, { data: materialReceipt({ body: revised }) }),
  });
  await career.loadCareer();
  const receipt = await career.editMaterial({ materialId: 'mat-9', body: revised });
  assert.equal(receipt.materialId, 'mat-9');
  assert.equal(receipt.status, 'draft', 'the revision stays a reviewable draft');
  const body = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials')[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['body', 'expectedRevision', 'materialId', 'requestId']);
});

test('B4a: preflight material read failure keeps the local draft and says no edit was submitted', async () => {
  const source = await readFile(new URL('../src/career/progress-preparation.tsx', import.meta.url), 'utf8');
  const preflight = source.slice(source.indexOf("setReviseErrCode('claim-preservation-preflight-failed')"), source.indexOf('try {\n      await career.editMaterial'));
  assert.match(preflight, /setDraftNotice\(`无法读取现有材料来保全引用主张；修订尚未提交/);
  assert.match(source.slice(source.indexOf('savePreparationDraft(draftFromSections())'), source.indexOf("setReviseErrCode('claim-preservation-preflight-failed')")), /await career\.material\(materialId\)/);
  assert.match(preflight, /throw error/, 'the failed preflight remains an explicit handled failure');
});

test('B4b: a committed edit remains successful when the follow-up material read fails', async () => {
  const events = [];
  await preparationPage.finishCommittedPreparationEdit({
    clearDraft() { events.push('clear'); }, async readBack() { events.push('read'); throw new Error('readback unavailable'); },
    onCommitted() { events.push('committed'); }, onCleanupFailure() { events.push('cleanup-warning'); },
    onReadbackFailure() { events.push('readback-warning'); }, onReadback() { events.push('readback-ok'); },
  });
  assert.deepEqual(events, ['committed', 'clear', 'read', 'readback-warning']);
});

test('M4: local draft removal failure after successful edit remains a committed outcome', async () => {
  const events = [];
  await preparationPage.finishCommittedPreparationEdit({
    clearDraft() { events.push('clear'); throw new Error('storage removal failed'); },
    async readBack() { events.push('read'); return { sections: [prepSection] }; },
    onCommitted() { events.push('committed'); },
    onCleanupFailure() { events.push('cleanup-warning'); },
    onReadbackFailure() { events.push('readback-warning'); },
    onReadback(body, cleanupFailed) { events.push(`readback-ok:${body.sections.length}:${cleanupFailed}`); },
  });
  assert.deepEqual(events, ['committed', 'clear', 'cleanup-warning', 'read', 'readback-ok:1:true']);
});

test('B5: an unknown preparation outcome is reconciled through the preparations receipt (seam: 未知对账)', async () => {
  let postFails = true;
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/preparations': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: prepReceipt() }); },
    'GET /api/v1/career/preparations/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.generatePreparation({ applicationId: 'app-1', focus: 'interview_prep' }), error => error.code === 'outcome_unknown');
  const pending = career.pendingPreparationWrite();
  assert.ok(pending);
  const missing = await career.reconcilePendingPreparation().catch(error => error);
  assert.ok(career.isReceiptMissing(missing), 'a missing receipt is distinguishable for the recovery UI');
  assert.ok(career.pendingPreparationWrite(), 'a missing receipt keeps the intent');
  postFails = false;
  const receipt = await career.retryPendingPreparation();
  assert.equal(receipt.preparationId, 'prep-1');
  assert.equal(career.pendingPreparationWrite(), null);
  const bodies = careerCall('/preparations').filter(c => (c.options.method ?? 'GET') === 'POST').map(c => c.options.data);
  assert.equal(bodies.length, 2);
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the resend replays the original request id');
  assert.equal(bodies[1].expectedRevision, bodies[0].expectedRevision, 'and the original profile revision');
});

// ---- C 组：断网只留本地草稿，不静默改申请状态 ----

test('C1: offline keeps the local preparation draft editable, never writes progress, and never auto-submits on reconnect', async () => {
  let materialUp = false;
  await freshLogin(meA, {
    'POST /api/v1/career/materials': call => { if (materialUp) stub.succeed(call, { data: materialReceipt() }); else stub.fail(call, 'request:fail 网络不可用'); },
  });
  await career.loadCareer();
  // 断网前草稿不存在；本地草稿 seam 按 scope 隔离存储。
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep'), undefined);
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: '断网中继续编辑' }] });
  const kept = platform.readPreparationDraft('app-1', 'interview_prep');
  assert.ok(kept, 'the local draft survives');
  assert.equal(kept.sections[0].content, '断网中继续编辑');
  assert.equal(typeof kept.savedAt, 'string');
  // 断网保存修订：网络失败 → 本地草稿保留，申请状态零改动。
  const failed = await career.editMaterial({ materialId: 'mat-9', body: { sections: [{ heading: '面试要点', content: '断网中继续编辑', claims: [] }] } }).catch(error => error);
  assert.equal(errorCode(failed), 'outcome_unknown');
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep').sections[0].content, '断网中继续编辑', 'the draft is still editable locally');
  assert.equal(progressWrites().length, 0, 'no progress write is ever attempted — the application status is not silently changed');
  assert.equal(career.pendingProgressWrite(), null);
  // 重联网：不自动提交——没有任何新请求发出，直到显式同步。
  materialUp = true;
  await new Promise(resolve => setTimeout(resolve, 30));
  const materialPosts = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials');
  assert.equal(materialPosts.length, 1, 'only the failed attempt — nothing auto-submits after the network returns');
  const receipt = await career.retryPendingMaterial(); // 显式同步
  assert.equal(receipt.materialId, 'mat-9');
  assert.equal(stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials').length, 2, 'the explicit sync performs exactly one more write');
});

// ---- D 组：账号切换缓存隔离 ----

test('D1: switching accounts never shows the previous user preparation draft', async () => {
  await freshLogin(meA, {});
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: 'A 的私人草稿' }] });
  assert.ok(platform.readPreparationDraft('app-1', 'interview_prep'));
  // 登出语义（runtime.logout = auth.logout + clearPrivateCache）：wk:career:* 一并清除。
  await logoutLikeRuntime();
  assert.ok(![...stub.state.storage.keys()].some(key => key.startsWith('wk:career:')), 'logout clears the career-controlled cache');
  // 键隔离第二层的真实验证在 D1b（此处 freshLogin 的 stub.reset() 会清库，B 读到
  // undefined 同时来自清库，不能单独证明键隔离）。
  await freshLogin(meB, {});
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep'), undefined, 'account B sees no trace of account A');
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', sections: [{ heading: '面试要点', content: 'B 的草稿' }] });
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep').sections[0].content, 'B 的草稿');
});

// OCR med-57：不清库、仅切作用域（与登录同一原语 scope.switchTo），比对 A/B 写入的
// 原始受控键——红线“账号切换不残留上一用户草稿”由键隔离本身保障，非空验证。
test('D1b: scope keys alone isolate drafts even when the cache survives (不清库、仅切作用域)', async () => {
  await freshLogin(meA, {});
  const keyA = platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: 'A 的私人草稿' }] });
  assert.ok(keyA.startsWith('wk:career:prep-draft:'), 'drafts live in the controlled store');
  // 缓存保留，只切作用域（模拟 clearPrivateCache 失效/遗漏的场景）。
  const stampA = runtime.auth.scope.capture();
  runtime.auth.scope.switchTo({ origin: stampA.origin, userId: 'u2', tenantId: '2' });
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep'), undefined, 'B 的作用域键读不到 A 的草稿——隔离来自键本身，不是清库');
  const keyB = platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', sections: [{ heading: '面试要点', content: 'B 的草稿' }] });
  assert.notEqual(keyB, keyA, 'B writes a different scope key');
  assert.equal(stub.state.storage.get(keyA)?.sections?.[0]?.content, 'A 的私人草稿', "A's raw key is not overwritten by B's write");
  assert.equal(stub.state.storage.get(keyB)?.sections?.[0]?.content, 'B 的草稿');
  // 切回 A：A 的草稿仍在（隔离是双向分流，不是删除）。
  runtime.auth.scope.switchTo({ origin: stampA.origin, userId: stampA.userId, tenantId: stampA.tenantId });
  assert.equal(platform.readPreparationDraft('app-1', 'interview_prep')?.sections?.[0]?.content, 'A 的私人草稿');
});

// ---- E/F 组：空间切换与跨租户 ----

test('E1: a scope change invalidates an in-flight progress write — nothing is applied or persisted (seam: 空间切换)', async () => {
  await freshLogin(meA, {
    'POST /api/v1/career/applications/app-1/progress': () => {/* hangs until the test answers */},
  });
  const pending = career.appendProgressEvent({ applicationId: 'app-1', eventType: 'submitted' }, 0);
  await runtime.auth.logout(); // logout invalidates the scope
  stub.succeed(stub.lastCall('request'), { data: prReceipt() });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(career.pendingProgressWrite(), null, 'a stale response must not persist a recovery intent for the wrong scope');
});

test('F1: cross-tenant progress and preparation reads stay typed forbidden', async () => {
  await freshLogin(meA, {
    'GET /api/v1/career/applications/app-x/progress': call => stub.succeed(call, { statusCode: 403, data: { error: { code: 'forbidden', message: 'career workspace not allowed' } } }),
    'GET /api/v1/career/applications/app-x/preparations': call => stub.succeed(call, { statusCode: 403, data: { error: { code: 'forbidden', message: 'career workspace not allowed' } } }),
  });
  await assert.rejects(career.applicationProgress('app-x'), error => error.code === 'forbidden');
  await assert.rejects(career.listPreparations('app-x'), error => error.code === 'forbidden');
});

// ---- 评审修复轮 R1（F1 主张保全 / F2 本地优先合并）----

const claimOf = id => ({ claimId: id, text: `主张${id}`, needsReview: false });
const matViewFor = body => ({
  materialId: 'mat-9', status: 'draft',
  pinnedEvidence: { opportunityId: 'opp-1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64), profileRevision: 3 },
  body, reviewRisks: [], versionCount: 1, versions: [{ version: 1, createdAt: T }], createdAt: T, updatedAt: T,
});

test('R1-P1: the local draft round-trips section claims', () => {
  stub.reset();
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: '断网编辑', claims: [claimOf('c1')] }] });
  const kept = platform.readPreparationDraft('app-1', 'interview_prep');
  assert.ok(kept, 'draft present');
  assert.deepEqual(kept.sections[0].claims, [claimOf('c1')], 'claims travel with the retained draft (F1 前置：本地草稿携带主张快照)');
  // 旧格式草稿（无 claims 字段）仍可读取，claims 为 []——由提交前取回兜底。
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'cover_letter', sections: [{ heading: '求职信', content: '旧格式' }] });
  const legacy = platform.readPreparationDraft('app-1', 'cover_letter');
  assert.deepEqual(legacy.sections[0].claims, [], 'legacy drafts without claims stay readable');
});

test('R1-P2: merging a retained draft into the server body is local-first — locally added sections survive, claims come from the server (F2)', () => {
  const server = [
    { heading: '面试要点', content: '服务端正文', claims: [claimOf('c1')] },
    { heading: '本地已删除的节', content: 'x', claims: [claimOf('c2')] },
  ];
  const local = [
    { heading: '面试要点', content: '本地编辑后的正文' },
    { heading: '断网新增节', content: '断网期间新增的内容' },
  ];
  const merged = platform.attachClaimsFromServer(local, server);
  assert.equal(merged.length, 2, 'local is the base — the section count follows the local draft');
  assert.equal(merged[0].heading, '面试要点');
  assert.equal(merged[0].content, '本地编辑后的正文', 'the user\'s local words win');
  assert.deepEqual(merged[0].claims, [claimOf('c1')], 'claims are re-attached from the server body');
  assert.equal(merged[1].heading, '断网新增节');
  assert.deepEqual(merged[1].claims, [], 'a genuinely new section starts claim-free');
  assert.ok(!merged.some(section => section.heading === '本地已删除的节'), 'a section deleted locally is not silently resurrected');
});

test('R1-P3: empty claims are recovered from the material domain before submission — never silently dropped (F1)', () => {
  const sections = [
    { heading: '面试要点', content: '编辑后', claims: [] },
    { heading: '系统设计', content: '新增节', claims: [] },
  ];
  const server = [
    { heading: '面试要点', content: '旧正文', claims: [claimOf('c1'), claimOf('c3')] },
    { heading: '其他节', content: 'y', claims: [claimOf('c9')] },
  ];
  const recovered = platform.recoverEmptyClaims(sections, server);
  assert.deepEqual(recovered[0].claims, [claimOf('c1'), claimOf('c3')], 'same-heading server claims are attached before the write');
  assert.deepEqual(recovered[1].claims, [], 'a new section with no same-heading server counterpart stays claim-free');
  const alreadyClaimed = [{ heading: '面试要点', content: 'z', claims: [claimOf('c7')] }];
  assert.deepEqual(platform.recoverEmptyClaims(alreadyClaimed, server)[0].claims, [claimOf('c7')], 'existing claims are never overwritten by the recovery');
});

test('R1-P4: the composed save chain preserves server claims through the real material write (F1 端到端)', async () => {
  const serverBody = { sections: [{ heading: '面试要点', content: '服务端正文', claims: [claimOf('c1')] }] };
  await freshLogin(meA, {
    'GET /api/v1/career/materials/mat-9': call => stub.succeed(call, { data: matViewFor(serverBody) }),
    'POST /api/v1/career/materials': call => stub.succeed(call, { data: materialReceipt({ materialId: 'mat-9', body: serverBody }) }),
  });
  await career.loadCareer();
  // 旧格式本地草稿（heading/content only——正是 F1 场景：断网独立入口保存后 claims 未知）。
  platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: '编辑后' }] });
  const draft = platform.readPreparationDraft('app-1', 'interview_prep');
  // 页面 saveRevision 的同链组合（t-button 不可单测，逐句复刻页面逻辑）：
  const editable = draft.sections.map(section => ({ heading: section.heading, content: section.content, claims: Array.isArray(section.claims) ? section.claims : [] }));
  let body = career.bodyFromEditable(editable);
  if (body.sections.some(section => (section.claims ?? []).length === 0)) {
    body = { sections: platform.recoverEmptyClaims(body.sections, (await career.material('mat-9')).body.sections) };
  }
  await career.editMaterial({ materialId: 'mat-9', body });
  const posted = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials')[0].options.data;
  assert.deepEqual(posted.body.sections[0].claims, [claimOf('c1')], 'claims survive the write — the server body is not replaced by a claim-free one');
  assert.equal(posted.body.sections[0].content, '编辑后', 'the edited content still lands');
});
