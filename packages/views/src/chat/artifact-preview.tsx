import { useEffect, useState } from 'react';
import type { ChatArtifact } from '@weknora/domain/chat/artifacts';
import { renderChatMarkdown } from './markdown.ts';
import type { ChatCopyTable } from './chat-copy.ts';

export type ArtifactPreviewKind = 'text' | 'markdown' | 'image' | 'pdf' | 'download-only';

export interface ArtifactPreviewPayload {
  body: string | Blob | ArrayBuffer;
  contentType?: string;
}

export interface ArtifactPreviewModel {
  kind: ArtifactPreviewKind;
  label: string;
}

export const ARTIFACT_PREVIEW_MIN_WIDTH = 520;
export const ARTIFACT_PREVIEW_DEFAULT_WIDTH = 760;

export function clampArtifactPreviewWidth(width: number, viewportWidth: number): number {
  const safeViewport = Number.isFinite(viewportWidth) && viewportWidth > 0 ? viewportWidth : 1440;
  const maxWidth = Math.min(1600, Math.max(ARTIFACT_PREVIEW_MIN_WIDTH, Math.floor(safeViewport * 0.95)));
  return Math.max(ARTIFACT_PREVIEW_MIN_WIDTH, Math.min(maxWidth, Math.round(width)));
}

export function formatArtifactSize(size: number | undefined): string {
  if (!size || size < 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return unit === 0 ? `${value} ${units[unit]}` : `${value.toFixed(1)} ${units[unit]}`;
}

export function formatArtifactDateTime(raw: string | undefined): string {
  if (!raw) return '—';
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) return raw;
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function extension(fileName: string): string {
  return fileName.trim().toLowerCase().split('.').pop() ?? '';
}

export function artifactPreviewModel(artifact: Pick<ChatArtifact, 'fileName' | 'fileType'>, copy?: ChatCopyTable): ArtifactPreviewModel {
  const type = artifact.fileType?.trim().toLowerCase() ?? '';
  const ext = extension(artifact.fileName);
  if (type === 'text/html' || type === 'application/xhtml+xml' || ext === 'html' || ext === 'htm') return { kind: 'download-only', label: copy?.artifactPreviewDownloadOnly ?? 'Download to view' };
  if (type === 'application/pdf' || ext === 'pdf') return { kind: 'pdf', label: copy?.artifactPreviewPdf ?? 'PDF preview' };
  if (type.startsWith('image/') && !type.startsWith('image/svg')) return { kind: 'image', label: copy?.artifactPreviewImage ?? 'Image preview' };
  if (ext === 'md' || ext === 'markdown' || type === 'text/markdown') return { kind: 'markdown', label: copy?.artifactPreviewMarkdown ?? 'Markdown preview' };
  if (type.startsWith('text/') || ['csv', 'tsv', 'txt', 'log', 'json', 'xml', 'yaml', 'yml', 'mmd', 'mermaid'].includes(ext)) return { kind: 'text', label: copy?.artifactPreviewText ?? 'Text preview' };
  return { kind: 'download-only', label: copy?.artifactPreviewDownloadOnly ?? 'Download to view' };
}

function payloadBlob(payload: ArtifactPreviewPayload): Blob {
  if (typeof Blob !== 'undefined' && payload.body instanceof Blob) return payload.body;
  return new Blob([payload.body], { type: payload.contentType || 'application/octet-stream' });
}

export async function readArtifactPreviewText(payload: ArtifactPreviewPayload): Promise<string> {
  return payloadBlob(payload).text();
}

function PreviewBody({ model, fileName, payload, url, text }: { model: ArtifactPreviewModel; fileName: string; payload: ArtifactPreviewPayload; url: string; text: string }) {
  if (model.kind === 'image') return <img className="wk-chat-artifact-preview-image" src={url} alt={fileName} />;
  if (model.kind === 'pdf') return <iframe className="wk-chat-artifact-preview-pdf" src={url} title={`${fileName} PDF preview`} sandbox="" />;
  if (model.kind === 'markdown') return <div className="wk-chat-artifact-preview-markdown" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(text) }} />;
  return <pre className="wk-chat-artifact-preview-text">{text}</pre>;
}

export interface ArtifactPreviewProps {
  artifact: Pick<ChatArtifact, 'fileName' | 'fileType'>;
  payload?: ArtifactPreviewPayload;
  loading?: boolean;
  error?: string;
  onClose(): void;
  onDownload?(): void;
  copy?: ChatCopyTable;
  showHeader?: boolean;
}

export function ArtifactPreview({ artifact, payload, loading = false, error, onClose, onDownload, copy, showHeader = true }: ArtifactPreviewProps) {
  const model = artifactPreviewModel(artifact, copy);
  const [url, setUrl] = useState('');
  const [text, setText] = useState('');
  const [payloadError, setPayloadError] = useState('');

  useEffect(() => {
    if (!payload || (model.kind !== 'image' && model.kind !== 'pdf')) {
      setUrl('');
      return;
    }
    if (typeof URL === 'undefined' || typeof URL.createObjectURL !== 'function') return;
    const objectUrl = URL.createObjectURL(payloadBlob(payload));
    setUrl(objectUrl);
    return () => URL.revokeObjectURL(objectUrl);
  }, [model.kind, payload]);

  useEffect(() => {
    if (!payload || (model.kind !== 'text' && model.kind !== 'markdown')) {
      setText('');
      setPayloadError('');
      return;
    }
    let active = true;
    setText('');
    setPayloadError('');
    void readArtifactPreviewText(payload)
      .then((value) => { if (active) setText(value); })
      .catch((cause: unknown) => {
        if (!active) return;
        setPayloadError(cause instanceof Error && cause.message ? cause.message : 'Preview failed');
      });
    return () => { active = false; };
  }, [model.kind, payload]);

  return <section className={`wk-chat-artifact-preview${showHeader ? '' : ' wk-chat-artifact-preview--embedded'}`} role={showHeader ? 'dialog' : 'document'} aria-label={`${artifact.fileName} preview`}>
    {showHeader ? <header><h3>{artifact.fileName}</h3><button type="button" onClick={onClose}>{copy?.artifactPreviewBack ?? 'Back'}</button>{onDownload ? <button type="button" onClick={onDownload}>{copy?.artifactPreviewDownload ?? 'Download'}</button> : null}</header> : null}
    {loading ? <p role="status">{copy?.artifactPreviewLoading ?? 'Loading preview…'}</p> : error || payloadError ? <p role="alert">{error || payloadError}</p> : model.kind === 'download-only' ? <p>{model.label}</p> : payload ? <PreviewBody model={model} fileName={artifact.fileName} payload={payload} url={url} text={text} /> : <p>{model.label}</p>}
  </section>;
}
