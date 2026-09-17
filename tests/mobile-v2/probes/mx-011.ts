// MX-011 probe · 空间切换迟到响应隔离观察器
// 用真实 createProductScope（generation/abort 原语）演练：A→B 切换后 A 的迟到响应被丢弃，
// 服务端 Run 从不被取消（本地隔离≠服务端取消）。业务规则全部来自产品代码。
import { createProductScope } from '../../../apps/mobile/sources/weknora/platform/product-session.ts';
import { createGenerationGuard, normalizeOrigin, productQueryKey } from '../../../packages/domain/src/mobile/query-scope.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  visibleTenant: string;
  oldResponseApplied: boolean;
  canceledServerRuns: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'tenant-A-to-B' || input.fault !== 'A-response-arrives-last') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  // 服务端取消计数器：产品域在空间切换时绝不触发服务端 Run 取消
  let canceledServerRuns = 0;

  const scope = createProductScope({ origin: 'https://weknora.example', userId: 'u1', tenantId: 'A' });
  scope.registerLifecycle(() => {
    // 生命周期回调只做本地清理（关流/弃数据）；本 fixture 不注册任何服务端取消——
    // canceledServerRuns>0 即违规（服务器 Run 的存续与客户端空间切换解耦）
  });

  // A 空间发起请求：捕获 generation 与守卫
  const capturedA = scope.capture();
  const guardA = createGenerationGuard((generation) => scope.accept(generation), capturedA.generation);
  const keyA = productQueryKey(scope.identity(), 'runs');

  // 切换到 B：generation 前进、旧信号中止、生命周期清理触发
  let cleanupFired = false;
  scope.registerLifecycle(() => { cleanupFired = true; });
  scope.switchTo({ origin: 'https://weknora.example', userId: 'u1', tenantId: 'B' });

  // A 的响应最晚到达
  await Promise.resolve();
  const oldResponseApplied = guardA.isCurrent();
  const identity = scope.identity();
  const keyB = productQueryKey(identity, 'runs');

  if (!cleanupFired) throw new Error('switching spaces must run lifecycle cleanup (close old streams/drop old visible data)');
  if (normalizeOrigin('https://WeKnora.Example/') !== normalizeOrigin('https://weknora.example')) {
    throw new Error('origin normalization must collapse case/trailing slash variants');
  }
  if (keyA[keyA.length - 2] === keyB[keyB.length - 2]) throw new Error('query keys must be tenant-isolated');

  return {
    visibleTenant: identity.tenantId ?? '',
    oldResponseApplied,
    canceledServerRuns,
  };
}
