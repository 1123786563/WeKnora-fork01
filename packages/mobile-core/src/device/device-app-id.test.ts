import assert from 'node:assert/strict';
import test from 'node:test';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createDeviceRegistry, DeviceError } from './device-registry.ts';
import { createScenarioDeviceRemote } from './in-memory-device-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

/** 包一层捕获 issueIntent/register 的 app_id 入参（wire 透传观测）。 */
function capturingScenario() {
  const scenario = createScenarioDeviceRemote();
  const capturedApps: string[] = [];
  let lastRegisterInput: { appId?: string } | undefined;
  const remote = {
    issueIntent(deviceId: string, appId?: string) {
      capturedApps.push(appId ?? 'official');
      return scenario.remote.issueIntent(deviceId, appId);
    },
    register(input: { deviceId: string; token: string; platform: string; registrationIntent: string; appId?: string }) {
      lastRegisterInput = input;
      return scenario.remote.register(input);
    },
    revoke: scenario.remote.revoke.bind(scenario.remote),
    list: scenario.remote.list.bind(scenario.remote),
  };
  const leaseRef: { lease?: ReturnType<typeof leased>['lease'] } = { lease: leased().lease };
  const registry = createDeviceRegistry({ remote, lease: () => leaseRef.lease });
  return { scenario, registry, capturedApps: () => [...capturedApps], lastRegisterInput: () => lastRegisterInput };
}

test('register carries the app id on both wire calls and defaults to official', async () => {
  const { registry, capturedApps, lastRegisterInput } = capturingScenario();
  const record = await registry.register({ deviceId: 'd', token: 't', platform: 'ios' });
  assert.equal(record.appId, 'official');
  const enterprise = await registry.register({ deviceId: 'd2', token: 't2', platform: 'ios', appId: 'enterprise:acme' });
  assert.equal(enterprise.appId, 'enterprise:acme');
  assert.deepEqual(capturedApps(), ['official', 'enterprise:acme'], 'issueIntent receives the app id');
  assert.equal(lastRegisterInput()?.appId, 'enterprise:acme', 'register receives the app id');
});

test('invalid app ids fail closed before any wire call', async () => {
  const { registry, capturedApps } = capturingScenario();
  for (const appId of ['Official', 'enterprise', 'enterprise:', 'enterprise:Acme', 'enterprise:a_b', 'enterprise:-ab', `enterprise:${'x'.repeat(33)}`, 'ios']) {
    await assert.rejects(
      registry.register({ deviceId: 'd', token: 't', platform: 'ios', appId }),
      (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_INVALID_INPUT',
      `${JSON.stringify(appId)} must be rejected client-side`,
    );
  }
  assert.deepEqual(capturedApps(), [], 'validation happens before any wire call');
});

test('the same device id under two apps keeps two independent records', async () => {
  const { scenario, registry } = ((): ReturnType<typeof capturingScenario> => {
    const inner = createScenarioDeviceRemote();
    const leaseRef: { lease?: ReturnType<typeof leased>['lease'] } = { lease: leased().lease };
    return { scenario: inner, registry: createDeviceRegistry({ remote: inner.remote, lease: () => leaseRef.lease }), capturedApps: () => [], lastRegisterInput: () => undefined };
  })();
  const official = await registry.register({ deviceId: 'shared', token: 't1', platform: 'ios' });
  const enterprise = await registry.register({ deviceId: 'shared', token: 't2', platform: 'ios', appId: 'enterprise:acme' });
  assert.equal(official.revision, 1);
  assert.equal(enterprise.revision, 1, 'the enterprise register must not take over the official record');
  assert.deepEqual(
    scenario.snapshot().active.map((row) => `${row.appId}:${row.deviceId}`).sort(),
    ['enterprise:acme:shared', 'official:shared'],
    'two independent records keyed by app',
  );
});
