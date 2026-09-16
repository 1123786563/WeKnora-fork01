import assert from 'node:assert/strict';
import test from 'node:test';

test('queues steer input without dropping stable correlation fields', async () => {
  const { createChatSteerApi } = await import('./steer.ts');
  let request: unknown;
  const api = createChatSteerApi(async (input) => {
    request = input;
    return {
      success: true,
      status: 'queued',
      steer_id: '5e7e5a75-31af-4831-b15d-aa3f884334b9',
      assistant_message_id: 'assistant-1',
      delivery: 'after',
    };
  });

  assert.equal((await api.enqueue('session/1', {
    query: 'follow up',
    delivery: 'after',
    steerId: '5e7e5a75-31af-4831-b15d-aa3f884334b9',
    expectedAssistantMessageId: 'assistant-1',
    mentionedItems: [{ id: 'kb-1' }],
    channel: 'web',
  })).status, 'queued');
  assert.deepEqual(request, {
    method: 'POST',
    path: '/api/v1/sessions/session%2F1/steer',
    body: {
      query: 'follow up',
      delivery: 'after',
      steer_id: '5e7e5a75-31af-4831-b15d-aa3f884334b9',
      expected_assistant_message_id: 'assistant-1',
      mentioned_items: [{ id: 'kb-1' }],
      channel: 'web',
    },
  });
});

test('rejects invalid delivery before issuing a request', async () => {
  const { createChatSteerApi } = await import('./steer.ts');
  let calls = 0;
  const api = createChatSteerApi(async () => {
    calls += 1;
    return { success: true, status: 'new_run' };
  });

  await assert.rejects(() => api.enqueue('session-1', { query: 'x', delivery: 'soon' as 'after' }), /inject or after/);
  assert.equal(calls, 0);
});

test('lists, promotes, and removes queue entries through exact routes', async () => {
  const { createChatSteerApi } = await import('./steer.ts');
  const requests: unknown[] = [];
  const responses = [
    { success: true, assistant_message_id: 'assistant-1', items: [{ steer_id: 'steer-1', content: 'x', delivery: 'after' }] },
    { success: true, status: 'queued', steer_id: 'steer-1', assistant_message_id: 'assistant-1', delivery: 'inject' },
    { success: true, status: 'deleted', removed: true, steer_id: 'steer-1' },
  ];
  const api = createChatSteerApi(async (request) => {
    requests.push(request);
    return responses.shift();
  });

  assert.equal((await api.list('session-1')).items[0]?.steer_id, 'steer-1');
  assert.equal((await api.promote('session-1', 'steer/1')).delivery, 'inject');
  assert.equal((await api.remove('session-1', 'steer/1')).status, 'deleted');
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/sessions/session-1/steer' },
    { method: 'POST', path: '/api/v1/sessions/session-1/steer/steer%2F1/inject', body: {} },
    { method: 'DELETE', path: '/api/v1/sessions/session-1/steer/steer%2F1' },
  ]);
});
