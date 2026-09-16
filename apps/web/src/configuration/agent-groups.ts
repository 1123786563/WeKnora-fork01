// Agent list grouping ported from the Vue baseline
// frontend/src/views/agent/AgentList.vue (builtin always on top, then
// created-by-me, then shared-from-others; collapsible headers with counts
// and a search box over name/description).

export type AgentGroupKey = 'builtin' | 'mine' | 'shared';

export interface AgentRowLike {
  id?: unknown;
  name?: unknown;
  description?: unknown;
  is_builtin?: unknown;
  created_by?: unknown;
  permission?: unknown;
  [key: string]: unknown;
}

/** Mirrors the Vue list's client-side guard for the permissions present here. */
export function canManageAgent(agent: AgentRowLike): boolean {
  if (agent.is_builtin === true) return false;
  if (typeof agent.permission === 'string' && agent.permission !== 'editor' && agent.permission !== 'admin') return false;
  return true;
}

export function agentGroupOf(agent: AgentRowLike, currentUserId: string): AgentGroupKey {
  if (agent.is_builtin === true) return 'builtin';
  const creator = typeof agent.created_by === 'string' ? agent.created_by : '';
  return creator !== '' && creator === currentUserId ? 'mine' : 'shared';
}

export function groupAgents<T extends AgentRowLike>(agents: readonly T[], currentUserId: string): Record<AgentGroupKey, T[]> {
  const groups: Record<AgentGroupKey, T[]> = { builtin: [], mine: [], shared: [] };
  for (const agent of agents) groups[agentGroupOf(agent, currentUserId)].push(agent);
  return groups;
}

export function filterAgentsByQuery<T extends AgentRowLike>(agents: readonly T[], query: string): T[] {
  const needle = query.trim().toLowerCase();
  if (needle === '') return [...agents];
  return agents.filter((agent) => {
    const name = typeof agent.name === 'string' ? agent.name.toLowerCase() : '';
    const description = typeof agent.description === 'string' ? agent.description.toLowerCase() : '';
    return name.includes(needle) || description.includes(needle);
  });
}
