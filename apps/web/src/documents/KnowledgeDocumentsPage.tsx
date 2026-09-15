import { useEffect, useMemo, useRef, useState } from "react";
import type { MouseEvent as ReactMouseEvent, ReactNode } from "react";
import type { KnowledgeDocument, KnowledgeTag, ModelConfiguration, ParserEngineInfo, WeKnoraClient } from "@weknora/api-client";
import {
  processingStatusLabel,
  normalizeKnowledgeProcessingStatus,
  buildKnowledgeTimeline,
  isKnowledgeProcessingActive,
  type KnowledgeTimelineStep,
} from "@weknora/domain/knowledge/processing";
import { flattenKnowledgeFolders as flattenFolders } from "@weknora/domain/knowledge/folders";
import { Button, Checkbox, Dialog, Input, Select, Sheet, Status, Textarea } from "@weknora/ui";
import { createTranslator, useAppLocale } from "../i18n.ts";
import { observeUploadProgress } from "../platform/http.ts";
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
  GRAPH_EXTRACT_DEFAULT_EXAMPLE,
  graphDatabaseEnabled,
  graphSectionAvailable,
  hasGraphAdminRole,
  mergeFolderOptions,
  mergeUploadEntries,
  multimodalSectionIssue,
  normalizeUploadUrl,
  removeUploadEntry,
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
  computeKBPermissions,
  kbTypeRedirectPath,
  resolveKBSurfaceTabs,
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
import { TagFilterPanel, TagPickerDialog } from "./TagPickerDialog.tsx";
import { tagSurfaceT } from "./tags-locale.ts";
import {
  loadKnowledgeDocuments,
  type KnowledgeDocumentListState,
} from "./list.ts";
import {
  computeSupportedFileTypes,
  computeUnsupportedFileTypes,
  dateRangeToTimeParams,
  documentsKBSettingsPath,
  isFilteringDocuments,
} from "./page-chrome.ts";
import { toggleDocumentSelection, useMarqueeSelection } from "./selection.ts";
import {
  DocumentEmptyState,
  DocumentsBreadcrumb,
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
}

type UploadDialogMode = "file" | "manual" | "reparse";
/** Vue UploadConfirmResult per-URL append: one shared normalize helper. */

/** t built the way the settings panels do: shared i18n first, dialog table fallback. */
export type UploadDialogT = (key: string, values?: Record<string, string | number>) => string;

function displayName(document: KnowledgeDocument): string {
  return document.file_name || document.title || document.id;
}

export function folderPathCrumbs(path: string | undefined): Array<{ name: string; path: string }> {
  if (!path) return [];
  const segments = path.split('/').filter(Boolean);
  return segments.map((name, index) => ({ name, path: segments.slice(0, index + 1).join('/') }));
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
    <span ref={ref} className="wk-row-tag-chips inline-flex min-w-0 max-w-full flex-nowrap gap-1 overflow-hidden font-mono! text-[0.8rem]! text-muted!" title={overflow ? tags.map((tag) => tag.name || "").join(", ") : undefined}>
      {visible.map((tag) => <span key={tag.id} className="row-tag max-w-[120px] font-mono! text-[0.8rem]! text-muted! cursor-default overflow-hidden rounded-[3px] border border-[var(--wk-border,#e4e7ec)] px-[5px] py-0 text-[11px] text-ellipsis whitespace-nowrap text-[var(--wk-muted,#667085)]">{tag.name}</span>)}
      {overflow ? <span className="row-tag-overflow font-mono! text-[0.8rem]! text-muted! inline-flex h-[18px] min-w-[18px] flex-none cursor-default items-center justify-center rounded-full border border-[var(--wk-border,#e4e7ec)] px-[5px] py-0 text-[10px] leading-none text-[var(--wk-muted,#667085)]">+{overflow}</span> : null}
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

function DocumentCardActionMenu({ document, canDownload, t, actions, onDownload, onEdit, onViewTrace, onMove, onBatchManage, onReparse, onCancelParse, onDelete }: {
  document: KnowledgeDocument;
  canDownload: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
  actions: ReturnType<typeof documentRowActions>;
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
  const close = () => setOpen(false);
  const menuItem = (label: string, icon: ReactNode, handler: () => void, danger = false) => <button type="button" role="menuitem" className={`flex w-full cursor-pointer items-center gap-2 rounded-[6px] border-0 bg-transparent px-3 py-2 text-left text-[14px] leading-5 [font:inherit] hover:bg-surface-wash ${danger ? "text-danger" : "text-ink"}`} onClick={() => { close(); handler(); }}>{icon}<span>{label}</span></button>;
  const downloadable = document.source === "file" || document.source === "manual" || !document.source;
  return <span className="relative inline-flex shrink-0" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) close(); }}>
    <button type="button" className={`inline-flex h-7 w-7 cursor-pointer items-center justify-center rounded-[5px] border-0 bg-transparent p-0 text-muted hover:bg-surface-wash ${open ? "bg-surface-wash" : ""}`} aria-label={t("knowledgeBase.documents.title")} title={t("knowledgeBase.documents.title")} aria-haspopup="menu" aria-expanded={open} onClick={(event) => { event.stopPropagation(); setOpen((value) => !value); }}><MoreIcon /></button>
    <span className="absolute right-0 top-[calc(100%+6px)] z-[220] flex min-w-[180px] flex-col rounded-[8px] border border-line-soft bg-surface p-1 shadow-[0_6px_24px_rgb(15_23_42/12%)] [&[hidden]]:hidden" role="menu" hidden={!open}>
      {canDownload && downloadable ? menuItem(t("knowledgeBase.detail.download", { name: displayName(document) }), <DownloadIcon />, onDownload) : null}
      {document.source === "manual" ? menuItem(t("knowledgeBase.editDocument"), <EditIcon size={16} />, onEdit) : null}
      {actions.canCancelParse || document.trace ? menuItem(t("knowledgeBase.timeline.title"), <MoreIcon />, onViewTrace) : null}
      {actions.canReparse && !actions.canCancelParse ? menuItem(t("knowledgeBase.rebuildDocument"), <RefreshIcon />, onReparse) : null}
      {actions.canCancelParse ? menuItem(t("knowledgeBase.documents.cancelParse"), <RefreshIcon />, onCancelParse) : null}
      {menuItem(t("knowledgeBase.moveToFolder.action"), <MoveIcon />, onMove)}
      {menuItem(t("menu.batchManage"), <MoreIcon />, onBatchManage)}
      {menuItem(t("knowledgeBase.deleteDocument"), <DeleteIcon />, onDelete, true)}
    </span>
  </span>;
}

export function documentCardHoverPosition(
  card: { left: number; right: number; top: number },
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
  const belowY = card.top + popover.height + offset;
  if (belowY <= viewport.height - 10) {
    return { x: Math.max(10, Math.min(card.left, viewport.width - popover.width - 10)), y: belowY };
  }
  return { x: Math.max(10, Math.min(card.left, viewport.width - popover.width - 10)), y: Math.max(10, card.top - popover.height - offset) };
}

export function hasDocumentGridContent(items: readonly KnowledgeDocument[], folders: readonly { path: string }[]): boolean {
  return items.length > 0 || folders.length > 0;
}

function DocumentCardHoverPopover({ document, position, t }: {
  document: KnowledgeDocument;
  position: { x: number; y: number };
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const parseStatus = document.parse_status;
  const statusKey = parseStatus === "failed" ? "knowledgeBase.parsingFailed" : undefined;
  return <div className="knowledge-card-hover-popover fixed z-[250] w-[360px] max-w-[calc(100vw-20px)] rounded-[8px] border border-line-soft bg-surface px-4 py-3 text-[12px] shadow-[0_8px_24px_rgb(16_24_40/14%)]" style={{ left: position.x, top: position.y }} role="tooltip">
    <div className="mb-2 truncate text-[14px] font-semibold text-primary-deep" title={displayName(document)}>{displayName(document)}</div>
    {statusKey ? <div className={`mb-2 ${parseStatus === "failed" ? "text-danger" : "text-warning"}`}>{t(statusKey)}</div> : typeof document.description === "string" && document.description ? <div className="mb-2 line-clamp-3 whitespace-pre-wrap break-words text-muted">{document.description}</div> : null}
    {typeof document.source === "string" && document.source ? <div className="mb-2 flex min-w-0 items-center gap-1 truncate text-muted" title={document.source}><LinkIcon size={12} /> <span className="truncate">{document.source}</span></div> : null}
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted">
      {document.created_at ? <span>{t("knowledgeBase.createdAt")}：{formatDocumentTime(document.created_at)}</span> : null}
      {document.updated_at ? <span>{t("knowledgeBase.updatedAt")}：{formatDocumentTime(document.updated_at)}</span> : null}
      <span>{documentTypeLabel(document)}</span>
    </div>
    {documentTags(document).length > 0 ? <div className="mt-2 flex flex-wrap gap-1">{documentTags(document).map((tag) => <span key={tag.id} className="rounded-full border border-line-soft px-1.5 py-0.5 text-[11px] text-muted">{tag.name}</span>)}</div> : null}
    <div className="mt-2 text-[11px] text-muted">{t("knowledgeBase.clickToViewFull")}</div>
  </div>;
}

type DocumentViewMode = "grid" | "list";

export function DocumentCardGrid({
  items,
  folders,
  selected,
  canContribute,
  canDownload,
  t,
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
  canContribute: boolean;
  canDownload: boolean;
  t: (key: string, values?: Record<string, string | number>) => string;
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
  return <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3" data-document-view="grid">
    {folders.map((folder) => <button key={`folder-${folder.path}`} type="button" className="min-w-[240px] h-[136px] box-border flex flex-col overflow-hidden rounded-lg border border-line-soft bg-surface p-0 text-left shadow-[0_1px_2px_rgb(0_0_0/6%)] transition-[border-color,box-shadow,background-color] duration-200 hover:border-primary/40 hover:bg-surface-wash hover:shadow-[0_4px_14px_rgb(0_0_0/7%)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30" title={folder.path} onClick={() => onOpenFolder(folder.path)}>
      <span className="flex min-h-0 flex-1 flex-col justify-start gap-2 overflow-hidden px-[14px] pb-[10px] pt-3">
        <FolderIcon size={28} className="shrink-0 text-primary opacity-[0.88]" />
        <strong className="line-clamp-2 min-h-0 flex-1 text-sm font-medium leading-5 text-primary-deep">{folder.name}</strong>
      </span>
      <span className="shrink-0 border-t border-line-soft px-[14px] py-2 text-xs leading-[1.4] text-muted">{t("knowledgeBase.folderTree.folderCardCount", { count: folder.total_count })}</span>
    </button>)}
    {items.map((document) => {
      const status = documentStatus(document, t);
      const actions = documentRowActions(document.parse_status);
      return <article key={document.id} className="flex h-[136px] min-w-[240px] flex-col overflow-hidden rounded-[8px] border border-line-soft bg-surface p-0 shadow-[0_1px_2px_rgb(0_0_0/6%)] transition-[border-color,box-shadow,background-color] duration-200 hover:border-primary/40 hover:shadow-[0_4px_14px_rgb(0_0_0/7%)]" onMouseEnter={(event) => scheduleHover(event, document)} onMouseLeave={clearHover}>
        <div className="flex min-h-0 flex-1 flex-col px-[14px] pb-2 pt-[10px]">
          <div className="mb-[6px] flex h-6 shrink-0 items-start gap-0">
            {canContribute ? <Checkbox type="checkbox" checked={selected.has(document.id)} onChange={(event) => onToggle(document.id, event.target.checked)} aria-label={t("knowledgeBase.documents.select", { name: displayName(document) })} /> : null}
            <button type="button" className="min-w-0 flex-1 truncate border-0 bg-transparent p-0 text-left text-[14px] font-semibold leading-6 tracking-[.01em] text-primary-deep hover:underline" onClick={() => onOpen(document)} title={displayName(document)}>{displayName(document)}</button>
            <Status tone={status.tone}>{status.label}</Status>
            {canContribute ? <DocumentCardActionMenu document={document} canDownload={canDownload} t={t} actions={actions} onDownload={() => onDownload(document)} onEdit={() => onEdit(document)} onViewTrace={() => onViewTrace(document)} onMove={() => onMove(document)} onBatchManage={() => onBatchManage(document)} onReparse={() => onReparse(document)} onCancelParse={() => onCancelParse(document)} onDelete={() => onDelete(document)} /> : null}
          </div>
          <p className="m-0 line-clamp-2 min-h-0 flex-1 overflow-hidden text-[12px] font-normal leading-[19px] text-muted">{document.summary_status === "processing" ? t("knowledgeBase.generatingSummary") : typeof document.description === "string" ? document.description : document.folder_path ?? t("knowledgeBase.documents.root")}</p>
        </div>
        <div className="flex h-8 shrink-0 items-center justify-between gap-2 border-t border-line-soft bg-surface px-[14px]">
          <button type="button" className="min-w-0 max-w-[60%] truncate border-0 bg-transparent p-0 text-left text-[12px] text-muted hover:text-primary" title={document.folder_path || t("knowledgeBase.documents.root")} onClick={(event) => { event.stopPropagation(); if (document.folder_path) onOpenFolder(document.folder_path); }}>{document.folder_path || t("knowledgeBase.documents.root")}</button>
          <div className="flex min-w-0 items-center justify-end gap-1 overflow-hidden" onClick={(event) => event.stopPropagation()}>
            {documentTags(document).length > 0 ? <DocumentTagChips tags={documentTags(document)} /> : null}
            {canContribute ? <div className="flex shrink-0 items-center gap-1">
              <Button type="button" onClick={() => onTagEdit(document)}>{t("knowledgeBase.tagLabel")}</Button>
              {actions.canReparse && !actions.canCancelParse ? <Button type="button" onClick={() => onReparse(document)}>{t("knowledgeBase.documents.reparse")}</Button> : null}
              {actions.canCancelParse ? <Button type="button" onClick={() => onCancelParse(document)}>{t("knowledgeBase.documents.cancelParse")}</Button> : null}
            </div> : null}
          </div>
        </div>
      </article>;
    })}
    {hovered ? <DocumentCardHoverPopover document={hovered.document} position={hovered.position} t={t} /> : null}
  </div>;
}

function documentStatus(
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
  if (status === "completed")
    return { label: processingStatusLabel(status), tone: "success" };
  if (status === "failed" || status === "cancelled")
    return { label: processingStatusLabel(status), tone: "error" };
  return { label: processingStatusLabel(status), tone: "warning" };
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
              <FolderIcon size={16} className="shrink-0" />
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
        <p role="alert" className="wk-muted text-muted" style={{ margin: "4px 0 0", fontSize: "0.85rem" }}>
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
        <p className="wk-muted text-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
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
        <p className="wk-muted text-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
          {labels.reparseHint}
        </p>
      </div>
    );
  }
  const itemCount = props.entries.length + props.urls.length;
  if (itemCount === 0) {
    return <p className="wk-muted text-muted" style={{ margin: "0 0 0.75rem" }}>{labels.noItems}</p>;
  }
  return (
    <ul className="wk-upload-confirm-files m-0 mb-3 max-h-56 list-none overflow-auto p-0">
      {props.urls.map((url, index) => (
        <li key={`url-${url}-${index}`} className="mb-[2px] flex items-center gap-3 rounded-[6px] pb-[6px] pl-2 pr-[6px] pt-[6px] hover:bg-[rgba(16,24,40,0.04)]">
          <LinkIcon size={16} className="shrink-0" />
          <span title={url}>{url}</span>
          <span className="wk-muted text-muted">{labels.urlItemLabel}</span>
          <Button
            type="button"
            disabled={props.uploading}
            aria-label={labels.remove}
            onClick={() => props.onRemoveUrl(index)}
          >
            {labels.remove}
          </Button>
        </li>
      ))}
      {props.entries.map((entry, index) => {
        const state = props.uploadStates[index];
        const relativeDir = uploadEntryRelativeDir(entry);
        return (
          <li key={`${entry.name}-${index}`} className="mb-[2px] flex items-center gap-3 rounded-[6px] pb-[6px] pl-2 pr-[6px] pt-[6px] hover:bg-[rgba(16,24,40,0.04)]">
            <span aria-hidden className="[overflow-wrap:anywhere]" style={{ flex: "0 0 auto" }}>{fileTypeBadge(entry.name)}</span>
            <span title={uploadEntryDisplayTitle(entry)}>{entry.name}</span>
            <span className="wk-muted text-muted">
              {relativeDir ? (
                <>
                  <span title={relativeDir}>{relativeDir}</span>
                  <span aria-hidden> · </span>
                </>
              ) : null}
              {formatBytes(entry.size)}
            </span>
            <Status tone={entryStatusTone(state?.status)}>
              {state?.status === "error" ? (state.message ?? labels.statusLabel("error")) : labels.statusLabel(state?.status ?? "pending")}
            </Status>
            <Button
              type="button"
              disabled={props.uploading}
              aria-label={labels.remove}
              onClick={() => props.onRemoveEntry(index)}
            >
              {labels.remove}
            </Button>
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
      <svg className="wk-upload-nav-icon text-[var(--wk-accent,#4a7dff)]" width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden style={{ flex: "0 0 auto" }}>
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
    <nav className="wk-upload-section-nav" aria-label={props.navLabel} style={{ display: "flex", flexWrap: "wrap", gap: "0.25rem 0.75rem", margin: "0 0 0.75rem" }}>
      {props.items.map((item) => (
        <button
          key={item.key}
          type="button"
          className={[
            "wk-upload-nav-item",
            item.active ? "is-active bg-[rgba(74,125,255,0.08)]" : "",
            item.issue ? "has-issue" : "",
          ].filter(Boolean).join(" ")}
          aria-current={item.active ? "true" : undefined}
          data-section-target={item.key}
          style={{
            display: "inline-flex",
            alignItems: "baseline",
            gap: "6px",
            padding: "2px 8px",
            border: "1px solid var(--wk-border, #e4e7ec)",
            borderRadius: "6px",
            background: "transparent",
            cursor: "pointer",
            fontSize: "0.85rem",
          }}
          onClick={() => props.onSelect(item.key)}
        >
          {item.icon ? <NavIcon name={item.icon} /> : null}
          <span>{item.label}</span>
          <span
            className={`wk-upload-nav-status tone-${item.tone ?? "default"}`}
            title={item.statusTitle}
            style={{ color: item.tone === "error" ? "var(--wk-danger, #d92d20)" : item.tone === "warning" ? "var(--wk-warning, #b54708)" : "var(--wk-muted, #667085)" }}
          >
            {truncateNavText(item.status)}
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
  onFiles: (files: File[]) => void;
}

/**
 * The dialog's "continue add" affordance (Vue KbUploadSourceDropdown.vue):
 * one trigger opening a file/folder/URL menu; the menu entries drive hidden
 * multiple and webkitdirectory file inputs, URL opens the import sub-dialog.
 */
export function UploadSourceDropdown(props: UploadSourceDropdownProps) {
  const wrapRef = useRef<HTMLSpanElement | null>(null);
  function openNativeInput(kind: "file" | "folder") {
    const input = wrapRef.current?.querySelector<HTMLInputElement>(`input[data-upload-source-input="${kind}"]`);
    input?.click();
  }
  function handleAction(key: UploadSourceDropdownAction) {
    props.onSelect(key);
    if (key === "file" || key === "folder") openNativeInput(key);
  }
  return (
    <span ref={wrapRef} className="wk-upload-source" style={{ position: "relative", display: "inline-flex" }}>
      <input
        type="file"
        multiple
        className="wk-upload-source__hidden-input"
        data-upload-source-input="file"
        style={{ display: "none" }}
        aria-hidden
        tabIndex={-1}
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length > 0) props.onFiles(files);
        }}
      />
      <input
        type="file"
        multiple
        {...({ webkitdirectory: "" } as Record<string, unknown>)}
        className="wk-upload-source__hidden-input"
        data-upload-source-input="folder"
        style={{ display: "none" }}
        aria-hidden
        tabIndex={-1}
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length > 0) props.onFiles(files);
        }}
      />
      <button
        type="button"
        className="wk-upload-source__trigger"
        aria-label={props.tooltip}
        title={props.tooltip}
        aria-haspopup="menu"
        aria-expanded={props.open}
        onClick={props.onToggle}
        style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: "28px", height: "28px", border: "1px solid var(--wk-border, #e4e7ec)", borderRadius: "6px", background: "transparent", cursor: "pointer", fontSize: "14px" }}
      >
        <span aria-hidden>＋</span>
      </button>
      {props.open ? (
        <span
          className="wk-upload-source__menu min-w-40"
          role="menu"
          aria-label={props.tooltip}
          style={{ position: "absolute", top: "32px", left: 0, zIndex: 40, minWidth: "160px", padding: "4px", border: "1px solid var(--wk-border, #e4e7ec)", borderRadius: "8px", background: "var(--wk-surface, #fff)", boxShadow: "0 8px 24px rgba(0,0,0,0.12)", display: "flex", flexDirection: "column" }}
        >
          {props.items.map((item) => (
            <button
              key={item.key}
              type="button"
              role="menuitem"
              className="wk-upload-source__item hover:bg-[rgba(16,24,40,0.05)]"
              data-upload-source={item.key}
              onClick={() => handleAction(item.key)}
              style={{ display: "flex", alignItems: "center", gap: "6px", padding: "6px 8px", border: "none", borderRadius: "6px", background: "transparent", cursor: "pointer", textAlign: "left", fontSize: "0.9rem" }}
            >
              {item.key === "file" ? <FileIcon size={16} /> : item.key === "folder" ? <FolderIcon size={16} /> : item.key === "url" ? <LinkIcon size={16} /> : <EditIcon size={16} />}
              {item.label}
            </button>
          ))}
        </span>
      ) : null}
    </span>
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
  return <div className={`wk-upload-single-select relative w-[280px] max-[720px]:w-full ${className}`.trim()} ref={rootRef}>
    <button type="button" className="wk-upload-single-select__trigger flex w-full min-h-8 items-center justify-between px-[10px] text-left bg-[var(--wk-surface,#fff)] border border-[var(--wk-border,#e4e7ec)] rounded-[6px] text-[var(--wk-text,#101828)] cursor-pointer [font:inherit] hover:border-[var(--wk-accent,#07c05f)] hover:shadow-[0_0_0_2px_rgb(7_192_95/15%)] focus-visible:border-[var(--wk-accent,#07c05f)] focus-visible:shadow-[0_0_0_2px_rgb(7_192_95/15%)] focus-visible:outline-none" role="combobox" aria-label={ariaLabel} aria-expanded={open} onClick={() => setOpen((current) => !current)} onKeyDown={(event) => {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setOpen(true); setActiveIndex((current) => Math.max(0, Math.min(options.length - 1, current + (event.key === "ArrowDown" ? 1 : -1)))); }
      else if (event.key === "Enter" && open) { event.preventDefault(); choose(activeIndex); }
      else if (event.key === "Escape") setOpen(false);
    }}><span>{options[selectedIndex]?.label ?? placeholder ?? value}</span>{clearable && value ? <span role="button" tabIndex={0} aria-label={`清除${ariaLabel}`} onClick={(event) => { event.stopPropagation(); onChange(""); setOpen(false); }}>×</span> : null}<span aria-hidden="true">⌄</span></button>
    {open ? <div className="wk-upload-single-select__popup absolute left-0 right-0 top-[calc(100%+4px)] z-20 grid max-h-[240px] overflow-auto p-1 bg-[var(--wk-surface,#fff)] border border-[var(--wk-border,#e4e7ec)] rounded-[6px] shadow-[0_8px_24px_rgb(16_24_40/14%)]" role="listbox">{options.map((option, index) => <button type="button" role="option" aria-selected={option.value === value} aria-disabled={option.disabled || undefined} disabled={option.disabled} className={`${index === activeIndex ? "is-active " : ""}${option.disabled ? "is-disabled " : ""}rounded-[6px] px-[10px] py-2 text-left bg-transparent border-0 cursor-pointer [font:inherit] hover:bg-[rgb(7_192_95/10%)] disabled:text-[var(--wk-muted,#98a2b3)] disabled:cursor-not-allowed disabled:opacity-70 ${option.value === value ? "bg-[rgb(7_192_95/10%)]" : ""}`.trim() || undefined} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}>{option.label}</button>)}</div> : null}
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
  return <div className="relative w-[280px] max-[720px]:w-full wk-upload-multi-select" ref={rootRef}>
    <div className="wk-upload-multi-select__field flex min-h-8 flex-wrap items-center gap-1 p-1 bg-[var(--wk-surface,#fff)] border border-[var(--wk-border,#e4e7ec)] rounded-[6px] focus-within:border-[var(--wk-accent,#07c05f)] focus-within:shadow-[0_0_0_2px_rgb(7_192_95/15%)]" onClick={() => setOpen(true)}>
      {values.map((value) => <span className="wk-upload-multi-select__chip inline-flex max-w-full items-center gap-1 bg-[rgb(7_192_95/10%)] rounded-[4px] px-[6px] py-[3px] text-[var(--wk-text,#101828)] text-[12px]" key={value}>{options.find((option) => option.value === value)?.label ?? value}<button type="button" className="cursor-pointer border-0 bg-transparent p-0 text-[var(--wk-muted,#667085)]" aria-label={`移除 ${value}`} onClick={(event) => { event.stopPropagation(); toggle(value); }}>×</button></span>)}
      <input type="text" className="border-0 flex-1 min-w-[80px] outline-none px-1 py-[3px] [font:inherit]" role="combobox" aria-label={ariaLabel} aria-expanded={open} value={query} placeholder={values.length ? "" : ariaLabel} onFocus={() => setOpen(true)} onChange={(event) => { setQuery(event.target.value); setActiveIndex(0); setOpen(true); }} onKeyDown={(event) => {
        if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setActiveIndex((current) => Math.max(0, Math.min(filtered.length - 1, current + (event.key === "ArrowDown" ? 1 : -1)))); }
        else if (event.key === "Enter" && filtered[activeIndex]) { event.preventDefault(); toggle(filtered[activeIndex].value); setQuery(""); }
        else if (event.key === "Backspace" && !query && values.length) toggle(values[values.length - 1]);
        else if (event.key === "Escape") { setOpen(false); setQuery(""); }
      }} />
    </div>
    {open ? <div className="wk-upload-multi-select__popup absolute left-0 right-0 top-[calc(100%+4px)] z-20 grid max-h-[240px] overflow-auto p-1 bg-[var(--wk-surface,#fff)] border border-[var(--wk-border,#e4e7ec)] rounded-[6px] shadow-[0_8px_24px_rgb(16_24_40/14%)]" role="listbox" aria-label={ariaLabel}>{filtered.map((option, index) => <button type="button" role="option" aria-selected={values.includes(option.value)} className={`${index === activeIndex ? "is-active bg-[rgb(7_192_95/10%)]" : "bg-transparent"} border-0 rounded-[6px] cursor-pointer [font:inherit] px-[10px] py-2 text-left hover:bg-[rgb(7_192_95/10%)]`} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => toggle(option.value)}>{option.label}</button>)}{filtered.length === 0 ? <span className="wk-muted text-muted">{ariaLabel}</span> : null}</div> : null}
  </div>;
}

export function UploadClearableInput({ value, placeholder, ariaLabel, onChange }: { value: string; placeholder: string; ariaLabel: string; onChange: (value: string) => void }) {
  return <div className="wk-upload-clearable-input relative flex w-[280px] items-center">
    <Input className="box-border w-full pr-[28px]" value={value} placeholder={placeholder} aria-label={ariaLabel} onChange={(event) => onChange(event.target.value)} />
    {value ? <button type="button" className="absolute right-[2px] cursor-pointer border-0 bg-transparent p-[2px_6px] text-[var(--wk-muted,#667085)] text-[16px] leading-none" aria-label={`清除${ariaLabel}`} onClick={() => onChange("")}>×</button> : null}
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
  return <div className={`wk-upload-number-input ${className} inline-flex ${wide ? "w-[200px]" : "w-[88px]"} h-8 items-center overflow-hidden bg-[var(--wk-surface,#fff)] border border-[var(--wk-border,#e4e7ec)] rounded-[6px] focus-within:border-[var(--wk-accent,#07c05f)] focus-within:shadow-[0_0_0_2px_rgb(7_192_95/15%)]`.trim()}>
    <button type="button" className="inline-flex h-full w-[25px] cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-[var(--wk-muted,#667085)] text-[16px] enabled:hover:bg-[rgb(7_192_95/10%)] enabled:hover:text-[var(--wk-text,#101828)] focus-visible:bg-[rgb(7_192_95/10%)] focus-visible:text-[var(--wk-text,#101828)] focus-visible:outline-none disabled:cursor-not-allowed disabled:text-[var(--wk-border,#d0d5dd)]" aria-label={`减少${ariaLabel}`} disabled={value <= min} onClick={() => adjust(-step)}>−</button>
    <input
      type="number"
      className="w-9 min-w-0 flex-1 border-0 bg-transparent box-border text-center text-[var(--wk-text,#101828)] outline-none [appearance:textfield] [font:inherit] [&::-webkit-inner-spin-button]:m-0 [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:m-0 [&::-webkit-outer-spin-button]:appearance-none"
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
    <button type="button" className="inline-flex h-full w-[25px] cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-[var(--wk-muted,#667085)] text-[16px] enabled:hover:bg-[rgb(7_192_95/10%)] enabled:hover:text-[var(--wk-text,#101828)] focus-visible:bg-[rgb(7_192_95/10%)] focus-visible:text-[var(--wk-text,#101828)] focus-visible:outline-none disabled:cursor-not-allowed disabled:text-[var(--wk-border,#d0d5dd)]" aria-label={`增加${ariaLabel}`} disabled={value >= max} onClick={() => adjust(step)}>+</button>
  </div>;
}

export function UploadSwitch({ checked, ariaLabel, onChange }: { checked: boolean; ariaLabel: string; onChange: (checked: boolean) => void }) {
  return <button type="button" className={`wk-upload-switch${checked ? " is-checked" : ""} inline-flex h-5 w-9 cursor-pointer rounded-full border-0 p-[2px] [transition:background-color_.16s_ease] ${checked ? "bg-[var(--wk-accent,#07c05f)]" : "bg-[var(--wk-border,#d0d5dd)]"} focus-visible:[box-shadow:0_0_0_2px_rgb(7_192_95/15%)] focus-visible:[outline:2px_solid_var(--wk-accent,#07c05f)] focus-visible:[outline-offset:2px]`} role="switch" aria-checked={checked} aria-label={ariaLabel} onClick={() => onChange(!checked)}>
    <span className={`wk-upload-switch__handle block h-4 w-4 rounded-full bg-white [transition:transform_.16s_ease] ${checked ? "[transform:translateX(16px)]" : "[transform:translateX(0px)]"}`} aria-hidden="true" />
  </button>;
}

function UploadSettingRow({ label, description, children, className = "" }: { label: ReactNode; description?: string; children: ReactNode; className?: string }) {
  // CSS cascade: .wk-upload-setting-row--separators stretched its control.
  const separators = className.includes("wk-upload-setting-row--separators");
  return <div className={`wk-upload-setting-row ${className} flex items-start justify-between gap-4 border-b border-[var(--wk-border,#e4e7ec)] py-[10px] max-[720px]:flex-col`.trim()}>
    <div className="wk-upload-setting-info flex min-w-0 flex-1 flex-col gap-[5px]"><label className="font-medium">{label}</label>{description ? <p className="wk-muted text-muted m-0 text-[12px]">{description}</p> : null}</div>
    <div className={`wk-upload-setting-control flex w-[280px] flex-none justify-end max-[720px]:w-full ${separators ? "items-stretch" : "items-start"}`}>{children}</div>
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
              <p className="wk-muted text-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
                {t("uploadConfirm.pdfForceScanned.description")}
              </p>
            </div>
            <UploadSwitch checked={state.pdfForceScanned} ariaLabel={t("uploadConfirm.pdfForceScanned.label")} onChange={(checked) => update({ pdfForceScanned: checked })} />
          </div>
        ) : null}
        {props.parserLoading ? (
          <div className="wk-upload-parser-loading" role="status">{t("settings.parser.loading")}</div>
        ) : props.parserEngines.length === 0 ? (
          <p className="wk-muted text-muted">{t("settings.parser.noEngineDetected")}</p>
        ) : (
          <div className="wk-upload-parser-group overflow-hidden rounded-[8px] border border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface-subtle,#f8f9fb)]">
          {parserFileGroups.map((group) => (
            <div className="wk-upload-parser-row flex items-center justify-between gap-4 border-b border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface,#fff)] px-[14px] py-[10px] last:border-b-0 max-[720px]:flex-col max-[720px]:items-stretch" key={group.key}>
              <div className="wk-upload-parser-info block min-w-0 flex-[0_0_168px] max-[720px]:flex-[0_0_auto]">
                <strong className="text-[13px] font-medium">{group.label}</strong>
                <span className="wk-upload-parser-extensions mt-0 flex flex-wrap gap-1">{group.extensions.map((extension) => <span className="rounded-[4px] bg-[rgb(120_135_155/10%)] px-[6px] py-[2px] text-[11px] text-[var(--wk-muted,#667085)] [font-family:var(--app-font-family-mono,ui-monospace,monospace)]" key={extension}>.{extension}</span>)}</span>
              </div>
              <div className="wk-upload-parser-control flex min-w-0 flex-1 flex-col items-stretch gap-[10px] max-[720px]:w-full">
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
                {options.length === 0 ? <div className="wk-upload-parser-warning flex items-center gap-1 text-[12px] leading-[1.4] text-[var(--wk-warning,#b54708)]" role="note"><span>{t("settings.parser.noEngineDetected")}</span>{props.onConfigureParserSettings ? <button type="button" className="cursor-pointer whitespace-nowrap border-0 bg-transparent p-0 text-[var(--wk-accent,#07c05f)] [font:inherit] hover:underline focus-visible:underline" onClick={props.onConfigureParserSettings}>{t("settings.parserEngine")}</button> : null}</div> : null}
                {group.extensions.includes("xlsx") && parserEngineFor(group.extensions) === "builtin" ? (
                  <label className="wk-upload-parser-xlsx-header inline-flex items-center gap-[5px] text-[12px] text-[var(--wk-muted,#667085)]">
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
        <legend className="wk-visually-hidden sr-only">{t("knowledgeEditor.chunking.title")}</legend>
        <div className="wk-upload-section-header mb-4"><h2 className="m-0 mb-1 text-[1.1rem] font-semibold text-[var(--wk-text,#101828)]">{t("knowledgeEditor.chunking.title")}</h2><p className="m-0 text-[.85rem] leading-normal text-[var(--wk-muted,#667085)]">{t("knowledgeEditor.chunking.description")}</p></div>
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
          className="more-options-toggle"
          aria-expanded={props.moreOpen}
          onClick={props.onToggleMore}
          style={{ background: "none", border: "none", cursor: "pointer", padding: "4px 0", color: "inherit" }}
        >
          <span aria-hidden>{props.moreOpen ? "▾" : "▸"}</span> {t("uploadConfirm.moreOptions")}
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
        <legend className="wk-visually-hidden sr-only">{t("knowledgeEditor.multimodal.title")}</legend>
        <div className="wk-upload-section-header mb-4"><h2 className="m-0 mb-1 text-[1.1rem] font-semibold text-[var(--wk-text,#101828)]">{t("knowledgeEditor.multimodal.title")}</h2><p className="m-0 text-[.85rem] leading-normal text-[var(--wk-muted,#667085)]">{t("knowledgeEditor.multimodal.description")}</p></div>
        {props.multimodalIssue ? (
          <p className="wk-muted text-muted" role="note">{t("uploadConfirm.multimodalSetupHint")}</p>
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
                  className="box-border w-full"
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
        <legend className="wk-visually-hidden sr-only">{t("knowledgeEditor.asr.title")}</legend>
        <div className="wk-upload-section-header mb-4"><h2 className="m-0 mb-1 text-[1.1rem] font-semibold text-[var(--wk-text,#101828)]">{t("knowledgeEditor.asr.title")}</h2><p className="m-0 text-[.85rem] leading-normal text-[var(--wk-muted,#667085)]">{t("knowledgeEditor.asr.description")}</p></div>
        {props.asrIssue ? (
          <p className="wk-muted text-muted" role="note">{t("uploadConfirm.asrSetupHint")}</p>
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
                  className="box-border w-full"
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
        <legend className="wk-visually-hidden sr-only">{t("knowledgeEditor.advanced.questionGeneration.label")}</legend>
        <div className="wk-upload-section-header mb-4"><h2 className="m-0 mb-1 text-[1.1rem] font-semibold text-[var(--wk-text,#101828)]">{t("knowledgeEditor.advanced.questionGeneration.label")}</h2><p className="m-0 text-[.85rem] leading-normal text-[var(--wk-muted,#667085)]">{t("knowledgeEditor.advanced.questionGeneration.description")}</p></div>
        <div className="wk-upload-question-row flex items-start justify-between gap-4 border-b border-[var(--wk-border,#e4e7ec)] py-[10px] max-[720px]:flex-col">
          <div className="wk-upload-question-info grid min-w-0 flex-1 gap-1">
            <label id="wk-question-enabled-label" className="font-medium">{t("knowledgeEditor.advanced.questionGeneration.label")}</label>
            <p className="wk-muted text-muted m-0 text-[12px]">{t("knowledgeEditor.advanced.questionGeneration.countDescription")}</p>
          </div>
          <div className="wk-upload-question-control flex items-center gap-[10px]">
            {state.questionEnabled ? <UploadNumberInput min={1} max={10} step={1} ariaLabel={t("knowledgeEditor.advanced.questionGeneration.countLabel")} value={state.questionCount} onChange={(value) => update({ questionCount: value })} /> : null}
            <GraphSwitch id="wk-question-enabled" checked={state.questionEnabled} labelId="wk-question-enabled-label" onChange={(checked) => update({ questionEnabled: checked })} />
          </div>
        </div>
        {state.questionEnabled ? (
          <div className="wk-upload-question-instructions flex items-start justify-between gap-4 border-b border-[var(--wk-border,#e4e7ec)] py-[10px] max-[720px]:flex-col">
            <div className="wk-upload-question-info grid min-w-0 flex-1 gap-1">
              <label htmlFor="wk-question-instructions" className="font-medium">{t("knowledgeEditor.advanced.questionGeneration.instructionsLabel")}</label>
              <p className="wk-muted text-muted m-0 text-[12px]">{t("knowledgeEditor.advanced.questionGeneration.instructionsDescription")}</p>
            </div>
            <Textarea id="wk-question-instructions" className="box-border min-h-[72px] w-[280px] max-[720px]:w-full" rows={3} maxLength={4000} placeholder={t("knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder")} value={state.questionInstructions} onChange={(event) => update({ questionInstructions: event.target.value })} />
          </div>
        ) : null}
      </fieldset>
      {props.graphAvailable && props.graphSettings ? (
        <fieldset className="wk-upload-confirm-graph m-0 mb-3 rounded-[8px] border border-[var(--wk-border,#e4e7ec)] p-3" id="wk-upload-section-graph" data-section="graph" style={sectionStyle("graph")}>
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
    <div className="wk-graph-tags-field flex min-h-8 min-w-[240px] flex-1 flex-wrap items-center gap-[6px] rounded-[6px] border border-[var(--wk-border,#d0d5dd)] bg-[var(--wk-bg,#fff)] px-2 py-1 focus-within:border-[var(--wk-accent,#4a7dff)] focus-within:shadow-[0_0_0_2px_rgb(74_125_255/14%)]" role="listbox" aria-label={ariaLabel} aria-multiselectable="true">
      {tags.map((tag) => (
        <span key={tag} className="wk-graph-tag inline-flex max-w-full items-center gap-1 rounded-[4px] bg-[var(--wk-bg-muted,#f2f4f7)] px-[6px] py-[2px] text-[0.8125rem] text-[var(--wk-text,#344054)]" role="option" aria-selected="true">
          <span>{tag}</span>
          <button type="button" className="cursor-pointer border-0 bg-transparent p-0 leading-none text-[var(--wk-muted,#667085)]" aria-label={`移除 ${tag}`} onClick={() => remove(tag)}>×</button>
        </span>
      ))}
      <Input
        type="text"
        className="h-6 w-full min-w-[120px] flex-1 border-0 bg-transparent p-0 outline-none box-border"
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
      {tags.length > 0 ? <button type="button" className="wk-graph-tags-clear cursor-pointer border-0 bg-transparent p-0 leading-none text-[var(--wk-muted,#667085)]" aria-label="清除关系类型" onClick={() => onChange([])}>×</button> : null}
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
    <div ref={rootRef} className="wk-graph-relation-select relative min-w-[150px] flex-1">
      <Input
        type="text"
        className="box-border w-full"
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
      {clearable && value ? <button type="button" className="wk-graph-relation-clear absolute right-[6px] top-1/2 cursor-pointer border-0 bg-transparent px-[3px] py-0 text-[var(--wk-muted,#667085)] [transform:translateY(-50%)]" aria-label={`清除${ariaLabel}`} onMouseDown={(event) => { event.preventDefault(); choose(""); }}>×</button> : null}
      {open ? (
        <div role="listbox" className="wk-graph-relation-options absolute inset-x-0 top-[calc(100%+4px)] z-[5] flex max-h-[180px] flex-col overflow-y-auto rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface,#fff)] p-1 shadow-[0_8px_20px_rgb(16_24_40/14%)]">
          {filtered.map((option, index) => <button type="button" role="option" aria-selected={option === value} className={`px-2 py-[6px] text-left border-0 bg-transparent cursor-pointer hover:bg-[var(--wk-bg-muted,#f2f4f7)] ${index === activeIndex || option === value ? "is-active bg-[var(--wk-bg-muted,#f2f4f7)]" : ""}`} key={option} onMouseDown={(event) => { event.preventDefault(); choose(option); }}>{option}</button>)}
          {canCreate ? <button type="button" role="option" className={`px-2 py-[6px] text-left border-0 bg-transparent cursor-pointer hover:bg-[var(--wk-bg-muted,#f2f4f7)] ${activeIndex === filtered.length ? "is-active bg-[var(--wk-bg-muted,#f2f4f7)]" : ""}`} onMouseDown={(event) => { event.preventDefault(); choose(filter.trim()); }}>创建“{filter.trim()}”</button> : null}
          {filtered.length === 0 && !canCreate ? <span className="wk-muted text-muted">{placeholder}</span> : null}
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
      className={`wk-graph-switch${checked ? " is-checked" : ""} relative inline-flex h-6 w-10 flex-none cursor-pointer items-center rounded-full border-0 p-0 [transition:background-color_.2s_cubic-bezier(0.38,0,0.24,1)] ${checked ? "bg-[var(--wk-accent,#07c05f)]" : "bg-[var(--wk-bg-muted,#d0d5dd)]"} focus-visible:[outline:2px_solid_var(--wk-accent,#4a7dff)] focus-visible:[outline-offset:2px]`}
      role="switch"
      aria-checked={checked}
      aria-labelledby={labelId}
      onClick={() => onChange(!checked)}
    >
      <span className={`wk-graph-switch-handle absolute left-[3px] top-[3px] h-[18px] w-[18px] rounded-full bg-white shadow-[0_1px_3px_rgb(16_24_40/20%)] [transition:transform_.2s_cubic-bezier(0.38,0,0.24,1)] ${checked ? "[transform:translateX(16px)]" : "[transform:translateX(0px)]"}`} aria-hidden="true" />
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
    <div className="wk-graph-settings w-full">
      <div className="section-header mb-4">
        <h2 className="m-0 mb-1 text-[1.1rem] font-semibold text-[var(--wk-text,#101828)]">{t("graphSettings.title")}</h2>
        <p className="section-desc m-0 text-[.85rem] leading-normal text-[var(--wk-muted,#667085)]">{t("graphSettings.description")}</p>
      </div>
      {!props.graphDatabaseOn ? (
        <p className="wk-graph-alert m-0 mb-3 rounded-[6px] bg-[rgba(183,121,8,0.08)] px-3 py-2 text-[.85rem] text-[var(--wk-warning,#b54708)]" role="alert">{t("graphSettings.disabledWarning")}</p>
      ) : null}
      <div className="settings-group">
        <div className="wk-graph-setting-row flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0">
          <div className="setting-info flex-[0_0_40%] max-w-[40%] pr-3">
            <label id="wk-graph-enabled-label" className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.enableLabel")}</label>
            <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.enableDescription")}</p>
          </div>
          <div className="setting-control flex max-w-[55%] flex-[0_0_55%] items-center justify-end">
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
            <div className="wk-graph-setting-row is-vertical flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0 flex-col items-stretch">
              <div className="setting-info">
                <label htmlFor="wk-graph-instructions" className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.customInstructionsLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.customInstructionsDescription")}</p>
              </div>
              <div className="setting-control is-full flex w-full max-w-full flex-[0_0_55%] flex-col items-start justify-end gap-2">
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
            <div className="wk-graph-setting-row is-vertical flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0 flex-col items-stretch">
              <div className="setting-info">
                <label htmlFor="wk-graph-tags" className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.tagsLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.tagsDescription")}</p>
              </div>
              <div className="setting-control is-full flex w-full max-w-full flex-[0_0_55%] flex-col items-start justify-end gap-2">
                <div className="wk-graph-tags-group flex w-full items-start gap-3">
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
                <div className="wk-graph-add-tag w-full">
                  <Input
                    className="box-border w-full"
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
                  <p className="wk-graph-tip m-0 flex items-center gap-[6px] text-[0.8rem] text-[var(--wk-muted,#667085)]">{t("graphSettings.completeModelConfig")}</p>
                ) : null}
              </div>
            </div>
            <div className="wk-graph-setting-row is-vertical flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0 flex-col items-stretch">
              <div className="setting-info">
                <label htmlFor="wk-graph-text" className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.sampleTextLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.sampleTextDescription")}</p>
              </div>
              <div className="setting-control is-full">
                <div className="wk-graph-text-group flex w-full flex-col items-start gap-3">
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
                  <span className="wk-graph-text-limit -mt-1 self-end text-[0.75rem] leading-[1.25] text-[var(--wk-muted,#667085)]" aria-live="polite">{graphExtract.text.length}/5000</span>
                </div>
                {!llmAvailable ? (
                  <p className="wk-graph-tip m-0 flex items-center gap-[6px] text-[0.8rem] text-[var(--wk-muted,#667085)]">{t("graphSettings.completeModelConfig")}</p>
                ) : null}
              </div>
            </div>
            {graphExtract.nodes.length > 0 ? (
              <div className="wk-graph-setting-row is-vertical flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0 flex-col items-stretch">
                <div className="setting-info">
                  <label className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.entityListLabel")}</label>
                  <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.entityListDescription")}</p>
                </div>
                <div className="setting-control is-full flex w-full max-w-full flex-[0_0_55%] flex-col items-start justify-end gap-2">
                  <div className="wk-graph-node-list flex w-full flex-col gap-4">
                    {graphExtract.nodes.map((node, nodeIndex) => (
                      <div key={nodeIndex} className="wk-graph-node-item rounded-[8px] border border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface,#fff)] p-3" data-graph-node={node.name || undefined}>
                        <div className="wk-graph-node-header mb-2 flex items-center gap-2">
                          <span aria-hidden>👤</span>
                          <Input
                            type="text"
                            className="wk-graph-node-name w-full flex-1 box-border"
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
                        <div className="wk-graph-node-attributes flex flex-col gap-[0.4rem] pl-[1.75rem]">
                          {node.attributes.map((attribute, attrIndex) => (
                            <div key={attrIndex} className="wk-graph-attribute-item flex items-center gap-2">
                              <Input
                                type="text"
                                className="w-full flex-1 box-border"
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
                            className="wk-graph-add-attr self-start"
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

            <div className="wk-graph-setting-row flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0">
              <div className="setting-info flex-[0_0_40%] max-w-[40%] pr-3">
                <label className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.manageEntitiesLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.manageEntitiesDescription")}</p>
              </div>
              <div className="setting-control flex max-w-[55%] flex-[0_0_55%] items-center justify-end">
                <button
                  type="button"
                  className="wk-graph-add-btn border border-[var(--wk-accent,#4a7dff)] rounded-[6px] bg-[var(--wk-accent,#4a7dff)] px-3 py-1 text-[.85rem] text-white cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                  onClick={() => patch({ nodes: [...graphExtract.nodes, { name: "", attributes: [] }] })}
                >
                  {t("graphSettings.addEntity")}
                </button>
              </div>
            </div>
            {graphExtract.relations.length > 0 ? (
              <div className="wk-graph-setting-row is-vertical flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0 flex-col items-stretch">
                <div className="setting-info">
                  <label className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.relationListLabel")}</label>
                  <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.relationListDescription")}</p>
                </div>
                <div className="setting-control is-full flex w-full max-w-full flex-[0_0_55%] flex-col items-start justify-end gap-2">
                  <div className="wk-graph-relation-list flex w-full flex-col gap-[0.6rem]">
                    {graphExtract.relations.map((relation, index) => (
                      <div key={index} className="wk-graph-relation-item flex items-center gap-[0.6rem] rounded-[8px] border border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface,#fff)] p-[0.6rem]">
                        <GraphRelationSelect value={relation.node1} options={graphExtract.nodes.map((node) => node.name)} placeholder={t("graphSettings.selectEntity")} ariaLabel={t("graphSettings.selectEntity")} onChange={(value) => updateRelation(index, { ...relation, node1: value })} />
                        <span aria-hidden className="flex-none text-[var(--wk-muted,#667085)]">→</span>
                        <GraphRelationSelect value={relation.type} options={graphExtract.tags} placeholder={t("graphSettings.selectRelationType")} ariaLabel={t("graphSettings.selectRelationType")} creatable clearable onChange={(value) => updateRelation(index, { ...relation, type: value })} />
                        <span aria-hidden className="flex-none text-[var(--wk-muted,#667085)]">→</span>
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
            <div className="wk-graph-setting-row flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0">
              <div className="setting-info flex-[0_0_40%] max-w-[40%] pr-3">
                <label className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.manageRelationsLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.manageRelationsDescription")}</p>
              </div>
              <div className="setting-control flex max-w-[55%] flex-[0_0_55%] items-center justify-end">
                <button
                  type="button"
                  className="wk-graph-add-btn border border-[var(--wk-accent,#4a7dff)] rounded-[6px] bg-[var(--wk-accent,#4a7dff)] px-3 py-1 text-[.85rem] text-white cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                  onClick={() => patch({ relations: [...graphExtract.relations, { node1: "", node2: "", type: "" }] })}
                >
                  {t("graphSettings.addRelation")}
                </button>
              </div>
            </div>
            <div className="wk-graph-setting-row flex items-start justify-between gap-3 border-b border-[var(--wk-border,#e4e7ec)] py-3 last:border-b-0">
              <div className="setting-info flex-[0_0_40%] max-w-[40%] pr-3">
                <label className="mb-[2px] block text-[0.9rem] font-medium text-[var(--wk-text,#101828)]">{t("graphSettings.extractActionsLabel")}</label>
                <p className="wk-muted text-muted m-0 text-[0.8rem]">{t("graphSettings.extractActionsDescription")}</p>
              </div>
              <div className="setting-control flex max-w-[55%] flex-[0_0_55%] items-center justify-end">
                <div className="wk-graph-actions flex flex-wrap gap-3">
                  {props.canRunExtract ? (
                    <button
                      type="button"
                      className="wk-graph-add-btn border border-[var(--wk-accent,#4a7dff)] rounded-[6px] bg-[var(--wk-accent,#4a7dff)] px-3 py-1 text-[.85rem] text-white cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
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
                  <button type="button" className="cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-transparent px-3 py-1 text-[.85rem]" onClick={loadDefaultExample}>{t("graphSettings.defaultExample")}</button>
                  <button type="button" className="cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-transparent px-3 py-1 text-[.85rem]" onClick={clearExample}>{t("graphSettings.clearExample")}</button>
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
export function UploadProgressMask({ percent }: { percent: number }) {
  const clamped = clampUploadPercent(percent);
  return (
    <div className="wk-upload-mask absolute inset-0 z-[5] flex items-center justify-center rounded-[8px] bg-[rgba(255,255,255,0.92)]" role="status" aria-live="polite">
      <div className="wk-upload-mask__card flex flex-col items-center gap-2 rounded-[8px] border border-[var(--wk-border,#e4e7ec)] bg-[var(--wk-surface,#fff)] px-6 py-4 shadow-[0_4px_12px_rgba(16,24,40,0.08)]">
        <span className="wk-upload-mask__label text-[0.9rem] font-semibold text-[var(--wk-accent,#4a7dff)]">{`Uploading ${clamped}%`}</span>
        <div
          className="wk-upload-mask__bar h-[6px] w-64 max-w-[70vw] overflow-hidden rounded-[3px] bg-[var(--wk-border,#e4e7ec)]"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={clamped}
        >
          <div className="wk-upload-mask__fill h-full bg-[var(--wk-accent,#4a7dff)] [transition:width_.2s_ease]" style={{ width: `${clamped}%` }} />
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
  neutral: "border-[var(--wk-border,#e4e7ec)] text-[var(--wk-text,#101828)]",
  success: "border-[var(--wk-success,#12b76a)] text-[var(--wk-success,#027a48)]",
  warning: "border-[var(--wk-warning,#b54708)] text-[var(--wk-warning,#9a3412)]",
  error: "border-[var(--wk-danger,#d92d20)] text-[var(--wk-danger,#b42318)]",
};

export function KnowledgeDocumentsPage({
  client,
  knowledgeBaseId,
  onOpenDocument,
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
  const [kbList, setKbList] = useState<KBChromeListItem[]>([]);
  const [canContribute, setCanContribute] = useState(true);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState(query);
  const [parseStatus, setParseStatus] = useState("");
  // Vue selectedTagIds/tagFilterCleared/tagFilterPanelVisible (KnowledgeBase.vue L534-547):
  // multi-select tag filter with a trigger label + popup chip panel.
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);
  const [tagFilterCleared, setTagFilterCleared] = useState(false);
  const [tagFilterOpen, setTagFilterOpen] = useState(false);
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
  const lastSelectedIndex = useRef(-1);
  const documentListRef = useRef<HTMLUListElement | null>(null);
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
  const [traceState, setTraceState] = useState<{ status: "idle" | "loading" | "success" | "error"; steps: KnowledgeTimelineStep[]; parseStatus?: string; message?: string }>({ status: "idle", steps: [] });
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
    ] as const)
      .then(([kb, me, list]) => {
        if (!active) return;
        setKbMeta(kb as KBSurfaceKB);
        setMe(me as KBSurfaceMe | null);
        setConfirmState(uploadConfirmStateFromKb(kb as KBSurfaceKB));
        setCanContribute(
          computeKBPermissions(kb as KBSurfaceKB, me as KBSurfaceMe | null)
            .canContribute,
        );
        setKbList(
          (list as { id: unknown; name: unknown; type?: unknown }[]).map((item) => ({
            id: String(item.id),
            name: String(item.name),
            type: typeof item.type === "string" ? item.type : undefined,
          })),
        );
        const redirect = kbTypeRedirectPath(kb as KBSurfaceKB);
        if (redirect) window.location.replace(redirect);
      })
      .catch(() => {
        if (active) setKbMeta(null);
      });
    return () => {
      active = false;
    };
  }, [client, knowledgeBaseId]);

  useEffect(() => {
    let active = true;
    void client.knowledgeBases.settings.parserEngines().then((result) => { if (active) { setParserEngines(result.data); setParserEnginesLoading(false); } }).catch(() => { if (active) { setParserEngines([]); setParserEnginesLoading(false); } });
    return () => { active = false; };
  }, [client]);

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
  const tabs = useMemo(
    () => (kbMeta ? resolveKBSurfaceTabs(kbMeta) : ["documents" as const]),
    [kbMeta],
  );

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

  function stageFiles(files: Iterable<File>) {
    const entries = toUploadEntries(files);
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
    if (!Number.isInteger(confirmState.chunkSize) || confirmState.chunkSize < 100 || confirmState.chunkSize > 4000) {
      setUploadError(t("knowledgeEditor.chunking.sizeDescription"));
      return;
    }
    if (
      !Number.isInteger(confirmState.chunkOverlap) ||
      confirmState.chunkOverlap < 0 ||
      confirmState.chunkOverlap > 500 ||
      confirmState.chunkOverlap >= confirmState.chunkSize
    ) {
      setUploadError(t("knowledgeEditor.chunking.overlapDescription"));
      return;
    }
    // Vue validateBeforeConfirm: required models, with auto-enable on media batches.
    if (hasImages && (!confirmState.multimodalEnabled || !confirmState.vllmModelId.trim())) {
      setUploadError(ct("uploadConfirm.vlmModelRequired"));
      updateConfirm({ multimodalEnabled: true });
      return;
    }
    if (!hasImages && confirmState.multimodalEnabled && !confirmState.vllmModelId.trim()) {
      setUploadError(ct("uploadConfirm.vlmModelSelectRequired"));
      return;
    }
    if (hasAudio && (!confirmState.asrEnabled || !confirmState.asrModelId.trim())) {
      setUploadError(ct("uploadConfirm.asrModelRequired"));
      updateConfirm({ asrEnabled: true });
      return;
    }
    if (!hasAudio && confirmState.asrEnabled && !confirmState.asrModelId.trim()) {
      setUploadError(ct("uploadConfirm.asrModelSelectRequired"));
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
      if (finalStates.some((state) => state.status === "error")) return;
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
    setTraceState({ status: "loading", steps: [] });
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
        const spans = await client.knowledgeBases.documents.spans(traceDocument.id);
        if (!active) return;
        const parseStatus = typeof spans.parse_status === "string" ? spans.parse_status : traceDocument.parse_status;
        setTraceState({ status: "success", steps: buildKnowledgeTimeline(spans), parseStatus });
        if (!isKnowledgeProcessingActive(parseStatus) && polling !== undefined) {
          window.clearInterval(polling);
          polling = undefined;
        }
      } catch (error) {
        if (active) setTraceState({ status: "error", steps: [], message: errorMessage(error, t) });
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
      anchor.download = displayName(document);
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

  // Vue's batch popconfirm filters documents that are already being parsed
  // before the batch endpoint is called.
  function reparseSelected() {
    if (!selected.size) return;
    const ids = filterReparseIds([...selected], items);
    if (!ids.length) {
      setMutationError("All selected documents are already being processed.");
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
    // Same required-model validation the upload path applies.
    if (confirmState.multimodalEnabled && !confirmState.vllmModelId.trim()) {
      setUploadError(ct("uploadConfirm.vlmModelSelectRequired"));
      return;
    }
    if (confirmState.asrEnabled && !confirmState.asrModelId.trim()) {
      setUploadError(ct("uploadConfirm.asrModelSelectRequired"));
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
    <main className="wk-page wk-documents-page max-w-[1180px]! mx-auto box-border pl-9 pr-7 py-6">
      {stageNotice ? <div className={`${stageNoticeClass(stageNotice.tone)} fixed left-1/2 top-[1.25rem] z-[1000] -translate-x-1/2 max-w-[min(30rem,calc(100vw-2rem))] rounded-[6px] border bg-[var(--wk-surface,#fff)] px-[0.875rem] py-[0.625rem] text-[0.875rem] shadow-[0_6px_20px_rgb(16_24_40/14%)] ${STAGE_NOTICE_TONE_CLASS[stageNotice.tone]}`} role="alert" aria-live="polite">{stageNotice.text}</div> : null}
      <header className="wk-header wk-document-header mb-5 flex items-start justify-between gap-4">
        <div className="document-header-title flex min-w-0 flex-col gap-1">
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
          />
          <p className="document-subtitle m-0 text-[14px] font-normal leading-[20px] text-[var(--wk-muted,#66758b)]">{t("knowledgeEditor.document.subtitle")}</p>
          <ParserHint
            t={t}
            types={unsupportedFileTypes}
            onConfigure={() => window.location.assign(documentsKBSettingsPath(knowledgeBaseId))}
          />
          {!canContribute ? (
            <Status tone="warning">
              {t("knowledgeBase.documents.viewerReadonly")}
            </Status>
          ) : null}
        </div>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <nav
            className="wk-kb-tabs"
            aria-label={t("knowledgeBase.documents.title")}
          >
            {tabs.map((tab) => (
              <a
                key={tab}
                className={tab === "documents" ? "is-active" : ""}
                href={
                  tab === "documents"
                    ? `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}`
                    : `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/${tab}`
                }
              >
                {tab === "documents"
                  ? t("knowledgeBase.documents.tabDocuments")
                  : tab === "wiki"
                    ? t("knowledgeBase.documents.tabWiki")
                    : t("knowledgeBase.documents.tabGraph")}
              </a>
            ))}
          </nav>
          <Button
            type="button"
            onClick={() => setReloadToken((value) => value + 1)}
          >
            {t("knowledgeBase.documents.reload")}
          </Button>
        </div>
      </header>
      <div className="wk-documents-surface">
        {uploadError && !uploadDialogOpen ? <Status tone="error">{uploadError}</Status> : null}
        <div
            className={
              dragActive && canContribute
                ? showFolderTree
                  ? "wk-documents-layout wk-dropzone is-active relative grid grid-cols-[minmax(160px,220px)_1fr] gap-[1.25rem] max-[720px]:grid-cols-1 outline-2 outline-dashed outline-offset-[-4px] outline-[var(--wk-accent,#4a7dff)]"
                  : "wk-documents-layout wk-dropzone is-active relative grid grid-cols-1 gap-[1.25rem] max-[720px]:grid-cols-1 outline-2 outline-dashed outline-offset-[-4px] outline-[var(--wk-accent,#4a7dff)]"
                : showFolderTree
                  ? "wk-documents-layout wk-dropzone relative grid grid-cols-[minmax(160px,220px)_1fr] gap-[1.25rem] max-[720px]:grid-cols-1"
                  : "wk-documents-layout wk-dropzone relative grid grid-cols-1 gap-[1.25rem] max-[720px]:grid-cols-1"
            }
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
          {dragActive && canContribute ? (
            <p className="wk-dropzone-hint m-0 mb-2 text-[.85rem] text-[var(--wk-muted,#667085)]" role="status">
              {t("knowledgeBase.emptyKnowledgeDragDrop")}
            </p>
          ) : null}
          {uploading && canContribute ? (
            <UploadProgressMask percent={batchUploadProgress(uploadStates)} />
          ) : null}
          {showFolderTree ? <aside className="wk-folder-panel border-r border-line-soft pr-[1rem] max-[720px]:border-b max-[720px]:border-r-0 max-[720px]:p-0 max-[720px]:pb-[1rem]">
            <strong>{t("knowledgeBase.documents.folders")}</strong>
            {folderState.status === "loading" ? (
              <Status>{t("knowledgeBase.documents.loadingFolders")}</Status>
            ) : null}
            {folderState.status === "error" ? (
              <Status tone="error">{folderState.message}</Status>
            ) : null}
            <ul className="wk-folder-list mb-0 ml-0 mr-0 mt-[0.75rem] list-none p-0">
              {folders.map((folder) => (
                <li
                  key={folder.path}
                  style={{ paddingLeft: `${folder.depth * 0.8}rem` }}
                >
                  <button
                    type="button"
                    className={
                      folderPath === (folder.path || undefined)
                        ? "is-active w-full border-0 rounded-[5px] bg-surface-wash text-primary-deep cursor-pointer flex justify-between py-[0.45rem] px-[0.5rem] text-left hover:bg-surface-wash hover:text-primary-deep"
                        : "w-full border-0 rounded-[5px] bg-transparent text-muted-strong cursor-pointer flex justify-between py-[0.45rem] px-[0.5rem] text-left hover:bg-surface-wash hover:text-primary-deep"
                    }
                    onClick={() => setFolderPath(folder.path || undefined)}
                  >
                    {folder.name} <span className="text-[0.8rem] text-muted">{folder.total_count}</span>
                  </button>
                </li>
              ))}
            </ul>
          </aside> : null}
          <section className="wk-document-results">
            {showFolderTree ? <nav className="doc-folder-path mb-2 flex min-h-6 items-center gap-1 overflow-x-auto text-xs text-muted" aria-label={t("knowledgeBase.folderTree.title")}>
              <button type="button" className={folderPath ? "doc-folder-path__crumb border-0 bg-transparent px-1 py-0.5 text-muted underline-offset-2 hover:text-primary-deep hover:underline" : "doc-folder-path__crumb is-current border-0 bg-transparent px-1 py-0.5 font-medium text-ink"} onClick={() => setFolderPath(undefined)}>
                {t("knowledgeBase.folderTree.rootRow")}
              </button>
              {folderPathCrumbs(folderPath).map((crumb, index, crumbs) => <span key={crumb.path} className="contents">
                <span className="doc-folder-path__sep px-0.5 text-muted" aria-hidden="true">›</span>
                {index === crumbs.length - 1 ? <span className="doc-folder-path__crumb is-current px-1 py-0.5 font-medium text-ink">{crumb.name}</span> : <button type="button" className="doc-folder-path__crumb border-0 bg-transparent px-1 py-0.5 text-muted underline-offset-2 hover:text-primary-deep hover:underline" onClick={() => setFolderPath(crumb.path)}>{crumb.name}</button>}
              </span>)}
              {filtering ? <span className="doc-folder-path__scope ml-1 px-1 text-muted">({t("knowledgeBase.folderTree.searchingSubtree")})</span> : null}
            </nav> : null}
            <div className="doc-filter-bar grid shrink-0 items-center gap-x-3 gap-y-2 pb-3 [grid-template-areas:'search_trailing'_'filters_filters'] [grid-template-columns:1fr_auto] max-[960px]:[grid-template-areas:'search'_'trailing'_'filters'] max-[960px]:[grid-template-columns:1fr]">
              <div className="doc-search-input relative flex min-w-0 w-full items-center [grid-area:search]">
                <SearchIcon size={16} className="doc-search-icon pointer-events-none absolute left-[10px] text-[var(--wk-muted,#98a2b8)]" />
                <Input
                  className="doc-search-field box-border h-8 w-full rounded-full border border-transparent bg-[rgba(0,0,0,0.04)] py-2 pl-8 pr-3 text-[13px] text-[var(--wk-text,#101828)] outline-none [transition:border-color_.15s_ease,background-color_.15s_ease] focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") setDebouncedQuery(query);
                  }}
                  placeholder={t("knowledgeBase.docSearchPlaceholder")}
                  aria-label={t("knowledgeBase.docSearchPlaceholder")}
                />
              </div>
              <div className="doc-filter-bar__filters box-border flex h-8 min-w-0 flex-nowrap items-center gap-3 overflow-x-auto [grid-area:filters] [scrollbar-width:thin]">
                <div className="doc-filter-field doc-tag-filter relative w-[140px] flex-none">
                  <button
                    type="button"
                    className="doc-tag-filter-trigger doc-filter-control inline-flex h-8 min-h-8 w-full cursor-pointer items-center justify-between gap-[6px] rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-transparent px-2 py-0 text-left text-[13px] text-[var(--wk-text,#344054)]"
                    aria-haspopup="true"
                    aria-expanded={tagFilterOpen}
                    aria-label={t("knowledgeBase.tagFilterTitle")}
                    title={tagTriggerTitle}
                    onClick={() => setTagFilterOpen((open) => !open)}
                  >
                    <span className="doc-tag-filter-trigger__label truncate">{tagTriggerLabel}</span>
                    <span className="doc-tag-filter-trigger__caret text-[10px] text-[var(--wk-muted,#98a2b8)]" aria-hidden>
                      {tagFilterOpen ? "▴" : "▾"}
                    </span>
                  </button>
                  {tagFilterOpen ? (
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
                    />
                  ) : null}
                </div>
                <label className="doc-filter-field w-[140px] flex-none">
                  <span className="wk-visually-hidden sr-only">{t("knowledgeBase.fileTypeFilter")}</span>
                  <Select
                    className="doc-filter-control h-8 w-full cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[rgba(0,0,0,0.02)] px-2 py-0 text-[13px] text-[var(--wk-text,#101828)] outline-none focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                    value={fileType}
                    onChange={(event) => setFileType(event.target.value)}
                  >
                    <option value="">{t("knowledgeBase.allFileTypes")}</option>
                    {DOCUMENT_FILE_TYPE_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.labelKey ? t(option.labelKey) : option.label}
                      </option>
                    ))}
                  </Select>
                </label>
                <label className="doc-filter-field w-[140px] flex-none">
                  <span className="wk-visually-hidden sr-only">{t("knowledgeBase.parseStatusFilter")}</span>
                  <Select
                    className="doc-filter-control h-8 w-full cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[rgba(0,0,0,0.02)] px-2 py-0 text-[13px] text-[var(--wk-text,#101828)] outline-none focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                    value={parseStatus}
                    onChange={(event) => setParseStatus(event.target.value)}
                  >
                    <option value="">{t("knowledgeBase.allParseStatuses")}</option>
                    {DOCUMENT_PARSE_STATUS_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {t(option.labelKey ?? option.value)}
                      </option>
                    ))}
                  </Select>
                </label>
                <label className="doc-filter-field w-[140px] flex-none">
                  <span className="wk-visually-hidden sr-only">{t("knowledgeBase.sourceFilter")}</span>
                  <Select
                    className="doc-filter-control h-8 w-full cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[rgba(0,0,0,0.02)] px-2 py-0 text-[13px] text-[var(--wk-text,#101828)] outline-none focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                    value={source}
                    onChange={(event) => setSource(event.target.value)}
                  >
                    <option value="">{t("knowledgeBase.allSources")}</option>
                    {DOCUMENT_SOURCE_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {t(option.labelKey ?? option.value)}
                      </option>
                    ))}
                  </Select>
                </label>
                <div className="doc-filter-field doc-filter-field--wide doc-date-range flex w-[280px] flex-none items-center gap-[6px]">
                  <Input
                    type="date"
                    className="doc-date-input h-8 w-auto min-w-0 flex-[1_1_0] cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[rgba(0,0,0,0.02)] px-2 py-0 text-[13px] text-[var(--wk-text,#101828)] outline-none focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                    value={updatedFrom}
                    max={updatedTo || undefined}
                    aria-label={t("knowledgeBase.updatedTimeFrom")}
                    title={t("knowledgeBase.updatedTimeFrom")}
                    onChange={(event) => setUpdatedFrom(event.target.value)}
                  />
                  <span className="doc-date-range-sep shrink-0 text-[var(--wk-muted,#98a2b8)]" aria-hidden>—</span>
                  <Input
                    type="date"
                    className="doc-date-input h-8 w-auto min-w-0 flex-[1_1_0] cursor-pointer rounded-[6px] border border-[var(--wk-border,#e4e7ec)] bg-[rgba(0,0,0,0.02)] px-2 py-0 text-[13px] text-[var(--wk-text,#101828)] outline-none focus:border-[var(--wk-brand,#0052d9)] focus:bg-[var(--wk-surface,#fff)]"
                    value={updatedTo}
                    min={updatedFrom || undefined}
                    aria-label={t("knowledgeBase.updatedTimeTo")}
                    title={t("knowledgeBase.updatedTimeTo")}
                    onChange={(event) => setUpdatedTo(event.target.value)}
                  />
                </div>
              </div>
              <div className="doc-filter-bar__trailing relative z-[1] flex shrink-0 items-center gap-2 [grid-area:trailing]">
                <div className="doc-view-toggle inline-flex items-center rounded-[6px] border border-[var(--wk-border,#e4e7ec)]" role="group" aria-label={t("knowledgeBase.viewModeToggle")}>
                  <button type="button" className={`h-8 border-0 px-2 [font:inherit] ${viewMode === "grid" ? "bg-surface-wash text-primary-deep" : "bg-transparent text-muted"}`} aria-pressed={viewMode === "grid"} aria-label={t("knowledgeBase.viewModeGrid")} title={t("knowledgeBase.viewModeGrid")} onClick={() => setViewMode("grid")}><GridIcon size={16} /></button>
                  <button type="button" className={`h-8 border-0 border-l border-line-soft px-2 [font:inherit] ${viewMode === "list" ? "bg-surface-wash text-primary-deep" : "bg-transparent text-muted"}`} aria-pressed={viewMode === "list"} aria-label={t("knowledgeBase.viewModeList")} title={t("knowledgeBase.viewModeList")} onClick={() => setViewMode("list")}><ListIcon size={16} /></button>
                </div>
                {canContribute ? (
                  <div className="doc-filter-actions">
                    <UploadSourceDropdown
                      tooltip={t("knowledgeBase.addDocument")}
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
            {canContribute && (state.status === "loading" || items.length > 0) ? <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
              <label className="wk-select-all inline-flex items-center gap-1 whitespace-nowrap">
                <Checkbox
                  type="checkbox"
                  checked={allOnPageSelected}
                  disabled={items.length === 0}
                  onChange={toggleAllOnPage}
                  aria-label={t("knowledgeBase.selectAll")}
                />
                {t("knowledgeBase.selectAll")}
              </label>
              <span className="mr-auto text-[0.85rem] text-muted">
                {t("knowledgeBase.documents.selectedOnPage", {
                  count: selectedOnPage,
                })}
                {selected.size > selectedOnPage
                  ? ` · ${t("knowledgeBase.documents.selectedTotal", { count: selected.size })}`
                  : ""}
              </span>
              {/* Vue DocumentBatchBar: 取消选择 keeps batch mode escapable. */}
              <Button
                type="button"
                disabled={!selected.size}
                onClick={() => setSelected(new Set())}
              >
                {t("knowledgeBase.clearSelection")}
              </Button>
              {canContribute ? (
                <>
                  <Button
                    type="button"
                    disabled={!selected.size}
                    onClick={() => void reparseSelected()}
                  >
                    {t("knowledgeBase.documents.reparse")}
                  </Button>
                  <Button
                    type="button"
                    disabled={!selected.size}
                    onClick={() => {
                      setMoving(true);
                      setMoveTarget(folderPath ?? "");
                    }}
                  >
                    {t("knowledgeBase.documents.move")}
                  </Button>
                  {/* Vue DocumentBatchBar 批量打标签 → BatchTagDialog (L2164-2167). */}
                  <Button
                    type="button"
                    disabled={!selected.size}
                    onClick={() => setTagDialog({ mode: "batch" })}
                  >
                    {tt("knowledgeBase.batchTag")}
                  </Button>
                  <Button
                    type="button"
                    disabled={!selected.size}
                    onClick={() => setConfirmingDelete(true)}
                  >
                    {t("knowledgeBase.documents.delete")}
                  </Button>
                </>
              ) : null}
              <Button
                type="button"
                disabled={!selected.size}
                onClick={() => void cancelSelectedParse()}
              >
                {t("knowledgeBase.documents.cancelParse")}
              </Button>
            </div> : null}
            {moving && canContribute ? (
              <div
                className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"
                role="form"
                aria-label={t("knowledgeBase.documents.moveDestination")}
              >
                <label>
                  {t("knowledgeBase.documents.moveDestination")}{" "}
                  <Select
                    value={moveTarget}
                    onChange={(event) => setMoveTarget(event.target.value)} className="max-w-[9rem] rounded-control border border-line-control bg-surface text-ink px-[0.65rem] py-[0.55rem] [font:inherit]"
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
            {state.status === "success" && items.length === 0 ? (
              <DocumentEmptyState
                t={t}
                variant={
                  filtering ? "search" : folderPath !== undefined ? "folder" : "illustration"
                }
              />
            ) : null}
            {state.status === "success" && viewMode === "grid" && hasDocumentGridContent(items, folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? ""))) ? (
              <DocumentCardGrid
                items={items}
                folders={folders.filter((folder) => folder.path && folder.path.split("/").slice(0, -1).join("/") === (folderPath ?? ""))}
                selected={selected}
                canContribute={canContribute}
                canDownload={true}
                t={t}
                onOpen={(document) => onOpenDocument?.(document)}
                onOpenFolder={(path) => setFolderPath(path || undefined)}
                onToggle={(id, checked) => setSelected((current) => { const next = new Set(current); if (checked) next.add(id); else next.delete(id); return next; })}
                onTagEdit={(document) => setTagDialog({ mode: "single", document })}
                onReparse={(document) => reparseOne(document)}
                onCancelParse={(document) => void cancelOneParse(document.id)}
                onDownload={(document) => void downloadDocument(document)}
                onEdit={(document) => void openManualEdit(document)}
                onViewTrace={(document) => openTrace(document)}
                onMove={(document) => { setSelected(new Set([document.id])); setMoving(true); setMoveTarget(document.folder_path ?? ""); }}
                onBatchManage={(document) => setSelected(new Set([document.id]))}
                onDelete={setConfirmingDeleteDocument}
              />
            ) : null}
            {state.status === "success" && items.length > 0 && viewMode === "list" ? (
              <ul
                ref={documentListRef}
                className={`wk-list wk-document-list relative m-0 list-none p-0${marquee.visible ? " is-marquee-active cursor-crosshair" : ""}`}
                onMouseDown={marquee.onMouseDown}
              >
                {marquee.visible ? (
                  <li
                    className={`wk-document-marquee-box is-${marquee.mode} items-center! pointer-events-none absolute z-[4] rounded-[2px] border ${marquee.mode === "subtract" ? "border-[color-mix(in_srgb,var(--wk-danger,#d92d20)_75%,transparent)]! bg-[color-mix(in_srgb,var(--wk-danger,#d92d20)_10%,transparent)]" : "border-[color-mix(in_srgb,var(--wk-accent,#4a7dff)_75%,transparent)]! bg-[color-mix(in_srgb,var(--wk-accent,#4a7dff)_12%,transparent)]"} flex justify-between gap-4 border-b border-line-soft py-[0.9rem]`}
                    style={{ left: marquee.left, top: marquee.top, width: marquee.width, height: marquee.height }}
                    aria-hidden="true"
                  />
                ) : null}
                {items.map((document) => {
                  const status = documentStatus(document, t);
                  const actions = documentRowActions(document.parse_status);
                  return (
                    <li key={document.id} data-select-id={document.id} className="items-center! flex justify-between gap-4 border-b border-line-soft py-[0.9rem]">
                      {canContribute ? <Checkbox
                          type="checkbox"
                          aria-label={t("knowledgeBase.documents.select", {
                            name: displayName(document),
                          })}
                          checked={selected.has(document.id)}
                          onChange={(event) => toggleSelected(document.id, (event.nativeEvent as MouseEvent).shiftKey)}
                        /> : null}
                      <div className="wk-list-item-copy grid gap-[0.2rem] min-w-0">
                        <button
                          type="button"
                          className="border-0 bg-transparent cursor-pointer p-0 text-left text-primary-deep [font:inherit] [font-weight:650]! hover:underline"
                          onClick={() => onOpenDocument?.(document)}
                        >
                          {displayName(document)}
                        </button>
                        <span className="font-mono text-[0.8rem] text-muted">
                          {document.folder_path ||
                            t("knowledgeBase.documents.root")}
                          {document.file_type ? ` · ${document.file_type}` : ""}
                          {document.source ? ` · ${document.source}` : ""}
                        </span>
                        {documentTags(document).length > 0 ? <DocumentTagChips tags={documentTags(document)} /> : null}
                      </div>
                      <Status tone={status.tone}>{status.label}</Status>
                      {canContribute ? (
                        <span className="wk-row-actions font-mono text-[0.8rem] text-muted">
                          {/* Vue row tag cell: click opens TagEditDialog (L333). */}
                          <Button
                            type="button"
                            onClick={() => setTagDialog({ mode: "single", document })}
                          >
                            {tt("knowledgeBase.tagLabel")}
                          </Button>
                          {actions.canReparse && !actions.canCancelParse ? (
                            <Button
                              type="button"
                              onClick={() => reparseOne(document)}
                            >
                              {t("knowledgeBase.documents.reparse")}
                            </Button>
                          ) : null}
                          {actions.canCancelParse ? (
                            <Button
                              type="button"
                              onClick={() => void cancelOneParse(document.id)}
                            >
                              {t("knowledgeBase.documents.cancelParse")}
                            </Button>
                          ) : null}
                        </span>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
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
          </section>
        </div>
      </div>
      {uploadDialogOpen ? (
        <Dialog
          open
          title={dialogTitle}
          onClose={dialogMode === "reparse" ? closeReparseDialog : (uploading ? () => {} : cancelStagedUploads)}
        >
          {dialogMode === "file" ? (
            <p className="wk-upload-confirm-summary" style={{ display: "flex", alignItems: "center", gap: "0.5rem", margin: "0 0 0.5rem" }}>
              <span className="wk-files-count" aria-label={ct("uploadConfirm.parseConfig")} style={{ minWidth: "1.4rem", textAlign: "center", borderRadius: "999px", padding: "0 0.35rem", border: "1px solid var(--wk-border, #e4e7ec)", fontSize: "0.85rem" }}>
                {batchItemCount}
              </span>
              <span className="wk-muted text-muted">{ct("uploadConfirm.parseConfig")}</span>
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
            </p>
          ) : null}
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
          {sourceUrlDialogOpen ? (
            <Dialog
              open
              title={t("knowledgeBase.importURLTitle")}
              onClose={() => setSourceUrlDialogOpen(false)}
            >
              <div className="wk-upload-url-dialog flex flex-col gap-2">
                <label>
                  {t("knowledgeBase.urlLabel")}{" "}
                  <Input
                    autoFocus
                    className="box-border w-full"
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
                <p className="wk-muted text-muted" style={{ margin: "0.25rem 0 0", fontSize: "0.85rem" }}>{t("knowledgeBase.urlTip")}</p>
                <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
          {dialogMode === "file" ? (
            <fieldset className="wk-upload-confirm-destination" style={{ position: "relative", marginBottom: "0.75rem" }}>
              <legend>{ct("uploadConfirm.destinationLabel")}</legend>
              <button
                type="button"
                className="wk-destination-crumb"
                title={uploadTargetFolder || rootRowLabel}
                aria-label={ct("uploadConfirm.destinationChange")}
                aria-expanded={destinationPickerOpen}
                onClick={() => {
                  setDestinationPickerOpen((open) => !open);
                  setCreatingUnder(null);
                  setNewFolderName("");
                  setDestinationPickerDuplicate(false);
                }}
                style={{ display: "inline-flex", alignItems: "center", gap: "0.5rem", padding: "2px 10px", border: "1px solid var(--wk-border, #e4e7ec)", borderRadius: "6px", background: "transparent", cursor: "pointer" }}
              >
                <span className="wk-muted text-muted">{ct("uploadConfirm.destinationLabel")}</span>
                <span>{destinationBreadcrumb(uploadTargetFolder, rootRowLabel)}</span>
                <span aria-hidden>{destinationPickerOpen ? "▾" : "▸"}</span>
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
          {dialogMode !== "reparse" ? (
            <fieldset className="wk-upload-confirm-tags mb-3 block" id="wk-upload-section-tags" data-section="tags" style={{ display: activeSection === "tags" ? undefined : "none" }}>
              <legend>{ct("uploadConfirm.tabTags")}</legend>
              <p className="wk-muted text-muted" style={{ margin: "0 0 0.4rem", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsDescription")}</p>
              <label>
                <span className="wk-visually-hidden sr-only">{ct("uploadConfirm.tagsPlaceholder")}</span>
                <UploadMultiSelect
                  values={pendingTagIds}
                  options={tags.map((tag) => ({ value: tag.id, label: tag.name }))}
                  ariaLabel={ct("uploadConfirm.tagsPlaceholder")}
                  onChange={setPendingTagIds}
                />
              </label>
              {!uploading && tags.length === 0 ? (
                <p className="wk-muted text-muted" style={{ margin: "0.25rem 0 0", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsEmpty")}</p>
              ) : null}
            </fieldset>
          ) : null}
          <UploadSectionNav
            items={sectionNavItems.map((item) => ({ ...item, active: activeSection === item.key }))}
            navLabel={ct("uploadConfirm.configNav")}
            onSelect={goToSection}
          />
          <UploadConfirmSections
            state={confirmState}
            update={updateConfirm}
            hasPdf={hasPdf}
            multimodalIssue={multimodalIssue}
            asrIssue={asrIssue}
            parserEngines={parserEngines}
            parserLoading={parserEnginesLoading}
            onConfigureParserSettings={() => window.location.assign(documentsKBSettingsPath(knowledgeBaseId))}
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
          {uploadError ? <Status tone="error">{uploadError}</Status> : null}
          <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
            <Button
              type="button"
              loading={uploading}
              disabled={!canConfirm && !uploading}
              onClick={() => void confirmUpload()}
            >
              {confirmButtonText}
            </Button>
            <Button
              type="button"
              onClick={dialogMode === "reparse" ? closeReparseDialog : cancelStagedUploads}
            >
              {ct("uploadConfirm.cancel")}
            </Button>
          </div>
        </Dialog>
      ) : null}
      {manualDialogOpen && canContribute ? (
        <Dialog
          open
          title={t("knowledgeBase.documents.createDocument")}
          onClose={() => setManualDialogOpen(false)}
        >
          <div className="wk-upload-url-dialog flex flex-col gap-2">
            <label>
              {t("knowledgeBase.documents.manualTitle")}{" "}
              <Input
                autoFocus
                className="box-border w-full"
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
            <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
          <div className="wk-upload-url-dialog flex flex-col gap-2">
            {manualEditLoading ? <Status>{t("common.loading")}</Status> : null}
            <label>
              {t("knowledgeBase.documents.manualTitle")} {" "}
              <Input
                autoFocus
                className="box-border w-full"
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
            <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
          width="min(820px, 92vw)"
          className="min-w-0 border-l border-line-soft"
        >
          <section className="wk-processing-timeline" aria-live="polite" aria-busy={traceState.status === "loading"}>
            {traceState.status === "loading" ? <Status>{t("common.loading")}</Status> : null}
            {traceState.status === "error" ? <Status tone="error">{traceState.message}</Status> : null}
            {traceState.status === "success" ? (
              <ol className="m-0 flex list-none flex-col gap-2 p-0">
                {traceState.steps.map((step) => (
                  <li key={step.stage} data-state={step.state} className="flex items-center justify-between rounded-[6px] border border-line-soft px-3 py-2 text-[13px]">
                    <span>{t(`knowledgeBase.timeline.stage.${step.stage}`)}</span>
                    <span className={step.state === "failed" ? "text-danger" : step.state === "done" ? "text-success" : step.state === "running" ? "text-primary" : "text-muted"}>
                      {t(`knowledgeBase.timeline.${step.state}`)}
                    </span>
                  </li>
                ))}
              </ol>
            ) : null}
          </section>
        </Sheet>
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
          <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
          <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
          <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
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
    </main>
  );
}
