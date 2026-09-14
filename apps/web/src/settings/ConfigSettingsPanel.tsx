import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { settingsConfigPatch, tenantModelIds } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

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

export function ConfigSettingsPanel({ client, section, initialValue, models, embeddingLocked, stats, onSaved }: {
  client: WeKnoraClient;
  section: ConfigSection;
  initialValue: unknown;
  models?: readonly SettingsModelOption[];
  embeddingLocked?: boolean;
  /** ChathistorySettings.vue stats block (getChatHistoryKBStats). */
  stats?: unknown;
  onSaved?: () => void;
}) {
  const api = configApi(client, section);
  const t = settingsT(readInitialLocale());
  const saveSuccessKey = section === 'retrieval' ? 'retrievalSettings.toasts.saveSuccess' : section === 'chathistory' ? 'chatHistorySettings.toasts.saveSuccess' : 'settings.parser.saveSuccess';
  const saveFailedKey = section === 'retrieval' ? 'retrievalSettings.toasts.saveFailed' : section === 'chathistory' ? 'chatHistorySettings.toasts.saveFailed' : 'settings.parser.saveFailed';
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
      setNotice(t(saveSuccessKey));
      onSaved?.();
    } catch (reason) { setError(reason instanceof Error ? reason.message : t(saveFailedKey, { message: '' })); }
    finally { setBusy(false); }
  }

  async function testParser() {
    if (section !== 'parser') return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const result = await client.settings.parser.check(settingsConfigPatch('parser', values));
      setNotice(result.connected ? t('settings.parser.checkSuccess') : t('settings.parser.checkDoneStatusUpdated'));
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('settings.parser.checkFailed')); }
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

  return (
    <>
    <Card>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void save(event)}>{section === 'retrieval' ? <><label>{t('retrievalSettings.embeddingTopKLabel')}<input type="number" min={1} max={100} value={String(values.embedding_top_k)} onChange={(event) => setValue('embedding_top_k', event.target.value)} /></label><label>{t('retrievalSettings.vectorThresholdLabel')}<input type="number" min={0} max={1} step={0.05} value={String(values.vector_threshold)} onChange={(event) => setValue('vector_threshold', event.target.value)} /></label><label>{t('retrievalSettings.keywordThresholdLabel')}<input type="number" min={0} max={1} step={0.05} value={String(values.keyword_threshold)} onChange={(event) => setValue('keyword_threshold', event.target.value)} /></label><label>{t('retrievalSettings.rerankTopKLabel')}<input type="number" min={1} max={100} value={String(values.rerank_top_k)} onChange={(event) => setValue('rerank_top_k', event.target.value)} /></label><label>{t('retrievalSettings.rerankThresholdLabel')}<input type="number" min={-10} max={10} step={0.1} value={String(values.rerank_threshold)} onChange={(event) => setValue('rerank_threshold', event.target.value)} /></label><label>{t('retrievalSettings.rerankModelLabel')}{modelOptions.length > 0 ? modelSelect('rerank_model_id', false) : <input value={String(values.rerank_model_id)} onChange={(event) => setValue('rerank_model_id', event.target.value)} />}</label></> : section === 'chathistory' ? <><div className="setting-row"><div className="setting-info"><label>{t('chatHistorySettings.enableLabel')}</label><p className="desc">{t('chatHistorySettings.enableDescription')}</p></div><div className="setting-control"><label className="relative inline-flex shrink-0 cursor-pointer items-center"><input type="checkbox" className="peer absolute m-0 h-[18px] w-[34px] cursor-pointer opacity-0" checked={values.enabled === true} onChange={(event) => setValue('enabled', event.target.checked)} /><span aria-hidden="true" className="relative inline-block h-[22px] w-[42px] rounded-full bg-[rgb(120_135_155_/_0.35)] transition-[background] duration-150 ease-[ease] after:absolute after:left-[2px] after:top-[2px] after:h-[18px] after:w-[18px] after:rounded-full after:bg-white after:content-[''] after:transition-[translate] after:duration-150 after:ease-[ease] peer-checked:bg-accent-strong peer-checked:after:translate-x-[20px] peer-disabled:opacity-50" /></label></div></div><div className="setting-row"><div className="setting-info"><label>{t('chatHistorySettings.embeddingModelLabel')}</label><p className="desc">{t('chatHistorySettings.embeddingModelDescription')}</p></div><div className="setting-control setting-control--model">{modelOptions.length > 0 ? modelSelect('embedding_model_id', embeddingLocked === true || values.enabled !== true) : <input value={String(values.embedding_model_id)} disabled={embeddingLocked === true || values.enabled !== true} onChange={(event) => setValue('embedding_model_id', event.target.value)} />}</div>{embeddingLocked === true ? <p className="desc warning-text text-[#b26a08]" data-testid="embedding-locked-note">{t('chatHistorySettings.embeddingModelLocked')}</p> : null}</div></> : <><label>{t('settings.parser.selfHostedEndpoint')}{/* TODO(migration): MinerU endpoint label has no settings.* key */}<input type="url" value={String(values.mineru_endpoint)} placeholder="https://parser.example" onChange={(event) => setValue('mineru_endpoint', event.target.value)} /></label><label>MinerU API key{/* TODO(migration): no settings.* key */}<input type="password" autoComplete="new-password" placeholder="Leave blank to keep the configured key" value={String(values.mineru_api_key)} onChange={(event) => setValue('mineru_api_key', event.target.value)} /></label><p className="wk-muted">The current API key is never returned to or prefilled in this form.</p></>}<div className="wk-list-actions"><Button type="submit" loading={busy} disabled={!dirty} data-testid="config-save">{t('common.save')}</Button>{section === 'parser' ? <Button type="button" disabled={busy} onClick={() => void testParser()}>{t('settings.parser.testConnection')}</Button> : null}</div></form></Card>
    {section === 'chathistory' ? (
      <div className="stats-section mt-5" data-testid="chat-history-stats">
        <h3 className="stats-title m-0 mb-3 text-[15px] font-semibold">{t('chatHistorySettings.statsTitle')}</h3>
        {stats !== null && stats !== undefined && typeof stats === 'object' && (stats as Record<string, unknown>).enabled === true && (stats as Record<string, unknown>).knowledge_base_id ? (
          <div className="stats-grid grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3">
            <div className="stat-card rounded-[10px] border border-[rgba(120,135,155,0.3)] bg-white p-4 text-center">
              <div className="stat-value text-[22px] font-semibold [font-variant-numeric:tabular-nums]">{String((stats as Record<string, unknown>).indexed_message_count ?? 0)}</div>
              <div className="stat-label mt-1 text-xs text-[#5c6b83]">{t('chatHistorySettings.statsIndexedMessages')}</div>
            </div>
          </div>
        ) : (
          <div className="stats-empty rounded-[10px] border border-dashed border-[rgba(120,135,155,0.35)] bg-[rgba(127,142,166,0.06)] px-4 py-7 text-center">
            <p className="stats-empty-title m-0 mb-1.5 font-semibold">{t('chatHistorySettings.statsNotConfigured')}</p>
            <p className="stats-empty-desc m-0 text-[13px] text-[#5c6b83]">{t('chatHistorySettings.statsNotConfiguredDesc')}</p>
          </div>
        )}
      </div>
    ) : null}
    </>
  );
}
