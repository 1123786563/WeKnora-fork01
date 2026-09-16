import type { NativeFileSource } from '@weknora/api-client';

export interface PickedFileLike {
  uri: string;
  name?: string | null;
  mimeType?: string | null;
  size?: number | null;
}

export function toNativeFileSource(file: PickedFileLike): NativeFileSource {
  return {
    uri: file.uri,
    name: file.name?.trim() || 'upload',
    type: file.mimeType || 'application/octet-stream',
    ...(file.size === undefined || file.size === null ? {} : { size: file.size }),
  };
}

export function authHeaderForFileDownload(accessToken: string | undefined): string | undefined {
  return accessToken ? `Bearer ${accessToken}` : undefined;
}
