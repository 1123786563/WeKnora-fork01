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
