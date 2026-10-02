import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createTaskOfficeRemote } from './task-office.ts';
import { ApiError } from '../errors.ts';

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

test('task office remote creates the goal session, starts the durable run and reconciles by request id', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const request = async (input: ClientRequest) => {
    requests.push({ method: input.method, path: input.path, body: input.body });
    if (input.method === 'POST' && input.path === '/api/v1/sessions') {
      return { success: true, data: { id: 'session-77', title: '整理本周反馈并生成周报', is_pinned: false } };
    }
    if (input.method === 'POST' && input.path === '/api/v1/workbench/executions') {
      return { success: true, data: { run_id: 'run-77', request_id: (input.body as { request_id: string }).request_id, status: 'queued' } };
    }
    if (input.method === 'GET' && input.path === '/api/v1/workbench/executions/requests/req-77') {
      return { success: true, data: { state: 'admitted', run_id: 'run-77' } };
    }
    throw new Error(`unexpected ${input.method} ${input.path}`);
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request });

  const session = await remote.createSession({ title: '整理本周反馈并生成周报' });
  assert.equal(session.sessionId, 'session-77');

  const ack = await remote.start({ request_id: 'req-77', session_id: 'session-77', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '整理本周反馈并生成周报', budget_upper: 200 });
  assert.deepEqual(ack, { run_id: 'run-77', request_id: 'req-77', status: 'queued' });

  const lookup = await remote.lookup('req-77');
  assert.deepEqual(lookup, { state: 'admitted', run_id: 'run-77' });

  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/sessions', body: { title: '整理本周反馈并生成周报', engine_type: 'trpc' } },
    { method: 'POST', path: '/api/v1/workbench/executions', body: { request_id: 'req-77', session_id: 'session-77', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '整理本周反馈并生成周报', budget_upper: 200 } },
    { method: 'GET', path: '/api/v1/workbench/executions/requests/req-77', body: undefined },
  ]);
});

test('task office remote start validates the frozen seven fields before any request', async () => {
  let calls = 0;
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => { calls += 1; return {}; } });
  await assert.rejects(
    remote.start({ request_id: '', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 't', budget_upper: 1 }),
    /request_id/,
  );
  await assert.rejects(
    remote.start({ request_id: 'r', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 't', budget_upper: -1 }),
    /budget_upper/,
  );
  assert.equal(calls, 0, 'validation must reject before any HTTP traffic');
});

test('task office remote maps the inbox wire into module DTO rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: [
          { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z' },
        ],
      };
    },
  });
  const items = await remote.inbox();
  assert.deepEqual(items, [{ interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z' }]);
  assert.equal(requests[0].path, '/api/v1/workbench/interactions?limit=50');
});

test('task office remote decide maps the ack and classifies honest outcomes', async () => {
  const requests: ClientRequest[] = [];
  const ack = { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 5, run_id: 'run-1' };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      if (requests.length === 1) return { success: true, data: ack };
      throw new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' });
    },
  });
  const item = { interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval' as const, argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '' };
  const record = await remote.decide({ item, decisionId: 'd-1', action: 'approve' });
  assert.deepEqual(record, { interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', decisionId: 'd-1', action: 'approve', argsHash: 'sha256:aa', expectedRevision: 5 });
  assert.deepEqual(requests[0].body, { id: 'i-1', decision_id: 'd-1', kind: 'tool_approval', action: 'approve', args_hash: 'sha256:aa', expected_revision: 4 });

  const coded = async (error: ApiError): Promise<string | undefined> => {
    const failing = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => { throw error; } });
    try {
      await failing.decide({ item, decisionId: 'd-1', action: 'approve' });
      return undefined;
    } catch (caught) {
      return (caught as { code?: string }).code;
    }
  };
  assert.equal(await coded(new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' })), 'INTERACTION_SUPERSEDED');
  // B3-F43：400 是确定性客户端错误（action/kind 不匹配等），原始 ApiError 透传——
  // 不得冒充「已被处理」终态后丢失原因、阻断重试与提示。
  assert.equal(await coded(new ApiError({ status: 400, code: 'HTTP_400', message: 'interaction_action_mismatch' })), 'HTTP_400', '400 透传原始 wire code，不再翻译为 superseded');
  assert.equal(await coded(new ApiError({ status: 502, code: 'command_recovery_unknown', message: 'command_recovery_unknown: remote interaction' })), 'INTERACTION_DELIVERY_UNKNOWN');
  assert.equal(await coded(new ApiError({ status: 502, code: 'HTTP_502', message: 'upstream broke' })), 'HTTP_502', '非 command_recovery_unknown 的 502 不得伪装成 delivery-unknown（原样上抛，wire code 透传）');
  assert.equal(await coded(new ApiError({ status: 410, code: 'HTTP_410', message: 'interaction_expired' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 404, code: 'HTTP_404', message: 'not found' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 403, code: 'HTTP_403', message: 'revoked' })), 'INTERACTION_GONE');
  assert.equal(await coded(new ApiError({ status: 500, code: 'HTTP_500', message: 'boom' })), 'HTTP_500', '未知错误原样上抛（由上层折叠为后端错误）');
});

test('task office remote command: 202 ack maps to runId/nextRunId; 409 and transport failures map to contract codes', async () => {
  const seen: ClientRequest[] = [];
  const request = async (input: ClientRequest): Promise<unknown> => {
    seen.push(input);
    const action = typeof input.body === 'object' && input.body !== null && (input.body as { action?: string }).action === 'queue_next' ? 'queue_next' : 'cancel';
    return { success: true, data: { run_id: 'run-1', action, next_run_id: 'run-2' } };
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example', request });
  const ack = await remote.command({ runId: 'run-1', action: 'queue_next', text: 'next', expectedRevision: 5, intentId: 'qid-1' });
  assert.equal(ack.nextRunId, 'run-2');
  assert.equal(seen[0]?.path, '/api/v1/workbench/executions/run-1/commands');
  assert.deepEqual(seen[0]?.body, { action: 'queue_next', text: 'next', expected_revision: 5, external_pending_id: 'qid-1' });

  const conflictRemote = createTaskOfficeRemote({ origin: 'https://weknora.example', request: async () => { throw new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' }); } });
  await assert.rejects(
    () => conflictRemote.command({ runId: 'run-1', action: 'cancel', expectedRevision: 5 }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_COMMAND_CONFLICT',
  );

  const transportRemote = createTaskOfficeRemote({ origin: 'https://weknora.example', request: async () => { throw new Error('network down'); } });
  await assert.rejects(
    () => transportRemote.command({ runId: 'run-1', action: 'cancel', expectedRevision: 5 }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_COMMAND_UNKNOWN',
  );
});

test('task office remote command folds every ApiError into the two contract codes', async () => {
  // 与 decide 不同：command 的翻译是封闭二分——409/404 确定性冲突，其余（含 502
  // command_recovery_unknown 与 5xx）投递结果未知，一律折叠为 TASK_COMMAND_UNKNOWN。
  const coded = async (error: ApiError): Promise<string | undefined> => {
    const failing = createTaskOfficeRemote({ origin: 'https://weknora.example', request: async () => { throw error; } });
    try {
      await failing.command({ runId: 'run-1', action: 'cancel', expectedRevision: 5 });
      return undefined;
    } catch (caught) {
      return (caught as { code?: string }).code;
    }
  };
  assert.equal(await coded(new ApiError({ status: 404, code: 'HTTP_404', message: 'not found' })), 'TASK_COMMAND_CONFLICT');
  assert.equal(await coded(new ApiError({ status: 502, code: 'command_recovery_unknown', message: 'command_recovery_unknown: remote command' })), 'TASK_COMMAND_UNKNOWN');
  assert.equal(await coded(new ApiError({ status: 500, code: 'HTTP_500', message: 'boom' })), 'TASK_COMMAND_UNKNOWN');
});
