import { artifactDownloadPath, normalizeArtifactMetadata, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import type { ClientBinaryResponse, ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;
type RequestBinary = (input: ClientRequest) => Promise<ClientBinaryResponse>;

function record(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function listResponse(value: unknown): ChatArtifact[] {
  const body = record(value);
  if (!body || body.success !== true || !Array.isArray(body.data)) throw new Error('Invalid artifact list response');
  return body.data.map((item) => {
    const artifact = normalizeArtifactMetadata(item);
    if (!artifact) throw new Error('Invalid artifact metadata');
    return artifact;
  });
}

function sessionPath(sessionId: string): string {
  if (!sessionId.trim()) throw new Error('sessionId must not be empty');
  return `/api/v1/sessions/${encodeURIComponent(sessionId)}`;
}

export function createChatArtifactsApi(request: Request, requestBinary: RequestBinary) {
  return {
    async session(sessionId: string, signal?: AbortSignal): Promise<ChatArtifact[]> {
      return listResponse(await request({ method: 'GET', path: `${sessionPath(sessionId)}/artifacts`, ...(signal === undefined ? {} : { signal }) }));
    },
    async message(sessionId: string, messageId: string, signal?: AbortSignal): Promise<ChatArtifact[]> {
      if (!messageId.trim()) throw new Error('messageId must not be empty');
      return listResponse(await request({ method: 'GET', path: `${sessionPath(sessionId)}/messages/${encodeURIComponent(messageId)}/artifacts`, ...(signal === undefined ? {} : { signal }) }));
    },
    async download(sessionId: string, messageId: string, index: number, signal?: AbortSignal): Promise<ClientBinaryResponse> {
      return requestBinary({ method: 'GET', path: artifactDownloadPath(sessionId, messageId, index), ...(signal === undefined ? {} : { signal }) });
    },
  };
}

export type ChatArtifactsApi = ReturnType<typeof createChatArtifactsApi>;
