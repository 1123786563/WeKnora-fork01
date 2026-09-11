import { groupChatReferences, type ChatReferenceGroup } from '@weknora/domain/chat/references';
import { initialChatStreamState, type ChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import type { ChatMessage, ChatStreamEvent } from '@weknora/contracts';

export function replayMobileChatEvents(events: readonly ChatStreamEvent[]): ChatStreamState {
  let state = initialChatStreamState();
  for (const event of events) state = reduceChatStream(state, event);
  return state;
}

export function selectAssistantMessageId(event: ChatStreamEvent): string | undefined {
  const data = typeof event.data === 'object' && event.data !== null && !Array.isArray(event.data)
    ? event.data as Record<string, unknown>
    : undefined;
  const candidates = [event.message_id, event.assistant_message_id, data?.assistant_message_id, data?.message_id];
  return candidates.find((value): value is string => typeof value === 'string' && value.trim() !== '');
}

export function selectReferenceGroups(state: ChatStreamState): ChatReferenceGroup[] {
  return groupChatReferences(state.references);
}

export function selectIncompleteAssistant(messages: readonly ChatMessage[]): ChatMessage | undefined {
  return [...messages].reverse().find((message) => message.role === 'assistant' && message.is_completed === false);
}

export function shouldRenderPendingUser(messages: readonly ChatMessage[], pendingUser: string | null): boolean {
  return pendingUser !== null && !messages.some((message) => message.role === 'user' && message.content === pendingUser);
}

export function selectMessageArtifacts(message: ChatMessage): unknown[] {
  const value = message.artifacts;
  return Array.isArray(value) ? value : [];
}
