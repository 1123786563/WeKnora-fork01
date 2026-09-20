import assert from 'node:assert/strict';
import test from 'node:test';

// R483 F4 — Vue KB detail document row menu parity (R482 B1 diff #8).
// The Vue menu (frontend/src/views/knowledge/components/DocumentActionMenu.vue)
// renders, for a failed file document with download/mutate rights and a real
// trace: 下载 / 查看 Trace / 重建知识 / 移动到目录 / 移动到... / 批量管理 /
// 删除文档. This module models the item list, the /spans probe the trace item
// is gated on, the compact trace summary the hover popover shows, and the
// move-to-KB sub-flow state machine.
import {
  DEFAULT_MOVE_MODE,
  buildTraceSummary,
  documentMenuItems,
  formatTraceDurationMs,
  knowledgeSpansViewHasTrace,
  moveMenuViewAfterBack,
  type DocumentMenuInput,
} from './doc-row-menu.ts';

test('documentMenuItems renders the seven Vue items in order for a failed file document', () => {
  const input: DocumentMenuInput = {
    source: 'file',
    parseStatus: 'failed',
    canDownload: true,
    canMutateKnowledge: true,
    traceAvailable: true,
  };
  assert.deepEqual(documentMenuItems(input), [
    { action: 'download', labelKey: 'knowledgeBase.downloadDocument' },
    { action: 'view-trace', labelKey: 'knowledgeStages.viewTrace' },
    { action: 'reparse', labelKey: 'knowledgeBase.rebuildDocument' },
    { action: 'move-folder', labelKey: 'knowledgeBase.moveToFolder.action' },
    { action: 'move-kb', labelKey: 'knowledgeBase.moveDocument' },
    { action: 'batch-manage', labelKey: 'menu.batchManage' },
    { action: 'delete', labelKey: 'knowledgeBase.deleteDocument' },
  ]);
});

test('documentMenuItems keeps the Vue gates: manual edit, in-flight cancel-parse, url without download', () => {
  const inFlight = documentMenuItems({
    source: 'manual',
    parseStatus: 'processing',
    canDownload: true,
    canMutateKnowledge: true,
  });
  // In-flight: trace visible without a probe, cancel-parse replaces the
  // popconfirm reparse entry (Vue renders plain reparse + cancel-parse).
  assert.deepEqual(
    inFlight.map((item) => item.action),
    ['download', 'edit', 'view-trace', 'reparse', 'cancel-parse', 'move-folder', 'move-kb', 'batch-manage', 'delete'],
  );
  const urlDoc = documentMenuItems({
    source: 'url',
    parseStatus: 'completed',
    canDownload: true,
    canMutateKnowledge: true,
    traceAvailable: false,
  });
  assert.deepEqual(
    urlDoc.map((item) => item.action),
    ['reparse', 'move-folder', 'move-kb', 'batch-manage', 'delete'],
  );
  const readOnly = documentMenuItems({
    source: 'file',
    parseStatus: 'completed',
    canDownload: false,
    canMutateKnowledge: false,
  });
  // Vue batch-manage gate: canMutateKnowledge || canDownload; delete renders
  // whenever the menu itself renders (Vue DocumentActionMenu has no gate).
  assert.deepEqual(
    readOnly.map((item) => item.action),
    ['reparse', 'delete'],
  );
});

test('documentMenuItems hides the trace item until the /spans probe confirms a real trace', () => {
  const noProbeYet = documentMenuItems({
    source: 'file',
    parseStatus: 'failed',
    canDownload: true,
    canMutateKnowledge: true,
  });
  assert.equal(noProbeYet.some((item) => item.action === 'view-trace'), false);
  const probedFalse = documentMenuItems({
    source: 'file',
    parseStatus: 'failed',
    canDownload: true,
    canMutateKnowledge: true,
    traceAvailable: false,
  });
  assert.equal(probedFalse.some((item) => item.action === 'view-trace'), false);
});

// Vue utils/knowledgeTrace.ts knowledgeSpansPayloadHasTrace.
test('knowledgeSpansViewHasTrace ports the Vue span_id / current_attempt rule', () => {
  assert.equal(knowledgeSpansViewHasTrace({ trace: { span_id: 'span-1' } }), true);
  assert.equal(knowledgeSpansViewHasTrace({ trace: {}, current_attempt: 2 }), true);
  assert.equal(knowledgeSpansViewHasTrace({ trace: {} }), false);
  assert.equal(knowledgeSpansViewHasTrace({ trace: null }), false);
  assert.equal(knowledgeSpansViewHasTrace({}), false);
  assert.equal(knowledgeSpansViewHasTrace(null), false);
});

// Vue knowledge-processing-timeline.vue formatDuration.
test('formatTraceDurationMs formats like the Vue timeline', () => {
  assert.equal(formatTraceDurationMs(17), '17ms');
  assert.equal(formatTraceDurationMs(940), '940ms');
  assert.equal(formatTraceDurationMs(1700), '1.70s');
  assert.equal(formatTraceDurationMs(61000), '1m1.0s');
  assert.equal(formatTraceDurationMs(125000), '2m5.0s');
  assert.equal(formatTraceDurationMs(undefined), '—');
  assert.equal(formatTraceDurationMs(-5), '—');
});

// Vue totalMs computed: max(trace.duration_ms, observed span tail span).
test('buildTraceSummary reads total duration from the trace root first', () => {
  // Vue totalMs = max(root duration_ms, observed span window): the root
  // duration covers the whole pipeline even when the serialized span window
  // is shorter (async postprocess tails work the other way round).
  const summary = buildTraceSummary({
    parse_status: 'failed',
    trace: {
      name: 'knowledge_processing',
      status: 'failed',
      duration_ms: 17,
      started_at: '2026-09-19T02:35:00.000Z',
      finished_at: '2026-09-19T02:35:00.005Z',
      children: [],
    },
  });
  assert.equal(summary.totalMs, 17);
  assert.equal(summary.duration, '17ms');
  assert.equal(summary.stageTotal, 5);
});

test('buildTraceSummary falls back to the observed span window when duration_ms is absent', () => {
  const summary = buildTraceSummary({
    parse_status: 'completed',
    trace: {
      status: 'done',
      started_at: '2026-09-19T02:35:00.000Z',
      finished_at: '2026-09-19T02:35:02.500Z',
      children: [],
    },
  });
  assert.equal(summary.totalMs, 2500);
  assert.equal(summary.duration, '2.50s');
});

test('buildTraceSummary counts traversed stages for the progress fallback caption', () => {
  const summary = buildTraceSummary({
    trace: {
      children: [
        { name: 'docreader', status: 'done' },
        { name: 'chunking', status: 'done' },
      ],
    },
  });
  assert.equal(summary.totalMs, 0);
  assert.equal(summary.duration, '—');
  // 2 done stages traversed → Vue currentStageIndex = 3/5.
  assert.equal(summary.stageIndex, 3);
  assert.equal(summary.stageTotal, 5);
});

// Vue handleMoveBack (KnowledgeBase.vue L1539-1544).
test('moveMenuViewAfterBack steps back through the Vue sub-views', () => {
  assert.equal(moveMenuViewAfterBack('confirm'), 'targets');
  assert.equal(moveMenuViewAfterBack('targets'), 'normal');
  assert.equal(moveMenuViewAfterBack('normal'), 'normal');
});

// Vue handleMoveSelectTarget resets the mode to reuse_vectors.
test('the move confirm view opens on the Vue default mode', () => {
  assert.equal(DEFAULT_MOVE_MODE, 'reuse_vectors');
});
