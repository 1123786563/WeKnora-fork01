import assert from 'node:assert/strict';
import test from 'node:test';

import { chatDraftKey } from './draft.ts';

test('isolates a session draft by origin, user, tenant, and session', () => {
  assert.deepEqual(
    chatDraftKey({ origin: 'https://api.example.test/', userId: 'user-a', tenantId: 'tenant-a', sessionId: 'session-1' }),
    ['weknora', 'chat-draft', 'https://api.example.test', 'user-a', 'tenant-a', 'session-1'],
  );
  assert.notDeepEqual(
    chatDraftKey({ origin: 'https://api.example.test', userId: 'user-a', tenantId: 'tenant-b', sessionId: 'session-1' }),
    chatDraftKey({ origin: 'https://api.example.test', userId: 'user-a', tenantId: 'tenant-a', sessionId: 'session-1' }),
  );
});

test('rejects blank session identifiers rather than sharing an anonymous draft', () => {
  assert.throws(
    () => chatDraftKey({ origin: 'https://api.example.test', userId: null, tenantId: null, sessionId: ' ' }),
    /sessionId must not be empty/,
  );
});
