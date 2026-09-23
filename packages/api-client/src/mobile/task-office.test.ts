import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createTaskOfficeRemote } from './task-office.ts';

const overviewData = {
  counts: { active_runs: 1, pending_interactions: 2, unread_notifications: 3 },
  in_progress: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', run_status: 'waiting_user',
    execution_status: 'waiting_user', settlement_status: 'pending', attention: 'required', updated_at: '2026-09-23T00:00:00Z',
  }],
  pending_interactions: [
    { id: 'i1', kind: 'tool_approval', created_at: '2026-09-23T00:00:00Z' },
    { id: 'i2', kind: 'budget', created_at: '2026-09-23T00:00:01Z' },
  ],
  recently_completed: [{
    run_id: 'r2', session_id: 't2', title: 'finished research', run_status: 'succeeded',
    execution_status: 'succeeded', settlement_status: 'settled', attention: 'none', updated_at: '2026-09-22T00:00:00Z',
  }],
  recent_artifacts: [],
  as_of: '2026-09-23T00:00:02Z',
};

const listData = {
  items: [{
    run_id: 'r1', session_id: 't1', title: 'weekly report', status: 'running',
    attention: 'required', archived_at: '2026-09-23T01:00:00Z', created_at: '2026-09-22T00:00:00Z', updated_at: '2026-09-23T00:00:00Z',
  }],
  next_cursor: 'cursor-2',
};

test('task office remote maps the overview wire into the module DTO', async () => {
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: overviewData }),
  });
  const overview = await remote.overview();
  assert.deepEqual(overview, {
    needsMe: [
      { interactionId: 'i1', kind: 'tool_approval', createdAt: '2026-09-23T00:00:00Z' },
      { interactionId: 'i2', kind: 'budget', createdAt: '2026-09-23T00:00:01Z' },
    ],
    running: [{ runId: 'r1', taskId: 't1', title: 'weekly report', runStatus: 'waiting_user', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }],
    recentlyCompleted: [{ runId: 'r2', taskId: 't2', title: 'finished research', runStatus: 'succeeded', attention: 'none', updatedAt: '2026-09-22T00:00:00Z' }],
    unreadNotifications: 3,
    asOf: '2026-09-23T00:00:02Z',
  });
});

test('task office remote forwards list facets and maps rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return input.path.includes('/workbench/executions?') ? { success: true, data: listData } : { success: true, data: {} };
    },
  });
  const page = await remote.list({ search: 'quarterly', archived: true, status: 'succeeded', limit: 50 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions?status=succeeded&q=quarterly&archived=true&limit=50');
  assert.deepEqual(page, {
    items: [{ runId: 'r1', taskId: 't1', title: 'weekly report', runStatus: 'running', attention: 'required', updatedAt: '2026-09-23T00:00:00Z' }],
    nextCursor: 'cursor-2',
  });
});

test('archive and restore hit the task lifecycle routes with encoded ids', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { task_id: 'ignored', archived: true } };
    },
  });
  await remote.archive('task/9');
  await remote.restore('task/9');
  assert.deepEqual(requests.map((input) => `${input.method} ${input.path}`), [
    'POST /api/v1/workbench/tasks/task%2F9/archive',
    'DELETE /api/v1/workbench/tasks/task%2F9/archive',
  ]);
});

test('non-success envelopes and malformed origins fail loudly', async () => {
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: false, error: 'nope' }),
  });
  await assert.rejects(remote.overview(), /success/);
  await assert.rejects(remote.archive('t1'), /success/);
  assert.throws(() => createTaskOfficeRemote({ origin: 'http://insecure.example.test', request: async () => ({}) }), /HTTPS/);
});
