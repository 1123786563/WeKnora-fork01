import assert from 'node:assert/strict';
import test from 'node:test';
import {
  getOnboardingPresentation,
  validateLoginForm,
  validateRegisterForm,
} from './auth-state.ts';
import { createAuthApi, persistLogin } from './api.ts';
import type { ClientRequest } from '@weknora/api-client';

test('login validation mirrors the Vue contract for required, email, and password length', () => {
  assert.deepEqual(validateLoginForm({ email: '', password: '' }), {
    email: 'auth.emailRequired',
    password: 'auth.passwordRequired',
  });
  assert.deepEqual(validateLoginForm({ email: 'broken', password: 'short' }), {
    email: 'auth.emailInvalid',
    password: 'auth.passwordMinLength',
  });
  assert.deepEqual(validateLoginForm({ email: 'user@example.com', password: 'correct-password' }), {});
});

test('registration validation covers Vue username, password, and confirmation rules', () => {
  assert.deepEqual(validateRegisterForm({ username: 'a', email: 'bad', password: 'short', confirmPassword: 'no' }), {
    username: 'auth.usernameMinLength',
    email: 'auth.emailInvalid',
    password: 'auth.passwordMinLength',
    confirmPassword: 'auth.passwordMismatch',
  });
  assert.deepEqual(validateRegisterForm({ username: 'valid_name', email: 'u@example.com', password: 'correct-password', confirmPassword: 'correct-password' }), {});
  assert.deepEqual(validateRegisterForm({ username: '有 空格', email: 'u@example.com', password: 'correct-password', confirmPassword: 'correct-password' }), {
    username: 'auth.usernameInvalid',
  });
});

test('onboarding presentation distinguishes loading, failed, create permission, and invitation-only', () => {
  assert.deepEqual(getOnboardingPresentation({ status: 'loading', canCreateTenant: false, pendingInvitationCount: 0 }), { kind: 'loading' });
  assert.deepEqual(getOnboardingPresentation({ status: 'error', canCreateTenant: false, pendingInvitationCount: 0 }), { kind: 'error' });
  assert.deepEqual(getOnboardingPresentation({ status: 'ready', canCreateTenant: true, pendingInvitationCount: 2 }), { kind: 'create-and-invite', pendingInvitationCount: 2 });
  assert.deepEqual(getOnboardingPresentation({ status: 'ready', canCreateTenant: false, pendingInvitationCount: 1 }), { kind: 'invite-only', pendingInvitationCount: 1 });
});

test('auth adapter preserves the Vue endpoint paths and login persistence contract', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAuthApi({ request: async (input: ClientRequest) => { requests.push(input); return input.path === '/api/v1/auth/login' ? { success: true, token: 'access', refresh_token: 'refresh', user: { id: 'u-1' }, tenant: { id: 7 } } : { success: true, registration_mode: 'invite_only', complex_password_enabled: true }; } } as never);
  const session = await api.login('u@example.com', 'password');
  assert.equal(session.tenantId, '7');
  assert.deepEqual(requests[0], { method: 'POST', path: '/api/v1/auth/login', body: { email: 'u@example.com', password: 'password' } });
  const values = new Map<string, string>();
  persistLogin(session, { setItem: (key, value) => { values.set(key, value); }, removeItem: (key) => { values.delete(key); } });
  assert.equal(values.get('weknora_token'), 'access');
  assert.equal(values.get('weknora_refresh_token'), 'refresh');
  assert.equal(values.get('weknora_selected_tenant_id'), '7');
});
