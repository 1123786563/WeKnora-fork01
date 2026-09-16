import { useEffect, useMemo, useRef, useState } from "react";
import * as React from "react";
import type { ModelConfiguration, WeKnoraClient } from "@weknora/api-client";
import { Button, NumberInput, Status, Switch, Textarea } from "@weknora/ui";
import { useAppLocale } from "../i18n.ts";
import {
  createModelTranslator,
  formatContextWindow,
  modelHasContextWindow,
  modelSupportsThinking,
  modelType,
  type ModelType,
} from "./model-settings.ts";
import { ModelOptionSelect } from "./ModelOptionSelect.tsx";

type Props = {
  client: WeKnoraClient;
  models: readonly ModelConfiguration[];
  onClose: () => void;
};

const TYPE_ORDER: ModelType[] = ["chat", "embedding", "rerank", "vllm", "asr"];
const OBSERVATION_LABEL_KEYS: Record<string, string> = {
  dimension: "modelSettings.debug.metrics.dimension",
  result_count: "modelSettings.debug.metrics.resultCount",
  answer_characters: "modelSettings.debug.metrics.answerChars",
  reasoning_characters: "modelSettings.debug.metrics.reasoningChars",
  reasoning_returned: "modelSettings.debug.metrics.reasoningReturned",
  text_characters: "modelSettings.debug.metrics.textChars",
  segment_count: "modelSettings.debug.metrics.segmentCount",
};
type DebugRun = {
  id: number;
  label: string;
  result: Awaited<ReturnType<WeKnoraClient["configuration"]["models"]["debug"]>>;
};

function modelLabel(model: ModelConfiguration): string {
  const displayName = (model as Record<string, unknown>).display_name;
  return typeof displayName === "string" && displayName.trim()
    ? displayName
    : model.name;
}
function vendorLabel(t: (key: string) => string, model: ModelConfiguration): string {
  if (model.source === "local") return "Ollama";
  const parameters = (model.parameters ?? {}) as Record<string, unknown>;
  const provider = typeof parameters.provider === "string" ? parameters.provider : "";
  if (provider === "generic") return t("modelSettings.source.custom");
  if (provider) {
    const key = `model.editor.providers.${provider}.label`;
    const viaT = t(key);
    return viaT !== key ? viaT : provider;
  }
  return modelType(model) === "vllm" || modelType(model) === "asr"
    ? t("modelSettings.source.openaiCompatible")
    : t("modelSettings.source.remote");
}
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function ModelDebugPanel({ client, models, onClose }: Props) {
  const locale = useAppLocale();
  const t = useMemo(() => createModelTranslator(locale), [locale]);
  const [selectedType, setSelectedType] = useState<ModelType>(() => {
    const available = TYPE_ORDER.filter((type) => models.some((item) => modelType(item) === type));
    return available[0] ?? "chat";
  });
  const [selectedId, setSelectedId] = useState(() => {
    const available = models.filter((item) => modelType(item) === selectedType);
    return available[0]?.id ?? "";
  });
  const [input, setInput] = useState("");
  const [documents, setDocuments] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [temperature, setTemperature] = useState(0.7);
  const [topP, setTopP] = useState(1);
  const [maxTokens, setMaxTokens] = useState(1024);
  const [thinking, setThinking] = useState(false);
  const [file, setFile] = useState<File | undefined>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<DebugRun["result"] | null>(null);
  const [history, setHistory] = useState<DebugRun[]>([]);
  const runSequence = useRef(0);
  const availableTypes = useMemo(
    () => TYPE_ORDER.filter((type) => models.some((item) => modelType(item) === type)),
    [models],
  );
  const filteredModels = useMemo(
    () => models.filter((item) => modelType(item) === selectedType),
    [models, selectedType],
  );
  const selected = filteredModels.find((item) => item.id === selectedId) ?? filteredModels[0];
  const selectedTypeResolved = selected ? modelType(selected) : selectedType;
  const isChat = selectedTypeResolved === "chat";
  const isRerank = selectedTypeResolved === "rerank";
  const isEmbedding = selectedTypeResolved === "embedding";
  const isVllm = selectedTypeResolved === "vllm";
  const needsFile = selectedTypeResolved === "vllm" || selectedTypeResolved === "asr";
  const supportsThinking = selected ? modelSupportsThinking(selected as unknown as Parameters<typeof modelSupportsThinking>[0]) : false;
  const parsedDocuments = useMemo(
    () =>
      documents
        .split("\n")
        .map((item) => item.trim())
        .filter(Boolean),
    [documents],
  );
  /* Run gating — ModelDebugDrawer.vue canRun: VLLM/ASR need a file, ASR needs
     nothing else, rerank needs a query plus documents, everything else a query. */
  const canRun =
    Boolean(selected) &&
    (needsFile ? Boolean(file) : true) &&
    (selectedTypeResolved === "asr" ? true : Boolean(input.trim())) &&
    (!isRerank || parsedDocuments.length > 0) &&
    !busy;

  function resetResult() {
    setResult(null);
    setHistory([]);
  }
  useEffect(() => {
    if (availableTypes.length === 0) {
      setSelectedId("");
      return;
    }

    const nextType = availableTypes.includes(selectedType) ? selectedType : availableTypes[0];
    if (nextType !== selectedType) setSelectedType(nextType);

    const nextModels = models.filter((item) => modelType(item) === nextType);
    if (!nextModels.some((item) => item.id === selectedId)) {
      setSelectedId(nextModels[0]?.id ?? "");
    }
  }, [availableTypes, models, selectedId, selectedType]);

  function selectType(type: ModelType) {
    if (selectedType === type) return;
    setSelectedType(type);
    const first = models.find((item) => modelType(item) === type);
    setSelectedId(first?.id ?? "");
    setInput("");
    setDocuments("");
    setFile(undefined);
    resetResult();
  }
  function selectModel(id: string) {
    setSelectedId(id);
    const next = models.find((item) => item.id === id);
    if (!next || !modelSupportsThinking(next as unknown as Parameters<typeof modelSupportsThinking>[0])) setThinking(false);
    resetResult();
  }

  async function run() {
    if (!selected || !canRun) return;
    setBusy(true);
    setError(null);
    try {
      const thinkingValue = supportsThinking ? thinking : false;
      const nextResult = await client.configuration.models.debug(selected.id, {
        input: input.trim() || undefined,
        documents: isRerank ? parsedDocuments : undefined,
        file,
        options:
          isChat
            ? {
                systemPrompt: systemPrompt.trim() || undefined,
                temperature,
                topP,
                maxTokens,
                thinking: thinkingValue,
              }
            : undefined,
      });
      runSequence.current += 1;
      const label = supportsThinking
        ? t(thinkingValue ? "modelSettings.debug.thinkOn" : "modelSettings.debug.thinkOff")
        : t("modelSettings.debug.runLabel", { n: runSequence.current });
      setResult(nextResult);
      setHistory((current) => [{ id: runSequence.current, label, result: nextResult }, ...current].slice(0, 6));
    } catch (cause) {
      setError(
        cause instanceof Error && cause.message ? cause.message : t("modelSettings.debug.requestFailed"),
      );
    } finally {
      setBusy(false);
    }
  }

  async function copyResult() {
    if (!result) return;
    try {
      await navigator.clipboard?.writeText(JSON.stringify(result, null, 2));
      setError(null);
    } catch {
      setError(t("common.error"));
    }
  }

  const inputLabel = isEmbedding
    ? t("modelSettings.debug.embeddingInput")
    : isVllm
      ? t("modelSettings.debug.vlmPrompt")
      : t("modelSettings.debug.query");
  const inputPlaceholder = isEmbedding
    ? t("modelSettings.debug.embeddingPlaceholder")
    : isVllm
      ? t("modelSettings.debug.vlmPromptPlaceholder")
      : t("modelSettings.debug.queryPlaceholder");
  const metrics = result
    ? Object.keys(OBSERVATION_LABEL_KEYS)
        .filter((key) => result.observations[key] !== undefined && result.observations[key] !== null)
        .map((key) => ({
          key,
          label: t(OBSERVATION_LABEL_KEYS[key]!),
          value: typeof result.observations[key] === "boolean"
            ? t(result.observations[key] === true ? "common.yes" : "common.no")
            : String(result.observations[key]),
        }))
    : [];

  return (
    <div
      className="mt-4 max-w-[720px] rounded-[8px] border border-[#cbd5e1] bg-white p-4 shadow-[0_10px_30px_rgba(23,32,51,.12)]"
      role="dialog"
      aria-modal="true"
      aria-label={t("modelSettings.debug.title")}
    >
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div>
          <h3>{t("modelSettings.debug.title")}</h3>
          <p className="wk-muted text-muted m-0">{t("modelSettings.debug.description")}</p>
        </div>
        <Button type="button" onClick={onClose}>
          {t("common.close")}
        </Button>
      </div>
      <div className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem]">
        <div className="form-item">
          <h4>{t("modelSettings.debug.groupModel")}</h4>
          {availableTypes.length > 1 ? (
            <div className="wk-model-type-options" role="radiogroup" aria-label={t("modelSettings.debug.modelType")}>
              {availableTypes.map((type) => (
                <button
                  key={type}
                  type="button"
                  role="radio"
                  aria-checked={selectedTypeResolved === type}
                  className={selectedTypeResolved === type ? "is-active" : ""}
                  onClick={() => selectType(type)}
                >
                  {t(`modelSettings.typeShort.${type}`)}
                </button>
              ))}
            </div>
          ) : null}
          <label>
            {t("modelSettings.debug.model")}
            <ModelOptionSelect
              value={selected?.id ?? ""}
              disabled={filteredModels.length === 0}
              options={filteredModels.map((model) => ({
                value: model.id,
                label: `${modelLabel(model)}${vendorLabel(t, model) ? ` · ${vendorLabel(t, model)}` : ""}${modelHasContextWindow(typeof model.type === "string" ? model.type : "") ? ` · ${formatContextWindow(((model.parameters ?? {}) as Record<string, unknown>).context_window as number | undefined)}` : ""}`,
              }))}
              onChange={selectModel}
            />
          </label>
          {filteredModels.length === 0 ? (
            <p className="wk-muted text-muted">{t("modelSettings.debug.noModelsForType")}</p>
          ) : null}
        </div>
        {selected ? (
          <div className="form-item">
            <h4>{t("modelSettings.debug.groupInput")}</h4>
            {selectedTypeResolved !== "asr" ? (
              <label>
                {inputLabel}
                <Textarea
                  rows={4}
                  value={input}
                  onChange={(event) => setInput(event.target.value)}
                  placeholder={inputPlaceholder}
                />
              </label>
            ) : null}
            {isRerank ? (
              <label>
                {t("modelSettings.debug.documents")}
                <Textarea
                  rows={4}
                  value={documents}
                  onChange={(event) => setDocuments(event.target.value)}
                  placeholder={t("modelSettings.debug.documentsPlaceholder")}
                />
                <span className="wk-muted text-muted">{t("modelSettings.debug.documentsHint")}</span>
              </label>
            ) : null}
            {needsFile ? (
              <label>
                {t(isVllm ? "modelSettings.debug.imageFile" : "modelSettings.debug.audioFile")}
                <input
                  type="file"
                  accept={isVllm ? "image/*" : "audio/*"}
                  onChange={(event) => {
                    setFile(event.target.files?.[0]);
                    resetResult();
                  }}
                />
                {file ? <span className="wk-muted text-muted">{file.name} · {formatBytes(file.size)}</span> : null}
              </label>
            ) : null}
          </div>
        ) : null}
        {selected && isChat ? (
          <fieldset className="grid gap-[.7rem] rounded-[6px] border border-[#edf0f5] p-[.8rem]">
            <legend>{t("modelSettings.debug.parameters")}</legend>
            <div className="form-item"><label>Temperature</label><NumberInput min={0} max={2} step={0.1} value={temperature} onValueChange={(value) => setTemperature(Number(value))} /></div>
            <div className="form-item"><label>Top P</label><NumberInput min={0.01} max={1} step={0.1} value={topP} onValueChange={(value) => setTopP(Number(value))} /></div>
            <div className="form-item"><label>Max Tokens</label><NumberInput min={1} max={8192} step={128} value={maxTokens} onValueChange={(value) => setMaxTokens(Number(value))} /></div>
            <label>
              {t("modelSettings.debug.systemPrompt")}
              <Textarea
                rows={2}
                value={systemPrompt}
                onChange={(event) => setSystemPrompt(event.target.value)}
                placeholder={t("modelSettings.debug.systemPromptPlaceholder")}
              />
            </label>
            {supportsThinking ? (
              <div className="mt-1 flex min-h-[22px] cursor-pointer flex-wrap items-center gap-x-2">
                <Switch checked={thinking} onCheckedChange={setThinking} aria-label={t("modelSettings.debug.thinking")} />
                <span className="text-[13px] font-medium text-[rgba(0,0,0,0.9)]">{t("modelSettings.debug.thinking")}</span>
                <span className="ml-11 mt-[2px] basis-full text-xs leading-[1.5] text-[#8a8a8a]">{t("modelSettings.debug.thinkingDesc")}</span>
              </div>
            ) : null}
          </fieldset>
        ) : null}
        {error ? <Status tone="error">{error}</Status> : null}
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <Button type="button" disabled={!canRun} loading={busy} onClick={() => void run()}>
            {t("modelSettings.debug.run")}
          </Button>
          {result ? (
            <Button type="button" onClick={() => void copyResult()}>
              {t("modelSettings.debug.copyResult")}
            </Button>
          ) : null}
        </div>
        {history.length > 1 ? (
          <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]" role="list" aria-label={t("modelSettings.debug.history")}>
            {history.map((run) => (
              <Button
                key={run.id}
                type="button"
                onClick={() => setResult(run.result)}
              >
                {run.label} · {run.result.elapsedMs} ms
              </Button>
            ))}
          </div>
        ) : null}
        {result ? (
          <section
            className={
              result.ok
                ? "mt-4 grid gap-2 rounded-[6px] border border-[#86efac] p-[.8rem]"
                : "mt-4 grid gap-2 rounded-[6px] border border-[#fca5a5] p-[.8rem]"
            }
          >
            <strong>{t(result.ok ? "modelSettings.debug.success" : "modelSettings.debug.failed")}</strong>
            <span>{result.elapsedMs} ms</span>
            {metrics.length > 0 ? (
              <p className="wk-muted text-muted">
                {metrics.map((metric) => `${metric.label}: ${metric.value}`).join(" · ")}
              </p>
            ) : null}
            {result.error ? <Status tone="error">{result.error}</Status> : null}
            <details open>
              <summary>{t("modelSettings.debug.rawResponse")}</summary>
              <pre className="max-h-[18rem] overflow-auto whitespace-pre-wrap bg-[#f7f9fc] p-[.7rem]">{JSON.stringify(result.rawResponse, null, 2)}</pre>
            </details>
            <details>
              <summary>{t("modelSettings.debug.requestPreview")}</summary>
              <pre className="max-h-[18rem] overflow-auto whitespace-pre-wrap bg-[#f7f9fc] p-[.7rem]">{JSON.stringify(result.request, null, 2)}</pre>
            </details>
          </section>
        ) : null}
      </div>
    </div>
  );
}
