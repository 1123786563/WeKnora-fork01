import assert from 'node:assert/strict';
import test from 'node:test';

import { isKnowledgeProcessingTerminal, normalizeKnowledgeProcessingStatus, processingStatusLabel } from './processing.ts';

test('preserves every backend processing state', () => {
  for (const status of ['pending', 'processing', 'finalizing', 'completed', 'failed', 'deleting', 'cancelled']) {
    assert.equal(normalizeKnowledgeProcessingStatus(status), status);
  }
  assert.equal(isKnowledgeProcessingTerminal('completed'), true);
  assert.equal(isKnowledgeProcessingTerminal('processing'), false);
  assert.equal(processingStatusLabel('finalizing'), 'Finalizing');
  assert.equal(processingStatusLabel('completed'), 'Completed');
});

test('rejects unknown processing states instead of presenting them as completed', () => {
  assert.throws(() => normalizeKnowledgeProcessingStatus('indexed'), /Unknown knowledge processing status/);
});

// ===== Processing timeline =====

test('timeline keeps polling only while parsing is in flight', async () => {
  const { shouldPollKnowledgeSpans, isKnowledgeProcessingActive } = await import('./processing.ts');
  for (const active of ['pending', 'processing', 'finalizing']) {
    assert.equal(shouldPollKnowledgeSpans(active), true, active);
    assert.equal(isKnowledgeProcessingActive(active), true, active);
  }
  for (const terminal of ['completed', 'failed', 'cancelled', 'deleting', undefined, '', 'unknown']) {
    assert.equal(shouldPollKnowledgeSpans(terminal), false, String(terminal));
  }
});

test('timeline renders the ordered pipeline stages from the spans trace', async () => {
  const { buildKnowledgeTimeline } = await import('./processing.ts');
  const spans = {
    knowledge_id: 'k1',
    parse_status: 'processing',
    current_stage: 'embedding',
    trace: {
      name: 'pipeline',
      children: [
        { name: 'docreader', status: 'completed', duration_ms: 1200 },
        { name: 'chunking', status: 'completed', duration_ms: 300 },
        { name: 'embedding', status: 'running' },
      ],
    },
  };
  const steps = buildKnowledgeTimeline(spans);
  assert.deepEqual(steps.map((step) => step.stage), ['docreader', 'chunking', 'embedding', 'multimodal', 'postprocess']);
  assert.deepEqual(steps.map((step) => step.state), ['done', 'done', 'running', 'pending', 'pending']);
});

test('timeline treats unstarted and failed stages explicitly', async () => {
  const { buildKnowledgeTimeline } = await import('./processing.ts');
  const steps = buildKnowledgeTimeline({
    parse_status: 'failed',
    trace: { name: 'root', children: [{ name: 'docreader', status: 'failed' }] },
  });
  assert.deepEqual(steps.map((step) => step.state), ['failed', 'pending', 'pending', 'pending', 'pending']);
});

test('timeline without a trace still marks the current stage as running', async () => {
  const { buildKnowledgeTimeline } = await import('./processing.ts');
  const steps = buildKnowledgeTimeline({ parse_status: 'processing', current_stage: 'docreader', trace: null });
  assert.equal(steps[0]!.state, 'running');
  assert.equal(steps[1]!.state, 'pending');
});

test('trace rows preserve tree depth and stable keys for expandable waterfall rendering', async () => {
  const { flattenKnowledgeSpans } = await import('./processing.ts');
  const rows = flattenKnowledgeSpans({ name: 'pipeline', children: [{ name: 'embedding', children: [{ name: 'provider-call' }] }] });
  assert.deepEqual(rows.map((row) => ({ key: row.key, depth: row.depth, hasChildren: row.hasChildren, name: row.node.name })), [
    { key: 'root', depth: 0, hasChildren: true, name: 'pipeline' },
    { key: 'root.0', depth: 1, hasChildren: true, name: 'embedding' },
    { key: 'root.0.0', depth: 2, hasChildren: false, name: 'provider-call' },
  ]);
});

// R472-A1 (R470 遗留): the backend serializes spans with started_at/
// finished_at timestamps — Vue's nodeStart/nodeEnd read exactly those
// fields — and a disabled multimodal stage closes as status 'skipped'.
// The React domain model only knew start_time/end_time and had no
// skipped state, so the fixture below rendered multimodal as 进行中
// (running) instead of Vue's 已跳过.
test('timeline renders a skipped multimodal span as skipped with started_at/finished_at timestamps', async () => {
  const { buildKnowledgeTimeline } = await import('./processing.ts');
  const spans = {
    knowledge_id: 'k1',
    parse_status: 'completed',
    current_stage: 'postprocess',
    trace: {
      name: 'pipeline',
      children: [
        { name: 'docreader', status: 'done', started_at: '2026-09-18T10:00:00Z', finished_at: '2026-09-18T10:00:02Z', duration_ms: 2000 },
        { name: 'chunking', status: 'done', started_at: '2026-09-18T10:00:02Z', finished_at: '2026-09-18T10:00:02Z', duration_ms: 300 },
        { name: 'embedding', status: 'done', started_at: '2026-09-18T10:00:02Z', finished_at: '2026-09-18T10:00:05Z', duration_ms: 3000 },
        { name: 'multimodal', status: 'skipped', started_at: '2026-09-18T10:00:05Z', finished_at: '2026-09-18T10:00:05Z', duration_ms: 1 },
        { name: 'postprocess', status: 'done', started_at: '2026-09-18T10:00:05Z', finished_at: '2026-09-18T10:00:05Z', duration_ms: 9 },
      ],
    },
  };
  const steps = buildKnowledgeTimeline(spans);
  assert.deepEqual(steps.map((step) => step.state), ['done', 'done', 'done', 'skipped', 'done']);
});

test('span status resolves finished_at/started_at aliases like Vue nodeStart/nodeEnd', async () => {
  const { buildKnowledgeTimeline } = await import('./processing.ts');
  // Backend spans without an explicit status still carry started_at/
  // finished_at; a closed span is done, an open one is running.
  const steps = buildKnowledgeTimeline({
    parse_status: 'processing',
    current_stage: 'embedding',
    trace: {
      name: 'pipeline',
      children: [
        { name: 'docreader', started_at: '2026-09-18T10:00:00Z', finished_at: '2026-09-18T10:00:02Z' },
        { name: 'embedding', started_at: '2026-09-18T10:00:02Z' },
      ],
    },
  });
  assert.equal(steps[0]!.state, 'done');
  assert.equal(steps[2]!.state, 'running');
  assert.equal(steps[3]!.state, 'pending');
});
