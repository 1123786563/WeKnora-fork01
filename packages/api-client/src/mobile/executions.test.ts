import test from 'node:test';
import assert from 'node:assert/strict';
import { ContractError } from '@weknora/contracts';
import { ApiError } from '../errors.ts';
import type { ClientRequest } from '../client.ts';
import { createExecutionsApi, executionEventsRequest, type ExecutionCommandInput, type StartExecutionInput } from './executions.ts';

const execution = {
  schema_version: 1,
  run_id: 'run/1',
  session_id: 'session-1',
  revision: 2,
  driver: 'platform',
  run_status: 'running',
  execution_status: 'running',
  settlement_status: 'reserved',
  seq: 4,
  capabilities: { cancel: { state: 'supported', reason: '' } },
};

const snapshot = {
  execution,
  watermark: 4,
  events: [{
    schema_version: 1,
    run_id: 'run/1',
    attempt_id: 'attempt-1',
    seq: 4,
    type: 'run.started',
    occurred_at: '2026-09-16T00:00:00Z',
    payload: {},
  }],
};

function respond(data: unknown) {
  return async (_input: ClientRequest): Promise<unknown> => ({ success: true, data });
}

test('timeout cannot silently retry a billable start', async () => {
  let calls = 0;
  const api = createExecutionsApi(async () => { calls += 1; throw new Error('TIMEOUT'); });
  const input: StartExecutionInput = {
    request_id: 'q', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 'hello', budget_upper: 10,
  };
  await assert.rejects(api.start(input), /TIMEOUT/);
  assert.equal(calls, 1);
});

test('uses encoded paths, exact bodies, and forwards abort signals', async () => {
  const requests: ClientRequest[] = [];
  const api = createExecutionsApi(async (input) => {
    requests.push(input);
    return {
      success: true,
      data: input.path.includes('/requests/')
        ? { state: 'pending' }
        : input.path.endsWith('/snapshot')
          ? snapshot
          : input.path.endsWith('/commands')
            ? { run_id: 'run/1', action: 'steer' }
            : input.method === 'POST'
              ? { run_id: 'run/1', request_id: 'req-1', status: 'queued' }
              : execution,
    };
  });
  const signal = new AbortController().signal;
  await api.get('run/1', signal);
  await api.snapshot('run/1', signal);
  const start: StartExecutionInput = {
    request_id: 'req-1', session_id: 's/1', agent_id: 'a', target_id: 'platform', workspace_ref: 'w', text: 'hello', budget_upper: 4,
  };
  await api.start(start, signal);
  const command: ExecutionCommandInput = { action: 'steer', text: 'continue', expected_revision: 2 };
  await api.command('run/1', command, signal);
  await api.lookup('req/1', signal);
  assert.deepEqual(requests.map(({ method, path, body, signal: actual }) => ({ method, path, body, signal: actual })), [
    { method: 'GET', path: '/api/v1/workbench/executions/run%2F1', body: undefined, signal },
    { method: 'GET', path: '/api/v1/workbench/executions/run%2F1/snapshot', body: undefined, signal },
    { method: 'POST', path: '/api/v1/workbench/executions', body: start, signal },
    { method: 'POST', path: '/api/v1/workbench/executions/run%2F1/commands', body: command, signal },
    { method: 'GET', path: '/api/v1/workbench/executions/requests/req%2F1', body: undefined, signal },
  ]);
});

test('unwraps and validates execution and snapshot contracts', async () => {
  const api = createExecutionsApi(respond(execution));
  assert.deepEqual(await api.get('run/1'), execution);
  const snapshotApi = createExecutionsApi(respond(snapshot));
  assert.deepEqual(await snapshotApi.snapshot('run/1'), snapshot);
  const malformed = createExecutionsApi(async () => ({ success: true, data: { ...execution, run_id: '' } }));
  await assert.rejects(malformed.get('r'), (error: unknown) => error instanceof ContractError);
  const wrongEnvelope = createExecutionsApi(async () => ({ success: false, data: execution }));
  await assert.rejects(wrongEnvelope.get('r'), /success/);
});

test('lookup returns explicit reconciliation states and rejects ambiguous payloads', async () => {
  const api = createExecutionsApi(respond({ state: 'pending' }));
  assert.deepEqual(await api.lookup('req'), { state: 'pending' });
  const unknown = createExecutionsApi(respond({ state: 'unknown', reason: 'not found' }));
  assert.deepEqual(await unknown.lookup('req'), { state: 'unknown', reason: 'not found' });
  const invalid = createExecutionsApi(respond({ state: 'pending', run_id: 42 }));
  await assert.rejects(invalid.lookup('req'), /run_id/);
  const boolean = createExecutionsApi(respond(true));
  await assert.rejects(boolean.lookup('req'), /data/);
});

test('preserves typed HTTP errors and never turns malformed 200 into success', async () => {
  const error = new ApiError({ status: 409, code: 'CONFLICT', message: 'already exists' });
  const api = createExecutionsApi(async () => { throw error; });
  await assert.rejects(api.start({ request_id: 'q', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 'x', budget_upper: 1 }), (actual: unknown) => actual === error);
  const invalid = createExecutionsApi(async () => ({ success: true, data: { nope: true } }));
  await assert.rejects(invalid.command('run', { action: 'cancel', expected_revision: 0 }), /run_id/);
});

test('builds a resumable event request without inventing cursor semantics', () => {
  assert.deepEqual(executionEventsRequest('run/1'), {
    method: 'GET', path: '/api/v1/workbench/executions/run%2F1/events', headers: undefined,
  });
  assert.deepEqual(executionEventsRequest('run/1', '4'), {
    method: 'GET', path: '/api/v1/workbench/executions/run%2F1/events', headers: { 'Last-Event-ID': '4' },
  });
  assert.throws(() => executionEventsRequest('run/1', ' '), /lastEventID/);
  assert.throws(() => executionEventsRequest('run/1', '-1'), /safe sequence/);
  assert.throws(() => executionEventsRequest('run/1', 'abc'), /safe sequence/);
  assert.throws(() => executionEventsRequest('run/1', '9007199254740992'), /safe sequence/);
});

test('parses the exact W04 start and W05 command acknowledgement fixtures', async () => {
  const input: StartExecutionInput = {
    request_id: 'req-1', session_id: 's-1', agent_id: 'a-1', target_id: 'platform', workspace_ref: '', text: 'hello', budget_upper: 10,
  };
  const api = createExecutionsApi(async ({ method, path }) => ({
    success: true,
    data: method === 'POST' && path.endsWith('/commands')
      ? { run_id: 'run-1', action: 'cancel' }
      : { run_id: 'run-1', request_id: 'req-1', status: 'queued' },
  }));
  assert.deepEqual(await api.start(input), { run_id: 'run-1', request_id: 'req-1', status: 'queued' });
  assert.deepEqual(await api.command('run-1', { action: 'cancel', expected_revision: 0 }), { run_id: 'run-1', action: 'cancel' });
});

test('list encodes facets without tenant/owner and parses the page', async () => {
  const requests: ClientRequest[] = [];
  const page = {
    items: [{
      run_id: 'run-1', session_id: 's-1', agent_id: 'a&tenant_id=other', target_id: 'platform',
      workspace_ref: 'ws', space_id: 'sp', status: 'running', created_at: '2026-09-12T10:00:00Z', updated_at: '2026-09-12T10:00:01Z',
    }],
    next_cursor: 'cur-1',
  };
  const api = createExecutionsApi(async (input) => {
    requests.push(input);
    return { success: true, data: page };
  });
  const signal = new AbortController().signal;
  const result = await api.list({ status: 'running', agent_id: 'a&tenant_id=other', cursor: 'cur-0', limit: 30 }, signal);
  assert.equal(requests[0].method, 'GET');
  assert.equal(requests[0].path, '/api/v1/workbench/executions?status=running&agent_id=a%26tenant_id%3Dother&cursor=cur-0&limit=30');
  assert.equal(requests[0].signal, signal);
  assert.deepEqual(result, {
    items: [{
      run_id: 'run-1', session_id: 's-1', agent_id: 'a&tenant_id=other', target_id: 'platform',
      workspace_ref: 'ws', space_id: 'sp', status: 'running', created_at: '2026-09-12T10:00:00Z', updated_at: '2026-09-12T10:00:01Z',
    }],
    next_cursor: 'cur-1',
  });

  const emptyApi = createExecutionsApi(respond({ items: [] }));
  assert.deepEqual(await emptyApi.list(), { items: [] });
});

test('list rejects malformed pages instead of trusting them', async () => {
  const badStatus = createExecutionsApi(respond({ items: [{ run_id: 'r', session_id: 's', status: 'jogging', created_at: 'c', updated_at: 'u' }] }));
  await assert.rejects(badStatus.list(), /status/);
  const missingRun = createExecutionsApi(respond({ items: [{ session_id: 's', status: 'running', created_at: 'c', updated_at: 'u' }] }));
  await assert.rejects(missingRun.list(), /run_id/);
  const noItems = createExecutionsApi(respond({ next_cursor: 'c' }));
  await assert.rejects(noItems.list(), /items/);
  const badCursor = createExecutionsApi(respond({ items: [], next_cursor: 7 }));
  await assert.rejects(badCursor.list(), /next_cursor/);
});
