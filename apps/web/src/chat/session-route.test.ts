import assert from 'node:assert/strict';
import test from 'node:test';

import { chatSessionIdFromPath } from './session-route.ts';

test('selects only the persisted-session route and decodes its identifier', () => {
  assert.equal(chatSessionIdFromPath('/platform/chat/session%2F1'), 'session/1');
  assert.equal(chatSessionIdFromPath('/platform/creatChat'), null);
});

test('does not interpret a malformed or unrelated path as a session id', () => {
  assert.equal(chatSessionIdFromPath('/platform/chat/%E0%A4%A'), null);
  assert.equal(chatSessionIdFromPath('/platform/knowledge-bases'), null);
});
