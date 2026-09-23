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
import './documents-u.css';

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

const DETAIL_COPY: Record<Locale, { load: string; bytes: string; status: string; source: string; folder: string; type: string; root: string; loading: string; retry: string; download: string; downloadFailed: string; fullscreen: string; exitFullscreen: string }> = {
  'zh-CN': { load: '文档加载失败', bytes: '文档内容读取失败', status: '状态', source: '来源', folder: '文件夹', type: '类型', root: '根目录', loading: '正在加载预览…', retry: '重试预览', download: '下载', downloadFailed: '下载失败。', fullscreen: '全屏', exitFullscreen: '退出全屏' },
  'en-US': { load: 'Unable to load document', bytes: 'Unable to load document bytes', status: 'Status', source: 'Source', folder: 'Folder', type: 'Type', root: 'Root', loading: 'Loading preview…', retry: 'Retry preview', download: 'Download', downloadFailed: 'Download failed.', fullscreen: 'Fullscreen', exitFullscreen: 'Exit fullscreen' },
  'ja-JP': { load: 'ドキュメントを読み込めません', bytes: 'ドキュメント内容を読み込めません', status: '状態', source: 'ソース', folder: 'フォルダー', type: '種類', root: 'ルート', loading: 'プレビューを読み込み中…', retry: 'プレビューを再試行', download: 'ダウンロード', downloadFailed: 'ダウンロードに失敗しました。', fullscreen: '全画面表示', exitFullscreen: '全画面表示を終了' },
  'ko-KR': { load: '문서를 불러오지 못했습니다', bytes: '문서 내용을 불러오지 못했습니다', status: '상태', source: '소스', folder: '폴더', type: '유형', root: '루트', loading: '미리보기를 불러오는 중…', retry: '미리보기 다시 시도', download: '다운로드', downloadFailed: '다운로드하지 못했습니다.', fullscreen: '전체 화면', exitFullscreen: '전체 화면 종료' },
  'ru-RU': { load: 'Не удалось загрузить документ', bytes: 'Не удалось загрузить содержимое документа', status: 'Статус', source: 'Источник', folder: 'Папка', type: 'Тип', root: 'Корень', loading: 'Загрузка предпросмотра…', retry: 'Повторить предпросмотр', download: 'Скачать', downloadFailed: 'Не удалось скачать файл.', fullscreen: 'На весь экран', exitFullscreen: 'Выйти из полноэкранного режима' },
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

  const [headerDownloadState, setHeaderDownloadState] = useState<'idle' | 'loading' | 'error'>('idle');
  async function downloadHeader() {
    if (state.status !== 'success') return;
    setHeaderDownloadState('loading');
    try {
      const response = await client.knowledgeBases.documents.download(documentId);
      const contentType = response.contentType || response.headers['content-type'];
      const url = URL.createObjectURL(previewBodyAsBlob(response.body, contentType));
      const anchor = window.document.createElement('a');
      anchor.href = url;
      anchor.download = buildDocumentPreview(state.document, client.knowledgeBases.documents.previewPath(documentId)).fileName;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
      setHeaderDownloadState('idle');
    } catch {
      setHeaderDownloadState('error');
    }
  }

  const detailTitle = state.status === 'success' ? documentDetailTitle(state.document, t('common.typeDocument')) : t('common.typeDocument');
  const document = state.status === 'success' ? state.document : null;
  const headerDownloadVisible = document !== null && canMutateDocument && documentCanDownload(document);
  // Vue DocContent header (doc-content.vue L1584-1600): icon + title at the
  // left, download / chart-line header actions at the right, then the drawer
  // close button. The Sheet close button follows the title node, so the
  // actions ride inside the title node's right group.
  return <Sheet open onClose={onBack} closeLabel={t('common.close')} resizeLabel="Resize drawer" side="right" width="653px" resizable minWidth={480} maxWidth={1600} storageKey="weknora-doc-drawer-width" className="wk-document-detail-drawer [&header_h2]:w-full"
    headerIcon={<span className="doc-drawer-header-icon wk-kdd-1"><FileDetailIcon size={16} /></span>}
    title={<span className="doc-drawer-header-inner wk-kdd-2">
      <span className="doc-drawer-header-title wk-kdd-3">{detailTitle}</span>
      <span className="doc-drawer-header-actions wk-kdd-4">
        {headerDownloadVisible ? <button type="button" className="header-action-btn wk-kdd-5" aria-label={copy.download} title={copy.download} onClick={() => void downloadHeader()}>{headerDownloadState === 'loading' ? <span className="border-t-transparent wk-kdd-6" aria-hidden="true" /> : <DownloadIcon size={16} />}</button> : null}
        {document ? <button type="button" className="header-action-btn wk-kdd-5" aria-label={t('knowledgeBase.timeline.title')} title={t('knowledgeBase.timeline.title')} onClick={() => setTraceOpen(true)}><ChartLineIcon size={16} /></button> : null}
      </span>
    </span>}
  >
  <main className="wk-page wk-document-detail-page wk-kdd-7">
    <section className="wk-document-detail-surface" aria-live="polite" aria-busy={state.status === 'loading'}>
    {state.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
    {state.status === 'empty' ? <Status>{t('common.empty')}</Status> : null}
    {state.status === 'error' ? <div className="wk-kdd-8"><Status tone="error">{state.message}</Status><Button type="button" variant="text" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>{t('common.retry')}</Button></div> : null}
    {state.status === 'success' && timelineSteps.length > 0 ? <Card><section aria-label={t('knowledgeBase.timeline.title')} className="wk-processing-timeline"><strong>{t('knowledgeBase.timeline.title')}</strong><ol>{timelineSteps.map((step) => <li key={step.stage} data-state={step.state}>{t('knowledgeBase.timeline.stage.' + step.stage)} — {t('knowledgeBase.timeline.' + step.state)}</li>)}</ol></section></Card> : null}
  {state.status === 'success' ? <DocumentDetail client={client} document={state.document} canEdit={canMutateDocument} contentView={contentView} onContentViewChange={setContentView} parentContextCache={parentContextCache.current} /> : null}
    </section>
  {traceOpen ? <Sheet open title={t('knowledgeBase.timeline.title')} onClose={() => setTraceOpen(false)} side="right" width="820px" resizable minWidth={560} maxWidth={1400} storageKey="weknora-trace-drawer-width" className="wk-kdd-9">
    <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === 'loading'}>
      {traceState.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
      {traceState.status === 'error' ? <Status tone="error">{traceState.message}</Status> : null}
      {traceState.status === 'success' ? <div className="wk-kdd-10">
        {/* R474/A3: Vue drawer head low-cost affordances — the LIVE badge
            (kp-live-badge: parse polling or any running/pending span) and the
            当前阶段 n/5 counter (currentStageIndex: first running/failed
            stage, else traversed done/skipped + 1, capped). Attempt tabs and
            the stop-parsing control remain interactive follow-ups. */}
        <div className="wk-kdd-11">
          {isKnowledgeProcessingActive(traceState.parseStatus) || traceState.nodes.some((row) => { const state = knowledgeTraceNodeState(row); return state === 'running' || state === 'pending'; }) ? <span className="wk-trace-live wk-kdd-12" title={TRACE_HEAD_COPY[locale].liveTooltip}><span className="wk-kdd-13" aria-hidden="true" />LIVE</span> : null}
          {traceState.steps.length ? (() => { const runningIdx = traceState.steps.findIndex((step) => step.state === 'running' || step.state === 'failed'); const traversed = traceState.steps.filter((step) => step.state === 'done' || step.state === 'skipped').length; const current = runningIdx >= 0 ? runningIdx + 1 : Math.min(traversed + 1, traceState.steps.length); return <span className="wk-kdd-14">{TRACE_HEAD_COPY[locale].stagesProgress} <strong className="wk-kdd-15">{current}/{traceState.steps.length}</strong></span>; })() : null}
        </div>
        <div className="wk-kdd-11">
          <Button type="button" loading={traceAction === 'loading'} onClick={() => setTraceRefresh((value) => value + 1)}>{t('common.refresh')}</Button>
          {canMutateDocument && traceState.parseStatus === 'failed' ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('reparse')}>{t('knowledgeBase.rebuildDocument')}</Button> : null}
          {canMutateDocument && isKnowledgeProcessingActive(traceState.parseStatus) ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('cancel')}>{t('knowledgeBase.documents.cancelParse')}</Button> : null}
        </div>
        {traceState.parseStatus === 'failed' ? <Status tone="error">{traceState.lastError?.error_message || t('knowledgeBase.timeline.failed')}</Status> : null}
        <ol className="wk-kdd-16" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.steps.map((step) => <li key={step.stage} data-state={step.state} className="wk-kdd-17"><span>{t(`knowledgeBase.timeline.stage.${step.stage}`)}</span><span>{t(`knowledgeBase.timeline.${step.state}`)}</span></li>)}
        </ol>
        {traceState.nodes.length > 0 ? <div className="wk-kdd-18"><ol className="wk-kdd-19 wk-kdd-trace-ol" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.nodes.filter((row) => row.depth === 0 || expandedTraceNodes.has(row.key.slice(0, row.key.lastIndexOf('.')))).map((row) => { const nodeState = knowledgeTraceNodeState(row); return <li key={row.key} data-state={nodeState} className="wk-kdd-20" style={{ paddingLeft: `${12 + row.depth * 16}px` }}>
            {row.hasChildren ? <button type="button" className="wk-kdd-21" aria-expanded={expandedTraceNodes.has(row.key)} aria-label={t('knowledgeBase.timeline.title')} onClick={() => setExpandedTraceNodes((current) => { const next = new Set(current); if (next.has(row.key)) next.delete(row.key); else next.add(row.key); return next; })}>{expandedTraceNodes.has(row.key) ? '⌄' : '›'}</button> : <span className="wk-kdd-22" aria-hidden="true" />}
            <button type="button" className="wk-kdd-23" onClick={() => setSelectedTraceNode(row)}>{row.node.name || row.node.stage || row.key}</button>{nodeState === 'pending' ? null : <span className={nodeState === 'failed' ? 'wk-kdd-96' : nodeState === 'done' ? 'wk-kdd-97' : nodeState === 'running' ? 'wk-kdd-98' : 'wk-kdd-46'}>{t(`knowledgeBase.timeline.${nodeState}`)}</span>}<span className="wk-kdd-24">{nodeState === 'pending' || typeof row.node.duration_ms !== 'number' ? '—' : `${row.node.duration_ms}ms`}</span>
          </li>; })}
        </ol></div> : null}
        {selectedTraceNode ? <section className="wk-kdd-25"><div className="wk-kdd-26"><strong className="wk-kdd-27">{selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key}</strong><Button type="button" onClick={() => setSelectedTraceNode(null)}>{t('knowledgeBase.documents.cancel')}</Button></div><pre className="wk-kdd-28">{JSON.stringify(selectedTraceNode.node, null, 2)}</pre></section> : null}
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
  return <section className="wk-document-chunks wk-kdd-29" aria-label={t('knowledgeBase.viewChunks')} hidden={view === 'preview'}>
    <div className="wk-kdd-30"><h3 className="wk-kdd-31">{t('knowledgeBase.viewChunks')} {state.total ? `(${state.total})` : ''}</h3></div>
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
        ? <DocumentMarkdownBody markdown={mergedContent} labels={MERMAID_VIEWER_COPY[locale]} className="wk-document-merged markdown-content wk-kdd-32" />
        : <div className="wk-document-merged wk-kdd-33">—</div>
      : null}
    {state.status === 'success' && view !== 'merged' ? <><div className="wk-kdd-34">{state.chunks.map((chunk, index) => <article key={chunk.id} className="wk-kdd-35" data-chunk-id={chunk.id}>
      <div className="wk-kdd-26"><strong className="wk-kdd-36">{t('knowledgeBase.segment')} {(state.page - 1) * 25 + index + 1}</strong>{parentChunkId(chunk) || generatedQuestions(chunk).length > 0 || canEdit ? <span className="wk-kdd-37">{parentChunkId(chunk) ? <Button type="button" variant="text" className="wk-parent-context-toggle" title={t('knowledgeBase.viewParentContext')} aria-label={t('knowledgeBase.viewParentContext')} aria-expanded={parentContextId === chunk.id} onClick={() => openParentContext(chunk)}><GitBranchIcon /></Button> : null}{generatedQuestions(chunk).length > 0 || canEdit ? <Button type="button" variant="text" className="wk-chunk-questions-toggle" title={t('knowledgeBase.generatedQuestions')} aria-label={t('knowledgeBase.generatedQuestions')} aria-expanded={questionsId === chunk.id} onClick={() => openQuestions(chunk)}><HelpCircleIcon /></Button> : null}{canEdit ? <><Button type="button" onClick={() => { setParentContextId(null); closeQuestions(); setEditingId(chunk.id); setDraft(chunk.content || ''); }}>{t('common.edit')}</Button><Button type="button" loading={historyLoading === chunk.id} onClick={() => showHistory(chunk)}>{t('knowledgeBase.chunkHistory')}</Button><Button type="button" loading={savingId === chunk.id} onClick={() => void toggleEnabled(chunk)}>{chunk.is_enabled ? t('knowledgeBase.disableChunk') : t('knowledgeBase.enableChunk')}</Button>{chunk.index_status === 'failed' ? <Button type="button" title={t('knowledgeBase.retryIndex')} aria-label={t('knowledgeBase.retryIndex')} loading={retryingId === chunk.id} onClick={() => void retryIndex(chunk)}>{t('knowledgeBase.retryIndex')}</Button> : null}</> : null}</span> : null}</div>
      {editingId === chunk.id ? <><textarea aria-label={t('knowledgeBase.segment')} value={draft} onChange={(event) => setDraft(event.target.value)} className="wk-kdd-38" /><div className="wk-kdd-39"><Button type="button" loading={savingId === chunk.id} onClick={() => void save(chunk)}>{t('common.save')}</Button><Button type="button" onClick={() => { setEditingId(null); setDraft(''); }}>{t('common.cancel')}</Button></div></> : <DocumentMarkdownBody markdown={chunk.content || '—'} labels={MERMAID_VIEWER_COPY[locale]} className="wk-document-chunk-content markdown-content wk-kdd-40" />}
      {history?.id === chunk.id ? <div className="wk-kdd-41"><strong className="wk-kdd-36">{t('knowledgeBase.chunkHistory')}</strong>{history?.rows.length === 0 ? <Status>{t('common.noData')}</Status> : <ol className="wk-kdd-42">{history?.rows.map((row) => <li key={row.revision} className="wk-kdd-43"><span>Revision {row.revision}: {row.content || '—'}</span><Button type="button" className="wk-kdd-44" onClick={() => void (async () => { const updated = await client.knowledgeBases.documents.revertChunk(document.id, chunk.id, row.revision, chunk.content_revision ?? 0); setState((current) => ({ ...current, chunks: current.chunks.map((item) => item.id === chunk.id ? updated : item) })); showHistory(updated); })()}>{t('knowledgeBase.chunkReverted')}</Button></li>)}</ol>}</div> : null}
      {parentContextId === chunk.id && parentChunkId(chunk) ? <div className="wk-chunk-parent-context wk-kdd-41" aria-label={t('knowledgeBase.viewParentContext')}>
        <div className="wk-kdd-45"><span className="wk-kdd-46"><GitBranchIcon /></span>{t('knowledgeBase.viewParentContext')}</div>
        {parentContextLoading === chunk.id
          ? <div className="chunk-popup-state" role="status"><Status>{t('common.loading')}</Status></div>
          : <DocumentMarkdownBody markdown={parentContextCache.get(parentChunkId(chunk)!) || ''} labels={MERMAID_VIEWER_COPY[locale]} className="wk-chunk-parent-context-body markdown-content wk-kdd-47" />}
      </div> : null}
      {questionsId === chunk.id ? (() => { const rows = generatedQuestions(chunk); return <div className="wk-chunk-questions wk-kdd-41" aria-label={t('knowledgeBase.generatedQuestions')}>
        <div className="wk-kdd-48">
          <div className="wk-kdd-49"><span className="wk-kdd-46"><HelpCircleIcon /></span>{t('knowledgeBase.generatedQuestions')}<span className="wk-kdd-50">{rows.length}</span>{hasStaleGeneratedQuestions(chunk) ? <span className="wk-kdd-50">{t('knowledgeBase.staleGeneratedQuestions')}</span> : null}</div>
          {canEdit ? <span className="wk-kdd-37">
            <Button type="button" variant="text" title={t('knowledgeBase.addGeneratedQuestion')} aria-label={t('knowledgeBase.addGeneratedQuestion')} onClick={() => { setQuestionComposerId(chunk.id); setQuestionDraft(''); setEditingQuestion(null); setConfirmingDelete(null); }}>＋</Button>
            <Button type="button" variant="text" title={t('knowledgeBase.regenerateQuestions')} aria-label={t('knowledgeBase.regenerateQuestions')} loading={regeneratingQuestions === chunk.id} onClick={() => void regenerateQuestions(chunk)}>↻</Button>
          </span> : null}
        </div>
        {canEdit && questionComposerId === chunk.id ? <div className="question-composer wk-kdd-51">
          <input value={questionDraft} placeholder={t('knowledgeBase.addGeneratedQuestion')} aria-label={t('knowledgeBase.addGeneratedQuestion')} className="wk-kdd-52" onChange={(event) => setQuestionDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void addQuestion(chunk); }} />
          <Button type="button" variant="text" title={t('common.cancel')} aria-label={t('common.cancel')} disabled={savingQuestionComposer === chunk.id} onClick={() => { setQuestionComposerId(null); setQuestionDraft(''); }}>×</Button>
          <Button type="button" loading={savingQuestionComposer === chunk.id} disabled={!questionDraft.trim()} onClick={() => void addQuestion(chunk)}>{t('common.add')}</Button>
        </div> : null}
        {rows.length ? <ul className="questions-list wk-kdd-16">
          {rows.map((question) => <li key={question.id} className="question-item wk-kdd-53">
            <span className="wk-kdd-54"><HelpCircleIcon /></span>
            {editingQuestion?.chunkId === chunk.id && editingQuestion.questionId === question.id ? <span className="wk-kdd-55">
              <input value={questionEditDraft} aria-label={t('common.edit')} className="wk-kdd-52" onChange={(event) => setQuestionEditDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void saveQuestionEdit(chunk, question); }} />
              <Button type="button" variant="text" onClick={() => { setEditingQuestion(null); setQuestionEditDraft(''); }}>{t('common.cancel')}</Button>
              <Button type="button" loading={savingQuestionKey === `${chunk.id}:${question.id}`} onClick={() => void saveQuestionEdit(chunk, question)}>{t('common.save')}</Button>
            </span> : confirmingDelete?.chunkId === chunk.id && confirmingDelete.questionId === question.id ? <span className="wk-kdd-56">
              <span className="wk-kdd-57">{t('knowledgeBase.confirmDeleteQuestion')}</span>
              <Button type="button" variant="text" onClick={() => setConfirmingDelete(null)}>{t('common.cancel')}</Button>
              <Button type="button" loading={deletingQuestion?.chunkId === chunk.id && deletingQuestion.questionId === question.id} onClick={() => void deleteQuestion(chunk, question)}>{t('common.confirmDelete')}</Button>
            </span> : <>
              <span className="question-text wk-kdd-58">{question.question}</span>
              {canEdit && !question.id.startsWith('legacy-') ? <span className="question-actions wk-kdd-59">
                <Button type="button" variant="text" title={t('common.edit')} aria-label={t('common.edit')} onClick={() => { setEditingQuestion({ chunkId: chunk.id, questionId: question.id }); setQuestionEditDraft(question.question); setConfirmingDelete(null); }}>{t('common.edit')}</Button>
                <Button type="button" variant="text" title={t('common.delete')} aria-label={t('common.delete')} onClick={() => { setConfirmingDelete({ chunkId: chunk.id, questionId: question.id }); setEditingQuestion(null); }}>{t('common.delete')}</Button>
              </span> : null}
            </>}
          </li>)}
        </ul> : questionComposerId !== chunk.id ? <div className="questions-empty wk-kdd-60">{t('knowledgeBase.noGeneratedQuestions')}</div> : null}
      </div>; })() : null}
    </article>)}</div></> : null}
    {/* Vue renders the chunk pagination for both merged and chunks views
        (viewMode merged || chunks), so 全文 can advance past page one, and
        keeps it mounted across a page transition with the requested page
        shown (v-model chunkPage) while in-flight clicks are ignored. */}
    {(state.status === 'success' || pageTransition) && view !== 'preview' && state.total > 25 ? <nav className="wk-kdd-61" aria-label={t('knowledgeBase.viewChunks')}><Button type="button" disabled={state.page <= 1 || pageTransition} onClick={() => load(state.page - 1)}>{t('common.back')}</Button><span>{state.pendingPage ?? state.page}</span><Button type="button" disabled={state.page * 25 >= state.total || pageTransition} onClick={() => load(state.page + 1)}>{t('common.next')}</Button></nav> : null}
  </section>;
}

type PreviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'text'; text: string }
  | { status: 'blob'; url: string }
  | { status: 'spreadsheet'; spreadsheet: SpreadsheetPreviewModel }
  | { status: 'error'; message: string };

function DocumentDetail({ document, client, canEdit, contentView, onContentViewChange, parentContextCache }: { document: KnowledgeDocument; client: WeKnoraClient; canEdit: boolean; contentView: ContentView; onContentViewChange: (view: ContentView) => void; parentContextCache: Map<string, string> }) {
  const model = buildDocumentPreview(document, client.knowledgeBases.documents.previewPath(document.id));
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const copy = DETAIL_COPY[locale];
  const [previewState, setPreviewState] = useState<PreviewState>({ status: 'idle' });
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [summaryEditing, setSummaryEditing] = useState(false);
  const [summaryDraft, setSummaryDraft] = useState(String(document.description || ''));
  const [metadataEditing, setMetadataEditing] = useState(false);
  const [metadataDraft, setMetadataDraft] = useState<MetadataDraftRow[]>(() => metadataRowsFromObject(document.custom_metadata as Record<string, unknown> | undefined));
  const [detailsSaving, setDetailsSaving] = useState(false);
  const [detailsError, setDetailsError] = useState<string | null>(null);
  // Vue document-preview.vue previewRoot: the inline (non-fullscreen) toolbar
  // hosts the fullscreen toggle above the preview surface.
  const previewRef = useRef<HTMLDivElement>(null);
  const togglePreviewFullscreen = () => {
    const el = previewRef.current;
    if (!el) return;
    try {
      const doc = window.document as Document & { fullscreenElement?: Element | null; exitFullscreen?: () => Promise<void> };
      if (doc.fullscreenElement === el) void doc.exitFullscreen?.();
      else void el.requestFullscreen?.();
    } catch { /* embedded hosts may disallow the Fullscreen API (Vue:133-135) */ }
  };

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
  // Vue details.time = formatStringDate(new Date(data.updated_at)) (useKnowledgeBase.ts L177).
  const documentTime = (document as KnowledgeDocument & { time?: unknown }).time
    ?? (document as KnowledgeDocument & { updated_at?: unknown }).updated_at
    ?? (document as KnowledgeDocument & { created_at?: unknown }).created_at;
  // Vue getTimeLabel / getTypeLabel / getContentLabel (doc-content.vue L986-1040).
  const timeLabel = document.type === 'url' ? t('knowledgeBase.importTime') : document.type === 'manual' ? t('knowledgeBase.createTime') : t('knowledgeBase.uploadTime');
  const typeLabel = document.type === 'url' ? t('knowledgeBase.typeURL') : document.type === 'manual' ? t('knowledgeBase.typeManual') : (document.file_type ? String(document.file_type).toUpperCase() : t('knowledgeBase.typeFile'));
  const contentLabel = document.type === 'url' ? t('knowledgeBase.webContent') : document.type === 'manual' ? t('knowledgeBase.documentContent') : t('knowledgeBase.fileContent');
  const customMetadata = (document.custom_metadata as Record<string, unknown> | undefined) ?? {};
  const hasCustomMetadata = Object.keys(customMetadata).length > 0;
  const sectionTitleClass = 'm-0 mb-1 flex items-center gap-2 text-[13px] font-semibold text-ink before:h-[14px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:bg-primary before:content-[""]';
  const tabs = CONTENT_TABS[locale];
  return <>
    {detailsError ? <Status tone="error">{detailsError}</Status> : null}
    {/* ── 基本信息 (Vue setting-drawer__section + doc-detail-rows) ──
        Vue section: padding 12px 0 16px, flex-column gap 14px, first-child
        padding-top 0 (doc-content.vue:2284-2299); the measured Vue rhythm
        leaves ~19px under the dl before the section border. */}
    <section className="wk-document-metadata-section wk-kdd-62">
      <h4 className={sectionTitleClass}>{t('knowledgeBase.detailSectionMeta')}</h4>
      <dl className="wk-document-metadata wk-kdd-63">
        {documentTime ? <div className="doc-detail-row wk-kdd-64"><dt>{timeLabel}</dt><dd>{formatDetailTime(documentTime)}</dd></div> : null}
        <div className="doc-detail-row wk-kdd-64"><dt>{t('knowledgeBase.infoCard.type')}</dt><dd className="wk-kdd-65">{/* Vue doc-content.vue:1648 t-tag variant="light" — 浅灰底无边框，非描边盒；行盒 20px（t-tag 高）。 */}
<span className="doc-type-tag wk-kdd-66">{typeLabel}</span></dd></div>
        {document.channel && document.channel !== 'web' ? <div className="doc-detail-row wk-kdd-64"><dt>{t('knowledgeBase.infoCard.source')}</dt><dd>{String(document.channel)}</dd></div> : null}
        {rawTags.length > 0 ? <div className="doc-detail-row wk-kdd-64"><dt>{t('knowledgeBase.tagLabel')}</dt><dd className="wk-kdd-37">{rawTags.map((tag) => <span key={String(tag.id ?? tag.name)} className="doc-tag-chip wk-kdd-67">{tag.name}</span>)}</dd></div> : null}
      </dl>
    </section>
    {/* ── 自定义元数据 (Vue metadata-section) ── */}
    <section className="wk-kdd-68" aria-label={t('knowledgeBase.customMetadata')}>
      <div className="wk-kdd-69">
        <h4 className={sectionTitleClass}>{t('knowledgeBase.customMetadata')}<InfoOutlineIcon size={14} className="wk-kdd-70" /></h4>
        {canEdit && !metadataEditing ? <IconActionButton label={t('common.edit')} onClick={() => { setDetailsError(null); const rows = metadataRowsFromObject(document.custom_metadata as Record<string, unknown> | undefined); setMetadataDraft(rows.length ? rows : [metadataRow()]); setMetadataEditing(true); }}><EditIcon size={15} /></IconActionButton> : null}
      </div>
      {metadataEditing ? <MetadataEditor rows={metadataDraft} saving={detailsSaving} onChange={setMetadataDraft} onCancel={() => { setMetadataEditing(false); setDetailsError(null); }} onSave={(value) => void saveDetails({ custom_metadata: value })} />
      : hasCustomMetadata ? <div className="metadata-grid wk-kdd-71">{Object.entries(customMetadata).map(([key, value]) => <div key={key} className="metadata-item wk-kdd-72"><span className="metadata-item-key wk-kdd-73">{key}</span><span className="metadata-item-value wk-kdd-74">{formatMetadataValue(value)}</span></div>)}</div>
      : canEdit ? <button type="button" className="metadata-empty-action wk-kdd-75" onClick={() => { setDetailsError(null); setMetadataDraft([metadataRow()]); setMetadataEditing(true); }}><PlusIcon size={15} /><span>{t('knowledgeBase.addMetadataField')}</span></button>
      : <span className="wk-kdd-33">{t('knowledgeBase.noCustomMetadata')}</span>}
    </section>
    {/* ── 摘要 (Vue summary-section) ── */}
    <section className="wk-kdd-68" aria-label={t('knowledgeBase.documentSummary')}>
      <div className="wk-kdd-69">
        <h4 className={sectionTitleClass}>{t('knowledgeBase.documentSummary')}</h4>
        {canEdit && !summaryEditing ? <span className="summary-title-actions wk-kdd-76">
          <IconActionButton label={t('common.edit')} onClick={() => { setSummaryDraft(String(document.description || '')); setSummaryEditing(true); }}><EditIcon size={15} /></IconActionButton>
          <IconActionButton label={t('knowledgeBase.regenerateSummary')} onClick={() => {}}><RefreshIcon size={15} /></IconActionButton>
        </span> : null}
      </div>
      {summaryEditing ? <><textarea value={summaryDraft} onChange={(event) => setSummaryDraft(event.target.value)} className="wk-kdd-77" /><div className="wk-kdd-39"><Button type="button" loading={detailsSaving} onClick={() => void saveDetails({ description: summaryDraft })}>{t('common.save')}</Button><Button type="button" onClick={() => setSummaryEditing(false)}>{t('common.cancel')}</Button></div></>
      : summaryDraft ? <p className="wk-kdd-78">{summaryDraft}</p>
      : <div className="summary_loading wk-kdd-79">
          <FileUnknownIcon size={18} className="wk-kdd-80" />
          <span>{t('knowledgeBase.noDocumentSummary')}</span>
          {canEdit ? <button type="button" aria-label={t('knowledgeBase.generateSummary')} className="wk-kdd-81"><RefreshIcon size={14} /><span>{t('knowledgeBase.generateSummary')}</span></button> : null}
        </div>}
    </section>
    {/* ── 文件内容 (Vue doc-content-section: last section — gap 12px, no bottom border/padding) ── */}
    <section className="doc-content-section wk-kdd-82">
      <div className="doc-content-section-head wk-kdd-83">
        <div className="doc-content-section-head-left wk-kdd-84">
          <h4 className={sectionTitleClass}>{contentLabel}</h4>
        </div>
        <div className="view-mode-buttons wk-kdd-85">
          {canPreviewDocument(document) ? <button type="button" className={'view-mode-btn wk-kdd-99 ' + (contentView === 'preview' ? 'wk-kdd-100' : 'wk-kdd-101')} onClick={() => onContentViewChange('preview')}>{tabs.preview}</button> : null}
          {/* Vue doc-content.vue:1821-1830 — the 全文 tab renders ONLY when
              the document is not previewable (canPreview() false branch);
              previewable documents show 预览 + 查看分块 only. */}
          {!canPreviewDocument(document) ? <button type="button" className={'view-mode-btn wk-kdd-99 ' + (contentView === 'merged' ? 'wk-kdd-100' : 'wk-kdd-101')} onClick={() => onContentViewChange('merged')}>{tabs.merged}</button> : null}
          <button type="button" className={'view-mode-btn wk-kdd-99 ' + (contentView === 'chunks' ? 'wk-kdd-100' : 'wk-kdd-101')} onClick={() => onContentViewChange('chunks')}>{tabs.chunks}</button>
        </div>
      </div>
      {/* Vue pins an embedded audio player above the content views for audio
          files (audio-player-section), since canPreview() keeps them off the
          preview tab; the blob is already fetched by the preview effect. */}
      {inlineKind === 'audio' && previewState.status === 'blob' ? <div className="wk-document-audio-player wk-kdd-86"><DocumentPreviewContent kind="audio" url={previewState.url} fileName={model.fileName} /></div> : null}
      {contentView === 'preview' ? (previewState.status === 'loading' ? <Status>{copy.loading}</Status> : previewState.status === 'error' ? <><Status tone="error">{previewState.message}</Status><Button type="button" onClick={() => setPreviewAttempt((attempt) => attempt + 1)}>{copy.retry}</Button></> : previewState.status === 'text' && inlineKind ? (inlineKind === 'markdown'
        // Vue document-preview.vue:484 + :497-505 — the preview mounts inside
        // .document-preview (min-height 200px) with an inline toolbar row
        // (fullscreen toggle, right-aligned) above the .preview-markdown card
        // (radius 6, padding 20px 24px, max-height calc(100vh - 200px)).
        ? <div ref={previewRef} className="document-preview wk-kdd-87" aria-label={model.fileName}>
            {/* Vue preview-toolbar (inline): a bordered row ~38px tall with the
                fullscreen toggle right-aligned, 8px above the preview card. */}
            <div className="preview-toolbar-actions wk-kdd-88">
              <button type="button" className="wk-kdd-89" aria-label={copy.fullscreen} title={copy.fullscreen} onClick={togglePreviewFullscreen}><FullscreenIcon size={14} /></button>
            </div>
            <div className='wk-document-md-preview wk-kdd-mdwrap'><DocumentMarkdownBody markdown={previewState.text} labels={MERMAID_VIEWER_COPY[locale]} className='md-content wk-kdd-mdcontent' /></div>
          </div>
        : <DocumentPreviewContent kind={inlineKind} text={previewState.text} fileName={model.fileName} mermaidLabels={MERMAID_VIEWER_COPY[locale]} />) : previewState.status === 'spreadsheet' && inlineKind ? <DocumentPreviewContent kind={inlineKind} spreadsheet={previewState.spreadsheet} fileName={model.fileName} /> : previewState.status === 'blob' && inlineKind ? <DocumentPreviewContent kind={inlineKind} url={previewState.url} fileName={model.fileName} /> : null) : null}
    </section>
    <DocumentChunks client={client} document={document} canEdit={canEdit} view={contentView} parentContextCache={parentContextCache} />
  </>;
}

function formatMetadataValue(value: unknown): string {
  if (value === null) return 'null';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

/** Vue doc-content icon-action-btn: 28x28 square text button with tooltip copy
    (doc-content.vue:2176-2188); hover goes brand green on brand-light wash. */
function IconActionButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
  return <button type="button" aria-label={label} title={label} className="icon-action-btn wk-kdd-90" onClick={onClick}>{children}</button>;
}

/** Vue header doc icon (doc-drawer-header-icon): t-icon file 字形（本地 sprite
 * d，stroke 2 / 默认 miter / butt cap —— 自绘 path 在像素 diff 下整块红）。 */
function FileDetailIcon({ size = 16 }: { size?: number }) {
  return <Icon size={size} cap="butt"><path d="M14 2v6h6m-6-6h1l5 5v1m-6-6H4v20h16V8" /></Icon>;
}

function EditIcon({ size = 15 }: { size?: number }) {
  // t-icon edit（sprite）：单 path，square cap。
  return <Icon size={size} cap="square"><path d="m14.105 6.004-9.318 9.318L3.998 20l4.679-.79 9.317-9.317m-3.89-3.889 3.89 3.89m-3.89-3.89 3.058-3.057 3.889 3.89-3.057 3.056" /></Icon>;
}

function DownloadIcon({ size = 16 }: { size?: number }) {
  // t-icon download（sprite）
  return <Icon size={size} cap="square"><path d="M16.5 10.5 12 15l-4.5-4.5m4.5 3.25V4M20.5 15v5h-17v-5" /></Icon>;
}

function ChartLineIcon({ size = 16 }: { size?: number }) {
  // t-icon chart-line（sprite）
  return <Icon size={size} cap="square"><path d="M21 21H3V3" /><path d="m20.5 8-5 5-4-4-5 5" /></Icon>;
}

function RefreshIcon({ size = 15 }: { size?: number }) {
  // t-icon refresh（sprite）
  return <Icon size={size} cap="square"><path d="M21.448 13c-.5 4.777-4.539 8.5-9.448 8.5A9.501 9.501 0 0 1 3.38 16m-.88 4.5v-5h3M2.552 11C3.052 6.223 7.09 2.5 12 2.5A9.501 9.501 0 0 1 20.62 8m.88-4.5v5h-3" /></Icon>;
}

function InfoOutlineIcon({ size = 14, className }: { size?: number; className?: string }) {
  // t-icon info-circle（sprite）
  return <Icon size={size} cap="square" className={className}><path d="M2 12C2 6.477 6.477 2 12 2s10 4.477 10 10-4.477 10-10 10S2 17.523 2 12Z" /><path d="M12 16.5V11m0-3.5h-.004v-.004H12V7.5Z" /></Icon>;
}

function PlusIcon({ size = 15 }: { size?: number }) {
  // t-icon add（sprite）
  return <Icon size={size} cap="square"><path d="M12 5v14m7-7H5" /></Icon>;
}

function FileUnknownIcon({ size = 18, className }: { size?: number; className?: string }) {
  // t-icon file-unknown（sprite）
  return <Icon size={size} cap="square" className={className}><path d="M20 10.5V7l-5-5H4v20h9.5M14 2v6h6" /><path d="M15.5 16.249C15.5 15.007 16.62 14 18 14s2.5 1.007 2.5 2.249c0 .61-.27 1.164-.71 1.57L18 19.506v.115m-.001 3.374h.004V23h-.004v-.004Z" /></Icon>;
}

// Vue 全屏/收起 preview-toolbar 按钮（document-preview.vue:497-505 t-icon fullscreen）。
function FullscreenIcon({ size = 15 }: { size?: number }) {
  // t-icon fullscreen（sprite）
  return <Icon size={size} cap="square"><path d="M6.343 17.657 17.657 6.343M18.5 11V5.5H13M5.5 13v5.5H11" /></Icon>;
}

function Icon({ size = 16, stroke, cap, className, children }: { size?: number; stroke?: number; cap?: 'round' | 'square' | 'butt'; className?: string; children: React.ReactNode }) {
  // tdesign sprite 图标无 stroke-linejoin（默认 miter）；cap 逐图标传入。
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={stroke ?? 2} strokeLinecap={cap ?? 'round'} aria-hidden="true" focusable="false" className={className}>{children}</svg>;
}

/** Vue formatStringDate (frontend/src/utils/index.ts L55): local YYYY-MM-DD HH:mm:ss. */
function formatDetailTime(value: unknown): string {
  if (typeof value !== 'string' || !value) return String(value ?? '');
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (input: number) => String(input).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
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

function MetadataEditor({ rows, saving, onChange, onCancel, onSave }: {
  rows: MetadataDraftRow[]; saving: boolean;
  onChange: (rows: MetadataDraftRow[]) => void; onCancel: () => void; onSave: (value: Record<string, unknown>) => void;
}) {
  const t = createTranslator(useAppLocale());
  const [validationError, setValidationError] = useState<string | null>(null);
  const updateRow = (id: number, update: Partial<MetadataDraftRow>) => onChange(rows.map((row) => row.id === id ? { ...row, ...update } : row));
  const save = () => {
    const validation = validateMetadataRows(rows);
    if (!validation.ok) { setValidationError(validation.message); return; }
    setValidationError(null);
    onSave(validation.value);
  };
  return <div className="metadata-editor" aria-label={t('knowledgeBase.customMetadata')}>
    {validationError ? <Status tone="error">{validationError}</Status> : null}
    <div className="wk-kdd-91">{rows.map((row) => <div key={row.id} className="wk-kdd-92">
      <input aria-label={t('knowledgeBase.metadataKeyPlaceholder')} placeholder={t('knowledgeBase.metadataKeyPlaceholder')} value={row.key} onChange={(event) => updateRow(row.id, { key: event.target.value })} className="wk-kdd-93" />
      <select aria-label={t('knowledgeBase.metadataTypeText')} value={row.type} onChange={(event) => { const type = event.target.value as MetadataValueType; updateRow(row.id, { type, value: type === 'null' ? '' : type === 'boolean' ? 'false' : row.value }); }} className="wk-kdd-94">
        <option value="text">{t('knowledgeBase.metadataTypeText')}</option><option value="number">{t('knowledgeBase.metadataTypeNumber')}</option><option value="boolean">{t('knowledgeBase.metadataTypeBoolean')}</option><option value="null">{t('knowledgeBase.metadataTypeNull')}</option>
      </select>
      <input aria-label={t('knowledgeBase.metadataValuePlaceholder')} placeholder={t('knowledgeBase.metadataValuePlaceholder')} value={row.value} disabled={row.type === 'null'} onChange={(event) => updateRow(row.id, { value: event.target.value })} className="wk-kdd-93" />
      <Button type="button" variant="text" onClick={() => onChange(rows.filter((item) => item.id !== row.id))}>{t('common.delete')}</Button>
    </div>)}</div>
    <div className="wk-kdd-95"><Button type="button" disabled={rows.length >= 20} onClick={() => onChange([...rows, metadataRow()])}>{t('knowledgeBase.addMetadataField')}</Button><Button type="button" loading={saving} onClick={save}>{t('common.save')}</Button><Button type="button" onClick={onCancel}>{t('common.cancel')}</Button></div>
  </div>;
}
