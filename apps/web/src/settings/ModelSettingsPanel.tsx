import { useEffect, useMemo, useRef, useState } from "react";
import * as React from "react";
import type {
  ModelConfiguration,
  WeKnoraClient,
} from "@weknora/api-client";
import { Icon as TIcon } from "tdesign-icons-react";
import {
  Button as TButton,
  Dropdown as TDropdown,
  Empty as TEmpty,
  Input as TInput,
  InputNumber as TInputNumber,
  Loading as TLoading,
  Popconfirm as TPopconfirm,
  Switch as TSwitch,
  Tabs,
  Tooltip as TTooltip,
} from "tdesign-react";
// S6 抽屉收编：packages/ui 旧栈 表单栈离开（T15 硬前置）。Status 为无 TDesign 对应
// 的语义 p 封装，走 shared/wk-legacy 原生标签 + .wk-status 类平移。
import { WkStatus as Status } from "../shared/wk-legacy.tsx";
import { SettingDrawer } from "./SettingDrawer.tsx";
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
// Vue ModelSettings.vue typeIcon（L395-404）：TDesign 自带 icon name 直译。
const TYPE_ICON: Record<ModelType, string> = {
  chat: "chat",
  embedding: "chart-bubble",
  rerank: "filter-sort",
  vllm: "image",
  asr: "sound",
};
/* CredentialResource.vue 同构端口（frontend/src/components/credentials/
 * CredentialResource.vue）：configured=faux input(✓ 已配置 + 更换|移除)、
 * confirm-remove=⚠ 确认行、unconfigured=faux empty(配置)、editing=t-input
 * + 底部 取消/保存。字段 row-label 仅在多字段（api_key+app_secret）时渲染，
 * 单字段沿用父级 .form-label（ModelEditorDialog API key / MCP 同款约定）。 */
function CredentialResourcePort(props: {
  t: (key: string) => string;
  fields: Array<{ key: "api_key" | "app_secret"; label: string }>;
  configured: Record<string, boolean>;
  values: Record<string, string>;
  onValueChange: (key: "api_key" | "app_secret", value: string) => void;
  onSave: (key: "api_key" | "app_secret") => void;
  onRemove: (key: "api_key" | "app_secret") => void;
  busy: boolean;
}) {
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [pendingRemoveKey, setPendingRemoveKey] = useState<string | null>(null);
  const single = props.fields.length === 1;
  return (
    <div className="credential-resource">
      {props.fields.map((field) => {
        const configured = !!props.configured[field.key];
        const editing = editingKey === field.key;
        const pendingRemove = pendingRemoveKey === field.key;
        return (
          <div className="credential-row" key={field.key}>
            {!single ? <div className="credential-row-label">{field.label}</div> : null}
            {editing ? (
              <div className="credential-edit">
                <TInput
                  type="password"
                  autocomplete="new-password"
                  prefixIcon={<TIcon name="lock-on" />}
                  value={props.values[field.key] ?? ""}
                  placeholder={props.t("credential.inputPlaceholder")}
                  onChange={(value) => props.onValueChange(field.key, String(value))}
                />
                <div className="credential-edit-actions">
                  <TButton size="small" variant="text" type="button" onClick={() => { setEditingKey(null); props.onValueChange(field.key, ""); }}>
                    {props.t("common.cancel")}
                  </TButton>
                  <TButton
                    size="small"
                    theme="primary"
                    loading={props.busy}
                    disabled={!(props.values[field.key] ?? "").trim()}
                    type="button"
                    onClick={() => { props.onSave(field.key); setEditingKey(null); }}
                  >
                    {props.t("common.save")}
                  </TButton>
                </div>
              </div>
            ) : configured && pendingRemove ? (
              <div className="credential-faux-input is-confirm-remove">
                <TIcon name="error-circle-filled" className="status-icon warn" />
                <span className="credential-faux-text danger">{props.t("credential.confirmRemovePrompt")}</span>
                <div className="credential-actions">
                  <TButton size="small" variant="text" type="button" onClick={() => setPendingRemoveKey(null)}>
                    {props.t("common.cancel")}
                  </TButton>
                  <span className="action-divider" />
                  <TButton
                    size="small"
                    variant="text"
                    theme="danger"
                    loading={props.busy}
                    type="button"
                    onClick={() => { props.onRemove(field.key); setPendingRemoveKey(null); }}
                  >
                    {props.t("credential.confirmRemove")}
                  </TButton>
                </div>
              </div>
            ) : configured ? (
              <div className="credential-faux-input" title={props.t("credential.configured")}>
                <TIcon name="check-circle-filled" className="status-icon success" />
                <span className="credential-faux-text">{props.t("credential.configured")}</span>
                <div className="credential-actions">
                  <TButton size="small" variant="text" type="button" onClick={() => setEditingKey(field.key)}>
                    {props.t("credential.update")}
                  </TButton>
                  <span className="action-divider" />
                  <TButton size="small" variant="text" theme="danger" type="button" onClick={() => setPendingRemoveKey(field.key)}>
                    {props.t("credential.remove")}
                  </TButton>
                </div>
              </div>
            ) : (
              <div className="credential-faux-input is-empty" onClick={() => setEditingKey(field.key)}>
                <TIcon name="lock-on" className="status-icon muted" />
                <span className="credential-faux-text muted">{props.t("credential.unconfigured")}</span>
                <div className="credential-actions">
                  <TButton size="small" variant="text" theme="primary" type="button" onClick={(event) => { event.stopPropagation(); setEditingKey(field.key); }}>
                    {props.t("credential.configure")}
                  </TButton>
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

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
function fromTInputNumber(value: number | string): number | "" {
  if (typeof value === "number") return value;
  return value.trim() === "" ? "" : Number(value);
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
    // Vue 侧确认由卡面 t-popconfirm 承载（ModelSettings.vue L82-91），此处直删。
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

  // Type badge icon per ModelSettings.vue typeIcon (L395-404) — t-icon glyph 直译。
  function typeIconName(type: ModelType): string {
    return TYPE_ICON[type];
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
    <div className="model-settings" data-testid="model-settings">
      {/* Vue ModelSettings.vue section-header 逐节点平移（T12b）：h2 + 描述 +
          模型测试 t-button（#icon slot → icon prop；label 前导空格单文本节点，
          台账 #11/#13 先例）+ builtin-models-hint 提示盒。样式 settings.td.css §12。 */}
      <div className="section-header">
        <div className="section-header__top">
          <div>
            <h2>{t("modelSettings.title")}</h2>
            <p className="section-description">{t("modelSettings.description")}</p>
          </div>
          {canCreate ? (
            <TButton type="button" theme="primary" variant="text" size="medium" className="model-test-trigger" icon={<TIcon name="play-circle" />} onClick={() => setDebugOpen(true)}>
              {` ${t("modelSettings.actions.debugModel")}`}
            </TButton>
          ) : null}
        </div>
        <div className="builtin-models-hint" role="note">
          <p className="builtin-hint-label">{t("modelSettings.builtinModels.title")}</p>
          <p className="builtin-hint-text">
            {t(role === "system-admin" ? "modelSettings.builtinModels.descriptionAdmin" : "modelSettings.builtinModels.description")}
          </p>
          {/* Vue 文本插值与 t-icon 间换行缩进＝尾部空格文本节点，单文本节点复刻。 */}
          <a className="doc-link" href={BUILTIN_MODELS_DOC} target="_blank" rel="noopener noreferrer">
            {`${t("modelSettings.builtinModels.viewGuide")} `}
            <TIcon name="link" className="link-icon" />
          </a>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {usageConflict ? <ModelUsageNotice modelName={usageConflict.modelName} details={usageConflict.details} onClose={() => setUsageConflict(null)} /> : null}
      {/* Vue t-tabs（v-model=activeTypeFilter）：label-only 面板，content 区
          display:none（§12 :deep 平移）。 */}
      <Tabs value={filter} onChange={(value) => setFilter(value as "all" | ModelType)} className="model-type-tabs">
        <Tabs.TabPanel value="all" label={`${t("common.all")}(${models.length})`} />
        {TYPES.map((type) => (
          <Tabs.TabPanel key={type} value={type} label={`${typeLabelOf(type)}(${models.filter((item) => modelType(item) === type).length})`} />
        ))}
      </Tabs>
      <TLoading loading={false} size="small" className="model-list-loading">
      {!canCreate && visible.length === 0 ? (
        <div className="empty-state">
          <TEmpty description={emptyHint} />
        </div>
      ) : (
        <div className="model-grid">
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
            return (
              <div
                key={`${type}-${model.id}`}
                className={"model-card model-card--" + type
                  + (builtin ? " model-card--builtin" : "")
                  + (canEdit ? " model-card--clickable" : "")}
                role={canEdit ? "button" : undefined}
                tabIndex={canEdit ? 0 : undefined}
                onClick={canEdit ? () => openEdit(model) : undefined}
                onKeyDown={canEdit ? (event) => { if (event.key === "Enter") openEdit(model); } : undefined}
              >
                <div className="model-card__badge" aria-label={typeLabelOf(type)}>
                  <TIcon name={typeIconName(type)} size="18px" />
                </div>
                <div className="model-card__body">
                  <div className="model-card__header">
                    <h3 className="model-card__title">{label(model)}</h3>
                    {builtin ? (
                      <span className="model-card__lock" title={t("modelSettings.builtinTag")} aria-label={t("modelSettings.builtinTag")}>
                        <TIcon name={role === "system-admin" ? "edit-1" : "lock-on"} />
                      </span>
                    ) : null}
                    {canEdit ? (
                      <div className="model-card__actions" onClick={(event) => event.stopPropagation()}>
                        {/* Vue getModelOptions：builtin（system-admin）→仅编辑；
                            非 builtin（admin）→编辑+复制。 */}
                        <TDropdown
                          options={builtin
                            ? [{ content: t("common.edit"), value: "edit" }]
                            : [{ content: t("common.edit"), value: "edit" }, { content: t("common.copy"), value: "copy" }]}
                          placement="bottom-right"
                          trigger="click"
                          onClick={(data) => {
                            const value = String(data?.value ?? "");
                            if (value === "edit") openEdit(model);
                            else if (value === "copy") void copyModel(model);
                          }}
                        >
                          <TButton variant="text" shape="square" size="small" className="model-card__action-btn model-card__more">
                            <TIcon name="ellipsis" />
                          </TButton>
                        </TDropdown>
                        {!builtin ? (
                          <TPopconfirm
                            content={t("modelSettings.confirmDelete", { name: label(model) })}
                            confirmBtn={{ content: t("common.delete"), theme: "danger" }}
                            cancelBtn={{ content: t("common.cancel") }}
                            placement="bottom-right"
                            onConfirm={() => void remove(model)}
                          >
                            <TTooltip content={t("common.delete")} placement="top">
                              <TButton theme="danger" shape="square" variant="text" size="small" className="model-card__action-btn model-card__delete" icon={<TIcon name="delete" />} onClick={(event) => event.stopPropagation()} />
                            </TTooltip>
                          </TPopconfirm>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                  <p className="model-card__subtitle">
                    <span>{vendorLabel(model)}</span>
                    {type === "embedding" && typeof dimension === "number" ? (
                      <>
                        <span className="model-card__sep">·</span>
                        <span>{`${t("model.editor.dimensionLabel")} ${dimension}`}</span>
                      </>
                    ) : null}
                    {/* Vue ModelSettings.vue L110-117 renders the ctx chip for
                        every chat/vllm card; formatContextWindow falls back to
                        the 200K default (dimmed) when no value is stored. */}
                    {(type === "chat" || type === "vllm") ? (
                      <>
                        <span className="model-card__sep">·</span>
                        <span
                          className={"model-card__ctx" + (isDefaultContextWindow(contextWindow) ? " model-card__ctx--default" : "")}
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
                        <span className="model-card__sep">·</span>
                        <span className="model-card__vision" title={t("model.editor.supportsVisionLabel")} aria-label={t("model.editor.supportsVisionLabel")}>
                          <TIcon name="image" size="12px" />
                        </span>
                      </>
                    ) : null}
                  </p>
                </div>
              </div>
            );
          })}
          {canCreate ? (
            <button type="button" className="model-card model-card--add" data-guide="settings-add-model" onClick={openAdd}>
              <span className="model-card--add__icon" aria-hidden="true">
                <TIcon name="add" />
              </span>
              <span className="model-card--add__label">{t("modelSettings.actions.addModel")}</span>
            </button>
          ) : null}
        </div>
      )}
      </TLoading>
      {/* Vue ModelEditorDialog.vue:1-403 — SettingDrawer 抽屉（
          model-editor-drawer 家族 + sectionType 类型 pill + sectionSource 来源
          + remote 厂商 section + sectionAdvanced）。基线册 px-models-model-card
          根因收敛：wk-model-editor 居中壳 → SettingDrawer 家族。 */}
      {draft ? <SettingDrawer
        visible
        title={draft.id ? t("model.editor.editTitle") : t("model.editor.addTitle")}
        description={t(`model.editor.description.${draft.type}`) || t("model.editor.description.default")}
        icon={TYPE_ICON[draft.type]}
        confirmLoading={busy}
        confirmDisabled={draft.provider === "weknoracloud" && wkcState !== "configured"}
        drawerClass="model-editor-drawer"
        footerLeft={draft.source === "remote" ? <>
          <TButton
            variant="outline"
            loading={checking}
            disabled={
              !draft.name ||
              (!draft.baseUrl && draft.provider !== "weknoracloud") ||
              (draft.provider === "weknoracloud" && wkcState !== "configured")
            }
            onClick={() => void testConnection()}
          >
            {!checking && remoteMessage?.ok ? <TIcon name="check-circle-filled" className="status-icon available" />
              : !checking && remoteMessage && !remoteMessage.ok ? <TIcon name="close-circle-filled" className="status-icon unavailable" /> : null}
            {t("model.editor.testConnection")}
          </TButton>
          {remoteMessage ? (
            <span className={'footer-test-message ' + (remoteMessage.ok ? 'success' : 'error')} title={remoteMessage.text}>
              {remoteMessage.text}
            </span>
          ) : null}
        </> : undefined}
        onVisibleChange={(visible) => { if (!visible) closeEditor(); }}
        onCancel={closeEditor}
        onConfirm={() => { void save(new Event("submit") as unknown as React.FormEvent<HTMLFormElement>); }}
      >
        <form className="t-form t-form--layout-vertical" onSubmit={(event) => void save(event)}>
          {!draft.id ? (
            <section className="setting-drawer__section">
              <h4 className="setting-drawer__section-title">{t("model.editor.sectionType")}</h4>
              <div className="model-type-options" role="radiogroup" aria-label={t("model.editor.typeLabel")}>
                {TYPES.map((type) => (
                  <button
                    key={type}
                    type="button"
                    role="radio"
                    aria-checked={draft.type === type}
                    className={'model-type-option' + (draft.type === type ? ' is-active' : '')}
                    onClick={() => void selectModelType(type)}
                  >
                    <TIcon name={TYPE_ICON[type]} className="model-type-option__icon" />
                    <span className="model-type-option__label">{typeLabelOf(type)}</span>
                  </button>
                ))}
              </div>
            </section>
          ) : null}

          {/* Vue ModelEditorDialog.vue:54-149 — sectionSource：来源 pill +
              rerank/ollama 提示 + Ollama 本地模型选择行 */}
          <section className="setting-drawer__section">
            <h4 className="setting-drawer__section-title">{t("model.editor.sectionSource")}</h4>
            <div className="form-item">
              <div className="source-options" role="radiogroup" aria-label={t("model.editor.sourceLabel")}>
                <button
                  type="button"
                  role="radio"
                  aria-checked={draft.source === "remote"}
                  className={'source-option' + (draft.source === "remote" ? ' is-active' : '')}
                  onClick={() => selectSource("remote")}
                >
                  <TIcon name="cloud" className="source-option__icon" />
                  <span className="source-option__label">{t("model.editor.sourceRemote")}</span>
                </button>
                <button
                  type="button"
                  role="radio"
                  aria-checked={draft.source === "local"}
                  className={'source-option' + (draft.source === "local" ? ' is-active' : '') + (ollamaStatus === false || draft.type === "rerank" ? ' is-disabled' : '')}
                  disabled={ollamaStatus === false || draft.type === "rerank"}
                  onClick={() => selectSource("local")}
                >
                  <TIcon name="server" className="source-option__icon" />
                  <span className="source-option__label">{t("model.editor.sourceLocal")}</span>
                </button>
              </div>
              {draft.type === "rerank" ? (
                <div className="ollama-unavailable-tip rerank-tip">
                  <TIcon name="info-circle-filled" className="tip-icon info" />
                  <span className="tip-text">{t("model.editor.ollamaNotSupportRerank")}</span>
                </div>
              ) : draft.source === "local" && ollamaStatus === false ? (
                <div className="ollama-unavailable-tip">
                  <TIcon name="error-circle-filled" className="tip-icon" />
                  <span className="tip-text">{t("model.editor.ollamaUnavailable")}</span>
                  <TButton type="button" variant="text" size="small" className="tip-link" onClick={() => navigate("/platform/settings?section=ollama")}>
                    <TIcon name="jump" />
                    {t("model.editor.goToOllamaSettings")}
                  </TButton>
                </div>
              ) : null}
              {draft.source === "local" ? (
                <div className="form-item">
                  <label className="form-label required">{t("model.modelName")}</label>
                  {/* Vue ModelEditorDialog.vue:109-147 — .model-select-row：
                      filterable t-select（自研 combobox 逻辑零改动保留）+ 刷新
                      按钮（text small refresh icon）。 */}
                  <div className="model-select-row">
                  <div className="wk-ollama-combobox-wrap">
                      {/* 自研 combobox（Vue 端为 ModelEditorDialog.vue 的 filterable
                          t-select；React 侧逻辑零改动保留自研键盘导航），input 走原生
                          标签以保住 role/aria 契约（tdesign Input 不透传，台账 #8 同因），
                          chrome 由 .wk-ollama-combobox-wrap input 对齐 t-input。 */}
                      <input
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
                        <div className="wk-ollama-listbox" id="wk-ollama-listbox" role="listbox">
                          {ollamaSuggestions.map((item, index) => (
                            <button
                              type="button"
                              key={item.name}
                              role="option"
                              aria-selected={index === ollamaHighlight}
                              className={"wk-ollama-option" + (index === ollamaHighlight ? " is-active" : "")}
                              onMouseDown={(event) => { event.preventDefault(); selectOllamaModel(item.name); }}
                            >
                              <span className="wk-ollama-option__check" aria-hidden="true">✓</span>
                              <span className="wk-ollama-option__name">{item.name}</span>
                              <span className="wk-ollama-option__size">{formatModelSize((item as Record<string, unknown>).size)}</span>
                            </button>
                          ))}
                          {ollamaDownloadOffered ? (
                            <button
                              type="button"
                              role="option"
                              aria-selected={ollamaHighlight === ollamaSuggestions.length}
                              className={"wk-ollama-option" + (ollamaHighlight === ollamaSuggestions.length ? " is-active" : "")}
                              onMouseDown={(event) => { event.preventDefault(); setOllamaOpen(false); void downloadOllamaModel(); }}
                            >
                              <span aria-hidden="true">⬇</span>
                              <span className="wk-ollama-option__name wk-ollama-option__download">{t("model.editor.downloadLabel", { keyword: ollamaKeyword })}</span>
                            </button>
                          ) : null}
                        </div>
                      ) : null}
                  </div>
                  <TButton type="button" variant="text" size="small" className="refresh-btn" disabled={ollamaBusy} onClick={() => void refreshOllamaModels()}>
                    <TIcon name="refresh" />
                    {t("model.editor.refreshList")}
                  </TButton>
                  </div>
                  {nameError ? <span className="wk-field-error">{nameError}</span> : null}
                  {ollamaBusy ? <Status>{t("common.loading")}</Status> : null}
                  {downloadTask || downloadProgress !== null ? (
                    <Status>{`${draft.name} · ${downloadProgress !== null ? `${downloadProgress.toFixed(1)}%` : "0%"}`}</Status>
                  ) : null}
                </div>
              ) : null}
            </div>
          </section>

          {draft.source === "remote" ? (
            <section className="setting-drawer__section">
              <h4 className="setting-drawer__section-title">{t("model.editor.sectionProvider")}</h4>
              <div className="form-item">
                <label className="form-label">{t("model.editor.providerLabel")}</label>
                <ModelOptionSelect
                  value={draft.provider}
                  disabled={loadingProviders}
                  options={editorProviderOptions}
                  onChange={onProviderChange}
                />
              </div>
              {draft.provider === "weknoracloud" ? (
                wkcState === "loading" ? (
                  <div className="weknoracloud-hint">
                    <TIcon name="loading" className="spinning hint-icon hint-icon--loading" />
                    <span>{t("settings.weknoraCloud.checkingStatus")}</span>
                  </div>
                ) : wkcState === "configured" ? (
                  <div className="weknoracloud-hint weknoracloud-hint--ok">
                    <TIcon name="check-circle-filled" className="hint-icon hint-icon--ok" />
                    <div>{t("settings.weknoraCloud.modelHintConfigured")}</div>
                  </div>
                ) : (
                  <div className="weknoracloud-hint weknoracloud-hint--warn">
                    <TIcon name="error-circle-filled" className="hint-icon hint-icon--warn" />
                    <div style={{ flex: 1 }}>
                      {t(wkcState === "expired" ? "settings.weknoraCloud.credentialExpired" : "settings.weknoraCloud.credentialUnconfigured")}
                      <div style={{ marginTop: 8 }}>
                        <TButton type="button" variant="text" size="small" style={{ padding: 0, height: 'auto' }} onClick={() => navigate("/platform/settings?section=weknoracloud")}>
                          <TIcon name="jump" />
                          {t("settings.weknoraCloud.goToSettings")}
                        </TButton>
                      </div>
                    </div>
                  </div>
                )
              ) : null}
              <div className="form-item">
                <label className="form-label required">{t("model.modelName")}</label>
                <TInput
                  maxlength={100}
                  placeholder={t(modelNamePlaceholderKey(draft.type, draft.source))}
                  disabled={draft.provider === "weknoracloud" && wkcState !== "configured"}
                  value={draft.name}
                  onChange={(value) => changeName(String(value))}
                  onBlur={blurName}
                />
                {nameError ? <span className="wk-field-error">{nameError}</span> : null}
              </div>
              <div className="form-item">
                <label className="form-label">{t("model.editor.displayNameLabel")}</label>
                <TInput
                  maxlength={100}
                  placeholder={t("model.editor.displayNamePlaceholder")}
                  value={draft.displayName}
                  onChange={(value) => updateDraft("displayName", String(value))}
                />
                <p className="form-desc">{t("model.editor.displayNameDesc")}</p>
              </div>
              {draft.provider !== "weknoracloud" ? (
                <>
              <div className="form-item">
                <label className="form-label required">{t("model.editor.baseUrlLabel")}</label>
                <TInput
                  type="url"
                  placeholder={t(baseUrlPlaceholderKey(draft.type))}
                  value={draft.baseUrl}
                  onChange={(value) => changeBaseUrl(String(value))}
                  onBlur={blurBaseUrl}
                />
                {baseUrlError ? <span className="wk-field-error">{baseUrlError}</span> : null}
              </div>
                {draft.id ? (
                  <div className="form-item">
                    <label className="form-label">{apiKeyLabel}</label>
                    {/* Vue edit 模式：凭证由 CredentialResource 卡管理
                        （ModelEditorDialog.vue:251-252），signed rerank 追加
                        app_secret 字段行 + 提示。 */}
                    <CredentialResourcePort
                      t={t}
                      fields={[
                        { key: "api_key", label: apiKeyLabel },
                        ...(signed ? [{ key: "app_secret" as const, label: secretKeyLabel }] : []),
                      ]}
                      configured={{
                        api_key: !!draft.credentials?.api_key?.configured,
                        app_secret: !!draft.credentials?.app_secret?.configured,
                      }}
                      values={{ apiKey: credentialValues.apiKey, app_secret: credentialValues.appSecret }}
                      onValueChange={(key, value) => setCredentialValues((current) => ({ ...current, [key === "api_key" ? "apiKey" : "appSecret"]: value }))}
                      onSave={(key) => void saveCredentialField(key === "api_key" ? "apiKey" : "appSecret")}
                      onRemove={(key) => void removeCredentialField(key)}
                      busy={credentialBusy}
                    />
                    {signed ? <p className="form-desc">{credentialHint}</p> : null}
                  </div>
                ) : (
                  <div className="form-item">
                    <label className="form-label">{apiKeyLabel}</label>
                    <TInput
                      type="password"
                      autocomplete="new-password"
                      spellCheck={false}
                      placeholder={apiKeyPlaceholder}
                      value={draft.apiKey}
                      onChange={(value) => updateDraft("apiKey", String(value))}
                    />
                    {signed ? (
                      <>
                        <label className="form-label">{secretKeyLabel}</label>
                        <TInput
                          type="password"
                          autocomplete="new-password"
                          spellCheck={false}
                          placeholder={secretKeyPlaceholder}
                          value={draft.appSecret}
                          onChange={(value) => updateDraft("appSecret", String(value))}
                        />
                        <p className="form-desc">{credentialHint}</p>
                      </>
                    ) : null}
                  </div>
                )}
                {signed === "lkeap" ? (
                  <div className="form-item">
                    <label className="form-label">{t("model.editor.lkeap.regionLabel")}</label>
                    <TInput
                      placeholder={t("model.editor.lkeap.regionPlaceholder")}
                      value={draft.lkeapRegion}
                      onChange={(value) => updateDraft("lkeapRegion", String(value))}
                    />
                    <p className="form-desc">{t("model.editor.lkeap.regionDesc")}</p>
                  </div>
                ) : null}
              <div className="form-item">
                <div className="custom-headers-header">
                  <label className="form-label" style={{ marginBottom: 0 }}>{t("model.editor.customHeadersLabel")}</label>
                  <TButton variant="text" size="small" theme="primary" type="button" icon={<TIcon name="add" />} onClick={addCustomHeader}>
                    {t("model.editor.customHeadersAdd")}
                  </TButton>
                </div>
                <p className="form-desc custom-headers-desc">{t("model.editor.customHeadersDesc")}</p>
                {draft.customHeaders.length > 0 ? <div className="custom-headers-list">
                  {draft.customHeaders.map((item, index) => (
                    <div className="custom-header-row" key={index}>
                      <TInput
                        className="custom-header-key"
                        value={item.key}
                        placeholder={t("model.editor.customHeadersKeyPlaceholder")}
                        onChange={(value) => updateCustomHeader(index, "key", String(value))}
                      />
                      <TInput
                        className="custom-header-value"
                        value={item.value}
                        placeholder={t("model.editor.customHeadersValuePlaceholder")}
                        onChange={(value) => updateCustomHeader(index, "value", String(value))}
                      />
                      <TButton
                        variant="text"
                        shape="square"
                        size="small"
                        className="custom-header-remove"
                        aria-label={t("common.delete")}
                        type="button"
                        onClick={() => removeCustomHeader(index)}
                      >
                        <TIcon name="close" />
                      </TButton>
                    </div>
                  ))}
                </div> : null}
              </div>
                </>
              ) : null}
            </section>
          ) : null}

          {/* Vue ModelEditorDialog.vue:316-399 — sectionAdvanced */}
          {["embedding", "chat", "vllm"].includes(draft.type) ? (
            <section className="setting-drawer__section">
              <h4 className="setting-drawer__section-title">{t("model.editor.sectionAdvanced")}</h4>
              {draft.type === "embedding" ? (
                <>
                  <div className="form-item">
                    <label className="form-label">{t("model.editor.dimensionLabel")}</label>
                    <div className="dimension-control">
                      <TInputNumber
                        min={128}
                        max={4096}
                        placeholder={t("model.editor.dimensionPlaceholder")}
                        disabled={!draft.supportsDimensionOverride || (draft.source === "local" && checking)}
                        value={draft.dimension}
                        onChange={(value) => updateDraft("dimension", fromTInputNumber(value))}
                      />
                      {draft.source === "local" && draft.name ? (
                        <TButton type="button" variant="text" size="small" className="dimension-check-btn" disabled={checking || !draft.name.trim()} onClick={() => void checkOllamaDimension()}>
                          <TIcon name="refresh" />
                          {t("model.editor.checkDimension")}
                        </TButton>
                      ) : null}
                    </div>
                    {dimensionMessage ? (
                      <p className={'dimension-hint' + (dimensionMessage.ok ? ' success' : '')}>{dimensionMessage.text}</p>
                    ) : null}
                  </div>
                  <div className="form-item">
                    <label className="form-label">{t("model.editor.dimensionOverrideLabel")}</label>
                    <div className="vision-toggle">
                      <TSwitch value={draft.supportsDimensionOverride} onChange={(checked) => updateDraft("supportsDimensionOverride", Boolean(checked))} />
                      <span className="form-desc form-desc--inline">{t("model.editor.dimensionOverrideDesc")}</span>
                    </div>
                  </div>
                </>
              ) : null}
              {draft.type === "chat" || draft.type === "vllm" ? (
                <div className="form-item">
                  <label className="form-label">{t("model.editor.contextWindowLabel")}</label>
                  <TInputNumber
                    min={1024}
                    max={10000000}
                    placeholder={t("model.editor.contextWindowPlaceholder", { value: DEFAULT_MODEL_CONTEXT_WINDOW })}
                    value={draft.contextWindow}
                    onChange={(value) => updateDraft("contextWindow", fromTInputNumber(value))}
                  />
                  <p className="form-desc">{t("model.editor.contextWindowDesc")}</p>
                </div>
              ) : null}
              {draft.type === "chat" ? (
                <div className="form-item">
                  <label className="form-label">{t("model.editor.supportsVisionLabel")}</label>
                  <div className="vision-toggle">
                    <TSwitch value={draft.supportsVision} onChange={(checked) => updateDraft("supportsVision", Boolean(checked))} />
                    <span className="form-desc form-desc--inline">{t("model.editor.supportsVisionDesc")}</span>
                  </div>
                </div>
              ) : null}
              {draft.type === "chat" && draft.source === "remote" ? (
                <div className="form-item">
                  <label className="form-label">{t("model.editor.thinkingControlLabel")}</label>
                  <ModelOptionSelect
                    value={draft.thinkingControl}
                    options={thinkingOptions}
                    onChange={(value) => {
                      setThinkingManual(true);
                      updateDraft("thinkingControl", value);
                    }}
                  />
                  <p className="form-desc">{t("model.editor.thinkingControlDesc")}</p>
                </div>
              ) : null}
              <div className="form-item">
                <label className="form-label">{t("model.editor.maxConcurrencyLabel")}</label>
                <TInputNumber
                  min={0}
                  max={4096}
                  placeholder={t("model.editor.maxConcurrencyPlaceholder")}
                  value={draft.maxConcurrency}
                  onChange={(value) => updateDraft("maxConcurrency", fromTInputNumber(value))}
                />
                <p className="form-desc">{t("model.editor.maxConcurrencyDesc")}</p>
              </div>
            </section>
          ) : null}

          {draftError ? <Status tone="error">{draftError}</Status> : null}
        </form>
      </SettingDrawer> : null}
      {debugOpen ? (
        <ModelDebugPanel
          client={client}
          models={models}
          onClose={() => setDebugOpen(false)}
        />
      ) : null}
    </div>
  );
}
