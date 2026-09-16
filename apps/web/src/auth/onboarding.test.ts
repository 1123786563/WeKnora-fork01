import { test } from 'node:test';
import assert from 'node:assert/strict';
import { onboardingView, validateCreateTenant } from './onboarding.ts';

test('a valid tenant redirects out of the boundary (Vue watch hasValidTenant)', () => {
  assert.deepEqual(onboardingView({ authenticated: true, hasTenant: true, canCreateTenant: true, pendingInvitationCount: 0 }), { kind: 'redirect' });
});

test('unauthenticated policy refresh surfaces the retry error state', () => {
  assert.equal(onboardingView({ authenticated: false, hasTenant: false, canCreateTenant: false, pendingInvitationCount: 0 }).kind, 'policy-error');
  assert.deepEqual(onboardingView(null), { kind: 'loading-policy' });
});

test('invite-only users see invitations entry without a create action', () => {
  const view = onboardingView({ authenticated: true, hasTenant: false, canCreateTenant: false, pendingInvitationCount: 3 });
  assert.deepEqual(view, { kind: 'ready', canCreateTenant: false, inviteOnly: true, pendingInvitationCount: 3 });
});

test('create-tenant validation mirrors backend binding rules (name 1-128, description 512)', () => {
  assert.deepEqual(validateCreateTenant({ name: '   ', description: '' }), { name: ['tenant.create.nameRequired'] });
  assert.deepEqual(validateCreateTenant({ name: 'x'.repeat(129), description: '' }), { name: ['tenant.create.nameTooLong'] });
  assert.deepEqual(validateCreateTenant({ name: 'ws', description: 'd'.repeat(513) }), { description: ['tenant.create.descriptionTooLong'] });
  assert.deepEqual(validateCreateTenant({ name: '工作区', description: 'ok' }), {});
});
