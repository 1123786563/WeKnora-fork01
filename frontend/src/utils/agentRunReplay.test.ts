import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRunReplay, isTerminalRunStatus } from './agentRunReplay';

const delta = (seq: number, text: string, attempt = 'm1') => ({
  seq, attempt_id: attempt, type: 'answer_delta', payload: { text },
});

test('emits incremental deltas without duplicating text', () => {
  const replay = createRunReplay();
  const first = replay.consume([delta(1, '你好', 'm1')]);
  assert.equal(first.reset, true);
  assert.equal(first.delta, '你好');
  const second = replay.consume([delta(2, '，世界')]);
  assert.equal(second.reset, false);
  assert.equal(second.delta, '，世界');
});

test('replaying an overlapping cursor is a no-op', () => {
  const replay = createRunReplay();
  replay.consume([delta(1, 'a'), delta(2, 'b')]);
  const again = replay.consume([delta(1, 'a'), delta(2, 'b')]);
  assert.equal(again.changed, false);
  assert.equal(again.delta, '');
});

test('attempt replacement resets and returns the new full text', () => {
  const replay = createRunReplay();
  replay.consume([delta(1, '未完成')]);
  const next = replay.consume([
    { seq: 2, attempt_id: 'm2', type: 'attempt_replaced', payload: {} },
    delta(3, '全新', 'm2'),
  ]);
  assert.equal(next.reset, true);
  assert.equal(next.delta, '全新');
});

test('terminal statuses are recognized', () => {
  assert.equal(isTerminalRunStatus('succeeded'), true);
  assert.equal(isTerminalRunStatus('failed'), true);
  assert.equal(isTerminalRunStatus('canceled'), true);
  assert.equal(isTerminalRunStatus('running'), false);
  assert.equal(isTerminalRunStatus(undefined), false);
});
