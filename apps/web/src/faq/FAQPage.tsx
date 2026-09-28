import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { DragEvent, ReactNode } from 'react';
import { createPortal } from 'react-dom';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, FAQImportProgress, KnowledgeBase, KnowledgeTag, WeKnoraClient } from '@weknora/api-client';
import * as XLSX from 'xlsx';
import {
  Button,
  Dialog,
  MessagePlugin,
  Drawer,
  Dropdown,
  Input as TdInput,
  Loading as TdLoading,
  Popconfirm,
  Popup,
  Radio,
  RadioGroup,
  Select as TdSelect,
  Skeleton as TdSkeleton,
  Slider,
  Switch as TdSwitch,
  Tag as TdTag,
  Textarea as TdTextarea,
  Tooltip,
} from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { navigate as clientNavigate } from '../platform/navigation.ts';
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from '../knowledge/permissions.ts';
import { KBInfoPopover, type KBInfoPopoverKB } from '../documents/KBInfoPopover.tsx';
import { findSharedKBGrant } from '../wiki/edit-permission.ts';
import { normalizeFAQPayload, parseExcelFile, parseFAQImportText, serializeFAQEntries } from './import-export.ts';
import './faq.td.css';

// FAQ knowledge-base page — TDesign 平移（Vue 事实源
// frontend/src/views/knowledge/components/FAQEntryManager.vue 及其子组件
// FAQBatchBar.vue / FAQTagTooltip.vue / KBSwitcherDropdown.vue / KBInfoPopover.vue）。
// DOM/类名/样式值逐项复刻；容器根 .faq-manager-wrapper 来自 KnowledgeBase.vue:3883。

type Translate = ReturnType<typeof createTranslator>;

/** Vue FAQEntryManager.loadEntries: hasMore = entries.length < total. */
export function faqHasMore(loaded: number, total: number): boolean {
  return Number.isFinite(total) && total > 0 && loaded < Math.floor(total);
}

/** Vue tagSearchQuery filters the loaded sidebar tags by name. */
export function filterFaqTags<T extends { name: string }>(tags: readonly T[], query: string): T[] {
  const normalized = query.trim().toLocaleLowerCase();
  return normalized ? tags.filter((tag) => tag.name.toLocaleLowerCase().includes(normalized)) : [...tags];
}

/** Vue arrangeCards breakpoints (FAQEntryManager.vue:2721-2729). */
export function faqMasonryColumnCount(containerWidth: number): number {
  if (containerWidth >= 2560) return 12;
  if (containerWidth >= 1920) return 10;
  if (containerWidth >= 1536) return 8;
  if (containerWidth >= 1280) return 6;
  if (containerWidth >= 1024) return 5;
  if (containerWidth >= 768) return 4;
  if (containerWidth >= 640) return 3;
  return 1;
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

/** Vue handleImport refuses an absent file or an empty parsed preview. */
export function faqImportBlocked(fileName: string | null | undefined, previewLength: number): boolean {
  return !fileName || previewLength <= 0;
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

export function faqBatchSuccessKey(input: FAQEntryFieldsUpdate):
  | 'knowledgeEditor.faq.statusEnableSuccess'
  | 'knowledgeEditor.faq.statusDisableSuccess'
  | 'knowledgeEditor.faq.recommendedEnabled'
  | 'knowledgeBase.tagUpdateSuccess' {
  if (input.is_enabled === true) return 'knowledgeEditor.faq.statusEnableSuccess';
  if (input.is_enabled === false) return 'knowledgeEditor.faq.statusDisableSuccess';
  if (input.is_recommended === true) return 'knowledgeEditor.faq.recommendedEnabled';
  return 'knowledgeBase.tagUpdateSuccess';
}

export function faqDeleteSuccessKey(count: number): 'knowledgeEditor.faqImport.deleteSuccess' | 'knowledgeEditor.faq.batchDeleteSuccess' {
  return count > 1 ? 'knowledgeEditor.faq.batchDeleteSuccess' : 'knowledgeEditor.faqImport.deleteSuccess';
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

/** Vue FAQTagTooltip.vue — wrapper + body-teleported fixed bubble（类名/几何逐项平移，
 *  定位/翻转逻辑同 Vue updatePosition）。 */
export function FaqTagTooltip({ content, type = 'answer', placement = 'top', children }: FaqTagTooltipProps) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<{ top: number; left: number; placement: FaqTooltipPlacement } | null>(null);
  const wrapperRef = useRef<HTMLDivElement | null>(null);
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
    <div
      ref={wrapperRef}
      className="faq-tag-wrapper"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      {children}
      {open ? createPortal(
        <div
          ref={bubbleRef}
          className={`faq-tag-tooltip tooltip-${type} placement-${resolvedPlacement}`}
          style={{ top: (position?.top ?? 0) + 'px', left: (position?.left ?? 0) + 'px' }}
        >
          <div className="tooltip-content">{content}</div>
        </div>,
        document.body,
      ) : null}
    </div>
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
  'zh-CN': { 'common.operationFailed': '操作失败', 'common.close': '关闭', 'common.clear': '清除' },
  'en-US': { 'common.operationFailed': 'Operation failed', 'common.close': 'Close', 'common.clear': 'Clear' },
  'ja-JP': { 'common.operationFailed': '操作に失敗しました', 'common.close': '閉じる', 'common.clear': 'クリア' },
  'ko-KR': { 'common.operationFailed': '작업 실패', 'common.close': '닫기', 'common.clear': '지우기' },
  'ru-RU': { 'common.operationFailed': 'Операция не выполнена', 'common.close': 'Закрыть', 'common.clear': 'Очистить' },
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

function defaultNavigate(path: string): void { clientNavigate(path); }

/** Vue @/assets/img/more.png 内联副本（documents 域同款，卡片三点菜单触发器）。 */
const MORE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgBAMAAACBVGfHAAAAD1BMVEUAAAAwMTMwMDMwMjIwMTPbLw9bAAAABHRSTlMA3llYOk1BewAAABxJREFUKM9jGGnAUAiJAAERRwSBXUBRCIkYYQAAnNMDYY7Uun8AAAAASUVORK5CYII=';

// --- Breadcrumb + kb-title-actions（Vue :5-97，KBSwitcherDropdown/KBInfoPopover 平移） ---

export interface FAQBreadcrumbProps {
  t?: Translate;
  knowledgeBaseId?: string;
  kbName?: string | null;
  kbList?: KBListItem[];
  kbMeta?: FAQKBMeta;
  /** KBInfoPopover 完整数据源（B2 批 2）：优先于 kbMeta——六段 section 全量渲染。 */
  kbInfo?: KBInfoPopoverKB | null;
  /** Vue authStore.user?.id（owner 判定）。 */
  infoUserId?: string;
  /** Vue orgStore currentSharedKb（来自 org_name/shared_at 行）。 */
  infoSharedKb?: { orgName: string; sharedAt: string } | null;
  /** Vue effectiveKBPermission（getKBPermission || my_permission）。 */
  infoPermission?: string;
  canManage?: boolean;
  /** 导入结果条（Vue showImportResultBadge 分支）。 */
  importResult?: FAQImportResultView | null;
  importResultExpanded?: boolean;
  onToggleImportResultExpanded?: () => void;
  onCloseImportResult?: () => void;
  onDownloadFailedEntries?: () => void;
  /** 导入进行中条（Vue isImportInProgress && taskStatus 分支）。 */
  importTask?: FAQImportTaskView | null;
  onNavigate?: (path: string) => void;
  onOpenKBSettings?: () => void;
}

// Vue KBSwitcherDropdown：当前 KB 置顶，其余保持调用方顺序。
function sortKbListForSwitcher(kbList: readonly KBListItem[], currentKbId: string): KBListItem[] {
  const current = kbList.find((kb) => kb.id === currentKbId);
  if (!current) return [...kbList];
  return [current, ...kbList.filter((kb) => kb.id !== currentKbId)];
}

export function FAQBreadcrumb(props: FAQBreadcrumbProps = {}) {
  const {
    t: tr,
    knowledgeBaseId = '',
    kbName = null,
    kbList = [],
    kbMeta,
    kbInfo = null,
    infoUserId,
    infoSharedKb = null,
    infoPermission,
    canManage = false,
    importResult = null,
    importResultExpanded = false,
    onToggleImportResultExpanded = () => {},
    onCloseImportResult = () => {},
    onDownloadFailedEntries = () => {},
    importTask = null,
    onNavigate = defaultNavigate,
    onOpenKBSettings,
  } = props;
  const t = tr ?? createTranslator('zh-CN');
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const sortedKbList = sortKbListForSwitcher(kbList, knowledgeBaseId);
  const showImportResultBadge = faqImportResultVisible(importResult, Boolean(importTask));
  return (
    <div className="faq-title-row">
      <h2 className="faq-breadcrumb">
        <button type="button" className="breadcrumb-link" onClick={() => onNavigate(faqKBListPath)}>{t('menu.knowledgeBase')}</button>
        <TIcon name="chevron-right" className="breadcrumb-separator" />
        {kbList.length ? (
          <Popup
            visible={switcherOpen}
            trigger="click"
            placement="bottom-left"
            overlayStyle={{ padding: 0 }}
            overlayInnerStyle={{ padding: 0 }}
            onVisibleChange={setSwitcherOpen}
            content={(
              <div className="kb-switcher-card">
                <div className="kb-switcher-list">
                  {sortedKbList.map((kb) => (
                    <button
                      key={kb.id}
                      type="button"
                      className={'kb-switcher-row' + (kb.id === knowledgeBaseId ? ' active' : '')}
                      onClick={() => { setSwitcherOpen(false); if (kb.id !== knowledgeBaseId) onNavigate(faqKBDetailPath(kb.id)); }}
                    >
                      <TIcon name={kb.type === 'faq' ? 'chat-bubble-help' : 'folder'} size="16px" className="kb-switcher-row-icon" />
                      <span className="kb-switcher-row-name" title={kb.name}>{kb.name}</span>
                      {kb.id === knowledgeBaseId ? <TIcon name="check" size="14px" className="kb-switcher-row-check" /> : null}
                    </button>
                  ))}
                  {!sortedKbList.length ? <div className="kb-switcher-empty">{t('common.noData')}</div> : null}
                </div>
              </div>
            )}
          >
            <button type="button" className="breadcrumb-link dropdown" disabled={!knowledgeBaseId}>
              {kbName == null
                ? <TdSkeleton animation="gradient" rowCol={[{ width: '120px', height: '20px' }]} />
                : <><span>{kbName}</span><TIcon name="chevron-down" /></>}
            </button>
          </Popup>
        ) : (
          <button type="button" className="breadcrumb-link" disabled={!knowledgeBaseId} onClick={() => onNavigate(faqKBDetailPath(knowledgeBaseId))}>
            {kbName == null
              ? <TdSkeleton animation="gradient" rowCol={[{ width: '120px', height: '20px' }]} />
              : kbName}
          </button>
        )}
        <TIcon name="chevron-right" className="breadcrumb-separator" />
        <span className="breadcrumb-current">{t('knowledgeEditor.faq.title')}</span>
      </h2>
      <div className="kb-title-actions">
        {(kbInfo ?? kbMeta) ? (
          <KBInfoPopover
            t={t}
            kbInfo={(kbInfo ?? kbMeta) as KBInfoPopoverKB}
            userId={infoUserId}
            sharedKb={infoSharedKb}
            permission={infoPermission}
          />
        ) : null}
        {canManage ? (
          <Tooltip content={t('knowledgeBase.settings')} placement="top">
            <button type="button" className="kb-settings-button" aria-label={t('knowledgeBase.settings')} title={t('knowledgeBase.settings')} disabled={!knowledgeBaseId} onClick={() => { if (!knowledgeBaseId) return; if (onOpenKBSettings) onOpenKBSettings(); else onNavigate(faqKBSettingsPath(knowledgeBaseId)); }}>
              <TIcon name="setting" size="16px" />
            </button>
          </Tooltip>
        ) : null}
        {showImportResultBadge && importResult ? (
          <div className={'faq-import-host' + (importResultExpanded ? ' is-expanded' : '')}>
            <button
              type="button"
              className="faq-import-trigger"
              aria-label={t('FAQ.import.totalData')}
              onClick={(event) => { event.stopPropagation(); onToggleImportResultExpanded(); }}
            >
              <TIcon name="check-circle-filled" size="16px" />
            </button>
            <div className="faq-import-panel">
              <div className="faq-import-strip faq-import-strip--result faq-import-strip--panel">
                <span className="faq-import-strip__text">{faqImportResultSummary(importResult, t)}</span>
                <TdTag size="small" variant="light" theme={importResult.import_mode === 'append' ? 'primary' : 'warning'}>
                  {importResult.import_mode === 'append' ? t('FAQ.import.appendMode') : t('FAQ.import.replaceMode')}
                </TdTag>
                {importResult.failed_entries_url && importResult.failed_count > 0 ? (
                  <Button variant="text" theme="danger" size="small" className="faq-import-strip__link" onClick={() => onDownloadFailedEntries()}>
                    {t('FAQ.import.downloadReasons')}
                  </Button>
                ) : null}
                <span className="faq-import-strip__time">{formatImportTime(importResult.imported_at)}</span>
                <button type="button" className="faq-import-strip__close" aria-label={t('common.close')} onClick={() => onCloseImportResult()}>
                  <TIcon name="close" size="14px" />
                </button>
              </div>
            </div>
          </div>
        ) : importTask ? (
          <div className={'faq-import-strip faq-import-strip--in-title faq-import-strip--' + importTask.status}>
            <TIcon
              name={importTask.status === 'running' ? 'loading' : importTask.status === 'success' ? 'check-circle-filled' : importTask.status === 'failed' ? 'error-circle-filled' : 'time-filled'}
              size="16px"
              className={'faq-import-strip__icon' + (importTask.status === 'running' ? ' is-spinning' : '')}
            />
            <span className="faq-import-strip__text">{importTask.text}</span>
            <div className="faq-import-strip__bar">
              <div className="faq-import-strip__bar-fill" style={{ width: `${importTask.progress}%` }} />
            </div>
            <span className="faq-import-strip__count">{importTask.processed}/{importTask.total}</span>
          </div>
        ) : null}
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
  /** KBInfoPopover 完整数据源（B2 批 2）。 */
  kbInfo?: KBInfoPopoverKB | null;
  infoUserId?: string;
  infoSharedKb?: { orgName: string; sharedAt: string } | null;
  infoPermission?: string;
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
  /** Vue canManage（三点菜单/批量删除门槛）；默认跟随 canContribute。 */
  canManage?: boolean;
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
  /** R491 1b: Vue FAQBatchBar cancel — clears the whole selection. */
  onClearSelection?: () => void;
  /** R491 1c: Vue batch-tag overlay state (FAQEntryManager.vue:685-735). */
  batchTagOpen?: boolean;
  batchTagValue?: string;
  batchTagBusy?: boolean;
  onOpenBatchTag?: () => void;
  onBatchTagValueChange?: (value: string) => void;
  onBatchTagConfirm?: () => void;
  onCloseBatchTag?: () => void;
  /** Vue batch bar per-action loading（tagLoading/statusAction/deleteLoading）。 */
  batchTagLoading?: boolean;
  batchStatusAction?: 'enable' | 'disable' | null;
  batchDeleteLoading?: boolean;
  onNavigate?: (path: string) => void;
  onOpenKBSettings?: () => void;
  onKeywordDraftChange?: (value: string) => void;
  onSearchSubmit?: () => void;
  onSearchClear?: () => void;
  onToggleTag?: (tagId: string) => void;
  onClearTagFilter?: () => void;
  onOpenTagManage?: () => void;
  onOpenCreate?: () => void;
  onOpenImport?: () => void;
  onCloseImport?: () => void;
  onImportModeChange?: (mode: 'append' | 'replace') => void;
  onImportFile?: (file: File) => void;
  onDownloadExample?: (format: 'json' | 'csv' | 'excel') => void;
  onImportConfirm?: () => void;
  onExport?: (format: 'csv' | 'json') => void;
  onToggleSelect?: (id: number, checked: boolean) => void;
  onToggleSelectAll?: (checked: boolean) => void;
  onEditEntry?: (entry: FAQEntry) => void;
  onDeleteEntry?: (entry: FAQEntry) => void;
  /** Vue handleEntryTagChange — 卡片底部标签下拉改标签。 */
  onEntryTagChange?: (entryId: number, tagSeqId: string) => void;
  /** Vue handleEntryStatusChange — per-card enable/disable toggle. */
  onToggleEntryStatus?: (entry: FAQEntry, value: boolean) => void;
  onBatchEnable?: () => void;
  onBatchDisable?: () => void;
  onBatchDelete?: () => void;
  onLoadMore?: () => void;
  onCloseEditor?: () => void;
  onFormChange?: (patch: Partial<FormState>) => void;
  onEditorSubmit?: () => void;
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
    kbInfo = null,
    infoUserId,
    infoSharedKb = null,
    infoPermission,
    kbList = [],
    tags = [],
    activeTagIds = [],
    entries = [],
    total = 0,
    hasMore = null,
    loadingMore = false,
    loading = false,
    canContribute = false,
    canManage = canContribute,
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
    onClearSelection = () => {},
    batchTagOpen = false,
    batchTagValue = '',
    batchTagBusy = false,
    onOpenBatchTag = () => {},
    onBatchTagValueChange = () => {},
    onBatchTagConfirm = () => {},
    onCloseBatchTag = () => {},
    batchTagLoading = false,
    batchStatusAction = null,
    batchDeleteLoading = false,
    onNavigate = defaultNavigate,
    onOpenKBSettings,
    onKeywordDraftChange = () => {},
    onSearchSubmit = () => {},
    onSearchClear = () => {},
    onToggleTag = () => {},
    onClearTagFilter = () => {},
    onOpenTagManage = () => {},
    onOpenCreate = () => {},
    onOpenImport = () => {},
    onCloseImport = () => {},
    onImportModeChange = () => {},
    onImportFile = () => {},
    onDownloadExample = () => {},
    onImportConfirm = () => {},
    onExport = () => {},
    onToggleSelect = () => {},
    onToggleSelectAll = () => {},
    onEditEntry = () => {},
    onDeleteEntry = () => {},
    onEntryTagChange = () => {},
    onBatchEnable = () => {},
    onBatchDisable = () => {},
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
  const [tagSearchQuery, setTagSearchQuery] = useState('');
  // Vue tagFilterCleared (:1139) — 筛选清空后的瞬时 is-placeholder 态。
  const [tagFilterCleared, setTagFilterCleared] = useState(false);
  const [tagFilterTriggerHover, setTagFilterTriggerHover] = useState(false);
  const [masonryRevision, setMasonryRevision] = useState(0);
  const [importResultExpanded, setImportResultExpanded] = useState(false);
  const [collapsedSections, setCollapsedSections] = useState<FAQSectionCollapseState>({});
  // B4: Vue stores expanded on each hit with default false (:2669) — a per-id set.
  const [expandedResults, setExpandedResults] = useState<ReadonlySet<number>>(new Set());
  // Vue entry.showMore — one open card more-menu at a time (FAQEntryManager.vue:265-283)。
  const [moreMenuId, setMoreMenuId] = useState<number | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    if (!importOpen && !editorOpen && !searchOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      if (importOpen) onCloseImport();
      else if (editorOpen) onCloseEditor();
      else onCloseSearchTest();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [editorOpen, importOpen, onCloseEditor, onCloseImport, onCloseSearchTest, searchOpen]);
  // A new search replaces the hit list, so stale expansions drop first (Vue :2664-2673).
  const runSearchTest = () => {
    setExpandedResults(new Set());
    onSearchTestSubmit();
  };
  const toggleSearchResult = (id: number) => setExpandedResults((current) => toggleSearchResultId(current, id));
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const cardListRef = useRef<HTMLDivElement | null>(null);
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

  // Vue arrangeCards: masonry columns with the same responsive breakpoints,
  // gap, and shortest-column placement as FAQEntryManager.vue.
  useLayoutEffect(() => {
    const list = cardListRef.current;
    if (!list || entries.length === 0) return;
    const cards = Array.from(list.querySelectorAll<HTMLElement>('.faq-card'));
    const gap = 12;
    const columnCount = faqMasonryColumnCount(list.offsetWidth);
    const columnWidth = (list.offsetWidth - gap * (columnCount - 1)) / columnCount;
    const requestFrame = (callback: FrameRequestCallback) => typeof window.requestAnimationFrame === 'function'
      ? window.requestAnimationFrame(callback)
      : window.setTimeout(() => callback(Date.now()), 0);
    const cancelFrame = (id: number) => typeof window.cancelAnimationFrame === 'function'
      ? window.cancelAnimationFrame(id)
      : window.clearTimeout(id);
    const frame = requestFrame(() => {
      const heights = new Array<number>(columnCount).fill(0);
      cards.forEach((card) => {
        card.style.position = 'absolute';
        card.style.width = `${columnWidth}px`;
      });
      const measureFrame = requestFrame(() => {
        cards.forEach((card) => {
          const column = heights.indexOf(Math.min(...heights));
          card.style.top = `${heights[column]}px`;
          card.style.left = `${column * (columnWidth + gap)}px`;
          heights[column] += (card.offsetHeight || card.getBoundingClientRect().height) + gap;
        });
        list.style.position = 'relative';
        list.style.height = `${Math.max(...heights)}px`;
      });
      return () => cancelFrame(measureFrame);
    });
    return () => cancelFrame(frame);
  }, [entries.length, collapsedSections, masonryRevision]);

  useEffect(() => {
    const handleResize = () => {
      const list = cardListRef.current;
      if (!list || entries.length === 0) return;
      list.style.height = '';
      setMasonryRevision((revision) => revision + 1);
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [entries.length]);

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
  // Vue activeTagFilterLabel (:1141-1153)
  const activeTagFilterLabel = activeTagIds.length === 0
    ? (tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags'))
    : activeTagIds.length === 1
      ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
      : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
  const activeTagFilterTitle = activeTagIds.length === 1
    ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
    : activeTagFilterLabel;
  const showTagFilterClear = activeTagIds.length > 0 && tagFilterTriggerHover;
  const isTagFilterPlaceholder = activeTagIds.length === 0 && tagFilterCleared;
  const visibleTags = filterFaqTags(tags, tagSearchQuery);
  const canSelectEntries = canContribute || canManage;
  // Vue faqCreateOptions（canEdit 门控，prefixIcon 同款 16px）
  const faqCreateOptions = canContribute ? [
    { content: t('knowledgeEditor.faq.editorCreate'), value: 'create', prefixIcon: <TIcon name="add" size="16px" /> },
    { content: t('knowledgeEditor.faqImport.importButton'), value: 'import', prefixIcon: <TIcon name="upload" size="16px" /> },
  ] : [];
  const faqExportOptions = [
    { content: t('knowledgeEditor.faqExport.exportCSV'), value: 'export_csv' },
    { content: t('knowledgeEditor.faqExport.exportJSON'), value: 'export_json' },
  ];
  const downloadExampleOptions = [
    { content: t('knowledgeEditor.faqImport.downloadExampleJSON'), value: 'json' },
    { content: t('knowledgeEditor.faqImport.downloadExampleCSV'), value: 'csv' },
    { content: t('knowledgeEditor.faqImport.downloadExampleExcel'), value: 'excel' },
  ];
  const tagDropdownOptions = tags.map((tag) => ({ content: tag.name, value: String(tag.seq_id) }));
  const tagSelectOptions = tags.map((tag) => ({ label: tag.name, value: tag.seq_id }));
  const handleFaqAction = (value: unknown) => {
    switch (String(value)) {
      case 'create': onOpenCreate(); break;
      case 'import': onOpenImport(); break;
      case 'search': onOpenSearchTest(); break;
      case 'export_csv': onExport('csv'); break;
      case 'export_json': onExport('json'); break;
      case 'export': onExport('csv'); break;
    }
  };
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
  const selectedEntries = entries.filter((entry) => selected.has(entry.id));
  const selectedEnabledCount = selectedEntries.filter((entry) => entry.is_enabled !== false).length;
  const selectedDisabledCount = selectedEntries.length - selectedEnabledCount;
  const batchActionLoading = batchTagLoading || batchStatusAction != null || batchDeleteLoading;

  return (
    <main className="faq-manager-wrapper">
      <div className="faq-manager">
        <div className="faq-content">
          {/* Header */}
          <div className="faq-header">
            <div className="faq-header-title">
              <FAQBreadcrumb
                t={t}
                knowledgeBaseId={knowledgeBaseId}
                kbName={kbName}
                kbMeta={kbMeta}
                kbInfo={kbInfo}
                infoUserId={infoUserId}
                infoSharedKb={infoSharedKb}
                infoPermission={infoPermission}
                kbList={kbList}
                canManage={canManage}
                importResult={importResult}
                importResultExpanded={importResultExpanded}
                onToggleImportResultExpanded={() => setImportResultExpanded((current) => !current)}
                onCloseImportResult={onCloseImportResult}
                onDownloadFailedEntries={onDownloadFailedEntries}
                importTask={importTask}
                onNavigate={onNavigate}
                onOpenKBSettings={onOpenKBSettings}
              />
              <p className="faq-subtitle">{t('knowledgeEditor.faq.subtitle')}</p>
            </div>
          </div>

          <div className="faq-main">
            <div className="faq-card-area">
              {/* 搜索栏与标签筛选 */}
              <div className="faq-filter-bar">
                <TdInput
                  value={keywordDraft}
                  placeholder={t('knowledgeEditor.faq.searchPlaceholder')}
                  clearable
                  className="faq-search-input"
                  prefixIcon={<TIcon name="search" size="16px" />}
                  onChange={(value) => onKeywordDraftChange(String(value))}
                  onClear={() => onSearchClear()}
                  onEnter={() => onSearchSubmit()}
                />
                <div className="faq-filter-bar__filters">
                  <Popup
                    visible={tagPanelOpen}
                    trigger="click"
                    placement="bottom-left"
                    overlayClassName="tag-filter-popup"
                    overlayInnerStyle={{ padding: 0 }}
                    onVisibleChange={setTagPanelOpen}
                    content={(
                      <div className="tag-filter-panel" onClick={(event) => event.stopPropagation()}>
                        <div className="tag-filter-panel__header">
                          <div className="tag-filter-panel__title">
                            <span>{t('knowledgeBase.tagFilterTitle')}</span>
                            <span className="tag-filter-panel__count">({tags.length})</span>
                          </div>
                        </div>
                        <div className="tag-search-bar">
                          <TdInput
                            value={tagSearchQuery}
                            size="small"
                            placeholder={t('knowledgeBase.tagSearchPlaceholder')}
                            clearable
                            prefixIcon={<TIcon name="search" size="14px" />}
                            onChange={(value) => setTagSearchQuery(String(value))}
                          />
                        </div>
                        <div className="tag-filter-panel__body">
                          <div className="tag-filter-chips">
                            {visibleTags.map((tag) => (
                              <button
                                key={tag.id}
                                type="button"
                                className={'tag-filter-chip' + (activeTagIds.includes(tag.id) ? ' active' : '')}
                                title={`${tag.name} (${tag.chunk_count || 0})`}
                                onClick={() => { setTagFilterCleared(false); onToggleTag(tag.id); }}
                              >
                                <span className="tag-filter-chip__label">{tag.name}</span>
                                <span className="tag-filter-chip__count">{tag.chunk_count || 0}</span>
                              </button>
                            ))}
                          </div>
                          {!visibleTags.length ? <div className="tag-empty-state">{t('knowledgeBase.tagEmptyResult')}</div> : null}
                        </div>
                        {canContribute ? (
                          <div className="tag-filter-panel__footer">
                            <Button variant="text" size="small" className="tag-manage-link" onClick={(event) => { event.stopPropagation(); onOpenTagManage(); }}>
                              {t('knowledgeBase.tagManageLink')}
                            </Button>
                          </div>
                        ) : null}
                      </div>
                    )}
                  >
                    <div className="doc-filter-field">
                      <button
                        type="button"
                        className={'doc-tag-filter-trigger doc-filter-field__control'
                          + (tagPanelOpen ? ' open' : '')
                          + (isTagFilterPlaceholder ? ' is-placeholder' : '')}
                        aria-label={t('knowledgeBase.tagFilterTitle')}
                        title={activeTagFilterTitle}
                        onMouseEnter={() => setTagFilterTriggerHover(true)}
                        onMouseLeave={() => setTagFilterTriggerHover(false)}
                      >
                        <span className="doc-tag-filter-trigger__prefix" aria-hidden="true">
                          <TIcon name="discount" size="16px" />
                        </span>
                        <span className="doc-tag-filter-trigger__label">{activeTagFilterLabel}</span>
                        <span className="doc-tag-filter-trigger__suffix">
                          {showTagFilterClear ? (
                            <span
                              className="t-input__suffix t-input__suffix-icon t-input__clear"
                              aria-label={t('common.clear')}
                              onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); }}
                              onMouseDown={(event) => event.stopPropagation()}
                            >
                              <TIcon name="close-circle-filled" className="t-input__suffix-clear" />
                            </span>
                          ) : (
                            <TIcon name="chevron-down" size="16px" className={'doc-tag-filter-trigger__caret' + (tagPanelOpen ? ' open' : '')} />
                          )}
                        </span>
                      </button>
                    </div>
                  </Popup>
                </div>
                <div className="faq-filter-bar__trailing">
                  {/* 新建：新建条目 / 导入 */}
                  {faqCreateOptions.length ? (
                    <Tooltip content={t('knowledgeEditor.faq.createGroup')} placement="top">
                      <Dropdown options={faqCreateOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
                        <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" icon={<TIcon name="add" size="16px" />} />
                      </Dropdown>
                    </Tooltip>
                  ) : null}
                  {/* 导出 */}
                  <Dropdown options={faqExportOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
                    <Tooltip content={t('knowledgeEditor.faqExport.exportButton')} placement="top">
                      <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" loading={exportLoading} icon={<TIcon name="download" size="16px" />} />
                    </Tooltip>
                  </Dropdown>
                  {/* 检索 */}
                  <Tooltip content={t('knowledgeEditor.faq.searchTest')} placement="top">
                    <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" aria-label={t('knowledgeEditor.faq.searchTest')} onClick={() => handleFaqAction('search')} icon={<TIcon name="search" size="16px" />} />
                  </Tooltip>
                </div>
              </div>
              {/* Card List Container with Scroll */}
              <div
                ref={scrollRef}
                className={'faq-scroll-container' + (selected.size > 0 && canSelectEntries ? ' has-batch-bar' : '')}
                onScroll={handleContainerScroll}
              >
                {/* FAQ 骨架屏 */}
                {loading && entries.length === 0 ? (
                  <div className="faq-skeleton-grid">
                    {Array.from({ length: 6 }, (_, index) => (
                      <div key={'faq-skel-' + index} className="faq-card faq-card-skeleton">
                        <div className="faq-card-header">
                          <TdSkeleton animation="gradient" rowCol={[{ width: '80%', height: '16px' }]} />
                        </div>
                        <div className="faq-card-body">
                          <TdSkeleton animation="gradient" rowCol={[{ width: '100%', height: '13px' }, { width: '90%', height: '13px' }, { width: '60%', height: '13px' }]} />
                        </div>
                        <div className="faq-skel-footer">
                          <TdSkeleton animation="gradient" rowCol={[[{ width: '50px', height: '18px', type: 'rect' }, { width: '60px', height: '18px', type: 'rect' }]]} />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : entries.length > 0 ? (
                  <div ref={cardListRef} className="faq-card-list">
                    {entries.map((entry) => {
                      const isSelected = selected.has(entry.id);
                      const tagName = typeof entry.tag_id === 'number' ? tagNameBySeq.get(entry.tag_id) : undefined;
                      // Vue faq-section (:291-357): similar/negative render only when
                      // non-empty, answers always; bodies start collapsed and toggle.
                      const section = (name: 'similar' | 'negative' | 'answers', labelKey: string, values: string[], always = false) => {
                        if (!always && values.length === 0) return null;
                        const collapsed = isSectionCollapsed(collapsedSections, entry.id, name);
                        return (
                          <div className={'faq-section ' + name} key={name}>
                            <div className="faq-section-label clickable" {...sectionButton(entry.id, name)}>
                              <span>{t(labelKey)}</span>
                              <span className="section-count">({values.length})</span>
                              <TIcon name={collapsed ? 'chevron-right' : 'chevron-down'} className="collapse-icon" />
                            </div>
                            {collapsed ? null : (
                              <div className="faq-tags">
                                {values.map((value, index) => (
                                  <FaqTagTooltip key={index} content={value} placement="top" type={name === 'negative' ? 'negative' : name === 'answers' ? 'answer' : 'similar'}>
                                    <TdTag size="small" variant="light-outline" className="question-tag" theme={name === 'negative' ? 'warning' : name === 'answers' ? 'success' : undefined}>
                                      {value}
                                    </TdTag>
                                  </FaqTagTooltip>
                                ))}
                              </div>
                            )}
                          </div>
                        );
                      };
                      return (
                        <div
                          key={entry.id}
                          className={'faq-card'
                            + (isSelected ? ' selected' : '')
                            + (canSelectEntries ? ' is-selectable' : '')}
                          onClick={canSelectEntries ? () => onToggleSelect(entry.id, !isSelected) : undefined}
                        >
                          {/* Card Header */}
                          <div className="faq-card-header">
                            <div className="faq-header-top">
                              <div className="faq-question" title={entry.standard_question}>{entry.standard_question}</div>
                              <div className="faq-card-actions">
                                {canManage ? (
                                  <Popup
                                    visible={moreMenuId === entry.id}
                                    overlayClassName="card-more-popup"
                                    trigger="click"
                                    destroyOnClose
                                    placement="bottom-right"
                                    onVisibleChange={(visible: boolean) => setMoreMenuId(visible ? entry.id : null)}
                                    content={(
                                      <div className="popup-menu" onClick={(event) => event.stopPropagation()}>
                                        <div className="popup-menu-item" onClick={(event) => { event.stopPropagation(); setMoreMenuId(null); onEditEntry(entry); }}>
                                          <TIcon className="menu-icon" name="edit" />
                                          <span>{t('common.edit')}</span>
                                        </div>
                                        <div className="popup-menu-item delete" onClick={(event) => { event.stopPropagation(); setMoreMenuId(null); onDeleteEntry(entry); }}>
                                          <TIcon className="menu-icon" name="delete" />
                                          <span>{t('common.delete')}</span>
                                        </div>
                                      </div>
                                    )}
                                  >
                                    <div className="card-more-btn" onClick={(event) => event.stopPropagation()}>
                                      <img className="more-icon" src={MORE_PNG} alt="" />
                                    </div>
                                  </Popup>
                                ) : null}
                              </div>
                            </div>
                          </div>

                          {/* Card Body */}
                          <div className="faq-card-body">
                            {section('similar', 'knowledgeEditor.faq.similarQuestions', entry.similar_questions)}
                            {section('negative', 'knowledgeEditor.faq.negativeQuestions', entry.negative_questions)}
                            {section('answers', 'knowledgeEditor.faq.answers', entry.answers, true)}
                          </div>

                          {/* Card Footer */}
                          <div className="faq-card-footer">
                            <div className="faq-card-tag" onClick={(event) => event.stopPropagation()}>
                              {canContribute && tags.length ? (
                                <Dropdown options={tagDropdownOptions} trigger="click" onClick={(item) => onEntryTagChange(entry.id, String((item as { value?: unknown }).value ?? ''))}>
                                  <TdTag size="small" variant="light-outline" className="faq-tag-chip">
                                    <span className="tag-text">{tagName ?? t('knowledgeBase.untagged')}</span>
                                  </TdTag>
                                </Dropdown>
                              ) : (
                                <TdTag size="small" variant="light-outline" className="faq-tag-chip">
                                  <span className="tag-text">{tagName ?? t('knowledgeBase.untagged')}</span>
                                </TdTag>
                              )}
                            </div>
                            <div className="faq-card-status" onClick={(event) => event.stopPropagation()}>
                              <Tooltip content={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')} placement="top">
                                <div className="status-item-compact">
                                  <TdSwitch
                                    size="small"
                                    value={entry.is_enabled}
                                    loading={statusUpdatingIds.includes(entry.id)}
                                    disabled={statusUpdatingIds.includes(entry.id) || !canContribute}
                                    onChange={(value: boolean) => onToggleEntryStatus(entry, value)}
                                  />
                                </div>
                              </Tooltip>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : !loading ? (
                  <div className="faq-empty-state">
                    <div className="empty-content">
                      <TIcon name="file-add" size="48px" className="empty-icon" />
                      <div className="empty-text">{t('knowledgeEditor.faq.emptyTitle')}</div>
                      <div className="empty-desc">{t('knowledgeEditor.faq.emptyDesc')}</div>
                    </div>
                  </div>
                ) : null}
                {loadingMore ? (
                  <div className="faq-load-more">
                    <TdLoading size="small" text={t('common.loading')} />
                  </div>
                ) : null}
                {hasMore === false && entries.length > 0 ? (
                  <div className="faq-no-more">{t('common.noMoreData')}</div>
                ) : null}
              </div>
              <div className="faq-batch-bar-anchor">
                {/* FAQBatchBar.vue 1:1 —— count>0 && (canEdit||canManage) 才渲染 */}
                {selected.size > 0 && (canContribute || canManage) ? (
                  <div className="faq-batch-bar" role="region" aria-label={t('knowledgeBase.selectedCount', { count: selected.size })}>
                    <div className="faq-batch-bar__inner">
                      <div className="faq-batch-bar__selection">
                        <span className="faq-batch-bar__count">{t('knowledgeBase.selectedCount', { count: selected.size })}</span>
                        <Button variant="text" theme="default" size="small" disabled={batchActionLoading} onClick={() => onClearSelection()}>
                          {t('knowledgeBase.clearSelection')}
                        </Button>
                      </div>
                      <div className="faq-batch-bar__actions">
                        {canContribute ? (
                          <Button theme="default" variant="outline" size="small" disabled={batchActionLoading} loading={batchTagLoading} onClick={() => onOpenBatchTag()}>
                            <TIcon name="discount" size="14px" /><span>{t('knowledgeEditor.faq.batchUpdateTag')}</span>
                          </Button>
                        ) : null}
                        {canContribute && selectedDisabledCount > 0 ? (
                          <Button theme="default" variant="outline" size="small" disabled={batchActionLoading} loading={batchStatusAction === 'enable'} onClick={() => onBatchEnable()}>
                            <TIcon name="check-circle" size="14px" /><span>{t('knowledgeEditor.faq.batchEnable')}</span>
                          </Button>
                        ) : null}
                        {canContribute && selectedEnabledCount > 0 ? (
                          <Button theme="default" variant="outline" size="small" disabled={batchActionLoading} loading={batchStatusAction === 'disable'} onClick={() => onBatchDisable()}>
                            <TIcon name="minus-circle" size="14px" /><span>{t('knowledgeEditor.faq.batchDisable')}</span>
                          </Button>
                        ) : null}
                        {canManage ? (
                          <Popconfirm
                            theme="warning"
                            content={t('knowledgeEditor.faq.confirmBatchDelete', { count: selected.size })}
                            confirmBtn={{ content: t('knowledgeBase.confirmDelete'), theme: 'danger' }}
                            cancelBtn={{ content: t('common.cancel') }}
                            placement="top"
                            onConfirm={() => onBatchDelete()}
                          >
                            <Button theme="danger" variant="outline" size="small" disabled={batchActionLoading} loading={batchDeleteLoading} onClick={(event) => event.stopPropagation()}>
                              <TIcon name="delete" size="14px" /><span>{t('knowledgeEditor.faq.batchDelete')}</span>
                            </Button>
                          </Popconfirm>
                        ) : null}
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>

        {/* Editor Drawer */}
        <Drawer
          visible={editorOpen}
          header={editorMode === 'create' ? t('knowledgeEditor.faq.editorCreate') : t('knowledgeEditor.faq.editorEdit')}
          closeBtn
          size="520px"
          placement="right"
          className="faq-editor-drawer"
          onClose={() => onCloseEditor()}
          footer={(
            <div className="faq-editor-drawer-footer">
              <Button theme="default" variant="outline" onClick={() => onCloseEditor()}>
                {t('common.cancel')}
              </Button>
              <Button theme="primary" loading={saving} onClick={() => onEditorSubmit()}>
                {editorMode === 'create' ? t('knowledgeEditor.faq.editorCreate') : t('common.save')}
              </Button>
            </div>
          )}
        >
          <div className="faq-editor-drawer-content">
            <form className="faq-editor-form" onSubmit={(event) => { event.preventDefault(); onEditorSubmit(); }}>
              <div className="settings-group">
                {/* 标准问 */}
                <div className="setting-row vertical setting-row-primary">
                  <div className="setting-info">
                    <label className="required-label">
                      {t('knowledgeEditor.faq.standardQuestion')}
                      <span className="required-mark">*</span>
                    </label>
                    <p className="desc">{t('knowledgeEditor.faq.standardQuestionDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <TdInput value={form.question} maxlength={200} className="full-width-input" onChange={(value) => onFormChange({ question: String(value) })} />
                  </div>
                </div>

                {/* 相似问 */}
                <div className="setting-row vertical setting-row-optional setting-row-similar">
                  <div className="setting-info">
                    <label className="optional-label">{t('knowledgeEditor.faq.similarQuestions')}</label>
                    <p className="desc optional-desc">{t('knowledgeEditor.faq.similarQuestionsDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="full-width-input-wrapper">
                      <TdInput
                        value={form.similarDraft}
                        placeholder={t('knowledgeEditor.faq.similarPlaceholder')}
                        className="full-width-input"
                        onEnter={() => addSimilar()}
                        onChange={(value) => onFormChange({ similarDraft: String(value) })}
                      />
                      <Button
                        theme="primary"
                        variant="outline"
                        disabled={!form.similarDraft.trim() || form.similarQuestions.length >= FAQ_SIMILAR_CAP}
                        className="add-item-btn"
                        size="small"
                        onClick={() => addSimilar()}
                        icon={<TIcon name="add" size="16px" />}
                      />
                    </div>
                    {form.similarQuestions.length > 0 ? (
                      <div className="item-list">
                        {form.similarQuestions.map((question, index) => (
                          <div key={index} className="item-row">
                            <div className="item-content">{question}</div>
                            <Button theme="default" variant="text" size="small" className="remove-item-btn" onClick={() => onFormChange({ similarQuestions: removeListItem(form.similarQuestions, index) })} icon={<TIcon name="close" size="16px" />} />
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </div>

                {/* 反例 */}
                <div className="setting-row vertical setting-row-optional setting-row-negative">
                  <div className="setting-info">
                    <label className="optional-label">{t('knowledgeEditor.faq.negativeQuestions')}</label>
                    <p className="desc optional-desc">{t('knowledgeEditor.faq.negativeQuestionsDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="full-width-input-wrapper">
                      <TdInput
                        value={form.negativeDraft}
                        placeholder={t('knowledgeEditor.faq.negativePlaceholder')}
                        className="full-width-input"
                        onEnter={() => addNegative()}
                        onChange={(value) => onFormChange({ negativeDraft: String(value) })}
                      />
                      <Button
                        theme="primary"
                        variant="outline"
                        disabled={!form.negativeDraft.trim() || form.negativeQuestions.length >= FAQ_NEGATIVE_CAP}
                        className="add-item-btn"
                        size="small"
                        onClick={() => addNegative()}
                        icon={<TIcon name="add" size="16px" />}
                      />
                    </div>
                    {form.negativeQuestions.length > 0 ? (
                      <div className="item-list">
                        {form.negativeQuestions.map((question, index) => (
                          <div key={index} className="item-row negative">
                            <div className="item-content">{question}</div>
                            <Button theme="default" variant="text" size="small" className="remove-item-btn" onClick={() => onFormChange({ negativeQuestions: removeListItem(form.negativeQuestions, index) })} icon={<TIcon name="close" size="16px" />} />
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </div>

                {/* 答案 */}
                <div className="setting-row vertical setting-row-primary setting-row-answer">
                  <div className="setting-info">
                    <label className="required-label">
                      {t('knowledgeEditor.faq.answers')}
                      <span className="required-mark">*</span>
                    </label>
                    <p className="desc">{t('knowledgeEditor.faq.answersDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="textarea-container">
                      <div className="full-width-input-wrapper textarea-wrapper">
                        <TdTextarea
                          value={form.answerDraft}
                          placeholder={t('knowledgeEditor.faq.answerPlaceholder')}
                          autosize={{ minRows: 3, maxRows: 6 }}
                          className="full-width-textarea"
                          onChange={(value) => onFormChange({ answerDraft: String(value) })}
                          onKeydown={(context) => {
                            const event = (context as { e?: KeyboardEvent }).e;
                            if (event && (event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                              event.preventDefault();
                              addAnswer();
                            }
                          }}
                        />
                        <Button
                          theme="primary"
                          variant="outline"
                          disabled={!form.answerDraft.trim() || form.answers.length >= FAQ_ANSWER_CAP}
                          className="add-item-btn"
                          size="small"
                          onClick={() => addAnswer()}
                          icon={<TIcon name="add" size="16px" />}
                        />
                      </div>
                      <div className="item-count">{form.answers.length}/5</div>
                    </div>
                    {form.answers.length > 0 ? (
                      <div className="item-list">
                        {form.answers.map((answer, index) => (
                          <div key={index} className="item-row answer-row">
                            <div className="item-content">{answer}</div>
                            <Button theme="default" variant="text" size="small" className="remove-item-btn" onClick={() => onFormChange({ answers: removeListItem(form.answers, index) })} icon={<TIcon name="close" size="16px" />} />
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </div>

                <div className="setting-row vertical">
                  <div className="setting-info">
                    <label>{t('knowledgeBase.tagLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.tagDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <TdSelect
                      value={form.tagId.trim() ? Number(form.tagId) : undefined}
                      className="full-width-input"
                      options={tagSelectOptions}
                      clearable
                      placeholder={t('knowledgeEditor.faq.tagPlaceholder')}
                      onChange={(value) => onFormChange({ tagId: value == null || value === '' ? '' : String(value) })}
                    />
                  </div>
                </div>
              </div>
            </form>
          </div>
        </Drawer>

        {/* Import Dialog —— Vue Teleport to body；React 侧以 position:fixed 覆盖层
            就地渲染（视觉/层叠等价；createPortal 在静态渲染测试中不可用） */}
        {importOpen ? (
          <div className="faq-import-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>
            <div className="faq-import-modal">
              {/* 关闭按钮 */}
              <button className="close-btn" aria-label={t('general.close')} onClick={() => onCloseImport()}>
                <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                  <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                </svg>
              </button>

              <div className="faq-import-container">
                <div className="faq-import-header">
                  <h2 className="import-title">{t('knowledgeEditor.faqImport.title')}</h2>
                </div>

                <div className="faq-import-content">
                  {/* 导入模式选择 */}
                  <div className="import-form-item">
                    <label className="import-form-label required">{t('knowledgeEditor.faqImport.modeLabel')}</label>
                    <RadioGroup value={importMode} className="import-radio-group" onChange={(value) => onImportModeChange(value === 'replace' ? 'replace' : 'append')}>
                      <Radio.Button value="append">{t('knowledgeEditor.faqImport.appendMode')}</Radio.Button>
                      <Radio.Button value="replace">{t('knowledgeEditor.faqImport.replaceMode')}</Radio.Button>
                    </RadioGroup>
                  </div>

                  {/* 文件上传区域 */}
                  <div className="import-form-item">
                    <div className="file-label-row">
                      <label className="import-form-label required">{t('knowledgeEditor.faqImport.fileLabel')}</label>
                      <Dropdown options={downloadExampleOptions} placement="bottom-right" trigger="click" onClick={(item) => onDownloadExample(String((item as { value?: unknown }).value) as 'json' | 'csv' | 'excel')}>
                        <Button theme="default" variant="outline" size="small" className="download-example-btn">
                          <TIcon name="download" size="16px" /><span>{t('knowledgeEditor.faqImport.downloadExample')}</span>
                        </Button>
                      </Dropdown>
                    </div>
                    <div className="file-upload-wrapper">
                      <input
                        ref={fileInputRef}
                        type="file"
                        accept=".json,.csv,.xlsx,.xls"
                        className="file-input-hidden"
                        onChange={(event) => {
                          const file = event.target.files?.[0];
                          if (file) onImportFile(file);
                          event.target.value = '';
                        }}
                      />
                      <div
                        className={'file-upload-area' + (importFileName ? ' has-file' : '')}
                        onClick={() => fileInputRef.current?.click()}
                        onDragOver={(event: DragEvent<HTMLDivElement>) => event.preventDefault()}
                        onDragEnter={(event: DragEvent<HTMLDivElement>) => event.preventDefault()}
                        onDrop={(event: DragEvent<HTMLDivElement>) => { event.preventDefault(); const file = event.dataTransfer.files?.[0]; if (file) onImportFile(file); }}
                      >
                        <div className="file-upload-content">
                          <TIcon name="upload" size="32px" className="upload-icon" />
                          <div className="upload-text">
                            {importFileName ? (
                              <span className="upload-file-name">{importFileName}</span>
                            ) : (
                              <span className="upload-primary-text">{t('knowledgeEditor.faqImport.clickToUpload')}</span>
                            )}
                            {importFileName ? null : <span className="upload-secondary-text">{t('knowledgeEditor.faqImport.dragDropTip')}</span>}
                          </div>
                        </div>
                      </div>
                      <p className="import-form-tip">{t('knowledgeEditor.faqImport.fileTip')}</p>
                    </div>
                  </div>

                  {/* 预览区域 */}
                  {importPreview.length ? (
                    <div className="import-preview">
                      <div className="preview-header">
                        <TIcon name="file-view" size="16px" className="preview-icon" />
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
                      {importPreview.length > 5 ? (
                        <p className="preview-more">{t('knowledgeEditor.faqImport.previewMore', { count: importPreview.length - 5 })}</p>
                      ) : null}
                    </div>
                  ) : null}
                </div>

                <div className="faq-import-footer">
                  <Button theme="default" variant="outline" onClick={() => onCloseImport()} disabled={importBusy && importTask?.status === 'running'}>
                    {t('common.cancel')}
                  </Button>
                  <Button
                    theme="primary"
                    loading={importBusy && !importTask}
                    disabled={importTask?.status === 'running'}
                    onClick={() => onImportConfirm()}
                  >
                    {importTask?.status === 'success' ? t('common.close') : importTask?.status === 'failed' ? t('common.retry') : t('knowledgeEditor.faqImport.importButton')}
                  </Button>
                </div>
              </div>
            </div>
          </div>
        ) : null}

        {/* Batch Tag Dialog —— 同导入弹窗，fixed 覆盖层就地渲染 */}
        {batchTagOpen ? (
          <div className="batch-tag-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCloseBatchTag(); }}>
            <div className="batch-tag-modal">
              {/* 关闭按钮 */}
              <button className="batch-tag-close-btn" aria-label={t('general.close')} onClick={() => onCloseBatchTag()}>
                <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                  <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                </svg>
              </button>

              <div className="batch-tag-container">
                <div className="batch-tag-header">
                  <h2 className="batch-tag-title">{t('knowledgeEditor.faq.batchUpdateTag')}</h2>
                </div>

                <div className="batch-tag-content">
                  <div className="batch-tag-tip">
                    <TIcon name="info-circle" size="16px" className="tip-icon" />
                    <span>{t('knowledgeEditor.faq.batchUpdateTagTip', { count: selected.size })}</span>
                  </div>
                  <div className="batch-tag-form">
                    <div className="batch-tag-form-item">
                      <label className="batch-tag-form-label">{t('knowledgeBase.tagLabel')}</label>
                      <TdSelect
                        value={batchTagValue.trim() ? Number(batchTagValue) : undefined}
                        options={tagSelectOptions}
                        placeholder={t('knowledgeBase.tagPlaceholder')}
                        clearable
                        filterable
                        className="batch-tag-select"
                        empty={<div className="tag-select-empty">{t('knowledgeBase.noTags')}</div>}
                        onChange={(value) => onBatchTagValueChange(value == null || value === '' ? '' : String(value))}
                      />
                    </div>
                  </div>
                </div>

                <div className="batch-tag-footer">
                  <Button theme="default" variant="outline" onClick={() => onCloseBatchTag()}>
                    {t('common.cancel')}
                  </Button>
                  <Button theme="primary" loading={batchTagBusy} disabled={batchTagBusy} onClick={() => onBatchTagConfirm()}>
                    {t('common.confirm')}
                  </Button>
                </div>
              </div>
            </div>
          </div>
        ) : null}

        {/* Search Test Drawer —— 默认 footer 的确认按钮在 vue-next 走
            confirmBtnAction（关抽屉），tdesign-react 仅回调 onConfirm——补齐关闭。 */}
        <Drawer
          visible={searchOpen}
          header={t('knowledgeEditor.faq.searchTestTitle')}
          closeBtn
          size="420px"
          placement="right"
          className="faq-search-drawer"
          onClose={() => onCloseSearchTest()}
          onConfirm={() => onCloseSearchTest()}
        >
          <div className="search-test-content">
            <div className="search-form">
              <div className="settings-group">
                {/* 查询文本 */}
                <div className="setting-row vertical search-first-row">
                  <div className="setting-info">
                    <label>{t('knowledgeEditor.faq.queryLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.queryPlaceholder')}</p>
                  </div>
                  <div className="setting-control">
                    <TdInput
                      value={searchForm.query}
                      placeholder={t('knowledgeEditor.faq.queryPlaceholder')}
                      className="full-width-input faq-search-query-input"
                      onEnter={() => runSearchTest()}
                      onChange={(value) => onSearchFormChange({ query: String(value) })}
                    />
                  </div>
                </div>

                {/* 相似度阈值 */}
                <div className="setting-row vertical">
                  <div className="setting-info">
                    <label>{t('knowledgeEditor.faq.similarityThresholdLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.vectorThresholdDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="slider-wrapper">
                      <Slider
                        value={searchForm.vectorThreshold}
                        min={FAQ_SEARCH_VECTOR_THRESHOLD.min}
                        max={FAQ_SEARCH_VECTOR_THRESHOLD.max}
                        step={FAQ_SEARCH_VECTOR_THRESHOLD.step}
                        tooltipProps={{ trigger: 'hover' }}
                        onChange={(value: number) => onSearchFormChange({ vectorThreshold: Number(value) })}
                      />
                      <div className="slider-value">{searchForm.vectorThreshold.toFixed(2)}</div>
                    </div>
                  </div>
                </div>

                {/* 匹配数量 */}
                <div className="setting-row vertical">
                  <div className="setting-info">
                    <label>{t('knowledgeEditor.faq.matchCountLabel')}</label>
                    <p className="desc">{t('knowledgeEditor.faq.matchCountDesc')}</p>
                  </div>
                  <div className="setting-control">
                    <div className="slider-wrapper">
                      <Slider
                        value={searchForm.matchCount}
                        min={FAQ_SEARCH_MATCH_COUNT.min}
                        max={FAQ_SEARCH_MATCH_COUNT.max}
                        step={FAQ_SEARCH_MATCH_COUNT.step}
                        tooltipProps={{ trigger: 'hover' }}
                        onChange={(value: number) => onSearchFormChange({ matchCount: Number(value) })}
                      />
                      <div className="slider-value">{searchForm.matchCount}</div>
                    </div>
                  </div>
                </div>

                {/* 搜索按钮 */}
                <div className="setting-row vertical">
                  <div className="setting-control">
                    <Button theme="primary" block loading={searching} className="search-button" onClick={() => runSearchTest()}>
                      {searching ? t('knowledgeEditor.faq.searching') : t('knowledgeEditor.faq.searchButton')}
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            {/* Search Results */}
            {searchResults.length > 0 || hasSearched ? (
              <FAQSearchResults t={t} results={searchResults} expandedIds={expandedResults} onToggle={toggleSearchResult} />
            ) : null}
          </div>
        </Drawer>
      </div>
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
                <div className="result-header" onClick={() => onToggle(result.id)}>
                  <div className="result-question-wrapper">
                    <div className="result-main">
                      <div className="result-question">
                        <span className="result-index">{index + 1}.</span>
                        {result.standard_question}
                      </div>
                      {result.matched_question && result.matched_question !== result.standard_question ? (
                        <div className="matched-question">
                          <span className="matched-label">{t('knowledgeEditor.faq.matchedQuestion')}:</span>
                          <span className="matched-text">{result.matched_question}</span>
                        </div>
                      ) : null}
                    </div>
                    <div className="result-meta">
                      <TdTag size="small" variant="light-outline" className="score-tag">
                        {(result.score || 0).toFixed(3)}
                      </TdTag>
                    </div>
                    <TIcon name={expanded ? 'chevron-up' : 'chevron-down'} className="expand-icon" />
                  </div>
                </div>
                {expanded ? (
                  <div className="result-body">
                    {result.answers?.length ? (
                      <div className="result-section">
                        <div className="section-label">{t('knowledgeEditor.faq.answers')}</div>
                        <div className="result-tags">
                          {result.answers.map((answer, answerIndex) => (
                            <Tooltip key={answerIndex} content={answer} placement="top">
                              <TdTag size="small" theme="success" variant="light" className="answer-tag">{answer}</TdTag>
                            </Tooltip>
                          ))}
                        </div>
                      </div>
                    ) : null}
                    {result.similar_questions?.length ? (
                      <div className="result-section">
                        <div className="section-label">{t('knowledgeEditor.faq.similarQuestions')}</div>
                        <div className="result-tags">
                          {result.similar_questions.map((question, questionIndex) => (
                            <Tooltip key={questionIndex} content={question} placement="top">
                              <TdTag size="small" variant="light-outline" className="question-tag">{question}</TdTag>
                            </Tooltip>
                          ))}
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

  // Vue KbTagManageDrawer 为抽屉形态；此弹窗仅在「管理标签」交互路径可达（非扫描态），
  // 沿用 tdesign Dialog（cancelBtn variant 补齐见台账 #1）。
  return (
    <Dialog
      visible={open}
      header={t('knowledgeBase.tagManageTitle')}
      closeBtn
      footer={null}
      cancelBtn={null}
      confirmBtn={null}
      onClose={() => onClose()}
    >
      <p className="faq-tag-manage-desc">{t('knowledgeBase.tagManageDescription')}</p>
      {error ? <p className="faq-tag-manage-error" role="alert">{error}</p> : null}
      <div className="faq-tag-manage-toolbar">
        <TdInput className="faq-tag-manage-search" value={query} placeholder={t('knowledgeBase.tagSearchPlaceholder')} onChange={(value) => setQuery(String(value))} clearable />
        <Button variant="outline" disabled={busy} onClick={() => { setCreating(true); setEditingId(null); }}>{t('knowledgeBase.tagCreateAction')}</Button>
      </div>
      {creating ? (
        <div className="faq-tag-manage-edit">
          <TdInput
            autofocus
            maxlength={40}
            value={draft}
            placeholder={t('knowledgeBase.tagNamePlaceholder')}
            onChange={(value) => setDraft(String(value))}
            onEnter={() => void createTag()}
            onKeydown={(context) => { if ((context as { e?: KeyboardEvent }).e?.key === 'Escape') setCreating(false); }}
          />
          <Button variant="outline" loading={busy} onClick={() => void createTag()}>{t('common.create')}</Button>
          <Button variant="outline" disabled={busy} onClick={() => setCreating(false)}>{t('common.cancel')}</Button>
        </div>
      ) : null}
      <ul className="faq-tag-manage-list">
        {visible.map((tag) => editingId === tag.id ? (
          <li key={tag.id} className="faq-tag-manage-row">
            <TdInput
              autofocus
              maxlength={40}
              value={editingName}
              onChange={(value) => setEditingName(String(value))}
              onEnter={() => void updateTag()}
              onKeydown={(context) => { if ((context as { e?: KeyboardEvent }).e?.key === 'Escape') setEditingId(null); }}
            />
            <Button variant="outline" loading={busy} onClick={() => void updateTag()}>{t('common.save')}</Button>
            <Button variant="outline" disabled={busy} onClick={() => setEditingId(null)}>{t('common.cancel')}</Button>
          </li>
        ) : (
          <li key={tag.id} className="faq-tag-manage-row">
            <span className="faq-tag-manage-name">
              <strong>{tag.name}</strong>
              <small>{t('knowledgeBase.tagManageFaqCount', { count: tag.chunk_count || 0 })}</small>
            </span>
            <Button variant="outline" disabled={busy} onClick={() => { setEditingId(tag.id); setEditingName(tag.name); setCreating(false); }}>{t('knowledgeBase.tagEditAction')}</Button>
            <Button variant="outline" disabled={busy || !Number.isSafeInteger(tag.seq_id)} onClick={() => void removeTag(tag)}>{t('knowledgeBase.tagDeleteAction')}</Button>
          </li>
        ))}
        {visible.length === 0 ? <li><p className="faq-tag-manage-empty">{t('knowledgeBase.tagEmptyResult')}</p></li> : null}
      </ul>
    </Dialog>
  );
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
const FAQ_EXAMPLE_ENTRIES: FAQEntryPayload[] = [
  { standard_question: '什么是 WeKnora？', answers: ['WeKnora 是一个智能知识库管理系统'], similar_questions: ['WeKnora 是什么？'], negative_questions: [], tag_name: '产品介绍' },
  { standard_question: '如何创建知识库？', answers: ['点击新建知识库并填写相关信息'], similar_questions: ['怎么创建知识库？'], negative_questions: [], tag_name: '使用指南' },
];
function downloadExampleFile(format: 'json' | 'csv' | 'excel'): void {
  const entries = FAQ_EXAMPLE_ENTRIES.map(({ tag_name: _tagName, ...entry }) => entry);
  let bytes: BlobPart;
  let extension: string;
  let mime: string;
  if (format === 'excel') {
    const worksheet = XLSX.utils.json_to_sheet(entries);
    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, worksheet, 'FAQ');
    bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' }) as ArrayBuffer;
    extension = 'xlsx';
    mime = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
  } else {
    const text = format === 'json' ? JSON.stringify(entries, null, 2) : serializeFAQEntries(entries as FAQEntry[], 'csv');
    bytes = text;
    extension = format;
    mime = format === 'json' ? 'application/json;charset=utf-8' : 'text/csv;charset=utf-8';
  }
  const url = URL.createObjectURL(new Blob([bytes], { type: mime }));
  const link = document.createElement('a');
  link.href = url;
  link.download = `faq_example.${extension}`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
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
  // Vue KnowledgeBase.vue:88 — isFAQ = (kbInfo?.type || '') === 'faq'; the FAQ
  // manager only mounts on the v-else branch of v-if="!isFAQ", so a document
  // KB can never reach the FAQ view (Vue has no /faq route — the KB detail
  // always renders the documents view). React's standalone /knowledgeBase/:id/
  // faq route mirrors that gate: nothing FAQ-related is fetched until the KB
  // type resolves, and a non-FAQ KB is redirected (replace, so the dead /faq
  // URL drops off history) to the KB detail documents route.
  const [faqGate, setFaqGate] = useState<'pending' | 'allowed' | 'blocked'>('pending');
  const [kb, setKb] = useState<KnowledgeBase | null>(null);
  const [kbList, setKbList] = useState<KBListItem[]>([]);
  const [tags, setTags] = useState<KnowledgeTag[]>([]);
  // Vue authStore.user?.id（KBInfoPopover owner 判定）。
  const [meId, setMeId] = useState('');
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
  // R491 1b/1c: Vue batch bar state — the tag dialog (FAQEntryManager.vue:687)
  // and the delete popconfirm (FAQBatchBar.vue:65-74) replace the old inline select.
  const [batchTagOpen, setBatchTagOpen] = useState(false);
  const [batchTagValue, setBatchTagValue] = useState('');
  const [batchTagBusy, setBatchTagBusy] = useState(false);
  const [batchTagLoading, setBatchTagLoading] = useState(false);
  const [batchStatusAction, setBatchStatusAction] = useState<'enable' | 'disable' | null>(null);
  const [batchDeleteLoading, setBatchDeleteLoading] = useState(false);
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
  // Vue onMounted → restoreImportTask + loadImportResult (:2825-2829) — gated
  // on the resolved FAQ type so a document KB fires zero /api/v1/faq requests.
  useEffect(() => { if (faqGate === 'allowed') void loadLastResult(knowledgeBaseId); }, [faqGate, loadLastResult, knowledgeBaseId]);
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
      }).catch((error) => {
        const message = error instanceof Error && error.message ? error.message : t('common.operationFailed');
        setImportTask((current) => current ? { ...current, status: 'failed', error: message } : current);
        setMessage({ tone: 'error', text: message });
      });
    }, 1500);
    return () => clearInterval(timer);
  }, [importTask, faq, knowledgeBaseId, loadLastResult]);
  useEffect(() => {
    if (importTask?.status !== 'completed') return;
    const timer = setTimeout(() => setImportTask(null), 3000);
    return () => clearTimeout(timer);
  }, [importTask?.status]);
  const [canContribute, setCanContribute] = useState(false);
  const [canManage, setCanManage] = useState(false);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const navigate = useCallback((path: string) => { clientNavigate(path); }, []);
  // Vue MessagePlugin 语义：message 状态变化即 toast（页面 DOM 无内联错误块）。
  useEffect(() => {
    if (!message) return;
    const theme = message.tone === 'error' ? 'error' : message.tone === 'success' ? 'success' : 'warning';
    MessagePlugin[theme](message.text);
  }, [message]);
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
  // switcher), tags (filter) and the caller to gate viewer accounts. The KB
  // record also decides the Vue isFAQ gate: FAQ work only proceeds for
  // type === 'faq'; an unresolvable type falls through as allowed so a
  // transient settings failure degrades to the old error surface (the load
  // failure handling below stops any retry storm) instead of a hard redirect.
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
      const meRow = me as { user?: { id?: unknown } } | null;
      setMeId(meRow?.user && typeof meRow.user.id === 'string' ? meRow.user.id : '');
      const permissions = computeKBPermissions(kbRow as KBSurfaceKB, me as KBSurfaceMe | null);
      setCanContribute(permissions.canContribute);
      setCanManage(permissions.canContribute);
      setFaqGate((typeof kbRow?.type === 'string' ? kbRow.type : '') === 'faq' ? 'allowed' : 'blocked');
    }).catch(() => { if (active) { setCanContribute(false); setFaqGate('allowed'); } });
    return () => { active = false; };
  }, [client, knowledgeBaseId]);

  // Vue keeps document KBs on the documents view — mirror that by replacing
  // the unreachable /faq URL with the KB detail route.
  useEffect(() => {
    if (faqGate !== 'blocked') return;
    clientNavigate(faqKBDetailPath(knowledgeBaseId), 'replace');
  }, [faqGate, knowledgeBaseId]);

  // KBInfoPopover 访问段（B2 批 2）：Vue orgStore currentSharedKb /
  // effectiveKBPermission 的 org shared-knowledge-bases 行解析（edit-permission
  // findSharedKBGrant 同源），失败静默降级为非共享视图。
  const [infoShared, setInfoShared] = useState<{ sharedKb: { orgName: string; sharedAt: string } | null; permission: string }>({ sharedKb: null, permission: '' });
  useEffect(() => {
    let active = true;
    void client.identity.organizations.knowledgeBaseShares.listShared().then((rows) => {
      if (!active) return;
      const grant = findSharedKBGrant(rows, knowledgeBaseId);
      const row = Array.isArray(rows)
        ? (rows as Array<Record<string, unknown>>).find((entry) => {
          const kb = entry?.knowledge_base as { id?: unknown } | null | undefined;
          return kb && String(kb.id) === knowledgeBaseId;
        })
        : null;
      setInfoShared({
        sharedKb: grant && row ? {
          orgName: typeof row.org_name === 'string' ? row.org_name : '',
          sharedAt: typeof row.shared_at === 'string' ? row.shared_at : '',
        } : null,
        permission: grant?.permission ?? '',
      });
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
    } catch (error) {
      // Loading-reset: a failed page must also drop hasMore, or the
      // FAQPageView fill-short-page effect re-fires loadMore on every render
      // while entries stay empty — the unbounded 400 retry chain a document
      // KB used to produce. Vue settles loading/loadingMore in its finally;
      // this keeps the auto-append terminal too.
      setHasMore(false);
      setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') });
    } finally {
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
  // First page loads only after the gate resolves the KB as FAQ type — a
  // document KB never reaches the FAQ endpoints (Vue isFAQ gate parity).
  useEffect(() => { if (faqGate === 'allowed') void load(false); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [client, knowledgeBaseId, keyword, activeTagIds, faqGate]);

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
  async function save() {
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
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
    finally { setSaving(false); }
  }
  async function updateSelection(input: FAQEntryFieldsUpdate) {
    if (!selected.size) return;
    try { await faq.updateFields(knowledgeBaseId, { by_id: Object.fromEntries([...selected].map((id) => [id, input])) }); await load(false); setMessage({ tone: 'success', text: t(faqBatchSuccessKey(input), { count: selected.size }) }); setSelected(new Set()); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
  }
  // R491 1c: Vue handleBatchTag (FAQEntryManager.vue:1806-1826) — confirm from
  // the batch-tag dialog; an empty value clears the tag (null), success uses
  // knowledgeEditor.messages.updateSuccess, the dialog closes and the
  // selection resets.
  async function confirmBatchTag() {
    if (!selected.size || batchTagBusy) return;
    const tagId = batchTagValue.trim() ? Number(batchTagValue) : null;
    if (tagId !== null && (!Number.isSafeInteger(tagId) || tagId < 0)) { setMessage({ tone: 'error', text: 'Tag ID must be a non-negative integer.' }); return; }
    setBatchTagBusy(true);
    setBatchTagLoading(true);
    try {
      await faq.updateTags(knowledgeBaseId, { updates: Object.fromEntries([...selected].map((id) => [id, tagId])) });
      await load(false);
      setMessage({ tone: 'success', text: t('knowledgeEditor.messages.updateSuccess') });
      setBatchTagOpen(false); setBatchTagValue(''); setSelected(new Set());
    }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
    finally { setBatchTagBusy(false); setBatchTagLoading(false); }
  }
  async function removeMany(ids: number[]) {
    if (!ids.length) return;
    setBatchDeleteLoading(ids.length > 1);
    try { await faq.removeMany(knowledgeBaseId, ids); await load(false); setMessage({ tone: 'success', text: t(faqDeleteSuccessKey(ids.length), { count: ids.length }) }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
    finally { setBatchDeleteLoading(false); }
  }
  // Vue handleEntryTagChange — 单卡片改标签（updateFAQEntryTagBatch + 回滚）。
  async function updateEntryTag(entryId: number, tagSeqId: string) {
    if (!knowledgeBaseId) return;
    const target = entries.find((entry) => entry.id === entryId);
    const previousTagId = target?.tag_id;
    const normalized = tagSeqId ? Number(tagSeqId) : null;
    if (normalized === previousTagId) return;
    try {
      await faq.updateTags(knowledgeBaseId, { updates: { [entryId]: normalized } });
      setMessage({ tone: 'success', text: t('knowledgeEditor.messages.updateSuccess') });
      await load(false);
    } catch (error) {
      setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.operationFailed') });
    }
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
    if (faqImportBlocked(importFile?.name, importPreview.length)) {
      setMessage({ tone: 'warning', text: t('knowledgeEditor.faqImport.selectFile') });
      return;
    }
    const file = importFile;
    if (!file) return;
    setImportBusy(true); setMessage(null);
    try {
      const format = importFormatFromName(file.name);
      // Vue parseExcelFile (FAQEntryManager.vue:1999) — binary parse for .xlsx/.xls.
      const imported = format === 'excel' ? await parseExcelFile(file) : parseFAQImportText(await file.text(), format);
      const result = await faq.upsert(knowledgeBaseId, { entries: imported, mode: importMode });
      setImportOpen(false); setImportFile(null); setImportPreview([]);
      // Vue FAQEntryManager.vue:2091-2165 — the strip replaces the message
      // and polls the backend task until completion.
      setImportTask({ task_id: result.task_id, kb_id: knowledgeBaseId, status: 'processing', progress: 0, processed: 0, total: imported.length });
      await load(false);
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
    finally { setImportBusy(false); }
  }
  async function exportEntries(format: 'csv' | 'json') {
    setExportLoading(true);
    try { downloadText(await faq.exportEntries(knowledgeBaseId, format), format); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
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

  // Pending = KB type unresolved; blocked = redirect to the documents view has
  // fired. Either way no FAQ markup renders — Vue never mounts the manager.
  if (faqGate !== 'allowed') return null;

  return (
    <>
      <FAQPageView
      t={t}
      knowledgeBaseId={knowledgeBaseId}
      kbName={kb?.name ?? null}
      kbMeta={metaFromKB(kb)}
      kbInfo={kb as unknown as KBInfoPopoverKB | null}
      infoUserId={meId}
      infoSharedKb={infoShared.sharedKb}
      infoPermission={infoShared.permission}
      kbList={kbList}
      tags={tags}
      activeTagIds={activeTagIds}
      entries={entries}
      total={total}
      hasMore={hasMore}
      loadingMore={loadingMore}
      loading={loading}
      canContribute={canContribute}
      canManage={canManage}
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
      batchTagLoading={batchTagLoading}
      batchStatusAction={batchStatusAction}
      batchDeleteLoading={batchDeleteLoading}
      onBatchEnable={() => { setBatchStatusAction('enable'); void updateSelection({ is_enabled: true }).finally(() => setBatchStatusAction(null)); }}
      onBatchDisable={() => { setBatchStatusAction('disable'); void updateSelection({ is_enabled: false }).finally(() => setBatchStatusAction(null)); }}
      onClearSelection={() => setSelected(new Set())}
      onNavigate={navigate}
      batchTagOpen={batchTagOpen}
      batchTagValue={batchTagValue}
      batchTagBusy={batchTagBusy}
      onOpenBatchTag={() => { setBatchTagValue(''); setBatchTagOpen(true); }}
      onBatchTagValueChange={setBatchTagValue}
      onBatchTagConfirm={() => void confirmBatchTag()}
      onCloseBatchTag={() => setBatchTagOpen(false)}
      onBatchDelete={() => void removeMany([...selected]).then(() => setSelected(new Set()))}
      onKeywordDraftChange={setKeywordDraft}
      onSearchSubmit={() => setKeyword(keywordDraft.trim())}
      onSearchClear={() => { setKeywordDraft(''); setKeyword(''); }}
      onToggleTag={(tagId) => setActiveTagIds((current) => current.includes(tagId) ? current.filter((id) => id !== tagId) : [...current, tagId])}
      onClearTagFilter={() => setActiveTagIds([])}
      onOpenTagManage={() => setTagManageOpen(true)}
      onOpenCreate={() => openEditor()}
      onOpenImport={() => { setImportFile(null); setImportPreview([]); setImportOpen(true); }}
      onCloseImport={() => setImportOpen(false)}
      onImportModeChange={setImportMode}
      onImportFile={handleImportFile}
      onDownloadExample={downloadExampleFile}
      onImportConfirm={() => void confirmImport()}
      onExport={(format) => void exportEntries(format)}
      onToggleSelect={(id, checked) => setSelected((current) => { const next = new Set(current); if (checked) next.add(id); else next.delete(id); return next; })}
      onToggleSelectAll={(checked) => setSelected(checked ? new Set(entries.map((entry) => entry.id)) : new Set())}
      onEditEntry={openEditor}
      onDeleteEntry={(entry) => void removeMany([entry.id])}
      onEntryTagChange={(entryId, tagSeqId) => { void updateEntryTag(entryId, tagSeqId); }}
      onToggleEntryStatus={(entry, value) => void toggleEntryStatus(entry, value)}
      statusUpdatingIds={statusUpdatingIds}
      onLoadMore={loadMore}
      onCloseEditor={() => setEditing(undefined)}
      onFormChange={(patch) => setForm((current) => ({ ...current, ...patch }))}
      onEditorSubmit={() => void save()}
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
