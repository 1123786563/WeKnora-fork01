import { useEffect, useState } from 'react';
import type { KnowledgeChunk, KnowledgeChunkRevision, KnowledgeDocument, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Sheet, Status } from '@weknora/ui';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { buildDocumentPreview, DocumentPreviewContent, isInlinePreviewKind, previewBodyAsBlob, readCurrentPreviewText, type InlinePreviewKind } from './preview.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { buildKnowledgeTimeline, flattenKnowledgeSpans, isKnowledgeProcessingActive, type KnowledgeTimelineNode } from '@weknora/domain/knowledge/processing';
import { startProcessingTimeline, type ProcessingTimelineSubscription } from './processing-timeline.ts';
import type { KnowledgeTimelineStep } from '@weknora/domain/knowledge/processing';
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from '../knowledge/permissions.ts';

interface KnowledgeDocumentDetailPageProps {
  client: WeKnoraClient;
  documentId: string;
  onBack: () => void;
}

type DocumentLoadState =
  | { status: 'loading' }
  | { status: 'success'; document: KnowledgeDocument }
  | { status: 'empty' }
  | { status: 'error'; message: string };

function isDocumentDetail(value: unknown): value is KnowledgeDocument {
  return Boolean(value) && typeof value === 'object' && typeof (value as { id?: unknown }).id === 'string' && Boolean((value as { id?: string }).id?.trim());
}

/** Vue DocContent#getDisplayTitle removes a file extension but keeps manual and URL titles intact. */
export function documentDetailTitle(document: KnowledgeDocument | undefined, fallback: string): string {
  if (!document) return fallback;
  const title = document.file_name || document.title || '';
  if (!title) return fallback;
  if (document.source !== 'file') return title;
  const extensionAt = title.lastIndexOf('.');
  return extensionAt > 0 ? title.slice(0, extensionAt) : title;
}

function documentCanDownload(document: KnowledgeDocument): boolean {
  return document.source === 'file' || document.source === 'manual' || (!document.source && Boolean(document.file_name));
}

const DETAIL_COPY: Record<Locale, { load: string; bytes: string; status: string; source: string; folder: string; type: string; root: string; unavailable: string; downloadOnly: string; loading: string; retry: string; download: string; downloadFailed: string }> = {
  'zh-CN': { load: '文档加载失败', bytes: '文档内容读取失败', status: '状态', source: '来源', folder: '文件夹', type: '类型', root: '根目录', unavailable: '处理完成后才能预览文档。当前状态以服务端为准。', downloadOnly: '此文件类型不支持内嵌预览，请下载原文件。', loading: '正在加载预览…', retry: '重试预览', download: '下载', downloadFailed: '下载失败。' },
  'en-US': { load: 'Unable to load document', bytes: 'Unable to load document bytes', status: 'Status', source: 'Source', folder: 'Folder', type: 'Type', root: 'Root', unavailable: 'Preview is unavailable until processing reaches completed. Current status is authoritative.', downloadOnly: 'Inline preview is unavailable for this file type. Download the original file instead.', loading: 'Loading preview…', retry: 'Retry preview', download: 'Download', downloadFailed: 'Download failed.' },
  'ja-JP': { load: 'ドキュメントを読み込めません', bytes: 'ドキュメント内容を読み込めません', status: '状態', source: 'ソース', folder: 'フォルダー', type: '種類', root: 'ルート', unavailable: '処理が完了するまでプレビューできません。現在の状態はサーバーを正とします。', downloadOnly: 'このファイル形式はインラインプレビューに対応していません。元のファイルをダウンロードしてください。', loading: 'プレビューを読み込み中…', retry: 'プレビューを再試行', download: 'ダウンロード', downloadFailed: 'ダウンロードに失敗しました。' },
  'ko-KR': { load: '문서를 불러오지 못했습니다', bytes: '문서 내용을 불러오지 못했습니다', status: '상태', source: '소스', folder: '폴더', type: '유형', root: '루트', unavailable: '처리가 완료될 때까지 미리보기를 사용할 수 없습니다. 현재 상태는 서버 기준입니다.', downloadOnly: '이 파일 형식은 인라인 미리보기를 지원하지 않습니다. 원본 파일을 다운로드하세요.', loading: '미리보기를 불러오는 중…', retry: '미리보기 다시 시도', download: '다운로드', downloadFailed: '다운로드하지 못했습니다.' },
  'ru-RU': { load: 'Не удалось загрузить документ', bytes: 'Не удалось загрузить содержимое документа', status: 'Статус', source: 'Источник', folder: 'Папка', type: 'Тип', root: 'Корень', unavailable: 'Предпросмотр станет доступен после завершения обработки. Текущий статус определяется сервером.', downloadOnly: 'Для этого типа файла нет встроенного предпросмотра. Скачайте исходный файл.', loading: 'Загрузка предпросмотра…', retry: 'Повторить предпросмотр', download: 'Скачать', downloadFailed: 'Не удалось скачать файл.' },
};

type ContentView = 'preview' | 'merged' | 'chunks';

const CONTENT_TABS: Record<Locale, { preview: string; merged: string; chunks: string }> = {
  'zh-CN': { preview: '预览', merged: '全文', chunks: '查看分块' },
  'en-US': { preview: 'Preview', merged: 'Full Text', chunks: 'View Chunks' },
  'ja-JP': { preview: 'プレビュー', merged: '全文', chunks: 'チャンクを表示' },
  'ko-KR': { preview: '미리보기', merged: '전체 텍스트', chunks: '청크 보기' },
  'ru-RU': { preview: 'Предпросмотр', merged: 'Полный текст', chunks: 'Просмотр фрагментов' },
};

export type MetadataValueType = 'text' | 'number' | 'boolean' | 'null';
export interface MetadataDraftRow { id: number; key: string; value: string; type: MetadataValueType }

let metadataRowSeed = 0;
function metadataRow(key = '', value: unknown = ''): MetadataDraftRow {
  const type: MetadataValueType = value === null ? 'null' : typeof value === 'number' ? 'number' : typeof value === 'boolean' ? 'boolean' : 'text';
  return { id: ++metadataRowSeed, key, value: value === null ? '' : String(value), type };
}

export function metadataRowsFromObject(input: Record<string, unknown> | null | undefined): MetadataDraftRow[] {
  return Object.entries(input || {}).map(([key, value]) => metadataRow(key, value));
}

function metadataValue(row: MetadataDraftRow, key: string): unknown {
  if (row.type === 'null') return null;
  if (row.type === 'boolean') return row.value === 'true';
  if (row.type === 'number') {
    const value = Number(row.value);
    if (!row.value.trim() || !Number.isFinite(value)) throw new Error(`元数据字段 ${key} 必须是有效数字`);
    return value;
  }
  return row.value;
}

export function metadataRowsToObject(rows: MetadataDraftRow[]): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) throw new Error('元数据字段名不能为空');
    if (Object.prototype.hasOwnProperty.call(result, key)) throw new Error(`元数据字段 ${key} 重复`);
    result[key] = metadataValue(row, key);
  }
  return result;
}

export function validateMetadataRows(rows: MetadataDraftRow[]): { ok: true; value: Record<string, unknown> } | { ok: false; message: string } {
  try { return { ok: true, value: metadataRowsToObject(rows) }; }
  catch (error) { return { ok: false, message: error instanceof Error ? error.message : '元数据校验失败' }; }
}

export function knowledgeTraceNodeState(row: KnowledgeTimelineNode): 'pending' | 'running' | 'done' | 'failed' {
  const rawStatus = typeof row.node.status === 'string' ? row.node.status.toLowerCase() : '';
  if (/(fail|error|cancel|abort)/.test(rawStatus)) return 'failed';
  if (/(run|progress|active|start)/.test(rawStatus)) return 'running';
  if (/(complete|done|success|finish|ok)/.test(rawStatus) || Boolean(row.node.end_time)) return 'done';
  return 'pending';
}

export function KnowledgeDocumentDetailPage({ client, documentId, onBack }: KnowledgeDocumentDetailPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const copy = DETAIL_COPY[locale];
  const [state, setState] = useState<DocumentLoadState>({ status: 'loading' });
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [canMutateDocument, setCanMutateDocument] = useState(false);
  const [timelineSteps, setTimelineSteps] = useState<KnowledgeTimelineStep[]>([]);
  const [traceOpen, setTraceOpen] = useState(false);
  const [traceState, setTraceState] = useState<{ status: 'idle' | 'loading' | 'success' | 'error'; steps: KnowledgeTimelineStep[]; nodes: KnowledgeTimelineNode[]; parseStatus?: string; lastError?: { error_code?: string; error_message?: string } | null; message?: string }>({ status: 'idle', steps: [], nodes: [] });
  const [traceRefresh, setTraceRefresh] = useState(0);
  const [traceAction, setTraceAction] = useState<'idle' | 'loading' | 'error'>('idle');
  const [expandedTraceNodes, setExpandedTraceNodes] = useState<Set<string>>(new Set());
  const [selectedTraceNode, setSelectedTraceNode] = useState<KnowledgeTimelineNode | null>(null);
  const [contentView, setContentView] = useState<ContentView>('merged');
  useEffect(() => {
    let active = true;
    setState({ status: 'loading' });
    void client.knowledgeBases.documents.get(documentId).then((document) => {
      if (!active) return;
      setState(isDocumentDetail(document) ? { status: 'success', document } : { status: 'empty' });
    }).catch((error: unknown) => { if (active) setState({ status: 'error', message: error instanceof Error ? error.message : copy.load }); });
    return () => { active = false; };
  }, [client, documentId, loadAttempt]);

  useEffect(() => {
    if (state.status !== 'success') return;
    const model = buildDocumentPreview(state.document, client.knowledgeBases.documents.previewPath(state.document.id));
    setContentView(model.ready && isInlinePreviewKind(model.kind) ? 'preview' : 'merged');
  }, [client, state.status === 'success' ? state.document.id : undefined]);

  // Vue receives KB-level permissions from KnowledgeBase.vue. The route only
  // has a document id, so resolve the same read-only gate from the loaded
  // document's KB before exposing download or mutation affordances.
  useEffect(() => {
    if (state.status !== 'success' || !state.document.knowledge_base_id) {
      setCanMutateDocument(false);
      return;
    }
    let active = true;
    setCanMutateDocument(false);
    void Promise.all([
      client.knowledgeBases.settings.get(state.document.knowledge_base_id),
      client.auth.me().catch(() => null),
    ]).then(([kb, me]) => {
      if (active) setCanMutateDocument(computeKBPermissions(kb as KBSurfaceKB, me as KBSurfaceMe | null).canContribute);
    }).catch(() => {
      if (active) setCanMutateDocument(false);
    });
    return () => { active = false; };
  }, [client, state]);

  // Must-fix #2: processing timeline — poll the spans endpoint every 2s while
  // the document is pending/processing/finalizing; quiesce stop at terminal.
  const parseStatus = state.status === 'success' ? state.document.parse_status : undefined;
  useEffect(() => {
    if (!isKnowledgeProcessingActive(parseStatus)) {
      setTimelineSteps([]);
      return;
    }
    let subscription: ProcessingTimelineSubscription | undefined;
    void client.knowledgeBases.documents.get(documentId).then((document) => {
      if (!isKnowledgeProcessingActive(document.parse_status)) return;
      subscription = startProcessingTimeline({
        documentId,
        getSpans: (id) => client.knowledgeBases.documents.spans(id),
        onUpdate: (steps) => setTimelineSteps(steps),
      });
    }).catch(() => {});
    return () => subscription?.stop();
  }, [client, documentId, parseStatus]);

  useEffect(() => {
    if (!traceOpen || state.status !== 'success') return;
    let active = true;
    let polling: number | undefined;
    let inFlight = false;
    const load = async () => {
      if (!active || inFlight) return;
      inFlight = true;
      try {
        const spans = await client.knowledgeBases.documents.spans(documentId);
        if (!active) return;
        const parseStatus = typeof spans.parse_status === 'string' ? spans.parse_status : state.document.parse_status;
        const nodes = flattenKnowledgeSpans(spans.trace);
        setTraceState({ status: 'success', steps: buildKnowledgeTimeline(spans), nodes, parseStatus, lastError: spans.last_error });
        setExpandedTraceNodes((current) => current.size > 0 ? current : new Set(nodes.map((row) => row.key)));
        if (!isKnowledgeProcessingActive(parseStatus) && polling !== undefined) {
          window.clearInterval(polling);
          polling = undefined;
        }
      } catch (error: unknown) {
        if (active) setTraceState({ status: 'error', steps: [], nodes: [], message: error instanceof Error ? error.message : copy.load });
      } finally {
        inFlight = false;
      }
    };
    setTraceState({ status: 'loading', steps: [], nodes: [] });
    void load().then(() => {
      if (active && isKnowledgeProcessingActive(state.document.parse_status)) polling = window.setInterval(() => void load(), 2000);
    });
    return () => { active = false; if (polling !== undefined) window.clearInterval(polling); };
  }, [client, documentId, traceOpen, traceRefresh]);

  async function runTraceAction(action: 'reparse' | 'cancel') {
    if (state.status !== 'success' || !canMutateDocument || traceAction === 'loading') return;
    setTraceAction('loading');
    try {
      if (action === 'reparse') await client.knowledgeBases.documents.reparse(documentId);
      else await client.knowledgeBases.documents.cancelParse(documentId);
      const document = await client.knowledgeBases.documents.get(documentId);
      setState({ status: 'success', document });
      setTraceRefresh((value) => value + 1);
      setTraceAction('idle');
    } catch (error: unknown) {
      setTraceAction('error');
      setTraceState((current) => ({ ...current, status: 'error', message: error instanceof Error ? error.message : copy.load }));
    }
  }

  const detailTitle = state.status === 'success' ? documentDetailTitle(state.document, t('common.typeDocument')) : t('common.typeDocument');
  return <Sheet open title={detailTitle} onClose={onBack} closeLabel={t('common.close')} resizeLabel="Resize drawer" side="right" width="654px" resizable minWidth={480} maxWidth={1600} storageKey="weknora-doc-drawer-width" className="wk-document-detail-drawer">
  <main className="wk-page wk-document-detail-page max-w-[820px]! box-border px-[1.25rem] py-6"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div className="min-w-0"><div className="document-title-row flex min-h-8 items-center"><h2 className="document-breadcrumb m-0 flex min-w-0 items-center gap-2 text-[20px] font-semibold leading-8"><button type="button" className="breadcrumb-link inline-flex shrink-0 cursor-pointer items-center gap-1 rounded-[6px] border-none bg-transparent px-2 py-1 -mx-2 -my-1 text-[14px] font-normal leading-6 text-[var(--wk-muted,#66758b)] [font:inherit] hover:bg-[var(--wk-surface,#fff)] hover:text-[var(--wk-brand,#00a870)]" onClick={onBack}>← {t('knowledgeBase.detail.back')}</button><span className="breadcrumb-separator text-[14px] font-normal text-[var(--wk-muted,#98a2b8)]" aria-hidden="true">›</span><span className="breadcrumb-current min-w-0 truncate">{detailTitle}</span></h2></div></div><div className="flex shrink-0 items-center gap-2">{state.status === 'success' ? <Button type="button" onClick={() => setTraceOpen(true)}>{t('knowledgeBase.timeline.title')}</Button> : null}</div></header>
    <section className="wk-document-detail-surface" aria-live="polite" aria-busy={state.status === 'loading'}>
    {state.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
    {state.status === 'empty' ? <Status>{t('common.empty')}</Status> : null}
    {state.status === 'error' ? <div className="flex flex-wrap items-center gap-3"><Status tone="error">{state.message}</Status><Button type="button" variant="text" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>{t('common.retry')}</Button></div> : null}
    {state.status === 'success' && timelineSteps.length > 0 ? <Card><section aria-label={t('knowledgeBase.timeline.title')} className="wk-processing-timeline"><strong>{t('knowledgeBase.timeline.title')}</strong><ol>{timelineSteps.map((step) => <li key={step.stage} data-state={step.state}>{t('knowledgeBase.timeline.stage.' + step.stage)} — {t('knowledgeBase.timeline.' + step.state)}</li>)}</ol></section></Card> : null}
  {state.status === 'success' ? <><div className="mb-4 flex flex-wrap gap-2" role="tablist" aria-label={t('knowledgeBase.documentContent')}>
    {(() => { const tabs = CONTENT_TABS[locale]; const model = buildDocumentPreview(state.document, client.knowledgeBases.documents.previewPath(documentId)); return <>
      {model.ready && isInlinePreviewKind(model.kind) ? <Button type="button" role="tab" aria-selected={contentView === 'preview'} onClick={() => setContentView('preview')}>{tabs.preview}</Button> : null}
      <Button type="button" role="tab" aria-selected={contentView === 'merged'} onClick={() => setContentView('merged')}>{tabs.merged}</Button>
      <Button type="button" role="tab" aria-selected={contentView === 'chunks'} onClick={() => setContentView('chunks')}>{tabs.chunks}</Button>
    </>; })()}
  </div><DocumentDetail client={client} document={state.document} canEdit={canMutateDocument} canDownload={canMutateDocument && documentCanDownload(state.document)} previewPath={client.knowledgeBases.documents.previewPath(documentId)} downloadPath={client.knowledgeBases.documents.downloadPath(documentId)} showPreview={contentView === 'preview'} /><DocumentChunks client={client} document={state.document} canEdit={canMutateDocument} view={contentView} /></> : null}
    </section>
  {traceOpen ? <Sheet open title={t('knowledgeBase.timeline.title')} onClose={() => setTraceOpen(false)} side="right" width="820px" resizable minWidth={560} maxWidth={1400} storageKey="weknora-trace-drawer-width" className="min-w-0 border-l border-line-soft">
    <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === 'loading'}>
      {traceState.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
      {traceState.status === 'error' ? <Status tone="error">{traceState.message}</Status> : null}
      {traceState.status === 'success' ? <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" loading={traceAction === 'loading'} onClick={() => setTraceRefresh((value) => value + 1)}>{t('common.refresh')}</Button>
          {canMutateDocument && traceState.parseStatus === 'failed' ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('reparse')}>{t('knowledgeBase.rebuildDocument')}</Button> : null}
          {canMutateDocument && isKnowledgeProcessingActive(traceState.parseStatus) ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('cancel')}>{t('knowledgeBase.documents.cancelParse')}</Button> : null}
        </div>
        {traceState.parseStatus === 'failed' ? <Status tone="error">{traceState.lastError?.error_message || t('knowledgeBase.timeline.failed')}</Status> : null}
        <ol className="m-0 flex list-none flex-col gap-2 p-0" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.steps.map((step) => <li key={step.stage} data-state={step.state} className="flex items-center justify-between rounded-[6px] border border-line-soft px-3 py-2 text-[13px]"><span>{t(`knowledgeBase.timeline.stage.${step.stage}`)}</span><span>{t(`knowledgeBase.timeline.${step.state}`)}</span></li>)}
        </ol>
        {traceState.nodes.length > 0 ? <div className="overflow-x-auto rounded-[8px] border border-line-soft"><ol className="m-0 list-none divide-y divide-line-soft p-0" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.nodes.filter((row) => row.depth === 0 || expandedTraceNodes.has(row.key.slice(0, row.key.lastIndexOf('.')))).map((row) => { const nodeState = knowledgeTraceNodeState(row); return <li key={row.key} data-state={nodeState} className="flex min-w-[480px] items-center gap-2 px-3 py-2 text-[13px] hover:bg-surface-wash" style={{ paddingLeft: `${12 + row.depth * 16}px` }}>
            {row.hasChildren ? <button type="button" className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-control border-0 bg-transparent text-muted hover:bg-hover-wash" aria-expanded={expandedTraceNodes.has(row.key)} aria-label={t('knowledgeBase.timeline.title')} onClick={() => setExpandedTraceNodes((current) => { const next = new Set(current); if (next.has(row.key)) next.delete(row.key); else next.add(row.key); return next; })}>{expandedTraceNodes.has(row.key) ? '⌄' : '›'}</button> : <span className="inline-block h-6 w-6 shrink-0" aria-hidden="true" />}
            <button type="button" className="min-w-0 flex-1 truncate border-0 bg-transparent p-0 text-left font-mono text-ink hover:underline" onClick={() => setSelectedTraceNode(row)}>{row.node.name || row.node.stage || row.key}</button><span className={nodeState === 'failed' ? 'text-danger' : nodeState === 'done' ? 'text-success' : nodeState === 'running' ? 'text-primary' : 'text-muted'}>{t(`knowledgeBase.timeline.${nodeState}`)}</span><span className="w-20 shrink-0 text-right font-mono text-[11px] text-muted">{typeof row.node.duration_ms === 'number' ? `${row.node.duration_ms}ms` : '—'}</span>
          </li>; })}
        </ol></div> : null}
        {selectedTraceNode ? <section className="rounded-[8px] border border-line-soft bg-surface-wash p-3"><div className="mb-2 flex items-center justify-between gap-2"><strong className="truncate text-[13px]">{selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key}</strong><Button type="button" onClick={() => setSelectedTraceNode(null)}>{t('knowledgeBase.documents.cancel')}</Button></div><pre className="m-0 max-h-[240px] overflow-auto whitespace-pre-wrap break-words text-[11px] leading-[1.5] text-muted">{JSON.stringify(selectedTraceNode.node, null, 2)}</pre></section> : null}
      </div> : null}
    </section>
  </Sheet> : null}
  </main>
  </Sheet>;
}

function DocumentChunks({ client, document, canEdit, view }: { client: WeKnoraClient; document: KnowledgeDocument; canEdit: boolean; view: ContentView }) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [state, setState] = useState<{ status: 'loading' | 'success' | 'error'; chunks: KnowledgeChunk[]; total: number; page: number; message?: string }>({ status: 'loading', chunks: [], total: 0, page: 1 });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [history, setHistory] = useState<{ id: string; rows: KnowledgeChunkRevision[] } | null>(null);
  const [historyLoading, setHistoryLoading] = useState<string | null>(null);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);

  const load = (page = 1) => {
    setState((current) => ({ ...current, status: 'loading', message: undefined }));
    void client.knowledgeBases.documents.chunks(document.id, page).then((result) => {
      setState({ status: 'success', chunks: result.data, total: result.total, page: result.page });
    }).catch((error: unknown) => {
      setState((current) => ({ ...current, status: 'error', message: error instanceof Error ? error.message : t('common.error') }));
    });
  };
  useEffect(load, [client, document.id]);

  const save = async (chunk: KnowledgeChunk) => {
    if (!draft.trim()) return;
    setSavingId(chunk.id);
    try {
      setMutationError(null);
      const updated = await client.knowledgeBases.documents.updateChunk(document.id, chunk.id, { content: draft, expected_revision: chunk.content_revision ?? 0 });
      setState((current) => ({ ...current, status: 'success', chunks: current.chunks.map((row) => row.id === chunk.id ? updated : row) }));
      setEditingId(null);
      setDraft('');
    } catch (error: unknown) {
      setMutationError(error instanceof Error ? error.message : t('common.error'));
    } finally { setSavingId(null); }
  };

  const toggleEnabled = async (chunk: KnowledgeChunk) => {
    setMutationError(null);
    setSavingId(chunk.id);
    try {
      const updated = await client.knowledgeBases.documents.updateChunk(document.id, chunk.id, { is_enabled: !chunk.is_enabled, expected_revision: chunk.content_revision ?? 0 });
      setState((current) => ({ ...current, chunks: current.chunks.map((row) => row.id === chunk.id ? updated : row) }));
    } catch (error: unknown) {
      setMutationError(error instanceof Error ? error.message : t('common.error'));
    } finally { setSavingId(null); }
  };

  const retryIndex = async (chunk: KnowledgeChunk) => {
    setMutationError(null);
    setSavingId(chunk.id);
    try {
      const updated = await client.knowledgeBases.documents.updateChunk(document.id, chunk.id, { expected_revision: chunk.content_revision ?? 0 });
      setState((current) => ({ ...current, chunks: current.chunks.map((row) => row.id === chunk.id ? updated : row) }));
    } catch (error: unknown) {
      setMutationError(error instanceof Error ? error.message : t('common.error'));
    } finally { setSavingId(null); }
  };

  const showHistory = (chunk: KnowledgeChunk) => {
    setHistoryLoading(chunk.id);
    void client.knowledgeBases.documents.chunkRevisions(document.id, chunk.id).then((rows) => setHistory({ id: chunk.id, rows })).catch((error: unknown) => setState((current) => ({ ...current, message: error instanceof Error ? error.message : t('common.error') }))).finally(() => setHistoryLoading(null));
  };

  const mergedContent = state.chunks.slice().sort((left, right) => Number(left.chunk_index ?? 0) - Number(right.chunk_index ?? 0)).map((chunk) => chunk.content || '').filter(Boolean).join('\n\n');
  return <section className="wk-document-chunks mt-4" aria-label={t('knowledgeBase.viewChunks')} hidden={view === 'preview'}>
    <div className="mb-3 flex items-center justify-between gap-3"><h3 className="m-0 text-[13px] font-semibold">{t('knowledgeBase.viewChunks')} {state.total ? `(${state.total})` : ''}</h3></div>
    {mutationError ? <Status tone="error">{mutationError}</Status> : null}
    {state.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
    {state.status === 'error' ? <><Status tone="error">{state.message}</Status><Button type="button" onClick={() => load(state.page)}>{t('common.retry')}</Button></> : null}
    {state.status === 'success' && state.chunks.length === 0 ? <Status>{t('common.empty')}</Status> : null}
    {state.status === 'success' && view === 'merged'
      ? mergedContent
        ? <div className="wk-document-merged markdown-content min-w-0 text-[13px] leading-[1.65] text-ink [overflow-wrap:anywhere] [&_p]:my-[0.4em] [&_p:first-child]:mt-0 [&_p:last-child]:mb-0 [&_h1]:mb-[0.4em] [&_h1]:mt-[0.8em] [&_h1]:text-lg [&_h1]:leading-[1.3] [&_h2]:mb-[0.4em] [&_h2]:mt-[0.8em] [&_h2]:text-base [&_h2]:leading-[1.3] [&_h3]:mb-[0.4em] [&_h3]:mt-[0.8em] [&_h3]:text-sm [&_h3]:leading-[1.3] [&_ul]:my-[0.5em] [&_ul]:pl-5 [&_ol]:my-[0.5em] [&_ol]:pl-5 [&_li]:my-[0.15em] [&_pre]:my-[0.6em] [&_pre]:overflow-x-auto [&_pre]:rounded-card [&_pre]:bg-surface-muted [&_pre]:px-3 [&_pre]:py-2.5 [&_pre]:text-xs [&_code]:font-mono [&_code]:text-xs [&_blockquote]:my-[0.6em] [&_blockquote]:border-l-2 [&_blockquote]:border-line-soft [&_blockquote]:pl-3" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(mergedContent) }} />
        : <div className="wk-document-merged text-[13px] text-muted">—</div>
      : null}
    {state.status === 'success' && view !== 'merged' ? <><div className="flex flex-col gap-3">{state.chunks.map((chunk, index) => <article key={chunk.id} className="rounded-[8px] border border-line-soft bg-surface p-3" data-chunk-id={chunk.id}>
      <div className="mb-2 flex items-center justify-between gap-2"><strong className="text-[12px]">{t('knowledgeBase.segment')} {index + 1}</strong>{canEdit ? <span className="flex flex-wrap gap-1"><Button type="button" onClick={() => { setEditingId(chunk.id); setDraft(chunk.content || ''); }}>{t('common.edit')}</Button><Button type="button" loading={historyLoading === chunk.id} onClick={() => showHistory(chunk)}>{t('knowledgeBase.chunkHistory')}</Button><Button type="button" loading={savingId === chunk.id} onClick={() => void toggleEnabled(chunk)}>{chunk.is_enabled ? t('knowledgeBase.disableChunk') : t('knowledgeBase.enableChunk')}</Button>{chunk.index_status === 'failed' ? <Button type="button" loading={savingId === chunk.id} onClick={() => void retryIndex(chunk)}>{t('common.retry')}</Button> : null}</span> : null}</div>
      {editingId === chunk.id ? <><textarea aria-label={t('knowledgeBase.segment')} value={draft} onChange={(event) => setDraft(event.target.value)} className="min-h-[120px] w-full rounded-control border border-line-soft p-2" /><div className="mt-2 flex gap-2"><Button type="button" loading={savingId === chunk.id} onClick={() => void save(chunk)}>{t('common.save')}</Button><Button type="button" onClick={() => { setEditingId(null); setDraft(''); }}>{t('common.cancel')}</Button></div></> : <p className="m-0 whitespace-pre-wrap text-[13px] text-ink">{chunk.content || '—'}</p>}
      {history?.id === chunk.id ? <div className="mt-3 border-t border-line-soft pt-3"><strong className="text-[12px]">{t('knowledgeBase.chunkHistory')}</strong>{history?.rows.length === 0 ? <Status>{t('common.noData')}</Status> : <ol className="m-0 mt-2 list-decimal pl-5 text-[12px]">{history?.rows.map((row) => <li key={row.revision} className="mb-2"><span>Revision {row.revision}: {row.content || '—'}</span><Button type="button" className="ml-2" onClick={() => void (async () => { const updated = await client.knowledgeBases.documents.revertChunk(document.id, chunk.id, row.revision, chunk.content_revision ?? 0); setState((current) => ({ ...current, chunks: current.chunks.map((item) => item.id === chunk.id ? updated : item) })); showHistory(updated); })()}>{t('knowledgeBase.chunkReverted')}</Button></li>)}</ol>}</div> : null}
    </article>)}</div>{state.total > 25 ? <nav className="mt-3 flex items-center justify-between" aria-label={t('knowledgeBase.viewChunks')}><Button type="button" disabled={state.page <= 1} onClick={() => load(state.page - 1)}>{t('common.back')}</Button><span>{state.page}</span><Button type="button" disabled={state.page * 25 >= state.total} onClick={() => load(state.page + 1)}>{t('common.next')}</Button></nav> : null}</> : null}
  </section>;
}

type PreviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'text'; text: string }
  | { status: 'blob'; url: string }
  | { status: 'error'; message: string };

function DocumentDetail({ document, client, canEdit, canDownload, previewPath, downloadPath, showPreview }: { document: KnowledgeDocument; client: WeKnoraClient; canEdit: boolean; canDownload: boolean; previewPath: string; downloadPath: string; showPreview: boolean }) {
  const model = buildDocumentPreview(document, previewPath);
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const copy = DETAIL_COPY[locale];
  const [previewState, setPreviewState] = useState<PreviewState>({ status: 'idle' });
  const [downloadState, setDownloadState] = useState<'idle' | 'loading' | 'error'>('idle');
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [summaryEditing, setSummaryEditing] = useState(false);
  const [summaryDraft, setSummaryDraft] = useState(String(document.description || ''));
  const [metadataEditing, setMetadataEditing] = useState(false);
  const [metadataDraft, setMetadataDraft] = useState<MetadataDraftRow[]>(() => metadataRowsFromObject(document.custom_metadata as Record<string, unknown> | undefined));
  const [detailsSaving, setDetailsSaving] = useState(false);
  const [detailsError, setDetailsError] = useState<string | null>(null);

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
        const text = await readCurrentPreviewText(response.body, () => active);
        if (text !== undefined) setPreviewState({ status: 'text', text });
        return;
      }
      objectUrl = URL.createObjectURL(previewBodyAsBlob(response.body, contentType));
      setPreviewState({ status: 'blob', url: objectUrl });
    }).catch((error: unknown) => {
      if (active && !(error instanceof Error && error.name === 'AbortError')) setPreviewState({ status: 'error', message: error instanceof Error ? error.message : copy.bytes });
    });
    return () => {
      active = false;
      controller.abort();
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [client, document.id, model.kind, model.ready, previewAttempt]);

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
    } catch {
      setDownloadState('error');
    }
  }

  async function saveDetails(input: { description?: string; custom_metadata?: Record<string, unknown> }) {
    setDetailsSaving(true);
    setDetailsError(null);
    try {
      const updated = await client.knowledgeBases.documents.updateDetails(document.id, input);
      Object.assign(document, updated);
      if (input.description !== undefined) setSummaryDraft(String(updated.description ?? input.description));
      if (input.custom_metadata !== undefined) setMetadataDraft(metadataRowsFromObject((updated.custom_metadata as Record<string, unknown> | undefined) ?? input.custom_metadata));
      setSummaryEditing(false);
      setMetadataEditing(false);
    } catch (error: unknown) {
      setDetailsError(error instanceof Error ? error.message : copy.load);
    } finally { setDetailsSaving(false); }
  }

  const inlineKind: InlinePreviewKind | undefined = isInlinePreviewKind(model.kind) ? model.kind : undefined;
  const rawTags = Array.isArray((document as KnowledgeDocument & { tags?: unknown }).tags) ? (document as KnowledgeDocument & { tags: Array<{ id?: string | number; name?: string }> }).tags : [];
  const documentTime = (document as KnowledgeDocument & { time?: unknown }).time;
  return <Card className="wk-document-detail-card">
    <div className="mb-3 flex items-center justify-end gap-2 border-b border-line-soft pb-3">
      {canDownload ? <Button type="button" aria-label={copy.download} loading={downloadState === 'loading'} onClick={() => { setDownloadState('idle'); void download(); }}>{copy.download}</Button> : null}
    </div>
    {detailsError ? <Status tone="error">{detailsError}</Status> : null}<section className="wk-document-metadata-section border-b border-line-soft pb-4"><h3 className="m-0 mb-3 flex items-center gap-2 text-[13px] font-semibold text-ink before:h-[14px] before:w-[3px] before:rounded-[2px] before:bg-primary before:content-['']">{createTranslator(useAppLocale())('knowledgeBase.detailSectionMeta')}</h3><dl className="wk-document-metadata m-0 flex flex-col gap-2.5 [&_dd]:m-0 [&_dd]:min-w-0 [&_dd]:break-words [&_dt]:w-[72px] [&_dt]:shrink-0 [&_dt]:text-[13px] [&_dt]:text-muted"><div className="flex items-start gap-3"><dt>{copy.status}</dt><dd>{String(document.parse_status || 'unknown')}</dd></div><div className="flex items-start gap-3"><dt>{copy.source}</dt><dd>{String(document.source || 'file')}</dd></div>{documentTime ? <div className="flex items-start gap-3"><dt>{copy.status}</dt><dd>{formatDetailTime(documentTime)}</dd></div> : null}{document.channel && document.channel !== 'web' ? <div className="flex items-start gap-3"><dt>{copy.source}</dt><dd>{String(document.channel)}</dd></div> : null}<div className="flex items-start gap-3"><dt>{copy.folder}</dt><dd>{String(document.folder_path || copy.root)}</dd></div><div className="flex items-start gap-3"><dt>{copy.type}</dt><dd>{String(document.file_type || model.kind).toUpperCase()}</dd></div>{rawTags.length > 0 ? <div className="flex items-start gap-3"><dt>{t('knowledgeBase.tagLabel')}</dt><dd className="flex flex-wrap gap-1">{rawTags.map((tag) => <span key={String(tag.id ?? tag.name)} className="rounded-full border border-line-soft px-2 py-0.5 text-[11px] text-muted">{tag.name}</span>)}</dd></div> : null}</dl></section>
    <section className="border-b border-line-soft py-4" aria-label={t('knowledgeBase.documentSummary')}><div className="mb-2 flex items-center justify-between gap-2"><h3 className="m-0 text-[13px] font-semibold">{t('knowledgeBase.documentSummary')}</h3>{canEdit && !summaryEditing ? <Button type="button" onClick={() => setSummaryEditing(true)}>{t('common.edit')}</Button> : null}</div>{summaryEditing ? <><textarea value={summaryDraft} onChange={(event) => setSummaryDraft(event.target.value)} className="min-h-[100px] w-full rounded-control border border-line-soft p-2" /><div className="mt-2 flex gap-2"><Button type="button" loading={detailsSaving} onClick={() => void saveDetails({ description: summaryDraft })}>{t('common.save')}</Button><Button type="button" onClick={() => setSummaryEditing(false)}>{t('common.cancel')}</Button></div></> : <p className="m-0 whitespace-pre-wrap text-[13px] text-muted">{summaryDraft || '—'}</p>}</section>
    <MetadataEditor editing={metadataEditing} rows={metadataDraft} saving={detailsSaving} canEdit={canEdit} onStart={() => { setDetailsError(null); const rows = metadataRowsFromObject(document.custom_metadata as Record<string, unknown> | undefined); setMetadataDraft(rows.length ? rows : [metadataRow()]); setMetadataEditing(true); }} onChange={setMetadataDraft} onCancel={() => { setMetadataEditing(false); setDetailsError(null); }} onSave={(value) => void saveDetails({ custom_metadata: value })} />
    {showPreview ? (!model.ready ? <Status tone="warning">{copy.unavailable}</Status> : model.downloadOnly ? <Status>{copy.downloadOnly}</Status> : previewState.status === 'loading' ? <Status>{copy.loading}</Status> : previewState.status === 'error' ? <><Status tone="error">{previewState.message}</Status><Button type="button" onClick={() => setPreviewAttempt((attempt) => attempt + 1)}>{copy.retry}</Button></> : previewState.status === 'text' && inlineKind ? <DocumentPreviewContent kind={inlineKind} text={previewState.text} fileName={model.fileName} /> : previewState.status === 'blob' && inlineKind ? <DocumentPreviewContent kind={inlineKind} url={previewState.url} fileName={model.fileName} /> : null) : null}
    {canDownload && downloadState === 'error' ? <Status tone="error">{copy.downloadFailed}</Status> : null}
  </Card>;
}

function formatDetailTime(value: unknown): string {
  if (typeof value !== 'string' || !value) return String(value ?? '');
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function MetadataEditor({ editing, rows, saving, canEdit, onStart, onChange, onCancel, onSave }: {
  editing: boolean; rows: MetadataDraftRow[]; saving: boolean; canEdit: boolean;
  onStart: () => void; onChange: (rows: MetadataDraftRow[]) => void; onCancel: () => void; onSave: (value: Record<string, unknown>) => void;
}) {
  const t = createTranslator(useAppLocale());
  const [validationError, setValidationError] = useState<string | null>(null);
  const updateRow = (id: number, update: Partial<MetadataDraftRow>) => onChange(rows.map((row) => row.id === id ? { ...row, ...update } : row));
  const start = () => onStart();
  const save = () => {
    const validation = validateMetadataRows(rows);
    if (!validation.ok) { setValidationError(validation.message); return; }
    setValidationError(null);
    onSave(validation.value);
  };
  return <section className="border-b border-line-soft py-4" aria-label={t('knowledgeBase.customMetadata')}>
    <div className="mb-2 flex items-center justify-between gap-2"><h3 className="m-0 text-[13px] font-semibold">{t('knowledgeBase.customMetadata')}</h3>{canEdit && !editing ? <Button type="button" onClick={start}>{t('common.edit')}</Button> : null}</div>
    {editing ? <>
      {validationError ? <Status tone="error">{validationError}</Status> : null}
      <div className="flex flex-col gap-2">{rows.map((row) => <div key={row.id} className="flex items-start gap-2">
        <input aria-label={t('knowledgeBase.metadataKeyPlaceholder')} placeholder={t('knowledgeBase.metadataKeyPlaceholder')} value={row.key} onChange={(event) => updateRow(row.id, { key: event.target.value })} className="min-w-0 flex-1 rounded-control border border-line-soft px-2 py-1 text-[12px]" />
        <select aria-label={t('knowledgeBase.metadataTypeText')} value={row.type} onChange={(event) => { const type = event.target.value as MetadataValueType; updateRow(row.id, { type, value: type === 'null' ? '' : type === 'boolean' ? 'false' : row.value }); }} className="rounded-control border border-line-soft px-2 py-1 text-[12px]">
          <option value="text">{t('knowledgeBase.metadataTypeText')}</option><option value="number">{t('knowledgeBase.metadataTypeNumber')}</option><option value="boolean">{t('knowledgeBase.metadataTypeBoolean')}</option><option value="null">{t('knowledgeBase.metadataTypeNull')}</option>
        </select>
        <input aria-label={t('knowledgeBase.metadataValuePlaceholder')} placeholder={t('knowledgeBase.metadataValuePlaceholder')} value={row.value} disabled={row.type === 'null'} onChange={(event) => updateRow(row.id, { value: event.target.value })} className="min-w-0 flex-1 rounded-control border border-line-soft px-2 py-1 text-[12px]" />
        <Button type="button" variant="text" onClick={() => onChange(rows.filter((item) => item.id !== row.id))}>{t('common.delete')}</Button>
      </div>)}</div>
      <div className="mt-2 flex flex-wrap gap-2"><Button type="button" disabled={rows.length >= 20} onClick={() => onChange([...rows, metadataRow()])}>{t('knowledgeBase.addMetadataField')}</Button><Button type="button" loading={saving} onClick={save}>{t('common.save')}</Button><Button type="button" onClick={onCancel}>{t('common.cancel')}</Button></div>
    </> : <div className="flex flex-col gap-1 text-[12px] text-muted">{rows.length ? rows.map((row) => <div key={row.id} className="flex gap-3"><span className="font-medium text-ink">{row.key}</span><span>{row.type === 'null' ? 'null' : row.value}</span></div>) : <span>{t('knowledgeBase.noCustomMetadata')}</span>}</div>}
  </section>;
}
