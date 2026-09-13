import { useCallback, useEffect, useRef, useState } from 'react';
import type { DragEvent, FocusEvent, FormEvent, ReactNode } from 'react';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, FAQImportProgress, KnowledgeBase, KnowledgeTag, WeKnoraClient } from '@weknora/api-client';
import { Button, Status } from '@weknora/ui';
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

type FormState = { question: string; similar: string; negative: string; answers: string; tagId: string; enabled: boolean; recommended: boolean };
const emptyForm: FormState = { question: '', similar: '', negative: '', answers: '', tagId: '', enabled: true, recommended: false };
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
  } = props;
  const t = tr ?? createTranslator('zh-CN');
  const [tagPanelOpen, setTagPanelOpen] = useState(false);
  const [createMenuOpen, setCreateMenuOpen] = useState(false);
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
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
              <button type="button" className="content-bar-icon-btn" aria-label={t('knowledgeEditor.faq.searchTest')} title={t('knowledgeEditor.faq.searchTest')} onClick={onSearchSubmit}>
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
              <ul className="wk-list wk-faq-list">
                <li className="wk-faq-list-head">
                  {canContribute ? <label className="wk-faq-select-all"><input type="checkbox" checked={selected.size === entries.length && entries.length > 0} onChange={(event) => onToggleSelectAll(event.target.checked)} /> {t('common.all')}</label> : <span />}
                  <span className="wk-faq-range">{entries.length} / {total}</span>
                </li>
                {entries.map((entry) => (
                  <li key={entry.id} className="wk-faq-item">
                    {canContribute ? <label><input type="checkbox" checked={selected.has(entry.id)} onChange={(event) => onToggleSelect(entry.id, event.target.checked)} /></label> : null}
                    <div className="wk-list-item-copy">
                      <strong>{entry.standard_question}</strong>
                      <span>{entry.answers.join(' · ')}</span>
                      <small>{entry.similar_questions.length + ' ' + t('knowledgeEditor.faq.similarQuestions')} · {entry.negative_questions.length + ' ' + t('knowledgeEditor.faq.negativeQuestions')} · {entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')}{entry.is_recommended ? ' · ' + t('knowledgeEditor.faq.recommended') : ''}{typeof entry.tag_id === 'number' && tagNameBySeq.has(entry.tag_id) ? ' · ' + tagNameBySeq.get(entry.tag_id) : ''}</small>
                    </div>
                    {canContribute ? (
                      <div className="wk-list-item-actions">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={entry.is_enabled}
                          aria-label={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')}
                          title={entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')}
                          className={'faq-status-switch' + (entry.is_enabled ? ' is-on' : '')}
                          disabled={statusUpdatingIds.includes(entry.id)}
                          onClick={() => onToggleEntryStatus(entry, !entry.is_enabled)}
                        >
                          <span className="faq-status-switch__thumb" />
                        </button>
                        <Button type="button" onClick={() => onEditEntry(entry)}>{t('common.edit')}</Button>
                        <Button type="button" onClick={() => onDeleteEntry(entry)}>{t('common.delete')}</Button>
                      </div>
                    ) : null}
                  </li>
                ))}
              </ul>
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

      {editorOpen ? (
        <section className="faq-editor-overlay" aria-label={editorTitle}>
          <aside className="faq-editor-drawer">
            <div className="faq-editor-header">
              <h2>{editorTitle}</h2>
              <button type="button" className="faq-modal-close" aria-label={t('common.close')} onClick={onCloseEditor}><CloseIcon size={16} /></button>
            </div>
            <form className="faq-editor-form" onSubmit={onEditorSubmit}>
              <label className="faq-editor-field">
                <span className="faq-editor-label">{t('knowledgeEditor.faq.standardQuestion')} *</span>
                <input required value={form.question} onChange={(event) => onFormChange({ question: event.target.value })} />
              </label>
              <label className="faq-editor-field">
                <span className="faq-editor-label">{t('knowledgeEditor.faq.similarQuestions')}</span>
                <textarea rows={3} value={form.similar} placeholder={t('knowledgeEditor.faq.similarPlaceholder')} onChange={(event) => onFormChange({ similar: event.target.value })} />
              </label>
              <label className="faq-editor-field">
                <span className="faq-editor-label">{t('knowledgeEditor.faq.negativeQuestions')}</span>
                <textarea rows={3} value={form.negative} placeholder={t('knowledgeEditor.faq.negativePlaceholder')} onChange={(event) => onFormChange({ negative: event.target.value })} />
              </label>
              <label className="faq-editor-field">
                <span className="faq-editor-label">{t('knowledgeEditor.faq.answers')} *</span>
                <textarea required rows={4} value={form.answers} placeholder={t('knowledgeEditor.faq.answerPlaceholder')} onChange={(event) => onFormChange({ answers: event.target.value })} />
              </label>
              <label className="faq-editor-field">
                <span className="faq-editor-label">{t('knowledgeBase.tagLabel')}</span>
                <select value={form.tagId} onChange={(event) => onFormChange({ tagId: event.target.value })}>
                  <option value="">{t('knowledgeEditor.faq.tagPlaceholder')}</option>
                  {[...tagNameBySeq.entries()].map(([seqId, name]) => <option key={seqId} value={String(seqId)}>{name}</option>)}
                </select>
              </label>
              <div className="faq-editor-checks">
                <label><input type="checkbox" checked={form.enabled} onChange={(event) => onFormChange({ enabled: event.target.checked })} /> {t('knowledgeEditor.faq.statusEnabled')}</label>
                <label><input type="checkbox" checked={form.recommended} onChange={(event) => onFormChange({ recommended: event.target.checked })} /> {t('knowledgeEditor.faq.recommended')}</label>
              </div>
              <div className="faq-editor-footer">
                <Button type="button" onClick={onCloseEditor}>{t('common.cancel')}</Button>
                <Button type="submit" loading={saving}>{editorMode === 'create' ? t('knowledgeEditor.faq.editorCreate') : t('common.save')}</Button>
              </div>
            </form>
          </aside>
        </section>
      ) : null}
    </main>
  );
}

// --- Container: data wiring (unchanged API usage, new presentation) ----------------

function formFrom(entry: FAQEntry | null): FormState {
  return entry ? { question: entry.standard_question, similar: entry.similar_questions.join('\n'), negative: entry.negative_questions.join('\n'), answers: entry.answers.join('\n'), tagId: typeof entry.tag_id === 'number' ? String(entry.tag_id) : '', enabled: entry.is_enabled, recommended: entry.is_recommended } : emptyForm;
}
function payloadFrom(form: FormState): FAQEntryPayload {
  return normalizeFAQPayload({ standard_question: form.question, similar_questions: form.similar.split('\n'), negative_questions: form.negative.split('\n'), answers: form.answers.split('\n'), tag_id: form.tagId.trim() ? Number(form.tagId) : null, is_enabled: form.enabled, is_recommended: form.recommended });
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
  const t = createTranslator(locale);
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
  // Vue FAQEntryManager.vue:2091-2165 — after upsert returns a task_id the
  // page polls importProgress until completed/failed; success refreshes the
  // list and collapses the strip after 3s; 404 stops the polling.
  const [importTask, setImportTask] = useState<FAQImportProgress | null>(null);
  useEffect(() => {
    if (!importTask || (importTask.status !== 'processing' && importTask.status !== 'pending')) return;
    const timer = setInterval(() => {
      faq.importProgress(importTask.task_id).then((next) => {
        setImportTask((current) => (current ? { ...current, ...next } : next));
        if (next.status === 'completed') void load(false);
      }).catch(() => { setImportTask(null); });
    }, 1500);
    return () => clearInterval(timer);
  }, [importTask, faq]);
  useEffect(() => {
    if (importTask?.status !== 'completed') return;
    const timer = setTimeout(() => setImportTask(null), 3000);
    return () => clearTimeout(timer);
  }, [importTask?.status]);
  const [canContribute, setCanContribute] = useState(true);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const navigate = useCallback((path: string) => { window.location.assign(path); }, []);

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
    event.preventDefault(); setSaving(true); setMessage(null);
    try { const payload = payloadFrom(form); if (editing) await faq.update(knowledgeBaseId, editing.id, payload); else await faq.create(knowledgeBaseId, payload); setMessage({ tone: 'success', text: editing ? 'FAQ entry updated.' : 'FAQ entry created.' }); setEditing(undefined); await load(false); }
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

  return (
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
    />
  );
}
