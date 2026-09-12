import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError, parseMessageSuggestionResponse } from '../src/index.ts';

const valid = {
  success: true,
  data: {
    id: 'set-1',
    session_id: 'session-1',
    assistant_message_id: 'message-1',
    status: 'ready',
    allow_regenerate: true,
    questions: [{ id: 'q-1', text: 'What changed?', source: 'model', category: 'deepen' }],
  },
};

test('parses a strict message suggestion set without discarding attribution fields', () => {
  assert.deepEqual(parseMessageSuggestionResponse(valid), valid.data);
});

test('rejects malformed suggestion items instead of rendering an empty fallback', () => {
  assert.throws(
    () => parseMessageSuggestionResponse({ ...valid, data: { ...valid.data, questions: [{ text: 'missing id', source: 'model' }] } }),
    ContractError,
  );
});
