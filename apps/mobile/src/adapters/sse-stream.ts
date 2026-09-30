/** 流式读取授权 SSE 响应。非 2xx 以 ApiError 形态拒绝（Runtime 401 重试与远端 409 映射都依赖该形态）。 */
export type SseFetchLike = (input: string, init?: { method?: string; headers?: Record<string, string>; signal?: AbortSignal }) => Promise<Response>;

export async function streamAuthorizedSse(
  origin: string,
  input: { method: string; path: string; headers?: Record<string, string>; signal?: AbortSignal },
  accessToken: string,
  onChunk: (chunk: string) => void,
  fetchLike: SseFetchLike,
): Promise<void> {
  const response = await fetchLike(origin + input.path, {
    method: input.method,
    headers: { ...(input.headers ?? {}), authorization: `Bearer ${accessToken}`, accept: 'text/event-stream' },
    ...(input.signal === undefined ? {} : { signal: input.signal }),
  });
  if (!response.ok || !response.body) {
    const error = new Error(`workbench event stream failed with HTTP ${response.status}`);
    error.name = 'ApiError';
    (error as unknown as { status?: number }).status = response.status;
    throw error;
  }
  const reader = (response.body as ReadableStream<Uint8Array>).getReader();
  const decoder = new TextDecoder();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (value !== undefined && value.length > 0) onChunk(decoder.decode(value, { stream: true }));
  }
  onChunk(decoder.decode());
}
