// Documents page chrome, rebuilt against the Vue baseline
// frontend/src/views/knowledge/KnowledgeBase.vue document branch:
//   breadcrumb 知识库 › {kbName} › 文档 with KB switcher, info card and settings
//   gear (document-title-row / kb-title-actions); parser-hint warning line for
//   types without an available engine; empty-knowledge illustration state.
// All copy flows through the shared packages/i18n catalog (menu.*,
// knowledgeEditor.*, knowledgeBase.*) — no literals in this file.

import { Fragment, useState } from 'react';
import type { ReactNode } from 'react';
import { Popup, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createTranslator } from '../i18n.ts';
import { navigate } from '../platform/navigation.ts';
import { KBInfoPopover, type KBInfoPopoverKB } from './KBInfoPopover.tsx';
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

function defaultNavigate(path: string): void { navigate(path); }

// --- Inline icons (no TDesign / icon font; feather-style strokes) -----------------

export function Icon({ size = 16, className, children }: { size?: number; className?: string; children: ReactNode }) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">{children}</svg>
  );
}
const Chevrons = {
  right: 'M9 18l6-6-6-6',
  down: 'M6 9l6 6 6-6',
};
export function SearchIcon(props: { size?: number; className?: string }) { return <Icon {...props}><circle cx="11" cy="11" r="8" /><path d="M21 21l-4.35-4.35" /></Icon>; }
export function FolderIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M3 6.5A2.5 2.5 0 015.5 4h4l2 2h7A2.5 2.5 0 0121 8.5v8A2.5 2.5 0 0118.5 19h-13A2.5 2.5 0 013 16.5z" /></Icon>; }
export function FileIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M6 3h8l4 4v14H6z" /><path d="M14 3v5h5M9 13h6M9 17h6" /></Icon>; }
export function LinkIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M10 13a5 5 0 007.07.07l2-2a5 5 0 00-7.07-7.07l-1.15 1.15" /><path d="M14 11a5 5 0 00-7.07-.07l-2 2A5 5 0 0012 20l1.15-1.15" /></Icon>; }
export function EditIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M12 20h9" /><path d="M16.5 3.5a2.12 2.12 0 013 3L8 18l-4 1 1-4z" /></Icon>; }
export function GridIcon(props: { size?: number; className?: string }) { return <Icon {...props}><rect x="4" y="4" width="6" height="6" rx="1" /><rect x="14" y="4" width="6" height="6" rx="1" /><rect x="4" y="14" width="6" height="6" rx="1" /><rect x="14" y="14" width="6" height="6" rx="1" /></Icon>; }
export function ListIcon(props: { size?: number; className?: string }) { return <Icon {...props}><path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" /></Icon>; }

// --- Breadcrumb (Vue .document-title-row + .kb-title-actions) ---------------------

/**
 * Vue isWiki renders the third crumb level as a 文档 / Wiki / 图谱 tab row
 * (KnowledgeBase.vue L2359-2380) instead of the plain 文档 label. The pages
 * describe their tabs (labels/hrefs/active state); the component only renders
 * the shared anatomy.
 */
export interface DocumentsBreadcrumbTab {
  key: string;
  label: string;
  href: string;
  active?: boolean;
  /** Vue wraps the graph tab in t-tooltip content=tabGraphTip. */
  title?: string;
}

export interface DocumentsBreadcrumbProps {
  t: Translate;
  knowledgeBaseId: string;
  kbName?: string | null;
  kbList?: KBChromeListItem[];
  kbMeta?: KBChromeMeta;
  /** KBInfoPopover 完整数据源（B2 批 2）：优先于 kbMeta。 */
  kbInfo?: KBInfoPopoverKB | null;
  /** Vue authStore.user?.id（owner 判定）。 */
  infoUserId?: string;
  /** Vue orgStore currentSharedKb。 */
  infoSharedKb?: { orgName: string; sharedAt: string } | null;
  /** Vue effectiveKBPermission。 */
  infoPermission?: string;
  /** Vue passes supportedFileTypes into KBInfoPopover. */
  supportedFileTypes?: string[];
  /** Vue gates the gear on canManage; the page maps it to its permission signal. */
  canManage?: boolean;
  /**
   * Vue ⚙ opens the in-place KB settings surface without leaving the page
   * (KnowledgeBase.vue:2388 → uiStore.openKBSettings). Pages that host such
   * an overlay pass this callback; without it the gear keeps the historical
   * settings-route navigation (/knowledgeBase/<id>/settings).
   */
  onOpenSettings?: () => void;
  onNavigate?: (path: string) => void;
  /** When present, the third crumb level is the Vue breadcrumb-tab row. */
  tabs?: DocumentsBreadcrumbTab[];
}

export function DocumentsBreadcrumb(props: DocumentsBreadcrumbProps) {
  const { t, knowledgeBaseId, kbName, kbList = [], kbMeta, kbInfo = null, infoUserId, infoSharedKb = null, infoPermission, supportedFileTypes, canManage = false, onOpenSettings, onNavigate = defaultNavigate, tabs } = props;
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const hasTabs = !!tabs && tabs.length > 0;
  // Vue KBSwitcherDropdown：当前 KB 置顶，其余保持调用方顺序。
  const sortedKbList = (() => {
    const current = kbList.find((kb) => kb.id === knowledgeBaseId);
    if (!current) return kbList;
    return [current, ...kbList.filter((kb) => kb.id !== knowledgeBaseId)];
  })();
  return (
    <div className="document-title-row">
      <h2 className="document-breadcrumb">
        <button type="button" className="breadcrumb-link" onClick={() => onNavigate(documentsKBListPath)}>{t('menu.knowledgeBase')}</button>
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
                      onClick={() => { setSwitcherOpen(false); if (kb.id !== knowledgeBaseId) onNavigate(documentsKBDetailPath(kb.id)); }}
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
              {kbName == null ? '…' : <><span>{kbName}</span><TIcon name="chevron-down" /></>}
            </button>
          </Popup>
        ) : (
          <button type="button" className="breadcrumb-link" disabled={!knowledgeBaseId}>
            {kbName == null ? '…' : kbName}
          </button>
        )}
        <TIcon name="chevron-right" className="breadcrumb-separator" />
        {hasTabs ? (
          <>
            {tabs!.map((tab, index) => (
              <Fragment key={tab.key}>
                {index > 0 ? <span className="breadcrumb-tab-sep" aria-hidden="true">/</span> : null}
                {tab.title ? (
                  <Tooltip content={tab.title} placement="bottom">
                    <span
                      className={'breadcrumb-tab' + (tab.active ? ' active' : '')}
                      role="link"
                      tabIndex={0}
                      onClick={() => onNavigate(tab.href)}
                    >{tab.label}</span>
                  </Tooltip>
                ) : (
                  <span
                    className={'breadcrumb-tab' + (tab.active ? ' active' : '')}
                    role="link"
                    tabIndex={0}
                    onClick={() => onNavigate(tab.href)}
                  >{tab.label}</span>
                )}
              </Fragment>
            ))}
          </>
        ) : (
          <span className="breadcrumb-current">{t('knowledgeEditor.document.title')}</span>
        )}
      </h2>
      <div className="kb-title-actions">
        {(kbInfo ?? kbMeta) ? (
          <KBInfoPopover
            t={t}
            /* kbMeta fallback 仅服务测试桩（真实调用方均传完整 kbInfo）；
               完整 popover 数据面以后端 created_at/统计/绑定字段为准。 */
            kbInfo={(kbInfo ?? { type: kbMeta!.type, description: kbMeta!.description }) as KBInfoPopoverKB}
            supportedFileTypes={supportedFileTypes}
            userId={infoUserId}
            sharedKb={infoSharedKb}
            permission={infoPermission}
          />
        ) : null}
        {canManage ? (
          <Tooltip content={t('knowledgeBase.settings')} placement="top">
            <button type="button" className="kb-settings-button" aria-label={t('knowledgeBase.settings')} title={t('knowledgeBase.settings')} disabled={!knowledgeBaseId} onClick={() => { if (!knowledgeBaseId) return; if (onOpenSettings) onOpenSettings(); else onNavigate(documentsKBSettingsPath(knowledgeBaseId)); }}>
              <TIcon name="setting" size="16px" />
            </button>
          </Tooltip>
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
    <p className="parser-hint" onClick={onConfigure}>
      <TIcon name="info-circle" className="parser-hint-icon" />
      <span>{t('knowledgeBase.unsupportedTypesHint', { types: types.map((fileType) => `.${fileType}`).join('、') })}</span>
      <span className="parser-hint-link">{t('knowledgeBase.goToParserSettings')} →</span>
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
      <div className="doc-empty-state">
        <p className="doc-empty-folder">{t('knowledgeBase.folderTree.emptyFolder')}</p>
      </div>
    );
  }
  if (variant === 'search') {
    return (
      <div className="doc-empty-state">
        <p className="doc-empty-folder">{t('knowledgeBase.folderTree.emptySearch')}</p>
      </div>
    );
  }
  // Vue EmptyKnowledge.vue（.empty + upload.svg + 三行文案）。
  return (
    <div className="empty">
      <img className="empty-img" src={emptyIllustration} alt="" />
      <span className="empty-txt">{t('knowledgeBase.emptyKnowledgeDragDrop')}</span>
      <span className="empty-type-txt">{t('knowledgeBase.pdfDocFormat')}</span>
      <span className="empty-type-txt">{t('knowledgeBase.textMarkdownFormat')}</span>
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
  // R483 F2 (R482 B1 差异1): the manual filter option reads
  // knowledgeBase.typeManual like Vue fileTypeOptions (KnowledgeBase.vue),
  // not upload.onlineEdit (that key belongs to the add-document dropdown).
  { value: 'manual', labelKey: 'knowledgeBase.typeManual' },
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
