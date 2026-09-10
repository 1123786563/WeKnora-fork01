import {
  parseSteerDeleteResponse,
  parseSteerListResponse,
  parseSteerMutationResponse,
  type SteerDeleteResponse,
  type SteerDelivery,
  type SteerListResponse,
  type SteerMutationResponse,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface EnqueueSteerInput {
  query: string;
  delivery?: SteerDelivery;
  expectedAssistantMessageId?: string;
  steerId?: string;
  mentionedItems?: unknown[];
  channel?: string;
}

function encodedId(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function optionalNonEmpty(value: string | undefined, name: string): void {
  if (value !== undefined && (typeof value !== 'string' || value.trim() === '')) {
    throw new Error(`${name} must not be empty`);
  }
}

export function createChatSteerApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async enqueue(
      sessionId: string,
      input: EnqueueSteerInput,
      signal?: AbortSignal,
    ): Promise<SteerMutationResponse> {
      if (typeof input.query !== 'string' || input.query.trim() === '') throw new Error('query must not be empty');
      if (input.delivery !== undefined && input.delivery !== 'inject' && input.delivery !== 'after') {
        throw new Error('delivery must be inject or after');
      }
      optionalNonEmpty(input.expectedAssistantMessageId, 'expectedAssistantMessageId');
      optionalNonEmpty(input.steerId, 'steerId');
      if (input.mentionedItems !== undefined && !Array.isArray(input.mentionedItems)) {
        throw new Error('mentionedItems must be an array');
      }
      return parseSteerMutationResponse(await request({
        method: 'POST',
        path: `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/steer`,
        body: {
          query: input.query,
          ...(input.delivery === undefined ? {} : { delivery: input.delivery }),
          ...(input.steerId === undefined ? {} : { steer_id: input.steerId }),
          ...(input.expectedAssistantMessageId === undefined
            ? {}
            : { expected_assistant_message_id: input.expectedAssistantMessageId }),
          ...(input.mentionedItems === undefined ? {} : { mentioned_items: input.mentionedItems }),
          ...(input.channel === undefined ? {} : { channel: input.channel }),
        },
        ...(signal === undefined ? {} : { signal }),
      }));
    },

    async list(sessionId: string, signal?: AbortSignal): Promise<SteerListResponse> {
      return parseSteerListResponse(await request({
        method: 'GET',
        path: `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/steer`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },

    async promote(sessionId: string, steerId: string, signal?: AbortSignal): Promise<SteerMutationResponse> {
      return parseSteerMutationResponse(await request({
        method: 'POST',
        path: `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/steer/${encodedId(steerId, 'steerId')}/inject`,
        body: {},
        ...(signal === undefined ? {} : { signal }),
      }));
    },

    async remove(sessionId: string, steerId: string, signal?: AbortSignal): Promise<SteerDeleteResponse> {
      return parseSteerDeleteResponse(await request({
        method: 'DELETE',
        path: `/api/v1/sessions/${encodedId(sessionId, 'sessionId')}/steer/${encodedId(steerId, 'steerId')}`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

export type ChatSteerApi = ReturnType<typeof createChatSteerApi>;
