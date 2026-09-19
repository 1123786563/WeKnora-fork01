import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

// @weknora/ui pulls in theme.css; node:test needs the same short-circuit as
// the other settings panel tests (usage-panel.test.tsx).
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import { ApiError } from '@weknora/api-client';
import type { ChatMessage, MessageFeedbackRow } from '@weknora/contracts';
const {
  feedbackSummary,
  isQueryHistoryDisabledError,
  messageReferenceCount,
  queryHistoryDateParts,
  queryHistoryListParams,
  sessionSourceText,
  shiftEndTimeExclusive,
} = await import('./QueryHistoryPanel.tsx');

function feedback(overrides: Partial<MessageFeedbackRow>): MessageFeedbackRow {
  return {
    id: 1,
    message_id: 'm-1',
    session_id: 's-1',
    user_id: 'u-1',
    rating: 'like',
    ...overrides,
  } as MessageFeedbackRow;
}

test('feedbackSummary counts like and dislike rows separately', () => {
  const rows: MessageFeedbackRow[] = [
    feedback({ id: 1, rating: 'like' }),
    feedback({ id: 2, rating: 'dislike' }),
    feedback({ id: 3, rating: 'like' }),
    feedback({ id: 4, rating: 'like' }),
  ];
  assert.deepEqual(feedbackSummary(rows), { like: 3, dislike: 1 });
});

test('feedbackSummary zeroes out on an empty snapshot', () => {
  assert.deepEqual(feedbackSummary([]), { like: 0, dislike: 0 });
});

test('shiftEndTimeExclusive moves the picked end date forward one day', () => {
  assert.equal(shiftEndTimeExclusive('2026-09-19'), '2026-09-20');
  assert.equal(shiftEndTimeExclusive('2026-12-31'), '2027-01-01');
  // Malformed input passes through untouched (clampAnalyticsRange sanitized
  // the applied range before this helper runs).
  assert.equal(shiftEndTimeExclusive('not-a-date'), 'not-a-date');
});

test('queryHistoryListParams builds the source=all audit query from the applied filter', () => {
  const params = queryHistoryListParams(
    { userId: '  u-42  ', range: { startTime: '2026-09-01', endTime: '2026-09-19' }, feedback: 'dislike' },
    2,
    50,
  );
  assert.deepEqual(params, {
    source: 'all',
    page: 2,
    pageSize: 50,
    userId: 'u-42',
    startTime: '2026-09-01',
    // end_time is exclusive on the backend; the picked end date must be
    // covered, so it shifts forward one day.
    endTime: '2026-09-20',
    feedback: 'dislike',
  });
});

test('queryHistoryListParams omits the empty user filter and the "all" feedback filter', () => {
  const params = queryHistoryListParams(
    { userId: '   ', range: { startTime: '2026-09-01', endTime: '2026-09-19' }, feedback: 'all' },
    0,
    50,
  );
  assert.deepEqual(params, {
    source: 'all',
    page: 0,
    pageSize: 50,
    startTime: '2026-09-01',
    endTime: '2026-09-20',
  });
  assert.equal('userId' in params, false);
  assert.equal('feedback' in params, false);
});

test('messageReferenceCount reads the knowledge_references array length', () => {
  const none: ChatMessage = { id: 'm-1', session_id: 's-1', role: 'user', content: 'hi' };
  assert.equal(messageReferenceCount(none), 0);
  const withRefs = { ...none, knowledge_references: [{ id: 'k-1' }, { id: 'k-2' }, { id: 'k-3' }] } as ChatMessage;
  assert.equal(messageReferenceCount(withRefs), 3);
});
test('isQueryHistoryDisabledError matches only HTTP 403 responses', () => {
  assert.equal(isQueryHistoryDisabledError(new ApiError({ status: 403, code: 'HTTP_403', message: 'query history is disabled for this tenant' })), true);
  assert.equal(isQueryHistoryDisabledError(new ApiError({ status: 500, code: 'HTTP_500', message: 'boom' })), false);
  assert.equal(isQueryHistoryDisabledError(new Error('plain failure')), false);
  assert.equal(isQueryHistoryDisabledError('not an error'), false);
  assert.equal(isQueryHistoryDisabledError(null), false);
});

test('queryHistoryDateParts renders a valid timestamp and passes garbage through', () => {
  const parts = queryHistoryDateParts('2026-09-19T08:30:00Z', 'zh-CN');
  assert.match(parts.date, /^\d{4}[/-]\d{2}[/-]\d{2}$/);
  assert.match(parts.time, /^\d{2}:\d{2}:\d{2}$/);
  assert.deepEqual(queryHistoryDateParts(undefined, 'zh-CN'), { date: '-', time: '' });
  assert.deepEqual(queryHistoryDateParts('garbage', 'zh-CN'), { date: 'garbage', time: '' });
});

test('sessionSourceText maps the empty web bucket to "web"', () => {
  assert.equal(sessionSourceText({ id: 's-1', title: 't', is_pinned: false, source: '' }), 'web');
  assert.equal(sessionSourceText({ id: 's-1', title: 't', is_pinned: false, source: 'im' }), 'im');
  assert.equal(sessionSourceText({ id: 's-1', title: 't', is_pinned: false }), 'web');
});
