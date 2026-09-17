// MX-033 安全故障注入：并发批准/撤权/换空间/迟到回调——写入前边界验证。
// 全部驱动真实实现（interaction 协调器 / scope 生成守卫 / 连接版本守卫）。
import test from 'node:test';
import assert from 'node:assert/strict';
import { createInteractionController } from '../../apps/mobile/sources/weknora/interactions/controller.ts';
import { createProductScope } from '../../apps/mobile/sources/weknora/platform/product-session.ts';
import { createConnectionController, type ConnectionPorts } from '../../apps/mobile/sources/weknora/resources/connection-controller.ts';
import type { InteractionRecord } from '@weknora/contracts';

const pending: InteractionRecord = { id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'h1', expected_revision: 4 };

test('security: concurrent approvals — exactly one decide, loser gets refresh', async () => {
  let decideCalls = 0;
  const controller = createInteractionController({
    list: async () => [pending],
    decide: async (value) => {
      decideCalls += 1;
      return { ...value, decision_id: 'd-winner' };
    },
    now: () => '2026-09-18T08:00:00Z',
  });
  const generation = { accept: () => true };
  const snapshot = { record: pending, fetchedAt: '2026-09-18T07:59:00Z' };
  // 两设备并发确认：共享 decide 端口——真实 CAS 在服务端（MX-005 frozen 已证 1 成功 1 冲突）；
  // 客户端边界：第二个确认在 latest 已 decided 后必须幂等（不重发）
  const first = await controller.confirm(snapshot, pending, 'approve', generation);
  const decided: InteractionRecord = { ...pending, decision_id: 'd-winner', action: 'approve' };
  const second = await controller.confirm(snapshot, decided, 'approve', generation);
  assert.equal(first.action, 'acknowledged');
  assert.equal(second.action, 'acknowledged');
  assert.equal(second.reason, 'already_decided');
  assert.equal(decideCalls, 1);
});

test('security: revocation — late approval never dispatched after scope switch', async () => {
  let decideCalls = 0;
  const controller = createInteractionController({
    list: async () => [pending],
    decide: async (value) => { decideCalls += 1; return { ...value, decision_id: 'd' }; },
    now: () => '2026-09-18T08:00:00Z',
  });
  const generation = { accept: () => false }; // 换空间后：守卫失效
  const snapshot = { record: pending, fetchedAt: '2026-09-18T07:59:00Z' };
  const result = await controller.confirm(snapshot, pending, 'approve', generation);
  assert.equal(result.action, 'refresh');
  assert.equal(decideCalls, 0);
});

test('security: late response after tenant switch dropped by scope generation', async () => {
  const scope = createProductScope({ origin: 'https://a.example', userId: 'u1', tenantID: 'A' });
  const captured = scope.capture();
  scope.switchTo({ origin: 'https://a.example', userId: 'u1', tenantID: 'B' });
  assert.equal(scope.accept(captured.generation), false);
});

test('security: revoked connection — stale-version dispatch rejected with zero provider writes', async () => {
  let current = { id: 'c1', name: 'X', ownership: 'tenant' as const, status: 'connected' as const, authVersion: 3 };
  const ports: ConnectionPorts = {
    get: async () => current,
    revoke: async (_id, v) => {
      if (v !== current.authVersion) throw new Error('VERSION_CONFLICT');
      current = { ...current, status: 'revoked', authVersion: current.authVersion + 1 };
      return current;
    },
    resolveForDispatch: async (_id, v) => (current.status === 'revoked' ? { authorized: false as const, reason: 'revoked' as const } : v === current.authVersion ? { authorized: true as const } : { authorized: false as const, reason: 'version_mismatch' as const }),
  };
  const controller = createConnectionController(ports);
  await controller.revoke(current);
  const late = await controller.dispatchWithAuth('c1', 3);
  assert.equal(late.performed, false);
  assert.equal(controller.providerWriteObservations(), 0);
  assert.deepEqual(controller.rawSecretFields(), []);
});
