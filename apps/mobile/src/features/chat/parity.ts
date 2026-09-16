import { groupChatReferences, type ChatReferenceGroup } from '@weknora/domain/chat/references';
import { initialChatStreamState, type ChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import type { ChatMessage, ChatStreamEvent } from '@weknora/contracts';

export interface ChatRunToken {
  sessionId: string;
  runId: string;
}

export function isCurrentChatRun(active: ChatRunToken | null, event: ChatRunToken): boolean {
  return active?.sessionId === event.sessionId && active.runId === event.runId;
}

export function buildMobileChatRequestBody(
  query: string,
  knowledgeBaseIds: readonly string[],
  attachmentIds: readonly string[] = [],
): Record<string, unknown> {
  return {
    query,
    knowledge_base_ids: [...knowledgeBaseIds],
    attachment_ids: [...attachmentIds],
    channel: 'mobile',
  };
}

export function shouldRenderLiveAssistant(sending: boolean, answer: string): boolean {
  return sending && answer.length > 0;
}

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

export function findRetryQuery(messages: readonly ChatMessage[], assistantMessageId: string): string | undefined {
  const assistantIndex = messages.findIndex((message) => message.id === assistantMessageId && message.role === 'assistant');
  if (assistantIndex < 0) return undefined;
  for (let index = assistantIndex - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === 'user') return message.content.trim() || undefined;
  }
  return undefined;
}

export function shouldRenderPendingUser(messages: readonly ChatMessage[], pendingUser: string | null): boolean {
  return pendingUser !== null && !messages.some((message) => message.role === 'user' && message.content === pendingUser);
}

export function selectMessageArtifacts(message: ChatMessage): unknown[] {
  const value = message.artifacts;
  return Array.isArray(value) ? value : [];
}
