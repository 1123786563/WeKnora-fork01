import test from 'node:test';
import assert from 'node:assert/strict';
import { createCraftWorkbenchController, type CraftEventFrame, type CraftSubscribeInput } from './controller.ts';
import { createScopeController } from '@weknora/domain/scope';
import type { CraftApi } from '@weknora/api-client';
import type { CraftRunView, CraftWorkspaceView } from '@weknora/contracts';

const runView = (status: string, seq = 0): CraftRunView => ({
  run_id: 'run-1', session_id: 's1', status: status as CraftRunView['status'],
  wait_reason: '', revision: 1, epoch: 1, seq, pending_id: null,
});

const workspaceView = (status: string | null, lastSeq: number): CraftWorkspaceView => ({
  session_id: 's1', workspace_id: 'w1', kind: 'web', title: 't', engine_type: 'trpc',
  workspace: { id: 'w1', session_id: 's1', user_id: 'u1', sandbox_id: 'sbx', generation: '1', runtime_digest: 'd', revision: 1 },
  active_run_id: status === null ? null : 'run-1', pending_id: null, last_seq: lastSeq,
  active_run: status === null ? null : runView(status, lastSeq),
  current_version: { id: 'v0', workspace_id: 'w1', run_id: 'run-0', kind: 'web', files: [], checks: [] },
});

interface RecordedSubscribe {
  input: CraftSubscribeInput;
  index: number;
}

/** SSE transport fake: each scripted behaviour runs when subscribe is called. */
function fakeEventTransport(behaviors: ((call: RecordedSubscribe) => Promise<void>)[]) {
  const calls: RecordedSubscribe[] = [];
  return {
    calls,
    transport: {
      async subscribe(input: CraftSubscribeInput): Promise<void> {
        const call = { input, index: calls.length };
        calls.push(call);
        const behavior = behaviors[Math.min(call.index, behaviors.length - 1)];
        return behavior(call);
      },
    },
  };
}

const pending = (): Promise<void> => new Promise<void>(() => {});
const settle = async (times = 6): Promise<void> => {
  for (let i = 0; i < times; i += 1) await new Promise((resolve) => setTimeout(resolve, 0));
};

const frame = (event: string, data: unknown): CraftEventFrame => ({ event, data: JSON.stringify(data) });
const wire = (seq: number, payload: unknown): CraftEventFrame => frame(String(seq), { seq, type: 'craft', payload });
const craftPayload = (kind: string, data: Record<string, unknown> = {}) => ({ workspace_id: 'w1', delegation_id: 'd1', tool_call_id: 't1', kind, data });

function fakeApi(options: { views: CraftWorkspaceView[]; submitResults?: (Error | CraftRunView)[] }) {
  const getCalls: string[] = [];
  const submitCalls: { sessionId: string; body: { request_id: string; prompt: string } }[] = [];
  const api = {
    async get(sessionId: string): Promise<CraftWorkspaceView> {
      getCalls.push(sessionId);
      return options.views[Math.min(getCalls.length - 1, options.views.length - 1)]!;
    },
    async submit(sessionId: string, input: { request_id: string; prompt: string }): Promise<CraftRunView> {
      submitCalls.push({ sessionId, body: input });
      const results = options.submitResults ?? [runView('queued')];
      const result = results[Math.min(submitCalls.length - 1, results.length - 1)]!;
      if (result instanceof Error) throw result;
      return result;
    },
  } as unknown as CraftApi;
  return { api, getCalls, submitCalls };
}

test('load snapshots first, then subscribes after last_seq', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 3)] });
  const sse = fakeEventTransport([() => pending()]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  assert.equal(fake.getCalls.length, 1);
  assert.equal(sse.calls.length, 1);
  assert.equal(sse.calls[0]?.input.after, 3);
  assert.equal(sse.calls[0]?.input.runId, 'run-1');
  assert.equal(controller.state()?.seq, 3);
  assert.equal(controller.state()?.mainStatus, 'running');
  controller.dispose();
});

test('duplicate events drop; seq gaps abort with SEQ_GAP for the C03 reload entry', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 3)] });
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(4, craftPayload('delegation.text', { text: 'a' })));
      input.onEvent(wire(4, craftPayload('delegation.text', { text: 'a' })));
      input.onEvent(wire(9, craftPayload('delegation.finished')));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  assert.equal(controller.state()?.seq, 4);
  const error = controller.lastError();
  assert.equal(error?.code, 'SEQ_GAP');
  assert.equal(error?.expectedSeq, 5);
  assert.equal(error?.receivedSeq, 9);
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  await settle();
  assert.equal(sse.calls.length, 1);
  controller.dispose();
});

test('child finished never finishes the main message; the run projection does', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 0)] });
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(1, craftPayload('delegation.finished')));
      assert.equal(controller.state()?.mainStatus, 'running');
      assert.equal(controller.state()?.delegationStatus, 'finished');
      input.onEvent(frame('run', { run_id: 'run-1', session_id: 's1', status: 'succeeded', wait_reason: '', revision: 2, epoch: 1, seq: 1, capabilities: {} }));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  assert.equal(controller.state()?.mainStatus, 'succeeded');
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  await settle();
  assert.equal(sse.calls.length, 1);
  assert.equal(fake.submitCalls.length, 0);
  controller.dispose();
});

test('cursor_expired surfaces as CURSOR_EXPIRED and stops the subscription', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 3)] });
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(frame('error', { code: 'cursor_expired', message: 'trim horizon', seq: 3 }));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  assert.equal(controller.lastError()?.code, 'CURSOR_EXPIRED');
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  await settle();
  assert.equal(sse.calls.length, 1);
  controller.dispose();
});

test('network failures reconnect the subscription; submit happens exactly once with one request id', async () => {
  const fake = fakeApi({ views: [workspaceView(null, 0), workspaceView('running', 5)] });
  const sse = fakeEventTransport([
    async ({ input }) => {
      input.onEvent(wire(1, craftPayload('delegation.started')));
      // clean stream end (server closed after its terminal check)
    },
    async () => {
      throw new Error('network down');
    },
    ({ input }) => {
      input.onEvent(wire(6, craftPayload('delegation.text', { text: 'x' })));
      return pending();
    },
  ]);
  // C03 moved reconnects onto the backoff schedule; this W04 test pins the
  // immediate-cycle behaviour, so it injects the zero-delay schedule.
  const zeroBackoff = { delayMs: (): number => 0, sleep: async (): Promise<void> => {} };
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport, backoff: zeroBackoff });
  await controller.load('s1');
  assert.equal(sse.calls.length, 0);
  await controller.submit('画一个落地页');
  assert.equal(fake.submitCalls.length, 1);
  const requestId = fake.submitCalls[0]?.body.request_id;
  assert.match(requestId ?? '', /.+/);
  assert.equal(sse.calls[0]?.input.after, 0);
  await settle();
  // stream end → snapshot refresh (last_seq 5) → reconnect after 5;
  // network error → refresh → reconnect after 5. No submit ever repeats.
  assert.equal(sse.calls.length, 3);
  assert.equal(sse.calls[1]?.input.after, 5);
  assert.equal(sse.calls[2]?.input.after, 5);
  assert.equal(controller.state()?.seq, 6);
  // A refresh (load again) resubscribes but NEVER submits again.
  await controller.load('s1');
  await settle();
  assert.ok(sse.calls.length >= 4);
  assert.equal(sse.calls[3]?.input.after, 6);
  assert.equal(fake.submitCalls.length, 1);
  assert.equal(fake.submitCalls[0]?.body.request_id, requestId);
  controller.dispose();
});

test('a failed submit retries with the SAME request id', async () => {
  const fake = fakeApi({ views: [workspaceView(null, 0)], submitResults: [new Error('network reset'), runView('queued', 0)] });
  const sse = fakeEventTransport([() => pending()]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  await assert.rejects(() => controller.submit('hi'), /network reset/);
  await controller.submit('hi');
  assert.equal(fake.submitCalls.length, 2);
  assert.equal(fake.submitCalls[0]?.body.request_id, fake.submitCalls[1]?.body.request_id);
  assert.equal(controller.state()?.runId, 'run-1');
  assert.equal(controller.state()?.mainStatus, 'queued');
  assert.equal(sse.calls[0]?.input.runId, 'run-1');
  controller.dispose();
});

test('switching scope generation destroys the old subscription and clears the selection', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 3)] });
  let deliver: ((frame: CraftEventFrame) => void) | null = null;
  const sse = fakeEventTransport([
    ({ input }) => {
      deliver = (frameValue) => input.onEvent(frameValue);
      return pending();
    },
    () => pending(),
  ]);
  const scope = createScopeController();
  const controller = createCraftWorkbenchController({ api: fake.api, scope, events: sse.transport });
  await controller.load('s1');
  assert.equal(controller.state()?.versionId, 'v0');
  scope.switchScope('https://other', 'u2', null);
  assert.equal(controller.state(), null);
  assert.equal(sse.calls[0]?.input.signal.aborted, true);
  // A late frame from the destroyed stream must not resurrect state.
  deliver?.(wire(4, craftPayload('artifact.published', { version_id: 'v9' })));
  await settle();
  assert.equal(controller.state(), null);
  assert.equal(sse.calls.length, 1);
  controller.dispose();
});

test('artifact.published selects the published version; foreign events only advance the cursor', async () => {
  const fake = fakeApi({ views: [workspaceView('running', 0)] });
  const sse = fakeEventTransport([
    ({ input }) => {
      input.onEvent(wire(1, craftPayload('artifact.published', { version_id: 'v2' })));
      input.onEvent(wire(2, { type: 'chat', text: 'not craft' }));
      input.onEvent(wire(3, craftPayload('workspace.unavailable')));
      return pending();
    },
  ]);
  const controller = createCraftWorkbenchController({ api: fake.api, scope: createScopeController(), events: sse.transport });
  await controller.load('s1');
  const state = controller.state();
  assert.equal(state?.versionId, 'v2');
  assert.equal(state?.seq, 3);
  assert.equal(state?.mainStatus, 'running');
  assert.equal(controller.lastError(), null);
  controller.dispose();
});
