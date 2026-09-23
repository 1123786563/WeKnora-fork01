import { leaseActive } from '../runtime/scope-lease.ts';
import { TaskOfficeError } from './task-office-errors.ts'; // R1-F21：不再从 task-office.ts 值导入（解运行时环）
import { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf } from './task-timeline.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { TaskLifecycleState, TaskTimelineEntry } from './task-timeline.ts';
import type { AttentionState } from './task-office-errors.ts';

export const TASK_DETAIL_HISTORY_LIMIT = 200;

export type TaskConnectionState = 'syncing' | 'live' | 'interrupted' | 'drained';
export type TaskInterruptionReason = 'gap' | 'cursor-expired' | 'stream-error' | 'stream-ended-nonterminal' | 'persist-failed' | 'stream-unavailable';

export interface TaskBackendEvent {
  runId: string;
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export interface TaskBackendDetail {
  taskId: string;
  runId: string;
  title: string;
  attention: AttentionState;
  archivedAt?: string;
  execution: { runStatus: string; executionStatus: string; settlementStatus: string; revision: number; seq: number };
  watermark: number;
  incomplete: boolean;
  events: TaskBackendEvent[];
}

export interface TaskStreamControlFrame { code: string; message: string }

export interface TaskDetailBackendPort {
  detail(runId: string): Promise<TaskBackendDetail>;
  /** Resolves when the server ends the stream; rejects on transport error. Cursor trimming rejects with an Error whose code is 'TASK_STREAM_CURSOR_EXPIRED'. */
  stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }): Promise<void>;
}

export interface PersistedTaskProjection {
  taskId: string;
  runId: string;
  cursor: number;
  events: TaskBackendEvent[];
  savedAt: string;
}

export interface TaskProjectionStore {
  load(runId: string): Promise<PersistedTaskProjection | undefined>;
  save(projection: PersistedTaskProjection): Promise<void>;
}

export interface TaskDetailView {
  taskId: string;
  runId: string;
  title: string;
  lifecycle: TaskLifecycleState;
  runStatus: string;
  attention: AttentionState;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  cursor: number;
  incomplete: boolean;
  connection: TaskConnectionState;
  interruption?: { reason: TaskInterruptionReason; message?: string };
  timeline: TaskTimelineEntry[];
  duplicateSeqs: number[];
}

export interface TaskHandle {
  hydrate(): Promise<TaskDetailView>;
  view(): TaskDetailView | undefined;
  updates(listener: (view: TaskDetailView) => void): () => void;
  resync(): Promise<TaskDetailView>;
  close(reason?: string): void;
}

export interface TaskDetailPorts {
  backend: TaskDetailBackendPort;
  store: TaskProjectionStore;
  lease(): ScopeLease | undefined;
}

const AUTO_RESYNC_LIMIT = 2;
// persist 写放大治理（R1-F23）：内存态每事件推进，仅按 50 事件跨度或终态/中断落盘。
// 崩溃安全权衡：重启丢失的窗口 ≤ 50 个事件，hydrate 的 mergeEventHistory 用权威快照兜底。
const PERSIST_MIN_STRIDE = 50;
// chain 背压上限：在途步骤超限时中止流并置 gap——无界排队会同时拖垮内存与游标一致性。
const CHAIN_DEPTH_LIMIT = 1000;

// input.taskId 契约（B2-F26）：仅用于调用方关联；持久化写入服务端权威的 detail.taskId
// （persist 内 detail!.taskId），故本函数不读取 input.taskId，这是有意为之。
export function createTaskDetail(input: { taskId: string; runId: string }, ports: TaskDetailPorts): TaskHandle {
  let closed = false;
  let current: TaskDetailView | undefined;
  let detail: TaskBackendDetail | undefined;
  let events: TaskBackendEvent[] = [];
  let committedCursor = 0;
  let duplicateSeqs: number[] = [];
  let interruption: TaskDetailView['interruption'];
  let controller: AbortController | undefined;
  let streamEpoch = 0;
  let chain: Promise<void> = Promise.resolve();
  let chainDepth = 0;
  let persistedCursor = 0; // 最近一次成功落盘的游标（stride 合并的基准）
  let autoResyncs = 0;
  const listeners = new Set<(view: TaskDetailView) => void>();

  const requireOpen = (): void => {
    if (closed) throw new TaskOfficeError('TASK_OFFICE_DETAIL_CLOSED');
  };
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const wrap = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const buildView = (connection: TaskConnectionState): TaskDetailView => {
    const base = detail!;
    const runStatus = terminalRunStatusOf(base.execution.runStatus, events);
    const terminal = isTerminalRunStatus(runStatus);
    return {
      taskId: base.taskId,
      runId: base.runId,
      title: base.title,
      lifecycle: taskLifecycleOf(base.archivedAt, runStatus),
      runStatus,
      attention: base.attention,
      // 镜像 internal/application/repository/agent_run_snapshot.go:83-88（executionFromRun）：
      // runStatus 为终态（succeeded/failed/canceled）即 settlement='settled'、executionStatus=runStatus。
      // 快照未刷新前流内终态事件也要推进视图，不得停留在旧 'pending'（R1-F27）。
      executionStatus: terminal ? runStatus : base.execution.executionStatus,
      settlementStatus: terminal ? 'settled' : base.execution.settlementStatus,
      revision: base.execution.revision,
      cursor: committedCursor,
      incomplete: base.incomplete,
      connection,
      ...(interruption === undefined ? {} : { interruption }),
      timeline: projectTimeline(events),
      duplicateSeqs: [...duplicateSeqs],
    };
  };
  const notify = (connection: TaskConnectionState): void => {
    if (closed || detail === undefined) return;
    current = buildView(connection);
    for (const listener of [...listeners]) {
      try {
        listener(current);
      } catch {
        /* 订阅者异常不截断广播（R1-F24）：其余订阅者仍收到通知 */
      }
    }
  };
  // 四个流式入口共用（R1-F44）：lease 失效即静默停流（abort + 不再 notify），
  // 对齐模块注释「scope lease 撤销后的一切结果按 TASK_OFFICE_SCOPE_CHANGED 拒绝」的流内语义。
  const streamGuard = (): boolean => {
    if (closed) return false;
    const lease = ports.lease();
    if (!lease || !leaseActive(lease)) {
      abortStream();
      return false;
    }
    return true;
  };
  const abortStream = (): void => {
    streamEpoch += 1;
    controller?.abort();
    controller = undefined;
  };
  const persist = async (cursor: number, history: TaskBackendEvent[]): Promise<void> => {
    await ports.store.save({ taskId: detail!.taskId, runId: input.runId, cursor, events: history.slice(-TASK_DETAIL_HISTORY_LIMIT), savedAt: new Date().toISOString() });
  };
  /** best-effort 强制 flush（终态/中断/close 前）：只补落未落盘的跨度，失败不抛——失败可见性由 interruption 表达。 */
  const flushPersisted = async (): Promise<void> => {
    if (committedCursor === persistedCursor || detail === undefined) return;
    try {
      await persist(committedCursor, events);
      persistedCursor = committedCursor;
    } catch {
      /* best-effort：内存投影仍正确，重启后由 hydrate 兜底 */
    }
  };
  const hydrate = async (): Promise<TaskDetailView> => {
    requireOpen();
    const lease = requireLease();
    // 进入即递增 streamEpoch：作废在途流与并发旧 hydrate（interrupt/resync 已各自先 abort，此处为幂等加固）。
    abortStream();
    const epoch = streamEpoch;
    if (detail !== undefined) notify('syncing');
    const fetched = await wrap(() => ports.backend.detail(input.runId));
    if (epoch !== streamEpoch || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    detail = fetched;
    const persisted = await ports.store.load(input.runId).catch(() => undefined);
    if (epoch !== streamEpoch) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    events = mergeEventHistory(persisted?.events ?? [], fetched.events, fetched.watermark).slice(-TASK_DETAIL_HISTORY_LIMIT);
    committedCursor = fetched.watermark;
    duplicateSeqs = [];
    interruption = undefined;
    try {
      await persist(committedCursor, events);
      persistedCursor = committedCursor; // stride 基准重置为水合成功落盘的水位
    } catch (cause) {
      interruption = { reason: 'persist-failed', message: cause instanceof Error ? cause.message : String(cause) };
    }
    if (epoch !== streamEpoch) {
      // persist 挂起点期间被取代：不再 startStream/notify；从未发布过视图时如实拒绝。
      if (current === undefined) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return current;
    }
    if (isTerminalRunStatus(terminalRunStatusOf(fetched.execution.runStatus, events))) {
      notify('drained');
      return current!;
    }
    if (interruption !== undefined) {
      notify('interrupted');
      return current!;
    }
    startStream();
    notify('live');
    return current!;
  };
  const interrupt = async (reason: TaskInterruptionReason, message?: string): Promise<void> => {
    abortStream();
    await flushPersisted(); // 中断前强制落盘未持久跨度（R1-F23：best-effort）
    interruption = { reason, ...(message === undefined ? {} : { message }) };
    notify('interrupted'); // 缺口/裁剪/持久化失败先可见，绝不静默
    if (autoResyncs >= AUTO_RESYNC_LIMIT) return;
    autoResyncs += 1;
    try {
      await hydrate();
    } catch {
      /* 保持 interrupted 已通知状态；显式 resync() 可再试 */
    }
  };
  const enqueueChain = (step: () => Promise<void>): void => {
    if (chainDepth > CHAIN_DEPTH_LIMIT) {
      abortStream();
      void interrupt('gap', 'stream queue overflow');
      return;
    }
    chainDepth += 1;
    chain = chain.then(step).catch(() => undefined).finally(() => { chainDepth -= 1; });
  };
  const startStream = (): void => {
    abortStream();
    const epoch = ++streamEpoch;
    controller = new AbortController();
    void ports.backend.stream({
      runId: input.runId,
      cursor: committedCursor,
      signal: controller.signal,
      onEvent: (event) => { if (epoch === streamEpoch) enqueueChain(() => processEvent(event)); },
      onControl: (frame) => { if (epoch === streamEpoch) enqueueChain(() => processControl(frame)); },
    }).then(
      () => { if (epoch === streamEpoch) enqueueChain(() => streamEnded()); },
      (error) => { if (epoch === streamEpoch) enqueueChain(() => streamFailed(error)); },
    );
  };
  const processEvent = async (event: TaskBackendEvent): Promise<void> => {
    if (!streamGuard() || detail === undefined) return;
    const stepEpoch = streamEpoch; // 步骤开始时的快照：persist 挂起点期间被 resync/interrupt 取代则丢弃回写
    if (event.runId !== input.runId) {
      await interrupt('stream-error', `event belongs to run ${event.runId}`);
      return;
    }
    if (!Number.isSafeInteger(event.seq) || event.seq < 1) {
      await interrupt('stream-error', `invalid event sequence ${String(event.seq)}`);
      return;
    }
    if (event.seq <= committedCursor) {
      duplicateSeqs = [...duplicateSeqs, event.seq].slice(-50); // 重复事件幂等跳过，可观测但有界（B2-F27）
      notify(current?.connection === 'drained' ? 'drained' : 'live');
      return;
    }
    if (event.seq !== committedCursor + 1) {
      await interrupt('gap', `expected seq ${committedCursor + 1}, received ${event.seq}`);
      return;
    }
    const nextEvents = [...events, event].slice(-TASK_DETAIL_HISTORY_LIMIT);
    // 挂起点期间被取代（resync 开了新流 / lease 撤销）：陈旧回写不得回退游标（R1-F43，对齐 hydrate 的既有守卫语义）。
    if (stepEpoch !== streamEpoch || !streamGuard()) return;
    // R1-F23 写放大治理：内存先推进；仅跨度 ≥ 50 或终态时落盘（行为变更点——内存态推进
    // 不再以 persist 成功为前提，崩溃窗口 ≤ 50 事件由 hydrate 的 mergeEventHistory 兜底）。
    events = nextEvents;
    committedCursor = event.seq;
    autoResyncs = 0;
    const terminal = isTerminalRunStatus(terminalRunStatusOf(detail!.execution.runStatus, events));
    if (terminal || event.seq - persistedCursor >= PERSIST_MIN_STRIDE) {
      try {
        await persist(event.seq, nextEvents);
        persistedCursor = event.seq;
      } catch (cause) {
        await interrupt('persist-failed', cause instanceof Error ? cause.message : String(cause));
        return;
      }
    }
    notify('live');
  };
  const processControl = async (frame: TaskStreamControlFrame): Promise<void> => {
    if (!streamGuard()) return;
    if (frame.code === 'heartbeat' || frame.code === 'keepalive') return; // 保活帧良性（R1-F45），不打断流
    if (frame.code === 'cursor_expired') {
      await interrupt('cursor-expired', frame.message);
      return;
    }
    await interrupt('stream-error', `${frame.code}: ${frame.message}`);
  };
  const streamEnded = async (): Promise<void> => {
    if (!streamGuard() || detail === undefined) return;
    if (isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events))) {
      await flushPersisted(); // 终态必已落盘（幂等：processEvent 已 flush 时跳过）
      interruption = undefined;
      notify('drained');
      return;
    }
    await interrupt('stream-ended-nonterminal', 'the event stream ended before a terminal status');
  };
  const streamFailed = async (error: unknown): Promise<void> => {
    if (!streamGuard()) return;
    const message = error instanceof Error ? error.message : String(error);
    // code 优先（R1-F22）：跨包错误合同靠结构化 code，message 全等仅作过渡兼容。
    const code = (error as { code?: unknown } | null)?.code;
    if (code === 'RUNTIME_STREAM_UNAVAILABLE' || message === 'RUNTIME_STREAM_UNAVAILABLE') {
      // 流通道缺失（runtime authorizedStream 未解析出 transport）不是瞬时故障：
      // 直接置为 interrupted 并停止自动 resync，REST 详情仍可用，显式 resync() 可再试。
      interruption = { reason: 'stream-unavailable', message: '此部署未提供实时流通道，可手动刷新同步' };
      autoResyncs = AUTO_RESYNC_LIMIT;
      notify('interrupted');
      return;
    }
    if (code === 'TASK_STREAM_CURSOR_EXPIRED') {
      await interrupt('cursor-expired', message);
      return;
    }
    await interrupt('stream-error', message);
  };
  return {
    // async 包装：close 后的调用必须以 rejected Promise 形式拒绝（assert.rejects 契约），而不是同步抛错。
    hydrate: async () => { requireOpen(); return hydrate(); },
    view: () => current,
    updates(listener) {
      if (closed) return () => undefined;
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    async resync(): Promise<TaskDetailView> {
      requireOpen();
      autoResyncs = 0; // 显式重同步解除有界自动重连的上限
      abortStream();
      return hydrate();
    },
    close(reason?: string) {
      closed = true;
      abortStream();
      void flushPersisted(); // close 强制落盘未持久跨度（best-effort，R1-F23）
      listeners.clear();
      void reason;
    },
  };
}
