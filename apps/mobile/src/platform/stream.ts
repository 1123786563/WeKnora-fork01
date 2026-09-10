import { createAbortError, createServerSentEventParser, parseChatEvent } from '@weknora/api-client';
import type { ChatStreamEvent } from '@weknora/contracts';

export async function consumeNativeSSE(response: Response, onEvent: (event: ChatStreamEvent) => void, signal?: AbortSignal): Promise<void> {
  if (!response.ok) throw new Error(`SSE request failed with HTTP ${response.status}`);
  if (!response.body) throw new Error('Native fetch did not expose a readable response body');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
  const abort = () => { void reader.cancel().catch(() => undefined); };
  signal?.addEventListener('abort', abort, { once: true });
  try {
    for (;;) {
      if (signal?.aborted) throw createAbortError();
      const part = await reader.read();
      if (part.done) break;
      if (part.value) parser.push(decoder.decode(part.value, { stream: true }));
    }
    const tail = decoder.decode();
    if (tail) parser.push(tail);
    parser.finish();
  } finally { signal?.removeEventListener('abort', abort); reader.releaseLock?.(); }
}
