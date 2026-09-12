import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { settingsConfigPatch, tenantModelIds } from './surface.ts';

type ConfigSection = 'retrieval' | 'chathistory' | 'parser';
type ConfigValues = Record<string, unknown>;

export interface SettingsModelOption { readonly id: string; readonly name?: string }

function object(value: unknown): ConfigValues { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as ConfigValues : {}; }
function text(value: unknown): string { return typeof value === 'string' ? value : ''; }
function number(value: unknown, fallback: number): number { return typeof value === 'number' && Number.isFinite(value) ? value : fallback; }

function initialValues(section: ConfigSection, value: unknown): ConfigValues {
  const row = object(value);
  if (section === 'retrieval') return {
    embedding_top_k: number(row.embedding_top_k, 50) || 50,
    vector_threshold: number(row.vector_threshold, 0.15),
    keyword_threshold: number(row.keyword_threshold, 0.3),
    rerank_top_k: number(row.rerank_top_k, 10) || 10,
    rerank_threshold: number(row.rerank_threshold, 0.2),
    rerank_model_id: text(row.rerank_model_id),
  };
  if (section === 'chathistory') return { enabled: row.enabled === true, embedding_model_id: text(row.embedding_model_id) };
  return { mineru_endpoint: text(row.mineru_endpoint), mineru_api_key: '' };
}

function configApi(client: WeKnoraClient, section: ConfigSection) {
  if (section === 'retrieval') return client.settings.retrieval;
  if (section === 'chathistory') return client.settings.chatHistory.config;
  return client.settings.parser.config;
}

function isDirty(section: ConfigSection, saved: ConfigValues, current: ConfigValues): boolean {
  const keys = section === 'parser' ? ['mineru_endpoint', 'mineru_api_key'] : Object.keys(saved);
  return keys.some((key) => String(saved[key]) !== String(current[key]));
}

export function ConfigSettingsPanel({ client, section, initialValue, models, embeddingLocked, onSaved }: {
  client: WeKnoraClient;
  section: ConfigSection;
  initialValue: unknown;
  models?: readonly SettingsModelOption[];
  embeddingLocked?: boolean;
  onSaved?: () => void;
}) {
  const api = configApi(client, section);
  const savedValues = useMemo(() => initialValues(section, initialValue), [section, initialValue]);
  const [values, setValues] = useState<ConfigValues>(savedValues);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setValues(initialValues(section, initialValue)); }, [section, initialValue]);

  const modelOptions: readonly SettingsModelOption[] = models ?? [];
  const allowedModelIds = tenantModelIds(modelOptions);
  const dirty = isDirty(section, savedValues, values);

  function setValue(key: string, value: unknown) { setValues((current) => ({ ...current, [key]: value })); }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const patch = settingsConfigPatch(section, values, allowedModelIds.length > 0 ? { allowedModelIds } : {});
      const saved = await api.update(patch);
      setValues((current) => ({ ...initialValues(section, saved), ...(section === 'parser' ? { mineru_api_key: '' } : {}) }));
      setNotice('Settings saved by the server.');
      onSaved?.();
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to save settings; the current form values were kept.'); }
    finally { setBusy(false); }
  }

  async function testParser() {
    if (section !== 'parser') return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const result = await client.settings.parser.check(settingsConfigPatch('parser', values));
      setNotice(result.connected ? 'Parser engine check succeeded.' : 'Parser engine check completed; no document reader is connected.');
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Parser engine check failed.'); }
    finally { setBusy(false); }
  }

  const modelSelect = (key: 'rerank_model_id' | 'embedding_model_id', disabled: boolean) => <select
    data-testid={key}
    value={String(values[key] ?? '')}
    disabled={disabled}
    onChange={(event) => setValue(key, event.target.value)}
  >
    <option value="">—</option>
    {modelOptions.map((model) => <option key={model.id} value={model.id}>{model.name ? model.name + ' (' + model.id + ')' : model.id}</option>)}
  </select>;

  const title = section === 'retrieval' ? 'Retrieval parameters' : section === 'chathistory' ? 'Chat-history indexing' : 'Parser engine configuration';
  return <Card><h3>{title}</h3>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void save(event)}>{section === 'retrieval' ? <><label>Embedding top K<input type="number" min={1} max={100} value={String(values.embedding_top_k)} onChange={(event) => setValue('embedding_top_k', event.target.value)} /></label><label>Vector threshold<input type="number" min={0} max={1} step={0.05} value={String(values.vector_threshold)} onChange={(event) => setValue('vector_threshold', event.target.value)} /></label><label>Keyword threshold<input type="number" min={0} max={1} step={0.05} value={String(values.keyword_threshold)} onChange={(event) => setValue('keyword_threshold', event.target.value)} /></label><label>Rerank top K<input type="number" min={1} max={100} value={String(values.rerank_top_k)} onChange={(event) => setValue('rerank_top_k', event.target.value)} /></label><label>Rerank threshold<input type="number" min={-10} max={10} step={0.1} value={String(values.rerank_threshold)} onChange={(event) => setValue('rerank_threshold', event.target.value)} /></label><label>Rerank model{modelOptions.length > 0 ? modelSelect('rerank_model_id', false) : <input value={String(values.rerank_model_id)} onChange={(event) => setValue('rerank_model_id', event.target.value)} />}</label></> : section === 'chathistory' ? <><label><input type="checkbox" checked={values.enabled === true} onChange={(event) => setValue('enabled', event.target.checked)} /> Enable chat-history indexing</label><label>Embedding model{modelOptions.length > 0 ? modelSelect('embedding_model_id', embeddingLocked === true || values.enabled !== true) : <input value={String(values.embedding_model_id)} disabled={embeddingLocked === true || values.enabled !== true} onChange={(event) => setValue('embedding_model_id', event.target.value)} />}</label>{embeddingLocked === true ? <p className="wk-muted" data-testid="embedding-locked-note">Indexed chat messages already exist; the embedding model is locked until the index is cleared.</p> : null}<p className="wk-muted">The server owns the generated knowledge-base identity and indexed-message statistics.</p></> : <><label>MinerU endpoint<input type="url" value={String(values.mineru_endpoint)} placeholder="https://parser.example" onChange={(event) => setValue('mineru_endpoint', event.target.value)} /></label><label>MinerU API key<input type="password" autoComplete="new-password" placeholder="Leave blank to keep the configured key" value={String(values.mineru_api_key)} onChange={(event) => setValue('mineru_api_key', event.target.value)} /></label><p className="wk-muted">The current API key is never returned to or prefilled in this form.</p></>}<div className="wk-list-actions"><Button type="submit" loading={busy} disabled={!dirty} data-testid="config-save">Save settings</Button>{section === 'parser' ? <Button type="button" disabled={busy} onClick={() => void testParser()}>Test parser engines</Button> : null}</div></form></Card>;
}
