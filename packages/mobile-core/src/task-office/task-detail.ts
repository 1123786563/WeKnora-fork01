import { leaseActive } from '../runtime/scope-lease.ts';
import { TaskOfficeError } from './task-office.ts';
import { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf } from './task-timeline.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { TaskLifecycleState, TaskTimelineEntry } from './task-timeline.ts';
import type { AttentionState } from './task-office.ts';

export const TASK_DETAIL_HISTORY_LIMIT = 200;

export type TaskConnectionState = 'syncing' | 'live' | 'interrupted' | 'drained';
export type TaskInterruptionReason = 'gap' | 'cursor-expired' | 'stream-error' | 'stream-ended-nonterminal' | 'persist-failed';

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
    return {
      taskId: base.taskId,
      runId: base.runId,
      title: base.title,
      lifecycle: taskLifecycleOf(base.archivedAt, runStatus),
      runStatus,
      attention: base.attention,
      executionStatus: base.execution.executionStatus,
      settlementStatus: base.execution.settlementStatus,
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
    for (const listener of [...listeners]) listener(current);
  };
  const abortStream = (): void => {
    streamEpoch += 1;
    controller?.abort();
    controller = undefined;
  };
  const persist = async (cursor: number, history: TaskBackendEvent[]): Promise<void> => {
    await ports.store.save({ taskId: detail!.taskId, runId: input.runId, cursor, events: history.slice(-TASK_DETAIL_HISTORY_LIMIT), savedAt: new Date().toISOString() });
  };
  const hydrate = async (): Promise<TaskDetailView> => {
    requireOpen();
    const lease = requireLease();
    if (detail !== undefined) notify('syncing');
    const fetched = await wrap(() => ports.backend.detail(input.runId));
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    detail = fetched;
    const persisted = await ports.store.load(input.runId).catch(() => undefined);
    events = mergeEventHistory(persisted?.events ?? [], fetched.events, fetched.watermark).slice(-TASK_DETAIL_HISTORY_LIMIT);
    committedCursor = fetched.watermark;
    duplicateSeqs = [];
    interruption = undefined;
    try {
      await persist(committedCursor, events);
    } catch (cause) {
      interruption = { reason: 'persist-failed', message: cause instanceof Error ? cause.message : String(cause) };
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
  const startStream = (): void => {
    abortStream();
    const epoch = ++streamEpoch;
    controller = new AbortController();
    void ports.backend.stream({
      runId: input.runId,
      cursor: committedCursor,
      signal: controller.signal,
      onEvent: (event) => { chain = chain.then(() => processEvent(event)).catch(() => undefined); },
      onControl: (frame) => { chain = chain.then(() => processControl(frame)).catch(() => undefined); },
    }).then(
      () => { if (epoch === streamEpoch) chain = chain.then(() => streamEnded()).catch(() => undefined); },
      (error) => { if (epoch === streamEpoch) chain = chain.then(() => streamFailed(error)).catch(() => undefined); },
    );
  };
  const processEvent = async (event: TaskBackendEvent): Promise<void> => {
    if (closed || detail === undefined) return;
    if (event.runId !== input.runId) {
      await interrupt('stream-error', `event belongs to run ${event.runId}`);
      return;
    }
    if (!Number.isSafeInteger(event.seq) || event.seq < 1) {
      await interrupt('stream-error', `invalid event sequence ${String(event.seq)}`);
      return;
    }
    if (event.seq <= committedCursor) {
      duplicateSeqs = [...duplicateSeqs, event.seq]; // 重复事件幂等跳过，但可观测
      notify(current?.connection === 'drained' ? 'drained' : 'live');
      return;
    }
    if (event.seq !== committedCursor + 1) {
      await interrupt('gap', `expected seq ${committedCursor + 1}, received ${event.seq}`);
      return;
    }
    const nextEvents = [...events, event].slice(-TASK_DETAIL_HISTORY_LIMIT);
    try {
      await persist(event.seq, nextEvents); // 先持久化，后推进已提交游标
    } catch (cause) {
      await interrupt('persist-failed', cause instanceof Error ? cause.message : String(cause));
      return;
    }
    events = nextEvents;
    committedCursor = event.seq;
    autoResyncs = 0;
    notify('live');
  };
  const processControl = async (frame: TaskStreamControlFrame): Promise<void> => {
    if (closed) return;
    if (frame.code === 'cursor_expired') {
      await interrupt('cursor-expired', frame.message);
      return;
    }
    await interrupt('stream-error', `${frame.code}: ${frame.message}`);
  };
  const streamEnded = async (): Promise<void> => {
    if (closed || detail === undefined) return;
    if (isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events))) {
      interruption = undefined;
      notify('drained');
      return;
    }
    await interrupt('stream-ended-nonterminal', 'the event stream ended before a terminal status');
  };
  const streamFailed = async (error: unknown): Promise<void> => {
    if (closed) return;
    const code = (error as { code?: unknown } | null)?.code;
    const message = error instanceof Error ? error.message : String(error);
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
      listeners.clear();
      void reason;
    },
  };
}
