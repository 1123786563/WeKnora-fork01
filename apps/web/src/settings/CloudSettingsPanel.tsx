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
  const [secretVisible, setSecretVisible] = useState(false);
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
    {/* Vue WeKnoraCloudSettings.vue: header renders directly on the section
        background (no card), banner + form rows with row separators, bordered
        models box with row separators, gray usage hint box. */}
    <div>
      <div className="mb-6">
        <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[23px] text-[rgba(0,0,0,0.9)]">{t('settings.weknoraCloud.title')}</h2>
        {/* Vue .section-description: 14px, line-height 1.5 (21px). */}
        <p className="m-0 mb-[10px] text-[14px] leading-[21px] text-[rgba(0,0,0,0.6)]">{t('settings.weknoraCloud.description')}{' '}
          <a className="inline-flex items-center gap-[3px] text-[#07c05f] [font-weight:450] hover:underline" href="https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html" target="_blank" rel="noopener noreferrer">{t('settings.weknoraCloud.viewDocs')}<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" /><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" /></svg></a>
        </p>
      </div>
      {credentialState === 'unconfigured' ? <div className="mb-5 flex items-center gap-2 rounded-[6px] border border-[#e7e7e7] bg-[#f3f3f3] px-[14px] py-[10px] text-[13px] leading-[18px] text-[rgba(0,0,0,0.6)]"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" className="shrink-0" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg><span>{t('settings.weknoraCloud.unconfigured')}</span></div>
        : credentialState === 'expired' ? <div className="mb-5 flex items-start gap-2 rounded-[6px] border border-[#fed7aa] border-l-[3px] border-l-[#f97316] bg-[#fff7ed] px-4 py-3 text-[13px] text-[#9a3412]"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#f97316" strokeWidth="1.8" className="mt-[2px] shrink-0" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg><span><strong>{t('settings.weknoraCloud.expired')}</strong><br />{typeof status.reason === 'string' && status.reason ? status.reason : t('settings.weknoraCloud.expiredDefault')}</span></div>
        : <div className="mb-5 flex items-center justify-between gap-3 text-[13px] text-muted-strong"><span className="flex items-center gap-2"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--wks-brand, #0a7f43)" strokeWidth="1.8" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M8 12l3 3 5-6" /></svg><span className="flex-1">{t('settings.weknoraCloud.configured')}</span></span>{!formExpanded ? <Button type="button" size="small" onClick={() => setFormExpanded(true)}>{t('settings.weknoraCloud.reconfigure')}</Button> : null}</div>}
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {formExpanded ? <div className="mb-6 grid">
        <div className="flex items-center justify-between gap-4 border-b border-[#e7e7e7] py-4 max-[720px]:flex-col max-[720px]:items-start">
          <div className="min-w-0 flex-1"><label className="mb-1 block text-[14px] font-medium leading-[17px] text-[rgba(0,0,0,0.9)]">{t('settings.weknoraCloud.appIdLabel')}</label><p className="m-0 text-[13px] leading-[19.5px] text-[rgba(0,0,0,0.6)]">{t('settings.weknoraCloud.appIdDesc')}</p></div>
          <span className="block w-[280px] shrink-0"><Input className="w-full" autoComplete="off" placeholder={t('settings.weknoraCloud.appIdPlaceholder')} value={appId} onChange={(event) => setAppId(event.target.value)} /></span>
        </div>
        <div className="flex items-center justify-between gap-4 border-b border-[#e7e7e7] py-4 max-[720px]:flex-col max-[720px]:items-start">
          <div className="min-w-0 flex-1"><label className="mb-1 block text-[14px] font-medium leading-[17px] text-[rgba(0,0,0,0.9)]">{t('settings.weknoraCloud.appSecretLabel')}</label><p className="m-0 text-[13px] leading-[19.5px] text-[rgba(0,0,0,0.6)]">{t('settings.weknoraCloud.appSecretDesc')}</p></div>
          {/* Vue t-input type="password" carries the built-in browse-off eye
              suffix; reproduce the toggle with an overlaid button. */}
          <span className="relative block w-[280px] shrink-0"><Input className="w-full" type={secretVisible ? 'text' : 'password'} autoComplete="new-password" placeholder={t('settings.weknoraCloud.appSecretPlaceholder')} value={appSecret} onChange={(event) => setAppSecret(event.target.value)} /><button type="button" aria-label={secretVisible ? '隐藏' : '显示'} title={secretVisible ? '隐藏' : '显示'} className="absolute right-[9px] top-1/2 z-[1] flex h-[24px] w-[24px] -translate-y-1/2 cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-[#98a2b3] hover:text-[rgba(0,0,0,0.9)]" onClick={() => setSecretVisible((visible) => !visible)}><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{secretVisible ? <><path d="M2 12s3.5-6.5 10-6.5S22 12 22 12s-3.5 6.5-10 6.5S2 12 2 12z" /><circle cx="12" cy="12" r="2.6" /></> : <><path d="M2 12s3.5-6.5 10-6.5c2 0 3.8.6 5.3 1.5M22 12s-3.5 6.5-10 6.5c-2 0-3.8-.6-5.3-1.5" /><path d="M4 20 20 4" /></>}</svg></button></span>
        </div>
        {/* Vue action-row keeps the 16px bottom padding of .setting-row. */}
        <div className="flex items-center justify-between gap-3 pt-5 pb-4">
          <p className="m-0 text-[13px] text-[rgba(0,0,0,0.6)]">{t('settings.weknoraCloud.saveHint')}</p>
          <Button type="button" variant="primary" loading={saving} disabled={!appId.trim() || !appSecret.trim()} onClick={() => void save()}>{t('settings.weknoraCloud.saveBtn')}</Button>
        </div>
      </div> : null}
    </div>
    <section className={`mb-6 rounded-[8px] border border-[#e7e7e7] p-4 ${credentialState !== 'configured' ? 'bg-[#f3f3f3]' : 'bg-surface'}`}>
      <div className="mb-[14px]">
        <h3 className="m-0 mb-[6px] text-[15px] font-semibold leading-[21px] text-[rgba(0,0,0,0.9)]">{t('settings.weknoraCloud.modelsSection.title')}</h3>
        <p className="m-0 text-[13px] leading-[20.15px] text-[rgba(0,0,0,0.6)]">{credentialState === 'configured' ? t('settings.weknoraCloud.modelsSection.descReady') : t('settings.weknoraCloud.modelsSection.descPending')}</p>
      </div>
      <div className="grid">
        {WKC_MODEL_KINDS.map((kind) => <div key={kind} className={`flex items-center justify-between gap-3 border-b border-[#e7e7e7] py-3 first:pt-1 last:border-b-0 last:pb-0 ${credentialState !== 'configured' ? 'opacity-90' : ''}`}>
          <div className="flex min-w-0 items-center gap-[10px]">
            {/* Vue row label/code use line-height normal (17–20px) — the
                inherited 21px inflates each row and drifts the list. */}
            <span className="min-w-[88px] text-[14px] font-medium leading-[normal] text-[rgba(0,0,0,0.9)]">{kindLabel(kind)}</span>
            <code className="rounded-[4px] bg-[#f3f3f3] px-[6px] [font-family:monospace] text-[12px] leading-[18px] text-[rgba(0,0,0,0.6)]">{WKC_MODEL_NAME_BY_KIND[kind]}</code>
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
              : <span className="text-[12px] leading-[15px] text-[rgba(0,0,0,0.4)]">{t('settings.weknoraCloud.modelsSection.statusPending')}</span>}
          </div>
        </div>)}
      </div>
      {credentialState === 'configured' && missingKinds.length > 1 ? (
        <div className="mt-[14px] border-t border-dashed border-[#e7e7e7] pt-[14px]">
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
      ) : credentialState === 'configured' && missingKinds.length === 0 ? <p className="mb-0 mt-[14px] flex items-center gap-1.5 text-[13px] text-[#0a7f43]"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><circle cx="12" cy="12" r="10" /><path d="M8 12l3 3 5-6" /></svg>{t('settings.weknoraCloud.modelsSection.allReady')}</p> : null}
    </section>
    <div className="rounded-[8px] border border-[#e7e7e7] bg-[#f3f3f3] px-4 py-[14px]">
      {/* Vue .hint-title: 13px/500 lh normal (18px). */}
      <p className="m-0 mb-2 text-[13px] font-medium leading-[normal] text-[rgba(0,0,0,0.4)]">{t('settings.weknoraCloud.usageTitle')}</p>
      <p className="mb-0 mt-0 text-[13px] leading-[1.8] text-[rgba(0,0,0,0.6)]">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="block">{line}</span>)}</p>
    </div>
  </div>;
}
