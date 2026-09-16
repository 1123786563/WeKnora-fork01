import type { KnowledgeDocument } from '@weknora/api-client';
import { createElement, useEffect, useRef, type ReactElement } from 'react';
import * as XLSX from 'xlsx';
import { hydrateMermaidBlocksWithBrowserDefaults } from '@weknora/views/chat/mermaid';
import { previewKindForFile, previewStatus, type KnowledgePreviewKind } from '@weknora/domain/knowledge/preview';

export interface KnowledgeDocumentPreviewModel {
  kind: DocumentPreviewKind;
  availability: {
    kind: 'ready' | 'processing' | 'error' | 'unsupported';
    label: string;
  };
  ready: boolean;
  downloadOnly: boolean;
  path: string;
  fileName: string;
}

export type InlinePreviewKind = 'text' | 'markdown' | 'image' | 'pdf' | 'audio' | 'video' | 'spreadsheet' | 'mermaid';
export type DocumentPreviewKind = KnowledgePreviewKind | 'mermaid';
export type PreviewBody = string | Blob | ArrayBuffer;

export interface SpreadsheetPreviewSheet {
  name: string;
  rows: string[][];
}

export interface SpreadsheetPreviewModel {
  sheets: SpreadsheetPreviewSheet[];
}

export function isInlinePreviewKind(kind: DocumentPreviewKind): kind is InlinePreviewKind {
  return kind === 'text' || kind === 'markdown' || kind === 'image' || kind === 'pdf' || kind === 'audio' || kind === 'video' || kind === 'spreadsheet' || kind === 'mermaid';
}

function previewKindForDocument(fileName: string): DocumentPreviewKind {
  const extension = fileName.trim().toLowerCase().split('.').pop() ?? '';
  return extension === 'mmd' || extension === 'mermaid' ? 'mermaid' : previewKindForFile(fileName);
}

export async function readPreviewText(body: PreviewBody): Promise<string> {
  if (typeof body === 'string') return body;
  if (typeof Blob !== 'undefined' && body instanceof Blob) return body.text();
  if (body instanceof ArrayBuffer) return new TextDecoder().decode(body);
  throw new Error('Preview body is not readable as text');
}

/**
 * Decode text only while its owning preview is still current.
 *
 * A document switch or drawer close can happen after the preview response
 * arrives but before Blob.text() resolves. Returning undefined makes that
 * stale result a no-op instead of replacing a newly opened preview.
 */
export async function readCurrentPreviewText(body: PreviewBody, isCurrent: () => boolean): Promise<string | undefined> {
  const text = await readPreviewText(body);
  return isCurrent() ? text : undefined;
}

export function previewBodyAsBlob(body: PreviewBody, contentType?: string): Blob {
  if (typeof Blob !== 'undefined' && body instanceof Blob) {
    if (!contentType || body.type === contentType) return body;
    return new Blob([body], { type: contentType });
  }
  return new Blob([body], { type: contentType || 'application/octet-stream' });
}

export async function readSpreadsheetPreview(body: PreviewBody, fileName: string): Promise<SpreadsheetPreviewModel> {
  let input: string | ArrayBuffer;
  if (typeof body === 'string') input = body;
  else if (body instanceof ArrayBuffer) input = body;
  else input = await (body as Blob).arrayBuffer();

  const workbook = typeof input === 'string'
    ? XLSX.read(input, { type: 'string' })
    : XLSX.read(input, { type: 'array' });
  return {
    sheets: workbook.SheetNames.map((name) => {
      const sheet = workbook.Sheets[name];
      const rows = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '', raw: false })
        .map((row) => row.map((cell) => cell == null ? '' : String(cell)));
      return { name, rows };
    }),
  };
}

export function DocumentPreviewContent({
  kind,
  text,
  url,
  fileName,
  spreadsheet,
}: {
  kind: InlinePreviewKind | 'mermaid';
  text?: string;
  url?: string;
  fileName?: string;
  spreadsheet?: SpreadsheetPreviewModel;
}): ReactElement {
  if (kind === 'spreadsheet') {
    return createElement('div', { className: 'wk-preview-spreadsheet', 'aria-label': `${fileName || 'Document'} content` },
      ...(spreadsheet?.sheets || []).map((sheet) => createElement('section', { className: 'mb-4 last:mb-0', key: sheet.name },
        createElement('h3', { className: 'mb-2 text-sm font-semibold text-ink' }, sheet.name),
        createElement('div', { className: 'overflow-x-auto rounded-control border border-line-soft' },
          createElement('table', { className: 'min-w-full border-collapse text-left text-[12px] text-ink' },
            sheet.rows.length > 0 ? createElement('thead', { className: 'bg-surface-muted' }, createElement('tr', null,
              ...sheet.rows[0].map((cell, index) => createElement('th', { className: 'border-b border-line-soft px-3 py-2 font-semibold', key: `${sheet.name}-head-${index}` }, cell)))) : null,
            sheet.rows.length > 1 ? createElement('tbody', null, ...sheet.rows.slice(1).map((row, rowIndex) => createElement('tr', { className: 'border-b border-line-soft last:border-b-0', key: `${sheet.name}-row-${rowIndex}` },
              ...row.map((cell, cellIndex) => createElement('td', { className: 'px-3 py-2 align-top', key: `${sheet.name}-${rowIndex}-${cellIndex}` }, cell))))) : null,
          ),
        ),
      )),
    );
  }
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
  if (kind === 'mermaid') return createElement(MermaidPreview, { text: text || '', fileName });
  return createElement('iframe', {
    className: 'wk-preview-pdf',
    src: url,
    title: `${fileName || 'Document'} PDF preview`,
  });
}

function MermaidPreview({ text, fileName }: { text: string; fileName?: string }): ReactElement {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!root.current || typeof document === 'undefined') return;
    const code = document.createElement('code');
    code.textContent = text;
    const source = document.createElement('pre');
    source.dataset.markdownDiagram = 'mermaid';
    source.append(code);
    root.current.replaceChildren(source);
    void hydrateMermaidBlocksWithBrowserDefaults(root.current, 'wk-document-mermaid').catch(() => {});
  }, [text]);
  return createElement('div', { ref: root, className: 'wk-preview-mermaid min-h-16 overflow-auto', 'aria-label': `${fileName || 'Document'} Mermaid diagram` });
}

export function buildDocumentPreview(document: KnowledgeDocument, previewPath: string): KnowledgeDocumentPreviewModel {
  const fileName = document.file_name || document.title || 'document';
  const status = previewStatus(document);
  const kind = previewKindForDocument(fileName);
  const inline = isInlinePreviewKind(kind);
  const availability = status.kind === 'processing'
    ? { kind: 'processing' as const, label: status.label }
    : status.kind === 'unavailable'
      ? { kind: 'error' as const, label: status.label }
      : !inline
        ? { kind: 'unsupported' as const, label: 'Unsupported file type' }
        : { kind: 'ready' as const, label: status.label };
  return { kind, availability, ready: status.kind === 'ready', downloadOnly: !inline, path: previewPath, fileName };
}
