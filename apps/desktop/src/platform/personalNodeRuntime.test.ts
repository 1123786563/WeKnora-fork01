import assert from 'node:assert/strict';
import test from 'node:test';
import { createDesktopPersonalNodeRuntime } from './runtime.ts';

test('desktop app runtime consumes the Wails credential bridge and clears it on revoke', async () => {
  const values = new Map([['node', 'desktop-bearer']]);
  let revoked = 0;
  const connector = createDesktopPersonalNodeRuntime({
    credentialKey: 'node',
    credentialBridge: {
      readCredential: (key) => values.get(key) ?? null,
      removeCredential: (key) => { values.delete(key); },
    },
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
  (connector as any).registration = { id: 'node-1', runtime_id: 'r', external_target_id: 'x', public_key: 'pk', credential_version: 1, state: 'active' };
  await connector.revoke();
  assert.equal(revoked, 1);
  assert.equal(values.has('node'), false);
});

test('desktop app runtime fails closed without a Wails credential', () => {
  assert.throws(() => createDesktopPersonalNodeRuntime({
    credentialBridge: { readCredential: () => null },
    registrationClient: {} as any,
    transport: { baseURL: 'https://paseo.example.test', allowedOrigins: ['https://paseo.example.test'] },
  }), /PASEO_CREDENTIAL_MISSING/);
});
