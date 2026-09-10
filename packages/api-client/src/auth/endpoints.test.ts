import assert from 'node:assert/strict';
import test from 'node:test';
import { createAuthApi } from './endpoints.ts';

test('parses login and refresh only when the success envelope contains both tokens', async () => {
  const calls: unknown[] = [];
  const auth = createAuthApi(async (request) => {
    calls.push(request);
    if (request.path.endsWith('/login')) return { success: true, token: 'a', refresh_token: 'r', user: { id: 'u-1' } };
    return { success: true, access_token: 'a-2', refresh_token: 'r-2' };
  });
  assert.deepEqual(await auth.login({ email: 'user@example.test', password: 'secret' }), { token: 'a', refreshToken: 'r', user: { id: 'u-1' }, tenant: undefined, memberships: undefined });
  assert.deepEqual(await auth.refresh('r'), { access_token: 'a-2', refresh_token: 'r-2' });
  assert.equal((calls[0] as { method: string }).method, 'POST');
});

test('rejects success false and missing required auth fields', async () => {
  const auth = createAuthApi(async () => ({ success: true, token: 'only-access' }));
  await assert.rejects(auth.login({ email: 'user@example.test', password: 'secret' }), /refresh token is required/);
  const failed = createAuthApi(async () => ({ success: false, message: 'invalid credentials' }));
  await assert.rejects(failed.login({ email: 'user@example.test', password: 'wrong' }), /invalid credentials/);
});

test('requires the structured auth/me payload', async () => {
  const auth = createAuthApi(async () => ({ success: true, data: { user: { id: 'u-1' }, tenant: null } }));
  assert.deepEqual(await auth.me(), { user: { id: 'u-1' }, tenant: null, memberships: undefined, tenant_required: false, capabilities: undefined });
});
