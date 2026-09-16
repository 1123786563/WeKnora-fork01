import assert from 'node:assert/strict';
import test from 'node:test';

import { findResumeTargetMessage, markChatMessageStopped } from './resume.ts';

test('stopping the active assistant marks its transient row complete so reload does not resume it', () => {
  const messages = [
    { id: 'user-1', session_id: 's1', role: 'user' as const, content: 'hello', is_completed: true },
    { id: 'stream-s1', session_id: 's1', role: 'assistant' as const, content: 'partial', is_completed: false },
  ];
  const stopped = markChatMessageStopped(messages, 's1', 'assistant-9');
  assert.equal(stopped[1]?.is_completed, true);
  assert.equal(findResumeTargetMessage(stopped), undefined);
});

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
