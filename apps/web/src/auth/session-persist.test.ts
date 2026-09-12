import { test } from 'node:test';
import assert from 'node:assert/strict';
import { computeAuthLanding } from './session-persist.ts';

test('response without tenant lands on workspace onboarding (Vue Login.vue:599)', () => {
  const landing = computeAuthLanding({ token: 't', refreshToken: 'r', user: { id: 'u1', tenant_id: null } });
  assert.equal(landing.target, '/onboarding/workspace');
  assert.equal(landing.activeTenantId, null);
  assert.equal(landing.tenantOverride, null);
});

test('active tenant equal to home does not create an override and honours next', () => {
  const session = { token: 't', refreshToken: 'r', user: { id: 'u1', tenant_id: 7 }, tenant: { id: 7, name: 'home' } };
  const landing = computeAuthLanding(session, '/platform/settings');
  assert.equal(landing.target, '/platform/settings');
  assert.equal(landing.tenantOverride, null);
  assert.equal(landing.activeTenantId, '7');
});

test('active tenant differing from home seeds the X-Tenant-ID override', () => {
  const session = { token: 't', refreshToken: 'r', user: { id: 'u1', tenant_id: 7 }, tenant: { id: 9, name: 'remembered' } };
  const landing = computeAuthLanding(session);
  assert.deepEqual(landing.tenantOverride, { tenantId: '9' });
  assert.equal(landing.homeTenantId, '7');
  assert.equal(landing.activeTenantId, '9');
});

test('unsafe next paths fall back to the knowledge-base list', () => {
  const session = { token: 't', refreshToken: 'r', user: { id: 'u1', tenant_id: 7 }, tenant: { id: 7 } };
  assert.equal(computeAuthLanding(session, '//evil.example').target, '/platform/knowledge-bases');
  assert.equal(computeAuthLanding(session, '/platform/chat/1').target, '/platform/chat/1');
});
