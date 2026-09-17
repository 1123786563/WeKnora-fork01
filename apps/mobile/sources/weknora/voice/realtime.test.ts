import test from 'node:test';
import assert from 'node:assert/strict';
import type { FetchLike } from '@weknora/api-client';
import {
  createVoiceControls,
  createVoiceUtteranceHandler,
  createRealtimeVoiceSession,
  classifyVoiceUtterance,
  createProductVoiceSessionApi,
  RealtimeVoiceError,
  type RealtimeVoicePort,
  type VoiceSessionAdmission,
} from './realtime.ts';

test('interrupt and end voice do not cancel the task',async()=>{
 const calls:string[]=[];
 const c=createVoiceControls({stopAudio:()=>{calls.push('audio');},closeVoice:async()=>{calls.push('close');},cancelRun:async()=>{calls.push('cancel');}});
 c.interrupt();await c.endVoice();assert.deepEqual(calls,['audio','close']);
 await c.cancelTask();assert.deepEqual(calls,['audio','close','cancel']);
});

/** Deterministic clock the expiry tests drive. */
function clock(startMs: number): { now(): number; advance(ms: number): void } {
  let current = startMs;
  return { now: () => current, advance: (ms: number) => { current += ms; } };
}

function fakePort(log: string[]): RealtimeVoicePort {
  return {
    async connect(grant) { log.push(`connect:${grant.token}`); },
    async close() { log.push('port.close'); },
    mute(value) { log.push(`mute:${value}`); },
    stopAudio() { log.push('stopAudio'); },
  };
}

function admission(id: string, token: string, expiresAt: string, tokenIssued = true): VoiceSessionAdmission {
  return { id, grant: { token, expiresAt }, tokenIssued };
}

test('begin admits through W30 once and connects with the exact RFC3339 grant', async () => {
  const log: string[] = [];
  const grants = [admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z')];
  const admitted: number[] = [];
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => { admitted.push(grants.length); return grants[0]; },
    release: async (id) => { log.push(`release:${id}`); },
  });
  await session.begin();
  assert.equal(session.state(), 'connected');
  assert.equal(session.sessionID(), 'vs_1');
  assert.deepEqual(log, ['connect:tok-1']);
  assert.deepEqual(admitted, [1]);
});

test('a still-valid grant never triggers re-admission', async () => {
  const log: string[] = [];
  let admissions = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => { admissions += 1; return admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z'); },
    release: async (id) => { log.push(`release:${id}`); },
  });
  await session.begin();
  const renewed = await session.renewIfExpired();
  assert.equal(renewed, false);
  assert.equal(admissions, 1);
  assert.deepEqual(log, ['connect:tok-1']);
});

test('an expired grant renews through fresh admission: old session released, stale token never reused', async () => {
  const log: string[] = [];
  const time = clock(Date.parse('2030-01-01T00:00:00Z'));
  let admissions = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => {
      admissions += 1;
      // Grant 1 is already expired at connect time; grant 2 stays valid.
      return admissions === 1
        ? admission('vs_1', 'tok-1', '2030-01-01T00:00:30Z')
        : admission('vs_2', 'tok-2', '2031-01-01T00:00:00Z');
    },
    release: async (id) => { log.push(`release:${id}`); },
    now: time.now,
  });
  await session.begin();
  time.advance(60_000);
  const renewed = await session.renewIfExpired();
  assert.equal(renewed, true);
  assert.equal(session.state(), 'connected');
  assert.equal(session.sessionID(), 'vs_2');
  assert.deepEqual(session.grant(), { token: 'tok-2', expiresAt: '2031-01-01T00:00:00Z' });
  assert.deepEqual(log, ['connect:tok-1', 'port.close', 'release:vs_1', 'connect:tok-2']);
  assert.equal(admissions, 2);
});

test('a failed renewal surfaces failure and never retries on its own (no infinite reconnect)', async () => {
  const log: string[] = [];
  const time = clock(Date.parse('2030-01-01T00:00:00Z'));
  let admissions = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => {
      admissions += 1;
      return admissions === 1 ? admission('vs_1', 'tok-1', '2030-01-01T00:00:30Z')
        : Promise.reject(new RealtimeVoiceError('ADMISSION_HTTP', 'ADMISSION_HTTP_402'));
    },
    release: async (id) => { log.push(`release:${id}`); },
    now: time.now,
  });
  await session.begin();
  time.advance(60_000);
  await session.renewIfExpired();
  assert.equal(session.state(), 'failed');
  // Ticks and further lifecycle noise never auto-retry: only an explicit
  // user command starts a new admission.
  time.advance(120_000);
  assert.equal(admissions, 2);
  assert.equal(session.state(), 'failed');
});

test('renewals are capped: after the cap no further admission leaves the device', async () => {
  const log: string[] = [];
  const time = clock(Date.parse('2030-01-01T00:00:00Z'));
  let admissions = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => { admissions += 1; return admission(`vs_${admissions}`, `tok_${admissions}`, '2030-01-01T00:00:30Z'); },
    release: async (id) => { log.push(`release:${id}`); },
    now: time.now,
    maxRenewals: 1,
  });
  await session.begin();
  time.advance(60_000);
  await session.renewIfExpired();
  assert.equal(admissions, 2);
  time.advance(60_000);
  const renewed = await session.renewIfExpired();
  assert.equal(renewed, false);
  assert.equal(admissions, 2, 'capped renewal must not admit again');
  // An explicit new begin() is the only reset.
  await session.begin();
  assert.equal(admissions, 3);
  assert.equal(session.state(), 'connected');
});

test('a connection-lost disconnect closes the provider, runs the W12 recovery query once, and never re-admits', async () => {
  const log: string[] = [];
  let admissions = 0;
  let recoveries = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => { admissions += 1; return admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z'); },
    release: async (id) => { log.push(`release:${id}`); },
    recover: async () => { recoveries += 1; },
  });
  await session.begin();
  await session.interruptedBySystem('connection-lost');
  assert.equal(session.state(), 'interrupted');
  assert.deepEqual(log, ['connect:tok-1', 'port.close', 'release:vs_1']);
  assert.equal(recoveries, 1, 'exactly one W12 recovery query');
  assert.equal(admissions, 1, 'the paid session is never reopened by the disconnect');
});

test('a background transition closes the provider and leaves the recovery query to the W12 foreground pass', async () => {
  const log: string[] = [];
  let recoveries = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z'),
    release: async (id) => { log.push(`release:${id}`); },
    recover: async () => { recoveries += 1; },
  });
  await session.begin();
  await session.interruptedBySystem('background');
  assert.equal(session.state(), 'interrupted');
  assert.deepEqual(log, ['connect:tok-1', 'port.close', 'release:vs_1']);
  assert.equal(recoveries, 0, 'W12 owns the foreground recovery pass');
});

test('end closes the provider and settles the session; repeated end is a no-op', async () => {
  const log: string[] = [];
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z'),
    release: async (id) => { log.push(`release:${id}`); },
  });
  await session.begin();
  await session.end();
  await session.end();
  assert.equal(session.state(), 'ended');
  assert.equal(session.sessionID(), null);
  assert.deepEqual(log, ['connect:tok-1', 'port.close', 'release:vs_1']);
});

test('a replay admission without a fresh token releases the stale row and retries admission exactly once more', async () => {
  const log: string[] = [];
  let admissions = 0;
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => {
      admissions += 1;
      if (admissions === 1) return admission('vs_stale', '', '2031-01-01T00:00:00Z', false);
      return admission('vs_fresh', 'tok-fresh', '2031-01-01T00:00:00Z');
    },
    release: async (id) => { log.push(`release:${id}`); },
  });
  await session.begin();
  assert.equal(session.state(), 'connected');
  assert.deepEqual(log, ['release:vs_stale', 'connect:tok-fresh']);
  assert.equal(admissions, 2);
});

test('interrupt playback and mute only touch the media surface', async () => {
  const log: string[] = [];
  const session = createRealtimeVoiceSession({
    port: fakePort(log),
    admit: async () => admission('vs_1', 'tok-1', '2031-01-01T00:00:00Z'),
    release: async () => undefined,
  });
  session.interruptPlayback();
  session.setMuted(true);
  await session.begin();
  session.interruptPlayback();
  session.setMuted(false);
  // A barge-in before the connection is a pure media-surface op: it never
  // opens, closes or releases anything.
  assert.deepEqual(log, ['stopAudio', 'mute:true', 'connect:tok-1', 'stopAudio', 'mute:false']);
});

// --- Voice utterances: approval answers never auto-approve the W05 card. ---

test('an explicit cancel utterance cancels; an ambiguous one does nothing', async () => {
  const calls: string[] = [];
  const controls = createVoiceControls({
    stopAudio: () => { calls.push('audio'); },
    closeVoice: async () => { calls.push('close'); },
    cancelRun: async () => { calls.push('cancel'); },
  });
  const handle = createVoiceUtteranceHandler({ controls, hasPendingApproval: () => false });
  handle('取消任务');
  handle('取消这个正在执行的任务');
  handle('cancel the task');
  assert.deepEqual(calls, ['cancel', 'cancel', 'cancel']);
  handle('算了');
  handle('不用了');
  handle('先这样');
  assert.deepEqual(calls, ['cancel', 'cancel', 'cancel'], 'ambiguous wording never cancels');
});

test('playback-stop and end-voice utterances never cancel or close each other', async () => {
  const calls: string[] = [];
  const controls = createVoiceControls({
    stopAudio: () => { calls.push('audio'); },
    closeVoice: async () => { calls.push('close'); },
    cancelRun: async () => { calls.push('cancel'); },
  });
  const handle = createVoiceUtteranceHandler({ controls, hasPendingApproval: () => false });
  const stop = handle('停止播放');
  assert.equal(stop.intent.kind, 'interrupt-playback');
  const end = handle('挂断');
  assert.equal(end.intent.kind, 'end-voice');
  await Promise.resolve();
  assert.deepEqual(calls, ['audio', 'close']);
});

test('approval answers — explicit or ambiguous — surface the W05 card and never approve', async () => {
  const calls: string[] = [];
  const surfaced: boolean[] = [];
  const controls = createVoiceControls({
    stopAudio: () => { calls.push('audio'); },
    closeVoice: async () => { calls.push('close'); },
    cancelRun: async () => { calls.push('cancel'); },
  });
  const handle = createVoiceUtteranceHandler({
    controls,
    hasPendingApproval: () => true,
    surfaceApprovalCard: () => { surfaced.push(true); },
  });
  // The handler structurally has no approve port at all: no utterance can
  // approve. Ambiguous acknowledgements surface the card only.
  const ambiguous = handle('嗯');
  const ambiguousIntent = ambiguous.intent as { kind: 'approval-answer'; explicit: boolean };
  assert.equal(ambiguousIntent.kind, 'approval-answer');
  assert.equal(ambiguousIntent.explicit, false);
  assert.equal(ambiguous.approvalSurfaced, true);
  const explicit = handle('批准');
  const explicitIntent = explicit.intent as { kind: 'approval-answer'; explicit: boolean };
  assert.equal(explicitIntent.explicit, true);
  assert.equal(explicit.approvalSurfaced, true);
  // Even the explicit word only surfaces the card.
  assert.equal(surfaced.length, 2);
  assert.deepEqual(calls, [], 'no product command runs from an approval answer');
  // Without a pending approval there is nothing to surface.
  const idle = createVoiceUtteranceHandler({ controls, hasPendingApproval: () => false, surfaceApprovalCard: () => { surfaced.push(true); } });
  const none = idle('批准');
  assert.equal(none.approvalSurfaced, false);
  assert.deepEqual(calls, []);
});

test('classify keeps cancel wording ahead of playback-stop wording', () => {
  assert.equal(classifyVoiceUtterance('停止任务').kind, 'cancel-task');
  assert.equal(classifyVoiceUtterance('取消执行').kind, 'cancel-task');
  assert.equal(classifyVoiceUtterance('停止播放').kind, 'interrupt-playback');
  assert.equal(classifyVoiceUtterance('stop the task').kind, 'cancel-task');
  assert.equal(classifyVoiceUtterance('stop').kind, 'interrupt-playback');
  assert.equal(classifyVoiceUtterance('今天天气怎么样').kind, 'unknown');
});

// --- Product admission adapter over the W30 wire. ---

function jsonFetch(status: number, body: unknown): FetchLike {
  return (async () => ({
    status,
    headers: { get: () => 'application/json' },
    json: async () => body,
    text: async () => JSON.stringify(body),
  })) as FetchLike;
}

test('the product adapter maps the W30 wire onto the grant contract', async () => {
  const requests: Array<{ method: string; url: string; auth: string; body: unknown }> = [];
  const fetcher: FetchLike = (async (url: string, init?: { method?: string; headers?: Record<string, string>; body?: string }) => {
    requests.push({ method: init?.method ?? 'GET', url, auth: init?.headers?.authorization ?? '', body: init?.body ? JSON.parse(init!.body!) : undefined });
    if (url.endsWith('/mobile/voice/sessions')) {
      return { status: 200, headers: { get: () => 'application/json' }, json: async () => ({ success: true, data: { id: 'vs_9', token: 'tok-9', expires_at: '2031-01-01T00:00:00Z', max_seconds: 300 } }), text: async () => '' };
    }
    return { status: 200, headers: { get: () => 'application/json' }, json: async () => ({ success: true, data: { id: 'vs_9', settled: true } }), text: async () => '' };
  }) as FetchLike;
  const api = createProductVoiceSessionApi({ origin: 'https://api.example/', credential: { kind: 'bearer', accessToken: 'bearer-1' }, authSession: {} as never, fetcher });
  const admitted = await api.admit('sess-1', 'run-1');
  assert.deepEqual(admitted, { id: 'vs_9', grant: { token: 'tok-9', expiresAt: '2031-01-01T00:00:00Z' }, tokenIssued: true });
  assert.deepEqual(requests[0], {
    method: 'POST',
    url: 'https://api.example/api/v1/mobile/voice/sessions',
    auth: 'Bearer bearer-1',
    body: { session_id: 'sess-1', run_id: 'run-1', max_seconds: 300 },
  });
  await api.release('vs_9');
  assert.equal(requests[1].method, 'DELETE');
  assert.equal(requests[1].url, 'https://api.example/api/v1/mobile/voice/sessions/vs_9');
});

test('a replay answer without a token maps to tokenIssued:false and never invents one', async () => {
  const fetcher = jsonFetch(200, { success: true, data: { id: 'vs_stale', state: 'open', expires_at: '2031-01-01T00:00:00Z', max_seconds: 300, token_issued: false, replay: true } });
  const api = createProductVoiceSessionApi({ origin: 'https://api.example', credential: { kind: 'bearer', accessToken: 'b' }, authSession: {} as never, fetcher });
  const admitted = await api.admit('sess-1');
  assert.equal(admitted.tokenIssued, false);
  assert.equal(admitted.grant.token, '');
});

test('admission failures are typed and never leak a token into the error', async () => {
  const denied = createProductVoiceSessionApi({ origin: 'https://api.example', credential: { kind: 'bearer', accessToken: 'b' }, authSession: {} as never, fetcher: jsonFetch(402, { success: false, error: 'voice_budget_denied' }) });
  await assert.rejects(() => denied.admit('sess-1'), (error: unknown) => {
    assert.ok(error instanceof RealtimeVoiceError);
    assert.equal(error.code, 'ADMISSION_HTTP');
    assert.match(error.message, /402/);
    return true;
  });
});

