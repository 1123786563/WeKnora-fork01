import { test } from 'node:test';
import assert from 'node:assert/strict';
import { applyRunEvent, type ClientRunState } from './agentRunState';

test('replacement clears incomplete text', () => {
  const old: ClientRunState = { seq: 4, attemptId: 'm1', text: '未完成' };
  const next = applyRunEvent(old, {seq:5, attempt_id:'m2', type:'attempt_replaced', payload:{}});
  assert.equal(next.text, '');
  assert.equal(next.attemptId, 'm2');
});
test('duplicate and stale events are ignored', () => {
  const old: ClientRunState = { seq: 5, attemptId: 'm1', text: 'answer' };
  assert.deepEqual(applyRunEvent(old, {seq:5, attempt_id:'m1', type:'answer', payload:{text:'x'}}), old);
  assert.deepEqual(applyRunEvent(old, {seq:4, attempt_id:'m1', type:'answer', payload:{text:'x'}}), old);
});
test('answer deltas only apply to active attempt', () => {
  const old: ClientRunState = { seq: 1, attemptId: 'm1', text: 'a' };
  assert.deepEqual(applyRunEvent(old, {seq:2, attempt_id:'m0', type:'answer_delta', payload:{text:'bad'}}), old);
  assert.equal(applyRunEvent(old, {seq:2, attempt_id:'m1', type:'answer_delta', payload:{text:'bc'}}).text, 'abc');
});
