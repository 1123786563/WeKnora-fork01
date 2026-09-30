import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileVoiceSessionRemote } from './voice-sessions.ts';

interface RecordedCall { method: string; path: string; headers?: Record<string, string>; body?: unknown }

function remoteOf(responder: (call: RecordedCall) => unknown): { remote: ReturnType<typeof createMobileVoiceSessionRemote>; calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const remote = createMobileVoiceSessionRemote({
    origin: 'https://weknora.example.test',
    request: async (input: ClientRequest) => {
      calls.push({ method: input.method, path: input.path, headers: input.headers, body: input.body });
      const result = responder(calls[calls.length - 1]!);
      if (result instanceof Error) throw result;
      return result;
    },
  });
  return { remote, calls };
}

const ok = (data: unknown): unknown => ({ success: true, data });
const httpError = (status: number, serverError: string): Error =>
  Object.assign(new Error(serverError), { status });

test('open posts the W30 session wire: session_id/run_id/max_seconds body and parses the grant envelope', async () => {
  const { remote, calls } = remoteOf(() => ok({ id: 'vs_1', token: 'wvp1.abc.def', expires_at: '2026-09-26T00:10:00Z', max_seconds: 600, price_version: 'voice-v1' }));
  const grant = await remote.open({ productSessionId: 'room-1', runId: 'run-9', maxSeconds: 300 });
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.method, 'POST');
  assert.equal(calls[0]!.path, '/api/v1/mobile/voice/sessions');
  assert.deepEqual(calls[0]!.body, { session_id: 'room-1', run_id: 'run-9', max_seconds: 300 });
  assert.deepEqual(grant, { id: 'vs_1', token: 'wvp1.abc.def', expiresAt: '2026-09-26T00:10:00Z', maxSeconds: 600, priceVersion: 'voice-v1' });
});

test('open parses an idempotent replay envelope without a token (token_issued false path)', async () => {
  const { remote } = remoteOf(() => ok({ id: 'vs_1', state: 'open', expires_at: '2026-09-26T00:10:00Z', max_seconds: 600, token_issued: false, replay: true }));
  const grant = await remote.open({ productSessionId: 'room-1' });
  assert.equal(grant.token, undefined, '重放响应没有明文 token——字段如实缺席');
  assert.equal(grant.maxSeconds, 600);
});

test('open error translation covers the full W30 status matrix', async () => {
  const cases: Array<{ status: number; serverError: string; expected: string }> = [
    { status: 400, serverError: 'invalid_voice_session_request', expected: 'VOICE_SESSION_INVALID' },
    { status: 400, serverError: 'voice_window_too_short', expected: 'VOICE_SESSION_INVALID' },
    { status: 402, serverError: 'voice_budget_denied', expected: 'VOICE_BUDGET_DENIED' },
    { status: 409, serverError: 'voice_session_unknown_pending', expected: 'VOICE_SESSION_CONFLICT' },
    { status: 409, serverError: 'voice_session_exists', expected: 'VOICE_SESSION_CONFLICT' },
    { status: 502, serverError: 'voice_provider_unavailable', expected: 'VOICE_PROVIDER_UNAVAILABLE' },
    { status: 503, serverError: 'voice_charging_unconfigured', expected: 'VOICE_CHARGING_UNCONFIGURED' },
    { status: 503, serverError: 'voice_open_unknown', expected: 'VOICE_OPEN_UNKNOWN' },
  ];
  for (const testCase of cases) {
    const { remote } = remoteOf(() => {
      throw httpError(testCase.status, testCase.serverError);
    });
    await assert.rejects(() => remote.open({ productSessionId: 'room-1' }), (error: unknown) => (error as { code?: string }).code === testCase.expected, `${testCase.status} ${testCase.serverError} → ${testCase.expected}`);
  }
});

test('end deletes the session id (encodeURIComponent) and parses settle receipts honestly', async () => {
  const { remote, calls } = remoteOf(() => ok({ id: 'vs 1', state: 'closed', settled: true, audio_seconds: 5 }));
  const receipt = await remote.end('vs 1');
  assert.equal(calls[0]!.method, 'DELETE');
  assert.equal(calls[0]!.path, '/api/v1/mobile/voice/sessions/vs%201');
  assert.deepEqual(receipt, { id: 'vs 1', state: 'closed', settled: true });
  const { remote: replayRemote } = remoteOf(() => ok({ id: 'vs_1', state: 'closed', settled: true, replay: true }));
  assert.deepEqual(await replayRemote.end('vs_1'), { id: 'vs_1', state: 'closed', settled: true, replay: true });
  const { remote: retryRemote } = remoteOf(() => ok({ id: 'vs_1', state: 'unknown', settled: false, reason: 'pending_reconciliation' }));
  assert.deepEqual(await retryRemote.end('vs_1'), { id: 'vs_1', state: 'unknown', settled: false, reason: 'pending_reconciliation' });
});

test('end error translation: 404 is not-found, other http failures are end-failed', async () => {
  const { remote: missing } = remoteOf(() => {
    throw httpError(404, 'Not Found');
  });
  await assert.rejects(() => missing.end('vs_x'), (error: unknown) => (error as { code?: string }).code === 'VOICE_SESSION_NOT_FOUND');
  const { remote: broken } = remoteOf(() => {
    throw httpError(500, 'voice_session_store_unavailable');
  });
  await assert.rejects(() => broken.end('vs_x'), (error: unknown) => (error as { code?: string }).code === 'VOICE_SESSION_END_FAILED');
});

test('malformed envelopes are rejected rather than trusted', async () => {
  const { remote } = remoteOf(() => ({ success: false, error: 'nope' }));
  await assert.rejects(() => remote.open({ productSessionId: 'room-1' }), /success envelope/);
  const { remote: shapeless } = remoteOf(() => ok({ expires_at: 'x', max_seconds: 1 }));
  await assert.rejects(() => shapeless.open({ productSessionId: 'room-1' }), /data\.id/);
});

test('origin is validated at construction', () => {
  assert.throws(() => createMobileVoiceSessionRemote({ origin: 'http://insecure.example', request: async () => undefined }));
});
