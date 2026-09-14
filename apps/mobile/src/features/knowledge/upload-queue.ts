import type { NativeFileSource } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { knowledgeListLabel } from './list.ts';
import type { UploadEvent } from './upload-progress.ts';

export interface KnowledgeUploadQueueResult {
  succeeded: number;
  failures: Array<{ file: NativeFileSource; error: string }>;
  aborted: boolean;
}

export interface KnowledgeUploadQueueOptions {
  signal: AbortSignal;
  locale: Locale;
  upload: (file: NativeFileSource, signal: AbortSignal) => Promise<unknown>;
  dispatch: (event: UploadEvent) => void;
  createUploadId?: (file: NativeFileSource, index: number) => string;
}

export function knowledgeUploadErrorLabel(locale: Locale, cause: unknown): string {
  const message = cause instanceof Error ? cause.message : '';
  // Backends may expose either the stable code or a wrapped request error.
  if (/\bduplicate(?:_file)?\b|\bfile_exists\b/i.test(message)) {
    return knowledgeListLabel(locale, 'knowledgeBase.fileExists');
  }
  return knowledgeListLabel(locale, 'knowledgeBase.uploadFailed');
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
      const error = knowledgeUploadErrorLabel(options.locale, cause);
      failures.push({ file, error });
      options.dispatch({ type: 'complete', uploadId, kbId, status: 'error', error });
      if (options.signal.aborted) break;
    }
  }

  return { succeeded, failures, aborted: options.signal.aborted };
}
