import { ApiError, createAbortError, errorFromResult, isNamedError } from '../errors.ts';
import type { HttpRequest, HttpResult, HttpTransport } from '../ports.ts';
import { consumeStreamResult, createServerSentEventParser, parseChatEvent } from '../chat/stream.ts';
import type { ChatStreamEvent } from '@weknora/contracts';
import { createEmbedApi, type EmbedApi } from './index.ts';
import type { ClientRequest } from '../client.ts';

export interface EmbedClientOptions {
  baseURL: string;
  transport: HttpTransport;
  timeoutMs?: number;
}

function joinURL(baseURL: string, path: string): string {
  const base = baseURL.replace(/\/+$/, '');
  const suffix = path.startsWith('/') ? path : `/${path}`;
  return `${base}${suffix}`;
}

/** Lightweight client entry for the isolated iframe; it does not import the authenticated app APIs. */
export function createEmbedClient(options: EmbedClientOptions): { request: (input: ClientRequest) => Promise<unknown>; embed: EmbedApi } {
  const timeoutMs = options.timeoutMs ?? 30_000;

  async function request(input: ClientRequest): Promise<unknown> {
    const controller = new AbortController();
    let timedOut = false;
    const cancel = () => controller.abort();
    if (input.signal?.aborted) controller.abort();
    else input.signal?.addEventListener('abort', cancel, { once: true });
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs);
    const request: HttpRequest = {
      method: input.method,
      url: joinURL(options.baseURL, input.path),
      headers: { accept: 'application/json', ...input.headers },
      body: input.body,
      signal: controller.signal,
    };
    try {
      const result: HttpResult = await options.transport.send(request);
      if (controller.signal.aborted) throw createAbortError();
      if (result.status === 204) return undefined;
      if (result.status < 200 || result.status >= 300) throw errorFromResult(result.status, result.body, result.headers);
      return result.body;
    } catch (error: unknown) {
      if (error instanceof ApiError) throw error;
      if (timedOut || isNamedError(error, 'TimeoutError')) throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out', cause: error });
      if (input.signal?.aborted || isNamedError(error, 'AbortError')) throw new ApiError({ code: 'CANCELLED', message: 'Request was cancelled', cause: error });
      throw error;
    } finally {
      clearTimeout(timer);
      input.signal?.removeEventListener('abort', cancel);
    }
  }

  const embed = createEmbedApi(request, async (streamRequest, onEvent, signal) => {
    const input = signal === undefined ? streamRequest : { ...streamRequest, signal };
    if (options.transport.sendStream) {
      const result = await options.transport.sendStream({
        method: input.method,
        url: joinURL(options.baseURL, input.path),
        headers: { accept: 'application/json', ...input.headers },
        body: input.body,
        signal: input.signal,
      });
      await consumeStreamResult(result, onEvent);
      return;
    }
    const body = await request(input);
    if (typeof body !== 'string') throw new Error('Embed chat stream returned a non-text body');
    const parser = createServerSentEventParser((event) => onEvent(parseChatEvent(event)));
    parser.push(body);
    parser.finish();
  });

  return { request, embed };
}

export type EmbedClient = ReturnType<typeof createEmbedClient>;
export type EmbedClientEventHandler = (event: ChatStreamEvent) => void;
