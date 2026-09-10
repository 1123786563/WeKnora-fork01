import { responseType, type ChatStreamEvent } from '@weknora/contracts';

export type ChatRunPhase = 'idle' | 'streaming' | 'completed' | 'stopped' | 'error';

export interface ChatToolCall {
  id: string;
  name?: string;
  status: 'pending' | 'completed' | 'failed';
  result?: unknown;
}

export interface ChatApproval {
  pendingId: string;
  toolCallId?: string;
  status: 'pending' | 'resolved';
  decision?: string;
}

export interface ChatStreamState {
  phase: ChatRunPhase;
  answer: string;
  thinking: string;
  references: unknown[];
  toolCalls: Record<string, ChatToolCall>;
  approvals: Record<string, ChatApproval>;
  seenEventIds: readonly string[];
  lastEventId?: string;
  error?: string;
}

export const initialChatStreamState = (): ChatStreamState => ({
  phase: 'idle', answer: '', thinking: '', references: [], toolCalls: {}, approvals: {}, seenEventIds: [],
});

function text(value: unknown): string { return typeof value === 'string' ? value : ''; }
function payload(event: ChatStreamEvent): Record<string, unknown> {
  return typeof event.data === 'object' && event.data !== null ? event.data as Record<string, unknown> : event;
}

export function reduceChatStream(state: ChatStreamState, event: ChatStreamEvent): ChatStreamState {
  const eventId = typeof event.event_id === 'string' && event.event_id ? event.event_id : undefined;
  if (eventId && state.seenEventIds.includes(eventId)) return state;
  const next: ChatStreamState = {
    ...state,
    toolCalls: { ...state.toolCalls },
    approvals: { ...state.approvals },
    references: state.references.slice(),
    seenEventIds: eventId ? [...state.seenEventIds, eventId] : state.seenEventIds,
    ...(eventId ? { lastEventId: eventId } : {}),
  };
  const kind = responseType(event);
  const data = payload(event);
  switch (kind) {
    case 'answer': next.phase = 'streaming'; next.answer += text(event.content ?? data.content); break;
    case 'thinking': next.phase = 'streaming'; next.thinking += text(event.content ?? data.content); break;
    case 'references': {
      const refs = data.references ?? event.references;
      if (Array.isArray(refs)) next.references.push(...refs);
      break;
    }
    case 'tool_call': {
      const id = text(data.tool_call_id ?? event.tool_call_id);
      if (id) next.toolCalls[id] = { id, name: text(data.tool_name ?? event.tool_name) || undefined, status: 'pending' };
      next.phase = 'streaming';
      break;
    }
    case 'tool_result': {
      const id = text(data.tool_call_id ?? event.tool_call_id);
      if (id) next.toolCalls[id] = { ...(next.toolCalls[id] ?? { id }), id, status: 'completed', result: data.result ?? event.result };
      break;
    }
    case 'tool_approval_required': {
      const pendingId = text(data.pending_id ?? event.pending_id);
      if (pendingId) next.approvals[pendingId] = { pendingId, toolCallId: text(data.tool_call_id ?? event.tool_call_id) || undefined, status: 'pending' };
      next.phase = 'streaming';
      break;
    }
    case 'tool_approval_resolved': {
      const pendingId = text(data.pending_id ?? event.pending_id);
      if (pendingId) next.approvals[pendingId] = { ...(next.approvals[pendingId] ?? { pendingId }), pendingId, status: 'resolved', decision: text(data.decision ?? event.decision) || undefined };
      break;
    }
    case 'complete': next.phase = 'completed'; break;
    case 'stop': next.phase = 'stopped'; break;
    case 'error': next.phase = 'error'; next.error = text(event.error ?? data.error ?? event.content) || 'Chat stream failed'; break;
    default: break;
  }
  return next;
}
