import type { KnowledgeDocument } from '@weknora/api-client';

export type DocumentStatus = { key: string; tone: 'primary' | 'success' | 'warning' | 'danger' | 'default'; busy: boolean };

export function getDocumentStatus(document: Pick<KnowledgeDocument, 'parse_status' | 'summary_status'> & { id: string }): DocumentStatus {
  const status = document.parse_status;
  if (status === 'pending' || status === 'processing') return { key: 'processing', tone: 'primary', busy: true };
  if (status === 'finalizing') {
    return { key: document.summary_status === 'pending' || document.summary_status === 'processing' ? 'generating-summary' : 'finalizing', tone: 'primary', busy: true };
  }
  if (status === 'failed') return { key: 'failed', tone: 'danger', busy: false };
  if (status === 'cancelled') return { key: 'cancelled', tone: 'warning', busy: false };
  if ((status as string) === 'draft') return { key: 'draft', tone: 'warning', busy: false };
  if (status === 'completed' && (document.summary_status === 'pending' || document.summary_status === 'processing')) {
    return { key: 'generating-summary', tone: 'primary', busy: true };
  }
  if (status === 'completed') return { key: 'completed', tone: 'success', busy: false };
  return { key: 'unknown', tone: 'default', busy: false };
}

export interface DocumentPage { items: KnowledgeDocument[]; total: number; page: number; pageSize: number }

export function normalizeDocumentPage(value: unknown): DocumentPage {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('Invalid document page');
  const row = value as Record<string, unknown>;
  if (row.success !== true) throw new Error('Invalid document page success');
  if (!Array.isArray(row.data)) throw new Error('Invalid document page data');
  const items = row.data.map((item, index) => {
    if (typeof item !== 'object' || item === null || Array.isArray(item) || typeof (item as { id?: unknown }).id !== 'string') {
      throw new Error(`Invalid document id at data[${index}]`);
    }
    return item as KnowledgeDocument;
  });
  return {
    items,
    total: Number(row.total ?? items.length),
    page: Number(row.page ?? 1),
    pageSize: Number(row.page_size ?? row.pageSize ?? 20),
  };
}

export interface UploadFileLike { name: string; size: number }
export interface UploadProgressTask { uploadId: string; fileName?: string; progress: number; status: 'uploading' | 'success' | 'error'; error?: string }
export interface UploadProgressSummary { total: number; completed: number; progress: number; hasError: boolean }

export function summarizeUploadProgress(tasks: UploadProgressTask[]): UploadProgressSummary {
  if (tasks.length === 0) return { total: 0, completed: 0, progress: 0, hasError: false };
  return {
    total: tasks.length,
    completed: tasks.filter((task) => task.status !== 'uploading').length,
    progress: Math.min(100, Math.max(0, Math.round(tasks.reduce((sum, task) => sum + Math.min(100, Math.max(0, task.progress)), 0) / tasks.length))),
    hasError: tasks.some((task) => task.status === 'error'),
  };
}

export interface UploadRules { maxBytes?: number; acceptedExtensions?: string[] }
export function validateUpload(files: UploadFileLike[], rules: UploadRules = {}): { valid: boolean; errors: string[] } {
  if (files.length === 0) return { valid: false, errors: ['select-file'] };
  const accepted = rules.acceptedExtensions?.map((extension) => extension.toLowerCase());
  if (accepted?.length && files.some((file) => !accepted.includes(file.name.slice(file.name.lastIndexOf('.')).toLowerCase()))) {
    return { valid: false, errors: ['unsupported-type'] };
  }
  if (rules.maxBytes !== undefined && files.some((file) => file.size > rules.maxBytes!)) return { valid: false, errors: ['file-too-large'] };
  return { valid: true, errors: [] };
}

export function toggleDocumentSelection(selection: Set<string>, id: string, checked: boolean, allIds: string[] = []): Set<string> {
  const next = new Set(selection);
  if (id === 'all') return checked ? new Set(allIds) : new Set();
  if (checked) next.add(id); else next.delete(id);
  return next;
}

export function canDocumentAction(action: 'preview' | 'download' | 'edit' | 'reparse' | 'cancel' | 'delete' | 'tags', permission: { canView: boolean; canEdit: boolean }, document?: { type?: string; [key: string]: unknown }): boolean {
  if (!permission.canView) return false;
  if (action === 'preview') return true;
  if (action === 'download') return document?.type === 'file' || document?.type === 'manual';
  return permission.canEdit;
}

export interface TimelineStep { key: 'uploaded' | 'parsing' | 'enriching' | 'indexed' | 'failed' | 'cancelled'; state: 'done' | 'active' | 'error' | 'muted' }

export interface MergeableChunk { content?: string; start_at?: unknown; end_at?: unknown; chunk_index?: unknown; [key: string]: unknown }

const MIN_CHUNK_OVERLAP = 12;

/**
 * Vue doc-content#appendChunkContent: stitch `next` onto the merged text while
 * removing the overlap the chunker re-emits. Matching is done on the text
 * itself (suffix of acc found in a bounded head window of next) so repadded
 * table headers and entity-encoded length drift are both tolerated; a
 * non-positive position overlap means strictly adjacent ranges and concat's
 * directly to avoid cutting real repeated content.
 */
function appendChunkContent(acc: string, next: string, positionOverlap: number): string {
  if (!acc) return next;
  if (!next) return acc;
  if (positionOverlap <= 0) return acc + next;
  const span = Math.max(positionOverlap, 0);
  const maxK = Math.min(acc.length, next.length, Math.max(span * 3, 400));
  const headSlack = Math.max(span * 2, 320);
  for (let k = maxK; k >= MIN_CHUNK_OVERLAP; k--) {
    const suffix = acc.slice(acc.length - k);
    const pos = next.indexOf(suffix);
    if (pos !== -1 && pos <= headSlack) return acc + next.slice(pos + k);
  }
  return acc + next;
}

/**
 * Vue doc-content#mergeChunks: order by start_at (chunk_index fallback), join
 * real position gaps with a blank line, and trim re-emitted overlaps.
 */
export function mergeChunkContents(chunks: MergeableChunk[]): string {
  if (!chunks || chunks.length === 0) return '';
  const sorted = [...chunks].sort((left, right) =>
    Number(left.start_at ?? left.chunk_index ?? 0) - Number(right.start_at ?? right.chunk_index ?? 0));
  let merged = sorted[0].content || '';
  let mergedEnd = Number(sorted[0].end_at ?? 0);
  for (let index = 1; index < sorted.length; index++) {
    const chunk = sorted[index];
    const startAt = Number(chunk.start_at ?? 0);
    const endAt = Number(chunk.end_at ?? 0);
    const content = chunk.content || '';
    if (!content) continue;
    if (startAt > mergedEnd && mergedEnd > 0) merged = merged + '\n\n' + content;
    else merged = appendChunkContent(merged, content, mergedEnd - startAt);
    if (endAt > mergedEnd) mergedEnd = endAt;
  }
  return merged;
}

export function buildProcessingTimeline(document: Pick<KnowledgeDocument, 'parse_status' | 'summary_status'> & { id: string }): TimelineStep[] {
  const status = getDocumentStatus(document);
  const steps: TimelineStep[] = [
    { key: 'uploaded', state: 'done' },
    { key: 'parsing', state: status.key === 'processing' ? 'active' : status.key === 'failed' || status.key === 'cancelled' ? 'error' : 'done' },
  ];
  if (status.key === 'failed') return [...steps, { key: 'failed', state: 'error' }];
  if (status.key === 'cancelled') return [...steps, { key: 'cancelled', state: 'error' }];
  if (status.key === 'processing') return steps;
  steps.push({ key: 'enriching', state: status.key === 'generating-summary' || status.key === 'finalizing' ? 'active' : 'done' });
  if (status.key === 'generating-summary' || status.key === 'finalizing') return steps;
  steps.push({ key: 'indexed', state: status.key === 'completed' ? 'done' : 'muted' });
  return steps;
}
