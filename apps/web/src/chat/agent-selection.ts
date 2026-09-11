export interface ChatAgentOption {
  id: string;
  name: string;
}

export interface WebChatStreamOptions {
  sessionId: string;
  mode: 'knowledge' | 'agent';
  body: Record<string, unknown>;
}

export function initialAgentSelection(
  search: string,
  agents: readonly ChatAgentOption[],
  disabledIds: readonly string[],
): string {
  const requested = new URLSearchParams(search).get('agentId')?.trim() ?? '';
  return requested && agents.some((agent) => agent.id === requested) && !disabledIds.includes(requested)
    ? requested
    : '';
}

export function buildWebChatStreamOptions(sessionId: string, content: string, agentId: string | undefined): WebChatStreamOptions {
  const selected = agentId?.trim();
  if (!selected) return { sessionId, mode: 'knowledge', body: { query: content, channel: 'web' } };
  return {
    sessionId,
    mode: 'agent',
    body: { query: content, agent_enabled: true, agent_id: selected, channel: 'web' },
  };
}
