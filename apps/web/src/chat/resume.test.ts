import assert from 'node:assert/strict';
import test from 'node:test';

import { findResumeTargetMessage } from './resume.ts';

test('resume targets the last incomplete assistant message only', () => {
  assert.equal(findResumeTargetMessage([]), undefined);
  assert.equal(findResumeTargetMessage([
    { id: 'a1', session_id: 's', role: 'assistant', content: 'done', is_completed: true },
  ]), undefined);
  assert.equal(findResumeTargetMessage([
    { id: 'a1', session_id: 's', role: 'assistant', content: 'done', is_completed: true },
    { id: 'u1', session_id: 's', role: 'user', content: 'q', is_completed: true },
    { id: 'a2', session_id: 's', role: 'assistant', content: 'partial', is_completed: false },
  ]), 'a2');
  // IM-originated replies never stream through this server; skip them.
  assert.equal(findResumeTargetMessage([
    { id: 'a2', session_id: 's', role: 'assistant', content: 'partial', is_completed: false, channel: 'im' },
  ]), undefined);
});
