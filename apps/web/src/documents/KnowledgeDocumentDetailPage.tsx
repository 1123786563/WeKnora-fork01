import { useEffect, useState } from 'react';
import type { KnowledgeDocument, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Sheet, Status } from '@weknora/ui';
import { buildDocumentPreview, DocumentPreviewContent, isInlinePreviewKind, previewBodyAsBlob, readPreviewText, type InlinePreviewKind } from './preview.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { buildKnowledgeTimeline, flattenKnowledgeSpans, isKnowledgeProcessingActive, type KnowledgeTimelineNode } from '@weknora/domain/knowledge/processing';
import { startProcessingTimeline, type ProcessingTimelineSubscription } from './processing-timeline.ts';
import type { KnowledgeTimelineStep } from '@weknora/domain/knowledge/processing';

interface KnowledgeDocumentDetailPageProps {
  client: WeKnoraClient;
  documentId: string;
  onBack: () => void;
}

const DETAIL_COPY: Record<Locale, { load: string; bytes: string; status: string; source: string; folder: string; type: string; root: string; unavailable: string; downloadOnly: string; loading: string; retry: string; download: string; downloadFailed: string }> = {
  'zh-CN': { load: '文档加载失败', bytes: '文档内容读取失败', status: '状态', source: '来源', folder: '文件夹', type: '类型', root: '根目录', unavailable: '处理完成后才能预览文档。当前状态以服务端为准。', downloadOnly: '此文件类型不支持内嵌预览，请下载原文件。', loading: '正在加载预览…', retry: '重试预览', download: '下载', downloadFailed: '下载失败。' },
  'en-US': { load: 'Unable to load document', bytes: 'Unable to load document bytes', status: 'Status', source: 'Source', folder: 'Folder', type: 'Type', root: 'Root', unavailable: 'Preview is unavailable until processing reaches completed. Current status is authoritative.', downloadOnly: 'Inline preview is unavailable for this file type. Download the original file instead.', loading: 'Loading preview…', retry: 'Retry preview', download: 'Download', downloadFailed: 'Download failed.' },
  'ja-JP': { load: 'ドキュメントを読み込めません', bytes: 'ドキュメント内容を読み込めません', status: '状態', source: 'ソース', folder: 'フォルダー', type: '種類', root: 'ルート', unavailable: '処理が完了するまでプレビューできません。現在の状態はサーバーを正とします。', downloadOnly: 'このファイル形式はインラインプレビューに対応していません。元のファイルをダウンロードしてください。', loading: 'プレビューを読み込み中…', retry: 'プレビューを再試行', download: 'ダウンロード', downloadFailed: 'ダウンロードに失敗しました。' },
  'ko-KR': { load: '문서를 불러오지 못했습니다', bytes: '문서 내용을 불러오지 못했습니다', status: '상태', source: '소스', folder: '폴더', type: '유형', root: '루트', unavailable: '처리가 완료될 때까지 미리보기를 사용할 수 없습니다. 현재 상태는 서버 기준입니다.', downloadOnly: '이 파일 형식은 인라인 미리보기를 지원하지 않습니다. 원본 파일을 다운로드하세요.', loading: '미리보기를 불러오는 중…', retry: '미리보기 다시 시도', download: '다운로드', downloadFailed: '다운로드하지 못했습니다.' },
  'ru-RU': { load: 'Не удалось загрузить документ', bytes: 'Не удалось загрузить содержимое документа', status: 'Статус', source: 'Источник', folder: 'Папка', type: 'Тип', root: 'Корень', unavailable: 'Предпросмотр станет доступен после завершения обработки. Текущий статус определяется сервером.', downloadOnly: 'Для этого типа файла нет встроенного предпросмотра. Скачайте исходный файл.', loading: 'Загрузка предпросмотра…', retry: 'Повторить предпросмотр', download: 'Скачать', downloadFailed: 'Не удалось скачать файл.' },
};

export function KnowledgeDocumentDetailPage({ client, documentId, onBack }: KnowledgeDocumentDetailPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const copy = DETAIL_COPY[locale];
  const [state, setState] = useState<{ status: 'loading' } | { status: 'success'; document: KnowledgeDocument } | { status: 'error'; message: string }>({ status: 'loading' });
  const [timelineSteps, setTimelineSteps] = useState<KnowledgeTimelineStep[]>([]);
  const [traceOpen, setTraceOpen] = useState(false);
  const [traceState, setTraceState] = useState<{ status: 'idle' | 'loading' | 'success' | 'error'; steps: KnowledgeTimelineStep[]; nodes: KnowledgeTimelineNode[]; parseStatus?: string; lastError?: { error_code?: string; error_message?: string } | null; message?: string }>({ status: 'idle', steps: [], nodes: [] });
  const [traceRefresh, setTraceRefresh] = useState(0);
  const [traceAction, setTraceAction] = useState<'idle' | 'loading' | 'error'>('idle');
  const [expandedTraceNodes, setExpandedTraceNodes] = useState<Set<string>>(new Set());
  const [selectedTraceNode, setSelectedTraceNode] = useState<KnowledgeTimelineNode | null>(null);
  useEffect(() => {
    let active = true;
    setState({ status: 'loading' });
    void client.knowledgeBases.documents.get(documentId).then((document) => { if (active) setState({ status: 'success', document }); }).catch((error: unknown) => { if (active) setState({ status: 'error', message: error instanceof Error ? error.message : copy.load }); });
    return () => { active = false; };
  }, [client, documentId]);

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
    if (state.status !== 'success' || traceAction === 'loading') return;
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

  const detailTitle = state.status === 'success' ? state.document.file_name || state.document.title || documentId : documentId;
  return <main className="wk-page wk-document-detail-page max-w-[820px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div className="min-w-0"><div className="document-title-row flex min-h-8 items-center"><h2 className="document-breadcrumb m-0 flex min-w-0 items-center gap-2 text-[20px] font-semibold leading-8"><button type="button" className="breadcrumb-link inline-flex shrink-0 cursor-pointer items-center gap-1 rounded-[6px] border-none bg-transparent px-2 py-1 -mx-2 -my-1 text-[14px] font-normal leading-6 text-[var(--wk-muted,#66758b)] [font:inherit] hover:bg-[var(--wk-surface,#fff)] hover:text-[var(--wk-brand,#00a870)]" onClick={onBack}>← {t('knowledgeBase.detail.back')}</button><span className="breadcrumb-separator text-[14px] font-normal text-[var(--wk-muted,#98a2b8)]" aria-hidden="true">›</span><span className="breadcrumb-current min-w-0 truncate">{detailTitle}</span></h2></div></div><div className="flex shrink-0 items-center gap-2">{state.status === 'success' ? <Button type="button" onClick={() => setTraceOpen(true)}>{t('knowledgeBase.timeline.title')}</Button> : null}</div></header>
    {state.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
    {state.status === 'error' ? <Status tone="error">{state.message}</Status> : null}
    {state.status === 'success' && timelineSteps.length > 0 ? <Card><section aria-label={t('knowledgeBase.timeline.title')} className="wk-processing-timeline"><strong>{t('knowledgeBase.timeline.title')}</strong><ol>{timelineSteps.map((step) => <li key={step.stage} data-state={step.state}>{t('knowledgeBase.timeline.stage.' + step.stage)} — {t('knowledgeBase.timeline.' + step.state)}</li>)}</ol></section></Card> : null}
  {state.status === 'success' ? <DocumentDetail client={client} document={state.document} previewPath={client.knowledgeBases.documents.previewPath(documentId)} downloadPath={client.knowledgeBases.documents.downloadPath(documentId)} /> : null}
  {traceOpen ? <Sheet open title={t('knowledgeBase.timeline.title')} onClose={() => setTraceOpen(false)} side="right" width="820px" resizable minWidth={560} maxWidth={1400} storageKey="weknora-trace-drawer-width" className="min-w-0 border-l border-line-soft">
    <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === 'loading'}>
      {traceState.status === 'loading' ? <Status>{t('common.loading')}</Status> : null}
      {traceState.status === 'error' ? <Status tone="error">{traceState.message}</Status> : null}
      {traceState.status === 'success' ? <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" loading={traceAction === 'loading'} onClick={() => setTraceRefresh((value) => value + 1)}>{t('common.refresh')}</Button>
          {traceState.parseStatus === 'failed' ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('reparse')}>{t('knowledgeBase.rebuildDocument')}</Button> : null}
          {isKnowledgeProcessingActive(traceState.parseStatus) ? <Button type="button" loading={traceAction === 'loading'} onClick={() => void runTraceAction('cancel')}>{t('knowledgeBase.documents.cancelParse')}</Button> : null}
        </div>
        {traceState.parseStatus === 'failed' ? <Status tone="error">{traceState.lastError?.error_message || t('knowledgeBase.timeline.failed')}</Status> : null}
        <ol className="m-0 flex list-none flex-col gap-2 p-0" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.steps.map((step) => <li key={step.stage} data-state={step.state} className="flex items-center justify-between rounded-[6px] border border-line-soft px-3 py-2 text-[13px]"><span>{t(`knowledgeBase.timeline.stage.${step.stage}`)}</span><span>{t(`knowledgeBase.timeline.${step.state}`)}</span></li>)}
        </ol>
        {traceState.nodes.length > 0 ? <div className="overflow-x-auto rounded-[8px] border border-line-soft"><ol className="m-0 list-none divide-y divide-line-soft p-0" aria-label={t('knowledgeBase.timeline.title')}>
          {traceState.nodes.filter((row) => row.depth === 0 || expandedTraceNodes.has(row.key.slice(0, row.key.lastIndexOf('.')))).map((row) => <li key={row.key} className="flex min-w-[480px] items-center gap-2 px-3 py-2 text-[13px] hover:bg-surface-wash" style={{ paddingLeft: `${12 + row.depth * 16}px` }}>
            {row.hasChildren ? <button type="button" className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-control border-0 bg-transparent text-muted hover:bg-hover-wash" aria-expanded={expandedTraceNodes.has(row.key)} aria-label={t('knowledgeBase.timeline.title')} onClick={() => setExpandedTraceNodes((current) => { const next = new Set(current); if (next.has(row.key)) next.delete(row.key); else next.add(row.key); return next; })}>{expandedTraceNodes.has(row.key) ? '⌄' : '›'}</button> : <span className="inline-block h-6 w-6 shrink-0" aria-hidden="true" />}
            <button type="button" className="min-w-0 flex-1 truncate border-0 bg-transparent p-0 text-left font-mono text-ink hover:underline" onClick={() => setSelectedTraceNode(row)}>{row.node.name || row.node.stage || row.key}</button><span className="w-20 shrink-0 text-right font-mono text-[11px] text-muted">{typeof row.node.duration_ms === 'number' ? `${row.node.duration_ms}ms` : '—'}</span>
          </li>)}
        </ol></div> : null}
        {selectedTraceNode ? <section className="rounded-[8px] border border-line-soft bg-surface-wash p-3"><div className="mb-2 flex items-center justify-between gap-2"><strong className="truncate text-[13px]">{selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key}</strong><Button type="button" onClick={() => setSelectedTraceNode(null)}>{t('knowledgeBase.documents.cancel')}</Button></div><pre className="m-0 max-h-[240px] overflow-auto whitespace-pre-wrap break-words text-[11px] leading-[1.5] text-muted">{JSON.stringify(selectedTraceNode.node, null, 2)}</pre></section> : null}
      </div> : null}
    </section>
  </Sheet> : null}
  </main>;
}

type PreviewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'text'; text: string }
  | { status: 'blob'; url: string }
  | { status: 'error'; message: string };

function DocumentDetail({ document, client, previewPath, downloadPath }: { document: KnowledgeDocument; client: WeKnoraClient; previewPath: string; downloadPath: string }) {
  const model = buildDocumentPreview(document, previewPath);
  const copy = DETAIL_COPY[useAppLocale()];
  const [previewState, setPreviewState] = useState<PreviewState>({ status: 'idle' });
  const [downloadState, setDownloadState] = useState<'idle' | 'loading' | 'error'>('idle');
  const [previewAttempt, setPreviewAttempt] = useState(0);

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

  const inlineKind: InlinePreviewKind | undefined = isInlinePreviewKind(model.kind) ? model.kind : undefined;
  return <Card><dl className="wk-document-metadata grid grid-cols-[repeat(auto-fit,minmax(140px,1fr))] gap-[0.75rem] mb-[1.25rem] ml-0 mr-0 mt-0 [&_dd]:mb-0 [&_dd]:ml-0 [&_dd]:mr-0 [&_dd]:mt-[0.2rem] [&_dd]:[overflow-wrap:anywhere] [&_div]:bg-canvas [&_div]:p-[0.7rem] [&_div]:rounded-control [&_dt]:text-[0.78rem] [&_dt]:text-muted"><div><dt>{copy.status}</dt><dd>{String(document.parse_status || 'unknown')}</dd></div><div><dt>{copy.source}</dt><dd>{String(document.source || 'file')}</dd></div><div><dt>{copy.folder}</dt><dd>{String(document.folder_path || copy.root)}</dd></div><div><dt>{copy.type}</dt><dd>{String(document.file_type || model.kind)}</dd></div></dl>
    {!model.ready ? <Status tone="warning">{copy.unavailable}</Status> : model.downloadOnly ? <Status>{copy.downloadOnly}</Status> : previewState.status === 'loading' ? <Status>{copy.loading}</Status> : previewState.status === 'error' ? <><Status tone="error">{previewState.message}</Status><Button type="button" onClick={() => setPreviewAttempt((attempt) => attempt + 1)}>{copy.retry}</Button></> : previewState.status === 'text' && inlineKind ? <DocumentPreviewContent kind={inlineKind} text={previewState.text} fileName={model.fileName} /> : previewState.status === 'blob' && inlineKind ? <DocumentPreviewContent kind={inlineKind} url={previewState.url} fileName={model.fileName} /> : null}
    <p><Button type="button" loading={downloadState === 'loading'} onClick={() => { setDownloadState('idle'); void download(); }}>{copy.download} {model.fileName}</Button>{downloadState === 'error' ? <Status tone="error">{copy.downloadFailed}</Status> : null}</p>
  </Card>;
}
