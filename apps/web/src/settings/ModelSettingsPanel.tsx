import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import * as React from "react";
import type {
  ModelConfiguration,
  WeKnoraClient,
} from "@weknora/api-client";
import { Button, Card, Input, NumberInput, Status, Switch } from "@weknora/ui";
import { roleAtLeast } from "@weknora/views/settings/registry";
import { ModelDebugPanel } from "./ModelDebugPanel.tsx";
import { ModelOptionSelect } from "./ModelOptionSelect.tsx";
import { ModelUsageNotice } from "../configuration/ModelUsageNotice.tsx";
import { navigate } from "../platform/navigation.ts";
import { modelInUseDetails, type ModelUsageDetails } from "../configuration/model-usage.ts";
import { useAppLocale } from "../i18n.ts";
import {
  baseUrlPlaceholderKey,
  createModelTranslator,
  customHeadersMap,
  DEFAULT_MODEL_CONTEXT_WINDOW,
  defaultThinkingControl,
  effectiveContextWindow,
  fallbackProviderOptions,
  formatContextWindow,
  formatModelSize,
  modelFieldErrorKey,
  isDefaultContextWindow,
  modelDraftFromRecord,
  modelNamePlaceholderKey,
  modelPayload,
  modelType,
  modelValidationErrorKey,
  newModelDraft,
  providerDefaultUrl,
  providerText,
  signedRerankProvider,
  storedThinkingControl,
  subsectionToFilter,
  validateModelDraft,
  type CustomHeaderItem,
  type ModelDraft,
  type ModelProviderOption,
  type ModelType,
} from "./model-settings.ts";

type Props = {
  client: WeKnoraClient;
  role: "viewer" | "admin" | "owner" | "system-admin";
  initialModels: readonly ModelConfiguration[];
  /** settingsInitialSubSection deep link (ModelSettings.vue lines 329-337). */
  initialSubSection?: string;
};
const TYPES: ModelType[] = ["chat", "embedding", "rerank", "vllm", "asr"];
// Tailwind 迁移：原 settings-wrapper.css 的 .model-card--<type> .model-card__badge
// 配色改为静态映射 utilities（rgba/hex 任意值精确还原）。
const CARD_BADGE_TONE: Record<ModelType, string> = {
  chat: "bg-[rgba(0,82,217,0.1)] text-[#0052d9]",
  embedding: "bg-[rgba(98,53,187,0.1)] text-[#6235bb]",
  rerank: "bg-[rgba(184,92,0,0.12)] text-[#b85c00]",
  vllm: "bg-[rgba(201,62,62,0.1)] text-[#c93e3e]",
  asr: "bg-[rgba(17,128,83,0.1)] text-[#118053]",
};
const BUILTIN_MODELS_DOC = "https://github.com/Tencent/WeKnora/blob/main/docs/BUILTIN_MODELS.md";
const THINKING_CONTROL_OPTIONS: Array<{ value: string; key: string }> = [
  { value: "none", key: "none" },
  { value: "chat_template_kwargs", key: "chatTemplateKwargs" },
  { value: "enable_thinking", key: "enableThinking" },
  { value: "thinking_type", key: "thinkingType" },
];
type WkcCredentialState = "loading" | "unconfigured" | "configured" | "expired";

function params(model: ModelConfiguration): Record<string, unknown> {
  return model.parameters &&
    typeof model.parameters === "object" &&
    !Array.isArray(model.parameters)
    ? model.parameters
    : {};
}
function label(model: ModelConfiguration): string {
  const value = (model as Record<string, unknown>).display_name;
  return typeof value === "string" && value.trim() ? value : model.name;
}
function isBuiltin(model: ModelConfiguration): boolean {
  return (model as Record<string, unknown>).is_builtin === true;
}
function payloadString(value: unknown, keys: string[]): string | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  for (const key of keys) {
    const candidate = (value as Record<string, unknown>)[key];
    if (typeof candidate === "string" && candidate.trim()) return candidate;
  }
  return null;
}
function payloadNumber(value: unknown, key: string): number | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const candidate = (value as Record<string, unknown>)[key];
  return typeof candidate === "number" && Number.isFinite(candidate) ? candidate : undefined;
}
function toNumberInput(value: string): number | "" {
  if (value === "") return "";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : "";
}

export function ModelSettingsPanel({ client, role, initialModels, initialSubSection }: Props) {
  const locale = useAppLocale();
  const t = useMemo(() => createModelTranslator(locale), [locale]);
  const [models, setModels] =
    useState<readonly ModelConfiguration[]>(initialModels);
  // Deep link: a subsection query value preselects the type tab
  // (ModelSettings.vue watches uiStore.settingsInitialSubSection).
  const [filter, setFilter] = useState<"all" | ModelType>(() => subsectionToFilter(initialSubSection) ?? "all");
  useEffect(() => {
    const next = subsectionToFilter(initialSubSection);
    if (next) setFilter(next);
  }, [initialSubSection]);
  const [draft, setDraft] = useState<ModelDraft | null>(null);
  const [providerOptions, setProviderOptions] = useState<ModelProviderOption[]>([]);
  const [loadingProviders, setLoadingProviders] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [draftError, setDraftError] = useState<string | null>(null);
  const [debugOpen, setDebugOpen] = useState(false);
  // Per-field blur validation (ModelEditorDialog.vue rules, lines 907-946).
  const [nameError, setNameError] = useState<string | null>(null);
  const [baseUrlError, setBaseUrlError] = useState<string | null>(null);
  // Ollama combobox dropdown state (ModelEditorDialog.vue filterable select).
  const [ollamaOpen, setOllamaOpen] = useState(false);
  const [ollamaHighlight, setOllamaHighlight] = useState(0);
  // Which card action menu is open (ModelSettings.vue ellipsis dropdown).
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [usageConflict, setUsageConflict] = useState<{ modelName: string; details: ModelUsageDetails } | null>(null);
  const [ollamaStatus, setOllamaStatus] = useState<boolean | null>(null);
  const [ollamaModels, setOllamaModels] = useState<Awaited<ReturnType<WeKnoraClient["settings"]["ollama"]["models"]>>>([]);
  const [ollamaBusy, setOllamaBusy] = useState(false);
  const [downloadTask, setDownloadTask] = useState<string | null>(null);
  const [downloadProgress, setDownloadProgress] = useState<number | null>(null);
  const [checking, setChecking] = useState(false);
  const [remoteMessage, setRemoteMessage] = useState<{ ok: boolean; text: string } | null>(null);
  const [dimensionMessage, setDimensionMessage] = useState<{ ok: boolean; text: string } | null>(null);
  const [wkcState, setWkcState] = useState<WkcCredentialState>("loading");
  const [thinkingManual, setThinkingManual] = useState(false);
  const [credentialValues, setCredentialValues] = useState<{ apiKey: string; appSecret: string }>({ apiKey: "", appSecret: "" });
  const [credentialBusy, setCredentialBusy] = useState(false);
  const downloadTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  // Vue authStore.hasRole('admin') (ModelSettings.vue L49/129) — a system
  // admin outranks owner in SettingsRole ranking, so roleAtLeast keeps the
  // add-model card + 模型测试 entry visible when the router folds the
  // is_system_admin session to 'system-admin'.
  const canCreate = roleAtLeast(role, "admin");
  const visible = useMemo(
    () =>
      filter === "all"
        ? models
        : models.filter((item) => modelType(item) === filter),
    [filter, models],
  );

  useEffect(() => {
    setModels(initialModels);
  }, [initialModels]);

  function stopDownloadPolling() {
    if (downloadTimerRef.current) {
      clearInterval(downloadTimerRef.current);
      downloadTimerRef.current = null;
    }
  }
  useEffect(() => stopDownloadPolling, []);

  /* Provider catalogue — API list first, i18n labels win over API text,
     hardcoded fallback filtered by model type (ModelEditorDialog.vue
     loadProviders + providerOptions computed). */
  async function fetchProviderOptions(type: ModelType): Promise<ModelProviderOption[]> {
    try {
      const items = await client.configuration.models.providers.list(type);
      if (items.length > 0) {
        return items.map((item) => ({
          value: item.value,
          label: providerText(t, item.value, "label", item.label),
          description: providerText(t, item.value, "description", item.description),
          defaultUrls: item.defaultUrls,
          modelTypes: item.modelTypes,
        }));
      }
    } catch {
      // fall through to the hardcoded catalogue
    }
    return fallbackProviderOptions(t, type);
  }
  useEffect(() => {
    if (!draft) return;
    let active = true;
    setLoadingProviders(true);
    void fetchProviderOptions(draft.type)
      .then((options) => { if (active) setProviderOptions(options); })
      .finally(() => { if (active) setLoadingProviders(false); });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, draft?.type, t]);

  /* Ollama service status — checked whenever the editor opens; the Local
     option stays disabled while Ollama is down (ModelEditorDialog.vue
     checkOllamaServiceStatus + source-options disabled binding). */
  const editorKey = draft ? (draft.id ?? "new") : null;
  useEffect(() => {
    if (!draft) {
      setOllamaStatus(null);
      setOllamaModels([]);
      return;
    }
    let active = true;
    setOllamaBusy(true);
    void client.settings.ollama.status()
      .then((status) => { if (active) setOllamaStatus(status.available === true); })
      .catch(() => { if (active) setOllamaStatus(false); })
      .finally(() => { if (active) setOllamaBusy(false); });
    return () => { active = false; };
    // editorKey changes on every editor session (add or edit) so the status is
    // re-checked on each open like ModelEditorDialog.vue's visible watcher.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, editorKey]);

  useEffect(() => {
    if (!draft || draft.source !== "local") return;
    let active = true;
    void client.settings.ollama.models()
      .then((items) => { if (active) setOllamaModels(items); })
      .catch(() => { if (active) setOllamaModels([]); });
    return () => { active = false; };
  }, [client, draft?.source]);

  /* WeKnoraCloud credential gate — configured / unconfigured / expired
     (ModelEditorDialog.vue checkWkcCredentialStatus). */
  useEffect(() => {
    if (!draft || draft.provider !== "weknoracloud") {
      setWkcState("loading");
      return;
    }
    let active = true;
    setWkcState("loading");
    void client.settings.weknoraCloud.status()
      .then((status) => {
        if (!active) return;
        const row = status as Record<string, unknown>;
        if (row.needs_reinit === true) setWkcState("expired");
        else if (row.has_models === true) setWkcState("configured");
        else setWkcState("unconfigured");
      })
      .catch(() => { if (active) setWkcState("unconfigured"); });
    return () => { active = false; };
  }, [client, draft?.provider]);

  function updateDraft<K extends keyof ModelDraft>(key: K, value: ModelDraft[K]) {
    setDraft((current) => (current ? { ...current, [key]: value } : current));
  }

  function resetEditorFeedback() {
    setError(null);
    setNotice(null);
    setDraftError(null);
    setRemoteMessage(null);
    setDimensionMessage(null);
    setUsageConflict(null);
  }
  function openAdd() {
    stopDownloadPolling();
    setDownloadTask(null);
    setDownloadProgress(null);
    setCredentialValues({ apiKey: "", appSecret: "" });
    setThinkingManual(false);
    resetEditorFeedback();
    const previous = preservedDraftRef.current;
    if (previous && !previous.id) {
      setDraft(previous);
    } else {
      setDraft(newModelDraft());
    }
    preservedDraftRef.current = null;
    setNameError(null);
    setBaseUrlError(null);
    setOllamaOpen(false);
  }
  function openEdit(model: ModelConfiguration) {
    stopDownloadPolling();
    setDownloadTask(null);
    setDownloadProgress(null);
    setCredentialValues({ apiKey: "", appSecret: "" });
    setThinkingManual(Boolean(storedThinkingControl(model)));
    resetEditorFeedback();
    preservedDraftRef.current = null;
    setNameError(null);
    setBaseUrlError(null);
    setOllamaOpen(false);
    setDraft(modelDraftFromRecord(model));
  }
  /* Editor session state: Vue keeps the live form inside ModelEditorDialog
     and only preserves it when an ADD session was dismissed through
     ESC/overlay (visible watcher lines 1063-1104). Cancel and successful
     save reset it (handleCancel lines 1715-1719, handleConfirm line 1571). */
  const preservedDraftRef = useRef<ModelDraft | null>(null);
  const draftRef = useRef<ModelDraft | null>(null);
  draftRef.current = draft;

  function closeEditor() {
    stopDownloadPolling();
    setDownloadTask(null);
    setDownloadProgress(null);
    setDraft(null);
    setDraftError(null);
    setRemoteMessage(null);
    setDimensionMessage(null);
    setThinkingManual(false);
    setNameError(null);
    setBaseUrlError(null);
    setOllamaOpen(false);
    preservedDraftRef.current = null;
  }

  /* ESC closes the editor while keeping the add draft for the next add open.
     ESC inside the Ollama combobox only closes the dropdown, so the handler
     ignores key events coming from the combobox wrapper. */
  useEffect(() => {
    if (!draft) return;
    function onEditorKeydown(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      const target = event.target;
      if (target instanceof HTMLElement && target.closest(".wk-ollama-combobox-wrap")) return;
      const current = draftRef.current;
      closeEditor();
      if (current && !current.id) preservedDraftRef.current = current;
    }
    document.addEventListener("keydown", onEditorKeydown);
    return () => document.removeEventListener("keydown", onEditorKeydown);
  }, [draft !== null]);

  /* Type switch — ModelEditorDialog.vue selectModelType: rerank is forced to
     remote, embedding-only fields are cleared for other types, the connection
     result resets, unsupported providers fall back to generic. */
  async function selectModelType(type: ModelType) {
    if (!draft || draft.id || draft.type === type) return;
    const options = await fetchProviderOptions(type);
    setProviderOptions(options);
    setDraft((current) => {
      if (!current || current.id || current.type === type) return current;
      const next: ModelDraft = { ...current, type };
      if (type === "rerank") next.source = "remote";
      if (type !== "embedding") {
        next.dimension = "";
        next.supportsDimensionOverride = false;
      }
      if (type !== "chat") next.supportsVision = false;
      if (!options.some((option) => option.value === next.provider)) {
        next.provider = "generic";
        next.baseUrl = "";
      }
      return next;
    });
    setRemoteMessage(null);
    setDimensionMessage(null);
    setThinkingManual(false);
    if (type === "chat") {
      setDraft((current) => (
        current && !current.id && current.source === "remote"
          ? { ...current, thinkingControl: defaultThinkingControl(current.provider, current.name) }
          : current
      ));
    }
  }

  /* Source switch — resets every check result (ModelEditorDialog.vue source
     watcher) and keeps the thinking default in sync for chat models. */
  function selectSource(source: "remote" | "local") {
    if (!draft || draft.source === source) return;
    if (source === "local" && (draft.type === "rerank" || ollamaStatus === false)) return;
    setRemoteMessage(null);
    setDimensionMessage(null);
    stopDownloadPolling();
    setDownloadTask(null);
    setDownloadProgress(null);
    if (!draft.id && source === "remote" && draft.type === "chat") {
      setThinkingManual(false);
    }
    setDraft((current) => {
      if (!current) return current;
      const next = { ...current, source };
      if (!current.id && source === "remote" && current.type === "chat") {
        next.thinkingControl = defaultThinkingControl(current.provider, current.name);
      }
      return next;
    });
  }

  /* Provider switch — autofills the documented default URL, prefills the
     recommended signed-rerank model names, resets the connection result and
     re-syncs the thinking default (ModelEditorDialog.vue handleProviderChange). */
  function onProviderChange(value: string) {
    if (!draft || draft.provider === value) return;
    const option = providerOptions.find((item) => item.value === value);
    setRemoteMessage(null);
    if (draft.type === "chat" && draft.source === "remote") {
      setThinkingManual(false);
    }
    setDraft((current) => {
      if (!current) return current;
      const next = { ...current, provider: value };
      if (option) {
        const defaultUrl = providerDefaultUrl(option.defaultUrls, current.type);
        if (defaultUrl) next.baseUrl = defaultUrl;
        if (!current.name.trim()) {
          if (value === "lkeap" && current.type === "rerank") next.name = "lke-reranker-base";
          if (value === "volcengine" && current.type === "rerank") next.name = "doubao-seed-rerank";
        }
      }
      if (current.type === "chat" && current.source === "remote") {
        next.thinkingControl = defaultThinkingControl(value, current.name);
      }
      return next;
    });
  }

  /* Model name change — clears the dimension hint and keeps the thinking
     default following the model name until the user picks manually
     (ModelEditorDialog.vue modelName watchers). */
  function onNameChange(value: string) {
    if (!draft) return;
    setDimensionMessage(null);
    const prevDefault = defaultThinkingControl(draft.provider, draft.name);
    setDraft((current) => {
      if (!current) return current;
      const next = { ...current, name: value };
      if (!current.id && current.type === "chat" && current.source === "remote") {
        if (!thinkingManual || current.thinkingControl === prevDefault) {
          next.thinkingControl = defaultThinkingControl(current.provider, value);
        }
      }
      return next;
    });
  }

  function addCustomHeader() {
    updateDraft("customHeaders", [...draft?.customHeaders ?? [], { key: "", value: "" }]);
  }
  function updateCustomHeader(index: number, field: keyof CustomHeaderItem, value: string) {
    setDraft((current) => {
      if (!current) return current;
      const customHeaders = current.customHeaders.map((item, itemIndex) =>
        itemIndex === index ? { ...item, [field]: value } : item,
      );
      return { ...current, customHeaders };
    });
  }
  function removeCustomHeader(index: number) {
    setDraft((current) => {
      if (!current) return current;
      return { ...current, customHeaders: current.customHeaders.filter((_, itemIndex) => itemIndex !== index) };
    });
  }

  async function refreshOllamaModels() {
    setOllamaBusy(true);
    try {
      setOllamaModels(await client.settings.ollama.models());
      setNotice(t("model.editor.listRefreshed"));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("model.editor.loadModelListFailed"));
    } finally {
      setOllamaBusy(false);
    }
  }

  async function downloadOllamaModel() {
    const name = draft?.name.trim();
    if (!draft || draft.source !== "local" || !name || ollamaBusy || checking) return;
    setOllamaBusy(true);
    setError(null);
    try {
      const result = await client.settings.ollama.download(name);
      const task = payloadString(result, ["task_id", "taskId", "id"]);
      setDownloadTask(task);
      setDownloadProgress(0);
      setNotice(t("model.editor.downloadStarted", { name }));
      if (task) startDownloadPolling(task, name);
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t("model.editor.downloadStartFailed"));
    } finally {
      setOllamaBusy(false);
    }
  }

  /* 1s progress polling until the backend reports completed/failed —
     mirrors ModelEditorDialog.vue startDownload, including auto-selecting the
     finished model and refreshing the local inventory. */
  function startDownloadPolling(taskId: string, modelName: string) {
    stopDownloadPolling();
    downloadTimerRef.current = setInterval(() => {
      void (async () => {
        try {
          const progress = await client.settings.ollama.progress(taskId);
          const percent = payloadNumber(progress, "progress");
          if (typeof percent === "number") setDownloadProgress(percent);
          const status = payloadString(progress, ["status"]);
          if (status === "completed") {
            stopDownloadPolling();
            setDownloadProgress(null);
            setDownloadTask(null);
            setNotice(t("model.editor.downloadCompleted", { name: modelName }));
            try {
              setOllamaModels(await client.settings.ollama.models());
            } catch { /* inventory refresh is best-effort */ }
            updateDraft("name", modelName);
          } else if (status === "failed") {
            stopDownloadPolling();
            setDownloadProgress(null);
            setDownloadTask(null);
            setError(payloadString(progress, ["message"]) ?? t("model.editor.downloadFailed", { name: modelName }));
          }
        } catch {
          // keep polling; transient progress errors are non-fatal
        }
      })();
    }, 1000);
  }

  /* Local embedding dimension probe — runs the same embedding connection
     test the backend uses (ModelEditorDialog.vue checkOllamaDimension). */
  async function checkOllamaDimension() {
    if (!draft || draft.source !== "local" || draft.type !== "embedding" || !draft.name.trim() || checking) return;
    setChecking(true);
    setDimensionMessage(null);
    try {
      const result = await client.configuration.models.connection.embedding({
        source: "local",
        modelName: draft.name,
        dimension: typeof draft.dimension === "number" ? draft.dimension : undefined,
        supportsDimensionOverride: draft.supportsDimensionOverride,
      });
      if (result.available && typeof result.dimension === "number" && result.dimension > 0) {
        updateDraft("dimension", result.dimension);
        setDimensionMessage({ ok: true, text: t("model.editor.dimensionDetected", { value: result.dimension }) });
      } else {
        setDimensionMessage({ ok: false, text: t("model.editor.dimensionFailed") });
      }
    } catch {
      setDimensionMessage({ ok: false, text: t("model.editor.dimensionFailed") });
    } finally {
      setChecking(false);
    }
  }

  /* Remote connection test — per-type endpoints and payloads exactly like
     ModelEditorDialog.vue checkRemoteAPI, including the edit-mode modelId
     passthrough and customHeaders only when at least one pair is set. */
  async function testConnection() {
    if (!draft || checking) return;
    if (!draft.name || (!draft.baseUrl && draft.provider !== "weknoracloud")) {
      setDraftError(t("model.editor.fillModelAndUrl"));
      return;
    }
    setChecking(true);
    setRemoteMessage(null);
    setDraftError(null);
    const headers = customHeadersMap(draft.customHeaders);
    const headerPayload = Object.keys(headers).length > 0 ? { customHeaders: headers } : {};
    const idPayload = draft.id ? { modelId: draft.id } : {};
    const apiKey = draft.apiKey || "";
    try {
      let result: { available: boolean; message: string; dimension?: number };
      if (draft.type === "embedding") {
        result = await client.configuration.models.connection.embedding({
          source: "remote",
          modelName: draft.name,
          baseUrl: draft.baseUrl || "",
          apiKey,
          dimension: typeof draft.dimension === "number" ? draft.dimension : undefined,
          supportsDimensionOverride: draft.supportsDimensionOverride,
          provider: draft.provider,
          ...idPayload,
          ...headerPayload,
        });
        if (result.available && typeof result.dimension === "number" && result.dimension > 0) {
          updateDraft("dimension", result.dimension);
          setDimensionMessage({ ok: true, text: t("model.editor.remoteDimensionDetected", { value: result.dimension }) });
        }
      } else if (draft.type === "rerank") {
        const signed = signedRerankProvider("rerank", draft.provider);
        const signedExtra = signed
          ? {
              ...(signed === "lkeap" ? { extraConfig: { region: (draft.lkeapRegion || "ap-guangzhou").trim() } } : {}),
              ...(draft.appSecret.trim() ? { appSecret: draft.appSecret.trim() } : {}),
            }
          : {};
        result = await client.configuration.models.connection.rerank({
          modelName: draft.name,
          baseUrl: draft.baseUrl || "",
          apiKey,
          provider: draft.provider,
          ...idPayload,
          ...headerPayload,
          ...signedExtra,
        });
      } else if (draft.type === "asr") {
        result = await client.configuration.models.connection.asr({
          modelName: draft.name,
          baseUrl: draft.baseUrl || "",
          apiKey,
          provider: draft.provider,
          ...idPayload,
          ...headerPayload,
        });
      } else {
        result = await client.configuration.models.connection.remote({
          modelName: draft.name,
          baseUrl: draft.baseUrl || "",
          apiKey,
          provider: draft.provider,
          ...idPayload,
          ...headerPayload,
        });
      }
      if (result.available) {
        setRemoteMessage({ ok: true, text: t("model.editor.connectionSuccess") });
      } else {
        setRemoteMessage({ ok: false, text: result.message || t("model.editor.connectionFailed") });
      }
    } catch (cause) {
      setRemoteMessage({
        ok: false,
        text: cause instanceof Error && cause.message ? cause.message : t("model.editor.connectionConfigError"),
      });
    } finally {
      setChecking(false);
    }
  }

  async function reload() {
    try {
      setModels(await client.configuration.models.list());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("modelSettings.toasts.saveFailed"));
    }
  }

  /* Save — duplicate-submit protected, validation errors rendered inline in
     the editor with the exact Vue toast copy (ModelSettings.vue
     handleModelSave + ModelEditorDialog.vue handleConfirm). */
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || busy) return;
    const invalid = validateModelDraft(draft);
    if (invalid.length > 0) {
      setDraftError(invalid.map((code) => t(modelValidationErrorKey(code))).join(" "));
      return;
    }
    setBusy(true);
    setDraftError(null);
    setNotice(null);
    try {
      const payload = modelPayload(draft);
      if (draft.id) {
        await client.configuration.models.update(draft.id, payload);
        setNotice(t("modelSettings.toasts.updated"));
      } else {
        await client.configuration.models.create(payload);
        setNotice(t("modelSettings.toasts.added"));
      }
      closeEditor();
      await reload();
    } catch (cause) {
      setDraftError(cause instanceof Error && cause.message ? cause.message : t("modelSettings.toasts.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function remove(model: ModelConfiguration) {
    if (!canCreate || isBuiltin(model) || busy) return;
    if (!window.confirm(t("modelSettings.confirmDelete", { name: label(model) }))) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    setUsageConflict(null);
    try {
      await client.configuration.models.remove(model.id);
      setNotice(t("modelSettings.toasts.deleted"));
      await reload();
    } catch (cause) {
      const details = modelInUseDetails(cause);
      if (details) {
        setUsageConflict({ modelName: label(model), details });
        return;
      }
      setError(cause instanceof Error && cause.message ? cause.message : t("modelSettings.toasts.deleteFailed"));
    } finally {
      setBusy(false);
    }
  }

  /* Duplicate — ModelSettings.vue copyModel: copy-suffix name, credentials
     never copied (listModels strips secrets), deep-copied parameters. */
  function generateCopyName(originalName: string): string {
    const suffix = t("modelSettings.copySuffix");
    const existingNames = new Set(models.map((item) => item.name));
    let candidate = `${originalName}${suffix}`;
    let counter = 2;
    while (existingNames.has(candidate)) {
      candidate = `${originalName}${suffix} ${counter}`;
      counter += 1;
    }
    return candidate;
  }
  async function copyModel(model: ModelConfiguration) {
    if (!canCreate || isBuiltin(model) || busy) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await client.configuration.models.create({
        name: generateCopyName(model.name),
        display_name: label(model) === model.name ? "" : label(model),
        type: model.type,
        source: typeof model.source === "string" ? model.source : "remote",
        description: typeof (model as Record<string, unknown>).description === "string"
          ? (model as Record<string, unknown>).description as string
          : "",
        parameters: JSON.parse(JSON.stringify(params(model))) as Record<string, unknown>,
      });
      setNotice(t("modelSettings.toasts.copied"));
      await reload();
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t("modelSettings.toasts.copyFailed"));
    } finally {
      setBusy(false);
    }
  }

  /* Credential subresource writes — edit mode owns credentials through
     /models/:id/credentials (ModelEditorDialog.vue CredentialResource). */
  async function saveCredentialField(field: "apiKey" | "appSecret") {
    if (!draft?.id || credentialBusy) return;
    const value = field === "apiKey" ? credentialValues.apiKey : credentialValues.appSecret;
    if (!value.trim()) return;
    setCredentialBusy(true);
    setDraftError(null);
    try {
      await client.configuration.models.credentials.put(draft.id, field === "apiKey" ? { apiKey: value.trim() } : { appSecret: value.trim() });
      setCredentialValues((current) => ({ ...current, [field]: "" }));
      updateDraft("credentials", {
        ...(draft.credentials ?? {}),
        [field === "apiKey" ? "api_key" : "app_secret"]: { configured: true },
      });
      setNotice(t("settings.weknoraCloud.credentialConfigured"));
    } catch (cause) {
      setDraftError(cause instanceof Error && cause.message ? cause.message : t("modelSettings.toasts.saveFailed"));
    } finally {
      setCredentialBusy(false);
    }
  }
  async function removeCredentialField(field: "api_key" | "app_secret") {
    if (!draft?.id || credentialBusy) return;
    if (!window.confirm(t("common.delete"))) return;
    setCredentialBusy(true);
    setDraftError(null);
    try {
      await client.configuration.models.credentials.remove(draft.id, field);
      updateDraft("credentials", { ...(draft.credentials ?? {}), [field]: { configured: false } });
      setNotice(t("common.success"));
    } catch (cause) {
      setDraftError(cause instanceof Error && cause.message ? cause.message : t("modelSettings.toasts.saveFailed"));
    } finally {
      setCredentialBusy(false);
    }
  }

  /* Per-field blur validation: each field renders its own message with the
     exact ModelEditorDialog.vue rules copy; the message clears as soon as
     the field is fixed. */
  function blurName() {
    if (!draft) return;
    const key = modelFieldErrorKey("name", draft);
    setNameError(key ? t(key) : null);
  }
  function changeName(value: string) {
    onNameChange(value);
    if (nameError && draft) {
      const key = modelFieldErrorKey("name", { ...draft, name: value });
      setNameError(key ? t(key) : null);
    }
  }
  function blurBaseUrl() {
    if (!draft) return;
    const key = modelFieldErrorKey("baseUrl", draft);
    setBaseUrlError(key ? t(key) : null);
  }
  function changeBaseUrl(value: string) {
    updateDraft("baseUrl", value);
    if (baseUrlError && draft) {
      const key = modelFieldErrorKey("baseUrl", { ...draft, baseUrl: value });
      setBaseUrlError(key ? t(key) : null);
    }
  }

  /* Ollama combobox: inventory filtered by the typed keyword, keyboard
     navigation and the Vue download option for unknown keywords
     (ModelEditorDialog.vue lines 109-133). */
  const ollamaKeyword = draft ? draft.name.trim() : "";
  const ollamaSuggestions = useMemo(
    () => ollamaModels.filter((item) => item.name.includes(ollamaKeyword)),
    [ollamaModels, ollamaKeyword],
  );
  const ollamaDownloadOffered = ollamaKeyword.length > 0 && !ollamaModels.some((item) => item.name === ollamaKeyword);
  function selectOllamaModel(name: string) {
    if (!draft) return;
    onNameChange(name);
    setNameError(null);
    setOllamaOpen(false);
  }
  function onComboboxKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    const maxIndex = ollamaSuggestions.length + (ollamaDownloadOffered ? 1 : 0);
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setOllamaOpen(true);
      setOllamaHighlight((current) => Math.min(maxIndex - 1, current + 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setOllamaHighlight((current) => Math.max(0, current - 1));
    } else if (event.key === "Enter") {
      if (!ollamaOpen) return;
      event.preventDefault();
      if (ollamaDownloadOffered && ollamaHighlight === ollamaSuggestions.length) {
        setOllamaOpen(false);
        void downloadOllamaModel();
        return;
      }
      const picked = ollamaSuggestions[ollamaHighlight];
      if (picked) selectOllamaModel(picked.name);
    } else if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      setOllamaOpen(false);
    }
  }

  // Type badge icons per ModelSettings.vue typeIcon (lines 395-404) —
  // stroke SVGs mirroring the t-icon names (chat / chart-bubble /
  // filter-sort / image / sound).
  function badgeIcon(type: ModelType): ReactNode {
    const stroke = { fill: "none", stroke: "currentColor", strokeWidth: 1.7, strokeLinecap: "round" as const, strokeLinejoin: "round" as const };
    const map: Record<ModelType, ReactNode> = {
      chat: <svg width="18" height="18" viewBox="0 0 24 24" {...stroke}><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" /></svg>,
      embedding: <svg width="18" height="18" viewBox="0 0 24 24" {...stroke}><path d="M5 20V10M12 20V4M19 20v-7" /></svg>,
      rerank: <svg width="18" height="18" viewBox="0 0 24 24" {...stroke}><path d="M3 6h13M3 12h9M3 18h5" /><path d="M16 14l4 4 4-4" transform="scale(0.75) translate(4 4)" /></svg>,
      vllm: <svg width="18" height="18" viewBox="0 0 24 24" {...stroke}><rect x="3" y="3" width="18" height="18" rx="2" /><circle cx="8.5" cy="8.5" r="1.5" /><path d="M21 15l-5-5L5 21" /></svg>,
      asr: <svg width="18" height="18" viewBox="0 0 24 24" {...stroke}><path d="M11 5L6 9H2v6h4l5 4V5z" /><path d="M15.5 8.5a5 5 0 0 1 0 7M19 5a9 9 0 0 1 0 14" /></svg>,
    };
    return map[type];
  }
  function typeLabelOf(type: ModelType): string {
    return t(`modelSettings.typeShort.${type}`);
  }
  function vendorLabel(model: ModelConfiguration): string {
    if (model.source === "local") return "Ollama";
    const provider = typeof params(model).provider === "string" ? params(model).provider as string : "";
    if (provider === "generic") return t("modelSettings.source.custom");
    if (provider) {
      const viaT = t(`model.editor.providers.${provider}.label`);
      if (viaT !== `model.editor.providers.${provider}.label`) return viaT;
      return provider;
    }
    return modelType(model) === "vllm" || modelType(model) === "asr"
      ? t("modelSettings.source.openaiCompatible")
      : t("modelSettings.source.remote");
  }

  const editorProviderOptions = useMemo(() => {
    if (!draft) return providerOptions;
    if (providerOptions.some((option) => option.value === draft.provider)) return providerOptions;
    return [
      ...providerOptions,
      {
        value: draft.provider,
        label: providerText(t, draft.provider, "label", draft.provider),
        description: providerText(t, draft.provider, "description", ""),
        defaultUrls: {},
        modelTypes: [draft.type],
      },
    ];
  }, [providerOptions, draft, t]);

  const emptyHint = filter === "all"
    ? t("modelSettings.chat.empty")
    : t(`modelSettings.${filter}.empty`);
  const signed = draft ? signedRerankProvider(draft.type, draft.provider) : null;
  const apiKeyLabel = signed
    ? t(signed === "volcengine" ? "model.editor.volcengine.accessKeyLabel" : "model.editor.lkeap.secretIdLabel")
    : t("model.editor.apiKeyOptional");
  const apiKeyPlaceholder = signed
    ? t(signed === "volcengine" ? "model.editor.volcengine.accessKeyPlaceholder" : "model.editor.lkeap.secretIdPlaceholder")
    : t("model.editor.apiKeyPlaceholder");
  const secretKeyLabel = signed === "volcengine"
    ? t("model.editor.volcengine.secretKeyLabel")
    : t("model.editor.lkeap.secretKeyLabel");
  const secretKeyPlaceholder = signed === "volcengine"
    ? t("model.editor.volcengine.secretKeyPlaceholder")
    : t("model.editor.lkeap.secretKeyPlaceholder");
  const credentialHint = signed
    ? t(signed === "volcengine" ? "model.editor.volcengine.rerankCredentialHint" : "model.editor.lkeap.rerankCredentialHint")
    : null;
  // R484 G4 D4 — Vue always renders the full thinkingControlDesc form-desc
  // under the select (ModelEditorDialog.vue line 385); the per-option hint
  // only appears inside the dropdown options below.
  const thinkingOptions = useMemo(() => THINKING_CONTROL_OPTIONS.map((option) => ({
    value: option.value,
    label: t(`model.editor.thinkingControl.${option.key}.label`),
    description: t(`model.editor.thinkingControl.${option.key}.hint`),
  })), [t]);

  return (
    <section className="grid gap-4" data-testid="model-settings">
      {/* Vue ModelSettings.vue section-header: mb28, h2 20/600 mb8 normal,
          desc 14px lh1.6 secondary; the builtin-models-hint box lives INSIDE
          the header (mt12, pad 10/12, radius 6) so the mb28 → tabs chain
          lands on Vue's t-tabs y=245.5. */}
      <div className="section-header">
        <div className="flex items-center justify-between gap-5 max-[720px]:flex-col max-[720px]:items-start">
        <div>
          <h2>{t("modelSettings.title")}</h2>
          <p className="section-description m-0">{t("modelSettings.description")}</p>
        </div>
        {canCreate ? (
          <button type="button" className="inline-flex cursor-pointer items-center gap-2 border-0 bg-transparent px-0 py-[6px] font-[inherit] text-sm font-semibold text-[#07c05f] hover:text-[#06b04d] focus-visible:text-[#06b04d]" onClick={() => setDebugOpen(true)}>
            <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" strokeWidth="1.6" /><path d="M10 8.8v6.4l5.4-3.2z" fill="currentColor" /></svg>
            {t("modelSettings.actions.debugModel")}
          </button>
        ) : null}
        </div>
        <div className="mt-3 rounded-md border border-[#e7e7e7] bg-[#f3f3f3] px-3 py-[10px] leading-[18px]" role="note">
          <p className="m-0 mb-1 text-xs font-medium leading-[17px] tracking-[0.02em] text-[rgba(0,0,0,0.4)]"><strong>{t("modelSettings.builtinModels.title")}</strong></p>
          <p className="m-0 mb-[6px] text-[13px] leading-[1.55] text-[rgba(0,0,0,0.6)]">
            {t(role === "system-admin" ? "modelSettings.builtinModels.descriptionAdmin" : "modelSettings.builtinModels.description")}
          </p>
          <a className="inline-flex items-center gap-1 align-top text-[13px] leading-[18px] text-[var(--wk-brand,#07c05f)] no-underline hover:underline" href={BUILTIN_MODELS_DOC} target="_blank" rel="noopener noreferrer">
            {t("modelSettings.builtinModels.viewGuide")}
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M10 14a5 5 0 007.5.5l3-3a5 5 0 00-7-7l-1.7 1.7" /><path d="M14 10a5 5 0 00-7.5-.5l-3 3a5 5 0 007 7l1.7-1.7" /></svg>
          </a>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {usageConflict ? <ModelUsageNotice modelName={usageConflict.modelName} details={usageConflict.details} onClose={() => setUsageConflict(null)} /> : null}
      <nav className="wk-model-tabs flex flex-wrap gap-0 border-b border-b-[#e7e7e7]" aria-label={t("model.editor.typeLabel")}>
        <button
          type="button"
          className={filter === "all" ? "cursor-pointer border-0 border-b-[3px] border-b-[#07c05f]! bg-transparent px-3 py-3 text-[13px] leading-[20px] text-[#506078] text-[#07c05f]! [font-weight:650] is-active" : "cursor-pointer border-0 border-b-[3px] border-b-transparent bg-transparent px-3 py-3 text-[13px] leading-[20px] text-[#506078]"}
          onClick={() => setFilter("all")}
        >
          {t("common.all")}({models.length})
        </button>
        {TYPES.map((type) => (
          <button
            type="button"
            key={type}
            className={filter === type ? "cursor-pointer border-0 border-b-[3px] border-b-[#07c05f]! bg-transparent px-3 py-3 text-[13px] leading-[20px] text-[#506078] text-[#07c05f]! [font-weight:650] is-active" : "cursor-pointer border-0 border-b-[3px] border-b-transparent bg-transparent px-3 py-3 text-[13px] leading-[20px] text-[#506078]"}
            onClick={() => setFilter(type)}
          >
            {typeLabelOf(type)}({models.filter((item) => modelType(item) === type).length})
          </button>
        ))}
      </nav>
      {!canCreate && visible.length === 0 ? (
        <Status>{emptyHint}</Status>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))]! gap-3!">
          {visible.map((model) => {
            const type = modelType(model);
            const modelParams = params(model);
            const builtin = isBuiltin(model);
            const canEdit = builtin ? role === "system-admin" : canCreate;
            const embeddingParams = modelParams.embedding_parameters;
            const dimension = typeof modelParams.dimension === "number"
              ? modelParams.dimension
              : embeddingParams && typeof embeddingParams === "object" && !Array.isArray(embeddingParams) && typeof (embeddingParams as Record<string, unknown>).dimension === "number"
                ? (embeddingParams as Record<string, unknown>).dimension
                : undefined;
            const contextWindow = typeof modelParams.context_window === "number" ? modelParams.context_window : undefined;
            const supportsVision = modelParams.supports_vision === true;
            const menuOpen = menuFor === model.id;
            return (
              <div
                key={model.id}
                className={"wk-vmodel-card group/card relative box-border flex min-w-0 items-start gap-3 rounded-[10px] border border-[#e7e7e7] px-4 py-[14px] transition-[border-color,box-shadow] duration-[180ms] ease-[ease]"
                  + (builtin ? " bg-[#f3f3f3] hover:border-[#e7e7e7] hover:shadow-none" : " bg-white")
                  + (canEdit ? " cursor-pointer" : "")
                  + (canEdit && !builtin ? " hover:border-[rgba(7,192,95,0.65)] hover:shadow-[0_4px_14px_rgba(15,23,42,0.08)] hover:outline-none focus-visible:border-[rgba(7,192,95,0.65)] focus-visible:shadow-[0_4px_14px_rgba(15,23,42,0.08)] focus-visible:outline-none" : "")}
                onClick={canEdit ? () => openEdit(model) : undefined}
              >
                <div className={"mt-px flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] text-base " + CARD_BADGE_TONE[type]} aria-label={typeLabelOf(type)}>{badgeIcon(type)}</div>
                <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
                  <div className="flex min-w-0 items-center gap-1.5">
                    <h3 className="m-0 min-w-0 flex-1 truncate text-sm font-semibold leading-[1.4]">{label(model)}</h3>
                    {builtin ? (
                      <span className="shrink-0 text-[13px] text-[#8a97ab] opacity-60 group-hover/card:opacity-100" title={t("modelSettings.builtinTag")} aria-label={t("modelSettings.builtinTag")}>
                        {role === "system-admin"
                          ? <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M17 3a2.85 2.85 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /></svg>
                          : <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>}
                      </span>
                    ) : null}
                    {canEdit ? (
                      <div className="group/actions relative flex shrink-0 items-center gap-0.5" onClick={(event) => event.stopPropagation()}>
                        <button
                          type="button"
                          className="cursor-pointer border-0 bg-transparent px-1.5 py-[2px] text-sm opacity-0 transition-opacity duration-150 ease-[ease] group-focus-within/actions:opacity-100 group-focus-within/card:opacity-100 group-hover/card:opacity-100 text-[#7a879c]"
                          aria-haspopup="menu"
                          aria-expanded={menuOpen}
                          onClick={() => setMenuFor(menuOpen ? null : model.id)}
                        >
                          {/* Vue card menu is an icon glyph (no text node). */}
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><circle cx="5" cy="12" r="1.7" /><circle cx="12" cy="12" r="1.7" /><circle cx="19" cy="12" r="1.7" /></svg>
                        </button>
                        {menuOpen ? (
                          <div className="absolute right-0 top-[26px] z-[5] flex min-w-[96px] flex-col rounded-lg border border-[rgba(120,135,155,0.3)] bg-white shadow-[0_8px_24px_rgba(23,32,51,0.16)]" role="menu">
                            <button type="button" role="menuitem" className="cursor-pointer border-0 bg-transparent px-3 py-2 text-left font-[inherit] text-[13px] hover:bg-[rgba(127,142,166,0.1)] disabled:cursor-default disabled:text-[#9aa6b8]" onClick={() => { setMenuFor(null); openEdit(model); }}>
                              {t("common.edit")}
                            </button>
                            {!builtin ? (
                              <button type="button" role="menuitem" disabled={busy} className="cursor-pointer border-0 bg-transparent px-3 py-2 text-left font-[inherit] text-[13px] hover:bg-[rgba(127,142,166,0.1)] disabled:cursor-default disabled:text-[#9aa6b8]" onClick={() => { setMenuFor(null); void copyModel(model); }}>
                                {t("common.copy")}
                              </button>
                            ) : null}
                          </div>
                        ) : null}
                        {!builtin ? (
                          <button
                            type="button"
                            className="cursor-pointer border-0 bg-transparent px-1.5 py-[2px] text-sm opacity-0 transition-opacity duration-150 ease-[ease] group-focus-within/actions:opacity-100 group-focus-within/card:opacity-100 group-hover/card:opacity-100 text-[#c23434]"
                            title={t("common.delete")}
                            aria-label={t("common.delete")}
                            disabled={busy}
                            onClick={() => void remove(model)}
                          >
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M10 11v6M14 11v6" /></svg>
                          </button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                  <p className="m-0 mt-[2px] truncate text-xs leading-[1.5] text-[#5c6b83]">
                    <span>{vendorLabel(model)}</span>
                    {type === "embedding" && typeof dimension === "number" ? (
                      <>
                        <span className="mx-[4px] text-[#97a3b6]">·</span>
                        <span>{t("model.editor.dimensionLabel")} {dimension}</span>
                      </>
                    ) : null}
                    {/* Vue ModelSettings.vue L110-117 renders the ctx chip for
                        every chat/vllm card; formatContextWindow falls back to
                        the 200K default (dimmed) when no value is stored. */}
                    {(type === "chat" || type === "vllm") ? (
                      <>
                        <span className="mx-[4px] text-[#97a3b6]">·</span>
                        <span
                          className="tabular-nums"
                          title={isDefaultContextWindow(contextWindow)
                            ? t("model.editor.contextWindowDefaultHint", { value: formatContextWindow(contextWindow) })
                            : t("model.editor.contextWindowTokens", { count: effectiveContextWindow(contextWindow) })}
                        >
                          {formatContextWindow(contextWindow)}
                        </span>
                      </>
                    ) : null}
                    {type === "chat" && supportsVision ? (
                      <>
                        <span className="mx-[4px] text-[#97a3b6]">·</span>
                        <span className="model-card__vision" title={t("model.editor.supportsVisionLabel")} aria-label={t("model.editor.supportsVisionLabel")}>👁</span>
                      </>
                    ) : null}
                  </p>
                </div>
              </div>
            );
          })}
          {canCreate ? (
            <button type="button" className="flex min-h-[68px] cursor-pointer flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed border-[#e7e7e7] bg-transparent px-4 py-[14px] font-[inherit] text-[rgba(0,0,0,0.4)] hover:border-[#07c05f] hover:bg-[rgba(7,192,95,0.06)] hover:text-[#07c05f] focus-visible:border-[#07c05f] focus-visible:bg-[rgba(7,192,95,0.06)] focus-visible:text-[#07c05f]" onClick={openAdd}>
              <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-[rgba(7,192,95,0.1)] text-[#07c05f]" aria-hidden="true"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><path d="M12 5v14M5 12h14" /></svg></span>
              <span className="text-[13px] font-medium leading-[18px]">{t("modelSettings.actions.addModel")}</span>
            </button>
          ) : null}
        </div>
      )}
      {draft ? (
        <div
          className="wk-model-editor-overlay fixed inset-0 z-[1300] flex items-stretch justify-end bg-[rgba(23,32,51,.34)]"
          onMouseDown={(event) => { if (event.target === event.currentTarget) closeEditor(); }}
        >
        <div
          className="wk-model-editor wk-model-editor-drawer box-border h-full w-[560px] max-w-full overflow-y-auto border-l border-l-[#dce3ed] bg-white pt-[1.25rem] pr-6 pb-8 pl-6 shadow-[-10px_0_30px_rgba(23,32,51,.12)] max-[720px]:w-full max-[720px]:p-4"
          role="dialog"
          aria-modal="true"
          aria-label={draft.id ? t("model.editor.editTitle") : t("model.editor.addTitle")}
        >
          <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col sticky -top-[1.25rem] z-[1] bg-white pt-[1.25rem] max-[720px]:-top-[1rem] max-[720px]:pt-4">
            <div>
              <h3>{draft.id ? t("model.editor.editTitle") : t("model.editor.addTitle")}</h3>
              <p className="wk-muted text-muted m-0">
                {t(`model.editor.description.${draft.type}`) || t("model.editor.description.default")}
              </p>
            </div>
            <Button type="button" disabled={busy} onClick={closeEditor}>
              {t("common.close")}
            </Button>
          </div>
          <form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]" onSubmit={(event) => void save(event)}>
            {!draft.id ? (
              <div className="form-item">
                <h4>{t("model.editor.sectionType")}</h4>
                <div className="wk-model-type-options" role="radiogroup" aria-label={t("model.editor.typeLabel")}>
                  {TYPES.map((type) => (
                    <button
                      key={type}
                      type="button"
                      role="radio"
                      aria-checked={draft.type === type}
                      className={draft.type === type ? "is-active" : ""}
                      onClick={() => void selectModelType(type)}
                    >
                      {typeLabelOf(type)}
                    </button>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="form-item">
              <h4>{t("model.editor.sectionSource")}</h4>
              <div className="wk-source-options" role="radiogroup" aria-label={t("model.editor.sourceLabel")}>
                <button
                  type="button"
                  role="radio"
                  aria-checked={draft.source === "remote"}
                  className={draft.source === "remote" ? "is-active" : ""}
                  onClick={() => selectSource("remote")}
                >
                  {t("model.editor.sourceRemote")}
                </button>
                <button
                  type="button"
                  role="radio"
                  aria-checked={draft.source === "local"}
                  className={draft.source === "local" ? "is-active" : ""}
                  disabled={ollamaStatus === false || draft.type === "rerank"}
                  onClick={() => selectSource("local")}
                >
                  {t("model.editor.sourceLocal")}
                </button>
              </div>
              {draft.type === "rerank" ? (
                <p className="wk-muted text-muted">{t("model.editor.ollamaNotSupportRerank")}</p>
              ) : draft.source === "local" && ollamaStatus === false ? (
                <p className="wk-muted text-muted">
                  {t("model.editor.ollamaUnavailable")}{" "}
                  <Button type="button" onClick={() => navigate("/platform/settings?section=ollama")}>
                    {t("model.editor.goToOllamaSettings")}
                  </Button>
                </p>
              ) : null}
              {draft.source === "local" ? (
                <div className="form-item">
                  <label>
                    {t("model.modelName")}
                  </label>
                  <div className="wk-ollama-combobox-wrap relative grid w-full gap-1">
                      <Input
                        role="combobox"
                        aria-expanded={ollamaOpen}
                        aria-controls="wk-ollama-listbox"
                        autoComplete="off"
                        placeholder={t("model.searchPlaceholder")}
                        value={draft.name}
                        onChange={(event) => { changeName(event.target.value); setOllamaOpen(true); setOllamaHighlight(0); }}
                        onFocus={() => setOllamaOpen(true)}
                        onBlur={() => { setOllamaOpen(false); blurName(); }}
                        onKeyDown={onComboboxKeyDown}
                      />
                      {ollamaOpen ? (
                        <div className="relative z-[6] grid max-h-[220px] overflow-auto rounded-lg border border-[rgba(120,135,155,0.35)] bg-white shadow-[0_8px_24px_rgba(23,32,51,0.14)]" id="wk-ollama-listbox" role="listbox">
                          {ollamaSuggestions.map((item, index) => (
                            <button
                              type="button"
                              key={item.name}
                              role="option"
                              aria-selected={index === ollamaHighlight}
                              className={"flex w-full cursor-pointer items-center gap-2 border-0 bg-transparent px-3 py-2 text-left font-[inherit] hover:bg-[rgba(7,192,95,0.1)]" + (index === ollamaHighlight ? " bg-[rgba(7,192,95,0.1)]" : "")}
                              onMouseDown={(event) => { event.preventDefault(); selectOllamaModel(item.name); }}
                            >
                              <span className="text-xs text-[#0a8f4c]" aria-hidden="true">✓</span>
                              <span className="flex-1 text-[13px]">{item.name}</span>
                              <span className="text-xs text-[#7a879c]">{formatModelSize((item as Record<string, unknown>).size)}</span>
                            </button>
                          ))}
                          {ollamaDownloadOffered ? (
                            <button
                              type="button"
                              role="option"
                              aria-selected={ollamaHighlight === ollamaSuggestions.length}
                              className={"flex w-full cursor-pointer items-center gap-2 border-0 bg-transparent px-3 py-2 text-left font-[inherit] hover:bg-[rgba(7,192,95,0.1)]" + (ollamaHighlight === ollamaSuggestions.length ? " bg-[rgba(7,192,95,0.1)]" : "")}
                              onMouseDown={(event) => { event.preventDefault(); setOllamaOpen(false); void downloadOllamaModel(); }}
                            >
                              <span aria-hidden="true">⬇</span>
                              <span className="flex-1 text-[13px] text-[#0a8f4c]">{t("model.editor.downloadLabel", { keyword: ollamaKeyword })}</span>
                            </button>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                    {nameError ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{nameError}</span> : null}
                  <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                    <Button type="button" disabled={ollamaBusy} onClick={() => void refreshOllamaModels()}>
                      {t("model.editor.refreshList")}
                    </Button>
                  </div>
                  {ollamaBusy ? <Status>{t("common.loading")}</Status> : null}
                  {downloadTask || downloadProgress !== null ? (
                    <Status>{`${draft.name} · ${downloadProgress !== null ? `${downloadProgress.toFixed(1)}%` : "0%"}`}</Status>
                  ) : null}
                </div>
              ) : null}
            </div>

            {draft.source === "remote" ? (
              <div className="form-item">
                <h4>{t("model.editor.sectionProvider")}</h4>
                <label>
                  {t("model.editor.providerLabel")}
                  <ModelOptionSelect
                    value={draft.provider}
                    disabled={loadingProviders}
                    options={editorProviderOptions}
                    onChange={onProviderChange}
                  />
                </label>
                {draft.provider === "weknoracloud" ? (
                  wkcState === "loading" ? (
                    <Status>{t("settings.weknoraCloud.checkingStatus")}</Status>
                  ) : wkcState === "configured" ? (
                    <Status tone="success">{t("settings.weknoraCloud.modelHintConfigured")}</Status>
                  ) : (
                    <Status tone="warning">
                      {t(wkcState === "expired" ? "settings.weknoraCloud.credentialExpired" : "settings.weknoraCloud.credentialUnconfigured")}{" "}
                      <Button type="button" onClick={() => navigate("/platform/settings?section=weknoracloud")}>
                        {t("settings.weknoraCloud.goToSettings")}
                      </Button>
                    </Status>
                  )
                ) : null}
                <label>
                  {t("model.modelName")}
                  <Input
                    required
                    maxLength={100}
                    placeholder={t(modelNamePlaceholderKey(draft.type, draft.source))}
                    disabled={draft.provider === "weknoracloud" && wkcState !== "configured"}
                    value={draft.name}
                    onChange={(event) => changeName(event.target.value)}
                    onBlur={blurName}
                  />
                  {nameError ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{nameError}</span> : null}
                </label>
                <label>
                  {t("model.editor.displayNameLabel")}
                      <Input
                    maxLength={100}
                    placeholder={t("model.editor.displayNamePlaceholder")}
                    value={draft.displayName}
                    onChange={(event) => updateDraft("displayName", event.target.value)}
                  />
                  <span className="wk-muted text-muted">{t("model.editor.displayNameDesc")}</span>
                </label>
                {draft.provider !== "weknoracloud" ? (
                  <>
                    <label>
                      {t("model.editor.baseUrlLabel")}
                      <Input
                        type="url"
                        required
                        placeholder={t(baseUrlPlaceholderKey(draft.type))}
                        value={draft.baseUrl}
                        onChange={(event) => changeBaseUrl(event.target.value)}
                        onBlur={blurBaseUrl}
                      />
                      {baseUrlError ? <span className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{baseUrlError}</span> : null}
                    </label>
                    {draft.id ? (
                      <div className="form-item">
                        <label>
                          {apiKeyLabel}
                          <Input
                            type="password"
                            autoComplete="new-password"
                            placeholder={apiKeyPlaceholder}
                            value={credentialValues.apiKey}
                            onChange={(event) => setCredentialValues((current) => ({ ...current, apiKey: event.target.value }))}
                          />
                        </label>
                        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                          {draft.credentials?.api_key?.configured ? <span title="configured" className="mr-auto text-[0.85rem] text-muted">✓</span> : null}
                          <Button
                            type="button"
                            disabled={credentialBusy || !credentialValues.apiKey.trim()}
                            onClick={() => void saveCredentialField("apiKey")}
                          >
                            {t("common.save")}
                          </Button>
                          {draft.credentials?.api_key?.configured ? (
                            <Button type="button" disabled={credentialBusy} onClick={() => void removeCredentialField("api_key")}>
                              {t("common.delete")}
                            </Button>
                          ) : null}
                        </div>
                        {signed ? (
                          <>
                            <label>
                              {secretKeyLabel}
                              <Input
                                type="password"
                                autoComplete="new-password"
                                placeholder={secretKeyPlaceholder}
                                value={credentialValues.appSecret}
                                onChange={(event) => setCredentialValues((current) => ({ ...current, appSecret: event.target.value }))}
                              />
                            </label>
                            <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                              {draft.credentials?.app_secret?.configured ? <span title="configured" className="mr-auto text-[0.85rem] text-muted">✓</span> : null}
                              <Button
                                type="button"
                                disabled={credentialBusy || !credentialValues.appSecret.trim()}
                                onClick={() => void saveCredentialField("appSecret")}
                              >
                                {t("common.save")}
                              </Button>
                              {draft.credentials?.app_secret?.configured ? (
                                <Button type="button" disabled={credentialBusy} onClick={() => void removeCredentialField("app_secret")}>
                                  {t("common.delete")}
                                </Button>
                              ) : null}
                            </div>
                            <p className="wk-muted text-muted">{credentialHint}</p>
                          </>
                        ) : null}
                      </div>
                    ) : (
                      <>
                        <label>
                          {apiKeyLabel}
                  <Input
                            type="password"
                            autoComplete="new-password"
                            spellCheck={false}
                            placeholder={apiKeyPlaceholder}
                            value={draft.apiKey}
                            onChange={(event) => updateDraft("apiKey", event.target.value)}
                          />
                        </label>
                        {signed ? (
                          <>
                            <label>
                              {secretKeyLabel}
                      <Input
                                type="password"
                                autoComplete="new-password"
                                spellCheck={false}
                                placeholder={secretKeyPlaceholder}
                                value={draft.appSecret}
                                onChange={(event) => updateDraft("appSecret", event.target.value)}
                              />
                            </label>
                            <p className="wk-muted text-muted">{credentialHint}</p>
                          </>
                        ) : null}
                      </>
                    )}
                    {signed === "lkeap" ? (
                      <label>
                        {t("model.editor.lkeap.regionLabel")}
                        <Input
                          placeholder={t("model.editor.lkeap.regionPlaceholder")}
                          value={draft.lkeapRegion}
                          onChange={(event) => updateDraft("lkeapRegion", event.target.value)}
                        />
                        <span className="wk-muted text-muted">{t("model.editor.lkeap.regionDesc")}</span>
                      </label>
                    ) : null}
                    <fieldset>
                      <legend>{t("model.editor.customHeadersLabel")}</legend>
                      <p className="wk-muted text-muted">{t("model.editor.customHeadersDesc")}</p>
                      {draft.customHeaders.map((item, index) => (
                        <div className="wk-model-header-row" key={index}>
                                <Input
                            value={item.key}
                            placeholder={t("model.editor.customHeadersKeyPlaceholder")}
                            aria-label={t("model.editor.customHeadersKeyPlaceholder")}
                            onChange={(event) => updateCustomHeader(index, "key", event.target.value)}
                          />
                          <Input
                            value={item.value}
                            placeholder={t("model.editor.customHeadersValuePlaceholder")}
                            aria-label={t("model.editor.customHeadersValuePlaceholder")}
                            onChange={(event) => updateCustomHeader(index, "value", event.target.value)}
                          />
                          <Button
                            type="button"
                            aria-label={t("common.delete")}
                            onClick={() => removeCustomHeader(index)}
                          >
                            ✕
                          </Button>
                        </div>
                      ))}
                      <Button type="button" onClick={addCustomHeader}>
                        {t("model.editor.customHeadersAdd")}
                      </Button>
                    </fieldset>
                  </>
                ) : null}
              </div>
            ) : null}

            {["embedding", "chat", "vllm"].includes(draft.type) ? (
              <div className="form-item">
                <h4>{t("model.editor.sectionAdvanced")}</h4>
                {draft.type === "embedding" ? (
                  <>
                    <label>
                      {t("model.editor.dimensionLabel")}
                      <NumberInput
                        min={128}
                        max={4096}
                        placeholder={t("model.editor.dimensionPlaceholder")}
                        disabled={!draft.supportsDimensionOverride || (draft.source === "local" && checking)}
                        value={draft.dimension}
                        onValueChange={(value) => updateDraft("dimension", value)}
                      />
                    </label>
                    {draft.source === "local" && draft.name ? (
                      <Button type="button" disabled={checking || !draft.name.trim()} onClick={() => void checkOllamaDimension()}>
                        {t("model.editor.checkDimension")}
                      </Button>
                    ) : null}
                    {dimensionMessage ? (
                      <Status tone={dimensionMessage.ok ? "success" : "error"}>{dimensionMessage.text}</Status>
                    ) : null}
                    <div className="mt-1 flex min-h-[22px] cursor-pointer flex-wrap items-center gap-x-2">
                      <Switch checked={draft.supportsDimensionOverride} onCheckedChange={(checked) => updateDraft("supportsDimensionOverride", checked)} aria-label={t("model.editor.dimensionOverrideLabel")} />
                      <span className="text-[13px] font-medium text-[rgba(0,0,0,0.9)]">{t("model.editor.dimensionOverrideLabel")}</span>
                      <span className="ml-11 mt-[2px] basis-full text-xs leading-[1.5] text-[#8a8a8a]">{t("model.editor.dimensionOverrideDesc")}</span>
                    </div>
                  </>
                ) : null}
                {draft.type === "chat" || draft.type === "vllm" ? (
                  <label>
                    {t("model.editor.contextWindowLabel")}
                    <NumberInput
                      min={1024}
                      max={10000000}
                      placeholder={t("model.editor.contextWindowPlaceholder", { value: DEFAULT_MODEL_CONTEXT_WINDOW })}
                      value={draft.contextWindow}
                      onValueChange={(value) => updateDraft("contextWindow", value)}
                    />
                    <span className="wk-muted text-muted">{t("model.editor.contextWindowDesc")}</span>
                  </label>
                ) : null}
                {draft.type === "chat" ? (
                  <>
                    <div className="mt-1 flex min-h-[22px] cursor-pointer flex-wrap items-center gap-x-2">
                      <Switch checked={draft.supportsVision} onCheckedChange={(checked) => updateDraft("supportsVision", checked)} aria-label={t("model.editor.supportsVisionLabel")} />
                      <span className="text-[13px] font-medium text-[rgba(0,0,0,0.9)]">{t("model.editor.supportsVisionLabel")}</span>
                      <span className="ml-11 mt-[2px] basis-full text-xs leading-[1.5] text-[#8a8a8a]">{t("model.editor.supportsVisionDesc")}</span>
                    </div>
                  </>
                ) : null}
                {draft.type === "chat" && draft.source === "remote" ? (
                  <label>
                    {t("model.editor.thinkingControlLabel")}
                    <ModelOptionSelect
                      value={draft.thinkingControl}
                      options={thinkingOptions}
                      onChange={(value) => {
                        setThinkingManual(true);
                        updateDraft("thinkingControl", value);
                      }}
                    />
                    <span className="wk-muted text-muted">{t("model.editor.thinkingControlDesc")}</span>
                  </label>
                ) : null}
                <label>
                  {t("model.editor.maxConcurrencyLabel")}
                  <NumberInput
                    min={0}
                    max={4096}
                    placeholder={t("model.editor.maxConcurrencyPlaceholder")}
                    value={draft.maxConcurrency}
                    onValueChange={(value) => updateDraft("maxConcurrency", value)}
                  />
                  <span className="wk-muted text-muted">{t("model.editor.maxConcurrencyDesc")}</span>
                </label>
              </div>
            ) : null}

            {draftError ? <Status tone="error">{draftError}</Status> : null}
            <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
              {draft.source === "remote" ? (
                <Button
                  type="button"
                  loading={checking}
                  disabled={
                    !draft.name ||
                    (!draft.baseUrl && draft.provider !== "weknoracloud") ||
                    (draft.provider === "weknoracloud" && wkcState !== "configured")
                  }
                  onClick={() => void testConnection()}
                >
                  {t("model.editor.testConnection")}
                </Button>
              ) : null}
              {remoteMessage ? (
                <Status tone={remoteMessage.ok ? "success" : "error"}>{remoteMessage.text}</Status>
              ) : null}
              {/* R484 G4 D4 — Vue footer order (SettingDrawer.vue footer):
                  footer-left 测试连接, footer-right 取消 then 保存. */}
              <Button type="button" disabled={busy} onClick={closeEditor}>
                {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                loading={busy}
                disabled={draft.provider === "weknoracloud" && wkcState !== "configured"}
              >
                {t("common.save")}
              </Button>
            </div>
          </form>
        </div>
        </div>
      ) : null}
      {debugOpen ? (
        <ModelDebugPanel
          client={client}
          models={models}
          onClose={() => setDebugOpen(false)}
        />
      ) : null}
    </section>
  );
}
