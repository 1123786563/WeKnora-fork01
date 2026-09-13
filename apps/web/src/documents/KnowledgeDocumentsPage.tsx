import { useEffect, useMemo, useRef, useState } from "react";
import type { KnowledgeDocument, ModelConfiguration, ParserEngineInfo, WeKnoraClient } from "@weknora/api-client";
import {
  processingStatusLabel,
  normalizeKnowledgeProcessingStatus,
} from "@weknora/domain/knowledge/processing";
import { flattenKnowledgeFolders as flattenFolders } from "@weknora/domain/knowledge/folders";
import { Button, Card, Dialog, Status } from "@weknora/ui";
import { createTranslator, useAppLocale } from "../i18n.ts";
import {
  applyUploadOverrides,
  asrSectionIssue,
  batchHasAudio,
  batchHasImages,
  batchUploadExtensions,
  buildUploadConfirmOverrides,
  commitFolderName,
  destinationBreadcrumb,
  folderPickerRows,
  formatBytes,
  mergeFolderOptions,
  mergeUploadEntries,
  multimodalSectionIssue,
  normalizeUploadUrl,
  removeUploadEntry,
  runUploadPipeline,
  toUploadEntries,
  uploadConfirmStateFromKb,
  uploadConfirmT,
  uploadEntryDisplayTitle,
  uploadEntryRelativeDir,
  uploadSectionStatus,
  type FolderOption,
  type Locale,
  type UploadConfirmUIState,
  type UploadEntry,
  type UploadEntryState,
  type UploadEntryStatus,
} from "./upload-pipeline.ts";
import "./documents.css";
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
  loadKnowledgeDocuments,
  type KnowledgeDocumentListState,
} from "./list.ts";

interface KnowledgeDocumentsPageProps {
  client: WeKnoraClient;
  knowledgeBaseId: string;
  onOpenDocument?: (document: KnowledgeDocument) => void;
}

type UploadSource = "file" | "url" | "manual";
type UploadDialogMode = "file" | "manual" | "reparse";
type UploadConfirmSectionKey = "tags" | "parser" | "chunking" | "multimodal" | "asr" | "question";

/** t built the way the settings panels do: shared i18n first, dialog table fallback. */
export type UploadDialogT = (key: string, values?: Record<string, string | number>) => string;

function displayName(document: KnowledgeDocument): string {
  return document.file_name || document.title || document.id;
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

function errorMessage(error: unknown): string {
  const candidate = error as {
    code?: unknown;
    message?: unknown;
    status?: unknown;
  };
  if (candidate?.status === 413 || candidate?.code === "PAYLOAD_TOO_LARGE")
    return "Upload is too large (413). Choose a smaller file and retry.";
  return candidate?.message && typeof candidate.message === "string"
    ? candidate.message
    : "The document operation failed.";
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
              <span aria-hidden style={{ flex: "0 0 auto" }}>{row.isRoot ? "📂" : "📁"}</span>
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
              <span aria-hidden>📁</span>
              <input
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
        <p role="alert" className="wk-muted" style={{ margin: "4px 0 0", fontSize: "0.85rem" }}>
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
        <p className="wk-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
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
        <p className="wk-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
          {labels.reparseHint}
        </p>
      </div>
    );
  }
  const itemCount = props.entries.length + props.urls.length;
  if (itemCount === 0) {
    return <p className="wk-muted" style={{ margin: "0 0 0.75rem" }}>{labels.noItems}</p>;
  }
  return (
    <ul className="wk-upload-confirm-files">
      {props.urls.map((url, index) => (
        <li key={`url-${url}-${index}`}>
          <span aria-hidden style={{ flex: "0 0 auto" }}>🔗</span>
          <span title={url}>{url}</span>
          <span className="wk-muted">{labels.urlItemLabel}</span>
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
          <li key={`${entry.name}-${index}`}>
            <span aria-hidden style={{ flex: "0 0 auto" }}>{fileTypeBadge(entry.name)}</span>
            <span title={uploadEntryDisplayTitle(entry)}>{entry.name}</span>
            <span className="wk-muted">
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
          className={item.issue ? "wk-upload-nav-item has-issue" : "wk-upload-nav-item"}
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

// --- Config sections (Vue config-panel parity) --------------------------------

export interface UploadConfirmSectionsProps {
  state: UploadConfirmUIState;
  update: (patch: Partial<UploadConfirmUIState>) => void;
  hasPdf: boolean;
  multimodalIssue: boolean;
  asrIssue: boolean;
  parserEngines: ParserEngineInfo[];
  vllmModels: ModelConfiguration[];
  asrModels: ModelConfiguration[];
  moreOpen: boolean;
  onToggleMore: () => void;
  /** Shared-i18n-first translator (covers knowledgeEditor.* / settings.*). */
  t: UploadDialogT;
}

const CHUNKING_STRATEGY_OPTIONS = [
  { value: "auto", labelKey: "knowledgeEditor.chunking.strategies.auto.label" },
  { value: "heading", labelKey: "knowledgeEditor.chunking.strategies.heading.label" },
  { value: "heuristic", labelKey: "knowledgeEditor.chunking.strategies.heuristic.label" },
  { value: "legacy", labelKey: "knowledgeEditor.chunking.strategies.legacy.label" },
] as const;

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
  const parserFileTypes = [...new Set(props.parserEngines.flatMap((engine) => engine.FileTypes ?? []))]
    .filter((fileType) => fileType !== "url")
    .sort();
  return (
    <>
      <fieldset className="wk-upload-confirm-parser" id="wk-upload-section-parser" data-section="parser">
        <legend>{t("settings.parserEngine")}</legend>
        {props.hasPdf ? (
          <div className="setting-row" style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start", marginBottom: "0.5rem" }}>
            <div style={{ flex: 1 }}>
              <label htmlFor="wk-pdf-force-scanned">{t("uploadConfirm.pdfForceScanned.label")}</label>
              <p className="wk-muted" style={{ margin: 0, fontSize: "0.85rem" }}>
                {t("uploadConfirm.pdfForceScanned.description")}
              </p>
            </div>
            <label className="wk-checkbox">
              <input
                id="wk-pdf-force-scanned"
                type="checkbox"
                checked={state.pdfForceScanned}
                onChange={(event) => update({ pdfForceScanned: event.target.checked })}
              />
            </label>
          </div>
        ) : null}
        {props.parserEngines.length === 0 ? (
          <p className="wk-muted">{t("settings.parser.noEngineDetected")}</p>
        ) : (
          parserFileTypes.map((fileType) => (
            <label key={fileType}>
              .{fileType}
              <select
                value={state.parserRules.find((rule) => rule.file_types.includes(fileType))?.engine ?? ""}
                onChange={(event) =>
                  update({
                    parserRules: (() => {
                      const remaining = state.parserRules.filter((rule) => !rule.file_types.includes(fileType));
                      return event.target.value ? [...remaining, { file_types: [fileType], engine: event.target.value }] : remaining;
                    })(),
                  })
                }
              >
                <option value="">{t("uploadConfirm.navParserDefault")}</option>
                {props.parserEngines
                  .filter((engine) => (engine.FileTypes ?? []).includes(fileType))
                  .map((engine) => (
                    <option key={engine.Name} value={engine.Name} disabled={engine.Available === false}>
                      {engine.Name}
                      {engine.Available === false ? ` — ${engine.UnavailableReason || t("settings.parser.unavailable")}` : ""}
                    </option>
                  ))}
              </select>
            </label>
          ))
        )}
      </fieldset>
      <fieldset className="wk-upload-confirm-chunking" id="wk-upload-section-chunking" data-section="chunking">
        <legend>{t("knowledgeEditor.chunking.title")}</legend>
        <label>
          {t("knowledgeEditor.chunking.strategyLabel")}{" "}
          <select value={state.chunkStrategy} onChange={(event) => update({ chunkStrategy: event.target.value })}>
            {CHUNKING_STRATEGY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
            ))}
          </select>
        </label>
        <label>
          {t("knowledgeEditor.chunking.sizeLabel")}{" "}
          <input
            type="number"
            min={100}
            max={4000}
            step={50}
            value={state.chunkSize}
            onChange={(event) => update({ chunkSize: Number(event.target.value) })}
          />
        </label>
        <label>
          {t("knowledgeEditor.chunking.overlapLabel")}{" "}
          <input
            type="number"
            min={0}
            max={500}
            step={20}
            value={state.chunkOverlap}
            onChange={(event) => update({ chunkOverlap: Number(event.target.value) })}
          />
        </label>
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
            <label>
              {t("knowledgeEditor.chunking.separatorsLabel")}{" "}
              <select
                multiple
                value={state.separators}
                onChange={(event) =>
                  update({ separators: Array.from(event.target.selectedOptions).map((option) => option.value) })
                }
              >
                {CHUNKING_SEPARATOR_OPTIONS.map((option) => (
                  <option key={option.labelKey} value={option.value}>{t(option.labelKey)}</option>
                ))}
              </select>
            </label>
            <label>
              {t("knowledgeEditor.chunking.tokenLimitLabel")}{" "}
              <input
                type="number"
                min={0}
                max={8192}
                step={64}
                value={state.tokenLimit}
                onChange={(event) => update({ tokenLimit: Number(event.target.value) })}
              />
            </label>
            <label>
              {t("knowledgeEditor.chunking.languagesLabel")}{" "}
              <select
                multiple
                value={state.languages}
                onChange={(event) =>
                  update({ languages: Array.from(event.target.selectedOptions).map((option) => option.value) })
                }
              >
                {CHUNKING_LANGUAGE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
                ))}
              </select>
            </label>
            <label className="wk-checkbox">
              <input
                type="checkbox"
                checked={state.enableParentChild}
                onChange={(event) => update({ enableParentChild: event.target.checked })}
              />{" "}
              {t("knowledgeEditor.chunking.parentChildLabel")}
            </label>
            {state.enableParentChild ? (
              <>
                <label>
                  {t("knowledgeEditor.chunking.parentChunkSizeLabel")}{" "}
                  <input
                    type="number"
                    min={512}
                    max={8192}
                    step={64}
                    value={state.parentChunkSize}
                    onChange={(event) => update({ parentChunkSize: Number(event.target.value) })}
                  />
                </label>
                <label>
                  {t("knowledgeEditor.chunking.childChunkSizeLabel")}{" "}
                  <input
                    type="number"
                    min={64}
                    max={2048}
                    step={32}
                    value={state.childChunkSize}
                    onChange={(event) => update({ childChunkSize: Number(event.target.value) })}
                  />
                </label>
              </>
            ) : null}
          </div>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-multimodal" id="wk-upload-section-multimodal" data-section="multimodal">
        <legend>{t("knowledgeEditor.sidebar.multimodal")}</legend>
        {props.multimodalIssue ? (
          <p className="wk-muted" role="note">{t("uploadConfirm.multimodalSetupHint")}</p>
        ) : null}
        <label className="wk-checkbox">
          <input
            type="checkbox"
            checked={state.multimodalEnabled}
            onChange={(event) => update({ multimodalEnabled: event.target.checked })}
          />{" "}
          {t("knowledgeEditor.advanced.multimodal.label")}
        </label>
        {state.multimodalEnabled ? (
          <>
            <label>
              {t("knowledgeEditor.advanced.multimodal.vllmLabel")} <span aria-hidden>*</span>
              {props.vllmModels.length > 0 ? (
                <select
                  required
                  value={state.vllmModelId}
                  onChange={(event) => update({ vllmModelId: event.target.value })}
                >
                  <option value="">{t("knowledgeEditor.advanced.multimodal.vllmPlaceholder")}</option>
                  {props.vllmModels.map((model) => (
                    <option key={model.id} value={model.id}>{model.name}</option>
                  ))}
                </select>
              ) : (
                <input
                  required
                  value={state.vllmModelId}
                  onChange={(event) => update({ vllmModelId: event.target.value })}
                  placeholder={t("knowledgeEditor.advanced.multimodal.vllmPlaceholder")}
                />
              )}
            </label>
            <label>
              {t("knowledgeEditor.advanced.multimodal.descriptionLanguageLabel")}{" "}
              <select
                value={state.descriptionLanguage}
                onChange={(event) => update({ descriptionLanguage: event.target.value })}
              >
                <option value="">{t("knowledgeEditor.advanced.multimodal.descriptionLanguageAuto")}</option>
                {MULTIMODAL_LANGUAGE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
                ))}
              </select>
            </label>
            <label>
              {t("knowledgeEditor.advanced.multimodal.customInstructionsLabel")}{" "}
              <textarea
                rows={3}
                maxLength={4000}
                placeholder={t("knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder")}
                value={state.customInstructions}
                onChange={(event) => update({ customInstructions: event.target.value })}
              />
            </label>
          </>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-asr" id="wk-upload-section-asr" data-section="asr">
        <legend>{t("knowledgeEditor.sidebar.asr")}</legend>
        {props.asrIssue ? (
          <p className="wk-muted" role="note">{t("uploadConfirm.asrSetupHint")}</p>
        ) : null}
        <label className="wk-checkbox">
          <input
            type="checkbox"
            checked={state.asrEnabled}
            onChange={(event) => update({ asrEnabled: event.target.checked })}
          />{" "}
          {t("knowledgeEditor.asr.label")}
        </label>
        {state.asrEnabled ? (
          <>
            <label>
              {t("knowledgeEditor.asr.modelLabel")} <span aria-hidden>*</span>
              {props.asrModels.length > 0 ? (
                <select
                  required
                  value={state.asrModelId}
                  onChange={(event) => update({ asrModelId: event.target.value })}
                >
                  <option value="">{t("knowledgeEditor.asr.modelPlaceholder")}</option>
                  {props.asrModels.map((model) => (
                    <option key={model.id} value={model.id}>{model.name}</option>
                  ))}
                </select>
              ) : (
                <input
                  required
                  value={state.asrModelId}
                  onChange={(event) => update({ asrModelId: event.target.value })}
                  placeholder={t("knowledgeEditor.asr.modelPlaceholder")}
                />
              )}
            </label>
            <label>
              {t("knowledgeEditor.asr.languageLabel")}{" "}
              <input
                value={state.asrLanguage}
                placeholder={t("knowledgeEditor.asr.languagePlaceholder")}
                onChange={(event) => update({ asrLanguage: event.target.value })}
              />
            </label>
          </>
        ) : null}
      </fieldset>
      <fieldset className="wk-upload-confirm-question" id="wk-upload-section-question" data-section="question">
        <legend>{t("knowledgeEditor.advanced.questionGeneration.label")}</legend>
        <div style={{ display: "flex", gap: "0.75rem", alignItems: "center" }}>
          <label className="wk-checkbox" style={{ flex: 1 }}>
            <input
              type="checkbox"
              checked={state.questionEnabled}
              onChange={(event) => update({ questionEnabled: event.target.checked })}
            />{" "}
            {t("knowledgeEditor.advanced.questionGeneration.label")}
          </label>
          {state.questionEnabled ? (
            <label>
              <span className="wk-visually-hidden">{t("knowledgeEditor.advanced.questionGeneration.countLabel")}</span>
              <input
                type="number"
                min={1}
                max={10}
                step={1}
                value={state.questionCount}
                onChange={(event) => update({ questionCount: Number(event.target.value) })}
                style={{ width: "5rem" }}
              />
            </label>
          ) : null}
        </div>
        {state.questionEnabled ? (
          <label>
            {t("knowledgeEditor.advanced.questionGeneration.instructionsLabel")}{" "}
            <textarea
              rows={3}
              maxLength={4000}
              placeholder={t("knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder")}
              value={state.questionInstructions}
              onChange={(event) => update({ questionInstructions: event.target.value })}
            />
          </label>
        ) : null}
      </fieldset>
    </>
  );
}

// --- Page ---------------------------------------------------------------------

export function KnowledgeDocumentsPage({
  client,
  knowledgeBaseId,
  onOpenDocument,
}: KnowledgeDocumentsPageProps) {
  const locale = useAppLocale() as Locale;
  const t = createTranslator(locale);
  // Dialog copy: shared i18n keys first, then the Vue-ported uploadConfirm table.
  const ct: UploadDialogT = uploadConfirmT(locale);
  const [reloadToken, setReloadToken] = useState(0);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
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
  const [kbMeta, setKbMeta] = useState<KBSurfaceKB | null>(null);
  const [canContribute, setCanContribute] = useState(true);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState(query);
  const [parseStatus, setParseStatus] = useState("");
  const [tagId, setTagId] = useState("");
  const [folderPath, setFolderPath] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [moving, setMoving] = useState(false);
  const [moveTarget, setMoveTarget] = useState("");
  const [uploadSource, setUploadSource] = useState<UploadSource>("file");
  // Multi-file upload parity: staged files wait behind a confirm dialog
  // (Vue UploadConfirmDialog) before any upload call is issued.
  const [pendingEntries, setPendingEntries] = useState<UploadEntry[]>([]);
  const [pendingUrl, setPendingUrl] = useState("");
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
  const [stageNotice, setStageNotice] = useState<{ tone: "neutral" | "warning"; text: string } | null>(null);
  const [moreUrl, setMoreUrl] = useState("");
  const [parserEngines, setParserEngines] = useState<ParserEngineInfo[]>([]);
  const [tenantModels, setTenantModels] = useState<ModelConfiguration[]>([]);
  const [uploadStates, setUploadStates] = useState<readonly UploadEntryState[]>(
    [],
  );
  const [dragActive, setDragActive] = useState(false);
  const [url, setUrl] = useState("");
  const [manualTitle, setManualTitle] = useState("");
  const [manualContent, setManualContent] = useState("");
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const uploadController = useRef<AbortController | null>(null);
  const uploadPipelineController = useRef<AbortController | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [pendingReparse, setPendingReparse] = useState<{
    document: KnowledgeDocument;
    processConfig: unknown;
  } | null>(null);
  const [pendingBatchReparse, setPendingBatchReparse] = useState<string[] | null>(null);
  const pageSize = 20;

  function seedConfirmFromKb() {
    setConfirmState(uploadConfirmStateFromKb(kbMeta));
    setChunkingMoreOpen(false);
  }

  function updateConfirm(patch: Partial<UploadConfirmUIState>) {
    setConfirmState((current) => ({ ...current, ...patch }));
  }

  // Audit #6: KB-type routing — an FAQ KB must land on the FAQ route.
  // The same fetch drives permission gating and tab visibility, and seeds the
  // upload-confirm dialog defaults (Vue initFromKbInfo).
  useEffect(() => {
    let active = true;
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
    ])
      .then(([kb, me]) => {
        if (!active) return;
        setKbMeta(kb as KBSurfaceKB);
        setConfirmState(uploadConfirmStateFromKb(kb as KBSurfaceKB));
        setCanContribute(
          computeKBPermissions(kb as KBSurfaceKB, me as KBSurfaceMe | null)
            .canContribute,
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
    void client.knowledgeBases.settings.parserEngines().then((result) => { if (active) setParserEngines(result.data); }).catch(() => { if (active) setParserEngines([]); });
    return () => { active = false; };
  }, [client]);

  useEffect(() => {
    let active = true;
    void client.configuration.models.list().then((models) => { if (active) setTenantModels(models); }).catch(() => { if (active) setTenantModels([]); });
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
      tag_ids: tagId || undefined,
      folder_path: folderPath,
      folder_recursive: folderPath !== undefined,
    }).then((next) => {
      if (active) setState(next);
    });
    return () => {
      active = false;
    };
  }, [
    client,
    folderPath,
    knowledgeBaseId,
    page,
    parseStatus,
    debouncedQuery,
    reloadToken,
  ]);

  // Audit #10: 300ms debounce so fast typing issues a single API call.
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 300);
    return () => clearTimeout(timer);
  }, [query]);

  useEffect(() => {
    let active = true;
    setFolderState({ status: "loading" });
    void Promise.all([
      client.knowledgeBases.documents.folders(knowledgeBaseId),
      client.knowledgeBases.documents.tags(knowledgeBaseId),
    ])
      .then(([tree, nextTags]) => {
        if (active) {
          setFolderState({ status: "success", tree });
          setTags(nextTags);
        }
      })
      .catch((error: unknown) => {
        if (active)
          setFolderState({ status: "error", message: errorMessage(error) });
      });
    return () => {
      active = false;
    };
  }, [client, knowledgeBaseId, reloadToken]);

  useEffect(() => {
    setPage(1);
  }, [folderPath, parseStatus, query, tagId]);

  const folders = useMemo(
    () => (folderState.tree ? flattenFolders(folderState.tree) : []),
    [folderState.tree],
  );
  const vllmModels = useMemo(() => tenantModels.filter((model) => String(model.type ?? "").toLowerCase() === 'vllm'), [tenantModels]);
  const asrModels = useMemo(() => tenantModels.filter((model) => String(model.type ?? '').toLowerCase() === 'asr'), [tenantModels]);

  const dialogMode: UploadDialogMode = pendingReparse ? "reparse" : pendingManual ? "manual" : "file";
  // Vue batchFileExts: files + URL paths + manual markdown media + reparse type.
  const batchExts = useMemo(() => batchUploadExtensions({
    entries: pendingEntries,
    urls: pendingUrl ? [pendingUrl] : [],
    manualContent: dialogMode === "manual" ? pendingManual?.content : undefined,
    reparseFileType: dialogMode === "reparse" ? pendingReparse?.document.file_type : undefined,
  }), [pendingEntries, pendingUrl, pendingManual, pendingReparse, dialogMode]);
  const hasPdf = batchExts.includes("pdf");
  const hasImages = batchHasImages(batchExts, dialogMode === "manual" ? pendingManual?.content : undefined);
  const hasAudio = batchHasAudio(batchExts);
  const multimodalIssue = multimodalSectionIssue({ state: confirmState, hasImages });
  const asrIssue = asrSectionIssue({ state: confirmState, hasAudio });
  const batchItemCount = pendingEntries.length + (pendingUrl ? 1 : 0);
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

  function toggleSelected(id: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function stageFiles(files: Iterable<File>) {
    const entries = toUploadEntries(files);
    if (entries.length === 0) return;
    if (pendingEntries.length === 0) setUploadTargetFolder(folderPath ?? "");
    // Vue appendFiles: dedupe by (relative path|name)+size and report counts.
    const merged = mergeUploadEntries(pendingEntries, entries);
    setPendingEntries(merged.entries);
    setUploadStates([]);
    setUploadError(null);
    if (merged.addedCount > 0) setStageNotice({ tone: "neutral", text: ct("uploadConfirm.filesAdded", { count: merged.addedCount }) });
    else if (merged.duplicateCount > 0) setStageNotice({ tone: "warning", text: ct("uploadConfirm.filesAllDuplicate") });
  }

  function appendMoreUrl() {
    const normalized = normalizeUploadUrl(moreUrl);
    if (!normalized) {
      setUploadError(t("knowledgeBase.documents.url"));
      return;
    }
    if (normalized === pendingUrl) {
      setStageNotice({ tone: "warning", text: ct("uploadConfirm.urlDuplicate") });
      setMoreUrl("");
      return;
    }
    setPendingUrl(normalized);
    setUploadError(null);
    setStageNotice({ tone: "neutral", text: ct("uploadConfirm.urlAdded") });
    setMoreUrl("");
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
    setPendingUrl("");
    setPendingManual(null);
    setUploadTargetFolder("");
    setPendingTagIds([]);
    setUploadStates([]);
    setUploading(false);
    setStageNotice(null);
    setMoreUrl("");
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

  function removeStagedUrl() {
    setPendingUrl("");
  }

  // Sequential uploads (one call per file) with per-file status; a per-file
  // failure keeps the dialog open so the errors stay visible (Vue parity).
  async function confirmUpload() {
    if (dialogMode === "reparse") {
      await confirmReparse();
      return;
    }
    if (!pendingManual && pendingEntries.length === 0 && !pendingUrl) {
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
        setUploadError(errorMessage(error));
      } finally { setUploading(false); }
      return;
    }
    if (pendingUrl) {
      try {
        const created = await client.knowledgeBases.documents.createFromUrl(
          knowledgeBaseId,
          {
            url: pendingUrl,
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
        setPendingUrl("");
        setReloadToken((value) => value + 1);
        emitKnowledgeUploadEvent("knowledgeFileUploaded", { kbId: knowledgeBaseId });
      } catch (error) {
        setUploadError(errorMessage(error));
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
        upload: async (entry, tagIds, signal) => {
          const created = await client.knowledgeBases.documents.upload(
            knowledgeBaseId,
            {
              file: entry.file,
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
        },
        onStateChange: (states) => {
          setUploadStates(states);
          states.forEach((state, index) => emitKnowledgeUploadEvent("knowledgeFileUploadProgress", {
            uploadId: `${uploadBatchId}-${index}`,
            kbId: knowledgeBaseId,
            fileName: state.entry.name,
            progress: state.status === "done" || state.status === "error" ? 100 : state.status === "uploading" ? 50 : 0,
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
      setPendingUrl("");
      setPendingTagIds([]);
      setUploadStates([]);
      setStageNotice(null);
      setMoreUrl("");
      resetDestinationPicker();
      setChunkingMoreOpen(false);
    } catch (error) {
      setUploadError(errorMessage(error));
    } finally {
      if (uploadPipelineController.current === controller)
        uploadPipelineController.current = null;
      setUploading(false);
    }
  }

  async function upload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUploadError(null);
    if (uploadSource === "file") {
      // Files already staged by the input/dropzone: the confirm dialog owns them.
      if (pendingEntries.length > 0) return;
      setUploadError(t("knowledgeBase.documents.file"));
      return;
    }
    if (uploadSource === "url") {
      const normalizedUrl = normalizeUploadUrl(url);
      if (!normalizedUrl) {
        setUploadError(t("knowledgeBase.documents.url"));
        return;
      }
      if (normalizedUrl === pendingUrl) {
        setStageNotice({ tone: "warning", text: ct("uploadConfirm.urlDuplicate") });
        return;
      }
      setPendingUrl(normalizedUrl);
      if (!pendingUrl && pendingEntries.length === 0) setUploadTargetFolder(folderPath ?? "");
      setPendingTagIds([]);
      setUploadError(null);
      setUrl("");
      setStageNotice({ tone: "neutral", text: ct("uploadConfirm.urlAdded") });
      return;
    }
    setUploading(true);
    const controller = new AbortController();
    uploadController.current = controller;
    try {
      if (!manualTitle.trim() || !manualContent.trim())
        throw new Error(t("knowledgeBase.documents.manualTitle"));
      if (confirmState.multimodalEnabled && !confirmState.vllmModelId.trim())
        throw new Error(ct("uploadConfirm.vlmModelSelectRequired"));
      if (confirmState.asrEnabled && !confirmState.asrModelId.trim())
        throw new Error(ct("uploadConfirm.asrModelSelectRequired"));
      setPendingManual({ title: manualTitle.trim(), content: manualContent });
      setPendingTagIds([]);
      setUploadError(null);
      setManualTitle("");
      setManualContent("");
      return;
    } catch (error) {
      setUploadError(errorMessage(error));
    } finally {
      if (uploadController.current === controller)
        uploadController.current = null;
      setUploading(false);
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
      setMutationError(errorMessage(error));
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
      setMutationError(errorMessage(error));
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
      setMutationError(errorMessage(error));
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
      setMutationError(errorMessage(error));
    }
  }

  async function cancelOneParse(id: string) {
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.cancelParse(id);
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error));
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
      setMutationError(errorMessage(error));
    }
  }

  async function updateSelectedTags() {
    if (!selected.size) return;
    const raw = window.prompt(
      "Set tag IDs for selected documents (comma-separated; empty clears tags):",
      tagId,
    );
    if (raw === null) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.documents.updateTags(
        Object.fromEntries(
          [...selected].map((id) => [
            id,
            raw
              .split(",")
              .map((value) => value.trim())
              .filter(Boolean),
          ]),
        ),
      );
      setReloadToken((value) => value + 1);
    } catch (error) {
      setMutationError(errorMessage(error));
    }
  }

  function goToSection(key: UploadConfirmSectionKey) {
    document.getElementById(`wk-upload-section-${key}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

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
    return items;
  }, [confirmState, pendingTagIds.length, hasPdf, hasImages, hasAudio, tenantModels, dialogMode, ct, t, multimodalIssue, asrIssue]);

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
  const uploadDialogOpen = canContribute && (pendingEntries.length > 0 || !!pendingUrl || !!pendingManual || !!pendingReparse);
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
    <main className="wk-page wk-documents-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p>
          <h1>{t("knowledgeBase.documents.title")}</h1>
          <p className="wk-muted">{t("knowledgeBase.documents.subtitle")}</p>
          {!canContribute ? (
            <Status tone="warning">
              {t("knowledgeBase.documents.viewerReadonly")}
            </Status>
          ) : null}
        </div>
        <div className="wk-list-actions">
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
      <Card>
        {canContribute ? (
          <form className="wk-upload-panel" onSubmit={upload}>
            <div className="wk-toolbar">
              <label>
                {t("knowledgeBase.documents.source")}{" "}
                <select
                  value={uploadSource}
                  onChange={(event) =>
                    setUploadSource(event.target.value as UploadSource)
                  }
                >
                  <option value="file">
                    {t("knowledgeBase.documents.sourceFile")}
                  </option>
                  <option value="url">
                    {t("knowledgeBase.documents.sourceUrl")}
                  </option>
                  <option value="manual">
                    {t("knowledgeBase.documents.sourceManual")}
                  </option>
                </select>
              </label>
              {uploadSource === "file" ? (
                <label>
                  {t("knowledgeBase.documents.file")}{" "}
                  <input
                    type="file"
                    multiple
                    onChange={(event) => {
                      stageFiles(event.target.files ?? []);
                      event.target.value = "";
                    }}
                  />
                </label>
              ) : null}
              {uploadSource === "url" ? (
                <label>
                  {t("knowledgeBase.documents.url")}{" "}
                  <input
                    value={url}
                    onChange={(event) => setUrl(event.target.value)}
                    placeholder="https://…"
                  />
                </label>
              ) : null}
              {uploadSource === "manual" ? (
                <>
                  <label>
                    {t("knowledgeBase.documents.manualTitle")}{" "}
                    <input
                      value={manualTitle}
                      onChange={(event) => setManualTitle(event.target.value)}
                    />
                  </label>
                  <label>
                    {t("knowledgeBase.documents.manualContent")}{" "}
                    <textarea
                      value={manualContent}
                      onChange={(event) => setManualContent(event.target.value)}
                      rows={2}
                    />
                  </label>
                </>
              ) : null}
              <Button type="submit" loading={uploading}>
                {uploadSource === "file"
                  ? t("knowledgeBase.documents.uploadFile")
                  : uploadSource === "url"
                    ? t("knowledgeBase.documents.importUrl")
                    : t("knowledgeBase.documents.createDocument")}
              </Button>
              {uploading ? (
                <Button
                  type="button"
                  onClick={() => uploadController.current?.abort()}
                >
                  {t("knowledgeBase.documents.cancel")}
                </Button>
              ) : null}
            </div>
            {uploadError && !uploadDialogOpen ? <Status tone="error">{uploadError}</Status> : null}
          </form>
        ) : null}
        <div
          className={
            dragActive && canContribute
              ? "wk-documents-layout wk-dropzone is-active"
              : "wk-documents-layout wk-dropzone"
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
            <p className="wk-dropzone-hint" role="status">
              Drop files to stage them for upload
            </p>
          ) : null}
          <aside className="wk-folder-panel">
            <strong>{t("knowledgeBase.documents.folders")}</strong>
            {folderState.status === "loading" ? (
              <Status>{t("knowledgeBase.documents.loadingFolders")}</Status>
            ) : null}
            {folderState.status === "error" ? (
              <Status tone="error">{folderState.message}</Status>
            ) : null}
            <ul className="wk-folder-list">
              {folders.map((folder) => (
                <li
                  key={folder.path}
                  style={{ paddingLeft: `${folder.depth * 0.8}rem` }}
                >
                  <button
                    type="button"
                    className={
                      folderPath === (folder.path || undefined)
                        ? "is-active"
                        : ""
                    }
                    onClick={() => setFolderPath(folder.path || undefined)}
                  >
                    {folder.name} <span>{folder.total_count}</span>
                  </button>
                </li>
              ))}
            </ul>
          </aside>
          <section className="wk-document-results">
            <div className="wk-toolbar" role="search">
              <label>
                {t("knowledgeBase.documents.search")}{" "}
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("knowledgeBase.documents.searchPlaceholder")}
                />
              </label>
              <label>
                {t("knowledgeBase.documents.status")}{" "}
                <select
                  value={parseStatus}
                  onChange={(event) => setParseStatus(event.target.value)}
                >
                  <option value="">
                    {t("knowledgeBase.documents.allStatuses")}
                  </option>
                  <option value="pending">Pending</option>
                  <option value="processing">Processing</option>
                  <option value="finalizing">Finalizing</option>
                  <option value="completed">Completed</option>
                  <option value="failed">Failed</option>
                  <option value="deleting">Deleting</option>
                  <option value="cancelled">Cancelled</option>
                </select>
              </label>
              <label>
                {t("knowledgeBase.documents.tag")}{" "}
                <select
                  value={tagId}
                  onChange={(event) => setTagId(event.target.value)}
                >
                  <option value="">
                    {t("knowledgeBase.documents.allTags")}
                  </option>
                  {tags.map((tag) => (
                    <option key={tag.id} value={tag.id}>
                      {tag.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="wk-list-actions">
              <span>
                {t("knowledgeBase.documents.selectedOnPage", {
                  count: selectedOnPage,
                })}
                {selected.size > selectedOnPage
                  ? ` · ${t("knowledgeBase.documents.selectedTotal", { count: selected.size })}`
                  : ""}
              </span>
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
                  <Button
                    type="button"
                    disabled={!selected.size}
                    onClick={() => void updateSelectedTags()}
                  >
                    {t("knowledgeBase.documents.setTags")}
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
            </div>
            {moving && canContribute ? (
              <div
                className="wk-list-actions"
                role="form"
                aria-label={t("knowledgeBase.documents.moveDestination")}
              >
                <label>
                  {t("knowledgeBase.documents.moveDestination")}{" "}
                  <select
                    value={moveTarget}
                    onChange={(event) => setMoveTarget(event.target.value)}
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
                  </select>
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
              <Status>{t("knowledgeBase.documents.noDocuments")}</Status>
            ) : null}
            {state.status === "success" && items.length > 0 ? (
              <ul className="wk-list wk-document-list">
                {items.map((document) => {
                  const status = documentStatus(document, t);
                  const actions = documentRowActions(document.parse_status);
                  return (
                    <li key={document.id}>
                      <input
                        type="checkbox"
                        aria-label={t("knowledgeBase.documents.select", {
                          name: displayName(document),
                        })}
                        checked={selected.has(document.id)}
                        onChange={() => toggleSelected(document.id)}
                      />
                      <div className="wk-list-item-copy">
                        <button
                          type="button"
                          className="wk-document-link"
                          onClick={() => onOpenDocument?.(document)}
                        >
                          {displayName(document)}
                        </button>
                        <span>
                          {document.folder_path ||
                            t("knowledgeBase.documents.root")}
                          {document.file_type ? ` · ${document.file_type}` : ""}
                          {document.source ? ` · ${document.source}` : ""}
                        </span>
                      </div>
                      <Status tone={status.tone}>{status.label}</Status>
                      {canContribute ? (
                        <span className="wk-row-actions">
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
      </Card>
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
              <span className="wk-muted">{ct("uploadConfirm.parseConfig")}</span>
            </p>
          ) : null}
          <UploadFilesPanel
            mode={dialogMode}
            entries={pendingEntries}
            urls={pendingUrl ? [pendingUrl] : []}
            uploadStates={uploadStates}
            manualTitle={pendingManual?.title}
            manualCharCount={pendingManual?.content.length}
            reparseFileName={pendingReparse ? displayName(pendingReparse.document) : undefined}
            uploading={uploading}
            labels={filesPanelLabels}
            onRemoveUrl={removeStagedUrl}
            onRemoveEntry={removeStagedUpload}
          />
          {dialogMode === "file" && !uploading ? (
            <div className="wk-list-actions" style={{ marginBottom: "0.75rem" }}>
              <label>
                {ct("uploadConfirm.continueAdd")}{" "}
                <input
                  type="file"
                  multiple
                  onChange={(event) => {
                    stageFiles(event.target.files ?? []);
                    event.target.value = "";
                  }}
                />
              </label>
              <label>
                {t("knowledgeBase.documents.url")}{" "}
                <input
                  value={moreUrl}
                  onChange={(event) => setMoreUrl(event.target.value)}
                  placeholder="https://…"
                />
              </label>
              <Button type="button" onClick={appendMoreUrl}>
                {t("knowledgeBase.documents.importUrl")}
              </Button>
            </div>
          ) : null}
          {stageNotice ? <Status tone={stageNotice.tone === "warning" ? "warning" : "neutral"}>{stageNotice.text}</Status> : null}
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
                <span className="wk-muted">{ct("uploadConfirm.destinationLabel")}</span>
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
            <fieldset className="wk-upload-confirm-tags" id="wk-upload-section-tags" data-section="tags">
              <legend>{ct("uploadConfirm.tabTags")}</legend>
              <p className="wk-muted" style={{ margin: "0 0 0.4rem", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsDescription")}</p>
              <label>
                <span className="wk-visually-hidden">{ct("uploadConfirm.tagsPlaceholder")}</span>
                <select
                  multiple
                  value={pendingTagIds}
                  onChange={(event) =>
                    setPendingTagIds(
                      Array.from(event.target.selectedOptions).map(
                        (option) => option.value,
                      ),
                    )
                  }
                >
                  {tags.map((tag) => (
                    <option key={tag.id} value={tag.id}>
                      {tag.name}
                    </option>
                  ))}
                </select>
              </label>
              {!uploading && tags.length === 0 ? (
                <p className="wk-muted" style={{ margin: "0.25rem 0 0", fontSize: "0.85rem" }}>{ct("uploadConfirm.tagsEmpty")}</p>
              ) : null}
            </fieldset>
          ) : null}
          <UploadSectionNav items={sectionNavItems} navLabel={ct("uploadConfirm.configNav")} onSelect={goToSection} />
          <UploadConfirmSections
            state={confirmState}
            update={updateConfirm}
            hasPdf={hasPdf}
            multimodalIssue={multimodalIssue}
            asrIssue={asrIssue}
            parserEngines={parserEngines}
            vllmModels={vllmModels}
            asrModels={asrModels}
            moreOpen={chunkingMoreOpen}
            onToggleMore={() => setChunkingMoreOpen((open) => !open)}
            t={ct}
          />
          {uploadError ? <Status tone="error">{uploadError}</Status> : null}
          <div className="wk-list-actions">
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
          <div className="wk-list-actions">
            <Button type="button" onClick={() => void deleteSelected()}>
              {t("knowledgeBase.documents.delete")}
            </Button>
            <Button type="button" onClick={() => setConfirmingDelete(false)}>
              {t("knowledgeBase.documents.cancel")}
            </Button>
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
          <div className="wk-list-actions">
            <Button type="button" onClick={() => void confirmBatchReparse()}>
              {ct("uploadConfirm.confirmReparse")}
            </Button>
            <Button type="button" onClick={() => setPendingBatchReparse(null)}>
              {ct("uploadConfirm.cancel")}
            </Button>
          </div>
        </Dialog>
      ) : null}
    </main>
  );
}
