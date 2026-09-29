import test from 'node:test';
import assert from 'node:assert/strict';
import type { ScopeLease, TaskListPage } from '@weknora/mobile-core';
import { createTaskOfficeListController, taskOfficeLifecycleKey } from './task-office-state.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

function page(taskId: string): TaskListPage {
  return { items: [{ taskId, runId: `run-${taskId}`, title: taskId, runStatus: 'running', attention: 'none', updatedAt: 'now' }], duplicateRunIds: [] };
}

function revocableLease() {
  const listeners = new Set<() => void>();
  const lease = { onRevoke(listener: () => void) { listeners.add(listener); return () => listeners.delete(listener); } } as unknown as ScopeLease;
  return { lease, revoke() { for (const listener of [...listeners]) listener(); } };
}

test('Task Office route clears visible rows synchronously on lease revocation and remounts by user and lease', async () => {
  const held = deferred<TaskListPage>();
  const first = revocableLease();
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let readCount = 0;
  const controller = createTaskOfficeListController({ openingLease: first.lease, read: () => readCount++ === 0 ? Promise.resolve(page('existing-private-task')) : held.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  await controller.load();
  assert.deepEqual(states.at(-1)?.tasks, ['existing-private-task']);
  const reading = controller.load();
  first.revoke();
  assert.deepEqual(states.at(-1), { tasks: [], loading: false, error: undefined }, 'revocation immediately hides previous scoped rows');
  held.resolve(page('old-user-task'));
  await reading;
  assert.deepEqual(states.at(-1), { tasks: [], loading: false, error: undefined }, 'a late response cannot restore revoked rows');
  const second = revocableLease();
  assert.notEqual(taskOfficeLifecycleKey('https://host', 'tenant', 'user-1', first.lease), taskOfficeLifecycleKey('https://host', 'tenant', 'user-2', second.lease));
  assert.notEqual(taskOfficeLifecycleKey('https://host', 'tenant', 'user-1', first.lease), taskOfficeLifecycleKey('https://host', 'tenant', 'user-1', second.lease));
});

test('Task Office route only publishes the newest read result or failure', async () => {
  const lease = revocableLease();
  const reads = [deferred<TaskListPage>(), deferred<TaskListPage>()];
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let index = 0;
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => reads[index++]!.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const older = controller.load();
  const newer = controller.load();
  reads[1]!.resolve(page('newer-task'));
  await newer;
  reads[0]!.reject(new Error('superseded failure'));
  await older;
  assert.deepEqual(states.at(-1), { tasks: ['newer-task'], loading: false, error: undefined });
});

test('Task Office route ignores an older successful read after a newer refresh', async () => {
  const lease = revocableLease();
  const reads = [deferred<TaskListPage>(), deferred<TaskListPage>()];
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let index = 0;
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => reads[index++]!.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const older = controller.load();
  const newer = controller.load();
  reads[1]!.resolve(page('newer-task'));
  await newer;
  reads[0]!.resolve(page('older-task'));
  await older;
  assert.deepEqual(states.at(-1), { tasks: ['newer-task'], loading: false, error: undefined });
});

test('Task Office route invalidates pending reads when disposed', async () => {
  const held = deferred<TaskListPage>();
  const lease = revocableLease();
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => held.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const reading = controller.load();
  controller.dispose();
  held.resolve(page('late-task'));
  await reading;
  assert.deepEqual(states.at(-1), { tasks: [], loading: true, error: undefined }, 'disposed controller publishes no late state');
});
