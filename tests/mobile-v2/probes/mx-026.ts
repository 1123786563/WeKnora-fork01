// MX-026 probe · 执行目标能力观察器
// frozen 场景：remote-create-true-observe-false。
// 真实 selectExecutionTarget：create✓observe✗ 的远程目标不可选（admittedRemoteCount=0），
// 回退 platform（selectedTarget=platform）；能力解释不只靠禁用态。
import {
  platformTarget,
  selectExecutionTarget,
  targetCapabilityExplanation,
  type ExecutionTargetOption,
} from '../../../packages/domain/src/mobile/target-options.ts';

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  selectedTarget: string;
  admittedRemoteCount: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'remote-create-true-observe-false') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const createOnlyRemote: ExecutionTargetOption = {
    id: 'target-remote-1',
    name: '开发机 A',
    kind: 'remote',
    visibility: 'visible',
    capability: { create: true, observe: false, reason: '远程节点未就绪' },
  };

  // frozen：可创建但不可观察 → 拒绝（admittedRemoteCount=0），不静默选择
  const frozen = selectExecutionTarget([createOnlyRemote]);
  if (frozen.admittedRemoteCount !== 0) throw new Error('create-only remote must not be admitted');
  if (frozen.selectedTarget !== 'platform') throw new Error(`fallback must be platform, got ${frozen.selectedTarget}`);
  if (!frozen.rejected.some((r) => r.id === 'target-remote-1' && r.reason.startsWith('observe_unavailable:'))) {
    throw new Error(`rejection must carry observe_unavailable reason, got ${JSON.stringify(frozen.rejected)}`);
  }
  // 能力解释包含原因文本（不只靠禁用态）
  const explanation = targetCapabilityExplanation(createOnlyRemote);
  if (!explanation.includes('远程节点未就绪')) throw new Error(`explanation must carry the reason, got "${explanation}"`);

  // 对照：create+observe 双满足 → 可选且偏好尊重
  const healthyRemote: ExecutionTargetOption = { ...createOnlyRemote, id: 'target-remote-2', capability: { create: true, observe: true, reason: '' } };
  const healthy = selectExecutionTarget([createOnlyRemote, healthyRemote], 'target-remote-2');
  if (healthy.admittedRemoteCount !== 1 || healthy.selectedTarget !== 'target-remote-2') {
    throw new Error('healthy remote must be admitted and preferred');
  }
  // 不可见远程拒绝
  const hidden = selectExecutionTarget([{ ...healthyRemote, id: 'x', visibility: 'hidden' }]);
  if (hidden.admittedRemoteCount !== 0 || !hidden.rejected.some((r) => r.reason === 'not_visible')) throw new Error('hidden remote must be rejected');
  // 平台目标恒可选
  if (platformTarget().capability.create !== true || platformTarget().capability.observe !== true) throw new Error('platform target must always be capable');

  return { selectedTarget: frozen.selectedTarget, admittedRemoteCount: frozen.admittedRemoteCount };
}
