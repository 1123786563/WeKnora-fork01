import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend, emptyOverview } from './in-memory-task-backend.ts';
import { createTaskOffice, TaskOfficeError, type TaskBackendOverview, type TaskBackendPage } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWith(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioTaskBackend>[0] = {}) {
  const backend = createScenarioTaskBackend(handlers);
  return { backend, office: createTaskOffice({ backend, lease: () => leaseRef.lease }) };
}

function backendRun(runId: string, taskId = `task-${runId}`) {
  return { runId, taskId, title: `title-${runId}`, runStatus: 'running', attention: 'none' as const, updatedAt: '2026-09-23T00:00:00Z' };
}

test('home aggregates needs-me, running and recently-completed from one backend call', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const overview: TaskBackendOverview = {
    needsMe: [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }],
    running: [backendRun('r1')],
    recentlyCompleted: [{ ...backendRun('r2'), runStatus: 'succeeded' }],
    unreadNotifications: 3,
    asOf: '2026-09-23T00:00:01Z',
  };
  const { backend, office } = officeWith(leaseRef, { overview: async () => overview });
  const view = await office.home();
  assert.deepEqual(view.running, [backendRun('r1')]);
  assert.deepEqual(view.recentlyCompleted, [{ ...backendRun('r2'), runStatus: 'succeeded' }]);
  assert.deepEqual(view.needsMe, [{ interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' }]);
  assert.equal(view.unreadNotifications, 3);
  assert.equal(backend.calls.length, 1, 'one aggregate call, no per-session follow-ups');
});

test('a late home response after the scope lease was revoked never resolves with foreign-scope data', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<TaskBackendOverview>();
  const { office } = officeWith(leaseRef, { overview: () => gate.promise });
  const pending = office.home();
  revocable.revoke();
  gate.resolve({ ...emptyOverview(), running: [backendRun('foreign')] });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('a newer tasks query supersedes an in-flight older one', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const first = deferred<TaskBackendPage>();
  const second = deferred<TaskBackendPage>();
  let call = 0;
  const { office } = officeWith(leaseRef, { list: () => (call += 1) === 1 ? first.promise : second.promise });
  const older = office.tasks({ search: 'old' });
  const newer = office.tasks({ search: 'new' });
  first.resolve({ items: [backendRun('r-old')] });
  second.resolve({ items: [backendRun('r-new')] });
  await assert.rejects(older, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  const page = await newer;
  assert.deepEqual(page.items.map((item) => item.runId), ['r-new']);
});

test('moreTasks continues the owned cursor and duplicate run keys are reported, never rendered twice', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pages: Array<TaskBackendPage | Error> = [
    { items: [backendRun('r-a'), backendRun('r-b')], nextCursor: 'cursor-1' },
    { items: [backendRun('r-b'), backendRun('r-c')] }, // 服务端重复键：r-b 重现
  ];
  let call = 0;
  const { backend, office } = officeWith(leaseRef, { list: () => Promise.resolve(pages[call++]!) });
  const first = await office.tasks({});
  assert.deepEqual(first.items.map((item) => item.runId), ['r-a', 'r-b']);
  assert.equal(first.nextCursor, 'cursor-1');
  const second = await office.moreTasks();
  assert.deepEqual(second.items.map((item) => item.runId), ['r-c'], 'the repeated key is not rendered again');
  assert.deepEqual(second.duplicateRunIds, ['r-b'], 'the duplicate is observable');
  // 已耗尽：不再发后端请求，返回空页且保留重复键观测。
  const exhausted = await office.moreTasks();
  assert.deepEqual(exhausted, { items: [], duplicateRunIds: ['r-b'] });
  assert.equal(backend.calls.filter((entry) => entry.kind === 'list').length, 2);
});

test('empty and failing backends surface as an empty view and a typed backend error', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWith(leaseRef);
  assert.deepEqual(await office.home(), emptyOverview());
  assert.deepEqual((await office.tasks({})).items, []);

  const failing = officeWith(leaseRef, { list: () => Promise.reject(new Error('HTTP 500')) });
  await assert.rejects(failing.office.tasks({}), (error: unknown) =>
    error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND' && (error.cause as Error).message === 'HTTP 500');
});

test('archive and restore invalidate the accumulated query and fail closed without a lease', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const archived: string[] = [];
  const restored: string[] = [];
  const { office } = officeWith(leaseRef, {
    list: async () => ({ items: [backendRun('r-a')], nextCursor: 'cursor-1' }),
    archive: async (taskId) => { archived.push(taskId); },
    restore: async (taskId) => { restored.push(taskId); },
  });
  await office.tasks({});
  await office.archive('task-r-a ');
  assert.deepEqual(archived, ['task-r-a'], 'task ids are trimmed before hitting the backend');
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
  await office.tasks({});
  await office.restore('task-r-a');
  assert.deepEqual(restored, ['task-r-a']);
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');

  revocable.revoke();
  await assert.rejects(office.archive('task-r-a'), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  await assert.rejects(office.home(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('queries are normalized before they reach the backend', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWith(leaseRef, { list: async () => ({ items: [] }) });
  await office.tasks({ search: '   quarterly   review ', limit: 5000, status: 'running' });
  const listCall = backend.calls.find((entry) => entry.kind === 'list');
  assert.ok(listCall && listCall.kind === 'list');
  assert.deepEqual({ search: listCall.input.search, limit: listCall.input.limit, status: listCall.input.status, archived: listCall.input.archived },
    { search: 'quarterly review', limit: 100, status: 'running', archived: undefined });
  await office.tasks({ search: '   ' });
  const blank = backend.calls.filter((entry) => entry.kind === 'list')[1]!;
  assert.ok(blank.kind === 'list');
  assert.equal(blank.input.search, undefined, 'blank search is normalized away');
});

test('a successful archive rejects in-flight tasks() with SUPERSEDED and clears accumulation (R1-F19)', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pending = deferred<TaskBackendPage>();
  const { office } = officeWith(leaseRef, {
    list: () => pending.promise,
    archive: async () => undefined,
  });
  const inflight = office.tasks({});
  await office.archive('task-1'); // 在途 list 尚未 resolve：写成功即作废在途读
  pending.resolve({ items: [backendRun('r-archived')] }); // 迟到的归档前快照不得被采纳
  await assert.rejects(inflight, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  await assert.rejects(office.moreTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
});
