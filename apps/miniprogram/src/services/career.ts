import { client, auth } from './runtime.ts';
import { record, text } from './views.ts';
import { CareerDesk } from '../../../../packages/career-core/src/desk.ts';
import type { CareerRemote } from '../../../../packages/career-core/src/desk.ts';
import type { CareerView, CareerReceipt, CareerChangeSet, CareerAction, CareerUpload, CareerDocumentSource, OpportunityReceipt, OpportunityEvidence, EvaluationReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { decodeCareerUpload, decodeCareerSources, decodeOpportunityReceipt, decodeEvaluationReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { decodeOpportunityEvidencePage, decodeApplicationReceipt, decodeMaterialReceipt, decodeMaterialView, decodeMaterialVersionList, decodeMaterialExportReceipt, decodeMaterialExportList, decodeMaterialExportDownload, decodeSubmissionReceipt, decodeSubmissionList, decodeCareerExportReceipt, decodeCareerDeletionBoundary, decodeCareerDeletionReceipt } from '../../../../packages/api-client/src/career.ts';
import type { ApplicationReceipt, MaterialReceipt, MaterialView, MaterialVersionList, MaterialExportReceipt, MaterialExportList, MaterialExportDownload, MaterialExportFormat, SubmissionReceipt, SubmissionList, SubmissionChannel, MaterialBody, CareerExportReceipt, CareerDeletionBoundaryView, CareerDeletionReceipt } from '../../../../packages/api-client/src/career.ts';
import type { NativeFileSource } from '@weknora/api-client';
import { requestId as newRequestId } from '../core/intent.ts';
import { storage } from '../platform/storage.ts';
import { scopeKey } from '../core/scope.ts';
import { CAREER_STORE_PREFIX, createControlledStore, type SharedImportDraft, openExportedDocument, EXPORT_GRANT_EXPIRED, type ExportDownloadGrant, type ExportOpenRecord } from '../adapters/career-platform.ts';

export { prepareSharedImport, careerPlatform, CAREER_STORE_PREFIX } from '../adapters/career-platform.ts';
export type { SharedImportDraft, SharedEntry } from '../adapters/career-platform.ts';

// T24 小程序 Career service：消费冻结的服务端合同（Career open/list/act/changes/receipt、
// searches 三路由、opportunities import/evidence、sources upload）。微信身份只关联既有
// 档案：本端没有任何“创建档案”调用，open 由服务端按认证 scope 派生唯一档案。

let desk: CareerDesk | undefined;
/** 测试隔离用：丢弃共享 desk 单例。 */
export function resetCareerDesk(): void { desk = undefined; }
function remote(): CareerRemote {
  return {
    open: async signal => record(await client.request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) as CareerView,
    list: async signal => record(await client.request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) as CareerView,
    changes: async (since, signal) => record(await client.request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) })) as CareerChangeSet,
    act: async (action, signal) => record(await client.request({ method: 'POST', path: '/api/v1/career/act', body: action, ...(signal ? { signal } : {}) })) as CareerReceipt,
    receipt: async (requestId, signal) => record(await client.request({ method: 'GET', path: `/api/v1/career/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) })) as CareerReceipt,
  };
}
export function careerDesk(): CareerDesk { if (!desk) desk = new CareerDesk(remote()); return desk; }
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
  const searchId = text(r.searchId); if (!searchId) throw new Error('无效的搜索 ID');
  const coverage = r.coverage === undefined ? {} : record(r.coverage);
  const sourcesRaw = Array.isArray(coverage.sources) ? coverage.sources : [];
  const sources: SearchCoverageSource[] = sourcesRaw.map(raw => {
    const s = record(raw);
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
    return {
      resultId: text(row.resultId), sourceId: text(row.sourceId), link: text(row.link),
      checkedAt: text(row.checkedAt), qualification: text(row.qualification), uncertainty: text(row.uncertainty),
    };
  });
  const scopeNotes = Array.isArray(r.scopeNotes) ? r.scopeNotes.filter((v): v is string => typeof v === 'string') : [];
  return {
    searchId, requestId: text(r.requestId), status: r.status, query: text(r.query),
    rows, sources, scopeNotes,
    ...(typeof r.failureCode === 'string' && r.failureCode ? { failureCode: r.failureCode } : {}),
    checkedAt: text(r.checkedAt), hasNoVettedSources: sources.length === 0,
  };
}

interface PendingSearchIntent { requestId: string; query: string }
const store = createControlledStore();
const searchKey = (): string => `${CAREER_STORE_PREFIX}search:${scopeKey(auth.scope.capture())}`;
export function pendingSearch(): PendingSearchIntent | null {
  const value = store.read(searchKey());
  if (!value || typeof value !== 'object') return null;
  const r = value as Record<string, unknown>;
  if (typeof r.requestId !== 'string' || typeof r.query !== 'string') return null;
  return { requestId: r.requestId, query: r.query };
}
function ambiguous(error: unknown): boolean {
  const code = (error as { code?: unknown })?.code;
  if (typeof code === 'string') {
    if (['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED', 'outcome_unknown'].includes(code)) return true;
    if (['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved', 'search_quota_refused'].includes(code)) return false;
  }
  const status = (error as { status?: unknown })?.status;
  return typeof status !== 'number' || status >= 500;
}
function unknownOutcome(requestId: string, query: string, cause: unknown): Error {
  return Object.assign(new Error('搜索结果未知：请用原请求恢复后再试', { cause }), { code: 'outcome_unknown', requestId, query });
}
/** 一次性找岗：POST /searches 的冻结合同（requestId/query/expectedRevision，禁多余字段）。 */
export async function searchOnce(query: string): Promise<SearchOutcome> {
  const trimmed = query.trim();
  if (!trimmed) throw new Error('请先输入想找的岗位或要求');
  const id = newRequestId();
  try {
    return decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: trimmed, expectedRevision: revision() } }));
  } catch (error) {
    if (ambiguous(error)) { store.write(searchKey(), { requestId: id, query: trimmed }); throw unknownOutcome(id, trimmed, error); }
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
/** 对账确认服务端无记录（404）后的安全重试：同 request id 幂等重发。 */
export async function retryPendingSearch(): Promise<SearchOutcome> {
  const pending = pendingSearch();
  if (!pending) throw new Error('没有待恢复的搜索');
  try {
    const outcome = decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: pending.requestId, query: pending.query, expectedRevision: revision() } }));
    store.remove(searchKey());
    return outcome;
  } catch (error) {
    if (ambiguous(error)) throw unknownOutcome(pending.requestId, pending.query, error);
    throw error;
  }
}

/** 上传已有简历：抽取事实以待确认 proposal 返回（未确认不生效）。 */
export async function uploadResume(file: NativeFileSource): Promise<CareerUpload> {
  return decodeCareerUpload(await client.request({
    method: 'POST', path: '/api/v1/career/sources/upload', nativeFile: file,
    multipartFields: { requestId: newRequestId(), expectedRevision: String(revision()) },
  }));
}
export async function listSources(): Promise<CareerDocumentSource[]> {
  return decodeCareerSources(await client.request({ method: 'GET', path: '/api/v1/career/sources' }));
}

/** 分享导入第二阶段：核对预览后才提交，提交原文而非摘录。 */
export async function confirmSharedImport(draft: SharedImportDraft): Promise<OpportunityReceipt> {
  return decodeOpportunityReceipt(await client.request({
    method: 'POST', path: '/api/v1/career/opportunities/import',
    body: { requestId: draft.requestId, rawText: draft.rawText, ...(draft.sourceLabel ? { sourceLabel: draft.sourceLabel } : {}) },
  }));
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
export async function evaluateOpportunity(opportunityId: string, snapshotId: string): Promise<EvaluationReceipt> {
  if (!opportunityId.trim() || !snapshotId.trim()) throw new Error('缺少岗位快照信息');
  return decodeEvaluationReceipt(await client.request({ method: 'POST', path: '/api/v1/career/evaluations', body: { requestId: newRequestId(), opportunityId: opportunityId.trim(), snapshotId: snapshotId.trim() } }));
}

export interface ApplicationIntentInput { opportunityId: string; snapshotId: string; evaluationId: string; batchIdentity: string; continueDespiteHardFailure: boolean }
interface StoredIntent<T> { requestId: string; input: T; expectedRevision?: number }
function intentKey(kind: string): string { return `${CAREER_STORE_PREFIX}${kind}:${scopeKey(auth.scope.capture())}`; }
function readIntent<T>(key: string): StoredIntent<T> | null {
  const value = store.read(key);
  if (!value || typeof value !== 'object') return null;
  const parsed = value as { requestId?: unknown; input?: unknown; expectedRevision?: unknown };
  if (typeof parsed.requestId !== 'string' || !parsed.requestId || typeof parsed.input !== 'object' || !parsed.input) return null;
  const expectedRevision = typeof parsed.expectedRevision === 'number' && Number.isSafeInteger(parsed.expectedRevision) && parsed.expectedRevision >= 0 ? parsed.expectedRevision : undefined;
  return { requestId: parsed.requestId, input: parsed.input as T, ...(expectedRevision !== undefined ? { expectedRevision } : {}) };
}
/** 空间切换/AUTH_REQUIRED 是确定失败：旧作用域的响应不能为新作用域留恢复意图。 */
function definiteLocalFailure(error: unknown): boolean { return /SCOPE_CHANGED|AUTH_REQUIRED/i.test(`${(error as Error)?.message ?? ''} ${(error as { code?: unknown })?.code ?? ''}`); }
/** 写入 + 未知结果恢复。expectedRevision 在发送前捕获并随 intent 持久化：服务端幂等
 * 指纹是全量请求体（含 expectedRevision），安全重发必须逐字节重放原始值——desk 修订
 * 前进后用当前值重发必然 ErrIdempotencyConflict，恢复死路。 */
async function writeRecoverable<T>(kind: string, describe: string, input: unknown, send: (id: string, expected: number) => Promise<unknown>): Promise<T> {
  const id = newRequestId();
  const expected = revision();
  const stamp = auth.scope.capture();
  try {
    return await send(id, expected) as T;
  } catch (error) {
    if (ambiguous(error) && !auth.scope.isCurrent(stamp)) {
      throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    }
    if (ambiguous(error)) {
      store.write(intentKey(kind), { requestId: id, input, expectedRevision: expected });
      throw Object.assign(new Error(`${describe}结果未知：请用原请求对账后再试`, { cause: error }), { code: 'outcome_unknown', requestId: id });
    }
    throw error;
  }
}
async function reconcileIntent<T>(kind: string, describe: string, fetch: (id: string) => Promise<unknown>): Promise<T> {
  const pending = readIntent<Record<string, unknown>>(intentKey(kind));
  if (!pending) throw new Error(`没有待对账的${describe}`);
  const receipt = await fetch(pending.requestId);
  store.remove(intentKey(kind));
  return receipt as T;
}
async function retryIntent<T>(kind: string, describe: string, resend: (id: string, expected: number) => Promise<unknown>): Promise<T> {
  const pending = readIntent<Record<string, unknown>>(intentKey(kind));
  if (!pending) throw new Error(`没有待恢复的${describe}`);
  const expected = pending.expectedRevision ?? revision();
  const stamp = auth.scope.capture();
  try {
    const receipt = await resend(pending.requestId, expected);
    store.remove(intentKey(kind));
    return receipt as T;
  } catch (error) {
    if (ambiguous(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (ambiguous(error)) throw Object.assign(new Error(`${describe}结果未知：请用原请求对账后再试`, { cause: error }), { code: 'outcome_unknown', requestId: pending.requestId });
    throw error;
  }
}

// —— 申请（T14 合同）：一岗一批一申请；硬条件不符必须显式继续 ——
export async function createApplication(input: ApplicationIntentInput): Promise<ApplicationReceipt> {
  if (!input.opportunityId.trim() || !input.snapshotId.trim() || !input.evaluationId.trim() || !input.batchIdentity.trim()) throw new Error('申请缺少岗位、评估或批次信息');
  const batch = input.batchIdentity.trim();
  return writeRecoverable<ApplicationReceipt>('application', '申请创建', input, async (id, expected) =>
    decodeApplicationReceipt(await client.request({ method: 'POST', path: '/api/v1/career/applications', body: { requestId: id, opportunityId: input.opportunityId.trim(), snapshotId: input.snapshotId.trim(), evaluationId: input.evaluationId.trim(), batchIdentity: batch, continueDespiteHardFailure: input.continueDespiteHardFailure, expectedRevision: expected } })));
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
    decodeApplicationReceipt(await client.request({ method: 'POST', path: '/api/v1/career/applications', body: { requestId: id, opportunityId: pending.input.opportunityId.trim(), snapshotId: pending.input.snapshotId.trim(), evaluationId: pending.input.evaluationId.trim(), batchIdentity: pending.input.batchIdentity.trim(), continueDespiteHardFailure: pending.input.continueDespiteHardFailure, expectedRevision: expected } })));
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
    decodeMaterialReceipt(await client.request({ method: 'POST', path: '/api/v1/career/materials', body: materialRequestBody(id, intent, expected) })));
}
export async function confirmMaterial(materialId: string): Promise<MaterialReceipt> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return writeRecoverable<MaterialReceipt>('material', '材料确认', { op: 'confirm' as const, materialId: materialId.trim() }, async (id, expected) =>
    decodeMaterialReceipt(await client.request({ method: 'POST', path: '/api/v1/career/materials/confirm', body: { requestId: id, materialId: materialId.trim(), expectedRevision: expected } })));
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
      return decodeMaterialReceipt(await client.request({ method: 'POST', path: '/api/v1/career/materials', body: materialRequestBody(id, intent, expected) }));
    }
    return decodeMaterialReceipt(await client.request({ method: 'POST', path: '/api/v1/career/materials/confirm', body: { requestId: id, materialId: String(stored.materialId ?? ''), expectedRevision: expected } }));
  });
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
    decodeMaterialExportReceipt(await client.request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}/exports`, body: { requestId: id, version, expectedRevision: expected } })));
}
export async function listMaterialExports(materialId: string): Promise<MaterialExportList> {
  if (!materialId.trim()) throw new Error('缺少材料编号');
  return decodeMaterialExportList(await client.request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId.trim())}/exports` }));
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
    decodeSubmissionReceipt(await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId.trim())}/submissions`, body: submissionRequestBody(id, input, expected) })));
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
    decodeSubmissionReceipt(await client.request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(pending.input.applicationId)}/submissions`, body: submissionRequestBody(id, pending.input, expected) })));
}
export async function listSubmissions(applicationId: string): Promise<SubmissionList> {
  if (!applicationId.trim()) throw new Error('缺少申请编号');
  return decodeSubmissionList(await client.request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId.trim())}/submissions` }));
}

// ---- T32 全空间生命周期：导出、删除边界与完整删除（与 Web ExportDeletionPage 同源）----
// 五路由（internal/router/routes_career.go）：POST /exports、GET /exports/receipt、
// GET /deletions/boundary、POST /deletions、GET /deletions/receipt。同一批 api-client
// 解码器（同源同版本）；写入一律 requestId + expectedRevision，结果未知用原 requestId
// 对账/安全重发；删除回执只有在全部步骤成功后才报 deleted，部分失败保留可恢复状态。

// —— 全空间导出：同步内联归档（档案/原始岗位快照/申请事件/材料版本/投递记录）+ sha256 摘要 ——
export async function exportWholeSpace(): Promise<CareerExportReceipt> {
  return writeRecoverable<CareerExportReceipt>('spaceExport', '导出', {}, async (id, expected) =>
    decodeCareerExportReceipt(await client.request({ method: 'POST', path: '/api/v1/career/exports', body: { requestId: id, expectedRevision: expected } })));
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
    decodeCareerExportReceipt(await client.request({ method: 'POST', path: '/api/v1/career/exports', body: { requestId: id, expectedRevision: expected } })));
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
  const id = newRequestId();
  const expected = revision();
  const stamp = auth.scope.capture();
  try {
    const receipt = decodeCareerDeletionReceipt(await client.request({ method: 'POST', path: '/api/v1/career/deletions', body: { requestId: id, expectedRevision: expected } }));
    if (receipt.status !== 'deleted') store.write(intentKey('spaceDeletion'), { requestId: id, input: {}, expectedRevision: expected });
    return receipt;
  } catch (error) {
    if (ambiguous(error) && !auth.scope.isCurrent(stamp)) {
      throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    }
    if (ambiguous(error)) {
      store.write(intentKey('spaceDeletion'), { requestId: id, input: {}, expectedRevision: expected });
      throw Object.assign(new Error('删除结果未知：请用原请求对账后再试', { cause: error }), { code: 'outcome_unknown', requestId: id });
    }
    throw error;
  }
}
export function pendingSpaceDeletion(): StoredIntent<Record<string, unknown>> | null { return readIntent<Record<string, unknown>>(intentKey('spaceDeletion')); }
export async function reconcilePendingSpaceDeletion(): Promise<CareerDeletionReceipt> {
  const pending = pendingSpaceDeletion();
  if (!pending) throw new Error('没有待对账的删除');
  const receipt = decodeCareerDeletionReceipt(await client.request({ method: 'GET', path: `/api/v1/career/deletions/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
  if (receipt.status === 'deleted') store.remove(intentKey('spaceDeletion'));
  return receipt;
}
export async function retryPendingSpaceDeletion(): Promise<CareerDeletionReceipt> {
  const pending = pendingSpaceDeletion();
  if (!pending) throw new Error('没有待恢复的删除');
  const expected = pending.expectedRevision ?? revision();
  const stamp = auth.scope.capture();
  try {
    const receipt = decodeCareerDeletionReceipt(await client.request({ method: 'POST', path: '/api/v1/career/deletions', body: { requestId: pending.requestId, expectedRevision: expected } }));
    if (receipt.status === 'deleted') store.remove(intentKey('spaceDeletion'));
    return receipt;
  } catch (error) {
    if (ambiguous(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (ambiguous(error)) throw Object.assign(new Error('删除结果未知：请用原请求对账后再试', { cause: error }), { code: 'outcome_unknown', requestId: pending.requestId });
    throw error;
  }
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
