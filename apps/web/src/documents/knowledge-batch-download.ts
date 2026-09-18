/**
 * Batch-download decision helpers, ported from upstream
 * views/knowledge/KnowledgeBase.vue handleBatchDownload (L450-504) and
 * views/knowledge/knowledgeDownloadFileName.ts. The Vue flow filters the
 * selection down to documents with a downloadable original, enforces the
 * 200-file / 512 MiB caps, warns about skipped entries, then streams the
 * ZIP back through the authenticated request seam and triggers a browser
 * save with a knowledge-files-<ts>.zip name.
 */

export const MAX_BATCH_DOWNLOAD_FILES = 200;
export const MAX_BATCH_DOWNLOAD_BYTES = 512 * 1024 * 1024;

export interface BatchDownloadItem {
  id: string;
  type?: string;
  file_path?: string;
  file_size?: number | string;
}

/** Upstream isBatchDownloadableKnowledge: manual docs always, others need a stored original. */
export function isBatchDownloadableKnowledge(item?: BatchDownloadItem | null): boolean {
  if (!item) return false;
  if (item.type === 'manual') return true;
  return Boolean(item.file_path);
}

export interface BatchDownloadSelection {
  /** Selected ids that have a downloadable original, in selection order. */
  ids: string[];
  /** Selected entries without an original file (upstream warns about them). */
  skipped: number;
}

export function selectBatchDownloadIds(selected: Iterable<string>, itemsById: Map<string, BatchDownloadItem | undefined>): BatchDownloadSelection {
  const ids: string[] = [];
  let skipped = 0;
  for (const id of selected) {
    if (isBatchDownloadableKnowledge(itemsById.get(id))) ids.push(id);
    else skipped += 1;
  }
  return { ids, skipped };
}

/** Known byte total — entries with a missing/non-finite size simply do not add up. */
export function batchDownloadKnownBytes(ids: readonly string[], itemsById: Map<string, BatchDownloadItem | undefined>): number {
  let knownBytes = 0;
  for (const id of ids) {
    const size = Number(itemsById.get(id)?.file_size);
    if (Number.isFinite(size) && size > 0) knownBytes += size;
  }
  return knownBytes;
}

export type BatchDownloadWarningKey =
  | 'knowledgeBase.batchDownloadNoFiles'
  | 'knowledgeBase.batchDownloadHint'
  | 'knowledgeBase.batchDownloadTooLarge';

/**
 * Pre-flight gate mirroring the Vue warnings, in the same order: nothing
 * downloadable → noFiles; over the file cap → hint; over the byte cap →
 * tooLarge. Null means the download may proceed.
 */
export function batchDownloadPreflight(selection: BatchDownloadSelection, knownBytes: number): BatchDownloadWarningKey | null {
  if (selection.ids.length === 0) return 'knowledgeBase.batchDownloadNoFiles';
  if (selection.ids.length > MAX_BATCH_DOWNLOAD_FILES) return 'knowledgeBase.batchDownloadHint';
  if (knownBytes > MAX_BATCH_DOWNLOAD_BYTES) return 'knowledgeBase.batchDownloadTooLarge';
  return null;
}

/** Upstream download name: knowledge-files-<epoch-ms>.zip. */
export function batchDownloadZipName(now: number = Date.now()): string {
  return `knowledge-files-${now}.zip`;
}

/** DOM save: anchor click on an object URL, revoked a minute later like the Vue flow. */
export function saveBatchDownloadBlob(blob: Blob, fileName: string, doc: Document = document, win: Window = window): void {
  const url = URL.createObjectURL(blob);
  const link = doc.createElement('a');
  link.href = url;
  link.download = fileName;
  doc.body.appendChild(link);
  link.click();
  link.remove();
  // Give the browser time to start the save before releasing the URL.
  win.setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
