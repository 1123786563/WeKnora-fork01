import { parseActionSuccessResponse, parseMessageSuggestionResponse, type MessageSuggestionSet } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

function encodedId(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function suggestionPath(sessionId: string, messageId: string): string {
  return `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/messages/${encodedId(messageId, 'messageId')}/suggestions`;
}

export type SuggestionEventType = 'impression' | 'click' | 'dismiss';

export function createChatSuggestionsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async ensure(sessionId: string, messageId: string, regenerate = false, signal?: AbortSignal): Promise<MessageSuggestionSet> {
      return parseMessageSuggestionResponse(await request({
        method: 'POST', path: suggestionPath(sessionId, messageId), body: { regenerate },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async get(sessionId: string, messageId: string, signal?: AbortSignal): Promise<MessageSuggestionSet> {
      return parseMessageSuggestionResponse(await request({
        method: 'GET', path: suggestionPath(sessionId, messageId),
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async recordEvent(sessionId: string, suggestionSetId: string, eventType: SuggestionEventType, questionId = '', signal?: AbortSignal): Promise<void> {
      if (!['impression', 'click', 'dismiss'].includes(eventType)) throw new Error('eventType must be impression, click, or dismiss');
      if (typeof questionId !== 'string') throw new Error('questionId must be a string');
      encodedId(suggestionSetId, 'suggestionSetId');
      const response = await request({
        method: 'POST', path: `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/suggestion-events`,
        body: { suggestion_set_id: suggestionSetId, question_id: questionId, event_type: eventType },
        ...(signal === undefined ? {} : { signal }),
      });
      // The server deliberately returns 204 for telemetry. A non-empty body,
      // when supplied by a proxy, must still be a valid success envelope.
      if (response !== undefined) parseActionSuccessResponse(response);
    },
  };
}

export type ChatSuggestionsApi = ReturnType<typeof createChatSuggestionsApi>;
