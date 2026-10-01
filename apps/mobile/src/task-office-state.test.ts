import test from 'node:test';
import assert from 'node:assert/strict';
import type { ScopeLease, TaskListPage } from '@weknora/mobile-core';
import { createTaskOfficeListController, taskOfficeLifecycleKey } from './app/task-office-state.ts';

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
  let active = true;
  const lease = { onRevoke(listener: () => void) { if (!active) { listener(); return () => undefined; } listeners.add(listener); return () => listeners.delete(listener); } } as unknown as ScopeLease;
  return { lease, listenerCount: () => listeners.size, revoke() { if (!active) return; active = false; for (const listener of [...listeners]) listener(); listeners.clear(); } };
}

test('Task Office route clears visible rows synchronously on lease revocation and remounts by user and lease', async () => {
  const held = deferred<TaskListPage>();
  const first = revocableLease();
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let readCount = 0;
  const controller = createTaskOfficeListController({ openingLease: first.lease, read: () => readCount++ === 0 ? Promise.resolve(page('existing-private-task')) : held.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const unmount = controller.mount();
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
  unmount();
});

test('Task Office route only publishes the newest read result or failure', async () => {
  const lease = revocableLease();
  const reads = [deferred<TaskListPage>(), deferred<TaskListPage>()];
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let index = 0;
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => reads[index++]!.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const unmount = controller.mount();
  const older = controller.load();
  const newer = controller.load();
  reads[1]!.resolve(page('newer-task'));
  await newer;
  reads[0]!.reject(new Error('superseded failure'));
  await older;
  assert.deepEqual(states.at(-1), { tasks: ['newer-task'], loading: false, error: undefined });
  unmount();
});

test('Task Office route ignores an older successful read after a newer refresh', async () => {
  const lease = revocableLease();
  const reads = [deferred<TaskListPage>(), deferred<TaskListPage>()];
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  let index = 0;
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => reads[index++]!.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const unmount = controller.mount();
  const older = controller.load();
  const newer = controller.load();
  reads[1]!.resolve(page('newer-task'));
  await newer;
  reads[0]!.resolve(page('older-task'));
  await older;
  assert.deepEqual(states.at(-1), { tasks: ['newer-task'], loading: false, error: undefined });
  unmount();
});

test('Task Office route invalidates pending reads when disposed', async () => {
  const held = deferred<TaskListPage>();
  const lease = revocableLease();
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: () => held.promise, publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  const unmount = controller.mount();
  const reading = controller.load();
  unmount();
  held.resolve(page('late-task'));
  await reading;
  assert.deepEqual(states.at(-1), { tasks: [], loading: true, error: undefined }, 'disposed controller publishes no late state');
});

test('Task Office route subscribes only after mount, clears if the lease was already revoked, and releases the listener on cleanup', async () => {
  const lease = revocableLease();
  const states: Array<{ tasks: string[]; loading: boolean; error?: string }> = [];
  const controller = createTaskOfficeListController({ openingLease: lease.lease, read: async () => page('must-not-load'), publish: (state) => states.push({ tasks: state.tasks.map(({ taskId }) => taskId), loading: state.loading, error: state.error }) });
  assert.equal(lease.listenerCount(), 0, 'render-time controller construction must not subscribe');
  lease.revoke();
  const cleanup = controller.mount();
  assert.deepEqual(states.at(-1), { tasks: [], loading: false, error: undefined }, 'mount immediately fails closed for a previously revoked lease');
  assert.equal(lease.listenerCount(), 0, 'already revoked leases invoke the callback immediately without retaining it');
  cleanup();
  await controller.load();
  assert.equal(states.length, 1, 'revoked/unmounted controller cannot start another visible read');

  const mountedLease = revocableLease();
  const mountedController = createTaskOfficeListController({ openingLease: mountedLease.lease, read: async () => page('never-needed'), publish: () => {} });
  assert.equal(mountedLease.listenerCount(), 0, 'controller construction has no render-time subscription');
  const mountedCleanup = mountedController.mount();
  assert.equal(mountedLease.listenerCount(), 1, 'committed mount owns one revocation listener');
  mountedCleanup();
  assert.equal(mountedLease.listenerCount(), 0, 'unmount releases the revocation callback');
});
