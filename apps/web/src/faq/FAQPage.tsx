import { useCallback, useEffect, useState } from 'react';
import type { DragEvent, FocusEvent, FormEvent, ReactNode } from 'react';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, KnowledgeBase, KnowledgeTag, WeKnoraClient } from '@weknora/api-client';
import { Button, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { computeKBPermissions, type KBSurfaceKB, type KBSurfaceMe } from '../knowledge/permissions.ts';
import { pagerState } from '../pagination.ts';
import { normalizeFAQPayload, parseFAQImportText } from './import-export.ts';
import './faq.css';

// FAQ knowledge-base page, rebuilt against the Vue baseline
// frontend/src/views/knowledge/components/FAQEntryManager.vue:
//   breadcrumb 知识库 › {kbName} › 问答 with KB switcher, info card and settings gear;
//   subtitle + full-width rounded search + 全部标签 tag filter + icon buttons;
//   centered 暂无 FAQ 条目 empty state. All copy flows through the shared
//   packages/i18n catalog (knowledgeEditor.faq*/faqImport*/faqExport*, menu.*,
//   knowledgeBase.*) — no literals in this file.

type Translate = ReturnType<typeof createTranslator>;

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
const PAGE_SIZE = 50;

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
  page?: number;
  pageSize?: number;
  loading?: boolean;
  canContribute?: boolean;
  selected?: Set<number>;
  keywordDraft?: string;
  importOpen?: boolean;
  importMode?: 'append' | 'replace';
  importFileName?: string | null;
  importBusy?: boolean;
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
  onBatchEnable?: () => void;
  onBatchDisable?: () => void;
  onBatchRecommend?: () => void;
  onBatchTagChange?: (value: string) => void;
  onBatchSetTag?: () => void;
  onBatchDelete?: () => void;
  onPageChange?: (page: number) => void;
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
    page = 1,
    pageSize = PAGE_SIZE,
    loading = false,
    canContribute = true,
    selected = new Set<number>(),
    keywordDraft = '',
    importOpen = false,
    importMode = 'append',
    importFileName = null,
    importBusy = false,
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
    onPageChange = () => {},
    onCloseEditor = () => {},
    onFormChange = () => {},
    onEditorSubmit = () => {},
  } = props;
  const t = tr ?? createTranslator('zh-CN');
  const [tagPanelOpen, setTagPanelOpen] = useState(false);
  const [createMenuOpen, setCreateMenuOpen] = useState(false);
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const pager = pagerState(total, page, pageSize);
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

          <div className="faq-scroll-container">
            {loading && entries.length === 0 ? (
              <div className="faq-skeleton-grid" aria-hidden="true">
                {Array.from({ length: 6 }, (_, index) => <div key={index} className="faq-card-skeleton" />)}
              </div>
            ) : entries.length > 0 ? (
              <ul className="wk-list wk-faq-list">
                <li className="wk-faq-list-head">
                  {canContribute ? <label className="wk-faq-select-all"><input type="checkbox" checked={selected.size === entries.length && entries.length > 0} onChange={(event) => onToggleSelectAll(event.target.checked)} /> {t('common.all')}</label> : <span />}
                  <span className="wk-faq-range">{pager.start}-{pager.end} / {pager.total}</span>
                </li>
                {entries.map((entry) => (
                  <li key={entry.id} className="wk-faq-item">
                    {canContribute ? <label><input type="checkbox" checked={selected.has(entry.id)} onChange={(event) => onToggleSelect(entry.id, event.target.checked)} /></label> : null}
                    <div className="wk-list-item-copy">
                      <strong>{entry.standard_question}</strong>
                      <span>{entry.answers.join(' · ')}</span>
                      <small>{entry.similar_questions.length + ' ' + t('knowledgeEditor.faq.similarQuestions')} · {entry.negative_questions.length + ' ' + t('knowledgeEditor.faq.negativeQuestions')} · {entry.is_enabled ? t('knowledgeEditor.faq.statusEnabled') : t('knowledgeEditor.faq.statusDisabled')}{entry.is_recommended ? ' · ' + t('knowledgeEditor.faq.recommended') : ''}{typeof entry.tag_id === 'number' && tagNameBySeq.has(entry.tag_id) ? ' · ' + tagNameBySeq.get(entry.tag_id) : ''}</small>
                    </div>
                    {canContribute ? <div className="wk-list-item-actions"><Button type="button" onClick={() => onEditEntry(entry)}>{t('common.edit')}</Button><Button type="button" onClick={() => onDeleteEntry(entry)}>{t('common.delete')}</Button></div> : null}
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

          {pager.total > pageSize ? (
            <nav className="wk-pagination" aria-label={t('knowledgeEditor.faq.title')}>
              <Button type="button" disabled={!pager.hasPrevious} onClick={() => onPageChange(page - 1)}>{t('knowledgeBase.documents.previous')}</Button>
              <span>{t('knowledgeBase.faq.page', { page: pager.page, total: pager.total })}</span>
              <Button type="button" disabled={!pager.hasNext} onClick={() => onPageChange(page + 1)}>{t('knowledgeBase.documents.next')}</Button>
            </nav>
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
  const [kb, setKb] = useState<KnowledgeBase | null>(null);
  const [kbList, setKbList] = useState<KBListItem[]>([]);
  const [tags, setTags] = useState<KnowledgeTag[]>([]);
  const [entries, setEntries] = useState<FAQEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
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
  const [batchTag, setBatchTag] = useState('');
  const [canContribute, setCanContribute] = useState(true);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const faq = client.knowledge.faq;
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

  async function load() {
    setLoading(true);
    try {
      const result = await faq.list(knowledgeBaseId, { page, page_size: PAGE_SIZE, keyword: keyword || undefined, tag_ids: activeTagIds.length ? activeTagIds.join(',') : undefined });
      setEntries(result.data);
      setTotal(result.total ?? result.data.length);
      setSelected(new Set());
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to load FAQ entries' }); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [client, knowledgeBaseId, keyword, page, activeTagIds]);

  function openEditor(entry: FAQEntry | null = null) { setEditing(entry); setForm(formFrom(entry)); setMessage(null); }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage(null);
    try { const payload = payloadFrom(form); if (editing) await faq.update(knowledgeBaseId, editing.id, payload); else await faq.create(knowledgeBaseId, payload); setMessage({ tone: 'success', text: editing ? 'FAQ entry updated.' : 'FAQ entry created.' }); setEditing(undefined); await load(); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to save FAQ entry' }); }
    finally { setSaving(false); }
  }
  async function updateSelection(input: FAQEntryFieldsUpdate) {
    if (!selected.size) return;
    try { await faq.updateFields(knowledgeBaseId, { by_id: Object.fromEntries([...selected].map((id) => [id, input])) }); await load(); setMessage({ tone: 'success', text: 'Selected FAQ entries updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected entries' }); }
  }
  async function updateSelectedTag() {
    if (!selected.size) return;
    const tagId = batchTag.trim() ? Number(batchTag) : null;
    if (tagId !== null && (!Number.isSafeInteger(tagId) || tagId < 0)) { setMessage({ tone: 'error', text: 'Tag ID must be a non-negative integer.' }); return; }
    try { await faq.updateTags(knowledgeBaseId, { updates: Object.fromEntries([...selected].map((id) => [id, tagId])) }); await load(); setBatchTag(''); setMessage({ tone: 'success', text: 'Selected FAQ tags updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected tags' }); }
  }
  async function removeMany(ids: number[]) {
    try { await faq.removeMany(knowledgeBaseId, ids); await load(); setMessage({ tone: 'success', text: 'Selected FAQ entries deleted.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to delete FAQ entries' }); }
  }
  async function confirmImport() {
    if (!importFile) return;
    setImportBusy(true); setMessage(null);
    try {
      const text = await importFile.text();
      const format = importFile.name.toLowerCase().endsWith('.json') ? 'json' : 'csv';
      const imported = parseFAQImportText(text, format);
      const result = await faq.upsert(knowledgeBaseId, { entries: imported, mode: importMode });
      setImportOpen(false); setImportFile(null);
      setMessage({ tone: 'success', text: 'Import ' + importMode + ' queued (' + result.task_id + ').' });
      setPage(1);
      await load();
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
      page={page}
      pageSize={PAGE_SIZE}
      loading={loading}
      canContribute={canContribute}
      selected={selected}
      keywordDraft={keywordDraft}
      importOpen={importOpen}
      importMode={importMode}
      importFileName={importFile?.name ?? null}
      importBusy={importBusy}
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
      onSearchSubmit={() => { setKeyword(keywordDraft.trim()); setPage(1); }}
      onSearchClear={() => { setKeywordDraft(''); setKeyword(''); setPage(1); }}
      onToggleTag={(tagId) => { setActiveTagIds((current) => current.includes(tagId) ? current.filter((id) => id !== tagId) : [...current, tagId]); setPage(1); }}
      onOpenCreate={() => openEditor()}
      onOpenImport={() => { setImportFile(null); setImportOpen(true); }}
      onCloseImport={() => setImportOpen(false)}
      onImportModeChange={setImportMode}
      onImportFile={setImportFile}
      onImportConfirm={() => void confirmImport()}
      onExport={(format) => void exportEntries(format)}
      onToggleSelect={(id, checked) => setSelected((current) => { const next = new Set(current); if (checked) next.add(id); else next.delete(id); return next; })}
      onToggleSelectAll={(checked) => setSelected(checked ? new Set(entries.map((entry) => entry.id)) : new Set())}
      onEditEntry={openEditor}
      onDeleteEntry={(entry) => void removeMany([entry.id])}
      onBatchEnable={() => void updateSelection({ is_enabled: true })}
      onBatchDisable={() => void updateSelection({ is_enabled: false })}
      onBatchRecommend={() => void updateSelection({ is_recommended: true })}
      onBatchTagChange={setBatchTag}
      onBatchSetTag={() => void updateSelectedTag()}
      onBatchDelete={() => void removeMany([...selected])}
      onPageChange={setPage}
      onCloseEditor={() => setEditing(undefined)}
      onFormChange={(patch) => setForm((current) => ({ ...current, ...patch }))}
      onEditorSubmit={(event) => void save(event)}
    />
  );
}
