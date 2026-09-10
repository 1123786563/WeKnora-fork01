import assert from 'node:assert/strict';
import test from 'node:test';

import { createWeKnoraClient } from '../client.ts';
import { createJsonTransport } from '../transport/json.ts';

const response = (status: number, body: unknown) => ({
  status,
  headers: new Headers({ 'content-type': 'application/json' }),
  json: async () => body,
  text: async () => JSON.stringify(body),
});

test('creates and updates knowledge bases through typed response parsing', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return response(200, { success: true, data: { id: 'kb-1', name: 'Updated', type: 'document' } });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });
  assert.equal((await client.knowledgeBases.create({ name: 'Docs' })).id, 'kb-1');
  assert.equal((await client.knowledgeBases.update('kb/a', { name: 'Updated' })).name, 'Updated');
  assert.equal(requests[0]?.method, 'POST');
  assert.equal(requests[1]?.url, 'https://api.example.test/api/v1/knowledge-bases/kb%2Fa');
  assert.deepEqual(await requests[0]?.json(), { name: 'Docs' });
});

test('deletes a knowledge base and preserves non-success responses as errors', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return response(requests.length === 1 ? 204 : 409, requests.length === 1 ? '' : { code: 'KB_IN_USE', message: 'in use' });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });
  await client.knowledgeBases.remove('kb-1');
  assert.equal(requests[0]?.method, 'DELETE');
  await assert.rejects(client.knowledgeBases.remove('kb-1'), (error: unknown) => error instanceof Error && error.message.includes('in use'));
});
