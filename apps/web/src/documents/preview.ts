import type { KnowledgeDocument } from '@weknora/api-client';
import { createElement, type ReactElement } from 'react';
import { previewKindForFile, previewStatus, type KnowledgePreviewKind } from '@weknora/domain/knowledge/preview';

export interface KnowledgeDocumentPreviewModel {
  kind: KnowledgePreviewKind;
  ready: boolean;
  downloadOnly: boolean;
  path: string;
  fileName: string;
}

export type InlinePreviewKind = 'text' | 'markdown' | 'image' | 'pdf';
export type PreviewBody = string | Blob | ArrayBuffer;

export function isInlinePreviewKind(kind: KnowledgePreviewKind): kind is InlinePreviewKind {
  return kind === 'text' || kind === 'markdown' || kind === 'image' || kind === 'pdf';
}

export async function readPreviewText(body: PreviewBody): Promise<string> {
  if (typeof body === 'string') return body;
  if (typeof Blob !== 'undefined' && body instanceof Blob) return body.text();
  if (body instanceof ArrayBuffer) return new TextDecoder().decode(body);
  throw new Error('Preview body is not readable as text');
}

export function previewBodyAsBlob(body: PreviewBody, contentType?: string): Blob {
  if (typeof Blob !== 'undefined' && body instanceof Blob) {
    if (!contentType || body.type === contentType) return body;
    return new Blob([body], { type: contentType });
  }
  return new Blob([body], { type: contentType || 'application/octet-stream' });
}

export function DocumentPreviewContent({
  kind,
  text,
  url,
  fileName,
}: {
  kind: InlinePreviewKind;
  text?: string;
  url?: string;
  fileName?: string;
}): ReactElement {
  if (kind === 'text' || kind === 'markdown') {
    return createElement('pre', { className: 'wk-preview-text', 'aria-label': `${fileName || 'Document'} content` }, text || '');
  }
  if (kind === 'image') {
    return createElement('img', { className: 'wk-preview-image', src: url, alt: fileName || 'Document preview' });
  }
  return createElement('iframe', {
    className: 'wk-preview-pdf',
    src: url,
    title: `${fileName || 'Document'} PDF preview`,
  });
}

export function buildDocumentPreview(document: KnowledgeDocument, previewPath: string): KnowledgeDocumentPreviewModel {
  const fileName = document.file_name || document.title || 'document';
  const status = previewStatus(document);
  const kind = previewKindForFile(fileName);
  return { kind, ready: status.kind === 'ready', downloadOnly: !isInlinePreviewKind(kind), path: previewPath, fileName };
}
