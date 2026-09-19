import { parseActionSuccessResponse, parseMessageFeedbackListResponse, type FeedbackRating } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

function feedbackPath(sessionId: string, messageId?: string): string {
  const session = encodeURIComponent(sessionId);
  if (sessionId.trim() === '') throw new Error('sessionId must not be empty');
  if (messageId === undefined) return `/api/v1/messages/${session}/feedback/mine`;
  if (messageId.trim() === '') throw new Error('messageId must not be empty');
  return `/api/v1/messages/${session}/${encodeURIComponent(messageId)}/feedback`;
}

export function createFeedbackApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async submit(sessionId: string, messageId: string, rating: FeedbackRating, comment = '', signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({
        method: 'POST',
        path: feedbackPath(sessionId, messageId),
        body: { rating, comment },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async remove(sessionId: string, messageId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'DELETE', path: feedbackPath(sessionId, messageId), ...(signal === undefined ? {} : { signal }) }));
    },
    async mine(sessionId: string, signal?: AbortSignal) {
      return parseMessageFeedbackListResponse(await request({ method: 'GET', path: feedbackPath(sessionId), ...(signal === undefined ? {} : { signal }) }));
    },
  };
}
export type FeedbackApi = ReturnType<typeof createFeedbackApi>;
