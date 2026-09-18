import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Status } from '@weknora/ui';
import { cloudCredentialPatch } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

/* Full port of Vue WeKnoraCloudSettings.vue: credential state banner
   (unconfigured / expired / configured), the APPID/APPSECRET form with the
   重新配置 collapse, the 云模型接入 rows (chat/embedding/rerank/vlm) with
   per-kind and batch adds gated on the credential state, and the numbered
   usage steps. Model building mirrors frontend/src/utils/weknoraCloudModels.ts. */

function row(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }

const WEKNORA_CLOUD_BASE_URL = 'https://weknora.weixin.qq.com';
const WEKNORA_CLOUD_PROVIDER = 'weknoracloud';

type WkcModelKind = 'chat' | 'embedding' | 'rerank' | 'vllm';
const WKC_MODEL_KINDS: WkcModelKind[] = ['chat', 'embedding', 'rerank', 'vllm'];
const WKC_MODEL_NAME_BY_KIND: Record<WkcModelKind, string> = { chat: 'chat', embedding: 'embedding', rerank: 'rerank', vllm: 'vlm' };
const BACKEND_TYPE_BY_KIND: Record<WkcModelKind, string> = { chat: 'KnowledgeQA', embedding: 'Embedding', rerank: 'Rerank', vllm: 'VLLM' };

function isWeKnoraCloudModel(model: Record<string, unknown>): boolean {
  const parameters = row(model.parameters);
  return parameters.provider === WEKNORA_CLOUD_PROVIDER;
}

function existingWkcKinds(models: Array<Record<string, unknown>>): Set<WkcModelKind> {
  const found = new Set<WkcModelKind>();
  for (const model of models) {
    if (!isWeKnoraCloudModel(model)) continue;
    for (const kind of WKC_MODEL_KINDS) {
      if (String(model.type ?? '') === BACKEND_TYPE_BY_KIND[kind]) found.add(kind);
    }
  }
  return found;
}

function buildWkcModelConfig(kind: WkcModelKind, displayName: string, dimension?: number): Record<string, unknown> {
  const parameters: Record<string, unknown> = {
    base_url: WEKNORA_CLOUD_BASE_URL,
    provider: WEKNORA_CLOUD_PROVIDER,
  };
  if (kind === 'embedding' && dimension) {
    parameters.embedding_parameters = { dimension, truncate_prompt_tokens: 0 };
  }
  if (kind === 'vllm') {
    parameters.enable_multimodal = true;
  }
  return {
    name: WKC_MODEL_NAME_BY_KIND[kind],
    display_name: displayName,
    type: BACKEND_TYPE_BY_KIND[kind],
    parameters,
  };
}

export function CloudSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const t = settingsT(readInitialLocale());
  const [status, setStatus] = useState(() => row(initialValue));
  const [appId, setAppId] = useState('');
  const [appSecret, setAppSecret] = useState('');
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [toast, setToast] = useState<{ tone: 'success' | 'warning' | 'error'; text: string } | null>(null);
  // Vue checkStatus collapses the form once credentials are configured;
  // derive the initial state during first render (static renders included).
  const [formExpanded, setFormExpanded] = useState(() => {
    const initial = row(initialValue);
    return !(initial.has_models === true && initial.needs_reinit !== true);
  });
  const [existingKinds, setExistingKinds] = useState<Set<WkcModelKind>>(new Set());
  const [addingModels, setAddingModels] = useState(false);
  const [addingKind, setAddingKind] = useState<WkcModelKind | null>(null);
  const [confirmAdd, setConfirmAdd] = useState<WkcModelKind[] | null>(null);

  const needsReinit = status.needs_reinit === true;
  // Vue checkStatus: hasCredentials derives from has_models (the status
  // endpoint carries no separate credential flag).
  const hasCredentials = status.has_models === true && !needsReinit;
  const credentialState: 'unconfigured' | 'expired' | 'configured' = needsReinit ? 'expired' : hasCredentials ? 'configured' : 'unconfigured';

  const showToast = (tone: 'success' | 'warning' | 'error', text: string) => setToast({ tone, text });
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 3000);
    return () => clearTimeout(timer);
  }, [toast]);

  const refreshExistingKinds = useCallback(async () => {
    try {
      const models = await client.configuration.models.list();
      setExistingKinds(existingWkcKinds(models.map((model) => row(model as unknown))));
    } catch { setExistingKinds(new Set()); }
  }, [client]);

  useEffect(() => { void refreshExistingKinds(); }, [refreshExistingKinds]);


  async function reload() {
    setBusy(true); setError(null); setNotice(null);
    try {
      const next = await client.settings.weknoraCloud.status();
      setStatus(next);
      // Vue checkStatus: a configured credential collapses the form behind
      // the 重新配置 button.
      const nextReinit = next.needs_reinit === true;
      if (next.has_models === true && !nextReinit) setFormExpanded(false);
    }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('common.error')); }
    finally { setBusy(false); }
  }

  async function save() {
    if (!appId.trim() || !appSecret.trim()) { showToast('warning', t('settings.weknoraCloud.fillRequired')); return; }
    setSaving(true); setError(null); setNotice(null);
    try {
      await client.settings.weknoraCloud.saveCredentials(cloudCredentialPatch(appId, appSecret));
      showToast('success', t('settings.weknoraCloud.saveSuccess'));
      setAppId(''); setAppSecret('');
      await reload();
      setFormExpanded(false);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('settings.weknoraCloud.saveFailed'));
    } finally { setSaving(false); }
  }

  async function resolveEmbeddingDimension(): Promise<number> {
    const result = await client.configuration.models.connection.embedding({
      source: 'remote',
      modelName: WKC_MODEL_NAME_BY_KIND.embedding,
      baseUrl: WEKNORA_CLOUD_BASE_URL,
      provider: WEKNORA_CLOUD_PROVIDER,
    });
    const payload = row(result);
    if (payload.available !== true || !payload.dimension) {
      throw new Error(typeof payload.message === 'string' && payload.message ? payload.message : t('settings.weknoraCloud.addModelsEmbeddingFailed'));
    }
    return Number(payload.dimension);
  }

  async function addModels(kinds: WkcModelKind[]) {
    const targets = kinds.filter((kind) => !existingKinds.has(kind));
    if (targets.length === 0) return;
    setAddingModels(true);
    setAddingKind(targets.length === 1 ? targets[0] : null);
    let success = 0;
    let failed = 0;
    let embeddingDimension: number | undefined;
    try {
      for (const kind of targets) {
        try {
          if (kind === 'embedding') embeddingDimension = embeddingDimension ?? await resolveEmbeddingDimension();
          const displayName = t(`settings.weknoraCloud.addModelsDisplayName.${kind}`);
          await client.configuration.models.create(buildWkcModelConfig(kind, displayName, kind === 'embedding' ? embeddingDimension : undefined));
          setExistingKinds((current) => new Set([...current, kind]));
          success += 1;
        } catch { failed += 1; }
      }
      if (success > 0 && failed === 0) showToast('success', t('settings.weknoraCloud.addModelsSuccess', { count: success }));
      else if (success > 0) showToast('warning', t('settings.weknoraCloud.addModelsPartial', { success, failed }));
      else showToast('error', t('settings.weknoraCloud.addModelsFailed'));
    } finally {
      setAddingModels(false);
      setAddingKind(null);
      setConfirmAdd(null);
    }
  }

  const missingKinds = WKC_MODEL_KINDS.filter((kind) => !existingKinds.has(kind));
  const kindLabel = (kind: WkcModelKind): string => t(`modelSettings.typeShort.${kind}`);

  return <div className="wk-settings-cloud">
    {toast ? <div role="status" aria-live="polite" className={`fixed left-1/2 top-[24px] z-[10060] -translate-x-1/2 rounded-[8px] px-[14px] py-[8px] text-[13px] text-white shadow-[0_4px_12px_rgba(0,0,0,0.2)] ${toast.tone === 'success' ? 'bg-[rgba(7,192,95,0.9)]' : toast.tone === 'warning' ? 'bg-[rgba(250,173,20,0.92)]' : 'bg-[rgba(213,73,65,0.92)]'}`}>{toast.text}</div> : null}
    <Card>
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div><h3>{t('settings.weknoraCloud.title')}</h3>
          <p className="wk-muted text-muted m-0">{t('settings.weknoraCloud.description')}{' '}
            <a className="text-[#245a9b] hover:underline" href="https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html" target="_blank" rel="noopener noreferrer">{t('settings.weknoraCloud.viewDocs')}</a>
          </p>
        </div>
      </div>
      {credentialState === 'unconfigured' ? <div className="mb-3 flex items-center gap-2 rounded-[8px] bg-[rgba(250,173,20,0.08)] px-3 py-2 text-[13px] text-[#ad4b00]"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg><span>{t('settings.weknoraCloud.unconfigured')}</span></div>
        : credentialState === 'expired' ? <div className="mb-3 flex items-start gap-2 rounded-[8px] bg-[rgba(249,115,22,0.08)] px-3 py-2 text-[13px] text-[#c2610c]"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" className="mt-[2px] shrink-0" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg><span><strong>{t('settings.weknoraCloud.expired')}</strong><br />{typeof status.reason === 'string' && status.reason ? status.reason : t('settings.weknoraCloud.expiredDefault')}</span></div>
        : <div className="mb-3 flex items-center justify-between gap-3 rounded-[8px] bg-[rgba(7,192,95,0.08)] px-3 py-2 text-[13px] text-[#0a7f43]"><span className="flex items-center gap-2"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M8 12l3 3 5-6" /></svg>{t('settings.weknoraCloud.configured')}</span>{!formExpanded ? <Button type="button" size="small" onClick={() => setFormExpanded(true)}>{t('settings.weknoraCloud.reconfigure')}</Button> : null}</div>}
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {formExpanded ? <div className="mb-4 grid gap-3">
        <div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] items-start gap-[.8rem] max-[720px]:grid-cols-1">
          <div><label className="text-[13px] font-semibold text-[#27364d]">{t('settings.weknoraCloud.appIdLabel')}</label><p className="m-0 text-[12px] text-muted-strong">{t('settings.weknoraCloud.appIdDesc')}</p></div>
          <Input className="max-w-[280px]" autoComplete="off" placeholder={t('settings.weknoraCloud.appIdPlaceholder')} value={appId} onChange={(event) => setAppId(event.target.value)} />
        </div>
        <div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] items-start gap-[.8rem] max-[720px]:grid-cols-1">
          <div><label className="text-[13px] font-semibold text-[#27364d]">{t('settings.weknoraCloud.appSecretLabel')}</label><p className="m-0 text-[12px] text-muted-strong">{t('settings.weknoraCloud.appSecretDesc')}</p></div>
          <Input className="max-w-[280px]" type="password" autoComplete="new-password" placeholder={t('settings.weknoraCloud.appSecretPlaceholder')} value={appSecret} onChange={(event) => setAppSecret(event.target.value)} />
        </div>
        <div className="flex items-center justify-between gap-3">
          <p className="m-0 text-[12px] text-muted-strong">{t('settings.weknoraCloud.saveHint')}</p>
          <Button type="button" loading={saving} disabled={!appId.trim() || !appSecret.trim()} onClick={() => void save()}>{t('settings.weknoraCloud.saveBtn')}</Button>
        </div>
      </div> : null}
    </Card>
    <Card>
      <h3 className="m-0">{t('settings.weknoraCloud.modelsSection.title')}</h3>
      <p className="mt-1 text-[13px] text-muted-strong">{credentialState === 'configured' ? t('settings.weknoraCloud.modelsSection.descReady') : t('settings.weknoraCloud.modelsSection.descPending')}</p>
      <div className={`mt-3 grid gap-2 ${credentialState !== 'configured' ? 'opacity-60' : ''}`}>
        {WKC_MODEL_KINDS.map((kind) => <div key={kind} className="flex items-center justify-between gap-3 rounded-[8px] border border-line-soft px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <span className="text-[13px] font-medium text-ink">{kindLabel(kind)}</span>
            <code className="rounded-[4px] bg-surface-wash px-1.5 py-0.5 font-mono text-[12px] text-muted">{WKC_MODEL_NAME_BY_KIND[kind]}</code>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {credentialState === 'configured' && existingKinds.has(kind) ? <Status tone="success">{t('settings.weknoraCloud.modelsSection.statusAdded')}</Status>
              : credentialState === 'configured' ? (
                <span className="relative inline-flex">
                  <Button type="button" size="small" loading={addingKind === kind} disabled={addingModels && addingKind !== kind} onClick={() => setConfirmAdd([kind])}>{t('settings.weknoraCloud.modelsSection.addOne')}</Button>
                  {confirmAdd && confirmAdd.length === 1 && confirmAdd[0] === kind ? <span className="absolute right-0 top-full z-[60] mt-[6px] flex w-[240px] flex-col gap-2 rounded-[8px] border border-[#e7e7ea] bg-white p-3 text-[13px] shadow-[0_4px_16px_rgba(0,0,0,0.12)]">
                    <span>{t('settings.weknoraCloud.modelsSection.confirmAddOne', { type: kindLabel(kind), name: WKC_MODEL_NAME_BY_KIND[kind] })}</span>
                    <span className="flex items-center justify-end gap-2">
                      <Button type="button" size="small" variant="text" onClick={() => setConfirmAdd(null)}>{t('common.cancel')}</Button>
                      <Button type="button" size="small" onClick={() => void addModels([kind])}>{t('settings.weknoraCloud.modelsSection.addOne')}</Button>
                    </span>
                  </span> : null}
                </span>)
              : <span className="text-[12px] text-muted-strong">{t('settings.weknoraCloud.modelsSection.statusPending')}</span>}
          </div>
        </div>)}
      </div>
      {credentialState === 'configured' && missingKinds.length > 1 ? (
        <div className="mt-3">
          <span className="relative inline-flex">
            <Button type="button" loading={addingModels && !addingKind} disabled={addingModels && !!addingKind} onClick={() => setConfirmAdd([...missingKinds])}>{t('settings.weknoraCloud.modelsSection.addAllBtn', { count: missingKinds.length })}</Button>
            {confirmAdd && confirmAdd.length > 1 ? <span className="absolute right-0 top-full z-[60] mt-[6px] flex w-[260px] flex-col gap-2 rounded-[8px] border border-[#e7e7ea] bg-white p-3 text-[13px] shadow-[0_4px_16px_rgba(0,0,0,0.12)]">
              <span>{t('settings.weknoraCloud.modelsSection.confirmAddAll', { count: confirmAdd.length })}</span>
              <span className="flex items-center justify-end gap-2">
                <Button type="button" size="small" variant="text" onClick={() => setConfirmAdd(null)}>{t('common.cancel')}</Button>
                <Button type="button" size="small" onClick={() => void addModels(confirmAdd)}>{t('settings.weknoraCloud.modelsSection.addAllConfirm')}</Button>
              </span>
            </span> : null}
          </span>
        </div>
      ) : credentialState === 'configured' && missingKinds.length === 0 ? <p className="mb-0 mt-3 flex items-center gap-1 text-[13px] text-[#0a7f43]"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M8 12l3 3 5-6" /></svg>{t('settings.weknoraCloud.modelsSection.allReady')}</p> : null}
    </Card>
    <Card>
      <p className="m-0 text-[13px] font-semibold text-ink">{t('settings.weknoraCloud.usageTitle')}</p>
      <p className="mb-0 mt-1 text-[13px] leading-[1.6] text-muted-strong">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="block">{line}</span>)}</p>
    </Card>
  </div>;
}
