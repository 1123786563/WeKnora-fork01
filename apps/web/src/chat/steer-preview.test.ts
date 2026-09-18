import assert from 'node:assert/strict';
import test from 'node:test';

import type { ChatMessage } from '@weknora/api-client';
import {
  clearSteerPreviewPending,
  discardSteerPreviews,
  markSteerPreviewFailed,
  previewSteerUserMessage,
  reconcileSteerPreview,
} from './steer-preview.ts';

/*
 * R475-A3 — Vue inject optimistic preview parity (chat/index.vue handleSteerMsg
 * delivery === 'inject' → previewSteerMessage, frontend/src/utils/steerStreamFork.ts):
 * an inject surfaces as a user bubble immediately (steer_pending) so the typed
 * draft never vanishes between submit and the SSE receipt; a failed enqueue
 * keeps the row flagged (steer_failed, Vue _steerFailed) and the SSE receipt /
 * idle-send / stop paths discard or settle it.
 */

function history(): ChatMessage[] {
  return [
    { id: 'user-1', session_id: 'session-1', role: 'user', content: 'hello', is_completed: true },
    { id: 'assistant-1', session_id: 'session-1', role: 'assistant', content: 'hi', is_completed: true },
  ];
}

test('previewSteerUserMessage appends an optimistic pending user bubble for the inject', () => {
  const next = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: '现在就补充' });
  assert.equal(next.length, 3);
  const row = next[2];
  assert.deepEqual(row, {
    id: 'injected-steer-a',
    session_id: 'session-1',
    role: 'user',
    content: '现在就补充',
    is_completed: true,
    steer_id: 'steer-a',
    isSteer: true,
    steer_pending: true,
  });
});

test('previewSteerUserMessage is idempotent per steer id (retries do not duplicate the bubble)', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  rows = previewSteerUserMessage(rows, { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  assert.equal(rows.filter((row) => row.steer_id === 'steer-a').length, 1);
});

test('previewSteerUserMessage retry clears the failed flag (Vue delete preview._steerFailed)', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  rows = markSteerPreviewFailed(rows, 'steer-a');
  rows = previewSteerUserMessage(rows, { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  assert.equal(rows[2]?.steer_failed, undefined, 'the retry resets the failed flag');
  assert.equal(rows[2]?.steer_pending, true, 'the row stays pending for the new attempt');
  assert.equal(rows.filter((row) => row.steer_id === 'steer-a').length, 1);
});

test('previewSteerUserMessage carries the mentioned items onto the optimistic row', () => {
  const mentions = [{ id: 'kb-1', name: 'Docs', type: 'kb' as const }];
  const next = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-b', content: 'use docs', mentionedItems: mentions });
  assert.deepEqual(next[2]?.mentioned_items, mentions);
});

test('markSteerPreviewFailed flags the pending row and keeps it on screen (Vue _steerFailed)', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  rows = markSteerPreviewFailed(rows, 'steer-a');
  assert.equal(rows[2]?.steer_failed, true);
  assert.equal(rows[2]?.steer_pending, true, 'the row stays pending until retried or dropped');
});

test('clearSteerPreviewPending settles the row once the server already injected it', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  rows = markSteerPreviewFailed(rows, 'steer-a');
  rows = clearSteerPreviewPending(rows, 'steer-a');
  assert.equal(rows[2]?.steer_pending, undefined);
  assert.equal(rows[2]?.steer_failed, undefined);
  assert.equal(rows[2]?.steer_id, 'steer-a');
});

test('discardSteerPreviews removes pending preview rows for the given steer ids', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'a' });
  rows = previewSteerUserMessage(rows, { sessionId: 'session-1', steerId: 'steer-b', content: 'b' });
  rows = discardSteerPreviews(rows, ['steer-a']);
  assert.deepEqual(rows.map((row) => row.id), ['user-1', 'assistant-1', 'injected-steer-b']);
});

test('discardSteerPreviews without ids drops every pending preview (stop confirmed / session switch)', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'a' });
  rows = previewSteerUserMessage(rows, { sessionId: 'session-1', steerId: 'steer-b', content: 'b' });
  rows = discardSteerPreviews(rows);
  assert.deepEqual(rows.map((row) => row.id), ['user-1', 'assistant-1']);
});

test('discardSteerPreviews keeps settled and failed preview rows', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'a' });
  rows = clearSteerPreviewPending(rows, 'steer-a');
  rows = discardSteerPreviews(rows, ['steer-a']);
  assert.deepEqual(rows.map((row) => row.id), ['user-1', 'assistant-1', 'injected-steer-a']);
});

test('discardSteerPreviews matches by row id too so the SSE receipt replaces the optimistic row', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'a' });
  // The SSE feed addresses the injected row as `injected-<steerId>`; an older
  // backend issuing its own steer id only matches through the row id shape.
  rows = discardSteerPreviews(rows, ['steer-server-x'], ['injected-steer-a']);
  assert.deepEqual(rows.map((row) => row.id), ['user-1', 'assistant-1']);
});

test('reconcileSteerPreview rebases the optimistic row onto the server steer id', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-client-1', content: 'x' });
  rows = reconcileSteerPreview(rows, 'steer-client-1', 'steer-server-9');
  assert.deepEqual(
    rows.map((row) => ({ id: row.id, steer_id: row.steer_id, pending: row.steer_pending === true })),
    [
      { id: 'user-1', steer_id: undefined, pending: false },
      { id: 'assistant-1', steer_id: undefined, pending: false },
      { id: 'injected-steer-server-9', steer_id: 'steer-server-9', pending: true },
    ],
    'the pending row adopts the server steer id and row id so the SSE receipt replaces it');
});

test('reconcileSteerPreview drops the optimistic duplicate when the SSE receipt landed first', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-client-1', content: 'x' });
  // The SSE receipt already appended the persisted row under the server id.
  rows = [...rows, { id: 'msg-42', session_id: 'session-1', role: 'user', content: 'x', is_completed: true, steer_id: 'steer-server-9' }];
  rows = reconcileSteerPreview(rows, 'steer-client-1', 'steer-server-9');
  assert.deepEqual(rows.map((row) => row.id), ['user-1', 'assistant-1', 'msg-42']);
});

test('reconcileSteerPreview with the same id or an empty server id is a no-op', () => {
  let rows = previewSteerUserMessage(history(), { sessionId: 'session-1', steerId: 'steer-a', content: 'x' });
  let next = reconcileSteerPreview(rows, 'steer-a', 'steer-a');
  assert.deepEqual(next.map((row) => row.id), rows.map((row) => row.id));
  next = reconcileSteerPreview(rows, 'steer-a', '');
  assert.deepEqual(next.map((row) => row.id), rows.map((row) => row.id));
});
