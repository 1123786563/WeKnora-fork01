import test from 'node:test';
import assert from 'node:assert/strict';
import { createRunProgressPresenter, runProgressStatusLabel } from './live-progress.ts';

function ports(overrides: Partial<Parameters<typeof createRunProgressPresenter>[0]> = {}) {
  const presented: { live: string[]; notifications: Array<{ runID: string; title: string; body: string }>; cleared: string[] } = { live: [], notifications: [], cleared: [] };
  const base = {
    isLiveActivityAvailable: () => true,
    setLiveActivity: async (hint: { runID: string; label: string }) => { presented.live.push(`${hint.runID}:${hint.label}`); },
    clearLiveActivity: async (runID: string) => { presented.cleared.push(runID); },
    notify: async (request: { runID: string; title: string; body: string }) => { presented.notifications.push(request); },
  };
  return { ports: { ...base, ...overrides } as Parameters<typeof createRunProgressPresenter>[0], presented };
}

test('a run status rides the Live Activity when the capability exists', async () => {
  const { ports: input, presented } = ports();
  const presenter = createRunProgressPresenter(input);
  await presenter.onRunStatus('run-1', 'running');
  assert.deepEqual(presented.live, ['run-1:运行中']);
  assert.deepEqual(presented.notifications, []);
});

test('a missing Live Activity degrades to a plain notification and keeps the foreground usable', async () => {
  const { ports: input, presented } = ports({ isLiveActivityAvailable: () => false, setLiveActivity: undefined });
  const presenter = createRunProgressPresenter(input);
  await presenter.onRunStatus('run-1', 'pending');
  assert.deepEqual(presented.live, []);
  assert.equal(presented.notifications.length, 1);
  assert.equal(presented.notifications[0].runID, 'run-1');
  assert.match(presented.notifications[0].title, /任务/);
  // The body carries only the status label — foreground recovery stays the
  // W12 pass, the notification never tries to render conversation content.
  assert.equal(presented.notifications[0].body, '任务排队中');
});

test('a failing Live Activity falls back to the plain notification', async () => {
  const { ports: input, presented } = ports({ setLiveActivity: async () => { throw new Error('activitykit unavailable'); } });
  const presenter = createRunProgressPresenter(input);
  await presenter.onRunStatus('run-1', 'running');
  assert.deepEqual(presented.live, []);
  assert.equal(presented.notifications.length, 1);
});

test('background progress never leaks message content', async () => {
  const { ports: input, presented } = ports();
  const presenter = createRunProgressPresenter(input);
  // Even an adversarial status string can only reach the fixed label map —
  // there is no content input anywhere on this surface.
  await presenter.onRunStatus('run-1', 'running');
  await presenter.clear('run-1');
  const serialized = JSON.stringify(presented);
  assert.ok(!serialized.includes('SECRET'), 'no conversation content may reach the notification surface');
  assert.ok(!serialized.includes('message'), 'no message payload may reach the notification surface');
});

test('terminal statuses map to fixed final labels and clear the activity', async () => {
  const { ports: input, presented } = ports();
  const presenter = createRunProgressPresenter(input);
  await presenter.onRunStatus('run-1', 'succeeded');
  assert.deepEqual(presented.cleared, ['run-1']);
  assert.equal(presented.notifications.length, 1);
  assert.match(presented.notifications[0].title, /完成/);
  assert.equal(presenter.lastHint(), null);
});

test('duplicate statuses present exactly once', async () => {
  const { ports: input, presented } = ports();
  const presenter = createRunProgressPresenter(input);
  await presenter.onRunStatus('run-1', 'running');
  await presenter.onRunStatus('run-1', 'running');
  assert.deepEqual(presented.live, ['run-1:运行中']);
});

test('the presenter owns no worker, no audio and no navigation surface', async () => {
  const { ports: input } = ports();
  const presenter = createRunProgressPresenter(input);
  // Structural guarantee: the only entries are the status subscription and
  // the clear pass — nothing that executes work, holds audio sessions or
  // navigates the app.
  assert.deepEqual(Object.keys(presenter).sort(), ['clear', 'lastHint', 'onRunStatus']);
  assert.equal(runProgressStatusLabel('running'), '运行中');
  assert.equal(runProgressStatusLabel('weird-status'), '状态更新');
});
