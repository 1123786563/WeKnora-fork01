/**
 * Agent 目录领域模型（MX-016）。
 * 规则：
 * - 展示名称/能力摘要，不携带敏感 Prompt 配置（原样字段过滤）；
 * - 三态能力（supported/unavailable/forbidden + reason）驱动可选性——
 *   不可用必须显示原因，禁止隐藏入口或静默 fallback 到其它 Agent；
 * - 编码类 Agent 依赖实际已授权执行目标（targetId 已授权才 supported），
 *   不以名字猜测 shell 能力；
 * - 筛选（类型/关键词）纯函数；选中语义返回原表单（导航由宿主处理）。
 */

export type AgentCapabilityState = 'supported' | 'unavailable' | 'forbidden';

export interface AgentOption {
  id: string;
  name: string;
  summary: string;
  kind: 'general' | 'coding' | 'analysis' | 'custom';
  /** 能力状态：无记录=unavailable（没有能力事实不得放行） */
  capability: { state: AgentCapabilityState; reason: string };
  /** 编码类：绑定的授权执行目标（缺省=未授权） */
  authorizedTargetId?: string;
}

export interface AgentDirectory {
  agents: readonly AgentOption[];
}

/** 从原始目录数据（可能含敏感 Prompt 字段）投影为可展示选项。 */
export function toAgentOptions(rows: ReadonlyArray<Record<string, unknown>>): AgentOption[] {
  return rows.map((row) => {
    const kind = row.kind === 'coding' || row.kind === 'analysis' || row.kind === 'custom' ? row.kind : 'general';
    const capabilityRaw = row.capability as { state?: unknown; reason?: unknown } | undefined;
    const state = capabilityRaw?.state === 'supported' || capabilityRaw?.state === 'forbidden' ? capabilityRaw.state : 'unavailable';
    const reason = typeof capabilityRaw?.reason === 'string' ? capabilityRaw.reason : state === 'supported' ? '' : 'capability_not_reported';
    const option: AgentOption = {
      id: String(row.id ?? ''),
      name: String(row.name ?? row.id ?? ''),
      summary: String(row.summary ?? ''),
      kind,
      capability: { state, reason },
      ...(typeof row.authorizedTargetId === 'string' ? { authorizedTargetId: row.authorizedTargetId } : {}),
    };
    return option;
  }).filter((option) => option.id !== '');
}

/** 编码类 Agent 能力裁决：必须绑定实际已授权 target 才 supported。 */
export function codingAgentCapability(option: AgentOption, authorizedTargets: ReadonlyArray<{ id: string; revoked?: boolean }>): AgentOption['capability'] {
  if (option.kind !== 'coding') return option.capability;
  const authorized = authorizedTargets.find((target) => target.id === option.authorizedTargetId && !target.revoked);
  if (!authorized) {
    return { state: 'unavailable', reason: 'driver_unavailable' };
  }
  return { state: 'supported', reason: '' };
}

/** 选择裁决：不可用/禁止不可选；不可用时返回原因（禁止静默 fallback）。 */
export function selectAgent(directory: AgentDirectory, agentId: string, authorizedTargets: ReadonlyArray<{ id: string; revoked?: boolean }> = []): { agent?: AgentOption; unavailableReason?: string } {
  const base = directory.agents.find((agent) => agent.id === agentId);
  if (!base) return { unavailableReason: 'agent_not_found' };
  const effective: AgentOption = { ...base, capability: codingAgentCapability(base, authorizedTargets) };
  if (effective.capability.state !== 'supported') {
    return { unavailableReason: effective.capability.reason || effective.capability.state };
  }
  return { agent: effective };
}

/** 默认选中：首个 supported（不选 unavailable；无可用时 undefined——表单显示原因）。 */
export function defaultAgent(directory: AgentDirectory, authorizedTargets: ReadonlyArray<{ id: string; revoked?: boolean }> = []): AgentOption | undefined {
  return directory.agents.find((agent) => codingAgentCapability(agent, authorizedTargets).state === 'supported');
}

/** 类型/关键词筛选（纯函数，仅过滤展示；不改能力裁决）。 */
export function filterAgents(directory: AgentDirectory, filter: { kind?: AgentOption['kind']; keyword?: string }): AgentOption[] {
  const keyword = filter.keyword?.trim().toLowerCase() ?? '';
  return directory.agents.filter((agent) => {
    if (filter.kind && agent.kind !== filter.kind) return false;
    if (keyword && !`${agent.name}${agent.summary}`.toLowerCase().includes(keyword)) return false;
    return true;
  });
}
