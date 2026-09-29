import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createDeviceRegistry, DeviceError } from './device-registry.ts';
import { createScenarioDeviceRemote } from './in-memory-device-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

function registryWith(leaseRef: { lease?: ScopeLease }) {
  const scenario = createScenarioDeviceRemote();
  const registry = createDeviceRegistry({ remote: scenario.remote, lease: () => leaseRef.lease });
  return { scenario, registry };
}

test('register performs the two-step intent then bind and returns the server record', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  const record = await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });

  assert.equal(record.deviceId, 'device-1');
  assert.equal(record.platform, 'ios');
  assert.equal(record.revision, 1);
  assert.equal(scenario.snapshot().intentsIssued, 1, 'exactly one registration intent must be issued');
});

test('a stale-intent conflict (409) retries with one fresh intent and then succeeds', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);
  scenario.conflictNextRegisters(1); // 第一次 register 409（模拟并发/过期 intent）

  const record = await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'android' });

  assert.equal(record.revision, 1);
  assert.equal(scenario.snapshot().intentsIssued, 2, 'exactly one re-issued intent after the conflict');
});

test('a second consecutive conflict surfaces DEVICE_CONFLICT instead of retrying forever', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);
  scenario.conflictNextRegisters(2);

  await assert.rejects(
    registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_CONFLICT',
  );
  assert.equal(scenario.snapshot().intentsIssued, 2, 'the retry bound is exactly one extra intent');
});

test('re-registering the same device takes over the token (upsert, revision bumps)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  const second = await registry.register({ deviceId: 'device-1', token: 'push-token-b', platform: 'ios' });

  assert.ok(second.revision > 1, 'token takeover must bump the durable revision');
  assert.deepEqual(scenario.snapshot().active.map((row) => row.deviceId), ['device-1'], 'one active binding, never two');
});

test('concurrent registers are serialized so interleaved epochs cannot corrupt the takeover', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await Promise.all([
    registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' }),
    registry.register({ deviceId: 'device-1', token: 'push-token-b', platform: 'ios' }),
  ]);

  assert.equal(scenario.snapshot().active.length, 1, 'serial mutation leaves exactly one active binding');
  assert.equal(scenario.snapshot().active[0]!.revision, 2);
});

test('revoking an unknown device surfaces DEVICE_NOT_FOUND; revoking a registered device removes it', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await assert.rejects(
    registry.revoke({ deviceId: 'ghost' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_NOT_FOUND',
  );
  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  await registry.revoke({ deviceId: 'device-1' });
  assert.deepEqual(scenario.snapshot().active, []);
});

test('invalid input never reaches the remote', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  const invalid = [
    { deviceId: '', token: 't', platform: 'ios' as const },
    { deviceId: '   ', token: 't', platform: 'ios' as const },
    { deviceId: 'd', token: '', platform: 'ios' as const },
    { deviceId: 'd', token: 't', platform: 'webos' as unknown as 'ios' },
    { deviceId: 'x'.repeat(129), token: 't', platform: 'ios' as const },
    { deviceId: 'd', token: 'x'.repeat(4097), platform: 'ios' as const },
  ];
  for (const input of invalid) {
    await assert.rejects(
      registry.register(input),
      (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_INVALID_INPUT',
    );
  }
  assert.equal(scenario.snapshot().intentsIssued, 0, 'validation happens before any wire call');
});

test('a revoked scope lease rejects register, revoke and list with DEVICE_SCOPE_CHANGED', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { registry } = registryWith(leaseRef);

  revocable.revoke(); // 切租户/换部署/登出后 Runtime 撤销 lease（#32 revoke 咽喉）

  for (const attempt of [
    () => registry.register({ deviceId: 'd', token: 't', platform: 'ios' }),
    () => registry.revoke({ deviceId: 'd' }),
    () => registry.list(),
  ]) {
    await assert.rejects(attempt(), (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_SCOPE_CHANGED');
  }
});

test('a late register completing after lease revocation is rejected, never resolved as success', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { scenario, registry } = registryWith(leaseRef);
  const slowRemote = {
    remote: {
      ...scenario.remote,
      issueIntent: async (deviceId: string) => {
        const intent = await scenario.remote.issueIntent(deviceId);
        revocable.revoke(); // intent 返回后、bind 提交前 scope 变化
        return intent;
      },
    },
  };
  const fenced = createDeviceRegistry({ remote: slowRemote.remote, lease: () => leaseRef.lease });

  await assert.rejects(
    fenced.register({ deviceId: 'd', token: 't', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_SCOPE_CHANGED',
  );
});

test('revoke maps a 409 revision conflict to DEVICE_CONFLICT (symmetry with register)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);
  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  // revision 不匹配（本地视图落后于服务端）：服务端 ErrMobileDeviceRevision → 409。
  await assert.rejects(
    registry.revoke({ deviceId: 'device-1', revision: 42 }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_CONFLICT',
    'B3-F5：revoke 的 409 必须映射 DEVICE_CONFLICT（与 register 的冲突路径对称）',
  );
  assert.equal(scenario.snapshot().active.length, 1, '冲突的撤销不生效');
});

test('a register response with a dirty revision fails closed as DEVICE_BACKEND', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const scenario = createScenarioDeviceRemote();
  const dirty = {
    remote: {
      ...scenario.remote,
      register: scenario.remote.register.bind(scenario.remote),
    },
  };
  // 覆写 register 返回脏 revision（非安全整数）——服务端序列化缺陷时不得产生幻影 revision。
  const dirtyRemote = {
    ...dirty.remote,
    register: async (input: Parameters<typeof scenario.remote.register>[0]) => {
      const record = await scenario.remote.register(input);
      return { ...record, revision: 1.5 };
    },
  };
  const registry = createDeviceRegistry({ remote: dirtyRemote, lease: () => leaseRef.lease });
  await assert.rejects(
    registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_BACKEND',
    'B3-F6：脏 revision 响应 fail-closed，不产生幻影 revision',
  );
});

test('a register success completing after lease revocation still fails closed (F50)', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const scenario = createScenarioDeviceRemote();
  const slowBind = {
    remote: {
      ...scenario.remote,
      register: async (input: Parameters<typeof scenario.remote.register>[0]) => {
        revocable.revoke(); // bind 即将成功返回——但 scope 已撤销
        return scenario.remote.register(input);
      },
    },
  };
  const registry = createDeviceRegistry({ remote: slowBind.remote, lease: () => leaseRef.lease });
  await assert.rejects(
    registry.register({ deviceId: 'd', token: 't', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_SCOPE_CHANGED',
    'B3-F50：迟到成功不越 scope 原样返回',
  );
});
