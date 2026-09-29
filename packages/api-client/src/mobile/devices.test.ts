import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileDeviceRemote } from './devices.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

function recorder(responder: (input: ClientRequest) => unknown): { seen: ClientRequest[]; request: (input: ClientRequest) => Promise<unknown> } {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 POST /api/v1/mobile/devices/:id/registration-intent 响应（internal/handler/mobile_device.go:159）。 */
const INTENT_WIRE = { success: true, data: { registration_intent: 'aW50ZW50.WQ', scope_generation: 2 } };
/** 真实 PUT /api/v1/mobile/devices/:id 响应（internal/handler/mobile_device.go:237-240）。 */
const REGISTER_WIRE = { success: true, data: { device_id: 'device-1', environment: 'production', platform: 'ios', scope_generation: 2, revision: 1 } };
/** 真实 GET /api/v1/mobile/devices 响应行（internal/application/repository/mobile_device.go:29-44：TokenCiphertext/TokenHash/TenantID/SpaceID 均 json:"-"，不上 wire）。 */
const LIST_WIRE = {
  success: true,
  data: [
    { device_id: 'device-1', owner_id: 'user-1', environment: 'production', platform: 'ios', revision: 3, scope_generation: 5, revoked_at: null, last_seen_at: '2026-09-24T00:00:00Z' },
    { device_id: 'device-2', owner_id: 'user-1', environment: 'production', platform: 'android', revision: 1, scope_generation: 1 },
  ],
};

test('issueIntent posts the registration-intent endpoint and unwraps the envelope', async () => {
  const spy = recorder(() => INTENT_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const intent = await remote.issueIntent('device-1');

  assert.deepEqual(intent, { registrationIntent: 'aW50ZW50.WQ', scopeGeneration: 2 });
  assert.equal(spy.seen[0]!.method, 'POST');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1/registration-intent');
});

test('register puts the sealed wire body and maps the returned record', async () => {
  const spy = recorder(() => REGISTER_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const record = await remote.register({ deviceId: 'device-1', token: 'push-token', platform: 'ios', registrationIntent: 'aW50ZW50.WQ' });

  assert.deepEqual(record, { appId: 'official', deviceId: 'device-1', platform: 'ios', environment: 'production', revision: 1, scopeGeneration: 2 });
  assert.equal(spy.seen[0]!.method, 'PUT');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1');
  assert.deepEqual(spy.seen[0]!.body, { token: 'push-token', platform: 'ios', registration_intent: 'aW50ZW50.WQ' });
});

test('revoke deletes with an optional revision query and treats 204 as success', async () => {
  const spy = recorder(() => undefined);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  await remote.revoke({ deviceId: 'device-1' });
  await remote.revoke({ deviceId: 'device-1', revision: 3 });

  assert.equal(spy.seen[0]!.method, 'DELETE');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1');
  assert.equal(spy.seen[1]!.path, '/api/v1/mobile/devices/device-1?revision=3');
});

test('device ids are URL-encoded into the path (injection-safe) and revision must be a positive integer', async () => {
  const spy = recorder(() => INTENT_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  await remote.issueIntent('a/b c?x=1');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/a%2Fb%20c%3Fx%3D1/registration-intent');

  await assert.rejects(remote.revoke({ deviceId: 'd', revision: 0 }), /positive integer/);
  await assert.rejects(remote.revoke({ deviceId: 'd', revision: 1.5 }), /positive integer/);
});

test('list maps rows and never surfaces token ciphertext columns (Review Focus #3)', async () => {
  const spy = recorder(() => LIST_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.list();

  assert.deepEqual(rows, [
    { appId: 'official', deviceId: 'device-1', platform: 'ios', environment: 'production', revision: 3, scopeGeneration: 5, lastSeenAt: '2026-09-24T00:00:00Z' },
    { appId: 'official', deviceId: 'device-2', platform: 'android', environment: 'production', revision: 1, scopeGeneration: 1 },
  ]);
  const serialized = JSON.stringify(rows);
  for (const forbidden of ['token_ciphertext', 'token_hash', 'tenant_id', 'owner_id']) {
    assert.equal(serialized.includes(forbidden), false, `semantic rows must not carry ${forbidden}`);
  }
});

test('malformed envelopes and origins fail fast', async () => {
  const failing = recorder(() => ({ success: false }));
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.issueIntent('device-1'), /success/);
  await assert.rejects(remote.list(), /success/);

  assert.throws(() => createMobileDeviceRemote({ origin: 'http://weknora.example.test', request: failing.request }), /HTTPS/);
  assert.throws(() => createMobileDeviceRemote({ origin: 'https://weknora.example.test/path', request: failing.request }), /path/);
});
