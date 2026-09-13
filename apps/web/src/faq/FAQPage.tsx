import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { DragEvent, FocusEvent, FormEvent, ReactNode } from 'react';
import { createPortal } from 'react-dom';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, FAQImportProgress, KnowledgeBase, KnowledgeTag, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Status } from '@weknora/ui';
import { formatMessage, type Locale } from '@weknora/i18n';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from '../knowledge/permissions.ts';
import { normalizeFAQPayload, parseExcelFile, parseFAQImportText } from './import-export.ts';
import './faq.css';

// FAQ knowledge-base page, rebuilt against the Vue baseline
// frontend/src/views/knowledge/components/FAQEntryManager.vue:
//   breadcrumb 知识库 › {kbName} › 问答 with KB switcher, info card and settings gear;
//   subtitle + full-width rounded search + 全部标签 tag filter + icon buttons;
//   centered 暂无 FAQ 条目 empty state. All copy flows through the shared
//   packages/i18n catalog (knowledgeEditor.faq*/faqImport*/faqExport*, menu.*,
//   knowledgeBase.*) — no literals in this file.

type Translate = ReturnType<typeof createTranslator>;

// faqManager.import.* lives in the shared @weknora/i18n catalog
// (generated/faqImport.ts) — formatMessage resolves it per active locale.

/** Vue FAQEntryManager.loadEntries: hasMore = entries.length < total. */
export function faqHasMore(loaded: number, total: number): boolean {
  return Number.isFinite(total) && total > 0 && loaded < Math.floor(total);
}

/** Optimistic per-entry status flip — pure, so a failed update can roll back. */
export function setEntryStatus<T extends { id: number; is_enabled: boolean }>(entries: readonly T[], id: number, value: boolean): T[] {
  let changed = false;
  const next = entries.map((entry) => {
    if (entry.id !== id) return entry;
    changed = true;
    return { ...entry, is_enabled: value } as T;
  });
  return changed ? next : (entries as T[]);
}

/** Vue processFile format split (FAQEntryManager.vue:1905): JSON / Excel / CSV. */
export function importFormatFromName(name: string): 'json' | 'csv' | 'excel' {
  const lower = name.toLowerCase();
  if (lower.endsWith('.json')) return 'json';
  if (lower.endsWith('.xlsx') || lower.endsWith('.xls')) return 'excel';
  return 'csv';
}

export interface FAQImportTaskView { status: string; text: string; progress: number; processed: number; total: number }

/** Vue importProgressText (FAQEntryManager.vue:1321): error ‖ server message ‖ status copy. */
export function importProgressText(task: { status: string; message?: string; error?: string }, locale: Locale = 'zh-CN'): string {
  if (task.error) return task.error;
  if (task.message && task.message.trim()) return task.message.trim();
  const status = task.status === 'processing' ? 'running' : task.status === 'completed' ? 'success' : task.status;
  const key = status === 'running' ? 'faqManager.import.importing'
    : status === 'success' ? 'faqManager.import.importDone'
    : task.status === 'failed' ? 'faqManager.import.importFailed'
    : 'faqManager.import.waiting';
  return formatMessage(locale, key);
}

/** Normalise a raw progress payload into the strip view model. */
export function faqImportTaskView(task: { status: string; progress?: number; processed?: number; total?: number; message?: string; error?: string }): FAQImportTaskView {
  const status = task.status === 'processing' ? 'running' : task.status === 'completed' ? 'success' : task.status;
  return {
    status,
    text: importProgressText(task),
    progress: Math.min(100, Math.max(0, Math.round(task.progress ?? 0))),
    processed: task.processed ?? 0,
    total: task.total ?? 0,
  };
}

/** Vue FAQEntryManager list limits (similar questions and answers). */
export const FAQ_SIMILAR_CAP = 10;
export const FAQ_NEGATIVE_CAP = 10;
export const FAQ_ANSWER_CAP = 5;

/** Vue add handlers: trim, reject duplicates/empty values, and enforce cap. */
export function pushListItem(list: readonly string[], draft: string, cap: number): { list: string[]; added: boolean } {
  const value = draft.trim();
  if (!value || list.includes(value) || list.length >= cap) return { list: [...list], added: false };
  return { list: [...list, value], added: true };
}

/** Vue remove handlers: immutable indexed removal. */
export function removeListItem(list: readonly string[], index: number): string[] {
  if (index < 0 || index >= list.length) return [...list];
  return list.filter((_, itemIndex) => itemIndex !== index);
}

/** Vue editorRules order: standard question before answers. */
export function editorFormError(form: { question?: string; answers?: readonly string[] }): 'question' | 'answers' | null {
  if (!form.question?.trim()) return 'question';
  if (!form.answers?.some((answer) => answer.trim())) return 'answers';
  return null;
}

export function faqSaveResultKey(editing: boolean): 'knowledgeEditor.messages.createSuccess' | 'knowledgeEditor.messages.updateSuccess' {
  return editing ? 'knowledgeEditor.messages.updateSuccess' : 'knowledgeEditor.messages.createSuccess';
}

// --- B4: search test drawer (Vue FAQEntryManager.vue:734-853, 1329-1338, 2650-2695) -

export interface FAQSearchFormState { query: string; vectorThreshold: number; matchCount: number }
export type FAQSearchHit = FAQEntry & { score?: number; matched_question?: string; match_type?: string };

/** Vue t-slider bounds (FAQEntryManager.vue:760, 775). */
export const FAQ_SEARCH_VECTOR_THRESHOLD = { min: 0, max: 1, step: 0.1 } as const;
export const FAQ_SEARCH_MATCH_COUNT = { min: 1, max: 50, step: 1 } as const;

/** Vue searchForm reactive defaults (FAQEntryManager.vue:1334-1338). */
export function faqSearchDefaultForm(): FAQSearchFormState {
  return { query: '', vectorThreshold: 0.7, matchCount: 10 };
}

/** Vue handleSearch guard: a blank (untrimmed) query warns and skips the request (:2651-2654). */
export function faqSearchBlocked(form: FAQSearchFormState): boolean {
  return !form.query.trim();
}

/** Vue handleSearch request mapping (FAQEntryManager.vue:2659-2663). */
export function faqSearchRequestFrom(form: FAQSearchFormState): { query_text: string; vector_threshold: number; match_count: number } {
  return { query_text: form.query.trim(), vector_threshold: form.vectorThreshold, match_count: form.matchCount };
}

/** Vue res.data spread + score sort desc (FAQEntryManager.vue:2664-2673). */
export function faqSearchResultsFromResponse(raw: unknown): FAQSearchHit[] {
  const data = (raw as { data?: unknown } | null | undefined)?.data;
  if (!Array.isArray(data)) return [];
  return (data as FAQSearchHit[])
    .filter((hit) => typeof hit === 'object' && hit !== null && typeof hit.id === 'number')
    .map((hit) => ({ ...hit, score: typeof hit.score === 'number' ? hit.score : 0 }))
    .sort((a, b) => (b.score || 0) - (a.score || 0));
}

/** Immutable flip for one expanded hit (Vue result.expanded toggle, :2693-2695). */
export function toggleSearchResultId(ids: ReadonlySet<number>, id: number): Set<number> {
  const next = new Set(ids);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

// --- B5: FAQTagTooltip (Vue frontend/src/components/FAQTagTooltip.vue) --------------

export type FaqTooltipPlacement = 'top' | 'bottom' | 'left' | 'right';
export interface FaqTooltipRect { top: number; left: number; width: number; height: number; bottom: number; right: number }

/** Vue updatePosition (FAQTagTooltip.vue:63-101): 8px gap, 8px viewport clamp,
 *  and the top→bottom flip when the space above is under the padding. Returns
 *  the resolved placement so the arrow class matches the rendered side. */
export function faqTooltipPosition(
  placement: FaqTooltipPlacement,
  rect: FaqTooltipRect,
  tip: FaqTooltipRect,
  viewport: { width: number; height: number },
): { top: number; left: number; placement: FaqTooltipPlacement } {
  let top = 0;
  let left = 0;
  switch (placement) {
    case 'top': top = rect.top - tip.height - 8; left = rect.left + (rect.width / 2) - (tip.width / 2); break;
    case 'bottom': top = rect.bottom + 8; left = rect.left + (rect.width / 2) - (tip.width / 2); break;
    case 'left': top = rect.top + (rect.height / 2) - (tip.height / 2); left = rect.left - tip.width - 8; break;
    case 'right': top = rect.top + (rect.height / 2) - (tip.height / 2); left = rect.right + 8; break;
  }
  const padding = 8;
  if (left < padding) left = padding;
  if (left + tip.width > viewport.width - padding) left = viewport.width - tip.width - padding;
  let resolved = placement;
  if (top < padding) {
    // 上方空间不足 → 改为下方显示 (Vue :91-98).
    if (placement === 'top') { top = rect.bottom + 8; resolved = 'bottom'; }
    else top = padding;
  }
  if (top + tip.height > viewport.height - padding) top = viewport.height - tip.height - padding;
  return { top, left, placement: resolved };
}

export interface FaqTagTooltipProps {
  content: string;
  type?: 'answer' | 'similar' | 'negative';
  placement?: FaqTooltipPlacement;
  children?: ReactNode;
}

/** Vue FAQTagTooltip.vue — wrapper + body-teleported fixed bubble. Trigger is
 *  hover (Vue :109-118) plus click for pointer/touch per the parity task;
 *  position follows updatePosition with the same scroll/resize listeners. */
export function FaqTagTooltip({ content, type = 'answer', placement = 'top', children }: FaqTagTooltipProps) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<{ top: number; left: number; placement: FaqTooltipPlacement } | null>(null);
  const wrapperRef = useRef<HTMLSpanElement | null>(null);
  const bubbleRef = useRef<HTMLDivElement | null>(null);
  const measure = useCallback(() => {
    const wrapper = wrapperRef.current;
    const bubble = bubbleRef.current;
    if (!wrapper || !bubble) return;
    const box = wrapper.getBoundingClientRect();
    const tipBox = bubble.getBoundingClientRect();
    setPosition(faqTooltipPosition(
      placement,
      { top: box.top, left: box.left, width: box.width, height: box.height, bottom: box.bottom, right: box.right },
      { top: tipBox.top, left: tipBox.left, width: tipBox.width, height: tipBox.height, bottom: tipBox.bottom, right: tipBox.right },
      { width: window.innerWidth, height: window.innerHeight },
    ));
  }, [placement]);
  useLayoutEffect(() => {
    if (!open) return;
    measure();
    // Vue re-positions while mounted (:120-128).
    window.addEventListener('scroll', measure, true);
    window.addEventListener('resize', measure);
    return () => {
      window.removeEventListener('scroll', measure, true);
      window.removeEventListener('resize', measure);
    };
  }, [open, measure]);
  const resolvedPlacement = position?.placement ?? placement;
  return (
    <span
      ref={wrapperRef}
      className="faq-tag-wrapper"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onClick={(event) => { event.stopPropagation(); setOpen((current) => !current); }}
    >
      {children}
      {open ? createPortal(
        <div
          ref={bubbleRef}
          className={`faq-tag-tooltip tooltip-${type} placement-${resolvedPlacement}`}
          style={{ top: (position?.top ?? 0) + 'px', left: (position?.left ?? 0) + 'px' }}
          role="tooltip"
        >
          <span className="tooltip-content">{content}</span>
        </div>,
        document.body,
      ) : null}
    </span>
  );
}

// --- B6: persisted import result (Vue FAQEntryManager.vue last-result) --------------

export interface FAQImportResultView {
  total_entries: number;
  success_count: number;
  failed_count: number;
  skipped_count: number;
  partial_failed_count: number;
  merged_count: number;
  added_count: number;
  message: string;
  import_mode: string;
  imported_at?: string;
  task_id: string;
  failed_entries_url?: string;
  display_status: 'open' | 'close';
}

/** Vue getLastCompletedTaskKey (:2276-2278). */
export function faqLastCompletedTaskKey(kbId: string): string {
  return `faq_import_last_completed_${kbId}`;
}

/** Vue saveLastCompletedTaskId (:2280-2287) — best-effort localStorage write. */
export function saveLastCompletedTaskId(kbId: string, taskId: string): void {
  if (!kbId) return;
  try { window.localStorage.setItem(faqLastCompletedTaskKey(kbId), taskId); } catch { /* Vue logs; storage stays best-effort */ }
}

/** Vue getLastCompletedTaskId (:2289-2296). */
export function getLastCompletedTaskId(kbId: string): string | null {
  if (!kbId) return null;
  try { return window.localStorage.getItem(faqLastCompletedTaskKey(kbId)); } catch { return null; }
}

/** Vue loadImportResult mapping (:2308-2341): only completed, non-closed
 *  results surface; missing counts default to 0 and mode to append. */
export function faqImportResultFromProgress(data: unknown): FAQImportResultView | null {
  if (typeof data !== 'object' || data === null) return null;
  const row = data as Record<string, unknown>;
  if (row.status !== 'completed') return null;
  if (row.display_status === 'close') return null;
  const num = (value: unknown): number => (typeof value === 'number' && Number.isFinite(value) ? value : 0);
  const str = (value: unknown): string => (typeof value === 'string' ? value : '');
  return {
    total_entries: num(row.total),
    success_count: num(row.success_count),
    failed_count: num(row.failed_count),
    skipped_count: num(row.skipped_count),
    partial_failed_count: num(row.partial_failed_count),
    merged_count: num(row.merged_count),
    added_count: num(row.added_count),
    message: str(row.message),
    import_mode: str(row.import_mode) || 'append',
    imported_at: str(row.imported_at) || undefined,
    task_id: str(row.task_id),
    failed_entries_url: str(row.failed_entries_url) || undefined,
    display_status: 'open',
  };
}

/** Vue importResultSummary (:1277-1303): backend message wins, otherwise the
 *  count parts joined with ' · ' (merged branch implies the added sub-part). */
export function faqImportResultSummary(result: FAQImportResultView, t: Translate): string {
  if (result.message.trim()) return result.message.trim();
  const parts: string[] = [];
  parts.push(`${t('FAQ.import.totalData')} ${result.total_entries}`);
  if (result.merged_count > 0) {
    if (result.added_count > 0) parts.push(`${t('FAQ.import.added')} ${result.added_count}`);
    parts.push(`${t('FAQ.import.merged')} ${result.merged_count}`);
  } else if (result.success_count > 0) {
    parts.push(`${t('FAQ.import.success')} ${result.success_count}`);
  }
  if (result.partial_failed_count > 0) parts.push(`${t('FAQ.import.partialFailed')} ${result.partial_failed_count}`);
  if (result.failed_count > 0) parts.push(`${t('FAQ.import.failed')} ${result.failed_count}`);
  if (result.skipped_count > 0) parts.push(`${t('FAQ.import.skipped')} ${result.skipped_count}`);
  return parts.join(' · ');
}

/** Vue showImportResultBadge (:1266-1270): open result and no import task. */
export function faqImportResultVisible(result: FAQImportResultView | null | undefined, hasActiveTask: boolean): boolean {
  return !!result && result.display_status === 'open' && !hasActiveTask;
}

/** Vue formatImportTime (:2369-2377) as local YYYY-MM-DD HH:mm. */
export function formatImportTime(timeStr?: string): string {
  if (!timeStr) return '';
  const date = new Date(timeStr);
  if (Number.isNaN(date.getTime())) return timeStr;
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// Layered fallback for keys the Vue locales carry but the shared catalog does not
// (common.operationFailed drives the Vue search-error toast, :2675; common.close
// labels the drawer close button) — formatMessage wins once a key lands upstream,
// mirroring packages/views/src/integrations/messages.ts.
const FAQ_FALLBACK_MESSAGES: Partial<Record<Locale, Record<string, string>>> = {
  'zh-CN': { 'common.operationFailed': '操作失败', 'common.close': '关闭' },
  'en-US': { 'common.operationFailed': 'Operation failed', 'common.close': 'Close' },
  'ja-JP': { 'common.operationFailed': '操作に失敗しました', 'common.close': '閉じる' },
  'ko-KR': { 'common.operationFailed': '작업 실패', 'common.close': '닫기' },
  'ru-RU': { 'common.operationFailed': 'Операция не выполнена', 'common.close': 'Закрыть' },
};

/** Translator with the byte-exact Vue fallbacks layered under the shared catalog. */
export function createFaqTranslator(locale: Locale): Translate {
  const base = createTranslator(locale);
  return (key: string, values?: Record<string, string | number>): string => {
    const raw = base(key, values);
    if (raw !== key) return raw;
    return FAQ_FALLBACK_MESSAGES[locale]?.[key] ?? raw;
  };
}

type FAQSectionCollapseState = Record<string, boolean>;
function sectionCollapseKey(entryId: number, section: string): string { return `${entryId}:${section}`; }

/** Vue FAQ cards default each details section to collapsed and toggle locally. */
export function isSectionCollapsed(state: FAQSectionCollapseState, entryId: number, section: string): boolean {
  return state[sectionCollapseKey(entryId, section)] ?? true;
}

export function toggleSection(state: FAQSectionCollapseState, entryId: number, section: string): FAQSectionCollapseState {
  const key = sectionCollapseKey(entryId, section);
  return { ...state, [key]: !isSectionCollapsed(state, entryId, section) };
}

export interface KBListItem { id: string; name: string; type?: string }
export interface FAQKBMeta { type?: string; description?: string; createdAt?: string }

/** Vue: handleNavigateToKbList → /platform/knowledge-bases. */
export const faqKBListPath = '/platform/knowledge-bases';
/** Vue: gear opens the KB editor; React's destination is the KB settings route. */
export function faqKBSettingsPath(knowledgeBaseId: string): string {
  return '/knowledgeBase/' + encodeURIComponent(knowledgeBaseId) + '/settings';
}
/** Vue: KBSwitcherDropdown select → KB detail route (FAQ KBs land on /faq). */
export function faqKBDetailPath(knowledgeBaseId: string): string {
  return '/knowledgeBase/' + encodeURIComponent(knowledgeBaseId);
}

function defaultNavigate(path: string): void { window.location.assign(path); }

/** Vue dropdowns close on outside click — mirror it via focusout. */
function closeOnBlur(event: FocusEvent<HTMLElement>, close: () => void): void {
  if (!event.currentTarget.contains(event.relatedTarget as Node | null)) close();
}

// --- Inline icons (no TDesign / icon font; feather-style strokes) -----------------

function Icon({ size = 16, className, children }: { size?: number; className?: string; children: ReactNode }) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">{children}</svg>
  );
}
const Chevrons = {
  right: 'M9 18l6-6-6-6',
  down: 'M6 9l6 6 6-6',
  up: 'M18 15l-6-6-6 6',
};
function SearchIcon(props: { size?: number; className?: string }) {
  return <Icon {...props}><circle cx="11" cy="11" r="8" /><path d="M21 21l-4.35-4.35" /></Icon>;
}
function GearIcon(props: { size?: number; className?: string }) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z" />
    </Icon>
  );
}
function AddIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M12 5v14M5 12h14" /></Icon>; }
function DownloadIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" /><path d="M7 10l5 5 5-5" /><path d="M12 15V3" /></Icon>; }
function InfoIcon(props: { size?: number; className?: string }) { return <Icon {...props}><circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" /></Icon>; }
function FileAddIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" /><path d="M14 2v6h6" /><path d="M12 18v-6M9 15h6" /></Icon>; }
function CloseIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M18 6L6 18M6 6l12 12" /></Icon>; }
function TagIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M20.59 13.41l-7.17 7.17a2 2 0 01-2.83 0L2 12V2h10l8.59 8.59a2 2 0 010 2.82z" /><path d="M7 7h.01" /></Icon>; }
function UploadIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" /><path d="M17 8l-5-5-5 5" /><path d="M12 3v12" /></Icon>; }
// Vue card-more-btn uses @/assets/img/more.png (horizontal ⋯) — inline feather-style dots.
function MoreIcon(props: { size?: number; className?: string }) {
  return <Icon {...props}><circle cx="5" cy="12" r="1.7" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1.7" fill="currentColor" stroke="none" /><circle cx="19" cy="12" r="1.7" fill="currentColor" stroke="none" /></Icon>;
}

// --- Breadcrumb (Vue faq-breadcrumb + kb-title-actions) ---------------------------

export interface FAQBreadcrumbProps {
  t?: Translate;
  knowledgeBaseId?: string;
  kbName?: string | null;
  kbList?: KBListItem[];
  kbMeta?: FAQKBMeta;
  onNavigate?: (path: string) => void;
}

export function FAQBreadcrumb({ t: tr, knowledgeBaseId = '', kbName, kbList = [], kbMeta, onNavigate = defaultNavigate }: FAQBreadcrumbProps = {}) {
  const t = tr ?? createTranslator('zh-CN');
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  return (
    <div className="faq-title-row">
      <h2 className="faq-breadcrumb">
        <button type="button" className="breadcrumb-link" onClick={() => onNavigate(faqKBListPath)}>{t('menu.knowledgeBase')}</button>
        <Icon size={14} className="breadcrumb-separator"><path d={Chevrons.right} /></Icon>
        <span className="faq-kb-switcher" onBlur={(event) => closeOnBlur(event, () => setSwitcherOpen(false))}>
          <button type="button" className="breadcrumb-link dropdown" aria-haspopup="menu" aria-expanded={switcherOpen} onClick={() => setSwitcherOpen((open) => !open)}>
            <span>{kbName ?? '…'}</span>
            <Icon size={14} className="breadcrumb-caret"><path d={Chevrons.down} /></Icon>
          </button>
          <span className="faq-menu faq-switcher-menu" role="menu" hidden={!switcherOpen}>
            {kbList.map((kb) => (
              <button key={kb.id} type="button" role="menuitem" className={'faq-menu-item' + (kb.id === knowledgeBaseId ? ' is-active' : '')} onClick={() => { setSwitcherOpen(false); onNavigate(faqKBDetailPath(kb.id)); }}>
                {kb.name}
              </button>
            ))}
          </span>
        </span>
        <Icon size={14} className="breadcrumb-separator"><path d={Chevrons.right} /></Icon>
        <span className="breadcrumb-current">{t('knowledgeEditor.faq.title')}</span>
      </h2>
      <div className="kb-title-actions">
        <span className="kb-info-host" onBlur={(event) => closeOnBlur(event, () => setInfoOpen(false))}>
          <button type="button" className="kb-info-button" aria-label={t('knowledgeBase.infoCard.tooltip')} title={t('knowledgeBase.infoCard.tooltip')} aria-expanded={infoOpen} onClick={() => setInfoOpen((open) => !open)}>
            <InfoIcon size={16} />
          </button>
          <span className="kb-info-card" hidden={!infoOpen}>
            <span className="kb-info-card-header">{t('knowledgeBase.infoCard.title')}</span>
            <span className="kb-info-card-row"><span className="kb-info-card-label">{t('knowledgeBase.infoCard.type')}</span><span className="kb-info-card-value">{kbMeta?.type?.toLowerCase() === 'faq' ? t('knowledgeEditor.basic.typeFAQ') : t('knowledgeEditor.basic.typeDocument')}</span></span>
            {kbMeta?.description ? <span className="kb-info-card-row"><span className="kb-info-card-label">{t('knowledgeBase.description')}</span><span className="kb-info-card-value">{kbMeta.description}</span></span> : null}
            {kbMeta?.createdAt ? <span className="kb-info-card-row"><span className="kb-info-card-label">{t('knowledgeBase.infoCard.createdAt')}</span><span className="kb-info-card-value">{kbMeta.createdAt}</span></span> : null}
          </span>
        </span>
        <button type="button" className="kb-settings-button" aria-label={t('knowledgeBase.settings')} title={t('knowledgeBase.settings')} disabled={!knowledgeBaseId} onClick={() => knowledgeBaseId && onNavigate(faqKBSettingsPath(knowledgeBaseId))}>
          <GearIcon size={14} />
        </button>
      </div>
    </div>
  );
}

// --- Page view (Vue faq-header / faq-filter-bar / faq-scroll-container) ------------

// Vue editor form (FAQEntryManager.vue:443-563): list fields with drafts,
// not one textarea per list — add/remove semantics live in pushListItem/removeListItem.
type FormState = {
  question: string;
  similarQuestions: string[];
  negativeQuestions: string[];
  answers: string[];
  tagId: string;
  enabled: boolean;
  recommended: boolean;
  similarDraft: string;
  negativeDraft: string;
  answerDraft: string;
};
const emptyForm: FormState = { question: '', similarQuestions: [], negativeQuestions: [], answers: [], tagId: '', enabled: true, recommended: false, similarDraft: '', negativeDraft: '', answerDraft: '' };
// Vue FAQEntryManager.vue:1058 — the scroll list appends 20 rows per page.
const PAGE_SIZE = 20;

export interface FAQPageViewProps {
  t?: Translate;
  knowledgeBaseId?: string;
  kbName?: string | null;
  kbMeta?: FAQKBMeta;
  kbList?: KBListItem[];
  tags?: KnowledgeTag[];
  activeTagIds?: string[];
  entries?: FAQEntry[];
  total?: number;
  /** Vue hasMore tri-state: null until the first page loads. */
  hasMore?: boolean | null;
  /** Vue loadingMore — spinner in .faq-load-more while the next page appends. */
  loadingMore?: boolean;
  loading?: boolean;
  canContribute?: boolean;
  selected?: Set<number>;
  keywordDraft?: string;
  importOpen?: boolean;
  importMode?: 'append' | 'replace';
  importFileName?: string | null;
  importBusy?: boolean;
  /** Vue importState.preview — parsed rows shown inside the import dialog. */
  importPreview?: FAQEntryPayload[];
  /** Vue importState.taskStatus — header .faq-import-strip progress affordance. */
  importTask?: FAQImportTaskView | null;
  /** B6: Vue importResult — persisted last-import result (FAQEntryManager.vue:1263). */
  importResult?: FAQImportResultView | null;
  /** B6: Vue closeImportResult — PUT display-status 'close' then hide. */
  onCloseImportResult?: () => void;
  /** B6: Vue downloadFailedEntries — open the failed-entries CSV. */
  onDownloadFailedEntries?: () => void;
  /** Vue entryStatusLoading — per-entry ids whose status update is in flight. */
  statusUpdatingIds?: readonly number[];
  editorOpen?: boolean;
  editorTitle?: string;
  editorMode?: 'create' | 'edit';
  form?: FormState;
  saving?: boolean;
  exportLoading?: boolean;
  message?: { tone: 'error' | 'success' | 'warning'; text: string } | null;
  batchTag?: string;
  onNavigate?: (path: string) => void;
  onKeywordDraftChange?: (value: string) => void;
  onSearchSubmit?: () => void;
  onSearchClear?: () => void;
  onToggleTag?: (tagId: string) => void;
  onOpenTagManage?: () => void;
  onOpenCreate?: () => void;
  onOpenImport?: () => void;
  onCloseImport?: () => void;
  onImportModeChange?: (mode: 'append' | 'replace') => void;
  onImportFile?: (file: File) => void;
  onImportConfirm?: () => void;
  onExport?: (format: 'csv' | 'json') => void;
  onToggleSelect?: (id: number, checked: boolean) => void;
  onToggleSelectAll?: (checked: boolean) => void;
  onEditEntry?: (entry: FAQEntry) => void;
  onDeleteEntry?: (entry: FAQEntry) => void;
  /** Vue handleEntryStatusChange — per-card enable/disable toggle. */
  onToggleEntryStatus?: (entry: FAQEntry, value: boolean) => void;
  onBatchEnable?: () => void;
  onBatchDisable?: () => void;
  onBatchRecommend?: () => void;
  onBatchTagChange?: (value: string) => void;
  onBatchSetTag?: () => void;
  onBatchDelete?: () => void;
  onLoadMore?: () => void;
  onOpenEditor?: () => void;
  onCloseEditor?: () => void;
  onFormChange?: (patch: Partial<FormState>) => void;
  onEditorSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  /** B4: Vue search test drawer (FAQEntryManager.vue:734-853). */
  searchOpen?: boolean;
  searchForm?: FAQSearchFormState;
  searching?: boolean;
  hasSearched?: boolean;
  searchResults?: FAQSearchHit[];
  onOpenSearchTest?: () => void;
  onCloseSearchTest?: () => void;
  onSearchFormChange?: (patch: Partial<FAQSearchFormState>) => void;
  onSearchTestSubmit?: () => void;
}

export function FAQPageView(props: FAQPageViewProps = {}) {
  const {
    t: tr,
    knowledgeBaseId = '',
    kbName = null,
    kbMeta,
    kbList = [],
    tags = [],
    activeTagIds = [],
    entries = [],
    total = 0,
    hasMore = null,
    loadingMore = false,
    loading = false,
    canContribute = true,
    selected = new Set<number>(),
    keywordDraft = '',
    importOpen = false,
    importMode = 'append',
    importFileName = null,
    importBusy = false,
    importPreview = [],
    importTask = null,
    importResult = null,
    onCloseImportResult = () => {},
    onDownloadFailedEntries = () => {},
    statusUpdatingIds = [],
    editorOpen = false,
    editorTitle = '',
    editorMode = 'create',
    form = emptyForm,
    saving = false,
    exportLoading = false,
    message = null,
    batchTag = '',
    onNavigate = defaultNavigate,
    onKeywordDraftChange = () => {},
    onSearchSubmit = () => {},
    onSearchClear = () => {},
    onToggleTag = () => {},
    onOpenTagManage = () => {},
    onOpenCreate = () => {},
    onOpenImport = () => {},
    onCloseImport = () => {},
    onImportModeChange = () => {},
    onImportFile = () => {},
    onImportConfirm = () => {},
    onExport = () => {},
    onToggleSelect = () => {},
    onToggleSelectAll = () => {},
    onEditEntry = () => {},
    onDeleteEntry = () => {},
    onBatchEnable = () => {},
    onBatchDisable = () => {},
    onBatchRecommend = () => {},
    onBatchTagChange = () => {},
    onBatchSetTag = () => {},
    onBatchDelete = () => {},
    onLoadMore = () => {},
    onToggleEntryStatus = () => {},
    onCloseEditor = () => {},
    onFormChange = () => {},
    onEditorSubmit = () => {},
    searchOpen = false,
    searchForm = faqSearchDefaultForm(),
    searching = false,
    hasSearched = false,
    searchResults = [],
    onOpenSearchTest = () => {},
    onCloseSearchTest = () => {},
    onSearchFormChange = () => {},
    onSearchTestSubmit = () => {},
  } = props;
  const t = tr ?? createTranslator('zh-CN');
  const [tagPanelOpen, setTagPanelOpen] = useState(false);
  const [createMenuOpen, setCreateMenuOpen] = useState(false);
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const [collapsedSections, setCollapsedSections] = useState<FAQSectionCollapseState>({});
  // Vue entry.showMore — one open card more-menu at a time (FAQEntryManager.vue:265-283).
  const [moreMenuId, setMoreMenuId] = useState<number | null>(null);
  // B4: Vue stores expanded on each hit with default false (:2669) — a per-id set.
  const [expandedResults, setExpandedResults] = useState<ReadonlySet<number>>(new Set());
  // A new search replaces the hit list, so stale expansions drop first (Vue :2664-2673).
  const runSearchTest = () => {
    setExpandedResults(new Set());
    onSearchTestSubmit();
  };
  const toggleSearchResult = (id: number) => setExpandedResults((current) => toggleSearchResultId(current, id));
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Vue handleScroll (FAQEntryManager.vue:1624): within 200px of the bottom,
  // ask the container for the next page of entries. The inner container only
  // scrolls once the shell bounds its height, so mirror the handler onto the
  // window as well — whichever scrolls first triggers the append.
  const handleListScroll = (metrics: { top: number; viewport: number; height: number }) => {
    if (!hasMore || loadingMore) return;
    if (metrics.top + metrics.viewport < metrics.height - 200) return;
    onLoadMore();
  };
  const handleContainerScroll = () => {
    const el = scrollRef.current;
    if (el) handleListScroll({ top: el.scrollTop, viewport: el.clientHeight, height: el.scrollHeight });
  };
  const handleWindowScroll = () => {
    const doc = document.documentElement;
    handleListScroll({ top: doc.scrollTop, viewport: window.innerHeight, height: doc.scrollHeight });
  };
  useEffect(() => {
    if (!hasMore || loadingMore) return;
    window.addEventListener('scroll', handleWindowScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleWindowScroll);
  });
  // Vue fills a short page that cannot scroll yet (FAQEntryManager.vue:1640-1651).
  useEffect(() => {
    if (!hasMore || loadingMore) return;
    const el = scrollRef.current;
    if (el && el.scrollHeight <= el.clientHeight + 50) onLoadMore();
  });
  const tagNameBySeq = new Map<number, string>();
  for (const tag of tags) {
    if (typeof tag.seq_id === 'number') tagNameBySeq.set(tag.seq_id, tag.name);
  }
  const activeTagLabel = activeTagIds.length === 0
    ? t('knowledgeBase.allTags')
    : activeTagIds.length === 1
      ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
      : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
  // Vue addSimilar/addNegative/addAnswer (FAQEntryManager.vue:1714-1753) — the
  // view composes list mutations on top of the shared form patch channel.
  const addSimilar = () => {
    const next = pushListItem(form.similarQuestions, form.similarDraft, FAQ_SIMILAR_CAP);
    onFormChange(next.added ? { similarQuestions: next.list, similarDraft: '' } : {});
  };
  const addNegative = () => {
    const next = pushListItem(form.negativeQuestions, form.negativeDraft, FAQ_NEGATIVE_CAP);
    onFormChange(next.added ? { negativeQuestions: next.list, negativeDraft: '' } : {});
  };
  const addAnswer = () => {
    const next = pushListItem(form.answers, form.answerDraft, FAQ_ANSWER_CAP);
    onFormChange(next.added ? { answers: next.list, answerDraft: '' } : {});
  };
  const sectionButton = (entryId: number, section: string) => ({
    'aria-expanded': !isSectionCollapsed(collapsedSections, entryId, section),
    onClick: () => setCollapsedSections((current) => toggleSection(current, entryId, section)),
  });

  return (
    <main className="wk-page wk-faq-page">
      <header className="faq-header">
        <div className="faq-header-title">
          <FAQBreadcrumb t={t} knowledgeBaseId={knowledgeBaseId} kbName={kbName} kbList={kbList} kbMeta={kbMeta} onNavigate={onNavigate} />
          <p className="faq-subtitle">{t('knowledgeEditor.faq.subtitle')}</p>
          {importTask ? (
            <div className={'faq-import-strip faq-import-strip--' + importTask.status} role="status">
              <span className={'faq-import-strip__icon' + (importTask.status === 'running' ? ' is-spinning' : '')} aria-hidden="true" />
              <span className="faq-import-strip__text">{importTask.text}</span>
              <span className="faq-import-strip__bar"><span className="faq-import-strip__bar-fill" style={{ width: importTask.progress + '%' }} /></span>
              <span className="faq-import-strip__count">{importTask.processed}/{importTask.total}</span>
            </div>
          ) : null}
          {faqImportResultVisible(importResult, Boolean(importTask)) ? (
            // B6: Vue 导入结果持久化条 (:52-79) — summary + mode tag + failed
            // entries link + time + close; persists until closed or replaced.
            <div className="faq-import-strip faq-import-strip--result" role="status">
              <span className="faq-import-strip__text">{faqImportResultSummary(importResult!, t)}</span>
              <span className={'faq-import-mode-tag ' + (importResult!.import_mode === 'append' ? 'is-append' : 'is-replace')}>
                {importResult!.import_mode === 'append' ? t('FAQ.import.appendMode') : t('FAQ.import.replaceMode')}
              </span>
              {importResult!.failed_entries_url && importResult!.failed_count > 0 ? (
                <button type="button" className="faq-import-strip__link" onClick={onDownloadFailedEntries}>{t('FAQ.import.downloadReasons')}</button>
              ) : null}
              <span className="faq-import-strip__time">{formatImportTime(importResult!.imported_at)}</span>
              <button type="button" className="faq-import-strip__close" aria-label={t('common.close')} title={t('common.close')} onClick={onCloseImportResult}><CloseIcon size={12} /></button>
            </div>
          ) : null}
        </div>
      </header>

      <div className="faq-main">
        <div className="faq-card-area">
          <div className="faq-filter-bar">
            <div className="faq-search-input">
              <SearchIcon size={16} className="faq-search-icon" />
              <input
                type="search"
                value={keywordDraft}
                placeholder={t('knowledgeEditor.faq.searchPlaceholder')}
                aria-label={t('knowledgeBase.faq.search')}
                onChange={(event) => onKeywordDraftChange(event.target.value)}
                onKeyDown={(event) => { if (event.key === 'Enter') onSearchSubmit(); }}
              />
              {keywordDraft ? <button type="button" className="faq-search-clear" aria-label={t('common.close')} onClick={onSearchClear}><CloseIcon size={14} /></button> : null}
            </div>
            <div className="faq-filter-bar__filters">
              <span className="doc-filter-field" onBlur={(event) => closeOnBlur(event, () => setTagPanelOpen(false))}>
                <button type="button" className="doc-tag-filter-trigger" aria-label={t('knowledgeBase.tagFilterTitle')} title={t('knowledgeBase.tagFilterTitle')} aria-haspopup="menu" aria-expanded={tagPanelOpen} onClick={() => setTagPanelOpen((open) => !open)}>
                  <span className="doc-tag-filter-trigger__prefix"><TagIcon size={16} /></span>
                  <span className="doc-tag-filter-trigger__label">{activeTagLabel}</span>
                  <span className="doc-tag-filter-trigger__suffix"><Icon size={16} className="doc-tag-filter-trigger__caret"><path d={Chevrons.down} /></Icon></span>
                </button>
                <span className="faq-menu tag-filter-panel" hidden={!tagPanelOpen}>
                  <span className="tag-filter-panel__header">
                    <span>{t('knowledgeBase.tagFilterTitle')}</span>
                    <span className="tag-filter-panel__count">({tags.length})</span>
                  </span>
                  <span className="tag-filter-chips">
                    {tags.map((tag) => (
                      <button key={tag.id} type="button" className={'tag-filter-chip' + (activeTagIds.includes(tag.id) ? ' active' : '')} title={tag.name + ' (' + (tag.chunk_count || 0) + ')'} onClick={() => onToggleTag(tag.id)}>
                        <span className="tag-filter-chip__label">{tag.name}</span>
                        <span className="tag-filter-chip__count">{tag.chunk_count || 0}</span>
                      </button>
                    ))}
                    {tags.length === 0 ? <span className="tag-empty-state">{t('knowledgeBase.tagEmptyResult')}</span> : null}
                  </span>
                  {canContribute ? <button type="button" className="tag-filter-panel__manage" onClick={onOpenTagManage}>{t('knowledgeBase.tagManageLink')}</button> : null}
                </span>
              </span>
            </div>
            <div className="faq-filter-bar__trailing">
              {canContribute ? (
                <span className="faq-icon-menu-host" onBlur={(event) => closeOnBlur(event, () => setCreateMenuOpen(false))}>
                  <button type="button" className="content-bar-icon-btn" aria-label={t('knowledgeEditor.faq.createGroup')} title={t('knowledgeEditor.faq.createGroup')} aria-haspopup="menu" aria-expanded={createMenuOpen} onClick={() => setCreateMenuOpen((open) => !open)}>
                    <AddIcon size={16} />
                  </button>
                  <span className="faq-menu faq-icon-menu" role="menu" hidden={!createMenuOpen}>
                    <button type="button" role="menuitem" className="faq-menu-item" onClick={() => { setCreateMenuOpen(false); onOpenCreate(); }}>{t('knowledgeEditor.faq.editorCreate')}</button>
                    <button type="button" role="menuitem" className="faq-menu-item" onClick={() => { setCreateMenuOpen(false); onOpenImport(); }}>{t('knowledgeEditor.faqImport.importButton')}</button>
                  </span>
                </span>
              ) : null}
              <span className="faq-icon-menu-host" onBlur={(event) => closeOnBlur(event, () => setExportMenuOpen(false))}>
                <button type="button" className="content-bar-icon-btn" aria-label={t('knowledgeEditor.faqExport.exportButton')} title={t('knowledgeEditor.faqExport.exportButton')} aria-haspopup="menu" aria-expanded={exportMenuOpen} disabled={exportLoading} onClick={() => setExportMenuOpen((open) => !open)}>
                  <DownloadIcon size={16} />
                </button>
                <span className="faq-menu faq-icon-menu" role="menu" hidden={!exportMenuOpen}>
                  <button type="button" role="menuitem" className="faq-menu-item" onClick={() => { setExportMenuOpen(false); onExport('csv'); }}>{t('knowledgeEditor.faqExport.exportCSV')}</button>
                  <button type="button" role="menuitem" className="faq-menu-item" onClick={() => { setExportMenuOpen(false); onExport('json'); }}>{t('knowledgeEditor.faqExport.exportJSON')}</button>
                </span>
              </span>
              <button type="button" className="content-bar-icon-btn" aria-label={t('knowledgeEditor.faq.searchTest')} title={t('knowledgeEditor.faq.searchTest')} onClick={onOpenSearchTest}>
                <SearchIcon size={16} />
              </button>
            </div>
          </div>

          {message ? <Status tone={message.tone}>{message.text}</Status> : null}

          <div className="faq-scroll-container" ref={scrollRef} onScroll={handleContainerScroll}>
            {loading && entries.length === 0 ? (
              <div className="faq-skeleton-grid" aria-hidden="true">
                {Array.from({ length: 6 }, (_, index) => <div key={index} className="faq-card-skeleton" />)}
              </div>
            ) : entries.length > 0 ? (
              <div className="faq-card-list">
                {/* Vue faq-card-list (FAQEntryManager.vue:254-410): header question +
                    more menu, three collapsible sections, footer tag chip + switch. */}
                {entries.map((entry) => {
                  const isSelected = selected.has(entry.id);
                  const tagName = typeof entry.tag_id === 'number' ? tagNameBySeq.get(entry.tag_id) : undefined;
                  // Vue faq-section (:291-357): similar/negative render only when
                  // non-empty, answers always; bodies start collapsed and toggle.
                  const section = (name: 'similar' | 'negative' | 'answers', labelKey: string, values: string[], always = false) => {
                    if (!always && values.length === 0) return null;
                    const collapsed = isSectionCollapsed(collapsedSections, entry.id, name);
                    const tagClass = name === 'negative' ? 'question-tag is-negative' : name === 'answers' ? 'question-tag is-answer' : 'question-tag';
                    return <section className={'faq-section ' + name} key={name}>
                      <button type="button" className="faq-section-label clickable" {...sectionButton(entry.id, name)}>
                        <span>{t(labelKey)}</span>
                        <span className="section-count">({values.length})</span>
                        <Icon size={13} className="collapse-icon"><path d={collapsed ? Chevrons.right : Chevrons.down} /></Icon>
                      </button>
                      {/* B5: Vue wraps each chip in FAQTagTooltip (:303-354) — the
                          bubble carries the full text instead of a native title. */}
                      <div className="faq-tags" hidden={collapsed}>
                        {values.map((value, index) => (
                          <FaqTagTooltip key={index} content={value} placement="top" type={name === 'negative' ? 'negative' : name === 'answers' ? 'answer' : 'similar'}>
                            <span className={tagClass}>{value}</span>
                          </FaqTagTooltip>
                        ))}
                      </div>
                    </section>;
                  };
                  return (
                    <article
                      key={entry.id}
                      className={'faq-card' + (canContribute ? ' is-selectable' : '') + (isSelected ? ' selected' : '')}
                      onClick={canContribute ? () => onToggleSelect(entry.id, !isSelected) : undefined}
                    >
                      <div className="faq-card-header">
                        <div className="faq-header-top">
                          {canContribute ? (
                            <label className="faq-card-check" onClick={(event) => event.stopPropagation()}>
                              <input type="checkbox" checked={isSelected} aria-label={entry.standard_question} onChange={(event) => onToggleSelect(entry.id, event.target.checked)} />
                            </label>
                          ) : null}
                          <strong className="faq-question" title={entry.standard_question}>{entry.standard_question}</strong>
                          {canContribute ? (
                            <span className="faq-more-host" onBlur={(event) => closeOnBlur(event, () => setMoreMenuId(null))} onClick={(event) => event.stopPropagation()}>
                              <button
                                type="button"
                                className="card-more-btn"
                                aria-label={t('knowledgeBase.columnActions')}
                                title={t('knowledgeBase.columnActions')}
                                aria-haspopup="menu"
                                aria-expanded={moreMenuId === entry.id}
                                onClick={() => setMoreMenuId((current) => (current === entry.id ? null : entry.id))}
                              >
                                <MoreIcon size={16} />
                              </button>
                              {/* Vue popup-menu (:271-282): edit then delete */}
                              <span className="faq-menu card-more-popup" role="menu" hidden={moreMenuId !== entry.id}>
                                <button type="button" role="menuitem" className="faq-menu-item" onClick={() => { setMoreMenuId(null); onEditEntry(entry); }}>{t('common.edit')}</button>
                                <button type="button" role="menuitem" className="faq-menu-item is-danger" onClick={() => { setMoreMenuId(null); onDeleteEntry(entry); }}>{t('common.delete')}</button>
                              </span>
                            </span>
                          ) : null}
                        </div>
                      </div>
                      <div className="faq-card-body">
                        {section('similar', 'knowledgeEditor.faq.similarQuestions', entry.similar_questions)}
                        {section('negative', 'knowledgeEditor.faq.negativeQuestions', entry.negative_questions)}
                        {section('answers', 'knowledgeEditor.faq.answers', entry.answers, true)}
                      </div>
                      <div className="faq-card-footer">
                        <div className="faq-card-tag">
                          {/* B5 refine: the native title (d3a39b7b) is replaced by the
                              FAQTagTooltip bubble — hover opens the fixed, viewport-clamped
                              bubble with the full tag name; tag-text truncation stays. */}
                          <FaqTagTooltip content={tagName ?? t('knowledgeBase.untagged')} placement="top">
                            <span className="faq-tag-chip"><span className="tag-text">{tagName ?? t('knowledgeBase.untagged')}</span></span>
                          </FaqTagTooltip>
                        </div>
                        {canContribute ? (
                          <div className="faq-card-status" onClick={(event) => event.stopPropagation()}>
                            <button type="button" role="switch" aria-checked={entry.is_enabled} aria-label={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')} title={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')} className={'faq-status-switch' + (entry.is_enabled ? ' is-on' : '')} disabled={statusUpdatingIds.includes(entry.id)} onClick={() => onToggleEntryStatus(entry, !entry.is_enabled)}><span className="faq-status-switch__thumb" /></button>
                          </div>
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : (
              <div className="faq-empty-state">
                <div className="empty-content">
                  <FileAddIcon size={48} className="empty-icon" />
                  <div className="empty-text">{t('knowledgeEditor.faq.emptyTitle')}</div>
                  <div className="empty-desc">{t('knowledgeEditor.faq.emptyDesc')}</div>
                </div>
              </div>
            )}
            {loadingMore ? <div className="faq-load-more">{t('common.loading')}</div> : null}
            {hasMore === false && entries.length > 0 ? <div className="faq-no-more">{t('common.noMoreData')}</div> : null}
          </div>

          {canContribute && selected.size > 0 ? (
            <div className="wk-list-actions faq-batch-bar" aria-label="FAQ batch actions">
              <span>{t('common.itemCount', { count: selected.size })}</span>
              <Button type="button" onClick={onBatchEnable}>{t('knowledgeEditor.faq.batchEnable')}</Button>
              <Button type="button" onClick={onBatchDisable}>{t('knowledgeEditor.faq.batchDisable')}</Button>
              <Button type="button" onClick={onBatchRecommend}>{t('knowledgeEditor.faq.recommended')}</Button>
              <select className="wk-batch-tag" value={batchTag} onChange={(event) => onBatchTagChange(event.target.value)} aria-label={t('knowledgeBase.tagLabel')}>
                <option value="">{t('knowledgeBase.untagged')}</option>
                {[...tagNameBySeq.entries()].map(([seqId, name]) => <option key={seqId} value={String(seqId)}>{name}</option>)}
              </select>
              <Button type="button" onClick={onBatchSetTag}>{t('knowledgeEditor.faq.batchUpdateTag')}</Button>
              <Button type="button" onClick={onBatchDelete}>{t('knowledgeEditor.faq.batchDelete')}</Button>
            </div>
          ) : null}

        </div>
      </div>

      {importOpen ? (
        <section className="faq-import-overlay" aria-label={t('knowledgeEditor.faqImport.title')}>
          <div className="faq-import-modal">
            <button type="button" className="faq-modal-close" aria-label={t('common.close')} onClick={onCloseImport}><CloseIcon size={16} /></button>
            <div className="faq-import-header"><h2>{t('knowledgeEditor.faqImport.title')}</h2></div>
            <div className="faq-import-content">
              <div className="import-form-item">
                <label className="import-form-label">{t('knowledgeEditor.faqImport.modeLabel')}</label>
                <div className="import-radio-group" role="radiogroup" aria-label={t('knowledgeEditor.faqImport.modeLabel')}>
                  <label className={'import-radio-button' + (importMode === 'append' ? ' is-active' : '')}>
                    <input type="radio" name="faq-import-mode" value="append" checked={importMode === 'append'} onChange={() => onImportModeChange('append')} /> {t('knowledgeEditor.faqImport.appendMode')}
                  </label>
                  <label className={'import-radio-button' + (importMode === 'replace' ? ' is-active' : '')}>
                    <input type="radio" name="faq-import-mode" value="replace" checked={importMode === 'replace'} onChange={() => onImportModeChange('replace')} /> {t('knowledgeEditor.faqImport.replaceMode')}
                  </label>
                </div>
              </div>
              <div className="import-form-item">
                <label className="import-form-label">{t('knowledgeEditor.faqImport.fileLabel')}</label>
                <div
                  className="file-upload-area"
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event: DragEvent<HTMLDivElement>) => { event.preventDefault(); const file = event.dataTransfer.files?.[0]; if (file) onImportFile(file); }}
                >
                  <UploadIcon size={28} className="upload-icon" />
                  <span className="upload-primary-text">{importFileName ?? t('knowledgeEditor.faqImport.clickToUpload')}</span>
                  {importFileName ? null : <span className="upload-secondary-text">{t('knowledgeEditor.faqImport.dragDropTip')}</span>}
                  <input type="file" accept=".json,.csv,.xlsx,.xls,application/json,text/csv" onChange={(event) => { const file = event.target.files?.[0]; if (file) onImportFile(file); event.target.value = ''; }} />
                </div>
                <p className="import-form-tip">{t('knowledgeEditor.faqImport.fileTip')}</p>
              </div>
              {importPreview.length > 0 ? (
                <div className="import-preview">
                  <div className="preview-header">
                    <span className="preview-icon" aria-hidden="true"><FileAddIcon size={16} /></span>
                    <span className="preview-title">{t('knowledgeEditor.faqImport.previewCount', { count: importPreview.length })}</span>
                  </div>
                  <div className="preview-list">
                    {importPreview.slice(0, 5).map((item, index) => (
                      <div key={index} className="preview-item">
                        <span className="preview-index">{index + 1}</span>
                        <span className="preview-question">{item.standard_question}</span>
                      </div>
                    ))}
                  </div>
                  {importPreview.length > 5 ? <p className="preview-more">{t('knowledgeEditor.faqImport.previewMore', { count: importPreview.length - 5 })}</p> : null}
                </div>
              ) : null}
            </div>
            <div className="faq-import-footer">
              <Button type="button" onClick={onCloseImport}>{t('common.cancel')}</Button>
              <Button type="button" loading={importBusy} disabled={!importFileName || importBusy} onClick={onImportConfirm}>{t('knowledgeEditor.faqImport.importButton')}</Button>
            </div>
          </div>
        </section>
      ) : null}

      {/* B1: Vue editor drawer (FAQEntryManager.vue:440-577) — 520px right
          drawer, one settings-row per field with the shared desc keys, list
          editors with add/remove, and a pinned cancel/save footer. */}
      {editorOpen ? (
        <section className="faq-editor-overlay" aria-label={editorTitle}>
          <aside className="faq-editor-drawer">
            <div className="faq-editor-header">
              <h2>{editorTitle}</h2>
              <button type="button" className="faq-modal-close" aria-label={t('common.close')} onClick={onCloseEditor}><CloseIcon size={16} /></button>
            </div>
            <form className="faq-editor-form" onSubmit={onEditorSubmit}>
              <div className="faq-editor-form-body">
                {message?.tone === 'error' ? <div className="faq-editor-error" role="alert"><Status tone="error">{message.text}</Status></div> : null}
                <div className="settings-group">
                  <div className="setting-row">
                    <div className="setting-info">
                      <label className="required-label" htmlFor="faq-editor-question">{t('knowledgeEditor.faq.standardQuestion')} <span className="required-mark">*</span></label>
                      <p className="desc">{t('knowledgeEditor.faq.standardQuestionDesc')}</p>
                    </div>
                    <div className="setting-control">
                      <input id="faq-editor-question" className="full-width-input" {...({ maxlength: 200 } as React.InputHTMLAttributes<HTMLInputElement>)} value={form.question} onChange={(event) => onFormChange({ question: event.target.value })} />
                    </div>
                  </div>
                  <div className="setting-row setting-row-optional setting-row-similar">
                    <div className="setting-info">
                      <label className="optional-label" htmlFor="faq-editor-similar">{t('knowledgeEditor.faq.similarQuestions')}</label>
                      <p className="desc optional-desc">{t('knowledgeEditor.faq.similarQuestionsDesc')}</p>
                    </div>
                    <div className="setting-control">
                      <div className="full-width-input-wrapper">
                        <input
                          id="faq-editor-similar"
                          className="full-width-input"
                          placeholder={t('knowledgeEditor.faq.similarPlaceholder')}
                          value={form.similarDraft}
                          onChange={(event) => onFormChange({ similarDraft: event.target.value })}
                          onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addSimilar(); } }}
                        />
                        <button type="button" className="add-item-btn" aria-label={t('knowledgeEditor.faq.similarQuestions')} disabled={!form.similarDraft.trim() || form.similarQuestions.length >= FAQ_SIMILAR_CAP} onClick={addSimilar}><AddIcon size={14} /></button>
                      </div>
                      {form.similarQuestions.length > 0 ? (
                        <div className="item-list">
                          {form.similarQuestions.map((question, index) => (
                            <div key={index} className="item-row">
                              <div className="item-content">{question}</div>
                              <button type="button" className="remove-item-btn" aria-label={t('common.delete')} onClick={() => onFormChange({ similarQuestions: removeListItem(form.similarQuestions, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row setting-row-optional setting-row-negative">
                    <div className="setting-info">
                      <label className="optional-label" htmlFor="faq-editor-negative">{t('knowledgeEditor.faq.negativeQuestions')}</label>
                      <p className="desc optional-desc">{t('knowledgeEditor.faq.negativeQuestionsDesc')}</p>
                    </div>
                    <div className="setting-control">
                      <div className="full-width-input-wrapper">
                        <input
                          id="faq-editor-negative"
                          className="full-width-input"
                          placeholder={t('knowledgeEditor.faq.negativePlaceholder')}
                          value={form.negativeDraft}
                          onChange={(event) => onFormChange({ negativeDraft: event.target.value })}
                          onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addNegative(); } }}
                        />
                        <button type="button" className="add-item-btn" aria-label={t('knowledgeEditor.faq.negativeQuestions')} disabled={!form.negativeDraft.trim() || form.negativeQuestions.length >= FAQ_NEGATIVE_CAP} onClick={addNegative}><AddIcon size={14} /></button>
                      </div>
                      {form.negativeQuestions.length > 0 ? (
                        <div className="item-list">
                          {form.negativeQuestions.map((question, index) => (
                            <div key={index} className="item-row negative">
                              <div className="item-content">{question}</div>
                              <button type="button" className="remove-item-btn" aria-label={t('common.delete')} onClick={() => onFormChange({ negativeQuestions: removeListItem(form.negativeQuestions, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row setting-row-primary setting-row-answer">
                    <div className="setting-info">
                      <label className="required-label" htmlFor="faq-editor-answer">{t('knowledgeEditor.faq.answers')} <span className="required-mark">*</span></label>
                      <p className="desc">{t('knowledgeEditor.faq.answersDesc')}</p>
                    </div>
                    <div className="setting-control">
                      <div className="textarea-container">
                        <div className="full-width-input-wrapper textarea-wrapper">
                          <textarea
                            id="faq-editor-answer"
                            className="full-width-textarea"
                            rows={3}
                            placeholder={t('knowledgeEditor.faq.answerPlaceholder')}
                            value={form.answerDraft}
                            onChange={(event) => onFormChange({ answerDraft: event.target.value })}
                            onKeyDown={(event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); addAnswer(); } }}
                          />
                          <button type="button" className="add-item-btn" aria-label={t('knowledgeEditor.faq.answers')} disabled={!form.answerDraft.trim() || form.answers.length >= FAQ_ANSWER_CAP} onClick={addAnswer}><AddIcon size={14} /></button>
                        </div>
                        <div className="item-count">{form.answers.length}/{FAQ_ANSWER_CAP}</div>
                      </div>
                      {form.answers.length > 0 ? (
                        <div className="item-list">
                          {form.answers.map((answer, index) => (
                            <div key={index} className="item-row answer-row">
                              <div className="item-content">{answer}</div>
                              <button type="button" className="remove-item-btn" aria-label={t('common.delete')} onClick={() => onFormChange({ answers: removeListItem(form.answers, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row">
                    <div className="setting-info">
                      <label htmlFor="faq-editor-tag">{t('knowledgeBase.tagLabel')}</label>
                      <p className="desc">{t('knowledgeEditor.faq.tagDesc')}</p>
                    </div>
                    <div className="setting-control">
                      <select id="faq-editor-tag" className="full-width-input" value={form.tagId} onChange={(event) => onFormChange({ tagId: event.target.value })}>
                        <option value="">{t('knowledgeEditor.faq.tagPlaceholder')}</option>
                        {[...tagNameBySeq.entries()].map(([seqId, name]) => <option key={seqId} value={String(seqId)}>{name}</option>)}
                      </select>
                    </div>
                  </div>
                </div>
              </div>
              <div className="faq-editor-footer">
                <Button type="button" onClick={onCloseEditor}>{t('common.cancel')}</Button>
                <Button type="submit" loading={saving}>{editorMode === 'create' ? t('knowledgeEditor.faq.editorCreate') : t('common.save')}</Button>
              </div>
            </form>
          </aside>
        </section>
      ) : null}

      {/* B4: Vue search test drawer (FAQEntryManager.vue:734-853) — 420px right
          drawer, query input + two sliders with the shared desc keys, a primary
          search button, and a ranked result list with 3-decimal score tags. */}
      {searchOpen ? (
        <section className="faq-editor-overlay" aria-label={t('knowledgeEditor.faq.searchTestTitle')}>
          <aside className="faq-editor-drawer faq-search-drawer">
            <div className="faq-editor-header">
              <h2>{t('knowledgeEditor.faq.searchTestTitle')}</h2>
              <button type="button" className="faq-modal-close" aria-label={t('common.close')} onClick={onCloseSearchTest}><CloseIcon size={16} /></button>
            </div>
            <div className="faq-editor-form-body">
              {message?.tone === 'error' ? <div className="faq-editor-error" role="alert"><Status tone="error">{message.text}</Status></div> : null}
              <div className="settings-group">
                <div className="setting-row search-first-row">
                  <div className="setting-info">
                    <label htmlFor="faq-search-query">{t('knowledgeEditor.faq.queryLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.queryPlaceholder')}</p>
                  </div>
                  <div className="setting-control">
                    <input
                      id="faq-search-query"
                      className="full-width-input"
                      placeholder={t('knowledgeEditor.faq.queryPlaceholder')}
                      value={searchForm.query}
                      onChange={(event) => onSearchFormChange({ query: event.target.value })}
                      onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); runSearchTest(); } }}
                    />
                  </div>
                </div>
                <div className="setting-row">
                  <div className="setting-info">
                    <label htmlFor="faq-search-threshold">{t('knowledgeEditor.faq.similarityThresholdLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.vectorThresholdDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="slider-wrapper">
                      <input
                        id="faq-search-threshold"
                        type="range" min={FAQ_SEARCH_VECTOR_THRESHOLD.min} max={FAQ_SEARCH_VECTOR_THRESHOLD.max} step={FAQ_SEARCH_VECTOR_THRESHOLD.step}
                        value={searchForm.vectorThreshold}
                        onChange={(event) => onSearchFormChange({ vectorThreshold: Number(event.target.value) })}
                      />
                      <div className="slider-value">{searchForm.vectorThreshold.toFixed(2)}</div>
                    </div>
                  </div>
                </div>
                <div className="setting-row">
                  <div className="setting-info">
                    <label htmlFor="faq-search-match-count">{t('knowledgeEditor.faq.matchCountLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.matchCountDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="slider-wrapper">
                      <input
                        id="faq-search-match-count"
                        type="range" min={FAQ_SEARCH_MATCH_COUNT.min} max={FAQ_SEARCH_MATCH_COUNT.max} step={FAQ_SEARCH_MATCH_COUNT.step}
                        value={searchForm.matchCount}
                        onChange={(event) => onSearchFormChange({ matchCount: Number(event.target.value) })}
                      />
                      <div className="slider-value">{searchForm.matchCount}</div>
                    </div>
                  </div>
                </div>
                <div className="setting-row">
                  <div className="setting-control">
                    <Button type="button" className="search-button" loading={searching} onClick={runSearchTest}>
                      {searching ? t('knowledgeEditor.faq.searching') : t('knowledgeEditor.faq.searchButton')}
                    </Button>
                  </div>
                </div>
              </div>
              {searchResults.length > 0 || hasSearched ? (
                <FAQSearchResults t={t} results={searchResults} expandedIds={expandedResults} onToggle={toggleSearchResult} />
              ) : null}
            </div>
          </aside>
        </section>
      ) : null}
    </main>
  );
}

// --- B4 results list (Vue search-results block, FAQEntryManager.vue:792-851) -------

export interface FAQSearchResultsProps {
  t?: Translate;
  results?: readonly FAQSearchHit[];
  expandedIds?: ReadonlySet<number>;
  onToggle?: (id: number) => void;
}

export function FAQSearchResults({ t: tr, results = [], expandedIds = new Set<number>(), onToggle = () => {} }: FAQSearchResultsProps = {}) {
  const t = tr ?? createTranslator('zh-CN');
  return (
    <div className="search-results">
      <div className="results-header">
        <span>{t('knowledgeEditor.faq.searchResults')} ({results.length})</span>
      </div>
      {results.length === 0 ? (
        <div className="no-results">{t('knowledgeEditor.faq.noResults')}</div>
      ) : (
        <div className="results-list">
          {results.map((result, index) => {
            const expanded = expandedIds.has(result.id);
            return (
              <div key={result.id} className={'result-card' + (expanded ? ' expanded' : '')}>
                <button type="button" className="result-header" aria-expanded={expanded} onClick={() => onToggle(result.id)}>
                  <span className="result-main">
                    <span className="result-question"><span className="result-index">{index + 1}.</span> {result.standard_question}</span>
                    {result.matched_question && result.matched_question !== result.standard_question ? (
                      <span className="matched-question">
                        <span className="matched-label">{t('knowledgeEditor.faq.matchedQuestion')}:</span>
                        <span className="matched-text">{result.matched_question}</span>
                      </span>
                    ) : null}
                  </span>
                  <span className="score-tag">{(result.score || 0).toFixed(3)}</span>
                  <Icon size={14} className="expand-icon"><path d={expanded ? Chevrons.up : Chevrons.down} /></Icon>
                </button>
                {expanded ? (
                  <div className="result-body">
                    {result.answers?.length ? (
                      <div className="result-section">
                        <div className="section-label">{t('knowledgeEditor.faq.answers')}</div>
                        <div className="result-tags">
                          {/* B5: Vue search rows use t-tooltip (:829-833). */}
                          {result.answers.map((answer, answerIndex) => <FaqTagTooltip key={answerIndex} content={answer} type="answer" placement="top"><span className="question-tag is-answer">{answer}</span></FaqTagTooltip>)}
                        </div>
                      </div>
                    ) : null}
                    {result.similar_questions?.length ? (
                      <div className="result-section">
                        <div className="section-label">{t('knowledgeEditor.faq.similarQuestions')}</div>
                        <div className="result-tags">
                          {result.similar_questions.map((question, questionIndex) => <FaqTagTooltip key={questionIndex} content={question} type="similar" placement="top"><span className="question-tag">{question}</span></FaqTagTooltip>)}
                        </div>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// --- Container: data wiring (unchanged API usage, new presentation) ----------------

function FAQTagManageDialog({ client, knowledgeBaseId, tags, open, onClose, onChanged, t }: {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  tags: readonly KnowledgeTag[];
  open: boolean;
  onClose: () => void;
  onChanged: () => Promise<void>;
  t: Translate;
}) {
  const [query, setQuery] = useState('');
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const visible = tags.filter((tag) => !query.trim() || tag.name.toLowerCase().includes(query.trim().toLowerCase()));

  useEffect(() => {
    if (!open) return;
    setQuery(''); setCreating(false); setDraft(''); setEditingId(null); setEditingName(''); setError('');
  }, [open]);

  async function createTag() {
    const name = draft.trim();
    if (!name || busy) return;
    setBusy(true); setError('');
    try { await client.knowledge.documents.createTag(knowledgeBaseId, { name }); setCreating(false); setDraft(''); await onChanged(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('common.operationFailed')); }
    finally { setBusy(false); }
  }
  async function updateTag() {
    if (!editingId || !editingName.trim() || busy) return;
    setBusy(true); setError('');
    try { await client.knowledge.documents.updateTag(knowledgeBaseId, editingId, { name: editingName.trim() }); setEditingId(null); await onChanged(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('common.operationFailed')); }
    finally { setBusy(false); }
  }
  async function removeTag(tag: KnowledgeTag) {
    const seqId = tag.seq_id;
    if (!Number.isSafeInteger(seqId) || busy || !window.confirm(t('knowledgeBase.tagDeleteDesc', { name: tag.name }))) return;
    setBusy(true); setError('');
    try { await client.knowledge.documents.deleteTag(knowledgeBaseId, seqId!); await onChanged(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('common.operationFailed')); }
    finally { setBusy(false); }
  }

  return <Dialog open={open} title={t('knowledgeBase.tagManageTitle')} onClose={onClose}>
    <p className="wk-muted">{t('knowledgeBase.tagManageDescription')}</p>
    {error ? <Status tone="error">{error}</Status> : null}
    <div className="faq-tag-manage-toolbar"><input value={query} placeholder={t('knowledgeBase.tagSearchPlaceholder')} onChange={(event) => setQuery(event.target.value)} /><Button type="button" disabled={busy} onClick={() => { setCreating(true); setEditingId(null); }}>{t('knowledgeBase.tagCreateAction')}</Button></div>
    {creating ? <div className="faq-tag-manage-edit"><input autoFocus maxLength={40} value={draft} placeholder={t('knowledgeBase.tagNamePlaceholder')} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void createTag(); if (event.key === 'Escape') setCreating(false); }} /><Button type="button" loading={busy} onClick={() => void createTag()}>{t('common.create')}</Button><Button type="button" disabled={busy} onClick={() => setCreating(false)}>{t('common.cancel')}</Button></div> : null}
    <ul className="faq-tag-manage-list">
      {visible.map((tag) => editingId === tag.id ? <li key={tag.id} className="faq-tag-manage-row"><input autoFocus maxLength={40} value={editingName} onChange={(event) => setEditingName(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void updateTag(); if (event.key === 'Escape') setEditingId(null); }} /><Button type="button" loading={busy} onClick={() => void updateTag()}>{t('common.save')}</Button><Button type="button" disabled={busy} onClick={() => setEditingId(null)}>{t('common.cancel')}</Button></li> : <li key={tag.id} className="faq-tag-manage-row"><span><strong>{tag.name}</strong><small>{t('knowledgeBase.tagManageFaqCount', { count: tag.chunk_count || 0 })}</small></span><Button type="button" disabled={busy} onClick={() => { setEditingId(tag.id); setEditingName(tag.name); setCreating(false); }}>{t('knowledgeBase.tagEditAction')}</Button><Button type="button" disabled={busy || !Number.isSafeInteger(tag.seq_id)} onClick={() => void removeTag(tag)}>{t('knowledgeBase.tagDeleteAction')}</Button></li>)}
      {visible.length === 0 ? <li><Status>{t('knowledgeBase.tagEmptyResult')}</Status></li> : null}
    </ul>
  </Dialog>;
}

// Vue openEditor (FAQEntryManager.vue:1684-1702) — copies the entry's lists and
// clears the input drafts; create starts from the empty form.
function formFrom(entry: FAQEntry | null): FormState {
  return entry ? {
    question: entry.standard_question,
    similarQuestions: [...(entry.similar_questions || [])],
    negativeQuestions: [...(entry.negative_questions || [])],
    answers: [...(entry.answers || [])],
    tagId: typeof entry.tag_id === 'number' ? String(entry.tag_id) : '',
    enabled: entry.is_enabled,
    recommended: entry.is_recommended,
    similarDraft: '',
    negativeDraft: '',
    answerDraft: '',
  } : { ...emptyForm, similarQuestions: [], negativeQuestions: [], answers: [] };
}
function payloadFrom(form: FormState): FAQEntryPayload {
  return normalizeFAQPayload({ standard_question: form.question, similar_questions: form.similarQuestions, negative_questions: form.negativeQuestions, answers: form.answers, tag_id: form.tagId.trim() ? Number(form.tagId) : null, is_enabled: form.enabled, is_recommended: form.recommended });
}
function downloadText(text: string, format: 'csv' | 'json') {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([text], { type: format === 'json' ? 'application/json' : 'text/csv' }));
  link.download = `faq-export.${format}`;
  link.click();
  URL.revokeObjectURL(link.href);
}
function metaFromKB(kb: KnowledgeBase | null): FAQKBMeta | undefined {
  if (!kb) return undefined;
  return {
    type: typeof kb.type === 'string' ? kb.type : undefined,
    description: typeof kb.description === 'string' ? kb.description : undefined,
    createdAt: typeof kb.created_at === 'string' ? kb.created_at.slice(0, 10) : undefined,
  };
}

export function FAQPage({ client, knowledgeBaseId }: { client: WeKnoraClient; knowledgeBaseId: string }) {
  const locale = useAppLocale();
  const t = createFaqTranslator(locale);
  const faq = client.knowledge.faq;
  const [kb, setKb] = useState<KnowledgeBase | null>(null);
  const [kbList, setKbList] = useState<KBListItem[]>([]);
  const [tags, setTags] = useState<KnowledgeTag[]>([]);
  const [entries, setEntries] = useState<FAQEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [editing, setEditing] = useState<FAQEntry | null | undefined>(undefined);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [keywordDraft, setKeywordDraft] = useState('');
  const [keyword, setKeyword] = useState('');
  const [activeTagIds, setActiveTagIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [exportLoading, setExportLoading] = useState(false);
  const [importBusy, setImportBusy] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [importMode, setImportMode] = useState<'append' | 'replace'>('append');
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importPreview, setImportPreview] = useState<FAQEntryPayload[]>([]);
  const [statusUpdatingIds, setStatusUpdatingIds] = useState<readonly number[]>([]);
  const [batchTag, setBatchTag] = useState('');
  const [tagManageOpen, setTagManageOpen] = useState(false);
  // B4: Vue search test state (FAQEntryManager.vue:1329-1338).
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchForm, setSearchForm] = useState<FAQSearchFormState>(faqSearchDefaultForm());
  const [searching, setSearching] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const [searchResults, setSearchResults] = useState<FAQSearchHit[]>([]);
  // Vue FAQEntryManager.vue:2091-2165 — after upsert returns a task_id the
  // page polls importProgress until completed/failed; success refreshes the
  // list and collapses the strip after 3s; 404 stops the polling.
  const [importTask, setImportTask] = useState<FAQImportProgress | null>(null);
  // B6: Vue importResult (FAQEntryManager.vue:1263) — the last completed import
  // result, restored from localStorage on mount and refreshed after each import.
  const [lastResult, setLastResult] = useState<FAQImportResultView | null>(null);
  const loadLastResult = useCallback(async (kbId: string) => {
    const taskId = getLastCompletedTaskId(kbId);
    if (!taskId) { setLastResult(null); return; }
    try {
      // GET /api/v1/faq/import/progress/{task_id} (Vue loadImportResult :2299-2342);
      // raw request keeps the server display_status the typed client drops.
      const body = await client.request({ method: 'GET', path: `/api/v1/faq/import/progress/${encodeURIComponent(taskId)}` }) as { success?: unknown; data?: unknown } | null;
      const data = body && body.success === true ? body.data : null;
      setLastResult(faqImportResultFromProgress(data));
    } catch {
      setLastResult(null);
    }
  }, [client]);
  // Vue onMounted → restoreImportTask + loadImportResult (:2825-2829).
  useEffect(() => { void loadLastResult(knowledgeBaseId); }, [loadLastResult, knowledgeBaseId]);
  useEffect(() => {
    if (!importTask || (importTask.status !== 'processing' && importTask.status !== 'pending')) return;
    const timer = setInterval(() => {
      faq.importProgress(importTask.task_id).then((next) => {
        setImportTask((current) => (current ? { ...current, ...next } : next));
        if (next.status === 'completed') {
          void load(false);
          // B6: Vue :2131-2134/:2144 — persist the completed task id and load
          // the fresh result so the strip survives reloads until closed.
          saveLastCompletedTaskId(knowledgeBaseId, next.task_id || importTask.task_id);
          void loadLastResult(knowledgeBaseId);
        }
      }).catch(() => { setImportTask(null); });
    }, 1500);
    return () => clearInterval(timer);
  }, [importTask, faq, knowledgeBaseId, loadLastResult]);
  useEffect(() => {
    if (importTask?.status !== 'completed') return;
    const timer = setTimeout(() => setImportTask(null), 3000);
    return () => clearTimeout(timer);
  }, [importTask?.status]);
  const [canContribute, setCanContribute] = useState(true);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const navigate = useCallback((path: string) => { window.location.assign(path); }, []);
  // B6: Vue closeImportResult (:2345-2356) — persist 'close' server-side, then
  // hide locally; on failure the strip stays (Vue only logs).
  const closeImportResult = useCallback(async () => {
    try {
      await client.request({ method: 'PUT', path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/faq/import/last-result/display`, body: { display_status: 'close' } });
      setLastResult((current) => (current ? { ...current, display_status: 'close' } : current));
    } catch (error) { console.error('Failed to close import result:', error); }
  }, [client, knowledgeBaseId]);
  // B6: Vue downloadFailedEntries (:2359-2366).
  const downloadFailedEntries = useCallback(() => {
    if (!lastResult?.failed_entries_url) { setMessage({ tone: 'warning', text: t('FAQ.import.noFailedRecords') }); return; }
    window.open(lastResult.failed_entries_url, '_blank');
  }, [lastResult, t]);

  // Page receives knowledgeBaseId only — fetch the KB record, the KB list (crumb
  // switcher), tags (filter) and the caller to gate viewer accounts.
  useEffect(() => {
    let active = true;
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.knowledgeBases.list().catch(() => [] as KnowledgeBase[]),
      client.knowledge.documents.tags(knowledgeBaseId, { page_size: 200 }).catch(() => [] as KnowledgeTag[]),
      client.auth.me().catch(() => null),
    ]).then(([kbRow, list, tagRows, me]) => {
      if (!active) return;
      setKb(kbRow);
      setKbList(list.map((item) => ({ id: String(item.id), name: item.name, type: typeof item.type === 'string' ? item.type : undefined })));
      setTags(tagRows);
      setCanContribute(computeKBPermissions(kbRow as KBSurfaceKB, me as KBSurfaceMe | null).canContribute);
    }).catch(() => {});
    return () => { active = false; };
  }, [client, knowledgeBaseId]);

  // Scroll-append guard shared with the sync loadMore callback.
  const loadingMoreRef = useRef(false);
  // Vue loadEntries(append): page 1 resets the list; appends accumulate and
  // hasMore follows entries.length < total (FAQEntryManager.vue:1554-1614).
  async function load(append = false) {
    if (append) setLoadingMore(true);
    else {
      loadingMoreRef.current = true;
      setLoading(true);
      setEntries([]);
      setSelected(new Set());
    }
    try {
      const nextPage = append ? Math.floor(entries.length / PAGE_SIZE) + 1 : 1;
      const result = await faq.list(knowledgeBaseId, { page: nextPage, page_size: PAGE_SIZE, keyword: keyword || undefined, tag_ids: activeTagIds.length ? activeTagIds.join(',') : undefined });
      const loaded = append ? entries.length + result.data.length : result.data.length;
      setEntries((current) => (append ? [...current, ...result.data] : result.data));
      setTotal(result.total ?? 0);
      setHasMore(faqHasMore(loaded, result.total ?? 0));
      if (!append) setSelected(new Set());
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to load FAQ entries' }); }
    finally {
      loadingMoreRef.current = false;
      setLoading(false);
      setLoadingMore(false);
    }
  }
  const loadMore = useCallback(() => {
    if (loadingMoreRef.current || loading || !hasMore) return;
    void load(true);
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [client, knowledgeBaseId, keyword, activeTagIds, entries.length, loading, hasMore]);
  useEffect(() => { void load(false); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [client, knowledgeBaseId, keyword, activeTagIds]);

  // Vue handleEntryStatusChange (FAQEntryManager.vue:1487): optimistic flip via
  // the entries/fields batch, per-direction success copy, rollback on failure.
  async function toggleEntryStatus(entry: FAQEntry, value: boolean) {
    if (!knowledgeBaseId || statusUpdatingIds.includes(entry.id) || entry.is_enabled === value) return;
    const previous = entry.is_enabled;
    setStatusUpdatingIds((current) => [...current, entry.id]);
    setEntries((current) => setEntryStatus(current, entry.id, value));
    try {
      await faq.updateFields(knowledgeBaseId, { by_id: { [entry.id]: { is_enabled: value } } });
      setMessage({ tone: 'success', text: t(value ? 'knowledgeEditor.faq.statusEnableSuccess' : 'knowledgeEditor.faq.statusDisableSuccess') });
    } catch (error) {
      setEntries((current) => setEntryStatus(current, entry.id, previous));
      setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('knowledgeEditor.faq.statusUpdateFailed') });
    } finally {
      setStatusUpdatingIds((current) => current.filter((id) => id !== entry.id));
    }
  }
  function openEditor(entry: FAQEntry | null = null) { setEditing(entry); setForm(formFrom(entry)); setMessage(null); }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    // Vue editorRules (FAQEntryManager.vue:1542-1552) validate before any request;
    // success copy reuses the shared message keys (:1771,1774).
    const invalid = editorFormError(form);
    if (invalid) {
      setMessage({ tone: 'error', text: t(invalid === 'question' ? 'knowledgeEditor.messages.nameRequired' : 'knowledgeEditor.faq.answerRequired') });
      return;
    }
    setSaving(true); setMessage(null);
    try {
      const payload = payloadFrom(form);
      if (editing) await faq.update(knowledgeBaseId, editing.id, payload); else await faq.create(knowledgeBaseId, payload);
      setMessage({ tone: 'success', text: t(faqSaveResultKey(Boolean(editing))) });
      setEditing(undefined);
      await load(false);
    }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to save FAQ entry' }); }
    finally { setSaving(false); }
  }
  async function updateSelection(input: FAQEntryFieldsUpdate) {
    if (!selected.size) return;
    try { await faq.updateFields(knowledgeBaseId, { by_id: Object.fromEntries([...selected].map((id) => [id, input])) }); await load(false); setMessage({ tone: 'success', text: 'Selected FAQ entries updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected entries' }); }
  }
  async function updateSelectedTag() {
    if (!selected.size) return;
    const tagId = batchTag.trim() ? Number(batchTag) : null;
    if (tagId !== null && (!Number.isSafeInteger(tagId) || tagId < 0)) { setMessage({ tone: 'error', text: 'Tag ID must be a non-negative integer.' }); return; }
    try { await faq.updateTags(knowledgeBaseId, { updates: Object.fromEntries([...selected].map((id) => [id, tagId])) }); await load(false); setBatchTag(''); setMessage({ tone: 'success', text: 'Selected FAQ tags updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected tags' }); }
  }
  async function removeMany(ids: number[]) {
    try { await faq.removeMany(knowledgeBaseId, ids); await load(false); setMessage({ tone: 'success', text: 'Selected FAQ entries deleted.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to delete FAQ entries' }); }
  }
  // Vue processFile (FAQEntryManager.vue:1900): parse immediately, surface the
  // row count as an in-dialog preview; Excel goes through parseExcelFile
  // (FAQEntryManager.vue:1999) on the vendored xlsx-0.20.2 build.
  function handleImportFile(file: File) {
    setImportFile(file);
    setImportPreview([]);
    const format = importFormatFromName(file.name);
    if (format === 'excel') {
      void parseExcelFile(file)
        .then((rows) => setImportPreview(rows))
        .catch(() => { setMessage({ tone: 'error', text: t('knowledgeEditor.faqImport.parseFailed') }); setImportPreview([]); });
      return;
    }
    void file.text().then((text) => {
      try { setImportPreview(parseFAQImportText(text, format)); }
      catch { setMessage({ tone: 'error', text: t('knowledgeEditor.faqImport.parseFailed') }); setImportPreview([]); }
    });
  }
  async function confirmImport() {
    if (!importFile) return;
    setImportBusy(true); setMessage(null);
    try {
      const format = importFormatFromName(importFile.name);
      // Vue parseExcelFile (FAQEntryManager.vue:1999) — binary parse for .xlsx/.xls.
      const imported = format === 'excel' ? await parseExcelFile(importFile) : parseFAQImportText(await importFile.text(), format);
      const result = await faq.upsert(knowledgeBaseId, { entries: imported, mode: importMode });
      setImportOpen(false); setImportFile(null); setImportPreview([]);
      // Vue FAQEntryManager.vue:2091-2165 — the strip replaces the message
      // and polls the backend task until completion.
      setImportTask({ task_id: result.task_id, kb_id: knowledgeBaseId, status: 'processing', progress: 0, processed: 0, total: imported.length });
      await load(false);
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to import FAQ entries' }); }
    finally { setImportBusy(false); }
  }
  async function exportEntries(format: 'csv' | 'json') {
    setExportLoading(true);
    try { downloadText(await faq.exportEntries(knowledgeBaseId, format), format); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to export FAQ entries' }); }
    finally { setExportLoading(false); }
  }
  // Vue handleSearch (FAQEntryManager.vue:2650-2680): blank query warns without a
  // request; a failure surfaces error?.message || common.operationFailed and clears
  // the previous hits.
  async function runSearchTest() {
    if (faqSearchBlocked(searchForm)) {
      setMessage({ tone: 'warning', text: t('knowledgeEditor.faq.queryPlaceholder') });
      return;
    }
    setSearching(true);
    setHasSearched(true);
    try {
      const raw = await faq.search(knowledgeBaseId, faqSearchRequestFrom(searchForm));
      setSearchResults(faqSearchResultsFromResponse(raw));
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.operationFailed') });
      setSearchResults([]);
    } finally {
      setSearching(false);
    }
  }

  async function reloadAfterTagChange() {
    const nextTags = await client.knowledge.documents.tags(knowledgeBaseId, { page_size: 200 });
    setTags(nextTags);
    setActiveTagIds((current) => current.filter((id) => nextTags.some((tag) => tag.id === id)));
    await load(false);
  }

  return (
    <>
      <FAQPageView
      t={t}
      knowledgeBaseId={knowledgeBaseId}
      kbName={kb?.name ?? null}
      kbMeta={metaFromKB(kb)}
      kbList={kbList}
      tags={tags}
      activeTagIds={activeTagIds}
      entries={entries}
      total={total}
      hasMore={hasMore}
      loadingMore={loadingMore}
      loading={loading}
      canContribute={canContribute}
      selected={selected}
      keywordDraft={keywordDraft}
      importOpen={importOpen}
      importMode={importMode}
      importFileName={importFile?.name ?? null}
      importBusy={importBusy}
      importPreview={importPreview}
      importTask={importTask ? faqImportTaskView(importTask) : null}
      importResult={lastResult}
      onCloseImportResult={() => void closeImportResult()}
      onDownloadFailedEntries={downloadFailedEntries}
      editorOpen={editing !== undefined}
      editorTitle={editing ? t('knowledgeEditor.faq.editorEdit') : t('knowledgeEditor.faq.editorCreate')}
      editorMode={editing ? 'edit' : 'create'}
      form={form}
      saving={saving}
      exportLoading={exportLoading}
      message={message}
      batchTag={batchTag}
      onNavigate={navigate}
      onKeywordDraftChange={setKeywordDraft}
      onSearchSubmit={() => setKeyword(keywordDraft.trim())}
      onSearchClear={() => { setKeywordDraft(''); setKeyword(''); }}
      onToggleTag={(tagId) => setActiveTagIds((current) => current.includes(tagId) ? current.filter((id) => id !== tagId) : [...current, tagId])}
      onOpenTagManage={() => setTagManageOpen(true)}
      onOpenCreate={() => openEditor()}
      onOpenImport={() => { setImportFile(null); setImportPreview([]); setImportOpen(true); }}
      onCloseImport={() => setImportOpen(false)}
      onImportModeChange={setImportMode}
      onImportFile={handleImportFile}
      onImportConfirm={() => void confirmImport()}
      onExport={(format) => void exportEntries(format)}
      onToggleSelect={(id, checked) => setSelected((current) => { const next = new Set(current); if (checked) next.add(id); else next.delete(id); return next; })}
      onToggleSelectAll={(checked) => setSelected(checked ? new Set(entries.map((entry) => entry.id)) : new Set())}
      onEditEntry={openEditor}
      onDeleteEntry={(entry) => void removeMany([entry.id])}
      onToggleEntryStatus={(entry, value) => void toggleEntryStatus(entry, value)}
      statusUpdatingIds={statusUpdatingIds}
      onBatchEnable={() => void updateSelection({ is_enabled: true })}
      onBatchDisable={() => void updateSelection({ is_enabled: false })}
      onBatchRecommend={() => void updateSelection({ is_recommended: true })}
      onBatchTagChange={setBatchTag}
      onBatchSetTag={() => void updateSelectedTag()}
      onBatchDelete={() => void removeMany([...selected])}
      onLoadMore={loadMore}
      onCloseEditor={() => setEditing(undefined)}
      onFormChange={(patch) => setForm((current) => ({ ...current, ...patch }))}
      onEditorSubmit={(event) => void save(event)}
      searchOpen={searchOpen}
      searchForm={searchForm}
      searching={searching}
      hasSearched={hasSearched}
      searchResults={searchResults}
      onOpenSearchTest={() => setSearchOpen(true)}
      onCloseSearchTest={() => setSearchOpen(false)}
      onSearchFormChange={(patch) => setSearchForm((current) => ({ ...current, ...patch }))}
      onSearchTestSubmit={() => void runSearchTest()}
      />
      <FAQTagManageDialog client={client} knowledgeBaseId={knowledgeBaseId} tags={tags} open={tagManageOpen} onClose={() => setTagManageOpen(false)} onChanged={reloadAfterTagChange} t={t} />
    </>
  );
}
