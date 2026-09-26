import assert from 'node:assert/strict';
import test from 'node:test';

import { createWeKnoraClient } from './client.ts';
import { createJsonTransport } from './transport/json.ts';

const jsonResponse = (status: number, body: unknown) => ({
  status,
  headers: new Headers({ 'content-type': 'application/json' }),
  json: async () => body,
  text: async () => typeof body === 'string' ? body : JSON.stringify(body),
});

test('userFavorites.list requests GET /user/favorites?type=agent and parses rows', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return jsonResponse(200, {
      success: true,
      data: [
        { user_id: 'user-1', tenant_id: 7, resource_type: 'agent', resource_id: 'a-1', created_at: '2026-09-01T00:00:00Z' },
        { user_id: 'user-1', tenant_id: 7, resource_type: 'agent', resource_id: 'a-2', created_at: '2026-09-02T00:00:00Z' },
      ],
    });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  const rows = await client.userFavorites.list('agent');

  assert.deepEqual(rows.map((row) => row.resource_id), ['a-1', 'a-2']);
  assert.equal(rows[0]?.resource_type, 'agent');
  assert.equal(rows[0]?.tenant_id, 7);
  assert.equal(requests[0]?.method, 'GET');
  assert.equal(requests[0]?.url, 'https://api.example.test/api/v1/user/favorites?type=agent');
});

test('userFavorites.add posts {type,id} to /user/favorites', async () => {
  const requests: Array<{ method: string; url: string; body: unknown }> = [];
  const transport = createJsonTransport(async (input, init) => {
    const request = new Request(input, init);
    requests.push({ method: request.method, url: request.url, body: init?.body });
    return jsonResponse(200, { success: true });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  await client.userFavorites.add('agent', 'a-1');

  assert.equal(requests[0]?.method, 'POST');
  assert.equal(requests[0]?.url, 'https://api.example.test/api/v1/user/favorites');
  assert.deepEqual(JSON.parse(String(requests[0]?.body)), { type: 'agent', id: 'a-1' });
});

test('userFavorites.remove deletes /user/favorites/:type/:id', async () => {
  const requests: Array<{ method: string; url: string }> = [];
  const transport = createJsonTransport(async (input, init) => {
    const request = new Request(input, init);
    requests.push({ method: request.method, url: request.url });
    return jsonResponse(200, { success: true });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  await client.userFavorites.remove('agent', 'a/1');

  assert.equal(requests[0]?.method, 'DELETE');
  assert.equal(requests[0]?.url, 'https://api.example.test/api/v1/user/favorites/agent/a%2F1');
});

test('userFavorites.list rejects rows with a non-allowlisted resource_type', async () => {
  const transport = createJsonTransport(async () => jsonResponse(200, {
    success: true,
    data: [{ user_id: 'user-1', tenant_id: 7, resource_type: 'prompt', resource_id: 'a-1', created_at: 'x' }],
  }));
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  await assert.rejects(() => client.userFavorites.list('agent'), /resource_type must be "kb" or "agent"/);
});

test('userFavorites.add rejects a non-allowlisted type before any request', async () => {
  let fired = 0;
  const transport = createJsonTransport(async () => { fired += 1; return jsonResponse(200, { success: true }); });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  await assert.rejects(() => client.userFavorites.add('prompt' as 'agent', 'a-1'), /type must be "kb" or "agent"/);
  await assert.rejects(() => client.userFavorites.add('agent', ''), /resourceId must be a non-empty string/);
  assert.equal(fired, 0);
});
