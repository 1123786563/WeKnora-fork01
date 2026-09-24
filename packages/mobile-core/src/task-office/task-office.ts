import {
  createInMemorySubmissionStore,
  createSubmissionCoordinator,
  evaluateSubmitReadiness,
  inputDigest,
  toStartInput,
  SubmissionConflictError,
  type NewTaskDraft,
  type SubmissionEntry,
  type SubmissionScope,
  type SubmissionStore,
  type SubmissionTransport,
  type TaskAttachmentRef,
} from '@weknora/domain/mobile';
import { leaseActive, leaseScopeOf } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createAttentionDecider } from './attention-inbox.ts';
import type { AttentionDecisionInput, AttentionDecisionReceipt, InboxView, InteractionBackendPort } from './attention-inbox.ts';
import { createInMemoryTaskProjectionStore } from './in-memory-task-detail.ts';
import { legacyTaskGates } from './legacy-tasks.ts';
import type {
  LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPage, LegacyTaskBackendPort,
  LegacyTaskCard, LegacyTaskListPage,
} from './legacy-tasks.ts';
import { createTaskDetail } from './task-detail.ts';
import type { TaskDetailBackendPort, TaskHandle, TaskProjectionStore } from './task-detail.ts';
import { TaskOfficeError } from './task-office-errors.ts';
import type { AttentionState } from './task-office-errors.ts';

export { TaskOfficeError } from './task-office-errors.ts';
export type { AttentionState, TaskOfficeErrorCode } from './task-office-errors.ts';

/**
 * Task Office 深模块（module-seams §5）——T04 交付读侧与归档生命周期：
 * - home()：一次聚合读返回三段视图（需要我处理 / 正在运行 / 最近完成）；
 * - tasks()/moreTasks()：模块拥有 cursor 与查询身份，屏不维护分页状态；
 * - archive()/restore()：归档写路径，成功后累积查询失效；
 * - 迟到拒绝：scope lease 撤销（切租户/换部署/退出）后的一切结果按
 *   TASK_OFFICE_SCOPE_CHANGED 拒绝；更新的查询使旧查询按 SUPERSEDED 拒绝；
 * - 重复键：跨页重复的 runId 不再渲染，经 duplicateRunIds 观测。
 * open(taskId, runId) 已由 T05（#35）交付详情句柄；start(goal) 属 #36。
 */

export type TaskStatusFilter = 'running' | 'waiting_user' | 'succeeded' | 'failed' | 'canceled';

export interface TaskCard {
  taskId: string;
  runId: string;
  title: string;
  runStatus: string;
  attention: AttentionState;
  updatedAt: string;
}

export interface InteractionCard {
  interactionId: string;
  kind: string;
  createdAt: string;
}

export interface HomeView {
  needsMe: InteractionCard[];
  running: TaskCard[];
  recentlyCompleted: TaskCard[];
  unreadNotifications: number;
  asOf: string;
}

export interface TaskOfficeQuery {
  status?: TaskStatusFilter;
  agentId?: string;
  search?: string;
  archived?: boolean;
  limit?: number;
}

export interface TaskListPage {
  items: TaskCard[];
  nextCursor?: string;
  duplicateRunIds: string[];
}

export interface TaskBackendRun {
  runId: string;
  taskId: string;
  title: string;
  runStatus: string;
  attention: AttentionState;
  updatedAt: string;
}

export interface TaskBackendOverview {
  needsMe: InteractionCard[];
  running: TaskBackendRun[];
  recentlyCompleted: TaskBackendRun[];
  unreadNotifications: number;
  asOf: string;
}

export interface TaskBackendPage {
  items: TaskBackendRun[];
  nextCursor?: string;
}

export interface TaskBackendListInput {
  status?: string;
  agentId?: string;
  search?: string;
  archived?: boolean;
  cursor?: string;
  limit?: number;
}

/** T06 Start wire 输入：七字段冻结（与 api-client StartExecutionInput / domain MobileStartInput 逐字一致）。 */
export interface TaskBackendStartInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number;
}

export interface TaskBackendStartAck {
  run_id: string;
  request_id: string;
  status: string;
}

export interface TaskBackendLookup {
  state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown';
  run_id?: string;
  reason?: string;
}

/** 一次用户目标的完整形态：附件/知识只参与就绪裁决与草稿，绝不进入 Start body。 */
export interface TaskOfficeGoal {
  text: string;
  agentId: string;
  budgetUpper: number;
  knowledgeIds?: string[];
  attachments?: TaskAttachmentRef[];
}

/** module-seams §5.2：start(goal) 持久化意图并返回 TaskStartReceipt。 */
export interface TaskStartReceipt {
  requestId: string;
  phase: SubmissionEntry['phase'];
  runId?: string;
  dispatched: boolean;
}

export interface TaskBackendPort {
  overview(): Promise<TaskBackendOverview>;
  list(input: TaskBackendListInput): Promise<TaskBackendPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
  /** T06：一个初始目标创建 Task 前的目标会话（taskId = sessionId，ADR-0004）。 */
  createSession(input: { title: string }): Promise<{ sessionId: string }>;
  /** T06：持久幂等 Start（POST /api/v1/workbench/executions；七字段 wire）。 */
  start(input: TaskBackendStartInput): Promise<TaskBackendStartAck>;
  /** T06：request 对账（GET /api/v1/workbench/executions/requests/:request_id）。 */
  lookup(requestId: string): Promise<TaskBackendLookup>;
}

export interface TaskOfficePorts {
  backend: TaskBackendPort;
  lease(): ScopeLease | undefined;
  /** T05 详情与 SSE 恢复端口；缺失时 open() fail closed（TASK_OFFICE_DETAIL_UNAVAILABLE）。 */
  detail?: TaskDetailBackendPort;
  /** 持久化投影存储（App 重启恢复）；缺省为 office 内共享的 in-memory store。 */
  store?: TaskProjectionStore;
  /** T06 进程内提交存储（domain SubmissionStore 的同步契约：save 失败必须同步抛出且不发送）；
   *  缺省 office 内 in-memory。不得注入异步实现——耐久层是下面的 intentLog。 */
  submissionStore?: SubmissionStore;
  /** T06 耐久意图日志：office 在 Start POST 前 await save；重入/重启恢复时 load/listScope
   *  复用原 sessionId 与 goal。缺省 office 内 in-memory（进程内）——持久化是组合根的显式决策。 */
  intentLog?: SubmissionIntentLog;
  /** T06：一次用户意图的新 request_id 生成器；缺省 crypto.randomUUID（平台无实现时必须显式注入）。 */
  newRequestId?: () => string;
  /** T08: 类型化交互端口（Attention Inbox 读 + 决定）。缺失时 inbox()/decide() fail closed。 */
  interactions?: InteractionBackendPort;
  /** T14（#44）Legacy Task 端口；缺失时 legacy 入口 fail closed（TASK_OFFICE_LEGACY_UNAVAILABLE）。 */
  legacy?: LegacyTaskBackendPort;
}

export interface TaskOffice {
  home(): Promise<HomeView>;
  tasks(query: TaskOfficeQuery): Promise<TaskListPage>;
  moreTasks(): Promise<TaskListPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
  /** T08: 收件箱读——本人全部待处理交互（跨 run、含决定上下文）。 */
  inbox(): Promise<InboxView>;
  /** T08: 类型化决定——冻结 decision_id 幂等重放；receipt 如实区分 recorded / delivery-unknown / superseded / gone。 */
  decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>;
  open(input: { taskId: string; runId: string }): TaskHandle;
  start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>;
  reconcilePending(): Promise<TaskStartReceipt[]>;
  /** T14（#44）追加区：Legacy Task 读投影与普通追问。 */
  legacyTasks(query: { search?: string; archived?: boolean; limit?: number }): Promise<LegacyTaskListPage>;
  moreLegacyTasks(): Promise<LegacyTaskListPage>;
  legacyHistory(taskId: string): Promise<LegacyMessage[]>;
  followUp(input: LegacyFollowUpInput): Promise<void>;
}

/** T06 耐久意图记录：重启后用原 session 与原 goal 重建 digest 一致的 Start 输入。 */
export interface SubmissionIntentRecord {
  requestId: string;
  sessionId: string;
  goal: TaskOfficeGoal;
  scope: SubmissionScope;
  persistedAt: string;
}

/** 耐久意图日志（office await；RN SecureStore 等异步持久层的忠实契约）。 */
export interface SubmissionIntentLog {
  save(record: SubmissionIntentRecord): Promise<void>;
  load(requestId: string): Promise<SubmissionIntentRecord | undefined>;
  listScope(scope: SubmissionScope): Promise<SubmissionIntentRecord[]>;
  /** bound/rejected 后清理；不实现则记录留存（reconcile 幂等无害，仅列表增长）。 */
  remove?(requestId: string): Promise<void>;
}

export function createInMemoryIntentLog(): SubmissionIntentLog {
  const records = new Map<string, SubmissionIntentRecord>();
  const sameScope = (a: SubmissionScope, b: SubmissionScope): boolean => a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
  return {
    async save(record) { records.set(record.requestId, record); },
    async load(requestId) { return records.get(requestId); },
    async listScope(scope) { return [...records.values()].filter((record) => sameScope(record.scope, scope)); },
    async remove(requestId) { records.delete(requestId); },
  };
}

const searchMaxLen = 200;
const statusFilters: ReadonlySet<TaskStatusFilter> = new Set(['running', 'waiting_user', 'succeeded', 'failed', 'canceled']);

function normalizeQuery(query: TaskOfficeQuery): TaskBackendListInput {
  const search = typeof query.search === 'string' ? query.search.trim().replace(/\s+/g, ' ').slice(0, searchMaxLen) : '';
  const agentId = typeof query.agentId === 'string' ? query.agentId.trim() : '';
  const status = query.status !== undefined && statusFilters.has(query.status) ? query.status : undefined;
  const limit = typeof query.limit === 'number' && Number.isSafeInteger(query.limit) && query.limit > 0 ? Math.min(query.limit, 100) : 20;
  return {
    ...(status === undefined ? {} : { status }),
    ...(agentId === '' ? {} : { agentId }),
    ...(search === '' ? {} : { search }),
    ...(query.archived === true ? { archived: true } : {}),
    limit,
  };
}

interface listAccumulation {
  input: TaskBackendListInput;
  cursor?: string;
  seen: Set<string>;
  duplicates: string[];
}

export function createTaskOffice(ports: TaskOfficePorts): TaskOffice {
  let homeEpoch = 0;
  let listEpoch = 0;
  let accumulated: listAccumulation | undefined;
  const defaultDetailStore = createInMemoryTaskProjectionStore();
  const submissionStore = ports.submissionStore ?? createInMemorySubmissionStore();
  const intentLog = ports.intentLog ?? createInMemoryIntentLog();
  const submissionTransport: SubmissionTransport = {
    start: (input) => ports.backend.start(input),
    lookup: async (requestId) => {
      try {
        return await ports.backend.lookup(requestId);
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
    },
  };
  const submissions = createSubmissionCoordinator(submissionStore, submissionTransport);
  const submissionScopeOf = (lease: ScopeLease): SubmissionScope => {
    const scope = leaseScopeOf(lease);
    if (!scope) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return { origin: scope.deploymentOrigin, tenantID: scope.tenantId, userID: scope.userId };
  };
  const receiptOf = (entry: SubmissionEntry, dispatched: boolean): TaskStartReceipt => ({
    requestId: entry.request_id,
    phase: entry.phase,
    ...(entry.run_id === undefined ? {} : { runId: entry.run_id }),
    dispatched,
  });
  const nextRequestId = (): string => {
    if (ports.newRequestId) return ports.newRequestId();
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('newRequestId port is required on platforms without crypto.randomUUID') });
  };
  const goalDraftOf = (goal: TaskOfficeGoal): NewTaskDraft => ({
    text: goal.text,
    agentId: goal.agentId,
    budgetUpper: goal.budgetUpper,
    attachments: goal.attachments ?? [],
    knowledgeIds: goal.knowledgeIds ?? [],
  });
  // 意图一致性比较键：覆盖影响意图的字段（text/agentId/budgetUpper/knowledgeIds 排序）。
  // attachments 的 readiness 是状态不是意图（scanning→ready 不构成「换意图」），不进比较键。
  const goalKeyOf = (goal: TaskOfficeGoal): string => JSON.stringify({
    agentId: goal.agentId,
    budgetUpper: goal.budgetUpper,
    knowledgeIds: [...(goal.knowledgeIds ?? [])].sort(),
    text: goal.text,
  });
  const attention = createAttentionDecider({
    interactions: () => ports.interactions,
    lease: ports.lease,
    // 决定终态失效一切在途聚合读（与 archive/restore 的 R1-F19 同语义）：
    // 迟到的 home()/inbox() 不得把已决定行回填为 pending。
    onDecided: () => {
      homeEpoch += 1;
      listEpoch += 1;
      accumulated = undefined;
    },
  });

  let legacyListEpoch = 0;
  let legacyAccumulated: legacyListAccumulation | undefined;

  interface legacyListAccumulation {
    input: { search?: string; archived?: boolean; limit?: number };
    cursor?: string;
    seen: Set<string>;
    duplicates: string[];
  }

  const requireLegacy = (): LegacyTaskBackendPort => {
    if (ports.legacy === undefined) throw new TaskOfficeError('TASK_OFFICE_LEGACY_UNAVAILABLE');
    return ports.legacy;
  };
  const toLegacyCard = (task: LegacyBackendTask): LegacyTaskCard => ({
    taskId: task.taskId,
    title: task.title,
    attention: 'none',
    ...(task.archivedAt === undefined ? {} : { archivedAt: task.archivedAt }),
    updatedAt: task.updatedAt,
    kind: 'legacy',
    gates: legacyTaskGates(),
  });
  const accumulateLegacy = (state: legacyListAccumulation, page: LegacyTaskBackendPage): LegacyTaskListPage => {
    const items: LegacyTaskCard[] = [];
    for (const task of page.items) {
      if (state.seen.has(task.taskId)) {
        state.duplicates.push(task.taskId);
        continue;
      }
      state.seen.add(task.taskId);
      items.push(toLegacyCard(task));
    }
    state.cursor = page.nextCursor;
    return { items, ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }), duplicateTaskIds: [...state.duplicates] };
  };
  const normalizeLegacyQuery = (query: { search?: string; archived?: boolean; limit?: number }): { search?: string; archived?: boolean; limit?: number } => {
    const search = typeof query.search === 'string' ? query.search.trim().replace(/\s+/g, ' ').slice(0, searchMaxLen) : '';
    const limit = typeof query.limit === 'number' && Number.isSafeInteger(query.limit) && query.limit > 0 ? Math.min(query.limit, 100) : 20;
    return { ...(search === '' ? {} : { search }), ...(query.archived === true ? { archived: true } : {}), limit };
  };

  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const callBackend = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const settle = <T>(epoch: number, kind: 'home' | 'list' | 'legacy', lease: ScopeLease, value: T): T => {
    const currentEpoch = kind === 'home' ? homeEpoch : kind === 'list' ? listEpoch : legacyListEpoch;
    if (epoch !== currentEpoch) throw new TaskOfficeError('TASK_OFFICE_SUPERSEDED');
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return value;
  };
  const toCard = (run: TaskBackendRun): TaskCard => ({ ...run });
  const accumulate = (state: listAccumulation, page: TaskBackendPage): TaskListPage => {
    const items: TaskCard[] = [];
    for (const run of page.items) {
      if (state.seen.has(run.runId)) {
        state.duplicates.push(run.runId);
        continue;
      }
      state.seen.add(run.runId);
      items.push(toCard(run));
    }
    state.cursor = page.nextCursor;
    return { items, ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }), duplicateRunIds: [...state.duplicates] };
  };
  const mutate = async (taskId: string, action: (id: string) => Promise<void>): Promise<void> => {
    const lease = requireLease();
    const trimmed = taskId.trim();
    if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
    await callBackend(() => action(trimmed));
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    // 写成功作废一切在途读（R1-F19）：settle 比对 epoch，旧查询按 SUPERSEDED 拒绝，
    // 不得以归档前快照重建 accumulated。home 同理失效。
    listEpoch += 1;
    homeEpoch += 1;
    legacyListEpoch += 1;
    accumulated = undefined;
    legacyAccumulated = undefined;
  };

  return {
    async home(): Promise<HomeView> {
      const epoch = ++homeEpoch;
      const lease = requireLease();
      const overview = settle(epoch, 'home', lease, await callBackend(() => ports.backend.overview()));
      return {
        needsMe: overview.needsMe,
        running: overview.running.map(toCard),
        recentlyCompleted: overview.recentlyCompleted.map(toCard),
        unreadNotifications: overview.unreadNotifications,
        asOf: overview.asOf,
      };
    },
    async tasks(query: TaskOfficeQuery): Promise<TaskListPage> {
      const epoch = ++listEpoch;
      const lease = requireLease();
      const input = normalizeQuery(query);
      const page = settle(epoch, 'list', lease, await callBackend(() => ports.backend.list(input)));
      const state: listAccumulation = { input, seen: new Set<string>(), duplicates: [] };
      accumulated = state;
      return accumulate(state, page);
    },
    async moreTasks(): Promise<TaskListPage> {
      const state = accumulated;
      if (state === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_ACTIVE_QUERY');
      if (state.cursor === undefined) return { items: [], duplicateRunIds: [...state.duplicates] };
      const epoch = ++listEpoch;
      const lease = requireLease();
      const page = settle(epoch, 'list', lease, await callBackend(() => ports.backend.list({ ...state.input, cursor: state.cursor })));
      return accumulate(state, page);
    },
    archive(taskId: string): Promise<void> {
      return mutate(taskId, (id) => ports.backend.archive(id));
    },
    restore(taskId: string): Promise<void> {
      return mutate(taskId, (id) => ports.backend.restore(id));
    },
    inbox(): Promise<InboxView> {
      return attention.inbox();
    },
    decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt> {
      return attention.decide(input);
    },
    open(taskOpen: { taskId: string; runId: string }): TaskHandle {
      const taskId = taskOpen.taskId.trim();
      const runId = taskOpen.runId.trim();
      if (taskId === '' || runId === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      if (ports.detail === undefined) throw new TaskOfficeError('TASK_OFFICE_DETAIL_UNAVAILABLE');
      return createTaskDetail({ taskId, runId }, { backend: ports.detail, store: ports.store ?? defaultDetailStore, lease: ports.lease });
    },
    async start(startGoal: TaskOfficeGoal, options: { requestId?: string } = {}): Promise<TaskStartReceipt> {
      const lease = requireLease();
      const scope = submissionScopeOf(lease);
      const draft = goalDraftOf(startGoal);
      const readiness = evaluateSubmitReadiness(draft);
      if (!readiness.ready) {
        if (readiness.reason === 'attachments_not_ready') throw new TaskOfficeError('TASK_OFFICE_ATTACHMENTS_NOT_READY');
        throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      }
      const requestId = options.requestId ?? nextRequestId();
      const record = await intentLog.load(requestId);
      let sessionId: string;
      if (record === undefined) {
        // 新意图：目标会话是前置网络调用（无 Task/预算副作用，与 miniprogram AgentPage 同序），
        // 随后在 Start POST 之前耐久落盘意图记录——intentLog 写失败（磁盘满等）上抛且零 Start 派发。
        const session = await callBackend(() => ports.backend.createSession({ title: startGoal.text.trim().slice(0, 60) }));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
        sessionId = session.sessionId;
        await intentLog.save({ requestId, sessionId, goal: startGoal, scope, persistedAt: new Date().toISOString() });
      } else {
        // 重入/重启恢复：绝不新建 session——inputDigest 覆盖 session_id（submission.ts:65-74），
        // 换 session 会伪造成新意图并撞 digest 冲突。借旧 ID 发新意图零网络拒绝。
        if (record.scope.origin !== scope.origin || record.scope.tenantID !== scope.tenantID || record.scope.userID !== scope.userID) {
          throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: new Error(`intent ${requestId} belongs to a different scope`) });
        }
        if (goalKeyOf(record.goal) !== goalKeyOf(startGoal)) {
          throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: new Error(`intent ${requestId} was persisted with a different goal`) });
        }
        sessionId = record.sessionId;
      }
      const input = toStartInput(draft, { requestID: requestId, sessionId, targetId: 'platform', workspaceRef: '' });
      try {
        const outcome = await submissions.resume(input, scope);
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
        if (outcome.entry.phase === 'bound') await intentLog.remove?.(requestId);
        return receiptOf(outcome.entry, outcome.dispatched);
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        if (error instanceof SubmissionConflictError) throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: error });
        throw error;
      }
    },
    async reconcilePending(): Promise<TaskStartReceipt[]> {
      const lease = requireLease();
      const scope = submissionScopeOf(lease);
      const records = await intentLog.listScope(scope);
      const receipts: TaskStartReceipt[] = [];
      for (const record of records) {
        // 重启恢复：进程内 store 为空时按意图记录预建 awaiting_reconciliation entry
        // （digest 与原提交一致——同一 sessionId + 同一 goal），再走 lookup 对账。
        if (submissionStore.load(record.requestId) === undefined) {
          const input = toStartInput(goalDraftOf(record.goal), { requestID: record.requestId, sessionId: record.sessionId, targetId: 'platform', workspaceRef: '' });
          submissionStore.save({ request_id: record.requestId, input_digest: inputDigest(input), scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() });
        }
        const entry = await submissions.reconcile(record.requestId, scope);
        if (entry.phase === 'bound') await intentLog.remove?.(record.requestId);
        receipts.push(receiptOf(entry, false));
      }
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return receipts;
    },
    /** T14（#44）追加区。 */
    async legacyTasks(query: { search?: string; archived?: boolean; limit?: number }): Promise<LegacyTaskListPage> {
      const epoch = ++legacyListEpoch;
      const lease = requireLease();
      const input = normalizeLegacyQuery(query);
      const page = settle(epoch, 'legacy', lease, await callBackend(() => requireLegacy().list(input)));
      const state: legacyListAccumulation = { input, seen: new Set<string>(), duplicates: [] };
      legacyAccumulated = state;
      return accumulateLegacy(state, page);
    },
    async moreLegacyTasks(): Promise<LegacyTaskListPage> {
      const state = legacyAccumulated;
      if (state === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_ACTIVE_QUERY');
      if (state.cursor === undefined) return { items: [], duplicateTaskIds: [...state.duplicates] };
      const epoch = ++legacyListEpoch;
      const lease = requireLease();
      const page = settle(epoch, 'legacy', lease, await callBackend(() => requireLegacy().list({ ...state.input, cursor: state.cursor })));
      return accumulateLegacy(state, page);
    },
    async legacyHistory(taskId: string): Promise<LegacyMessage[]> {
      const lease = requireLease();
      const trimmed = taskId.trim();
      if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      const messages = await callBackend(() => requireLegacy().history(trimmed));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return messages;
    },
    async followUp(input: LegacyFollowUpInput): Promise<void> {
      const lease = requireLease();
      const taskId = (input?.taskId ?? '').trim();
      const question = (input?.question ?? '').trim();
      if (taskId === '' || question === '' || question.length > 8000) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      await callBackend(() => requireLegacy().followUp({ taskId, question, ...(input.signal === undefined ? {} : { signal: input.signal }) }));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      // 追问改变了会话 updated_at：一切在途读失效（与 archive/restore 同规则）。
      listEpoch += 1;
      homeEpoch += 1;
      legacyListEpoch += 1;
      accumulated = undefined;
      legacyAccumulated = undefined;
    },
  };
}
