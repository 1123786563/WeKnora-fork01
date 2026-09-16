import assert from 'node:assert/strict';
import test from 'node:test';

import { createEmbedApi, buildEmbedFileRequest } from './index.ts';

test('builds channel-scoped protected file requests with embed credentials', () => {
  assert.deepEqual(buildEmbedFileRequest('channel/a', 'ems short-lived', 'resource://tenant/a-b.png'), {
    method: 'GET',
    path: '/api/v1/embed/channel%2Fa/files?file_path=resource%3A%2F%2Ftenant%2Fa-b.png',
    headers: { Authorization: 'Embed ems short-lived' },
  });
});

test('refuses public URLs and credential-free protected file requests', () => {
  assert.equal(buildEmbedFileRequest('channel', 'token', 'https://cdn.example/image.png'), null);
  assert.equal(buildEmbedFileRequest('channel', '', 'resource://handle'), null);
  assert.equal(buildEmbedFileRequest('', 'token', 'resource://handle'), null);
});

test('embed API keeps session signatures and visitor ids in headers', async () => {
  const requests: unknown[] = [];
  const api = createEmbedApi(async (request) => {
    requests.push(request);
    if (request.path.endsWith('/config')) return { success: true, data: { channel_id: 'c', agent_id: 'a' } };
    return { success: true, data: [] };
  });

  await api.config('c', 'publish-token');
  await api.messages('c', 'embed-token', 'session/1', 20, undefined, 'sig');
  await api.messageSuggestions('c', 'embed-token', 'session/1', 'message/1', 'sig', 'visitor/1');

  assert.deepEqual(requests, [
    {
      method: 'GET',
      path: '/api/v1/embed/c/config',
      headers: { Authorization: 'Embed publish-token' },
    },
    {
      method: 'GET',
      path: '/api/v1/embed/c/messages/session%2F1/load?limit=20',
      headers: { Authorization: 'Embed embed-token', 'X-Embed-Session': 'sig' },
    },
    {
      method: 'GET',
      path: '/api/v1/embed/c/sessions/session%2F1/messages/message%2F1/suggestions',
      headers: { Authorization: 'Embed embed-token', 'X-Embed-Session': 'sig', 'X-Embed-Visitor': 'visitor/1' },
    },
  ]);
});
