// WeKnora Cloud settings — TDesign 同构迁移（T12b）平移自
// frontend/src/views/settings/WeKnoraCloudSettings.vue。DOM/类名/文案逐节点
// 对照 Vue SFC（凭证状态条 / 配置表单 / 云模型接入区 / 使用说明）；样式在
// settings.td.css §11（scoped 块以 .weknoracloud-settings 根类限定）。
import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Icon as TIcon } from 'tdesign-icons-react';
import { Button as TButton, Input as TInput, Popconfirm as TPopconfirm, Tag as TTag } from 'tdesign-react';
import { cloudCredentialPatch } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { pushSettingsToast } from './settings-toast.tsx';

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
  const initial = row(initialValue);
  // Vue checkStatus：凭证就绪（has_models 且未失效）时折叠表单。
  const initialReinit = initial.needs_reinit === true;
  const initialConfigured = initial.has_models === true && !initialReinit;
  const [form, setForm] = useState({ appId: '', appSecret: '' });
  const [saving, setSaving] = useState(false);
  const [needsReinit, setNeedsReinit] = useState(initialReinit);
  const [reinitReason, setReinitReason] = useState(typeof initial.reason === 'string' ? initial.reason : '');
  const [hasCredentials, setHasCredentials] = useState(initialConfigured);
  const [formExpanded, setFormExpanded] = useState(!initialConfigured);
  const [existingKinds, setExistingKinds] = useState<Set<WkcModelKind>>(new Set());
  const [addingModels, setAddingModels] = useState(false);
  const [addingKind, setAddingKind] = useState<WkcModelKind | null>(null);

  const credentialState: 'unconfigured' | 'expired' | 'configured' = needsReinit ? 'expired' : hasCredentials ? 'configured' : 'unconfigured';

  const refreshExistingKinds = useCallback(async () => {
    try {
      const models = await client.configuration.models.list();
      setExistingKinds(existingWkcKinds(models.map((model) => row(model as unknown))));
    } catch { setExistingKinds(new Set()); }
  }, [client]);

  useEffect(() => {
    // Vue checkStatus：已配置时拉取云模型接入状态。
    if (hasCredentials) void refreshExistingKinds();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
          if (kind === 'embedding') {
            embeddingDimension = embeddingDimension ?? await resolveEmbeddingDimension();
          }
          const displayName = t(`settings.weknoraCloud.addModelsDisplayName.${kind}`);
          await client.configuration.models.create(buildWkcModelConfig(kind, displayName, kind === 'embedding' ? embeddingDimension : undefined));
          setExistingKinds((current) => new Set([...current, kind]));
          success += 1;
        } catch { failed += 1; }
      }
      if (success > 0 && failed === 0) pushSettingsToast(t('settings.weknoraCloud.addModelsSuccess', { count: success }), 'success');
      else if (success > 0) pushSettingsToast(t('settings.weknoraCloud.addModelsPartial', { success, failed }), 'warning');
      else pushSettingsToast(t('settings.weknoraCloud.addModelsFailed'), 'error');
    } finally {
      setAddingModels(false);
      setAddingKind(null);
    }
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

  // Vue handleSave：校验→保存→toast→清空→置已配置并折叠→刷新接入状态。
  async function handleSave() {
    if (!form.appId || !form.appSecret) {
      pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');
      return;
    }
    setSaving(true);
    try {
      await client.settings.weknoraCloud.saveCredentials(cloudCredentialPatch(form.appId, form.appSecret));
      pushSettingsToast(t('settings.weknoraCloud.saveSuccess'), 'success');
      setForm({ appId: '', appSecret: '' });
      setNeedsReinit(false);
      setReinitReason('');
      setHasCredentials(true);
      setFormExpanded(false);
      await refreshExistingKinds();
    } catch (reason) {
      pushSettingsToast(reason instanceof Error && reason.message ? reason.message : t('settings.weknoraCloud.saveFailed'), 'error');
    } finally { setSaving(false); }
  }

  const missingKinds = WKC_MODEL_KINDS.filter((kind) => !existingKinds.has(kind));
  const kindLabel = (kind: WkcModelKind) => t(`modelSettings.typeShort.${kind}`);

  return (
    <div className="weknoracloud-settings">
      <div className="section-header">
        <h2>{t('settings.weknoraCloud.title')}</h2>
        <p className="section-description">
          {`${t('settings.weknoraCloud.description')} `}
          <a
            className="doc-link"
            href="https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html"
            target="_blank"
            rel="noopener noreferrer"
          >
            {`${t('settings.weknoraCloud.viewDocs')} `}
            <TIcon name="link" className="link-icon" />
          </a>
        </p>
      </div>

      {/* 未配置 */}
      {credentialState === 'unconfigured' ? <div className="credential-status unconfigured">
        <TIcon name="info-circle" style={{ fontSize: '16px', flexShrink: 0 }} />
        <span>{t('settings.weknoraCloud.unconfigured')}</span>
      </div>
        : credentialState === 'expired' ? <div className="credential-warning">
          <TIcon name="error-circle" style={{ fontSize: '16px', color: '#f97316', flexShrink: 0, marginTop: '1px' }} />
          <div className="warning-text">
            <strong>{t('settings.weknoraCloud.expired')}</strong><br />
            {reinitReason || t('settings.weknoraCloud.expiredDefault')}
          </div>
        </div>
          : <div className="credential-status success">
            <TIcon name="check-circle" style={{ fontSize: '16px', color: 'var(--td-brand-color)', flexShrink: 0 }} />
            <span className="status-text">{t('settings.weknoraCloud.configured')}</span>
            {!formExpanded ? <TButton variant="outline" theme="default" size="small" icon={<TIcon name="edit" />} onClick={() => setFormExpanded(true)}>
              {` ${t('settings.weknoraCloud.reconfigure')}`}
            </TButton> : null}
          </div>}

      {/* 配置表单 */}
      {formExpanded ? <div className="settings-group">
        <div className="setting-row">
          <div className="setting-info">
            <label className="setting-label">{t('settings.weknoraCloud.appIdLabel')}</label>
            <p className="setting-desc">{t('settings.weknoraCloud.appIdDesc')}</p>
          </div>
          <div className="setting-control">
            <TInput
              value={form.appId}
              placeholder={t('settings.weknoraCloud.appIdPlaceholder')}
              autocomplete="off"
              style={{ width: '280px' }}
              onChange={(value) => setForm((current) => ({ ...current, appId: String(value ?? '') }))}
            />
          </div>
        </div>
        <div className="setting-row">
          <div className="setting-info">
            <label className="setting-label">{t('settings.weknoraCloud.appSecretLabel')}</label>
            <p className="setting-desc">{t('settings.weknoraCloud.appSecretDesc')}</p>
          </div>
          <div className="setting-control">
            <TInput
              value={form.appSecret}
              type="password"
              placeholder={t('settings.weknoraCloud.appSecretPlaceholder')}
              autocomplete="new-password"
              style={{ width: '280px' }}
              onChange={(value) => setForm((current) => ({ ...current, appSecret: String(value ?? '') }))}
            />
          </div>
        </div>
        <div className="setting-row action-row">
          <div className="setting-info">
            <p className="setting-desc">{t('settings.weknoraCloud.saveHint')}</p>
          </div>
          <div className="setting-control">
            <TButton
              theme="primary"
              loading={saving}
              disabled={!form.appId || !form.appSecret}
              onClick={() => void handleSave()}
            >
              {t('settings.weknoraCloud.saveBtn')}
            </TButton>
          </div>
        </div>
      </div> : null}

      {/* 云模型：凭证就绪后原地展示接入状态 */}
      <section className={'models-section' + (credentialState !== 'configured' ? ' models-section--disabled' : '')}>
        <div className="models-section__header">
          <h3 className="models-section__title">{t('settings.weknoraCloud.modelsSection.title')}</h3>
          <p className="models-section__desc">
            {credentialState === 'configured'
              ? t('settings.weknoraCloud.modelsSection.descReady')
              : t('settings.weknoraCloud.modelsSection.descPending')}
          </p>
        </div>

        <div className="models-list">
          {WKC_MODEL_KINDS.map((kind) => <div key={kind} className="model-row">
            <div className="model-row__main">
              <span className="model-row__label">{kindLabel(kind)}</span>
              <code className="model-row__id">{WKC_MODEL_NAME_BY_KIND[kind]}</code>
            </div>
            <div className="model-row__action">
              {credentialState === 'configured' && existingKinds.has(kind) ? <TTag theme="success" variant="light" size="small">
                {t('settings.weknoraCloud.modelsSection.statusAdded')}
              </TTag>
                : credentialState === 'configured' ? <TPopconfirm
                  content={t('settings.weknoraCloud.modelsSection.confirmAddOne', { type: kindLabel(kind), name: WKC_MODEL_NAME_BY_KIND[kind] })}
                  confirmBtn={{ content: t('settings.weknoraCloud.modelsSection.addOne'), theme: 'primary' }}
                  cancelBtn={{ content: t('common.cancel') }}
                  placement="left"
                  onConfirm={() => void addModels([kind])}
                >
                  <TButton
                    size="small"
                    variant="outline"
                    theme="primary"
                    loading={addingKind === kind}
                    disabled={addingModels && addingKind !== kind}
                  >
                    {t('settings.weknoraCloud.modelsSection.addOne')}
                  </TButton>
                </TPopconfirm>
                  : <span className="model-row__pending">
                    {t('settings.weknoraCloud.modelsSection.statusPending')}
                  </span>}
            </div>
          </div>)}
        </div>

        {credentialState === 'configured' && missingKinds.length > 1 ? <div className="models-section__batch">
          <TPopconfirm
            content={t('settings.weknoraCloud.modelsSection.confirmAddAll', { count: missingKinds.length })}
            confirmBtn={{ content: t('settings.weknoraCloud.modelsSection.addAllConfirm'), theme: 'primary' }}
            cancelBtn={{ content: t('common.cancel') }}
            placement="top"
            onConfirm={() => void addModels([...missingKinds])}
          >
            <TButton
              theme="primary"
              size="small"
              loading={addingModels && !addingKind}
              disabled={addingModels && !!addingKind}
            >
              {t('settings.weknoraCloud.modelsSection.addAllBtn', { count: missingKinds.length })}
            </TButton>
          </TPopconfirm>
        </div>
          : credentialState === 'configured' && missingKinds.length === 0 ? <p className="models-section__ready">
            <TIcon name="check-circle-filled" className="models-section__ready-icon" />
            {` ${t('settings.weknoraCloud.modelsSection.allReady')}`}
          </p> : null}
      </section>

      {/* 使用说明 */}
      <div className="usage-hint">
        <p className="hint-title">{t('settings.weknoraCloud.usageTitle')}</p>
        <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
      </div>
    </div>
  );
}
