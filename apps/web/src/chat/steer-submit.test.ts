import assert from 'node:assert/strict';
import test from 'node:test';

import { buildSteerAction, isSteerConflict, STEER_CONFLICT_STATUS } from './steer-submit.ts';

test('idle steer routes through the normal send path', () => {
  const action = buildSteerAction({
    streaming: false,
    content: 'next question',
    assistantMessageId: undefined,
    newSteerId: () => 'sid-1',
  });
  assert.deepEqual(action, { kind: 'send', submission: { content: 'next question', status: 'pending' } });
});

test('streaming steer enqueues with the expected assistant message id and a client steer id', () => {
  const action = buildSteerAction({
    streaming: true,
    content: 'correction',
    assistantMessageId: 'assistant-9',
    newSteerId: () => 'sid-2',
  });
  assert.deepEqual(action, {
    kind: 'enqueue',
    input: {
      query: 'correction',
      delivery: 'after',
      channel: 'web',
      expectedAssistantMessageId: 'assistant-9',
      steerId: 'sid-2',
    },
  });
});

test('conflict detection recognises the 409 status for re-base', () => {
  assert.equal(STEER_CONFLICT_STATUS, 409);
  assert.equal(isSteerConflict({ status: 409 }), true);
  assert.equal(isSteerConflict({ status: 500 }), false);
  assert.equal(isSteerConflict(new Error('x')), false);
});
