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

export interface ChatOAuthApproval {
  pendingId: string;
  serviceId?: string;
  serviceName?: string;
  toolName?: string;
  status: 'pending' | 'resolved';
  authorized?: boolean;
  reason?: string;
}

export interface ChatInjectedUserMessage {
  steerId: string;
  content: string;
  userMessageId?: string;
}

export interface ChatStreamState {
  phase: ChatRunPhase;
  answer: string;
  thinking: string;
  references: unknown[];
  toolCalls: Record<string, ChatToolCall>;
  approvals: Record<string, ChatApproval>;
  oauthApprovals: Record<string, ChatOAuthApproval>;
  seenEventIds: readonly string[];
  lastEventId?: string;
  assistantMessageId?: string;
  error?: string;
  sessionTitle?: string;
  artifactsPending: boolean;
  injectedUserMessages: readonly ChatInjectedUserMessage[];
}

export const initialChatStreamState = (): ChatStreamState => ({
  phase: 'idle', answer: '', thinking: '', references: [], toolCalls: {}, approvals: {}, oauthApprovals: {}, seenEventIds: [],
  artifactsPending: false, injectedUserMessages: [],
});

function text(value: unknown): string { return typeof value === 'string' ? value : ''; }
function payload(event: ChatStreamEvent): Record<string, unknown> {
  return typeof event.data === 'object' && event.data !== null ? event.data as Record<string, unknown> : event;
}

export function reduceChatStream(state: ChatStreamState, event: ChatStreamEvent): ChatStreamState {
  const eventId = typeof event.event_id === 'string' && event.event_id ? event.event_id : undefined;
  if (eventId && state.seenEventIds.includes(eventId)) return state;
  const data = payload(event);
  const next: ChatStreamState = {
    ...state,
    toolCalls: { ...state.toolCalls },
    approvals: { ...state.approvals },
    oauthApprovals: { ...state.oauthApprovals },
    references: state.references.slice(),
    seenEventIds: eventId ? [...state.seenEventIds, eventId] : state.seenEventIds,
    ...(eventId ? { lastEventId: eventId } : {}),
  };
  const assistantMessageId = text(event.message_id ?? data.message_id ?? data.assistant_message_id);
  if (assistantMessageId) next.assistantMessageId = assistantMessageId;
  const kind = responseType(event);
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
    case 'mcp_oauth_required': {
      const pendingId = text(data.pending_id ?? event.pending_id);
      if (pendingId) {
        next.oauthApprovals[pendingId] = {
          pendingId,
          serviceId: text(data.service_id ?? event.service_id) || undefined,
          serviceName: text(data.service_name ?? event.service_name) || undefined,
          toolName: text(data.mcp_tool_name ?? event.mcp_tool_name) || undefined,
          status: 'pending',
        };
      }
      next.phase = 'streaming';
      break;
    }
    case 'mcp_oauth_resolved': {
      const pendingId = text(data.pending_id ?? event.pending_id);
      const serviceId = text(data.service_id ?? event.service_id) || undefined;
      const authorizedValue = data.authorized ?? event.authorized;
      const authorized = typeof authorizedValue === 'boolean' ? authorizedValue : undefined;
      if (pendingId) {
        const current = next.oauthApprovals[pendingId] ?? { pendingId, serviceId, status: 'pending' as const };
        next.oauthApprovals[pendingId] = {
          ...current,
          ...(serviceId ? { serviceId } : {}),
          status: 'resolved',
          ...(authorized === undefined ? {} : { authorized }),
          ...(text(data.reason ?? event.reason) ? { reason: text(data.reason ?? event.reason) } : {}),
        };
      }
      if (authorized === true && serviceId) {
        for (const [id, approval] of Object.entries(next.oauthApprovals)) {
          if (id !== pendingId && approval.status === 'pending' && approval.serviceId === serviceId) {
            next.oauthApprovals[id] = { ...approval, status: 'resolved', authorized: true };
          }
        }
      }
      break;
    }
    case 'session_title': {
      const title = text(data.title) || text(event.content);
      if (title) next.sessionTitle = title;
      break;
    }
    case 'artifacts_pending': next.artifactsPending = true; break;
    case 'user_message_injected': {
      const steerId = text(data.steer_id);
      if (steerId) {
        next.injectedUserMessages = [...state.injectedUserMessages, {
          steerId,
          content: text(data.content),
          ...(text(data.user_message_id) ? { userMessageId: text(data.user_message_id) } : {}),
        }];
      }
      break;
    }
    case 'complete': next.phase = 'completed'; next.artifactsPending = false; break;
    case 'stop': next.phase = 'stopped'; next.artifactsPending = false; break;
    case 'error': next.phase = 'error'; next.error = text(event.error ?? data.error ?? event.content) || 'Chat stream failed'; break;
    default: break;
  }
  return next;
}
