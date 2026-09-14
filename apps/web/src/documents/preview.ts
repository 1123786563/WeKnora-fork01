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

export type InlinePreviewKind = 'text' | 'markdown' | 'image' | 'pdf' | 'audio' | 'video';
export type PreviewBody = string | Blob | ArrayBuffer;

export function isInlinePreviewKind(kind: KnowledgePreviewKind): kind is InlinePreviewKind {
  return kind === 'text' || kind === 'markdown' || kind === 'image' || kind === 'pdf' || kind === 'audio' || kind === 'video';
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
  if (kind === 'audio') {
    return createElement('audio', { className: 'wk-preview-audio block w-full max-w-[480px]', src: url, controls: true, 'aria-label': fileName || 'Audio preview' });
  }
  if (kind === 'video') {
    return createElement('video', { className: 'wk-preview-video block max-h-[calc(100vh-240px)] max-w-full', src: url, controls: true, playsInline: true, 'aria-label': fileName || 'Video preview' });
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
