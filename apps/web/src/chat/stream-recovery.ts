/**
 * Last-Event-ID reconnection for the chat SSE stream.
 *
 * The api-client already sends a Last-Event-ID header when the stream options
 * carry lastEventId (packages/api-client/src/chat/stream.ts), and the reducer
 * records the newest SSE id per run (domain reducer ChatStreamState). This
 * module holds the caller-side policy: on a mid-flight transport failure,
 * retry the same request exactly once, replaying from the last seen event id.
 * A retry is only offered when at least one event was received (otherwise
 * there is nothing to resume from and the failure is simply surfaced).
 */

/** An SSE error event is a terminal application result, not a broken socket. */
export class ChatStreamApplicationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ChatStreamApplicationError';
  }
}

export function isChatStreamApplicationError(error: unknown): error is ChatStreamApplicationError {
  return error instanceof ChatStreamApplicationError;
}

/**
 * Vue stream failure copy (frontend/src/api/chat/streame.ts): a failed
 * handshake throws `HTTP ${status}` and the onerror fail path surfaces
 * `${error.streamFailed}: ${message}` — the localized prefix followed by the
 * transport reason, e.g. 「流式连接失败: HTTP 404」. The api-client raises
 * `Chat stream failed with HTTP <status>`, so the HTTP status is extracted
 * instead of embedding the whole English sentence.
 */
export function streamFailureMessage(cause: unknown, streamFailedLabel: string): string {
  const reason = cause instanceof Error ? cause.message : String(cause);
  const httpStatus = reason.match(/HTTP (\d{3})/);
  return `${streamFailedLabel}: ${httpStatus ? `HTTP ${httpStatus[1]}` : reason}`;
}

/**
 * Returns the retry stream options for a failed stream, or null when a resume
 * is not possible (no events received, or a resume was already attempted).
 */
export function resumeStreamOptions<T extends object>(options: T, lastEventId: string | undefined): (T & { lastEventId: string }) | null {
  if (!lastEventId) return null;
  // Already resumed once (or the caller resumed explicitly); do not loop.
  if ((options as { lastEventId?: string }).lastEventId) return null;
  return { ...options, lastEventId };
}

/**
 * Wraps a stream feed so the newest SSE event id is captured for recovery.
 * Returns the last seen event id via the boxed holder.
 */
export interface LastEventIdHolder {
  id?: string;
}

export function feedWithLastEventId(feed: (event: { event_id?: string }) => void, holder: LastEventIdHolder): (event: { event_id?: string }) => void {
  return (event) => {
    const id = event.event_id;
    if (typeof id === 'string' && id) holder.id = id;
    feed(event);
  };
}
