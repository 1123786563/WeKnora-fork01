import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Fragment } from "react";
import type { MouseEvent as ReactMouseEvent, ReactNode } from "react";
import type { KnowledgeDocument, KnowledgeTag, ModelConfiguration, ParserEngineInfo, WeKnoraClient } from "@weknora/api-client";
import {
  normalizeKnowledgeProcessingStatus,
  buildKnowledgeTimeline,
  flattenKnowledgeSpans,
  isKnowledgeProcessingActive,
  type KnowledgeTimelineStep,
  type KnowledgeTimelineNode,
} from "@weknora/domain/knowledge/processing";
import { knowledgeSpansLastError, resolveKnowledgeSpansView } from "./processing-timeline.ts";
import { findSharedKBGrant } from "../wiki/edit-permission.ts";
import {
  DEFAULT_MOVE_MODE,
  buildTraceSummary,
  documentMenuItems,
  knowledgeSpansViewHasTrace,
  moveMenuViewAfterBack,
  type DocumentMenuAction,
  type DocumentMoveKbController,
  type KnowledgeMoveMode,
  type MoveTargetKb,
  type TraceSummary,
} from "./doc-row-menu.ts";
import { flattenKnowledgeFolders as flattenFolders } from "@weknora/domain/knowledge/folders";
import { Button, Checkbox, Dialog, Input, Select, Sheet, Status, Textarea } from "@weknora/ui";
/* TDesign 平移（playbook §1）：文档域可见结构全部走 tdesign-react；上方 @weknora/ui
 * 引用仅剩上传确认弹窗/移动目录条等 R490 留守段（文件内标注），待上传弹窗域迁移时一并清除。 */
import {
  Button as TdButton,
  Checkbox as TdCheckbox,
  DateRangePicker,
  Dropdown,
  Loading as TdLoading,
  Popconfirm,
  Popup,
  Radio as TdRadio,
  Skeleton as TdSkeleton,
  Select as TdSelect,
  Tag as TdTag,
  Tooltip,
  Input as TdInput,
} from "tdesign-react";
import { Icon as TIcon } from "tdesign-icons-react";
import "./documents.td.css";

/** Vue @/assets/img/more.png 内联副本（kb-list 同款，卡片三点菜单触发器）。 */
const MORE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgBAMAAACBVGfHAAAAD1BMVEUAAAAwMTMwMDMwMjIwMTPbLw9bAAAABHRSTlMA3llYOk1BewAAABxJREFUKM9jGGnAUAiJAAERRwSBXUBRCIkYYQAAnNMDYY7Uun8AAAAASUVORK5CYII=';

/** Vue utils/files getFileIcon（列表行文件类型图标名）。 */
function getFileIconName(document: KnowledgeDocument): string {
  const type = document.type;
  if (type === 'manual') return 'edit';
  if (type === 'url') return 'link';
  const ext = (String((document as { file_type?: string }).file_type ?? '').toLowerCase() || (String(document.file_name ?? '').split('.').pop()?.toLowerCase() ?? ''));
  if (!ext) return 'file';
  if (['pdf'].includes(ext)) return 'file-pdf';
  if (['doc', 'docx'].includes(ext)) return 'file-word';
  if (['xls', 'xlsx', 'csv'].includes(ext)) return 'file-excel';
  if (['ppt', 'pptx'].includes(ext)) return 'file-powerpoint';
  if (['txt', 'md', 'markdown', 'json', 'log', 'yaml', 'yml', 'xml'].includes(ext)) return 'file';
  if (['py', 'pyc', 'pyo', 'js', 'mjs', 'cjs', 'ts', 'tsx', 'jsx', 'go', 'rs', 'java', 'c', 'cc', 'cpp', 'h', 'hpp', 'sh', 'bash', 'rb', 'php', 'sql', 'html', 'htm'].includes(ext)) return 'code';
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg'].includes(ext)) return 'image';
  if (['mp3', 'wav', 'm4a', 'flac', 'ogg', 'aac'].includes(ext)) return 'sound';
  if (['mp4', 'mov', 'webm', 'mkv', 'avi'].includes(ext)) return 'video';
  return 'file';
}

/** Vue DocumentListView getSourceInfo（来源图标 + 标签）。 */
function getSourceInfo(document: KnowledgeDocument, t: (key: string) => string): { icon: string; label: string } {
  const ch = (document as { channel?: string }).channel;
  if (ch === 'feishu') return { icon: 'cloud-download', label: t('knowledgeBase.channelFeishu') };
  if (ch === 'feishu_drive') return { icon: 'cloud-download', label: t('knowledgeBase.channelFeishuDrive') };
  if (ch === 'lark_drive') return { icon: 'cloud-download', label: t('knowledgeBase.channelLarkDrive') };
  if (ch === 'notion') return { icon: 'cloud-download', label: t('knowledgeBase.channelNotion') };
  if (ch === 'yuque') return { icon: 'cloud-download', label: t('knowledgeBase.channelYuque') };
  if (ch === 'gitlab') return { icon: 'cloud-download', label: t('knowledgeBase.channelGitLab') };
  if (ch === 'ima') return { icon: 'cloud-download', label: t('knowledgeBase.channelIma') };
  if (ch === 'wechat') return { icon: 'cloud-download', label: t('knowledgeBase.channelWechat') };
  if (ch === 'wecom') return { icon: 'cloud-download', label: t('knowledgeBase.channelWecom') };
  if (ch === 'dingtalk') return { icon: 'cloud-download', label: t('knowledgeBase.channelDingtalk') };
  if (ch === 'slack') return { icon: 'cloud-download', label: t('knowledgeBase.channelSlack') };
  if (ch === 'im') return { icon: 'cloud-download', label: t('knowledgeBase.channelIm') };
  if (document.type === 'url') return { icon: 'link', label: t('knowledgeBase.channelUrl') };
  if (document.type === 'manual') return { icon: 'edit', label: t('knowledgeBase.channelManual') };
  return { icon: 'upload', label: t('knowledgeBase.channelUpload') };
}

/** Vue DocumentListView computeStatus（行状态 tag 主题/图标）。 */
function listRowStatus(document: KnowledgeDocument, t: (key: string) => string): { label: string; theme: 'success' | 'warning' | 'danger' | 'primary' | 'default'; icon?: string; spin?: boolean } {
  const parse = String(document.parse_status ?? '');
  const summaryInFlight = document.summary_status === 'pending' || document.summary_status === 'processing';
  if (parse === 'pending' || parse === 'processing') return { label: t('knowledgeBase.statusProcessing'), theme: 'primary', icon: 'loading', spin: true };
  if (parse === 'finalizing') {
    if (summaryInFlight) return { label: t('knowledgeBase.generatingSummary'), theme: 'primary', icon: 'loading', spin: true };
    return { label: t('knowledgeBase.statusFinalizing'), theme: 'primary', icon: 'loading', spin: true };
  }
  if (parse === 'failed') return { label: t('knowledgeBase.statusFailed'), theme: 'danger', icon: 'close-circle' };
  if (parse === 'cancelled') return { label: t('knowledgeBase.statusCancelled'), theme: 'warning', icon: 'close-circle' };
  if (parse === 'draft') return { label: t('knowledgeBase.statusDraft'), theme: 'warning' };
  if (parse === 'completed' && summaryInFlight) return { label: t('knowledgeBase.generatingSummary'), theme: 'primary', icon: 'loading', spin: true };
  if (parse === 'completed') return { label: t('knowledgeBase.statusCompleted'), theme: 'success' };
  return { label: '--', theme: 'default' };
}
import { createTranslator, useAppLocale } from "../i18n.ts";
import { observeUploadProgress } from "../platform/http.ts";
import { navigate } from "../platform/navigation.ts";
import {
  applyUploadOverrides,
  asrSectionIssue,
  batchHasAudio,
  batchHasImages,
  batchUploadExtensions,
  batchUploadProgress,
  buildUploadConfirmOverrides,
  clampUploadPercent,
  commitFolderName,
  defaultUploadConfirmSection,
  destinationBreadcrumb,
  folderPickerRows,
  formatBytes,
  filterUploadFiles,
  GRAPH_EXTRACT_DEFAULT_EXAMPLE,
  graphDatabaseEnabled,
  graphSectionAvailable,
  hasGraphAdminRole,
  mergeFolderOptions,
  mergeUploadEntries,
  multimodalSectionIssue,
  normalizeUploadUrl,
  removeUploadEntry,
  retryableUploadEntries,
  runUploadPipeline,
  sectionAfterGraphAvailabilityChange,
  toUploadEntries,
  uploadConfirmStateFromKb,
  uploadConfirmT,
  uploadEntryDisplayTitle,
  uploadEntryRelativeDir,
  uploadProgressPercent,
  uploadSectionStatus,
  type FolderOption,
  type Locale,
  type UploadConfirmSectionKey,
  type UploadConfirmUIState,
  type UploadEntry,
  type UploadEntryState,
  type UploadEntryStatus,
  type UploadGraphNodeState,
  type UploadGraphRelationState,
  type UploadNodeExtractState,
} from "./upload-pipeline.ts";
import {
  canUploadKnowledgeDocuments,
  classifyKnowledgeBaseMetadataError,
  kbTypeRedirectPath,
  resolveKBSurfaceTabs,
  type KnowledgeBaseMetadataError,
  type KBSurfaceKB,
  type KBSurfaceMe,
} from "../knowledge/permissions.ts";
import {
  cancelParseDocuments,
  documentRowActions,
  filterReparseIds,
} from "./actions.ts";
import {
  commonTagIds,
  computeTagVisibleLimit,
  documentTags,
  joinTagIds,
  tagFilterLabel,
  tagFilterTitle,
  tagUpdatesFor,
} from "./tags.ts";
import { TagFilterPanel, TagManageDialog, TagPickerDialog } from "./TagPickerDialog.tsx";
import { tagSurfaceT } from "./tags-locale.ts";
import { useKbDetailGuideTrigger } from "../../../../packages/views/src/guides/use-kb-detail-guide-trigger.ts";
import uploadMaskIllustration from "./upload-mask.svg";
import "./documents-list.css";
import {
  loadKnowledgeDocuments,
  type KnowledgeDocumentListState,
} from "./list.ts";
import {
  batchDownloadKnownBytes,
  batchDownloadPreflight,
  batchDownloadZipName,
  saveBatchDownloadBlob,
  selectBatchDownloadIds,
} from "./knowledge-batch-download.ts";
import {
  computeSupportedFileTypes,
  computeUnsupportedFileTypes,
  dateRangeToTimeParams,
  documentsKBDetailPath,
  documentsKBSettingsPath,
  isFilteringDocuments,
} from "./page-chrome.ts";
import { toggleDocumentSelection, useMarqueeSelection } from "./selection.ts";
import { KnowledgeSettingsPage } from "../knowledge-settings/KnowledgeSettingsPage.tsx";
import { KnowledgeDocumentDetailPage } from "./KnowledgeDocumentDetailPage.tsx";
import './documents-u.css';
import {
  DocumentEmptyState,
  DocumentsBreadcrumb,
  type DocumentsBreadcrumbTab,
  EditIcon,
  Icon,
  LinkIcon,
  ParserHint,
  SearchIcon,
  DOCUMENT_FILE_TYPE_OPTIONS,
  DOCUMENT_PARSE_STATUS_OPTIONS,
  DOCUMENT_SOURCE_OPTIONS,
  FileIcon,
  FolderIcon,
  GridIcon,
  ListIcon,
  type KBChromeListItem,
} from "./DocumentsPageChrome.tsx";

interface KnowledgeDocumentsPageProps {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  onOpenDocument?: (document: KnowledgeDocument) => void;
  initialDocumentId?: string;
}

type UploadDialogMode = "file" | "manual" | "reparse";
/** Vue UploadConfirmResult per-URL append: one shared normalize helper. */

/** t built the way the settings panels do: shared i18n first, dialog table fallback. */
export type UploadDialogT = (key: string, values?: Record<string, string | number>) => string;

export interface UploadConfirmValidationFailure {
  section: "multimodal" | "asr";
  messageKey: "uploadConfirm.vlmModelRequired" | "uploadConfirm.vlmModelSelectRequired" | "uploadConfirm.asrModelRequired" | "uploadConfirm.asrModelSelectRequired";
  patch?: Partial<Pick<UploadConfirmUIState, "multimodalEnabled" | "asrEnabled">>;
}

/**
 * Vue validateBeforeConfirm makes the invalid configuration visible before it
 * reports the missing-model warning. Keep that decision pure so file upload
 * and reparse submissions cannot drift apart.
 */
export function uploadConfirmValidationFailure(input: {
  state: UploadConfirmUIState;
  hasImages: boolean;
  hasAudio: boolean;
}): UploadConfirmValidationFailure | null {
  if (input.hasImages && (!input.state.multimodalEnabled || !input.state.vllmModelId.trim())) {
    return { section: "multimodal", messageKey: "uploadConfirm.vlmModelRequired", patch: { multimodalEnabled: true } };
  }
  if (!input.hasImages && input.state.multimodalEnabled && !input.state.vllmModelId.trim()) {
    return { section: "multimodal", messageKey: "uploadConfirm.vlmModelSelectRequired" };
  }
  if (input.hasAudio && (!input.state.asrEnabled || !input.state.asrModelId.trim())) {
    return { section: "asr", messageKey: "uploadConfirm.asrModelRequired", patch: { asrEnabled: true } };
  }
  if (!input.hasAudio && input.state.asrEnabled && !input.state.asrModelId.trim()) {
    return { section: "asr", messageKey: "uploadConfirm.asrModelSelectRequired" };
  }
  return null;
}

/** Upload requests retain staged entries; dismissing their dialog would discard them. */
export function canCloseUploadConfirmDialog(uploading: boolean): boolean {
  return !uploading;
}

/**
 * Vue canEditKB parity for the upload surface, including shared editor grants.
 * Implementation moved to ../knowledge/permissions.ts so the graph page and
 * other KB surfaces gate management chrome with the identical signal; the
 * re-export keeps the historical import path stable.
 */
export { canUploadKnowledgeDocuments } from "../knowledge/permissions.ts";

function hasContributorRole(me: KBSurfaceMe | null | undefined): boolean {
  const roles = [me?.user?.role, ...(me?.memberships ?? []).map((membership) => membership.role)];
  return roles.some((role) => {
    const normalized = typeof role === "string" ? role.trim().toLowerCase() : "";
    // Vue hasRole('contributor') ranks viewer < contributor < admin < owner
    // (frontend/src/stores/auth.ts ROLE_LEVEL) — owner passes the gate.
    return normalized === "contributor" || normalized === "admin" || normalized === "owner" || normalized === "system_admin";
  }) || me?.user?.is_superuser === true;
}

function kbPermission(kb: KBSurfaceKB): string | undefined {
  const permission = kb.my_permission ?? kb.permission;
  return typeof permission === "string" && permission.trim() ? permission.trim().toLowerCase() : undefined;
}

/** Vue canDownloadKnowledge: downloads are narrower than upload/edit access.
 * The effective permission mirrors Vue effectiveKBPermission (KnowledgeBase.vue:326):
 * org share grant → kb.my_permission → '' — the KB row's `permission` field
 * is never consulted. */
export function canDownloadKnowledgeDocuments(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined, sharedRows?: unknown): boolean {
  if (!hasContributorRole(me)) return false;
  const kbId = typeof kb.id === 'string' ? kb.id : '';
  const grant = kbId ? findSharedKBGrant(sharedRows, kbId) : null;
  const rowPermission = typeof kb.my_permission === 'string' ? kb.my_permission.trim().toLowerCase() : '';
  const permission = (grant && grant.permission) || rowPermission || '';
  return !permission || ['owner', 'admin', 'editor'].includes(permission);
}

/** Vue canMutateKnowledge: move/delete/batch actions require contributor-level access. */
export function canMutateKnowledgeDocuments(kb: KBSurfaceKB, me: KBSurfaceMe | null | undefined): boolean {
  const userId = me?.user?.id;
  const creatorId = kb.creator_id ?? kb.created_by ?? kb.user_id;
  const isCreator = userId !== undefined && userId !== null && creatorId !== undefined && String(userId) === String(creatorId);
  const permission = kbPermission(kb);
  if (permission === "viewer") return false;
  if (permission === "owner" || permission === "admin" || permission === "editor") return true;
  if (isCreator || hasContributorRole(me)) return true;
  return false;
}

/** Vue useKnowledgeBase cardList mapping: the displayed document name drops
 * the extension (lastIndexOf '.' rule) while the raw file name stays
 * available for downloads. */
function documentRawName(document: KnowledgeDocument): string {
  return document.file_name || document.title || document.id;
}

function displayName(document: KnowledgeDocument): string {
  const raw = documentRawName(document);
  const dotIndex = raw.lastIndexOf(".");
  return dotIndex > 0 ? raw.slice(0, dotIndex) : raw;
}

export function folderPathCrumbs(path: string | undefined): Array<{ name: string; path: string }> {
  if (!path) return [];
  const segments = path.split('/').filter(Boolean);
  return segments.map((name, index) => ({ name, path: segments.slice(0, index + 1).join('/') }));
}

/** Vue's `missingStorageEngine` guard: a storage backend is authoritative,
 * while the provider projection keeps older API responses usable. */
export function isStorageEngineMissing(kb: KBSurfaceKB | null | undefined): boolean {
  if (!kb || String(kb.type ?? '').toLowerCase() === 'faq') return false;
  if (kb.storage_backend_id) return false;
  const providerConfig = kb.storage_provider_config;
  const provider = providerConfig && typeof providerConfig === 'object' && !Array.isArray(providerConfig)
    ? (providerConfig as { provider?: unknown }).provider
    : undefined;
  return !provider;
}

function DocumentTagChips({ tags }: { tags: ReturnType<typeof documentTags> }) {
  const ref = useRef<HTMLSpanElement | null>(null);
  const [visibleLimit, setVisibleLimit] = useState(99);
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => setVisibleLimit(computeTagVisibleLimit(element.clientWidth, tags.length));
    update();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, [tags.length]);
  const visible = tags.slice(0, visibleLimit);
  const overflow = Math.max(0, tags.length - visibleLimit);
  return (
    <span ref={ref} className="wk-row-tag-chips wk-kd-1" title={overflow ? tags.map((tag) => tag.name || "").join(", ") : undefined}>
      {visible.map((tag) => <span key={tag.id} className="row-tag wk-kd-2">{tag.name}</span>)}
      {overflow ? <span className="row-tag-overflow wk-kd-3">+{overflow}</span> : null}
    </span>
  );
}

function formatDocumentTime(value: unknown): string {
  if (typeof value !== "string" || !value) return "--";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--";
  const yy = String(date.getFullYear()).slice(2);
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${yy}-${month}-${day} ${hour}:${minute}`;
}

function documentTypeLabel(document: KnowledgeDocument): string {
  if (document.file_type) return document.file_type.toUpperCase();
  if (document.source === "url") return "URL";
  if (document.source === "manual") return "MANUAL";
  return "--";
}

function manualContentFromMetadata(metadata: unknown): { content: string; status: "draft" | "publish" } {
  try {
    const parsed = typeof metadata === "string" ? JSON.parse(metadata) : metadata;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const value = parsed as { content?: unknown; status?: unknown };
      return {
        content: typeof value.content === "string" ? value.content : "",
        status: value.status === "publish" ? "publish" : "draft",
      };
    }
  } catch {
    // The detail page still opens with an empty editor when legacy metadata is invalid.
  }
  return { content: "", status: "draft" };
}

function MoreIcon() { return <Icon size={16}><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></Icon>; }
function DownloadIcon() { return <Icon size={16}><path d="M12 3v12M7 10l5 5 5-5M5 21h14" /></Icon>; }
function RefreshIcon() { return <Icon size={16}><path d="M20 11a8 8 0 10-2.34 5.66M20 4v7h-7" /></Icon>; }
function DeleteIcon() { return <Icon size={16}><path d="M4 7h16M10 11v6M14 11v6M6 7l1 14h10l1-14M9 7V4h6v3" /></Icon>; }
function MoveIcon() { return <Icon size={16}><path d="M4 7h7l2 2h7v9a2 2 0 01-2 2H6a2 2 0 01-2-2z" /><path d="M12 11v6M9 14h6" /></Icon>; }
function AddFileIcon() { return <Icon size={16}><path d="M6 3h8l4 4v14H6z" /><path d="M14 3v5h5M12 12v6M9 15h6" /></Icon>; }
function ChevronDownIcon({ open = false }: { open?: boolean }) { return <Icon size={14} className={open ? "wk-kd-159" : "wk-kd-160"}><path d="m5 8 7 7 7-7" /></Icon>; }
// R483 F4 menu icons (Vue t-icon chart-bar / close-circle / swap / queue /
// chevron-left / root-list / arrow-right).
function ChartIcon() { return <Icon size={16}><path d="M4 20V10M10 20V4M16 20v-8M22 20H2" /></Icon>; }
function CancelParseIcon() { return <Icon size={16}><circle cx="12" cy="12" r="9" /><path d="M9 9l6 6M15 9l-6 6" /></Icon>; }
function SwapIcon() { return <Icon size={16}><path d="M4 7h13l-3-3M20 17H7l3 3" /></Icon>; }
function QueueIcon() { return <Icon size={16}><path d="M4 6h16M4 12h16M4 18h10" /></Icon>; }
function ChevronLeftIcon() { return <Icon size={16}><path d="m15 5-7 7 7 7" /></Icon>; }
function KBListIcon() { return <Icon size={16}><path d="M4 6h16M4 6v12a2 2 0 002 2h12a2 2 0 002-2V8a2 2 0 00-2-2h-8" /></Icon>; }
function ArrowRightIcon() { return <Icon size={14}><path d="M5 12h14M13 6l6 6-6 6" /></Icon>; }

function DocumentCardActionMenu({ document, canDownload, canMutateKnowledge, t, actions, traceAvailable, onMenuOpen, move, rowTrigger = false, onVisibleChange, onDownload, onEdit, onViewTrace, onMove, onBatchManage, onReparse, onCancelParse, onDelete }: {
  document: KnowledgeDocument;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
  actions: ReturnType<typeof documentRowActions>;
  /** Vue traceAvailableById probe result gating the 查看 Trace item. */
  traceAvailable?: boolean;
  /** Vue onMoreVisible(visible) → probeTraceAvailable. */
  onMenuOpen?: () => void;
  /** Cross-KB move sub-flow (Vue moveMenuMode targets/confirm views). */
  move?: DocumentMoveKbController;
  /** 列表行触发器：Vue row-more-btn（t-icon more）而非卡片 more-wrap。 */
  rowTrigger?: boolean;
  /** 列表行 menu-open 态回写（DocumentListRows moreOpenId）。 */
  onVisibleChange?: (visible: boolean) => void;
  onDownload: () => void;
  onEdit: () => void;
  onViewTrace: () => void;
  onMove: () => void;
  onBatchManage: () => void;
  onReparse: () => void;
  onCancelParse: () => void;
  onDelete: () => void;
}) {
  const [open, setOpen] = useState(false);
  const close = () => { setOpen(false); move?.onBack(); };
  // Vue DocumentActionMenu 菜单项（.doc-action-menu-item + t-icon .icon）。
  const menuItem = (label: string, icon: ReactNode, handler: () => void, danger = false) => (
    <div
      className={'doc-action-menu-item' + (danger ? ' danger' : '')}
      role="menuitem"
      onClick={(event) => { event.stopPropagation(); close(); handler(); }}
    >{icon}<span>{label}</span></div>
  );
  // Vue handleAction keeps the popup open for the move sub-flow (and its
  // folder-picker sibling); everything else closes the menu.
  const menuItemKeepOpen = (label: string, icon: ReactNode, handler: () => void) => (
    <div
      className="doc-action-menu-item"
      role="menuitem"
      onClick={(event) => { event.stopPropagation(); handler(); }}
    >{icon}<span>{label}</span></div>
  );
  const icons: Record<DocumentMenuAction, ReactNode> = {
    download: <TIcon className="icon" name="download" />,
    edit: <TIcon className="icon" name="edit" />,
    "view-trace": <TIcon className="icon" name="chart-bar" />,
    reparse: <TIcon className="icon" name="refresh" />,
    "cancel-parse": <TIcon className="icon" name="close-circle" />,
    "move-folder": <TIcon className="icon" name="folder" />,
    "move-kb": <TIcon className="icon" name="swap" />,
    "batch-manage": <TIcon className="icon" name="queue" />,
    delete: <TIcon className="icon" name="delete" />,
  };
  const handlers: Record<DocumentMenuAction, () => void> = {
    download: onDownload,
    edit: onEdit,
    "view-trace": onViewTrace,
    reparse: onReparse,
    "cancel-parse": onCancelParse,
    "move-folder": onMove,
    "move-kb": () => move?.onStart(),
    "batch-manage": onBatchManage,
    delete: onDelete,
  };
  const items = documentMenuItems({
    // Vue DocumentActionMenu gates download/edit on item.type, not source.
    source: document.type ?? undefined,
    parseStatus: document.parse_status,
    canDownload,
    canMutateKnowledge,
    traceAvailable,
  });
  const moveView = move && open && move.view !== "normal" ? move.view : undefined;
  return (
    <Popup
      visible={open}
      overlayClassName="card-more"
      trigger="click"
      destroyOnClose
      placement="bottom-right"
      onVisibleChange={(visible) => {
        if (visible) { setOpen(true); onMenuOpen?.(); onVisibleChange?.(true); return; }
        setOpen(false);
        move?.onBack();
        onVisibleChange?.(false);
      }}
      content={(
        <div className="card-menu" onClick={(event) => event.stopPropagation()}>
          {moveView === "targets" && move ? (
            <div className="move-menu" data-move-view="targets">
              <div className="move-menu-header" onClick={() => move.onBack()}>
                <TIcon name="chevron-left" size="16px" />
                <span>{t("knowledgeBase.moveToKnowledgeBase")}</span>
              </div>
              {move.loading ? (
                <div className="move-menu-loading"><TdLoading size="small" /></div>
              ) : move.targets.length === 0 ? (
                <div className="move-menu-empty">{t("knowledgeBase.moveNoTargets")}</div>
              ) : move.targets.map((kb) => (
                <div key={kb.id} className="card-menu-item" onClick={() => move.onSelectTarget(kb)}>
                  <TIcon className="icon" name="root-list" />
                  <span className="move-target-name">{kb.name}</span>
                  {kb.knowledge_count !== undefined ? <span className="move-target-count">{kb.knowledge_count}</span> : null}
                </div>
              ))}
            </div>
          ) : moveView === "confirm" && move ? (
            <div className="card-menu move-menu" data-move-view="confirm">
              <div className="move-menu-header" onClick={() => move.onBack()}>
                <TIcon name="chevron-left" size="16px" />
                <span>{t("knowledgeBase.moveConfirmTitle")}</span>
              </div>
              <div className="move-confirm-body">
                <div className="move-target-info">
                  <TIcon name="arrow-right" size="14px" />
                  <span>{move.selectedTargetName}</span>
                </div>
                {(["reuse_vectors", "reparse"] as const).map((mode) => (
                  <div
                    key={mode}
                    className={'move-mode-item' + (move.mode === mode ? ' active' : '')}
                    onClick={() => move.onModeChange(mode)}
                    role="radio"
                    aria-checked={move.mode === mode}
                  >
                    <TdRadio checked={move.mode === mode} />
                    <div className="move-mode-text">
                      <span className="move-mode-label">{t(mode === "reuse_vectors" ? "knowledgeBase.moveModeReuseVectors" : "knowledgeBase.moveModeReparse")}</span>
                      <span className="move-mode-desc">{t(mode === "reuse_vectors" ? "knowledgeBase.moveModeReuseVectorsDesc" : "knowledgeBase.moveModeReparseDesc")}</span>
                    </div>
                  </div>
                ))}
                <div className="move-confirm-actions">
                  <TdButton size="small" variant="outline" onClick={() => move.onBack()}>{t("common.cancel")}</TdButton>
                  <TdButton size="small" theme="primary" loading={move.submitting} onClick={() => move.onConfirm()}>{t("knowledgeBase.moveConfirm")}</TdButton>
                </div>
              </div>
            </div>
          ) : items.map((item) => item.action === "move-kb" && move
            ? <Fragment key={item.action}>{menuItemKeepOpen(t(item.labelKey), icons[item.action], handlers[item.action])}</Fragment>
            : item.action === "move-kb"
              ? <Fragment key={item.action}>{menuItem(t(item.labelKey), icons[item.action], handlers[item.action])}</Fragment>
              : <Fragment key={item.action}>{menuItem(t(item.labelKey), icons[item.action], handlers[item.action], item.action === "delete")}</Fragment>)}
        </div>
      )}
    >
      {rowTrigger ? (
        <button
          type="button"
          className="row-more-btn"
          aria-label={t("knowledgeBase.columnActions")}
        >
          <TIcon name="more" size="16px" />
        </button>
      ) : (
        <div
          className={'more-wrap' + (open ? ' active-more' : '')}
          aria-label={t("knowledgeBase.documents.title")}
          title={t("knowledgeBase.documents.title")}
          role="button"
          tabIndex={0}
          onClick={(event) => { event.stopPropagation(); setOpen((value) => { const next = !value; if (next) onMenuOpen?.(); return next; }); }}
        >
          <img className="more-icon" src={MORE_PNG} alt="" />
        </div>
      )}
    </Popup>
  );
}

export function documentCardHoverPosition(
  card: { left: number; right: number; top: number; bottom?: number },
  viewport: { width: number; height: number },
  popover = { width: 360, height: 300 },
  offset = 12,
): { x: number; y: number } {
  const rightX = card.right + offset;
  if (rightX + popover.width <= viewport.width - 10) {
    return { x: rightX, y: Math.max(10, Math.min(card.top, viewport.height - popover.height - 10)) };
  }
  const leftX = card.left - popover.width - offset;
  if (leftX >= 10) {
    return { x: leftX, y: Math.max(10, Math.min(card.top, viewport.height - popover.height - 10)) };
  }
  // Vue prefers an above-card fallback when neither side has room. Only use
  // the below-card position when the card is already near the top edge.
  const aboveY = card.top - popover.height - offset;
  if (aboveY >= 10) {
    return { x: Math.max(10, Math.min(card.left, viewport.width - popover.width - 10)), y: aboveY };
  }
  const belowY = (card.bottom ?? card.top + 136) + offset;
  if (belowY <= viewport.height - 10) {
    return { x: Math.max(10, Math.min(card.left, viewport.width - popover.width - 10)), y: belowY };
  }
  return { x: Math.max(10, Math.min(card.left, viewport.width - popover.width - 10)), y: Math.max(10, card.top - popover.height - offset) };
}

export function hasDocumentGridContent(items: readonly KnowledgeDocument[], folders: readonly { path: string }[]): boolean {
  return items.length > 0 || folders.length > 0;
}

/** Vue KnowledgeProcessingTimeline compact mode: stage dots plus the
 *  总耗时 caption the failed / in-flight document hover popover shows. */
function DocumentTraceCompact({ summary, t }: {
  summary: TraceSummary;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const caption = summary.totalMs > 0
    ? t("knowledgeStages.totalDuration", { d: summary.duration })
    : `${t("knowledgeBase.timeline.title")}：${summary.stageIndex}/${summary.stageTotal}${summary.activeStage ? ` · ${t(`knowledgeBase.timeline.stage.${summary.activeStage}`)}` : ""}`;
  return <div className="document-trace-compact" data-trace-total={summary.duration}>
    <div className="wk-kd-4">
      {summary.steps.map((step) => <span key={step.stage} aria-hidden className={`document-trace-dot document-trace-dot- wk-kd-135${step.state}`} title={`${t(`knowledgeBase.timeline.stage.${step.stage}`)} · ${t(`knowledgeBase.timeline.${step.state}`)}`} />)}
    </div>
    <div className="wk-kd-5">{caption}</div>
  </div>;
}

function DocumentCardHoverPopover({ document, position, t, loadTrace }: {
  document: KnowledgeDocument;
  position: { x: number; y: number };
  t: (key: string, values?: Record<string, string | number>) => string;
  /** Vue hover popover mounts KnowledgeProcessingTimeline with autoPoll=false:
   *  one spans fetch for the compact trace summary. */
  loadTrace?: (id: string) => Promise<TraceSummary | null>;
}) {
  const parseStatus = String(document.parse_status ?? "");
  const inFlight = parseStatus === "pending" || parseStatus === "processing" || parseStatus === "finalizing";
  const failed = parseStatus === "failed";
  const showTrace = inFlight || failed;
  const [trace, setTrace] = useState<TraceSummary | null>(null);
  useEffect(() => {
    if (!showTrace || !loadTrace) return;
    let active = true;
    void loadTrace(document.id).then((summary) => { if (active) setTrace(summary); }).catch(() => { /* keep the status fallback */ });
    return () => { active = false; };
  }, [document.id, showTrace, loadTrace]);
  const statusLabel = failed ? t("knowledgeBase.parsingFailed") : inFlight ? documentStatus(document, t).label : undefined;
  return <div className="knowledge-card-hover-popover wk-kd-6" style={{ left: position.x, top: position.y }} role="tooltip">
    <div className="wk-kd-7" title={displayName(document)}>{displayName(document)}</div>
    {showTrace ? <div className={`wk-kd-136 ${failed ? "wk-kd-137" : "wk-kd-138"}`}>{trace ? <DocumentTraceCompact summary={trace} t={t} /> : statusLabel}</div> : typeof document.description === "string" && document.description ? <div className="wk-kd-8">{document.description}</div> : null}
    {typeof document.source === "string" && document.source ? <div className="wk-kd-9" title={document.source}><LinkIcon size={12} /> <span className="wk-kd-10">{document.source}</span></div> : null}
    <div className="wk-kd-11">
      {document.created_at ? <span>{t("knowledgeBase.createdAt")}：{formatDocumentTime(document.created_at)}</span> : null}
      {document.updated_at ? <span>{t("knowledgeBase.updatedAt")}：{formatDocumentTime(document.updated_at)}</span> : null}
      <span>{documentTypeLabel(document)}</span>
    </div>
    {documentTags(document).length > 0 ? <div className="wk-kd-12">{documentTags(document).map((tag) => <span key={tag.id} className="wk-kd-13">{tag.name}</span>)}</div> : null}
    <div className="wk-kd-14">{t("knowledgeBase.clickToViewFull")}</div>
  </div>;
}

type DocumentViewMode = "grid" | "list";

export function documentSourceLabel(document: KnowledgeDocument, t: (key: string) => string): string {
  const channelKeys: Record<string, string> = {
    feishu: "knowledgeBase.channelFeishu", feishu_drive: "knowledgeBase.channelFeishuDrive", lark_drive: "knowledgeBase.channelLarkDrive",
    notion: "knowledgeBase.channelNotion", yuque: "knowledgeBase.channelYuque", gitlab: "knowledgeBase.channelGitLab",
    ima: "knowledgeBase.channelIma", wechat: "knowledgeBase.channelWechat", wecom: "knowledgeBase.channelWecom",
    dingtalk: "knowledgeBase.channelDingtalk", slack: "knowledgeBase.channelSlack", im: "knowledgeBase.channelIm",
  };
  const channel = typeof document.channel === "string" ? document.channel : "";
  if (channelKeys[channel]) return t(channelKeys[channel]);
  if (document.type === "url") return t("knowledgeBase.channelUrl");
  if (document.type === "manual" || document.source === "manual") return t("knowledgeBase.channelManual");
  return t("knowledgeBase.channelUpload");
}

export function documentFileSizeLabel(value: unknown): string {
  const bytes = typeof value === "number" ? value : typeof value === "string" && value.trim() ? Number(value) : NaN;
  if (!Number.isFinite(bytes) || bytes < 0) return "--";
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${Math.round(bytes / (1024 * 1024))} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function DocumentCardGrid({
  items,
  folders,
  selected,
  batchMode,
  canContribute,
  canMutateKnowledge: canMutateKnowledgeProp,
  canDownload,
  t,
  tagListCount,
  traceAvailableById,
  onProbeTrace,
  moveFor,
  loadTrace,
  onOpen,
  onOpenFolder,
  onToggle,
  onTagEdit,
  onReparse,
  onCancelParse,
  onDownload,
  onEdit,
  onViewTrace,
  onMove,
  onBatchManage,
  onDelete,
}: {
  items: KnowledgeDocument[];
  folders: Array<{ path: string; name: string; total_count: number }>;
  selected: Set<string>;
  batchMode: boolean;
  canContribute: boolean;
  canMutateKnowledge?: boolean;
  canDownload: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
  /** Vue tagList.length — 页面无任何标签时整个 card-tag-selector 不渲染（v-if）。 */
  tagListCount?: number;
  /** Vue traceAvailableById: probed /spans availability per document. */
  traceAvailableById?: Record<string, boolean>;
  onProbeTrace?: (document: KnowledgeDocument) => void;
  moveFor?: (document: KnowledgeDocument) => DocumentMoveKbController;
  loadTrace?: (id: string) => Promise<TraceSummary | null>;
  onOpen: (document: KnowledgeDocument) => void;
  onOpenFolder: (path: string) => void;
  onToggle: (id: string, checked: boolean) => void;
  onTagEdit: (document: KnowledgeDocument) => void;
  onReparse: (document: KnowledgeDocument) => void;
  onCancelParse: (document: KnowledgeDocument) => void;
  onDownload: (document: KnowledgeDocument) => void;
  onEdit: (document: KnowledgeDocument) => void;
  onViewTrace: (document: KnowledgeDocument) => void;
  onMove: (document: KnowledgeDocument) => void;
  onBatchManage: (document: KnowledgeDocument) => void;
  onDelete: (document: KnowledgeDocument) => void;
}) {
  const canMutateKnowledge = canMutateKnowledgeProp ?? canContribute;
  const [hovered, setHovered] = useState<{ document: KnowledgeDocument; position: { x: number; y: number } } | null>(null);
  const hoverTimer = useRef<number | null>(null);
  const clearHover = () => {
    if (hoverTimer.current !== null) window.clearTimeout(hoverTimer.current);
    hoverTimer.current = null;
    setHovered(null);
  };
  const scheduleHover = (event: ReactMouseEvent<HTMLElement>, document: KnowledgeDocument) => {
    if (hoverTimer.current !== null) window.clearTimeout(hoverTimer.current);
    const rect = event.currentTarget.getBoundingClientRect();
    hoverTimer.current = window.setTimeout(() => {
      setHovered({ document, position: documentCardHoverPosition(rect, { width: window.innerWidth, height: window.innerHeight }, { width: Math.min(360, window.innerWidth - 20), height: 300 }) });
    }, 300);
  };
  return <div className="doc-card-view" data-document-view="grid">
    <div className="doc-card-list doc-card-list-animated">
      {folders.map((folder) => (
        <div
          key={`folder-${folder.path}`}
          className="folder-card"
          title={folder.path}
          role="button"
          tabIndex={0}
          onClick={() => onOpenFolder(folder.path)}
          onKeyDown={(event) => { if (event.key === "Enter") onOpenFolder(folder.path); }}
        >
          <div className="folder-card__body">
            <TIcon name="folder" className="folder-card__icon" />
            <span className="folder-card__title">{folder.name}</span>
          </div>
          <div className="folder-card__footer">
            {t("knowledgeBase.folderTree.folderCardCount", { count: folder.total_count })}
          </div>
        </div>
      ))}
    {items.map((document) => {
      const status = documentStatus(document, t);
      const actions = documentRowActions(document.parse_status);
      const parseStatus = String(document.parse_status ?? "");
      const parseInFlight = parseStatus === "pending" || parseStatus === "processing" || parseStatus === "finalizing";
      const summaryInFlight = document.summary_status === "pending" || document.summary_status === "processing";
      const description = typeof document.description === "string" ? document.description : document.folder_path ?? t("knowledgeBase.documents.root");
      const tags = documentTags(document);
      return (
        <div
          key={document.id}
          className={'knowledge-card' + (selected.has(document.id) ? ' is-selected' : '') + (batchMode ? ' batch-mode' : '')}
          data-select-id={document.id}
          onClick={() => onOpen(document)}
          onMouseEnter={(event) => scheduleHover(event, document)}
          onMouseLeave={clearHover}
        >
          <div className="card-content">
            <div className="card-content-nav">
              {(canContribute || canDownload) && batchMode ? (
                <div className="card-nav-check" onClick={(event) => event.stopPropagation()}>
                  <TdCheckbox
                    className="card-select-checkbox t-size-s"
                    checked={selected.has(document.id)}
                    title={document.file_name}
                    onChange={(checked) => onToggle(document.id, Boolean(checked))}
                  />
                </div>
              ) : null}
              <span className="card-content-title" title={displayName(document)}>{displayName(document)}</span>
              {canContribute ? <DocumentCardActionMenu document={document} canDownload={canDownload} canMutateKnowledge={canMutateKnowledge} t={t} actions={actions} traceAvailable={traceAvailableById?.[document.id]} onMenuOpen={() => onProbeTrace?.(document)} move={moveFor?.(document)} onDownload={() => onDownload(document)} onEdit={() => onEdit(document)} onViewTrace={() => onViewTrace(document)} onMove={() => onMove(document)} onBatchManage={() => onBatchManage(document)} onReparse={() => onReparse(document)} onCancelParse={() => onCancelParse(document)} onDelete={() => onDelete(document)} /> : null}
            </div>
            {/* 解析状态区（Vue card-analyze 族） */}
            {parseInFlight ? (
              <div className="card-analyze card-analyze-trace">
                <TIcon name="loading" className="card-analyze-loading" />
                <span
                  className="card-analyze-txt card-analyze-trace-link"
                  role="button"
                  tabIndex={0}
                  title={t("knowledgeStages.viewTrace")}
                  onClick={(event) => { event.stopPropagation(); onViewTrace(document); }}
                >{status.label}</span>
                <button
                  type="button"
                  className="card-analyze-trace-btn"
                  title={t("knowledgeStages.viewTrace")}
                  aria-label={t("knowledgeStages.viewTrace")}
                  onClick={(event) => { event.stopPropagation(); onViewTrace(document); }}
                >
                  <TIcon name="chart-line" />
                </button>
              </div>
            ) : parseStatus === "failed" ? (
              <div className="card-analyze failure card-analyze-trace">
                <TIcon name="close-circle" className="card-analyze-loading failure" />
                <span
                  className="card-analyze-txt failure card-analyze-trace-link"
                  role="button"
                  tabIndex={0}
                  title={t("knowledgeStages.viewTrace")}
                  onClick={(event) => { event.stopPropagation(); onViewTrace(document); }}
                >{t("knowledgeBase.parsingFailed")}</span>
                <button
                  type="button"
                  className="card-analyze-trace-btn"
                  title={t("knowledgeStages.viewTrace")}
                  aria-label={t("knowledgeStages.viewTrace")}
                  onClick={(event) => { event.stopPropagation(); onViewTrace(document); }}
                >
                  <TIcon name="chart-bar" />
                </button>
              </div>
            ) : parseStatus === "draft" ? (
              <div className="card-draft">
                <TdTag size="small" theme="warning" variant="light-outline">{t("knowledgeBase.draft")}</TdTag>
                <span className="card-draft-tip">{t("knowledgeBase.draftTip")}</span>
              </div>
            ) : parseStatus === "completed" && summaryInFlight ? (
              <div className="card-analyze">
                <TIcon name="loading" className="card-analyze-loading" />
                <span className="card-analyze-txt">{t("knowledgeBase.generatingSummary")}</span>
              </div>
            ) : (
              <div className="card-content-txt">{description}</div>
            )}
          </div>
          <div className="card-bottom">
            {document.folder_path ? (
              <button type="button" className="card-folder" title={document.folder_path} onClick={(event) => { event.stopPropagation(); onOpenFolder(document.folder_path ?? ""); }}>
                <TIcon name="folder" />
                <span>{document.folder_path}</span>
              </button>
            ) : (
              <span className="card-time">{formatDocumentTime(document.updated_at ?? document.created_at)}</span>
            )}
            <div className="card-bottom-right">
              {(tagListCount ?? 0) > 0 && tags.length > 0 ? (
                <div className="card-tag-selector" onClick={(event) => event.stopPropagation()}>
                  <Tooltip content={tags.map((tag) => tag.name).join(", ")} placement="top">
                    <div className="card-tag-chips" onClick={() => { if (canContribute) onTagEdit(document); }}>
                      {tags.slice(0, 3).map((tag) => (
                        <TdTag key={tag.id} size="small" variant="light-outline" className="card-tag-chip">
                          <span className="tag-text">{tag.name}</span>
                        </TdTag>
                      ))}
                    </div>
                  </Tooltip>
                </div>
              ) : canContribute && (tagListCount ?? 0) > 0 ? (
                <div className="card-tag-selector" onClick={(event) => event.stopPropagation()}>
                  <span className="card-tag-add" onClick={() => onTagEdit(document)}>
                    <TIcon name="add" size="12px" />
                    <span>{t("knowledgeBase.tagLabel")}</span>
                  </span>
                </div>
              ) : null}
              <div className="card-type">
                <span>{documentTypeLabel(document)}</span>
              </div>
            </div>
          </div>
        </div>
      );
    })}
    </div>
    {hovered ? <DocumentCardHoverPopover document={hovered.document} position={hovered.position} t={t} loadTrace={loadTrace} /> : null}
  </div>;
}

/** Vue DocumentListView.vue 平移（.doc-list-view DOM：sticky 表头 + 网格行）。 */
function DocumentListRows({
  items,
  folders,
  showFolderTree,
  selected,
  canContribute,
  canMutateKnowledge,
  canDownload,
  t,
  tt,
  allOnPageSelected,
  someOnPageSelected,
  traceAvailableById,
  moveFor,
  onOpen,
  onOpenFolder,
  onToggleRow,
  onToggleAll,
  onProbeTrace,
  onTagEdit,
  onReparse,
  onCancelParse,
  onDownload,
  onEdit,
  onViewTrace,
  onMove,
  onBatchManage,
  onDelete,
}: {
  items: KnowledgeDocument[];
  folders: Array<{ path: string; name: string; total_count: number }>;
  showFolderTree: boolean;
  selected: Set<string>;
  canContribute: boolean;
  canMutateKnowledge?: boolean;
  canDownload: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
  tt: (key: string, values?: Record<string, string | number>) => string;
  allOnPageSelected: boolean;
  someOnPageSelected: boolean;
  traceAvailableById?: Record<string, boolean>;
  moveFor?: (document: KnowledgeDocument) => DocumentMoveKbController;
  onOpen: (document: KnowledgeDocument) => void;
  onOpenFolder: (path: string) => void;
  onToggleRow: (id: string, shiftKey: boolean) => void;
  onToggleAll: (value: boolean) => void;
  onProbeTrace?: (document: KnowledgeDocument) => void;
  onTagEdit: (document: KnowledgeDocument) => void;
  onReparse: (document: KnowledgeDocument) => void;
  onCancelParse: (document: KnowledgeDocument) => void;
  onDownload: (document: KnowledgeDocument) => void;
  onEdit: (document: KnowledgeDocument) => void;
  onViewTrace: (document: KnowledgeDocument) => void;
  onMove: (document: KnowledgeDocument) => void;
  onBatchManage: (document: KnowledgeDocument) => void;
  onDelete: (document: KnowledgeDocument) => void;
}) {
  const [moreOpenId, setMoreOpenId] = useState<string | null>(null);
  const canEditRow = canContribute || canDownload;
  return (
    <div className="doc-list-view">
      <div className="doc-list-sticky-sentinel" aria-hidden="true"></div>
      <div className="doc-list-header" role="row">
        <div className="cell cell-check" role="columnheader" onClick={(event) => event.stopPropagation()}>
          {canEditRow ? (
            <TdCheckbox
              className="doc-list-check t-size-s"
              checked={allOnPageSelected}
              indeterminate={someOnPageSelected}
              disabled={!items.length}
              title={t("knowledgeBase.selectAll")}
              onChange={(value) => onToggleAll(Boolean(value))}
            />
          ) : null}
        </div>
        <div className="cell cell-name" role="columnheader">{t("knowledgeBase.columnName")}</div>
        <div className="cell cell-tag" role="columnheader">{t("knowledgeBase.columnTag")}</div>
        <div className="cell cell-source" role="columnheader">{t("knowledgeBase.columnSource")}</div>
        <div className="cell cell-size" role="columnheader">{t("knowledgeBase.columnSize")}</div>
        <div className="cell cell-status" role="columnheader">{t("knowledgeBase.columnStatus")}</div>
        <div className="cell cell-time" role="columnheader">{t("knowledgeBase.columnUpdatedAt")}</div>
        {canContribute ? <div className="cell cell-actions" role="columnheader" /> : null}
      </div>

      <div className="doc-list-body">
        {/* Vue KnowledgeBase.vue L719-721: with the folder tree open it
            already lists the same folders, so the list skips duplicate
            sub-folder rows (tree closed keeps the navigable rows). */}
        {!showFolderTree ? folders.map((folder) => (
          <div
            key={`folder-${folder.path}`}
            className="doc-list-row doc-list-row--folder"
            title={folder.path}
            role="row"
            onClick={() => onOpenFolder(folder.path)}
          >
            <div className="cell cell-check" aria-hidden="true"></div>
            <div className="cell cell-name">
              <span className="row-file-icon-wrap">
                <TIcon name="folder" className="row-folder-icon" />
              </span>
              <div className="row-file-text">
                <span className="row-file-name">{folder.name}</span>
              </div>
            </div>
            <div className="cell cell-tag"></div>
            <div className="cell cell-source">
              <span className="row-folder-meta">
                {t("knowledgeBase.folderTree.folderCardCount", { count: folder.total_count })}
              </span>
            </div>
            <div className="cell cell-size"></div>
            <div className="cell cell-status"></div>
            <div className="cell cell-time"></div>
            {canContribute ? <div className="cell cell-actions" aria-hidden="true"></div> : null}
          </div>
        )) : null}

        {items.map((document) => {
          const status = listRowStatus(document, t);
          const actions = documentRowActions(document.parse_status);
          const source = getSourceInfo(document, t);
          const tags = documentTags(document);
          return (
            <div
              key={document.id}
              className={'doc-list-row' + (selected.has(document.id) ? ' selected' : '') + (moreOpenId === document.id ? ' menu-open' : '')}
              data-select-id={document.id}
              role="row"
              onClick={() => onOpen(document)}
            >
              <div className="cell cell-check" onClick={(event) => event.stopPropagation()}>
                {canEditRow ? (
                  <TdCheckbox
                    className="doc-list-check t-size-s"
                    checked={selected.has(document.id)}
                    title={document.file_name}
                    onChange={(_value, ctx) => onToggleRow(document.id, Boolean((ctx as { e?: { shiftKey?: boolean } } | undefined)?.e?.shiftKey))}
                  />
                ) : null}
              </div>

              <div className="cell cell-name">
                <span className="row-file-icon-wrap">
                  <TIcon name={getFileIconName(document)} />
                </span>
                <div className="row-file-text">
                  <span className="row-file-name" title={displayName(document)}>{displayName(document)}</span>
                  {document.folder_path ? (
                    <button type="button" className="row-file-folder" title={document.folder_path} onClick={(event) => { event.stopPropagation(); onOpenFolder(document.folder_path ?? ""); }}>
                      <TIcon name="folder" />
                      <span>{document.folder_path}</span>
                    </button>
                  ) : null}
                  {document.description ? <span className="row-file-desc" title={String(document.description)}>{String(document.description)}</span> : null}
                </div>
              </div>

              <div className="cell cell-tag">
                {tags.length > 0 ? (
                  <Tooltip content={tags.map((tag) => tag.name).join(", ")} placement="top">
                    <div className={'row-tag-chips' + (canContribute ? ' is-clickable' : '')} onClick={(event) => { event.stopPropagation(); if (canContribute) onTagEdit(document); }}>
                      {tags.slice(0, 3).map((tag) => (
                        <TdTag key={tag.id} size="small" variant="light-outline" className="row-tag">{tag.name}</TdTag>
                      ))}
                    </div>
                  </Tooltip>
                ) : (
                  <span className="row-tag-chips is-clickable" onClick={(event) => { event.stopPropagation(); if (canContribute) onTagEdit(document); }}>
                    <span className="row-tag-add">+ {tt("knowledgeBase.tagLabel")}</span>
                  </span>
                )}
              </div>

              <div className="cell cell-source">
                <TIcon className="row-source-icon" name={source.icon} />
                <span className="row-source-label">{source.label}</span>
              </div>

              <div className="cell cell-size">
                <span className="row-mono">{documentFileSizeLabel(document.file_size) || "--"}</span>
              </div>

              <div className="cell cell-status">
                {status.label !== "--" ? (
                  <TdTag size="small" theme={status.theme} variant="light-outline" className="row-status-tag" icon={status.icon ? <TIcon name={status.icon ?? ""} className={status.spin ? 'icon-spin' : undefined} /> : undefined}>
                    {status.label}
                  </TdTag>
                ) : (
                  <span className="row-muted">--</span>
                )}
              </div>

              <div className="cell cell-time">
                <span className="row-mono">{formatDocumentTime(document.updated_at ?? document.created_at)}</span>
              </div>

              {canContribute ? (
                <div className="cell cell-actions" onClick={(event) => event.stopPropagation()}>
                  <ListRowMoreMenu
                    document={document}
                    canDownload={canDownload}
                    canMutateKnowledge={canMutateKnowledge ?? canContribute}
                    t={t}
                    actions={actions}
                    traceAvailable={traceAvailableById?.[document.id]}
                    onMenuOpen={(visible) => { setMoreOpenId(visible ? document.id : null); if (visible) onProbeTrace?.(document); }}
                    move={moveFor?.(document)}
                    onDownload={() => onDownload(document)}
                    onEdit={() => onEdit(document)}
                    onViewTrace={() => onViewTrace(document)}
                    onMove={() => onMove(document)}
                    onBatchManage={() => onBatchManage(document)}
                    onReparse={() => onReparse(document)}
                    onCancelParse={() => onCancelParse(document)}
                    onDelete={() => onDelete(document)}
                  />
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}

/** Vue DocumentListView 行尾 t-popup + row-more-btn（三点菜单，t-icon more）。 */
function ListRowMoreMenu(props: {
  document: KnowledgeDocument;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
  actions: ReturnType<typeof documentRowActions>;
  traceAvailable?: boolean;
  onMenuOpen: (visible: boolean) => void;
  move?: DocumentMoveKbController;
  onDownload: () => void;
  onEdit: () => void;
  onViewTrace: () => void;
  onMove: () => void;
  onBatchManage: () => void;
  onReparse: () => void;
  onCancelParse: () => void;
  onDelete: () => void;
}) {
  return (
    <DocumentCardActionMenu
      document={props.document}
      canDownload={props.canDownload}
      canMutateKnowledge={props.canMutateKnowledge}
      t={props.t}
      actions={props.actions}
      traceAvailable={props.traceAvailable}
      onMenuOpen={() => props.onMenuOpen(true)}
      move={props.move}
      onDownload={props.onDownload}
      onEdit={props.onEdit}
      onViewTrace={props.onViewTrace}
      onMove={props.onMove}
      onBatchManage={props.onBatchManage}
      onReparse={props.onReparse}
      onCancelParse={props.onCancelParse}
      onDelete={props.onDelete}
      rowTrigger
      onVisibleChange={props.onMenuOpen}
    />
  );
}

export function documentStatus(
  document: KnowledgeDocument,
  t: (key: string) => string,
): { label: string; tone: "neutral" | "success" | "warning" | "error" } {
  if (!document.parse_status)
    return {
      label: t("knowledgeBase.documents.statusUnknown"),
      tone: "warning",
    };
  let status: ReturnType<typeof normalizeKnowledgeProcessingStatus>;
  try {
    status = normalizeKnowledgeProcessingStatus(document.parse_status);
  } catch {
    return {
      label: t("knowledgeBase.documents.statusUnknown"),
      tone: "warning",
    };
  }
  const statusLabelKey: Partial<Record<typeof status, string>> = {
    pending: "knowledgeBase.parseStatusPending",
    processing: "knowledgeBase.parseStatusProcessing",
    finalizing: "knowledgeBase.parseStatusFinalizing",
    completed: "knowledgeBase.parseStatusCompleted",
    failed: "knowledgeBase.parseStatusFailed",
    cancelled: "knowledgeBase.parseStatusCancelled",
  };
  // Vue DocumentCardView.vue uses the card-specific in-flight copy rather
  // than the filter/status option copy: pending/processing are “解析中...”,
  // while finalizing may be “生成摘要中” or “即将完成”.
  const label = status === "pending" || status === "processing"
    ? t("knowledgeBase.parsingInProgress")
    : status === "finalizing" && (document.summary_status === "pending" || document.summary_status === "processing")
      ? t("knowledgeBase.generatingSummary")
      : statusLabelKey[status] ? t(statusLabelKey[status]!) : t("knowledgeBase.documents.statusUnknown");
  if (status === "completed" && (document.summary_status === "pending" || document.summary_status === "processing")) {
    return { label: t("knowledgeBase.generatingSummary"), tone: "warning" };
  }
  if (status === "completed") return { label, tone: "success" };
  if (status === "failed") return { label, tone: "error" };
  if (status === "cancelled") return { label, tone: "warning" };
  return { label, tone: "warning" };
}

function errorMessage(error: unknown, t?: (key: string) => string): string {
  const candidate = error as {
    code?: unknown;
    message?: unknown;
    status?: unknown;
  };
  if (candidate?.status === 413 || candidate?.code === "PAYLOAD_TOO_LARGE")
    return t?.("common.error") ?? "Upload is too large (413). Choose a smaller file and retry.";
  return candidate?.message && typeof candidate.message === "string"
    ? candidate.message
    : t?.("common.error") ?? "The document operation failed.";
}

function emitKnowledgeUploadEvent(name: string, detail: Record<string, unknown>): void {
  if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent(name, { detail }));
}

/** process_overrides a document stored at upload time (reparse seed). */
function readStoredProcessOverrides(document: KnowledgeDocument): Parameters<typeof applyUploadOverrides>[1] {
  const metadata = document.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata)) {
    return (metadata as { process_overrides?: unknown }).process_overrides as Parameters<typeof applyUploadOverrides>[1] ?? null;
  }
  return null;
}

// --- Destination picker (Vue FolderPickerMenu.vue parity) ---------------------

export interface UploadDestinationPickerLabels {
  pickerLabel: string;
  rootRow: string;
  newFolderPlaceholder: string;
  newFolderAddRoot: string;
  newFolderAddUnder: (folder: string) => string;
  duplicate: string;
}

export interface UploadDestinationPickerProps {
  options: FolderOption[];
  currentPath: string;
  creatingUnder: string | null;
  newFolderName: string;
  duplicateWarning: boolean;
  labels: UploadDestinationPickerLabels;
  onChoose: (path: string) => void;
  onStartCreate: (parentPath: string) => void;
  onCancelCreate: () => void;
  onNewFolderNameChange: (value: string) => void;
  onCommitNewFolder: () => void;
}

/**
 * Folder tree/list the Vue upload dialog mounts inside its destination popup
 * (FolderPickerMenu.vue): root row, depth-indented folders, a per-row
 * "new sub-folder" affordance with an inline input (Enter commits, Esc
 * cancels), a check on the current destination and a duplicate warning.
 */
export function UploadDestinationPicker(props: UploadDestinationPickerProps) {
  const rows = folderPickerRows(props.options, props.creatingUnder, props.labels.rootRow);
  return (
    <div className="wk-folder-picker" style={{ minWidth: "208px", maxWidth: "280px" }}>
      <ul
        className="wk-folder-picker__list"
        aria-label={props.labels.pickerLabel}
        style={{ margin: 0, padding: 0, listStyle: "none", maxHeight: "260px", overflowY: "auto" }}
      >
        {rows.map((row) =>
          row.kind === "folder" ? (
            <li
              key={row.key}
              data-folder-path={row.path || undefined}
              className={props.currentPath === row.path ? "wk-folder-picker__item is-current" : "wk-folder-picker__item"}
              aria-current={props.currentPath === row.path ? "true" : undefined}
              title={row.path || undefined}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                height: "30px",
                boxSizing: "border-box",
                padding: `0 8px 0 ${row.depth * 12 + 10}px`,
                borderRadius: "6px",
                fontSize: "0.9rem",
                cursor: props.currentPath === row.path ? "default" : "pointer",
              }}
              onClick={() => props.onChoose(row.path)}
            >
              <FolderIcon size={16} className="wk-kd-15" />
              <span
                style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
              >
                {row.label}
              </span>
              <button
                type="button"
                className="wk-folder-picker__add"
                title={row.isRoot ? props.labels.newFolderAddRoot : props.labels.newFolderAddUnder(row.label)}
                aria-label={row.isRoot ? props.labels.newFolderAddRoot : props.labels.newFolderAddUnder(row.label)}
                onClick={(event) => {
                  event.stopPropagation();
                  props.onStartCreate(row.path);
                }}
              >
                ＋
              </button>
              {props.currentPath === row.path ? (
                <span aria-hidden className="wk-folder-picker__current">✓</span>
              ) : null}
            </li>
          ) : (
            <li
              key={row.key}
              className="wk-folder-picker__item--create"
              style={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                height: "30px",
                boxSizing: "border-box",
                padding: `0 8px 0 ${row.depth * 12 + 22}px`,
              }}
              onClick={(event) => event.stopPropagation()}
            >
              <FolderIcon size={16} />
              <Input
                className="wk-folder-picker__input"
                value={props.newFolderName}
                placeholder={props.labels.newFolderPlaceholder}
                aria-label={props.labels.newFolderPlaceholder}
                onChange={(event) => props.onNewFolderNameChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    props.onCommitNewFolder();
                  } else if (event.key === "Escape") {
                    event.preventDefault();
                    props.onCancelCreate();
                  }
                }}
                style={{ flex: 1, minWidth: 0, height: "24px", padding: "0 6px", border: "1px solid var(--wk-accent, #4a7dff)", borderRadius: "4px" }}
              />
            </li>
          ),
        )}
      </ul>
      {props.duplicateWarning ? (
        <p role="alert" className="wk-muted wk-kd-16" style={{ margin: "4px 0 0", fontSize: "0.85rem" }}>
          {props.labels.duplicate}
        </p>
      ) : null}
    </div>
  );
}

// --- Files panel (Vue UploadConfirmDialog files-panel parity) -----------------

export interface UploadFilesPanelLabels {
  urlItemLabel: string;
  remove: string;
  noItems: string;
  manualCharCount: (count: number) => string;
  reparseSource: string;
  reparseHint: string;
  statusLabel: (status: UploadEntryStatus) => string;
}

export interface UploadFilesPanelProps {
  mode: UploadDialogMode;
  entries: UploadEntry[];
  urls: string[];
  uploadStates: readonly UploadEntryState[];
  manualTitle?: string;
  manualCharCount?: number;
  reparseFileName?: string;
  uploading: boolean;
  labels: UploadFilesPanelLabels;
  onRemoveUrl: (index: number) => void;
  onRemoveEntry: (index: number) => void;
}

function entryStatusTone(status: UploadEntryStatus | undefined): "neutral" | "success" | "warning" | "error" {
  if (status === "done") return "success";
  if (status === "error") return "error";
  if (status === "uploading") return "warning";
  return "neutral";
}

/** Type badge derived from the file name (Vue getFileIcon role). */
function fileTypeBadge(name: string): string {
  const dot = name.lastIndexOf(".");
  const ext = dot < 0 ? "" : name.slice(dot + 1).toUpperCase();
  return ext || "FILE";
}

/**
 * The dialog's left panel: a manual publish preview, the reparse source, or
 * the staged URL + file rows with relative directories, per-file status and
 * single-item removal.
 */
export function UploadFilesPanel(props: UploadFilesPanelProps) {
  const { labels } = props;
  if (props.mode === "manual") {
    return (
      <div className="wk-upload-manual-panel" style={{ marginBottom: "0.75rem" }}>
        <p title={props.manualTitle} style={{ fontWeight: 600, margin: "0 0 2px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {props.manualTitle}
        </p>
        <p className="wk-muted wk-kd-16" style={{ margin: 0, fontSize: "0.85rem" }}>
          {labels.manualCharCount(props.manualCharCount ?? 0)}
        </p>
      </div>
    );
  }
  if (props.mode === "reparse") {
    return (
      <div className="wk-upload-reparse-panel" style={{ marginBottom: "0.75rem" }}>
        <p title={props.reparseFileName} style={{ fontWeight: 600, margin: "0 0 2px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {props.reparseFileName || labels.reparseSource}
        </p>
        <p className="wk-muted wk-kd-16" style={{ margin: 0, fontSize: "0.85rem" }}>
          {labels.reparseHint}
        </p>
      </div>
    );
  }
  const itemCount = props.entries.length + props.urls.length;
  if (itemCount === 0) {
    return <p className="wk-muted wk-kd-16" style={{ margin: "0 0 0.75rem" }}>{labels.noItems}</p>;
  }
  return (
    <ul className="wk-upload-confirm-files wk-kd-17">
      {props.urls.map((url, index) => (
        <li key={`url-${url}-${index}`} className="wk-kd-18">
          <span className="wk-kd-19"><LinkIcon size={16} /></span>
          <div className="wk-kd-20">
            <span className="wk-kd-21" title={url}>{url}</span>
            <span className="wk-kd-22">{labels.urlItemLabel}</span>
          </div>
          <button type="button" className="wk-kd-23" disabled={props.uploading} aria-label={labels.remove} onClick={() => props.onRemoveUrl(index)}>×</button>
        </li>
      ))}
      {props.entries.map((entry, index) => {
        const state = props.uploadStates[index];
        const relativeDir = uploadEntryRelativeDir(entry);
        return (
        <li key={`${entry.name}-${index}`} className="wk-kd-18">
            <span className="wk-kd-24" aria-hidden>{fileTypeBadge(entry.name)}</span>
            <div className="wk-kd-20">
              <span className="wk-kd-21" title={uploadEntryDisplayTitle(entry)}>{entry.name}</span>
              <span className="wk-kd-22">
              {relativeDir ? (
                <>
                  <span title={relativeDir}>{relativeDir}</span>
                  <span aria-hidden> · </span>
                </>
              ) : null}
              {formatBytes(entry.size)}
              </span>
            </div>
            <Status tone={entryStatusTone(state?.status)}>
              {state?.status === "error" ? (state.message ?? labels.statusLabel("error")) : labels.statusLabel(state?.status ?? "pending")}
            </Status>
            <button type="button" className="wk-kd-23" disabled={props.uploading} aria-label={labels.remove} onClick={() => props.onRemoveEntry(index)}>×</button>
          </li>
        );
      })}
    </ul>
  );
}

// --- Section nav (Vue settings-nav parity) -----------------------------------

export interface UploadSectionNavItem {
  key: UploadConfirmSectionKey;
  label: string;
  status: string;
  statusTitle: string;
  tone?: "warning" | "error" | "muted";
  issue: boolean;
  /** Vue t-icon name (only some entries carry one, e.g. graph: chart-bubble). */
  icon?: "chart-bubble";
  /** Vue activeSection === item.key. */
  active?: boolean;
}

/** Minimal inline stand-in for the Vue t-icon "chart-bubble" glyph. */
function NavIcon({ name }: { name: "chart-bubble" }) {
  if (name === "chart-bubble") {
    return (
      <svg className="wk-upload-nav-icon wk-kd-25" width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden style={{ flex: "0 0 auto" }}>
        <circle cx="5" cy="5" r="3" />
        <circle cx="11.5" cy="10.5" r="2.5" />
        <circle cx="4.5" cy="12" r="1.8" />
      </svg>
    );
  }
  return null;
}

export interface UploadSectionNavProps {
  items: UploadSectionNavItem[];
  navLabel: string;
  onSelect: (key: UploadConfirmSectionKey) => void;
}

/** Vue truncateNavText: nav status text is clamped with an ellipsis, full text on hover. */
function truncateNavText(text: string, max = 18): string {
  if (!text) return text;
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

export function UploadSectionNav(props: UploadSectionNavProps) {
  return (
    <nav className="wk-upload-section-nav" aria-label={props.navLabel}>
      {props.items.map((item) => (
        <button
          key={item.key}
          type="button"
          className={[
            "wk-upload-nav-item",
            "wk-kd-ring wk-kd-161",
            item.active ? "is-active wk-kd-162" : "",
            item.issue ? "has-issue wk-kd-163" : "",
          ].filter(Boolean).join(" ")}
          aria-current={item.active ? "true" : undefined}
          data-section-target={item.key}
          style={{
            display: "flex",
            alignItems: "flex-start",
            width: "100%",
            minHeight: "38px",
            gap: "6px",
            marginBottom: "0",
            padding: "9px 10px",
            border: "none",
            borderRadius: "6px",
            background: "transparent",
            cursor: "pointer",
            textAlign: "left",
            fontSize: "14px",
            transition: "all 0.2s ease",
          }}
          onClick={() => props.onSelect(item.key)}
        >
          {item.icon ? <NavIcon name={item.icon} /> : null}
          <span className="wk-kd-26"><span className="wk-kd-27">{item.label}</span>
          <span
            className={`wk-upload-nav-status tone-${item.tone ?? "default"} wk-kd-139`}
            title={item.statusTitle}
            style={{ color: item.tone === "error" ? "var(--wk-danger, #d92d20)" : item.tone === "warning" ? "var(--wk-warning, #b54708)" : "var(--wk-muted, #667085)" }}
          >
            {truncateNavText(item.status)}
          </span>
          </span>
          {item.issue ? <span className="wk-upload-nav-dot" aria-hidden style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--wk-danger, #d92d20)" }} /> : null}
        </button>
      ))}
    </nav>
  );
}

// --- Add-source dropdown (Vue KbUploadSourceDropdown parity) -------------------

export type UploadSourceDropdownAction = "file" | "folder" | "url" | "manual";

export interface UploadSourceDropdownProps {
  /** Vue tooltip prop — the uploadConfirm.continueAdd copy in this dialog. */
  tooltip: string;
  items: { key: UploadSourceDropdownAction; label: string }[];
  open: boolean;
  onToggle: () => void;
  /** Bubbled for the URL entry (which opens the import sub-dialog). */
  onSelect: (key: UploadSourceDropdownAction) => void;
  /** Picked files from the hidden multiple / webkitdirectory inputs. */
  onFiles: (files: File[], fromFolder?: boolean) => void;
  /** Guide spotlight anchor — Vue KbUploadSourceDropdown trigger carries
   * data-guide="kb-detail-add-doc" (KnowledgeBase.vue:2613). Only the
   * page-level dropdown passes it; the dialog's "continue add" stays anonymous. */
  guideTarget?: string;
}

/**
 * The dialog's "continue add" affordance (Vue KbUploadSourceDropdown.vue):
 * one trigger opening a file/folder/URL menu; the menu entries drive hidden
 * multiple and webkitdirectory file inputs, URL opens the import sub-dialog.
 */
export function UploadSourceDropdown(props: UploadSourceDropdownProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  function openNativeInput(kind: "file" | "folder") {
    const input = wrapRef.current?.querySelector<HTMLInputElement>(`input[data-upload-source-input="${kind}"]`);
    input?.click();
  }
  function handleAction(key: UploadSourceDropdownAction) {
    props.onSelect(key);
    if (key === "file" || key === "folder") openNativeInput(key);
  }
  // Vue KbUploadSourceDropdown prefixIcon 表（upload/folder-add/link/edit-1）。
  const itemIcon: Record<UploadSourceDropdownAction, string> = {
    file: "upload",
    folder: "folder-add",
    url: "link",
    manual: "edit-1",
  };
  return (
    <div ref={wrapRef} className="kb-upload-source-dropdown">
      <input
        type="file"
        multiple
        className="hidden-file-input"
        data-upload-source-input="file"
        autoComplete="off"
        aria-hidden
        tabIndex={-1}
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length > 0) props.onFiles(files, event.currentTarget.dataset.uploadSourceInput === "folder");
        }}
      />
      <input
        type="file"
        multiple
        {...({ webkitdirectory: "" } as Record<string, unknown>)}
        className="hidden-file-input"
        data-upload-source-input="folder"
        autoComplete="off"
        aria-hidden
        tabIndex={-1}
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length > 0) props.onFiles(files, event.currentTarget.dataset.uploadSourceInput === "folder");
        }}
      />
      <Tooltip content={props.tooltip} placement="top">
        <Dropdown
          trigger="click"
          placement="bottom-right"
          options={props.items.map((item) => ({
            content: item.label,
            value: item.key,
            prefixIcon: <TIcon name={itemIcon[item.key]} size="16px" />,
          }))}
          popupProps={{ onVisibleChange: (visible: boolean) => setDropdownOpen(visible) }}
          onClick={(data) => { setDropdownOpen(false); handleAction((data as { value: UploadSourceDropdownAction }).value); }}
        >
          <TdButton
            variant="text"
            theme="default"
            className="kb-upload-source-trigger content-bar-icon-btn"
            data-guide={props.guideTarget}
            size="small"
            aria-label={props.tooltip}
            title={props.tooltip}
            aria-haspopup="menu"
            aria-expanded={dropdownOpen || props.open}
          >
            <TIcon name="file-add" size="16px" />
          </TdButton>
        </Dropdown>
      </Tooltip>
    </div>
  );
}

// --- Config sections (Vue config-panel parity) --------------------------------

export interface UploadConfirmSectionsProps {
  state: UploadConfirmUIState;
  update: (patch: Partial<UploadConfirmUIState>) => void;
  hasPdf: boolean;
  multimodalIssue: boolean;
  asrIssue: boolean;
  parserEngines: ParserEngineInfo[];
  /** Vue KBParserSettings loading state; prevents an empty engine list from masquerading as loaded. */
  parserLoading?: boolean;
  /** Vue goToParserSettings callback for a file family with no available engine. */
  onConfigureParserSettings?: () => void;
  vllmModels: ModelConfiguration[];
  asrModels: ModelConfiguration[];
  moreOpen: boolean;
  onToggleMore: () => void;
  /** Vue v-show activeSection: keep inactive sections mounted, but hidden. */
  activeSection?: UploadConfirmSectionKey;
  /** Vue isGraphSectionAvailable: gate the graph section (v-if parity). */
  graphAvailable?: boolean;
  /** The UploadGraphSettings element rendered inside the gated graph fieldset. */
  graphSettings?: React.ReactNode;
  /** Shared-i18n-first translator (covers knowledgeEditor.* / settings.*). */
  t: UploadDialogT;
}

const CHUNKING_STRATEGY_OPTIONS = [
  { value: "auto", labelKey: "knowledgeEditor.chunking.strategies.auto.label" },
  { value: "heading", labelKey: "knowledgeEditor.chunking.strategies.heading.label" },
  { value: "heuristic", labelKey: "knowledgeEditor.chunking.strategies.heuristic.label" },
  { value: "legacy", labelKey: "knowledgeEditor.chunking.strategies.legacy.label" },
] as const;

export function UploadSingleSelect({ value, options, onChange, ariaLabel, className = "", placeholder = "", clearable = false }: {
  value: string;
  options: readonly { value: string; label: string; disabled?: boolean }[];
  onChange: (value: string) => void;
  ariaLabel: string;
  className?: string;
  placeholder?: string;
  clearable?: boolean;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const selectedIndex = options.findIndex((option) => option.value === value);
  const [activeIndex, setActiveIndex] = useState(Math.max(0, selectedIndex));
  useEffect(() => setActiveIndex(Math.max(0, selectedIndex)), [selectedIndex]);
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);
  const choose = (index: number) => { const option = options[index]; if (!option || option.disabled) return; onChange(option.value); setOpen(false); setActiveIndex(index); };
  return <div className={`wk-upload-single-select wk-kd-singlesel ${className}`.trim()} ref={rootRef}>
    <button type="button" className="wk-upload-single-select__trigger wk-kd-28" role="combobox" aria-label={ariaLabel} aria-expanded={open} onClick={() => setOpen((current) => !current)} onKeyDown={(event) => {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setOpen(true); setActiveIndex((current) => Math.max(0, Math.min(options.length - 1, current + (event.key === "ArrowDown" ? 1 : -1)))); }
      else if (event.key === "Enter" && open) { event.preventDefault(); choose(activeIndex); }
      else if (event.key === "Escape") setOpen(false);
    }}><span>{options[selectedIndex]?.label ?? placeholder ?? value}</span>{clearable && value ? <span role="button" tabIndex={0} aria-label={`清除${ariaLabel}`} onClick={(event) => { event.stopPropagation(); onChange(""); setOpen(false); }}>×</span> : null}<span aria-hidden="true">⌄</span></button>
    {open ? <div className="wk-upload-single-select__popup wk-kd-29" role="listbox">{options.map((option, index) => <button type="button" role="option" aria-selected={option.value === value} aria-disabled={option.disabled || undefined} disabled={option.disabled} className={`wk-kd-selopt ${index === activeIndex ? "is-active" : ""} ${option.disabled ? "is-disabled" : ""} ${option.value === value ? "wk-kd-selopt-selected" : ""}`.trim()} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}>{option.label}</button>)}</div> : null}
  </div>;
}

export function UploadMultiSelect({ values, options, onChange, ariaLabel }: {
  values: readonly string[];
  options: readonly { value: string; label: string }[];
  onChange: (values: string[]) => void;
  ariaLabel: string;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const filtered = options.filter((option) => option.label.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) { setOpen(false); setQuery(""); } };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);
  const toggle = (value: string) => onChange(values.includes(value) ? values.filter((item) => item !== value) : [...values, value]);
  return <div className="wk-upload-multi-select wk-kd-30" ref={rootRef}>
    <div className="wk-upload-multi-select__field wk-kd-31" onClick={() => setOpen(true)}>
      {values.map((value) => <span className="wk-upload-multi-select__chip wk-kd-32" key={value}>{options.find((option) => option.value === value)?.label ?? value}<button type="button" className="wk-kd-33" aria-label={`移除 ${value}`} onClick={(event) => { event.stopPropagation(); toggle(value); }}>×</button></span>)}
      <input type="text" className="wk-kd-34" role="combobox" aria-label={ariaLabel} aria-expanded={open} value={query} placeholder={values.length ? "" : ariaLabel} onFocus={() => setOpen(true)} onChange={(event) => { setQuery(event.target.value); setActiveIndex(0); setOpen(true); }} onKeyDown={(event) => {
        if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setActiveIndex((current) => Math.max(0, Math.min(filtered.length - 1, current + (event.key === "ArrowDown" ? 1 : -1)))); }
        else if (event.key === "Enter" && filtered[activeIndex]) { event.preventDefault(); toggle(filtered[activeIndex].value); setQuery(""); }
        else if (event.key === "Backspace" && !query && values.length) toggle(values[values.length - 1]);
        else if (event.key === "Escape") { setOpen(false); setQuery(""); }
      }} />
    </div>
    {open ? <div className="wk-upload-multi-select__popup wk-kd-29" role="listbox" aria-label={ariaLabel}>{filtered.map((option, index) => <button type="button" role="option" aria-selected={values.includes(option.value)} className={`${index === activeIndex ? "is-active wk-kd-140" : "wk-kd-141"} wk-kd-142`} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => toggle(option.value)}>{option.label}</button>)}{filtered.length === 0 ? <span className="wk-muted wk-kd-16">{ariaLabel}</span> : null}</div> : null}
  </div>;
}

export function UploadClearableInput({ value, placeholder, ariaLabel, onChange }: { value: string; placeholder: string; ariaLabel: string; onChange: (value: string) => void }) {
  return <div className="wk-upload-clearable-input wk-kd-35">
    <Input className="wk-kd-36" value={value} placeholder={placeholder} aria-label={ariaLabel} onChange={(event) => onChange(event.target.value)} />
    {value ? <button type="button" className="wk-kd-37" aria-label={`清除${ariaLabel}`} onClick={() => onChange("")}>×</button> : null}
  </div>;
}

export function UploadNumberInput({ value, min, max, step, ariaLabel, onChange, className = "" }: {
  value: number;
  min: number;
  max: number;
  step: number;
  ariaLabel: string;
  onChange: (value: number) => void;
  className?: string;
}) {
  const clamp = (next: number) => Math.min(max, Math.max(min, Number.isFinite(next) ? next : value));
  const adjust = (delta: number) => onChange(clamp(value + delta));
  // CSS cascade: .wk-upload-number-input--wide overrode the 88px base width.
  const wide = className.includes("wk-upload-number-input--wide");
  return <div className={`wk-upload-number-input ${className} wk-kd-num ${wide ? "wk-kd-num--wide" : ""}`.trim()}>
    <button type="button" className="wk-kd-38" aria-label={`减少${ariaLabel}`} disabled={value <= min} onClick={() => adjust(-step)}>−</button>
    <input
      type="number"
      className="wk-kd-num-input wk-kd-39"
      min={min}
      max={max}
      step={step}
      value={value}
      aria-label={ariaLabel}
      aria-valuemin={min}
      aria-valuemax={max}
      aria-valuenow={value}
      onChange={(event) => onChange(clamp(Number(event.target.value)))}
    />
    <button type="button" className="wk-kd-38" aria-label={`增加${ariaLabel}`} disabled={value >= max} onClick={() => adjust(step)}>+</button>
  </div>;
}

export function UploadSwitch({ checked, ariaLabel, onChange }: { checked: boolean; ariaLabel: string; onChange: (checked: boolean) => void }) {
  return <button type="button" className={`wk-upload-switch${checked ? " is-checked" : ""} wk-kd-143 ${checked ? "wk-kd-144" : "wk-kd-145"} wk-kd-146`} role="switch" aria-checked={checked} aria-label={ariaLabel} onClick={() => onChange(!checked)}>
    <span className={`wk-upload-switch__handle wk-kd-147 ${checked ? "wk-kd-148" : "wk-kd-149"}`} aria-hidden="true" />
  </button>;
}

function UploadSettingRow({ label, description, children, className = "" }: { label: ReactNode; description?: string; children: ReactNode; className?: string }) {
  // CSS cascade: .wk-upload-setting-row--separators stretched its control.
  const separators = className.includes("wk-upload-setting-row--separators");
  return <div className={`wk-upload-setting-row wk-kd-settingrow ${className}`.trim()}>
    <div className="wk-upload-setting-info wk-kd-40"><label className="wk-kd-41">{label}</label>{description ? <p className="wk-muted wk-kd-42">{description}</p> : null}</div>
    <div className={`wk-upload-setting-control wk-kd-150 ${separators ? "wk-kd-151" : "wk-kd-152"}`}>{children}</div>
  </div>;
}

const CHUNKING_SEPARATOR_OPTIONS = [
  { value: "\n\n", labelKey: "knowledgeEditor.chunking.separators.doubleNewline" },
  { value: "\n", labelKey: "knowledgeEditor.chunking.separators.singleNewline" },
  { value: "。", labelKey: "knowledgeEditor.chunking.separators.periodCn" },
  { value: "！", labelKey: "knowledgeEditor.chunking.separators.exclamationCn" },
  { value: "？", labelKey: "knowledgeEditor.chunking.separators.questionCn" },
  { value: "；", labelKey: "knowledgeEditor.chunking.separators.semicolonCn" },
  { value: ";", labelKey: "knowledgeEditor.chunking.separators.semicolonEn" },
  { value: " ", labelKey: "knowledgeEditor.chunking.separators.space" },
] as const;

const CHUNKING_LANGUAGE_OPTIONS = [
  { value: "de", labelKey: "knowledgeEditor.chunking.languageOptions.de" },
  { value: "en", labelKey: "knowledgeEditor.chunking.languageOptions.en" },
  { value: "zh", labelKey: "knowledgeEditor.chunking.languageOptions.zh" },
] as const;

const MULTIMODAL_LANGUAGE_OPTIONS = [
  { value: "Chinese", labelKey: "language.zhCN" },
  { value: "English", labelKey: "language.enUS" },
  { value: "Korean", labelKey: "language.koKR" },
  { value: "Russian", labelKey: "language.ruRU" },
] as const;

/**
 * The dialog's config panel: parser engine rules with the scanned-PDF
 * override, chunking (with the collapsed "more options" group), multimodal,
 * ASR and question generation. Shared by upload, manual and reparse modes.
 */
export function UploadConfirmSections(props: UploadConfirmSectionsProps) {
  const { state, update, t } = props;
  const sectionStyle = (key: UploadConfirmSectionKey): React.CSSProperties | undefined => props.activeSection === undefined ? undefined : { display: props.activeSection === key ? undefined : "none" };
  const parserFileTypes = [...new Set(props.parserEngines.flatMap((engine) => engine.FileTypes ?? []))]
    .filter((fileType) => fileType !== "url")
    .sort();
  const parserFileGroups = (() => {
    const known: Array<[string[], string]> = [
      [["pdf"], t("kbSettings.parser.fileTypePdf")],
      [["docx", "doc"], t("kbSettings.parser.fileTypeWord")],
      [["pptx", "ppt"], t("kbSettings.parser.fileTypePpt")],
      [["xlsx", "xls"], t("kbSettings.parser.fileTypeExcel")],
      [["epub"], t("kbSettings.parser.fileTypeEbook")],
      [["mhtml"], t("kbSettings.parser.fileTypeWebArchive")],
      [["csv"], t("kbSettings.parser.fileTypeCsv")],
      [["md", "markdown"], "Markdown"],
      [["txt"], t("kbSettings.parser.fileTypeText")],
      [["json"], t("kbSettings.parser.fileTypeJson")],
      [["jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"], t("kbSettings.parser.fileTypeImage")],
      [["mp3", "wav", "m4a", "flac", "ogg"], t("kbSettings.parser.fileTypeAudiovisual")],
    ];
    const groups = known.flatMap(([extensions, label]) => {
      const present = extensions.filter((extension) => parserFileTypes.includes(extension));
      return present.length ? [{ key: label, label, extensions: present }] : [];
    });
    const grouped = new Set(groups.flatMap((group) => group.extensions));
    return [...groups, ...parserFileTypes.filter((extension) => !grouped.has(extension)).map((extension) => ({ key: extension, label: extension.toUpperCase(), extensions: [extension] }))];
  })();
  const parserEngineOptions = (extensions: string[]) => {
    const engines = props.parserEngines.filter((engine) => engine.Available !== false && extensions.some((extension) => (engine.FileTypes ?? []).includes(extension)));
    const simple = new Set(["md", "markdown", "txt", "csv", "json"]);
    const defaultName = !extensions.every((extension) => simple.has(extension)) ? engines.find((engine) => engine.Name === "anydoc")?.Name ?? engines[0]?.Name : engines[0]?.Name;
    return engines.map((engine) => ({ value: engine.Name, label: engine.Name === defaultName ? `${engine.Name} (${t("kbSettings.parser.default")})` : engine.Name }));
  };
  const parserRuleFor = (extensions: string[]) => state.parserRules.find((rule) => rule.file_types.some((fileType) => extensions.includes(fileType)));
  const parserEngineFor = (extensions: string[]) => parserRuleFor(extensions)?.engine ?? parserEngineOptions(extensions)[0]?.value ?? "";
  const updateParserXlsxHeader = (extensions: string[], checked: boolean) => update({ parserRules: (() => {
    const current = parserRuleFor(extensions);
    if (current) return state.parserRules.map((rule) => rule === current ? { ...rule, xlsx_first_row_as_header: checked } : rule);
    return [...state.parserRules, { file_types: extensions, engine: "builtin", xlsx_first_row_as_header: checked }];
  })() });
  return (
    <>
      <fieldset className="wk-upload-confirm-parser" id="wk-upload-section-parser" data-section="parser" style={sectionStyle("parser")}>
        <legend>{t("settings.parserEngine")}</legend>
        {props.hasPdf ? (
          <div className="setting-row" style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start", marginBottom: "0.5rem" }}>
            <div style={{ flex: 1 }}>
              <label htmlFor="wk-pdf-force-scanned">{t("uploadConfirm.pdfForceScanned.label")}</label>
              <p className="wk-muted wk-kd-16" style={{ margin: 0, fontSize: "0.85rem" }}>
                {t("uploadConfirm.pdfForceScanned.description")}
              </p>
            </div>
            <UploadSwitch checked={state.pdfForceScanned} ariaLabel={t("uploadConfirm.pdfForceScanned.label")} onChange={(checked) => update({ pdfForceScanned: checked })} />
          </div>
        ) : null}
        {props.parserLoading ? (
          <div className="wk-upload-parser-loading" role="status">{t("settings.parser.loading")}</div>
        ) : props.parserEngines.length === 0 ? (
          <p className="wk-muted wk-kd-16">{t("settings.parser.noEngineDetected")}</p>
        ) : (
          <div className="wk-upload-parser-group wk-kd-43">
          {parserFileGroups.map((group) => (
            <div className="wk-upload-parser-row wk-kd-44" key={group.key}>
              <div className="wk-upload-parser-info flex-[0_0_168px] max-[720px]:flex-[0_0_auto] wk-kd-45">
                <strong className="wk-kd-46">{group.label}</strong>
                <span className="wk-upload-parser-extensions wk-kd-47">{group.extensions.map((extension) => <span className="wk-kd-48" key={extension}>.{extension}</span>)}</span>
              </div>
              <div className="wk-upload-parser-control wk-kd-49">
                {(() => {
                  const options = parserEngineOptions(group.extensions);
                  return <>
                <UploadSingleSelect
                  className="wk-upload-parser-select"
                  ariaLabel={group.label}
                  value={parserEngineFor(group.extensions)}
                  options={options}
                  onChange={(value) => update({ parserRules: (() => {
                    const current = parserRuleFor(group.extensions);
                    const remaining = state.parserRules.filter((rule) => !rule.file_types.some((fileType) => group.extensions.includes(fileType)));
                    return value ? [...remaining, { file_types: group.extensions, engine: value, ...(current?.xlsx_first_row_as_header === undefined ? {} : { xlsx_first_row_as_header: current.xlsx_first_row_as_header }) }] : remaining;
                  })() })}
                />
                {options.length === 0 ? <div className="wk-upload-parser-warning wk-kd-50" role="note"><span>{t("settings.parser.noEngineDetected")}</span>{props.onConfigureParserSettings ? <button type="button" className="wk-kd-51" onClick={props.onConfigureParserSettings}>{t("settings.parserEngine")}</button> : null}</div> : null}
                {group.extensions.includes("xlsx") && parserEngineFor(group.extensions) === "builtin" ? (
                  <label className="wk-upload-parser-xlsx-header wk-kd-52">
                    <Checkbox checked={parserRuleFor(group.extensions)?.xlsx_first_row_as_header === true} onChange={(event) => updateParserXlsxHeader(group.extensions, event.target.checked)} />
                    {t("kbSettings.parser.xlsxFirstRowAsHeader")}
                  </label>
                ) : null}
                  </>;
                })()}
              </div>
            </div>
          ))}
          </div>
        )}
      </fieldset>
      <fieldset className="wk-upload-confirm-chunking" id="wk-upload-section-chunking" data-section="chunking" style={sectionStyle("chunking")}>
        <legend className="wk-visually-hidden wk-kd-53">{t("knowledgeEditor.chunking.title")}</legend>
        <div className="wk-upload-section-header wk-kd-54"><h2 className="wk-kd-55">{t("knowledgeEditor.chunking.title")}</h2><p className="wk-kd-56">{t("knowledgeEditor.chunking.description")}</p></div>
        <UploadSettingRow label={t("knowledgeEditor.chunking.strategyLabel")} description={t("knowledgeEditor.chunking.strategyDescription")}>
          <UploadSingleSelect
            className="wk-upload-chunk-strategy-select"
            ariaLabel={t("knowledgeEditor.chunking.strategyLabel")}
            value={state.chunkStrategy}
            options={CHUNKING_STRATEGY_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))}
            onChange={(value) => update({ chunkStrategy: value })}
          />
        </UploadSettingRow>
        <UploadSettingRow label={t("knowledgeEditor.chunking.sizeLabel")} description={t("knowledgeEditor.chunking.sizeDescription")}>
          <UploadNumberInput className="wk-upload-number-input--wide" min={100} max={4000} step={50} ariaLabel={t("knowledgeEditor.chunking.sizeLabel")} value={state.chunkSize} onChange={(value) => update({ chunkSize: value })} />
        </UploadSettingRow>
        <UploadSettingRow label={t("knowledgeEditor.chunking.overlapLabel")} description={t("knowledgeEditor.chunking.overlapDescription")}>
          <UploadNumberInput className="wk-upload-number-input--wide" min={0} max={500} step={20} ariaLabel={t("knowledgeEditor.chunking.overlapLabel")} value={state.chunkOverlap} onChange={(value) => update({ chunkOverlap: value })} />
        </UploadSettingRow>
        <button
          type="button"
          className="more-options-toggle wk-kd-ring wk-kd-57"
          aria-expanded={props.moreOpen}
          onClick={props.onToggleMore}
        >
          <ChevronDownIcon open={props.moreOpen} /> {t("uploadConfirm.moreOptions")}
        </button>
        {props.moreOpen ? (
          <div className="settings-group--more">
            <UploadSettingRow label={t("knowledgeEditor.chunking.separatorsLabel")} description={t("knowledgeEditor.chunking.separatorsDescription")} className="wk-upload-setting-row--separators">
              <UploadMultiSelect ariaLabel={t("knowledgeEditor.chunking.separatorsLabel")} values={state.separators} options={CHUNKING_SEPARATOR_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))} onChange={(values) => update({ separators: values })} />
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.chunking.tokenLimitLabel")} description={t("knowledgeEditor.chunking.tokenLimitDescription")}>
              <UploadNumberInput className="wk-upload-number-input--wide" min={0} max={8192} step={64} ariaLabel={t("knowledgeEditor.chunking.tokenLimitLabel")} value={state.tokenLimit} onChange={(value) => update({ tokenLimit: value })} />
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.chunking.languagesLabel")} description={t("knowledgeEditor.chunking.languagesDescription")}>
              <UploadMultiSelect ariaLabel={t("knowledgeEditor.chunking.languagesLabel")} values={state.languages} options={CHUNKING_LANGUAGE_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))} onChange={(values) => update({ languages: values })} />
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.chunking.parentChildLabel")} description={t("knowledgeEditor.chunking.parentChildDescription")}>
              <UploadSwitch checked={state.enableParentChild} ariaLabel={t("knowledgeEditor.chunking.parentChildLabel")} onChange={(checked) => update({ enableParentChild: checked })} />
            </UploadSettingRow>
            {state.enableParentChild ? (
              <>
                <UploadSettingRow label={t("knowledgeEditor.chunking.parentChunkSizeLabel")} description={t("knowledgeEditor.chunking.parentChunkSizeDescription")}>
                  <UploadNumberInput className="wk-upload-number-input--wide" min={512} max={8192} step={64} ariaLabel={t("knowledgeEditor.chunking.parentChunkSizeLabel")} value={state.parentChunkSize} onChange={(value) => update({ parentChunkSize: value })} />
                </UploadSettingRow>
                <UploadSettingRow label={t("knowledgeEditor.chunking.childChunkSizeLabel")} description={t("knowledgeEditor.chunking.childChunkSizeDescription")}>
                  <UploadNumberInput className="wk-upload-number-input--wide" min={64} max={2048} step={32} ariaLabel={t("knowledgeEditor.chunking.childChunkSizeLabel")} value={state.childChunkSize} onChange={(value) => update({ childChunkSize: value })} />
                </UploadSettingRow>
              </>
            ) : null}
          </div>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-multimodal" id="wk-upload-section-multimodal" data-section="multimodal" style={sectionStyle("multimodal")}>
        <legend className="wk-visually-hidden wk-kd-53">{t("knowledgeEditor.multimodal.title")}</legend>
        <div className="wk-upload-section-header wk-kd-54"><h2 className="wk-kd-55">{t("knowledgeEditor.multimodal.title")}</h2><p className="wk-kd-56">{t("knowledgeEditor.multimodal.description")}</p></div>
        {props.multimodalIssue ? (
          <p className="wk-muted wk-kd-16" role="note">{t("uploadConfirm.multimodalSetupHint")}</p>
        ) : null}
        <UploadSettingRow label={t("knowledgeEditor.advanced.multimodal.label")} description={t("knowledgeEditor.advanced.multimodal.description")}>
          <UploadSwitch checked={state.multimodalEnabled} ariaLabel={t("knowledgeEditor.advanced.multimodal.label")} onChange={(checked) => update({ multimodalEnabled: checked })} />
        </UploadSettingRow>
        {state.multimodalEnabled ? (
          <>
            <UploadSettingRow label={<>{t("knowledgeEditor.advanced.multimodal.vllmLabel")} <span aria-hidden>*</span></>} description={t("knowledgeEditor.advanced.multimodal.vllmDescription")}>
              {props.vllmModels.length > 0 ? (
                <UploadSingleSelect className="wk-upload-model-select" ariaLabel={t("knowledgeEditor.advanced.multimodal.vllmLabel")} placeholder={t("knowledgeEditor.advanced.multimodal.vllmPlaceholder")} value={state.vllmModelId} options={[{ value: "", label: t("knowledgeEditor.advanced.multimodal.vllmPlaceholder") }, ...props.vllmModels.map((model) => ({ value: model.id, label: model.name }))]} onChange={(value) => update({ vllmModelId: value })} />
              ) : (
                <Input
                  className="wk-kd-58"
                  required
                  value={state.vllmModelId}
                  onChange={(event) => update({ vllmModelId: event.target.value })}
                  placeholder={t("knowledgeEditor.advanced.multimodal.vllmPlaceholder")}
                />
              )}
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.advanced.multimodal.descriptionLanguageLabel")} description={t("knowledgeEditor.advanced.multimodal.descriptionLanguageDescription")}>
              <UploadSingleSelect className="wk-upload-model-select" ariaLabel={t("knowledgeEditor.advanced.multimodal.descriptionLanguageLabel")} placeholder={t("knowledgeEditor.advanced.multimodal.descriptionLanguageAuto")} clearable value={state.descriptionLanguage} options={MULTIMODAL_LANGUAGE_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) }))} onChange={(value) => update({ descriptionLanguage: value })} />
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.advanced.multimodal.customInstructionsLabel")} description={t("knowledgeEditor.advanced.multimodal.customInstructionsDescription")}>
              <Textarea
                rows={3}
                maxLength={4000}
                placeholder={t("knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder")}
                value={state.customInstructions}
                onChange={(event) => update({ customInstructions: event.target.value })}
              />
            </UploadSettingRow>
          </>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-asr" id="wk-upload-section-asr" data-section="asr" style={sectionStyle("asr")}>
        <legend className="wk-visually-hidden wk-kd-53">{t("knowledgeEditor.asr.title")}</legend>
        <div className="wk-upload-section-header wk-kd-54"><h2 className="wk-kd-55">{t("knowledgeEditor.asr.title")}</h2><p className="wk-kd-56">{t("knowledgeEditor.asr.description")}</p></div>
        {props.asrIssue ? (
          <p className="wk-muted wk-kd-16" role="note">{t("uploadConfirm.asrSetupHint")}</p>
        ) : null}
        <UploadSettingRow label={t("knowledgeEditor.asr.label")} description={t("knowledgeEditor.asr.description")}>
          <UploadSwitch checked={state.asrEnabled} ariaLabel={t("knowledgeEditor.asr.label")} onChange={(checked) => update({ asrEnabled: checked })} />
        </UploadSettingRow>
        {state.asrEnabled ? (
          <>
            <UploadSettingRow label={<>{t("knowledgeEditor.asr.modelLabel")} <span aria-hidden>*</span></>} description={t("knowledgeEditor.asr.modelDescription")}>
              {props.asrModels.length > 0 ? (
                <UploadSingleSelect className="wk-upload-model-select" ariaLabel={t("knowledgeEditor.asr.modelLabel")} placeholder={t("knowledgeEditor.asr.modelPlaceholder")} value={state.asrModelId} options={[{ value: "", label: t("knowledgeEditor.asr.modelPlaceholder") }, ...props.asrModels.map((model) => ({ value: model.id, label: model.name }))]} onChange={(value) => update({ asrModelId: value })} />
              ) : (
                <Input
                  className="wk-kd-58"
                  required
                  value={state.asrModelId}
                  onChange={(event) => update({ asrModelId: event.target.value })}
                  placeholder={t("knowledgeEditor.asr.modelPlaceholder")}
                />
              )}
            </UploadSettingRow>
            <UploadSettingRow label={t("knowledgeEditor.asr.languageLabel")} description={t("knowledgeEditor.asr.languageDescription")}>
              <UploadClearableInput
                value={state.asrLanguage}
                placeholder={t("knowledgeEditor.asr.languagePlaceholder")}
                ariaLabel={t("knowledgeEditor.asr.languageLabel")}
                onChange={(value) => update({ asrLanguage: value })}
              />
            </UploadSettingRow>
          </>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-question" id="wk-upload-section-question" data-section="question" style={sectionStyle("question")}>
        <legend className="wk-visually-hidden wk-kd-53">{t("knowledgeEditor.advanced.questionGeneration.label")}</legend>
        <div className="wk-upload-section-header wk-kd-54"><h2 className="wk-kd-55">{t("knowledgeEditor.advanced.questionGeneration.label")}</h2><p className="wk-kd-56">{t("knowledgeEditor.advanced.questionGeneration.description")}</p></div>
        {/* R478 React-first mitigation (shared Vue+React UX defect, superset — NOT a parity claim):
            the backend silently skips question generation for KBs with vector
            and keyword search both off (kb.NeedsEmbeddingModel gate). Inform
            only; the switch and payload stay untouched so backend behavior
            is unchanged. */}
        {state.questionGenerationSkipped ? (
          <p role="note" className="wk-question-skip-hint wk-kd-59">{t("uploadConfirm.questionGeneration.skippedHint")}</p>
        ) : null}
        <div className="wk-upload-question-row wk-kd-60">
          <div className="wk-upload-question-info wk-kd-61">
            <label id="wk-question-enabled-label" className="wk-kd-41">{t("knowledgeEditor.advanced.questionGeneration.label")}</label>
            <p className="wk-muted wk-kd-42">{t("knowledgeEditor.advanced.questionGeneration.countDescription")}</p>
          </div>
          <div className="wk-upload-question-control wk-kd-62">
            {state.questionEnabled ? <UploadNumberInput min={1} max={10} step={1} ariaLabel={t("knowledgeEditor.advanced.questionGeneration.countLabel")} value={state.questionCount} onChange={(value) => update({ questionCount: value })} /> : null}
            <GraphSwitch id="wk-question-enabled" checked={state.questionEnabled} labelId="wk-question-enabled-label" onChange={(checked) => update({ questionEnabled: checked })} />
          </div>
        </div>
        {state.questionEnabled ? (
          <div className="wk-upload-question-instructions wk-kd-60">
            <div className="wk-upload-question-info wk-kd-61">
              <label htmlFor="wk-question-instructions" className="wk-kd-41">{t("knowledgeEditor.advanced.questionGeneration.instructionsLabel")}</label>
              <p className="wk-muted wk-kd-42">{t("knowledgeEditor.advanced.questionGeneration.instructionsDescription")}</p>
            </div>
            <Textarea id="wk-question-instructions" className="wk-kd-63" rows={3} maxLength={4000} placeholder={t("knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder")} value={state.questionInstructions} onChange={(event) => update({ questionInstructions: event.target.value })} />
          </div>
        ) : null}
      </fieldset>
      {props.graphAvailable && props.graphSettings ? (
        <fieldset className="wk-upload-confirm-graph wk-kd-64" id="wk-upload-section-graph" data-section="graph" style={sectionStyle("graph")}>
          {props.graphSettings}
        </fieldset>
      ) : null}
    </>
  );
}

// --- Graph settings (Vue GraphSettings.vue upload-dialog parity, N007) --------

/** One admin extraction call (POST /initialization/extract/*, routes_infra.go:123-125). */
export type UploadGraphExtractAction = "fabri-tag" | "fabri-text" | "text-relation";

export interface UploadGraphExtractResult {
  tags?: string[];
  text?: string;
  nodes?: UploadGraphNodeState[];
  relations?: UploadGraphRelationState[];
}

export interface UploadGraphSettingsProps {
  graphExtract: UploadNodeExtractState;
  /** Vue isGraphDatabaseEnabled for the embedded disabled alert. */
  graphDatabaseOn: boolean;
  /** Vue modelId prop — the KB summary model gates the LLM-backed actions. */
  llmModelId: string;
  /** Vue canRunGraphExtract: tenant role at least admin. */
  canRunExtract: boolean;
  /** Extraction endpoint caller; undefined keeps the admin-gated buttons inert. */
  runExtractAction?: (action: UploadGraphExtractAction, body: Record<string, unknown>) => Promise<UploadGraphExtractResult | null>;
  /** Success/error feedback line (Vue MessagePlugin toasts). */
  onNotify?: (message: string, tone: "neutral" | "success" | "warning" | "error") => void;
  onChange: (config: UploadNodeExtractState) => void;
  t: UploadDialogT;
}

/** Vue t-textarea autosize: clamp measured content to the configured row range. */
export function clampGraphTextareaHeight(scrollHeight: number, lineHeight: number, verticalPadding: number, minRows: number, maxRows: number) {
  const minHeight = lineHeight * minRows + verticalPadding;
  const maxHeight = lineHeight * maxRows + verticalPadding;
  return Math.min(maxHeight, Math.max(minHeight, scrollHeight));
}

export function moveGraphRelationOption(index: number, direction: "up" | "down", optionCount: number) {
  if (optionCount <= 0) return 0;
  return direction === "down"
    ? Math.min(index + 1, optionCount - 1)
    : Math.max(index - 1, 0);
}

export function GraphTagsField({ tags, onChange, placeholder, ariaLabel }: {
  tags: readonly string[];
  onChange: (tags: string[]) => void;
  placeholder: string;
  ariaLabel: string;
}) {
  const [draft, setDraft] = useState("");
  const remove = (tag: string) => onChange(tags.filter((item) => item !== tag));
  const addDraft = () => {
    const tag = draft.trim();
    if (!tag || tags.includes(tag)) { setDraft(""); return; }
    onChange([...tags, tag]);
    setDraft("");
  };
  return (
    <div className="wk-graph-tags-field wk-kd-65" role="listbox" aria-label={ariaLabel} aria-multiselectable="true">
      {tags.map((tag) => (
        <span key={tag} className="wk-graph-tag wk-kd-66" role="option" aria-selected="true">
          <span>{tag}</span>
          <button type="button" className="wk-kd-67" aria-label={`移除 ${tag}`} onClick={() => remove(tag)}>×</button>
        </span>
      ))}
      <Input
        type="text"
        className="wk-kd-68"
        role="combobox"
        aria-autocomplete="list"
        value={draft}
        placeholder={tags.length === 0 ? placeholder : ""}
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === ",") { event.preventDefault(); addDraft(); }
          else if (event.key === "Backspace" && !draft && tags.length > 0) remove(tags[tags.length - 1]);
        }}
      />
      {tags.length > 0 ? <button type="button" className="wk-graph-tags-clear wk-kd-67" aria-label="清除关系类型" onClick={() => onChange([])}>×</button> : null}
    </div>
  );
}

export function GraphRelationSelect({ value, options, placeholder, ariaLabel, creatable = false, clearable = false, onChange }: {
  value: string;
  options: readonly string[];
  placeholder: string;
  ariaLabel: string;
  creatable?: boolean;
  clearable?: boolean;
  onChange: (value: string) => void;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const normalized = filter.trim().toLocaleLowerCase();
  const filtered = normalized ? options.filter((option) => option.toLocaleLowerCase().includes(normalized)) : [...options];
  const canCreate = creatable && Boolean(filter.trim()) && !options.some((option) => option.toLocaleLowerCase() === normalized);
  const optionCount = filtered.length + (canCreate ? 1 : 0);
  useEffect(() => {
    if (!open) return;
    const closeOnOutside = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", closeOnOutside);
    return () => document.removeEventListener("mousedown", closeOnOutside);
  }, [open]);
  const choose = (next: string) => { onChange(next); setFilter(""); setActiveIndex(0); setOpen(false); };
  return (
    <div ref={rootRef} className="wk-graph-relation-select wk-kd-69">
      <Input
        type="text"
        className="wk-kd-58"
        role="combobox"
        aria-label={ariaLabel}
        aria-autocomplete="list"
        aria-expanded={open}
        value={open ? filter : value}
        placeholder={open ? placeholder : (value || placeholder)}
        onFocus={() => { setOpen(true); setFilter(""); setActiveIndex(0); }}
        onChange={(event) => { setFilter(event.target.value); setActiveIndex(0); setOpen(true); }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown") { event.preventDefault(); setOpen(true); setActiveIndex((index) => moveGraphRelationOption(index, "down", optionCount)); }
          if (event.key === "ArrowUp") { event.preventDefault(); setActiveIndex((index) => moveGraphRelationOption(index, "up", optionCount)); }
          if (event.key === "Enter" && optionCount > 0) { event.preventDefault(); choose(activeIndex < filtered.length ? filtered[activeIndex] : filter.trim()); }
          if (event.key === "Escape") { setOpen(false); event.currentTarget.blur(); }
        }}
      />
      {clearable && value ? <button type="button" className="wk-graph-relation-clear wk-kd-70" aria-label={`清除${ariaLabel}`} onMouseDown={(event) => { event.preventDefault(); choose(""); }}>×</button> : null}
      {open ? (
        <div role="listbox" className="wk-graph-relation-options inset-x-0 wk-kd-71">
          {filtered.map((option, index) => <button type="button" role="option" aria-selected={option === value} className={`wk-kd-153 ${index === activeIndex || option === value ? "is-active wk-kd-154" : ""}`} key={option} onMouseDown={(event) => { event.preventDefault(); choose(option); }}>{option}</button>)}
          {canCreate ? <button type="button" role="option" className={`wk-kd-153 ${activeIndex === filtered.length ? "is-active wk-kd-154" : ""}`} onMouseDown={(event) => { event.preventDefault(); choose(filter.trim()); }}>创建“{filter.trim()}”</button> : null}
          {filtered.length === 0 && !canCreate ? <span className="wk-muted wk-kd-16">{placeholder}</span> : null}
        </div>
      ) : null}
    </div>
  );
}

export function GraphSwitch({ id, checked, onChange, labelId }: {
  id: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  labelId: string;
}) {
  return (
    <button
      id={id}
      type="button"
      className={`wk-graph-switch${checked ? " is-checked" : ""} wk-kd-155 ${checked ? "wk-kd-144" : "wk-kd-156"} wk-kd-157`}
      role="switch"
      aria-checked={checked}
      aria-labelledby={labelId}
      onClick={() => onChange(!checked)}
    >
      <span className={`wk-graph-switch-handle wk-kd-158 ${checked ? "wk-kd-148" : "wk-kd-149"}`} aria-hidden="true" />
    </button>
  );
}

/**
 * Vue GraphSettings.vue ported for the upload-confirm dialog: enable switch
 * (turning it off clears the sample data but keeps custom instructions),
 * custom instructions, creatable relation-type tags, sample text, entity rows
 * with attributes, relation rows, add-entity/relation rows, the extraction
 * actions (admin-only LLM calls) and the default/clear example fixtures.
 */
export function UploadGraphSettings(props: UploadGraphSettingsProps) {
  const { t, graphExtract } = props;
  const llmAvailable = !!props.llmModelId;
  const emit = (next: UploadNodeExtractState) => props.onChange(next);
  const [tagFabring, setTagFabring] = useState(false);
  const [textFabring, setTextFabring] = useState(false);
  const instructionsRef = useRef<HTMLTextAreaElement | null>(null);
  const sampleTextRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    const fields = [
      { node: instructionsRef.current, value: graphExtract.customInstructions, minRows: 3, maxRows: 8 },
      { node: sampleTextRef.current, value: graphExtract.text, minRows: 6, maxRows: 12 },
    ];
    fields.forEach(({ node, minRows, maxRows }) => {
      if (!node) return;
      node.style.height = "auto";
      const computed = window.getComputedStyle(node);
      const lineHeight = Number.parseFloat(computed.lineHeight) || 22;
      const verticalPadding = (Number.parseFloat(computed.paddingTop) || 0) + (Number.parseFloat(computed.paddingBottom) || 0);
      const measured = node.scrollHeight || lineHeight * minRows + verticalPadding;
      node.style.height = `${clampGraphTextareaHeight(measured, lineHeight, verticalPadding, minRows, maxRows)}px`;
      node.style.overflowY = measured > lineHeight * maxRows + verticalPadding ? "auto" : "hidden";
    });
  }, [graphExtract.customInstructions, graphExtract.text]);

  function patch(partial: Partial<UploadNodeExtractState>) {
    emit({ ...graphExtract, ...partial });
  }

  // Vue handleEnabledChange: off clears text/tags/nodes/relations, keeps customInstructions.
  function handleEnabledChange(enabled: boolean) {
    if (!enabled) {
      emit({ ...graphExtract, enabled: false, text: "", tags: [], nodes: [], relations: [] });
      return;
    }
    emit({ ...graphExtract, enabled: true });
  }

  function updateNode(nodeIndex: number, next: UploadGraphNodeState) {
    patch({ nodes: graphExtract.nodes.map((node, index) => (index === nodeIndex ? next : node)) });
  }

  function removeNode(nodeIndex: number) {
    patch({ nodes: graphExtract.nodes.filter((_, index) => index !== nodeIndex) });
  }

  function updateRelation(relationIndex: number, next: UploadGraphRelationState) {
    patch({ relations: graphExtract.relations.map((relation, index) => (index === relationIndex ? next : relation)) });
  }

  function removeRelation(relationIndex: number) {
    patch({ relations: graphExtract.relations.filter((_, index) => index !== relationIndex) });
  }

  // Vue defaultExtractExample / clearExtractExample.
  function loadDefaultExample() {
    emit({
      ...graphExtract,
      text: GRAPH_EXTRACT_DEFAULT_EXAMPLE.text,
      tags: [...GRAPH_EXTRACT_DEFAULT_EXAMPLE.tags],
      nodes: GRAPH_EXTRACT_DEFAULT_EXAMPLE.nodes.map((node) => ({ ...node, attributes: [...node.attributes] })),
      relations: GRAPH_EXTRACT_DEFAULT_EXAMPLE.relations.map((relation) => ({ ...relation })),
    });
    props.onNotify?.(t("graphSettings.exampleLoaded"), "success");
  }

  function clearExample() {
    emit({ ...graphExtract, text: "", tags: [], nodes: [], relations: [] });
    props.onNotify?.(t("graphSettings.exampleCleared"), "success");
  }

  async function runAction(action: UploadGraphExtractAction, body: Record<string, unknown>, applyResult: (result: UploadGraphExtractResult) => UploadNodeExtractState, successMessage: string, failedMessage: string, setBusy?: (busy: boolean) => void) {
    if (!props.runExtractAction) return;
    setBusy?.(true);
    try {
      const result = await props.runExtractAction(action, body);
      if (result) {
        emit(applyResult(result));
        props.onNotify?.(successMessage, "success");
      } else {
        props.onNotify?.(failedMessage, "error");
      }
    } catch {
      props.onNotify?.(failedMessage, "error");
    } finally {
      setBusy?.(false);
    }
  }

  const [extracting, setExtracting] = useState(false);

  return (
    <div className="wk-graph-settings wk-kd-72">
      <div className="section-header wk-kd-54">
        <h2 className="wk-kd-55">{t("graphSettings.title")}</h2>
        <p className="section-desc wk-kd-56">{t("graphSettings.description")}</p>
      </div>
      {!props.graphDatabaseOn ? (
        <p className="wk-graph-alert wk-kd-73" role="alert">{t("graphSettings.disabledWarning")}</p>
      ) : null}
      <div className="settings-group">
        <div className="wk-graph-setting-row wk-kd-74">
          <div className="setting-info flex-[0_0_40%] wk-kd-75">
            <label id="wk-graph-enabled-label" className="wk-kd-76">{t("graphSettings.enableLabel")}</label>
            <p className="wk-muted wk-kd-77">{t("graphSettings.enableDescription")}</p>
          </div>
          <div className="setting-control flex-[0_0_55%] wk-kd-78">
            <GraphSwitch
              id="wk-graph-enabled"
              checked={graphExtract.enabled}
              onChange={handleEnabledChange}
              labelId="wk-graph-enabled-label"
            />
          </div>
        </div>

        {graphExtract.enabled ? (
          <>
            <div className="wk-graph-setting-row is-vertical wk-kd-79">
              <div className="setting-info">
                <label htmlFor="wk-graph-instructions" className="wk-kd-76">{t("graphSettings.customInstructionsLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.customInstructionsDescription")}</p>
              </div>
              <div className="setting-control is-full flex-[0_0_55%] wk-kd-80">
                <Textarea
                  ref={instructionsRef}
                  id="wk-graph-instructions"
                  rows={3}
                  maxLength={4000}
                  placeholder={t("graphSettings.customInstructionsPlaceholder")}
                  value={graphExtract.customInstructions}
                  onChange={(event) => patch({ customInstructions: event.target.value })}
                />
              </div>
            </div>
            <div className="wk-graph-setting-row is-vertical wk-kd-79">
              <div className="setting-info">
                <label htmlFor="wk-graph-tags" className="wk-kd-76">{t("graphSettings.tagsLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.tagsDescription")}</p>
              </div>
              <div className="setting-control is-full flex-[0_0_55%] wk-kd-80">
                <div className="wk-graph-tags-group wk-kd-81">
                  {props.canRunExtract ? (
                    <button
                      type="button"
                      className="wk-graph-gen-btn"
                      disabled={!llmAvailable || tagFabring}
                      onClick={() => {
                        void runAction(
                          "fabri-tag",
                          {},
                          (result) => ({ ...graphExtract, tags: result.tags ?? [] }),
                          t("graphSettings.tagsGenerated"),
                          t("graphSettings.tagsGenerateFailed"),
                          setTagFabring,
                        );
                      }}
                    >
                      {t("graphSettings.generateRandomTags")}
                    </button>
                  ) : null}
                  <GraphTagsField
                    tags={graphExtract.tags}
                    onChange={(tags) => patch({ tags })}
                    placeholder={t("graphSettings.tagsPlaceholder")}
                    ariaLabel={t("graphSettings.tagsLabel")}
                  />
                </div>
                <div className="wk-graph-add-tag wk-kd-72">
                  <Input
                    className="wk-kd-58"
                    type="text"
                    placeholder={t("graphSettings.tagsPlaceholder")}
                    aria-label={t("graphSettings.tagsPlaceholder")}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter") return;
                      event.preventDefault();
                      const value = event.currentTarget.value.trim();
                      if (value && !graphExtract.tags.includes(value)) patch({ tags: [...graphExtract.tags, value] });
                      event.currentTarget.value = "";
                    }}
                  />
                </div>
                {!llmAvailable ? (
                  <p className="wk-graph-tip wk-kd-82">{t("graphSettings.completeModelConfig")}</p>
                ) : null}
              </div>
            </div>
            <div className="wk-graph-setting-row is-vertical wk-kd-79">
              <div className="setting-info">
                <label htmlFor="wk-graph-text" className="wk-kd-76">{t("graphSettings.sampleTextLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.sampleTextDescription")}</p>
              </div>
              <div className="setting-control is-full">
                <div className="wk-graph-text-group wk-kd-83">
                  {props.canRunExtract ? (
                    <button
                      type="button"
                      className="wk-graph-gen-btn"
                      disabled={!llmAvailable || textFabring}
                      onClick={() => {
                        void runAction(
                          "fabri-text",
                          { tags: graphExtract.tags, model_id: props.llmModelId },
                          (result) => ({ ...graphExtract, text: result.text ?? "" }),
                          t("graphSettings.textGenerated"),
                          t("graphSettings.textGenerateFailed"),
                          setTextFabring,
                        );
                      }}
                    >
                      {t("graphSettings.generateRandomText")}
                    </button>
                  ) : null}
                  <Textarea
                    ref={sampleTextRef}
                    id="wk-graph-text"
                    rows={6}
                    maxLength={5000}
                    placeholder={t("graphSettings.sampleTextPlaceholder")}
                    value={graphExtract.text}
                    onChange={(event) => patch({ text: event.target.value })}
                    style={{ width: "100%" }}
                  />
                  <span className="wk-graph-text-limit -mt-1 wk-kd-84" aria-live="polite">{graphExtract.text.length}/5000</span>
                </div>
                {!llmAvailable ? (
                  <p className="wk-graph-tip wk-kd-82">{t("graphSettings.completeModelConfig")}</p>
                ) : null}
              </div>
            </div>
            {graphExtract.nodes.length > 0 ? (
              <div className="wk-graph-setting-row is-vertical wk-kd-79">
                <div className="setting-info">
                  <label className="wk-kd-76">{t("graphSettings.entityListLabel")}</label>
                  <p className="wk-muted wk-kd-77">{t("graphSettings.entityListDescription")}</p>
                </div>
                <div className="setting-control is-full flex-[0_0_55%] wk-kd-80">
                  <div className="wk-graph-node-list wk-kd-85">
                    {graphExtract.nodes.map((node, nodeIndex) => (
                      <div key={nodeIndex} className="wk-graph-node-item wk-kd-86" data-graph-node={node.name || undefined}>
                        <div className="wk-graph-node-header wk-kd-87">
                          <span aria-hidden>👤</span>
                          <Input
                            type="text"
                            className="wk-graph-node-name wk-kd-88"
                            placeholder={t("graphSettings.nodeNamePlaceholder")}
                            aria-label={t("graphSettings.nodeNamePlaceholder")}
                            value={node.name}
                            onChange={(event) => updateNode(nodeIndex, { ...node, name: event.target.value })}
                          />
                          <button
                            type="button"
                            aria-label={t("common.remove")}
                            onClick={() => removeNode(nodeIndex)}
                          >
                            ✕
                          </button>
                        </div>
                        <div className="wk-graph-node-attributes wk-kd-89">
                          {node.attributes.map((attribute, attrIndex) => (
                            <div key={attrIndex} className="wk-graph-attribute-item wk-kd-90">
                              <Input
                                type="text"
                                className="wk-kd-88"
                                placeholder={t("graphSettings.attributePlaceholder")}
                                aria-label={t("graphSettings.attributePlaceholder")}
                                value={attribute}
                                onChange={(event) =>
                                  updateNode(nodeIndex, {
                                    ...node,
                                    attributes: node.attributes.map((value, index) => (index === attrIndex ? event.target.value : value)),
                                  })
                                }
                              />
                              <button
                                type="button"
                                aria-label={t("common.remove")}
                                onClick={() =>
                                  updateNode(nodeIndex, {
                                    ...node,
                                    attributes: node.attributes.filter((_, index) => index !== attrIndex),
                                  })
                                }
                              >
                                ✕
                              </button>
                            </div>
                          ))}
                          <button
                            type="button"
                            className="wk-graph-add-attr wk-kd-91"
                            onClick={() => updateNode(nodeIndex, { ...node, attributes: [...node.attributes, ""] })}
                          >
                            {t("graphSettings.addAttribute")}
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ) : null}

            <div className="wk-graph-setting-row wk-kd-74">
              <div className="setting-info flex-[0_0_40%] wk-kd-75">
                <label className="wk-kd-76">{t("graphSettings.manageEntitiesLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.manageEntitiesDescription")}</p>
              </div>
              <div className="setting-control flex-[0_0_55%] wk-kd-78">
                <button
                  type="button"
                  className="wk-graph-add-btn wk-kd-92"
                  onClick={() => patch({ nodes: [...graphExtract.nodes, { name: "", attributes: [] }] })}
                >
                  {t("graphSettings.addEntity")}
                </button>
              </div>
            </div>
            {graphExtract.relations.length > 0 ? (
              <div className="wk-graph-setting-row is-vertical wk-kd-79">
                <div className="setting-info">
                  <label className="wk-kd-76">{t("graphSettings.relationListLabel")}</label>
                  <p className="wk-muted wk-kd-77">{t("graphSettings.relationListDescription")}</p>
                </div>
                <div className="setting-control is-full flex-[0_0_55%] wk-kd-80">
                  <div className="wk-graph-relation-list wk-kd-93">
                    {graphExtract.relations.map((relation, index) => (
                      <div key={index} className="wk-graph-relation-item wk-kd-94">
                        <GraphRelationSelect value={relation.node1} options={graphExtract.nodes.map((node) => node.name)} placeholder={t("graphSettings.selectEntity")} ariaLabel={t("graphSettings.selectEntity")} onChange={(value) => updateRelation(index, { ...relation, node1: value })} />
                        <span aria-hidden className="wk-kd-95">→</span>
                        <GraphRelationSelect value={relation.type} options={graphExtract.tags} placeholder={t("graphSettings.selectRelationType")} ariaLabel={t("graphSettings.selectRelationType")} creatable clearable onChange={(value) => updateRelation(index, { ...relation, type: value })} />
                        <span aria-hidden className="wk-kd-95">→</span>
                        <GraphRelationSelect value={relation.node2} options={graphExtract.nodes.map((node) => node.name)} placeholder={t("graphSettings.selectEntity")} ariaLabel={t("graphSettings.selectEntity")} onChange={(value) => updateRelation(index, { ...relation, node2: value })} />
                        <button
                          type="button"
                          aria-label={t("common.remove")}
                          onClick={() => removeRelation(index)}
                        >
                          ✕
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ) : null}
            <div className="wk-graph-setting-row wk-kd-74">
              <div className="setting-info flex-[0_0_40%] wk-kd-75">
                <label className="wk-kd-76">{t("graphSettings.manageRelationsLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.manageRelationsDescription")}</p>
              </div>
              <div className="setting-control flex-[0_0_55%] wk-kd-78">
                <button
                  type="button"
                  className="wk-graph-add-btn wk-kd-92"
                  onClick={() => patch({ relations: [...graphExtract.relations, { node1: "", node2: "", type: "" }] })}
                >
                  {t("graphSettings.addRelation")}
                </button>
              </div>
            </div>
            <div className="wk-graph-setting-row wk-kd-74">
              <div className="setting-info flex-[0_0_40%] wk-kd-75">
                <label className="wk-kd-76">{t("graphSettings.extractActionsLabel")}</label>
                <p className="wk-muted wk-kd-77">{t("graphSettings.extractActionsDescription")}</p>
              </div>
              <div className="setting-control flex-[0_0_55%] wk-kd-78">
                <div className="wk-graph-actions wk-kd-96">
                  {props.canRunExtract ? (
                    <button
                      type="button"
                      className="wk-graph-add-btn wk-kd-92"
                      disabled={!llmAvailable || !graphExtract.text || extracting}
                      onClick={() => {
                        setExtracting(true);
                        void runAction(
                          "text-relation",
                          { text: graphExtract.text, tags: graphExtract.tags, model_id: props.llmModelId },
                          (result) => ({
                            ...graphExtract,
                            nodes: result.nodes ?? [],
                            relations: result.relations ?? [],
                          }),
                          t("graphSettings.extractSuccess"),
                          t("graphSettings.extractFailed"),
                        ).finally(() => setExtracting(false));
                      }}
                    >
                      {extracting ? t("graphSettings.extracting") : t("graphSettings.startExtraction")}
                    </button>
                  ) : null}
                  <button type="button" className="wk-kd-97" onClick={loadDefaultExample}>{t("graphSettings.defaultExample")}</button>
                  <button type="button" className="wk-kd-97" onClick={clearExample}>{t("graphSettings.clearExample")}</button>
                </div>
              </div>
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

/**
 * Upload mask shown over the dropzone while a batch uploads: percent text plus
 * a thin bar. Vue parity: the global upload mask (upload-mask.vue, shown via
 * platform/index.vue ismask) carries the upload title, and the per-KB upload
 * panel (KnowledgeBaseList.vue:67-68) renders the percent as a fill-width bar;
 * the port puts that percent directly on the mask.
 */
export function UploadProgressMask({ percent, title = "Uploading", formats = ["pdf、doc", "text、markdown"] }: { percent: number; title?: string; formats?: readonly [string, string] }) {
  const clamped = clampUploadPercent(percent);
  return (
    <div className="wk-upload-mask wk-kd-98" role="status" aria-live="polite">
      <div className="wk-upload-mask__card wk-kd-99">
        <img className="wk-upload-mask__illustration wk-kd-100" src={uploadMaskIllustration} alt="" />
        <span className="wk-upload-mask__label wk-kd-101">{`${title} ${clamped}%`}</span>
        <span className="wk-upload-mask__type wk-kd-102">{formats[0]}</span>
        <span className="wk-upload-mask__type wk-kd-102">{formats[1]}</span>
        <div
          className="wk-upload-mask__bar wk-kd-103"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={clamped}
        >
          <div className="wk-upload-mask__fill wk-kd-104" style={{ width: `${clamped}%` }} />
        </div>
      </div>
    </div>
  );
}

// --- Page ---------------------------------------------------------------------

export function stageNoticeClass(tone: "neutral" | "success" | "warning" | "error") {
  return `wk-documents-toast ${tone}`;
}

/** documents.css toast tone palette, now inlined as utilities (class stays a hook). */
const STAGE_NOTICE_TONE_CLASS: Record<"neutral" | "success" | "warning" | "error", string> = {
  neutral: "wk-kd-notice-neutral",
  success: "wk-kd-notice-success",
  warning: "wk-kd-notice-warning",
  error: "wk-kd-notice-error",
};

export function KnowledgeDocumentsPage({
  client,
  knowledgeBaseId,
  onOpenDocument,
  initialDocumentId,
}: KnowledgeDocumentsPageProps) {
  const locale = useAppLocale() as Locale;
  const t = createTranslator(locale);
  // Dialog copy: shared i18n keys first, then the Vue-ported uploadConfirm table.
  const ct: UploadDialogT = uploadConfirmT(locale);
  // Tag surfaces: shared i18n first, byte-exact Vue fallback for the keys
  // packages/i18n still misses (common.confirm/common.clear/tenant.loadMore).
  const tt = tagSurfaceT(locale);
  const [reloadToken, setReloadToken] = useState(0);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [confirmingDeleteDocument, setConfirmingDeleteDocument] = useState<KnowledgeDocument | null>(null);
  const [state, setState] = useState<KnowledgeDocumentListState>({
    status: "loading",
  });
  const [folderState, setFolderState] = useState<{
    status: "loading" | "success" | "error";
    tree?: Awaited<ReturnType<typeof client.knowledgeBases.documents.folders>>;
    message?: string;
  }>({ status: "loading" });
  const [tags, setTags] = useState<
    Awaited<ReturnType<typeof client.knowledgeBases.documents.tags>>
  >([]);
  const [tagSearchQuery, setTagSearchQuery] = useState('');
  const [tagSearchDebounced, setTagSearchDebounced] = useState('');
  const [tagPage, setTagPage] = useState(1);
  const [tagTotal, setTagTotal] = useState(0);
  const [tagHasMore, setTagHasMore] = useState(false);
  const [tagLoadingMore, setTagLoadingMore] = useState(false);
  const [kbMeta, setKbMeta] = useState<KBSurfaceKB | null>(null);
  const [kbMetaError, setKbMetaError] = useState<KnowledgeBaseMetadataError | null>(null);
  const [kbMetaAttempt, setKbMetaAttempt] = useState(0);
  const [kbList, setKbList] = useState<KBChromeListItem[]>([]);
  // Vue keeps upload/mutation controls behind the resolved KB capability;
  // while the KB/auth requests are pending, render the viewer-safe state.
  const [canContribute, setCanContribute] = useState(false);
  const [canDownload, setCanDownload] = useState(false);
  const [canMutate, setCanMutate] = useState(false);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState(query);
  const [parseStatus, setParseStatus] = useState("");
  // Vue selectedTagIds/tagFilterCleared/tagFilterPanelVisible (KnowledgeBase.vue L534-547):
  // multi-select tag filter with a trigger label + popup chip panel.
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);
  const [tagFilterCleared, setTagFilterCleared] = useState(false);
  const [tagFilterOpen, setTagFilterOpen] = useState(false);
  // R490 B2 — Vue tagManageDrawerVisible: the 管理标签… drawer behind the tag
  // filter panel footer (KbTagManageDrawer.vue).
  const [tagManageOpen, setTagManageOpen] = useState(false);
  // Vue tagEditDialog/batchTagDialogVisible (L444, L734): per-document and batch tagging.
  const [tagDialog, setTagDialog] = useState<
    | { mode: "batch" }
    | { mode: "single"; document: KnowledgeDocument }
    | null
  >(null);
  const [tagDialogSaving, setTagDialogSaving] = useState(false);
  // Success feedback channel next to mutationError (Vue MessagePlugin.success).
  const [actionNotice, setActionNotice] = useState<{ tone: "success" | "neutral"; text: string } | null>(null);
  // Vue selectedFileType / selectedSource / updatedTimeRange filter refs.
  const [fileType, setFileType] = useState("");
  const [source, setSource] = useState("");
  const [updatedFrom, setUpdatedFrom] = useState("");
  const [updatedTo, setUpdatedTo] = useState("");
  const [folderPath, setFolderPath] = useState<string | undefined>(undefined);
  const [viewMode, setViewMode] = useState<DocumentViewMode>(() => {
    try { return window.localStorage.getItem("weknora.kb.docs.viewMode") === "list" ? "list" : "grid"; } catch { return "grid"; }
  });
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const openedInitialDocument = useRef(false);
  // Vue exposes document checkboxes and the batch bar only after the user
  // explicitly enters batch-management mode from a card/row action menu.
  const [batchMode, setBatchMode] = useState(false);
  const lastSelectedIndex = useRef(-1);
  const documentListRef = useRef<HTMLDivElement | null>(null);
  const [moving, setMoving] = useState(false);
  const [moveTarget, setMoveTarget] = useState("");
  // Vue page-level KbUploadSourceDropdown (doc-filter-actions) + its menu state.
  const [pageSourceMenuOpen, setPageSourceMenuOpen] = useState(false);
  // Vue include-manual entry (handleManualCreate) — a dialog feeding the
  // existing pendingManual staging instead of uiStore.openManualEditor.
  const [manualDialogOpen, setManualDialogOpen] = useState(false);
  const [manualEditDocument, setManualEditDocument] = useState<KnowledgeDocument | null>(null);
  const [manualEditLoading, setManualEditLoading] = useState(false);
  const [manualEditSaving, setManualEditSaving] = useState(false);
  const [traceDocument, setTraceDocument] = useState<KnowledgeDocument | null>(null);
  // Vue parity: the DocContent drawer opens IN PLACE over the still-mounted
  // KB page (KnowledgeBase.vue isCardDetails). Never navigate — the route
  // page would unmount the list behind the drawer.
  const [inlineDetailId, setInlineDetailId] = useState<string | null>(null);
  const openDocumentDetail = (document: KnowledgeDocument) => setInlineDetailId(document.id);
  // Vue traceAvailableById (KnowledgeBase.vue L356-397): opening a row menu
  // probes GET /spans once per document; in-flight parses count as traceable
  // without a request. Cache clears on every list reload.
  const traceAvailableRef = useRef<Record<string, boolean>>({});
  const traceProbeInflight = useRef<Set<string>>(new Set());
  const [traceAvailableById, setTraceAvailableById] = useState<Record<string, boolean>>({});
  useEffect(() => {
    traceAvailableRef.current = {};
    traceProbeInflight.current.clear();
    setTraceAvailableById({});
  }, [reloadToken, knowledgeBaseId]);
  function probeTraceAvailability(document: KnowledgeDocument) {
    const id = document.id;
    if (!id || traceProbeInflight.current.has(id)) return;
    if (isKnowledgeProcessingActive(document.parse_status)) {
      if (traceAvailableRef.current[id] !== true) {
        traceAvailableRef.current = { ...traceAvailableRef.current, [id]: true };
        setTraceAvailableById(traceAvailableRef.current);
      }
      return;
    }
    if (Object.prototype.hasOwnProperty.call(traceAvailableRef.current, id)) return;
    traceProbeInflight.current.add(id);
    void client.knowledgeBases.documents.spans(id)
      .then((response) => {
        traceAvailableRef.current = { ...traceAvailableRef.current, [id]: knowledgeSpansViewHasTrace(resolveKnowledgeSpansView(response)) };
        setTraceAvailableById(traceAvailableRef.current);
      })
      .catch(() => {
        traceAvailableRef.current = { ...traceAvailableRef.current, [id]: false };
        setTraceAvailableById(traceAvailableRef.current);
      })
      .finally(() => { traceProbeInflight.current.delete(id); });
  }
  /** Vue hover popover KnowledgeProcessingTimeline compact fetch (autoPoll=false). */
  const loadHoverTrace = useCallback(async (id: string): Promise<TraceSummary | null> => {
    try {
      return buildTraceSummary(resolveKnowledgeSpansView(await client.knowledgeBases.documents.spans(id)));
    } catch {
      return null;
    }
  }, [client]);
  // Cross-KB move sub-flow (Vue handleMoveKnowledge / handleMoveSelectTarget /
  // handleMoveConfirm / startMovePoll, KnowledgeBase.vue L1517-1600). Only the
  // row that opened the flow renders the picker; real moves are confirmed
  // here, the async task polls its progress endpoint until it settles.
  const [moveKb, setMoveKb] = useState<{
    documentId: string;
    view: 'targets' | 'confirm';
    targets: MoveTargetKb[];
    loading: boolean;
    selectedTargetId: string;
    selectedTargetName: string;
    mode: KnowledgeMoveMode;
    submitting: boolean;
  } | null>(null);
  const movePollTimer = useRef<number | null>(null);
  useEffect(() => () => { if (movePollTimer.current !== null) window.clearInterval(movePollTimer.current); }, []);
  function stopMovePoll() {
    if (movePollTimer.current !== null) {
      window.clearInterval(movePollTimer.current);
      movePollTimer.current = null;
    }
  }
  function startMoveToKb(document: KnowledgeDocument) {
    const documentId = document.id;
    setMoveKb({ documentId, view: 'targets', targets: [], loading: true, selectedTargetId: '', selectedTargetName: '', mode: DEFAULT_MOVE_MODE, submitting: false });
    void client.knowledgeBases.documents.moveTargets(knowledgeBaseId)
      .then((targets) => setMoveKb((current) => current && current.documentId === documentId ? { ...current, targets, loading: false } : current))
      .catch(() => setMoveKb((current) => current && current.documentId === documentId ? { ...current, targets: [], loading: false } : current));
  }
  function selectMoveTarget(kb: MoveTargetKb) {
    setMoveKb((current) => current ? { ...current, selectedTargetId: kb.id, selectedTargetName: kb.name, mode: DEFAULT_MOVE_MODE, view: 'confirm' } : current);
  }
  function backMoveMenu() {
    setMoveKb((current) => {
      if (!current) return current;
      const next = moveMenuViewAfterBack(current.view);
      return next === 'normal' ? null : { ...current, view: next };
    });
  }
  function changeMoveMode(mode: KnowledgeMoveMode) {
    setMoveKb((current) => current ? { ...current, mode } : current);
  }
  function startMoveProgressPoll(taskId: string) {
    stopMovePoll();
    movePollTimer.current = window.setInterval(() => {
      void client.knowledgeBases.documents.moveProgress(taskId)
        .then((progress) => {
          if (progress.status === 'completed') {
            stopMovePoll();
            const failed = progress.failed ?? 0;
            if (failed > 0) showStageNotice(t('knowledgeBase.moveCompletedWithErrors', { success: (progress.processed ?? 0) - failed, failed }), 'warning');
            else showStageNotice(t('knowledgeBase.moveCompleted'), 'success');
            setReloadToken((value) => value + 1);
          } else if (progress.status === 'failed') {
            stopMovePoll();
            showStageNotice(t('knowledgeBase.moveFailed'), 'error');
          }
        })
        .catch(() => { /* ignore poll errors like Vue */ });
    }, 2000);
  }
  async function confirmMoveToKb() {
    if (!moveKb || !moveKb.selectedTargetId || moveKb.submitting) return;
    setMoveKb((current) => current ? { ...current, submitting: true } : current);
    try {
      const start = await client.knowledgeBases.documents.move({
        knowledge_ids: [moveKb.documentId],
        source_kb_id: knowledgeBaseId,
        target_kb_id: moveKb.selectedTargetId,
        mode: moveKb.mode,
      });
      showStageNotice(t('knowledgeBase.moveStarted'), 'neutral');
      setMoveKb(null);
      if (start.taskId) startMoveProgressPoll(start.taskId);
      else setReloadToken((value) => value + 1);
    } catch (error) {
      showStageNotice(error instanceof Error && error.message ? error.message : t('knowledgeBase.moveFailed'), 'error');
      setMoveKb((current) => current ? { ...current, submitting: false } : current);
    }
  }
  // Vue pipes the move sub-flow state into every row menu; view stays 'normal'
  // on the rows that did not open it, but onStart stays callable everywhere.
  const moveControllerFor = useCallback((document: KnowledgeDocument): DocumentMoveKbController => {
    const active = moveKb && moveKb.documentId === document.id ? moveKb : null;
    return {
      view: active ? active.view : 'normal',
      targets: active?.targets ?? [],
      loading: active?.loading ?? false,
      selectedTargetName: active?.selectedTargetName ?? '',
      mode: active?.mode ?? DEFAULT_MOVE_MODE,
      submitting: active?.submitting ?? false,
      onStart: () => startMoveToKb(document),
      onSelectTarget: selectMoveTarget,
      onBack: backMoveMenu,
      onModeChange: changeMoveMode,
      onConfirm: () => void confirmMoveToKb(),
    };
  }, [moveKb, knowledgeBaseId, t, client]);
  const [traceState, setTraceState] = useState<{ status: "idle" | "loading" | "success" | "error"; steps: KnowledgeTimelineStep[]; nodes: KnowledgeTimelineNode[]; parseStatus?: string; message?: string; lastError?: { error_code?: string; error_message?: string } | null }>({ status: "idle", steps: [], nodes: [] });
  const [expandedTraceNodes, setExpandedTraceNodes] = useState<Set<string>>(new Set());
  const [selectedTraceNode, setSelectedTraceNode] = useState<KnowledgeTimelineNode | null>(null);
  const [confirmingTraceCancel, setConfirmingTraceCancel] = useState(false);
  // Multi-file upload parity: staged files wait behind a confirm dialog
  // (Vue UploadConfirmDialog) before any upload call is issued.
  const [pendingEntries, setPendingEntries] = useState<UploadEntry[]>([]);
  // Vue localUrls: the dialog stages a LIST of URL rows, not a single slot.
  const [pendingUrls, setPendingUrls] = useState<string[]>([]);
  const [pendingManual, setPendingManual] = useState<{ title: string; content: string } | null>(null);
  const [uploadTargetFolder, setUploadTargetFolder] = useState("");
  // Destination picker state (Vue destination-row + FolderPickerMenu).
  const [destinationPickerOpen, setDestinationPickerOpen] = useState(false);
  const [creatingUnder, setCreatingUnder] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState("");
  const [pendingFolderPaths, setPendingFolderPaths] = useState<string[]>([]);
  const [destinationPickerDuplicate, setDestinationPickerDuplicate] = useState(false);
  const [pendingTagIds, setPendingTagIds] = useState<string[]>([]);
  // Dialog config state (Vue uiState): one object seeded from the KB.
  const [confirmState, setConfirmState] = useState<UploadConfirmUIState>(() => uploadConfirmStateFromKb(null));
  const [chunkingMoreOpen, setChunkingMoreOpen] = useState(false);
  const [stageNotice, setStageNotice] = useState<{ tone: "neutral" | "success" | "warning" | "error"; text: string } | null>(null);
  const stageNoticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const showStageNotice = (text: string, tone: "neutral" | "success" | "warning" | "error") => {
    setStageNotice({ tone, text });
    if (stageNoticeTimer.current) clearTimeout(stageNoticeTimer.current);
    stageNoticeTimer.current = setTimeout(() => setStageNotice(null), 3000);
  };
  useEffect(() => () => {
    if (stageNoticeTimer.current) clearTimeout(stageNoticeTimer.current);
  }, []);
  const [parserEngines, setParserEngines] = useState<ParserEngineInfo[]>([]);
  const [parserEnginesLoading, setParserEnginesLoading] = useState(true);
  const [tenantModels, setTenantModels] = useState<ModelConfiguration[]>([]);
  // Vue editorResources.systemInfo (GET /api/v1/system/info): gates the graph section.
  const [systemInfo, setSystemInfo] = useState<Record<string, unknown> | null>(null);
  // Vue authStore.me — the admin gate for the graph extraction actions.
  const [me, setMe] = useState<KBSurfaceMe | null>(null);
  // Vue activeSection + the unavailable-fallback watch (L1309-1313).
  const [activeSection, setActiveSection] = useState<UploadConfirmSectionKey>("tags");
  // Vue KbUploadSourceDropdown: menu + URL sub-dialog state.
  const [sourceMenuOpen, setSourceMenuOpen] = useState(false);
  const [sourceUrlDialogOpen, setSourceUrlDialogOpen] = useState(false);
  const [sourceUrlValue, setSourceUrlValue] = useState("");
  const [uploadStates, setUploadStates] = useState<readonly UploadEntryState[]>(
    [],
  );
  const [dragActive, setDragActive] = useState(false);
  const [manualTitle, setManualTitle] = useState("");
  const [manualContent, setManualContent] = useState("");
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const uploadPipelineController = useRef<AbortController | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [pendingReparse, setPendingReparse] = useState<{
    document: KnowledgeDocument;
    processConfig: unknown;
  } | null>(null);
  const [pendingBatchReparse, setPendingBatchReparse] = useState<string[] | null>(null);
  const pageSize = 20;

  // Vue isFiltering: any active filter (search descends the folder subtree).
  const filtering = isFilteringDocuments({
    keyword: debouncedQuery,
    tagIds: selectedTagIds,
    fileType,
    parseStatus,
    source,
    timeRange: [updatedFrom, updatedTo],
  });

  function seedConfirmFromKb() {
    // Vue visible-watch: re-seed from the KB, then pick the default section
    // from the freshly-seeded state (getDefaultSection) and close more-options.
    const seeded = uploadConfirmStateFromKb(kbMeta);
    setConfirmState(seeded);
    setChunkingMoreOpen(false);
    setActiveSection(defaultUploadConfirmSection({
      mode: dialogMode,
      multimodalIssue: multimodalSectionIssue({
        state: seeded,
        hasImages: batchHasImages(batchUploadExtensions({ entries: pendingEntries, urls: pendingUrls }), pendingManual?.content),
      }),
      asrIssue: asrSectionIssue({
        state: seeded,
        hasAudio: batchHasAudio(batchUploadExtensions({ entries: pendingEntries, urls: pendingUrls })),
      }),
    }));
  }

  function updateConfirm(patch: Partial<UploadConfirmUIState>) {
    setConfirmState((current) => ({ ...current, ...patch }));
  }

  // Vue handleNodeExtractUpdate (L1375-1378): the graph config change also
  // mirrors its enabled flag into uiState.graphEnabled.
  function updateNodeExtract(config: UploadNodeExtractState) {
    setConfirmState((current) => ({ ...current, nodeExtract: config, graphEnabled: config.enabled }));
  }

  // The three admin extraction calls behind GraphSettings' generate/extract
  // buttons (Vue api/initialization.ts fabriTag/fabriText/extractTextRelations;
  // backend routes internal/router/routes_infra.go:123-125). api-client has no
  // bindings for them, so they go through the client's public request method.
  async function runGraphExtractAction(action: UploadGraphExtractAction, body: Record<string, unknown>): Promise<UploadGraphExtractResult | null> {
    const data = await client.request({
      method: "POST",
      path: `/api/v1/initialization/extract/${action}`,
      body,
    }) as { code?: unknown; data?: unknown };
    if (!data || typeof data !== "object" || data.code !== 0) return null;
    const payload = data.data;
    if (!payload || typeof payload !== "object") return null;
    const row = payload as Record<string, unknown>;
    const result: UploadGraphExtractResult = {};
    if (Array.isArray(row.tags)) result.tags = row.tags.filter((tag): tag is string => typeof tag === "string");
    if (typeof row.text === "string") result.text = row.text;
    if (Array.isArray(row.nodes)) result.nodes = row.nodes.filter((node): node is UploadGraphNodeState =>
      !!node && typeof node === "object" && typeof (node as { name?: unknown }).name === "string");
    if (Array.isArray(row.relations)) result.relations = row.relations.filter((relation): relation is UploadGraphRelationState =>
      !!relation && typeof relation === "object" && typeof (relation as { node1?: unknown }).node1 === "string");
    return result;
  }

  // Audit #6: KB-type routing — an FAQ KB must land on the FAQ route.
  // The same fetch drives permission gating and tab visibility, and seeds the
  // upload-confirm dialog defaults (Vue initFromKbInfo).
  useEffect(() => {
    try { window.localStorage.setItem("weknora.kb.docs.viewMode", viewMode); } catch { /* storage is optional */ }
  }, [viewMode]);

  useEffect(() => {
    let active = true;
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
      // Vue KBSwitcherDropdown input: the tenant KB list behind the crumb menu.
      client.knowledgeBases.list().catch(() => []),
      // Vue effectiveKBPermission consults the org shared-knowledge-bases
      // grant before kb.my_permission (KnowledgeBase.vue:326).
      client.identity.organizations.knowledgeBaseShares.listShared().catch(() => null),
    ] as const)
      .then(([kb, me, list, sharedRows]) => {
        if (!active) return;
        setKbMeta(kb as KBSurfaceKB);
        setKbMetaError(null);
        setMe(me as KBSurfaceMe | null);
        setConfirmState(uploadConfirmStateFromKb(kb as KBSurfaceKB));
        setCanContribute(canUploadKnowledgeDocuments(kb as KBSurfaceKB, me as KBSurfaceMe | null));
        setCanDownload(canDownloadKnowledgeDocuments(kb as KBSurfaceKB, me as KBSurfaceMe | null, sharedRows));
        setCanMutate(canMutateKnowledgeDocuments(kb as KBSurfaceKB, me as KBSurfaceMe | null));
        setKbList(
          (list as { id: unknown; name: unknown; type?: unknown }[]).map((item) => ({
            id: String(item.id),
            name: String(item.name),
            type: typeof item.type === "string" ? item.type : undefined,
          })),
        );
        const redirect = kbTypeRedirectPath(kb as KBSurfaceKB);
        if (redirect) navigate(redirect, 'replace');
      })
      .catch((error: unknown) => {
        if (active) {
          setKbMeta(null);
          setCanContribute(false);
          setCanDownload(false);
          setCanMutate(false);
          setKbMetaError(classifyKnowledgeBaseMetadataError(error, t("knowledgeBase.getInfoFailed")));
        }
      });
    return () => {
      active = false;
    };
  }, [client, knowledgeBaseId, kbMetaAttempt, locale]);

  useEffect(() => {
    let active = true;
    void client.knowledgeBases.settings.parserEngines().then((result) => { if (active) { setParserEngines(result.data); setParserEnginesLoading(false); } }).catch(() => { if (active) { setParserEngines([]); setParserEnginesLoading(false); } });
    return () => { active = false; };
  }, [client]);

  // Vue KnowledgeBase.vue:2741 + 339-345 — the one-shot kbDetail welcome tour
  // arms on detail-page entry (any tab, incl. deep links) for an editable,
  // non-FAQ knowledge base whose document list finished loading empty. The
  // shell guide host (PlatformShell) owns dismissal + welcome-tour gating.
  useKbDetailGuideTrigger({
    knowledgeBaseId,
    kbType: typeof kbMeta?.type === "string" ? kbMeta.type : null,
    canEdit: canContribute,
    documentsLoading: state.status !== "success",
    documentCount: state.status === "success" ? state.page.items.length : 0,
  });

  useEffect(() => {
    let active = true;
    void client.configuration.models.list().then((models) => { if (active) setTenantModels(models); }).catch(() => { if (active) setTenantModels([]); });
    return () => { active = false; };
  }, [client]);

  // Vue loadSystemInfo: on failure the graph section falls back to hidden.
  useEffect(() => {
    let active = true;
    void client.settings.system.info().then((info) => { if (active) setSystemInfo(info); }).catch(() => { if (active) setSystemInfo(null); });
    return () => { active = false; };
  }, [client]);

  useEffect(() => {
    let active = true;
    setState({ status: "loading" });
    void loadKnowledgeDocuments(client, knowledgeBaseId, {
      page,
      page_size: pageSize,
      keyword: debouncedQuery || undefined,
      parse_status: parseStatus || undefined,
      tag_ids: joinTagIds(selectedTagIds),
      file_type: fileType || undefined,
      source: source || undefined,
      ...dateRangeToTimeParams(updatedFrom || updatedTo ? [updatedFrom, updatedTo] : undefined),
      folder_path: folderPath,
      // Vue: browsing lists one folder level; filtering descends the subtree.
      folder_recursive: folderPath !== undefined && filtering,
    }, {
      unavailable: t("common.error"),
      failed: t("common.error"),
    }).then((next) => {
      if (active) setState(next);
    });
    return () => {
      active = false;
    };
  }, [
    client,
    folderPath,
    filtering,
    fileType,
    knowledgeBaseId,
    page,
    parseStatus,
    selectedTagIds,
    source,
    updatedFrom,
    updatedTo,
    debouncedQuery,
    reloadToken,
  ]);

  // Audit #10: 300ms debounce so fast typing issues a single API call.
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 300);
    return () => clearTimeout(timer);
  }, [query]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setTagPage(1);
      setTagSearchDebounced(tagSearchQuery.trim());
    }, 300);
    return () => clearTimeout(timer);
  }, [tagSearchQuery]);

  useEffect(() => {
    let active = true;
    setFolderState({ status: "loading" });
    void client.knowledgeBases.documents.folders(knowledgeBaseId)
      .then((tree) => { if (active) setFolderState({ status: "success", tree }); })
      .catch((error: unknown) => { if (active) setFolderState({ status: "error", message: errorMessage(error, t) }); });
    return () => { active = false; };
  }, [client, knowledgeBaseId, reloadToken]);

  useEffect(() => {
    let active = true;
    if (tagPage > 1) setTagLoadingMore(true);
    void client.knowledgeBases.documents.tagsPage(knowledgeBaseId, {
      page: tagPage,
      page_size: 50,
      keyword: tagSearchDebounced || undefined,
    }).then((result) => {
      if (!active) return;
      setTags((current) => tagPage === 1 ? result.data : [...current, ...result.data]);
      setTagTotal(result.total ?? result.data.length);
      setTagHasMore((tagPage === 1 ? result.data.length : tags.length + result.data.length) < (result.total ?? result.data.length));
    }).catch((error: unknown) => {
      if (active) setTagTotal(0);
    }).finally(() => { if (active) setTagLoadingMore(false); });
    return () => { active = false; };
  // `tags.length` is intentionally excluded: this effect owns the page append.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, knowledgeBaseId, reloadToken, tagPage, tagSearchDebounced]);

  useEffect(() => {
    setPage(1);
  }, [folderPath, parseStatus, query, selectedTagIds, fileType, source, updatedFrom, updatedTo]);

  // Vue watch(selectedTagIds, docSearchKeyword, …) (KnowledgeBase.vue L2261-2266):
  // clear the selection on any filter/kb change so batch ops never act on
  // documents the user can no longer see.
  useEffect(() => {
    setSelected(new Set());
    lastSelectedIndex.current = -1;
  }, [debouncedQuery, selectedTagIds, fileType, parseStatus, source, updatedFrom, updatedTo, knowledgeBaseId]);

  const folders = useMemo(
    () => (folderState.tree ? flattenFolders(folderState.tree) : []),
    [folderState.tree],
  );
  // Vue only allocates the folder column when the tree has a real folder;
  // the synthetic Root row alone must leave the document surface full width.
  const showFolderTree = folders.some((folder) => folder.path.trim() !== "");
  const vllmModels = useMemo(() => tenantModels.filter((model) => String(model.type ?? "").toLowerCase() === 'vllm'), [tenantModels]);
  const asrModels = useMemo(() => tenantModels.filter((model) => String(model.type ?? '').toLowerCase() === 'asr'), [tenantModels]);
  // Vue supportedFileTypes / unsupportedFileTypes computeds: the KB's
  // chunking_config.parser_engine_rules resolved against the tenant engines.
  const parserRules = useMemo(() => {
    const chunking = kbMeta?.chunking_config as { parser_engine_rules?: unknown } | null | undefined;
    return Array.isArray(chunking?.parser_engine_rules)
      ? (chunking.parser_engine_rules as { file_types: string[]; engine: string }[])
      : [];
  }, [kbMeta]);
  const supportedFileTypes = useMemo(
    () => computeSupportedFileTypes(parserEngines, parserRules),
    [parserEngines, parserRules],
  );
  const unsupportedFileTypes = useMemo(
    () => computeUnsupportedFileTypes(parserEngines, parserRules),
    [parserEngines, parserRules],
  );
  const storageEngineMissing = isStorageEngineMissing(kbMeta);

  const dialogMode: UploadDialogMode = pendingReparse ? "reparse" : pendingManual ? "manual" : "file";
  // Vue batchFileExts: files + URL paths + manual markdown media + reparse type.
  const batchExts = useMemo(() => batchUploadExtensions({
    entries: pendingEntries,
    urls: pendingUrls,
    manualContent: dialogMode === "manual" ? pendingManual?.content : undefined,
    reparseFileType: dialogMode === "reparse" ? pendingReparse?.document.file_type : undefined,
  }), [pendingEntries, pendingUrls, pendingManual, pendingReparse, dialogMode]);
  const hasPdf = batchExts.includes("pdf");
  const hasImages = batchHasImages(batchExts, dialogMode === "manual" ? pendingManual?.content : undefined);
  const hasAudio = batchHasAudio(batchExts);
  const multimodalIssue = multimodalSectionIssue({ state: confirmState, hasImages });
  const asrIssue = asrSectionIssue({ state: confirmState, hasAudio });
  const batchItemCount = pendingEntries.length + pendingUrls.length;
  // Graph section gating (Vue isGraphDatabaseEnabled / isGraphSectionAvailable)
  // plus the summary model and admin role its actions require.
  const graphDatabaseOn = graphDatabaseEnabled(systemInfo);
  const graphAvailable = graphSectionAvailable({ systemInfo, graphEnabled: confirmState.graphEnabled });
  const graphAdmin = hasGraphAdminRole(me);
  const llmModelId = typeof kbMeta?.summary_model_id === "string" ? kbMeta.summary_model_id : "";
  // Destination picker options: server folders plus folders created in-dialog.
  const pickerFolderOptions = useMemo(
    () => mergeFolderOptions(
      folders.filter((folder) => folder.path).map((folder) => ({ path: folder.path, name: folder.name, depth: folder.depth })),
      pendingFolderPaths,
    ),
    [folders, pendingFolderPaths],
  );

  function modelName(modelId: string): string | undefined {
    if (!modelId) return undefined;
    return tenantModels.find((model) => model.id === modelId)?.name;
  }

  const items = state.status === "success" ? state.page.items : [];

  // Vue's knowledge_id deep link opens the matching document drawer after the
  // first list response, including legacy /knowledgeBase URLs.
  useEffect(() => {
    if (openedInitialDocument.current || !initialDocumentId || state.status !== "success") return;
    const document = state.page.items.find((item) => String(item.id) === initialDocumentId);
    if (!document) return;
    openedInitialDocument.current = true;
    openDocumentDetail(document);
  }, [initialDocumentId, onOpenDocument, state]);
  const selectedOnPage = items.filter((item) => selected.has(item.id)).length;
  const allOnPageSelected = items.length > 0 && selectedOnPage === items.length;
  const selectedDocuments = items.filter((item) => selected.has(item.id));
  // Vue tagMap/activeTagFilterLabel/activeTagFilterTitle (KnowledgeBase.vue L684-729).
  const tagNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const tag of tags) map.set(tag.id, tag.name);
    return map;
  }, [tags]);
  const tagTriggerLabel = (() => {
    const label = tagFilterLabel(selectedTagIds, tagFilterCleared);
    if (label.kind === "single") return tagNameById.get(label.id) ?? t("knowledgeBase.allTags");
    if (label.kind === "multi") return t("knowledgeBase.tagFilterMulti", { count: selectedTagIds.length });
    if (label.kind === "placeholder") return t("knowledgeBase.tagFilterPlaceholder");
    return t("knowledgeBase.allTags");
  })();
  const tagTriggerTitle = tagFilterTitle(
    selectedTagIds,
    (id) => tagNameById.get(id),
    t("knowledgeBase.tagFilterTitle"),
  );

  // Vue handleTagRowClick (KnowledgeBase.vue L948-959): chip click toggles one tag.
  function toggleTagFilter(tagId: string) {
    setTagFilterCleared(false);
    setSelectedTagIds((current) =>
      current.includes(tagId) ? current.filter((value) => value !== tagId) : [...current, tagId],
    );
  }

  // Vue clearTagFilter (KnowledgeBase.vue L961-964): drop to the placeholder state.
  function clearTagFilter() {
    setTagFilterCleared(true);
    setSelectedTagIds([]);
  }

  function toggleAllOnPage() {
    setSelected(allOnPageSelected ? new Set() : new Set(items.map((item) => item.id)));
    lastSelectedIndex.current = -1;
  }
  const pageTotal = state.status === "success" ? state.page.total : 0;
  // resolveKBSurfaceTabs is the strict Vue isWiki gate (permissions.ts): a KB
  // with the wiki off yields no tabs at all — even with graph extraction on —
  // and the breadcrumb falls back to the plain 文档 crumb. While the KB
  // metadata is still loading the Vue page also shows the plain crumb
  // (isWiki=false until kbInfo lands), so the fallback is empty too.
  const tabs = useMemo(
    () => (kbMeta ? resolveKBSurfaceTabs(kbMeta) : []),
    [kbMeta],
  );

  // Vue ⚙ (KnowledgeBase.vue:2388 → uiStore.openKBSettings) opens the KB
  // settings surface in place instead of navigating away; the Dialog below
  // re-hosts KnowledgeSettingsPage for the same behavior.
  const [kbSettingsOpen, setKbSettingsOpen] = useState(false);
  // Vue renders the 文档/Wiki/图谱 row inline as the third breadcrumb level
  // (KnowledgeBase.vue:2359-2380, activeKbTab === 'documents' here); the
  // label keys match the graph page's breadcrumb tabs so both surfaces read
  // identically.
  const breadcrumbTabs: DocumentsBreadcrumbTab[] = tabs.map((tab) => ({
    key: tab,
    label: tab === "documents"
      ? t("knowledgeEditor.wikiBrowser.tabDocuments")
      : tab === "wiki"
        ? "Wiki" /* Vue template renders the wiki tab as the literal "Wiki" (KnowledgeBase.vue L2365) */
        : t("knowledgeEditor.wikiBrowser.tabGraph"),
    href: tab === "documents"
      ? documentsKBDetailPath(knowledgeBaseId)
      : `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}?tab=${tab}`,
    active: tab === "documents",
    title: tab === "graph" ? t("knowledgeEditor.wikiBrowser.tabGraphTip") : undefined,
  }));

  // Vue canConfirm: empty batch, empty manual content or a required model
  // missing (multimodal/ASR) disables the confirm button.
  const canConfirm = useMemo(() => {
    if (dialogMode === "file" && batchItemCount === 0) return false;
    if (dialogMode === "manual" && !pendingManual?.content?.trim()) return false;
    if (multimodalIssue) return false;
    if (asrIssue) return false;
    return true;
  }, [dialogMode, batchItemCount, pendingManual, multimodalIssue, asrIssue]);

  function toggleSelected(id: string, shiftKey = false) {
    const checked = !selected.has(id);
    setSelected((current) => {
      const result = toggleDocumentSelection({ ids: items.map((item) => item.id), selected: current, id, checked, shiftKey, lastIndex: lastSelectedIndex.current });
      lastSelectedIndex.current = result.lastIndex;
      return result.selected;
    });
  }

  const marquee = useMarqueeSelection({
    containerRef: documentListRef,
    selected,
    setSelected,
    enabled: canContribute && items.length > 0,
    onSelectionStart: () => setMoving(false),
  });

  function ensureDocumentKbReady(): boolean {
    if (!storageEngineMissing) return true;
    showStageNotice(t('knowledgeBase.missingStorageEngineUpload'), 'warning');
    return false;
  }

  function stageFiles(files: Iterable<File>, fromFolder = false) {
    if (!ensureDocumentKbReady()) return;
    const filtered = filterUploadFiles(files, { supportedFileTypes, fromFolder });
    const entries = toUploadEntries(filtered.validFiles);
    if (filtered.skippedCount > 0) showStageNotice(ct("knowledgeBase.filesSkippedNoEngine", { count: filtered.skippedCount }), "warning");
    if (filtered.videoFilteredCount > 0) showStageNotice(ct("knowledgeBase.videosFilteredNoVLM", { count: filtered.videoFilteredCount }), "warning");
    if (entries.length === 0) return;
    if (pendingEntries.length === 0) setUploadTargetFolder(folderPath ?? "");
    // Vue appendFiles: dedupe by (relative path|name)+size and report counts.
    const merged = mergeUploadEntries(pendingEntries, entries);
    setPendingEntries(merged.entries);
    setUploadStates([]);
    setUploadError(null);
    if (merged.addedCount > 0) showStageNotice(ct("uploadConfirm.filesAdded", { count: merged.addedCount }), "neutral");
    else if (merged.duplicateCount > 0) showStageNotice(ct("uploadConfirm.filesAllDuplicate"), "warning");
  }

  // Vue appendUrl: append to the staged URL list, dedupe with a warning.
  function appendStagedUrl(rawUrl: string, input: string): boolean {
    if (!ensureDocumentKbReady()) return false;
    const normalized = normalizeUploadUrl(rawUrl);
    if (!normalized) {
      if (input === "dialog") setUploadError(ct("knowledgeBase.invalidURL"));
      else setUploadError(t("knowledgeBase.documents.url"));
      return false;
    }
    if (pendingUrls.includes(normalized)) {
      showStageNotice(ct("uploadConfirm.urlDuplicate"), "warning");
      return false;
    }
    setPendingUrls((current) => [...current, normalized]);
    setUploadError(null);
    showStageNotice(ct("uploadConfirm.urlAdded"), "neutral");
    return true;
  }

  function resetDestinationPicker() {
    setDestinationPickerOpen(false);
    setCreatingUnder(null);
    setNewFolderName("");
    setPendingFolderPaths([]);
    setDestinationPickerDuplicate(false);
  }

  function cancelStagedUploads() {
    uploadPipelineController.current?.abort();
    uploadPipelineController.current = null;
    setPendingEntries([]);
    setPendingUrls([]);
    setPendingManual(null);
    setUploadTargetFolder("");
    setPendingTagIds([]);
    setUploadStates([]);
    setUploading(false);
    setStageNotice(null);
    setSourceMenuOpen(false);
    setSourceUrlDialogOpen(false);
    setPageSourceMenuOpen(false);
    setManualDialogOpen(false);
    resetDestinationPicker();
    setChunkingMoreOpen(false);
  }

  function closeReparseDialog() {
    if (uploading) return;
    setPendingReparse(null);
    // Vue re-seeds from the KB each time the dialog opens; restore defaults so
    // a canceled reparse never leak per-run overrides into the next upload.
    seedConfirmFromKb();
  }

  function closeUploadConfirmDialog() {
    if (!canCloseUploadConfirmDialog(uploading)) return;
    if (dialogMode === "reparse") closeReparseDialog();
    else cancelStagedUploads();
  }

  function choosePickerFolder(path: string) {
    // Vue onDestinationPicked: the confirmed folder becomes the batch target
    // and the popup closes (allowReselect).
    setUploadTargetFolder(path);
    setDestinationPickerOpen(false);
    setCreatingUnder(null);
    setNewFolderName("");
  }

  function startCreatingUnder(parentPath: string) {
    setDestinationPickerDuplicate(false);
    setCreatingUnder((current) => (current === parentPath ? null : parentPath));
    setNewFolderName("");
  }

  function commitPickerFolder() {
    if (creatingUnder === null) return;
    const result = commitFolderName(pickerFolderOptions, creatingUnder, newFolderName);
    if (result.status === "invalid") return;
    if (result.status === "duplicate") {
      setDestinationPickerDuplicate(true);
      return;
    }
    setDestinationPickerDuplicate(false);
    // Vue onDestinationCreated: created folders persist across picker cycles
    // until the upload lands them server-side.
    setPendingFolderPaths((current) => (current.includes(result.path) ? current : [...current, result.path]));
    setCreatingUnder(null);
    setNewFolderName("");
  }

  function removeStagedUpload(index: number) {
    if (uploading) return;
    setPendingEntries((current) => removeUploadEntry(current, index));
    setUploadStates((current) =>
      current.filter((_, stateIndex) => stateIndex !== index),
    );
  }

  function removeStagedUrl(index: number) {
    setPendingUrls((current) => current.filter((_, urlIndex) => urlIndex !== index));
  }

  // Sequential uploads (one call per file) with per-file status; a per-file
  // failure keeps the dialog open so the errors stay visible (Vue parity).
  async function confirmUpload() {
    if (dialogMode === "reparse") {
      await confirmReparse();
      return;
    }
    if (!pendingManual && pendingEntries.length === 0 && pendingUrls.length === 0) {
      setUploadError(ct("uploadConfirm.noItems"));
      return;
    }
    // No chunking range guard here: Vue handleConfirm only runs the
    // multimodal/ASR validateBeforeConfirm (UploadConfirmDialog.vue L1380-1411)
    // — the 100-4000 / 0-500 bounds are the number inputs' UI range (React
    // UploadNumberInput min/max matches the Vue t-input-number bounds), and the
    // KB seed falls back to the Vue defaults for "not customized" zeros. A
    // submit-time guard here used to swallow the confirm click with no POST
    // whenever a live KB stored chunk_size 0 (R474 A4 P2).
    const validationFailure = uploadConfirmValidationFailure({ state: confirmState, hasImages, hasAudio });
    if (validationFailure) {
      setUploadError(ct(validationFailure.messageKey));
      if (validationFailure.patch) updateConfirm(validationFailure.patch);
      goToSection(validationFailure.section);
      return;
    }
    const processConfig = buildUploadConfirmOverrides(confirmState);
    setUploadError(null);
    setUploading(true);
    if (pendingManual) {
      try {
        await client.knowledgeBases.documents.createManual(knowledgeBaseId, { title: pendingManual.title, content: pendingManual.content, status: "pending", process_config: processConfig });
        setPendingManual(null);
        setPendingTagIds([]);
        setChunkingMoreOpen(false);
        setReloadToken((value) => value + 1);
        emitKnowledgeUploadEvent("knowledgeFileUploaded", { kbId: knowledgeBaseId });
      } catch (error) {
        setUploadError(errorMessage(error, t));
      } finally { setUploading(false); }
      return;
    }
    if (pendingUrls.length > 0) {
      // Vue emits every staged URL with the batch; each becomes its own
      // document. A failure keeps the dialog open with the failed URL plus
      // the not-yet-attempted ones still staged.
      const remaining: string[] = [];
      let failed = false;
      for (const stagedUrl of pendingUrls) {
        try {
          const created = await client.knowledgeBases.documents.createFromUrl(
            knowledgeBaseId,
            {
              url: stagedUrl,
              tag_ids: pendingTagIds,
              process_config: processConfig,
            },
          );
          if (uploadTargetFolder && created.id) {
            await client.knowledgeBases.documents.moveToFolder(
              knowledgeBaseId,
              [created.id],
              uploadTargetFolder,
            );
          }
          setReloadToken((value) => value + 1);
          emitKnowledgeUploadEvent("knowledgeFileUploaded", { kbId: knowledgeBaseId });
        } catch (error) {
          remaining.push(stagedUrl);
          if (!failed) setUploadError(errorMessage(error, t));
          failed = true;
        }
      }
      setPendingUrls(remaining);
      if (failed) {
        setUploading(false);
        return;
      }
    }
    if (pendingEntries.length === 0) {
      setPendingTagIds([]);
      setUploading(false);
      return;
    }
    const controller = new AbortController();
    uploadPipelineController.current = controller;
    const uploadBatchId = `${knowledgeBaseId}-${Date.now()}`;
    pendingEntries.forEach((entry, index) => emitKnowledgeUploadEvent("knowledgeFileUploadStart", {
      uploadId: `${uploadBatchId}-${index}`,
      kbId: knowledgeBaseId,
      fileName: entry.name,
      progress: 0,
    }));
    try {
      const finalStates = await runUploadPipeline({
        entries: pendingEntries,
        tagIds: pendingTagIds,
        signal: controller.signal,
        upload: async (entry, tagIds, signal, onProgress) => {
          // Vue parity: uploadKnowledgeFile(..., onProgress) streams upload
          // percent (frontend/src/api/knowledge-base/index.ts:207-231). The page
          // mints the blob: source itself so the transport can key its byte
          // progress observer, then hands the native source to the client.
          const file = entry.file;
          const uri = URL.createObjectURL(file);
          const stopObserving = observeUploadProgress(uri, (progress) => {
            onProgress?.(uploadProgressPercent(progress));
          });
          try {
            const created = await client.knowledgeBases.documents.upload(
              knowledgeBaseId,
              {
                file: { uri, name: entry.name || file.name || "file", type: file.type || "application/octet-stream", size: file.size },
                fileName: entry.name,
                tag_ids: tagIds,
                process_config: processConfig,
              },
              signal,
            );
            if (uploadTargetFolder && created.id) {
              await client.knowledgeBases.documents.moveToFolder(
                knowledgeBaseId,
                [created.id],
                uploadTargetFolder,
              );
            }
            return created;
          } finally {
            stopObserving();
          }
        },
        onStateChange: (states) => {
          setUploadStates(states);
          states.forEach((state, index) => emitKnowledgeUploadEvent("knowledgeFileUploadProgress", {
            uploadId: `${uploadBatchId}-${index}`,
            kbId: knowledgeBaseId,
            fileName: state.entry.name,
            progress: state.status === "done" || state.status === "error" ? 100 : state.status === "uploading" ? state.progress ?? 0 : 0,
          }));
          states.forEach((state, index) => {
            if (state.status !== "done" && state.status !== "error") return;
            emitKnowledgeUploadEvent("knowledgeFileUploadComplete", {
              uploadId: `${uploadBatchId}-${index}`,
              kbId: knowledgeBaseId,
              fileName: state.entry.name,
              progress: 100,
              status: state.status === "done" ? "success" : "error",
              error: state.message,
            });
          });
        },
      });
      setReloadToken((value) => value + 1);
      if (finalStates.some((state) => state.status === "done")) emitKnowledgeUploadEvent("knowledgeFileUploaded", { kbId: knowledgeBaseId });
      if (finalStates.some((state) => state.status === "error")) {
        // Keep only failed files staged so the next confirmation retries the
        // failed subset instead of re-uploading documents already accepted.
        const failedStates = finalStates.filter((state) => state.status === "error");
        setPendingEntries(retryableUploadEntries(finalStates));
        setUploadStates(failedStates);
        return;
      }
      setPendingEntries([]);
      setPendingUrls([]);
      setPendingTagIds([]);
      setUploadStates([]);
      setStageNotice(null);
      resetDestinationPicker();
      setChunkingMoreOpen(false);
    } catch (error) {
      setUploadError(errorMessage(error, t));
    } finally {
      if (uploadPipelineController.current === controller)
        uploadPipelineController.current = null;
      setUploading(false);
    }
  }

  // Vue handleManualCreate: the dropdown's manual entry stages title/content
  // into the existing pendingManual confirm-dialog flow.
  function stageManualCreate() {
    if (!ensureDocumentKbReady()) return;
    if (!manualTitle.trim() || !manualContent.trim()) {
      setUploadError(t("knowledgeBase.documents.manualTitle"));
      return;
    }
    if (pendingEntries.length === 0 && pendingUrls.length === 0) setUploadTargetFolder(folderPath ?? "");
    setPendingManual({ title: manualTitle.trim(), content: manualContent });
    setPendingTagIds([]);
    setUploadError(null);
    setManualTitle("");
    setManualContent("");
    setManualDialogOpen(false);
  }

  async function openManualEdit(document: KnowledgeDocument) {
    setManualEditDocument(document);
    setManualEditLoading(true);
    setManualEditSaving(false);
    setUploadError(null);
    setManualTitle(displayName(document).replace(/\.md$/i, ""));
    setManualContent("");
    try {
      const detail = await client.knowledgeBases.documents.get(document.id);
      const manual = manualContentFromMetadata(detail.metadata);
      setManualTitle((detail.title || detail.file_name || displayName(document)).replace(/\.md$/i, ""));
      setManualContent(manual.content);
    } catch (error) {
      setUploadError(errorMessage(error, t));
    } finally {
      setManualEditLoading(false);
    }
  }

  function openTrace(document: KnowledgeDocument) {
    setTraceDocument(document);
    setTraceState({ status: "loading", steps: [], nodes: [] });
    setExpandedTraceNodes(new Set());
    setSelectedTraceNode(null);
  }

  useEffect(() => {
    if (!traceDocument) return;
    let active = true;
    let polling: number | undefined;
    let inFlight = false;
    const load = async () => {
      if (!active || inFlight) return;
      inFlight = true;
      try {
        const spans = resolveKnowledgeSpansView(await client.knowledgeBases.documents.spans(traceDocument.id));
        if (!active) return;
        const parseStatus = typeof spans.parse_status === "string" ? spans.parse_status : traceDocument.parse_status;
        const nodes = flattenKnowledgeSpans(spans.trace);
        setTraceState({ status: "success", steps: buildKnowledgeTimeline(spans), nodes, parseStatus, lastError: knowledgeSpansLastError(spans) });
        setExpandedTraceNodes((current) => current.size > 0 ? current : new Set(nodes.map((row) => row.key)));
        if (!isKnowledgeProcessingActive(parseStatus) && polling !== undefined) {
          window.clearInterval(polling);
          polling = undefined;
        }
      } catch (error) {
        if (active) setTraceState({ status: "error", steps: [], nodes: [], message: errorMessage(error, t) });
      } finally {
        inFlight = false;
      }
    };
    void load().then(() => {
      if (active && isKnowledgeProcessingActive(traceDocument.parse_status)) polling = window.setInterval(() => void load(), 2000);
    });
    return () => {
      active = false;
      if (polling !== undefined) window.clearInterval(polling);
    };
  }, [client, traceDocument]);

  async function saveManualEdit() {
    if (!manualEditDocument || manualEditSaving) return;
    if (!manualTitle.trim()) {
      setUploadError(t("knowledgeBase.documents.manualTitle"));
      return;
    }
    if (!manualContent.trim()) {
      setUploadError(t("knowledgeBase.documents.manualContent"));
      return;
    }
    setManualEditSaving(true);
    setUploadError(null);
    try {
      const metadata = manualContentFromMetadata(manualEditDocument.metadata);
      await client.knowledgeBases.documents.updateManual(manualEditDocument.id, {
        title: manualTitle.trim(),
        content: manualContent,
        status: metadata.status,
      });
      setManualEditDocument(null);
      setActionNotice({ tone: "success", text: t("knowledgeEditor.activity.actions.knowledge.updated") });
      setReloadToken((value) => value + 1);
    } catch (error) {
      setUploadError(errorMessage(error, t));
    } finally {
      setManualEditSaving(false);
    }
  }

  async function deleteSelected() {
    if (!selected.size) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.batchDelete(knowledgeBaseId, [
        ...selected,
      ]);
      setSelected(new Set());
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  async function downloadDocument(document: KnowledgeDocument) {
    setMutationError(null);
    try {
      const response = await client.knowledgeBases.documents.download(document.id);
      const body = response.body instanceof Blob ? response.body : new Blob([response.body], { type: response.contentType || response.headers["content-type"] || "application/octet-stream" });
      const url = URL.createObjectURL(body);
      const anchor = window.document.createElement("a");
      anchor.href = url;
      // Downloads keep the RAW file name (with extension) — Vue strips the
      // extension for display only; the persisted file_name stays authoritative.
      anchor.download = documentRawName(document);
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  async function deleteOneDocument() {
    if (!confirmingDeleteDocument) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.remove(confirmingDeleteDocument.id);
      setConfirmingDeleteDocument(null);
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  // Vue KnowledgeBase.vue handleBatchDownload (L453-504): filter to entries
  // with an original file, enforce the 200/512MiB caps with the upstream
  // warnings, stream the ZIP through the authenticated seam, save via an
  // object-URL anchor. Aborted on unmount / KB switch like the Vue flow.
  const [batchDownloading, setBatchDownloading] = useState(false);
  const batchDownloadController = useRef<AbortController | null>(null);
  useEffect(() => () => batchDownloadController.current?.abort(), []);
  async function handleBatchDownload() {
    if (batchDownloading || !selected.size) return;
    const itemsById = new Map(items.map((item) => [item.id, item]));
    const selection = selectBatchDownloadIds(selected, itemsById);
    const warningKey = batchDownloadPreflight(selection, batchDownloadKnownBytes(selection.ids, itemsById));
    if (warningKey) {
      showStageNotice(t(warningKey), "warning");
      return;
    }
    if (selection.skipped > 0) {
      showStageNotice(t("knowledgeBase.batchDownloadSkipped", { count: selection.skipped }), "warning");
    }
    const controller = new AbortController();
    batchDownloadController.current = controller;
    setBatchDownloading(true);
    try {
      const zip = await client.knowledgeBases.documents.batchDownload(knowledgeBaseId, selection.ids, controller.signal);
      if (controller.signal.aborted) return;
      const blob = zip.body instanceof Blob ? zip.body : new Blob([zip.body], { type: "application/zip" });
      saveBatchDownloadBlob(blob, batchDownloadZipName());
      showStageNotice(t("knowledgeBase.batchDownloadStarted"), "success");
    } catch (error) {
      if (!controller.signal.aborted) {
        showStageNotice(error instanceof Error ? error.message : t("knowledgeBase.batchDownloadFailed"), "error");
      }
    } finally {
      if (batchDownloadController.current === controller) {
        batchDownloadController.current = null;
        setBatchDownloading(false);
      }
    }
  }

  // Vue's batch popconfirm filters documents that are already being parsed
  // before the batch endpoint is called.
  function reparseSelected() {    if (!selected.size) return;
    const ids = filterReparseIds([...selected], items);
    if (!ids.length) {
      setMutationError(t("common.operationFailed"));
      return;
    }
    setMutationError(null);
    setPendingBatchReparse(ids);
  }

  async function confirmBatchReparse() {
    if (!pendingBatchReparse) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.batchReparse(knowledgeBaseId, pendingBatchReparse);
      setPendingBatchReparse(null);
      setSelected(new Set());
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  // Audit must-fix #1: cancel parse for every selected in-flight document.
  // (kept as the batch-bar parity slice dropped the dedicated 停止解析 button —
  // Vue DocumentBatchBar has no batch cancel-parse action; per-doc 停止解析
  // stays on the row menu / processing timeline.)
  void cancelSelectedParse;
  async function cancelSelectedParse() {
    if (!selected.size) return;
    setMutationError(null);
    const documentsApi = client.knowledgeBases.documents;
    try {
      await cancelParseDocuments(
        documentsApi,
        items.filter((item) => selected.has(item.id)),
      );
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  function reparseOne(document: KnowledgeDocument) {
    const storedOverrides = readStoredProcessOverrides(document);
    setMutationError(null);
    // Vue reparse mode: seed from KB defaults, then apply the overrides stored
    // at upload time so the user can adjust them before re-running.
    setConfirmState(applyUploadOverrides(uploadConfirmStateFromKb(kbMeta), storedOverrides));
    setChunkingMoreOpen(false);
    setPendingReparse({
      document,
      processConfig: storedOverrides ?? buildUploadConfirmOverrides(uploadConfirmStateFromKb(kbMeta)),
    });
  }

  async function confirmReparse() {
    if (!pendingReparse) return;
    setMutationError(null);
    // Reparse has no new media batch, but an enabled configuration still
    // requires its model and must reveal the corresponding section.
    const validationFailure = uploadConfirmValidationFailure({ state: confirmState, hasImages: false, hasAudio: false });
    if (validationFailure) {
      setUploadError(ct(validationFailure.messageKey));
      goToSection(validationFailure.section);
      return;
    }
    try {
      await client.knowledgeBases.documents.reparse(
        pendingReparse.document.id,
        buildUploadConfirmOverrides(confirmState),
      );
      setPendingReparse(null);
      seedConfirmFromKb();
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  async function cancelOneParse(id: string) {
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.cancelParse(id);
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  // Audit must-fix #6: inline folder select instead of window.prompt.
  async function moveSelected() {
    if (!selected.size) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.moveToFolder(
        knowledgeBaseId,
        [...selected],
        moveTarget.trim(),
      );
      setSelected(new Set());
      setMoving(false);
      setMoveTarget("");
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    }
  }

  // Vue onBatchTagConfirm (KnowledgeBase.vue L2169-2191) + handleKnowledgeTagChange
  // (L1005-1012): one PUT /knowledge/tags with an updates row per document, then
  // success feedback, selection cleared, list and tags reloaded.
  async function submitTagDialog(tagIds: string[]) {
    if (!tagDialog || tagDialogSaving) return;
    const targets = tagDialog.mode === "batch" ? selectedDocuments : [tagDialog.document];
    if (targets.length === 0) return;
    setTagDialogSaving(true);
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.updateTags(tagUpdatesFor(targets, tagIds));
      setTagDialog(null);
      setSelected(new Set());
      setActionNotice({
        tone: "success",
        text: tagDialog.mode === "single"
          ? tt("knowledgeBase.tagUpdateSuccess")
          : tt("knowledgeBase.batchTagSuccess", { count: targets.length }),
      });
      // reloadToken drives both the document list and the tag list effects.
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error, t));
    } finally {
      setTagDialogSaving(false);
    }
  }

  // Vue createKnowledgeBaseTag (POST /knowledge-bases/:id/tags); api-client has
  // no binding yet, same as the graph extract actions above.
  async function createKnowledgeTag(name: string): Promise<KnowledgeTag> {
    const data = await client.request({
      method: "POST",
      path: `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/tags`,
      body: { name },
    }) as { code?: unknown; data?: unknown };
    if (!data || typeof data !== "object" || data.code !== 0) throw new Error("Tag create failed");
    const payload = (data.data ?? {}) as { id?: unknown; name?: unknown };
    if (payload.id === undefined) throw new Error("Tag create failed");
    const created: KnowledgeTag = {
      id: String(payload.id),
      name: typeof payload.name === "string" ? payload.name : name,
    };
    setTags((current) => (current.some((tag) => tag.id === created.id) ? current : [...current, created]));
    return created;
  }

  function goToSection(key: UploadConfirmSectionKey) {
    setActiveSection(key);
    document.getElementById(`wk-upload-section-${key}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  // Vue watch(isGraphSectionAvailable) (L1309-1313): when the graph section
  // stops being available while it is the active one, fall back to the
  // default section for the current mode and issues.
  useEffect(() => {
    setActiveSection((current) =>
      sectionAfterGraphAvailabilityChange(
        current,
        graphAvailable,
        defaultUploadConfirmSection({ mode: dialogMode, multimodalIssue, asrIssue }),
      ),
    );
  }, [graphAvailable, dialogMode, multimodalIssue, asrIssue]);

  const sectionNavItems: UploadSectionNavItem[] = useMemo(() => {
    const statusInput = {
      state: confirmState,
      selectedTagCount: pendingTagIds.length,
      hasPdf,
      hasImages,
      hasAudio,
      vllmModelName: modelName(confirmState.vllmModelId),
      asrModelName: modelName(confirmState.asrModelId),
    };
    const toItem = (key: UploadConfirmSectionKey, label: string, issue = false): UploadSectionNavItem => {
      const status = uploadSectionStatus(key, statusInput);
      const text = status ? (status.text ?? ct(status.key, status.values) + (status.suffixKey ? ` · ${ct(status.suffixKey)}` : "")) : "";
      return {
        key,
        label,
        status: text,
        statusTitle: text,
        tone: status?.tone,
        issue,
      };
    };
    const items: UploadSectionNavItem[] = [];
    if (dialogMode !== "reparse") items.push(toItem("tags", ct("uploadConfirm.tabTags")));
    items.push(toItem("parser", ct("settings.parserEngine")));
    items.push(toItem("chunking", t("knowledgeEditor.sidebar.chunking")));
    items.push(toItem("multimodal", t("knowledgeEditor.sidebar.multimodal"), multimodalIssue));
    items.push(toItem("asr", t("knowledgeEditor.sidebar.asr"), asrIssue));
    items.push(toItem("question", t("knowledgeEditor.advanced.questionGeneration.label")));
    // Vue pushes graph last, only when isGraphSectionAvailable (L940-942).
    if (graphAvailable) {
      const graphItem = toItem("graph", t("knowledgeEditor.sidebar.graph"));
      items.push({ ...graphItem, icon: "chart-bubble" });
    }
    return items;
  }, [confirmState, pendingTagIds.length, hasPdf, hasImages, hasAudio, tenantModels, dialogMode, ct, t, multimodalIssue, asrIssue, graphAvailable]);

  const dialogTitle = dialogMode === "manual"
    ? ct("uploadConfirm.titleManual")
    : dialogMode === "reparse"
      ? ct("uploadConfirm.titleReparse")
      : ct("uploadConfirm.title");
  const confirmButtonText = dialogMode === "manual"
    ? ct("uploadConfirm.confirmManual")
    : dialogMode === "reparse"
      ? ct("uploadConfirm.confirmReparse")
      : ct("uploadConfirm.confirm");
  const uploadDialogOpen = canContribute && (pendingEntries.length > 0 || pendingUrls.length > 0 || !!pendingManual || !!pendingReparse);
  const rootRowLabel = t("knowledgeBase.folderTree.rootRow");
  const filesPanelLabels: UploadFilesPanelLabels = {
    urlItemLabel: ct("uploadConfirm.urlItemLabel"),
    remove: ct("common.remove"),
    noItems: ct("uploadConfirm.noItems"),
    manualCharCount: (count) => ct("uploadConfirm.manualCharCount", { count }),
    reparseSource: ct("uploadConfirm.reparseSource"),
    reparseHint: ct("uploadConfirm.reparseHint"),
    statusLabel: (status) => status === "done"
      ? t("knowledgeBase.timeline.done")
      : status === "uploading"
        ? t("knowledgeBase.timeline.running")
        : status === "error"
          ? t("knowledgeBase.timeline.failed")
          : t("knowledgeBase.timeline.pending"),
  };

  return (
    <div className="knowledge-layout">
      {stageNotice ? <div className={`${stageNoticeClass(stageNotice.tone)}wk-stage-notice`} role="alert" aria-live="polite">{stageNotice.text}</div> : null}
      <div className="document-header">
        <div className="document-header-title">
          <DocumentsBreadcrumb
            t={t}
            knowledgeBaseId={knowledgeBaseId}
            kbName={typeof kbMeta?.name === "string" ? kbMeta.name : null}
            kbList={kbList}
            kbMeta={{
              type: typeof kbMeta?.type === "string" ? kbMeta.type : undefined,
              description: typeof kbMeta?.description === "string" ? kbMeta.description : undefined,
              createdAt: typeof kbMeta?.created_at === "string" ? kbMeta.created_at.slice(0, 10) : undefined,
            }}
            supportedFileTypes={[...supportedFileTypes]}
            canManage={canContribute}
            onOpenSettings={() => setKbSettingsOpen(true)}
            tabs={breadcrumbTabs}
          />
          <p className="document-subtitle">{t("knowledgeEditor.document.subtitle")}</p>
          {kbMetaError ? (
            <div className="wk-kb-meta-error" role="alert">
              <Status tone="error">{kbMetaError.kind === "forbidden" ? `${kbMetaError.message} (403)` : kbMetaError.message}</Status>
              <Button type="button" variant="text" onClick={() => setKbMetaAttempt((attempt) => attempt + 1)}>
                {t("common.retry")}
              </Button>
            </div>
          ) : null}
          <ParserHint
            t={t}
            types={unsupportedFileTypes}
            onConfigure={() => navigate(documentsKBSettingsPath(knowledgeBaseId))}
          />
          {storageEngineMissing ? (
            <p className="storage-engine-warning" onClick={() => navigate(documentsKBSettingsPath(knowledgeBaseId))}>
              <TIcon name="info-circle" className="warning-icon" />
              <span>{t('knowledgeBase.missingStorageEngine')}</span>
              <span className="warning-link">{t('knowledgeBase.goToStorageSettings')} →</span>
            </p>
          ) : null}
        </div>
        {/* Vue's document header (KnowledgeBase.vue:2392-2472) has no
            top-right actions: the tab row lives in the breadcrumb and there
            is no manual reload button — uploads/uploads-in-progress refresh
            the lists through their own watchers. */}
      </div>
      <div className="knowledge-main" data-drag-active={dragActive && canContribute ? "true" : undefined}
            onDragOver={
              canContribute
                ? (event) => {
                    event.preventDefault();
                    setDragActive(true);
                  }
                : undefined
            }
            onDragLeave={canContribute ? () => setDragActive(false) : undefined}
            onDrop={
              canContribute
                ? (event) => {
                    event.preventDefault();
                    setDragActive(false);
                    stageFiles(event.dataTransfer.files);
                  }
                : undefined
            }
      >
        {uploadError && !uploadDialogOpen ? <Status tone="error">{uploadError}</Status> : null}
          {showFolderTree ? <aside className="wk-folder-panel wk-kd-105">
            {/* Vue folderTree title (目录), not documents.folders (文件夹). */}
            <strong>{t("knowledgeBase.folderTree.title")}</strong>
            {folderState.status === "loading" ? (
              <Status>{t("knowledgeBase.documents.loadingFolders")}</Status>
            ) : null}
            {folderState.status === "error" ? (
              <Status tone="error">{folderState.message}</Status>
            ) : null}
            <ul className="wk-folder-list wk-kd-106">
              {folders.map((folder) => (
                <li
                  key={folder.path}
                  style={{ paddingLeft: `${folder.depth * 0.8}rem` }}
                >
                  <button
                    type="button"
                    className={
                      folderPath === (folder.path || undefined)
                        ? "is-active wk-kd-164"
                        : "wk-kd-165"
                    }
                    onClick={() => setFolderPath(folder.path || undefined)}
                  >
                    {folder.path === "" ? rootRowLabel : folder.name} <span className="wk-kd-107">{folder.total_count}</span>
                  </button>
                </li>
              ))}
            </ul>
          </aside> : null}
          {uploading && canContribute ? (
            <UploadProgressMask
              percent={batchUploadProgress(uploadStates)}
              title={t("file.upload")}
              formats={[t("knowledgeBase.pdfDocFormat"), t("knowledgeBase.textMarkdownFormat")]}
            />
          ) : null}
          <div className="tag-content">
            <div className="doc-card-area">
            {showFolderTree ? <nav className="doc-folder-path" aria-label={t("knowledgeBase.folderTree.title")}>
              <button type="button" className="doc-folder-path__crumb is-current" onClick={() => setFolderPath(undefined)}>
                {t("knowledgeBase.folderTree.rootRow")}
              </button>
              {folderPathCrumbs(folderPath).map((crumb, index, crumbs) => <span key={crumb.path} className="wk-kd-108">
                <TIcon name="chevron-right" className="doc-folder-path__sep" />
                {index === crumbs.length - 1 ? <span className="doc-folder-path__crumb is-current">{crumb.name}</span> : <button type="button" className="doc-folder-path__crumb" onClick={() => setFolderPath(crumb.path)}>{crumb.name}</button>}
              </span>)}
              {filtering ? <span className="doc-folder-path__scope">({t("knowledgeBase.folderTree.searchingSubtree")})</span> : null}
            </nav> : null}
            <div className="doc-filter-bar">
            <div className="doc-search-input">
                <TdInput
                  className="doc-search-field"
                  value={query}
                  onChange={(value: unknown) => setQuery(String(value ?? ""))}
                  onEnter={() => setDebouncedQuery(query)}
                  clearable
                  placeholder={t("knowledgeBase.docSearchPlaceholder")}
                  aria-label={t("knowledgeBase.docSearchPlaceholder")}
                  prefixIcon={<TIcon name="search" size="16px" />}
                />
              </div>
              <div className="doc-filter-bar__filters">
                <div className="doc-filter-field">
                  <Popup
                    visible={tagFilterOpen}
                    trigger="click"
                    placement="bottom-left"
                    overlayClassName="tag-filter-popup"
                    overlayInnerStyle={{ padding: 0 }}
                    onVisibleChange={setTagFilterOpen}
                    content={(
                      <TagFilterPanel
                        t={tt}
                        tags={tags}
                        selectedIds={selectedTagIds}
                        onToggle={toggleTagFilter}
                        onClear={clearTagFilter}
                        onClose={() => setTagFilterOpen(false)}
                        searchQuery={tagSearchQuery}
                        onSearch={setTagSearchQuery}
                        hasMore={tagHasMore}
                        loadingMore={tagLoadingMore}
                        onLoadMore={() => setTagPage((pageNumber) => pageNumber + 1)}
                        total={tagTotal}
                        canManage={canContribute}
                        onManage={() => {
                          // Vue openTagManageDrawer closes the filter panel first.
                          setTagFilterOpen(false);
                          setTagManageOpen(true);
                        }}
                      />
                    )}
                  >
                    <div className="doc-filter-field">
                      <button
                        type="button"
                        className={'doc-tag-filter-trigger doc-filter-field__control' + (tagFilterOpen ? ' open' : '')}
                        aria-label={t("knowledgeBase.tagFilterTitle")}
                        title={tagTriggerTitle}
                      >
                        <span className="doc-tag-filter-trigger__prefix" aria-hidden="true">
                          <TIcon name="discount" size="16px" />
                        </span>
                        <span className="doc-tag-filter-trigger__label">{tagTriggerLabel}</span>
                        <span className="doc-tag-filter-trigger__suffix">
                          {selectedTagIds.length ? (
                            <span
                              className="t-input__suffix t-input__suffix-icon t-input__clear"
                              aria-label={t("common.clear")}
                              role="button"
                              tabIndex={0}
                              onClick={(event) => { event.stopPropagation(); clearTagFilter(); }}
                              onMouseDown={(event) => event.stopPropagation()}
                            >
                              <TIcon name="close-circle-filled" className="t-input__suffix-clear" />
                            </span>
                          ) : (
                            <TIcon
                              name="chevron-down"
                              size="16px"
                              className={'doc-tag-filter-trigger__caret' + (tagFilterOpen ? ' open' : '')}
                            />
                          )}
                        </span>
                      </button>
                    </div>
                  </Popup>
                </div>
                <div className="doc-filter-field">
                  <TdSelect
                    className="doc-type-select doc-filter-field__control"
                    value={fileType}
                    onChange={(value) => setFileType(String(value ?? ""))}
                    placeholder={t("knowledgeBase.fileTypeFilter")}
                    clearable
                    prefixIcon={<TIcon name="file" size="16px" />}
                    options={[
                      { label: t("knowledgeBase.allFileTypes"), value: "" },
                      ...DOCUMENT_FILE_TYPE_OPTIONS.map((option) => ({ label: option.labelKey ? t(option.labelKey) : (option.label ?? option.value), value: option.value })),
                    ]}
                  />
                </div>
                <div className="doc-filter-field">
                  <TdSelect
                    className="doc-type-select doc-filter-field__control"
                    value={parseStatus}
                    onChange={(value) => setParseStatus(String(value ?? ""))}
                    placeholder={t("knowledgeBase.parseStatusFilter")}
                    clearable
                    prefixIcon={<TIcon name="check-circle" size="16px" />}
                    options={[
                      { label: t("knowledgeBase.allParseStatuses"), value: "" },
                      ...DOCUMENT_PARSE_STATUS_OPTIONS.map((option) => ({ label: t(option.labelKey ?? option.value), value: option.value })),
                    ]}
                  />
                </div>
                <div className="doc-filter-field">
                  <TdSelect
                    className="doc-type-select doc-filter-field__control"
                    value={source}
                    onChange={(value) => setSource(String(value ?? ""))}
                    placeholder={t("knowledgeBase.sourceFilter")}
                    clearable
                    prefixIcon={<TIcon name="link" size="16px" />}
                    options={[
                      { label: t("knowledgeBase.allSources"), value: "" },
                      ...DOCUMENT_SOURCE_OPTIONS.map((option) => ({ label: t(option.labelKey ?? option.value), value: option.value })),
                    ]}
                  />
                </div>
                <div className="doc-filter-field doc-filter-field--wide">
                  <DateRangePicker
                    className="doc-date-range doc-filter-field__control"
                    value={[updatedFrom || "", updatedTo || ""]}
                    onChange={(value) => {
                      setUpdatedFrom(Array.isArray(value) && typeof value[0] === "string" ? value[0] : "");
                      setUpdatedTo(Array.isArray(value) && typeof value[1] === "string" ? value[1] : "");
                    }}
                    placeholder={[t("knowledgeBase.updatedTimeFrom"), t("knowledgeBase.updatedTimeTo")]}
                    clearable
                    allowInput
                    prefixIcon={<TIcon name="time" size="16px" />}
                    disableDate={{ after: new Date(new Date().setHours(23, 59, 59, 999)).toISOString() }}
                  />
                </div>
              </div>
              <div className="doc-filter-bar__trailing">
                {/* Vue KnowledgeBase.vue L2659: standalone 批量管理 toggle in the
                    trailing filter bar — visible whenever there is content and the
                    viewer may download or mutate; entering/exiting clears nothing
                    on entry and clears the selection on exit. */}
                {(canDownload || canContribute) && items.length ? (
                  <TdButton
                    variant="outline"
                    size="small"
                    disabled={batchDownloading}
                    onClick={() => { setBatchMode((value) => !value); if (batchMode) setSelected(new Set()); }}
                  >
                    {t(batchMode ? "knowledgeBase.clearSelection" : "menu.batchManage")}
                  </TdButton>
                ) : null}
                <div className="doc-view-toggle" role="group" aria-label={t("knowledgeBase.viewModeToggle")}>
                  <Tooltip content={t("knowledgeBase.viewModeGrid")} placement="top">
                    <button type="button" className={'doc-view-toggle-btn' + (viewMode === "grid" ? ' active' : '')} aria-pressed={viewMode === "grid"} onClick={() => setViewMode("grid")}><TIcon name="view-module" size="16px" /></button>
                  </Tooltip>
                  <Tooltip content={t("knowledgeBase.viewModeList")} placement="top">
                    <button type="button" className={'doc-view-toggle-btn' + (viewMode === "list" ? ' active' : '')} aria-pressed={viewMode === "list"} onClick={() => setViewMode("list")}><TIcon name="view-list" size="16px" /></button>
                  </Tooltip>
                </div>
                {canContribute ? (
                  <div className="doc-filter-actions">
                    <UploadSourceDropdown
                      tooltip={t("knowledgeBase.addDocument")}
                      guideTarget="kb-detail-add-doc"
                      items={[
                        { key: "file", label: ct("upload.uploadDocument") },
                        { key: "folder", label: ct("upload.uploadFolder") },
                        { key: "url", label: t("knowledgeBase.importURL") },
                        { key: "manual", label: t("upload.onlineEdit") },
                      ]}
                      open={pageSourceMenuOpen}
                      onToggle={() => setPageSourceMenuOpen((open) => !open)}
                      onFiles={(files) => stageFiles(files)}
                      onSelect={(key) => {
                        setPageSourceMenuOpen(false);
                        if ((key === "url" || key === "manual") && !ensureDocumentKbReady()) return;
                        if (key === "url") {
                          setSourceUrlValue("");
                          setSourceUrlDialogOpen(true);
                        }
                        if (key === "manual") {
                          setManualTitle("");
                          setManualContent("");
                          setUploadError(null);
                          setManualDialogOpen(true);
                        }
                      }}
                    />
                  </div>
                ) : null}
              </div>
            </div>
            {/* Vue DocumentBatchBar (DocumentBatchBar.vue) + .doc-batch-bar-anchor
                (KnowledgeBase.vue:2779/3550): the batch toolbar floats at the
                bottom of the list area (centered, max-width 920px white card) —
                NOT an inline block above the results. Left: 已选 N 项 +
                全选已加载/取消选择 text buttons; right: 批量下载(primary) /
                重建知识 / 批量打标签 / 移动到目录 / 批量删除. */}
            <div
              className="doc-batch-bar-anchor"
              role="region"
              aria-label={t("knowledgeBase.selectedCount", { count: selected.size })}
              style={{ display: (canDownload || canContribute) && (batchMode || selected.size > 0) ? undefined : "none" }}
            >
              {(canDownload || canContribute) && (batchMode || selected.size > 0) ? (
                <div className="doc-batch-bar">
                  <div className="batch-bar-inner">
                    <div className="batch-bar-left">
                      <span className="batch-bar-count">
                        {t("knowledgeBase.selectedCount", { count: selected.size })}
                      </span>
                      <TdButton variant="text" theme="default" size="small" className="batch-bar-clear" disabled={batchDownloading} onClick={toggleAllOnPage}>
                        {/* Vue knowledgeBase.selectLoaded (zh-CN.ts:6750 全选已加载);
                            the key is absent from the React catalog so the literal
                            travels with the Vue authority comment (same precedent
                            as the literal "Wiki" tab). */}
                        全选已加载
                      </TdButton>
                      <TdButton variant="text" theme="default" size="small" className="batch-bar-clear" disabled={batchDownloading} onClick={() => { setSelected(new Set()); setBatchMode(false); }}>
                        {t("knowledgeBase.clearSelection")}
                      </TdButton>
                    </div>
                    <div className="batch-bar-actions">
                      {canDownload ? (
                        <TdButton
                          theme="primary"
                          size="small"
                          loading={batchDownloading}
                          disabled={selected.size === 0 || selected.size > 200 || batchDownloading}
                          icon={<TIcon name="download" size="14px" />}
                          onClick={() => void handleBatchDownload()}
                        >
                          {t(batchDownloading ? "knowledgeBase.batchDownloading" : "knowledgeBase.batchDownload")}
                        </TdButton>
                      ) : null}
                      {canContribute ? (
                        <>
                          <Popconfirm
                            theme="warning"
                            content={t("knowledgeBase.confirmBatchReparseDocument", { count: selected.size })}
                            confirmBtn={{ content: t("knowledgeBase.confirmBatchReparse"), theme: "warning" }}
                            cancelBtn={{ content: t("common.cancel") }}
                            placement="top"
                            onConfirm={() => void reparseSelected()}
                          >
                            <TdButton theme="default" variant="outline" size="small" disabled={selected.size === 0 || batchDownloading} icon={<TIcon name="refresh" size="14px" />}>
                              {t("knowledgeBase.rebuildDocument")}
                            </TdButton>
                          </Popconfirm>
                          {/* Vue DocumentBatchBar 批量打标签 → BatchTagDialog (L2164-2167). */}
                          <TdButton
                            theme="default"
                            variant="outline"
                            size="small"
                            disabled={selected.size === 0 || batchDownloading}
                            icon={<TIcon name="discount" size="14px" />}
                            onClick={() => setTagDialog({ mode: "batch" })}
                          >
                            {tt("knowledgeBase.batchTag")}
                          </TdButton>
                          <TdButton
                            theme="default"
                            variant="outline"
                            size="small"
                            disabled={selected.size === 0 || batchDownloading}
                            icon={<TIcon name="folder" size="14px" />}
                            onClick={() => {
                              setMoving(true);
                              setMoveTarget(folderPath ?? "");
                            }}
                          >
                            {t("knowledgeBase.moveToFolder.action")}
                          </TdButton>
                          <Popconfirm
                            theme="warning"
                            content={t("knowledgeBase.confirmBatchDeleteDocument", { count: selected.size })}
                            confirmBtn={{ content: t("knowledgeBase.confirmDelete"), theme: "danger" }}
                            cancelBtn={{ content: t("common.cancel") }}
                            placement="top"
                            onConfirm={() => setConfirmingDelete(true)}
                          >
                            <TdButton theme="danger" variant="outline" size="small" disabled={selected.size === 0 || batchDownloading} icon={<TIcon name="delete" size="14px" />}>
                              {t("knowledgeBase.batchDelete")}
                            </TdButton>
                          </Popconfirm>
                        </>
                      ) : null}
                    </div>
                  </div>
                </div>
              ) : null}
            </div>
            {moving && canContribute ? (
              <div
                className="wk-list-actions wk-kd-109"
                role="form"
                aria-label={t("knowledgeBase.documents.moveDestination")}
              >
                <label>
                  {t("knowledgeBase.documents.moveDestination")}{" "}
                  <Select
                    value={moveTarget}
                    onChange={(event) => setMoveTarget(event.target.value)} className="wk-kd-110"
                  >
                    <option value="">
                      {t("knowledgeBase.documents.moveRoot")}
                    </option>
                    {folders
                      .filter((folder) => folder.path)
                      .map((folder) => (
                        <option key={folder.path} value={folder.path}>
                          {folder.name}
                        </option>
                      ))}
                  </Select>
                </label>
                <Button
                  type="button"
                  disabled={!selected.size}
                  onClick={() => void moveSelected()}
                >
                  {t("knowledgeBase.documents.moveConfirm")}
                </Button>
                <Button
                  type="button"
                  onClick={() => {
                    setMoving(false);
                    setMoveTarget("");
                  }}
                >
                  {t("knowledgeBase.documents.moveCancel")}
                </Button>
              </div>
            ) : null}
            {mutationError ? (
              <Status tone="error">{mutationError}</Status>
            ) : null}
            {actionNotice ? <Status tone={actionNotice.tone}>{actionNotice.text}</Status> : null}
            {state.status === "loading" ? (
              <Status>{t("knowledgeBase.documents.loadingDocuments")}</Status>
            ) : null}
            {state.status === "error" ? (
              <>
                <Status tone="error">{state.message}</Status>
                <Button
                  type="button"
                  onClick={() => setReloadToken((value) => value + 1)}
                >
                  {t("knowledgeBase.documents.tryAgain")}
                </Button>
              </>
            ) : null}
            <div
              ref={documentListRef}
              className={
                'doc-scroll-container' +
                (!items.length && !folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? "")).length && state.status === "success" ? ' is-empty' : '') +
                (marquee.visible ? ' is-marquee-active' : '')
              }
              onMouseDown={marquee.onMouseDown}
            >
              {marquee.visible ? (
                <div
                  className={'doc-marquee-box is-' + marquee.mode}
                  style={{ left: marquee.left, top: marquee.top, width: marquee.width, height: marquee.height }}
                  aria-hidden="true"
                />
              ) : null}
              {/* 文档骨架屏（Vue docListLoading && 空 && 无子目录） */}
              {state.status === "loading" && items.length === 0 && !folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? "")).length ? (
                <div className="doc-card-list doc-card-list-animated">
                  {Array.from({ length: 8 }, (_, n) => (
                    <div key={`doc-skel-${n}`} className="knowledge-card knowledge-card-skeleton">
                      <div className="card-content">
                        <div className="card-content-nav">
                          <TdSkeleton animation="gradient" rowCol={[{ width: '70%', height: '18px' }]} />
                        </div>
                        <TdSkeleton animation="gradient" rowCol={[{ width: '100%', height: '14px' }, { width: '60%', height: '14px' }]} />
                      </div>
                      <div className="card-bottom">
                        <TdSkeleton animation="gradient" rowCol={[[{ width: '80px', height: '14px' }, { width: '40px', height: '18px', type: 'rect' }]]} />
                      </div>
                    </div>
                  ))}
                </div>
              ) : state.status === "success" && viewMode === "grid" && hasDocumentGridContent(items, folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? ""))) ? (
                <DocumentCardGrid
                  items={items}
                  folders={folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? ""))}
                  selected={selected}
                  batchMode={batchMode}
                  canContribute={canContribute}
                  canMutateKnowledge={canMutate}
                  canDownload={canDownload}
                  t={t}
                  tagListCount={tags.length}
                  traceAvailableById={traceAvailableById}
                  onProbeTrace={probeTraceAvailability}
                  moveFor={moveControllerFor}
                  loadTrace={loadHoverTrace}
                  onOpen={openDocumentDetail}
                  onOpenFolder={(path) => setFolderPath(path || undefined)}
                  onToggle={(id, checked) => setSelected((current) => { const next = new Set(current); if (checked) next.add(id); else next.delete(id); return next; })}
                  onTagEdit={(document) => setTagDialog({ mode: "single", document })}
                  onReparse={(document) => reparseOne(document)}
                  onCancelParse={(document) => void cancelOneParse(document.id)}
                  onDownload={(document) => void downloadDocument(document)}
                  onEdit={(document) => void openManualEdit(document)}
                  onViewTrace={(document) => openTrace(document)}
                  onMove={(document) => { setBatchMode(true); setSelected(new Set([document.id])); setMoving(true); setMoveTarget(document.folder_path ?? ""); }}
                  onBatchManage={() => setBatchMode(true)}
                  onDelete={setConfirmingDeleteDocument}
                />
              ) : state.status === "success" && items.length > 0 && viewMode === "list" ? (
                <DocumentListRows
                  items={items}
                  folders={folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? ""))}
                  showFolderTree={showFolderTree}
                  selected={selected}
                  canContribute={canContribute}
                  canMutateKnowledge={canMutate}
                  canDownload={canDownload}
                  t={t}
                  tt={tt}
                  allOnPageSelected={allOnPageSelected}
                  someOnPageSelected={selected.size > 0 && !allOnPageSelected}
                  traceAvailableById={traceAvailableById}
                  moveFor={moveControllerFor}
                  onOpen={openDocumentDetail}
                  onOpenFolder={(path) => setFolderPath(path || undefined)}
                  onToggleRow={toggleSelected}
                  onToggleAll={toggleAllOnPage}
                  onProbeTrace={probeTraceAvailability}
                  onTagEdit={(document) => setTagDialog({ mode: "single", document })}
                  onReparse={(document) => reparseOne(document)}
                  onCancelParse={(document) => void cancelOneParse(document.id)}
                  onDownload={(document) => void downloadDocument(document)}
                  onEdit={(document) => void openManualEdit(document)}
                  onViewTrace={(document) => openTrace(document)}
                  onMove={(document) => { setBatchMode(true); setSelected(new Set([document.id])); setMoving(true); }}
                  onBatchManage={() => setBatchMode(true)}
                  onDelete={setConfirmingDeleteDocument}
                />
              ) : state.status === "success" ? (
                folderPath !== undefined || filtering ? (
                  <div className="doc-empty-state">
                    <p className="doc-empty-folder">
                      {filtering ? t("knowledgeBase.folderTree.emptySearch") : t("knowledgeBase.folderTree.emptyFolder")}
                    </p>
                  </div>
                ) : (
                  <DocumentEmptyState t={t} variant="illustration" />
                )
              ) : null}
              {state.status === "success" && pageTotal > pageSize ? (
                <nav
                  className="wk-pagination"
                  aria-label={t("knowledgeBase.documents.title")}
                >
                  <Button
                    type="button"
                    disabled={page <= 1}
                    onClick={() => setPage((value) => value - 1)}
                  >
                    {t("knowledgeBase.documents.previous")}
                  </Button>
                  <span>
                    {t("knowledgeBase.documents.page", {
                      page,
                      total: pageTotal,
                    })}
                  </span>
                  <Button
                    type="button"
                    disabled={page * pageSize >= pageTotal}
                    onClick={() => setPage((value) => value + 1)}
                  >
                    {t("knowledgeBase.documents.next")}
                  </Button>
                </nav>
              ) : null}
            </div>
          </div>
        </div>
      </div>
      {/* Vue DocContent 抽屉宿主（关闭态 0 高度，但作为 knowledge-layout 的尾随
          子项参与 gap:20px 布局——占位使 main 区高度与 Vue 一致）。 */}
      <div className="doc_content" />
      <>
        <Dialog
          open={uploadDialogOpen}
          title={dialogTitle}
          className="wk-upload-confirm-dialog"
          closeLabel={ct("common.close")}
          onClose={closeUploadConfirmDialog}
        >
          <div className="wk-upload-confirm-layout">
            <aside className="wk-upload-confirm-files-column">
              <div className="wk-upload-confirm-files-header">
                <div className="wk-upload-confirm-files-header-row">
                  <h2 className="wk-kd-111">{dialogTitle}</h2>
                  {dialogMode === "file" ? (
                    <div className="wk-upload-confirm-files-header-actions">
              <span className="wk-files-count" aria-label={ct("uploadConfirm.parseConfig")} style={{ minWidth: "1.4rem", textAlign: "center", borderRadius: "999px", padding: "0 0.35rem", border: "1px solid var(--wk-border, #e4e7ec)", fontSize: "0.85rem" }}>
                {batchItemCount}
              </span>
              <UploadSourceDropdown
                tooltip={ct("uploadConfirm.continueAdd")}
                items={[
                  { key: "file", label: ct("upload.uploadDocument") },
                  { key: "folder", label: ct("upload.uploadFolder") },
                  { key: "url", label: t("knowledgeBase.importURL") },
                ]}
                open={sourceMenuOpen}
                onToggle={() => setSourceMenuOpen((open) => !open)}
                onFiles={(files) => stageFiles(files)}
                onSelect={(key) => {
                  setSourceMenuOpen(false);
                  if (key === "url") {
                    setSourceUrlValue("");
                    setSourceUrlDialogOpen(true);
                  }
                }}
              />
                    </div>
                  ) : null}
                </div>
          {dialogMode === "file" ? (
            <fieldset className="wk-upload-confirm-destination" style={{ position: "relative", marginBottom: "0.75rem" }}>
              <legend>{ct("uploadConfirm.destinationLabel")}</legend>
              <button
                type="button"
                title={uploadTargetFolder || rootRowLabel}
                aria-label={ct("uploadConfirm.destinationChange")}
                aria-expanded={destinationPickerOpen}
                onClick={() => {
                  setDestinationPickerOpen((open) => !open);
                  setCreatingUnder(null);
                  setNewFolderName("");
                  setDestinationPickerDuplicate(false);
                }}
                className="wk-destination-crumb wk-kd-ring wk-kd-112"
              >
                <span className="wk-kd-15">{ct("uploadConfirm.destinationLabel")}</span>
                <span className="wk-kd-113">{destinationBreadcrumb(uploadTargetFolder, rootRowLabel)}</span>
                <span className="wk-kd-114" aria-hidden>{destinationPickerOpen ? "▾" : "▸"}</span>
              </button>
              {destinationPickerOpen ? (
                <div
                  className="wk-destination-popup"
                  style={{ position: "absolute", zIndex: 30, marginTop: "4px", padding: "6px", border: "1px solid var(--wk-border, #e4e7ec)", borderRadius: "8px", background: "var(--wk-surface, #fff)", boxShadow: "0 8px 24px rgba(0,0,0,0.12)" }}
                  role="group"
                  aria-label={ct("uploadConfirm.destinationChange")}
                >
                  <UploadDestinationPicker
                    options={pickerFolderOptions}
                    currentPath={uploadTargetFolder}
                    creatingUnder={creatingUnder}
                    newFolderName={newFolderName}
                    duplicateWarning={destinationPickerDuplicate}
                    labels={{
                      pickerLabel: ct("uploadConfirm.destinationChange"),
                      rootRow: rootRowLabel,
                      newFolderPlaceholder: t("knowledgeBase.moveToFolder.newFolderPlaceholder"),
                      newFolderAddRoot: t("knowledgeBase.moveToFolder.newFolderAddRoot"),
                      newFolderAddUnder: (folder) => t("knowledgeBase.moveToFolder.newFolderAddUnder", { folder }),
                      duplicate: t("knowledgeBase.moveToFolder.duplicate"),
                    }}
                    onChoose={choosePickerFolder}
                    onStartCreate={startCreatingUnder}
                    onCancelCreate={() => {
                      setCreatingUnder(null);
                      setNewFolderName("");
                    }}
                    onNewFolderNameChange={(value) => {
                      setNewFolderName(value);
                      setDestinationPickerDuplicate(false);
                    }}
                    onCommitNewFolder={commitPickerFolder}
                  />
                </div>
              ) : null}
            </fieldset>
          ) : null}
              </div>
          <UploadFilesPanel
            mode={dialogMode}
            entries={pendingEntries}
            urls={pendingUrls}
            uploadStates={uploadStates}
            manualTitle={pendingManual?.title}
            manualCharCount={pendingManual?.content.length}
            reparseFileName={pendingReparse ? displayName(pendingReparse.document) : undefined}
            uploading={uploading}
            labels={filesPanelLabels}
            onRemoveUrl={removeStagedUrl}
            onRemoveEntry={removeStagedUpload}
          />
            </aside>
            <aside className="wk-upload-confirm-settings-column">
              <div className="wk-upload-confirm-section-nav">
          <UploadSectionNav
            items={sectionNavItems.map((item) => ({ ...item, active: activeSection === item.key }))}
            navLabel={ct("uploadConfirm.configNav")}
            onSelect={goToSection}
          />
              </div>
              <main className="wk-upload-confirm-config-panel">
          {dialogMode !== "reparse" ? (
            <fieldset className="wk-upload-confirm-tags wk-kd-115" id="wk-upload-section-tags" data-section="tags" style={{ display: activeSection === "tags" ? undefined : "none" }}>
              <legend>{ct("uploadConfirm.tabTags")}</legend>
              <p className="wk-muted wk-kd-16" style={{ margin: "0 0 0.4rem", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsDescription")}</p>
              <label>
                <span className="wk-visually-hidden wk-kd-53">{ct("uploadConfirm.tagsPlaceholder")}</span>
                <UploadMultiSelect
                  values={pendingTagIds}
                  options={tags.map((tag) => ({ value: tag.id, label: tag.name }))}
                  ariaLabel={ct("uploadConfirm.tagsPlaceholder")}
                  onChange={setPendingTagIds}
                />
              </label>
              {!uploading && tags.length === 0 ? (
                <p className="wk-muted wk-kd-16" style={{ margin: "0.25rem 0 0", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsEmpty")}</p>
              ) : null}
            </fieldset>
          ) : null}
          <UploadConfirmSections
            state={confirmState}
            update={updateConfirm}
            hasPdf={hasPdf}
            multimodalIssue={multimodalIssue}
            asrIssue={asrIssue}
            parserEngines={parserEngines}
            parserLoading={parserEnginesLoading}
            onConfigureParserSettings={() => navigate(documentsKBSettingsPath(knowledgeBaseId))}
            vllmModels={vllmModels}
            asrModels={asrModels}
            moreOpen={chunkingMoreOpen}
            onToggleMore={() => setChunkingMoreOpen((open) => !open)}
            activeSection={activeSection}
            graphAvailable={graphAvailable}
            graphSettings={
              <UploadGraphSettings
                graphExtract={confirmState.nodeExtract}
                graphDatabaseOn={graphDatabaseOn}
                llmModelId={llmModelId}
                canRunExtract={graphAdmin}
                runExtractAction={runGraphExtractAction}
                onNotify={showStageNotice}
                onChange={updateNodeExtract}
                t={ct}
              />
            }
            t={ct}
          />
              </main>
            </aside>
          </div>
          {uploadError ? <Status tone="error">{uploadError}</Status> : null}
          <div className="wk-upload-confirm-footer wk-list-actions wk-kd-116">
            <Button
              type="button"
              disabled={!canCloseUploadConfirmDialog(uploading)}
              onClick={closeUploadConfirmDialog}
            >
              {ct("uploadConfirm.cancel")}
            </Button>
            <Button
              type="button"
              loading={uploading}
              disabled={!canConfirm && !uploading}
              onClick={() => void confirmUpload()}
            >
              {confirmButtonText}
            </Button>
          </div>
      </Dialog>
      </>
      {/* Vue opens URL import from the page-level source menu too; this must
          not be nested under uploadDialogOpen, which is only true after files
          have already been staged. */}
      {sourceUrlDialogOpen ? (
        <Dialog
          open
          title={t("knowledgeBase.importURLTitle")}
          onClose={() => setSourceUrlDialogOpen(false)}
        >
          <div className="wk-upload-url-dialog wk-kd-117">
            <label>
              {t("knowledgeBase.urlLabel")} {" "}
              <Input
                autoFocus
                className="wk-kd-58"
                value={sourceUrlValue}
                placeholder={t("knowledgeBase.urlPlaceholder")}
                onChange={(event) => setSourceUrlValue(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    if (appendStagedUrl(sourceUrlValue, "dialog")) {
                      setSourceUrlDialogOpen(false);
                      setSourceUrlValue("");
                    }
                  }
                }}
              />
            </label>
            <p className="wk-muted wk-kd-16" style={{ margin: "0.25rem 0 0", fontSize: "0.85rem" }}>{t("knowledgeBase.urlTip")}</p>
            <div className="wk-list-actions wk-kd-109">
              <Button
                type="button"
                onClick={() => {
                  if (appendStagedUrl(sourceUrlValue, "dialog")) {
                    setSourceUrlDialogOpen(false);
                    setSourceUrlValue("");
                  }
                }}
              >
                {ct("common.confirm")}
              </Button>
              <Button type="button" onClick={() => setSourceUrlDialogOpen(false)}>
                {ct("uploadConfirm.cancel")}
              </Button>
            </div>
          </div>
        </Dialog>
      ) : null}
      {manualDialogOpen && canContribute ? (
        <Dialog
          open
          title={t("knowledgeBase.documents.createDocument")}
          onClose={() => setManualDialogOpen(false)}
        >
          <div className="wk-upload-url-dialog wk-kd-117">
            <label>
              {t("knowledgeBase.documents.manualTitle")}{" "}
              <Input
                autoFocus
                className="wk-kd-58"
                value={manualTitle}
                onChange={(event) => setManualTitle(event.target.value)}
              />
            </label>
            <label>
              {t("knowledgeBase.documents.manualContent")}{" "}
              <Textarea
                value={manualContent}
                onChange={(event) => setManualContent(event.target.value)}
                rows={6}
              />
            </label>
            {uploadError ? <Status tone="error">{uploadError}</Status> : null}
            <div className="wk-list-actions wk-kd-109">
              <Button type="button" onClick={stageManualCreate}>
                {ct("common.confirm")}
              </Button>
              <Button type="button" onClick={() => setManualDialogOpen(false)}>
                {ct("uploadConfirm.cancel")}
              </Button>
            </div>
          </div>
        </Dialog>
      ) : null}
      {manualEditDocument && canContribute ? (
        <Dialog
          open
          title={t("knowledgeBase.editDocument")}
          onClose={() => { if (!manualEditSaving) setManualEditDocument(null); }}
        >
          <div className="wk-upload-url-dialog wk-kd-117">
            {manualEditLoading ? <Status>{t("common.loading")}</Status> : null}
            <label>
              {t("knowledgeBase.documents.manualTitle")} {" "}
              <Input
                autoFocus
                className="wk-kd-58"
                value={manualTitle}
                disabled={manualEditLoading || manualEditSaving}
                onChange={(event) => setManualTitle(event.target.value)}
              />
            </label>
            <label>
              {t("knowledgeBase.documents.manualContent")} {" "}
              <Textarea
                value={manualContent}
                disabled={manualEditLoading || manualEditSaving}
                onChange={(event) => setManualContent(event.target.value)}
                rows={8}
              />
            </label>
            {uploadError ? <Status tone="error">{uploadError}</Status> : null}
            <div className="wk-list-actions wk-kd-109">
              <Button type="button" disabled={manualEditLoading || manualEditSaving} onClick={() => void saveManualEdit()}>
                {manualEditSaving ? t("common.loading") : t("knowledgeEditor.buttons.saveAndClose")}
              </Button>
              <Button type="button" disabled={manualEditSaving} onClick={() => setManualEditDocument(null)}>
                {ct("uploadConfirm.cancel")}
              </Button>
            </div>
          </div>
        </Dialog>
      ) : null}
      {traceDocument && canContribute ? (
        <Sheet
          open
          title={`${t("knowledgeBase.timeline.title")}：${displayName(traceDocument)}`}
          onClose={() => setTraceDocument(null)}
          side="right"
          width="820px"
          resizable
          minWidth={560}
          maxWidth={1400}
          storageKey="weknora-trace-drawer-width"
          className="wk-kd-118"
        >
          <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === "loading"}>
            {traceState.status === "loading" ? <Status>{t("common.loading")}</Status> : null}
            {traceState.status === "error" ? <Status tone="error">{traceState.message}</Status> : null}
            {traceState.status === "success" ? (
              <div className="wk-kd-119">
                <div className="wk-kd-120">
                  <Button type="button" onClick={() => setTraceDocument((current) => current ? { ...current } : current)}>{t("knowledgeEditor.activity.retry")}</Button>
                  {traceState.parseStatus === "failed" ? <Button type="button" onClick={() => { const document = traceDocument; setTraceDocument(null); if (document) reparseOne(document); }}>{t("knowledgeBase.rebuildDocument")}</Button> : null}
                  {isKnowledgeProcessingActive(traceState.parseStatus) ? <Button type="button" onClick={() => setConfirmingTraceCancel(true)}>{t("knowledgeBase.documents.cancelParse")}</Button> : null}
                </div>
                {traceState.parseStatus === "failed" ? <Status tone="error">{traceState.lastError?.error_message || t("knowledgeBase.timeline.failed")}</Status> : null}
                <ol className="wk-kd-121" aria-label={t("knowledgeBase.timeline.title")}>
                {traceState.steps.map((step) => (
                  <li key={step.stage} data-state={step.state} className="wk-kd-122">
                    <span>{t(`knowledgeBase.timeline.stage.${step.stage}`)}</span>
                    <span className={step.state === "failed" ? "wk-kd-137" : step.state === "done" ? "wk-kd-167" : step.state === "running" ? "wk-kd-168" : "wk-kd-16"}>
                      {t(`knowledgeBase.timeline.${step.state}`)}
                    </span>
                  </li>
                ))}
                </ol>
                {traceState.nodes.length > 0 ? (
                  <div className="wk-kd-123">
                    <ol className="divide-line-soft wk-kd-124" aria-label={t("knowledgeBase.timeline.title")}>
                      {traceState.nodes.filter((row) => row.depth === 0 || expandedTraceNodes.has(row.key.slice(0, row.key.lastIndexOf(".")))).map((row) => {
                        const rawStatus = typeof row.node.status === "string" ? row.node.status.toLowerCase() : "pending";
                        const state = rawStatus.includes("fail") || rawStatus.includes("error") ? "failed" : rawStatus.includes("run") || rawStatus.includes("progress") || rawStatus.includes("active") ? "running" : rawStatus.includes("complete") || rawStatus.includes("done") || rawStatus.includes("success") || rawStatus.includes("finish") || rawStatus === "ok" || row.node.end_time ? "done" : "pending";
                        const label = row.node.name || row.node.stage || String(row.node.span_id || row.key);
                        return <li key={row.key} data-state={state} className="wk-kd-125" style={{ paddingLeft: `${12 + row.depth * 16}px` }}>
                          {row.hasChildren ? <button type="button" className="wk-kd-126" aria-expanded={expandedTraceNodes.has(row.key)} aria-label={t("knowledgeBase.timeline.title")} onClick={() => setExpandedTraceNodes((current) => { const next = new Set(current); if (next.has(row.key)) next.delete(row.key); else next.add(row.key); return next; })}>{expandedTraceNodes.has(row.key) ? "⌄" : "›"}</button> : <span className="wk-kd-127" aria-hidden="true" />}
                          <button type="button" className="wk-kd-128" onClick={() => setSelectedTraceNode(row)}>{label}</button>
                          <span className={state === "failed" ? "wk-kd-137" : state === "done" ? "wk-kd-167" : state === "running" ? "wk-kd-168" : "wk-kd-16"}>{t(`knowledgeBase.timeline.${state}`)}</span>
                          <span className="wk-kd-129">{typeof row.node.duration_ms === "number" ? `${row.node.duration_ms}ms` : "—"}</span>
                        </li>;
                      })}
                    </ol>
                  </div>
                ) : null}
                {selectedTraceNode ? <section className="wk-kd-130" aria-label={String(selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key)}>
                  <div className="wk-kd-131"><strong className="wk-kd-132">{selectedTraceNode.node.name || selectedTraceNode.node.stage || selectedTraceNode.key}</strong><Button type="button" onClick={() => setSelectedTraceNode(null)}>{t("knowledgeBase.documents.cancel")}</Button></div>
                  <pre className="wk-kd-133">{JSON.stringify(selectedTraceNode.node, null, 2)}</pre>
                </section> : null}
                {traceState.parseStatus === "failed" && traceState.message ? <Status tone="error">{traceState.message}</Status> : null}
              </div>
            ) : null}
          </section>
        </Sheet>
      ) : null}
      {confirmingTraceCancel && traceDocument && canContribute ? (
        <Dialog open title={t("knowledgeBase.documents.cancelParse")} onClose={() => setConfirmingTraceCancel(false)}>
          <p>{t("knowledgeBase.cancelParseConfirmBody", { title: displayName(traceDocument) })}</p>
          <div className="wk-list-actions wk-kd-109">
            <Button type="button" onClick={() => { setConfirmingTraceCancel(false); void cancelOneParse(traceDocument.id); }}>{t("knowledgeBase.documents.cancelParse")}</Button>
            <Button type="button" onClick={() => setConfirmingTraceCancel(false)}>{ct("uploadConfirm.cancel")}</Button>
          </div>
        </Dialog>
      ) : null}
      {confirmingDelete && canContribute ? (
        <Dialog
          open
          title={t("knowledgeBase.documents.delete")}
          onClose={() => setConfirmingDelete(false)}
        >
          <p>
            {t("knowledgeBase.documents.selectedTotal", {
              count: selected.size,
            })}
          </p>
          <div className="wk-list-actions wk-kd-109">
            <Button type="button" onClick={() => void deleteSelected()}>
              {t("knowledgeBase.documents.delete")}
            </Button>
            <Button type="button" onClick={() => setConfirmingDelete(false)}>
              {t("knowledgeBase.documents.cancel")}
            </Button>
          </div>
        </Dialog>
      ) : null}
      {confirmingDeleteDocument && canContribute ? (
        <Dialog
          open
          title={t("knowledgeBase.deleteDocument")}
          onClose={() => setConfirmingDeleteDocument(null)}
        >
          <p>{t("knowledgeBase.confirmDeleteDocument", { fileName: displayName(confirmingDeleteDocument) })}</p>
          {mutationError ? <Status tone="error">{mutationError}</Status> : null}
          <div className="wk-list-actions wk-kd-109">
            <Button type="button" onClick={() => void deleteOneDocument()}>{t("knowledgeBase.confirmDelete")}</Button>
            <Button type="button" onClick={() => setConfirmingDeleteDocument(null)}>{t("knowledgeBase.documents.cancel")}</Button>
          </div>
        </Dialog>
      ) : null}
      {pendingBatchReparse && canContribute ? (
        <Dialog
          open
          title={ct("uploadConfirm.titleReparse")}
          onClose={() => setPendingBatchReparse(null)}
        >
          <p>{t("knowledgeBase.confirmBatchReparseDocument", { count: pendingBatchReparse.length })}</p>
          {mutationError ? <Status tone="error">{mutationError}</Status> : null}
          <div className="wk-list-actions wk-kd-109">
            <Button type="button" onClick={() => void confirmBatchReparse()}>
              {ct("uploadConfirm.confirmReparse")}
            </Button>
            <Button type="button" onClick={() => setPendingBatchReparse(null)}>
              {ct("uploadConfirm.cancel")}
            </Button>
          </div>
        </Dialog>
      ) : null}
      {tagDialog ? (
        // Mounted fresh per open, so preSelectedIds re-seed like the Vue
        // dialogs' watch(visible) (BatchTagDialog.vue L126-135).
        <TagPickerDialog
          open
          t={tt}
          tags={tags}
          mode={tagDialog.mode}
          count={tagDialog.mode === "batch" ? selectedDocuments.length : 1}
          preSelectedIds={
            tagDialog.mode === "single"
              ? documentTags(tagDialog.document).map((tag) => tag.id)
              : commonTagIds(selectedDocuments)
          }
          canManage={canContribute}
          confirmLoading={tagDialogSaving}
          createTag={canContribute ? (name) => createKnowledgeTag(name) : undefined}
          onConfirm={(tagIds) => void submitTagDialog(tagIds)}
          onClose={() => setTagDialog(null)}
        />
      ) : null}
      {canContribute ? (
        <TagManageDialog
          open={tagManageOpen}
          t={tt}
          tags={tags}
          createTag={(name) => createKnowledgeTag(name)}
          updateTag={(tagId, name) =>
            client.knowledgeBases.documents.updateTag(knowledgeBaseId, tagId, { name })}
          deleteTag={(tag) =>
            client.knowledgeBases.documents.deleteTag(knowledgeBaseId, Number(tag.seq_id), true)}
          onClose={() => setTagManageOpen(false)}
          onChanged={(payload) => {
            // Vue onTagManageChanged: reload the tag list + document list
            // (reloadToken drives both effects); a deleted tag that is part
            // of the active filter clears the filter, and deletes outside the
            // filter re-poll after the backend's delayed cascade (800ms).
            setTagPage(1);
            setReloadToken((value) => value + 1);
            const deletedTagId = payload?.deletedTagId;
            if (deletedTagId && selectedTagIds.includes(deletedTagId)) {
              setSelectedTagIds([]);
              setTagFilterCleared(true);
              return;
            }
            if (deletedTagId) {
              window.setTimeout(() => {
                setReloadToken((value) => value + 1);
              }, 800);
            }
          }}
        />
      ) : null}
      {inlineDetailId ? (
        // Vue opens DocContent in place over the list (isCardDetails); the
        // drawer portal covers the still-mounted page like the Vue t-drawer.
        <KnowledgeDocumentDetailPage client={client} documentId={inlineDetailId} onBack={() => setInlineDetailId(null)} />
      ) : null}
      {kbSettingsOpen ? (
        <Dialog
          open
          title={t("knowledgeBase.settings")}
          closeLabel={t("common.close")}
          onClose={() => setKbSettingsOpen(false)}
          className="wk-kb-settings-dialog wk-kd-134"
        >
          {/* R484: the Vue settings footer 取消 (handleClose) discards the
              drafts and closes the drawer — onClose wires that close. */}
          <KnowledgeSettingsPage client={client} knowledgeBaseId={knowledgeBaseId} role={canContribute ? "admin" : "viewer"} onClose={() => setKbSettingsOpen(false)} />
        </Dialog>
      ) : null}
    </div>
  );
}
