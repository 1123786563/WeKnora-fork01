import type { ChatStreamEvent } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import type { HttpStreamResult } from '../ports.ts';

export interface ParsedServerSentEvent {
  id?: string;
  event?: string;
  data: string;
}

export type ServerSentEventHandler = (event: ParsedServerSentEvent) => void;

/** Incremental SSE parser; callers decide how JSON data maps to ChatStreamEvent. */
export function createServerSentEventParser(onEvent: ServerSentEventHandler) {
  let buffer = '';
  let id: string | undefined;
  let event: string | undefined;
  let data: string[] = [];

  const dispatch = () => {
    if (data.length === 0) {
      id = undefined;
      event = undefined;
      return;
    }
    onEvent({ ...(id === undefined ? {} : { id }), ...(event === undefined ? {} : { event }), data: data.join('\n') });
    id = undefined;
    event = undefined;
    data = [];
  };

  const consumeLine = (line: string) => {
    if (line === '') return dispatch();
    if (line.startsWith(':')) return;
    const separator = line.indexOf(':');
    const field = separator < 0 ? line : line.slice(0, separator);
    const value = separator < 0 ? '' : line.slice(separator + 1).replace(/^ /, '');
    if (field === 'id') id = value;
    else if (field === 'event') event = value;
    else if (field === 'data') data.push(value);
  };

  return {
    push(chunk: string) {
      buffer += chunk.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) consumeLine(line);
    },
    finish() {
      if (buffer) consumeLine(buffer);
      dispatch();
      buffer = '';
    },
  };
}

export interface ChatStreamRequestOptions {
  sessionId: string;
  mode?: 'knowledge' | 'agent';
  body: Record<string, unknown>;
  lastEventId?: string;
  signal?: AbortSignal;
}

export function buildChatStreamRequest(options: ChatStreamRequestOptions): ClientRequest {
  const mode = options.mode ?? 'knowledge';
  const prefix = mode === 'agent' ? '/api/v1/agent-chat/' : '/api/v1/knowledge-chat/';
  return {
    method: 'POST',
    path: `${prefix}${encodeURIComponent(options.sessionId)}`,
    headers: {
      accept: 'text/event-stream',
      ...(options.lastEventId ? { 'Last-Event-ID': options.lastEventId } : {}),
    },
    body: options.body,
    ...(options.signal === undefined ? {} : { signal: options.signal }),
  };
}

export function parseChatEvent(event: ParsedServerSentEvent): ChatStreamEvent {
  const value: unknown = JSON.parse(event.data);
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('Invalid chat SSE event');
  return {
    ...(value as ChatStreamEvent),
    ...(event.id ? { event_id: event.id } : {}),
    ...(event.event && !(value as ChatStreamEvent).type ? { type: event.event } : {}),
  };
}

export async function consumeChatStream(
  request: (input: ClientRequest) => Promise<unknown>,
  options: ChatStreamRequestOptions,
  onEvent: (event: ChatStreamEvent) => void,
): Promise<void> {
  const body = await request(buildChatStreamRequest(options));
  if (typeof body !== 'string') throw new Error('Chat stream returned a non-text body');
  const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
  parser.push(body);
  parser.finish();
}

export function consumeStreamResult(result: HttpStreamResult, onEvent: (event: ChatStreamEvent) => void): Promise<void> {
  if (result.status < 200 || result.status >= 300) throw new Error(`Chat stream failed with HTTP ${result.status}`);
  return (async () => {
    const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
    for await (const chunk of result.chunks) parser.push(chunk);
    parser.finish();
  })();
}
