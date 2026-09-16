import type { KnowledgeDocument } from '@weknora/contracts';
import { normalizeKnowledgeProcessingStatus, processingStatusLabel } from './processing.ts';

export type KnowledgePreviewKind = 'pdf' | 'image' | 'markdown' | 'spreadsheet' | 'docx' | 'pptx' | 'audio' | 'video' | 'text' | 'unsupported';

export function previewKindForFile(fileName: string): KnowledgePreviewKind {
  const extension = fileName.trim().toLowerCase().split('.').pop() ?? '';
  if (extension === 'pdf') return 'pdf';
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'tiff', 'tif', 'avif'].includes(extension)) return 'image';
  if (['md', 'markdown', 'mdx'].includes(extension)) return 'markdown';
  if (['csv', 'tsv', 'xls', 'xlsx'].includes(extension)) return 'spreadsheet';
  if (extension === 'docx') return 'docx';
  if (extension === 'pptx') return 'pptx';
  if (['mp3', 'wav', 'ogg', 'oga', 'm4a', 'aac', 'flac'].includes(extension)) return 'audio';
  if (['mp4', 'webm', 'ogv', 'mov', 'm4v'].includes(extension)) return 'video';
  if (['txt', 'json', 'xml', 'html', 'htm', 'yaml', 'yml', 'log'].includes(extension)) return 'text';
  return 'unsupported';
}

export function previewStatus(item: Pick<KnowledgeDocument, 'parse_status'>): { kind: 'ready' | 'processing' | 'unavailable'; label: string } {
  if (!item.parse_status) return { kind: 'unavailable', label: 'Unknown status' };
  let status: ReturnType<typeof normalizeKnowledgeProcessingStatus>;
  try {
    status = normalizeKnowledgeProcessingStatus(item.parse_status);
  } catch {
    return { kind: 'unavailable', label: 'Unknown status' };
  }
  if (status === 'completed') return { kind: 'ready', label: 'Ready' };
  if (status === 'failed' || status === 'cancelled') return { kind: 'unavailable', label: processingStatusLabel(status) };
  return { kind: 'processing', label: processingStatusLabel(status) };
}
