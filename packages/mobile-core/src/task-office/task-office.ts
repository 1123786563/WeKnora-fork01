import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createInMemoryTaskProjectionStore } from './in-memory-task-detail.ts';
import { createTaskDetail } from './task-detail.ts';
import type { TaskDetailBackendPort, TaskHandle, TaskProjectionStore } from './task-detail.ts';

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

export type AttentionState = 'none' | 'required';
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

export interface TaskBackendPort {
  overview(): Promise<TaskBackendOverview>;
  list(input: TaskBackendListInput): Promise<TaskBackendPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
}

export interface TaskOfficePorts {
  backend: TaskBackendPort;
  lease(): ScopeLease | undefined;
  /** T05 详情与 SSE 恢复端口；缺失时 open() fail closed（TASK_OFFICE_DETAIL_UNAVAILABLE）。 */
  detail?: TaskDetailBackendPort;
  /** 持久化投影存储（App 重启恢复）；缺省为 office 内共享的 in-memory store。 */
  store?: TaskProjectionStore;
}

export type TaskOfficeErrorCode =
  | 'TASK_OFFICE_SCOPE_CHANGED'
  | 'TASK_OFFICE_SUPERSEDED'
  | 'TASK_OFFICE_NO_ACTIVE_QUERY'
  | 'TASK_OFFICE_INVALID_INPUT'
  | 'TASK_OFFICE_BACKEND'
  | 'TASK_OFFICE_DETAIL_UNAVAILABLE'
  | 'TASK_OFFICE_DETAIL_CLOSED';

export class TaskOfficeError extends Error {
  constructor(readonly code: TaskOfficeErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'TaskOfficeError';
  }
}

export interface TaskOffice {
  home(): Promise<HomeView>;
  tasks(query: TaskOfficeQuery): Promise<TaskListPage>;
  moreTasks(): Promise<TaskListPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
  open(input: { taskId: string; runId: string }): TaskHandle;
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
  const settle = <T>(epoch: number, kind: 'home' | 'list', lease: ScopeLease, value: T): T => {
    const currentEpoch = kind === 'home' ? homeEpoch : listEpoch;
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
    accumulated = undefined;
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
    open(taskOpen: { taskId: string; runId: string }): TaskHandle {
      const taskId = taskOpen.taskId.trim();
      const runId = taskOpen.runId.trim();
      if (taskId === '' || runId === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      if (ports.detail === undefined) throw new TaskOfficeError('TASK_OFFICE_DETAIL_UNAVAILABLE');
      return createTaskDetail({ taskId, runId }, { backend: ports.detail, store: ports.store ?? defaultDetailStore, lease: ports.lease });
    },
  };
}
