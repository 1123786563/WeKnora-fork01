import { useEffect, useRef, useState } from 'react';
import type { KnowledgeChunk, KnowledgeChunkRevision, KnowledgeDocument, KnowledgeGeneratedQuestion, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Sheet, Status } from '@weknora/ui';
import { buildDocumentPreview, canPreviewDocument, DocumentMarkdownBody, DocumentPreviewContent, isInlinePreviewKind, previewBodyAsBlob, readCurrentPreviewText, readSpreadsheetPreview, type DocumentMermaidLabels, type InlinePreviewKind, type SpreadsheetPreviewModel } from './preview.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { buildKnowledgeTimeline, flattenKnowledgeSpans, isKnowledgeProcessingActive, type KnowledgeTimelineNode } from '@weknora/domain/knowledge/processing';
import { knowledgeSpansLastError, resolveKnowledgeSpansView, startProcessingTimeline, type ProcessingTimelineSubscription } from './processing-timeline.ts';
import { mergeChunkContents } from './model.ts';
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
  if (document.type !== 'file') return title;
  const extensionAt = title.lastIndexOf('.');
  return extensionAt > 0 ? title.slice(0, extensionAt) : title;
}

function documentCanDownload(document: KnowledgeDocument): boolean {
  return document.source === 'file' || document.source === 'manual' || (!document.source && Boolean(document.file_name));
}

const DETAIL_COPY: Record<Locale, { load: string; bytes: string; status: string; source: string; folder: string; type: string; root: string; loading: string; retry: string; download: string; downloadFailed: string }> = {
  'zh-CN': { load: '文档加载失败', bytes: '文档内容读取失败', status: '状态', source: '来源', folder: '文件夹', type: '类型', root: '根目录', loading: '正在加载预览…', retry: '重试预览', download: '下载', downloadFailed: '下载失败。' },
  'en-US': { load: 'Unable to load document', bytes: 'Unable to load document bytes', status: 'Status', source: 'Source', folder: 'Folder', type: 'Type', root: 'Root', loading: 'Loading preview…', retry: 'Retry preview', download: 'Download', downloadFailed: 'Download failed.' },
  'ja-JP': { load: 'ドキュメントを読み込めません', bytes: 'ドキュメント内容を読み込めません', status: '状態', source: 'ソース', folder: 'フォルダー', type: '種類', root: 'ルート', loading: 'プレビューを読み込み中…', retry: 'プレビューを再試行', download: 'ダウンロード', downloadFailed: 'ダウンロードに失敗しました。' },
  'ko-KR': { load: '문서를 불러오지 못했습니다', bytes: '문서 내용을 불러오지 못했습니다', status: '상태', source: '소스', folder: '폴더', type: '유형', root: '루트', loading: '미리보기를 불러오는 중…', retry: '미리보기 다시 시도', download: '다운로드', downloadFailed: '다운로드하지 못했습니다.' },
  'ru-RU': { load: 'Не удалось загрузить документ', bytes: 'Не удалось загрузить содержимое документа', status: 'Статус', source: 'Источник', folder: 'Папка', type: 'Тип', root: 'Корень', loading: 'Загрузка предпросмотра…', retry: 'Повторить предпросмотр', download: 'Скачать', downloadFailed: 'Не удалось скачать файл.' },
};

type ContentView = 'preview' | 'merged' | 'chunks';

// R474/A3 — trace drawer head copy, byte-exact from the Vue
// knowledgeStages.head.stagesProgress string
// (frontend/src/i18n/locales/*.ts). The LIVE badge text itself is the
// brand-style 'LIVE' in every locale, like Vue knowledgeStages.live.
const TRACE_HEAD_COPY: Record<Locale, { stagesProgress: string; liveTooltip: string }> = {
  'zh-CN': { stagesProgress: '当前阶段', liveTooltip: '解析进行中，每 2 秒自动刷新一次' },
  'en-US': { stagesProgress: 'Current stage', liveTooltip: 'Parsing in progress — auto-refreshes every 2s' },
  'ja-JP': { stagesProgress: '現在のステージ', liveTooltip: '解析中です。2秒ごとに自動更新されます' },
  'ko-KR': { stagesProgress: '현재 단계', liveTooltip: '파싱 진행 중 — 2초마다 자동 새로고침' },
  'ru-RU': { stagesProgress: 'Current stage', liveTooltip: 'Parsing in progress — auto-refreshes every 2s' },
};

const CONTENT_TABS: Record<Locale, { preview: string; merged: string; chunks: string }> = {
  'zh-CN': { preview: '预览', merged: '全文', chunks: '查看分块' },
  'en-US': { preview: 'Preview', merged: 'Full Text', chunks: 'View Chunks' },
  'ja-JP': { preview: 'プレビュー', merged: '全文', chunks: 'チャンクを表示' },
  'ko-KR': { preview: '미리보기', merged: '전체 텍스트', chunks: '청크 보기' },
  'ru-RU': { preview: 'Предпросмотр', merged: 'Полный текст', chunks: 'Просмотр фрагментов' },
};

// R465/A1 — the mermaid fullscreen viewer copy for the preview tab, verbatim
// from the Vue i18n mermaid.* strings (frontend/src/i18n/locales/*.ts); the
// Record<Locale, …> shape keeps all five locales present at typecheck time.
const MERMAID_VIEWER_COPY: Record<Locale, DocumentMermaidLabels> = {
  'zh-CN': { zoomIn: '放大', zoomOut: '缩小', reset: '重置', download: '下载图片', downloading: '下载中...', close: '关闭', expand: '全屏查看' },
  'en-US': { zoomIn: 'Zoom In', zoomOut: 'Zoom Out', reset: 'Reset', download: 'Download Image', downloading: 'Downloading...', close: 'Close', expand: 'Expand' },
  'ja-JP': { zoomIn: '拡大', zoomOut: '縮小', reset: 'リセット', download: '画像をダウンロード', downloading: 'ダウンロード中...', close: '閉じる', expand: '全画面表示' },
  'ko-KR': { zoomIn: '확대', zoomOut: '축소', reset: '초기화', download: '이미지 다운로드', downloading: '다운로드 중...', close: '닫기', expand: '전체 화면' },
  'ru-RU': { zoomIn: 'Увеличить', zoomOut: 'Уменьшить', reset: 'Сброс', download: 'Скачать изображение', downloading: 'Загрузка...', close: 'Закрыть', expand: 'На весь экран' },
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

export function knowledgeTraceNodeState(row: KnowledgeTimelineNode): 'pending' | 'running' | 'done' | 'failed' | 'skipped' {
  const rawStatus = typeof row.node.status === 'string' ? row.node.status.toLowerCase() : '';
  if (/(fail|error|cancel|abort)/.test(rawStatus)) return 'failed';
  // R472-A1: a skipped stage never executed (multimodal disabled); Vue shows
  // knowledgeStages.status.skipped 已跳过 instead of a running/pending dot.
  if (rawStatus === 'skipped' || rawStatus === 'skip') return 'skipped';
  // R474/A3: Vue reads node.status verbatim — an explicit 'pending' span
  // stays pending ('—' duration, no row status text) even once started_at
  // has been serialized; the timestamp fallbacks below must not swallow it.
  if (rawStatus === 'pending') return 'pending';
  if (/(run|progress|active|start)/.test(rawStatus)) return 'running';
  // Backend spans serialize finished_at/started_at — Vue nodeStart/nodeEnd —
  // so both spellings close/open a statusless span.
  if (/(complete|done|success|finish|ok)/.test(rawStatus) || Boolean(row.node.end_time ?? row.node.finished_at)) return 'done';
  if (Boolean(row.node.start_time ?? row.node.started_at)) return 'running';
  return 'pending';
}

export function KnowledgeDocumentDetailPage({ client, documentId, onBack }: KnowledgeDocumentDetailPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const copy = DETAIL_COPY[locale];
  // Vue keeps the parent-context cache on the doc-content instance, which
  // survives a details.id switch; DocumentChunks unmounts while the next
  // document loads, so the cache lives on this persistent page component.
  const parentContextCache = useRef<Map<string, string>>(new Map());
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
    // Vue defaults file types to 「预览」 only when canPreview() holds; audio
    // and non-file documents open on the merged (全文) view instead.
    setContentView(canPreviewDocument(state.document) ? 'preview' : 'merged');
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
        getSpans: async (id) => resolveKnowledgeSpansView(await client.knowledgeBases.documents.spans(id)),
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
        const spans = resolveKnowledgeSpansView(await client.knowledgeBases.documents.spans(documentId));
        if (!active) return;
        const parseStatus = typeof spans.parse_status === 'string' ? spans.parse_status : state.document.parse_status;
        const nodes = flattenKnowledgeSpans(spans.trace);
        setTraceState({ status: 'success', steps: buildKnowledgeTimeline(spans), nodes, parseStatus, lastError: knowledgeSpansLastError(spans) });
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
    {(() => { const tabs = CONTENT_TABS[locale]; return <>
      {canPreviewDocument(state.document) ? <Button type="button" role="tab" aria-selected={contentView === 'preview'} onClick={() => setContentView('preview')}>{tabs.preview}</Button> : null}
      <Button type="button" role="tab" aria-selected={contentView === 'merged'} onClick={() => setContentView('merged')}>{tabs.merged}</Button>
      <Button type="button" role="tab" aria-selected={contentView === 'chunks'} onClick={() => setContentView('chunks')}>{tabs.chunks}</Button>
    </>; })()}
  </div><DocumentDetail client={client} document={state.document} canEdit={canMutateDocument} canDownload={canMutateDocument && documentCanDownload(state.document)} previewPath={client.knowledgeBases.documents.previewPath(documentId)} downloadPath={client.knowledgeBases.documents.downloadPath(documentId)} showPreview={contentView === 'preview'} /><DocumentChunks client={client} document={state.document} canEdit={canMutateDocument} view={contentView} parentContextCache={parentContextCache.current} /></> : null}
    </section>
  {traceOpen ? <Sheet open title={t('knowledgeBase.timeline.title')} onClose={() => setTraceOpen(false)} side="right" width="820px" resizable minWidth={560} maxWidth={1400} storageKey="weknora-trace-drawer-width" className="min-w-0 border-l border-line-soft">
    <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === 'loading'}>
      {traceState.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
      {traceState.status === 'error' ? <Status tone="error">{traceState.message}</Status> : null}
      {traceState.status === 'success' ? <div className="flex flex-col gap-4">
        {/* R474/A3: Vue drawer head low-cost affordances — the LIVE badge
            (kp-live-badge: parse polling or any running/pending span) and the
            当前阶段 n/5 counter (currentStageIndex: first running/failed
            stage, else traversed done/skipped + 1, capped). Attempt tabs and
            the stop-parsing control remain interactive follow-ups. */}
        <div className="flex flex-wrap items-center gap-2">
          {isKnowledgeProcessingActive(traceState.parseStatus) || traceState.nodes.some((row) => { const state = knowledgeTraceNodeState(row); return state === 'running' || state === 'pending'; }) ? <span className="wk-trace-live inline-flex items-center gap-1 rounded-full bg-surface-wash px-2 py-[2px] text-[11px] font-semibold text-warning-text" title={TRACE_HEAD_COPY[locale].liveTooltip}><span className="inline-block h-[6px] w-[6px] animate-pulse rounded-full bg-warning-text" aria-hidden="true" />LIVE</span> : null}
          {traceState.steps.length ? (() => { const runningIdx = traceState.steps.findIndex((step) => step.state === 'running' || step.state === 'failed'); const traversed = traceState.steps.filter((step) => step.state === 'done' || step.state === 'skipped').length; const current = runningIdx >= 0 ? runningIdx + 1 : Math.min(traversed + 1, traceState.steps.length); return <span className="text-[12px] text-muted">{TRACE_HEAD_COPY[locale].stagesProgress} <strong className="font-mono">{current}/{traceState.steps.length}</strong></span>; })() : null}
        </div>
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
            <button type="button" className="min-w-0 flex-1 truncate border-0 bg-transparent p-0 text-left font-mono text-ink hover:underline" onClick={() => setSelectedTraceNode(row)}>{row.node.name || row.node.stage || row.key}</button>{nodeState === 'pending' ? null : <span className={nodeState === 'failed' ? 'text-danger' : nodeState === 'done' ? 'text-success' : nodeState === 'running' ? 'text-primary' : 'text-muted'}>{t(`knowledgeBase.timeline.${nodeState}`)}</span>}<span className="w-20 shrink-0 text-right font-mono text-[11px] text-muted">{nodeState === 'pending' || typeof row.node.duration_ms !== 'number' ? '—' : `${row.node.duration_ms}ms`}</span>
          </li>; })}
        </ol></div> : null}
        {selectedTraceNode ? <section className="rounded-[8px] border border-line-soft bg-surface-wash p-3"><div className="mb-2 flex items-center justify-between gap-2"><strong className="truncate text-[13px]">{selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key}</strong><Button type="button" onClick={() => setSelectedTraceNode(null)}>{t('knowledgeBase.documents.cancel')}</Button></div><pre className="m-0 max-h-[240px] overflow-auto whitespace-pre-wrap break-words text-[11px] leading-[1.5] text-muted">{JSON.stringify(selectedTraceNode.node, null, 2)}</pre></section> : null}
      </div> : null}
    </section>
  </Sheet> : null}
  </main>
  </Sheet>;
}

function DocumentChunks({ client, document, canEdit, view, parentContextCache }: { client: WeKnoraClient; document: KnowledgeDocument; canEdit: boolean; view: ContentView; parentContextCache: Map<string, string> }) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  // Vue doc-content keeps loadedChunkPage (displayed page) separate from the
  // pagination v-model chunkPage: `page` is the loaded page, `pendingPage` the
  // requested one. While a page fetch is in flight the section header and
  // pagination stay mounted and the content area shows the small
  // chunk-page-loading row (Vue v-else hides the stale page); a failed fetch
  // restores the loaded page with its content (Vue chunkLoadError branch).
  const [state, setState] = useState<{ status: 'loading' | 'success' | 'error'; chunks: KnowledgeChunk[]; total: number; page: number; pendingPage?: number; message?: string }>({ status: 'loading', chunks: [], total: 0, page: 1 });
  const [pageError, setPageError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [history, setHistory] = useState<{ id: string; rows: KnowledgeChunkRevision[] } | null>(null);
  const [historyLoading, setHistoryLoading] = useState<string | null>(null);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [retryNotice, setRetryNotice] = useState<{ tone: 'success' | 'error'; message: string } | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  // Vue parentContextPopup/Cache/Loading: the git-branch entry opens a
  // parent-context panel that lazy-loads GET /chunks/by-id/{parent_chunk_id}
  // once per parent id; failures surface parentContextLoadFailed and close.
  const [parentContextId, setParentContextId] = useState<string | null>(null);
  const [parentContextLoading, setParentContextLoading] = useState<string | null>(null);
  const [parentContextError, setParentContextError] = useState<string | null>(null);
  // Vue questionPopupChunk/questionComposerChunk/editingQuestionKey & friends
  // (doc-content L1323-1475): the questions panel, add composer, inline row
  // editor, popconfirm delete, and regenerate each keep their own busy state.
  const [questionsId, setQuestionsId] = useState<string | null>(null);
  const [questionComposerId, setQuestionComposerId] = useState<string | null>(null);
  const [questionDraft, setQuestionDraft] = useState('');
  const [editingQuestion, setEditingQuestion] = useState<{ chunkId: string; questionId: string } | null>(null);
  const [questionEditDraft, setQuestionEditDraft] = useState('');
  const [savingQuestionComposer, setSavingQuestionComposer] = useState<string | null>(null);
  const [savingQuestionKey, setSavingQuestionKey] = useState<string | null>(null);
  const [regeneratingQuestions, setRegeneratingQuestions] = useState<string | null>(null);
  const [deletingQuestion, setDeletingQuestion] = useState<{ chunkId: string; questionId: string } | null>(null);
  const [confirmingDelete, setConfirmingDelete] = useState<{ chunkId: string; questionId: string } | null>(null);
  const [questionNotice, setQuestionNotice] = useState<{ tone: 'success' | 'error' | 'warning'; message: string } | null>(null);

  const closeQuestions = () => {
    setQuestionsId(null);
    setQuestionComposerId(null);
    setQuestionDraft('');
    setEditingQuestion(null);
    setQuestionEditDraft('');
    setConfirmingDelete(null);
  };

  const patchChunkRow = (chunkId: string, patch: (chunk: KnowledgeChunk) => KnowledgeChunk) => {
    setState((current) => ({ ...current, chunks: current.chunks.map((row) => row.id === chunkId ? patch(row) : row) }));
  };

  const load = (page = 1) => {
    setPageError(null);
    setState((current) => ({ ...current, status: 'loading', pendingPage: page, message: undefined }));
    void client.knowledgeBases.documents.chunks(document.id, page).then((result) => {
      setState({ status: 'success', chunks: result.data, total: result.total, page: result.page, pendingPage: undefined });
    }).catch((error: unknown) => {
      const message = error instanceof Error ? error.message : t('common.error');
      setPageError(message);
      setState((current) => current.chunks.length > 0
        ? { ...current, status: 'success', pendingPage: undefined }
        : { status: 'error', chunks: [], total: 0, page: current.pendingPage ?? current.page, pendingPage: undefined, message });
    });
  };
  // A document switch must not leak the previous document's rows through the
  // transition state, so reset before the initial load (Vue resets
  // chunkPage/loadedChunkPage when details.id changes).
  useEffect(() => {
    setPageError(null);
    setState({ status: 'loading', chunks: [], total: 0, page: 1, pendingPage: undefined, message: undefined });
    // Vue watch(details.id) closes the question popup, composer, and inline
    // editor alongside the parent-context popup.
    setParentContextId(null);
    setParentContextError(null);
    closeQuestions();
    load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, document.id]);

  const pageTransition = state.status === 'loading' && state.chunks.length > 0;

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

  /** Vue retryChunkIndex: only expected_revision travels; a still-failed result raises indexFailed, otherwise indexRetrySuccess. */
  const retryIndex = async (chunk: KnowledgeChunk) => {
    setMutationError(null);
    setRetryNotice(null);
    setRetryingId(chunk.id);
    try {
      const updated = await client.knowledgeBases.documents.updateChunk(document.id, chunk.id, { expected_revision: chunk.content_revision ?? 0 });
      setState((current) => ({ ...current, chunks: current.chunks.map((row) => row.id === chunk.id ? updated : row) }));
      setRetryNotice(updated.index_status === 'failed'
        ? { tone: 'error', message: t('knowledgeBase.indexFailed') }
        : { tone: 'success', message: t('knowledgeBase.indexRetrySuccess') });
    } catch (error: unknown) {
      setRetryNotice({ tone: 'error', message: error instanceof Error ? error.message : t('common.error') });
    } finally { setRetryingId(null); }
  };

  const showHistory = (chunk: KnowledgeChunk) => {
    setParentContextId(null);
    closeQuestions();
    setHistoryLoading(chunk.id);
    void client.knowledgeBases.documents.chunkRevisions(document.id, chunk.id).then((rows) => setHistory({ id: chunk.id, rows })).catch((error: unknown) => setState((current) => ({ ...current, message: error instanceof Error ? error.message : t('common.error') }))).finally(() => setHistoryLoading(null));
  };

  /** Vue setParentContextPopupVisible + loadParentContext: opening the parent
   * context closes the question/history expansions (mutex) and lazy-loads the
   * parent chunk once per parent id; failures toast parentContextLoadFailed
   * and close the popup. */
  const openParentContext = (chunk: KnowledgeChunk) => {
    if (parentContextId === chunk.id) {
      setParentContextId(null);
      return;
    }
    setParentContextId(chunk.id);
    setEditingId(null);
    setDraft('');
    setHistory(null);
    setParentContextError(null);
    closeQuestions();
    const parentId = parentChunkId(chunk);
    if (!parentId || parentContextCache.has(parentId)) return;
    setParentContextLoading(chunk.id);
    void client.knowledgeBases.documents.getChunkById(parentId).then((parent) => {
      parentContextCache.set(parentId, parent.content || '');
    }).catch(() => {
      setParentContextError(t('knowledgeBase.parentContextLoadFailed'));
      setParentContextId(null);
    }).finally(() => setParentContextLoading(null));
  };

  /** Vue setQuestionPopupVisible: opening the questions panel closes the
   * parent-context and history expansions; closing also resets the composer
   * and the inline editor (closeQuestionComposer + cancelQuestionEdit). */
  const openQuestions = (chunk: KnowledgeChunk) => {
    if (questionsId === chunk.id) {
      closeQuestions();
      return;
    }
    setQuestionsId(chunk.id);
    setParentContextId(null);
    setEditingId(null);
    setDraft('');
    setHistory(null);
    setQuestionComposerId(null);
    setQuestionDraft('');
    setEditingQuestion(null);
    setConfirmingDelete(null);
  };

  /** Vue addQuestion: upsert without a question id, then merge result.data
   * into the local metadata and toast common.saveSuccess. */
  const addQuestion = async (chunk: KnowledgeChunk) => {
    const question = questionDraft.trim();
    if (!question) return;
    setSavingQuestionComposer(chunk.id);
    try {
      const saved = await client.knowledgeBases.documents.upsertGeneratedQuestion(chunk.id, question);
      patchChunkRow(chunk.id, (row) => upsertLocalQuestion(row, saved));
      setQuestionDraft('');
      setQuestionComposerId(null);
      setQuestionNotice({ tone: 'success', message: t('common.saveSuccess') });
    } catch (error: unknown) {
      setQuestionNotice({ tone: 'error', message: error instanceof Error ? error.message : t('common.error') });
    } finally { setSavingQuestionComposer(null); }
  };

  /** Vue saveQuestionEdit: unchanged text just cancels; otherwise upsert with
   * the existing question id and swap in the saved row. */
  const saveQuestionEdit = async (chunk: KnowledgeChunk, question: KnowledgeGeneratedQuestion) => {
    const value = questionEditDraft.trim();
    if (!value) return;
    if (value === question.question) {
      setEditingQuestion(null);
      setQuestionEditDraft('');
      return;
    }
    const key = `${chunk.id}:${question.id}`;
    setSavingQuestionKey(key);
    try {
      const saved = await client.knowledgeBases.documents.upsertGeneratedQuestion(chunk.id, value, question.id);
      patchChunkRow(chunk.id, (row) => upsertLocalQuestion(row, saved));
      setEditingQuestion(null);
      setQuestionEditDraft('');
      setQuestionNotice({ tone: 'success', message: t('common.saveSuccess') });
    } catch (error: unknown) {
      setQuestionNotice({ tone: 'error', message: error instanceof Error ? error.message : t('common.error') });
    } finally { setSavingQuestionKey(null); }
  };

  /** Vue handleDeleteQuestion: DELETE by question id, then splice the row out
   * of the local metadata (the popconfirm gate lives on the row buttons). */
  const deleteQuestion = async (chunk: KnowledgeChunk, question: KnowledgeGeneratedQuestion) => {
    // R474/A3: Vue handleDeleteQuestion (doc-content.vue) keeps a defensive
    // guard behind the hidden row buttons — legacy- ids can never reach
    // DELETE /chunks/by-id/:id/questions; they surface
    // knowledgeBase.legacyQuestionCannotDelete instead.
    if (isLegacyGeneratedQuestion(question)) {
      setQuestionNotice({ tone: 'warning', message: t('knowledgeBase.legacyQuestionCannotDelete') });
      return;
    }
    setDeletingQuestion({ chunkId: chunk.id, questionId: question.id });
    try {
      await client.knowledgeBases.documents.deleteGeneratedQuestion(chunk.id, question.id);
      patchChunkRow(chunk.id, (row) => {
        const metadata = chunkMetadata(row);
        if (Array.isArray(metadata.generated_questions)) {
          metadata.generated_questions = metadata.generated_questions.filter((item) => !(typeof item === 'object' && item !== null && (item as { id?: unknown }).id === question.id));
        }
        return writeChunkMetadata(row, metadata);
      });
      setConfirmingDelete(null);
      setQuestionNotice({ tone: 'success', message: t('common.deleteSuccess') });
    } catch (error: unknown) {
      setQuestionNotice({ tone: 'error', message: error instanceof Error ? error.message : t('common.deleteFailed') });
    } finally { setDeletingQuestion(null); }
  };

  /** Vue regenerateQuestions: the response array replaces generated_questions
   * and generated_questions_revision pins to the current content_revision. */
  const regenerateQuestions = async (chunk: KnowledgeChunk) => {
    setRegeneratingQuestions(chunk.id);
    try {
      const rows = await client.knowledgeBases.documents.regenerateGeneratedQuestions(chunk.id);
      patchChunkRow(chunk.id, (row) => {
        const metadata = chunkMetadata(row);
        metadata.generated_questions = rows;
        metadata.generated_questions_revision = row.content_revision || 0;
        return writeChunkMetadata(row, metadata);
      });
      setQuestionComposerId(null);
      setQuestionNotice({ tone: 'success', message: t('knowledgeBase.questionsRegenerated') });
    } catch (error: unknown) {
      setQuestionNotice({ tone: 'error', message: error instanceof Error ? error.message : t('common.error') });
    } finally { setRegeneratingQuestions(null); }
  };

  const mergedContent = mergeChunkContents(state.chunks);
  return <section className="wk-document-chunks mt-4" aria-label={t('knowledgeBase.viewChunks')} hidden={view === 'preview'}>
    <div className="mb-3 flex items-center justify-between gap-3"><h3 className="m-0 text-[13px] font-semibold">{t('knowledgeBase.viewChunks')} {state.total ? `(${state.total})` : ''}</h3></div>
    {mutationError ? <Status tone="error">{mutationError}</Status> : null}
    {retryNotice ? <Status tone={retryNotice.tone}>{retryNotice.message}</Status> : null}
    {questionNotice ? <Status tone={questionNotice.tone}>{questionNotice.message}</Status> : null}
    {parentContextError ? <Status tone="error">{parentContextError}</Status> : null}
    {state.status === 'loading' && !pageTransition ? <Status>{t('common.loading')}</Status> : null}
    {pageTransition ? <div className="wk-chunk-page-loading" role="status"><Status>{t('common.loading')}</Status></div> : null}
    {state.status === 'error' ? <><Status tone="error">{state.message}</Status><Button type="button" onClick={() => load(state.page)}>{t('common.retry')}</Button></> : null}
    {state.status === 'success' && pageError ? <><Status tone="error">{pageError}</Status><Button type="button" onClick={() => load(state.page)}>{t('common.retry')}</Button></> : null}
    {state.status === 'success' && state.chunks.length === 0 ? <Status>{t('common.empty')}</Status> : null}
    {state.status === 'success' && view === 'merged'
      ? mergedContent
        // Vue renders 全文 through processMarkdown then runs the mermaid
        // post-render pipeline; DocumentMarkdownBody renders the shared
        // markdown engine and hydrates inline mermaid blocks with the same
        // click-to-fullscreen behavior as the preview tab.
        ? <DocumentMarkdownBody markdown={mergedContent} labels={MERMAID_VIEWER_COPY[locale]} className="wk-document-merged markdown-content min-w-0 text-[13px] leading-[1.65] text-ink [overflow-wrap:anywhere] [&_p]:my-[0.4em] [&_p:first-child]:mt-0 [&_p:last-child]:mb-0 [&_h1]:mb-[0.4em] [&_h1]:mt-[0.8em] [&_h1]:text-lg [&_h1]:leading-[1.3] [&_h2]:mb-[0.4em] [&_h2]:mt-[0.8em] [&_h2]:text-base [&_h2]:leading-[1.3] [&_h3]:mb-[0.4em] [&_h3]:mt-[0.8em] [&_h3]:text-sm [&_h3]:leading-[1.3] [&_ul]:my-[0.5em] [&_ul]:pl-5 [&_ol]:my-[0.5em] [&_ol]:pl-5 [&_li]:my-[0.15em] [&_pre]:my-[0.6em] [&_pre]:overflow-x-auto [&_pre]:rounded-card [&_pre]:bg-surface-muted [&_pre]:px-3 [&_pre]:py-2.5 [&_pre]:text-xs [&_code]:font-mono [&_code]:text-xs [&_blockquote]:my-[0.6em] [&_blockquote]:border-l-2 [&_blockquote]:border-line-soft [&_blockquote]:pl-3" />
        : <div className="wk-document-merged text-[13px] text-muted">—</div>
      : null}
    {state.status === 'success' && view !== 'merged' ? <><div className="flex flex-col gap-3">{state.chunks.map((chunk, index) => <article key={chunk.id} className="rounded-[8px] border border-line-soft bg-surface p-3" data-chunk-id={chunk.id}>
      <div className="mb-2 flex items-center justify-between gap-2"><strong className="text-[12px]">{t('knowledgeBase.segment')} {(state.page - 1) * 25 + index + 1}</strong>{parentChunkId(chunk) || generatedQuestions(chunk).length > 0 || canEdit ? <span className="flex flex-wrap gap-1">{parentChunkId(chunk) ? <Button type="button" variant="text" className="wk-parent-context-toggle" title={t('knowledgeBase.viewParentContext')} aria-label={t('knowledgeBase.viewParentContext')} aria-expanded={parentContextId === chunk.id} onClick={() => openParentContext(chunk)}><GitBranchIcon /></Button> : null}{generatedQuestions(chunk).length > 0 || canEdit ? <Button type="button" variant="text" className="wk-chunk-questions-toggle" title={t('knowledgeBase.generatedQuestions')} aria-label={t('knowledgeBase.generatedQuestions')} aria-expanded={questionsId === chunk.id} onClick={() => openQuestions(chunk)}><HelpCircleIcon /></Button> : null}{canEdit ? <><Button type="button" onClick={() => { setParentContextId(null); closeQuestions(); setEditingId(chunk.id); setDraft(chunk.content || ''); }}>{t('common.edit')}</Button><Button type="button" loading={historyLoading === chunk.id} onClick={() => showHistory(chunk)}>{t('knowledgeBase.chunkHistory')}</Button><Button type="button" loading={savingId === chunk.id} onClick={() => void toggleEnabled(chunk)}>{chunk.is_enabled ? t('knowledgeBase.disableChunk') : t('knowledgeBase.enableChunk')}</Button>{chunk.index_status === 'failed' ? <Button type="button" title={t('knowledgeBase.retryIndex')} aria-label={t('knowledgeBase.retryIndex')} loading={retryingId === chunk.id} onClick={() => void retryIndex(chunk)}>{t('knowledgeBase.retryIndex')}</Button> : null}</> : null}</span> : null}</div>
      {editingId === chunk.id ? <><textarea aria-label={t('knowledgeBase.segment')} value={draft} onChange={(event) => setDraft(event.target.value)} className="min-h-[120px] w-full rounded-control border border-line-soft p-2" /><div className="mt-2 flex gap-2"><Button type="button" loading={savingId === chunk.id} onClick={() => void save(chunk)}>{t('common.save')}</Button><Button type="button" onClick={() => { setEditingId(null); setDraft(''); }}>{t('common.cancel')}</Button></div></> : <DocumentMarkdownBody markdown={chunk.content || '—'} labels={MERMAID_VIEWER_COPY[locale]} className="wk-document-chunk-content markdown-content m-0 min-w-0 text-[13px] text-ink [overflow-wrap:anywhere]" />}
      {history?.id === chunk.id ? <div className="mt-3 border-t border-line-soft pt-3"><strong className="text-[12px]">{t('knowledgeBase.chunkHistory')}</strong>{history?.rows.length === 0 ? <Status>{t('common.noData')}</Status> : <ol className="m-0 mt-2 list-decimal pl-5 text-[12px]">{history?.rows.map((row) => <li key={row.revision} className="mb-2"><span>Revision {row.revision}: {row.content || '—'}</span><Button type="button" className="ml-2" onClick={() => void (async () => { const updated = await client.knowledgeBases.documents.revertChunk(document.id, chunk.id, row.revision, chunk.content_revision ?? 0); setState((current) => ({ ...current, chunks: current.chunks.map((item) => item.id === chunk.id ? updated : item) })); showHistory(updated); })()}>{t('knowledgeBase.chunkReverted')}</Button></li>)}</ol>}</div> : null}
      {parentContextId === chunk.id && parentChunkId(chunk) ? <div className="wk-chunk-parent-context mt-3 border-t border-line-soft pt-3" aria-label={t('knowledgeBase.viewParentContext')}>
        <div className="mb-2 flex items-center gap-1 text-[12px] font-semibold"><span className="text-muted"><GitBranchIcon /></span>{t('knowledgeBase.viewParentContext')}</div>
        {parentContextLoading === chunk.id
          ? <div className="chunk-popup-state" role="status"><Status>{t('common.loading')}</Status></div>
          : <DocumentMarkdownBody markdown={parentContextCache.get(parentChunkId(chunk)!) || ''} labels={MERMAID_VIEWER_COPY[locale]} className="wk-chunk-parent-context-body markdown-content m-0 min-w-0 max-h-[480px] overflow-auto text-[13px] text-muted [overflow-wrap:anywhere]" />}
      </div> : null}
      {questionsId === chunk.id ? (() => { const rows = generatedQuestions(chunk); return <div className="wk-chunk-questions mt-3 border-t border-line-soft pt-3" aria-label={t('knowledgeBase.generatedQuestions')}>
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <div className="flex min-w-0 flex-wrap items-center gap-1 text-[12px] font-semibold"><span className="text-muted"><HelpCircleIcon /></span>{t('knowledgeBase.generatedQuestions')}<span className="font-normal text-muted">{rows.length}</span>{hasStaleGeneratedQuestions(chunk) ? <span className="font-normal text-muted">{t('knowledgeBase.staleGeneratedQuestions')}</span> : null}</div>
          {canEdit ? <span className="flex flex-wrap gap-1">
            <Button type="button" variant="text" title={t('knowledgeBase.addGeneratedQuestion')} aria-label={t('knowledgeBase.addGeneratedQuestion')} onClick={() => { setQuestionComposerId(chunk.id); setQuestionDraft(''); setEditingQuestion(null); setConfirmingDelete(null); }}>＋</Button>
            <Button type="button" variant="text" title={t('knowledgeBase.regenerateQuestions')} aria-label={t('knowledgeBase.regenerateQuestions')} loading={regeneratingQuestions === chunk.id} onClick={() => void regenerateQuestions(chunk)}>↻</Button>
          </span> : null}
        </div>
        {canEdit && questionComposerId === chunk.id ? <div className="question-composer mb-2 flex items-center gap-1">
          <input value={questionDraft} placeholder={t('knowledgeBase.addGeneratedQuestion')} aria-label={t('knowledgeBase.addGeneratedQuestion')} className="min-w-0 flex-1 rounded-control border border-line-soft px-2 py-1 text-[13px]" onChange={(event) => setQuestionDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void addQuestion(chunk); }} />
          <Button type="button" variant="text" title={t('common.cancel')} aria-label={t('common.cancel')} disabled={savingQuestionComposer === chunk.id} onClick={() => { setQuestionComposerId(null); setQuestionDraft(''); }}>×</Button>
          <Button type="button" loading={savingQuestionComposer === chunk.id} disabled={!questionDraft.trim()} onClick={() => void addQuestion(chunk)}>{t('common.add')}</Button>
        </div> : null}
        {rows.length ? <ul className="questions-list m-0 flex list-none flex-col gap-2 p-0">
          {rows.map((question) => <li key={question.id} className="question-item flex items-start gap-2 text-[13px]">
            <span className="shrink-0 text-muted"><HelpCircleIcon /></span>
            {editingQuestion?.chunkId === chunk.id && editingQuestion.questionId === question.id ? <span className="flex min-w-0 flex-1 items-center gap-1">
              <input value={questionEditDraft} aria-label={t('common.edit')} className="min-w-0 flex-1 rounded-control border border-line-soft px-2 py-1 text-[13px]" onChange={(event) => setQuestionEditDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void saveQuestionEdit(chunk, question); }} />
              <Button type="button" variant="text" onClick={() => { setEditingQuestion(null); setQuestionEditDraft(''); }}>{t('common.cancel')}</Button>
              <Button type="button" loading={savingQuestionKey === `${chunk.id}:${question.id}`} onClick={() => void saveQuestionEdit(chunk, question)}>{t('common.save')}</Button>
            </span> : confirmingDelete?.chunkId === chunk.id && confirmingDelete.questionId === question.id ? <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
              <span className="min-w-0 flex-1 text-muted">{t('knowledgeBase.confirmDeleteQuestion')}</span>
              <Button type="button" variant="text" onClick={() => setConfirmingDelete(null)}>{t('common.cancel')}</Button>
              <Button type="button" loading={deletingQuestion?.chunkId === chunk.id && deletingQuestion.questionId === question.id} onClick={() => void deleteQuestion(chunk, question)}>{t('common.confirmDelete')}</Button>
            </span> : <>
              <span className="question-text min-w-0 flex-1">{question.question}</span>
              {canEdit && !question.id.startsWith('legacy-') ? <span className="question-actions flex shrink-0 gap-1">
                <Button type="button" variant="text" title={t('common.edit')} aria-label={t('common.edit')} onClick={() => { setEditingQuestion({ chunkId: chunk.id, questionId: question.id }); setQuestionEditDraft(question.question); setConfirmingDelete(null); }}>{t('common.edit')}</Button>
                <Button type="button" variant="text" title={t('common.delete')} aria-label={t('common.delete')} onClick={() => { setConfirmingDelete({ chunkId: chunk.id, questionId: question.id }); setEditingQuestion(null); }}>{t('common.delete')}</Button>
              </span> : null}
            </>}
          </li>)}
        </ul> : questionComposerId !== chunk.id ? <div className="questions-empty flex items-center gap-2 text-[13px] text-muted">{t('knowledgeBase.noGeneratedQuestions')}</div> : null}
      </div>; })() : null}
    </article>)}</div></> : null}
    {/* Vue renders the chunk pagination for both merged and chunks views
        (viewMode merged || chunks), so 全文 can advance past page one, and
        keeps it mounted across a page transition with the requested page
        shown (v-model chunkPage) while in-flight clicks are ignored. */}
    {(state.status === 'success' || pageTransition) && view !== 'preview' && state.total > 25 ? <nav className="mt-3 flex items-center justify-between" aria-label={t('knowledgeBase.viewChunks')}><Button type="button" disabled={state.page <= 1 || pageTransition} onClick={() => load(state.page - 1)}>{t('common.back')}</Button><span>{state.pendingPage ?? state.page}</span><Button type="button" disabled={state.page * 25 >= state.total || pageTransition} onClick={() => load(state.page + 1)}>{t('common.next')}</Button></nav> : null}
  </section>;
}

type PreviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'text'; text: string }
  | { status: 'blob'; url: string }
  | { status: 'spreadsheet'; spreadsheet: SpreadsheetPreviewModel }
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
    // Vue fetches preview/audio content purely from type + extension
    // (canPreview/model kind) — parse_status never gates the fetch here.
    if (!model.ready) {
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
      if (model.kind === 'text' || model.kind === 'markdown' || model.kind === 'mermaid') {
        const text = await readCurrentPreviewText(response.body, () => active);
        if (text !== undefined) setPreviewState({ status: 'text', text });
        return;
      }
      if (model.kind === 'spreadsheet') {
        const spreadsheet = await readSpreadsheetPreview(response.body, model.fileName);
        if (active) setPreviewState({ status: 'spreadsheet', spreadsheet });
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
    {/* Vue pins an embedded audio player above the content views for audio
        files (audio-player-section), since canPreview() keeps them off the
        preview tab; the blob is already fetched by the preview effect. */}
    {inlineKind === 'audio' && previewState.status === 'blob' ? <div className="wk-document-audio-player mb-3 border-b border-line-soft pb-3"><DocumentPreviewContent kind="audio" url={previewState.url} fileName={model.fileName} /></div> : null}
    {showPreview ? (previewState.status === 'loading' ? <Status>{copy.loading}</Status> : previewState.status === 'error' ? <><Status tone="error">{previewState.message}</Status><Button type="button" onClick={() => setPreviewAttempt((attempt) => attempt + 1)}>{copy.retry}</Button></> : previewState.status === 'text' && inlineKind ? <DocumentPreviewContent kind={inlineKind} text={previewState.text} fileName={model.fileName} mermaidLabels={MERMAID_VIEWER_COPY[locale]} /> : previewState.status === 'spreadsheet' && inlineKind ? <DocumentPreviewContent kind={inlineKind} spreadsheet={previewState.spreadsheet} fileName={model.fileName} /> : previewState.status === 'blob' && inlineKind ? <DocumentPreviewContent kind={inlineKind} url={previewState.url} fileName={model.fileName} /> : null) : null}
    {canDownload && downloadState === 'error' ? <Status tone="error">{copy.downloadFailed}</Status> : null}
  </Card>;
}

function formatDetailTime(value: unknown): string {
  if (typeof value !== 'string' || !value) return String(value ?? '');
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

/** Vue doc-content git-branch icon (15px, feather-style strokes). */
function GitBranchIcon() {
  return <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><line x1="6" y1="3" x2="6" y2="15" /><circle cx="18" cy="6" r="3" /><circle cx="6" cy="18" r="3" /><path d="M18 9a9 9 0 0 1-9 9" /></svg>;
}

/** Vue hasParentChunk: the parent-context entry exists only while the chunk carries a parent_chunk_id. */
function parentChunkId(chunk: KnowledgeChunk): string | null {
  const value = chunk.parent_chunk_id;
  return typeof value === 'string' && value ? value : null;
}

function HelpCircleIcon() {
  return <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg>;
}

/** Vue getChunkMetadata: chunk metadata arrives either as a JSON string or an object. */
function chunkMetadata(chunk: KnowledgeChunk): Record<string, unknown> {
  const raw = chunk.metadata;
  if (typeof raw === 'string') {
    try { const parsed = JSON.parse(raw || '{}'); return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {}; } catch { return {}; }
  }
  return typeof raw === 'object' && raw !== null && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
}

/** R474/A3: Vue doc-content gates legacy- question ids twice — the row
 * buttons hide (template !startsWith('legacy-')) and handleDeleteQuestion
 * still guards (warning + return) so a legacy id can never reach the
 * DELETE endpoint. */
export function isLegacyGeneratedQuestion(question: KnowledgeGeneratedQuestion): boolean {
  return question.id.startsWith('legacy-');
}

/** Vue getGeneratedQuestions: metadata.generated_questions with legacy string
 * entries mapped onto legacy-{index} ids that can never be edited or deleted. */
function generatedQuestions(chunk: KnowledgeChunk): KnowledgeGeneratedQuestion[] {  const list = chunkMetadata(chunk).generated_questions;
  if (!Array.isArray(list)) return [];
  return list.map((item, index) => {
    if (typeof item === 'string') return { id: `legacy-${index}`, question: item };
    if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
      const row = item as { id?: unknown; question?: unknown; content_revision?: unknown };
      return {
        id: typeof row.id === 'string' && row.id ? row.id : `legacy-${index}`,
        question: typeof row.question === 'string' ? row.question : '',
        ...(typeof row.content_revision === 'number' ? { content_revision: row.content_revision } : {}),
      };
    }
    return { id: `legacy-${index}`, question: '' };
  });
}

/** Vue hasStaleGeneratedQuestions: a question pinned to an older content_revision
 * than the chunk revision (falling back to metadata.generated_questions_revision). */
function hasStaleGeneratedQuestions(chunk: KnowledgeChunk): boolean {
  const questions = generatedQuestions(chunk);
  if (!questions.length) return false;
  const metadata = chunkMetadata(chunk);
  const fallbackRevision = typeof metadata.generated_questions_revision === 'number' ? metadata.generated_questions_revision : 0;
  const currentRevision = typeof chunk.content_revision === 'number' ? chunk.content_revision : 0;
  return questions.some((question) => (typeof question.content_revision === 'number' ? question.content_revision : fallbackRevision) !== currentRevision);
}

/** Vue keeps the metadata kind: string chunks get a re-serialized string back. */
function writeChunkMetadata(chunk: KnowledgeChunk, metadata: Record<string, unknown>): KnowledgeChunk {
  return { ...chunk, metadata: typeof chunk.metadata === 'string' ? JSON.stringify(metadata) : metadata };
}

/** Vue upsertChunkGeneratedQuestion: upsert onto the raw generated_questions array. */
function upsertLocalQuestion(chunk: KnowledgeChunk, question: KnowledgeGeneratedQuestion): KnowledgeChunk {
  const metadata = chunkMetadata(chunk);
  const rows = Array.isArray(metadata.generated_questions) ? [...metadata.generated_questions] : [];
  const index = rows.findIndex((row) => typeof row === 'object' && row !== null && (row as { id?: unknown }).id === question.id);
  if (index >= 0) rows[index] = question;
  else rows.push(question);
  metadata.generated_questions = rows;
  return writeChunkMetadata(chunk, metadata);
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
