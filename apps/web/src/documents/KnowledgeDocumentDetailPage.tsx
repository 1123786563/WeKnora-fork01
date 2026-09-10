import { useEffect, useState } from 'react';
import type { KnowledgeDocument, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { buildDocumentPreview } from './preview.ts';

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
    {state.status === 'success' ? <DocumentDetail document={state.document} previewPath={client.knowledgeBases.documents.previewPath(documentId)} downloadPath={client.knowledgeBases.documents.downloadPath(documentId)} /> : null}
  </main>;
}

function DocumentDetail({ document, previewPath, downloadPath }: { document: KnowledgeDocument; previewPath: string; downloadPath: string }) {
  const model = buildDocumentPreview(document, previewPath);
  return <Card><dl className="wk-document-metadata"><div><dt>Status</dt><dd>{String(document.parse_status || 'unknown')}</dd></div><div><dt>Source</dt><dd>{String(document.source || 'file')}</dd></div><div><dt>Folder</dt><dd>{String(document.folder_path || 'Root')}</dd></div><div><dt>Type</dt><dd>{String(document.file_type || model.kind)}</dd></div></dl>
    {!model.ready ? <Status tone="warning">Preview is unavailable until processing reaches completed. Current status is authoritative.</Status> : model.kind === 'unsupported' ? <Status>Preview is not supported for this file type. Download the original file instead.</Status> : <div className="wk-preview-placeholder" data-preview-path={previewPath}>Preview available for {model.fileName} ({model.kind}).</div>}
    <p><a href={downloadPath} download={model.fileName}>Download {model.fileName}</a></p>
  </Card>;
}
