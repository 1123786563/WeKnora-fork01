import type { ClientBinaryResponse } from '@weknora/api-client';
import { safeArtifactFileName, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import type { WebPlatformAdapters } from '../platform/adapters.ts';

/** Save authenticated artifact bytes without exposing a storage URL to the browser. */
export async function saveArtifactDownload(
  artifact: Pick<ChatArtifact, 'fileName'>,
  response: Pick<ClientBinaryResponse, 'body' | 'contentType'>,
  saveFile: WebPlatformAdapters['saveFile'],
): Promise<void> {
  const filename = artifact.fileName.trim();
  if (!filename) throw new Error('artifact filename must not be empty');
  const safeFilename = safeArtifactFileName(filename);
  const body = response.body;
  const content = typeof Blob !== 'undefined' && body instanceof Blob
    ? body
    : new Blob([body], { type: response.contentType || 'application/octet-stream' });
  await saveFile(content, safeFilename);
}
