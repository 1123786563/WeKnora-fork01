import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { NumberInput, Status, Switch, Textarea } from '@weknora/ui';
import { ModelOptionSelect } from './ModelOptionSelect.tsx';
import { navigate } from '../platform/navigation.ts';
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
    catch (reason) { setError(reason instanceof Error ? reason.message : t('common.operationFailed')); }
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
  const setting = (label: string, description: string, control: ReactNode, hint?: string) => <div className="flex items-start justify-between border-b border-[#e7e7e7] py-5 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2"><div className="max-w-[65%] flex-1 pr-6 max-[720px]:max-w-full max-[720px]:pr-0"><label className="mb-1 block text-[15px] font-medium text-[rgba(0,0,0,0.9)]">{label}</label><p className="m-0 text-[13px] leading-[1.5] text-[rgba(0,0,0,0.6)]">{description}</p>{hint ? <p className="m-0 mt-1 text-[13px] leading-[1.5] text-[rgba(0,0,0,0.4)]">{hint}</p> : null}</div><div className="flex shrink-0 items-center justify-end">{control}</div></div>;
  const autoFields = writeMode === 'auto' ? <>
    {setting(t('memoryWorkspaceSettings.extractModelLabel'), t('memoryWorkspaceSettings.extractModelDescription'), <ModelOptionSelect value={extractModelId} options={options(['chat', 'vllm'])} disabled={!canEdit || busy} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=chat')} onChange={(value) => debouncedSave({ extractModelId: value })} />)}
    {setting(t('memoryWorkspaceSettings.extractDelayLabel'), t('memoryWorkspaceSettings.extractDelayDescription'), <NumberInput min={5} max={3600} step={15} value={extractDelaySeconds} disabled={!canEdit || busy} onValueChange={(value) => setExtractDelaySeconds(Number(value))} onBlur={() => debouncedSave()} />)}
    {setting(t('memoryWorkspaceSettings.extractMinIntervalLabel'), t('memoryWorkspaceSettings.extractMinIntervalDescription'), <NumberInput min={0} max={86400} step={60} value={extractMinIntervalSeconds} disabled={!canEdit || busy} onValueChange={(value) => setExtractMinIntervalSeconds(Number(value))} onBlur={() => debouncedSave()} />)}
    {setting(t('memoryWorkspaceSettings.interestThresholdLabel'), t('memoryWorkspaceSettings.interestThresholdDescription'), <NumberInput min={1} max={20} step={1} value={interestThreshold} disabled={!canEdit || busy} onValueChange={(value) => setInterestThreshold(Number(value))} onBlur={() => debouncedSave()} />)}
  </> : null;

  // Vue MemoryWorkspaceSettings.vue: bare section on the drawer background —
  // h2 header, neutral intro box with brand icon, bordered setting rows, and a
  // stacked full-width custom-prompt row.
  return <div className="w-full text-[rgba(0,0,0,0.9)]">
    <div className="mb-6">
      <h2 className="m-0 mb-2 text-[20px] font-semibold text-[rgba(0,0,0,0.9)]">{t('memoryWorkspaceSettings.title')}</h2>
      <p className="m-0 text-sm leading-[1.5] text-[rgba(0,0,0,0.6)]">{t('memoryWorkspaceSettings.description')}</p>
    </div>
    <div className="mb-2 flex items-start gap-2.5 rounded-lg bg-[#f3f3f3] px-4 py-[14px]" role="note">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="mt-0.5 shrink-0 text-[#07c05f]"><circle cx="8" cy="8" r="6.25" /><line x1="8" y1="7.4" x2="8" y2="11.2" /><line x1="8" y1="4.9" x2="8" y2="5.1" /></svg>
      <div>
        <p className="m-0 mb-1 text-sm font-medium text-[rgba(0,0,0,0.9)]">{t('memoryWorkspaceSettings.introTitle')}</p>
        <p className="m-0 text-[13px] leading-[1.6] text-[rgba(0,0,0,0.6)]">{t('memoryWorkspaceSettings.introDescription')}</p>
      </div>
    </div>
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <div className="flex flex-col">
      {setting(t('memoryWorkspaceSettings.enableLabel'), t('memoryWorkspaceSettings.enableDescription'), <Switch checked={enabled} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ enabled: checked })} aria-label={t('memoryWorkspaceSettings.enableLabel')} />)}
      {enabled ? <>{setting(t('memoryWorkspaceSettings.writeModeLabel'), t('memoryWorkspaceSettings.writeModeDescription'), <div className="inline-flex overflow-hidden rounded-[3px] border border-[#e7e7e7]" role="radiogroup" aria-label={t('memoryWorkspaceSettings.writeModeLabel')}>
        <button type="button" role="radio" aria-checked={writeMode === 'explicit_only'} className={'h-7 cursor-pointer border-0 bg-transparent px-4 py-0 font-[inherit] text-[length:inherit] leading-[inherit]' + (writeMode === 'explicit_only' ? ' is-active bg-[#07c05f] text-white hover:bg-[#06b04d]' : ' hover:text-[#07c05f]')} disabled={!canEdit || busy} onClick={() => debouncedSave({ writeMode: 'explicit_only' })}>{t('memoryWorkspaceSettings.writeModeExplicit')}</button>
        <button type="button" role="radio" aria-checked={writeMode === 'auto'} className={'h-7 cursor-pointer border-0 bg-transparent px-4 py-0 font-[inherit] text-[length:inherit] leading-[inherit] border-l border-l-[#e7e7e7]' + (writeMode === 'auto' ? ' is-active bg-[#07c05f] text-white border-l-[#07c05f] hover:bg-[#06b04d]' : ' hover:border-l-[#07c05f] hover:text-[#07c05f]')} disabled={!canEdit || busy} onClick={() => debouncedSave({ writeMode: 'auto' })}>{t('memoryWorkspaceSettings.writeModeAuto')}</button>
      </div>, writeMode === 'auto' ? t('memoryWorkspaceSettings.writeModeAutoHint') : t('memoryWorkspaceSettings.writeModeExplicitHint'))}{autoFields}
        <div className="flex flex-col items-stretch border-b border-[#e7e7e7] py-5 last:border-b-0">
          <div className="mb-2.5 max-w-full flex-1 pr-0">
            <label className="mb-1 block text-[15px] font-medium text-[rgba(0,0,0,0.9)]">{t('memoryWorkspaceSettings.instructionsLabel')}</label>
            <p className="m-0 text-[13px] leading-[1.5] text-[rgba(0,0,0,0.6)]">{t('memoryWorkspaceSettings.instructionsDescription')}</p>
          </div>
          <div className="flex w-full items-center justify-stretch">
            <Textarea className="w-full min-h-[72px] resize-y" maxLength={1000} rows={3} value={extractInstructions} disabled={!canEdit || busy} placeholder={t('memoryWorkspaceSettings.instructionsPlaceholder')} onChange={(event) => setExtractInstructions(event.target.value)} onBlur={() => debouncedSave()} />
          </div>
        </div>
        {setting(t('memoryWorkspaceSettings.vectorRecallLabel'), t('memoryWorkspaceSettings.vectorRecallDescription'), <Switch checked={vectorRecall} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ vectorRecall: checked })} aria-label={t('memoryWorkspaceSettings.vectorRecallLabel')} />)}
        {vectorRecall ? setting(t('memoryWorkspaceSettings.embeddingModelLabel'), t('memoryWorkspaceSettings.embeddingModelDescription'), <ModelOptionSelect value={embeddingModelId} options={options(['embedding'])} disabled={!canEdit || busy} clearable clearLabel={t('common.remove')} addModelLabel={t('model.addModelInSettings')} onAddModel={() => navigate('/platform/settings?section=models&subsection=embedding')} onChange={(value) => debouncedSave({ embeddingModelId: value })} />) : null}
        {setting(t('memoryWorkspaceSettings.conditioningLabel'), t('memoryWorkspaceSettings.conditioningDescription'), <Switch checked={retrievalConditioning} disabled={!canEdit || busy} onCheckedChange={(checked) => debouncedSave({ retrievalConditioning: checked })} aria-label={t('memoryWorkspaceSettings.conditioningLabel')} />)}
        {setting(t('memoryWorkspaceSettings.maxItemsLabel'), t('memoryWorkspaceSettings.maxItemsDescription'), <NumberInput min={10} max={2000} step={10} value={maxItems} disabled={!canEdit || busy} onValueChange={(value) => setMaxItems(Number(value))} onBlur={() => debouncedSave()} />)}
      </> : null}
    </div>
  </div>;
}
