import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import type { DragEvent, FocusEvent, FormEvent, ReactNode } from 'react';
import { createPortal } from 'react-dom';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, FAQImportProgress, KnowledgeBase, KnowledgeTag, WeKnoraClient } from '@weknora/api-client';
import * as XLSX from 'xlsx';
import { Button, Checkbox, Dialog, Input, Radio, Range, Select, Status, Textarea } from '@weknora/ui';
import { formatMessage, type Locale } from '@weknora/i18n';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { navigate as clientNavigate } from '../platform/navigation.ts';
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from '../knowledge/permissions.ts';
import { normalizeFAQPayload, parseExcelFile, parseFAQImportText, serializeFAQEntries } from './import-export.ts';
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

// --- R488 A3: Vue t-drawer exit semantics ----------------------------------------
// The Vue drawer transitions out over 0.28s (tdesign.css:17130-17158 transform
// transition on .t-drawer__content-wrapper) before hiding; React swaps the
// enter animation classes for the exit pair and defers the unmount by the same
// duration. renderToStaticMarkup mounts with the open value (no effects run),
// so SSR snapshots keep rendering the drawer.
export const FAQ_DRAWER_EXIT_MS = 280;
export function useDrawerExit(open: boolean, exitMs: number = FAQ_DRAWER_EXIT_MS): boolean {
  const [mounted, setMounted] = useState(open);
  useEffect(() => {
    if (open) {
      setMounted(true);
      return;
    }
    const timer = window.setTimeout(() => setMounted(false), exitMs);
    return () => window.clearTimeout(timer);
  }, [open, exitMs]);
  return mounted;
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

// Tailwind utilities for the Vue caret geometry: ::before = outline layer,
// ::after = fill layer; per-placement border-color + offset (faq-tag-tooltip
// pseudo rules, faq.css B5 block) expressed as arbitrary variants.
const TOOLTIP_CARET_UTIL = [
  "[&::before]:content-[''] [&::before]:absolute [&::before]:h-0 [&::before]:w-0 [&::before]:border-[5px] [&::before]:border-transparent",
  "[&::after]:content-[''] [&::after]:absolute [&::after]:h-0 [&::after]:w-0 [&::after]:border-[5px] [&::after]:border-transparent",
  "[&.placement-top::before]:bottom-[-10px] [&.placement-top::before]:left-1/2 [&.placement-top::before]:-translate-x-1/2 [&.placement-top::before]:border-t-[#e7ebf0]",
  "[&.placement-top::after]:bottom-[-9px] [&.placement-top::after]:left-1/2 [&.placement-top::after]:-translate-x-1/2 [&.placement-top::after]:border-t-white",
  "[&.placement-bottom::before]:top-[-10px] [&.placement-bottom::before]:left-1/2 [&.placement-bottom::before]:-translate-x-1/2 [&.placement-bottom::before]:border-b-[#e7ebf0]",
  "[&.placement-bottom::after]:top-[-9px] [&.placement-bottom::after]:left-1/2 [&.placement-bottom::after]:-translate-x-1/2 [&.placement-bottom::after]:border-b-white",
  "[&.placement-left::before]:right-[-10px] [&.placement-left::before]:top-1/2 [&.placement-left::before]:-translate-y-1/2 [&.placement-left::before]:border-l-[#e7ebf0]",
  "[&.placement-left::after]:right-[-9px] [&.placement-left::after]:top-1/2 [&.placement-left::after]:-translate-y-1/2 [&.placement-left::after]:border-l-white",
  "[&.placement-right::before]:left-[-10px] [&.placement-right::before]:top-1/2 [&.placement-right::before]:-translate-y-1/2 [&.placement-right::before]:border-r-[#e7ebf0]",
  "[&.placement-right::after]:left-[-9px] [&.placement-right::after]:top-1/2 [&.placement-right::after]:-translate-y-1/2 [&.placement-right::after]:border-r-white",
].join(' ');

// faq.css editor-control shared utilities (former .setting-control input/textarea/select
// + .full-width-input-wrapper descendant rules) hoisted so every control cites one source.
const EDITOR_CONTROL = 'box-border rounded-lg border border-[#cdd6e2] bg-surface px-2.5 py-2 text-sm leading-[1.5] text-ink font-[inherit] focus:border-accent-deep focus:outline-none';
const EDITOR_CONTROL_INLINE = 'box-border w-auto min-w-0 flex-1 rounded-lg border border-[#cdd6e2] bg-surface px-2.5 py-2 text-sm leading-[1.5] text-ink font-[inherit] focus:border-accent-deep focus:outline-none';

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
      className="faq-tag-wrapper relative inline-block max-w-full min-w-0 flex-[0_1_auto]"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onClick={(event) => { event.stopPropagation(); setOpen((current) => !current); }}
    >
      {children}
      {open ? createPortal(
        <div
          ref={bubbleRef}
          className={`faq-tag-tooltip tooltip-${type} placement-${resolvedPlacement} pointer-events-none fixed z-[9999] max-w-[320px] min-w-[100px] rounded-md border border-[#e7ebf0] bg-surface px-3.5 py-2.5 text-xs font-normal leading-[1.6] text-ink shadow-[0_0_8px_0_rgba(0,0,0,0.08)] [word-break:break-word] animate-[faq-tooltip-fade_0.15s_ease] ${TOOLTIP_CARET_UTIL}`}
          style={{ top: (position?.top ?? 0) + 'px', left: (position?.left ?? 0) + 'px' }}
          role="tooltip"
        >
          <span className="tooltip-content block whitespace-pre-wrap [word-break:break-word]">{content}</span>
        </div>,
        document.body,
      ) : null}
    </span>
  );
}

// --- R488 A3 residual: Vue tag t-select (FAQEntryManager.vue:559-564) ---------------
// A native <select> leaks every <option> text into innerText while the Vue
// t-select keeps its options inside a dropdown (K2 noise). This trigger +
// body-teleported listbox keeps the closed editor at the Vue text surface:
// one placeholder/label line, options only while open. Re-selecting the
// current tag clears the value (t-select clearable semantics).

export interface FAQTagSelectProps {
  id?: string;
  value: string;
  options: readonly { value: string; label: string }[];
  placeholder: string;
  ariaLabel?: string;
  onChange: (value: string) => void;
}

export function FAQTagSelect({ id, value, options, placeholder, ariaLabel, onChange }: FAQTagSelectProps) {
  const [open, setOpen] = useState(false);
  const [left, setLeft] = useState(0);
  const [top, setTop] = useState(0);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const listboxId = `faq-tag-options-${useId()}`;
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!triggerRef.current?.contains(event.target as Node)) setOpen(false); };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);
  useLayoutEffect(() => {
    if (!open) return;
    const box = triggerRef.current?.getBoundingClientRect();
    if (box) {
      setLeft(box.left);
      setTop(box.bottom + 4);
    }
  }, [open]);
  const selected = options.find((option) => option.value === value);
  return (
    <div className="relative w-full">
      <button
        ref={triggerRef}
        type="button"
        id={id}
        className={`${EDITOR_CONTROL} flex w-full cursor-pointer items-center justify-between gap-2 text-left`}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listboxId}
        aria-label={ariaLabel}
        data-value={value}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={(event) => { if (event.key === 'Escape') setOpen(false); }}
      >
        <span className="faq-tag-select-value truncate">{selected?.label ?? placeholder}</span>
        <span aria-hidden="true" className="ml-auto text-base leading-none text-faint">⌄</span>
      </button>
      {open ? createPortal(
        <div id={listboxId} role="listbox" aria-label={ariaLabel} className="fixed z-[1001] max-h-60 w-[var(--faq-tag-select-width,16rem)] min-w-[10rem] overflow-auto rounded-lg border border-[#e7e7e7] bg-surface p-1 shadow-[0_8px_24px_rgba(23,32,51,.14)] animate-[faq-tooltip-fade_0.15s_ease]" style={{ left: left + 'px', top: top + 'px', ['--faq-tag-select-width' as string]: ((triggerRef.current?.getBoundingClientRect().width ?? 256) + 'px') }}>
          {options.length === 0 ? <div className="px-2.5 py-2 text-[13px] leading-[1.5] text-faint">{placeholder}</div> : options.map((option) => (
            <button
              key={option.value}
              type="button"
              role="option"
              aria-selected={option.value === value}
              className={`m-0.5 flex w-full rounded-md border-0 px-2.5 py-2 text-left text-[13px] leading-[1.5] ${option.value === value ? 'bg-[rgba(0,168,112,0.12)] text-accent-deep' : 'bg-transparent text-ink hover:bg-[rgba(0,168,112,0.06)]'}`}
              onClick={() => { onChange(option.value === value ? '' : option.value); setOpen(false); }}
            >
              {option.label}
            </button>
          ))}
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
// Vue renders t-icon (TDesign two-tone glyphs, square line caps). Stroke-only
// ports of the exact TDesign path data keep the glyph outlines pixel-close
// without importing the icon font (parity: FAQEntryManager.vue toolbar/header).
function TIcon({ size = 16, className, children }: { size?: number; className?: string; children: ReactNode }) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true" focusable="false">{children}</svg>
  );
}
const Chevrons = {
  right: 'M9 18l6-6-6-6',
  down: 'M6 9l6 6 6-6',
  up: 'M18 15l-6-6-6 6',
};
// TDesign chevron-right / chevron-down (chevron-right.js / chevron-down.js).
const TChevrons = {
  right: 'M9.5 17.5L15 12L9.5 6.5',
  down: 'M17.5 9.5L12 15L6.5 9.5',
};
function SearchIcon(props: { size?: number; className?: string }) {
  // TDesign search.js stroke paths
  return <TIcon {...props}><path d="M15.8033 15.8033C12.8744 18.7322 8.12563 18.7322 5.1967 15.8033C2.26777 12.8744 2.26777 8.12563 5.1967 5.1967C8.12563 2.26777 12.8744 2.26777 15.8033 5.1967C18.7322 8.12563 18.7322 12.8744 15.8033 15.8033Z" /><path d="M15.8027 15.8037L21.106 21.107" /></TIcon>;
}
function GearIcon(props: { size?: number; className?: string }) {
  // TDesign setting.js stroke paths (hexagon nut + inner circle)
  return (
    <TIcon {...props}>
      <path d="M12.0001 2L20.6604 7V17L12.0001 22L3.33984 17V7L12.0001 2Z" />
      <path d="M16 12C16 14.2091 14.2091 16 12 16C9.79086 16 8 14.2091 8 12C8 9.79086 9.79086 8 12 8C14.2091 8 16 9.79086 16 12Z" />
    </TIcon>
  );
}
function AddIcon(props: { size?: number; className?: string }) { return <TIcon {...props}><path d="M12 5L12 19M19 12L5 12" /></TIcon>; }
function DownloadIcon(props: { size?: number; className?: string }) { return <TIcon {...props}><path d="M16.5 10.5L12 15L7.5 10.5M12 13.75V4" /><path d="M20.5 15V20H3.5V15" /></TIcon>; }
function InfoIcon(props: { size?: number; className?: string }) {
  // TDesign info-circle.js stroke paths
  return <TIcon {...props}><path d="M2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12Z" /><path d="M12 16.5L12 11M12 7.5L11.9961 7.5L11.9961 7.49609L12 7.49609L12 7.5Z" /></TIcon>;
}
function FileAddIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" /><path d="M14 2v6h6" /><path d="M12 18v-6M9 15h6" /></Icon>; }
function CloseIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M18 6L6 18M6 6l12 12" /></Icon>; }
function TagIcon(props: { size?: number; className?: string }) {
  // Vue uses t-icon name="discount" (FAQEntryManager.vue:177) — TDesign discount.js
  return <TIcon {...props}><path d="M11.878 22.0207L1.97852 12.1212L11.878 2.22168L21.0704 2.92879L21.7775 12.1212L11.878 22.0207Z" /><path d="M13.9998 7.17075C14.7809 6.3897 16.0472 6.3897 16.8283 7.17075C17.6093 7.9518 17.6093 9.21813 16.8283 9.99917C16.0472 10.7802 14.7809 10.7802 13.9998 9.99917C13.2188 9.21813 13.2188 7.9518 13.9998 7.17075Z" /></TIcon>;
}
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
    <div className="faq-title-row flex w-full flex-wrap items-center gap-2">
      <h2 className="faq-breadcrumb m-0 flex items-center gap-1.5 text-xl font-semibold leading-8 text-ink">
        <button type="button" className="breadcrumb-link -mx-2 -my-1 inline-flex cursor-pointer items-center gap-1 rounded-md border-0 bg-transparent px-2 py-1 text-xl font-semibold leading-8 text-muted [font-family:inherit] [transition:all_0.12s_ease] hover:enabled:bg-surface hover:enabled:text-accent-deep disabled:cursor-not-allowed disabled:text-faint" onClick={() => onNavigate(faqKBListPath)}>{t('menu.knowledgeBase')}</button>
        <TIcon size={14} className="faq-breadcrumb-separator shrink-0 text-faint"><path d={TChevrons.right} /></TIcon>
        <span className="faq-kb-switcher relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setSwitcherOpen(false))}>
          <button type="button" className="breadcrumb-link dropdown group/dd -mx-2 -my-1 inline-flex cursor-pointer items-center gap-1 rounded-md border-0 bg-transparent py-1 pr-1.5 pl-2 text-xl font-semibold leading-8 text-muted [font-family:inherit] [transition:all_0.12s_ease] hover:enabled:bg-surface hover:enabled:text-accent-deep disabled:cursor-not-allowed disabled:text-faint" aria-haspopup="menu" aria-expanded={switcherOpen} onClick={() => setSwitcherOpen((open) => !open)}>
            <span>{kbName ?? '…'}</span>
            <TIcon size={14} className="breadcrumb-caret shrink-0 transition-transform duration-[120ms] ease-[ease] group-hover/dd:translate-y-[1px]"><path d={TChevrons.down} /></TIcon>
          </button>
          <span className="faq-menu faq-switcher-menu absolute top-[calc(100%+6px)] left-0 right-auto z-[210] flex min-w-[200px] max-h-[320px] flex-col overflow-auto rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={!switcherOpen}>
            {kbList.map((kb) => (
              <button key={kb.id} type="button" role="menuitem" className={'faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] hover:bg-surface-alt ' + (kb.id === knowledgeBaseId ? 'is-active font-semibold text-accent-deep' : 'text-ink')} onClick={() => { setSwitcherOpen(false); onNavigate(faqKBDetailPath(kb.id)); }}>
                {kb.name}
              </button>
            ))}
          </span>
        </span>
        <TIcon size={14} className="faq-breadcrumb-separator shrink-0 text-faint"><path d={TChevrons.right} /></TIcon>
        <span className="breadcrumb-current text-xl font-semibold leading-8 text-ink">{t('knowledgeEditor.faq.title')}</span>
      </h2>
      <div className="faq-kb-title-actions inline-flex shrink-0 items-center gap-1.5">
        <span className="kb-info-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setInfoOpen(false))}>
          <button type="button" className="faq-kb-info-button inline-flex h-[26px] w-[26px] cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-black/40 [transition:all_0.2s_ease] hover:bg-[#f3f3f3] hover:text-accent-deep" aria-label={t('knowledgeBase.infoCard.tooltip')} title={t('knowledgeBase.infoCard.tooltip')} aria-expanded={infoOpen} onClick={() => setInfoOpen((open) => !open)}>
            <InfoIcon size={16} />
          </button>
          <span className="faq-kb-info-card absolute top-[calc(100%+8px)] right-0 z-[200] flex w-[320px] flex-col gap-2.5 rounded-[10px] border border-[#e3e8f0] bg-surface px-4 py-3.5 text-left text-[13px] leading-[1.5] shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" hidden={!infoOpen}>
            <span className="faq-kb-info-card-header border-b border-[#e3e8f0] pb-2 text-sm font-semibold leading-[1.5] text-ink">{t('knowledgeBase.infoCard.title')}</span>
            <span className="kb-info-card-row flex items-baseline gap-3"><span className="faq-kb-info-card-label w-16 shrink-0 text-faint">{t('knowledgeBase.infoCard.type')}</span><span className="kb-info-card-value text-ink [word-break:break-word]">{kbMeta?.type?.toLowerCase() === 'faq' ? t('knowledgeEditor.basic.typeFAQ') : t('knowledgeEditor.basic.typeDocument')}</span></span>
            {kbMeta?.description ? <span className="kb-info-card-row flex items-baseline gap-3"><span className="faq-kb-info-card-label w-16 shrink-0 text-faint">{t('knowledgeBase.description')}</span><span className="kb-info-card-value text-ink [word-break:break-word]">{kbMeta.description}</span></span> : null}
            {kbMeta?.createdAt ? <span className="kb-info-card-row flex items-baseline gap-3"><span className="faq-kb-info-card-label w-16 shrink-0 text-faint">{t('knowledgeBase.infoCard.createdAt')}</span><span className="kb-info-card-value text-ink [word-break:break-word]">{kbMeta.createdAt}</span></span> : null}
          </span>
        </span>
        <button type="button" className="faq-kb-settings-button inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-full border-0 bg-[#f3f3f3] p-0 text-black/60 [transition:all_0.2s_ease] hover:bg-[#e8e8e8] hover:text-accent-deep" aria-label={t('knowledgeBase.settings')} title={t('knowledgeBase.settings')} disabled={!knowledgeBaseId} onClick={() => knowledgeBaseId && onNavigate(faqKBSettingsPath(knowledgeBaseId))}>
          <GearIcon size={16} />
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
  /** R491 1b: Vue t-popconfirm around 批量删除 (FAQBatchBar.vue:65-74). */
  confirmingBatchDelete?: boolean;
  onConfirmBatchDelete?: () => void;
  onCancelBatchDelete?: () => void;
  onNavigate?: (path: string) => void;
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
  /** Vue handleEntryStatusChange — per-card enable/disable toggle. */
  onToggleEntryStatus?: (entry: FAQEntry, value: boolean) => void;
  onBatchEnable?: () => void;
  onBatchDisable?: () => void;
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
    canContribute = false,
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
    confirmingBatchDelete = false,
    onConfirmBatchDelete = () => {},
    onCancelBatchDelete = () => {},
    onNavigate = defaultNavigate,
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
  const [masonryRevision, setMasonryRevision] = useState(0);
  const [createMenuOpen, setCreateMenuOpen] = useState(false);
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const [exampleMenuOpen, setExampleMenuOpen] = useState(false);
  const [collapsedSections, setCollapsedSections] = useState<FAQSectionCollapseState>({});
  // Vue entry.showMore — one open card more-menu at a time (FAQEntryManager.vue:265-283).
  const [moreMenuId, setMoreMenuId] = useState<number | null>(null);
  // B4: Vue stores expanded on each hit with default false (:2669) — a per-id set.
  const [expandedResults, setExpandedResults] = useState<ReadonlySet<number>>(new Set());
  // R488 A3: both drawers ride the Vue t-drawer enter/exit motion (slide from
  // translateX(100%) + overlay fade, 0.28s) instead of vanishing on close.
  const editorMounted = useDrawerExit(Boolean(editorOpen));
  const searchMounted = useDrawerExit(Boolean(searchOpen));
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
  const activeTagLabel = activeTagIds.length === 0
    ? t('knowledgeBase.allTags')
    : activeTagIds.length === 1
      ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
      : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
  const visibleTags = filterFaqTags(tags, tagSearchQuery);
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
    <main className="faq-view m-0 box-border max-w-none ml-[4px] mr-[16px] px-8 pt-6 pb-12 max-md:p-4">
      <header className="faq-header mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="faq-header-title flex w-full flex-col gap-1">
          <FAQBreadcrumb t={t} knowledgeBaseId={knowledgeBaseId} kbName={kbName} kbList={kbList} kbMeta={kbMeta} onNavigate={onNavigate} />
          <p className="faq-subtitle m-0 text-sm font-normal leading-5 text-faint">{t('knowledgeEditor.faq.subtitle')}</p>
          {importTask ? (
            <div className={'faq-import-strip faq-import-strip--' + importTask.status + ' inline-flex w-fit max-w-full items-center gap-2 mt-0.5 rounded-md border px-2 py-1 pl-2.5 text-xs leading-[1.4] text-muted ' + (importTask.status === 'failed' ? 'border-[rgba(227,77,89,0.3)] bg-[rgba(227,77,89,0.06)]' : 'border-line-soft bg-surface-alt')} role="status">
              <span className={'faq-import-strip__icon block h-2 w-2 shrink-0 rounded-full ' + (importTask.status === 'running' ? 'animate-[faq-import-spin_1s_linear_infinite] bg-accent-deep' : importTask.status === 'success' ? 'bg-[#0f8a5f]' : importTask.status === 'failed' ? 'bg-[#e34d59]' : 'bg-faint')} aria-hidden="true" />
              <span className={'faq-import-strip__text min-w-0 max-w-[520px] flex-[0_1_auto] truncate ' + (importTask.status === 'failed' ? 'text-[#e34d59]' : '')}>{importTask.text}</span>
              <span className="faq-import-strip__bar h-1 w-[72px] shrink-0 overflow-hidden rounded-[2px] bg-black/[0.08]"><span className={'faq-import-strip__bar-fill block h-full rounded-[2px] [transition:width_0.3s_ease] ' + (importTask.status === 'success' ? 'bg-[#0f8a5f]' : importTask.status === 'failed' ? 'bg-[#e34d59]' : 'bg-accent-deep')} style={{ width: importTask.progress + '%' }} /></span>
              <span className="faq-import-strip__count shrink-0 text-xs leading-[1.4] text-faint tabular-nums">{importTask.processed}/{importTask.total}</span>
            </div>
          ) : null}
          {faqImportResultVisible(importResult, Boolean(importTask)) ? (
            // B6: Vue 导入结果持久化条 (:52-79) — summary + mode tag + failed
            // entries link + time + close; persists until closed or replaced.
            <div className="faq-import-strip faq-import-strip--result inline-flex w-fit max-w-full items-center gap-2 mt-0.5 rounded-md border border-[rgba(15,138,95,0.25)] bg-[rgba(0,168,112,0.06)] px-2 py-1 pl-2.5 text-xs leading-[1.4] text-muted" role="status">
              <span className="faq-import-strip__text min-w-0 max-w-[520px] flex-[0_1_auto] truncate text-ink">{faqImportResultSummary(importResult!, t)}</span>
              <span className={'faq-import-mode-tag shrink-0 rounded-full border px-2 py-px text-xs leading-[1.6] ' + (importResult!.import_mode === 'append' ? 'is-append border-[rgba(0,168,112,0.25)] bg-[rgba(0,168,112,0.1)] text-[#0f8a5f]' : 'is-replace border-[rgba(232,150,18,0.3)] bg-[rgba(232,150,18,0.12)] text-[#b26a00]')}>
                {importResult!.import_mode === 'append' ? t('FAQ.import.appendMode') : t('FAQ.import.replaceMode')}
              </span>
              {importResult!.failed_entries_url && importResult!.failed_count > 0 ? (
                <button type="button" className="faq-import-strip__link shrink-0 cursor-pointer border-0 bg-none p-0 text-xs leading-[1.4] text-[#e34d59] hover:underline" onClick={onDownloadFailedEntries}>{t('FAQ.import.downloadReasons')}</button>
              ) : null}
              <span className="faq-import-strip__time shrink-0 text-faint tabular-nums">{formatImportTime(importResult!.imported_at)}</span>
              <button type="button" className="faq-import-strip__close inline-flex h-[18px] w-[18px] shrink-0 cursor-pointer items-center justify-center rounded-[4px] border-0 bg-none p-0 text-faint hover:bg-black/5 hover:text-ink" aria-label={t('common.close')} title={t('common.close')} onClick={onCloseImportResult}><CloseIcon size={12} /></button>
            </div>
          ) : null}
        </div>
      </header>

      <div className="faq-main mt-2 flex min-h-0 flex-1">
        <div className="faq-card-area relative flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="faq-filter-bar flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 pb-3">
            <div className="faq-search-input relative flex min-w-0 items-center flex-[1_1_220px] max-md:flex-[1_1_100%]">
              <SearchIcon size={16} className="faq-search-icon pointer-events-none absolute left-[9px] text-faint" />
              <Input
                type="search"
                className="h-8 w-full appearance-none box-border rounded-md border border-transparent bg-surface-alt pl-[25px] pr-3 py-0 text-sm leading-[22px] text-ink font-[inherit] outline-none [transition:background_0.2s_ease,border-color_0.2s_ease] hover:border-accent-deep hover:bg-surface focus:border-accent-deep focus:bg-surface [&::-webkit-search-cancel-button]:hidden"
                value={keywordDraft}
                placeholder={t('knowledgeEditor.faq.searchPlaceholder')}
                aria-label={t('knowledgeBase.faq.search')}
                onChange={(event) => onKeywordDraftChange(event.target.value)}
                onKeyDown={(event) => { if (event.key === 'Enter') onSearchSubmit(); }}
              />
              {keywordDraft ? <button type="button" className="faq-search-clear absolute right-2 inline-flex h-5 w-5 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-faint hover:text-ink" aria-label={t('common.close')} onClick={onSearchClear}><CloseIcon size={14} /></button> : null}
            </div>
            <div className="faq-filter-bar__filters flex min-w-0 flex-none items-center gap-3 max-md:flex-[1_1_auto]">
              <span className="faq-filter-field relative inline-flex w-[140px] shrink-0 max-md:w-auto max-md:min-w-[120px] max-md:flex-[1_1_auto]" onBlur={(event) => closeOnBlur(event, () => setTagPanelOpen(false))}>
                <button type="button" className="faq-tag-filter-trigger inline-flex h-8 w-full cursor-pointer items-center box-border rounded-lg border border-transparent bg-surface-alt px-2 py-0 text-sm leading-none text-ink font-[inherit] [transition:background_0.2s_ease,border-color_0.2s_ease] hover:bg-[#e8ecf3]" aria-label={t('knowledgeBase.tagFilterTitle')} title={t('knowledgeBase.tagFilterTitle')} aria-haspopup="menu" aria-expanded={tagPanelOpen} onClick={() => setTagPanelOpen((open) => !open)}>
                  <span className="doc-tag-filter-trigger__prefix mr-2 inline-flex shrink-0 items-center text-faint"><TagIcon size={16} /></span>
                  <span className="doc-tag-filter-trigger__label min-w-0 flex-1 truncate text-left">{activeTagLabel}</span>
                  <span className="doc-tag-filter-trigger__suffix ml-2 inline-flex shrink-0 items-center">
                    {activeTagIds.length > 0 ? <span role="button" tabIndex={0} className="faq-tag-filter-clear inline-flex h-5 w-5 cursor-pointer items-center justify-center rounded-full text-faint hover:text-ink" aria-label={t('common.clear')} onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); onClearTagFilter(); }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); setTagPanelOpen(false); onClearTagFilter(); } }}><CloseIcon size={13} /></span> : <TIcon size={16} className="faq-tag-filter-trigger__caret shrink-0 text-faint"><path d={TChevrons.down} /></TIcon>}
                  </span>
                </button>
                <span className="faq-menu faq-tag-filter-panel absolute top-[calc(100%+6px)] left-0 right-auto z-[210] flex w-[260px] min-w-[260px] flex-col gap-2 rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" hidden={!tagPanelOpen}>
                  <span className="tag-filter-panel__header flex items-center gap-1 px-1.5 py-1 text-[13px] font-semibold leading-[1.5] text-ink">
                    <span>{t('knowledgeBase.tagFilterTitle')}</span>
                    <span className="tag-filter-panel__count font-normal text-faint">({tags.length})</span>
                  </span>
                  <label className="tag-search-bar relative block px-1.5">
                    <SearchIcon size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
                    <Input className="h-8 w-full pl-8 text-xs" value={tagSearchQuery} placeholder={t('knowledgeBase.tagSearchPlaceholder')} aria-label={t('knowledgeBase.tagSearchPlaceholder')} onChange={(event) => setTagSearchQuery(event.target.value)} />
                  </label>
                  <span className="faq-tag-filter-chips flex max-h-[260px] flex-wrap gap-1.5 overflow-auto px-1 pt-0.5 pb-1.5">
                    {visibleTags.map((tag) => (
                      <button key={tag.id} type="button" className={'faq-tag-filter-chip inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-[3px] text-xs leading-[1.5] font-[inherit] hover:border-accent-deep ' + (activeTagIds.includes(tag.id) ? 'active border-accent-deep bg-[rgba(0,168,112,0.08)] text-accent-deep' : 'border-[#e3e8f0] bg-surface text-ink')} title={tag.name + ' (' + (tag.chunk_count || 0) + ')'} onClick={() => onToggleTag(tag.id)}>
                        <span className="tag-filter-chip__label">{tag.name}</span>
                        <span className="faq-tag-filter-chip__count text-faint">{tag.chunk_count || 0}</span>
                      </button>
                    ))}
                    {visibleTags.length === 0 ? <span className="tag-empty-state p-1.5 text-[13px] leading-[1.5] text-faint">{t('knowledgeBase.tagEmptyResult')}</span> : null}
                  </span>
                  {canContribute ? <button type="button" className="tag-filter-panel__manage self-start cursor-pointer border-0 bg-transparent px-1.5 py-0.5 text-xs leading-[1.5] text-accent-deep font-[inherit] hover:underline" onClick={onOpenTagManage}>{t('knowledgeBase.tagManageLink')}</button> : null}
                </span>
              </span>
            </div>
          <div className="faq-filter-bar__trailing ml-auto flex flex-none items-center gap-1">
              {canContribute ? (
                <span className="faq-icon-menu-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setCreateMenuOpen(false))}>
                  <button type="button" className="content-bar-icon-btn inline-flex h-6 w-[30px] cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-black/60 [transition:all_0.15s_ease] hover:enabled:bg-[#eef1f6] hover:enabled:text-accent-deep disabled:cursor-default disabled:opacity-60" aria-label={t('knowledgeEditor.faq.createGroup')} title={t('knowledgeEditor.faq.createGroup')} aria-haspopup="menu" aria-expanded={createMenuOpen} onClick={() => setCreateMenuOpen((open) => !open)}>
                    <AddIcon size={16} />
                  </button>
                  <span className="faq-menu faq-icon-menu absolute top-[calc(100%+6px)] right-0 z-[210] flex min-w-[140px] flex-col rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={!createMenuOpen}>
                    <button type="button" role="menuitem" className="faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-ink hover:bg-surface-alt" onClick={() => { setCreateMenuOpen(false); onOpenCreate(); }}>{t('knowledgeEditor.faq.editorCreate')}</button>
                    <button type="button" role="menuitem" className="faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-ink hover:bg-surface-alt" onClick={() => { setCreateMenuOpen(false); onOpenImport(); }}>{t('knowledgeEditor.faqImport.importButton')}</button>
                  </span>
                </span>
              ) : null}
              <span className="faq-icon-menu-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setExportMenuOpen(false))}>
                <button type="button" className="content-bar-icon-btn inline-flex h-6 w-[30px] cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-black/60 [transition:all_0.15s_ease] hover:enabled:bg-[#eef1f6] hover:enabled:text-accent-deep disabled:cursor-default disabled:opacity-60" aria-label={t('knowledgeEditor.faqExport.exportButton')} title={t('knowledgeEditor.faqExport.exportButton')} aria-haspopup="menu" aria-expanded={exportMenuOpen} disabled={exportLoading} onClick={() => setExportMenuOpen((open) => !open)}>
                  <DownloadIcon size={16} />
                </button>
                <span className="faq-menu faq-icon-menu absolute top-[calc(100%+6px)] right-0 z-[210] flex min-w-[140px] flex-col rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={!exportMenuOpen}>
                  <button type="button" role="menuitem" className="faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-ink hover:bg-surface-alt" onClick={() => { setExportMenuOpen(false); onExport('csv'); }}>{t('knowledgeEditor.faqExport.exportCSV')}</button>
                  <button type="button" role="menuitem" className="faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-ink hover:bg-surface-alt" onClick={() => { setExportMenuOpen(false); onExport('json'); }}>{t('knowledgeEditor.faqExport.exportJSON')}</button>
                </span>
              </span>
              <button type="button" className="content-bar-icon-btn inline-flex h-6 w-[30px] cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-black/60 [transition:all_0.15s_ease] hover:enabled:bg-[#eef1f6] hover:enabled:text-accent-deep disabled:cursor-default disabled:opacity-60" aria-label={t('knowledgeEditor.faq.searchTest')} title={t('knowledgeEditor.faq.searchTest')} onClick={onOpenSearchTest}>
                <SearchIcon size={16} />
              </button>
            </div>
          </div>

          {message ? <Status tone={message.tone}>{message.text}</Status> : null}

          <div className="faq-scroll-container relative min-h-0 flex-1 overflow-x-hidden overflow-y-auto pr-1" ref={scrollRef} onScroll={handleContainerScroll}>
            {loading && entries.length === 0 ? (
              <div className="faq-skeleton-grid grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3" aria-hidden="true">
                {Array.from({ length: 6 }, (_, index) => <div key={index} className="faq-card-skeleton h-[120px] animate-[faq-shimmer_1.4s_ease_infinite] rounded-[10px] bg-[linear-gradient(100deg,#eef1f6_40%,#f7f9fc_50%,#eef1f6_60%)] bg-[length:200%_100%]" />)}
              </div>
            ) : entries.length > 0 ? (
              <div ref={cardListRef} className="faq-card-list relative min-w-0">
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
                    const tagClass = name === 'negative'
                      ? 'question-tag is-negative max-w-full overflow-hidden text-ellipsis whitespace-nowrap rounded-[5px] border border-[rgba(227,115,24,0.5)] bg-surface px-2 py-[3px] text-[11px] leading-[1.5] text-[#b45309]'
                      : name === 'answers'
                        ? 'question-tag is-answer max-w-full overflow-hidden text-ellipsis whitespace-nowrap rounded-[5px] border border-[rgba(0,168,112,0.45)] bg-surface px-2 py-[3px] text-[11px] leading-[1.5] text-accent-deep'
                        : 'question-tag max-w-full overflow-hidden text-ellipsis whitespace-nowrap rounded-[5px] border border-[#cdd6e2] bg-surface px-2 py-[3px] text-[11px] leading-[1.5] text-ink';
                    return <section className={'faq-section ' + name + ' flex min-w-0 flex-col gap-1.5'} key={name}>
                      <button type="button" className={"faq-section-label clickable flex cursor-pointer select-none items-center gap-[5px] border-0 bg-transparent px-0 py-0.5 m-0 text-left text-[11px] font-semibold uppercase leading-[1.5] tracking-[0.5px] text-muted font-[inherit] hover:text-ink [&::before]:content-[''] [&::before]:w-[3px] [&::before]:h-[10px] [&::before]:shrink-0 [&::before]:rounded-[2px] " + (name === 'negative' ? '[&::before]:bg-warning' : '[&::before]:bg-accent-deep')} {...sectionButton(entry.id, name)}>
                        <span>{t(labelKey)}</span>
                        <span className="section-count ml-1 font-normal text-faint">({values.length})</span>
                        <TIcon size={13} className="collapse-icon ml-auto shrink-0 text-faint"><path d={collapsed ? TChevrons.right : TChevrons.down} /></TIcon>
                      </button>
                      {/* B5: Vue wraps each chip in FAQTagTooltip (:303-354) — the
                          bubble carries the full text instead of a native title. */}
                      {/* R491 1a: the `flex` utility (display:flex) would override the UA
                          [hidden]{display:none} and leak answer bodies on load — guard with
                          [&[hidden]]:hidden like every faq-menu here (Vue :348 v-if). */}
                      <div className="faq-tags flex min-h-[18px] min-w-0 w-full flex-wrap gap-[5px] [&>*]:max-w-full [&>*]:min-w-0 [&>*]:flex-[0_1_auto] [&[hidden]]:hidden" hidden={collapsed}>
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
                      className={'faq-card flex min-w-0 max-w-full flex-col gap-1.5 overflow-hidden box-border rounded-[10px] border px-2.5 py-2.5 [transition:border-color_0.2s_ease,box-shadow_0.2s_ease,background-color_0.2s_ease] '
                        + (isSelected
                          ? 'selected border-accent-deep bg-[rgba(0,168,112,0.06)] shadow-[0_2px_8px_rgba(0,168,112,0.15)]'
                          : 'border-[#e3e8f0] bg-surface shadow-[0_1px_3px_rgba(15,23,42,0.05)]')
                        + (canContribute ? ' is-selectable cursor-pointer hover:border-accent-deep hover:shadow-[0_2px_8px_rgba(0,168,112,0.1)]' : '')}
                      onClick={canContribute ? () => onToggleSelect(entry.id, !isSelected) : undefined}
                    >
                      <div className="faq-card-header border-b border-line-soft pb-2.5">
                        <div className="faq-header-top flex items-start gap-2.5">
                          {canContribute ? (
                            <label className="faq-card-check pointer-events-none absolute h-0 w-0 overflow-hidden opacity-0" onClick={(event) => event.stopPropagation()}>
                              <Checkbox className="m-0 accent-accent-deep" checked={isSelected} aria-label={entry.standard_question} onChange={(event) => onToggleSelect(entry.id, event.target.checked)} />
                            </label>
                          ) : null}
                          <strong className="faq-question min-w-0 flex-1 overflow-hidden text-[15px] font-semibold leading-[1.5] text-ink [word-break:break-word] line-clamp-2" title={entry.standard_question}>{entry.standard_question}</strong>
                          {canContribute ? (
                            <span className="faq-more-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setMoreMenuId(null))} onClick={(event) => event.stopPropagation()}>
                              <button
                                type="button"
                                className="card-more-btn flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-muted opacity-60 hover:bg-surface-alt hover:opacity-100 aria-expanded:bg-surface-alt aria-expanded:opacity-100"
                                aria-label={t('knowledgeBase.columnActions')}
                                title={t('knowledgeBase.columnActions')}
                                aria-haspopup="menu"
                                aria-expanded={moreMenuId === entry.id}
                                onClick={() => setMoreMenuId((current) => (current === entry.id ? null : entry.id))}
                              >
                                <MoreIcon size={16} />
                              </button>
                              {/* Vue popup-menu (:271-282): edit then delete */}
                              <span className="faq-menu card-more-popup absolute top-[calc(100%+6px)] right-0 z-[210] flex min-w-[140px] flex-col rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={moreMenuId !== entry.id}>
                                <button type="button" role="menuitem" className="faq-menu-item flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-ink hover:bg-surface-alt" onClick={() => { setMoreMenuId(null); onEditEntry(entry); }}>{t('common.edit')}</button>
                                <button type="button" role="menuitem" className="faq-menu-item is-danger flex w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm leading-[1.5] whitespace-nowrap font-[inherit] text-[#e34d59] hover:bg-[rgba(227,77,89,0.08)]" onClick={() => { setMoreMenuId(null); onDeleteEntry(entry); }}>{t('common.delete')}</button>
                              </span>
                            </span>
                          ) : null}
                        </div>
                      </div>
                      <div className="faq-card-body flex min-w-0 flex-1 flex-col gap-1.5">
                        {section('similar', 'knowledgeEditor.faq.similarQuestions', entry.similar_questions)}
                        {section('negative', 'knowledgeEditor.faq.negativeQuestions', entry.negative_questions)}
                        {section('answers', 'knowledgeEditor.faq.answers', entry.answers, true)}
                      </div>
                      <div className="faq-card-footer flex items-center justify-between gap-1.5 -mx-2.5 -mb-2.5 border-t border-line-soft bg-[rgba(48,50,54,0.02)] px-3 py-2">
                        <div className="faq-card-tag inline-flex min-w-0">
                          {/* B5 refine: the native title (d3a39b7b) is replaced by the
                              FAQTagTooltip bubble — hover opens the fixed, viewport-clamped
                              bubble with the full tag name; tag-text truncation stays. */}
                          <FaqTagTooltip content={tagName ?? t('knowledgeBase.untagged')} placement="top">
                            <span className="faq-tag-chip inline-flex max-w-[160px] items-center rounded-[5px] border border-[#cdd6e2] bg-surface px-2 py-0.5 text-[11px] leading-[1.5] text-muted"><span className="tag-text truncate">{tagName ?? t('knowledgeBase.untagged')}</span></span>
                          </FaqTagTooltip>
                        </div>
                        {canContribute ? (
                          <div className="faq-card-status inline-flex items-center" onClick={(event) => event.stopPropagation()}>
                            <button type="button" role="switch" aria-checked={entry.is_enabled} aria-label={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')} title={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')} className={'faq-status-switch relative h-4 w-[26px] shrink-0 cursor-pointer rounded-[8px] border-0 p-0 [transition:background_0.2s_ease] ' + (entry.is_enabled ? 'is-on bg-[#07c05f]' : 'bg-[#c9d0dd]') + ' disabled:cursor-not-allowed disabled:opacity-[0.55]'} disabled={statusUpdatingIds.includes(entry.id)} onClick={() => onToggleEntryStatus(entry, !entry.is_enabled)}><span className={'faq-status-switch__thumb absolute left-[2px] top-[2px] h-3 w-3 rounded-full bg-white shadow-[0_1px_2px_rgba(15,23,42,0.2)] [transition:transform_0.2s_ease]' + (entry.is_enabled ? ' translate-x-[10px]' : '')} /></button>
                          </div>
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : (
              <div className="faq-empty-state flex min-h-[400px] items-center justify-center px-5 py-[60px]">
                <div className="empty-content flex max-w-[400px] flex-col items-center gap-4 text-center">
                  <FileAddIcon size={48} className="empty-icon text-[#c0c6d4] opacity-60" />
                  <div className="empty-text text-lg font-semibold leading-7 text-ink">{t('knowledgeEditor.faq.emptyTitle')}</div>
                  <div className="empty-desc text-sm font-normal leading-[22px] text-muted">{t('knowledgeEditor.faq.emptyDesc')}</div>
                </div>
              </div>
            )}
            {loadingMore ? <div className="faq-load-more flex items-center justify-center px-4 py-6 text-[13px] leading-[1.5] text-muted">{t('common.loading')}</div> : null}
            {hasMore === false && entries.length > 0 ? <div className="faq-no-more flex items-center justify-center px-4 py-6 text-[13px] italic [line-height:normal] text-faint">{t('common.noMoreData')}</div> : null}
          </div>

          {/* R491 1b: Vue FAQBatchBar.vue:33-79 — 已选 N 项 + 取消选择 on the
              left; 批量设置标签 (dialog), conditional 启用/禁用 and 批量删除
              (confirm) on the right. The old React-only recommend button and
              inline tag select are gone. */}
          {canContribute && selected.size > 0 ? (
            <div className="wk-list-actions faq-batch-bar border-t border-line-soft pt-3 mb-[0.75rem] flex flex-wrap items-center justify-between gap-[0.5rem]" role="region" aria-label={t('knowledgeBase.selectedCount', { count: selected.size })}>
              <div className="faq-batch-bar__selection flex shrink-0 items-center gap-1">
                <span className="faq-batch-bar__count text-[13px] font-medium text-muted">{t('knowledgeBase.selectedCount', { count: selected.size })}</span>
                <Button type="button" variant="text" onClick={onClearSelection}>{t('knowledgeBase.clearSelection')}</Button>
              </div>
              <div className="faq-batch-bar__actions flex flex-wrap items-center justify-end gap-[0.5rem]">
                {/* Vue FAQBatchBar.vue:53-63 (+ FAQEntryManager.vue:1046-1049):
                    批量启用 only renders when the selection has disabled entries,
                    批量禁用 only when it has enabled ones. */}
                {(() => {
                  const selectedEntries = entries.filter((entry) => selected.has(entry.id));
                  const selectedEnabledCount = selectedEntries.filter((entry) => entry.is_enabled !== false).length;
                  const selectedDisabledCount = selectedEntries.length - selectedEnabledCount;
                  return (
                    <>
                      <Button type="button" onClick={onOpenBatchTag}>{t('knowledgeEditor.faq.batchUpdateTag')}</Button>
                      {selectedDisabledCount > 0 ? <Button type="button" onClick={onBatchEnable}>{t('knowledgeEditor.faq.batchEnable')}</Button> : null}
                      {selectedEnabledCount > 0 ? <Button type="button" onClick={onBatchDisable}>{t('knowledgeEditor.faq.batchDisable')}</Button> : null}
                    </>
                  );
                })()}
                <Button type="button" onClick={onBatchDelete}>{t('knowledgeEditor.faq.batchDelete')}</Button>
              </div>
            </div>
          ) : null}

        </div>
      </div>

      {importOpen ? (
        <section className="faq-import-overlay fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 p-5 [backdrop-filter:blur(4px)]" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faqImport.title')} onMouseDown={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>
          <div className="faq-import-modal relative flex max-h-[90vh] w-full max-w-[600px] flex-col overflow-hidden rounded-[12px] bg-surface shadow-[0_6px_28px_rgba(15,23,42,0.08)]">
            <button type="button" className="faq-modal-close absolute right-[18px] top-[18px] z-10 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0 bg-surface-alt text-muted hover:text-ink" aria-label={t('common.close')} onClick={onCloseImport}><CloseIcon size={16} /></button>
            <div className="faq-import-header shrink-0 border-b border-[#e3e8f0] px-6 pb-4 pt-6"><h2 className="m-0 text-lg font-semibold leading-[1.5] text-ink">{t('knowledgeEditor.faqImport.title')}</h2></div>
            <div className="faq-import-content flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-6">
              <div className="import-form-item flex flex-col gap-2.5">
                <label className="import-form-label text-sm font-semibold leading-[1.5] text-ink">{t('knowledgeEditor.faqImport.modeLabel')}</label>
                <div className="import-radio-group inline-flex gap-0" role="radiogroup" aria-label={t('knowledgeEditor.faqImport.modeLabel')}>
                  <label className={'import-radio-button inline-flex cursor-pointer select-none items-center gap-1.5 border bg-surface px-4 py-[7px] text-sm leading-[1.5] first:rounded-l-lg last:rounded-r-lg last:border-l-0 ' + (importMode === 'append' ? 'is-active border-accent-deep bg-[rgba(0,168,112,0.06)] text-accent-deep' : 'border-[#e3e8f0] text-muted')}>
                    <Radio name="faq-import-mode" className="m-0 accent-accent-deep" value="append" checked={importMode === 'append'} onChange={() => onImportModeChange('append')} /> {t('knowledgeEditor.faqImport.appendMode')}
                  </label>
                  <label className={'import-radio-button inline-flex cursor-pointer select-none items-center gap-1.5 border bg-surface px-4 py-[7px] text-sm leading-[1.5] first:rounded-l-lg last:rounded-r-lg last:border-l-0 ' + (importMode === 'replace' ? 'is-active border-accent-deep bg-[rgba(0,168,112,0.06)] text-accent-deep' : 'border-[#e3e8f0] text-muted')}>
                    <Radio name="faq-import-mode" className="m-0 accent-accent-deep" value="replace" checked={importMode === 'replace'} onChange={() => onImportModeChange('replace')} /> {t('knowledgeEditor.faqImport.replaceMode')}
                  </label>
                </div>
              </div>
              <div className="import-form-item flex flex-col gap-2.5">
                <div className="file-label-row flex items-center justify-between gap-2">
                  <label className="import-form-label text-sm font-semibold leading-[1.5] text-ink">{t('knowledgeEditor.faqImport.fileLabel')}</label>
                  <span className="faq-example-menu-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setExampleMenuOpen(false))}>
                    <button type="button" className="download-example-btn inline-flex cursor-pointer items-center gap-1 rounded-md border border-line-control bg-surface px-2 py-1 text-xs leading-[1.5] text-muted hover:border-accent-deep hover:text-accent-deep" aria-haspopup="menu" aria-expanded={exampleMenuOpen} onClick={() => setExampleMenuOpen((open) => !open)}><DownloadIcon size={14} />{t('knowledgeEditor.faqImport.downloadExample')}</button>
                    <span className="faq-menu absolute top-[calc(100%+6px)] right-0 z-[210] flex min-w-[180px] flex-col rounded-lg border border-[#e3e8f0] bg-surface p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={!exampleMenuOpen}>
                      <button type="button" role="menuitem" className="faq-menu-item w-full cursor-pointer rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm text-ink hover:bg-surface-alt" onClick={() => { setExampleMenuOpen(false); onDownloadExample('json'); }}>{t('knowledgeEditor.faqImport.downloadExampleJSON')}</button>
                      <button type="button" role="menuitem" className="faq-menu-item w-full cursor-pointer rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm text-ink hover:bg-surface-alt" onClick={() => { setExampleMenuOpen(false); onDownloadExample('csv'); }}>{t('knowledgeEditor.faqImport.downloadExampleCSV')}</button>
                      <button type="button" role="menuitem" className="faq-menu-item w-full cursor-pointer rounded-md border-0 bg-transparent px-3 py-2 text-left text-sm text-ink hover:bg-surface-alt" onClick={() => { setExampleMenuOpen(false); onDownloadExample('excel'); }}>{t('knowledgeEditor.faqImport.downloadExampleExcel')}</button>
                    </span>
                  </span>
                </div>
                <div
                  className="file-upload-area relative flex cursor-pointer flex-col items-center gap-2 rounded-[10px] border border-dashed border-[#cdd6e2] bg-[#fafbfd] px-4 py-7 text-center hover:border-accent-deep"
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event: DragEvent<HTMLDivElement>) => { event.preventDefault(); const file = event.dataTransfer.files?.[0]; if (file) onImportFile(file); }}
                >
                  <UploadIcon size={28} className="upload-icon text-faint" />
                  <span className="upload-primary-text break-all text-sm leading-[1.5] text-ink">{importFileName ?? t('knowledgeEditor.faqImport.clickToUpload')}</span>
                  {importFileName ? null : <span className="upload-secondary-text text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faqImport.dragDropTip')}</span>}
                  <input type="file" className="absolute inset-0 cursor-pointer opacity-0" accept=".json,.csv,.xlsx,.xls,application/json,text/csv" onChange={(event) => { const file = event.target.files?.[0]; if (file) onImportFile(file); event.target.value = ''; }} />
                </div>
                <p className="import-form-tip m-0 text-xs leading-[1.6] text-faint">{t('knowledgeEditor.faqImport.fileTip')}</p>
              </div>
              {message?.tone === 'error' || message?.tone === 'warning' ? <div className="faq-import-feedback mb-1" role="alert"><Status tone={message.tone}>{message.text}</Status></div> : null}
              {importPreview.length > 0 ? (
                <div className="import-preview mt-4 rounded-lg border border-line-soft bg-canvas p-4">
                  <div className="preview-header mb-3 flex items-center gap-2 border-b border-line-soft pb-3">
                    <span className="preview-icon inline-flex shrink-0 text-accent-deep" aria-hidden="true"><FileAddIcon size={16} /></span>
                    <span className="preview-title text-sm font-medium leading-[1.5] text-ink">{t('knowledgeEditor.faqImport.previewCount', { count: importPreview.length })}</span>
                  </div>
                  <div className="preview-list mb-2 flex flex-col gap-2">
                    {importPreview.slice(0, 5).map((item, index) => (
                      <div key={index} className="preview-item flex items-start gap-3 rounded-md border border-line-soft bg-surface px-3 py-2.5 [transition:border-color_0.2s_ease] hover:border-accent-deep">
                        <span className="preview-index min-w-5 shrink-0 rounded-[4px] bg-surface-alt text-center text-xs leading-5 text-muted">{index + 1}</span>
                        <span className="preview-question min-w-0 text-[13px] leading-[1.5] text-ink [word-break:break-word]">{item.standard_question}</span>
                      </div>
                    ))}
                  </div>
                  {importPreview.length > 5 ? <p className="preview-more m-0 mt-1 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faqImport.previewMore', { count: importPreview.length - 5 })}</p> : null}
                </div>
              ) : null}
            </div>
            <div className="faq-import-footer flex flex-none justify-end gap-2.5 border-t border-[#e3e8f0] px-6 py-4">
              <Button type="button" onClick={onCloseImport}>{t('common.cancel')}</Button>
              <Button type="button" loading={importBusy} disabled={importBusy} onClick={onImportConfirm}>{t('knowledgeEditor.faqImport.importButton')}</Button>
            </div>
          </div>
        </section>
      ) : null}

      {/* R491 1c: Vue batch-tag overlay (FAQEntryManager.vue:685-735) — title,
          info tip with the selection count, one tag select (clearable via the
          placeholder option), and a cancel/confirm footer with loading. */}
      {batchTagOpen ? (
        <section className="batch-tag-overlay fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 p-5 [backdrop-filter:blur(4px)]" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faq.batchUpdateTag')} onMouseDown={(event) => { if (event.target === event.currentTarget) onCloseBatchTag(); }}>
          <div className="batch-tag-modal relative flex w-full max-w-[420px] flex-col overflow-hidden rounded-[12px] bg-surface shadow-[0_6px_28px_rgba(15,23,42,0.08)]">
            <button type="button" className="batch-tag-close-btn absolute right-[18px] top-[18px] z-10 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0 bg-surface-alt text-muted hover:text-ink" aria-label={t('common.close')} onClick={onCloseBatchTag}><CloseIcon size={16} /></button>
            <div className="batch-tag-header shrink-0 border-b border-[#e3e8f0] px-6 pb-4 pt-6">
              <h2 className="m-0 text-lg font-semibold leading-[1.5] text-ink">{t('knowledgeEditor.faq.batchUpdateTag')}</h2>
            </div>
            <div className="batch-tag-content flex flex-col gap-4 p-6">
              <div className="batch-tag-tip flex items-start gap-2 text-[13px] leading-[1.5] text-muted">
                <InfoIcon size={16} className="mt-0.5 shrink-0" />
                <span>{t('knowledgeEditor.faq.batchUpdateTagTip', { count: selected.size })}</span>
              </div>
              <div className="batch-tag-form flex flex-col gap-2">
                <label className="text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-batch-tag-select">{t('knowledgeBase.tagLabel')}</label>
                {/* Vue t-select clearable: the placeholder option maps back to
                    null → updates clear the tag (handleBatchTag :1810-1821). */}
                <Select id="faq-batch-tag-select" className="w-full rounded-control border border-line-control bg-surface px-[0.65rem] py-[0.55rem] text-sm text-ink [font:inherit]" value={batchTagValue} onChange={(event) => onBatchTagValueChange(event.target.value)} aria-label={t('knowledgeBase.tagLabel')}>
                  <option value="">{t('knowledgeBase.tagPlaceholder')}</option>
                  {[...tagNameBySeq.entries()].map(([seqId, name]) => <option key={seqId} value={String(seqId)}>{name}</option>)}
                </Select>
                {tagNameBySeq.size === 0 ? <p className="tag-select-empty m-0 text-xs leading-[1.5] text-faint">{t('knowledgeBase.noTags')}</p> : null}
              </div>
            </div>
            <div className="batch-tag-footer flex flex-none justify-end gap-2.5 border-t border-[#e3e8f0] px-6 py-4">
              <Button type="button" onClick={onCloseBatchTag}>{t('common.cancel')}</Button>
              <Button type="button" variant="primary" loading={batchTagBusy} disabled={batchTagBusy} onClick={onBatchTagConfirm}>{t('common.confirm')}</Button>
            </div>
          </div>
        </section>
      ) : null}

      {/* R491 1b: Vue wraps 批量删除 in a t-popconfirm (FAQBatchBar.vue:65-74)
          with confirmBatchDelete / knowledgeBase.confirmDelete / common.cancel —
          React renders the same copy in a Dialog (repo convention). */}
      {confirmingBatchDelete ? (
        <Dialog open title={t('knowledgeEditor.faq.batchDelete')} onClose={onCancelBatchDelete}>
          <p className="m-0">{t('knowledgeEditor.faq.confirmBatchDelete', { count: selected.size })}</p>
          <div className="wk-list-actions mb-[0.75rem] mt-4 flex items-center justify-end gap-[0.5rem]">
            <Button type="button" onClick={onConfirmBatchDelete}>{t('knowledgeBase.confirmDelete')}</Button>
            <Button type="button" onClick={onCancelBatchDelete}>{t('common.cancel')}</Button>
          </div>
        </Dialog>
      ) : null}

      {/* B1: Vue editor drawer (FAQEntryManager.vue:440-577) — 520px right
          drawer, one settings-row per field with the shared desc keys, list
          editors with add/remove, and a pinned cancel/save footer. R488 A3:
          enter/exit slide+fade per the Vue t-drawer motion. */}
      {editorMounted ? (
        <section className={`faq-editor-overlay fixed inset-0 z-[1000] flex items-stretch justify-end bg-black/50 p-0 [backdrop-filter:blur(4px)] ${editorOpen ? 'faq-drawer-overlay-enter' : 'faq-drawer-overlay-exit'}`} role="dialog" aria-modal="true" aria-label={editorTitle} onMouseDown={(event) => { if (event.target === event.currentTarget) onCloseEditor(); }}>
          <aside className={`faq-editor-drawer flex h-full w-[520px] max-w-[92vw] flex-col overflow-hidden bg-surface shadow-[-8px_0_28px_rgba(15,23,42,0.16)] max-md:w-screen max-md:max-w-[100vw] ${editorOpen ? 'faq-drawer-panel-enter' : 'faq-drawer-panel-exit'}`}>
            <div className="faq-editor-header flex items-center justify-between border-b border-[#e3e8f0] px-5 py-[18px]">
              <h2 className="m-0 text-lg font-semibold leading-[1.5] text-ink">{editorTitle}</h2>
              <button type="button" className="faq-modal-close static z-10 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0 bg-surface-alt text-muted hover:text-ink" aria-label={t('common.close')} onClick={onCloseEditor}><CloseIcon size={16} /></button>
            </div>
            <form className="faq-editor-form min-h-0 flex-1 flex flex-col" onSubmit={onEditorSubmit}>
              <div className="faq-editor-form-body min-h-0 flex-1 overflow-x-hidden overflow-y-auto p-5">
                {message?.tone === 'error' ? <div className="faq-editor-error mb-2.5" role="alert"><Status tone="error">{message.text}</Status></div> : null}
                <div className="settings-group flex flex-col gap-[18px]">
                  <div className="setting-row flex flex-col gap-2">
                    <div className="setting-info flex flex-col gap-0.5">
                      <label className="required-label text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-editor-question">{t('knowledgeEditor.faq.standardQuestion')} <span className="required-mark ml-0.5 text-[#e34d59]">*</span></label>
                      <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.standardQuestionDesc')}</p>
                    </div>
                    <div className="setting-control flex flex-col gap-2">
                      <Input id="faq-editor-question" className={'full-width-input w-full ' + EDITOR_CONTROL} maxLength={200} value={form.question} onChange={(event) => onFormChange({ question: event.target.value })} />
                    </div>
                  </div>
                  <div className="setting-row setting-row-optional setting-row-similar flex flex-col gap-2">
                    <div className="setting-info flex flex-col gap-0.5">
                      <label className="optional-label text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-editor-similar">{t('knowledgeEditor.faq.similarQuestions')}</label>
                      <p className="desc optional-desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.similarQuestionsDesc')}</p>
                    </div>
                    <div className="setting-control flex flex-col gap-2">
                      <div className="full-width-input-wrapper flex w-full items-center gap-2">
                        <Input
                          id="faq-editor-similar"
                          className={'full-width-input ' + EDITOR_CONTROL_INLINE}
                          placeholder={t('knowledgeEditor.faq.similarPlaceholder')}
                          value={form.similarDraft}
                          onChange={(event) => onFormChange({ similarDraft: event.target.value })}
                          onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addSimilar(); } }}
                        />
                        <button type="button" className="add-item-btn inline-flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-accent-deep bg-transparent text-accent-deep hover:enabled:bg-[rgba(0,168,112,0.06)] disabled:cursor-not-allowed disabled:opacity-[0.45]" aria-label={t('knowledgeEditor.faq.similarQuestions')} disabled={!form.similarDraft.trim() || form.similarQuestions.length >= FAQ_SIMILAR_CAP} onClick={addSimilar}><AddIcon size={14} /></button>
                      </div>
                      {form.similarQuestions.length > 0 ? (
                        <div className="item-list flex flex-col gap-1.5">
                          {form.similarQuestions.map((question, index) => (
                            <div key={index} className="item-row flex items-center gap-2 rounded-lg border border-line-soft bg-[#f8f9fc] px-2.5 py-1.5">
                              <div className="item-content min-w-0 flex-1 text-[13px] leading-[1.5] text-ink [word-break:break-word]">{question}</div>
                              <button type="button" className="remove-item-btn inline-flex h-[22px] w-[22px] shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-faint hover:bg-[rgba(227,77,89,0.08)] hover:text-[#e34d59]" aria-label={t('common.delete')} onClick={() => onFormChange({ similarQuestions: removeListItem(form.similarQuestions, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row setting-row-optional setting-row-negative flex flex-col gap-2">
                    <div className="setting-info flex flex-col gap-0.5">
                      <label className="optional-label text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-editor-negative">{t('knowledgeEditor.faq.negativeQuestions')}</label>
                      <p className="desc optional-desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.negativeQuestionsDesc')}</p>
                    </div>
                    <div className="setting-control flex flex-col gap-2">
                      <div className="full-width-input-wrapper flex w-full items-center gap-2">
                        <Input
                          id="faq-editor-negative"
                          className={'full-width-input ' + EDITOR_CONTROL_INLINE}
                          placeholder={t('knowledgeEditor.faq.negativePlaceholder')}
                          value={form.negativeDraft}
                          onChange={(event) => onFormChange({ negativeDraft: event.target.value })}
                          onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addNegative(); } }}
                        />
                        <button type="button" className="add-item-btn inline-flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-accent-deep bg-transparent text-accent-deep hover:enabled:bg-[rgba(0,168,112,0.06)] disabled:cursor-not-allowed disabled:opacity-[0.45]" aria-label={t('knowledgeEditor.faq.negativeQuestions')} disabled={!form.negativeDraft.trim() || form.negativeQuestions.length >= FAQ_NEGATIVE_CAP} onClick={addNegative}><AddIcon size={14} /></button>
                      </div>
                      {form.negativeQuestions.length > 0 ? (
                        <div className="item-list flex flex-col gap-1.5">
                          {form.negativeQuestions.map((question, index) => (
                            <div key={index} className="item-row negative flex items-center gap-2 rounded-lg border border-line-soft bg-[#f8f9fc] px-2.5 py-1.5">
                              <div className="item-content min-w-0 flex-1 text-[13px] leading-[1.5] text-ink [word-break:break-word]">{question}</div>
                              <button type="button" className="remove-item-btn inline-flex h-[22px] w-[22px] shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-faint hover:bg-[rgba(227,77,89,0.08)] hover:text-[#e34d59]" aria-label={t('common.delete')} onClick={() => onFormChange({ negativeQuestions: removeListItem(form.negativeQuestions, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row setting-row-primary setting-row-answer flex flex-col gap-2">
                    <div className="setting-info flex flex-col gap-0.5">
                      <label className="required-label text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-editor-answer">{t('knowledgeEditor.faq.answers')} <span className="required-mark ml-0.5 text-[#e34d59]">*</span></label>
                      <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.answersDesc')}</p>
                    </div>
                    <div className="setting-control flex flex-col gap-2">
                      <div className="textarea-container">
                        <div className="full-width-input-wrapper textarea-wrapper flex w-full items-center gap-2">
                          <Textarea
                            id="faq-editor-answer"
                            className={'full-width-textarea box-border w-auto min-w-0 min-h-[80px] flex-1 resize-y rounded-lg border border-[#cdd6e2] bg-surface px-2.5 py-2 text-sm leading-[1.5] text-ink font-[inherit] focus:border-accent-deep focus:outline-none'}
                            rows={3}
                            placeholder={t('knowledgeEditor.faq.answerPlaceholder')}
                            value={form.answerDraft}
                            onChange={(event) => onFormChange({ answerDraft: event.target.value })}
                            onKeyDown={(event) => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); addAnswer(); } }}
                          />
                          <button type="button" className="add-item-btn inline-flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-accent-deep bg-transparent text-accent-deep hover:enabled:bg-[rgba(0,168,112,0.06)] disabled:cursor-not-allowed disabled:opacity-[0.45]" aria-label={t('knowledgeEditor.faq.answers')} disabled={!form.answerDraft.trim() || form.answers.length >= FAQ_ANSWER_CAP} onClick={addAnswer}><AddIcon size={14} /></button>
                        </div>
                        <div className="item-count text-right text-xs leading-[1.5] text-faint">{form.answers.length}/{FAQ_ANSWER_CAP}</div>
                      </div>
                      {form.answers.length > 0 ? (
                        <div className="item-list flex flex-col gap-1.5">
                          {form.answers.map((answer, index) => (
                            <div key={index} className="item-row answer-row item-row flex items-center gap-2 rounded-lg border border-line-soft bg-[#f8f9fc] px-2.5 py-1.5">
                              <div className="item-content min-w-0 flex-1 text-[13px] leading-[1.5] text-ink [word-break:break-word]">{answer}</div>
                              <button type="button" className="remove-item-btn inline-flex h-[22px] w-[22px] shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-faint hover:bg-[rgba(227,77,89,0.08)] hover:text-[#e34d59]" aria-label={t('common.delete')} onClick={() => onFormChange({ answers: removeListItem(form.answers, index) })}><CloseIcon size={12} /></button>
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="setting-row flex flex-col gap-2">
                    <div className="setting-info flex flex-col gap-0.5">
                      <label className="text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-editor-tag">{t('knowledgeBase.tagLabel')}</label>
                      <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.tagDesc')}</p>
                    </div>
                    <div className="setting-control flex flex-col gap-2">
                      {/* R488 A3: Vue t-select — closed combobox keeps option texts
                          out of the drawer innerText (K2 noise); options open on
                          demand via the teleported listbox, re-select clears. */}
                      <div className="full-width-input w-full">
                        <FAQTagSelect
                          id="faq-editor-tag"
                          value={form.tagId}
                          options={[...tagNameBySeq.entries()].map(([seqId, tagName]) => ({ value: String(seqId), label: tagName }))}
                          placeholder={t('knowledgeEditor.faq.tagPlaceholder')}
                          ariaLabel={t('knowledgeBase.tagLabel')}
                          onChange={(tagId) => onFormChange({ tagId })}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <div className="faq-editor-footer flex flex-none justify-end gap-2.5 border-t border-[#e3e8f0] bg-surface px-5 py-3.5">
                <Button type="button" onClick={onCloseEditor}>{t('common.cancel')}</Button>
                <Button type="submit" loading={saving}>{editorMode === 'create' ? t('knowledgeEditor.faq.editorCreate') : t('common.save')}</Button>
              </div>
            </form>
          </aside>
        </section>
      ) : null}

      {/* B4: Vue search test drawer (FAQEntryManager.vue:734-853) — 420px right
          drawer, query input + two sliders with the shared desc keys, a primary
          search button, and a ranked result list with 3-decimal score tags.
          R488 A3: same t-drawer enter/exit motion as the editor drawer. */}
      {searchMounted ? (
        <section className={`faq-editor-overlay fixed inset-0 z-[1000] flex items-stretch justify-end bg-black/50 p-0 [backdrop-filter:blur(4px)] ${searchOpen ? 'faq-drawer-overlay-enter' : 'faq-drawer-overlay-exit'}`} role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faq.searchTestTitle')} onMouseDown={(event) => { if (event.target === event.currentTarget) onCloseSearchTest(); }}>
          <aside className={`faq-editor-drawer faq-search-drawer flex h-full w-[420px] max-w-[92vw] flex-col overflow-hidden bg-surface shadow-[-8px_0_28px_rgba(15,23,42,0.16)] max-md:w-screen max-md:max-w-[100vw] ${searchOpen ? 'faq-drawer-panel-enter' : 'faq-drawer-panel-exit'}`}>
            <div className="faq-editor-header flex items-center justify-between border-b border-[#e3e8f0] px-5 py-[18px]">
              <h2 className="m-0 text-lg font-semibold leading-[1.5] text-ink">{t('knowledgeEditor.faq.searchTestTitle')}</h2>
              <button type="button" className="faq-modal-close static z-10 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0 bg-surface-alt text-muted hover:text-ink" aria-label={t('common.close')} onClick={onCloseSearchTest}><CloseIcon size={16} /></button>
            </div>
            <div className="faq-editor-form-body min-h-0 flex-1 overflow-x-hidden overflow-y-auto p-5">
              {message?.tone === 'error' ? <div className="faq-editor-error mb-2.5" role="alert"><Status tone="error">{message.text}</Status></div> : null}
              <div className="settings-group flex flex-col gap-[18px]">
                <div className="setting-row search-first-row flex flex-col gap-2">
                  <div className="setting-info flex flex-col gap-0.5">
                    <label className="text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-search-query">{t('knowledgeEditor.faq.queryLabel')}</label>
                    <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.queryPlaceholder')}</p>
                  </div>
                  <div className="setting-control flex flex-col gap-2">
                    <Input
                      id="faq-search-query"
                      className={'full-width-input w-full ' + EDITOR_CONTROL}
                      placeholder={t('knowledgeEditor.faq.queryPlaceholder')}
                      value={searchForm.query}
                      onChange={(event) => onSearchFormChange({ query: event.target.value })}
                      onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); runSearchTest(); } }}
                    />
                  </div>
                </div>
                <div className="setting-row flex flex-col gap-2">
                  <div className="setting-info flex flex-col gap-0.5">
                    <label className="text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-search-threshold">{t('knowledgeEditor.faq.similarityThresholdLabel')}</label>
                    <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.vectorThresholdDesc')}</p>
                  </div>
                  <div className="setting-control flex flex-col gap-2">
                    <div className="slider-wrapper flex items-center gap-2.5">
                      <Range
                        id="faq-search-threshold"
                        className="min-w-0 flex-1 cursor-pointer accent-accent-deep"
                        min={FAQ_SEARCH_VECTOR_THRESHOLD.min} max={FAQ_SEARCH_VECTOR_THRESHOLD.max} step={FAQ_SEARCH_VECTOR_THRESHOLD.step}
                        value={searchForm.vectorThreshold}
                        onChange={(event) => onSearchFormChange({ vectorThreshold: Number(event.target.value) })}
                      />
                      <div className="slider-value min-w-10 text-right text-[13px] leading-[1.5] text-ink tabular-nums">{searchForm.vectorThreshold.toFixed(2)}</div>
                    </div>
                  </div>
                </div>
                <div className="setting-row flex flex-col gap-2">
                  <div className="setting-info flex flex-col gap-0.5">
                    <label className="text-sm font-semibold leading-[1.5] text-ink" htmlFor="faq-search-match-count">{t('knowledgeEditor.faq.matchCountLabel')}</label>
                    <p className="desc m-0 text-xs leading-[1.5] text-faint">{t('knowledgeEditor.faq.matchCountDesc')}</p>
                  </div>
                  <div className="setting-control flex flex-col gap-2">
                    <div className="slider-wrapper flex items-center gap-2.5">
                      <Range
                        id="faq-search-match-count"
                        className="min-w-0 flex-1 cursor-pointer accent-accent-deep"
                        min={FAQ_SEARCH_MATCH_COUNT.min} max={FAQ_SEARCH_MATCH_COUNT.max} step={FAQ_SEARCH_MATCH_COUNT.step}
                        value={searchForm.matchCount}
                        onChange={(event) => onSearchFormChange({ matchCount: Number(event.target.value) })}
                      />
                      <div className="slider-value min-w-10 text-right text-[13px] leading-[1.5] text-ink tabular-nums">{searchForm.matchCount}</div>
                    </div>
                  </div>
                </div>
                <div className="setting-row flex flex-col gap-2">
                  <div className="setting-control flex flex-col gap-2">
                    <Button type="button" className="search-button w-full justify-center" loading={searching} onClick={runSearchTest}>
                      {searching ? t('knowledgeEditor.faq.searching') : t('knowledgeEditor.faq.searchButton')}
                    </Button>
                  </div>
                </div>
              </div>
              {searchResults.length > 0 || hasSearched ? (
                <FAQSearchResults t={t} results={searchResults} expandedIds={expandedResults} onToggle={toggleSearchResult} />
              ) : null}
            </div>
            {/* R488 A6: Vue search drawer has no explicit #footer, so the
                TDesign t-drawer DEFAULT footer renders — placement="right"
                orders the row [确认(primary), 取消(default)] (drawer.mjs:222),
                left-aligned per .t-drawer__footer (tdesign.css:17190-17197
                text-align:left + button margin-left). 确认 runs the search
                test (the drawer's whole purpose), 取消 closes it. */}
            <div className="faq-editor-footer flex flex-none items-center gap-2 border-t border-[#e3e8f0] bg-surface px-5 py-3.5">
              <Button type="button" variant="primary" loading={searching} onClick={runSearchTest}>{t('common.confirm')}</Button>
              <Button type="button" onClick={onCloseSearchTest}>{t('common.cancel')}</Button>
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
    <div className="search-results mt-[18px] border-t border-[#e3e8f0] pt-3.5">
      <div className="results-header text-sm font-semibold leading-[1.5] text-ink">
        <span>{t('knowledgeEditor.faq.searchResults')} ({results.length})</span>
      </div>
      {results.length === 0 ? (
        <div className="no-results py-[18px] text-center text-[13px] leading-[1.5] text-faint">{t('knowledgeEditor.faq.noResults')}</div>
      ) : (
        <div className="results-list mt-2.5 flex flex-col gap-2.5">
          {results.map((result, index) => {
            const expanded = expandedIds.has(result.id);
            return (
              <div key={result.id} className={'result-card overflow-hidden rounded-lg border border-[#e3e8f0]' + (expanded ? ' expanded' : '')}>
                <button type="button" className="result-header flex w-full cursor-pointer items-center gap-2 border-0 bg-transparent px-3 py-2.5 text-left hover:bg-surface-alt" aria-expanded={expanded} onClick={() => onToggle(result.id)}>
                  <span className="result-main flex min-w-0 flex-1 flex-col gap-1">
                    <span className="result-question text-[13px] font-semibold leading-[1.5] text-ink [word-break:break-word]"><span className="result-index mr-0.5 text-muted">{index + 1}.</span> {result.standard_question}</span>
                    {result.matched_question && result.matched_question !== result.standard_question ? (
                      <span className="matched-question text-xs font-normal leading-[1.5] text-muted [word-break:break-word]">
                        <span className="matched-label text-faint">{t('knowledgeEditor.faq.matchedQuestion')}:</span>
                        <span className="matched-text text-muted">{result.matched_question}</span>
                      </span>
                    ) : null}
                  </span>
                  <span className="score-tag shrink-0 rounded-[4px] border border-[#cdd6e2] px-2 py-px text-xs leading-[1.5] text-muted tabular-nums">{(result.score || 0).toFixed(3)}</span>
                  <Icon size={14} className="expand-icon shrink-0 text-faint"><path d={expanded ? Chevrons.up : Chevrons.down} /></Icon>
                </button>
                {expanded ? (
                  <div className="result-body flex flex-col gap-2.5 px-3 pb-3">
                    {result.answers?.length ? (
                      <div className="result-section">
                        <div className="section-label mb-1.5 text-xs font-semibold leading-[1.5] text-muted">{t('knowledgeEditor.faq.answers')}</div>
                        <div className="result-tags flex flex-wrap gap-1.5">
                          {/* B5: Vue search rows use t-tooltip (:829-833). */}
                          {result.answers.map((answer, answerIndex) => <FaqTagTooltip key={answerIndex} content={answer} type="answer" placement="top"><span className="question-tag is-answer break-all rounded-[4px] border border-[rgba(0,168,112,0.4)] bg-[rgba(0,168,112,0.06)] px-2 py-0.5 text-xs leading-[1.5] text-accent-deep">{answer}</span></FaqTagTooltip>)}
                        </div>
                      </div>
                    ) : null}
                    {result.similar_questions?.length ? (
                      <div className="result-section">
                        <div className="section-label mb-1.5 text-xs font-semibold leading-[1.5] text-muted">{t('knowledgeEditor.faq.similarQuestions')}</div>
                        <div className="result-tags flex flex-wrap gap-1.5">
                          {result.similar_questions.map((question, questionIndex) => <FaqTagTooltip key={questionIndex} content={question} type="similar" placement="top"><span className="question-tag break-all rounded-[4px] border border-[#cdd6e2] px-2 py-0.5 text-xs leading-[1.5] text-muted">{question}</span></FaqTagTooltip>)}
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
    <p className="wk-muted text-muted">{t('knowledgeBase.tagManageDescription')}</p>
    {error ? <Status tone="error">{error}</Status> : null}
    <div className="faq-tag-manage-toolbar my-3 flex items-center gap-2"><Input className="min-w-0 flex-1" value={query} placeholder={t('knowledgeBase.tagSearchPlaceholder')} onChange={(event) => setQuery(event.target.value)} /><Button type="button" disabled={busy} onClick={() => { setCreating(true); setEditingId(null); }}>{t('knowledgeBase.tagCreateAction')}</Button></div>
    {creating ? <div className="faq-tag-manage-edit flex items-center gap-2"><Input autoFocus maxLength={40} className="min-w-0 flex-1" value={draft} placeholder={t('knowledgeBase.tagNamePlaceholder')} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void createTag(); if (event.key === 'Escape') setCreating(false); }} /><Button type="button" loading={busy} onClick={() => void createTag()}>{t('common.create')}</Button><Button type="button" disabled={busy} onClick={() => setCreating(false)}>{t('common.cancel')}</Button></div> : null}
    <ul className="faq-tag-manage-list m-0 mt-3 grid list-none gap-2 p-0">
      {visible.map((tag) => editingId === tag.id ? <li key={tag.id} className="faq-tag-manage-row flex items-center justify-between gap-2 border-b border-line-soft py-2"><Input autoFocus maxLength={40} className="min-w-0 flex-1" value={editingName} onChange={(event) => setEditingName(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void updateTag(); if (event.key === 'Escape') setEditingId(null); }} /><Button type="button" loading={busy} onClick={() => void updateTag()}>{t('common.save')}</Button><Button type="button" disabled={busy} onClick={() => setEditingId(null)}>{t('common.cancel')}</Button></li> : <li key={tag.id} className="faq-tag-manage-row flex items-center justify-between gap-2 border-b border-line-soft py-2"><span className="grid min-w-0 flex-1 gap-0.5"><strong>{tag.name}</strong><small className="text-xs leading-[1.5] text-faint">{t('knowledgeBase.tagManageFaqCount', { count: tag.chunk_count || 0 })}</small></span><Button type="button" disabled={busy} onClick={() => { setEditingId(tag.id); setEditingName(tag.name); setCreating(false); }}>{t('knowledgeBase.tagEditAction')}</Button><Button type="button" disabled={busy || !Number.isSafeInteger(tag.seq_id)} onClick={() => void removeTag(tag)}>{t('knowledgeBase.tagDeleteAction')}</Button></li>)}
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
  const [confirmingBatchDelete, setConfirmingBatchDelete] = useState(false);
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
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const navigate = useCallback((path: string) => { clientNavigate(path); }, []);
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
      setCanContribute(computeKBPermissions(kbRow as KBSurfaceKB, me as KBSurfaceMe | null).canContribute);
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
    try {
      await faq.updateTags(knowledgeBaseId, { updates: Object.fromEntries([...selected].map((id) => [id, tagId])) });
      await load(false);
      setMessage({ tone: 'success', text: t('knowledgeEditor.messages.updateSuccess') });
      setBatchTagOpen(false); setBatchTagValue(''); setSelected(new Set());
    }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
    finally { setBatchTagBusy(false); }
  }
  async function removeMany(ids: number[]) {
    try { await faq.removeMany(knowledgeBaseId, ids); await load(false); setMessage({ tone: 'success', text: t(faqDeleteSuccessKey(ids.length), { count: ids.length }) }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : t('common.error') }); }
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
      onBatchEnable={() => void updateSelection({ is_enabled: true })}
      onBatchDisable={() => void updateSelection({ is_enabled: false })}
      onClearSelection={() => setSelected(new Set())}
      onNavigate={navigate}
      batchTagOpen={batchTagOpen}
      batchTagValue={batchTagValue}
      batchTagBusy={batchTagBusy}
      onOpenBatchTag={() => { setBatchTagValue(''); setBatchTagOpen(true); }}
      onBatchTagValueChange={setBatchTagValue}
      onBatchTagConfirm={() => void confirmBatchTag()}
      onCloseBatchTag={() => setBatchTagOpen(false)}
      confirmingBatchDelete={confirmingBatchDelete}
      onConfirmBatchDelete={() => { setConfirmingBatchDelete(false); void removeMany([...selected]).then(() => setSelected(new Set())); }}
      onCancelBatchDelete={() => setConfirmingBatchDelete(false)}
      onBatchDelete={() => setConfirmingBatchDelete(true)}
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
      onToggleEntryStatus={(entry, value) => void toggleEntryStatus(entry, value)}
      statusUpdatingIds={statusUpdatingIds}
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
