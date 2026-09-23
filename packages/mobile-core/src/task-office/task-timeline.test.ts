import test from 'node:test';
import assert from 'node:assert/strict';
import {
  isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf, timelineKindLabel,
} from './task-timeline.ts';

const source = (seq: number, type: string, payload: Record<string, unknown> = {}) =>
  ({ seq, type, occurredAt: '2026-09-23T00:00:00Z', payload });

test('task lifecycle keeps archive, completion and cancellation separate from run status', () => {
  assert.equal(taskLifecycleOf(undefined, 'running'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'queued'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'waiting_user'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'reconciling'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'failed'), 'active', 'a failed run leaves the goal in progress');
  assert.equal(taskLifecycleOf(undefined, 'succeeded'), 'completed');
  assert.equal(taskLifecycleOf(undefined, 'canceled'), 'canceled');
  assert.equal(taskLifecycleOf('2026-09-23T00:00:00Z', 'succeeded'), 'archived', 'archive wins over completion');
});

test('terminal run status mirrors the server projection rule and precedence', () => {
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.completed')]), 'succeeded');
  assert.equal(terminalRunStatusOf('queued', [source(1, 'execution.succeeded')]), 'succeeded');
  assert.equal(terminalRunStatusOf('reconciling', [source(1, 'status.succeeded')]), 'succeeded');
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.completed'), source(2, 'run.canceled')]), 'canceled', 'canceled beats succeeded, mirroring agent_run_snapshot.go');
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.failed'), source(2, 'run.completed')]), 'failed', 'terminal facts follow the server precedence, not recency');
  assert.equal(terminalRunStatusOf('succeeded', [source(1, 'run.failed')]), 'succeeded', 'a settled base never regresses');
  assert.equal(terminalRunStatusOf('waiting_user', []), 'waiting_user');
  assert.equal(isTerminalRunStatus('succeeded'), true);
  assert.equal(isTerminalRunStatus('waiting_user'), false);
});

test('mergeEventHistory dedupes by seq, prefers snapshot facts and clamps corrupted cache rows', () => {
  const merged = mergeEventHistory(
    [source(1, 'run.started'), source(2, 'tool.started', { tool: 'cached' }), source(9, 'text.delta')],
    [source(2, 'tool.started', { tool: 'authoritative' }), source(3, 'run.completed')],
    3,
  );
  assert.deepEqual(merged.map((event) => event.seq), [1, 2, 3]);
  assert.deepEqual(merged[1]!.payload, { tool: 'authoritative' }, 'the same seq prefers the authoritative snapshot payload');
  assert.equal(merged.some((event) => event.seq === 9), false, 'a persisted row above the watermark is corrupted cache, never replayed');
});

test('projectTimeline classifies known kinds, preserves unknown types and keeps raw evidence', () => {
  const entries = projectTimeline([
    source(3, 'artifact.available', { artifact_id: 'a-1' }),
    source(1, 'run.started'),
    source(2, 'tool.started', { name: 'web.search', arguments: { q: '竞品' } }),
    source(4, 'future.receipt', { external: 'stub' }),
    source(5, 'interaction.required', {}),
  ]);
  assert.deepEqual(entries.map((entry) => entry.seq), [1, 2, 3, 4, 5], 'entries render in authoritative seq order');
  assert.deepEqual(entries.map((entry) => entry.kind), ['run_status', 'tool_activity', 'artifact', 'activity', 'approval']);
  assert.equal(entries[3]!.type, 'future.receipt', 'unknown types are preserved, never dropped or guessed');
  assert.equal(entries[3]!.summary, '执行状态已更新');
  assert.deepEqual(entries[1]!.evidence.payload, { name: 'web.search', arguments: { q: '竞品' } }, 'raw evidence stays available for expansion');
  assert.equal(timelineKindLabel('conclusion'), 'Agent 结论');
  assert.equal(timelineKindLabel('activity'), '活动');
});
