import assert from 'node:assert/strict';
import test from 'node:test';

import { startProcessingTimeline, resolveKnowledgeSpansView } from './processing-timeline.ts';
import { buildKnowledgeTimeline, flattenKnowledgeSpans } from '@weknora/domain/knowledge/processing';

// R469/A3 N009 capture: GET /api/v1/knowledge/{id}/spans for the completed
// document a20e53a5 replies with the backend envelope { success, data } —
// data carries parse_status/current_stage/trace (internal/handler/knowledge.go
// GetKnowledgeSpans). Vue reads res.data; the React api-client returns the
// raw body, so documents consumers must unwrap it the same way.
const R469_COMPLETED_SPANS = {
  success: true,
  data: {
    knowledge_id: 'a20e53a5',
    attempt: 1,
    latest_attempt: 1,
    current_attempt: 1,
    parse_status: 'completed',
    current_stage: '',
    trace: {
      span_id: 'root-1',
      kind: 'root',
      name: 'knowledge_processing',
      status: 'done',
      started_at: '2026-09-17T10:00:00Z',
      finished_at: '2026-09-17T10:02:10.400Z',
      duration_ms: 130400,
      children: [
        { span_id: 's1', kind: 'stage', name: 'docreader', status: 'done', started_at: '2026-09-17T10:00:00Z', finished_at: '2026-09-17T10:00:00.007Z', duration_ms: 7 },
        { span_id: 's2', kind: 'stage', name: 'chunking', status: 'done', duration_ms: 13 },
        { span_id: 's3', kind: 'stage', name: 'embedding', status: 'done', duration_ms: 5 },
        { span_id: 's4', kind: 'stage', name: 'multimodal', status: 'skipped' },
        {
          span_id: 's5', kind: 'stage', name: 'postprocess', status: 'done', duration_ms: 9,
          children: [
            { span_id: 's5-1', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30017, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-2', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30025, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-3', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30031, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
            { span_id: 's5-4', kind: 'span', name: 'postprocess.summary', status: 'failed', duration_ms: 30040, error_code: 'LLM_TIMEOUT', error_message: 'summary timed out' },
          ],
        },
      ],
    },
    last_error: null,
  },
};

test('spans envelope responses unwrap to the Vue spans view (N009 root cause)', () => {
  const view = resolveKnowledgeSpansView(R469_COMPLETED_SPANS);
  assert.equal(view.parse_status, 'completed', 'parse_status lives under data, not on the envelope');
  assert.equal(view.current_stage, '');
  assert.ok(view.trace && Array.isArray(view.trace.children) && view.trace.children.length === 5, 'trace tree is reachable at view.trace');
  // Already-unwrapped payloads pass through untouched.
  const bare = { parse_status: 'processing', trace: { name: 'root', children: [] } };
  assert.equal(resolveKnowledgeSpansView(bare), bare);
});

test('unwrapped completed trace renders real stage states, not the all-pending N009 symptom', () => {
  const view = resolveKnowledgeSpansView(R469_COMPLETED_SPANS);
  const steps = buildKnowledgeTimeline(view);
  // Vue drawer for this document renders 文档解析/分块/向量化 as 已完成 with
  // 7ms/13ms/5ms durations.
  assert.deepEqual(
    steps.filter((step) => step.stage === 'docreader' || step.stage === 'chunking' || step.stage === 'embedding').map((step) => step.stage + ':' + step.state),
    ['docreader:done', 'chunking:done', 'embedding:done'],
  );
  assert.equal(steps.every((step) => step.state === 'pending'), false, 'N009 regression guard: stages must not all render 等待中');
  // The failed postprocess.summary retries surface (Vue counts them as 失败任务×4);
  // the shared domain rolls them up onto the postprocess stage pill.
  assert.equal(steps.find((step) => step.stage === 'postprocess')?.state, 'failed');
  // R472-A1 fixed the recorded divergence: Vue labels the skipped multimodal
  // stage 已跳过, and the domain state model now carries a skipped state
  // (the old model mapped the skipped span to running → 进行中).
  assert.equal(steps.find((step) => step.stage === 'multimodal')?.state, 'skipped');
  // Vue waterfall rows: root + 5 stages + 4 failed summary sub-spans with durations.
  const nodes = flattenKnowledgeSpans(view.trace);
  assert.equal(nodes.filter((row) => row.node.name === 'postprocess.summary' && row.node.status === 'failed').length, 4);
  assert.equal(nodes.find((row) => row.node.name === 'docreader')?.node.duration_ms, 7);
});

function fakeTimer() {
  let handler: (() => void) | undefined;
  let cleared = false;
  return {
    timer: {
      setInterval(next: () => void, _ms: number) { handler = next; return handle; },
      clearInterval(handleToClear: unknown) { if (handleToClear === handle) cleared = true; },
    },
    async tick() { await handler?.(); },
    get cleared() { return cleared; },
  };
}
const handle = Symbol('handle');

async function flushTimelineTick(): Promise<void> {
  await new Promise<void>((resolve) => setImmediate(resolve));
}

test('fetches immediately, then polls spans every 2s while processing', async () => {
  const fake = fakeTimer();
  const fetches: string[] = [];
  const updates: string[][] = [];
  let status = 'processing';
  const subscription = startProcessingTimeline({
    documentId: 'doc-1',
    intervalMs: 2000,
    getSpans: async (id) => {
      fetches.push(id);
      return { parse_status: status, trace: { name: 'root', children: [{ name: 'docreader', status: 'completed' }] } };
    },
    onUpdate: (steps) => updates.push(steps.map((step) => step.stage + ':' + step.state)),
    timer: fake.timer,
  });
  await flushTimelineTick();
  // Vue fetches on mount, then its permanent interval fetches while active.
  await fake.tick(); await fake.tick();
  assert.deepEqual(fetches, ['doc-1', 'doc-1', 'doc-1']);
  assert.ok(updates[0]![0]!.startsWith('docreader:done'));
  // Document completes: the next poll reports the terminal status, fires
  // onStop semantics through updates, and later ticks quiesce to no-ops.
  status = 'completed';
  await fake.tick();
  assert.deepEqual(fetches, ['doc-1', 'doc-1', 'doc-1', 'doc-1']);
  assert.equal(subscription.stopped, false, 'the subscription remains mounted after a terminal update');
  assert.equal(fake.cleared, false, 'only unmount clears Vue\'s permanent interval');
  await fake.tick();
  assert.equal(fetches.length, 4, 'terminal ticks quiesce without another request');
  subscription.stop();
  assert.ok(fake.cleared);
  await fake.tick();
  assert.equal(fetches.length, 4, 'no polls after stop');
});

test('reports a terminal failure once while retaining the interval until unmount', async () => {
  const fake = fakeTimer();
  const stops: string[] = [];
  let status = 'pending';
  const subscription = startProcessingTimeline({
    documentId: 'doc-2',
    intervalMs: 2000,
    getSpans: async () => ({ parse_status: status }),
    onStop: (spans) => stops.push(String(spans?.parse_status)),
    timer: fake.timer,
  });
  await flushTimelineTick();
  assert.deepEqual(stops, []);
  status = 'failed';
  await fake.tick();
  assert.deepEqual(stops, ['failed']);
  assert.equal(subscription.stopped, false);
  assert.equal(fake.cleared, false);
  // Terminal: the permanent interval stays alive, but no later tick fetches
  // or re-fires onStop.
  await fake.tick();
  assert.deepEqual(stops, ['failed']);
  subscription.stop();
});

test('polling errors are surfaced without killing the interval', async () => {
  const fake = fakeTimer();
  const errors: unknown[] = [];
  let fail = true;
  const subscription = startProcessingTimeline({
    documentId: 'doc-3',
    intervalMs: 2000,
    getSpans: async () => { if (fail) throw new Error('boom'); return { parse_status: 'completed' }; },
    onError: (error) => errors.push(error),
    timer: fake.timer,
  });
  await flushTimelineTick();
  assert.equal(errors.length, 1);
  fail = false;
  await fake.tick();
  assert.equal(errors.length, 1, 'recovered on the next tick');
  subscription.stop();
});

test('keeps polling briefly after completion when the trace was recently active', async () => {
  const fake = fakeTimer();
  let fetchCount = 0;
  const subscription = startProcessingTimeline({
    documentId: 'doc-4',
    intervalMs: 2000,
    getSpans: async () => {
      fetchCount += 1;
      return {
        parse_status: 'completed',
        trace: {
          name: 'pipeline',
          finished_at: new Date().toISOString(),
          children: [{ name: 'postprocess', status: 'running' }],
        },
      };
    },
    timer: fake.timer,
  });
  await flushTimelineTick();
  await fake.tick();
  assert.equal(fetchCount, 2, 'Vue grace-polls recently active completed traces');
  subscription.stop();
});
