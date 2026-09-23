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

test('detail maps the snapshot wire (with task section) onto the module DTO', async () => {
  const execution = {
    schema_version: 1, run_id: 'r1', session_id: 's1', revision: 7, driver: 'platform',
    run_status: 'waiting_user', execution_status: 'waiting_user', settlement_status: 'pending', seq: 4, capabilities: {},
  };
  const wireEvent = { schema_version: 1, run_id: 'r1', attempt_id: 'a1', seq: 4, type: 'interaction.required', occurred_at: '2026-09-23T00:00:00Z', payload: { kind: 'tool_approval' } };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { execution, watermark: 4, incomplete: false, confirmed_watermark: 4, events: [wireEvent], task: { task_id: 's1', title: '报告', attention: 'required', archived_at: '2026-09-23T01:00:00Z' } } }),
  });
  const detail = await remote.detail('r1');
  assert.deepEqual(detail, {
    taskId: 's1', runId: 'r1', title: '报告', attention: 'required', archivedAt: '2026-09-23T01:00:00Z',
    execution: { runStatus: 'waiting_user', executionStatus: 'waiting_user', settlementStatus: 'pending', revision: 7, seq: 4 },
    watermark: 4, incomplete: false,
    events: [{ runId: 'r1', seq: 4, type: 'interaction.required', occurredAt: '2026-09-23T00:00:00Z', payload: { kind: 'tool_approval' } }],
  });
});

test('detail defaults the task layer for a legacy server without the task section', async () => {
  const execution = { schema_version: 1, run_id: 'r1', session_id: 's1', revision: 0, driver: 'platform', run_status: 'running', execution_status: 'running', settlement_status: 'pending', seq: 1, capabilities: {} };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { execution, watermark: 1, incomplete: false, confirmed_watermark: 1, events: [] } }),
  });
  const detail = await remote.detail('r1');
  assert.equal(detail.title, '');
  assert.equal(detail.attention, 'none');
  assert.equal('archivedAt' in detail, false);
});

test('the stream resumes from the cursor, routes control frames separately and never parses them as business events', async () => {
  const business = { schema_version: 1, run_id: 'r1', attempt_id: 'a1', seq: 6, type: 'run.completed', occurred_at: '2026-09-23T00:00:00Z', payload: {} };
  const seenRequests: Array<{ path: string; headers?: Record<string, string> }> = [];
  const wire = [
    ': heartbeat\n\n',
    `id: 6\nevent: run.completed\ndata: ${JSON.stringify(business)}\n\n`,
    `event: control\ndata: ${JSON.stringify({ code: 'cursor_expired', message: 'history trimmed' })}\n\n`,
  ].join('');
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('no JSON call expected'); },
    stream: async (input, onChunk) => {
      seenRequests.push({ path: input.path, headers: input.headers });
      onChunk(wire.slice(0, 20));
      onChunk(wire.slice(20));
    },
  });
  const events: unknown[] = [];
  const controls: Array<{ code: string; message: string }> = [];
  await remote.stream({ runId: 'r1', cursor: 5, signal: new AbortController().signal, onEvent: (event) => events.push(event), onControl: (frame) => controls.push(frame) });
  assert.equal(seenRequests[0]!.path, '/api/v1/workbench/executions/r1/events?version=2');
  assert.equal(seenRequests[0]!.headers?.['Last-Event-ID'], '5', 'resume continues from the module cursor');
  assert.deepEqual(events, [{ runId: 'r1', seq: 6, type: 'run.completed', occurredAt: '2026-09-23T00:00:00Z', payload: {} }]);
  assert.deepEqual(controls, [{ code: 'cursor_expired', message: 'history trimmed' }], 'control frames classify separately');
});

test('a 409 stream failure is translated to TASK_STREAM_CURSOR_EXPIRED and missing transport fails closed', async () => {
  const failing = async (): Promise<void> => {
    const error = new Error('HTTP 409');
    error.name = 'ApiError';
    (error as unknown as { status?: number }).status = 409;
    throw error;
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => ({}), stream: failing });
  await assert.rejects(
    remote.stream({ runId: 'r1', cursor: 9, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_STREAM_CURSOR_EXPIRED',
  );
  const unstreamed = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => ({}) });
  await assert.rejects(
    unstreamed.stream({ runId: 'r1', cursor: 9, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    /stream transport is required/,
  );
});

test('a malformed SSE frame fails the stream as a transport error, not a bare SyntaxError', async () => {
  const wire = 'event: control\ndata: {not-json}\n\n';
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('no JSON call expected'); },
    stream: async (_input, onChunk) => { onChunk(wire); },
  });
  await assert.rejects(
    remote.stream({ runId: 'r1', cursor: 5, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    /TASK_STREAM_MALFORMED_FRAME/,
  );
});
