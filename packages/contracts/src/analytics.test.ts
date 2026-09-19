import assert from 'node:assert/strict';
import test from 'node:test';
import { parseQueryTrendResponse, parseMessageFeedbackListResponse } from './analytics.ts';

test('parseQueryTrendResponse unwraps envelope and validates fields', () => {
  const result = parseQueryTrendResponse({ success: true, data: [{ date: '2026-09-19', queries: 3, likes: 1, dislikes: 0 }] });
  assert.deepEqual(result.items, [{ date: '2026-09-19', queries: 3, likes: 1, dislikes: 0 }]);
});
test('parseMessageFeedbackListResponse rejects unknown rating', () => {
  assert.throws(() => parseMessageFeedbackListResponse({ success: true, data: [{ id: 1, message_id: 'm', session_id: 's', rating: 'meh' }] }));
});
test('parse rejects non-envelope', () => {
  assert.throws(() => parseQueryTrendResponse({ data: [] }));
});
