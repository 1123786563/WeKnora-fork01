import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Card, NumberInput, Status, Switch } from '@weknora/ui';
import { ModelOptionSelect } from './ModelOptionSelect.tsx';
import { memoryWorkspacePatch } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type MemoryRow = Record<string, unknown>;

function rowId(row: MemoryRow): string { return typeof row.id === 'string' || typeof row.id === 'number' ? String(row.id) : ''; }
function rowContent(row: MemoryRow): string { return typeof row.content === 'string' ? row.content : ''; }

export function MemoryWorkspacePanel({ client, initialConfig, canEdit = true }: { client: WeKnoraClient; initialConfig: unknown; canEdit?: boolean }) {
  const t = settingsT(readInitialLocale());
  const row = initialConfig !== null && typeof initialConfig === 'object' && !Array.isArray(initialConfig) ? initialConfig as MemoryRow : {};
  const [enabled, setEnabled] = useState(row.enabled === true);
  const [writeMode, setWriteMode] = useState(row.write_mode === 'auto' ? 'auto' : 'explicit_only');
  const [maxItems, setMaxItems] = useState(typeof row.max_items === 'number' ? row.max_items : 200);
  const [extractModelId, setExtractModelId] = useState(typeof row.extract_model_id === 'string' ? row.extract_model_id : '');
  const [extractDelaySeconds, setExtractDelaySeconds] = useState(typeof row.extract_delay_seconds === 'number' ? row.extract_delay_seconds : 90);
  const [extractMinIntervalSeconds, setExtractMinIntervalSeconds] = useState(typeof row.extract_min_interval_seconds === 'number' ? row.extract_min_interval_seconds : 300);
  const [extractInstructions, setExtractInstructions] = useState(typeof row.extract_instructions === 'string' ? row.extract_instructions : '');
  const [interestThreshold, setInterestThreshold] = useState(typeof row.interest_threshold === 'number' ? row.interest_threshold : 3);
  const [embeddingModelId, setEmbeddingModelId] = useState(typeof row.embedding_model_id === 'string' ? row.embedding_model_id : '');
  const [models, setModels] = useState<Array<{ id: string; name: string; type?: string }>>([]);
  const [vectorRecall, setVectorRecall] = useState(row.vector_recall !== false);
  const [retrievalConditioning, setRetrievalConditioning] = useState(row.retrieval_conditioning !== false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const saveTimerRef = useRef<number | null>(null);
  const pendingSaveRef = useRef<Parameters<typeof save>[0]>({});

  useEffect(() => {
    setEnabled(row.enabled === true); setWriteMode(row.write_mode === 'auto' ? 'auto' : 'explicit_only'); setMaxItems(typeof row.max_items === 'number' ? row.max_items : 200); setExtractModelId(typeof row.extract_model_id === 'string' ? row.extract_model_id : ''); setExtractDelaySeconds(typeof row.extract_delay_seconds === 'number' ? row.extract_delay_seconds : 90); setExtractMinIntervalSeconds(typeof row.extract_min_interval_seconds === 'number' ? row.extract_min_interval_seconds : 300); setExtractInstructions(typeof row.extract_instructions === 'string' ? row.extract_instructions : ''); setInterestThreshold(typeof row.interest_threshold === 'number' ? row.interest_threshold : 3); setEmbeddingModelId(typeof row.embedding_model_id === 'string' ? row.embedding_model_id : ''); setVectorRecall(row.vector_recall !== false); setRetrievalConditioning(row.retrieval_conditioning !== false);
  }, [initialConfig]);

  useEffect(() => () => {
    if (saveTimerRef.current !== null) window.clearTimeout(saveTimerRef.current);
  }, []);

  useEffect(() => { void client.configuration.models.list().then((items) => setModels(items.map((item) => ({ id: item.id, name: item.name, type: typeof item.type === 'string' ? item.type : undefined })))).catch(() => setModels([])); }, [client]);

  async function save(next: { enabled?: boolean; writeMode?: string; maxItems?: number; extractModelId?: string; extractDelaySeconds?: number; extractMinIntervalSeconds?: number; extractInstructions?: string; interestThreshold?: number; embeddingModelId?: string; vectorRecall?: boolean; retrievalConditioning?: boolean }) {
    if (!canEdit) return;
    const values = { enabled: next.enabled ?? enabled, writeMode: next.writeMode ?? writeMode, maxItems: next.maxItems ?? maxItems, extractModelId: next.extractModelId ?? extractModelId, extractDelaySeconds: next.extractDelaySeconds ?? extractDelaySeconds, extractMinIntervalSeconds: next.extractMinIntervalSeconds ?? extractMinIntervalSeconds, extractInstructions: next.extractInstructions ?? extractInstructions, interestThreshold: next.interestThreshold ?? interestThreshold, embeddingModelId: next.embeddingModelId ?? embeddingModelId, vectorRecall: next.vectorRecall ?? vectorRecall, retrievalConditioning: next.retrievalConditioning ?? retrievalConditioning };
    setBusy(true); setError(null); setNotice(null);
    try { await client.settings.memory.workspace.update(memoryWorkspacePatch(values.enabled, values.writeMode, values.maxItems, values.vectorRecall, values.retrievalConditioning, values)); setEnabled(values.enabled); setWriteMode(values.writeMode); setMaxItems(values.maxItems); setExtractModelId(values.extractModelId); setExtractDelaySeconds(values.extractDelaySeconds); setExtractMinIntervalSeconds(values.extractMinIntervalSeconds); setExtractInstructions(values.extractInstructions); setInterestThreshold(values.interestThreshold); setEmbeddingModelId(values.embeddingModelId); setVectorRecall(values.vectorRecall); setRetrievalConditioning(values.retrievalConditioning); setNotice(t('memoryWorkspaceSettings.toasts.saveSuccess')); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to save workspace memory settings.'); }
    finally { setBusy(false); }
  }

  function debouncedSave(next: Parameters<typeof save>[0] = {}) {
    if (!canEdit) return;
    pendingSaveRef.current = { ...pendingSaveRef.current, ...next };
    if (saveTimerRef.current !== null) window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = window.setTimeout(() => {
      const pending = pendingSaveRef.current;
      pendingSaveRef.current = {};
      saveTimerRef.current = null;
      void save(pending);
    }, 500);
  }

  const options = (types: string[]) => models.filter((model) => types.includes((model.type ?? '').toLowerCase())).map((model) => ({ value: model.id, label: model.name }));
  const setting = (label: string, description: string, control: ReactNode, hint?: string) => <div className="setting-row flex items-start justify-between gap-6 border-b border-black/[.08] py-5 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2"><div className="setting-info min-w-0 flex-1 pr-4"><label className="block text-[13px] font-medium leading-[1.4] text-black/90">{label}</label><p className="desc mt-1 text-xs leading-[1.5] text-[#8a8a8a]">{description}</p>{hint ? <p className="desc hint mt-1 text-xs leading-[1.5] text-[#0a8f4c]">{hint}</p> : null}</div><div className="setting-control flex min-w-[220px] shrink-0 items-center justify-end max-[720px]:min-w-0 max-[720px]:w-full">{control}</div></div>;
  const autoFields = writeMode === 'auto' ? <>
    {setting(t('memoryWorkspaceSettings.extractModelLabel'), t('memoryWorkspaceSettings.extractModelDescription'), <ModelOptionSelect value={extractModelId} options={options(['chat', 'vllm'])} disabled={!canEdit || busy} addModelLabel={t('model.addModelInSettings')} onAddModel={() => window.location.assign('/platform/settings?section=models&subsection=chat')} onChange={(value) => debouncedSave({ extractModelId: value })} />)}
    {setting(t('memoryWorkspaceSettings.extractDelayLabel'), t('memoryWorkspaceSettings.extractDelayDescription'), <NumberInput min={5} max={3600} step={15} value={extractDelaySeconds} disabled={!canEdit || busy} onValueChange={(value) => setExtractDelaySeconds(Number(value))} onBlur={() => debouncedSave()} />)}
    {setting(t('memoryWorkspaceSettings.extractMinIntervalLabel'), t('memoryWorkspaceSettings.extractMinIntervalDescription'), <NumberInput min={0} max={86400} step={60} value={extractMinIntervalSeconds} disabled={!canEdit || busy} onValueChange={(value) => setExtractMinIntervalSeconds(Number(value))} onBlur={() => debouncedSave()} />)}
    {setting(t('memoryWorkspaceSettings.interestThresholdLabel'), t('memoryWorkspaceSettings.interestThresholdDescription'), <NumberInput min={1} max={20} step={1} value={interestThreshold} disabled={!canEdit || busy} onValueChange={(value) => setInterestThreshold(Number(value))} onBlur={() => debouncedSave()} />)}
    {setting(t('memoryWorkspaceSettings.instructionsLabel'), t('memoryWorkspaceSettings.instructionsDescription'), <textarea className="h-auto min-h-20 w-full resize-y rounded-[3px] border border-[#dcdcdc] p-2 text-[13px] focus:border-[#07c05f] focus:outline-none focus:ring-2 focus:ring-[#07c05f]/20" maxLength={1000} rows={3} value={extractInstructions} disabled={!canEdit || busy} placeholder={t('memoryWorkspaceSettings.instructionsPlaceholder')} onChange={(event) => setExtractInstructions(event.target.value)} onBlur={() => debouncedSave()} />)}
  </> : null;
  return <Card><div className="wk-settings-panel-heading"><div><h3>{t('memoryWorkspaceSettings.title')}</h3><p className="wk-muted">{t('memoryWorkspaceSettings.description')}</p></div></div><div className="wk-memory-workspace-intro mb-2 flex items-start gap-2.5 rounded-lg bg-[#f3f9f5] px-4 py-3.5 text-[13px] text-black/70"><strong className="shrink-0 font-medium text-black/90">{t('memoryWorkspaceSettings.introTitle')}</strong><span>{t('memoryWorkspaceSettings.introDescription')}</span></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="settings-group">{setting(t('memoryWorkspaceSettings.enableLabel'), t('memoryWorkspaceSettings.enableDescription'), <Switch checked={enabled} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ enabled: checked })} aria-label={t('memoryWorkspaceSettings.enableLabel')} />)}{enabled ? <>{setting(t('memoryWorkspaceSettings.writeModeLabel'), t('memoryWorkspaceSettings.writeModeDescription'), <ModelOptionSelect value={writeMode} options={[{ value: 'explicit_only', label: t('memoryWorkspaceSettings.writeModeExplicit') }, { value: 'auto', label: t('memoryWorkspaceSettings.writeModeAuto') }]} disabled={!canEdit || busy} onChange={(value) => debouncedSave({ writeMode: value })} />, writeMode === 'auto' ? t('memoryWorkspaceSettings.writeModeAutoHint') : t('memoryWorkspaceSettings.writeModeExplicitHint'))}{autoFields}{setting(t('memoryWorkspaceSettings.vectorRecallLabel'), t('memoryWorkspaceSettings.vectorRecallDescription'), <Switch checked={vectorRecall} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ vectorRecall: checked })} aria-label={t('memoryWorkspaceSettings.vectorRecallLabel')} />)}{vectorRecall ? setting(t('memoryWorkspaceSettings.embeddingModelLabel'), t('memoryWorkspaceSettings.embeddingModelDescription'), <ModelOptionSelect value={embeddingModelId} options={options(['embedding'])} disabled={!canEdit || busy} clearable clearLabel={t('common.remove')} addModelLabel={t('model.addModelInSettings')} onAddModel={() => window.location.assign('/platform/settings?section=models&subsection=embedding')} onChange={(value) => debouncedSave({ embeddingModelId: value })} />) : null}{setting(t('memoryWorkspaceSettings.conditioningLabel'), t('memoryWorkspaceSettings.conditioningDescription'), <Switch checked={retrievalConditioning} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ retrievalConditioning: checked })} aria-label={t('memoryWorkspaceSettings.conditioningLabel')} />)}{setting(t('memoryWorkspaceSettings.maxItemsLabel'), t('memoryWorkspaceSettings.maxItemsDescription'), <NumberInput min={10} max={2000} step={10} value={maxItems} disabled={!canEdit || busy} onValueChange={(value) => setMaxItems(Number(value))} onBlur={() => debouncedSave()} />)}</> : null}</div></Card>;
}
