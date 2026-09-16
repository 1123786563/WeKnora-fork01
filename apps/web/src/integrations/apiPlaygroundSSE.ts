import { createServerSentEventParser } from '@weknora/api-client';

// Ported from the Vue baseline frontend/src/views/integrations/apiPlaygroundSSE.ts.
// Frame layer reuses the shared api-client SSE parser (same pattern as
// packages/api-client/src/sandbox/skill-install.ts); only the business-level
// terminal-state reduction is playground-specific. Read errors (including
// AbortError from the playground's stop button) propagate to the caller so the
// drawer can settle the steps as stopped.

export interface ApiPlaygroundSSEProgress {
  raw: string;
  answer: string;
}

export type ApiPlaygroundSSEResult = ApiPlaygroundSSEProgress & (
  | { status: 'success' }
  | { status: 'failed'; reason: 'terminal-error' | 'unexpected-eof'; error?: string }
);

interface TerminalState {
  status: 'success' | 'failed';
  error?: string;
}

interface StreamEventPayload {
  response_type?: unknown;
  type?: unknown;
  content?: unknown;
  message?: unknown;
  error?: unknown;
  done?: unknown;
}

function readErrorMessage(payload: StreamEventPayload): string | undefined {
  if (typeof payload.content === 'string' && payload.content.trim()) return payload.content;
  if (typeof payload.message === 'string' && payload.message.trim()) return payload.message;
  if (typeof payload.error === 'string' && payload.error.trim()) return payload.error;
  if (
    payload.error
    && typeof payload.error === 'object'
    && 'message' in payload.error
    && typeof (payload.error as { message: unknown }).message === 'string'
    && (payload.error as { message: string }).message.trim()
  ) {
    return (payload.error as { message: string }).message;
  }
  return undefined;
}

function applyEventData(data: string, answerChunks: string[]): TerminalState | undefined {
  if (data.trim() === '[DONE]') return { status: 'success' };

  let payload: StreamEventPayload;
  try {
    const parsed: unknown = JSON.parse(data);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return undefined;
    payload = parsed as StreamEventPayload;
  } catch {
    return undefined;
  }

  const responseType = payload.response_type ?? payload.type;
  if (responseType === 'answer' && typeof payload.content === 'string') {
    answerChunks.push(payload.content);
  }
  if (responseType === 'error') {
    return { status: 'failed', error: readErrorMessage(payload) };
  }
  if (responseType === 'complete' || (responseType === 'answer' && payload.done === true)) {
    return { status: 'success' };
  }
  return undefined;
}

/**
 * Consume an API Playground SSE stream until its business-level terminal event.
 * The parser owns chunk buffering and terminal-state reduction so callers never
 * need to infer success from the transport reaching EOF. A terminal event
 * cancels the connection instead of draining it, and EOF without a terminal
 * event is a failure ('unexpected-eof'), matching the Vue playground.
 */
export async function consumeApiPlaygroundSSE(
  stream: ReadableStream<Uint8Array>,
  onProgress?: (progress: ApiPlaygroundSSEProgress) => void,
): Promise<ApiPlaygroundSSEResult> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  const answerChunks: string[] = [];
  let raw = '';
  let terminal: TerminalState | undefined;

  const progress = () => {
    const value = { raw, answer: answerChunks.join('') };
    onProgress?.(value);
    return value;
  };

  // Frames after a terminal event are ignored, mirroring the Vue reduction.
  const parser = createServerSentEventParser((event) => {
    if (terminal) return;
    terminal = applyEventData(event.data, answerChunks);
  });

  const finish = (): ApiPlaygroundSSEResult => {
    const value = progress();
    if (terminal!.status === 'failed') {
      return { ...value, status: 'failed', reason: 'terminal-error', error: terminal!.error };
    }
    return { ...value, status: 'success' };
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      raw += decoder.decode();
      parser.finish();
      if (terminal) return finish();
      const current = progress();
      return { ...current, status: 'failed', reason: 'unexpected-eof' };
    }

    const chunk = decoder.decode(value, { stream: true });
    raw += chunk;
    parser.push(chunk);
    if (terminal) {
      void reader.cancel().catch(() => undefined);
      return finish();
    }
    progress();
  }
}
