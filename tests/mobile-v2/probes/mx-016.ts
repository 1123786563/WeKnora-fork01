// MX-016 probe · Agent 能力选择观察器
// frozen 场景：coding-driver-unavailable。
// 真实领域模型（toAgentOptions/codingAgentCapability/selectAgent/defaultAgent）：
// 编码 Agent 无授权 target → 不可用（原因 driver_unavailable）；默认选择回退到首个可用（general）。
import {
  defaultAgent,
  selectAgent,
  toAgentOptions,
} from '../../../packages/domain/src/mobile/agent-options.ts';

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  selectedAgent: string;
  unavailableReason: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'coding-driver-unavailable') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  // 原始目录行（含会被投影过滤的敏感 prompt 字段）
  const rows = [
    { id: 'general', name: '通用助手', summary: '日常问答与整理', kind: 'general', capability: { state: 'supported', reason: '' }, prompt_template: 'SECRET-INTERNAL-PROMPT' },
    { id: 'coding', name: '编码 Agent', summary: '在授权目标上执行编码任务', kind: 'coding', capability: { state: 'supported', reason: '' }, authorizedTargetId: 'target-1', prompt_template: 'SECRET-INTERNAL-PROMPT' },
  ];
  const directory = { agents: toAgentOptions(rows) };

  // 授权目标为空：coding 不可用（不以名字启用 shell）
  const coding = selectAgent(directory, 'coding', []);
  if (coding.agent !== undefined || coding.unavailableReason !== 'driver_unavailable') {
    throw new Error(`coding without an authorized target must be unavailable(driver_unavailable), got ${JSON.stringify(coding)}`);
  }
  // 默认选择回退到首个可用（general）——不可用者不静默入选
  const fallback = defaultAgent(directory, []);
  if (!fallback || fallback.id !== 'general') {
    throw new Error('default selection must fall back to the first supported agent');
  }
  // 有授权 target 时 coding 可用
  const codingAuthorized = selectAgent(directory, 'coding', [{ id: 'target-1' }]);
  if (!codingAuthorized.agent) throw new Error('coding with an authorized target must be selectable');
  // 已撤销 target 同样不可用
  const codingRevoked = selectAgent(directory, 'coding', [{ id: 'target-1', revoked: true }]);
  if (codingRevoked.agent !== undefined) throw new Error('revoked target must not enable coding agent');
  // 敏感字段被投影过滤
  if (JSON.stringify(directory.agents).includes('SECRET-INTERNAL-PROMPT')) {
    throw new Error('prompt template must not leak into display options');
  }

  return {
    selectedAgent: fallback.id,
    unavailableReason: coding.unavailableReason ?? '',
  };
}
