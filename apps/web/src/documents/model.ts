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

export function canDocumentAction(action: 'preview' | 'download' | 'edit' | 'reparse' | 'cancel' | 'delete' | 'tags', permission: { canView: boolean; canEdit: boolean }): boolean {
  if (!permission.canView) return false;
  return action === 'preview' || action === 'download' ? true : permission.canEdit;
}

export interface TimelineStep { key: 'uploaded' | 'parsing' | 'enriching' | 'indexed' | 'failed' | 'cancelled'; state: 'done' | 'active' | 'error' | 'muted' }
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
