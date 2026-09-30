import { client, auth } from './runtime.ts';
import { record, text } from './views.ts';
import { CareerDesk } from '../../../../packages/career-core/src/desk.ts';
import type { CareerRemote } from '../../../../packages/career-core/src/desk.ts';
import type { CareerView, CareerReceipt, CareerChangeSet, CareerAction, CareerUpload, CareerDocumentSource, OpportunityReceipt, OpportunityEvidence, EvaluationReceipt, Evaluation } from '../../../../packages/career-core/src/contracts.ts';
import { decodeCareerUpload, decodeCareerSources, decodeOpportunityReceipt, decodeEvaluationReceipt, decodeEvaluation } from '../../../../packages/career-core/src/contracts.ts';
import { decodeOpportunityEvidencePage, decodeApplicationReceipt, decodeMaterialReceipt, decodeMaterialView, decodeMaterialVersionList, decodeMaterialExportReceipt, decodeMaterialExportList, decodeMaterialExportDownload, decodeSubmissionReceipt, decodeSubmissionList, decodeCareerExportReceipt, decodeCareerDeletionBoundary, decodeCareerDeletionReceipt, decodeProgressReceipt, decodeProgressView, decodePreparationReceipt, decodePreparationList } from '../../../../packages/api-client/src/career.ts';
import type { ApplicationReceipt, MaterialReceipt, MaterialView, MaterialVersionList, MaterialExportReceipt, MaterialExportList, MaterialExportDownload, MaterialExportFormat, SubmissionReceipt, SubmissionList, SubmissionChannel, MaterialBody, CareerExportReceipt, CareerDeletionBoundaryView, CareerDeletionReceipt, ProgressReceipt, ProgressView, ProgressEventType, PreparationReceipt, PreparationList, PreparationFocus } from '../../../../packages/api-client/src/career.ts';
import type { NativeFileSource } from '@weknora/api-client';
import { requestId as newRequestId } from '../core/intent.ts';
import { storage } from '../platform/storage.ts';
import type { ScopeStamp } from '../core/scope.ts';
import { CAREER_STORE_PREFIX, createControlledStore, recoverableWrite, retryRecoverable, readStoredIntent, intentKeyFor, ambiguousOutcome, definiteLocalFailure, decodeAs, abandonRecoverable, isRecoveryActive, withActiveRecovery, type StoredIntent } from './career-intent.ts';
import { type SharedImportDraft, openExportedDocument, EXPORT_GRANT_EXPIRED, type ExportDownloadGrant, type ExportOpenRecord } from '../adapters/career-platform.ts';

export { prepareSharedImport, careerPlatform, CAREER_STORE_PREFIX } from '../adapters/career-platform.ts';
export type { SharedImportDraft, SharedEntry } from '../adapters/career-platform.ts';

// T24 小程序 Career service：消费冻结的服务端合同（Career open/list/act/changes/receipt、
// searches 三路由、opportunities import/evidence、sources upload）。微信身份只关联既有
// 档案：本端没有任何“创建档案”调用，open 由服务端按认证 scope 派生唯一档案。

let desk: CareerDesk | undefined;
/** 测试隔离用：丢弃共享 desk 单例。 */
export function resetCareerDesk(): void { desk = undefined; }
const deskPendingStore = createControlledStore();
function remote(): CareerRemote {
  return {
    open: async signal => record(await client.request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) as CareerView,
    list: async signal => record(await client.request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) as CareerView,
    changes: async (since, signal) => record(await client.request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) })) as CareerChangeSet,
    act: async (action, signal) => record(await client.request({ method: 'POST', path: '/api/v1/career/act', body: action, ...(signal ? { signal } : {}) })) as CareerReceipt,
    receipt: async (requestId, signal) => record(await client.request({ method: 'GET', path: `/api/v1/career/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) })) as CareerReceipt,
  };
}
export function careerDesk(): CareerDesk { if (!desk) desk = new CareerDesk(remote(), deskPendingStore, scope => intentKeyFor('desk-action', { ...auth.scope.capture(), userId: scope.userId, tenantId: scope.tenantId })); return desk; }
const revision = (): number => careerDesk().snapshot?.revision ?? 0;

/** 把共享 desk 绑定到当前认证身份（同一用户/空间 = 同一份档案；切换即失效旧读）。 */
export function activateCareerScope(): void {
  const snap = auth.snapshot();
  careerDesk().activate(snap.userId, snap.tenantId);
}
/** 无既有档案时引导建档（而不是新建平行档案）的判据。 */
export function needsOnboarding(view?: CareerView): boolean { return !view || (view.facts.length === 0 && view.proposals.length === 0); }
export async function loadCareer(): Promise<CareerView | undefined> { activateCareerScope(); return careerDesk().open(); }
export async function refreshCareer(): Promise<CareerView | undefined> { activateCareerScope(); return careerDesk().refresh(); }
/** 增量同步：同一用户在 Web 修改档案后，小程序侧经 changes 合并可见。 */
export async function syncFromWeb(): Promise<CareerChangeSet | undefined> { return careerDesk().syncChanges(); }

// source.kind 必须落在服务端冻结白名单内（handler Act：propose/confirm→manual|user，
// confirm_proposal/dismiss→user_confirmation|user；客户端不得声称 parser 来源）。
const weappSource = { kind: 'user', label: '微信小程序' } as const;
export async function proposeFact(key: string, value: string): Promise<CareerReceipt | undefined> {
  return careerDesk().mutate({ action: 'propose', key, value, source: weappSource, requestId: newRequestId(), expectedRevision: revision() });
}
export async function confirmProposal(proposalId: string): Promise<CareerReceipt | undefined> {
  return careerDesk().mutate({ action: 'confirm_proposal', proposalId, source: weappSource, requestId: newRequestId(), expectedRevision: revision() });
}
export async function dismissProposal(proposalId: string): Promise<CareerReceipt | undefined> {
  return careerDesk().mutate({ action: 'dismiss', proposalId, source: weappSource, requestId: newRequestId(), expectedRevision: revision() });
}
export function pendingAction(): CareerAction | undefined { return careerDesk().pendingAction; }
export async function reconcilePending(): Promise<CareerReceipt | undefined> {
  const pending = careerDesk().pendingAction;
  if (!pending) throw new Error('没有待对账的操作');
  return careerDesk().reconcile(pending.requestId);
}
export async function retryPending(action: CareerAction): Promise<CareerReceipt | undefined> { return careerDesk().retryUnknown(action); }

export interface SearchRow { resultId: string; sourceId: string; link: string; checkedAt: string; qualification: string; uncertainty: string }
export interface SearchCoverageSource { sourceId: string; label: string; available: boolean; accessMethods: string[]; cities: string[]; failureCode?: string }
export interface SearchOutcome {
  searchId: string; requestId: string; status: 'completed' | 'failed'; query: string;
  rows: SearchRow[]; sources: SearchCoverageSource[]; scopeNotes: string[];
  failureCode?: string; checkedAt: string; hasNoVettedSources: boolean;
}
function decodeSearchOutcome(value: unknown): SearchOutcome {
  const r = record(value);
  if (r.kind !== 'search_once') throw new Error('无效的搜索回执');
  if (r.status !== 'completed' && r.status !== 'failed') throw new Error('无效的搜索状态');
  if (!text(r.requestId) || !text(r.query) || !text(r.checkedAt) || !Number.isFinite(Date.parse(text(r.checkedAt)))) throw new Error('搜索回执缺少请求、查询或有效检查时间');
  if (r.status === 'completed' && (!r.coverage || typeof r.coverage !== 'object' || !Array.isArray((r.coverage as Record<string, unknown>).sources) || !Array.isArray(r.results) || !Array.isArray(r.scopeNotes))) throw new Error('已完成搜索回执缺少覆盖、来源、结果或范围说明');
  const searchId = text(r.searchId); if (!searchId) throw new Error('无效的搜索 ID');
  const coverage = r.coverage === undefined ? {} : record(r.coverage);
  const sourcesRaw = Array.isArray(coverage.sources) ? coverage.sources : [];
  const sources: SearchCoverageSource[] = sourcesRaw.map(raw => {
    const s = record(raw);
    if (!text(s.sourceId) || typeof s.available !== 'boolean' || !Array.isArray(s.accessMethods) || !s.accessMethods.every(v => typeof v === 'string') || !Array.isArray(s.cities) || !s.cities.every(v => typeof v === 'string')) throw new Error('搜索来源数据无效');
    return {
      sourceId: text(s.sourceId), label: text(s.label, text(s.sourceId, '未命名来源')), available: s.available === true,
      accessMethods: Array.isArray(s.accessMethods) ? s.accessMethods.filter((v): v is string => typeof v === 'string') : [],
      cities: Array.isArray(s.cities) ? s.cities.filter((v): v is string => typeof v === 'string') : [],
      ...(typeof s.failureCode === 'string' && s.failureCode ? { failureCode: s.failureCode } : {}),
    };
  });
  const rowsRaw = Array.isArray(r.results) ? r.results : [];
  const rows: SearchRow[] = rowsRaw.map(raw => {
    const row = record(raw);
    if (!text(row.resultId) || !text(row.sourceId) || !text(row.link) || !text(row.checkedAt) || !Number.isFinite(Date.parse(text(row.checkedAt))) || !['qualified', 'needs_review', 'not_qualified'].includes(text(row.qualification)) || typeof row.uncertainty !== 'string') throw new Error('搜索结果证据字段无效');
    return {
      resultId: text(row.resultId), sourceId: text(row.sourceId), link: text(row.link),
      checkedAt: text(row.checkedAt), qualification: text(row.qualification), uncertainty: text(row.uncertainty),
    };
  });
  const scopeNotes = Array.isArray(r.scopeNotes) ? r.scopeNotes.filter((v): v is string => typeof v === 'string') : [];
  if (r.status === 'completed' && (!Array.isArray(r.scopeNotes) || !r.scopeNotes.every((note): note is string => typeof note === 'string'))) throw new Error('搜索范围说明无效');
  return {
    searchId, requestId: text(r.requestId), status: r.status, query: text(r.query),
    rows, sources, scopeNotes,
    ...(typeof r.failureCode === 'string' && r.failureCode ? { failureCode: r.failureCode } : {}),
    checkedAt: text(r.checkedAt), hasNoVettedSources: sources.length === 0,
  };
}

interface PendingSearchIntent { requestId: string; query: string; expectedRevision?: number }
const store = createControlledStore();
const searchKey = (stamp: ScopeStamp = auth.scope.capture()): string => intentKeyFor('search', stamp);
export function pendingSearch(): PendingSearchIntent | null {
  const value = store.read(searchKey());
  if (!value || typeof value !== 'object') return null;
  const r = value as { requestId?: unknown; query?: unknown; input?: { query?: unknown } | null; expectedRevision?: unknown };
  // OCR r3 ocr3-031：intent 收敛到 recoverableWrite 统一形状（query 在 input 内）；顶层
  // query 兼容历史 T24 形状。
  const query = typeof r.query === 'string' ? r.query : (typeof r.input?.query === 'string' ? r.input.query : undefined);
  if (typeof r.requestId !== 'string' || typeof query !== 'string') return null;
  const expectedRevision = typeof r.expectedRevision === 'number' && Number.isSafeInteger(r.expectedRevision) && r.expectedRevision >= 0 ? r.expectedRevision : undefined;
  return { requestId: r.requestId, query, ...(expectedRevision !== undefined ? { expectedRevision } : {}) };
}
function unknownOutcome(requestId: string, query: string, cause: unknown): Error {
  return Object.assign(new Error('搜索结果未知：请用原请求恢复后再试', { cause }), { code: 'outcome_unknown', requestId, query });
}
/** 一次性找岗：POST /searches 的冻结合同（requestId/query/expectedRevision，禁多余字段）。
 *  OCR r3 ocr3-031：写入形状收敛到两域共用的 recoverableWrite（stamp 守卫/歧义判据/
 *  expectedRevision 持久化/未对账封锁/解码守卫一份实现），消除与 OCR2-037 修复并存的
 *  手写副本——守卫口径再变时只改一处。intent 形状随之变为
 *  {requestId,input:{query},expectedRevision}，pendingSearch 兼容读取新旧两种形状。 */
export async function searchOnce(query: string): Promise<SearchOutcome> {
  const trimmed = query.trim();
  if (!trimmed) throw new Error('请先输入想找的岗位或要求');
  try {
    return await recoverableWrite<SearchOutcome>(store, {
      kind: 'search', describe: '搜索', input: { query: trimmed }, expected: revision(),
      send: async (id, expected) => decodeAs(decodeSearchOutcome, await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: trimmed, expectedRevision: expected } })),
    });
  } catch (error) {
    // 保持搜索域错误形状（附带 query）——recoverableWrite 的统一包装只带 requestId。
    if ((error as { code?: unknown })?.code === 'outcome_unknown') throw unknownOutcome((error as { requestId: string }).requestId, trimmed, (error as { cause?: unknown }).cause);
    throw error;
  }
}
/** 未知结果对账：用原 request id 读取服务端持久回执；成功即清除本端 intent。 */
export async function searchReceipt(): Promise<SearchOutcome | undefined> {
  const pending = pendingSearch();
  if (!pending) return undefined;
  const outcome = decodeSearchOutcome(await client.request({ method: 'GET', path: `/api/v1/career/searches/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
  store.remove(searchKey());
  return outcome;
}
/** 对账确认服务端尚无该请求的回执（404/not_found，handler 冻结映射）：请求可能未送达，
 * intent 保留；UI 依此呈现可操作恢复态（同 request id 安全重发，服务端幂等不重复执行）。 */
export function isReceiptMissing(error: unknown): boolean {
  const code = (error as { code?: unknown } | null | undefined)?.code;
  if (code === 'not_found') return true;
  return (error as { status?: unknown } | null | undefined)?.status === 404;
}
/** 对账确认服务端无记录（404）后的安全重试：同 request id 幂等重发——expectedRevision
 *  逐字节重放 intent 持久化的原值（修订前进后用当前值重发必然 idempotency_conflict，
 *  OCR high-8）；历史 intent 无修订记录时退回当前值兜底（与 retryIntent 同构）。 */
export async function retryPendingSearch(): Promise<SearchOutcome> {
  const pending = pendingSearch();
  if (!pending) throw new Error('没有待恢复的搜索');
  const expected = pending.expectedRevision ?? revision();
  const stamp = auth.scope.capture();
  try {
    const outcome = decodeAs(decodeSearchOutcome, await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: pending.requestId, query: pending.query, expectedRevision: expected } }));
    store.remove(searchKey());
    return outcome;
  } catch (error) {
    if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (ambiguousOutcome(error)) throw unknownOutcome(pending.requestId, pending.query, error);
    throw error;
  }
}

/** 上传已有简历：抽取事实以待确认 proposal 返回（未确认不生效）。OCR r3 ocr3-032：上传
 *  与其它写入同守「requestId + 对账」纪律——服务端 Upload 按 requestId 幂等（intent-hash
 *  指纹），超时后换新 requestId 重传会为同一简历创建全新 claim（重复存储/解析/重复 proposal）。
 *  结果未知时 intent 持久化；重选同一文件即可通过指纹校验，同编号安全重发不重复建 claim。 */
export interface UploadResumeIntent { fileName: string }
async function sendResumeUpload(file: NativeFileSource, id: string, expected: number): Promise<CareerUpload> {
  return decodeAs(decodeCareerUpload, await client.request({
    method: 'POST', path: '/api/v1/career/sources/upload', nativeFile: file,
    multipartFields: { requestId: id, expectedRevision: String(expected) },
  }));
}
export async function uploadResume(file: NativeFileSource): Promise<CareerUpload> {
  return recoverableWrite<CareerUpload>(store, { kind: 'upload', describe: '简历上传', input: { fileName: file.name }, expected: revision(), send: (id, expected) => sendResumeUpload(file, id, expected) });
}
export function pendingUpload(): StoredIntent<UploadResumeIntent> | null { return readIntent<UploadResumeIntent>(intentKey('upload')); }
/** 上传无按 requestId 的回执路由：恢复出口 = 重选同一文件同编号安全重发，或显式放弃。 */
export async function retryPendingUpload(file: NativeFileSource): Promise<CareerUpload> {
  const pending = pendingUpload();
  if (!pending) throw new Error('没有待恢复的简历上传');
  return retryRecoverable<CareerUpload>(store, 'upload', '简历上传', async (id, expected) => sendResumeUpload(file, id, expected), revision);
}
export function abandonPendingUpload(): void { abandonRecoverable(store, 'upload'); }
export async function listSources(): Promise<CareerDocumentSource[]> {
  return decodeCareerSources(await client.request({ method: 'GET', path: '/api/v1/career/sources' }));
}

/** 分享导入第二阶段：核对预览后才提交，提交原文而非摘录。 */
export async function confirmSharedImport(draft: SharedImportDraft): Promise<OpportunityReceipt> {
  const input = { rawText: draft.rawText, ...(draft.sourceLabel ? { sourceLabel: draft.sourceLabel } : {}) };
  const send = async (id: string): Promise<OpportunityReceipt> => decodeAs(decodeOpportunityReceipt, await client.request({ method: 'POST', path: '/api/v1/career/opportunities/import', body: { requestId: id, ...input } }));
  return recoverableWrite<OpportunityReceipt>(store, { kind: 'opportunity-import', describe: '职位导入', input, expected: revision(), reuseId: draft.requestId, send: async id => send(id) });
}
export function pendingSharedImport(): StoredIntent<{ rawText: string; sourceLabel?: string }> | null { return readIntent(intentKey('opportunity-import')); }
export async function reconcileSharedImport(): Promise<OpportunityReceipt> { return reconcileIntent('opportunity-import', '职位导入', async id => decodeOpportunityReceipt(await client.request({ method: 'GET', path: `/api/v1/career/opportunities/receipt?requestId=${encodeURIComponent(id)}` }))); }
export async function retrySharedImport(): Promise<OpportunityReceipt> {
  const pending = pendingSharedImport(); if (!pending) throw new Error('没有待恢复的职位导入');
  return retryIntent('opportunity-import', '职位导入', async (id) => decodeAs(decodeOpportunityReceipt, await client.request({ method: 'POST', path: '/api/v1/career/opportunities/import', body: { requestId: id, ...pending.input } })));
}
/** 导入后查看解析出的 JD 事实（needs_review 也能看到如实来源状态）。 */
export async function opportunityEvidence(opportunityId: string, snapshotId: string): Promise<OpportunityEvidence> {
  if (!opportunityId.trim() || !snapshotId.trim()) throw new Error('缺少岗位快照信息');
  return decodeOpportunityEvidencePage(await client.request({ method: 'GET', path: `/api/v1/career/opportunities/${encodeURIComponent(opportunityId)}?snapshotId=${encodeURIComponent(snapshotId)}` }));
}

// ---- T26 申请、材料、导出与本人投递：与 Web 同源同版本（同一批 api-client 解码器，
// 同一冻结合同）。写入一律 requestId + expectedRevision；结果未知用原 requestId 对账，
// 空间切换的旧响应绝不当成新事实落地。投递确认只记录用户声明，零自动提交零外发。 ----

/** 评估当前档案对固定岗位快照的资格（申请创建前的一步；三值结论如实返回）。 */
export async function evaluateOpportunity(opportunityId: string, snapshotId: string): Promise<Evaluation> {
  if (!opportunityId.trim() || !snapshotId.trim()) throw new Error('缺少岗位快照信息');
  const input = { opportunityId: opportunityId.trim(), snapshotId: snapshotId.trim() };
  return writeRecoverable<Evaluation>('evaluation', '资格评估', input, async id => {
    const receipt = decodeAs(decodeEvaluationReceipt, await client.request({ method: 'POST', path: '/api/v1/career/evaluations', body: { requestId: id, ...input } }));
    return decodeAs(decodeEvaluation, await client.request({ method: 'GET', path: `/api/v1/career/evaluations/${encodeURIComponent(receipt.evaluationId)}` }));
  });
}
export function pendingEvaluation(): StoredIntent<{ opportunityId: string; snapshotId: string }> | null { return readIntent(intentKey('evaluation')); }
export async function reconcilePendingEvaluation(): Promise<Evaluation> { return reconcileIntent('evaluation', '资格评估', async id => {
  const receipt = decodeEvaluationReceipt(await client.request({ method: 'GET', path: `/api/v1/career/evaluations/receipt?requestId=${encodeURIComponent(id)}` }));
  return decodeEvaluation(await client.request({ method: 'GET', path: `/api/v1/career/evaluations/${encodeURIComponent(receipt.evaluationId)}` }));
}); }
export async function retryPendingEvaluation(): Promise<Evaluation> { const pending = pendingEvaluation(); if (!pending) throw new Error('没有待恢复的资格评估'); return retryIntent('evaluation', '资格评估', async id => {
  const receipt = decodeAs(decodeEvaluationReceipt, await client.request({ method: 'POST', path: '/api/v1/career/evaluations', body: { requestId: id, ...pending.input } }));
  return decodeAs(decodeEvaluation, await client.request({ method: 'GET', path: `/api/v1/career/evaluations/${encodeURIComponent(receipt.evaluationId)}` }));
}); }

export interface ApplicationIntentInput { opportunityId: string; snapshotId: string; evaluationId: string; batchIdentity: string; continueDespiteHardFailure: boolean }
export type { StoredIntent } from './career-intent.ts';
// 恢复链机制（歧义判据/作用域 stamp 守卫/expectedRevision 持久化/安全重放）已抽到
// career-intent.ts，与 T30 适配器共用一份实现（OCR med-39：此前两套副本已漂移出
// high-7/high-8）。这里只保留薄封装：键按当前作用域铸（读取/对账语义）。
const intentKey = (kind: string): string => intentKeyFor(kind, auth.scope.capture());
function readIntent<T>(key: string): StoredIntent<T> | null { return readStoredIntent<T>(store, key); }
/** 写入 + 未知结果恢复。expectedRevision 在发送前捕获并随 intent 持久化：服务端幂等
 *  指纹是全量请求体（含 expectedRevision），安全重发必须逐字节重放原始值——desk 修订
 *  前进后用当前值重发必然 ErrIdempotencyConflict，恢复死路。expectedOverride 供 CAS
 *  域不是档案修订的写入（T17 进展=每应用事件计数）显式传入读取到的域值。 */
async function writeRecoverable<T>(kind: string, describe: string, input: unknown, send: (id: string, expected: number) => Promise<unknown>, expectedOverride?: number): Promise<T> {
  return recoverableWrite<T>(store, { kind, describe, input, expected: expectedOverride ?? revision(), send: async (id, expected) => (await send(id, expected)) as T });
}
async function reconcileIntent<T>(kind: string, describe: string, fetch: (id: string) => Promise<unknown>): Promise<T> {
  const stamp = auth.scope.capture();
  const key = intentKeyFor(kind, stamp);
  const pending = readIntent<Record<string, unknown>>(key);
  if (!pending) throw new Error(`没有待对账的${describe}`);
  return withActiveRecovery(kind, stamp, pending.requestId, async () => {
    const receipt = await fetch(pending.requestId);
    store.remove(key);
    return receipt as T;
  });
}
async function retryIntent<T>(kind: string, describe: string, resend: (id: string, expected: number) => Promise<unknown>, fallbackExpected?: () => number): Promise<T> {
  const stamp = auth.scope.capture();
  const pending = readIntent<Record<string, unknown>>(intentKeyFor(kind, stamp));
  if (!pending) return retryRecoverable<T>(store, kind, describe, async (id, expected) => (await resend(id, expected)) as T, fallbackExpected);
  return withActiveRecovery(kind, stamp, pending.requestId, () =>
    retryRecoverable<T>(store, kind, describe, async (id, expected) => (await resend(id, expected)) as T, fallbackExpected));
}

// —— 显式放弃各 kind 未对账 intent（OCR r3 ocr3-029 统一出口，rule/reminder 先例同语义）：
//    只清本端恢复记录，不动服务端事实——原写入若已落地，以服务端记录为准（回执/列表对账
//    可见）。未对账封锁（unresolved_action）挡住新写入时，这是唯一解除出口。 ——
export function abandonPendingSearch(): void { abandonRecoverable(store, 'search'); }
export function abandonPendingApplication(): void { abandonRecoverable(store, 'application'); }
export function abandonPendingMaterialWrite(): void { abandonRecoverable(store, 'material'); }
export function abandonPendingSubmission(): void { abandonRecoverable(store, 'submission'); }
export function abandonPendingMaterialPublish(): void { abandonRecoverable(store, 'export'); }
export function abandonPendingProgressWrite(): void { abandonRecoverable(store, 'progress'); }
export function abandonPendingPreparationWrite(): void { abandonRecoverable(store, 'preparation'); }
export function abandonPendingSpaceExport(expectedRequestId: string): boolean { return abandonRecoverable(store, 'spaceExport', expectedRequestId); }
export function spaceExportRecoveryActive(requestId: string): boolean { return isRecoveryActive('spaceExport', auth.scope.capture(), requestId); }
export function abandonPendingSpaceDeletion(expectedRequestId: string): boolean { return abandonRecoverable(store, 'spaceDeletion', expectedRequestId); }
export function spaceDeletionRecoveryActive(requestId: string): boolean { return isRecoveryActive('spaceDeletion', auth.scope.capture(), requestId); }

// —— 申请（T14 合同）：一岗一批一申请；硬条件不符必须显式继续 ——
export async function createApplication(input: ApplicationIntentInput): Promise<ApplicationReceipt> {
  if (!input.opportunityId.trim() || !input.snapshotId.trim() || !input.evaluationId.trim() || !input.batchIdentity.trim()) throw new Error('申请缺少岗位、评估或批次信息');
  const batch = input.batchIdentity.trim();
  return writeRecoverable<ApplicationReceipt>('application', '申请创建', input, async (id, expected) =>
    decodeAs(decodeApplicationReceipt, await client.request({ method: 'POST', path: '/api/v1/career/applications', body: { requestId: id, opportunityId: input.opportunityId.trim(), snapshotId: input.snapshotId.trim(), evaluationId: input.evaluationId.trim(), batchIdentity: batch, continueDespiteHardFailure: input.continueDespiteHardFailure, expectedRevision: expected } })));
}
export function pendingApplication(): StoredIntent<ApplicationIntentInput> | null { return readIntent<ApplicationIntentInput>(intentKey('application')); }
export async function reconcilePendingApplication(): Promise<ApplicationReceipt> {
  return reconcileIntent<ApplicationReceipt>('application', '申请', async id =>
    decodeApplicationReceipt(await client.request({ method: 'GET', path: `/api/v1/career/applications/receipt?requestId=${encodeURIComponent(id)}` })));
}
export async function retryPendingApplication(): Promise<ApplicationReceipt> {
  const pending = pendingApplication();
  if (!pending) throw new Error('没有待恢复的申请');
  return retryIntent<ApplicationReceipt>('application', '申请', async (id, expected) =>
    decodeAs(decodeApplicationReceipt, await client.request({ method: 'POST', path: '/api/v1/career/applications', body: { requestId: id, opportunityId: pending.input.opportunityId.trim(), snapshotId: pending.input.snapshotId.trim(), evaluationId: pending.input.evaluationId.trim(), batchIdentity: pending.input.batchIdentity.trim(), continueDespiteHardFailure: pending.input.continueDespiteHardFailure, expectedRevision: expected } })), revision);
}
export async function getApplication(applicationId: string): Promise<ApplicationReceipt> {
  if (!applicationId.trim()) throw new Error('缺少申请编号');
  return decodeApplicationReceipt(await client.request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId.trim())}` }));
}
/** link_state=linking/link_failed 的 Task 关联只能用原 requestId 对账（服务端幂等）。 */
export async function reconcileApplicationLink(requestId: string): Promise<ApplicationReceipt> {
  if (!requestId.trim()) throw new Error('缺少原请求编号');
  return decodeApplicationReceipt(await client.request({ method: 'POST', path: '/api/v1/career/applications/link/reconcile', body: { requestId: requestId.trim() } }));
}

// —— 材料（T15 合同）：同一结构化正文，确认生成不可变新版本，不覆盖旧版 ——
export interface EditMaterialIntent { materialId?: string; opportunityId?: string; snapshotId?: string; body: MaterialBody }
/** 可编辑小节：claims 原样保留（本端不编辑主张，也绝不丢弃 Web 端建立的主张）。 */
export interface EditableMaterialSection { heading: string; content: string; claims: MaterialBody['sections'][number]['claims'] }
/** 读取视图 → 可编辑模型：全部小节与 claims 进入编辑态（Web MaterialPage 同语义）。 */
export function editableFromBody(body: MaterialBody): EditableMaterialSection[] {
  return body.sections.map(section => ({ heading: section.heading, content: section.content, claims: section.claims }));
}
/** 可编辑模型 → 提交正文：往返不丢节不丢主张；空白小节被剔除，全空拒绝提交。 */
export function bodyFromEditable(sections: EditableMaterialSection[]): MaterialBody {
  const usable = sections.filter(section => section.heading.trim() || section.content.trim());
  if (!usable.length) throw Object.assign(new Error('材料正文至少需要一个小节'), { code: 'material_body_empty', recoverable: true });
  return { sections: usable.map(section => ({ heading: section.heading.trim() || '未命名小节', content: section.content, claims: section.claims })) };
}
function materialRequestBody(id: string, intent: EditMaterialIntent, expected: number): Record<string, unknown> {
  if (intent.materialId?.trim()) return { requestId: id, materialId: intent.materialId.trim(), body: intent.body, expectedRevision: expected };
  if (intent.opportunityId?.trim() && intent.snapshotId?.trim()) return { requestId: id, opportunityId: intent.opportunityId.trim(), snapshotId: intent.snapshotId.trim(), body: intent.body, expectedRevision: expected };
  throw new Error('材料编辑缺少岗位证据或材料编号');
}
export async function editMaterial(intent: EditMaterialIntent): Promise<MaterialReceipt> {
  materialRequestBody(newRequestId(), intent, revision()); // 先做参数校验，再进入可恢复写入
  return writeRecoverable<MaterialReceipt>('material', '材料编辑', { op: 'edit' as const, ...intent }, async (id, expected) =>
    decodeAs(decodeMaterialReceipt, await client.request({ method: 'POST', path: '/api/v1/career/materials', body: materialRequestBody(id, intent, expected) })));
}
export async function confirmMaterial(materialId: string): Promise<MaterialReceipt> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return writeRecoverable<MaterialReceipt>('material', '材料确认', { op: 'confirm' as const, materialId: materialId.trim() }, async (id, expected) =>
    decodeAs(decodeMaterialReceipt, await client.request({ method: 'POST', path: '/api/v1/career/materials/confirm', body: { requestId: id, materialId: materialId.trim(), expectedRevision: expected } })));
}
export function pendingMaterialWrite(): StoredIntent<Record<string, unknown>> | null { return readIntent<Record<string, unknown>>(intentKey('material')); }
export async function reconcilePendingMaterial(): Promise<MaterialReceipt> {
  return reconcileIntent<MaterialReceipt>('material', '材料写入', async id =>
    decodeMaterialReceipt(await client.request({ method: 'GET', path: `/api/v1/career/materials/receipt?requestId=${encodeURIComponent(id)}` })));
}
/** 安全重发按 intent 记录的写入种类重放同端点同内容（edit 绝不重放成 confirm）。 */
export async function retryPendingMaterial(): Promise<MaterialReceipt> {
  const pending = pendingMaterialWrite();
  if (!pending) throw new Error('没有待恢复的材料写入');
  const stored = pending.input as { op?: 'edit' | 'confirm'; materialId?: unknown; opportunityId?: unknown; snapshotId?: unknown; body?: unknown };
  return retryIntent<MaterialReceipt>('material', '材料写入', async (id, expected) => {
    if (stored.op === 'edit') {
      const intent: EditMaterialIntent = { ...(stored.materialId !== undefined ? { materialId: String(stored.materialId) } : {}), ...(stored.opportunityId !== undefined ? { opportunityId: String(stored.opportunityId) } : {}), ...(stored.snapshotId !== undefined ? { snapshotId: String(stored.snapshotId) } : {}), body: stored.body as MaterialBody };
      return decodeAs(decodeMaterialReceipt, await client.request({ method: 'POST', path: '/api/v1/career/materials', body: materialRequestBody(id, intent, expected) }));
    }
    return decodeAs(decodeMaterialReceipt, await client.request({ method: 'POST', path: '/api/v1/career/materials/confirm', body: { requestId: id, materialId: String(stored.materialId ?? ''), expectedRevision: expected } }));
  }, revision);
}
export async function material(materialId: string): Promise<MaterialView> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return decodeMaterialView(await client.request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}` }));
}
export async function materialVersions(materialId: string): Promise<MaterialVersionList> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return decodeMaterialVersionList(await client.request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}/versions` }));
}

// —— 导出（T16 合同）：发布双格式；下载走 signed-url → 认证兑付；过期授权重取 ——
export async function publishMaterial(materialId: string, version: number): Promise<MaterialExportReceipt> {
  if (!materialId.trim() || !Number.isSafeInteger(version) || version <= 0) throw new Error('发布需要材料编号与确认版本');
  return writeRecoverable<MaterialExportReceipt>('export', '材料发布', { materialId: materialId.trim(), version }, async (id, expected) =>
    decodeAs(decodeMaterialExportReceipt, await client.request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}/exports`, body: { requestId: id, version, expectedRevision: expected } })));
}
export async function listMaterialExports(materialId: string): Promise<MaterialExportList> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return decodeMaterialExportList(await client.request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}/exports` }));
}

// —— 材料发布恢复链（OCR2-001）：publishMaterial 的 writeRecoverable('export',…) 会把
// intent 落到 wk:career:export:<scope>，此前没有任何读取器——发布结果未知即成死数据。
// 服务端幂等合同：PublishMaterial 按全量请求体指纹（requestId+materialId+version+
// expectedRevision）replay，同编号重发绝不重复执行，也不产生重复导出记录。
export interface PublishMaterialIntent { materialId: string; version: number }
export function pendingMaterialPublish(): StoredIntent<PublishMaterialIntent> | null { return readIntent<PublishMaterialIntent>(intentKey('export')); }
/** 对账：服务端没有材料发布的专用 receipt 路由——以导出列表按原 requestId 匹配（列表
 *  条目与回执同构，均携带 requestId）。匹配到即该编号的发布已落库（含已撤销态），清
 *  intent 返回回执；匹配不到视同回执缺失（可能未送达），intent 保留由上层引导安全重发。 */
export async function reconcilePendingMaterialPublish(): Promise<MaterialExportReceipt | undefined> {
  const pending = pendingMaterialPublish();
  if (!pending) throw new Error('没有待对账的材料发布');
  const receipt = (await listMaterialExports(pending.input.materialId)).exports.find(entry => entry.requestId === pending.requestId);
  if (receipt) store.remove(intentKey('export'));
  return receipt;
}
/** 安全重发：同 requestId + 原 version 重放（服务端幂等 replay，不重复发布）；成功清 intent。 */
export async function retryPendingMaterialPublish(): Promise<MaterialExportReceipt> {
  const pending = pendingMaterialPublish();
  if (!pending) throw new Error('没有待恢复的材料发布');
  return retryIntent<MaterialExportReceipt>('export', '材料发布', async (id, expected) =>
    decodeAs(decodeMaterialExportReceipt, await client.request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(pending.input.materialId)}/exports`, body: { requestId: id, version: pending.input.version, expectedRevision: expected } })), revision);
}
const EXPORT_GRANT_TTL_SECONDS = 300;
async function issueExportGrant(materialId: string, exportId: string, format: MaterialExportFormat): Promise<MaterialExportDownload> {
  return decodeMaterialExportDownload(await client.request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/exports/${encodeURIComponent(exportId)}/signed-url`, body: { format, ttlSeconds: EXPORT_GRANT_TTL_SECONDS } }));
}
export interface MaterialExportOpenOutcome { grant: MaterialExportDownload; check: ExportOpenRecord }
/** 签发授权 → 认证兑付下载（digest 校验+打开）。授权过期（typed）自动重取一次再试。 */
export async function openMaterialExport(materialId: string, exportId: string, format: MaterialExportFormat): Promise<MaterialExportOpenOutcome> {
  if (!materialId.trim() || !exportId.trim()) throw new Error('下载需要材料与导出编号');
  const grant = await issueExportGrant(materialId.trim(), exportId.trim(), format);
  try {
    return { grant, check: await openExportedDocument(grant as ExportDownloadGrant) };
  } catch (error) {
    if ((error as { code?: unknown })?.code === EXPORT_GRANT_EXPIRED) {
      const refreshed = await issueExportGrant(materialId.trim(), exportId.trim(), format);
      return { grant: refreshed, check: await openExportedDocument(refreshed as ExportDownloadGrant) };
    }
    throw error;
  }
}

// —— 本人投递（T18 合同）：只记录用户声明的渠道/时间/版本绑定或显式未知 ——

/** 版本未选择与显式未知是两回事：一申请一记录不可逆，未选择必须阻断而非推断。 */
export const SUBMISSION_VERSION_UNKNOWN_CHOICE = '__unknown__';
export type SubmissionVersionChoice = { status: 'unselected' } | { status: 'unknown' } | { status: 'bound'; materialId: string; exportId: string };
export function resolveSubmissionVersion(choice: string, submittable: { exportId: string; materialId: string }[]): SubmissionVersionChoice {
  if (!choice) return { status: 'unselected' };
  if (choice === SUBMISSION_VERSION_UNKNOWN_CHOICE) return { status: 'unknown' };
  const found = submittable.find(entry => entry.exportId === choice);
  return found ? { status: 'bound', materialId: found.materialId, exportId: found.exportId } : { status: 'unselected' };
}

/** 声明时间严格解析（平台无关）：只接受 YYYY-MM-DD[ ]HH:mm[:ss] 本地时间；空=交给
 * 服务端盖确认时间；格式不符/日期滚动一律 invalid——绝不静默丢弃用户的声明。 */
const DECLARED_TIME_PATTERN = /^(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2})(?::(\d{2}))?$/;
export type DeclaredTime = { status: 'empty' } | { status: 'invalid' } | { status: 'ok'; iso: string };
export function parseDeclaredOccurredAt(value: string): DeclaredTime {
  const trimmed = value.trim();
  if (!trimmed) return { status: 'empty' };
  const match = trimmed.match(DECLARED_TIME_PATTERN);
  if (!match) return { status: 'invalid' };
  const [, year, month, day, hour, minute, second] = match;
  const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), second ? Number(second) : 0);
  // 分量回读校验：iOS JSC 与 V8 都会把 2026-02-31 滚成 3 月，必须显式拒绝。
  if (date.getFullYear() !== Number(year) || date.getMonth() !== Number(month) - 1 || date.getDate() !== Number(day) || date.getHours() !== Number(hour) || date.getMinutes() !== Number(minute)) return { status: 'invalid' };
  return { status: 'ok', iso: date.toISOString() };
}

export interface RecordSubmissionIntent { applicationId: string; channel: SubmissionChannel; occurredAt?: string; materialId?: string; exportId?: string; versionUnknown: boolean; note?: string }
function submissionRequestBody(id: string, input: RecordSubmissionIntent, expected: number): Record<string, unknown> {
  if (!input.applicationId.trim() || !['email', 'web', 'other'].includes(input.channel)) throw new Error('投递确认缺少申请或渠道');
  // 显式未知独占：不携带任何版本绑定；确认版本则必须双标识齐全。
  if (input.versionUnknown && (input.materialId !== undefined || input.exportId !== undefined)) throw new Error('版本未知声明不能同时携带版本绑定');
  if (!input.versionUnknown && (!input.materialId?.trim() || !input.exportId?.trim())) throw new Error('投递确认需绑定可提交版本，或显式声明版本未知');
  return {
    requestId: id, applicationId: input.applicationId.trim(), channel: input.channel,
    ...(input.occurredAt ? { occurredAt: input.occurredAt } : {}),
    ...(input.materialId?.trim() ? { materialId: input.materialId.trim() } : {}),
    ...(input.exportId?.trim() ? { exportId: input.exportId.trim() } : {}),
    versionUnknown: input.versionUnknown,
    ...(input.note?.trim() ? { note: input.note.trim() } : {}),
    expectedRevision: expected,
  };
}
export async function recordSubmission(input: RecordSubmissionIntent): Promise<SubmissionReceipt> {
  submissionRequestBody(newRequestId(), input, revision()); // 先校验，再进入可恢复写入
  return writeRecoverable<SubmissionReceipt>('submission', '投递确认', input, async (id, expected) =>
    decodeAs(decodeSubmissionReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId.trim())}/submissions`, body: submissionRequestBody(id, input, expected) })));
}
export function pendingSubmission(): StoredIntent<RecordSubmissionIntent> | null { return readIntent<RecordSubmissionIntent>(intentKey('submission')); }
export async function reconcilePendingSubmission(): Promise<SubmissionReceipt> {
  return reconcileIntent<SubmissionReceipt>('submission', '投递确认', async id =>
    decodeSubmissionReceipt(await client.request({ method: 'GET', path: `/api/v1/career/submissions/receipt?requestId=${encodeURIComponent(id)}` })));
}
export async function retryPendingSubmission(): Promise<SubmissionReceipt> {
  const pending = pendingSubmission();
  if (!pending) throw new Error('没有待恢复的投递确认');
  return retryIntent<SubmissionReceipt>('submission', '投递确认', async (id, expected) =>
    decodeAs(decodeSubmissionReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(pending.input.applicationId)}/submissions`, body: submissionRequestBody(id, pending.input, expected) })), revision);
}
export async function listSubmissions(applicationId: string): Promise<SubmissionList> {
  if (!applicationId.trim()) throw new Error('缺少申请编号');
  return decodeSubmissionList(await client.request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId.trim())}/submissions` }));
}

// ---- T28 申请进展时间线与按需准备（T17/T19 合同，与 Web 同源同版本）----
// 进展 append-only：纠错=追加引用事件，原事件保留可追溯；expectedRevision 域=每申请
// 事件计数（GET progress 的 revision 字段，首事件=0；progress.go CAS），不是档案修订。
// 准备锚定实际投递版：未确认投递返回 typed 提示态（409 preparation_version_unknown），
// 绝不静默改用最新材料版本；草稿物化 materials 域可修订（走上方 editMaterial 同一链）。

// —— 进展时间线（T17）：读取、录入、纠错、未知对账 ——
export interface ProgressEventInput { applicationId: string; eventType: ProgressEventType; note?: string; occurredAt?: string }
export interface CorrectProgressInputMin extends ProgressEventInput { correctsEventId: string }
function progressRequestBody(id: string, input: ProgressEventInput, expected: number): Record<string, unknown> {
  if (!input.applicationId.trim() || !input.eventType) throw new Error('进展事件缺少申请或事件类型');
  // 服务端把 HTTP 来源钉死为 manual（progressClientSource 白名单），客户端只声明该值。
  const body: Record<string, unknown> = { requestId: id, applicationId: input.applicationId.trim(), eventType: input.eventType, source: { kind: 'manual' }, expectedRevision: expected };
  const note = input.note?.trim();
  if (note) body.note = note;
  if (input.occurredAt) body.occurredAt = input.occurredAt;
  if ('correctsEventId' in input && input.correctsEventId) body.correctsEventId = (input as CorrectProgressInputMin).correctsEventId;
  return body;
}
/** 读取申请进展视图：事件按服务端权威顺序返回（seq 升序），7 阶段确定性投影随行。 */
export async function applicationProgress(applicationId: string): Promise<ProgressView> {
  if (!applicationId.trim()) throw new Error('缺少申请编号');
  return decodeProgressView(await client.request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId.trim())}/progress` }));
}
/** 追加一件不可变进展事件。expectedRevision 必须传 GET progress 读到的 revision（事件计数）。 */
export async function appendProgressEvent(input: ProgressEventInput, expectedRevision: number): Promise<ProgressReceipt> {
  if (!Number.isSafeInteger(expectedRevision) || expectedRevision < 0) throw new Error('进展事件缺少有效的预期修订（事件计数）');
  progressRequestBody(newRequestId(), input, expectedRevision); // 先校验，再进入可恢复写入
  return writeRecoverable<ProgressReceipt>('progress', '进展录入', input, async (id, expected) =>
    decodeAs(decodeProgressReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId.trim())}/progress`, body: progressRequestBody(id, input, expected) })), expectedRevision);
}
/** 纠错=追加引用事件（原事件保留）：只能纠 plain 事件，不能纠一条更正。 */
export async function correctProgressEvent(input: CorrectProgressInputMin, expectedRevision: number): Promise<ProgressReceipt> {
  if (!input.correctsEventId.trim()) throw new Error('纠错缺少被更正的事件编号');
  if (!Number.isSafeInteger(expectedRevision) || expectedRevision < 0) throw new Error('进展事件缺少有效的预期修订（事件计数）');
  progressRequestBody(newRequestId(), input, expectedRevision);
  return writeRecoverable<ProgressReceipt>('progress', '进展纠错', input, async (id, expected) =>
    decodeAs(decodeProgressReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId.trim())}/progress/correct`, body: progressRequestBody(id, input, expected) })), expectedRevision);
}
export function pendingProgressWrite(): StoredIntent<ProgressEventInput> | null { return readIntent<ProgressEventInput>(intentKey('progress')); }
export async function reconcilePendingProgress(): Promise<ProgressReceipt> {
  return reconcileIntent<ProgressReceipt>('progress', '进展写入', async id =>
    decodeProgressReceipt(await client.request({ method: 'GET', path: `/api/v1/career/progress/receipt?requestId=${encodeURIComponent(id)}` })));
}
export async function retryPendingProgress(): Promise<ProgressReceipt> {
  const pending = pendingProgressWrite();
  if (!pending) throw new Error('没有待恢复的进展写入');
  const input = pending.input as ProgressEventInput & { correctsEventId?: string };
  const correction = typeof input.correctsEventId === 'string' && input.correctsEventId;
  return retryIntent<ProgressReceipt>('progress', '进展写入', async (id, expected) =>
    decodeAs(decodeProgressReceipt, await client.request({
      method: 'POST',
      path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId.trim())}/progress${correction ? '/correct' : ''}`,
      body: progressRequestBody(id, input, expected),
    })));
}

// —— 面试准备（T19）：生成（锚定实际投递版）、列表、未知对账 ——
export const PREPARATION_VERSION_UNKNOWN = 'preparation_version_unknown';
/** 未确认实际投递版本的 typed 提示态（与 Web PreparationPage 同语义判定）。 */
export function isPreparationVersionUnknown(error: unknown): boolean {
  return (error as { code?: unknown } | null | undefined)?.code === PREPARATION_VERSION_UNKNOWN;
}
export interface GeneratePreparationIntent { applicationId: string; focus: PreparationFocus }
function preparationRequestBody(id: string, input: GeneratePreparationIntent, expected: number): Record<string, unknown> {
  if (!input.applicationId.trim() || !input.focus) throw new Error('准备生成缺少申请或焦点');
  return { requestId: id, applicationId: input.applicationId.trim(), focus: input.focus, expectedRevision: expected };
}
/** 生成准备草稿：锚定该申请实际投递版本（未确认投递→409 typed 提示态，由上层引导
 * 先记录投递；本端绝不自行改用最新材料版本）。CAS 域=档案头修订（expectedRevision）。 */
export async function generatePreparation(intent: GeneratePreparationIntent): Promise<PreparationReceipt> {
  preparationRequestBody(newRequestId(), intent, revision()); // 先校验，再进入可恢复写入
  return writeRecoverable<PreparationReceipt>('preparation', '准备生成', intent, async (id, expected) =>
    decodeAs(decodePreparationReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(intent.applicationId.trim())}/preparations`, body: preparationRequestBody(id, intent, expected) })));
}
export async function listPreparations(applicationId: string): Promise<PreparationList> {
  if (!applicationId.trim()) throw new Error('缺少申请编号');
  return decodePreparationList(await client.request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId.trim())}/preparations` }));
}
export function pendingPreparationWrite(): StoredIntent<GeneratePreparationIntent> | null { return readIntent<GeneratePreparationIntent>(intentKey('preparation')); }
export async function reconcilePendingPreparation(): Promise<PreparationReceipt> {
  return reconcileIntent<PreparationReceipt>('preparation', '准备生成', async id =>
    decodePreparationReceipt(await client.request({ method: 'GET', path: `/api/v1/career/preparations/receipt?requestId=${encodeURIComponent(id)}` })));
}
export async function retryPendingPreparation(): Promise<PreparationReceipt> {
  const pending = pendingPreparationWrite();
  if (!pending) throw new Error('没有待恢复的准备生成');
  return retryIntent<PreparationReceipt>('preparation', '准备生成', async (id, expected) =>
    decodeAs(decodePreparationReceipt, await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(pending.input.applicationId.trim())}/preparations`, body: preparationRequestBody(id, pending.input, expected) })), revision);
}

// ---- T32 全空间生命周期：导出、删除边界与完整删除（与 Web ExportDeletionPage 同源）----
// 五路由（internal/router/routes_career.go）：POST /exports、GET /exports/receipt、
// GET /deletions/boundary、POST /deletions、GET /deletions/receipt。同一批 api-client
// 解码器（同源同版本）；写入一律 requestId + expectedRevision，结果未知用原 requestId
// 对账/安全重发；删除回执只有在全部步骤成功后才报 deleted，部分失败保留可恢复状态。

// —— 全空间导出：同步内联归档（档案/原始岗位快照/申请事件/材料版本/投递记录）+ sha256 摘要 ——
export async function exportWholeSpace(): Promise<CareerExportReceipt> {
  return writeRecoverable<CareerExportReceipt>('spaceExport', '导出', {}, async (id, expected) =>
    decodeAs(decodeCareerExportReceipt, await client.request({ method: 'POST', path: '/api/v1/career/exports', body: { requestId: id, expectedRevision: expected } })));
}
export function pendingSpaceExport(): StoredIntent<Record<string, unknown>> | null { return readIntent<Record<string, unknown>>(intentKey('spaceExport')); }
export async function reconcilePendingSpaceExport(): Promise<CareerExportReceipt> {
  return reconcileIntent<CareerExportReceipt>('spaceExport', '导出', async id =>
    decodeCareerExportReceipt(await client.request({ method: 'GET', path: `/api/v1/career/exports/receipt?requestId=${encodeURIComponent(id)}` })));
}
export async function retryPendingSpaceExport(): Promise<CareerExportReceipt> {
  const pending = pendingSpaceExport();
  if (!pending) throw new Error('没有待恢复的导出');
  return retryIntent<CareerExportReceipt>('spaceExport', '导出', async (id, expected) =>
    decodeAs(decodeCareerExportReceipt, await client.request({ method: 'POST', path: '/api/v1/career/exports', body: { requestId: id, expectedRevision: expected } })), revision);
}
/** 删除后旧授权复验：按原请求编号读历史导出回执（完整删除后服务端应 404 not_found）。 */
export async function spaceExportReceipt(requestId: string): Promise<CareerExportReceipt> {
  if (!requestId.trim()) throw new Error('缺少原导出请求编号');
  return decodeCareerExportReceipt(await client.request({ method: 'GET', path: `/api/v1/career/exports/receipt?requestId=${encodeURIComponent(requestId.trim())}` }));
}

// —— 删除边界（pre-deletion explanation）：空间内将删什么、外部平台不可撤回什么、保留什么 ——
export async function deletionBoundary(): Promise<CareerDeletionBoundaryView> {
  return decodeCareerDeletionBoundary(await client.request({ method: 'GET', path: '/api/v1/career/deletions/boundary' }));
}

// —— 完整删除：步骤全成才报 deleted；部分失败同 requestId 可恢复，绝不称已完全删除。
// 与通用 writeRecoverable 的关键差异：部分失败（partial）是**成功的 HTTP 响应**，但
// 删除未到终态——intent 必须保留（存续至 deleted 终态），否则用户离开页面就丢了唯一
// 可恢复的原 request id。对账同理：非 deleted 回执不清 intent。
export async function deleteWholeSpace(): Promise<CareerDeletionReceipt> {
  const stamp = auth.scope.capture();
  const key = intentKeyFor('spaceDeletion', stamp);
  const pending = readIntent<Record<string, unknown>>(key);
  if (pending) throw Object.assign(new Error(`有一次未完成的删除（${pending.requestId.slice(0, 10)}…）：请先用原请求对账或重试，或明确放弃本机恢复记录`), { code: 'unresolved_action', requestId: pending.requestId });
  const id = newRequestId();
  const expected = revision();
  if (isRecoveryActive('spaceDeletion', stamp, 'starting')) throw Object.assign(new Error('有一次删除正在进行：请等待结果后再恢复'), { code: 'unresolved_action', requestId: id });
  return withActiveRecovery('spaceDeletion', stamp, 'starting', async () => {
  try {
    const receipt = decodeAs(decodeCareerDeletionReceipt, await client.request({ method: 'POST', path: '/api/v1/career/deletions', body: { requestId: id, expectedRevision: expected } }));
    if (receipt.status !== 'deleted') store.write(key, { requestId: id, input: {}, expectedRevision: expected });
    return receipt;
  } catch (error) {
    // OCR2-037：确定性本地失败（AUTH_REQUIRED 等）不落 intent——与 retryPendingSpaceDeletion 同一守卫口径。
    if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) {
      throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    }
    if (ambiguousOutcome(error)) {
      store.write(key, { requestId: id, input: {}, expectedRevision: expected });
      throw Object.assign(new Error('删除结果未知：请用原请求对账后再试', { cause: error }), { code: 'outcome_unknown', requestId: id });
    }
    throw error;
  }
  });
}
export function pendingSpaceDeletion(): StoredIntent<Record<string, unknown>> | null { return readIntent<Record<string, unknown>>(intentKey('spaceDeletion')); }
export async function reconcilePendingSpaceDeletion(): Promise<CareerDeletionReceipt> {
  const stamp = auth.scope.capture();
  const pending = pendingSpaceDeletion();
  if (!pending) throw new Error('没有待对账的删除');
  return withActiveRecovery('spaceDeletion', stamp, pending.requestId, async () => {
    const receipt = decodeCareerDeletionReceipt(await client.request({ method: 'GET', path: `/api/v1/career/deletions/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
    if (receipt.status === 'deleted') store.remove(intentKeyFor('spaceDeletion', stamp));
    return receipt;
  });
}
export async function retryPendingSpaceDeletion(): Promise<CareerDeletionReceipt> {
  const stamp = auth.scope.capture();
  const key = intentKeyFor('spaceDeletion', stamp);
  const pending = readIntent<Record<string, unknown>>(key);
  if (!pending) throw new Error('没有待恢复的删除');
  const expected = pending.expectedRevision ?? revision();
  return withActiveRecovery('spaceDeletion', stamp, pending.requestId, async () => {
  try {
    const receipt = decodeAs(decodeCareerDeletionReceipt, await client.request({ method: 'POST', path: '/api/v1/career/deletions', body: { requestId: pending.requestId, expectedRevision: expected } }));
    if (receipt.status === 'deleted') store.remove(key);
    return receipt;
  } catch (error) {
    if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (ambiguousOutcome(error)) throw Object.assign(new Error('删除结果未知：请用原请求对账后再试', { cause: error }), { code: 'outcome_unknown', requestId: pending.requestId });
    throw error;
  }
  });
}

/** 删除完成后的本机缓存清理：只清 wk:career:* 键并重置共享 desk；登录凭据与其他键不动。 */
export function clearCareerCaches(): string[] {
  const cleared: string[] = [];
  for (const key of storage.keys?.() ?? []) {
    if (key.startsWith(CAREER_STORE_PREFIX)) { storage.remove(key); cleared.push(key); }
  }
  resetCareerDesk();
  return cleared;
}
