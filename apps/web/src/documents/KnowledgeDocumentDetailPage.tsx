import { useEffect, useState } from 'react';
import type { KnowledgeDocument, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { buildDocumentPreview, DocumentPreviewContent, isInlinePreviewKind, previewBodyAsBlob, readPreviewText, type InlinePreviewKind } from './preview.ts';

interface KnowledgeDocumentDetailPageProps {
  client: WeKnoraClient;
  documentId: string;
  onBack: () => void;
}

export function KnowledgeDocumentDetailPage({ client, documentId, onBack }: KnowledgeDocumentDetailPageProps) {
  const [state, setState] = useState<{ status: 'loading' } | { status: 'success'; document: KnowledgeDocument } | { status: 'error'; message: string }>({ status: 'loading' });
  useEffect(() => {
    let active = true;
    setState({ status: 'loading' });
    void client.knowledgeBases.documents.get(documentId).then((document) => { if (active) setState({ status: 'success', document }); }).catch((error: unknown) => { if (active) setState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load document' }); });
    return () => { active = false; };
  }, [client, documentId]);

  return <main className="wk-page wk-document-detail-page"><header className="wk-header"><div><p className="wk-eyebrow">Document detail</p><h1>{state.status === 'success' ? state.document.file_name || state.document.title || documentId : documentId}</h1></div><Button type="button" onClick={onBack}>Back to documents</Button></header>
    {state.status === 'loading' ? <Status>Loading document…</Status> : null}
    {state.status === 'error' ? <Status tone="error">{state.message}</Status> : null}
    {state.status === 'success' ? <DocumentDetail client={client} document={state.document} previewPath={client.knowledgeBases.documents.previewPath(documentId)} downloadPath={client.knowledgeBases.documents.downloadPath(documentId)} /> : null}
  </main>;
}

type PreviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'text'; text: string }
  | { status: 'blob'; url: string }
  | { status: 'error'; message: string };

function detailError(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load document bytes';
}

function DocumentDetail({ document, client, previewPath, downloadPath }: { document: KnowledgeDocument; client: WeKnoraClient; previewPath: string; downloadPath: string }) {
  const model = buildDocumentPreview(document, previewPath);
  const [previewState, setPreviewState] = useState<PreviewState>({ status: 'idle' });
  const [downloadState, setDownloadState] = useState<'idle' | 'loading' | 'error'>('idle');

  useEffect(() => {
    if (!model.ready || !isInlinePreviewKind(model.kind)) {
      setPreviewState({ status: 'idle' });
      return;
    }
    const controller = new AbortController();
    let active = true;
    let objectUrl: string | undefined;
    setPreviewState({ status: 'loading' });
    void client.knowledgeBases.documents.preview(document.id, controller.signal).then(async (response) => {
      if (!active) return;
      const contentType = response.contentType || response.headers['content-type'];
      if (model.kind === 'text' || model.kind === 'markdown') {
        setPreviewState({ status: 'text', text: await readPreviewText(response.body) });
        return;
      }
      objectUrl = URL.createObjectURL(previewBodyAsBlob(response.body, contentType));
      setPreviewState({ status: 'blob', url: objectUrl });
    }).catch((error: unknown) => {
      if (active && !(error instanceof Error && error.name === 'AbortError')) setPreviewState({ status: 'error', message: detailError(error) });
    });
    return () => {
      active = false;
      controller.abort();
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [client, document.id, model.kind, model.ready]);

  async function download() {
    setDownloadState('loading');
    try {
      const response = await client.knowledgeBases.documents.download(document.id);
      const contentType = response.contentType || response.headers['content-type'];
      const url = URL.createObjectURL(previewBodyAsBlob(response.body, contentType));
      const anchor = window.document.createElement('a');
      anchor.href = url;
      anchor.download = model.fileName;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
      setDownloadState('idle');
    } catch (error: unknown) {
      setDownloadState('error');
      setPreviewState({ status: 'error', message: detailError(error) });
    }
  }

  const inlineKind: InlinePreviewKind | undefined = isInlinePreviewKind(model.kind) ? model.kind : undefined;
  return <Card><dl className="wk-document-metadata"><div><dt>Status</dt><dd>{String(document.parse_status || 'unknown')}</dd></div><div><dt>Source</dt><dd>{String(document.source || 'file')}</dd></div><div><dt>Folder</dt><dd>{String(document.folder_path || 'Root')}</dd></div><div><dt>Type</dt><dd>{String(document.file_type || model.kind)}</dd></div></dl>
    {!model.ready ? <Status tone="warning">Preview is unavailable until processing reaches completed. Current status is authoritative.</Status> : model.downloadOnly ? <Status>Inline preview is unavailable for this file type. Download the original file instead.</Status> : previewState.status === 'loading' ? <Status>Loading preview…</Status> : previewState.status === 'error' ? <Status tone="error">{previewState.message}</Status> : previewState.status === 'text' && inlineKind ? <DocumentPreviewContent kind={inlineKind} text={previewState.text} fileName={model.fileName} /> : previewState.status === 'blob' && inlineKind ? <DocumentPreviewContent kind={inlineKind} url={previewState.url} fileName={model.fileName} /> : null}
    <p><Button type="button" loading={downloadState === 'loading'} onClick={() => void download()}>Download {model.fileName}</Button>{downloadState === 'error' ? <Status tone="error">Download failed.</Status> : null}</p>
  </Card>;
}
