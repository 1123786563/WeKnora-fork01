import { ApiError } from '@weknora/api-client';
import type { ClientRequest, HttpRequest } from '@weknora/api-client';
import type { AuthorizedStreamTransport, AuthorizedTransport, BlobFetchPort } from '@weknora/mobile-core';
import { TransportFailure, streamWeapp, type WeappNetwork } from './transport.ts';

/**
 * 授权 REST 通道（与 apps/mobile composition.ts:103-107 同形态）：复用既有 ClientRequest
 * 通道，叠加 Bearer；不新建传输、不持有 token——token 由 Runtime 调用时注入。
 */
export function createAuthorizedRequestChannel(request: (input: ClientRequest) => Promise<unknown>): AuthorizedTransport {
  return (input, accessToken) =>
    request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
}

/**
 * 授权 SSE 通道（AuthorizedStreamTransport 契约）：pre-stream 非 2xx 以真 ApiError 拒绝——
 * Runtime 的 401 刷新重试（unauthorizedStatus 按 name==='ApiError'&&status===401 识别，
 * mobile-runtime.ts:86-90）与 api-client 远端 409→TASK_STREAM_CURSOR_EXPIRED 映射都依赖该形态。
 */
export function createAuthorizedStreamChannel(network: WeappNetwork, origin: string): AuthorizedStreamTransport {
  return (input, accessToken, onChunk) => streamAuthorized(network, origin, input, accessToken, onChunk);
}

async function streamAuthorized(
  network: WeappNetwork,
  origin: string,
  input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal },
  accessToken: string,
  onChunk: (chunk: string) => void,
): Promise<void> {
  const request: HttpRequest = {
    method: input.method,
    url: `${origin}${input.path}`,
    headers: { ...(input.headers ?? {}), authorization: `Bearer ${accessToken}`, accept: 'text/event-stream' },
    ...(input.body === undefined ? {} : { body: input.body }),
    ...(input.signal === undefined ? {} : { signal: input.signal }),
  };
  try {
    await streamWeapp(network, request, onChunk);
  } catch (error) {
    if (error instanceof TransportFailure && typeof error.status === 'number') {
      throw new ApiError({ status: error.status, code: `HTTP_${error.status}`, message: `authorized stream failed with HTTP ${error.status}` });
    }
    throw error;
  }
}

/** 免凭据字节抓取（Task Material 签名链接的兑现通道）：仅接受 http/https（module-seams §7.3 / Mimosa URL 约束）。 */
export function createTaroBlobFetch(network: WeappNetwork): BlobFetchPort {
  return {
    async fetch(url) {
      let parsed: URL;
      try { parsed = new URL(url); } catch { throw new Error('BLOB_FETCH_INVALID_URL'); }
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('BLOB_FETCH_UNSUPPORTED_SCHEME');
      const fetched = await new Promise<{ buffer: ArrayBuffer; mime: string }>((resolve, reject) => {
        network.request({
          url, method: 'GET', header: {}, timeout: 60_000, responseType: 'arraybuffer', dataType: 'text',
          success: result => {
            if (result.statusCode !== 200) {
              reject(Object.assign(new Error(`BLOB_FETCH_HTTP_${result.statusCode}`), { status: result.statusCode }));
              return;
            }
            if (!(result.data instanceof ArrayBuffer)) { reject(new Error('BLOB_FETCH_NOT_BINARY')); return; }
            const rawHeader = (result.header ?? {}) as Record<string, unknown>;
            const mime = typeof rawHeader['content-type'] === 'string' && rawHeader['content-type'] !== ''
              ? rawHeader['content-type'] : 'application/octet-stream';
            resolve({ buffer: result.data, mime });
          },
          fail: () => reject(new Error('BLOB_FETCH_NETWORK')),
        });
      });
      return { bytes: new Uint8Array(fetched.buffer), mime: fetched.mime };
    },
  };
}
