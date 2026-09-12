import { useEffect, useMemo, useRef, useState } from "react";
import type { KnowledgeDocument, ParserEngineInfo, WeKnoraClient } from "@weknora/api-client";
import {
  processingStatusLabel,
  normalizeKnowledgeProcessingStatus,
} from "@weknora/domain/knowledge/processing";
import { flattenKnowledgeFolders as flattenFolders } from "@weknora/domain/knowledge/folders";
import { Button, Card, Dialog, Status } from "@weknora/ui";
import { createTranslator, useAppLocale } from "../i18n.ts";
import {
  formatBytes,
  normalizeUploadUrl,
  removeUploadEntry,
  runUploadPipeline,
  toUploadEntries,
  uploadSummary,
  type UploadEntry,
  type UploadEntryState,
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
  reparseDocument,
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

export function KnowledgeDocumentsPage({
  client,
  knowledgeBaseId,
  onOpenDocument,
}: KnowledgeDocumentsPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
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
  const [pendingTagIds, setPendingTagIds] = useState<string[]>([]);
  const [chunkSize, setChunkSize] = useState(512);
  const [chunkOverlap, setChunkOverlap] = useState(50);
  const [chunkStrategy, setChunkStrategy] = useState("auto");
  const [parserEngines, setParserEngines] = useState<ParserEngineInfo[]>([]);
  const [parserRules, setParserRules] = useState<Array<{ file_types: string[]; engine: string }>>([]);
  const [multimodalEnabled, setMultimodalEnabled] = useState(false);
  const [vllmModelId, setVllmModelId] = useState("");
  const [descriptionLanguage, setDescriptionLanguage] = useState("");
  const [customInstructions, setCustomInstructions] = useState("");
  const [asrEnabled, setAsrEnabled] = useState(false);
  const [asrModelId, setAsrModelId] = useState("");
  const [asrLanguage, setAsrLanguage] = useState("");
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
  const pageSize = 20;

  // Audit #6: KB-type routing — an FAQ KB must land on the FAQ route.
  // The same fetch drives permission gating and tab visibility.
  useEffect(() => {
    let active = true;
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
    ])
      .then(([kb, me]) => {
        if (!active) return;
        setKbMeta(kb as KBSurfaceKB);
        const chunking =
          (kb as KBSurfaceKB & { chunking_config?: Record<string, unknown> })
            .chunking_config ?? {};
        if (typeof chunking.chunk_size === "number")
          setChunkSize(chunking.chunk_size);
        if (typeof chunking.chunk_overlap === "number")
          setChunkOverlap(chunking.chunk_overlap);
        if (typeof chunking.strategy === "string" && chunking.strategy)
          setChunkStrategy(chunking.strategy);
        if (Array.isArray(chunking.parser_engine_rules)) {
          setParserRules(chunking.parser_engine_rules.flatMap((rule: unknown) => {
            if (!rule || typeof rule !== "object") return [];
            const row = rule as Record<string, unknown>;
            return typeof row.engine === "string" && Array.isArray(row.file_types) && row.file_types.every((item) => typeof item === "string") ? [{ file_types: row.file_types as string[], engine: row.engine }] : [];
          }));
        }
        const config = kb as KBSurfaceKB & { vlm_config?: Record<string, unknown>; asr_config?: Record<string, unknown> };
        const vlm = config.vlm_config ?? {};
        setMultimodalEnabled(vlm.enabled === true || (kb as KBSurfaceKB & { enable_multimodel?: boolean }).enable_multimodel === true);
        setVllmModelId(typeof vlm.model_id === "string" ? vlm.model_id : "");
        setDescriptionLanguage(typeof vlm.description_language === "string" ? vlm.description_language : "");
        setCustomInstructions(typeof vlm.custom_instructions === "string" ? vlm.custom_instructions : "");
        const asr = config.asr_config ?? {};
        setAsrEnabled(asr.enabled === true);
        setAsrModelId(typeof asr.model_id === "string" ? asr.model_id : "");
        setAsrLanguage(typeof asr.language === "string" ? asr.language : "");
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
  const items = state.status === "success" ? state.page.items : [];
  const selectedOnPage = items.filter((item) => selected.has(item.id)).length;
  const pageTotal = state.status === "success" ? state.page.total : 0;
  const tabs = useMemo(
    () => (kbMeta ? resolveKBSurfaceTabs(kbMeta) : ["documents" as const]),
    [kbMeta],
  );

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
    setPendingEntries(entries);
    setUploadStates([]);
    setUploadError(null);
  }

  function cancelStagedUploads() {
    uploadPipelineController.current?.abort();
    uploadPipelineController.current = null;
    setPendingEntries([]);
    setPendingUrl("");
    setPendingTagIds([]);
    setUploadStates([]);
    setUploading(false);
  }

  function removeStagedUpload(index: number) {
    if (uploading) return;
    setPendingEntries((current) => removeUploadEntry(current, index));
    setUploadStates((current) =>
      current.filter((_, stateIndex) => stateIndex !== index),
    );
  }

  function buildProcessConfig() {
    return {
      parser_engine_rules: parserRules,
      enable_multimodel: multimodalEnabled,
      vlm_config: { enabled: multimodalEnabled, model_id: vllmModelId.trim(), description_language: descriptionLanguage.trim(), custom_instructions: customInstructions.trim() },
      asr_config: { enabled: asrEnabled, model_id: asrModelId.trim(), language: asrLanguage.trim() },
      chunking_config: { chunk_size: chunkSize, chunk_overlap: chunkOverlap, strategy: chunkStrategy },
    };
  }

  // Sequential uploads (one call per file) with per-file status; a per-file
  // failure keeps the dialog open so the errors stay visible (Vue parity).
  async function confirmUpload() {
    if (pendingEntries.length === 0 && !pendingUrl) return;
    if (!Number.isInteger(chunkSize) || chunkSize < 100 || chunkSize > 4000) {
      setUploadError(t("knowledgeEditor.chunking.sizeDescription"));
      return;
    }
    if (
      !Number.isInteger(chunkOverlap) ||
      chunkOverlap < 0 ||
      chunkOverlap > 500 ||
      chunkOverlap >= chunkSize
    ) {
      setUploadError(t("knowledgeEditor.chunking.overlapDescription"));
      return;
    }
    if (multimodalEnabled && !vllmModelId.trim()) {
      setUploadError("A VLM model is required when multimodal parsing is enabled.");
      return;
    }
    if (asrEnabled && !asrModelId.trim()) {
      setUploadError("An ASR model is required when audio transcription is enabled.");
      return;
    }
    const processConfig = buildProcessConfig();
    setUploadError(null);
    setUploading(true);
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
        if (folderPath !== undefined && created.id) {
          await client.knowledgeBases.documents.moveToFolder(
            knowledgeBaseId,
            [created.id],
            folderPath,
          );
        }
        setPendingUrl("");
        setPendingTagIds([]);
        setReloadToken((value) => value + 1);
      } catch (error) {
        setUploadError(errorMessage(error));
      } finally {
        setUploading(false);
      }
      return;
    }
    const controller = new AbortController();
    uploadPipelineController.current = controller;
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
          if (folderPath !== undefined && created.id) {
            await client.knowledgeBases.documents.moveToFolder(
              knowledgeBaseId,
              [created.id],
              folderPath,
            );
          }
          return created;
        },
        onStateChange: setUploadStates,
      });
      setReloadToken((value) => value + 1);
      if (finalStates.some((state) => state.status === "error")) return;
      setPendingEntries([]);
      setPendingUrl("");
      setPendingTagIds([]);
      setUploadStates([]);
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
      setPendingUrl(normalizedUrl);
      setPendingTagIds([]);
      setUploadError(null);
      setUrl("");
      return;
    }
    setUploading(true);
    const controller = new AbortController();
    uploadController.current = controller;
    try {
      if (!manualTitle.trim() || !manualContent.trim())
        throw new Error(t("knowledgeBase.documents.manualTitle"));
      if (multimodalEnabled && !vllmModelId.trim())
        throw new Error("A VLM model is required when multimodal parsing is enabled.");
      if (asrEnabled && !asrModelId.trim())
        throw new Error("An ASR model is required when audio transcription is enabled.");
      await client.knowledgeBases.documents.createManual(knowledgeBaseId, {
        title: manualTitle.trim(),
        content: manualContent,
        status: "pending",
        process_config: buildProcessConfig(),
      });
      setUrl("");
      setManualTitle("");
      setManualContent("");
      setReloadToken((value) => value + 1);
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

  // Audit must-fix #1: batch reparse (POST /api/v1/knowledge/batch-reparse).
  async function reparseSelected() {
    if (!selected.size) return;
    setMutationError(null);
    try {
      await client.knowledgeBases.batchReparse(knowledgeBaseId, [...selected]);
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

  async function reparseOne(id: string) {
    setMutationError(null);
    try {
      await reparseDocument(client.knowledgeBases.documents, id);
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
            {uploadError ? <Status tone="error">{uploadError}</Status> : null}
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
                          <Button
                            type="button"
                            onClick={() => void reparseOne(document.id)}
                          >
                            {t("knowledgeBase.documents.reparse")}
                          </Button>
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
      {(pendingEntries.length > 0 || pendingUrl) && canContribute ? (
        <Dialog open title="Confirm upload" onClose={cancelStagedUploads}>
          <p className="wk-upload-confirm-summary">
            {pendingUrl
              ? "1 URL ready to import."
              : `${uploadSummary(pendingEntries).count} file(s), ${uploadSummary(pendingEntries).totalLabel} total. Large files are chunked server-side using the knowledge base chunk configuration.`}
          </p>
          {pendingUrl ? (
            <ul className="wk-upload-confirm-files">
              <li>
                <span>{pendingUrl}</span>
                <span>URL</span>
                <Button
                  type="button"
                  disabled={uploading}
                  onClick={cancelStagedUploads}
                >
                  Remove
                </Button>
              </li>
            </ul>
          ) : null}
          {pendingEntries.length > 0 ? (
            <ul className="wk-upload-confirm-files">
              {pendingEntries.map((entry, index) => {
                const state = uploadStates[index];
                return (
                  <li key={entry.name + index}>
                    <span>{entry.name}</span>
                    <span>{formatBytes(entry.size)}</span>
                    <Status
                      tone={
                        state?.status === "done"
                          ? "success"
                          : state?.status === "error"
                            ? "error"
                            : state?.status === "uploading"
                              ? "warning"
                              : "neutral"
                      }
                    >
                      {state?.status === "done"
                        ? "Uploaded"
                        : state?.status === "error"
                          ? (state.message ?? "Failed")
                          : state?.status === "uploading"
                            ? "Uploading…"
                            : "Pending"}
                    </Status>
                    <Button
                      type="button"
                      disabled={uploading}
                      onClick={() => removeStagedUpload(index)}
                    >
                      Remove
                    </Button>
                  </li>
                );
              })}
            </ul>
          ) : null}
          <label className="wk-upload-confirm-tags">
            Tags{" "}
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
          <fieldset className="wk-upload-confirm-chunking">
            <legend>{t("knowledgeEditor.chunking.title")}</legend>
            <label>
              {t("knowledgeEditor.chunking.sizeLabel")}{" "}
              <input
                type="number"
                min={100}
                max={4000}
                value={chunkSize}
                onChange={(event) => setChunkSize(Number(event.target.value))}
              />
            </label>
            <label>
              {t("knowledgeEditor.chunking.overlapLabel")}{" "}
              <input
                type="number"
                min={0}
                max={500}
                value={chunkOverlap}
                onChange={(event) =>
                  setChunkOverlap(Number(event.target.value))
                }
              />
            </label>
            <label>
              {t("knowledgeEditor.chunking.strategyLabel")}{" "}
              <select
                value={chunkStrategy}
                onChange={(event) => setChunkStrategy(event.target.value)}
              >
                <option value="auto">Auto</option>
                <option value="heading">Heading</option>
                <option value="heuristic">Heuristic</option>
                <option value="legacy">Legacy</option>
              </select>
            </label>
          </fieldset>
          <fieldset className="wk-upload-confirm-parser">
            <legend>Parser engine</legend>
            <p className="wk-muted">Choose an available server parser for supported file types; blank keeps the server default.</p>
            {parserEngines.length === 0 ? <p className="wk-muted">No parser engine registry was returned.</p> : [...new Set(parserEngines.flatMap((engine) => engine.FileTypes ?? []))].filter((fileType) => fileType !== "url").sort().map((fileType) => <label key={fileType}>.{fileType}<select value={parserRules.find((rule) => rule.file_types.includes(fileType))?.engine ?? ""} onChange={(event) => setParserRules((current) => { const remaining = current.filter((rule) => !rule.file_types.includes(fileType)); return event.target.value ? [...remaining, { file_types: [fileType], engine: event.target.value }] : remaining; })}><option value="">Server default</option>{parserEngines.filter((engine) => (engine.FileTypes ?? []).includes(fileType)).map((engine) => <option key={engine.Name} value={engine.Name} disabled={engine.Available === false}>{engine.Name}{engine.Available === false ? ` — ${engine.UnavailableReason || "unavailable"}` : ""}</option>)}</select></label>)}
          </fieldset>
          <fieldset className="wk-upload-confirm-multimodal">
            <legend>Multimodal parsing</legend>
            <label className="wk-checkbox"><input type="checkbox" checked={multimodalEnabled} onChange={(event) => setMultimodalEnabled(event.target.checked)} /> Enable VLM descriptions</label>
            {multimodalEnabled ? <>
              <label>VLM model ID <input required value={vllmModelId} onChange={(event) => setVllmModelId(event.target.value)} placeholder="tenant model id" /></label>
              <label>Description language <input value={descriptionLanguage} onChange={(event) => setDescriptionLanguage(event.target.value)} placeholder="Auto" /></label>
              <label>Custom instructions <textarea rows={3} value={customInstructions} onChange={(event) => setCustomInstructions(event.target.value)} /></label>
            </> : null}
          </fieldset>
          <fieldset className="wk-upload-confirm-asr">
            <legend>Audio transcription</legend>
            <label className="wk-checkbox"><input type="checkbox" checked={asrEnabled} onChange={(event) => setAsrEnabled(event.target.checked)} /> Enable ASR</label>
            {asrEnabled ? <><label>ASR model ID <input required value={asrModelId} onChange={(event) => setAsrModelId(event.target.value)} placeholder="tenant model id" /></label><label>Language <input value={asrLanguage} onChange={(event) => setAsrLanguage(event.target.value)} placeholder="Auto" /></label></> : null}
          </fieldset>
          {uploadError ? <Status tone="error">{uploadError}</Status> : null}
          <div className="wk-list-actions">
            <Button
              type="button"
              loading={uploading}
              onClick={() => void confirmUpload()}
            >
              {pendingUrl
                ? "Import URL"
                : `Upload ${pendingEntries.length} file(s)`}
            </Button>
            <Button type="button" onClick={cancelStagedUploads}>
              {t("knowledgeBase.documents.cancel")}
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
    </main>
  );
}
