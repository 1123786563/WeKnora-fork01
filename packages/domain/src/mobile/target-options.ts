/**
 * 执行目标选择领域模型（MX-026 / M15）。
 * 冻结规则：
 * - 目标能力三态（capability）：create/observe 分开——「可创建远程目标」不等于
 *   「可观察其执行」（frozen：create=true observe=false 的远程目标不可选，回退 platform）；
 * - 普通可见性≠管理权限：列表展示受 visibility，选择受 capability；
 * - 无可用目标时默认 platform（不静默选择不可用远程）；能力不可用显示原因。
 */

export interface ExecutionTargetOption {
  id: string;
  name: string;
  kind: 'platform' | 'remote';
  visibility: 'visible' | 'hidden';
  capability: {
    create: boolean;
    observe: boolean;
    reason: string;
  };
}

export interface TargetSelection {
  selectedTarget: string;
  admittedRemoteCount: number;
  rejected: Array<{ id: string; reason: string }>;
}

/** 平台目标始终存在且可选（回退目标——不静默选择远程）。 */
export function platformTarget(): ExecutionTargetOption {
  return {
    id: 'platform',
    name: '平台执行',
    kind: 'platform',
    visibility: 'visible',
    capability: { create: true, observe: true, reason: '' },
  };
}

/**
 * 目标裁决：远程目标须 create+observe 双满足才可选；
 * create-only（不可观察）或不可见均拒绝并给原因；无可用远程→platform。
 */
export function selectExecutionTarget(targets: readonly ExecutionTargetOption[], preferred?: string): TargetSelection {
  const platform = platformTarget();
  const admitted: ExecutionTargetOption[] = [platform];
  const rejected: Array<{ id: string; reason: string }> = [];
  for (const target of targets) {
    if (target.kind !== 'remote' || target.visibility !== 'visible') {
      if (target.kind === 'remote') rejected.push({ id: target.id, reason: target.visibility !== 'visible' ? 'not_visible' : 'not_remote' });
      continue;
    }
    if (!target.capability.create) {
      rejected.push({ id: target.id, reason: `create_unavailable:${target.capability.reason || 'capability not reported'}` });
      continue;
    }
    if (!target.capability.observe) {
      // frozen 场景：可创建但不可观察（远程未就绪）——不可选
      rejected.push({ id: target.id, reason: `observe_unavailable:${target.capability.reason || 'capability not reported'}` });
      continue;
    }
    admitted.push(target);
  }
  const remoteCount = admitted.length - 1;
  const selected = preferred !== undefined && admitted.some((t) => t.id === preferred)
    ? preferred
    : 'platform';
  return { selectedTarget: selected, admittedRemoteCount: remoteCount, rejected };
}

/** 能力解释文案（不只靠禁用态——读屏与视觉共用）。 */
export function targetCapabilityExplanation(target: ExecutionTargetOption): string {
  if (target.kind === 'platform') return '平台执行始终可用';
  if (target.capability.create && target.capability.observe) return '该远程目标已授权且可观察执行';
  if (!target.capability.create && !target.capability.observe) return `该目标不可用：${target.capability.reason || '能力未上报'}`;
  if (!target.capability.create) return `该目标暂不可创建新任务：${target.capability.reason || '创建能力不可用'}（已有任务或可观察）`;
  return `该目标暂不可观察执行过程：${target.capability.reason || '观察能力不可用'}（需远程就绪）`;
}
