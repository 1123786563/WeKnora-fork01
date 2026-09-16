import assert from 'node:assert/strict';
import test from 'node:test';

import { createProductAuth, createProductAuthSession, parseLogin } from './login.ts';
import type { CredentialAdapter } from '../ports.ts';

test('normalizes a successful product login and preserves tenantless users', () => {
  assert.deepEqual(
    parseLogin({ success: true, token: 'access', refresh_token: 'refresh', user: { id: 'user-1' }, active_tenant: null }),
    { credential: { kind: 'bearer', accessToken: 'access', refreshToken: 'refresh' }, userId: 'user-1', tenantId: null },
  );
});

test('accepts legacy tenant field and numeric tenant ids', () => {
  assert.deepEqual(
    parseLogin({ success: true, token: 'access', user: { id: 'user-1' }, tenant: { id: 42 } }),
    { credential: { kind: 'bearer', accessToken: 'access' }, userId: 'user-1', tenantId: '42' },
  );
});

test('rejects failed and malformed login responses', () => {
  for (const value of [
    { success: false, token: 'access' },
    { success: true, token: '', user: { id: 'user-1' } },
    { success: true, token: 'access', user: { id: '' } },
    { success: true, token: 'access', refresh_token: 42, user: { id: 'user-1' } },
    { success: true, token: 'access', user: { id: 'user-1' }, active_tenant: { id: 0 } },
    { success: true, token: 'access', user: { id: 'user-1' }, active_tenant: 'tenant' },
  ]) {
    assert.throws(() => parseLogin(value), /INVALID_LOGIN|INVALID_TENANT_ID/);
  }
});

test('concurrent unauthorized requests share one refresh and retry once with rotated access token', async () => {
  let refreshCalls = 0;
  let protectedCalls = 0;
  let current = { kind: 'bearer' as const, accessToken: 'old', refreshToken: 'refresh' };
  const credentials: CredentialAdapter = {
    read: async () => current,
    write: async (value) => { current = value as typeof current; },
    clear: async () => { current = { kind: 'anonymous' as const } as typeof current; },
  };
  const session = createProductAuthSession({
    baseURL: 'https://weknora.example.test', credentials,
    transport: { send: async (request) => {
      if (request.url.endsWith('/refresh')) {
        refreshCalls += 1;
        return { status: 200, headers: {}, body: { success: true, access_token: 'new', refresh_token: 'rotated' } };
      }
      protectedCalls += 1;
      return request.headers.authorization === 'Bearer new'
        ? { status: 200, headers: {}, body: { ok: true } }
        : { status: 401, headers: {}, body: { success: false } };
    } },
  });
  const results = await Promise.all([session.request('/api/v1/protected'), session.request('/api/v1/protected')]);
  assert.deepEqual(results, [{ ok: true }, { ok: true }]);
  assert.equal(refreshCalls, 1);
  assert.equal(protectedCalls, 4);
});

test('product auth calls the product login and refresh endpoints with product wire fields', async () => {
  const requests: Array<{ url: string; body: unknown }> = [];
  const auth = createProductAuth({
    baseURL: 'https://weknora.example.test/',
    transport: { send: async (request) => {
      requests.push({ url: request.url, body: request.body });
      return { status: 200, headers: {}, body: request.url.endsWith('/login')
        ? { success: true, token: 'a', refresh_token: 'r', user: { id: 'u' } }
        : { success: true, access_token: 'b', refresh_token: 's' } };
    } },
  });
  await auth.login('a@example.test', 'secret');
  await auth.refresh('r');
  assert.deepEqual(requests, [
    { url: 'https://weknora.example.test/api/v1/auth/login', body: { email: 'a@example.test', password: 'secret' } },
    { url: 'https://weknora.example.test/api/v1/auth/refresh', body: { refreshToken: 'r' } },
  ]);
});

test('authenticated transport injects bearer and refreshes JSON, binary, and stream paths', async () => {
  async function freshSession() {
    let access = 'old';
    let refreshes = 0;
    const credentials: CredentialAdapter = {
      read: async () => ({ kind: 'bearer', accessToken: access, refreshToken: 'refresh' }),
      write: async (value) => { access = (value as { accessToken: string }).accessToken; },
      clear: async () => undefined,
    };
    const transport = {
      send: async (request: any) => request.headers.authorization === 'Bearer new' ? { status: 200, headers: {}, body: 'json' } : { status: 401, headers: {}, body: null },
      sendBinary: async (request: any) => request.headers.authorization === 'Bearer new' ? { status: 200, headers: {}, body: new Uint8Array([1]) } : { status: 401, headers: {}, body: null },
      sendStream: async (request: any) => request.headers.authorization === 'Bearer new' ? { status: 200, headers: {}, chunks: (async function* () { yield 'ok'; })() } : { status: 401, headers: {}, chunks: (async function* () {})() },
    };
    const session = createProductAuthSession({ baseURL: 'https://api.example', credentials, transport: {
      ...transport,
      send: async (request: any) => request.url.endsWith('/refresh')
        ? (++refreshes, { status: 200, headers: {}, body: { access_token: 'new', refresh_token: 'rotated' } })
        : transport.send(request),
    } });
    return { session, refreshes: () => refreshes };
  }
  const json = await freshSession();
  assert.equal((await json.session.transport.send({ method: 'GET', url: 'https://api.example/data', headers: {} })).status, 200);
  assert.equal(json.refreshes(), 1, 'JSON independently refreshes after 401');
  const binary = await freshSession();
  assert.equal((await binary.session.transport.sendBinary!({ method: 'GET', url: 'https://api.example/file', headers: {} })).status, 200);
  assert.equal(binary.refreshes(), 1, 'binary independently refreshes after 401');
  const stream = await freshSession();
  assert.equal((await stream.session.transport.sendStream!({ method: 'GET', url: 'https://api.example/events', headers: {}, signal: new AbortController().signal })).status, 200);
  assert.equal(stream.refreshes(), 1, 'stream independently refreshes after 401');
});
