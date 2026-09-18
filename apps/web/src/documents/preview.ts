import type { KnowledgeDocument } from '@weknora/api-client';
import { createElement, useEffect, useMemo, useRef, type MouseEvent as ReactMouseEvent, type ReactElement } from 'react';
import * as XLSX from 'xlsx';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { attachMermaidViewerToolbar, hydrateMermaidBlocksWithBrowserDefaults, type MermaidViewerToolbarLabels } from '@weknora/views/chat/mermaid';
import { previewKindForFile, type KnowledgePreviewKind } from '@weknora/domain/knowledge/preview';

export interface KnowledgeDocumentPreviewModel {
  kind: DocumentPreviewKind;
  ready: boolean;
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
  mermaidLabels,
  mermaidLoader,
}: {
  kind: InlinePreviewKind | 'mermaid';
  text?: string;
  url?: string;
  fileName?: string;
  spreadsheet?: SpreadsheetPreviewModel;
  mermaidLabels?: DocumentMermaidLabels;
  mermaidLoader?: DocumentMermaidLoader;
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
  if (kind === 'mermaid') return createElement(MermaidPreview, { text: text || '', fileName, labels: mermaidLabels, loader: mermaidLoader });
  return createElement('iframe', {
    className: 'wk-preview-pdf',
    src: url,
    title: `${fileName || 'Document'} PDF preview`,
  });
}

// ─── R465/A1 — mermaid fullscreen viewer (Vue document-preview parity) ───────
//
// Vue frontend/src/components/document-preview.vue renders .mmd/.mermaid files
// as a sanitized SVG and opens utils/mermaidViewer.ts openMermaidFullscreen on
// a click: a fixed overlay with the toolbar zoomIn/zoomOut/reset/download plus
// a close control (Vue order), 0.2 zoom stepping, drag panning and Escape /
// overlay-click dismissal. The zoom/download mechanics ride the shared views
// engine (packages/views/src/chat/mermaid-viewer.ts, R463) so this face only
// owns the overlay, the close control and the locale labels.

/** Toolbar + close + dialog-name copy (Vue i18n mermaid.* strings). */
export interface DocumentMermaidLabels extends MermaidViewerToolbarLabels {
  close: string;
  /** Vue mermaid.expand — used as the fullscreen dialog's accessible name. */
  expand: string;
}

/** Injectable hydrator for tests (defaults to the shared views loader). */
export type DocumentMermaidLoader = typeof hydrateMermaidBlocksWithBrowserDefaults;

// Vue close icon (frontend/src/utils/mermaidViewer.ts L102).
const MERMAID_CLOSE_ICON =
  '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';

/**
 * Open the fullscreen mermaid viewer for one diagram (Vue
 * openMermaidFullscreen): overlay + centered stage carrying the sanitized SVG,
 * shared-engine toolbar (zoomIn, zoomOut, reset, download) with the close
 * control appended last. Dismiss on close, overlay click, or Escape.
 */
export function openDocumentMermaidFullscreen(svgHtml: string, labels: DocumentMermaidLabels): void {
  if (typeof document === 'undefined') return;
  const overlay = document.createElement('div');
  overlay.className = 'wk-document-mermaid-viewer';
  overlay.setAttribute('role', 'dialog');
  overlay.setAttribute('aria-label', labels.expand);
  // Vue overlay chrome (mermaidViewer.ts L81) with the engine's flex-centered
  // stage layout (the toolbar transforms the stage from its centered slot).
  overlay.style.cssText =
    'position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;padding:24px;background:rgba(0,0,0,0.65);overflow:hidden;cursor:grab;';

  const stage = document.createElement('div');
  stage.className = 'wk-document-mermaid-viewer__stage';
  stage.innerHTML = svgHtml;
  const svgEl = stage.querySelector('svg');
  if (svgEl) {
    // Vue content chrome (mermaidViewer.ts L107-113).
    svgEl.style.display = 'block';
    svgEl.setAttribute('draggable', 'false');
  }
  stage.style.cssText =
    'max-width:100%;max-height:100%;overflow:auto;background:#fff;border-radius:12px;padding:32px;box-shadow:0 8px 32px rgba(0,0,0,0.2);cursor:default;';

  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.className = 'wk-document-mermaid-viewer__close';
  closeBtn.setAttribute('aria-label', labels.close);
  closeBtn.setAttribute('title', labels.close);
  closeBtn.innerHTML = MERMAID_CLOSE_ICON;
  closeBtn.style.cssText =
    'display:flex;align-items:center;justify-content:center;width:36px;height:36px;border:1px solid #e5e7eb;border-radius:6px;background:rgba(255,255,255,0.95);color:#6b7280;cursor:pointer;padding:0;box-shadow:0 2px 8px rgba(0,0,0,0.15);';

  // Vue order (mermaidViewer.ts L103): zoomIn, zoomOut, reset, download, close.
  const { toolbar, detach } = attachMermaidViewerToolbar(overlay, stage, labels);
  toolbar.appendChild(closeBtn);

  const onKeydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      close();
    }
  };
  const close = () => {
    detach();
    overlay.remove();
    document.removeEventListener('keydown', onKeydown, true);
  };
  closeBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    close();
  });
  overlay.addEventListener('click', (event) => {
    if (event.target === overlay) close();
  });
  document.addEventListener('keydown', onKeydown, true);

  overlay.append(toolbar, stage);
  document.body.appendChild(overlay);
}

function MermaidPreview({
  text,
  fileName,
  labels,
  loader = hydrateMermaidBlocksWithBrowserDefaults,
}: {
  text: string;
  fileName?: string;
  labels?: DocumentMermaidLabels;
  loader?: DocumentMermaidLoader;
}): ReactElement {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!root.current || typeof document === 'undefined') return;
    const code = document.createElement('code');
    code.textContent = text;
    const source = document.createElement('pre');
    source.dataset.markdownDiagram = 'mermaid';
    source.append(code);
    root.current.replaceChildren(source);
    void loader(root.current, 'wk-document-mermaid').catch(() => {});
  }, [text, loader]);
  // Vue document-preview.vue: the whole .preview-mermaid surface is clickable
  // (@click="openMermaid") and opens the fullscreen viewer only when a
  // rendered svg exists (`if (!svg) return`).
  const openViewer = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (!labels) return;
    const svg = event.currentTarget.querySelector('svg');
    if (!svg) return;
    openDocumentMermaidFullscreen((svg as SVGElement).outerHTML, labels);
  };
  return createElement('div', {
    ref: root,
    className: 'wk-preview-mermaid min-h-16 cursor-pointer overflow-auto',
    'aria-label': `${fileName || 'Document'} Mermaid diagram`,
    tabIndex: 0,
    onClick: openViewer,
  });
}

const AUDIO_PREVIEW_EXTENSIONS = new Set(['mp3', 'wav', 'ogg', 'oga', 'm4a', 'aac', 'flac']);

// ─── R466/A1 — merged/chunks inline mermaid hydration (Vue doc-content parity) ─
//
// Vue doc-content renders the 全文 (merged) view and every chunk body through
// processMarkdown — ```mermaid fences become diagram nodes — then
// runMarkdownPostRenderPipeline scans the markdown root, mermaid.run()s every
// diagram and bindMermaidClickEvents marks each rendered container clickable
// (cursor pointer, stopPropagation) so a click opens
// utils/mermaidViewer openMermaidFullscreen(svg.outerHTML). The pipeline reruns
// whenever the markdown body changes (chunk page turn / edit / view switch).
//
// React rides the shared views engine: renderChatMarkdown emits
// pre[data-markdown-diagram="mermaid"] and
// hydrateMermaidBlocksWithBrowserDefaults replaces it with a .wk-chat-mermaid
// figure (a failed render degrades back to the escaped code block — engine
// semantics). This face only owns the trigger and the click binding.

export interface DocumentMarkdownBodyProps {
  markdown: string;
  className?: string;
  labels: DocumentMermaidLabels;
  /** Injectable hydrator for tests (defaults to the shared views loader). */
  loader?: DocumentMermaidLoader;
}

/** Vue bindMermaidClickEvents: rendered diagrams become click-to-fullscreen. */
function bindDocumentMermaidClicks(root: HTMLElement, labels: DocumentMermaidLabels): void {
  root.querySelectorAll<HTMLElement>('.wk-chat-mermaid').forEach((figure) => {
    // Vue removes and re-adds listeners to avoid double binding; a dataset
    // flag gives the same idempotence for figures that survive a re-run.
    if (figure.dataset.mermaidFullscreenBound === 'true') return;
    figure.dataset.mermaidFullscreenBound = 'true';
    figure.style.cursor = 'pointer';
    figure.addEventListener('click', (event) => {
      event.stopPropagation();
      const svg = figure.querySelector('svg');
      if (svg) openDocumentMermaidFullscreen(svg.outerHTML, labels);
    });
  });
}

/**
 * Markdown body for the merged/chunks views (Vue md-content): renders
 * renderChatMarkdown HTML, then after render hydrates inline mermaid blocks
 * with the shared engine and binds the Vue click-to-fullscreen behavior.
 * Bodies without mermaid fences skip the hydration pass entirely.
 */
export function DocumentMarkdownBody({ markdown, className, labels, loader = hydrateMermaidBlocksWithBrowserDefaults }: DocumentMarkdownBodyProps): ReactElement {
  const root = useRef<HTMLDivElement>(null);
  const html = useMemo(() => renderChatMarkdown(markdown), [markdown]);
  useEffect(() => {
    const host = root.current;
    if (!host || typeof document === 'undefined') return;
    // Vue renderMermaidDiagrams only runs when the scan finds diagram nodes.
    if (!host.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let cancelled = false;
    void loader(host, 'wk-document-mermaid').then(() => {
      if (cancelled || !host.isConnected) return;
      bindDocumentMermaidClicks(host, labels);
    }).catch(() => {
      // Hydration failure keeps the escaped code block visible (engine fallback).
    });
    return () => { cancelled = true; };
  }, [html, labels, loader]);
  return createElement('div', { ref: root, className, dangerouslySetInnerHTML: { __html: html } });
}

/**
 * Vue doc-content#canPreview: only `type === 'file'` knowledge entries with a
 * resolvable preview extension, and never audio — Vue hides the preview tab
 * for audio and embeds the player above the content views instead. The
 * extension resolution mirrors Vue resolveFilePreviewExt: an explicit
 * file_type wins over the filename suffix. `source` is present on real
 * payloads but deliberately ignored — Vue gates on `type`, and backend file
 * knowledge carries `source: ""`.
 */
export function canPreviewDocument(document: { type?: string; source?: string; file_name?: string; title?: string; file_type?: string }): boolean {
  if (document.type !== 'file') return false;
  const normalizedType = (document.file_type || '').trim().replace(/^\./, '').toLowerCase();
  const fileName = document.file_name || document.title || '';
  const dot = fileName.lastIndexOf('.');
  const fromName = dot < 0 || dot === fileName.length - 1 ? '' : fileName.slice(dot + 1).toLowerCase();
  const extension = normalizedType || fromName;
  if (!extension || AUDIO_PREVIEW_EXTENSIONS.has(extension)) return false;
  return isInlinePreviewKind(previewKindForDocument(`doc.${extension}`));
}

/**
 * Vue doc-content#buildPreviewModel never consults parse_status: the preview
 * tab is gated by canPreview() (file type + resolvable extension, never
 * audio) and the embedded audio player loads for audio files regardless of
 * processing state. `ready` therefore mirrors exactly those conditions —
 * `type === 'file'` plus an inline-previewable extension — so a pending or
 * failed parse never hides preview content or the audio player.
 */
export function buildDocumentPreview(document: KnowledgeDocument, previewPath: string): KnowledgeDocumentPreviewModel {
  const fileName = document.file_name || document.title || 'document';
  const kind = previewKindForDocument(fileName);
  return { kind, ready: document.type === 'file' && isInlinePreviewKind(kind), path: previewPath, fileName };
}
