import assert from 'node:assert/strict';
import test from 'node:test';

import {
  appendMessages,
  hasOlderMessages,
  shouldStickToBottom,
  scrollTopAfterPrepend,
  sessionGroups,
} from './session-state.ts';

const message = (id: string, created_at: string, content = id) => ({
  id,
  session_id: 'session-1',
  role: 'user' as const,
  content,
  created_at,
});

test('prepends older history while de-duplicating messages by server id', () => {
  const current = [message('m2', '2026-09-12T10:02:00Z'), message('m3', '2026-09-12T10:03:00Z')];
  const older = [message('m1', '2026-09-12T10:01:00Z'), message('m2', '2026-09-12T10:02:00Z', 'duplicate')];
  assert.deepEqual(appendMessages(current, older), [
    message('m1', '2026-09-12T10:01:00Z'),
    message('m2', '2026-09-12T10:02:00Z'),
    message('m3', '2026-09-12T10:03:00Z'),
  ]);
});

test('uses the oldest loaded timestamp as the next history cursor', () => {
  assert.equal(hasOlderMessages([message('m1', '2026-09-12T10:01:00Z')], 2), false);
  assert.equal(hasOlderMessages([message('m1', '2026-09-12T10:01:00Z'), message('m2', '2026-09-12T10:02:00Z')], 2), true);
});

test('sticks to the live edge only when the user is near the bottom', () => {
  assert.equal(shouldStickToBottom({ scrollTop: 600, scrollHeight: 1000, clientHeight: 280 }), false);
  assert.equal(shouldStickToBottom({ scrollTop: 605, scrollHeight: 1000, clientHeight: 280 }), false);
  assert.equal(shouldStickToBottom({ scrollTop: 700, scrollHeight: 1000, clientHeight: 305 }), true);
});

test('preserves the visible history position when older rows are prepended', () => {
  assert.equal(scrollTopAfterPrepend(120, 900, 1300), 520);
});

test('groups sessions with pinned first and stable date buckets', () => {
  const groups = sessionGroups([
    { id: 'today', title: 'Today', is_pinned: false, updated_at: new Date().toISOString() },
    { id: 'pinned', title: 'Pinned', is_pinned: true, updated_at: new Date().toISOString() },
  ]);
  assert.deepEqual(groups.map((group) => group.key), ['pinned', 'today']);
  assert.deepEqual(groups[0]?.items.map((item) => item.id), ['pinned']);
});

test('keeps the flat session mode available without dropping pinned rows', () => {
  const groups = sessionGroups([
    { id: 'rest', title: 'Rest', is_pinned: false },
    { id: 'pinned', title: 'Pinned', is_pinned: true },
  ], new Date(), 'none');
  assert.deepEqual(groups.map((group) => group.key), ['pinned', 'all']);
});
