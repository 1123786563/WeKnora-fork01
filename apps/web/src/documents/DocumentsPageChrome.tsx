// Documents page chrome, rebuilt against the Vue baseline
// frontend/src/views/knowledge/KnowledgeBase.vue document branch:
//   breadcrumb 知识库 › {kbName} › 文档 with KB switcher, info card and settings
//   gear (document-title-row / kb-title-actions); parser-hint warning line for
//   types without an available engine; empty-knowledge illustration state.
// All copy flows through the shared packages/i18n catalog (menu.*,
// knowledgeEditor.*, knowledgeBase.*) — no literals in this file.

import { useState } from 'react';
import type { FocusEvent, ReactNode } from 'react';
import { createTranslator } from '../i18n.ts';
import {
  documentsKBDetailPath,
  documentsKBListPath,
  documentsKBSettingsPath,
} from './page-chrome.ts';

type Translate = ReturnType<typeof createTranslator>;

export interface KBChromeListItem { id: string; name: string; type?: string }

/** Vue KBInfoPopover inputs the documents page can source from kbMeta. */
export interface KBChromeMeta {
  type?: string;
  description?: string;
  createdAt?: string;
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
function InfoIcon(props: { size?: number; className?: string }) { return <Icon {...props}><circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" /></Icon>; }
export function SearchIcon(props: { size?: number; className?: string }) { return <Icon {...props}><circle cx="11" cy="11" r="8" /><path d="M21 21l-4.35-4.35" /></Icon>; }
function GearIcon(props: { size?: number; className?: string }) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z" />
    </Icon>
  );
}

// --- Breadcrumb (Vue .document-title-row + .kb-title-actions) ---------------------

export interface DocumentsBreadcrumbProps {
  t: Translate;
  knowledgeBaseId: string;
  kbName?: string | null;
  kbList?: KBChromeListItem[];
  kbMeta?: KBChromeMeta;
  /** Vue passes supportedFileTypes into KBInfoPopover. */
  supportedFileTypes?: string[];
  /** Vue gates the gear on canManage; the page maps it to its permission signal. */
  canManage?: boolean;
  onNavigate?: (path: string) => void;
}

export function DocumentsBreadcrumb(props: DocumentsBreadcrumbProps) {
  const { t, knowledgeBaseId, kbName, kbList = [], kbMeta, supportedFileTypes, canManage = false, onNavigate = defaultNavigate } = props;
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const sortedFileTypes = supportedFileTypes ? [...supportedFileTypes].sort() : [];
  return (
    <div className="document-title-row flex min-h-8 flex-wrap items-center gap-2">
      <h2 className="document-breadcrumb m-0 flex items-center gap-[6px] text-[20px] font-semibold leading-8 text-[var(--wk-text,#101828)]">
        <button type="button" className="breadcrumb-link inline-flex cursor-pointer items-center gap-1 rounded-[6px] border-none bg-transparent px-2 py-1 -mx-2 -my-1 text-[var(--wk-muted,#66758b)] [font:inherit] [transition:all_.12s_ease] hover:enabled:bg-[var(--wk-surface,#fff)] hover:enabled:text-[var(--wk-brand,#00a870)] disabled:cursor-not-allowed disabled:text-[var(--wk-muted,#98a2b8)]" onClick={() => onNavigate(documentsKBListPath)}>{t('menu.knowledgeBase')}</button>
        <Icon size={14} className="breadcrumb-separator shrink-0 text-[var(--wk-muted,#98a2b8)]"><path d={Chevrons.right} /></Icon>
        <span className="doc-kb-switcher relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setSwitcherOpen(false))}>
          <button type="button" className="breadcrumb-link dropdown group/dd inline-flex cursor-pointer items-center gap-1 rounded-[6px] border-none bg-transparent py-1 pl-2 pr-[6px] -mx-2 -my-1 text-[var(--wk-muted,#66758b)] [font:inherit] [transition:all_.12s_ease] hover:enabled:bg-[var(--wk-surface,#fff)] hover:enabled:text-[var(--wk-brand,#00a870)] disabled:cursor-not-allowed disabled:text-[var(--wk-muted,#98a2b8)]" aria-haspopup="menu" aria-expanded={switcherOpen} onClick={() => setSwitcherOpen((open) => !open)}>
            <span>{kbName ?? '…'}</span>
            <Icon size={14} className="breadcrumb-caret transition-transform duration-[120ms] group-hover/dd:translate-y-[1px]"><path d={Chevrons.down} /></Icon>
          </button>
          <span className="doc-switcher-menu absolute left-0 top-[calc(100%+6px)] z-[200] flex max-h-[280px] min-w-[180px] flex-col overflow-y-auto rounded-[10px] border border-[var(--wk-border,#e3e8f0)] bg-[var(--wk-surface,#fff)] p-1 shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" role="menu" hidden={!switcherOpen}>
            {kbList.map((kb) => (
              <button key={kb.id} type="button" role="menuitem" className={'doc-switcher-item cursor-pointer whitespace-nowrap rounded-[6px] border-none bg-transparent px-[10px] py-[7px] text-left text-[14px] text-[var(--wk-text,#101828)] hover:bg-[rgba(0,0,0,0.04)]' + (kb.id === knowledgeBaseId ? ' is-active font-semibold text-[var(--wk-brand,#00a870)]' : '')} onClick={() => { setSwitcherOpen(false); onNavigate(documentsKBDetailPath(kb.id)); }}>
                {kb.name}
              </button>
            ))}
          </span>
        </span>
        <Icon size={14} className="breadcrumb-separator shrink-0 text-[var(--wk-muted,#98a2b8)]"><path d={Chevrons.right} /></Icon>
        <span className="breadcrumb-current font-semibold text-[var(--wk-text,#101828)]">{t('knowledgeEditor.document.title')}</span>
      </h2>
      <div className="kb-title-actions ml-1 inline-flex shrink-0 items-center gap-[6px]">
        <span className="kb-info-host relative inline-flex" onBlur={(event) => closeOnBlur(event, () => setInfoOpen(false))}>
          <button type="button" className="kb-info-button inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-full border-none bg-[rgba(0,0,0,0.05)] p-0 text-[var(--wk-muted,#66758b)] [transition:all_.2s_ease] hover:bg-[rgba(0,0,0,0.09)] hover:text-[var(--wk-brand,#00a870)]" aria-label={t('knowledgeBase.infoCard.tooltip')} title={t('knowledgeBase.infoCard.tooltip')} aria-expanded={infoOpen} onClick={() => setInfoOpen((open) => !open)}>
            <InfoIcon size={16} />
          </button>
          <span className="kb-info-card absolute right-0 top-[calc(100%+8px)] z-[200] flex w-[320px] flex-col gap-[10px] rounded-[10px] border border-[var(--wk-border,#e3e8f0)] bg-[var(--wk-surface,#fff)] px-4 py-[14px] text-left text-[13px] shadow-[0_6px_24px_rgba(15,23,42,0.12)] [&[hidden]]:hidden" hidden={!infoOpen}>
            <span className="kb-info-card-header border-b border-[var(--wk-border,#e3e8f0)] pb-2 text-[14px] font-semibold text-[var(--wk-text,#101828)]">{t('knowledgeBase.infoCard.title')}</span>
            <span className="kb-info-card-row flex items-baseline gap-3"><span className="kb-info-card-label w-16 shrink-0 text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.infoCard.type')}</span><span className="kb-info-card-value text-[var(--wk-text,#101828)] [word-break:break-word]">{kbMeta?.type?.toLowerCase() === 'faq' ? t('knowledgeEditor.basic.typeFAQ') : t('knowledgeEditor.basic.typeDocument')}</span></span>
            {kbMeta?.description ? <span className="kb-info-card-row flex items-baseline gap-3"><span className="kb-info-card-label w-16 shrink-0 text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.description')}</span><span className="kb-info-card-value text-[var(--wk-text,#101828)] [word-break:break-word]">{kbMeta.description}</span></span> : null}
            {kbMeta?.createdAt ? <span className="kb-info-card-row flex items-baseline gap-3"><span className="kb-info-card-label w-16 shrink-0 text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.infoCard.createdAt')}</span><span className="kb-info-card-value text-[var(--wk-text,#101828)] [word-break:break-word]">{kbMeta.createdAt}</span></span> : null}
            {sortedFileTypes.length > 0 ? (
              <span className="kb-info-card-row flex items-baseline gap-3">
                <span className="kb-info-card-label w-16 shrink-0 text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.infoCard.supportedFileTypes')}</span>
                <span className="kb-info-card-value text-[var(--wk-text,#101828)] [word-break:break-word]">
                  {sortedFileTypes.map((fileType) => (
                    <span key={fileType} className="kb-info-filetype mb-1 mr-1 inline-block rounded-full border border-[var(--wk-border,#e4e7ec)] px-[6px] py-0 text-[12px] leading-[20px]">{`.${fileType}`}</span>
                  ))}
                </span>
              </span>
            ) : null}
          </span>
        </span>
        {canManage ? (
          <button type="button" className="kb-settings-button inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-full border-none bg-[rgba(0,0,0,0.05)] p-0 text-[var(--wk-muted,#66758b)] [transition:all_.2s_ease] hover:bg-[rgba(0,0,0,0.09)] hover:text-[var(--wk-brand,#00a870)]" aria-label={t('knowledgeBase.settings')} title={t('knowledgeBase.settings')} disabled={!knowledgeBaseId} onClick={() => knowledgeBaseId && onNavigate(documentsKBSettingsPath(knowledgeBaseId))}>
            <GearIcon size={14} />
          </button>
        ) : null}
      </div>
    </div>
  );
}

// --- Parser hint (Vue .parser-hint warning line) -----------------------------------

export interface ParserHintProps {
  t: Translate;
  /** Unresolved extensions without the leading dot, sorted. */
  types: readonly string[];
  onConfigure: () => void;
}

export function ParserHint({ t, types, onConfigure }: ParserHintProps) {
  if (types.length === 0) return null;
  return (
    <p className="parser-hint group m-0 mt-[2px] flex cursor-pointer items-center gap-1 text-[12px] leading-[1.4] text-[var(--wk-warning,#b54708)] [transition:color_.15s_ease] hover:text-[#d97706]" onClick={onConfigure}>
      <InfoIcon size={12} className="parser-hint-icon shrink-0" />
      <span>{t('knowledgeBase.unsupportedTypesHint', { types: types.map((fileType) => `.${fileType}`).join('、') })}</span>
      <span className="parser-hint-link ml-[2px] whitespace-nowrap text-[var(--wk-brand,#0052d9)] group-hover:underline">{t('knowledgeBase.goToParserSettings')} →</span>
    </p>
  );
}

// --- Empty state (Vue .doc-empty-state + EmptyKnowledge / folder-tree copies) ------

export type DocumentEmptyVariant = 'illustration' | 'folder' | 'search';

export interface DocumentEmptyStateProps {
  t: Translate;
  variant: DocumentEmptyVariant;
}

export function DocumentEmptyState({ t, variant }: DocumentEmptyStateProps) {
  if (variant === 'folder') {
    return (
      <div className="doc-empty-state flex w-full min-h-full flex-1 items-center justify-center px-5 py-[60px]">
        <p className="doc-empty-folder m-0 text-[14px] text-[var(--wk-muted,#66758b)]">{t('knowledgeBase.folderTree.emptyFolder')}</p>
      </div>
    );
  }
  if (variant === 'search') {
    return (
      <div className="doc-empty-state flex w-full min-h-full flex-1 items-center justify-center px-5 py-[60px]">
        <p className="doc-empty-folder m-0 text-[14px] text-[var(--wk-muted,#66758b)]">{t('knowledgeBase.folderTree.emptySearch')}</p>
      </div>
    );
  }
  return (
    <div className="doc-empty-state flex w-full min-h-full flex-1 items-center justify-center px-5 py-[60px]">
      <div className="doc-empty-illustration flex flex-col items-center justify-center">
        <img className="empty-img h-[162px] w-[162px]" src={emptyIllustration} alt="" />
        <span className="empty-txt m-0 mb-4 mt-3 text-[16px] font-semibold leading-[26px] text-[var(--wk-muted,#667085)]">{t('knowledgeBase.emptyKnowledgeDragDrop')}</span>
        <span className="empty-type-txt w-[217px] text-center text-[12px] font-normal text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.pdfDocFormat')}</span>
        <span className="empty-type-txt w-[217px] text-center text-[12px] font-normal text-[var(--wk-muted,#98a2b8)]">{t('knowledgeBase.textMarkdownFormat')}</span>
      </div>
    </div>
  );
}

// Bundled copy of the Vue illustration (frontend/src/assets/img/upload.svg);
// same asset the organizations slice shipped as empty-organizations.svg.
import emptyIllustration from './empty-documents.svg';

// --- Filter option tables (Vue fileTypeOptions / parseStatusOptions / sourceOptions) ---

export interface DocumentFilterOption { value: string; label?: string; labelKey?: string }

export const DOCUMENT_FILE_TYPE_OPTIONS: DocumentFilterOption[] = [
  { value: 'pdf', label: 'PDF' },
  { value: 'docx', label: 'DOCX' },
  { value: 'doc', label: 'DOC' },
  { value: 'pptx', label: 'PPTX' },
  { value: 'ppt', label: 'PPT' },
  { value: 'epub', label: 'EPUB' },
  { value: 'mhtml', label: 'MHTML' },
  { value: 'txt', label: 'TXT' },
  { value: 'md', label: 'MD' },
  { value: 'url', label: 'URL' },
  { value: 'manual', labelKey: 'upload.onlineEdit' },
  { value: 'mp3', label: 'MP3' },
  { value: 'wav', label: 'WAV' },
  { value: 'm4a', label: 'M4A' },
  { value: 'flac', label: 'FLAC' },
  { value: 'ogg', label: 'OGG' },
];

export const DOCUMENT_PARSE_STATUS_OPTIONS: DocumentFilterOption[] = [
  { value: 'pending', labelKey: 'knowledgeBase.parseStatusPending' },
  { value: 'processing', labelKey: 'knowledgeBase.parseStatusProcessing' },
  { value: 'completed', labelKey: 'knowledgeBase.parseStatusCompleted' },
  { value: 'failed', labelKey: 'knowledgeBase.parseStatusFailed' },
  { value: 'cancelled', labelKey: 'knowledgeBase.parseStatusCancelled' },
  { value: 'finalizing', labelKey: 'knowledgeBase.parseStatusFinalizing' },
  { value: 'draft', labelKey: 'knowledgeBase.parseStatusDraft' },
];

/** Channels plus the manual/url virtual sources the backend maps onto `type`. */
export const DOCUMENT_SOURCE_OPTIONS: DocumentFilterOption[] = [
  { value: 'web', labelKey: 'knowledgeBase.sourceUpload' },
  { value: 'url', labelKey: 'knowledgeBase.sourceUrl' },
  { value: 'manual', labelKey: 'knowledgeBase.sourceManual' },
  { value: 'api', labelKey: 'knowledgeBase.sourceApi' },
  { value: 'browser_extension', labelKey: 'knowledgeBase.sourceBrowserExtension' },
  { value: 'feishu', labelKey: 'knowledgeBase.channelFeishu' },
  { value: 'feishu_drive', labelKey: 'knowledgeBase.channelFeishuDrive' },
  { value: 'notion', labelKey: 'knowledgeBase.channelNotion' },
  { value: 'yuque', labelKey: 'knowledgeBase.channelYuque' },
  { value: 'gitlab', labelKey: 'knowledgeBase.channelGitLab' },
  { value: 'ima', labelKey: 'knowledgeBase.channelIma' },
  { value: 'wechat', labelKey: 'knowledgeBase.channelWechat' },
  { value: 'wecom', labelKey: 'knowledgeBase.channelWecom' },
  { value: 'dingtalk', labelKey: 'knowledgeBase.channelDingtalk' },
  { value: 'slack', labelKey: 'knowledgeBase.channelSlack' },
  { value: 'im', labelKey: 'knowledgeBase.channelIm' },
];
