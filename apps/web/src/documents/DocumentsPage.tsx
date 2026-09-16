import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { KnowledgeDocument, KnowledgeDocumentListParams, KnowledgeDocumentUploadInput } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { buildProcessingTimeline, canDocumentAction, getDocumentStatus, normalizeDocumentPage, toggleDocumentSelection, validateUpload, type DocumentPage } from './model.ts';

export interface DocumentsApi {
  list: (knowledgeBaseId: string, params?: KnowledgeDocumentListParams) => Promise<unknown>;
  get?: (documentId: string) => Promise<unknown>;
  preview?: (documentId: string) => Promise<unknown>;
  upload?: (knowledgeBaseId: string, input: KnowledgeDocumentUploadInput) => Promise<unknown>;
  updateTags?: (updates: Record<string, string[]>) => Promise<unknown>;
  reparse?: (knowledgeBaseId: string, ids: string[]) => Promise<unknown>;
  cancel?: (documentId: string) => Promise<unknown>;
  delete?: (knowledgeBaseId: string, ids: string[]) => Promise<unknown>;
  download?: (documentId: string) => Promise<unknown>;
}

export interface DocumentsPageProps {
  knowledgeBaseId: string;
  api: DocumentsApi;
  canView?: boolean;
  canEdit?: boolean;
  canDownload?: boolean;
  pageSize?: number;
}

type AsyncState = 'idle' | 'submitting';

function titleOf(document: KnowledgeDocument): string {
  return String(document.title || document.file_name || 'Untitled document');
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'The document request failed';
}

export function DocumentsPage({ knowledgeBaseId, api, canView = true, canEdit = false, canDownload = true, pageSize = 20 }: DocumentsPageProps) {
  const [page, setPage] = useState<DocumentPage | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [active, setActive] = useState<KnowledgeDocument | null>(null);
  const [detail, setDetail] = useState<unknown>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadState, setUploadState] = useState<AsyncState>('idle');
  const [actionState, setActionState] = useState<AsyncState>('idle');
  const [files, setFiles] = useState<File[]>([]);
  const [tagInput, setTagInput] = useState('');
  const fileInput = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    setLoading(true); setError(null);
    try {
      const result = normalizeDocumentPage(await api.list(knowledgeBaseId, { page: pageNumber, page_size: pageSize, keyword: keyword || undefined, parse_status: statusFilter || undefined }));
      setPage(result); setSelected(new Set());
    } catch (cause) { setError(errorMessage(cause)); }
    finally { setLoading(false); }
  }, [api, knowledgeBaseId, keyword, pageNumber, pageSize, statusFilter]);

  useEffect(() => { void load(); }, [load]);

  const items = page?.items ?? [];
  const ids = useMemo(() => items.map((item) => item.id), [items]);
  const submit = async (operation: () => Promise<unknown>, success: string) => {
    if (actionState === 'submitting') return;
    setActionState('submitting'); setError(null);
    try { await operation(); setError(null); window.setTimeout(() => undefined, 0); await load(); }
    catch (cause) { setError(`${success}: ${errorMessage(cause)}`); }
    finally { setActionState('idle'); }
  };

  const openDocument = async (document: KnowledgeDocument) => {
    setActive(document); setDetail(null); setPreview(null);
    if (!api.get) return;
    try { setDetail(await api.get(document.id)); } catch (cause) { setError(errorMessage(cause)); }
  };

  const openPreview = async () => {
    if (!active || !api.preview) return;
    try {
      const value = await api.preview(active.id);
      setPreview(typeof value === 'string' ? value : JSON.stringify(value, null, 2));
    } catch (cause) { setError(errorMessage(cause)); }
  };

  const download = async (documentId: string) => {
    if (!api.download) return;
    try { await api.download(documentId); }
    catch (cause) { setError(`Download failed: ${errorMessage(cause)}`); }
  };

  const upload = async () => {
    const validation = validateUpload(files);
    if (!validation.valid || !api.upload) { setUploadError(validation.errors[0] ?? 'upload-unavailable'); return; }
    if (uploadState === 'submitting') return;
    setUploadState('submitting'); setUploadError(null);
    try {
      for (const file of files) await api.upload(knowledgeBaseId, { file, fileName: file.name, tag_ids: tagInput.split(',').map((tag) => tag.trim()).filter(Boolean) });
      setFiles([]); if (fileInput.current) fileInput.current.value = ''; await load();
    } catch (cause) { setUploadError(errorMessage(cause)); }
    finally { setUploadState('idle'); }
  };

  const selectedIds = [...selected];
  const mutateSelected = (operation: () => Promise<unknown>, message: string) => {
    if (!canEdit || selectedIds.length === 0) return;
    if (!window.confirm(message)) return;
    void submit(operation, 'Could not complete batch operation');
  };

  if (!canView) return <main className="wk-page"><Card><Status tone="error">You do not have permission to view these documents.</Status></Card></main>;

  return <main className="wk-page" aria-label="Documents">
    <header className="wk-header">
      <div><p className="wk-eyebrow">Knowledge base</p><h1>Documents</h1><p className="wk-muted">Upload, inspect, tag, preview, and process documents.</p></div>
      <Button type="button" onClick={() => void load} disabled={loading}>Reload</Button>
    </header>
    <Card>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 16 }}>
        <input aria-label="Search documents" value={keyword} onChange={(event) => { setPageNumber(1); setKeyword(event.target.value); }} placeholder="Search documents" />
        <select aria-label="Filter by processing status" value={statusFilter} onChange={(event) => { setPageNumber(1); setStatusFilter(event.target.value); }}>
          <option value="">All statuses</option><option value="processing">Processing</option><option value="completed">Completed</option><option value="failed">Failed</option><option value="cancelled">Cancelled</option>
        </select>
      </div>
      {canEdit ? <section aria-label="Upload documents" style={{ border: '1px dashed #b8c5d6', padding: 12, marginBottom: 16 }}>
        <input ref={fileInput} type="file" multiple onChange={(event) => setFiles([...event.target.files ?? []])} />
        <input aria-label="Upload tags" value={tagInput} onChange={(event) => setTagInput(event.target.value)} placeholder="Tags, comma separated" />
        <Button type="button" onClick={() => void upload()} disabled={uploadState === 'submitting'}>{uploadState === 'submitting' ? 'Uploading…' : 'Upload'}</Button>
        {uploadError ? <Status tone="error">Upload failed: {uploadError}</Status> : null}
      </section> : <Status>Read-only access: upload and document mutations are unavailable.</Status>}
      {selected.size > 0 ? <div role="region" aria-label="Batch document actions" style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12 }}>
        <span>{selected.size} selected</span>
        <Button type="button" disabled={actionState === 'submitting'} onClick={() => mutateSelected(() => api.reparse ? api.reparse(knowledgeBaseId, selectedIds) : Promise.reject(new Error('Reparse unavailable')), 'Rebuild selected documents?')}>Rebuild</Button>
        <Button type="button" disabled={actionState === 'submitting'} onClick={() => mutateSelected(() => api.delete ? api.delete(knowledgeBaseId, selectedIds) : Promise.reject(new Error('Delete unavailable')), 'Delete selected documents?')}>Delete</Button>
        <Button type="button" onClick={() => setSelected(new Set())}>Clear</Button>
      </div> : null}
      {loading ? <Status>Loading documents…</Status> : null}
      {!loading && error ? <><Status tone="error">{error}</Status><Button type="button" onClick={() => void load}>Try again</Button></> : null}
      {!loading && !error && items.length === 0 ? <Status>No documents found.</Status> : null}
      {!loading && !error && items.length > 0 ? <>
        <div style={{ overflowX: 'auto' }}><table style={{ width: '100%', borderCollapse: 'collapse' }}><thead><tr><th><input type="checkbox" aria-label="Select all documents" checked={selected.size === items.length} onChange={(event) => setSelected(toggleDocumentSelection(selected, 'all', event.target.checked, ids))} /></th><th>Name</th><th>Status</th><th>Updated</th><th>Actions</th></tr></thead><tbody>{items.map((document) => {
          const status = getDocumentStatus({ ...document, id: document.id });
          return <tr key={document.id}><td><input type="checkbox" aria-label={`Select ${titleOf(document)}`} checked={selected.has(document.id)} onChange={(event) => setSelected(toggleDocumentSelection(selected, document.id, event.target.checked))} /></td><td><button type="button" onClick={() => void openDocument(document)} style={{ border: 0, background: 'none', padding: 0, cursor: 'pointer' }}>{titleOf(document)}</button><div className="wk-muted">{String(document.folder_path || document.file_type || '')}</div></td><td><Status tone={status.tone === 'danger' ? 'error' : status.tone === 'success' ? 'success' : 'neutral'}>{status.key}{status.busy ? '…' : ''}</Status></td><td>{String(document.updated_at || '—')}</td><td style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}><Button type="button" onClick={() => void openDocument(document)}>Details</Button>{canDocumentAction('download', { canView, canEdit }, document) && canDownload && api.download ? <Button type="button" onClick={() => void download(document.id)}>Download</Button> : null}{canDocumentAction('cancel', { canView, canEdit }) && status.busy && api.cancel ? <Button type="button" disabled={actionState === 'submitting'} onClick={() => void submit(() => api.cancel!(document.id), 'Could not cancel processing')}>Cancel</Button> : null}{canDocumentAction('reparse', { canView, canEdit }) && api.reparse ? <Button type="button" disabled={actionState === 'submitting'} onClick={() => mutateSelected(() => api.reparse!(knowledgeBaseId, [document.id]), 'Rebuild this document?')}>Rebuild</Button> : null}</td></tr>;
        })}</tbody></table></div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 12 }}><span>{page?.total ?? items.length} total</span><span><Button type="button" disabled={pageNumber <= 1} onClick={() => setPageNumber((value) => value - 1)}>Previous</Button> <Button type="button" disabled={pageNumber * pageSize >= (page?.total ?? 0)} onClick={() => setPageNumber((value) => value + 1)}>Next</Button></span></div>
      </> : null}
    </Card>
    {active ? <div role="dialog" aria-label={`Document details: ${titleOf(active)}`} style={{ position: 'fixed', inset: 20, overflow: 'auto', background: 'white', border: '1px solid #b8c5d6', padding: 20, zIndex: 2 }}><Button type="button" onClick={() => setActive(null)}>Close</Button><h2>{titleOf(active)}</h2><p>Status: {getDocumentStatus({ ...active, id: active.id }).key}</p><pre style={{ whiteSpace: 'pre-wrap' }}>{detail ? JSON.stringify(detail, null, 2) : 'Loading details…'}</pre><div style={{ display: 'flex', gap: 8 }}><Button type="button" disabled={!api.preview} onClick={() => void openPreview()}>Preview</Button>{canEdit && api.updateTags ? <Button type="button" onClick={() => void submit(() => api.updateTags!({ [active.id]: tagInput.split(',').map((tag) => tag.trim()).filter(Boolean) }), 'Could not update tags')}>Save tags</Button> : null}</div>{preview ? <pre aria-label="Document preview" style={{ whiteSpace: 'pre-wrap', borderTop: '1px solid #dce3ed', marginTop: 12, paddingTop: 12 }}>{preview}</pre> : null}<h3>Processing timeline</h3><ol>{buildProcessingTimeline({ ...active, id: active.id }).map((step) => <li key={step.key}>{step.key}: {step.state}</li>)}</ol></div> : null}
  </main>;
}

export { normalizeDocumentPage } from './model.ts';
