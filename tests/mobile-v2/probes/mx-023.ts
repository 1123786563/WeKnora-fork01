// MX-023 probe · 连接撤销版本守卫观察器
// frozen 场景：auth-version3 × revoke-then-dispatch。
// 真实 createConnectionController：v3 连接撤销→旧版本(v3 前快照)派发被拒（零外写）；
// 凭据字段结构性不存在。
import { createConnectionController, type ConnectionPorts } from '../../../apps/mobile/sources/weknora/resources/connection-controller.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  providerWriteCount: number;
  rawSecretFields: string[];
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'auth-version3' || input.fault !== 'revoke-then-dispatch') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const v3 = { id: 'conn-1', name: 'GitHub', ownership: 'tenant' as const, status: 'connected' as const, authVersion: 3 };
  let current = { ...v3 };
  const ports: ConnectionPorts = {
    get: async () => current,
    revoke: async (id, expectedVersion) => {
      if (expectedVersion !== current.authVersion) throw new Error('VERSION_CONFLICT');
      current = { ...current, status: 'revoked', authVersion: current.authVersion + 1 };
      return current;
    },
    resolveForDispatch: async (_id, authVersion) => {
      if (current.status === 'revoked') return { authorized: false, reason: 'revoked' as const };
      if (authVersion !== current.authVersion) return { authorized: false, reason: 'version_mismatch' as const };
      return { authorized: true };
    },
  };
  const controller = createConnectionController(ports);

  // 撤销（携带当前版本 3）→ 成功，状态 revoked、版本递增
  const revoked = await controller.revoke(v3);
  if (revoked.status !== 'revoked' || revoked.authVersion !== 4) throw new Error('revoke must carry version and invalidate state');

  // 迟到派发：撤销前捕获的 v3 快照 → 拒绝（零外写）
  const lateDispatch = await controller.dispatchWithAuth('conn-1', 3);
  if (lateDispatch.performed) throw new Error('dispatch on revoked connection must be rejected');
  // 旧版本派发（连接被重新授权到 v4 后用 v3）→ 版本拒绝
  current = { ...current, status: 'connected' }; // 重新授权场景
  const staleVersion = await controller.dispatchWithAuth('conn-1', 3);
  if (staleVersion.performed) throw new Error('stale auth version dispatch must be rejected');
  // 最新版本派发 → 允许（但本控制器不直写 Provider——计数仍 0）
  const fresh = await controller.dispatchWithAuth('conn-1', 4);
  if (!fresh.performed) throw new Error('fresh-version dispatch should be authorized');

  const providerWriteCount = controller.providerWriteObservations();
  if (providerWriteCount !== 0) throw new Error('controller must never perform provider writes directly');
  const rawSecretFields = controller.rawSecretFields();
  if (rawSecretFields.length !== 0) throw new Error('client model must carry no secret fields');

  return { providerWriteCount, rawSecretFields };
}
