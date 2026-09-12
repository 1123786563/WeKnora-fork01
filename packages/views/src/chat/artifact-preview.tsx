import { useEffect, useState } from 'react';
import type { ChatArtifact } from '@weknora/domain/chat/artifacts';
import { renderChatMarkdown } from './markdown.ts';

export type ArtifactPreviewKind = 'text' | 'markdown' | 'image' | 'pdf' | 'download-only';

export interface ArtifactPreviewPayload {
  body: string | Blob | ArrayBuffer;
  contentType?: string;
}

export interface ArtifactPreviewModel {
  kind: ArtifactPreviewKind;
  label: string;
}

function extension(fileName: string): string {
  return fileName.trim().toLowerCase().split('.').pop() ?? '';
}

export function artifactPreviewModel(artifact: Pick<ChatArtifact, 'fileName' | 'fileType'>): ArtifactPreviewModel {
  const type = artifact.fileType?.trim().toLowerCase() ?? '';
  const ext = extension(artifact.fileName);
  if (type === 'text/html' || type === 'application/xhtml+xml' || ext === 'html' || ext === 'htm') return { kind: 'download-only', label: 'Download to view' };
  if (type === 'application/pdf' || ext === 'pdf') return { kind: 'pdf', label: 'PDF preview' };
  if (type.startsWith('image/') && !type.startsWith('image/svg')) return { kind: 'image', label: 'Image preview' };
  if (ext === 'md' || ext === 'markdown' || type === 'text/markdown') return { kind: 'markdown', label: 'Markdown preview' };
  if (type.startsWith('text/') || ['csv', 'tsv', 'txt', 'log', 'json', 'xml', 'yaml', 'yml', 'mmd', 'mermaid'].includes(ext)) return { kind: 'text', label: 'Text preview' };
  return { kind: 'download-only', label: 'Download to view' };
}

function payloadBlob(payload: ArtifactPreviewPayload): Blob {
  if (typeof Blob !== 'undefined' && payload.body instanceof Blob) return payload.body;
  return new Blob([payload.body], { type: payload.contentType || 'application/octet-stream' });
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
}

export function ArtifactPreview({ artifact, payload, loading = false, error, onClose, onDownload }: ArtifactPreviewProps) {
  const model = artifactPreviewModel(artifact);
  const [url, setUrl] = useState('');
  const [text, setText] = useState('');

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
      return;
    }
    let active = true;
    void payloadBlob(payload).text().then((value) => { if (active) setText(value); });
    return () => { active = false; };
  }, [model.kind, payload]);

  return <section className="wk-chat-artifact-preview" role="dialog" aria-label={`${artifact.fileName} preview`}>
    <header><h3>{artifact.fileName}</h3><button type="button" onClick={onClose}>Back</button>{onDownload ? <button type="button" onClick={onDownload}>Download</button> : null}</header>
    {loading ? <p role="status">Loading preview…</p> : error ? <p role="alert">{error}</p> : model.kind === 'download-only' ? <p>{model.label}</p> : payload ? <PreviewBody model={model} fileName={artifact.fileName} payload={payload} url={url} text={text} /> : <p>{model.label}</p>}
  </section>;
}
