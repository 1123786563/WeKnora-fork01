import test from 'node:test';
import assert from 'node:assert/strict';
import { createExpoNotificationsProgressPort } from './native-progress-port.ts';

/** Structural expo-notifications double: records every observable call. */
function expoDouble(overrides: Partial<{ granted: boolean; scheduleError: Error }> = {}) {
  const calls: { permissions: number; scheduled: Array<{ title: string; body: string; data: Record<string, unknown> }>; dismissed: string[] } = {
    permissions: 0,
    scheduled: [],
    dismissed: [],
  };
  let nextID = 0;
  const mod = {
    getPermissionsAsync: async () => { calls.permissions += 1; return { granted: overrides.granted ?? true }; },
    scheduleNotificationAsync: async (request: { content: { title: string; body: string; data?: Record<string, unknown> } }) => {
      if (overrides.scheduleError) throw overrides.scheduleError;
      nextID += 1;
      calls.scheduled.push({ title: request.content.title, body: request.content.body, data: request.content.data ?? {} });
      return `notif-${nextID}`;
    },
    dismissNotificationAsync: async (identifier: string) => { calls.dismissed.push(identifier); },
  };
  return { mod, calls };
}

test('granted permission posts a status-only local notification', async () => {
  const { mod, calls } = expoDouble();
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务运行中' });
  assert.equal(calls.permissions, 1);
  assert.deepEqual(calls.scheduled, [{ title: '任务进行中', body: '任务运行中', data: { runID: 'run-1' } }]);
});

test('a missing permission posts nothing — no fake notification surface', async () => {
  const { mod, calls } = expoDouble({ granted: false });
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务运行中' });
  assert.deepEqual(calls.scheduled, []);
});

test('a new status replaces the previous progress notification for the same run', async () => {
  const { mod, calls } = expoDouble();
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务排队中' });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务运行中' });
  assert.equal(calls.scheduled.length, 2);
  assert.deepEqual(calls.dismissed, ['notif-1']);
});

test('clearLiveActivity dismisses the tracked notification idempotently', async () => {
  const { mod, calls } = expoDouble();
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务运行中' });
  await port.clearLiveActivity!('run-1');
  await port.clearLiveActivity!('run-1');
  assert.deepEqual(calls.dismissed, ['notif-1']);
});

test('the port declares the Live Activity native module absent (seam stays declaration-only)', async () => {
  const { mod } = expoDouble();
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  assert.equal(port.isLiveActivityAvailable(), false);
  assert.equal(port.setLiveActivity, undefined);
});

test('a scheduling failure never rejects the presenter path', async () => {
  const { mod, calls } = expoDouble({ scheduleError: new Error('poster unavailable') });
  const port = createExpoNotificationsProgressPort({ notifications: mod });
  await port.notify({ runID: 'run-1', title: '任务进行中', body: '任务运行中' });
  assert.equal(calls.scheduled.length, 0);
});
