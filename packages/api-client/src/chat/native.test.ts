import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError } from '@weknora/contracts';
import { ApiError } from '../errors.ts';
import type { HttpStreamResult } from '../ports.ts';
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

test('keeps missing or malformed version fields as contract errors', async () => {
  const missingVersion = event({ protocol: undefined });
  await assert.rejects(
    consumeNativeEventStream(async () => `data: ${JSON.stringify(missingVersion)}\n\n`, { sessionId: 'session-1', runId: runID }, () => {}),
    (error: unknown) => error instanceof ContractError && error.path === 'protocol',
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

test('consumes HttpTransport chunks incrementally and retains a non-success stream status', async () => {
  const seen: string[] = [];
  const streamed: HttpStreamResult = {
    status: 200, headers: {}, chunks: (async function* () {
      yield `id: event-8\ndata: ${JSON.stringify(event()).slice(0, 80)}`;
      yield `${JSON.stringify(event()).slice(80)}\n\n`;
    })(),
  };
  const api = createNativeAgentApi({ request: async () => { throw new Error('buffered request must not run'); }, sendStream: async () => streamed });
  await api.stream({ sessionId: 'session-1', runId: runID, lastEventId: cursor }, (item) => seen.push(item.event_id));
  assert.deepEqual(seen, ['event-8']);

  const failed = createNativeAgentApi({ request: async () => undefined, sendStream: async () => ({ status: 403, headers: {}, chunks: (async function* () {})() }) });
  await assert.rejects(failed.stream({ sessionId: 'session-1', runId: runID }, () => {}), (error: unknown) => (
    typeof error === 'object' && error !== null && (error as { status?: unknown }).status === 403
  ));
});

test('reloads the authoritative snapshot after 409 and reconnects with its cursor', async () => {
  const requests: Array<{ path: string; headers?: Record<string, string> }> = [];
  let opens = 0;
  const api = createNativeAgentApi({
    request: async (request) => {
      requests.push({ path: request.path, headers: request.headers });
      return { success: true, data: { last_event_id: 'v1:cnVuLzE:8' } };
    },
    sendStream: async (request) => {
      requests.push({ path: request.path, headers: request.headers });
      opens += 1;
      return opens === 1
        ? { status: 409, headers: {}, chunks: (async function* () {})() }
        : { status: 200, headers: {}, chunks: (async function* () { yield `data: ${JSON.stringify(event({ event_id: 'event-9', seq: '9' }))}\n\n`; })() };
    },
  });
  const received: string[] = [];
  await api.follow({ sessionId: 'session-1', runId: runID, lastEventId: cursor }, (item) => received.push(item.event_id));
  assert.deepEqual(received, ['event-9']);
  assert.equal(requests[1]?.path, '/api/v1/native-agent/sessions/session-1/runs/run%2F1');
  assert.equal(requests[2]?.headers?.['Last-Event-ID'], 'v1:cnVuLzE:8');
});

test('aborts an old scope before it can open a reconnect stream', async () => {
  let opened = 0;
  let captured: AbortSignal | undefined;
  const api = createNativeAgentApi({
    request: async () => ({ success: true, data: { last_event_id: cursor } }),
    sendStream: async (request) => {
      opened += 1;
      captured = request.signal;
      return { status: 409, headers: {}, chunks: (async function* () {})() };
    },
  });
  const lifecycle = api.createLifecycle();
  const pending = lifecycle.follow('tenant-1/session-1', { sessionId: 'session-1', runId: runID, lastEventId: cursor }, () => {});
  lifecycle.advanceScope('tenant-2/session-2');
  await pending;
  assert.equal(captured?.aborted, true);
  assert.equal(opened, 1);
});
