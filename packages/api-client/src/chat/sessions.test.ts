import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatSessionsApi } from './sessions.ts';

test('uses the server pagination and source query for session lists', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createChatSessionsApi(async (request) => {
    requests.push({ method: request.method, path: request.path });
    return { success: true, data: [], total: 0, page: 2, page_size: 30 };
  });

  await api.list({ page: 2, pageSize: 30, source: 'web', keyword: 'release' });

  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/sessions?page=2&page_size=30&source=web&keyword=release' }]);
});

test('clears only the selected session after a server-confirmed action', async () => {
  let request: { method: string; path: string; body?: unknown } | undefined;
  const api = createChatSessionsApi(async (input) => {
    request = input;
    return { success: true };
  });
  await api.clear('session/1');
  assert.deepEqual(request, { method: 'DELETE', path: '/api/v1/sessions/session%2F1/messages' });
});

test('deletes selected sessions through the batch endpoint in one request', async () => {
  let request: { method: string; path: string; body?: unknown; signal?: AbortSignal } | undefined;
  const api = createChatSessionsApi(async (input) => { request = input; return { success: true }; });
  const controller = new AbortController();
  await api.batchRemove(['session/1', 'session-2'], controller.signal);
  assert.deepEqual(request, { method: 'DELETE', path: '/api/v1/sessions/batch', body: { ids: ['session/1', 'session-2'] }, signal: controller.signal });
  await assert.rejects(() => api.batchRemove([]), /sessionIds must not be empty/);
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

test('fetches one typed session by id for consumers that need a single title (GET /api/v1/sessions/:id)', async () => {
    const requests: Array<{ method: string; path: string; signal?: AbortSignal }> = [];
    const api = createChatSessionsApi(async (input) => {
      requests.push({ method: input.method, path: input.path, signal: input.signal });
      return { success: true, data: { id: 'session/1', title: '  季度盘点助手  ', is_pinned: false } };
    });

    const controller = new AbortController();
    assert.deepEqual(await api.get('session/1', controller.signal), {
      id: 'session/1',
      title: '  季度盘点助手  ',
      is_pinned: false,
    });
    assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/sessions/session%2F1', signal: controller.signal }]);
  });

  test('refuses to fetch a session without an id', async () => {
    const api = createChatSessionsApi(async () => { throw new Error('must not request'); });
    await assert.rejects(() => api.get('   '), /sessionId must not be empty/);
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

test('keeps session rename, pin, unpin, and delete operations on typed server boundaries', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createChatSessionsApi(async (input) => {
    requests.push(input);
    if (input.method === 'PUT') return { success: true, data: { id: 'session/1', title: 'Renamed', is_pinned: false } };
    return { success: true, message: 'ok' };
  });

  assert.deepEqual(await api.update('session/1', { title: 'Renamed' }), { id: 'session/1', title: 'Renamed', is_pinned: false });
  await api.pin('session/1');
  await api.unpin('session/1');
  await api.remove('session/1');
  assert.deepEqual(requests, [
    { method: 'PUT', path: '/api/v1/sessions/session%2F1', body: { title: 'Renamed' } },
    { method: 'POST', path: '/api/v1/sessions/session%2F1/pin' },
    { method: 'DELETE', path: '/api/v1/sessions/session%2F1/pin' },
    { method: 'DELETE', path: '/api/v1/sessions/session%2F1' },
  ]);
});
