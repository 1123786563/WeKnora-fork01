import assert from 'node:assert/strict';
import test from 'node:test';
import { createMobilePersonalNodeRuntime } from './personalNodeRuntime.ts';

test('mobile app runtime consumes SecureStore credentials and clears them on revoke', async () => {
  const values = new Map([['node', 'mobile-bearer']]);
  let revoked = 0;
  const connector = await createMobilePersonalNodeRuntime({
    secureStore: {
      async getItemAsync(key) { return values.get(key) ?? null; },
      async deleteItemAsync(key) { values.delete(key); },
    },
    credentialKey: 'node',
    registrationClient: {
      async createChallenge() { throw new Error('unused'); },
      async complete() { throw new Error('unused'); },
      async revoke() { revoked += 1; },
    },
    transport: {
      baseURL: 'https://paseo.example.test',
      allowedOrigins: ['https://paseo.example.test'],
      fetchImpl: async () => new Response(null, { status: 204 }),
    },
  });
  const registration = { id: 'node-1', runtime_id: 'r', external_target_id: 'x', public_key: 'pk', credential_version: 1, state: 'active' as const };
  (connector as any).registration = registration;
  await connector.revoke();
  assert.equal(revoked, 1);
  assert.equal(values.has('node'), false);
});

test('mobile app runtime rejects an absent SecureStore credential before composition', async () => {
  await assert.rejects(() => createMobilePersonalNodeRuntime({
    secureStore: { async getItemAsync() { return null; }, async deleteItemAsync() {} },
    registrationClient: {} as any,
    transport: { baseURL: 'https://paseo.example.test', allowedOrigins: ['https://paseo.example.test'] },
  }), /PASEO_CREDENTIAL_MISSING/);
});
