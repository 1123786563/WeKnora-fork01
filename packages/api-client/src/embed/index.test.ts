import assert from 'node:assert/strict';
import test from 'node:test';

import { createEmbedApi, extractEmbedToken, embedHeaders, buildEmbedFileRequest } from './index.ts';

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

test('exchanges a publish token and creates a signed visitor session without bearer headers', async () => {
  const requests: Array<{ method: string; path: string; headers?: Record<string, string>; body?: unknown }> = [];
  const api = createEmbedApi(async (request) => {
    requests.push(request);
    if (request.path.endsWith('/exchange')) return { success: true, data: { session_token: 'ems-short', expires_in: 60 } };
    return { success: true, data: { id: 'session/1', sig: 'sig-1' } };
  });

  assert.deepEqual(await api.public.exchange('channel/1', 'publish-token'), { sessionToken: 'ems-short', expiresIn: 60 });
  assert.deepEqual(await api.public.createSession('channel/1', 'ems-short'), { id: 'session/1', signature: 'sig-1' });
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/embed/channel%2F1/exchange', headers: { Authorization: 'Embed publish-token' }, body: {} },
    { method: 'POST', path: '/api/v1/embed/channel%2F1/sessions', headers: { Authorization: 'Embed ems-short' }, body: {} },
  ]);
  assert.deepEqual(embedHeaders('ems-short', 'sig-1', 'visitor-1'), {
    Authorization: 'Embed ems-short',
    'X-Embed-Session': 'sig-1',
    'X-Embed-Visitor': 'visitor-1',
  });
});

test('keeps embed chat on the channel-scoped SSE route and forwards visitor context', async () => {
  const requests: Array<{ method: string; path: string; headers?: Record<string, string>; body?: unknown }> = [];
  const events: unknown[] = [];
  const api = createEmbedApi(async (request) => {
    requests.push(request);
    return { success: true, data: [] };
  }, async (request, onEvent) => {
    requests.push(request);
    onEvent({ type: 'answer', content: 'hello' });
  });

  await api.public.chat({
    channelId: 'channel/1', token: 'ems-short', sessionId: 'session/1', signature: 'sig-1', visitorId: 'visitor-1',
    body: { query: 'hi', attachment_uploads: [{ file_name: 'a.txt' }] },
  }, (event) => events.push(event));

  assert.deepEqual(requests, [{
    method: 'POST',
    path: '/api/v1/embed/channel%2F1/knowledge-chat/session%2F1',
    headers: { accept: 'text/event-stream', ...embedHeaders('ems-short', 'sig-1', 'visitor-1') },
    body: { query: 'hi', attachment_uploads: [{ file_name: 'a.txt' }] },
  }]);
  assert.deepEqual(events, [{ type: 'answer', content: 'hello' }]);
});

test('maps channel management and IM endpoints while preserving non-envelope data responses', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createEmbedApi(async (request) => {
    requests.push(request);
    if (request.method === 'DELETE') return { success: true };
    if (request.path.endsWith('/toggle')) return { data: { id: 'im-1', platform: 'feishu' } };
    if (request.path.includes('im-channels')) return { data: [{ id: 'im-1', platform: 'feishu', credentials: { token: 'must-not-leak' } }] };
    return { success: true, data: [{ id: 'embed-1', name: 'Support' }] };
  });

  assert.equal((await api.channels.listAll())[0]?.id, 'embed-1');
  assert.equal((await api.im.listAll())[0]?.id, 'im-1');
  await api.channels.remove('embed/1');
  await api.im.toggle('im/1');
  assert.deepEqual(requests.map(({ method, path }) => [method, path]), [
    ['GET', '/api/v1/embed-channels'],
    ['GET', '/api/v1/im-channels'],
    ['DELETE', '/api/v1/embed-channels/embed%2F1'],
    ['POST', '/api/v1/im-channels/im%2F1/toggle'],
  ]);
});

test('extracts tokens from query or hash without treating arbitrary URL text as a credential', () => {
  assert.equal(extractEmbedToken('https://host.test/embed/c-1?token=query-token'), 'query-token');
  assert.equal(extractEmbedToken('https://host.test/embed/c-1#token=hash-token'), 'hash-token');
  assert.equal(extractEmbedToken('https://host.test/embed/c-1?next=token=not-a-token'), '');
});

test('treats an empty (null) channel/im list as an empty list, not a parse error', async () => {
  const api = createEmbedApi(async () => ({ success: true, data: null }));
  assert.deepEqual(await api.channels.listAll(), []);
  assert.deepEqual(await api.im.listAll(), []);
});

test('uses channel-scoped suggestion endpoints with embed session headers', async () => {
  const requests: Array<{ method: string; path: string; headers?: Record<string, string>; body?: unknown }> = [];
  const api = createEmbedApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET') return { success: true, data: { id: 'set-1', session_id: 's-1', assistant_message_id: 'm-1', status: 'ready', allow_regenerate: true, questions: [{ id: 'q-1', text: 'Next?', source: 'agent' }] } };
    if (request.path.includes('suggestions')) return { success: true, data: { id: 'set-1', session_id: 's-1', assistant_message_id: 'm-1', status: 'ready', allow_regenerate: true, questions: [{ id: 'q-1', text: 'Next?', source: 'agent' }] } };
    return undefined;
  });

  await api.public.ensureMessageSuggestions('channel/1', 'ems-1', 'session/1', 'message/1', 'sig-1', 'visitor-1');
  await api.public.messageSuggestions('channel/1', 'ems-1', 'session/1', 'message/1', 'sig-1', 'visitor-1');
  await api.public.recordMessageSuggestionEvent('channel/1', 'ems-1', 'session/1', 'sig-1', 'visitor-1', 'set-1', 'click', 'q-1');

  assert.deepEqual(requests.map(({ method, path, body }) => ({ method, path, body })), [
    { method: 'POST', path: '/api/v1/embed/channel%2F1/sessions/session%2F1/messages/message%2F1/suggestions', body: { regenerate: false } },
    { method: 'GET', path: '/api/v1/embed/channel%2F1/sessions/session%2F1/messages/message%2F1/suggestions', body: undefined },
    { method: 'POST', path: '/api/v1/embed/channel%2F1/sessions/session%2F1/suggestion-events', body: { suggestion_set_id: 'set-1', question_id: 'q-1', event_type: 'click' } },
  ]);
  assert.equal(requests[0]?.headers?.['X-Embed-Session'], 'sig-1');
  assert.equal(requests[0]?.headers?.['X-Embed-Visitor'], 'visitor-1');
});
