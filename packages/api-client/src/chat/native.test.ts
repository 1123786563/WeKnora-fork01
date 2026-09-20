import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError } from '@weknora/contracts';
import { ApiError } from '../errors.ts';
import { buildNativeEventsRequest, createNativeAgentApi, consumeNativeEventStream, NativeProtocolError } from './native.ts';

const runID = 'run/1';
const cursor = 'v1:cnVuLzE:7';

function event(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    protocol: 'weknora.agent.v1',
    schema_version: 1,
    event_id: 'event-8',
    tenant_id: '9007199254740993',
    session_id: 'session-1',
    run_id: runID,
    attempt_id: 'attempt-1',
    seq: '8',
    kind: 'usage.observed',
    payload: {
      usage: {
        observation_id: 'usage-1', revision: '1', prompt_tokens: '9007199254740993', completion_tokens: '2', total_tokens: '9007199254740995',
        cached_tokens: '0', cache_read_tokens: '0', cache_create_tokens: '0', accounting_status: 'observed',
      },
    },
    ...overrides,
  };
}

test('builds a scoped resumable event request with the canonical Last-Event-ID', () => {
  assert.deepEqual(buildNativeEventsRequest('session/1', runID, cursor), {
    method: 'GET',
    path: '/api/v1/native-agent/sessions/session%2F1/runs/run%2F1/events',
    headers: { accept: 'text/event-stream', 'Last-Event-ID': cursor },
  });
  assert.throws(() => buildNativeEventsRequest('session-1', runID, 'v1:c29tZS1vdGhlci1ydW4:7'), /run_id/);
});

test('parses only the canonical public event projection without losing decimal counters', async () => {
  const received: unknown[] = [];
  await consumeNativeEventStream(
    async () => `id: event-8\ndata: ${JSON.stringify(event())}\n\n`,
    { sessionId: 'session-1', runId: runID, lastEventId: cursor },
    (item) => received.push(item),
  );
  assert.deepEqual(received, [event()]);
  const usage = (received[0] as { payload: { usage: { prompt_tokens: string } } }).payload.usage;
  assert.equal(usage.prompt_tokens, '9007199254740993');
});

test('rejects unsupported native wire versions with an upgrade-safe error', async () => {
  const unsupported = event({ protocol: 'weknora.agent.v2' });
  await assert.rejects(
    consumeNativeEventStream(async () => `data: ${JSON.stringify(unsupported)}\n\n`, { sessionId: 'session-1', runId: runID }, () => {}),
    (error: unknown) => error instanceof NativeProtocolError && error.code === 'client_upgrade_required',
  );
});

test('keeps malformed and private wire projections at the contract-validation boundary', async () => {
  const privateProjection = event({ payload: { ...event().payload as Record<string, unknown>, provider_receipt: 'secret' } });
  await assert.rejects(
    consumeNativeEventStream(async () => `data: ${JSON.stringify(privateProjection)}\n\n`, { sessionId: 'session-1', runId: runID }, () => {}),
    (error: unknown) => error instanceof ContractError && error.path === 'payload.provider_receipt',
  );
});

test('preserves transport errors and sends identical decision ids without client retries', async () => {
  const transportError = new ApiError({ status: 403, code: 'FORBIDDEN', message: 'revoked' });
  const blocked = createNativeAgentApi(async () => { throw transportError; });
  await assert.rejects(blocked.cancel('session-1', runID, { command_id: 'cancel-1', expected_revision: '3' }), (error: unknown) => error === transportError);

  const requests: Array<{ body?: unknown }> = [];
  const api = createNativeAgentApi(async (request) => {
    requests.push({ body: request.body });
    return { success: true, data: { status: 'held' } };
  });
  const input = {
    command_id: 'decision-1', expected_revision: '3', pending_revision: '4', plan_version: 2,
    args_hash: 'sha256:args', call_id: 'call-1', action: 'retry' as const, reason: 'approved',
  };
  await api.resolvePending('session-1', runID, 'pending-1', input);
  await api.resolvePending('session-1', runID, 'pending-1', input);
  assert.deepEqual(requests.map((request) => request.body), [input, input]);
});
