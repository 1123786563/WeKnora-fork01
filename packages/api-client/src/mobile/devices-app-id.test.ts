import assert from 'node:assert/strict';
import test from 'node:test';
import { createMobileDeviceRemote } from './devices.ts';

type Captured = { method: string; path: string; body?: unknown };

function capturingRemote(responses: Array<Record<string, unknown>>): { remote: ReturnType<typeof createMobileDeviceRemote>; calls: Captured[] } {
  const calls: Captured[] = [];
  let index = 0;
  const request = async (input: { method: string; path: string; body?: unknown }): Promise<unknown> => {
    calls.push({ method: input.method, path: input.path, body: input.body });
    const response = responses[Math.min(index, responses.length - 1)];
    index += 1;
    return { success: true, data: response };
  };
  return { remote: createMobileDeviceRemote({ origin: 'https://deployment.example', request }), calls };
}

const intentRow = { registration_intent: 'intent-1', scope_generation: 2 };
const registerRow = { device_id: 'd', platform: 'ios', environment: 'dev', scope_generation: 2, revision: 1, app_id: 'enterprise:acme' };

test('issueIntent and register carry app_id on the wire', async () => {
  const { remote, calls } = capturingRemote([intentRow, registerRow]);
  await remote.issueIntent('d', 'enterprise:acme');
  assert.deepEqual(calls[0], { method: 'POST', path: '/api/v1/mobile/devices/d/registration-intent', body: { app_id: 'enterprise:acme' } });
  await remote.register({ deviceId: 'd', token: 't', platform: 'ios', registrationIntent: 'intent-1', appId: 'enterprise:acme' });
  assert.deepEqual(
    (calls[1]!.body as Record<string, unknown>),
    { token: 't', platform: 'ios', registration_intent: 'intent-1', app_id: 'enterprise:acme' },
  );
});

test('register omits app_id when defaulted and records official from the response', async () => {
  const { remote, calls } = capturingRemote([registerRow]);
  const record = await remote.register({ deviceId: 'd', token: 't', platform: 'ios', registrationIntent: 'intent-1' });
  assert.equal('app_id' in (calls[0]!.body as Record<string, unknown>), false, 'official default stays implicit on the wire (server normalizes)');
  assert.equal(record.appId, 'enterprise:acme');
});

test('list maps app_id and falls back to official for legacy rows', async () => {
  const { remote } = capturingRemote([[{ device_id: 'd', platform: 'ios', environment: 'dev', revision: 1, scope_generation: 1 }, { device_id: 'd2', platform: 'android', environment: 'dev', revision: 1, scope_generation: 1, app_id: 'enterprise:acme' }]]);
  const rows = await remote.list();
  assert.equal(rows[0]!.appId, 'official');
  assert.equal(rows[1]!.appId, 'enterprise:acme');
});
