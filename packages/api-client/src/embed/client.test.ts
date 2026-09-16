import assert from 'node:assert/strict';
import test from 'node:test';
import { createEmbedClient } from './client.ts';

test('isolated embed client sends channel credentials without bearer or tenant context', async () => {
  const requests: Array<{ url: string; headers: Record<string, string> }> = [];
  const client = createEmbedClient({
    baseURL: 'https://embed.example.test',
    transport: {
      send: async (request) => {
        requests.push({ url: request.url, headers: request.headers });
        return { status: 200, headers: {}, body: { success: true, data: { channel_id: 'c-1', agent_id: 'a-1' } } };
      },
    },
  });

  await client.embed.public.config('c-1', 'ems-short');
  assert.deepEqual(requests, [{
    url: 'https://embed.example.test/api/v1/embed/c-1/config',
    headers: { accept: 'application/json', Authorization: 'Embed ems-short' },
  }]);
});
