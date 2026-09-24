import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createInMemoryTaskProjectionStore, createScenarioTaskDetailBackend, createScriptedTaskStream } from './in-memory-task-detail.ts';
import { createTaskDetail } from './task-detail.ts';
import type { TaskBackendDetail, TaskBackendEvent, TaskCommandPort, TaskDetailBackendPort, TaskProjectionStore, TaskStreamControlFrame } from './task-detail.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

const settle = async (rounds = 12): Promise<void> => {
  for (let index = 0; index < rounds; index += 1) await new Promise<void>((resolve) => setImmediate(resolve));
};

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

const event$ = (seq: number, type = 'text.delta'): TaskBackendEvent => ({ runId: 'run-1', seq, type, occurredAt: '2026-09-23T00:00:00Z', payload: { note: seq } });

function detail$(overrides: Partial<TaskBackendDetail> = {}): TaskBackendDetail {
  return {
    taskId: 'task-1', runId: 'run-1', title: '季度竞品报告', attention: 'required',
    execution: { runStatus: 'running', executionStatus: 'running', settlementStatus: 'pending', revision: 4, seq: 2 },
    watermark: 2, incomplete: false, events: [event$(1, 'run.started'), event$(2, 'tool.started')],
    ...overrides,
  };
}

function officeWithDetail(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioTaskDetailBackend>[0] = {}, store?: TaskProjectionStore) {
  const detailBackend = createScenarioTaskDetailBackend(handlers);
  const sharedStore = store ?? createInMemoryTaskProjectionStore();
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
    detail: detailBackend,
    store: sharedStore,
  });
  return { backend: detailBackend, store: sharedStore, office };
}

test('open rejects blank ids, a missing detail port, and use after close', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  assert.throws(() => office.open({ taskId: ' ', runId: 'run-1' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  const plain = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
  });
  assert.throws(() => plain.open({ taskId: 'task-1', runId: 'run-1' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_DETAIL_UNAVAILABLE');
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  handle.close('test');
  await assert.rejects(handle.hydrate(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_DETAIL_CLOSED');
});

test('hydrate renders the three layers result-first and keeps the timeline factual', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  assert.equal(handle.view(), undefined, 'no view before hydration');
  const views: unknown[] = [];
  const unsubscribe = handle.updates((view) => views.push(view));
  const view = await handle.hydrate();
  assert.equal(view.taskId, 'task-1');
  assert.equal(view.title, '季度竞品报告');
  assert.equal(view.lifecycle, 'active');
  assert.equal(view.runStatus, 'running');
  assert.equal(view.attention, 'required');
  assert.equal(view.executionStatus, 'running');
  assert.equal(view.settlementStatus, 'pending');
  assert.equal(view.revision, 4);
  assert.equal(view.cursor, 2);
  assert.equal(view.incomplete, false);
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2]);
  assert.equal(view.timeline[0]!.kind, 'run_status');
  assert.equal(view.timeline[1]!.kind, 'tool_activity');
  assert.equal(views.length, 1, 'hydration notifies subscribers exactly once with the settled view');
  assert.equal(view.connection, 'live', 'a non-terminal run opens the event stream');
  assert.deepEqual(view.duplicateSeqs, []);
  unsubscribe();
  handle.close();
});

test('a terminal or archived task drains without opening a stream', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => detail$({
      archivedAt: '2026-09-23T01:00:00Z',
      attention: 'none',
      execution: { runStatus: 'succeeded', executionStatus: 'succeeded', settlementStatus: 'settled', revision: 9, seq: 2 },
    }),
  });
  const view = await office.open({ taskId: 'task-1', runId: 'run-1' }).hydrate();
  assert.equal(view.lifecycle, 'archived', 'archive wins over completed');
  assert.equal(view.runStatus, 'succeeded');
  assert.equal(view.connection, 'drained');
  assert.equal(backend.streams.length, 0, 'terminal runs never subscribe');
});

test('hydrate merges the persisted projection with the authoritative snapshot', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 2, events: [event$(1, 'run.started'), event$(2, 'tool.started')], savedAt: '2026-09-23T00:00:00Z' });
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$({ watermark: 3, events: [event$(3, 'run.completed')] }),
  }, store);
  const view = await office.open({ taskId: 'task-1', runId: 'run-1' }).hydrate();
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3], 'hydration unions persisted and snapshot history');
  assert.equal(view.cursor, 3);
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 3, 'hydration persists the merged authoritative projection');
});

test('a revoked scope lease rejects hydration with foreign data never rendered', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<TaskBackendDetail>();
  const { office } = officeWithDetail(leaseRef, { detail: () => gate.promise });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const pending = handle.hydrate();
  revocable.revoke();
  gate.resolve(detail$({ title: 'foreign' }));
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(handle.view(), undefined);
});

test('live events append serially, duplicates are idempotent and observable', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, store, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const stream = backend.streams[0]!;
  assert.equal(stream.opened[0]!.cursor, 2, 'the stream resumes from the snapshot watermark');
  stream.emit(event$(3));
  await settle();
  assert.equal(handle.view()!.cursor, 3);
  assert.deepEqual(handle.view()!.timeline.map((entry) => entry.seq), [1, 2, 3]);
  stream.emit(event$(3)); // 服务端重复投递
  stream.emit(event$(2));
  await settle();
  assert.equal(handle.view()!.cursor, 3, 'duplicates never advance the cursor');
  assert.deepEqual(handle.view()!.duplicateSeqs, [3, 2], 'idempotent skips stay observable');
  assert.equal(handle.view()!.timeline.filter((entry) => entry.seq === 3).length, 1, 'no double render');
  stream.emit(event$(4, 'run.completed')); // 终态强制 flush（R1-F23：stride 内合并，终态必落盘）
  await settle();
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 4, 'terminal events are durable before drain');
  handle.close();
});

test('a foreign-run event or a malformed sequence interrupts the stream instead of entering the timeline', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  backend.streams[0]!.emit({ ...event$(3), runId: 'run-other' });
  await settle();
  assert.ok(reasons.includes('stream-error'), 'a foreign-run event is surfaced, never merged');
  assert.deepEqual(handle.view()!.timeline.map((entry) => entry.seq), [1, 2], 'the foreign event never enters the timeline');
  handle.close();

  const malformedRef: { lease?: ScopeLease } = {};
  malformedRef.lease = leased().lease;
  const second = officeWithDetail(malformedRef, { detail: async () => detail$() });
  const secondHandle = second.office.open({ taskId: 'task-1', runId: 'run-1' });
  const malformedReasons: Array<string | undefined> = [];
  secondHandle.updates((view) => malformedReasons.push(view.interruption?.reason));
  await secondHandle.hydrate();
  second.backend.streams[0]!.emit({ ...event$(0), type: 'run.started' });
  await settle();
  assert.ok(malformedReasons.includes('stream-error'), 'a zero sequence is surfaced, never appended');
  assert.equal(secondHandle.view()!.cursor, 2);
  secondHandle.close();
});

test('a sequence gap is never silently skipped: interrupt, then authoritative resync', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // 注：brief 原文此处为 event$(5, 'run.completed')——会使 hydrate 判定终态走 drained 不再开流，
  // 与本用例断言 streams.length===2（"the reopened stream continues"）自相矛盾；按文件内其他重同步
  // fixture 惯例改为 text.delta，保持「缺口→中断→权威重同步→重开流」的被测契约不变。
  const details = [detail$(), detail$({ watermark: 5, events: [event$(4, 'text.delta'), event$(5, 'text.delta')] })];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const seen: Array<{ connection: string; reason?: string }> = [];
  handle.updates((view) => seen.push({ connection: view.connection, reason: view.interruption?.reason }));
  await handle.hydrate();
  backend.streams[0]!.emit(event$(5)); // 3 缺失
  await settle();
  assert.ok(seen.some((state) => state.connection === 'interrupted' && state.reason === 'gap'), 'the gap is surfaced, never silent');
  const view = handle.view()!;
  assert.equal(view.cursor, 5, 'resync adopts the authoritative watermark');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 4, 5], 'the gapped event arrives via snapshot exactly once');
  assert.equal(backend.streams.length, 2);
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 5, 'the reopened stream continues from the new watermark');
  handle.close();
});

test('cursor trimming (409 or control frame) resyncs from a fresh snapshot', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // 场景 A：开流即 409（error.code = TASK_STREAM_CURSOR_EXPIRED）
  {
    const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
    const { office } = officeWithDetail(leaseRef, {
      detail: async () => details.shift()!,
      stream: ({ cursor }) => {
        if (cursor === 2) {
          const rejected = createScriptedTaskStream();
          queueMicrotask(() => rejected.fail(Object.assign(new Error('cursor expired'), { code: 'TASK_STREAM_CURSOR_EXPIRED' })));
          return rejected;
        }
        return undefined;
      },
    });
    const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
    const reasons: Array<string | undefined> = [];
    handle.updates((view) => reasons.push(view.interruption?.reason));
    await handle.hydrate();
    await settle();
    assert.ok(reasons.includes('cursor-expired'), 'the 409 surfaces as cursor-expired, never silent');
    assert.equal(handle.view()!.cursor, 3);
    handle.close();
  }
  // 场景 B：流中控制帧 cursor_expired
  {
    const details = [detail$(), detail$({ watermark: 4, events: [event$(4, 'text.delta')] })];
    const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
    const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
    const reasons: Array<string | undefined> = [];
    handle.updates((view) => reasons.push(view.interruption?.reason));
    await handle.hydrate();
    backend.streams[0]!.emit(event$(3));
    await settle();
    backend.streams[0]!.control({ code: 'cursor_expired', message: 'history trimmed' });
    await settle();
    assert.ok(reasons.includes('cursor-expired'));
    assert.equal(handle.view()!.cursor, 4);
    assert.equal(backend.streams[1]!.opened[0]!.cursor, 4);
    handle.close();
  }
});

test('a stride-boundary persist failure surfaces and recovers via authoritative resync (R1-F23 语义)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const baseStore = createInMemoryTaskProjectionStore();
  let failedOnce = false;
  const store: TaskProjectionStore = {
    load: (runId) => baseStore.load(runId),
    save: (projection) => {
      if (!failedOnce && projection.cursor === 52) { failedOnce = true; return Promise.reject(new Error('quota exceeded')); }
      return baseStore.save(projection);
    },
  };
  const resyncGate = deferred<TaskBackendDetail>();
  const details = [detail$(), resyncGate.promise];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! }, store);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  for (let seq = 3; seq <= 52; seq += 1) backend.streams[0]!.emit(event$(seq)); // 50 事件跨度触发 stride persist
  await settle(30);
  assert.ok(reasons.includes('persist-failed'), 'the failure is surfaced, never silent');
  assert.equal(handle.view()!.cursor, 52, 'R1-F23 语义：内存游标推进不再以 persist 成功为前提');
  resyncGate.resolve(detail$({ watermark: 52, events: [] }));
  await settle();
  assert.equal(handle.view()!.cursor, 52, 'the automatic resync re-persists authoritatively');
  assert.equal(baseStore.snapshot().find((row) => row.runId === 'run-1')!.cursor, 52);
  handle.close();
});

test('repeated stream failures stop automatic resync after the bound; explicit resync recovers', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$(), detail$()];
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => details.shift() ?? detail$(),
    stream: () => {
      const failing = createScriptedTaskStream();
      queueMicrotask(() => failing.fail(new Error('HTTP 503')));
      return failing;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle(30);
  assert.equal(backend.streams.length, 3, 'bounded: initial stream + two automatic resyncs, no storm');
  assert.equal(handle.view()!.connection, 'interrupted');
  handle.close();
  // 显式 resync 解除上限并恢复（独立句柄验证恢复路径本身）
  const second = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const secondHandle = second.office.open({ taskId: 'task-1', runId: 'run-1' });
  const recovered = await secondHandle.resync();
  second.backend.streams[0]!.emit(event$(3));
  await settle();
  assert.equal(recovered.connection, 'live');
  assert.equal(secondHandle.view()!.cursor, 3);
  secondHandle.close();
});

test('scenario: a network disconnect recovers through automatic resync and continues live', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => details.shift()!,
    stream: ({ cursor }) => {
      if (cursor === 2) {
        const dropped = createScriptedTaskStream();
        queueMicrotask(() => dropped.fail(new Error('network unreachable')));
        return dropped;
      }
      return undefined;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  await settle();
  assert.ok(reasons.includes('stream-error'), 'the disconnect is surfaced');
  assert.equal(handle.view()!.connection, 'live', 'automatic resync recovers the stream');
  assert.equal(handle.view()!.cursor, 3);
  backend.streams[1]!.emit(event$(4));
  await settle();
  assert.equal(handle.view()!.cursor, 4);
  handle.close();
});

test('scenario: app restart resumes from the persisted projection across server-side trimming', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [
    detail$(), // 第一次水合：watermark 2
    detail$({ watermark: 5, events: [event$(5, 'text.delta')] }), // 重启后：服务端窗口只剩 5
  ];
  const { backend, store, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  // 会话 1：水合并接收 3、4，随后 App 被杀（close 中止流）
  const first = office.open({ taskId: 'task-1', runId: 'run-1' });
  await first.hydrate();
  backend.streams[0]!.emit(event$(3));
  backend.streams[0]!.emit(event$(4));
  await settle();
  first.close('app-killed');
  await settle(); // close 的 best-effort flush 是异步落盘（R1-F23）
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 4, 'the killed session leaves a durable projection');
  // 会话 2（同一 office 默认共享 store，模拟重启后读回持久化投影）
  const second = office.open({ taskId: 'task-1', runId: 'run-1' });
  const view = await second.hydrate();
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3, 4, 5], 'trimmed server history is backfilled from the persisted projection');
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 5, 'the reopened stream resumes from the merged watermark');
  assert.equal(backend.detailCalls.length, 2);
  backend.streams[1]!.emit(event$(6, 'text.delta'));
  await settle();
  assert.equal(second.view()!.cursor, 6);
  second.close();
});

test('scenario: terminal drain ends the stream and never reconnects', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const stream = backend.streams[0]!;
  stream.emit(event$(3, 'run.completed'));
  await settle();
  stream.end(); // 服务端终态收流（Go StreamWorkbenchEvents 终态 drain 语义）
  await settle(20);
  const view = handle.view()!;
  assert.equal(view.runStatus, 'succeeded', 'terminal events advance the run status by the server rule');
  assert.equal(view.lifecycle, 'completed');
  assert.equal(view.connection, 'drained');
  assert.equal(backend.detailCalls.length, 1, 'drain is decided locally from durable events, no extra snapshot');
  assert.equal(backend.streams.length, 1, 'a drained task never reconnects');
  handle.close();
});

test('scenario: a non-terminal stream end resyncs once and continues live', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  backend.streams[0]!.end(); // 服务器中途收流，运行仍非终态
  await settle();
  assert.ok(reasons.includes('stream-ended-nonterminal'), 'a non-terminal end is surfaced');
  assert.equal(handle.view()!.connection, 'live', 'resync reopens the stream');
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 3);
  handle.close();
});

test('a revoked lease after hydration stops notifications and rejects resync', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const views: number[] = [];
  handle.updates(() => views.push(1));
  revocable.revoke();
  await settle();
  assert.deepEqual(views, [], 'no notifications after the scope died');
  await assert.rejects(handle.resync(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  handle.close();
});

test('a slower stale hydrate does not roll back the committed cursor or the event set', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  let call = 0;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => {
      call += 1;
      if (call === 1) { await settle(8); return detail$(); } // 旧响应：慢，watermark 2
      return detail$({ watermark: 4, events: [event$(1, 'run.started'), event$(2, 'tool.started'), event$(3, 'text.delta'), event$(4, 'run.completed')] }); // 新响应：watermark 4
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const stale = handle.hydrate();                     // 旧请求先发（挂起多个宏任务）
  await settle(1);
  await handle.resync();                              // 新请求后发先回：watermark 4 已提交
  await stale.catch(() => undefined);                 // 旧响应迟到到达（修复后被 epoch 守卫拒绝）
  const view = handle.view()!;
  assert.equal(view.cursor, 4, '迟到的旧 hydrate 不得回退 committedCursor');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3, 4], '不得以旧响应的事件集覆写新事件集');
  handle.close();
});

test('events from a superseded stream are dropped after resync', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // 模拟「abort 后仍有在途投递」的传输层：直接持有每条流的回调（reader 已缓冲的 chunk 在 cancel 后才送达）。
  // createScriptedTaskStream 在 abort 后 emit 是 no-op，无法覆盖该竞态。
  const sinks: Array<{ onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }> = [];
  const detailBackend: TaskDetailBackendPort = {
    detail: async () => detail$({ watermark: 2 }),
    stream: ({ signal, onEvent, onControl }) => {
      sinks.push({ onEvent, onControl });
      return new Promise<void>((resolve) => { signal.addEventListener('abort', () => resolve()); });
    },
  };
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
    store: createInMemoryTaskProjectionStore(),
    detail: detailBackend,
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();                              // 开流 A = sinks[0]
  await handle.resync();                               // 开流 B = sinks[1]，A 被取代
  sinks[0]!.onEvent(event$(9, 'tool.started'));        // 旧流迟到事件（seq 错位）：enqueue 前必须丢弃
  sinks[0]!.onControl({ code: 'cursor_expired', message: 'late frame' }); // 旧流迟到控制帧：同样丢弃
  await settle();
  const view = handle.view()!;
  assert.equal(view.connection, 'live', '被取代流的迟到事件/控制帧不得打断新流');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2]);
  assert.equal(reasons.includes('gap'), false, '迟到事件不得触发 gap interrupt');
  assert.equal(reasons.includes('cursor-expired'), false, '迟到控制帧不得触发 cursor-expired interrupt');
  handle.close();
});

test('an unavailable stream channel interrupts without futile auto-resyncs', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  let detailCalls = 0;
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => { detailCalls += 1; return detail$(); },
    stream: () => {
      const failing = createScriptedTaskStream();
      queueMicrotask(() => failing.fail(new Error('RUNTIME_STREAM_UNAVAILABLE')));
      return failing;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle(30);                                   // 给自动 resync 的窗口（对照既有 bounded-resync 用例的 settle(30)）
  assert.equal(handle.view()!.connection, 'interrupted');
  assert.equal(handle.view()!.interruption?.reason, 'stream-unavailable');
  assert.equal(backend.streams.length, 1, '流通道不可用不是瞬时故障：不得触发 AUTO_RESYNC_LIMIT 次徒劳 resync');
  assert.equal(detailCalls, 1);
  handle.close();
});

test('stream-unavailable is detected by error code, not message text (R1-F22)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const detailBackend: TaskDetailBackendPort = {
    detail: async () => detail$(),
    stream: async () => { throw Object.assign(new Error('wrapped: RUNTIME_STREAM_UNAVAILABLE'), { code: 'RUNTIME_STREAM_UNAVAILABLE' }); },
  };
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
    detail: detailBackend,
    store: createInMemoryTaskProjectionStore(),
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle();
  assert.equal(handle.view()!.connection, 'interrupted');
  assert.equal(handle.view()!.interruption?.reason, 'stream-unavailable');
  handle.close();
});

test('a throwing listener does not starve later subscribers (R1-F24)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const seen: number[] = [];
  handle.updates(() => { throw new Error('listener boom'); });
  handle.updates(() => { seen.push(1); });
  await handle.resync(); // 触发 notify（resync 期间 syncing + live 两次广播）
  // 隔离语义：抛错的订阅者不得截断广播——后续订阅者每次广播都收到（修复前 resync 直接 reject 且 seen=0）。
  assert.ok(seen.length >= 1, `later subscriber must be notified on every broadcast, got ${seen.length}`);
  handle.close();
});

test('events arriving after lease revocation are dropped, not merged (R1-F44)', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = lease;
  let emit: ((event: TaskBackendEvent) => void) | undefined;
  const detailBackend: TaskDetailBackendPort = {
    detail: async () => detail$(),
    stream: ({ onEvent, signal }) => new Promise<void>((resolve) => {
      emit = onEvent;
      signal.addEventListener('abort', () => resolve());
    }),
  };
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
    detail: detailBackend,
    store: createInMemoryTaskProjectionStore(),
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const before = handle.view()!;
  revocable.revoke();
  emit!(event$(3));
  await settle();
  assert.equal(handle.view()?.cursor, before.cursor); // 撤销后事件未合并
  handle.close();
});

test('a stale processEvent does not rewind the cursor after resync (R1-F43)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const gate = deferred<void>(); // 卡住 cursor=3 的 persist（hydrate 首次 persist cursor=2 不卡）
  const store = createInMemoryTaskProjectionStore();
  const gatedStore: TaskProjectionStore = {
    load: (runId) => store.load(runId),
    save: async (projection) => { if (projection.cursor === 3) await gate.promise; await store.save(projection); },
  };
  let detailCalls = 0;
  const secondDetail = deferred<TaskBackendDetail>();
  const laterDetail = deferred<TaskBackendDetail>(); // 第三次起卡住：旧实现中陈旧回写+自动 resync 会先暴露回退的游标
  const detailBackend = createScenarioTaskDetailBackend({
    detail: async () => {
      detailCalls += 1;
      return detailCalls <= 1 ? detail$() : detailCalls === 2 ? secondDetail.promise : laterDetail.promise;
    },
    stream: () => {
      const scripted = createScriptedTaskStream();
      queueMicrotask(() => { scripted.emit(event$(3)); scripted.end(); });
      return scripted;
    },
  });
  const office = createTaskOffice({ backend: createScenarioTaskBackend({}), lease: () => leaseRef.lease, detail: detailBackend, store: gatedStore });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate(); // events 推进到 3 的 processEvent 挂起在 gatedStore.save 上
  const resynced = handle.resync(); // 挂起期间 resync：watermark=10 的新 detail
  secondDetail.resolve(detail$({ watermark: 10, events: [event$(1, 'run.started'), event$(2, 'tool.started'), event$(3), event$(10, 'text.delta')] }));
  await resynced;
  gate.resolve(); // 放行陈旧 persist → 旧 processEvent 恢复，不得把游标回退到 3
  await settle();
  const view = handle.view()!;
  assert.equal(view.cursor, 10);
  assert.notEqual(view.interruption?.reason, 'gap'); // 无陈旧赋值引发的 gap 连锁抖动
  handle.close();
});

test('terminal run settles the execution view without waiting for resync (R1-F27)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: () => {
      const scripted = createScriptedTaskStream();
      queueMicrotask(() => { scripted.emit(event$(3, 'run.completed')); scripted.end(); });
      return scripted;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle();
  const view = handle.view()!;
  assert.equal(view.runStatus, 'succeeded');
  assert.equal(view.settlementStatus, 'settled'); // 当前保持快照 'pending' → 失败
  assert.equal(view.executionStatus, 'succeeded');
  handle.close();
});

test('heartbeat control frames are benign (R1-F45)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: () => {
      const scripted = createScriptedTaskStream();
      queueMicrotask(() => { scripted.control({ code: 'heartbeat', message: '' }); });
      return scripted;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle();
  assert.equal(handle.view()?.connection, 'live'); // 心跳不打断流
  handle.close();
});

test('persist coalesces bursts: one save per 50-event stride plus terminal flush (R1-F23)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  let saves = 0;
  const counting: TaskProjectionStore = {
    load: (runId) => store.load(runId),
    save: async (projection) => { saves += 1; await store.save(projection); },
  };
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: () => {
      const scripted = createScriptedTaskStream();
      queueMicrotask(() => {
        for (let seq = 3; seq <= 62; seq += 1) scripted.emit(event$(seq, seq === 62 ? 'run.completed' : 'text.delta'));
        scripted.end();
      });
      return scripted;
    },
  }, counting);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle(60);
  assert.ok(saves <= 3, `expected coalesced saves, got ${saves}`); // 60 事件 ≤ hydrate 1 + stride 1 + 终态 flush 1
  assert.equal(handle.view()?.cursor, 62);
  assert.equal(handle.view()?.connection, 'drained');
  const persisted = await store.load('run-1');
  assert.equal(persisted?.cursor, 62); // 终态必已 flush
  handle.close();
});

test('persist failure stays observable when the store keeps failing (R1-F23 语义保留)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const failing: TaskProjectionStore = { load: async () => undefined, save: async () => { throw new Error('disk full'); } };
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$(),
    stream: () => createScriptedTaskStream(),
  }, failing);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate(); // hydrate 内 persist 失败 → persist-failed interruption
  const view = handle.view()!;
  assert.equal(view.interruption?.reason, 'persist-failed');
  assert.equal(view.connection, 'interrupted'); // 不静默
  handle.close();
});

// —— T07（#37）：TaskHandle.act 干预合同（三态停止 / unknown 门 / parked queue-next） ——
// helper 与任务简报原文的差异点（按本文件既有惯例对齐）：
// 1) 工厂不自动 hydrate：用例显式 await（简报括注要求，保证断言时序确定）；
// 2) stream stub 用永不 resolve 的流（活动 Run 的真实 SSE 语义）：立即 resolve 的空流会让
//    每个非终态 hydrate 触发 stream-ended → 有界自动重连级联，与用例的 resync 时序竞争；
// 3) lease 复用本文件既有 leased() 的 RuntimeScopeLease（leaseActive 只认该实例），
//    用例经返回的 revocable 撤销（简报的 revokeLease() helper 本文件不存在）；
// 4) 简报用例 2 未复位 script.error——重试要得到 accepted 必须切回成功脚本（对齐冲突用例
//    的 script 切换惯例）；空数组字段断言补 `?? 0`（buildView 对空集省略字段的既有惯例）。
interface RecordedCommand { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }
interface CommandScript { result?: { runId: string; action: RecordedCommand['action']; nextRunId?: string }; error?: unknown }

function scriptedCommandPort(initial: CommandScript = {}) {
  const calls: RecordedCommand[] = [];
  const script: CommandScript = { ...initial };
  return {
    calls,
    script,
    port: {
      async command(input: RecordedCommand): Promise<{ runId: string; action: RecordedCommand['action']; nextRunId?: string }> {
        calls.push(input);
        if (script.error !== undefined) throw script.error;
        return script.result ?? { runId: input.runId, action: input.action };
      },
    },
  };
}

const codedError = (code: string): Error => Object.assign(new Error(code), { code });

function interventionDetail(execution: { runStatus: string; revision: number }): TaskBackendDetail {
  return {
    taskId: 's1', runId: 'run-1', title: 't', attention: 'none',
    execution: { runStatus: execution.runStatus, executionStatus: execution.runStatus, settlementStatus: 'pending', revision: execution.revision, seq: 0 },
    watermark: 0, incomplete: false, events: [],
  };
}

function newHandleForIntervention(execution: { runStatus: string; revision: number }, commands?: TaskCommandPort) {
  const { revocable, lease } = leased();
  const detailBackend = {
    detailResult: interventionDetail(execution),
    async detail(): Promise<TaskBackendDetail> { return this.detailResult; },
    stream(): Promise<void> { return new Promise<void>(() => undefined); }, // 活动 Run 的流保持打开，直至 resync/close abort
  };
  const handle = createTaskDetail(
    { taskId: 's1', runId: 'run-1' },
    { backend: detailBackend, store: createInMemoryTaskProjectionStore(), lease: () => lease, ...(commands === undefined ? {} : { commands }) },
  );
  return { handle, detailBackend, revocable };
}

test('act(stop) presents requested, then confirmed when the projection observes canceled', async () => {
  const commands = scriptedCommandPort();
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 4 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.boundRunId, 'run-1');
  assert.equal(receipt.revision, 4, 'the command must carry the observed revision');
  assert.equal(commands.calls[0]?.action, 'cancel');
  assert.equal(commands.calls[0]?.expectedRevision, 4);
  assert.equal(handle.view()?.stop?.phase, 'requested');
  // 快照观察到 canceled → confirmed（AC1）。
  detailBackend.detailResult = interventionDetail({ runStatus: 'canceled', revision: 5 });
  await handle.resync();
  assert.equal(handle.view()?.stop?.phase, 'confirmed');
  handle.close('done');
});

test('act(stop) with an unknown delivery outcome gates further writes until a resync reconciles', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_UNKNOWN') });
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'unknown');
  assert.equal(handle.view()?.stop?.phase, 'unknown');
  // AC2：unknown 未核对前，同句柄后续写意图一律拒绝。
  await assert.rejects(() => handle.act({ kind: 'steer', text: 'x' }), /TASK_OFFICE_COMMAND_UNKNOWN/);
  await assert.rejects(() => handle.act({ kind: 'stop' }), /TASK_OFFICE_COMMAND_UNKNOWN/);
  // 核对：仍 running ⇒ 取消未落地 ⇒ 门解除、停止卡清除。
  detailBackend.detailResult = interventionDetail({ runStatus: 'running', revision: 2 });
  await handle.resync();
  assert.equal(handle.view()?.stop, undefined);
  commands.script.error = undefined; // 简报原文缺此行：重试要走通必须切回成功脚本（见上方差异点 4）
  const retry = await handle.act({ kind: 'stop' });
  assert.equal(retry.outcome, 'accepted');
  handle.close('done');
});

test('act(stop) unknown reconciles to confirmed when the run was actually canceled', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_UNKNOWN') });
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  await handle.act({ kind: 'stop' });
  detailBackend.detailResult = interventionDetail({ runStatus: 'canceled', revision: 3 });
  await handle.resync();
  assert.equal(handle.view()?.stop?.phase, 'confirmed');
  handle.close('done');
});

test('a 409 conflict is a receipt, not the unknown gate', async () => {
  const commands = scriptedCommandPort({ error: codedError('TASK_COMMAND_CONFLICT') });
  const { handle } = newHandleForIntervention({ runStatus: 'running', revision: 1 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'conflict');
  assert.equal(handle.view()?.stop, undefined, 'a conflict leaves no stop card');
  // 冲突不进门：后续意图仍可发出。
  commands.script.error = undefined;
  const next = await handle.act({ kind: 'steer', text: 'go' });
  assert.equal(next.outcome, 'accepted');
  handle.close('done');
});

test('act(queue-next) on an active run parks locally and flushes on terminal observation', async () => {
  const commands = scriptedCommandPort();
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 7 }, commands.port);
  await handle.hydrate();
  const parked = await handle.act({ kind: 'queue-next', text: 'next instruction' });
  assert.equal(parked.outcome, 'parked', 'never claim a server queue that does not exist');
  assert.equal(commands.calls.length, 0);
  assert.deepEqual(handle.view()?.queuedNext?.map((q) => q.text), ['next instruction']);
  // 观察到终态 → flush 发出 queue_next，服务端准入下一 Run。
  detailBackend.detailResult = interventionDetail({ runStatus: 'succeeded', revision: 7 });
  await handle.resync();
  await handle.flushQueuedIntents();
  assert.equal(commands.calls.length, 1, 'hydrate 内的自动 flush 与显式 flush 合流，绝不重复派发');
  assert.equal(commands.calls[0]?.action, 'queue_next');
  assert.equal(commands.calls[0]?.intentId !== undefined, true, 'parked intents fire with a stable idempotency id');
  const receipt = handle.view()?.interventions?.at(-1);
  assert.equal(receipt?.outcome, 'accepted');
  assert.equal(handle.view()?.queuedNext?.length ?? 0, 0, '空队列按字段省略呈现（buildView 空集省略惯例）');
  handle.close('done');
});

test('act(queue-next) on a terminal run dispatches immediately', async () => {
  const commands = scriptedCommandPort({ result: { runId: 'run-1', action: 'queue_next', nextRunId: 'run-2' } });
  const { handle } = newHandleForIntervention({ runStatus: 'canceled', revision: 6 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'queue-next', text: 'restart now' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.boundRunId, 'run-1');
  assert.equal(receipt.nextRunId, 'run-2');
  handle.close('done');
});

test('act before hydrate and a missing commands port fail closed', async () => {
  const noPort = newHandleForIntervention({ runStatus: 'running', revision: 1 });
  await assert.rejects(() => noPort.handle.act({ kind: 'stop' }), /TASK_OFFICE_COMMAND_UNAVAILABLE/);
  const withPort = newHandleForIntervention({ runStatus: 'running', revision: 1 }, scriptedCommandPort().port);
  await assert.rejects(() => withPort.handle.act({ kind: 'stop' }), /TASK_OFFICE_NO_SNAPSHOT/);
  noPort.handle.close('done'); withPort.handle.close('done');
});

test('a command landing while the lease died is rejected, never silently recorded', async () => {
  // Scope Lease 失效后丢弃迟到结果（module-seams §5.3）：lease 在命令在途时撤销 → SCOPE_CHANGED。
  let release: (() => void) | undefined;
  type CancelAck = { runId: string; action: 'cancel' };
  const slowPort = { command: (): Promise<CancelAck> => new Promise((resolve) => { release = () => resolve({ runId: 'run-1', action: 'cancel' }); }) };
  const { handle, revocable } = newHandleForIntervention({ runStatus: 'running', revision: 9 }, slowPort);
  await handle.hydrate();
  const pending = handle.act({ kind: 'stop' });
  revocable.revoke();
  release!();
  await assert.rejects(() => pending, /TASK_OFFICE_SCOPE_CHANGED/);
  assert.equal(handle.view()?.interventions?.length ?? 0, 0, 'a late result must not be recorded as an intervention');
  handle.close('done');
});

test('a flush racing a dead scope lease is rejected, never silently recorded (§5.3)', async () => {
  // 与 act() 同一竞态同一处理：flush 的 queue_next 在 lease 死亡后，迟到 ack 绝不记 accepted 回执。
  let release: (() => void) | undefined;
  const calls: RecordedCommand[] = [];
  const gatedPort: TaskCommandPort = {
    command: (input) => {
      calls.push(input);
      return new Promise((resolve) => { release = () => resolve({ runId: input.runId, action: 'queue_next' }); });
    },
  };
  const { handle, detailBackend, revocable } = newHandleForIntervention({ runStatus: 'running', revision: 5 }, gatedPort);
  await handle.hydrate();
  const parked = await handle.act({ kind: 'queue-next', text: 'next instruction' });
  assert.equal(parked.outcome, 'parked');
  detailBackend.detailResult = interventionDetail({ runStatus: 'succeeded', revision: 5 });
  await handle.resync(); // 终态观察：hydrate 自动放行 flush，命令在 gatedPort 处挂起（在途）
  assert.equal(calls.length, 1, '自动 flush 已派发且尚未返回');
  const flushing = handle.flushQueuedIntents(); // 合流到同一在途 flush
  revocable.revoke();
  release!(); // 迟到 ack
  await assert.rejects(() => flushing, /TASK_OFFICE_SCOPE_CHANGED/);
  assert.equal(handle.view()?.interventions?.length, 1, '只保留 parked 回执，迟到的 ack 不得追加 accepted 回执');
  handle.close('done');
});

test('flushQueuedIntents after the scope died fails closed with SCOPE_CHANGED', async () => {
  const commands = scriptedCommandPort();
  const { handle, revocable } = newHandleForIntervention({ runStatus: 'canceled', revision: 3 }, commands.port);
  await handle.hydrate(); // 终态：drained（自动 flush 在 revoke 前已无队列空转）
  revocable.revoke();
  await assert.rejects(() => handle.flushQueuedIntents(), /TASK_OFFICE_SCOPE_CHANGED/);
  handle.close('done');
});
