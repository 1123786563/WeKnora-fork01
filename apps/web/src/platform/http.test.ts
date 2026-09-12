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

test('reads the current tenant for every request after a scope switch', async () => {
  let tenantId: string | null = 'tenant-a';
  const seen: Array<string | undefined> = [];
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    tenantId: () => tenantId,
    fetcher: (async (_url, init) => {
      seen.push(init?.headers?.['x-tenant-id']);
      return { status: 204, headers: new Headers(), json: async () => undefined, text: async () => '' };
    }) satisfies FetchLike,
  });

  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  tenantId = 'tenant-b';
  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });

  assert.deepEqual(seen, ['tenant-a', 'tenant-b']);
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

test('forwards streaming requests with the same scoped headers', async () => {
  let seen: Record<string, string> | undefined;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'stream-access' }, tenantId: 'tenant-stream',
    fetcher: (async (_url, init) => {
      seen = init?.headers;
      return { status: 200, headers: new Headers(), json: async () => ({}), text: async () => 'data: {}\n\n' };
    }) satisfies FetchLike,
  });
  const result = await transport.sendStream!({ method: 'POST', url: 'https://api.test/chat', headers: { accept: 'text/event-stream' }, body: { query: 'x' } });
  const chunks: string[] = [];
  for await (const chunk of result.chunks) chunks.push(chunk);
  assert.equal(seen?.authorization, 'Bearer stream-access');
  assert.equal(seen?.['x-tenant-id'], 'tenant-stream');
  assert.deepEqual(chunks, ['data: {}\n\n']);
});

test('forwards authenticated binary requests and retries them after bearer refresh', async () => {
  let accessToken = 'expired-binary-access';
  let refreshCalls = 0;
  const requests: string[] = [];
  const transport = createBrowserTransport({
    credential: () => ({ kind: 'bearer', accessToken, refreshToken: 'refresh-binary' }),
    tenantId: 'tenant-binary',
    fetcher: (async (_url, init) => {
      const authorization = init?.headers?.authorization ?? '';
      requests.push(authorization);
      if (authorization === 'Bearer expired-binary-access') {
        return {
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ success: false }),
          text: async () => '',
        };
      }
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/pdf', 'content-disposition': 'inline' }),
        json: async () => ({ success: false }),
        text: async () => 'unexpected text read',
        blob: async () => new Blob(['private pdf'], { type: 'application/pdf' }),
      };
    }) satisfies FetchLike,
    refresh: async () => {
      refreshCalls += 1;
      accessToken = 'fresh-binary-access';
    },
  });

  const result = await transport.sendBinary!({
    method: 'GET',
    url: 'https://api.test/api/v1/knowledge/doc-1/preview',
    headers: {},
  });

  assert.equal(result.status, 200);
  assert.equal(result.headers['content-disposition'], 'inline');
  assert.equal(await (result.body as Blob).text(), 'private pdf');
  assert.equal(refreshCalls, 1);
  assert.deepEqual(requests, ['Bearer expired-binary-access', 'Bearer fresh-binary-access']);
});

test('refreshes once and retries concurrent bearer requests after 401', async () => {
  let accessToken = 'expired-access';
  let refreshCalls = 0;
  let releaseRefresh!: () => void;
  const refreshGate = new Promise<void>((resolve) => { releaseRefresh = resolve; });
  const requests: string[] = [];
  const transport = createBrowserTransport({
    credential: () => ({ kind: 'bearer', accessToken, refreshToken: 'refresh-1' }),
    fetcher: (async (_url, init) => {
      const authorization = init?.headers?.authorization ?? '';
      requests.push(authorization);
      if (authorization === 'Bearer expired-access') {
        return { status: 401, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: false }), text: async () => '' };
      }
      return { status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: true, data: [] }), text: async () => '' };
    }) satisfies FetchLike,
    shouldRefresh: (request) => request.url.endsWith('/knowledge-bases'),
    refresh: async () => {
      refreshCalls += 1;
      await refreshGate;
      accessToken = 'fresh-access';
    },
  });

  const first = transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  const second = transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(refreshCalls, 1, 'concurrent 401 responses share one refresh operation');
  releaseRefresh();

  assert.deepEqual((await Promise.all([first, second])).map((result) => result.status), [200, 200]);
  assert.deepEqual(requests, ['Bearer expired-access', 'Bearer expired-access', 'Bearer fresh-access', 'Bearer fresh-access']);
});

test('does not replay a JSON write after 401', async () => {
  let refreshCalls = 0;
  let requests = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'expired-write', refreshToken: 'refresh-write' },
    fetcher: (async () => {
      requests += 1;
      return { status: 401, headers: new Headers(), json: async () => ({ success: false }), text: async () => '' };
    }) satisfies FetchLike,
    refresh: async () => { refreshCalls += 1; },
  });

  const result = await transport.send({ method: 'POST', url: 'https://api.test/api/v1/knowledge-bases', headers: {}, body: { name: 'must-not-duplicate' } });

  assert.equal(result.status, 401);
  assert.equal(requests, 1);
  assert.equal(refreshCalls, 0);
});

test('does not refresh 403 responses or retry an unauthorized response twice', async () => {
  let refreshCalls = 0;
  let requests = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1', refreshToken: 'refresh-1' },
    fetcher: (async () => {
      requests += 1;
      return { status: requests === 1 ? 403 : 401, headers: new Headers(), json: async () => ({}), text: async () => '' };
    }) satisfies FetchLike,
    shouldRefresh: () => true,
    refresh: async () => { refreshCalls += 1; },
  });

  assert.equal((await transport.send({ method: 'GET', url: 'https://api.test/forbidden', headers: {} })).status, 403);
  assert.equal((await transport.send({ method: 'GET', url: 'https://api.test/unauthorized', headers: {} })).status, 401);
  assert.equal(refreshCalls, 1);
  assert.equal(requests, 3);
});

test('keeps refresh endpoint failures from recursively refreshing', async () => {
  let refreshCalls = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1', refreshToken: 'refresh-1' },
    fetcher: (async () => ({ status: 401, headers: new Headers(), json: async () => ({}), text: async () => '' })) satisfies FetchLike,
    shouldRefresh: (request) => !request.url.endsWith('/auth/refresh'),
    refresh: async () => { refreshCalls += 1; },
  });

  assert.equal((await transport.send({ method: 'POST', url: 'https://api.test/api/v1/auth/refresh', headers: {} })).status, 401);
  assert.equal(refreshCalls, 0);
});

test('does not refresh Embed stream responses', async () => {
  let refreshCalls = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'embed', token: 'embed-token' },
    fetcher: (async () => ({ status: 401, headers: new Headers(), json: async () => ({}), text: async () => '', body: null })) satisfies FetchLike,
    shouldRefresh: () => true,
    refresh: async () => { refreshCalls += 1; },
  });

  const result = await transport.sendStream!({ method: 'GET', url: 'https://api.test/embed/chat', headers: {} });
  assert.equal(result.status, 401);
  assert.equal(refreshCalls, 0);
});
