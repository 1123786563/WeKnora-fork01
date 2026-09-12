import { useMemo, useState } from "react";
import * as React from "react";
import type { ModelConfiguration, WeKnoraClient } from "@weknora/api-client";
import { Button, Status } from "@weknora/ui";
import { modelType } from "./model-settings.ts";

type Props = {
  client: WeKnoraClient;
  models: readonly ModelConfiguration[];
  onClose: () => void;
};

function modelLabel(model: ModelConfiguration): string {
  const displayName = (model as Record<string, unknown>).display_name;
  return typeof displayName === "string" && displayName.trim()
    ? displayName
    : model.name;
}

export function ModelDebugPanel({ client, models, onClose }: Props) {
  const [selectedId, setSelectedId] = useState(
    () =>
      models.find((item) =>
        ["chat", "embedding", "rerank", "vllm", "asr"].includes(
          modelType(item),
        ),
      )?.id ?? "",
  );
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
  const [result, setResult] = useState<Awaited<
    ReturnType<WeKnoraClient["configuration"]["models"]["debug"]>
  > | null>(null);
  const selected = models.find((item) => item.id === selectedId);
  const selectedType = selected ? modelType(selected) : "chat";
  const isRerank = selectedType === "rerank";
  const needsFile = selectedType === "vllm" || selectedType === "asr";
  const canRun =
    Boolean(
      selected &&
        (needsFile ? file : input.trim()) &&
        (!isRerank || documents.trim()),
    ) && !busy;
  const parsedDocuments = useMemo(
    () =>
      documents
        .split("\n")
        .map((item) => item.trim())
        .filter(Boolean),
    [documents],
  );

  async function run() {
    if (!selected || !canRun) return;
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      setResult(
        await client.configuration.models.debug(selected.id, {
          input: input || undefined,
          documents: isRerank ? parsedDocuments : undefined,
          file,
          options:
            selectedType === "chat"
              ? {
                  systemPrompt: systemPrompt || undefined,
                  temperature,
                  topP,
                  maxTokens,
                  thinking,
                }
              : undefined,
        }),
      );
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Unable to debug model",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="wk-model-debug"
      role="dialog"
      aria-modal="true"
      aria-label="Debug model"
    >
      <div className="wk-settings-panel-heading">
        <div>
          <h3>Debug model</h3>
          <p className="wk-muted">
            Run a request through the selected configured model and inspect the
            response.
          </p>
        </div>
        <Button type="button" onClick={onClose}>
          Close
        </Button>
      </div>
      <div className="wk-settings-editor">
        <label>
          Model
          <select
            value={selectedId}
            onChange={(event) => {
              setSelectedId(event.target.value);
              setResult(null);
            }}
          >
            {models.map((model) => (
              <option key={model.id} value={model.id}>
                {modelLabel(model)} · {modelType(model)}
              </option>
            ))}
          </select>
        </label>
        {!needsFile ? (
          <label>
            {isRerank ? "Query" : "Input"}
            <textarea
              rows={4}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              placeholder={
                isRerank ? "Query to rank" : "Prompt or text to send"
              }
            />
          </label>
        ) : null}
        {isRerank ? (
          <label>
            Documents
            <textarea
              rows={4}
              value={documents}
              onChange={(event) => setDocuments(event.target.value)}
              placeholder="One document per line"
            />
          </label>
        ) : null}
        {needsFile ? (
          <label>
            File
            <input
              type="file"
              accept={selectedType === "vllm" ? "image/*" : "audio/*"}
              onChange={(event) => setFile(event.target.files?.[0])}
            />
            {file ? <span className="wk-muted">{file.name}</span> : null}
          </label>
        ) : null}
        {selectedType === "chat" ? (
          <fieldset>
            <legend>Chat parameters</legend>
            <label>
              Temperature
              <input
                type="number"
                min={0}
                max={2}
                step={0.1}
                value={temperature}
                onChange={(event) => setTemperature(Number(event.target.value))}
              />
            </label>
            <label>
              Top P
              <input
                type="number"
                min={0.01}
                max={1}
                step={0.1}
                value={topP}
                onChange={(event) => setTopP(Number(event.target.value))}
              />
            </label>
            <label>
              Max tokens
              <input
                type="number"
                min={1}
                max={8192}
                value={maxTokens}
                onChange={(event) => setMaxTokens(Number(event.target.value))}
              />
            </label>
            <label>
              System prompt
              <textarea
                rows={2}
                value={systemPrompt}
                onChange={(event) => setSystemPrompt(event.target.value)}
              />
            </label>
            <label className="wk-checkbox">
              <input
                type="checkbox"
                checked={thinking}
                onChange={(event) => setThinking(event.target.checked)}
              />{" "}
              Enable thinking
            </label>
          </fieldset>
        ) : null}
        {error ? <Status tone="error">{error}</Status> : null}
        <Button
          type="button"
          disabled={!canRun}
          loading={busy}
          onClick={() => void run()}
        >
          Run debug
        </Button>
        {result ? (
          <section
            className={
              result.ok
                ? "wk-model-debug-result is-ok"
                : "wk-model-debug-result is-error"
            }
          >
            <strong>{result.ok ? "Succeeded" : "Failed"}</strong>
            <span>{result.elapsedMs} ms</span>
            {result.error ? <Status tone="error">{result.error}</Status> : null}
            <details open>
              <summary>Response</summary>
              <pre>{JSON.stringify(result.rawResponse, null, 2)}</pre>
            </details>
            <details>
              <summary>Request preview</summary>
              <pre>{JSON.stringify(result.request, null, 2)}</pre>
            </details>
            <details>
              <summary>Observations</summary>
              <pre>{JSON.stringify(result.observations, null, 2)}</pre>
            </details>
          </section>
        ) : null}
      </div>
    </div>
  );
}
