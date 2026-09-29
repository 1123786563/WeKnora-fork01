import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createScenarioLegacyTaskBackend, LEGACY_TASK_NEW_RUN_REASON } from './legacy-tasks.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWith(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioLegacyTaskBackend>[0] = {}) {
  const legacy = createScenarioLegacyTaskBackend(handlers);
  return { legacy, office: createTaskOffice({ backend: createScenarioTaskBackend(), legacy, lease: () => leaseRef.lease }) };
}

function legacyRow(taskId: string, updatedAt = '2026-09-20T08:00:00Z') {
  return { taskId, title: `title-${taskId}`, attention: 'none' as const, updatedAt };
}

test('legacyTasks returns gate-annotated cards and continues the owned cursor', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pages = [
    { items: [legacyRow('lg-a'), legacyRow('lg-b')], nextCursor: 'cursor-1' },
    { items: [legacyRow('lg-b'), legacyRow('lg-c')] }, // 服务端重复键：lg-b 重现
  ];
  let call = 0;
  const { legacy, office } = officeWith(leaseRef, { list: () => Promise.resolve(pages[call++]!) });
  const first = await office.legacyTasks({});
  assert.deepEqual(first.items.map((item) => item.taskId), ['lg-a', 'lg-b']);
  assert.equal(first.nextCursor, 'cursor-1');
  assert.equal(first.items[0]!.kind, 'legacy');
  assert.equal(first.items[0]!.attention, 'none');
  // 显式门禁随卡片下发（模块内派生）：AC2。
  assert.equal(first.items[0]!.gates['run-command'].state, 'unavailable');
  assert.equal(first.items[0]!.gates['run-command'].reason, LEGACY_TASK_NEW_RUN_REASON);
  assert.equal(first.items[0]!.gates['follow-up'].state, 'supported');
  const second = await office.moreLegacyTasks();
  assert.deepEqual(second.items.map((item) => item.taskId), ['lg-c'], 'repeated key not rendered again');
  assert.deepEqual(second.duplicateTaskIds, ['lg-b']);
  assert.deepEqual(legacy.calls.map((c) => c.kind), ['list', 'list']);
});

test('legacy ports missing fail closed with a dedicated code', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const office = createTaskOffice({ backend: createScenarioLegacyTaskBackend(), lease: () => leaseRef.lease });
  await assert.rejects(office.legacyTasks({}), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_LEGACY_UNAVAILABLE');
});

test('legacy reads reject after the scope lease was revoked', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<{ items: [] }>();
  const { office } = officeWith(leaseRef, { list: () => gate.promise });
  const pending = office.legacyTasks({});
  revocable.revoke();
  gate.resolve({ items: [] }); // 迟到结果到达：settle 必须按 SCOPE_CHANGED 拒绝
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('followUp validates input, wraps backend failures and invalidates the active legacy query', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { legacy, office } = officeWith(leaseRef, {
    list: async () => ({ items: [legacyRow('lg-1')], nextCursor: 'cursor-1' }),
    followUp: async (input) => {
      if (input.question === 'boom') throw new Error('HTTP_409: another turn is already running');
    },
  });
  await assert.rejects(office.followUp({ taskId: '  ', question: 'x' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  await assert.rejects(office.followUp({ taskId: 'lg-1', question: '' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  await assert.rejects(office.followUp({ taskId: 'lg-1', question: 'x'.repeat(8001) }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');

  // 后端失败（409 another turn running）按 TASK_OFFICE_BACKEND 包装上抛，不静默重试、不换语义。
  await assert.rejects(
    office.followUp({ taskId: 'lg-1', question: 'boom' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND' && String((error as TaskOfficeError & { cause?: unknown }).cause).includes('409'),
  );

  // 建立活动查询（moreLegacyTasks 可续页），成功追问后查询失效。
  const first = await office.legacyTasks({});
  assert.equal(first.nextCursor, 'cursor-1');
  await office.followUp({ taskId: 'lg-1', question: '继续这个话题' });
  assert.deepEqual(legacy.calls.filter((c) => c.kind === 'followUp').length, 2);
  await assert.rejects(office.moreLegacyTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
});

test('a legacy task cannot be opened as a run handle: open requires a runId', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWith(leaseRef);
  assert.throws(() => office.open({ taskId: 'lg-1', runId: '' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
});

test('legacyHistory passes the trimmed task id through the lease gate', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { legacy, office } = officeWith(leaseRef, {
    history: async () => [{ messageId: 'm1', role: 'user', content: '第一问' }],
  });
  const messages = await office.legacyHistory(' lg-1 ');
  assert.equal(messages[0]!.messageId, 'm1');
  assert.equal((legacy.calls[0] as { kind: string }).kind, 'history');
});
