import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSuggestionsApi } from './suggestions.ts';

test('builds ensure/get/event requests with encoded ids and strict response parsing', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createChatSuggestionsApi(async (request) => {
    requests.push({ method: request.method, path: request.path, ...(request.body === undefined ? {} : { body: request.body }) });
    if (request.path.includes('suggestions')) {
      return {
        success: true,
        data: { id: 'set-1', session_id: 'session/1', assistant_message_id: 'message/1', status: 'ready', allow_regenerate: true, questions: [] },
      };
    }
    return undefined;
  });

  await api.ensure('session/1', 'message/1', true);
  await api.get('session/1', 'message/1');
  await api.recordEvent('session/1', 'set-1', 'click', 'question-1');
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/sessions/session%2F1/messages/message%2F1/suggestions', body: { regenerate: true } },
    { method: 'GET', path: '/api/v1/sessions/session%2F1/messages/message%2F1/suggestions' },
    { method: 'POST', path: '/api/v1/sessions/session%2F1/suggestion-events', body: { suggestion_set_id: 'set-1', question_id: 'question-1', event_type: 'click' } },
  ]);
});
