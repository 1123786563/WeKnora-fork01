import { useEffect, useMemo, useRef, useState } from 'react';
import type { KnowledgeDocument, WeKnoraClient } from '@weknora/api-client';
import { processingStatusLabel, normalizeKnowledgeProcessingStatus } from '@weknora/domain/knowledge/processing';
import { flattenKnowledgeFolders as flattenFolders } from '@weknora/domain/knowledge/folders';
import { Button, Card, Status } from '@weknora/ui';
import { loadKnowledgeDocuments, type KnowledgeDocumentListState } from './list.ts';

interface KnowledgeDocumentsPageProps {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  onOpenDocument?: (document: KnowledgeDocument) => void;
}

type UploadSource = 'file' | 'url' | 'manual';

function displayName(document: KnowledgeDocument): string {
  return document.file_name || document.title || document.id;
}

function documentStatus(document: KnowledgeDocument): { label: string; tone: 'neutral' | 'success' | 'warning' | 'error' } {
  if (!document.parse_status) return { label: 'Unknown status', tone: 'warning' };
  let status: ReturnType<typeof normalizeKnowledgeProcessingStatus>;
  try { status = normalizeKnowledgeProcessingStatus(document.parse_status); }
  catch { return { label: 'Unknown status', tone: 'warning' }; }
  if (status === 'completed') return { label: processingStatusLabel(status), tone: 'success' };
  if (status === 'failed' || status === 'cancelled') return { label: processingStatusLabel(status), tone: 'error' };
  return { label: processingStatusLabel(status), tone: 'warning' };
}

function errorMessage(error: unknown): string {
  const candidate = error as { code?: unknown; message?: unknown; status?: unknown };
  if (candidate?.status === 413 || candidate?.code === 'PAYLOAD_TOO_LARGE') return 'Upload is too large (413). Choose a smaller file and retry.';
  return candidate?.message && typeof candidate.message === 'string' ? candidate.message : 'The document operation failed.';
}

export function KnowledgeDocumentsPage({ client, knowledgeBaseId, onOpenDocument }: KnowledgeDocumentsPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<KnowledgeDocumentListState>({ status: 'error', message: 'Loading…' });
  const [folderState, setFolderState] = useState<{ status: 'loading' | 'success' | 'error'; tree?: Awaited<ReturnType<typeof client.knowledgeBases.documents.folders>>; message?: string }>({ status: 'loading' });
  const [tags, setTags] = useState<Awaited<ReturnType<typeof client.knowledgeBases.documents.tags>>>([]);
  const [query, setQuery] = useState('');
  const [parseStatus, setParseStatus] = useState('');
  const [tagId, setTagId] = useState('');
  const [folderPath, setFolderPath] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [uploadSource, setUploadSource] = useState<UploadSource>('file');
  const [file, setFile] = useState<File | null>(null);
  const [url, setUrl] = useState('');
  const [manualTitle, setManualTitle] = useState('');
  const [manualContent, setManualContent] = useState('');
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const uploadController = useRef<AbortController | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const pageSize = 20;

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadKnowledgeDocuments(client, knowledgeBaseId, {
      page, page_size: pageSize, keyword: query || undefined, parse_status: parseStatus || undefined,
      tag_ids: tagId || undefined,
      folder_path: folderPath,
      folder_recursive: folderPath !== undefined,
    }).then((next) => { if (active) setState(next); });
    return () => { active = false; };
  }, [client, folderPath, knowledgeBaseId, page, parseStatus, query, reloadToken]);

  useEffect(() => {
    let active = true;
    setFolderState({ status: 'loading' });
    void Promise.all([
      client.knowledgeBases.documents.folders(knowledgeBaseId),
      client.knowledgeBases.documents.tags(knowledgeBaseId),
    ]).then(([tree, nextTags]) => {
      if (active) { setFolderState({ status: 'success', tree }); setTags(nextTags); }
    }).catch((error: unknown) => { if (active) setFolderState({ status: 'error', message: errorMessage(error) }); });
    return () => { active = false; };
  }, [client, knowledgeBaseId, reloadToken]);

  useEffect(() => { setPage(1); }, [folderPath, parseStatus, query, tagId]);

  const folders = useMemo(() => folderState.tree ? flattenFolders(folderState.tree) : [], [folderState.tree]);
  const selectedOnPage = state.status === 'success' ? state.page.items.filter((item) => selected.has(item.id)).length : 0;

  function toggleSelected(id: string) {
    setSelected((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  }

  async function upload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUploadError(null);
    setUploading(true);
    const controller = new AbortController();
    uploadController.current = controller;
    try {
      if (uploadSource === 'file') {
        if (!file) throw new Error('Choose a file before uploading.');
        await client.knowledgeBases.documents.upload(knowledgeBaseId, { file, fileName: file.name }, controller.signal);
      } else if (uploadSource === 'url') {
        if (!url.trim()) throw new Error('Enter a URL before importing.');
        await client.knowledgeBases.documents.createFromUrl(knowledgeBaseId, { url: url.trim() });
      } else {
        if (!manualTitle.trim() || !manualContent.trim()) throw new Error('Manual documents require a title and content.');
        await client.knowledgeBases.documents.createManual(knowledgeBaseId, { title: manualTitle.trim(), content: manualContent, status: 'pending' });
      }
      setFile(null); setUrl(''); setManualTitle(''); setManualContent(''); setReloadToken((value) => value + 1);
    } catch (error) { setUploadError(errorMessage(error)); }
    finally { if (uploadController.current === controller) uploadController.current = null; setUploading(false); }
  }

  async function deleteSelected() {
    if (!selected.size) return;
    setMutationError(null);
    try { await client.knowledgeBases.documents.batchDelete(knowledgeBaseId, [...selected]); setSelected(new Set()); setReloadToken((value) => value + 1); }
    catch (error) { setMutationError(errorMessage(error)); }
  }

  async function moveSelected() {
    if (!selected.size) return;
    const destination = window.prompt('Move selected documents to folder (empty for root):', folderPath ?? '');
    if (destination === null) return;
    setMutationError(null);
    try { await client.knowledgeBases.documents.moveToFolder(knowledgeBaseId, [...selected], destination.trim()); setSelected(new Set()); setReloadToken((value) => value + 1); }
    catch (error) { setMutationError(errorMessage(error)); }
  }

  async function updateSelectedTags() {
    if (!selected.size) return;
    const raw = window.prompt('Set tag IDs for selected documents (comma-separated; empty clears tags):', tagId);
    if (raw === null) return;
    setMutationError(null);
    try { await client.knowledgeBases.documents.updateTags(Object.fromEntries([...selected].map((id) => [id, raw.split(',').map((value) => value.trim()).filter(Boolean)]))); setReloadToken((value) => value + 1); }
    catch (error) { setMutationError(errorMessage(error)); }
  }

  return <main className="wk-page wk-documents-page">
    <header className="wk-header">
      <div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>Documents</h1><p className="wk-muted">Upload, organize, process and preview documents without weakening backend status semantics.</p></div>
      <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Reload</Button>
    </header>
    <Card>
      <form className="wk-upload-panel" onSubmit={upload}>
        <div className="wk-toolbar"><label>Source <select value={uploadSource} onChange={(event) => setUploadSource(event.target.value as UploadSource)}><option value="file">File</option><option value="url">URL</option><option value="manual">Manual</option></select></label>
          {uploadSource === 'file' ? <label>File <input type="file" onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></label> : null}
          {uploadSource === 'url' ? <label>URL <input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://…" /></label> : null}
          {uploadSource === 'manual' ? <><label>Title <input value={manualTitle} onChange={(event) => setManualTitle(event.target.value)} /></label><label>Content <textarea value={manualContent} onChange={(event) => setManualContent(event.target.value)} rows={2} /></label></> : null}
          <Button type="submit" loading={uploading}>{uploadSource === 'file' ? 'Upload file' : uploadSource === 'url' ? 'Import URL' : 'Create document'}</Button>{uploading ? <Button type="button" onClick={() => uploadController.current?.abort()}>Cancel</Button> : null}
        </div>
        {uploadError ? <Status tone="error">{uploadError}</Status> : null}
      </form>
      <div className="wk-documents-layout">
        <aside className="wk-folder-panel"><strong>Folders</strong>{folderState.status === 'loading' ? <Status>Loading folders…</Status> : null}{folderState.status === 'error' ? <Status tone="error">{folderState.message}</Status> : null}<ul className="wk-folder-list">{folders.map((folder) => <li key={folder.path} style={{ paddingLeft: `${folder.depth * 0.8}rem` }}><button type="button" className={folderPath === (folder.path || undefined) ? 'is-active' : ''} onClick={() => setFolderPath(folder.path || undefined)}>{folder.name} <span>{folder.total_count}</span></button></li>)}</ul></aside>
        <section className="wk-document-results">
          <div className="wk-toolbar" role="search"><label>Search <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="File name or keyword" /></label><label>Status <select value={parseStatus} onChange={(event) => setParseStatus(event.target.value)}><option value="">All statuses</option><option value="pending">Pending</option><option value="processing">Processing</option><option value="finalizing">Finalizing</option><option value="completed">Completed</option><option value="failed">Failed</option><option value="deleting">Deleting</option><option value="cancelled">Cancelled</option></select></label><label>Tag <select value={tagId} onChange={(event) => setTagId(event.target.value)}><option value="">All tags</option>{tags.map((tag) => <option key={tag.id} value={tag.id}>{tag.name}</option>)}</select></label></div>
          <div className="wk-list-actions"><span>{selectedOnPage} selected on this page{selected.size > selectedOnPage ? ` · ${selected.size} total` : ''}</span><Button type="button" disabled={!selected.size} onClick={() => void moveSelected()}>Move</Button><Button type="button" disabled={!selected.size} onClick={() => void updateSelectedTags()}>Set tags</Button><Button type="button" disabled={!selected.size} onClick={() => void deleteSelected()}>Delete</Button></div>
          {mutationError ? <Status tone="error">{mutationError}</Status> : null}
          {state.status === 'error' && state.message === 'Loading…' ? <Status>Loading documents…</Status> : null}
          {state.status === 'error' && state.message !== 'Loading…' ? <><Status tone="error">{state.message}</Status><Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Try again</Button></> : null}
          {state.status === 'success' && state.page.items.length === 0 ? <Status>No documents match the current filters.</Status> : null}
          {state.status === 'success' && state.page.items.length > 0 ? <ul className="wk-list wk-document-list">{state.page.items.map((document) => { const status = documentStatus(document); return <li key={document.id}><input type="checkbox" aria-label={`Select ${displayName(document)}`} checked={selected.has(document.id)} onChange={() => toggleSelected(document.id)} /><div className="wk-list-item-copy"><button type="button" className="wk-document-link" onClick={() => onOpenDocument?.(document)}>{displayName(document)}</button><span>{document.folder_path || 'Root'}{document.file_type ? ` · ${document.file_type}` : ''}{document.source ? ` · ${document.source}` : ''}</span></div><Status tone={status.tone}>{status.label}</Status></li>; })}</ul> : null}
          {state.status === 'success' && state.page.total > pageSize ? <nav className="wk-pagination" aria-label="Document pages"><Button type="button" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>Previous</Button><span>Page {page} · {state.page.total} documents</span><Button type="button" disabled={page * pageSize >= state.page.total} onClick={() => setPage((value) => value + 1)}>Next</Button></nav> : null}
        </section>
      </div>
    </Card>
  </main>;
}
