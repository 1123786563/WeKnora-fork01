import { useEffect, useMemo, useState } from "react";
import * as React from "react";
import type {
  ModelConfiguration,
  ModelProvider,
  WeKnoraClient,
} from "@weknora/api-client";
import { Button, Card, Status } from "@weknora/ui";
import { ModelDebugPanel } from "./ModelDebugPanel.tsx";
import { ModelUsageNotice } from "../configuration/ModelUsageNotice.tsx";
import { modelInUseDetails, type ModelUsageDetails } from "../configuration/model-usage.ts";
import {
  modelCredentialInput,
  modelDraftFromRecord,
  modelPayload,
  modelType,
  newModelDraft,
  validateModelDraft,
  type ModelDraft,
  type ModelType,
} from "./model-settings.ts";

type Props = {
  client: WeKnoraClient;
  role: "viewer" | "admin" | "owner" | "system-admin";
  initialModels: readonly ModelConfiguration[];
};
const TYPES: ModelType[] = ["chat", "embedding", "rerank", "vllm", "asr"];
const TYPE_LABELS: Record<ModelType, string> = {
  chat: "Chat",
  embedding: "Embedding",
  rerank: "Rerank",
  vllm: "VLLM",
  asr: "ASR",
};

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

export function ModelSettingsPanel({ client, role, initialModels }: Props) {
  const [models, setModels] =
    useState<readonly ModelConfiguration[]>(initialModels);
  const [filter, setFilter] = useState<"all" | ModelType>("all");
  const [draft, setDraft] = useState<ModelDraft | null>(null);
  const [providers, setProviders] = useState<ModelProvider[]>([]);
  const [loadingProviders, setLoadingProviders] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [debugOpen, setDebugOpen] = useState(false);
  const [usageConflict, setUsageConflict] = useState<{ modelName: string; details: ModelUsageDetails } | null>(null);
  const [connectionResult, setConnectionResult] = useState<Awaited<
    ReturnType<WeKnoraClient["configuration"]["models"]["connection"]["remote"]>
  > | null>(null);
  const canCreate = role === "admin" || role === "owner";
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
  useEffect(() => {
    if (!draft) return;
    setLoadingProviders(true);
    void client.configuration.models.providers
      .list(draft.type)
      .then((items) =>
        setProviders(
          items.length > 0
            ? items
            : [
                {
                  value: "generic",
                  label: "Generic",
                  description: "",
                  defaultUrls: {},
                  modelTypes: [],
                },
              ],
        ),
      )
      .catch(() => {
        setProviders([
          {
            value: "generic",
            label: "Generic",
            description: "",
            defaultUrls: {},
            modelTypes: [],
          },
        ]);
        setError("Unable to load model providers; using Generic.");
      })
      .finally(() => setLoadingProviders(false));
  }, [client, draft?.type]);

  function updateDraft<K extends keyof ModelDraft>(
    key: K,
    value: ModelDraft[K],
  ) {
    setDraft((current) => (current ? { ...current, [key]: value } : current));
  }
  async function testConnection() {
    if (!draft || busy || draft.source !== "remote") return;
    setBusy(true);
    setError(null);
    setUsageConflict(null);
    setConnectionResult(null);
    const input = {
      source: draft.source,
      modelName: draft.name,
      baseUrl: draft.baseUrl,
      provider: draft.provider,
      dimension:
        typeof draft.dimension === "number" ? draft.dimension : undefined,
      customHeaders: draft.customHeaders,
      apiKey: draft.apiKey.trim() || undefined,
      appSecret: draft.appSecret.trim() || undefined,
      modelId: draft.id || undefined,
    };
    try {
      const result =
        draft.type === "embedding"
          ? await client.configuration.models.connection.embedding(input)
          : draft.type === "rerank"
            ? await client.configuration.models.connection.rerank(input)
            : draft.type === "asr"
              ? await client.configuration.models.connection.asr(input)
              : await client.configuration.models.connection.remote(input);
      setConnectionResult(result);
      if (!result.available) setError(result.message);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Unable to test model connection",
      );
    } finally {
      setBusy(false);
    }
  }
  async function reload() {
    try {
      setModels(await client.configuration.models.list());
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Unable to load models",
      );
    }
  }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || busy) return;
    const invalid = validateModelDraft(draft);
    if (invalid.length > 0) {
      setError(invalid.join(", "));
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    let savedId = draft.id;
    try {
      const saved = draft.id
        ? await client.configuration.models.update(
            draft.id,
            modelPayload(draft),
          )
        : await client.configuration.models.create(modelPayload(draft));
      savedId = saved.id;
      const credentials = modelCredentialInput(draft);
      if (Object.keys(credentials).length > 0) {
        try {
          await client.configuration.models.credentials.put(
            saved.id,
            credentials,
          );
        } catch (cause) {
          setDraft({ ...draft, id: saved.id });
          throw cause;
        }
      }
      setDraft(null);
      setNotice(draft.id ? "Model updated." : "Model added.");
      await reload();
    } catch (cause) {
      if (savedId)
        setDraft((current) =>
          current ? { ...current, id: savedId } : current,
        );
      setError(cause instanceof Error ? cause.message : "Unable to save model");
    } finally {
      setBusy(false);
    }
  }
  async function remove(model: ModelConfiguration) {
    if (
      !canCreate ||
      (model as Record<string, unknown>).is_builtin === true ||
      busy
    )
      return;
    if (!window.confirm(`Delete model “${label(model)}”?`)) return;
    setBusy(true);
    setError(null);
    setUsageConflict(null);
    try {
      await client.configuration.models.remove(model.id);
      setNotice("Model deleted.");
      await reload();
    } catch (cause) {
      const details = modelInUseDetails(cause);
      if (details) {
        setUsageConflict({ modelName: label(model), details });
        return;
      }
      setError(
        cause instanceof Error ? cause.message : "Unable to delete model",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="wk-model-settings" data-testid="model-settings">
      <div className="wk-settings-panel-heading">
        <div>
          <h3>Models</h3>
          <p className="wk-muted">
            Configure tenant model providers. Credentials are write-only and
            never returned in model records.
          </p>
        </div>
        <div className="wk-list-actions">
          <Button
            type="button"
            disabled={models.length === 0}
            onClick={() => setDebugOpen(true)}
          >
            Debug model
          </Button>
          <Button type="button" disabled={busy} onClick={() => void reload()}>
            Refresh
          </Button>
          {canCreate ? (
            <Button
              type="button"
              onClick={() => {
                setConnectionResult(null);
                setDraft(newModelDraft());
              }}
            >
              Add model
            </Button>
          ) : null}
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {usageConflict ? <ModelUsageNotice modelName={usageConflict.modelName} details={usageConflict.details} onClose={() => setUsageConflict(null)} /> : null}
      <nav className="wk-model-tabs" aria-label="Model type">
        <button
          type="button"
          className={filter === "all" ? "is-active" : ""}
          onClick={() => setFilter("all")}
        >
          All ({models.length})
        </button>
        {TYPES.map((type) => (
          <button
            type="button"
            key={type}
            className={filter === type ? "is-active" : ""}
            onClick={() => setFilter(type)}
          >
            {TYPE_LABELS[type]} (
            {models.filter((item) => modelType(item) === type).length})
          </button>
        ))}
      </nav>
      {visible.length === 0 ? (
        <Status>No models configured.</Status>
      ) : (
        <div className="wk-model-grid">
          {visible.map((model) => {
            const type = modelType(model);
            const modelParams = params(model);
            const builtin =
              (model as Record<string, unknown>).is_builtin === true;
            return (
              <Card
                key={model.id}
                className={`wk-model-card wk-model-card--${type}`}
              >
                <div className="wk-model-card-header">
                  <div>
                    <span className="wk-model-type">{TYPE_LABELS[type]}</span>
                    <h4>{label(model)}</h4>
                  </div>
                  {builtin ? (
                    <span title="Built-in model">Built-in</span>
                  ) : null}
                </div>
                <p className="wk-muted">
                  {typeof model.source === "string" ? model.source : "remote"} ·{" "}
                  {typeof modelParams.provider === "string"
                    ? modelParams.provider
                    : "generic"}
                  {typeof modelParams.dimension === "number"
                    ? ` · dimension ${modelParams.dimension}`
                    : ""}
                  {typeof modelParams.context_window === "number"
                    ? ` · ${modelParams.context_window} context`
                    : ""}
                </p>
                <div className="wk-list-actions">
                  {canCreate || (role === "system-admin" && builtin) ? (
                    <Button
                      type="button"
                      onClick={() => {
                        setConnectionResult(null);
                        setDraft(modelDraftFromRecord(model));
                      }}
                    >
                      Edit
                    </Button>
                  ) : null}
                  {canCreate && !builtin ? (
                    <Button
                      type="button"
                      disabled={busy}
                      onClick={() => void remove(model)}
                    >
                      Delete
                    </Button>
                  ) : null}
                </div>
              </Card>
            );
          })}
        </div>
      )}
      {draft ? (
        <div
          className="wk-model-editor"
          role="dialog"
          aria-modal="true"
          aria-label={draft.id ? "Edit model" : "Add model"}
        >
          <div className="wk-settings-panel-heading">
            <div>
              <h3>{draft.id ? "Edit model" : "Add model"}</h3>
              <p className="wk-muted">
                Choose a model type, source, provider, and connection details.
              </p>
            </div>
            <Button
              type="button"
              disabled={busy}
              onClick={() => setDraft(null)}
            >
              Close
            </Button>
          </div>
          <form
            className="wk-settings-editor"
            onSubmit={(event) => void save(event)}
          >
            {!draft.id ? (
              <label>
                Model type
                <select
                  value={draft.type}
                  onChange={(event) =>
                    updateDraft("type", event.target.value as ModelType)
                  }
                >
                  {TYPES.map((type) => (
                    <option key={type} value={type}>
                      {TYPE_LABELS[type]}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <label>
              Model name
              <input
                required
                maxLength={100}
                value={draft.name}
                onChange={(event) => updateDraft("name", event.target.value)}
              />
            </label>
            <label>
              Display name
              <input
                maxLength={100}
                value={draft.displayName}
                onChange={(event) =>
                  updateDraft("displayName", event.target.value)
                }
              />
            </label>
            <label>
              Source
              <select
                value={draft.source}
                onChange={(event) =>
                  updateDraft(
                    "source",
                    event.target.value as ModelDraft["source"],
                  )
                }
              >
                <option value="remote">Remote</option>
                <option value="local">Local</option>
              </select>
            </label>
            {draft.source === "local" ? (
              <Status tone="warning">
                Local models require the Ollama service and are not downloadable
                from this React surface yet.
              </Status>
            ) : null}
            {draft.source === "remote" ? (
              <>
                <label>
                  Provider
                  <select
                    value={draft.provider}
                    disabled={loadingProviders}
                    onChange={(event) =>
                      updateDraft("provider", event.target.value)
                    }
                  >
                    <option value="generic">Generic</option>
                    {providers.map((provider) => (
                      <option key={provider.value} value={provider.value}>
                        {provider.label}
                      </option>
                    ))}
                  </select>
                </label>
                {draft.provider !== "weknoracloud" ? (
                  <label>
                    Base URL
                    <input
                      type="url"
                      required
                      value={draft.baseUrl}
                      onChange={(event) =>
                        updateDraft("baseUrl", event.target.value)
                      }
                    />
                  </label>
                ) : null}
              </>
            ) : null}
            {draft.type === "embedding" ? (
              <label>
                Dimension
                <input
                  type="number"
                  min={128}
                  max={4096}
                  value={draft.dimension}
                  onChange={(event) =>
                    updateDraft(
                      "dimension",
                      event.target.value === ""
                        ? ""
                        : Number(event.target.value),
                    )
                  }
                />
              </label>
            ) : null}
            {draft.type === "chat" || draft.type === "vllm" ? (
              <label>
                Context window
                <input
                  type="number"
                  min={1024}
                  value={draft.contextWindow}
                  onChange={(event) =>
                    updateDraft(
                      "contextWindow",
                      event.target.value === ""
                        ? ""
                        : Number(event.target.value),
                    )
                  }
                />
              </label>
            ) : null}
            {draft.type === "chat" ? (
              <label>
                <input
                  type="checkbox"
                  checked={draft.supportsVision}
                  onChange={(event) =>
                    updateDraft("supportsVision", event.target.checked)
                  }
                />{" "}
                Supports vision
              </label>
            ) : null}
            <label>
              Max concurrency
              <input
                type="number"
                min={1}
                value={draft.maxConcurrency}
                onChange={(event) =>
                  updateDraft(
                    "maxConcurrency",
                    event.target.value === "" ? "" : Number(event.target.value),
                  )
                }
              />
            </label>
            {draft.type === "chat" && draft.source === "remote" ? (
              <label>
                Thinking control
                <select
                  value={draft.thinkingControl}
                  onChange={(event) =>
                    updateDraft("thinkingControl", event.target.value)
                  }
                >
                  <option value="">Default</option>
                  <option value="none">None</option>
                  <option value="enable_thinking">Enable thinking</option>
                  <option value="thinking_type">Thinking type</option>
                  <option value="chat_template_kwargs">
                    Chat template kwargs
                  </option>
                </select>
              </label>
            ) : null}
            <fieldset>
              <legend>Custom headers</legend>
              {Object.entries(draft.customHeaders).map(([key, value]) => (
                <div className="wk-model-header-row" key={key}>
                  <input
                    value={key}
                    aria-label="Header name"
                    onChange={(event) => {
                      const next = { ...draft.customHeaders };
                      delete next[key];
                      if (event.target.value.trim())
                        next[event.target.value] = value;
                      updateDraft("customHeaders", next);
                    }}
                  />
                  <input
                    value={value}
                    aria-label="Header value"
                    onChange={(event) =>
                      updateDraft("customHeaders", {
                        ...draft.customHeaders,
                        [key]: event.target.value,
                      })
                    }
                  />
                  <Button
                    type="button"
                    onClick={() => {
                      const next = { ...draft.customHeaders };
                      delete next[key];
                      updateDraft("customHeaders", next);
                    }}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <Button
                type="button"
                onClick={() =>
                  updateDraft("customHeaders", {
                    ...draft.customHeaders,
                    [`X-Custom-${Object.keys(draft.customHeaders).length + 1}`]:
                      "",
                  })
                }
              >
                Add header
              </Button>
            </fieldset>
            <label>
              API key
              <input
                type="password"
                autoComplete="new-password"
                value={draft.apiKey}
                placeholder={
                  draft.id ? "Leave blank to keep configured key" : ""
                }
                onChange={(event) => updateDraft("apiKey", event.target.value)}
              />
            </label>
            {draft.provider === "weknoracloud" ? (
              <label>
                App secret
                <input
                  type="password"
                  autoComplete="new-password"
                  value={draft.appSecret}
                  onChange={(event) =>
                    updateDraft("appSecret", event.target.value)
                  }
                />
              </label>
            ) : null}
            <div className="wk-list-actions">
              {draft.source === "remote" ? (
                <Button
                  type="button"
                  disabled={busy || !draft.name.trim() || !draft.baseUrl.trim()}
                  onClick={() => void testConnection()}
                >
                  Test connection
                </Button>
              ) : null}
              {connectionResult ? (
                <Status tone={connectionResult.available ? "success" : "error"}>
                  {connectionResult.message}
                  {connectionResult.dimension
                    ? ` (dimension ${connectionResult.dimension})`
                    : ""}
                </Status>
              ) : null}
              <Button type="submit" loading={busy}>
                {draft.id ? "Save changes" : "Create model"}
              </Button>
              <Button
                type="button"
                disabled={busy}
                onClick={() => setDraft(null)}
              >
                Cancel
              </Button>
            </div>
          </form>
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
