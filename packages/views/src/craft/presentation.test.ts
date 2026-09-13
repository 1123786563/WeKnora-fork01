import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  statusLabel,
  statusLabelEn,
  classifyPreview,
  interactionCardFrom,
  projectAssistant,
  historyRows,
  formatBytes,
  entryFileOf,
  craftStrings,
  archiveLiveTurn,
  createCraftMessageLog,
  type CraftTurnRecord,
} from './presentation.ts';
import type { CraftVersionView } from '@weknora/contracts';

test('child finished still shows main verification', () => {
  assert.equal(statusLabel('running', 'finished'), '主 Agent 正在检查结果');
  assert.equal(statusLabel('stopping', 'running'), '正在停止');
});

test('statusLabel maps every workbench status verbatim', () => {
  assert.equal(statusLabel('waiting_user', 'running'), '需要你的处理');
  assert.equal(statusLabel('queued', 'idle'), '等待执行');
  assert.equal(statusLabel('running', 'idle'), '正在生成');
  assert.equal(statusLabel('succeeded', 'finished'), '已完成');
  assert.equal(statusLabel('failed', 'finished'), '执行失败');
  assert.equal(statusLabel('canceled', 'finished'), '已停止');
  assert.equal(statusLabel('recovering', 'idle'), '状态待核对');
  assert.equal(statusLabel('idle', 'idle'), '状态待核对');
});

test('english labels keep the same branching', () => {
  assert.equal(statusLabelEn('running', 'finished'), 'Main agent is verifying the result');
  assert.equal(statusLabelEn('stopping', 'running'), 'Stopping');
  assert.equal(statusLabelEn('waiting_user', 'running'), 'Needs your input');
  assert.equal(statusLabelEn('succeeded', 'finished'), 'Completed');
  assert.equal(statusLabelEn('mystery', 'idle'), 'Status unknown');
});

// A delegation terminal event never completes the main message: only the Run
// projection (mainStatus from the SSE run frame / snapshot) does.
test('assistant completeness comes only from the main run terminal status', () => {
  const events = [
    { seq: 1, kind: 'delegation.started', data: { prompt_message_id: 'm1' } },
    { seq: 2, kind: 'delegation.text', data: { text: '页面已生成' } },
    { seq: 3, kind: 'delegation.tool', data: { tool: 'write', call_id: 'c1', part_id: 'p1', status: 'completed' } },
    { seq: 4, kind: 'delegation.finished', data: { status: 'succeeded' } },
  ];
  const childDone = projectAssistant(events, 'running');
  assert.equal(childDone.complete, false);
  assert.equal(childDone.text, '页面已生成');
  assert.equal(childDone.tools.length, 1);
  const mainDone = projectAssistant(events, 'succeeded');
  assert.equal(mainDone.complete, true);
  const canceled = projectAssistant(events, 'canceled');
  assert.equal(canceled.complete, true);
  const waiting = projectAssistant(events, 'waiting_user');
  assert.equal(waiting.complete, false);
});

test('artifact and interaction events project into the assistant message', () => {
  const events = [
    { seq: 1, kind: 'delegation.text', data: { text: 'a' } },
    { seq: 2, kind: 'artifact.published', data: { version_id: 'v-9' } },
    {
      seq: 3,
      kind: 'interaction.pending',
      data: { kind: 'permission', part_id: 'p-1', prompt: '运行构建命令', scope: 'npm run build' },
    },
    { seq: 4, kind: 'interaction.resolved', data: { interaction_id: 'p-1' } },
    {
      seq: 5,
      kind: 'interaction.pending',
      data: { kind: 'question', part_id: 'p-2', prompt: '使用哪个时间范围？' },
    },
  ];
  const projection = projectAssistant(events, 'running');
  assert.deepEqual(projection.artifactVersionIds, ['v-9']);
  assert.equal(projection.interactions.length, 2);
  assert.equal(projection.interactions[0].resolved, true);
  assert.equal(projection.interactions[1].resolved, false);
  assert.equal(projection.interactions[1].kind, 'question');
});

test('a question never offers approve; a permission never offers answer', () => {
  const question = interactionCardFrom({ kind: 'question', part_id: 'p-2', prompt: '时间范围？' }, false, 5);
  assert.deepEqual(question.allowedActions, ['answer', 'reject']);
  const permission = interactionCardFrom(
    { kind: 'permission', part_id: 'p-1', prompt: '运行构建', scope: 'npm run build' },
    false,
    6,
  );
  assert.deepEqual(permission.allowedActions, ['approve', 'reject']);
  assert.equal(permission.scope, 'npm run build');
  // Unknown kinds fail closed: explicit waiting, no approve.
  const unknown = interactionCardFrom({ kind: 'future-kind', part_id: 'p-3' }, false, 7);
  assert.equal(unknown.kind, 'unknown');
  assert.deepEqual(unknown.allowedActions, ['reject']);
});

test('preview distinguishes empty, loading, expired, failed and active', () => {
  assert.equal(classifyPreview({ hasArtifact: false, ticketUrl: null, ticketExpiresAt: null, ticketError: null, frameFailed: false, nowMs: 1000 }), 'empty');
  assert.equal(classifyPreview({ hasArtifact: true, ticketUrl: null, ticketExpiresAt: null, ticketError: null, frameFailed: false, nowMs: 1000 }), 'loading');
  assert.equal(classifyPreview({ hasArtifact: true, ticketUrl: 'https://p/x', ticketExpiresAt: new Date(500).toISOString(), ticketError: null, frameFailed: false, nowMs: 1000 }), 'expired');
  assert.equal(classifyPreview({ hasArtifact: true, ticketUrl: 'https://p/x', ticketExpiresAt: new Date(5000).toISOString(), ticketError: 'boom', frameFailed: false, nowMs: 1000 }), 'failed');
  assert.equal(classifyPreview({ hasArtifact: true, ticketUrl: 'https://p/x', ticketExpiresAt: new Date(5000).toISOString(), ticketError: null, frameFailed: false, nowMs: 1000 }), 'active');
  assert.equal(classifyPreview({ hasArtifact: true, ticketUrl: 'https://p/x', ticketExpiresAt: new Date(5000).toISOString(), ticketError: null, frameFailed: true, nowMs: 1000 }), 'failed');
});

function version(id: string, runId: string, checks: { name: string; status: string; detail: string }[]): CraftVersionView {
  return {
    id,
    workspace_id: 'ws-1',
    run_id: runId,
    kind: 'web',
    files: [
      { path: 'dist/index.html', ref: 'resource://v/' + id + '/index.html', sha256: 'a', mime: 'text/html', bytes: 120 },
      { path: 'dist/app.js', ref: 'resource://v/' + id + '/app.js', sha256: 'b', mime: 'text/javascript', bytes: 3400 },
    ],
    checks,
  };
}

test('history rows carry Run and Check facts; time only where the wire has it', () => {
  const updatedAt = '2026-09-13T10:00:00Z';
  const rows = historyRows(
    [version('v-2', 'run-2', [{ name: 'build', status: 'passed', detail: 'ok' }]), version('v-1', 'run-1', [])],
    'v-2',
    updatedAt,
    'zh',
  );
  assert.equal(rows.length, 2);
  assert.equal(rows[0].versionId, 'v-2');
  assert.equal(rows[0].isCurrent, true);
  assert.ok(rows[0].timeLabel !== null);
  assert.equal(rows[1].timeLabel, null);
  assert.equal(rows[1].runId, 'run-1');
  assert.deepEqual(
    rows[0].checks.map((c) => c.status),
    ['passed'],
  );
  assert.equal(rows[0].fileCount, 2);
  assert.equal(rows[0].totalBytes, 3520);
});

test('byte and filename helpers', () => {
  assert.equal(formatBytes(0), '0 B');
  assert.equal(formatBytes(3520), '3.4 KB');
  assert.equal(formatBytes(5 * 1024 * 1024), '5.0 MB');
  const file = entryFileOf(version('v-1', 'run-1', []));
  assert.equal(file !== null && file.path, 'dist/index.html');
  assert.equal(entryFileOf(null), null);
});


// B1 regression: the second send in the SAME session must keep the previous
// turn visible. The archive happens at send time (before the new run's
// subscription resets the message log), so the reset can no longer erase it.
test('second send archives the previous turn before the log resets (B1)', () => {
  const log = createCraftMessageLog();
  log.resetForRun('run-1');
  const frame = (seq: number, kind: string, data: Record<string, unknown>) => ({
    event: String(seq),
    data: JSON.stringify({ seq, attempt_id: null, type: 'craft', payload: { workspace_id: 'ws-1', delegation_id: 'd', tool_call_id: null, kind, data } }),
  });
  log.ingest(frame(1, 'delegation.started', { prompt_message_id: 'm1' }));
  log.ingest(frame(2, 'delegation.text', { text: '第一轮结果' }));

  // Turn 1 finishes (Run projection succeeded) and the user sends turn 2.
  const projection1 = projectAssistant(log.getSnapshot().events, 'succeeded');
  assert.equal(projection1.complete, true);
  const archive = archiveLiveTurn([], { runId: 'run-1', prompt: '第一句', projection: projection1 });
  assert.equal(archive.archived !== null, true);

  // The new run's subscription resets the store — exactly what used to erase
  // the conversation when archiving (dead) relied on a runId effect.
  log.resetForRun('run-2');
  assert.deepEqual(log.getSnapshot().events, []);
  assert.equal(log.getSnapshot().runId, 'run-2');

  // The archived turn survives independent of the store.
  assert.equal(archive.turns.length, 1);
  assert.equal(archive.turns[0].prompt, '第一句');
  assert.equal(archive.turns[0].assistant.text, '第一轮结果');
  assert.equal(archive.turns[0].assistant.complete, true);

  // Turn 2 streams into the reset store; both turns coexist.
  log.ingest(frame(1, 'delegation.text', { text: '第二轮结果' }));
  const projection2 = projectAssistant(log.getSnapshot().events, 'running');
  const archive2 = archiveLiveTurn([...archive.turns], { runId: 'run-2', prompt: '第二句', projection: projection2 });
  assert.equal(archive2.turns.length, 2);
  assert.equal(archive2.turns[0].assistant.text, '第一轮结果');
  assert.equal(archive2.turns[1].assistant.text, '第二轮结果');
});

test('archiveLiveTurn skips runs that never streamed and first sends', () => {
  const empty = projectAssistant([], 'idle');
  assert.equal(archiveLiveTurn([], { runId: null, prompt: '第一句', projection: empty }).archived, null);
  assert.equal(archiveLiveTurn([], { runId: 'run-1', prompt: null, projection: empty }).archived, null);
  const resumed = projectAssistant([{ seq: 1, kind: 'delegation.text', data: { text: '部分结果' } }], 'succeeded');
  const resumedArchive = archiveLiveTurn([], { runId: 'run-1', prompt: null, projection: resumed });
  assert.equal(resumedArchive.archived !== null, true);
  assert.equal(resumedArchive.turns[0].prompt, null);
  assert.equal(resumedArchive.turns[0].assistant.text, '部分结果');
});

test('message log dedupes reconnect replays and keeps only numbered craft frames', () => {
  const log = createCraftMessageLog();
  log.resetForRun('run-1');
  const frame = (seq: number, kind: string) => ({
    event: String(seq),
    data: JSON.stringify({ seq, attempt_id: null, type: 'craft', payload: { workspace_id: 'ws-1', delegation_id: null, tool_call_id: null, kind, data: {} } }),
  });
  log.ingest(frame(1, 'delegation.started'));
  log.ingest(frame(1, 'delegation.started'));
  log.ingest({ event: 'run', data: '{}' });
  log.ingest({ event: 'keepalive', data: '{}' });
  log.ingest({ event: 'error', data: '{"code":"x"}' });
  log.ingest({ data: 'not-json' });
  assert.equal(log.getSnapshot().events.length, 1);
  assert.equal(log.getSnapshot().events[0].seq, 1);
  log.resetForRun('run-1');
  assert.equal(log.getSnapshot().events.length, 1);
});

test('a replaced main-model attempt drops its abandoned partial text (C03)', () => {
  const log = createCraftMessageLog();
  log.resetForRun('run-1');
  const craftFrame = (seq: number, kind: string, data: Record<string, unknown>) => ({
    event: String(seq),
    data: JSON.stringify({ seq, attempt_id: null, type: 'craft', payload: { workspace_id: 'ws-1', delegation_id: 'd', tool_call_id: null, kind, data } }),
  });
  const attemptReplaced = (seq: number, previous: string) => ({
    event: String(seq),
    data: JSON.stringify({ seq, attempt_id: previous, type: 'attempt_replaced', payload: { previous_attempt_id: previous } }),
  });
  log.ingest(craftFrame(1, 'delegation.text', { text: '第一次尝试的前半' }));
  log.ingest(craftFrame(2, 'delegation.text', { text: '段未完成文本' }));
  log.ingest(attemptReplaced(3, 'a1'));
  log.ingest(craftFrame(4, 'delegation.text', { text: '第二次尝试的完整文本' }));
  const projection = projectAssistant(log.getSnapshot().events, 'running');
  // The tRPC projection REPLACES the abandoned partial: the new attempt's
  // text must never be appended after the old one.
  assert.equal(projection.text, '第二次尝试的完整文本');
  assert.equal(projection.complete, false); // still running, never "finished" by text
});

test('i18n dictionaries expose the same keys in zh and en', () => {
  const zh = craftStrings('zh');
  const en = craftStrings('en');
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
  assert.equal(zh.craftHomeTitle.length > 0, true);
  assert.equal(zh.craftHomeTitle !== en.craftHomeTitle, true);
});
