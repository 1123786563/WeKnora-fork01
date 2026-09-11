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

test('rejects auth/me identities without a stable user id', async () => {
  const auth = createAuthApi(async () => ({ success: true, data: { user: { email: 'user@example.test' }, tenant: null } }));
  await assert.rejects(auth.me(), /auth me user\.id is required/);
});

test('rejects an active auth/me tenant without an id', async () => {
  const auth = createAuthApi(async () => ({ success: true, data: { user: { id: 'user-1' }, tenant: { name: 'Broken' } } }));
  await assert.rejects(auth.me(), /auth me tenant\.id is required/);
});

test('does not treat a business-level logout failure as success', async () => {
  const failed = createAuthApi(async () => ({ success: false, message: 'logout rejected' }));
  await assert.rejects(failed.logout(), /logout rejected/);
  const ok = createAuthApi(async () => ({ success: true }));
  await ok.logout();
});

test('parses public registration, OIDC, Lite setup, and workspace auth endpoints', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const auth = createAuthApi(async (request) => {
    calls.push(request);
    if (request.path === '/api/v1/auth/config') return { success: true, registration_mode: 'invite_only', complex_password_enabled: true };
    if (request.path === '/api/v1/auth/oidc/config') return { success: true, enabled: true, provider_display_name: 'Acme SSO' };
    if (request.path.startsWith('/api/v1/auth/oidc/url')) return { success: true, authorization_url: 'https://idp.test/authorize', state: 'signed-state' };
    if (request.path === '/api/v1/auth/register') return { success: true, user: { id: 'u-1' }, tenant: { id: 7, name: 'Personal' } };
    if (request.path === '/api/v1/auth/auto-setup') return { success: true, token: 'lite-access', refresh_token: 'lite-refresh' };
    if (request.path === '/api/v1/auth/switch-tenant') return { success: true, token: 'tenant-access', refresh_token: 'tenant-refresh', active_tenant: { id: 8, name: 'Target' } };
    if (request.path === '/api/v1/auth/invitations/lookup') return { success: true, data: { tenant_id: 7, tenant_name: 'Personal', role: 'viewer', expires_at: '2030-01-01T00:00:00Z' } };
    if (request.path === '/api/v1/auth/register-by-invite') return { success: true, token: 'invite-access', refresh_token: 'invite-refresh' };
    if (request.path === '/api/v1/auth/validate') return { success: true, valid: true };
    throw new Error(`unexpected path ${request.path}`);
  });

  assert.deepEqual(await auth.registrationConfig(), { registrationMode: 'invite_only', complexPasswordEnabled: true });
  assert.deepEqual(await auth.oidcConfig(), { enabled: true, providerDisplayName: 'Acme SSO' });
  assert.deepEqual(await auth.oidcUrl('https://app.test/callback'), { authorizationUrl: 'https://idp.test/authorize', state: 'signed-state' });
  assert.deepEqual(await auth.register({ username: 'alice', email: 'alice@example.test', password: 'password' }), { user: { id: 'u-1' }, tenant: { id: 7, name: 'Personal' } });
  assert.deepEqual(await auth.autoSetup(), { token: 'lite-access', refreshToken: 'lite-refresh', user: undefined, tenant: undefined, memberships: undefined });
  assert.deepEqual(await auth.switchTenant(8, 'refresh-1'), { token: 'tenant-access', refreshToken: 'tenant-refresh', user: undefined, tenant: { id: 8, name: 'Target' }, memberships: undefined });
  assert.deepEqual(await auth.lookupInvitation('invite-token'), { tenantId: 7, tenantName: 'Personal', role: 'viewer', expiresAt: '2030-01-01T00:00:00Z' });
  assert.deepEqual(await auth.registerByInvite({ token: 'invite-token', email: 'new@example.test', username: 'new', password: 'password' }), { token: 'invite-access', refreshToken: 'invite-refresh', user: undefined, tenant: undefined, memberships: undefined });
  assert.deepEqual(await auth.validate(), { valid: true });
  assert.deepEqual(calls.find((call) => call.path === '/api/v1/auth/switch-tenant')?.body, { tenant_id: 8, refresh_token: 'refresh-1' });
});

test('rejects malformed public auth responses instead of hiding them', async () => {
  const auth = createAuthApi(async (request) => {
    if (request.path === '/api/v1/auth/oidc/url?redirect_uri=https%3A%2F%2Fapp.test%2Fcallback') return { success: true, state: 'missing-url' };
    if (request.path === '/api/v1/auth/invitations/lookup') return { success: true, data: { tenant_id: 9007199254740992, role: 'viewer', expires_at: '2030-01-01T00:00:00Z' } };
    return { success: false, message: 'rejected' };
  });

  await assert.rejects(auth.oidcUrl('https://app.test/callback'), /authorization URL is required/);
  await assert.rejects(auth.lookupInvitation('token'), /tenantId must be a safe integer/);
  await assert.rejects(auth.registrationConfig(), /rejected/);
});

test('keeps mobile OIDC redirect separate and exchanges its one-time code', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const auth = createAuthApi(async (request) => {
    calls.push(request);
    if (request.path.startsWith('/api/v1/auth/oidc/url')) return { success: true, authorization_url: 'https://idp.test/authorize', state: 'mobile-state' };
    if (request.path === '/api/v1/auth/oidc/exchange') return { success: true, token: 'access', refresh_token: 'refresh', tenant: null };
    throw new Error(`unexpected path ${request.path}`);
  });

  assert.deepEqual(await auth.oidcUrl('https://api.test/api/v1/auth/oidc/callback', 'weknora://oidc'), {
    authorizationUrl: 'https://idp.test/authorize', state: 'mobile-state',
  });
  assert.deepEqual(await auth.oidcExchange('provider-code', 'mobile-state'), {
    token: 'access', refreshToken: 'refresh', tenant: null, user: undefined, memberships: undefined,
  });
  assert.equal(calls[0].path, '/api/v1/auth/oidc/url?redirect_uri=https%3A%2F%2Fapi.test%2Fapi%2Fv1%2Fauth%2Foidc%2Fcallback&frontend_redirect_uri=weknora%3A%2F%2Foidc');
  assert.deepEqual(calls[1], {
    method: 'POST', path: '/api/v1/auth/oidc/exchange', body: { code: 'provider-code', state: 'mobile-state' },
  });
});

test('includes the mobile PKCE verifier in the server-side exchange body', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const auth = createAuthApi(async (request) => {
    calls.push(request);
    return { success: true, token: 'access', refresh_token: 'refresh', tenant: null };
  });

  await auth.oidcExchange('provider-code', 'mobile-state', 'verifier-value');

  assert.deepEqual(calls[0], {
    method: 'POST', path: '/api/v1/auth/oidc/exchange',
    body: { code: 'provider-code', state: 'mobile-state', code_verifier: 'verifier-value' },
  });
});
