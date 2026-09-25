import { client, auth } from './runtime.ts';
import { record, text } from './views.ts';
import { CareerDesk } from '../../../../packages/career-core/src/desk.ts';
import type { CareerRemote } from '../../../../packages/career-core/src/desk.ts';
import type { CareerView, CareerReceipt, CareerChangeSet, CareerAction, CareerUpload, CareerDocumentSource, OpportunityReceipt, OpportunityEvidence } from '../../../../packages/career-core/src/contracts.ts';
import { decodeCareerUpload, decodeCareerSources, decodeOpportunityReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { decodeOpportunityEvidencePage } from '../../../../packages/api-client/src/career.ts';
import type { NativeFileSource } from '@weknora/api-client';
import { requestId as newRequestId } from '../core/intent.ts';
import { scopeKey } from '../core/scope.ts';
import { CAREER_STORE_PREFIX, createControlledStore, type SharedImportDraft } from '../adapters/career-platform.ts';

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
