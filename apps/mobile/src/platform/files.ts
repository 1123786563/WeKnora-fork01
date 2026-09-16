import * as DocumentPicker from 'expo-document-picker';
import { Directory, File, Paths } from 'expo-file-system';
import * as Sharing from 'expo-sharing';
import { createAbortError, type Credential, type NativeFileSource } from '@weknora/api-client';
import { safeArtifactFileName } from '@weknora/domain/chat/artifacts';
import { authHeaderForFileDownload, toNativeFileSource } from './file-uris.ts';

export async function pickNativeFiles(multiple = true): Promise<NativeFileSource[]> {
  const result = await DocumentPicker.getDocumentAsync({
    // The cancellable native UploadTask requires a seekable file:// source;
    // Android's DocumentsUI otherwise returns a content:// URI whose backing
    // directory does not exist for expo-file-system's legacy task API.
    copyToCacheDirectory: true,
    multiple,
    type: '*/*',
  });
  if (result.canceled) return [];
  return result.assets.map(toNativeFileSource);
}

/** Single-file compatibility wrapper for chat attachments and other callers. */
export async function pickNativeFile(): Promise<NativeFileSource | null> {
  const files = await pickNativeFiles(false);
  return files[0] ?? null;
}

function bearerHeaders(credential: Credential): Record<string, string> {
  const authorization = credential.kind === 'bearer' ? authHeaderForFileDownload(credential.accessToken) : undefined;
  return authorization ? { authorization } : {};
}

export async function downloadKnowledgeFile(options: {
  baseURL: string;
  path: string;
  fileName: string;
  credential: Credential;
  signal?: AbortSignal;
}): Promise<string> {
  if (options.signal?.aborted) throw createAbortError('Download was cancelled');
  const destination = new File(new Directory(Paths.cache), safeArtifactFileName(options.fileName || 'download'));
  const result = await File.downloadFileAsync(`${options.baseURL.replace(/\/+$/, '')}${options.path}`, destination, {
    headers: bearerHeaders(options.credential),
    idempotent: true,
  });
  return result.uri;
}

export async function readNativeTextFile(uri: string): Promise<string> {
  return new File(uri).text();
}

export async function shareNativeFile(uri: string): Promise<void> {
  if (!(await Sharing.isAvailableAsync())) throw new Error('Native sharing is unavailable');
  await Sharing.shareAsync(uri);
}
