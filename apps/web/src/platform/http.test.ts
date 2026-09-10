import assert from 'node:assert/strict';
import test from 'node:test';
import type { FetchLike } from '@weknora/api-client';
import { createBrowserTransport } from './http.ts';

test('injects scoped auth headers without changing the shared transport', async () => {
  let seen: { url: string; headers: Record<string, string> } | undefined;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    tenantId: 'tenant-7',
    locale: 'zh-CN',
    requestId: () => 'request-1',
    fetcher: (async (url, init) => {
      seen = { url, headers: init?.headers ?? {} };
      return { status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: true, data: [] }), text: async () => '' };
    }) satisfies FetchLike,
  });

  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: { accept: 'application/json' } });
  assert.equal(seen?.headers.authorization, 'Bearer access-1');
  assert.equal(seen?.headers['x-tenant-id'], 'tenant-7');
  assert.equal(seen?.headers['accept-language'], 'zh-CN');
  assert.equal(seen?.headers['x-request-id'], 'request-1');
});

test('isolates embed headers from bearer tenant context', async () => {
  const seen: Record<string, string>[] = [];
  for (const credential of [{ kind: 'embed', token: 'visitor-token', sessionSig: 'sig-1', visitorId: 'visitor-1' } as const, { kind: 'anonymous' } as const]) {
    const transport = createBrowserTransport({ credential, tenantId: 'must-not-leak', fetcher: (async (_url, init) => {
      seen.push(init?.headers ?? {});
      return { status: 204, headers: new Headers(), json: async () => undefined, text: async () => '' };
    }) satisfies FetchLike });
    await transport.send({ method: 'GET', url: 'https://api.test/files/x', headers: {} });
  }
  assert.equal(seen[0]?.authorization, 'Embed visitor-token');
  assert.equal(seen[0]?.['x-tenant-id'], undefined);
  assert.equal(seen[0]?.['x-embed-session'], 'sig-1');
  assert.equal(seen[0]?.['x-embed-visitor'], 'visitor-1');
  assert.equal(seen[1]?.authorization, undefined);
});
