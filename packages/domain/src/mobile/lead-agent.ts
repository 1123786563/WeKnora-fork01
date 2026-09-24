import type { AgentOption } from './agent-options.ts';

/**
 * 主理 Agent（Lead Agent）推荐（MX-016 目录上的纯策略，CONTEXT.md「主理 Agent」：
 * 系统可以推荐，用户也可以显式选择）。
 * 规则（不猜测）：
 * - 候选 = capability.state === 'supported'（unavailable/forbidden 绝不推荐）；
 * - 通用目标入口（User Story 9）的默认主理优先 kind='general' 的首个 supported；
 * - 无 supported general 时取任意首个 supported；
 * - 无任何 supported → 显式 reason，表单展示原因而不是静默 fallback。
 */
export type LeadAgentRecommendation =
  | { agent: AgentOption; basis: 'kind-general' | 'first-supported' }
  | { agent: undefined; reason: 'no_supported_agent' };

export function recommendLeadAgent(agents: readonly AgentOption[]): LeadAgentRecommendation {
  const supported = agents.filter((candidate) => candidate.capability.state === 'supported');
  if (supported.length === 0) return { agent: undefined, reason: 'no_supported_agent' };
  const general = supported.find((candidate) => candidate.kind === 'general');
  if (general) return { agent: general, basis: 'kind-general' };
  return { agent: supported[0]!, basis: 'first-supported' };
}
