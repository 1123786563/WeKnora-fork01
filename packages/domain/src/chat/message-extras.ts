import type { ChatMessage } from '@weknora/contracts';

export interface AssistantMessageExtrasToolCall {
  id: string;
  name?: string;
  status: 'pending' | 'completed' | 'failed';
  result?: unknown;
}

export interface AssistantMessageExtras {
  thinking: string;
  toolCalls: AssistantMessageExtrasToolCall[];
}

interface PersistedAgentStep {
  thought?: unknown;
  reasoning_content?: unknown;
  tool_calls?: unknown;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : undefined;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

/**
 * Rebuilds the thinking trace and tool-call list for an assistant message so
 * they survive the post-turn history refresh. Live rows carry transient
 * `thinking` / `tool_calls` fields; persisted rows carry `agent_steps`
 * (thought / reasoning_content / tool_calls per ReAct iteration).
 */
export function assistantMessageExtras(message: ChatMessage | undefined): AssistantMessageExtras {
  const row = asRecord(message);
  if (!row) return { thinking: '', toolCalls: [] };
  const liveThinking = text(row.thinking);
  const liveToolCalls = Array.isArray(row.tool_calls) ? row.tool_calls : undefined;
  if (liveThinking || liveToolCalls) {
    return {
      thinking: liveThinking,
      toolCalls: (liveToolCalls ?? []).flatMap((value) => {
        const call = asRecord(value);
        if (!call || typeof call.id !== 'string') return [];
        const status = call.status === 'pending' || call.status === 'failed' ? call.status : 'completed';
        return [{ id: call.id, ...(text(call.name) ? { name: text(call.name) } : {}), status, ...(call.result === undefined ? {} : { result: call.result }) }];
      }),
    };
  }
  const steps = Array.isArray(row.agent_steps) ? row.agent_steps as PersistedAgentStep[] : [];
  const thinkingParts: string[] = [];
  const toolCalls: AssistantMessageExtrasToolCall[] = [];
  for (const step of steps) {
    const record = asRecord(step);
    if (!record) continue;
    const reasoning = text(record.reasoning_content) || text(record.thought);
    if (reasoning) thinkingParts.push(reasoning);
    const calls = Array.isArray(record.tool_calls) ? record.tool_calls : [];
    for (const value of calls) {
      const call = asRecord(value);
      if (!call) continue;
      const id = text(call.id) || 'tool-' + toolCalls.length;
      const result = asRecord(call.result);
      toolCalls.push({
        id,
        ...(text(call.name) ? { name: text(call.name) } : {}),
        status: 'completed',
        ...(result && text(result.output) ? { result: result.output } : {}),
      });
    }
  }
  return { thinking: thinkingParts.join('\n\n'), toolCalls };
}
