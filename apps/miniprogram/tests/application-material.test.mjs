import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// T26 小程序 申请/材料/投递的可观察行为：真实 transport + AuthCoordinator 装配（T24
// career-discovery 同款），假后端按 method+pathname 路由。career 端点是裸 JSON，
// 错误统一是 {error:{code,message}}。所有合同与 Web 同源：同一批 api-client 解码器
// （packages/api-client/src/career.ts），断言请求体字段与服务端冻结结构一一对应。
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
const { errorMessage } = await import('../src/core/errors.ts');

const T = '2026-09-25T08:00:00Z';
const encoder = new TextEncoder();
const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
const open3 = call => stub.succeed(call, { data: { revision: 3, facts: [], proposals: [] } });
const evaluation = (over = {}) => ({ kind: 'evaluation_created', requestId: 'srv-ev', evaluationId: 'ev-1', opportunityId: 'opp-1', snapshotId: 'snap-1', profileRevision: 3, status: 'eligible', ...over });
const application = (over = {}) => ({
  applicationId: 'app-1', requestId: 'srv-app', linkState: 'ready', taskId: 'task-1', qualified: true,
  pinnedEvidence: { opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', profileRevision: 3, evaluationStatus: 'eligible', batchIdentity: '2026秋招a批' }, ...over,
});
const matPin = { opportunityId: 'opp-1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64), profileRevision: 3 };
const matSection = { heading: '教育背景', content: '本科，计算机科学', claims: [{ claimId: 'c1', text: '本科学历', needsReview: false }] };
const materialBody = { sections: [matSection] };
const materialReceipt = (over = {}) => ({ kind: 'material_edited', requestId: 'srv-mat', materialId: 'mat-1', status: 'draft', pinnedEvidence: matPin, body: materialBody, reviewRisks: [], ...over });
const materialView = (over = {}) => ({ materialId: 'mat-1', status: 'confirmed', pinnedEvidence: matPin, body: materialBody, reviewRisks: [], versionCount: 2, versions: [{ version: 1, createdAt: T }, { version: 2, createdAt: T }], createdAt: T, updatedAt: T, ...over });
const contentDigest = 'c'.repeat(64);
const exportReceipt = (over = {}) => ({
  kind: 'material_published', requestId: 'srv-exp', exportId: 'exp-1', materialId: 'mat-1', version: 2, status: 'submittable', submittable: true, contentDigest,
  files: [
    { format: 'pdf', materialId: 'mat-1', version: 2, contentDigest, fileDigest: 'd'.repeat(64), size: 2048, verified: true },
    { format: 'docx', materialId: 'mat-1', version: 2, contentDigest, fileDigest: 'e'.repeat(64), size: 4096, verified: true },
  ], createdAt: T, ...over,
});
const submission = (over = {}) => ({
  kind: 'submission_recorded', requestId: 'srv-sub', applicationId: 'app-1', submissionId: 'sub-1', channel: 'web', occurredAt: T,
  versionConfirmed: true, boundVersion: { materialId: 'mat-1', exportId: 'exp-1', version: 2, contentDigest },
  confirmer: 'u1', revision: 3, createdAt: T, ...over,
});
const PDF_BYTES = '%PDF-1.4 weknora career material v2 (fixture)';
const grantFor = format => ({
  exportId: 'exp-1', materialId: 'mat-1', version: 2, format,
  digest: platform.sha256Hex(encoder.encode(format === 'pdf' ? PDF_BYTES : `${PDF_BYTES} docx`)),
  size: encoder.encode(format === 'pdf' ? PDF_BYTES : `${PDF_BYTES} docx`).length,
  expiresAt: 1790000000, signature: 'sig-1',
  url: `/api/v1/career/materials/mat-1/exports/exp-1/download?format=${format}&expires=1790000000&signature=sig-1`,
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
    'GET /api/v1/career/open': open3,
    ...extraRoutes,
  });
  await runtime.auth.login('u@example.test', 'pw');
  career.resetCareerDesk();
}
const careerCall = suffix => stub.state.calls.filter(c => new URL(c.options.url).pathname.startsWith('/api/v1/career') && (!suffix || new URL(c.options.url).pathname.includes(suffix)));
const errorCode = error => error?.code;

test('F0: sha256Hex matches public test vectors before any digest checks trust it', () => {
  assert.equal(platform.sha256Hex(encoder.encode('')), 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855');
  assert.equal(platform.sha256Hex(encoder.encode('abc')), 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
  assert.equal(platform.sha256Hex(encoder.encode('abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq')), '248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1');
});

test('A1: evaluation posts the frozen contract and returns the same receipt the web sees', async () => {
  await freshLogin({
    'POST /api/v1/career/evaluations': call => stub.succeed(call, { data: evaluation() }),
  });
  await career.loadCareer();
  const receipt = await career.evaluateOpportunity('opp-1', 'snap-1');
  assert.equal(receipt.evaluationId, 'ev-1');
  assert.equal(receipt.status, 'eligible');
  assert.equal(receipt.profileRevision, 3);
  const body = careerCall('/evaluations')[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['opportunityId', 'requestId', 'snapshotId'], 'body must match the frozen evaluation contract (DisallowUnknownFields)');
  assert.equal(careerCall('/evaluations')[0].options.header.Authorization, 'Bearer t1', 'identity flows through the authenticated client');
});

test('A2: application creation posts the frozen CreateApplicationInput against the same backend as web', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { data: application() }),
  });
  await career.loadCareer();
  const receipt = await career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '2026 秋招 A 批', continueDespiteHardFailure: false });
  assert.equal(receipt.applicationId, 'app-1');
  assert.equal(receipt.linkState, 'ready');
  assert.equal(receipt.pinnedEvidence.batchIdentity, '2026秋招a批');
  const call = careerCall('/applications')[0];
  assert.equal(new URL(call.options.url).pathname, '/api/v1/career/applications');
  assert.deepEqual(Object.keys(call.options.data).sort(), ['batchIdentity', 'continueDespiteHardFailure', 'evaluationId', 'expectedRevision', 'opportunityId', 'requestId', 'snapshotId'], 'the body must match the frozen contract (DisallowUnknownFields)');
  assert.equal(call.options.data.expectedRevision, 3, 'expected revision comes from the shared desk snapshot');
  assert.equal(call.options.header.Authorization, 'Bearer t1');
});

test('A3: material edits confirm a new immutable version and never overwrite the old one', async () => {
  await freshLogin({
    'POST /api/v1/career/materials': call => stub.succeed(call, { data: materialReceipt() }),
    'POST /api/v1/career/materials/confirm': call => stub.succeed(call, { data: materialReceipt({ kind: 'material_confirmed', status: 'confirmed', version: 2 }) }),
    'GET /api/v1/career/materials/mat-1/versions': call => stub.succeed(call, { data: { materialId: 'mat-1', versions: [{ version: 1, createdAt: T }, { version: 2, createdAt: T }] } }),
  });
  await career.loadCareer();
  const edited = await career.editMaterial({ opportunityId: 'opp-1', snapshotId: 'snap-1', body: materialBody });
  assert.equal(edited.materialId, 'mat-1');
  assert.equal(edited.status, 'draft');
  const editBody = careerCall('/materials')[0].options.data;
  assert.deepEqual(Object.keys(editBody).sort(), ['body', 'expectedRevision', 'opportunityId', 'requestId', 'snapshotId'], 'creation body carries opportunity evidence, not a material id');
  const confirmed = await career.confirmMaterial('mat-1');
  assert.equal(confirmed.version, 2, 'confirmation mints the next immutable version');
  const confirmBody = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials/confirm')[0].options.data;
  assert.deepEqual(Object.keys(confirmBody).sort(), ['expectedRevision', 'materialId', 'requestId']);
  const versions = await career.materialVersions('mat-1');
  assert.deepEqual(versions.versions.map(v => v.version), [1, 2], 'the old version stays listed — edits append, never overwrite');
});

test('A4: publishing posts the frozen export contract and returns the both-verified pair', async () => {
  await freshLogin({
    'POST /api/v1/career/materials/mat-1/exports': call => stub.succeed(call, { data: exportReceipt() }),
  });
  await career.loadCareer();
  const receipt = await career.publishMaterial('mat-1', 2);
  assert.equal(receipt.submittable, true);
  assert.equal(receipt.files.length, 2);
  assert.ok(receipt.files.every(f => f.verified));
  const body = careerCall('/exports')[0].options.data;
  assert.deepEqual(Object.keys(body).sort(), ['expectedRevision', 'requestId', 'version'], 'materialId travels in the path; the body stays frozen');
});

test('B1: a hard-ineligible application without explicit continuation is refused typed', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'hard_ineligible_requires_continue', message: 'requires explicit continuation' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false }), error => error.code === 'hard_ineligible_requires_continue');
  assert.equal(career.pendingApplication(), null, 'a definite refusal leaves no pending intent');
});

test('B2: explicit continuation creates the application and the hard warning stays attached', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { data: application({
      qualified: false,
      warning: { evaluationId: 'ev-1', evaluationStatus: 'ineligible', hardRuleId: 'graduation-year', reasonCode: 'graduation_year_mismatch' },
      pinnedEvidence: { opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', profileRevision: 3, evaluationStatus: 'ineligible', batchIdentity: '2026秋招a批' },
    }) }),
  });
  await career.loadCareer();
  const receipt = await career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '2026 秋招 A 批', continueDespiteHardFailure: true });
  assert.equal(receipt.qualified, false, 'qualified=false stays visible');
  assert.equal(receipt.warning.evaluationId, 'ev-1', 'the hard warning travels with the receipt for the persistent UI notice');
  assert.equal(careerCall('/applications')[0].options.data.continueDespiteHardFailure, true);
});

test('C1: a revision conflict on application creation surfaces the typed server receipt', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'revision_conflict', message: 'stale view', currentRevision: 9 } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false }), error => error.code === 'revision_conflict' && error.currentRevision === 9);
});

test('C2: one application per job and batch — the duplicate is the typed conflict it is', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { statusCode: 409, data: { error: { code: 'application_conflict', message: 'already applied to this job and batch' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '同一批', continueDespiteHardFailure: false }), error => error.code === 'application_conflict');
});

test('C3: a repeat submission confirmation is refused typed and the record list stays one', async () => {
  let posts = 0;
  await freshLogin({
    'POST /api/v1/career/applications/app-1/submissions': call => { posts++; if (posts === 1) stub.succeed(call, { data: submission() }); else stub.succeed(call, { statusCode: 409, data: { error: { code: 'submission_already_confirmed', message: 'already confirmed' } } }); },
    'GET /api/v1/career/applications/app-1/submissions': call => stub.succeed(call, { data: { submissions: [submission()] } }),
  });
  await career.loadCareer();
  const input = { applicationId: 'app-1', channel: 'web', materialId: 'mat-1', exportId: 'exp-1', versionUnknown: false };
  const first = await career.recordSubmission(input);
  assert.equal(first.submissionId, 'sub-1');
  await assert.rejects(career.recordSubmission(input), error => error.code === 'submission_already_confirmed');
  const list = await career.listSubmissions('app-1');
  assert.equal(list.submissions.length, 1, 'one application holds at most one submission record');
});

test('D1: an unknown application outcome is reconciled through the original request id', async () => {
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/applications': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: application() }); },
    'GET /api/v1/career/applications/receipt': call => stub.succeed(call, { data: application() }),
  });
  await career.loadCareer();
  await assert.rejects(career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false }), error => error.code === 'outcome_unknown');
  const pending = career.pendingApplication();
  assert.ok(pending, 'the unresolved application is kept for recovery');
  const sent = careerCall('/applications')[0].options.data.requestId;
  assert.equal(pending.requestId, sent);
  const receipt = await career.reconcilePendingApplication();
  assert.equal(receipt.applicationId, 'app-1');
  assert.equal(career.pendingApplication(), null, 'reconciliation clears the pending intent');
  assert.equal(new URL(careerCall('/applications/receipt')[0].options.url).searchParams.get('requestId'), sent, 'recovery replays the original request id');
});

test('D2: a missing material receipt during reconciliation keeps the intent and resends the same id', async () => {
  let postFails = true;
  await freshLogin({
    'POST /api/v1/career/materials/confirm': call => { if (postFails) stub.fail(call, 'request:fail network'); else stub.succeed(call, { data: materialReceipt({ kind: 'material_confirmed', status: 'confirmed', version: 2 }) }); },
    'GET /api/v1/career/materials/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.confirmMaterial('mat-1'), error => error.code === 'outcome_unknown');
  const pending = career.pendingMaterialWrite();
  assert.ok(pending, 'intent persisted for reconciliation');
  const missing = await career.reconcilePendingMaterial().catch(error => error);
  assert.ok(career.isReceiptMissing(missing), 'a missing receipt is distinguishable for the recovery UI');
  assert.equal(career.pendingMaterialWrite()?.requestId, pending.requestId, 'a missing receipt keeps the intent');
  postFails = false;
  const receipt = await career.retryPendingMaterial();
  assert.equal(receipt.version, 2);
  assert.equal(career.pendingMaterialWrite(), null, 'a successful safe resend clears the intent');
  const bodies = careerCall('/materials/confirm').map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[0].requestId, bodies[1].requestId, 'the resend replays the original request id');
});

test('D3: an unknown submission outcome is recovered through the submissions receipt endpoint', async () => {
  await freshLogin({
    'POST /api/v1/career/applications/app-1/submissions': call => stub.fail(call, 'request:fail timeout'),
    'GET /api/v1/career/submissions/receipt': call => stub.succeed(call, { data: submission() }),
  });
  await career.loadCareer();
  await assert.rejects(career.recordSubmission({ applicationId: 'app-1', channel: 'email', versionUnknown: true }), error => error.code === 'outcome_unknown');
  const pending = career.pendingSubmission();
  assert.ok(pending);
  const receipt = await career.reconcilePendingSubmission();
  assert.equal(receipt.submissionId, 'sub-1');
  assert.equal(career.pendingSubmission(), null);
  assert.equal(new URL(careerCall('/submissions/receipt')[0].options.url).searchParams.get('requestId'), pending.requestId);
});

test('E1: a scope change invalidates an in-flight application write — nothing is applied or persisted', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': () => {/* hangs until the test answers */},
  });
  await career.loadCareer();
  const pending = career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false });
  await runtime.auth.clear(); // logout invalidates the scope
  stub.succeed(stub.lastCall('request'), { data: application() });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(career.pendingApplication(), null, 'a stale response must not persist a recovery intent for the wrong scope');
});

test('F1: a download redeems the grant authenticated, verifies the digest, opens, and cleans up', async () => {
  const grant = grantFor('pdf');
  await freshLogin({
    'POST /api/v1/career/materials/mat-1/exports/exp-1/signed-url': call => stub.succeed(call, { data: grant }),
    'GET /api/v1/career/materials/mat-1/exports/exp-1/download': call => {
      stub.state.fileContents.set('http://tmp/dl.pdf', PDF_BYTES);
      stub.succeed(call, { statusCode: 200, tempFilePath: 'http://tmp/dl.pdf' });
    },
  });
  await career.loadCareer();
  const { grant: used, check } = await career.openMaterialExport('mat-1', 'exp-1', 'pdf');
  assert.equal(used.signature, 'sig-1');
  const grantCall = careerCall('/signed-url')[0];
  assert.deepEqual(Object.keys(grantCall.options.data).sort(), ['format', 'ttlSeconds'], 'the signed-url body stays frozen');
  const download = stub.state.calls.find(c => c.kind === 'downloadFile');
  assert.ok(download, 'native downloadFile used');
  assert.equal(download.options.header.Authorization, 'Bearer t1', 'the redemption carries the auth header');
  assert.equal(new URL(download.options.url).pathname, '/api/v1/career/materials/mat-1/exports/exp-1/download');
  assert.equal(new URL(download.options.url).searchParams.get('format'), 'pdf');
  assert.equal(new URL(download.options.url).searchParams.get('signature'), 'sig-1');
  // 校验记录：全字节 sha256 与授权 digest 比对（最强层级），大小一致，打开后副本即清理。
  assert.equal(check.digestMatched, true);
  assert.equal(check.expectedDigest, grant.digest);
  assert.equal(check.actualDigest, grant.digest);
  assert.equal(check.size, grant.size);
  assert.equal(check.format, 'pdf');
  assert.equal(check.version, 2);
  assert.equal(check.opened, true);
  assert.equal(stub.state.openedDocuments.length, 1, 'the document actually opened');
  assert.equal(stub.state.copies.length, 1, 'a private user-path copy was made before opening (D3)');
  assert.equal(stub.state.removedFiles.length, 1, 'the private copy is cleaned up right after opening');
});

test('F2: a tampered file fails the digest check typed and is never opened', async () => {
  await freshLogin({
    'POST /api/v1/career/materials/mat-1/exports/exp-1/signed-url': call => stub.succeed(call, { data: { ...grantFor('pdf'), digest: 'f'.repeat(64) } }),
    'GET /api/v1/career/materials/mat-1/exports/exp-1/download': call => {
      stub.state.fileContents.set('http://tmp/dl.pdf', PDF_BYTES);
      stub.succeed(call, { statusCode: 200, tempFilePath: 'http://tmp/dl.pdf' });
    },
  });
  await career.loadCareer();
  await assert.rejects(career.openMaterialExport('mat-1', 'exp-1', 'pdf'), error => error.code === 'export_digest_mismatch');
  assert.equal(stub.state.openedDocuments.length, 0, 'a mismatched file must not be opened');
});

test('F3: an expired grant is re-issued once and the retry succeeds', async () => {
  let grants = 0, downloads = 0;
  const grant = grantFor('pdf');
  await freshLogin({
    'POST /api/v1/career/materials/mat-1/exports/exp-1/signed-url': call => { grants++; stub.succeed(call, { data: { ...grant, signature: `sig-${grants}` } }); },
    'GET /api/v1/career/materials/mat-1/exports/exp-1/download': call => {
      downloads++;
      if (downloads === 1) {
        stub.state.fileContents.set('http://tmp/err.json', JSON.stringify({ error: { code: 'export_grant_invalid', message: 'grant expired' } }));
        stub.succeed(call, { statusCode: 404, tempFilePath: 'http://tmp/err.json' });
        return;
      }
      stub.state.fileContents.set('http://tmp/dl2.pdf', PDF_BYTES);
      stub.succeed(call, { statusCode: 200, tempFilePath: 'http://tmp/dl2.pdf' });
    },
  });
  await career.loadCareer();
  const { check } = await career.openMaterialExport('mat-1', 'exp-1', 'pdf');
  assert.equal(grants, 2, 'the expired grant is re-issued exactly once');
  assert.equal(downloads, 2);
  assert.equal(check.digestMatched, true);
  assert.equal(check.opened, true);
  assert.equal(new URL(stub.state.calls.filter(c => c.kind === 'downloadFile')[1].options.url).searchParams.get('signature'), 'sig-2', 'the retry redeems the fresh grant');
});

test('G1: submission confirmation performs exactly one record write and nothing else', async () => {
  await freshLogin({
    'POST /api/v1/career/applications/app-1/submissions': call => stub.succeed(call, { data: submission() }),
  });
  await career.loadCareer();
  const receipt = await career.recordSubmission({ applicationId: 'app-1', channel: 'web', materialId: 'mat-1', exportId: 'exp-1', versionUnknown: false, note: '本人已在官网投递' });
  assert.equal(receipt.versionConfirmed, true);
  assert.equal(receipt.boundVersion.version, 2, 'the bound version is frozen in the receipt');
  const posts = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/applications/app-1/submissions');
  assert.equal(posts.length, 1, 'exactly one record write');
  assert.deepEqual(Object.keys(posts[0].options.data).sort(), ['applicationId', 'channel', 'expectedRevision', 'exportId', 'materialId', 'note', 'requestId', 'versionUnknown']);
  assert.equal(posts[0].options.data.versionUnknown, false);
  // 零自动提交零外发：整个会话没有任何离开 API origin 的请求，也没有投递之外的 career 写。
  assert.ok(stub.state.calls.every(c => new URL(c.options.url).origin === 'https://api.example.test'), 'no request leaves the API origin');
  assert.ok(stub.state.calls.filter(c => c.kind !== 'request').length === 0, 'no native upload/download/mail side effect fires');
});

test('G2: the explicit unknown marker is exclusive — no version binding rides along', async () => {
  await freshLogin({
    'POST /api/v1/career/applications/app-1/submissions': call => stub.succeed(call, { data: submission({ versionConfirmed: false, boundVersion: undefined, channel: 'email' }) }),
  });
  await career.loadCareer();
  const receipt = await career.recordSubmission({ applicationId: 'app-1', channel: 'email', versionUnknown: true });
  assert.equal(receipt.versionConfirmed, false);
  const body = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/applications/app-1/submissions')[0].options.data;
  assert.equal('materialId' in body, false);
  assert.equal('exportId' in body, false);
  assert.equal(body.versionUnknown, true);
});

test('H1: reading a missing (or cross-tenant) application surfaces the typed not-found', async () => {
  await freshLogin({
    'GET /api/v1/career/applications/app-x': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'career application not found' } } }),
  });
  await career.loadCareer();
  const missing = await career.getApplication('app-x').catch(error => error);
  assert.equal(errorCode(missing), 'not_found');
  assert.ok(career.isReceiptMissing(missing), 'the UI can distinguish the recovery state');
});

test('H2: a forbidden material read stays typed for the actionable state', async () => {
  await freshLogin({
    'GET /api/v1/career/materials/mat-9': call => stub.succeed(call, { statusCode: 403, data: { error: { code: 'forbidden', message: 'career workspace not allowed' } } }),
  });
  await career.loadCareer();
  await assert.rejects(career.material('mat-9'), error => error.code === 'forbidden');
  assert.ok(errorMessage({ code: 'forbidden', message: 'forbidden' }).length > 0, 'the error chain renders a prompt');
});

test('H3: link_state linking reconciles through the original request id', async () => {
  await freshLogin({
    'POST /api/v1/career/applications': call => stub.succeed(call, { data: application({ linkState: 'linking', taskId: undefined }) }),
    'POST /api/v1/career/applications/link/reconcile': call => stub.succeed(call, { data: application() }),
  });
  await career.loadCareer();
  const created = await career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false });
  assert.equal(created.linkState, 'linking');
  const ready = await career.reconcileApplicationLink(created.requestId);
  assert.equal(ready.linkState, 'ready');
  const reconcileCall = careerCall('/link/reconcile')[0];
  assert.deepEqual(reconcileCall.options.data, { requestId: 'srv-app' }, 'link reconciliation replays the original request id');
});

// ---- 评审修复轮（F1-F5）：显式选择、原始指纹重放、正文往返、时间严格解析 ----

test('S1: an unselected version is never inferred as the explicit unknown (F1)', () => {
  const submittable = [{ exportId: 'exp-1', materialId: 'mat-1' }];
  assert.deepEqual(career.resolveSubmissionVersion('', submittable), { status: 'unselected' }, 'empty choice blocks the write');
  assert.deepEqual(career.resolveSubmissionVersion(career.SUBMISSION_VERSION_UNKNOWN_CHOICE, submittable), { status: 'unknown' }, 'the unknown marker must be explicitly picked');
  assert.deepEqual(career.resolveSubmissionVersion('exp-1', submittable), { status: 'bound', materialId: 'mat-1', exportId: 'exp-1' });
  assert.deepEqual(career.resolveSubmissionVersion('gone', submittable), { status: 'unselected' }, 'a vanished export id is unselected, not silently unknown');
});

test('D4: a safe resend replays the original expected revision, not the current desk revision (F3)', async () => {
  let postFails = true; let serverRevision = 3; let posts = 0;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/applications': call => { posts++; if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: application() }); },
    'GET /api/v1/career/applications/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  const failed = await career.createApplication({ opportunityId: 'opp-1', snapshotId: 'snap-1', evaluationId: 'ev-1', batchIdentity: '批', continueDespiteHardFailure: false }).catch(error => error);
  assert.equal(errorCode(failed), 'outcome_unknown');
  const pending = career.pendingApplication();
  assert.ok(pending, 'intent persisted');
  assert.equal(pending.expectedRevision, 3, 'the intent stores the fingerprint-bound original revision');
  serverRevision = 8; // meanwhile the profile moved on (web-side confirm)
  await career.refreshCareer();
  assert.equal(career.careerDesk().snapshot.revision, 8, 'desk advanced before the resend');
  postFails = false;
  const receipt = await career.retryPendingApplication();
  assert.equal(receipt.applicationId, 'app-1');
  const bodies = careerCall('/applications').filter(c => new URL(c.options.url).pathname === '/api/v1/career/applications').map(c => c.options.data);
  assert.equal(bodies.length, 2, 'one failed attempt plus one safe resend');
  assert.equal(bodies[1].expectedRevision, 3, 'the resend replays the original revision so the server fingerprint matches');
  assert.equal(bodies[1].requestId, bodies[0].requestId, 'the resend replays the original request id');
  assert.equal(career.pendingApplication(), null, 'a successful resend clears the intent');
});

test('D2b: an edit-intent safe resend replays the original edit body, never a confirm (F2)', async () => {
  let postFails = true; let serverRevision = 3;
  const editedBody = { sections: [{ heading: '教育背景', content: '本科（评审轮）', claims: [{ claimId: 'c1', text: '本科学历', needsReview: false }] }, { heading: '技能', content: 'TypeScript', claims: [] }] };
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/materials': call => { if (postFails) stub.fail(call, 'request:fail network'); else stub.succeed(call, { data: materialReceipt({ body: editedBody }) }); },
    'GET /api/v1/career/materials/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  const failed = await career.editMaterial({ materialId: 'mat-1', body: editedBody }).catch(error => error);
  assert.equal(errorCode(failed), 'outcome_unknown');
  const pending = career.pendingMaterialWrite();
  assert.equal(pending.input.op, 'edit', 'the intent records which write kind it was');
  assert.equal(pending.expectedRevision, 3);
  serverRevision = 9;
  await career.refreshCareer();
  postFails = false;
  const receipt = await career.retryPendingMaterial();
  assert.equal(receipt.materialId, 'mat-1');
  const editPosts = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials' && (c.options.method ?? 'GET') === 'POST').map(c => c.options.data);
  assert.equal(editPosts.length, 2, 'one failed edit plus one safe resend to the same endpoint');
  assert.deepEqual(editPosts[1], { requestId: editPosts[0].requestId, materialId: 'mat-1', body: editedBody, expectedRevision: 3 }, 'the resend replays the original edit body and revision byte-for-byte');
  const confirmPosts = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials/confirm');
  assert.equal(confirmPosts.length, 0, 'an edit intent must never be resent as a confirm');
  assert.equal(career.pendingMaterialWrite(), null);
});

test('D3b: a submission safe resend replays the original declared body and revision (F3)', async () => {
  let postFails = true; let serverRevision = 3;
  await freshLogin({
    'GET /api/v1/career/open': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'GET /api/v1/career/list': call => stub.succeed(call, { data: { revision: serverRevision, facts: [], proposals: [] } }),
    'POST /api/v1/career/applications/app-1/submissions': call => { if (postFails) stub.fail(call, 'request:fail timeout'); else stub.succeed(call, { data: submission() }); },
    'GET /api/v1/career/submissions/receipt': call => stub.succeed(call, { statusCode: 404, data: { error: { code: 'not_found', message: 'no receipt yet' } } }),
  });
  await career.loadCareer();
  const failed = await career.recordSubmission({ applicationId: 'app-1', channel: 'web', materialId: 'mat-1', exportId: 'exp-1', versionUnknown: false, note: '官网已投' }).catch(error => error);
  assert.equal(errorCode(failed), 'outcome_unknown');
  serverRevision = 6;
  await career.refreshCareer();
  postFails = false;
  const receipt = await career.retryPendingSubmission();
  assert.equal(receipt.submissionId, 'sub-1');
  const bodies = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/applications/app-1/submissions').map(c => c.options.data);
  assert.equal(bodies.length, 2);
  assert.equal(bodies[1].expectedRevision, 3, 'the resend replays the original revision');
  assert.deepEqual(bodies[1], bodies[0], 'the resent body is byte-for-byte the original one');
});

test('M1: the editable body round-trips every section and claim — nothing is silently dropped (F4)', () => {
  const webBody = { sections: [
    { heading: '教育背景', content: '本科，2026 届毕业。', claims: [{ claimId: 'c1', text: '本科学历', needsReview: false }] },
    { heading: '技能', content: 'TypeScript、分布式系统。', claims: [{ claimId: 'c2', text: '技能主张', needsReview: true, reviewNote: '待核对' }] },
  ] };
  assert.deepEqual(career.bodyFromEditable(career.editableFromBody(webBody)), webBody, 'a read-then-save round-trip preserves sections and claims verbatim');
  const appended = career.editableFromBody(webBody);
  appended.push({ heading: ' 项目经历 ', content: '小程序求职工作台。', claims: [] });
  const next = career.bodyFromEditable(appended);
  assert.equal(next.sections.length, 3, 'a new section appends instead of replacing the body');
  assert.deepEqual(next.sections[0].claims, webBody.sections[0].claims, 'existing claims ride along untouched');
  assert.equal(next.sections[2].heading, '项目经历', 'headings are trimmed but preserved');
  assert.throws(() => career.bodyFromEditable([{ heading: '  ', content: '   ', claims: [] }]), error => error.code === 'material_body_empty');
});

test('T1: a declared time is parsed strictly — invalid input is never silently dropped (F5)', () => {
  assert.deepEqual(career.parseDeclaredOccurredAt('  '), { status: 'empty' }, 'blank leaves the server to stamp the confirmation time');
  const ok = career.parseDeclaredOccurredAt('2026-09-25 20:00');
  assert.equal(ok.status, 'ok');
  assert.ok(!Number.isNaN(Date.parse(ok.iso)), 'the iso form parses everywhere');
  assert.equal(career.parseDeclaredOccurredAt('2026-09-25T20:30:15').status, 'ok', 'the T form with seconds is accepted');
  assert.equal(career.parseDeclaredOccurredAt('2026-02-31 20:00').status, 'invalid', 'rolled-over dates are rejected, not normalized');
  assert.equal(career.parseDeclaredOccurredAt('不是时间').status, 'invalid');
  assert.equal(career.parseDeclaredOccurredAt('2026-09-25').status, 'invalid', 'date-only is rejected: a claimed time needs minutes');
});
