import type { NativeFileSource } from '@weknora/api-client';
import type { UploadEvent } from './upload-progress.ts';

export interface KnowledgeUploadQueueResult {
  succeeded: number;
  failures: Array<{ file: NativeFileSource; error: string }>;
  aborted: boolean;
}

export interface KnowledgeUploadQueueOptions {
  signal: AbortSignal;
  upload: (file: NativeFileSource, signal: AbortSignal) => Promise<unknown>;
  dispatch: (event: UploadEvent) => void;
  createUploadId?: (file: NativeFileSource, index: number) => string;
}

function errorMessage(cause: unknown): string {
  return cause instanceof Error ? cause.message : 'Unable to upload file';
}

/** Uploads a picker batch through the existing single-file endpoint in order. */
export async function uploadKnowledgeFiles(
  files: readonly NativeFileSource[],
  kbId: string,
  options: KnowledgeUploadQueueOptions,
): Promise<KnowledgeUploadQueueResult> {
  const createUploadId = options.createUploadId ?? ((file, index) => `upload-${Date.now()}-${index}-${file.name}`);
  let succeeded = 0;
  const failures: KnowledgeUploadQueueResult['failures'] = [];

  for (const [index, file] of files.entries()) {
    if (options.signal.aborted) break;
    const uploadId = createUploadId(file, index);
    options.dispatch({ type: 'start', uploadId, kbId, fileName: file.name });
    try {
      await options.upload(file, options.signal);
      succeeded += 1;
      options.dispatch({ type: 'complete', uploadId, kbId, status: 'success', progress: 100 });
    } catch (cause) {
      const error = errorMessage(cause);
      failures.push({ file, error });
      options.dispatch({ type: 'complete', uploadId, kbId, status: 'error', error });
      if (options.signal.aborted) break;
    }
  }

  return { succeeded, failures, aborted: options.signal.aborted };
}
