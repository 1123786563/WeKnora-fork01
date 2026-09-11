import assert from 'node:assert/strict';
import test from 'node:test';
import type { Credential } from '@weknora/api-client';
import { resolveMobileApiBaseUrl } from './transport.ts';
import { createMobileTransport } from './transport.ts';

function jsonResponse(status: number, body: unknown) {
  return {
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

test('requires an explicit HTTP(S) mobile server address', () => {
  assert.equal(resolveMobileApiBaseUrl('https://weknora.example.test/'), 'https://weknora.example.test');
  assert.equal(resolveMobileApiBaseUrl('http://10.0.2.2:8080/api/v1'), 'http://10.0.2.2:8080/api/v1');
  assert.equal(resolveMobileApiBaseUrl('http://localhost:8080'), 'http://localhost:8080');
  assert.equal(resolveMobileApiBaseUrl('file:///tmp/weknora'), '');
  assert.equal(resolveMobileApiBaseUrl(''), '');
});

test('refreshes a bearer once before retrying an idempotent read after 401', async () => {
  let credential: Credential = { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' };
  let refreshes = 0;
  const authorizations: string[] = [];
  const transport = createMobileTransport({
    credential: () => credential,
    refresh: async () => {
      refreshes += 1;
      credential = { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' };
      return credential;
    },
    fetcher: async (_input, init) => {
      authorizations.push((init?.headers as Record<string, string> | undefined)?.authorization ?? '');
      return jsonResponse(authorizations.length === 1 ? 401 : 200, { success: true, data: [] });
    },
  });

  const result = await transport.send({ method: 'GET', url: 'https://api.example.test/api/v1/knowledge-bases', headers: {} });

  assert.equal(result.status, 200);
  assert.equal(refreshes, 1);
  assert.deepEqual(authorizations, ['Bearer old-access', 'Bearer new-access']);
});

test('reuses a credential rotated by another request without refreshing again', async () => {
  let credential: Credential = { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' };
  let refreshes = 0;
  let calls = 0;
  const authorizations: string[] = [];
  const transport = createMobileTransport({
    credential: () => credential,
    refresh: async () => {
      refreshes += 1;
      throw new Error('the already-rotated credential must be reused');
    },
    fetcher: async (_input, init) => {
      calls += 1;
      authorizations.push((init?.headers as Record<string, string> | undefined)?.authorization ?? '');
      if (calls === 1) {
        credential = { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' };
        return jsonResponse(401, { success: false, message: 'expired' });
      }
      return jsonResponse(200, { success: true, data: [] });
    },
  });

  const result = await transport.send({ method: 'GET', url: 'https://api.example.test/api/v1/knowledge-bases', headers: {} });

  assert.equal(result.status, 200);
  assert.equal(refreshes, 0);
  assert.deepEqual(authorizations, ['Bearer old-access', 'Bearer new-access']);
});

test('does not replay a write after 401', async () => {
  let refreshes = 0;
  let calls = 0;
  const transport = createMobileTransport({
    credential: () => ({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' }),
    refresh: async () => {
      refreshes += 1;
      return { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' };
    },
    fetcher: async () => {
      calls += 1;
      return jsonResponse(401, { success: false, message: 'expired' });
    },
  });

  const result = await transport.send({ method: 'POST', url: 'https://api.example.test/api/v1/knowledge-bases', headers: {}, body: { name: 'no replay' } });

  assert.equal(result.status, 401);
  assert.equal(calls, 1);
  assert.equal(refreshes, 0);
});

test('does not attach stale session context while a transition is active', async () => {
  let refreshes = 0;
  const requests: Record<string, string>[] = [];
  const transport = createMobileTransport({
    credential: () => ({ kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' }),
    tenantId: () => '7',
    isTransitioning: () => true,
    refresh: async () => { refreshes += 1; throw new Error('refresh must be blocked'); },
    fetcher: async (_input, init) => {
      requests.push((init?.headers as Record<string, string> | undefined) ?? {});
      return jsonResponse(401, { success: false, message: 'transitioning' });
    },
  });

  const result = await transport.send({ method: 'GET', url: 'https://api.example.test/api/v1/auth/me', headers: {} });

  assert.equal(result.status, 401);
  assert.equal(refreshes, 0);
  assert.equal(requests[0]?.authorization, undefined);
  assert.equal(requests[0]?.['x-tenant-id'], undefined);
});

test('refreshes once before retrying a bearer SSE handshake after 401', async () => {
  let credential: Credential = { kind: 'bearer', accessToken: 'old-access', refreshToken: 'old-refresh' };
  let refreshes = 0;
  const authorizations: string[] = [];
  const transport = createMobileTransport({
    credential: () => credential,
    refresh: async () => {
      refreshes += 1;
      credential = { kind: 'bearer', accessToken: 'new-access', refreshToken: 'new-refresh' };
      return credential;
    },
    fetcher: async (_input, init) => {
      authorizations.push((init?.headers as Record<string, string> | undefined)?.authorization ?? '');
      return {
        status: authorizations.length === 1 ? 401 : 200,
        headers: new Headers({ 'content-type': 'text/event-stream' }),
        body: null,
        json: async () => ({}),
        text: async () => '',
      };
    },
  });

  const result = await transport.sendStream!({ method: 'POST', url: 'https://api.example.test/api/v1/knowledge-chat/s-1', headers: {}, body: { query: 'hello' } });

  assert.equal(result.status, 200);
  assert.equal(refreshes, 1);
  assert.deepEqual(authorizations, ['Bearer old-access', 'Bearer new-access']);
});
