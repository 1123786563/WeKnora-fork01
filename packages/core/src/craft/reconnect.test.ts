// C03 snapshot reconnection controller tests.
//
// Pins the C03 contract on top of W04's controller:
//   * event gaps and expired cursors reload the AUTHORITATIVE snapshot
//     (replayAction drop/apply/reload) instead of failing permanently;
//   * reconnects back off 1/2/4/8/15s (schedule itself is the domain test;
//     here it is INJECTED so cycles run instantly and attempts are counted);
//   * snapshot replacement is atomic per generation: event-won projections
//     survive only when they are AHEAD of the snapshot watermark;
//   * dispose / scope switches kill pending timers and requests at once, and
//     a late fetch resolve can never write state across a scope change;
//   * transport failures reconnect the subscription only — the user prompt is
//     never re-sent and no cancel is ever issued.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createCraftWorkbenchController, type CraftEventFrame, type CraftSubscribeInput, type CraftSyncPhase } from './controller.ts';
import { createScopeController } from '@weknora/domain/scope';
import type { CraftApi } from '@weknora/api-client';
import type { CraftRunView, CraftWorkspaceView } from '@weknora/contracts';

const runView = (runId: string, status: string, seq = 0): CraftRunView => ({
  run_id: runId, session_id: 's1', status: status as CraftRunView['status'],
  wait_reason: '', revision: 1, epoch: 1, seq, pending_id: null,
});

const view = (options: { runId?: string; status?: string | null; lastSeq?: number; versionId?: string | null } = {}): CraftWorkspaceView => {
  const runId = options.runId ?? 'run-1';
  const status = options.status === undefined ? 'running' : options.status;
  return {
    session_id: 's1', workspace_id: 'w1', kind: 'web', title: 't', engine_type: 'trpc',
    workspace: { id: 'w1', session_id: 's1', user_id: 'u1', sandbox_id: 'sbx', generation: '1', runtime_digest: 'd', revision: 1 },
    active_run_id: status === null ? null : runId, pending_id: null, last_seq: options.lastSeq ?? 0,
    active_run: status === null ? null : runView(runId, status, options.lastSeq ?? 0),
    current_version: options.versionId === undefined
      ? { id: 'v0', workspace_id: 'w1', run_id: 'run-0', kind: 'web', files: [], checks: [] }
      : options.versionId === null ? null : { id: options.versionId, workspace_id: 'w1', run_id: 'run-0', kind: 'web', files: [], checks: [] },
  };
};

interface RecordedSubscribe {
  input: CraftSubscribeInput;
  index: number;
}

/** SSE transport fake: scripted behaviours run per subscribe call. */
function fakeEventTransport(behaviors: ((call: RecordedSubscribe) => Promise<void>)[]) {
  const calls: RecordedSubscribe[] = [];
  return {
    calls,
    transport: {
      async subscribe(input: CraftSubscribeInput): Promise<void> {
        const call = { input, index: calls.length };
        calls.push(call);
        const behavior = behaviors[Math.min(call.index, behaviors.length - 1)]!;
        return behavior(call);
      },
    },
  };
}

const pending = (): Promise<void> => new Promise<void>(() => {});
const settle = async (times = 8): Promise<void> => {
  for (let i = 0; i < times; i += 1) await new Promise((resolve) => setTimeout(resolve, 0));
};

const frame = (event: string, data: unknown): CraftEventFrame => ({ event, data: JSON.stringify(data) });
const wire = (seq: number, payload: unknown, attemptId: string | null = null): CraftEventFrame =>
  frame(String(seq), { seq, attempt_id: attemptId, type: 'craft', payload });
const craftPayload = (kind: string, data: Record<string, unknown> = {}) => ({ workspace_id: 'w1', delegation_id: 'd1', tool_call_id: 't1', kind, data });

/** View-scripted API fake; each get consumes one scripted view (last repeats). */
function fakeApi(views: CraftWorkspaceView[], submitResults: (Error | CraftRunView)[] = [runView('run-1', 'queued')]) {
  const getCalls: string[] = [];
  const submitCalls: { sessionId: string; body: { request_id: string; prompt: string } }[] = [];
  const api = {
    async get(sessionId: string): Promise<CraftWorkspaceView> {
      getCalls.push(sessionId);
      return views[Math.min(getCalls.length - 1, views.length - 1)]!;
    },
    async submit(sessionId: string, input: { request_id: string; prompt: string }): Promise<CraftRunView> {
      submitCalls.push({ sessionId, body: input });
      const result = submitResults[Math.min(submitCalls.length - 1, submitResults.length - 1)]!;
      if (result instanceof Error) throw result;
      return result;
    },
  } as unknown as CraftApi;
  return { api, getCalls, submitCalls };
}

/** Manually resolved API fake for scope-switch race tests. */
function deferredApi() {
  let getCalls = 0;
  let resolveGet: ((value: CraftWorkspaceView) => void) | null = null;
  const api = {
    async get(): Promise<CraftWorkspaceView> {
      getCalls += 1;
      return new Promise<CraftWorkspaceView>((resolve) => {
        resolveGet = resolve;
      });
    },
    async submit(): Promise<CraftRunView> {
      throw new Error('submit must never be called here');
    },
  } as unknown as CraftApi;
  return {
    api,
    get count(): number {
      return getCalls;
    },
    resolve(value: CraftWorkspaceView): void {
      const resolve = resolveGet;
      resolveGet = null;
      resolve?.(value);
    },
  };
}

/** Zero-delay recording backoff: reconnect cycles run instantly, attempts counted. */
function fastBackoff() {
  const attempts: number[] = [];
  const sleeps: number[] = [];
  return {
    attempts,
    sleeps,
    backoff: {
      delayMs(attempt: number): number {
        attempts.push(attempt);
        return 0;
      },
      async sleep(ms: number): Promise<void> {
        sleeps.push(ms);
      },
    },
  };
}

/** Zero-delay backoff whose sleeps resolve only on flush (timer-control tests). */
function manualBackoff() {
  const attempts: number[] = [];
  let wake: (() => void) | null = null;
  return {
    attempts,
    backoff: {
      delayMs(attempt: number): number {
        attempts.push(attempt);
        return 0;
      },
      sleep(): Promise<void> {
        return new Promise<void>((resolve) => {
          wake = resolve;
        });
      },
    },
    flush(): void {
      wake?.();
      wake = null;
    },
  };
}

function phaseRecorder() {
  const phases: CraftSyncPhase[] = [];
  const onSync = (phase: CraftSyncPhase): void => {
    phases.push(phase);
  };
  return { phases, onSync };
}

test('seq 1/2/2/4 triggers exactly one authoritative reload', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 0 }), view({ status: 'running', lastSeq: 4 })]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(1, craftPayload('delegation.text', { text: 'a' })));
      input.onEvent(wire(2, craftPayload('delegation.text', { text: 'b' })));
      input.onEvent(wire(2, craftPayload('delegation.text', { text: 'b' }))); // duplicate replay drops
      input.onEvent(wire(4, craftPayload('delegation.finished'))); // real gap: 3 is missing
      return pending();
    },
    () => pending(),
  ]);
  const fast = fastBackoff();
  const recorder = phaseRecorder();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fast.backoff, onSync: recorder.onSync });
  await controller.load('s1');
  assert.equal(controller.state()?.seq, 2);
  await settle();
  // One reload only: the gap consumed exactly one snapshot re-read, and the
  // resubscription after the fresh watermark (4) never re-triggers.
  assert.equal(fake.getCalls.length, 2);
  assert.equal(sse.calls.length, 2);
  assert.equal(sse.calls[1]?.input.after, 4);
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  assert.equal(controller.state()?.seq, 4);
  assert.equal(controller.state()?.mainStatus, 'running');
  assert.ok(recorder.phases.includes('syncing'));
  assert.ok(recorder.phases.includes('synced'));
  controller.dispose();
});

test('snapshot seq 4 followed by event 5 applies once, no duplicate reload', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 4 })]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(5, craftPayload('delegation.text', { text: 'next' })));
      input.onEvent(wire(5, craftPayload('delegation.text', { text: 'next' })));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fastBackoff().backoff });
  await controller.load('s1');
  assert.equal(sse.calls[0]?.input.after, 4);
  assert.equal(controller.state()?.seq, 5);
  assert.equal(controller.lastError(), null);
  await settle();
  assert.equal(fake.getCalls.length, 1);
  assert.equal(sse.calls.length, 1);
  controller.dispose();
});

test('scope switch during load: the late fetch resolve never writes', async () => {
  const deferred = deferredApi();
  const sse = fakeEventTransport([() => pending()]);
  const scope = createScopeController();
  const controller = createCraftWorkbenchController({ api: deferred.api, scope, events: sse.transport });
  const loading = controller.load('s1');
  await settle(2);
  scope.switchScope('https://other', 'u2', null);
  assert.equal(controller.state(), null);
  deferred.resolve(view({ status: 'running', lastSeq: 9 }));
  await loading;
  await settle();
  assert.equal(controller.state(), null);
  assert.equal(sse.calls.length, 0);
  assert.equal(deferred.count, 1);
  controller.dispose();
});

test('scope switch during a reconnect cycle: the late snapshot resolve never writes', async () => {
  const deferred = deferredApi();
  const sse = fakeEventTransport([
    async () => {
      throw new Error('network down');
    },
    () => pending(),
  ]);
  const fast = manualBackoff();
  const scope = createScopeController();
  const controller = createCraftWorkbenchController({ api: deferred.api, scope, events: sse.transport, backoff: fast.backoff });
  const loading = controller.load('s1');
  await settle(2);
  deferred.resolve(view({ status: 'running', lastSeq: 0 })); // initial load snapshot
  await loading;
  // The stream died: one reconnect is scheduled and its sleep is pending...
  await settle(2);
  assert.equal(fast.attempts.length, 1);
  fast.flush(); // backoff elapses -> the cycle's snapshot fetch goes out
  await settle(2);
  scope.switchScope('https://other', 'u2', null);
  deferred.resolve(view({ status: 'running', lastSeq: 7 }));
  await settle();
  assert.equal(controller.state(), null);
  assert.equal(sse.calls.length, 1);
  controller.dispose();
});

test('backoff counts consecutive failures and the prompt is never re-sent', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 0 })]);
  const sse = fakeEventTransport([
    async () => {
      throw new Error('network down 1');
    },
    async () => {
      throw new Error('network down 2');
    },
    async () => {
      throw new Error('network down 3');
    },
    () => pending(),
  ]);
  const fast = fastBackoff();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fast.backoff });
  await controller.load('s1');
  await settle();
  assert.deepEqual(fast.attempts, [0, 1, 2]);
  assert.ok(fast.sleeps.length >= 3);
  assert.equal(sse.calls.length, 4); // initial + three reconnects
  assert.equal(fake.submitCalls.length, 0); // never cancels, never re-sends
  assert.equal(controller.state()?.mainStatus, 'running');
  controller.dispose();
});

test('dispose during backoff terminates the pending reconnect', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 0 }), view({ status: 'running', lastSeq: 5 })]);
  const sse = fakeEventTransport([
    async () => {
      throw new Error('network down');
    },
    () => pending(),
  ]);
  const fast = manualBackoff();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fast.backoff });
  await controller.load('s1');
  await settle(2);
  assert.equal(fast.attempts.length, 1); // a reconnect is scheduled, sleeping
  controller.dispose();
  fast.flush(); // the sleep resolves AFTER dispose: nothing may happen
  await settle();
  assert.equal(fake.getCalls.length, 1);
  assert.equal(sse.calls.length, 1);
});

test('cursor_expired reloads the snapshot and resumes after the fresh last_seq', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 3 }), view({ status: 'running', lastSeq: 7 })]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(frame('error', { code: 'cursor_expired', message: 'trim horizon', seq: 3 }));
      return pending();
    },
    () => pending(),
  ]);
  const fast = fastBackoff();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fast.backoff });
  await controller.load('s1');
  assert.equal(controller.lastError()?.code, 'CURSOR_EXPIRED');
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  await settle();
  assert.equal(fake.getCalls.length, 2);
  assert.equal(sse.calls.length, 2);
  assert.equal(sse.calls[1]?.input.after, 7);
  controller.dispose();
});

test('reconnect() reloads and resubscribes, announcing syncing then synced', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 3 }), view({ status: 'running', lastSeq: 6 })]);
  const sse = fakeEventTransport([() => pending(), () => pending()]);
  const recorder = phaseRecorder();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, onSync: recorder.onSync });
  await controller.load('s1');
  assert.equal(sse.calls[0]?.input.after, 3);
  await controller.reconnect();
  assert.equal(fake.getCalls.length, 2);
  assert.equal(sse.calls.length, 2);
  assert.equal(sse.calls[1]?.input.after, 6);
  assert.equal(controller.state()?.seq, 6);
  const tail = recorder.phases.slice(-2);
  assert.deepEqual(tail, ['syncing', 'synced']);
  controller.dispose();
});

test('a terminal run projection ends the sync loop without reloads', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 0 })]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(frame('run', { run_id: 'run-1', session_id: 's1', status: 'succeeded', wait_reason: '', revision: 2, epoch: 1, seq: 0, capabilities: {} }));
    },
  ]);
  const recorder = phaseRecorder();
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fastBackoff().backoff, onSync: recorder.onSync });
  await controller.load('s1');
  await settle();
  assert.equal(controller.state()?.mainStatus, 'succeeded');
  assert.equal(fake.getCalls.length, 1); // no reload after the terminal frame
  assert.equal(sse.calls.length, 1);
  assert.ok(!recorder.phases.includes('syncing')); // completion is not "reconnecting"
  controller.dispose();
});

test('non-craft main-run events (attempt_replaced included) advance the seq without a false gap', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 0 })]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(1, craftPayload('delegation.text', { text: 'attempt-1 partial' })));
      input.onEvent(frame('2', { seq: 2, attempt_id: 'a1', type: 'attempt_replaced', payload: { previous_attempt_id: 'a1' } }));
      input.onEvent(wire(3, craftPayload('delegation.text', { text: 'attempt-2' })));
      input.onEvent(frame('4', { seq: 4, attempt_id: null, type: 'chat', payload: { text: 'foreign' } }));
      input.onEvent(wire(5, craftPayload('delegation.finished')));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fastBackoff().backoff });
  await controller.load('s1');
  const state = controller.state();
  assert.equal(state?.seq, 5); // every main-run event counted, no SEQ_GAP
  assert.equal(controller.lastError(), null);
  await settle();
  assert.equal(fake.getCalls.length, 1);
  controller.dispose();
});

test('snapshot replacement is atomic per generation: event-won fields survive only ahead of the watermark', async () => {
  const fake = fakeApi([
    view({ status: 'running', lastSeq: 0, versionId: 'v0' }),
    view({ status: 'running', lastSeq: 0, versionId: 'v0' }), // stale snapshot (lagging watermark)
    view({ status: 'running', lastSeq: 5, versionId: 'v3' }), // newer snapshot: full replace
    view({ runId: 'run-2', status: 'running', lastSeq: 2, versionId: null }), // different run: full replace
  ]);
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(1, craftPayload('artifact.published', { version_id: 'v9' })));
    },
    () => pending(),
    () => pending(),
    () => pending(),
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fastBackoff().backoff });
  await controller.load('s1');
  assert.equal(controller.state()?.versionId, 'v9');
  await settle();
  // Stream ended → reload: the stale snapshot (last_seq 0) cannot drag the
  // already-applied projection (seq 1, v9) backwards.
  assert.equal(fake.getCalls.length, 2);
  assert.equal(controller.state()?.seq, 1);
  assert.equal(controller.state()?.versionId, 'v9');
  // A NEWER snapshot atomically replaces everything.
  await controller.reconnect();
  assert.equal(controller.state()?.seq, 5);
  assert.equal(controller.state()?.versionId, 'v3');
  assert.equal(controller.state()?.delegationStatus, 'idle');
  // A different run replaces too — no cross-run max() carryover.
  await controller.reconnect();
  assert.equal(controller.state()?.runId, 'run-2');
  assert.equal(controller.state()?.seq, 2);
  assert.equal(controller.state()?.versionId, null);
  controller.dispose();
});

test('load() reuses the same session and run across refreshes and never creates a run', async () => {
  const fake = fakeApi([view({ status: 'running', lastSeq: 5 }), view({ status: 'running', lastSeq: 5 })]);
  const sse = fakeEventTransport([() => pending(), () => pending()]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: fastBackoff().backoff });
  await controller.load('s1');
  assert.equal(controller.state()?.runId, 'run-1');
  await controller.load('s1'); // refresh path
  assert.equal(fake.getCalls.length, 2);
  assert.equal(sse.calls.length, 2);
  assert.equal(sse.calls[1]?.input.after, 5);
  assert.equal(sse.calls[1]?.input.runId, 'run-1');
  assert.equal(controller.state()?.runId, 'run-1');
  assert.equal(fake.submitCalls.length, 0);
  controller.dispose();
});
