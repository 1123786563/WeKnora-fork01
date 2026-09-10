import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSessionsApi } from './sessions.ts';

test('uses the server pagination and source query for session lists', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createChatSessionsApi(async (request) => {
    requests.push({ method: request.method, path: request.path });
    return { success: true, data: [], total: 0, page: 2, page_size: 30 };
  });

  await api.list({ page: 2, pageSize: 30, source: 'web' });

  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/sessions?page=2&page_size=30&source=web' }]);
});

test('loads message history without requesting public resource URLs', async () => {
  let path = '';
  const api = createChatSessionsApi(async (request) => {
    path = request.path;
    return { success: true, data: [{ id: 'm-1', session_id: 'session-1', role: 'assistant', content: 'safe resource://handle' }] };
  });

  assert.equal((await api.messages('session-1', { beforeTime: '2026-09-10T10:00:00Z', limit: 20 }))[0]?.content, 'safe resource://handle');
  assert.equal(path, '/api/v1/messages/session-1/load?before_time=2026-09-10T10%3A00%3A00Z&limit=20');
});

test('creates an empty server session only after an explicit new-chat action', async () => {
  let request: { method: string; path: string; body?: unknown } | undefined;
  const api = createChatSessionsApi(async (input) => {
    request = input;
    return { success: true, data: { id: 'session-2', title: '', is_pinned: false } };
  });

  assert.deepEqual(await api.create(), { id: 'session-2', title: '', is_pinned: false });
  assert.deepEqual(request, { method: 'POST', path: '/api/v1/sessions', body: {} });
});
