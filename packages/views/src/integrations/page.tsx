import * as React from 'react';
import { useEffect, useRef, useState } from 'react';
import type { Locale } from '../../../i18n/src/index.ts';
import { INTEGRATION_SECTIONS, integrationSection, type IntegrationKey } from './registry.ts';
import { buildEmbedUpdatePayload } from './form.ts';
import { apiKeyAccessMode, apiKeyValueDisplay, isFreshKeyVisible, type ApiKeyRow } from './apiKeys.ts';
import { buildCLIConnectCommand } from './cli.ts';
import { integrationsLocale, integrationsT } from './messages.ts';
import { imPlatformLabel, imPlatformOrder, integrationSectionCopy } from './view.ts';
import {
  IM_WIZARD_STEPS,
  applyImPlatformChange,
  applyWeChatConfirmedCredentials,
  buildImCreatePayload,
  buildImUpdatePayload,
  createImWizardForm,
  imCallbackUrl,
  imConsoleLink,
  imCredentialFields,
  imPlatformSupportsThread,
  imWizardFormFromChannel,
  isWeChatBound,
  validateImWizardSave,
  validateImWizardStep,
  wechatQrImageUrl,
  type ImCredentialField,
  type ImWizardForm,
} from './imWizard.ts';

export interface IntegrationAgentOption { id: string; name: string }
export interface IntegrationKnowledgeBaseOption { id: string; name: string }

/** Vue WeChat QR ports (IMChannelPanel.vue lines 850-919) backed by
 *  client.embed.im.wechat in the route page. */
export interface IntegrationWeChatQrPorts {
  create: () => Promise<{ qrcodeUrl: string; qrcode: string }>;
  poll: (qrcode: string) => Promise<{ status: string; bot_token?: string; ilink_bot_id?: string; ilink_user_id?: string }>;
}

// Integrations surface ported to the Vue settings-drawer anatomy
// (frontend/src/views/integrations/IntegrationSettingsSection.vue):
// every tab renders a .section header (h2 + description, IM adds the
// 查看接入文档 doc link) and the channel panels keep the Vue list anatomy
// from IMChannelPanel.vue / AgentEmbedChannelPanel.vue: a "IM 渠道"-style
// count header, channel cards with platform badge / status tag / actions and
// a dashed 添加渠道 tile. All existing API wiring (embed channels preview,
// IM channel create/update/toggle/delete, API key create / one-time reveal /
// revoke, principal config, playground) is preserved.

export interface APIPrincipalConfig { mode: 'tenant' | 'direct_header' | 'signed_token'; direct_header_name: string; signed_token_header_name: string; require_direct_header: boolean; has_hmac_secret: boolean }

export interface IntegrationResource {
  id: string;
  name?: string;
  platform?: string;
  agent_id?: string;
  enabled?: boolean;
  [key: string]: unknown;
}

export interface IntegrationPrincipalToken { token: string; headerName: string; expiresInSeconds: number; externalUserId: string }
export interface IntegrationActions {
  onCreateEmbed?: (input: Record<string, unknown>) => Promise<void>;
  onUpdateEmbed?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onDeleteEmbed?: (id: string) => Promise<void>;
  onRotateEmbed?: (id: string) => Promise<void>;
  onCreateIm?: (input: { agentId: string; payload: Record<string, unknown> }) => Promise<void>;
  onUpdateIm?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onToggleIm?: (id: string) => Promise<void>;
  onDeleteIm?: (id: string) => Promise<void>;
  principal?: APIPrincipalConfig | null;
  onSavePrincipal?: (input: { mode: APIPrincipalConfig['mode']; requireDirectHeader: boolean; hmacSecret?: string }) => Promise<void>;
  onCreatePrincipalTestToken?: (externalUserId: string) => Promise<IntegrationPrincipalToken>;  onCreateApiKey?: (name: string) => Promise<ApiKeyRow>;  onRevokeApiKey?: (keyId: ApiKeyRow['id']) => Promise<void>;
  wechatQr?: IntegrationWeChatQrPorts;
}

export interface IntegrationsPageProps {
  embedded?: boolean;
  embedChannels: readonly IntegrationResource[];
  imChannels: readonly IntegrationResource[];
  apiBaseUrl: string;
  apiKeys?: readonly ApiKeyRow[];
  apiKeysLoading?: boolean;
  activeTab?: IntegrationKey;
  onTabChange?: (key: IntegrationKey) => void;
  initialTab?: IntegrationKey;
  loading?: boolean;
  error?: string;
  onReload?: () => void;
  onOpenEmbed?: (channel: IntegrationResource) => void;
  actions?: IntegrationActions;
  /** UI locale; defaults to the platform shell locale (localStorage 'locale'). */
  locale?: Locale;
  /** Vue lists agents for the bound-agent select (IMChannelPanel.vue agentOptions). */
  agents?: readonly IntegrationAgentOption[];
  /** Vue step-3 file-KB options (IMChannelPanel.vue knowledgeBases). */
  knowledgeBases?: readonly IntegrationKnowledgeBaseOption[];
}

function initialLocale(): Locale {
  try {
    const stored = window.localStorage.getItem('locale');
    return integrationsLocale(stored);
  } catch {
    return 'zh-CN';
  }
}

export function IntegrationsPage({ embedded = false, embedChannels, imChannels, apiBaseUrl, apiKeys = [], apiKeysLoading = false, activeTab, onTabChange, initialTab = 'embed', loading = false, error, onReload, onOpenEmbed, actions = {}, locale: localeProp, agents = [], knowledgeBases = [] }: IntegrationsPageProps) {
  const [locale, setLocale] = useState<Locale>(localeProp ?? initialLocale());
  useEffect(() => { if (localeProp) setLocale(localeProp); }, [localeProp]);
  const t = (key: string, values?: Record<string, string | number>) => integrationsT(locale, key, values);
  const [tab, setTabState] = useState<IntegrationKey>(initialTab);
  useEffect(() => { if (activeTab) setTabState(activeTab); }, [activeTab]);
  const setTab = (key: IntegrationKey) => { setTabState(key); onTabChange?.(key); };
  const [localError, setLocalError] = useState('');
  const [embedAgentId, setEmbedAgentId] = useState('');
  const [embedName, setEmbedName] = useState('');
  const [embedOrigins, setEmbedOrigins] = useState('https://example.com');
  const [showEmbedCreate, setShowEmbedCreate] = useState(false);
  // IM channel wizard state (Vue SettingDrawer): step, form, edit target and
  // the WeChat QR binding machine (idle -> wait -> scaned -> confirmed/expired).
  const [imWizardOpen, setImWizardOpen] = useState(false);
  const [imStep, setImStep] = useState(0);
  const [imForm, setImForm] = useState<ImWizardForm>(createImWizardForm());
  const [imNameTouched, setImNameTouched] = useState(false);
  const [imEditing, setImEditing] = useState<IntegrationResource | null>(null);
  const [imEditingEnabled, setImEditingEnabled] = useState(true);
  const [imWarning, setImWarning] = useState('');
  const [wechatQr, setWechatQr] = useState<{ imgSrc: string; code: string; status: string } | null>(null);
  const [wechatQrLoading, setWechatQrLoading] = useState(false);
  const [wechatQrError, setWechatQrError] = useState('');
  const wechatPollActive = useRef(false);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [editedNames, setEditedNames] = useState<Record<string, string>>({});
  const [principalMode, setPrincipalMode] = useState<APIPrincipalConfig['mode']>(actions.principal?.mode ?? 'tenant');
  const [requireDirectHeader, setRequireDirectHeader] = useState(actions.principal?.require_direct_header ?? false);
  const [hmacSecret, setHmacSecret] = useState('');
  const [externalUserId, setExternalUserId] = useState('playground-user');
  const [principalToken, setPrincipalToken] = useState<IntegrationPrincipalToken | null>(null);
  const [apiKey, setApiKey] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [playgroundPath, setPlaygroundPath] = useState('/api/v1/knowledge-chat/');
  const [playgroundBody, setPlaygroundBody] = useState('{\n  "query": "Hello from the API playground"\n}');
  const [playgroundOutput, setPlaygroundOutput] = useState('');
  const [freshApiKeyId, setFreshApiKeyId] = useState<ApiKeyRow['id'] | null>(null);
  const [newApiKeyName, setNewApiKeyName] = useState("");
  const [showApiKeyForm, setShowApiKeyForm] = useState(false);
  const [busy, setBusy] = useState(false);
  const section = INTEGRATION_SECTIONS.find((item) => item.key === tab)!;
  const copy = integrationSectionCopy(tab, locale);
  useEffect(() => { if (!actions.principal) return; setPrincipalMode(actions.principal.mode); setRequireDirectHeader(actions.principal.require_direct_header); }, [actions.principal]);
  const run = async (operation: () => Promise<void>) => { setBusy(true); setLocalError(''); try { await operation(); } catch (cause) { setLocalError(cause instanceof Error ? cause.message : 'Integration operation failed.'); } finally { setBusy(false); } };
  const createEmbed = () => run(async () => { if (!actions.onCreateEmbed) return; await actions.onCreateEmbed({ agent_id: embedAgentId.trim(), name: embedName.trim(), allowed_origins: embedOrigins.split(/[\n,]/).map((value) => value.trim()).filter(Boolean), enabled: true }); setEmbedName(''); setShowEmbedCreate(false); onReload?.(); });
  const stopWeChatPolling = () => { wechatPollActive.current = false; };
  useEffect(() => () => stopWeChatPolling(), []);
  const resetWeChatBinding = () => { stopWeChatPolling(); setWechatQr(null); setWechatQrLoading(false); setWechatQrError(''); };
  const closeImWizard = () => { resetWeChatBinding(); setImWizardOpen(false); setImEditing(null); setImWarning(''); };
  const openImCreate = () => {
    resetWeChatBinding();
    setImEditing(null);
    setImEditingEnabled(true);
    setImStep(0);
    setImNameTouched(false);
    setImWarning('');
    setImForm({ ...createImWizardForm(), targetAgentId: imForm.targetAgentId });
    setImWizardOpen(true);
  };
  // Vue openDrawer/editChannel: prefill from the channel and keep its name.
  const openImEdit = (channel: IntegrationResource) => {
    resetWeChatBinding();
    setImEditing(channel);
    setImEditingEnabled(channel.enabled !== false);
    setImStep(0);
    setImNameTouched(true);
    setImWarning('');
    setImForm(imWizardFormFromChannel(channel));
    setImWizardOpen(true);
  };
  const imPlatformPicked = (platform: string) => {
    resetWeChatBinding();
    setImForm((current) => applyImPlatformChange(current, platform as ImWizardForm['platform'], {
      channelNameTouched: imNameTouched,
      defaultNameFor: (key) => imPlatformLabel(key, locale),
    }));
  };
  const imNext = () => {
    const warning = validateImWizardStep(imForm, imStep);
    if (warning) { setImWarning(t(warning)); return; }
    setImWarning('');
    setImStep((step) => Math.min(step + 1, IM_WIZARD_STEPS.length - 1));
  };
  const imBack = () => { setImWarning(''); setImStep((step) => Math.max(step - 1, 0)); };
  // Vue pollOnce (lines 881-911): re-poll 500ms after each long-poll reply;
  // confirmed fills credentials, expired stops and shows the QR overlay.
  const pollWeChatStatus = async (code: string) => {
    const ports = actions.wechatQr;
    if (!ports) return;
    wechatPollActive.current = true;
    while (wechatPollActive.current) {
      let status = '';
      try {
        const result = await ports.poll(code);
        if (!wechatPollActive.current) return;
        status = result.status;
        if (status === 'confirmed') {
          setImForm((current) => ({ ...current, credentials: applyWeChatConfirmedCredentials(result) }));
          wechatPollActive.current = false;
          setWechatQr(null);
          return;
        }
        if (status === 'expired') { wechatPollActive.current = false; }
      } catch { /* transient network error: keep polling like Vue */ }
      setWechatQr((current) => (current && status ? { ...current, status } : current));
      if (!wechatPollActive.current) return;
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  };
  const startWeChatBinding = async () => {
    const ports = actions.wechatQr;
    stopWeChatPolling();
    setWechatQr(null);
    setWechatQrError('');
    if (!ports) { setWechatQrError('WeChat QR binding needs the wechatQr actions port (see IntegrationsRoutePage).'); return; }
    setWechatQrLoading(true);
    try {
      const created = await ports.create();
      setWechatQr({ imgSrc: wechatQrImageUrl(created.qrcodeUrl), code: created.qrcode, status: 'wait' });
      void pollWeChatStatus(created.qrcode);
    } catch (cause) {
      setWechatQrError(cause instanceof Error ? cause.message : 'Failed to generate QR code');
    } finally {
      setWechatQrLoading(false);
    }
  };
  const saveImWizard = () => run(async () => {
    if (imEditing) {
      if (!actions.onUpdateIm) return;
      const warning = validateImWizardSave(imForm);
      if (warning) { setImWarning(t(warning)); return; }
      await actions.onUpdateIm(imEditing.id, buildImUpdatePayload(imForm, imEditingEnabled, imPlatformLabel(imForm.platform, locale)));
    } else {
      if (!actions.onCreateIm) return;
      if (!imForm.targetAgentId) { setImWarning(t('integrations.selectAgentHint')); return; }
      const warning = validateImWizardSave(imForm);
      if (warning) { setImWarning(t(warning)); return; }
      await actions.onCreateIm({ agentId: imForm.targetAgentId, payload: buildImCreatePayload(imForm, imPlatformLabel(imForm.platform, locale)) });
    }
    closeImWizard();
    onReload?.();
  });
  const createApiKey = () => run(async () => { if (!actions.onCreateApiKey) return; const created = await actions.onCreateApiKey(newApiKeyName.trim()); setFreshApiKeyId(created.id); setNewApiKeyName(""); setShowApiKeyForm(false); });
  const revokeApiKey = (key: ApiKeyRow) => { if (!actions.onRevokeApiKey) return; if (!window.confirm(t('integrations.api.deleteApiKeyConfirm'))) return; void run(async () => { await actions.onRevokeApiKey?.(key.id); onReload?.(); }); };
  const savePrincipal = () => run(async () => { await actions.onSavePrincipal?.({ mode: principalMode, requireDirectHeader, ...(hmacSecret.trim() ? { hmacSecret: hmacSecret.trim() } : {}) }); setHmacSecret(''); });
  const createPrincipalToken = () => run(async () => { if (!actions.onCreatePrincipalTestToken) return; setPrincipalToken(await actions.onCreatePrincipalTestToken(externalUserId.trim())); });
  const runPlayground = () => run(async () => { const path = playgroundPath.trim().replace('{session_id}', encodeURIComponent(sessionId.trim())); if (!apiKey.trim()) throw new Error('Enter an API key for this request.'); let body: unknown; try { body = JSON.parse(playgroundBody); } catch { throw new Error('Request body must be valid JSON.'); } const headers: Record<string, string> = { 'Content-Type': 'application/json', Accept: 'text/event-stream', 'X-API-Key': apiKey.trim() }; if (principalToken) headers[principalToken.headerName] = principalToken.token; const response = await fetch(apiBaseUrl.replace(/\/$/, '') + path, { method: 'POST', headers, body: JSON.stringify(body) }); const text = await response.text(); if (!response.ok) throw new Error('HTTP ' + response.status + ': ' + text.slice(0, 500)); setPlaygroundOutput(text); });
  const startRename = (item: IntegrationResource) => { setRenaming(item.id); setRenameValue(editedNames[item.id] ?? item.name ?? ''); };
  const saveRename = (item: IntegrationResource) => run(async () => {
    // IM renames are handled by the wizard (Vue opens the same drawer for edit).
    if (tab === 'embed' && actions.onUpdateEmbed) { await actions.onUpdateEmbed(item.id, buildEmbedUpdatePayload(item, renameValue)); }
    setEditedNames((current) => ({ ...current, [item.id]: renameValue }));
    setRenaming(null);
    onReload?.();
  });
  const deleteChannel = (id: string) => { if (!window.confirm(copy.deleteConfirm)) return; run(async () => { if (tab === 'embed') await actions.onDeleteEmbed?.(id); else await actions.onDeleteIm?.(id); onReload?.(); }); };
  return (
    <main className="wk-integrations-page">
      {!embedded ? <><header className="wk-integrations-header">
        <div><h1>{t('integrations.title')}</h1><p className="wk-muted">{t('integrations.agentEditor.desc')}</p></div>
        {onReload ? <button className="wk-button" type="button" onClick={onReload}>{t('common.retry')}</button> : null}
      </header>
      <nav className="wk-integrations-tabs" aria-label={t('integrations.title')}>
        {INTEGRATION_SECTIONS.map((item) => <button type="button" key={item.key} className={item.key === tab ? 'is-active' : ''} onClick={() => setTab(item.key)}>{t('integrations.tabs.' + item.key)}</button>)}
      </nav></> : null}
      <section className="wk-integrations-panel">
        <div className="wk-int-section-header">
          <div className="wk-int-section-heading">
            <h2>{copy.heading}</h2>
            <p className="wk-int-section-desc">
              {copy.description}
              {copy.docLinkLabel && copy.docUrl ? <a className="wk-int-doc-link" href={copy.docUrl} target="_blank" rel="noreferrer noopener">{copy.docLinkLabel}<span className="wk-int-doc-icon" aria-hidden="true">↗</span></a> : null}
            </p>
          </div>
          {section.minRole === 'owner' ? <span className="wk-role-badge">Owner</span> : null}
        </div>
        {loading ? <p className="wk-status">{t('integrations.api.loading')}</p> : null}
        {error || localError ? <p className="wk-status wk-status-error" role="alert">{error || localError}</p> : null}
        {!loading && !error && (tab === 'im' || tab === 'embed') ? <ChannelListPanel
          variant={tab}
          copy={copy}
          locale={locale}
          items={tab === 'im' ? imChannels : embedChannels}
          showCreate={tab === 'im' ? imWizardOpen : showEmbedCreate}
          onToggleCreate={() => (tab === 'im' ? (imWizardOpen ? closeImWizard() : openImCreate()) : setShowEmbedCreate(!showEmbedCreate))}
          busy={busy}
          t={t}
          renamingId={renaming}
          renameValue={renameValue}
          onRenameValue={setRenameValue}
          onStartRename={startRename}
          onSaveRename={saveRename}
          onCancelRename={() => setRenaming(null)}
          // Vue makes the whole channel card clickable: embed opens the preview
          // session, IM opens the same wizard drawer used by create (editChannel).
          onOpenCard={tab === 'embed' ? onOpenEmbed : openImEdit}
          onToggle={tab === 'im' && actions.onToggleIm ? (id) => run(async () => { await actions.onToggleIm?.(id); onReload?.(); }) : undefined}
          onRotate={tab === 'embed' && actions.onRotateEmbed ? (id) => run(async () => { await actions.onRotateEmbed?.(id); }) : undefined}
          onDelete={actions.onDeleteEmbed || actions.onDeleteIm ? deleteChannel : undefined}
          imCreateSlot={tab === 'im' ? <ImWizardPanel
            locale={locale}
            t={t}
            apiBaseUrl={apiBaseUrl}
            agents={agents}
            knowledgeBases={knowledgeBases}
            form={imForm}
            onForm={setImForm}
            onPlatformPicked={imPlatformPicked}
            step={imStep}
            nameTouched={imNameTouched}
            onNameTouched={setImNameTouched}
            editing={imEditing}
            editingEnabled={imEditingEnabled}
            onEditingEnabled={setImEditingEnabled}
            warning={imWarning}
            wechatQr={wechatQr}
            wechatQrLoading={wechatQrLoading}
            wechatQrError={wechatQrError}
            onStartWeChatBinding={() => void startWeChatBinding()}
            busy={busy}
            canSubmit={Boolean(actions.onCreateIm || actions.onUpdateIm)}
            onNext={imNext}
            onBack={imBack}
            onSave={saveImWizard}
            onCancel={closeImWizard}
          /> : null}
          embedCreateSlot={tab === 'embed' ? <EmbedCreateForm
            t={t}
            agentId={embedAgentId}
            onAgentId={setEmbedAgentId}
            name={embedName}
            onName={setEmbedName}
            origins={embedOrigins}
            onOrigins={setEmbedOrigins}
            busy={busy}
            canSubmit={Boolean(actions.onCreateEmbed)}
            onSubmit={createEmbed}
            onCancel={() => setShowEmbedCreate(false)}
          /> : null}
        /> : null}
        {!loading && !error && tab === 'api' ? <ApiIntegrationPanel apiBaseUrl={apiBaseUrl} actions={actions} principalMode={principalMode} setPrincipalMode={setPrincipalMode} requireDirectHeader={requireDirectHeader} setRequireDirectHeader={setRequireDirectHeader} hmacSecret={hmacSecret} setHmacSecret={setHmacSecret} externalUserId={externalUserId} setExternalUserId={setExternalUserId} principalToken={principalToken} onSavePrincipal={savePrincipal} onCreatePrincipalToken={createPrincipalToken} apiKey={apiKey} setApiKey={setApiKey} sessionId={sessionId} setSessionId={setSessionId} playgroundPath={playgroundPath} setPlaygroundPath={setPlaygroundPath} playgroundBody={playgroundBody} setPlaygroundBody={setPlaygroundBody} playgroundOutput={playgroundOutput} onRunPlayground={runPlayground} busy={busy} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} freshApiKeyId={freshApiKeyId} newApiKeyName={newApiKeyName} setNewApiKeyName={setNewApiKeyName} showApiKeyForm={showApiKeyForm} setShowApiKeyForm={setShowApiKeyForm} onCreateApiKey={createApiKey} onRevokeApiKey={revokeApiKey} onCopyApiKey={(key) => { void navigator.clipboard.writeText(key.api_key).catch(() => undefined); }} t={t} /> : null}
        {!loading && !error && section.external ? <ExternalLandingPanel tab={tab} locale={locale} externalUrl={section.externalUrl} apiBaseUrl={apiBaseUrl} t={t} /> : null}
      </section>
    </main>
  );
}

type Translator = (key: string, values?: Record<string, string | number>) => string;

function ImWizardPanelLegacy({ locale, t, apiBaseUrl, agents = [], knowledgeBases = [], form, onForm, onPlatformPicked, step, nameTouched, onNameTouched, editing, editingEnabled, onEditingEnabled, warning, wechatQr, wechatQrLoading, wechatQrError, onStartWeChatBinding, busy, canSubmit, onNext, onBack, onSave, onCancel }: {
  locale: Locale; t: Translator; apiBaseUrl: string; agents: readonly IntegrationAgentOption[]; knowledgeBases: readonly IntegrationKnowledgeBaseOption[];
  form: ImWizardForm; onForm: (next: ImWizardForm) => void; onPlatformPicked: (platform: string) => void; step: number;
  nameTouched: boolean; onNameTouched: (touched: boolean) => void; editing: IntegrationResource | null; editingEnabled: boolean; onEditingEnabled: (enabled: boolean) => void;
  warning: string; wechatQr: { imgSrc: string; code: string; status: string } | null; wechatQrLoading: boolean; wechatQrError: string;
  onStartWeChatBinding: () => void; busy: boolean; canSubmit: boolean; onNext: () => void; onBack: () => void; onSave: () => void; onCancel: () => void;
}) {
  const update = <K extends keyof ImWizardForm>(key: K, value: ImWizardForm[K]) => onForm({ ...form, [key]: value });
  const fields = imCredentialFields(form.platform, form.mode);
  const consoleLink = imConsoleLink(form.platform);
  return <div className="wk-im-wizard" role="dialog" aria-label={t(editing ? 'agentEditor.im.editTitle' : 'agentEditor.im.createTitle')}>
    <div className="wk-im-wizard-steps">{IM_WIZARD_STEPS.map((item, index) => <span key={item.key} className={index === step ? 'is-active' : index < step ? 'is-complete' : ''}>{t(item.titleKey)}</span>)}</div>
    {step === 0 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.agentLabel')}<select value={form.targetAgentId} onChange={(event) => update('targetAgentId', event.target.value)} disabled={Boolean(editing)}><option value="">{t('agentEditor.im.selectAgent')}</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}</select></label><label>{t('agentEditor.im.nameLabel')}<input value={form.name} onChange={(event) => { onNameTouched(true); update('name', event.target.value); }} placeholder={t('agentEditor.im.namePlaceholder')} /></label><label>{t('agentEditor.im.platformLabel')}<select value={form.platform} onChange={(event) => onPlatformPicked(event.target.value)}>{imPlatformOrder().map((platform) => <option key={platform} value={platform}>{imPlatformLabel(platform, locale)}</option>)}</select></label>{nameTouched && !form.name.trim() ? <small className="wk-status-error">{t('agentEditor.im.nameRequired')}</small> : null}</div> : null}
    {step === 1 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.connectionMode')}<select value={form.mode} onChange={(event) => update('mode', event.target.value as ImWizardForm['mode'])}><option value="websocket">WebSocket</option><option value="webhook">Webhook</option><option value="longpoll">Long Poll</option></select></label><label>{t('agentEditor.im.outputMode')}<select value={form.outputMode} onChange={(event) => update('outputMode', event.target.value as ImWizardForm['outputMode'])}><option value="stream">Stream</option><option value="full">Full</option></select></label>{imPlatformSupportsThread(form.platform) ? <label>{t('agentEditor.im.sessionMode')}<select value={form.sessionMode} onChange={(event) => update('sessionMode', event.target.value as ImWizardForm['sessionMode'])}><option value="user">User</option><option value="thread">Thread</option></select></label> : null}</div> : null}
    {step === 2 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.knowledgeBaseLabel')}<select value={form.knowledgeBaseId} onChange={(event) => update('knowledgeBaseId', event.target.value)}><option value="">{t('agentEditor.im.noKnowledgeBase')}</option>{knowledgeBases.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}</select></label></div> : null}
    {step === 3 ? <div className="wk-im-wizard-fields">{editing ? <label className="wk-switch-row"><input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />{t('agentEditor.im.enabled')}</label> : null}{form.platform === 'wechat' ? <div className="wk-im-wechat-bind">{wechatQr ? <img src={wechatQr.imgSrc} alt={t('agentEditor.im.wechatQrAlt')} /> : null}<button className="wk-button" type="button" disabled={wechatQrLoading || busy} onClick={onStartWeChatBinding}>{wechatQrLoading ? t('common.loading') : t('agentEditor.im.wechatScanBind')}</button>{wechatQrError ? <p className="wk-status-error">{wechatQrError}</p> : null}</div> : fields.map((field) => <label key={field.key}>{field.label ?? t(field.labelKey ?? field.key)}<input type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'} value={String(form.credentials[field.key] ?? '')} min={field.min} max={field.max} placeholder={field.placeholder ?? (field.placeholderKey ? t(field.placeholderKey) : undefined)} onChange={(event) => onForm({ ...form, credentials: { ...form.credentials, [field.key]: field.type === 'number' ? Number(event.target.value) : event.target.value } })} />{field.hintKey ? <small className="wk-muted">{t(field.hintKey)}</small> : null}</label>)}{consoleLink ? <a href={consoleLink.url} target="_blank" rel="noreferrer noopener">{t(consoleLink.labelKey)}</a> : null}</div> : null}
    {warning ? <p className="wk-status-error" role="alert">{warning}</p> : null}<small className="wk-muted">{apiBaseUrl}</small><div className="wk-list-actions"><button className="wk-button" type="button" onClick={onCancel}>{t('common.cancel')}</button>{step > 0 ? <button className="wk-button" type="button" disabled={busy} onClick={onBack}>{t('common.previous')}</button> : null}{step < IM_WIZARD_STEPS.length - 1 ? <button className="wk-button wk-button--primary" type="button" disabled={busy} onClick={onNext}>{t('common.next')}</button> : <button className="wk-button wk-button--primary" type="button" disabled={busy || !canSubmit} onClick={onSave}>{t('common.save')}</button>}</div>
  </div>;
}

interface ChannelListCopy {
  heading: string;
  description: string;
  docLinkLabel?: string;
  docUrl?: string;
  channelsTitle: string;
  addTileLabel: string;
  emptyText: string;
  disabledLabel: string;
  unnamedLabel: string;
  defaultChannelName: string;
  deleteConfirm: string;
  createForm: 'im' | 'embed' | 'none';
}

function ChannelListPanel({ variant, copy, locale, items, showCreate, onToggleCreate, busy, t, renamingId, renameValue, onRenameValue, onStartRename, onSaveRename, onCancelRename, onOpenCard, onToggle, onRotate, onDelete, imCreateSlot, embedCreateSlot }: {
  variant: 'im' | 'embed';
  copy: ChannelListCopy;
  locale: Locale;
  items: readonly IntegrationResource[];
  showCreate: boolean;
  onToggleCreate: () => void;
  busy: boolean;
  t: Translator;
  renamingId: string | null;
  renameValue: string;
  onRenameValue: (value: string) => void;
  onStartRename: (item: IntegrationResource) => void;
  onSaveRename: (item: IntegrationResource) => void;
  onCancelRename: () => void;
  onOpenCard?: (item: IntegrationResource) => void;
  onToggle?: (id: string) => void;
  onRotate?: (id: string) => void;
  onDelete?: (id: string) => void;
  imCreateSlot?: React.ReactNode;
  embedCreateSlot?: React.ReactNode;
}) {
  return <div className="wk-channels-section">
    <div className="wk-channels-header">
      <span className="wk-channels-title">{copy.channelsTitle}</span>
      <span className="wk-channels-count">{items.length}</span>
    </div>
    {items.length === 0 && !showCreate ? <div className="wk-channels-empty"><p className="wk-status">{copy.emptyText}</p></div> : null}
    <div className="wk-channel-grid">
      {items.map((item) => {
        const platform = variant === 'im' && typeof item.platform === 'string' ? item.platform : '';
        const badgeText = platform ? imPlatformLabel(platform, locale).slice(0, 2) : '</>';
        const agentLine = typeof item.agent_name === 'string' && item.agent_name ? item.agent_name : typeof item.agent_id === 'string' && item.agent_id ? 'ID ' + item.agent_id : '';
        const name = editedNameOf(item) || item.name || copy.unnamedLabel;
        return <article className={onOpenCard ? 'wk-channel-card wk-channel-card--clickable' : 'wk-channel-card'} key={item.id} onClick={onOpenCard ? () => onOpenCard(item) : undefined}>
          <span className="wk-channel-card__badge" aria-hidden="true">{badgeText}</span>
          <div className="wk-channel-card__body">
            <div className="wk-channel-card__header">
              <h3 className="wk-channel-card__title">{name}</h3>
              {item.enabled === false ? <span className="wk-tag wk-tag--warning">{copy.disabledLabel}</span> : null}
            </div>
            {agentLine ? <span className="wk-channel-card__agent-name">{agentLine}</span> : null}
          </div>
          <div className="wk-channel-card__actions" onClick={(event) => event.stopPropagation()}>
            {onRotate ? <button className="wk-button wk-button--text" type="button" onClick={() => onRotate(item.id)}>{t('embedPublish.resetKeyTitle')}</button> : null}
            {onToggle ? <label className="wk-switch" title={item.enabled === false ? t('agentEditor.im.enabled') : copy.disabledLabel} onClick={(event) => event.stopPropagation()}>
              <input type="checkbox" role="switch" aria-label={t('agentEditor.im.enabled')} checked={item.enabled !== false} onChange={() => onToggle(item.id)} />
              <span className="wk-switch-knob" aria-hidden="true" />
            </label> : null}
            {/* Vue IM cards edit through the wizard drawer; embed keeps inline rename. */}
            {variant === 'embed' ? <button className="wk-button wk-button--text" type="button" onClick={() => onStartRename(item)}>{t('common.edit')}</button> : null}
            {onDelete ? <button className="wk-button wk-button--text wk-button--danger" type="button" onClick={() => onDelete(item.id)}>{t('common.delete')}</button> : null}
          </div>
        </article>;
      })}
      <button type="button" className="wk-channel-card wk-channel-card--add" onClick={onToggleCreate}>
        <span className="wk-channel-card__badge wk-channel-card__badge--add" aria-hidden="true">+</span>
        <div className="wk-channel-card__body">
          <div className="wk-channel-card__header">
            <span className="wk-channel-card__title">{copy.addTileLabel}</span>
          </div>
        </div>
      </button>
    </div>
    {showCreate ? imCreateSlot ?? embedCreateSlot : null}
  </div>;
}

function editedNameOf(item: IntegrationResource): string {
  return '';
}

// The IM wizard drawer (Vue IMChannelPanel.vue SettingDrawer, lines 72-579):
// 4 steps — basic / connection / file knowledge base / credentials — with the
// Vue step strip (active + done marks), footer Back / Next / Save buttons and
// the per-platform credential tables from imWizard.ts.
function ImWizardPanel({ locale, t, apiBaseUrl, agents = [], knowledgeBases = [], form, onForm, onPlatformPicked, step, nameTouched, onNameTouched, editing, editingEnabled, onEditingEnabled, warning, wechatQr, wechatQrLoading, wechatQrError, onStartWeChatBinding, busy, canSubmit, onNext, onBack, onSave, onCancel }: {
  locale: Locale;
  t: Translator;
  apiBaseUrl: string;
  agents?: readonly IntegrationAgentOption[];
  knowledgeBases?: readonly IntegrationKnowledgeBaseOption[];
  form: ImWizardForm;
  onForm: (form: ImWizardForm) => void;
  onPlatformPicked: (platform: string) => void;
  step: number;
  nameTouched: boolean;
  onNameTouched: (touched: boolean) => void;
  editing: IntegrationResource | null;
  editingEnabled: boolean;
  onEditingEnabled: (enabled: boolean) => void;
  warning: string;
  wechatQr: { imgSrc: string; code: string; status: string } | null;
  wechatQrLoading: boolean;
  wechatQrError: string;
  onStartWeChatBinding: () => void;
  busy: boolean;
  canSubmit: boolean;
  onNext: () => void;
  onBack: () => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  const isEditing = editing !== null;
  const consoleLink = imConsoleLink(form.platform);
  const patch = (values: Partial<ImWizardForm>) => onForm({ ...form, ...values });
  const chip = (active: boolean) => (active ? 'wk-option-chip wk-option-chip--active' : 'wk-option-chip');
  const submit = (event: React.FormEvent) => { event.preventDefault(); if (step < IM_WIZARD_STEPS.length - 1) onNext(); else onSave(); };
  const renderCredentialField = (item: ImCredentialField) => {
    if (item.type === 'switch') {
      return <label className="wk-check-row" key={item.key}>
        <input type="checkbox" checked={form.credentials[item.key] === true} onChange={(event) => patch({ credentials: { ...form.credentials, [item.key]: event.target.checked } })} />
        {item.labelKey ? t(item.labelKey) : item.label}
        {item.hintKey ? <span className="wk-muted">{t(item.hintKey)}</span> : null}
      </label>;
    }
    const value = form.credentials[item.key];
    const placeholder = item.placeholderKey ? t(item.placeholderKey) : item.placeholder;
    const hint = item.hintKey
      ? <span className="wk-muted">{t(item.hintKey)}{item.hintLink ? <a className="wk-int-doc-link" href={item.hintLink.url} target="_blank" rel="noreferrer noopener"> {t(item.hintLink.labelKey)}</a> : null}</span>
      : null;
    return <label key={item.key}>
      {item.labelKey ? t(item.labelKey) : item.label}{item.required ? <span aria-hidden="true"> *</span> : null}
      <input
        type={item.type === 'number' ? 'number' : item.type === 'password' ? 'password' : 'text'}
        value={typeof value === 'string' || typeof value === 'number' ? String(value) : ''}
        min={item.min}
        max={item.max}
        placeholder={placeholder}
        autoComplete="off"
        onChange={(event) => patch({ credentials: { ...form.credentials, [item.key]: item.type === 'number' ? (event.target.value === '' ? '' : Number(event.target.value)) : event.target.value } })}
      />
      {hint}
    </label>;
  };
  const bound = form.platform === 'wechat' && isWeChatBound(form.credentials);
  return <form className="wk-integration-form wk-channel-create" onSubmit={submit}>
    {/* Vue drawerTitle (lines 685-690). */}
    <h3>{isEditing ? (form.name.trim() || t('agentEditor.im.unnamed')) : t('agentEditor.im.addChannel')}</h3>
    <div className="wk-im-steps" role="list">
      {IM_WIZARD_STEPS.map((item, index) => (
        <span role="listitem" key={item.key} className={step === index ? 'wk-im-step is-active' : step > index ? 'wk-im-step is-done' : 'wk-im-step'}>
          <span className="wk-im-step-num" aria-hidden="true" style={step > index ? { background: '#eff4ff' } : step === index ? { background: '#2e6de6', color: '#fff', borderColor: '#2e6de6' } : undefined}>{step > index ? '✓' : index + 1}</span>
          <span className="wk-im-step-title">{t(item.titleKey)}</span>
        </span>
      ))}
    </div>
    {warning ? <p className="wk-status wk-status-error" role="alert">{warning}</p> : null}

    {step === 0 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('agentEditor.im.sectionChannel')}</legend>
      <label>{t('integrations.boundAgent')}
        {agents.length > 0
          ? <select required value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })}>
              <option value="" disabled>{t('integrations.selectAgentPlaceholder')}</option>
              {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
            </select>
          : <input required value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })} placeholder={t('integrations.selectAgentPlaceholder')} />}
      </label>
      <label>{t('agentEditor.im.platform')}
        {/* Vue disables the platform select while editing (line 114). */}
        <select value={form.platform} disabled={isEditing} onChange={(event) => onPlatformPicked(event.target.value)}>
          {imPlatformOrder().map((key) => <option key={key} value={key}>{imPlatformLabel(key, locale)}</option>)}
        </select>
      </label>
      <label>{t('agentEditor.im.channelName')}
        <input value={form.name} onFocus={() => onNameTouched(true)} onChange={(event) => { onNameTouched(true); patch({ name: event.target.value }); }} placeholder={t('agentEditor.im.channelNamePlaceholder')} />
      </label>
      {!isEditing ? <p className="wk-muted">{t('agentEditor.im.channelNameDefaultHint')}</p> : null}
      {isEditing ? <label className="wk-check-row">
        <input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />
        {t('agentEditor.im.enabled')}
      </label> : null}
    </fieldset> : null}

    {step === 1 ? <div className="wk-im-step-body">
      {/* Vue hides the access section for wechat (fixed longpoll/full, line 149). */}
      {form.platform !== 'wechat' ? <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionAccess')}</legend>
        <label>{t('agentEditor.im.mode')}
          <span className="wk-option-chips" role="radiogroup" aria-label={t('agentEditor.im.mode')}>
            <button type="button" role="radio" aria-checked={form.mode === 'websocket'} className={chip(form.mode === 'websocket')} disabled={form.platform === 'mattermost'} onClick={() => patch({ mode: 'websocket' })}>WebSocket</button>
            <button type="button" role="radio" aria-checked={form.mode === 'webhook'} className={chip(form.mode === 'webhook')} onClick={() => patch({ mode: 'webhook' })}>Webhook</button>
          </span>
        </label>
        <p className="wk-muted">{form.platform === 'mattermost' ? t('agentEditor.im.mattermostModeHint') : form.platform === 'yunzhijia' ? t('agentEditor.im.yunzhijiaModeHint') : t('agentEditor.im.modeHint')}</p>
        <label>{t('agentEditor.im.outputMode')}
          <span className="wk-option-chips" role="radiogroup" aria-label={t('agentEditor.im.outputMode')}>
            <button type="button" role="radio" aria-checked={form.outputMode === 'stream'} className={chip(form.outputMode === 'stream')} onClick={() => patch({ outputMode: 'stream' })}>{t('agentEditor.im.outputStream')}</button>
            <button type="button" role="radio" aria-checked={form.outputMode === 'full'} className={chip(form.outputMode === 'full')} onClick={() => patch({ outputMode: 'full' })}>{t('agentEditor.im.outputFull')}</button>
          </span>
        </label>
      </fieldset> : null}
      <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionSession')}</legend>
        <label>{t('agentEditor.im.sessionMode')}
          <span className="wk-option-chips" role="radiogroup" aria-label={t('agentEditor.im.sessionMode')}>
            <button type="button" role="radio" aria-checked={form.sessionMode === 'user'} className={chip(form.sessionMode === 'user')} onClick={() => patch({ sessionMode: 'user' })}>{t('agentEditor.im.sessionModeUser')}</button>
            <button type="button" role="radio" aria-checked={form.sessionMode === 'thread'} className={chip(form.sessionMode === 'thread')} disabled={!imPlatformSupportsThread(form.platform)} onClick={() => patch({ sessionMode: 'thread' })}>{t('agentEditor.im.sessionModeThread')}</button>
          </span>
        </label>
        <p className="wk-muted">{t('agentEditor.im.sessionModeHint')}</p>
      </fieldset>
      {isEditing && form.mode === 'webhook' ? <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionCallback')}</legend>
        <label>{t('agentEditor.im.callbackUrl')}
          <span className="wk-code-toolbar">
            <input className="wk-mono-input" readOnly value={imCallbackUrl(editing.id, apiBaseUrl)} />
            <button className="wk-button wk-button--text" type="button" title={t('integrations.api.copy')} onClick={() => { void navigator.clipboard.writeText(imCallbackUrl(editing.id, apiBaseUrl)).catch(() => undefined); }}>⧉</button>
          </span>
        </label>
      </fieldset> : null}
    </div> : null}

    {step === 2 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('agentEditor.im.sectionKnowledge')}</legend>
      <label>{t('agentEditor.im.fileKnowledgeBase')}
        <select value={form.knowledgeBaseId} onChange={(event) => patch({ knowledgeBaseId: event.target.value })}>
          <option value="">{t('agentEditor.im.fileKnowledgeBasePlaceholder')}</option>
          {knowledgeBases.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
        </select>
      </label>
      <p className="wk-muted">{t('agentEditor.im.fileKnowledgeBaseHint')}</p>
    </fieldset> : null}

    {step === 3 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('agentEditor.im.sectionCredentials')}</legend>
      {form.platform === 'wechat' ? <div>
        <p className="wk-muted">{t('agentEditor.im.wechatHint')}</p>
        {bound ? <p className="wk-status wk-status-ok" role="status">
          {t('agentEditor.im.wechatBindSuccess')}
          <button className="wk-button wk-button--text" type="button" onClick={onStartWeChatBinding}>{t('agentEditor.im.wechatRebind')}</button>
        </p> : wechatQr ? <div>
          <img src={wechatQr.imgSrc} alt="WeChat QR Code" width={200} height={200} style={{ background: '#fff' }} />
          {wechatQr.status === 'expired' ? <button className="wk-button" type="button" onClick={onStartWeChatBinding}>↻ {t('agentEditor.im.wechatQRExpired')}</button> : null}
          <p className="wk-muted">{wechatQr.status === 'scaned' ? t('agentEditor.im.wechatBinding') : t('agentEditor.im.wechatScanning')}</p>
        </div> : <div>
          <button className="wk-button" type="button" disabled={wechatQrLoading} onClick={onStartWeChatBinding}>{t('agentEditor.im.wechatScanBind')}</button>
        </div>}
        {wechatQrError ? <p className="wk-status wk-status-error" role="alert">{wechatQrError}</p> : null}
      </div> : <div>
        {consoleLink ? <p className="wk-muted">
          <a className="wk-int-doc-link" href={consoleLink.url} target="_blank" rel="noreferrer noopener">{t(consoleLink.labelKey)}</a>
          {' · '}{t('agentEditor.im.consoleTip')}
        </p> : null}
        {imCredentialFields(form.platform, form.mode).map(renderCredentialField)}
      </div>}
    </fieldset> : null}

    <div className="wk-form-actions">
      {step > 0 ? <button className="wk-button" type="button" onClick={onBack}>{t('integrations.wizard.back')}</button> : null}
      <button className="wk-button" type="submit" disabled={busy || !canSubmit}>{step < IM_WIZARD_STEPS.length - 1 ? t('integrations.wizard.next') : t('common.save')}</button>
      <button className="wk-button wk-button--text" type="button" onClick={onCancel}>{t('common.cancel')}</button>
    </div>
  </form>;
}

function EmbedCreateForm({ t, agentId, onAgentId, name, onName, origins, onOrigins, busy, canSubmit, onSubmit, onCancel }: {
  t: Translator;
  agentId: string;
  onAgentId: (value: string) => void;
  name: string;
  onName: (value: string) => void;
  origins: string;
  onOrigins: (value: string) => void;
  busy: boolean;
  canSubmit: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  return <form className="wk-integration-form wk-channel-create" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
    <h3>{t('embedPublish.create')}</h3>
    <label>{t('integrations.boundAgent')}<input required value={agentId} onChange={(event) => onAgentId(event.target.value)} placeholder="agent id" /></label>
    <label>{t('embedPublish.name')}<input value={name} onChange={(event) => onName(event.target.value)} placeholder={t('embedPublish.namePlaceholder')} /></label>
    <p className="wk-muted">{t('embedPublish.nameDefaultHint')}</p>
    <label>{t('embedPublish.allowedOrigins')}<input required value={origins} onChange={(event) => onOrigins(event.target.value)} placeholder={t('embedPublish.originsPlaceholder')} /></label>
    <div className="wk-form-actions">
      <button className="wk-button" type="submit" disabled={busy || !canSubmit}>{t('common.save')}</button>
      <button className="wk-button wk-button--text" type="button" onClick={onCancel}>{t('common.cancel')}</button>
    </div>
  </form>;
}


function ApiIntegrationPanel({ apiBaseUrl, actions, principalMode, setPrincipalMode, requireDirectHeader, setRequireDirectHeader, hmacSecret, setHmacSecret, externalUserId, setExternalUserId, principalToken, onSavePrincipal, onCreatePrincipalToken, apiKey, setApiKey, sessionId, setSessionId, playgroundPath, setPlaygroundPath, playgroundBody, setPlaygroundBody, playgroundOutput, onRunPlayground, busy, apiKeys, apiKeysLoading, freshApiKeyId, newApiKeyName, setNewApiKeyName, showApiKeyForm, setShowApiKeyForm, onCreateApiKey, onRevokeApiKey, onCopyApiKey, t }: {
  apiBaseUrl: string;
  actions: IntegrationActions;
  principalMode: APIPrincipalConfig['mode'];
  setPrincipalMode: (value: APIPrincipalConfig['mode']) => void;
  requireDirectHeader: boolean;
  setRequireDirectHeader: (value: boolean) => void;
  hmacSecret: string;
  setHmacSecret: (value: string) => void;
  externalUserId: string;
  setExternalUserId: (value: string) => void;
  principalToken: IntegrationPrincipalToken | null;
  onSavePrincipal: () => void;
  onCreatePrincipalToken: () => void;
  apiKey: string;
  setApiKey: (value: string) => void;
  sessionId: string;
  setSessionId: (value: string) => void;
  playgroundPath: string;
  setPlaygroundPath: (value: string) => void;
  playgroundBody: string;
  setPlaygroundBody: (value: string) => void;
  playgroundOutput: string;
  onRunPlayground: () => void;
  busy: boolean;
  apiKeys?: readonly ApiKeyRow[];
  apiKeysLoading?: boolean;
  freshApiKeyId?: ApiKeyRow['id'] | null;
  newApiKeyName?: string;
  setNewApiKeyName?: (value: string) => void;
  showApiKeyForm?: boolean;
  setShowApiKeyForm?: (value: boolean) => void;
  onCreateApiKey?: () => void;
  onRevokeApiKey?: (key: ApiKeyRow) => void;
  onCopyApiKey?: (key: ApiKeyRow) => void;
  t: Translator;
}) {
  const principal = actions.principal;
  return <div className="wk-api-integration">
    <section className="wk-api-band">
      <div className="wk-api-row">
        <div className="wk-api-row-info">
          <label>{t('integrations.api.baseUrl')}</label>
          <p>{t('integrations.api.baseUrlDesc')}</p>
        </div>
        <div className="wk-api-row-control">
          <input className="wk-mono-input" readOnly value={apiBaseUrl} aria-label={t('integrations.api.baseUrl')} />
          <button className="wk-button wk-button--text" type="button" title={t('integrations.api.copy')} onClick={() => { void navigator.clipboard.writeText(apiBaseUrl).catch(() => undefined); }}>{t('integrations.api.copy')}</button>
        </div>
      </div>
      <div className="wk-api-row">
        <div className="wk-api-row-info">
          <label>OpenAPI /docs</label>
          <p><a href={apiBaseUrl.replace(/\/+$/, '') + '/docs'} target="_blank" rel="noreferrer">{apiBaseUrl.replace(/\/+$/, '') + '/docs'}</a></p>
        </div>
      </div>
    </section>

    <section className="wk-api-keys">
      <div className="wk-api-keys-header">
        <div className="wk-api-keys-title">
          <label>{t('integrations.api.apiKeys')}</label>
          <p>{t('integrations.api.apiKeysDesc')}</p>
        </div>
        <button className="wk-button" type="button" onClick={() => setShowApiKeyForm?.(!showApiKeyForm)}>{t('integrations.api.createApiKey')}</button>
      </div>
      {freshApiKeyId !== null ? <p className="wk-status wk-status-ok" role="status">{t('integrations.api.apiKeyCreated')} · {t('integrations.api.secretSavedCopyHint')}</p> : null}
      {showApiKeyForm ? <form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); onCreateApiKey?.(); }}>
        <label>{t('integrations.api.apiKeyName')}<input required value={newApiKeyName ?? ''} onChange={(event) => setNewApiKeyName?.(event.target.value)} placeholder={t('integrations.api.apiKeyNamePlaceholder')} /></label>
        <div className="wk-form-actions">
          <button className="wk-button" type="submit" disabled={busy || !newApiKeyName?.trim()}>{t('integrations.api.createApiKey')}</button>
          <button className="wk-button wk-button--text" type="button" onClick={() => setShowApiKeyForm?.(false)}>{t('common.cancel')}</button>
        </div>
      </form> : null}
      {apiKeysLoading ? <p className="wk-status">{t('integrations.api.loading')}</p> : (apiKeys ?? []).length === 0 ? <p className="wk-status">{t('integrations.api.noApiKeys')}</p> : <div className="wk-api-key-table-wrap">
        <table className="wk-api-key-table">
          <thead><tr>
            <th>{t('integrations.api.apiKeyName')}</th>
            <th>{t('integrations.api.apiKeyValue')}</th>
            <th>{t('integrations.api.apiKeyAccessMode')}</th>
            <th>{t('integrations.api.createdAt')}</th>
            <th>{t('integrations.api.actions')}</th>
          </tr></thead>
          <tbody>
            {(apiKeys ?? []).map((key) => {
              const reveal = isFreshKeyVisible({ fresh: key.id === freshApiKeyId, hasValue: key.api_key !== '' });
              return <tr key={String(key.id)}>
                <td>{key.name}</td>
                <td><code>{apiKeyValueDisplay(key, reveal)}</code></td>
                <td>{apiKeyAccessMode(key)}</td>
                <td>{key.created_at ?? ''}</td>
                <td className="wk-api-key-actions">
                  {key.api_key ? <button className="wk-button" type="button" onClick={() => onCopyApiKey?.(key)}>{t('integrations.api.copy')}</button> : null}
                  <button className="wk-button wk-button--danger" type="button" onClick={() => onRevokeApiKey?.(key)}>{t('integrations.api.deleteApiKey')}</button>
                </td>
              </tr>;
            })}
          </tbody>
        </table>
      </div>}
    </section>

    <section className="wk-principal">
      <div className="wk-api-keys-header">
        <div className="wk-api-keys-title">
          <label>{t('integrations.api.principalMode')}</label>
          <p>{t('integrations.api.principalModeDesc')}</p>
        </div>
      </div>
      <p className="wk-muted">{t('integrations.api.principalScope')}</p>
      <div className="wk-option-chips" role="radiogroup" aria-label={t('integrations.api.principalMode')}>
        {([['tenant', 'integrations.api.modeTenant'], ['direct_header', 'integrations.api.modeDirect'], ['signed_token', 'integrations.api.modeSigned']] as const).map(([value, key]) => (
          <button key={value} type="button" role="radio" aria-checked={principalMode === value} className={principalMode === value ? 'wk-option-chip wk-option-chip--active' : 'wk-option-chip'} onClick={() => setPrincipalMode(value)}>{t(key)}</button>
        ))}
      </div>
      {principalMode === 'direct_header' ? <div className="wk-principal-fields">
        <p className="wk-muted wk-muted--warn">{t('integrations.api.directWarning')}</p>
        <label className="wk-check-row"><input type="checkbox" checked={requireDirectHeader} onChange={(event) => setRequireDirectHeader(event.target.checked)} />{t('integrations.api.requireDirectHeader')}</label>
        <p className="wk-muted">{t('integrations.api.requireDirectHeaderDesc')}</p>
      </div> : null}
      {principalMode === 'signed_token' ? <div className="wk-principal-fields">
        <label>{t('integrations.api.hmacSecret')}<input type="password" value={hmacSecret} onChange={(event) => setHmacSecret(event.target.value)} placeholder={principal?.has_hmac_secret ? t('integrations.api.secretConfigured') : ''} /></label>
        <p className="wk-muted">{t('integrations.api.hmacSecretDesc')}</p>
      </div> : null}
      {principalMode !== 'tenant' ? <div className="wk-form-actions">
        <button className="wk-button" type="button" disabled={busy} onClick={onSavePrincipal}>{t('common.save')}</button>
      </div> : null}
      <div className="wk-principal-fields">
        <label>{t('integrations.api.playgroundExternalUser')}<input value={externalUserId} onChange={(event) => setExternalUserId(event.target.value)} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} /></label>
        <div className="wk-form-actions">
          <button className="wk-button" type="button" disabled={busy} onClick={onCreatePrincipalToken}>{t('integrations.api.generateSecret')}</button>
        </div>
        {principalToken ? <p className="wk-status">{t('integrations.api.playgroundGeneratedToken')}: <code>{principalToken.token}</code> ({principalToken.headerName})</p> : null}
      </div>
    </section>

    <section className="wk-playground">
      <div className="wk-api-keys-header">
        <div className="wk-api-keys-title">
          <label>{t('integrations.api.playgroundTitle')}</label>
          <p>{t('integrations.api.playgroundDesc')}</p>
        </div>
      </div>
      <form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); onRunPlayground(); }}>
        <label>Session ID<input value={sessionId} onChange={(event) => setSessionId(event.target.value)} /></label>
        <label>Path<input value={playgroundPath} onChange={(event) => setPlaygroundPath(event.target.value)} /></label>
        <label>X-API-Key<input type="password" value={apiKey} onChange={(event) => setApiKey(event.target.value)} /></label>
        <label>Body<textarea rows={4} value={playgroundBody} onChange={(event) => setPlaygroundBody(event.target.value)} /></label>
        <div className="wk-form-actions">
          <button className="wk-button" type="submit" disabled={busy}>{t('integrations.api.playgroundRun')}</button>
        </div>
      </form>
      {playgroundOutput ? <pre className="wk-api-output">{playgroundOutput}</pre> : null}
    </section>
  </div>;
}

function ExternalLandingPanel({ tab, locale, externalUrl, apiBaseUrl, t }: { tab: IntegrationKey; locale: Locale; externalUrl?: string; apiBaseUrl: string; t: Translator }) {
  const cta = tab === 'cli'
    ? { label: t('integrations.cli.docs'), hint: t('integrations.cli.docsHint') }
    : tab === 'chrome'
      ? { label: t('integrations.chrome.installCta'), hint: t('integrations.chrome.installCtaHint') }
      : { label: t('integrations.claw.installCta'), hint: t('integrations.claw.installCtaHint') };
  const copy = (value: string) => { void navigator.clipboard.writeText(value).catch(() => undefined); };
  const cliConnectCommand = buildCLIConnectCommand(apiBaseUrl, typeof window === 'undefined' ? '' : window.location.origin);
  const cliSteps = [
    { key: 'install', title: t('integrations.cli.installTitle'), desc: t('integrations.cli.installDesc'), command: 'git clone https://github.com/Tencent/WeKnora.git\ncd WeKnora/cli\ngo build -o weknora .\nexport PATH="$PWD:$PATH"' },
    { key: 'connect', title: t('integrations.cli.connectTitle'), desc: t('integrations.cli.connectDesc'), command: cliConnectCommand },
    { key: 'verify', title: t('integrations.cli.verifyTitle'), desc: t('integrations.cli.verifyDesc'), command: 'weknora doctor\nweknora kb list' },
  ];
  const chromeCapabilities = ['shortcuts', 'notes', 'clip', 'qa'] as const;
  const chromeSteps = ['connect', 'install', 'port', 'api'] as const;
  const clawCapabilities = ['browse', 'search', 'manual', 'url', 'upload'] as const;
  const clawSteps = ['verify', 'install', 'env', 'api'] as const;
  return <div className="wk-int-landing">
    <div className="wk-int-landing-cta">
      {externalUrl ? <a className="wk-button" href={externalUrl} target="_blank" rel="noreferrer noopener">{cta.label}</a> : null}
      <span className="wk-muted">{cta.hint}</span>
    </div>
    {tab === 'cli' ? <section className="wk-landing-section">
      <h4>{t('integrations.cli.quickstart')}</h4>
      <ol className="wk-landing-steps">
        {cliSteps.map((step, index) => <li key={step.key} className="wk-landing-step">
          <span className="wk-landing-step-num">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="wk-landing-step-title">{step.title}</div>
            <p className="wk-landing-step-desc">{step.desc}</p>
            <div className="wk-code-toolbar"><pre>{step.command}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy(step.command)}>⧉</button></div>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {tab === 'cli' ? <section className="wk-landing-section">
      <h4>{t('integrations.cli.commandsTitle')}</h4>
      <p className="wk-muted">{t('integrations.cli.commandsDesc')}</p>
      <div className="wk-code-toolbar"><pre>{'weknora doc upload ./document.pdf --kb "KB_ID"\nweknora search chunks "query" --kb "KB_ID"\nweknora chat "question" --kb "KB_ID" --format text\nweknora agent list'}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy('weknora doc upload')}>⧉</button></div>
    </section> : null}
    {tab === 'cli' ? <section className="wk-landing-section">
      <h4>{t('integrations.cli.mcpTitle')}</h4>
      <p className="wk-muted">{t('integrations.cli.mcpDesc')}</p>
      <div className="wk-code-toolbar"><pre>{JSON.stringify({ mcpServers: { weknora: { command: 'weknora', args: ['--profile', 'weknora', 'mcp', 'serve'] } } }, null, 2)}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy('mcp')}>⧉</button></div>
    </section> : null}
    {tab === 'chrome' ? <section className="wk-landing-section">
      <h4>{t('integrations.chrome.capabilitiesTitle')}</h4>
      <div className="wk-capability-grid">
        {chromeCapabilities.map((key) => <div key={key} className="wk-capability-card">
          <h5>{t('integrations.chrome.capabilities.' + key + '.title')}</h5>
          <p>{t('integrations.chrome.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {tab === 'chrome' ? <section className="wk-landing-section">
      <h4>{t('integrations.chrome.stepsTitle')}</h4>
      <ol className="wk-landing-steps">
        {chromeSteps.map((key, index) => <li key={key} className="wk-landing-step">
          <span className="wk-landing-step-num">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="wk-landing-step-title">{t('integrations.chrome.steps.' + key + '.title')}</div>
            <p className="wk-landing-step-desc">{t('integrations.chrome.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {tab === 'claw' ? <section className="wk-landing-section">
      <h4>{t('integrations.claw.capabilitiesTitle')}</h4>
      <div className="wk-capability-grid">
        {clawCapabilities.map((key) => <div key={key} className="wk-capability-card">
          <h5>{t('integrations.claw.capabilities.' + key + '.title')}</h5>
          <p>{t('integrations.claw.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {tab === 'claw' ? <section className="wk-landing-section">
      <h4>{t('integrations.claw.stepsTitle')}</h4>
      <ol className="wk-landing-steps">
        {clawSteps.map((key, index) => <li key={key} className="wk-landing-step">
          <span className="wk-landing-step-num">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="wk-landing-step-title">{t('integrations.claw.steps.' + key + '.title')}</div>
            <p className="wk-landing-step-desc">{t('integrations.claw.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
  </div>;
}
