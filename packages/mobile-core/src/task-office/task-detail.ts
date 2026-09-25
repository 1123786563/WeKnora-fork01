import { leaseActive } from '../runtime/scope-lease.ts';
import { TaskOfficeError } from './task-office-errors.ts'; // R1-F21：不再从 task-office.ts 值导入（解运行时环）
import { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf } from './task-timeline.ts';
import { resolveUnknownStop } from './task-intent.ts';
import type { InterventionReceipt, StopPhase, TaskIntent } from './task-intent.ts';
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
  stop?: { phase: StopPhase; since: string; note?: string };
  queuedNext?: Array<{ intentId: string; text: string; queuedAt: string }>;
  interventions?: InterventionReceipt[];
}

export type TaskCommandAction = 'steer' | 'queue_next' | 'cancel';

/** T07 命令 seam：实现方（api-client remote）以结构化 `code` 错误表达确定性冲突
 * （TASK_COMMAND_CONFLICT）与投递结果未知（TASK_COMMAND_UNKNOWN）。 */
export interface TaskCommandPort {
  command(input: { runId: string; action: TaskCommandAction; text?: string; expectedRevision: number; intentId?: string }): Promise<{ runId: string; action: TaskCommandAction; nextRunId?: string }>;
}

export interface TaskHandle {
  hydrate(): Promise<TaskDetailView>;
  view(): TaskDetailView | undefined;
  updates(listener: (view: TaskDetailView) => void): () => void;
  resync(): Promise<TaskDetailView>;
  /** T07：受控干预（steer / queue-next / stop）；返回诚实回执，绝不编造服务端准入。 */
  act(intent: TaskIntent): Promise<InterventionReceipt>;
  /** T07：把 parked 的 queue-next 逐条发往服务端（观察到终态后调用/由 hydrate 自动触发）。 */
  flushQueuedIntents(): Promise<void>;
  close(reason?: string): void;
}

export interface TaskDetailPorts {
  backend: TaskDetailBackendPort;
  store: TaskProjectionStore;
  lease(): ScopeLease | undefined;
  /** T07 干预通道；缺失时 act() fail closed（TASK_OFFICE_COMMAND_UNAVAILABLE）。 */
  commands?: TaskCommandPort;
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
  // —— T07（#37）干预状态：三态停止、unknown 门、parked queue-next、诚实回执 ——
  let stopState: TaskDetailView['stop'];
  let unknownGate: { revision: number } | undefined;
  let revisionFloor = 0; // 202 后服务端 CAS 证明的 revision+1（steer/cancel 各 +1）
  const queuedNext: Array<{ intentId: string; text: string; queuedAt: string }> = [];
  const interventions: InterventionReceipt[] = [];
  let flushInFlight: Promise<void> | undefined; // hydrate 自动 flush 与显式 flush 合流，绝不重复派发同一 parked 意图
  const commandRevision = (): number => Math.max(detail?.execution.revision ?? 0, revisionFloor);
  const currentRunStatus = (): string => (detail === undefined ? '' : terminalRunStatusOf(detail.execution.runStatus, events));
  const nextIntentId = (): string => {
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('no id generator on this platform') });
  };
  const trimInterventions = (): void => { if (interventions.length > 20) interventions.splice(0, interventions.length - 20); };
  const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));
  const wrapCommand = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      const code = (error as { code?: unknown } | null)?.code;
      if (code === 'TASK_COMMAND_CONFLICT' || code === 'TASK_COMMAND_UNKNOWN') throw error; // 跨包契约码透传（#38 先例）
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const stopProjection = (runStatus: string): TaskDetailView['stop'] => {
    if (stopState === undefined) return undefined;
    if (stopState.phase === 'unknown') return stopState;
    if (isTerminalRunStatus(runStatus)) {
      return { ...stopState, phase: 'confirmed', ...(runStatus === 'canceled' ? {} : { note: `run ended as ${runStatus} before the stop landed` }) };
    }
    return stopState;
  };

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
    const stop = stopProjection(runStatus);
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
      ...(stop === undefined ? {} : { stop }),
      ...(queuedNext.length === 0 ? {} : { queuedNext: [...queuedNext] }),
      ...(interventions.length === 0 ? {} : { interventions: [...interventions] }),
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
  const pumpQueuedNext = async (lease: ScopeLease): Promise<void> => {
    while (queuedNext.length > 0 && unknownGate === undefined) {
      if (!isTerminalRunStatus(currentRunStatus())) return;
      const next = queuedNext[0]!;
      const dispatch = { runId: input.runId, action: 'queue_next' as const, text: next.text, expectedRevision: commandRevision(), intentId: next.intentId };
      try {
        const ack = await wrapCommand(() => ports.commands!.command(dispatch));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED'); // 迟到结果拒绝（§5.3，与 act() 同一竞态同一处理）
        queuedNext.shift();
        interventions.push({ intent: { kind: 'queue-next', text: next.text, intentId: next.intentId }, outcome: 'accepted', boundRunId: input.runId, revision: dispatch.expectedRevision, ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at: new Date().toISOString() });
      } catch (error) {
        if (error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED') throw error; // scope 死亡：不 shift、不记回执，flush 整体如实拒绝
        // one-shot：一次失败的 flush 不重试、不静默丢弃——以回执如实呈现后移除。
        queuedNext.shift();
        const code = (error as { code?: unknown } | null)?.code;
        interventions.push({ intent: { kind: 'queue-next', text: next.text, intentId: next.intentId }, outcome: code === 'TASK_COMMAND_CONFLICT' ? 'conflict' : 'unknown', boundRunId: input.runId, revision: dispatch.expectedRevision, note: messageOf(error), at: new Date().toISOString() });
        if (code !== 'TASK_COMMAND_CONFLICT') unknownGate = { revision: dispatch.expectedRevision };
      }
      trimInterventions();
    }
    if (current !== undefined) notify(current.connection);
  };
  /** hydrate 终态分支与显式调用共用同一在途 flush：并发调用合流，同一 parked 意图只派发一次。 */
  const flushQueuedIntents = async (): Promise<void> => {
    requireOpen();
    const lease = requireLease(); // §5.3：scope 已死亡的 flush fail closed（hydrate 自身已先过同一门禁）
    if (ports.commands === undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
    if (flushInFlight !== undefined) return flushInFlight;
    const run = pumpQueuedNext(lease);
    flushInFlight = run.finally(() => { flushInFlight = undefined; });
    await flushInFlight;
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
    if (unknownGate !== undefined) {
      const resolved = resolveUnknownStop(fetched.execution.runStatus);
      if (resolved === 'confirmed') {
        stopState = { phase: 'confirmed', since: stopState?.since ?? new Date().toISOString(), note: 'reconciled: canceled' };
      } else {
        stopState = undefined; // 取消未落地：门解除，用户可重试。
      }
      unknownGate = undefined;
    }
    if (isTerminalRunStatus(terminalRunStatusOf(fetched.execution.runStatus, events))) {
      notify('drained');
      void flushQueuedIntents().catch(() => undefined); // 终态观察即放行 parked queue-next（一次性，失败不重试）
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
    async act(intent: TaskIntent): Promise<InterventionReceipt> {
      requireOpen();
      const lease = requireLease(); // 捕获在途 lease：命令飞行期间撤销 → 迟到结果按 SCOPE_CHANGED 拒绝（§5.3）
      const commands = ports.commands;
      if (commands === undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
      if (detail === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_SNAPSHOT');
      if (unknownGate !== undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNKNOWN');
      const text = intent.kind === 'stop' ? '' : intent.text.trim();
      if ((intent.kind === 'steer' || intent.kind === 'queue-next') && text === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      const revision = commandRevision();
      const at = new Date().toISOString();
      if (intent.kind === 'queue-next' && !isTerminalRunStatus(currentRunStatus())) {
        // 单写者规则：活动 Run 占据写通道，queue_next 由模块本地 parked，观察到终态后再发出。
        const parked = { intentId: intent.intentId ?? nextIntentId(), text, queuedAt: at };
        queuedNext.push(parked);
        const receipt: InterventionReceipt = { intent, outcome: 'parked', boundRunId: input.runId, revision, at };
        interventions.push(receipt); trimInterventions();
        notify(current?.connection === undefined ? 'syncing' : current.connection);
        return receipt;
      }
      const intentId = intent.kind === 'queue-next' ? intent.intentId ?? nextIntentId() : undefined;
      const dispatch = intent.kind === 'stop'
        ? { runId: input.runId, action: 'cancel' as const, expectedRevision: revision }
        : intent.kind === 'steer'
          ? { runId: input.runId, action: 'steer' as const, text, expectedRevision: revision }
          : { runId: input.runId, action: 'queue_next' as const, text, expectedRevision: revision, ...(intentId === undefined ? {} : { intentId }) };
      try {
        const ack = await wrapCommand(() => commands.command(dispatch));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED'); // 迟到结果拒绝（§5.3）
        if (dispatch.action === 'steer' || dispatch.action === 'cancel') revisionFloor = revision + 1; // 服务端 CAS 证明
        if (dispatch.action === 'cancel') stopState = { phase: 'requested', since: at };
        if (dispatch.action === 'queue_next') {
          const index = queuedNext.findIndex((q) => q.intentId === intentId);
          if (index >= 0) queuedNext.splice(index, 1);
        }
        const receipt: InterventionReceipt = {
          intent, outcome: 'accepted', boundRunId: ack.runId === '' ? input.runId : ack.runId, revision,
          ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at,
        };
        interventions.push(receipt); trimInterventions();
        void hydrate().catch(() => undefined); // 重观察（一次；失败由流/下次 hydrate 兜底）
        return receipt;
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        const code = (error as { code?: unknown } | null)?.code;
        if (code === 'TASK_COMMAND_CONFLICT') {
          const receipt: InterventionReceipt = { intent, outcome: 'conflict', boundRunId: input.runId, revision, note: messageOf(error), at };
          interventions.push(receipt); trimInterventions();
          notify(current?.connection === undefined ? 'syncing' : current.connection); // 冲突回执立即可见
          void hydrate().catch(() => undefined);
          return receipt;
        }
        // 结果未知（传输失败/5xx/502 command_recovery_unknown）：进入 unknown 门（AC2）。
        if (intent.kind === 'stop') stopState = { phase: 'unknown', since: at, note: messageOf(error) };
        unknownGate = { revision };
        const receipt: InterventionReceipt = { intent, outcome: 'unknown', boundRunId: input.runId, revision, note: messageOf(error), at };
        interventions.push(receipt); trimInterventions();
        notify(current?.connection === undefined ? 'syncing' : current.connection); // 停止卡/回执立即可见
        return receipt;
      }
    },
    flushQueuedIntents(): Promise<void> {
      return flushQueuedIntents();
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
