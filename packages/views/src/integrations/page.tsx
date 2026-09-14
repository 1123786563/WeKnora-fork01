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
import {
  buildEmbedWizardPayload,
  createEmbedWizardForm,
  embedChannelKeyDisplay,
  embedOriginsWarning,
  embedIframeSnippet,
  embedOriginsTextFromChannel,
  embedSecureServerGoExample,
  embedSecureServerNodeExample,
  embedSecureWidgetSnippet,
  embedSnippetScenarioKey,
  embedWidgetSnippet,
  embedChannelUrl,
  embedWizardFormFromChannel,
  embedWizardSteps,
  parseEmbedAllowedOrigins,
  validateEmbedAllowedOrigins,
  validateEmbedWizardStep,
  type EmbedWizardForm,
  type EmbedWizardStep,
} from './embedWizard.ts';

/** Vue CustomAgent config flags drive the capability warnings of step 3. */
export interface IntegrationAgentOption { id: string; name: string; config?: Record<string, unknown> }
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
  /** Vue createEmbedChannel; resolves with the created channel (publish_token included). */
  onCreateEmbed?: (input: { agentId: string; payload: Record<string, unknown> }) => Promise<IntegrationResource | void>;
  onUpdateEmbed?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onDeleteEmbed?: (id: string) => Promise<void>;
  onRotateEmbed?: (id: string) => Promise<void>;
  /** Vue EmbedChannelPreview obtains a short-lived preview session token. */
  onPreviewSession?: (id: string) => Promise<string>;
  /** Vue openDrawer's getEmbedChannel refresh (publish_token / has_webhook_secret). */
  onEmbedDetail?: (id: string) => Promise<IntegrationResource | null>;
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
  onOpenApiPlayground?: () => void;
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

export function IntegrationsPage({ embedded = false, embedChannels, imChannels, apiBaseUrl, apiKeys = [], apiKeysLoading = false, activeTab, onTabChange, initialTab = 'embed', loading = false, error, onReload, onOpenEmbed, onOpenApiPlayground, actions = {}, locale: localeProp, agents = [], knowledgeBases = [] }: IntegrationsPageProps) {
  const [locale, setLocale] = useState<Locale>(localeProp ?? initialLocale());
  useEffect(() => { if (localeProp) setLocale(localeProp); }, [localeProp]);
  const t = (key: string, values?: Record<string, string | number>) => integrationsT(locale, key, values);
  const [tab, setTabState] = useState<IntegrationKey>(initialTab);
  useEffect(() => { if (activeTab) setTabState(activeTab); }, [activeTab]);
  const setTab = (key: IntegrationKey) => { setTabState(key); onTabChange?.(key); };
  const [localError, setLocalError] = useState('');
  // Embed channel wizard state (Vue SettingDrawer AgentEmbedChannelPanel.vue):
  // 5 create steps + the edit-only deploy step, origins textarea and key reveal.
  const [embedWizardOpen, setEmbedWizardOpen] = useState(false);
  const [embedStep, setEmbedStep] = useState(0);
  const [embedForm, setEmbedForm] = useState<EmbedWizardForm>(createEmbedWizardForm());
  const [embedOriginsText, setEmbedOriginsText] = useState('');
  const [embedNameTouched, setEmbedNameTouched] = useState(false);
  const [embedEditing, setEmbedEditing] = useState<IntegrationResource | null>(null);
  const [embedDetail, setEmbedDetail] = useState<IntegrationResource | null>(null);
  const [embedEditingEnabled, setEmbedEditingEnabled] = useState(true);
  const [embedWarning, setEmbedWarning] = useState('');
  const [embedStatus, setEmbedStatus] = useState('');
  const [embedSnippetTab, setEmbedSnippetTab] = useState<'iframe' | 'widget' | 'secure'>('iframe');
  const [embedServerTab, setEmbedServerTab] = useState<'node' | 'go'>('node');
  const [revealedKeys, setRevealedKeys] = useState<Record<string, boolean>>({});
  const [embedPreviewLoading, setEmbedPreviewLoading] = useState(false);
  const [embedPreview, setEmbedPreview] = useState<{ channel: IntegrationResource; token: string; mode: 'iframe' | 'widget' } | null>(null);
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
  // --- Embed wizard handlers (Vue AgentEmbedChannelPanel.vue script) ---
  const embedSteps = embedWizardSteps(embedEditing !== null);
  // Vue defaultEmbedChannelName (lines 578-584): "{agent} · 网页嵌入" fallback.
  const embedDefaultChannelName = (agentId: string): string => {
    const agent = agents.find((item) => item.id === agentId);
    if (agent?.name?.trim()) return t('embedPublish.defaultChannelNameWithAgent', { agent: agent.name.trim() });
    return t('embedPublish.defaultChannelName');
  };
  const embedResolvedName = (form: EmbedWizardForm): string => form.name.trim() || embedDefaultChannelName(form.agentId);
  const embedDrawerTitle = embedEditing
    ? (embedForm.name.trim() || embedDefaultChannelName(String(embedForm.agentId || embedEditing.agent_id || '')))
    : t('embedPublish.createTitle');
  const closeEmbedWizard = () => { setEmbedWizardOpen(false); setEmbedWarning(''); setEmbedStatus(''); };
  useEffect(() => {
    if (!imWizardOpen && !embedWizardOpen) return undefined;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (embedWizardOpen) closeEmbedWizard();
      else setImWizardOpen(false);
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [embedWizardOpen, imWizardOpen]);
  // Vue openCreate (lines 839-852).
  const openEmbedCreate = () => {
    setEmbedEditing(null);
    setEmbedDetail(null);
    setEmbedEditingEnabled(true);
    setEmbedStep(0);
    setEmbedNameTouched(false);
    setEmbedWarning('');
    setEmbedStatus('');
    setEmbedSnippetTab('iframe');
    setEmbedServerTab('node');
    setEmbedForm(createEmbedWizardForm());
    setEmbedOriginsText('');
    setEmbedWizardOpen(true);
  };
  // Vue openDrawer (lines 854-867): prefill, land on deploy, refresh detail.
  const openEmbedEdit = (channel: IntegrationResource) => {
    setEmbedEditing(channel);
    setEmbedDetail(channel);
    setEmbedEditingEnabled(channel.enabled !== false);
    setEmbedNameTouched(true);
    setEmbedWarning('');
    setEmbedStatus('');
    setEmbedSnippetTab('iframe');
    setEmbedServerTab('node');
    setEmbedForm(embedWizardFormFromChannel(channel as Record<string, unknown>));
    setEmbedOriginsText(embedOriginsTextFromChannel(channel as { allowed_origins?: unknown }));
    setEmbedStep(embedWizardSteps(true).length - 1);
    setEmbedWizardOpen(true);
    if (actions.onEmbedDetail) {
      void actions.onEmbedDetail(channel.id)
        .then((detail) => { if (detail) setEmbedDetail(detail); })
        .catch(() => setEmbedWarning(t('embedPublish.channelKeyLoadFailed')));
    }
  };
  // Vue watch [createAgentId, agents] -> applyDefaultChannelNameIfNeeded (L595-599, L720-722).
  const embedAgentPicked = (agentId: string) => {
    setEmbedForm((current) => {
      const next = { ...current, agentId };
      if (!embedEditing && !embedNameTouched) next.name = embedDefaultChannelName(agentId);
      return next;
    });
  };
  const embedNext = () => {
    const warning = validateEmbedWizardStep(embedForm, embedOriginsText, embedStep);
    if (warning) { setEmbedWarning(t(warning.key, warning.values)); return; }
    setEmbedWarning('');
    setEmbedStep((step) => Math.min(step + 1, embedSteps.length - 1));
  };
  const embedBack = () => { setEmbedWarning(''); setEmbedStep((step) => Math.max(step - 1, 0)); };
  // Vue goToWizardStep (lines 640-643): the step strip is freely clickable.
  const embedGoTo = (step: number) => {
    if (step < 0 || step >= embedSteps.length) return;
    setEmbedWarning('');
    setEmbedStep(step);
  };
  // Vue saveForm (lines 896-982).
  const saveEmbedWizard = () => run(async () => {
    if (!embedForm.agentId) { setEmbedWarning(t('integrations.selectAgentHint')); return; }
    const originsValidation = validateEmbedAllowedOrigins(parseEmbedAllowedOrigins(embedOriginsText));
    if (!originsValidation.ok) {
      const warning = embedOriginsWarning(originsValidation.error);
      setEmbedWarning(t(warning.key, warning.values));
      return;
    }
    setEmbedWarning('');
    const payload = buildEmbedWizardPayload(embedForm, {
      origins: originsValidation.origins,
      defaultName: embedResolvedName(embedForm),
      enabled: embedEditing ? embedEditingEnabled : true,
    });
    if (embedEditing) {
      if (!actions.onUpdateEmbed) return;
      await actions.onUpdateEmbed(embedEditing.id, payload);
      setEmbedStatus(t('embedPublish.updated'));
      const detail = actions.onEmbedDetail ? await actions.onEmbedDetail(embedEditing.id).catch(() => null) : null;
      const merged = detail ?? { ...embedEditing, ...payload };
      setEmbedDetail(merged);
      setEmbedForm(embedWizardFormFromChannel(merged as Record<string, unknown>));
      setEmbedOriginsText(embedOriginsTextFromChannel(merged as { allowed_origins?: unknown }));
    } else {
      if (!actions.onCreateEmbed) return;
      const created = await actions.onCreateEmbed({ agentId: embedForm.agentId, payload });
      setEmbedStatus(t(created?.publish_token ? 'embedPublish.createdWithToken' : 'embedPublish.created'));
      if (created) {
        setEmbedEditing(created);
        setEmbedDetail(created);
        setEmbedForm(embedWizardFormFromChannel(created as Record<string, unknown>));
        setEmbedOriginsText(embedOriginsTextFromChannel(created as { allowed_origins?: unknown }));
        if (created.publish_token) setRevealedKeys((current) => ({ ...current, [created.id]: true }));
        // Vue line 966: a successful create jumps to the deploy step.
        setEmbedStep(embedWizardSteps(true).length - 1);
      }
    }
    onReload?.();
  });
  // Vue performRotate (lines 1046-1063) through the route page port.
  const rotateEmbedKey = (channelId: string) => {
    if (!actions.onRotateEmbed) return;
    if (!window.confirm(t('embedPublish.resetKeyConfirmBody'))) return;
    void run(async () => {
      await actions.onRotateEmbed?.(channelId);
      const detail = actions.onEmbedDetail ? await actions.onEmbedDetail(channelId).catch(() => null) : null;
      if (detail) {
        setEmbedDetail(detail);
        setRevealedKeys((current) => ({ ...current, [channelId]: true }));
        setEmbedStatus(t('embedPublish.resetKeySuccess'));
      } else {
        setEmbedStatus(t('embedPublish.resetKeyFailed'));
      }
      onReload?.();
    });
  };
  const previewEmbedChannel = (channel: IntegrationResource) => {
    setEmbedPreviewLoading(true);
    void Promise.resolve()
      .then(async () => {
        const storedToken = typeof channel.publish_token === 'string' ? channel.publish_token : '';
        const token = storedToken || (actions.onPreviewSession ? await actions.onPreviewSession(channel.id) : '');
        if (!token) {
          onOpenEmbed?.(channel);
          return;
        }
        setEmbedPreview({ channel, token, mode: embedSnippetTab === 'widget' ? 'widget' : 'iframe' });
      })
      .catch(() => setEmbedWarning(t('embedPublish.previewUnavailable')))
      .finally(() => setEmbedPreviewLoading(false));
  };
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
    <>
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
          showCreate={tab === 'im' ? imWizardOpen : embedWizardOpen}
          onToggleCreate={() => (tab === 'im' ? (imWizardOpen ? closeImWizard() : openImCreate()) : (embedWizardOpen ? closeEmbedWizard() : openEmbedCreate()))}
          busy={busy}
          t={t}
          renamingId={renaming}
          renameValue={renameValue}
          onRenameValue={setRenameValue}
          onStartRename={startRename}
          onSaveRename={saveRename}
          onCancelRename={() => setRenaming(null)}
          // Vue makes the whole channel card clickable: both tabs open the same
          // wizard drawer used by create (AgentEmbedChannelPanel openDrawer L854).
          onOpenCard={tab === 'embed' ? openEmbedEdit : openImEdit}
          onToggle={tab === 'im' && actions.onToggleIm ? (id) => run(async () => { await actions.onToggleIm?.(id); onReload?.(); }) : undefined}
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
          embedCreateSlot={tab === 'embed' ? <EmbedWizardPanel
            t={t}
            apiBaseUrl={apiBaseUrl}
            agents={agents}
            title={embedDrawerTitle}
            form={embedForm}
            onForm={setEmbedForm}
            onAgentPicked={embedAgentPicked}
            step={embedStep}
            steps={embedSteps}
            originsText={embedOriginsText}
            onOriginsText={setEmbedOriginsText}
            onNameTouched={setEmbedNameTouched}
            editing={embedEditing}
            detail={embedDetail}
            editingEnabled={embedEditingEnabled}
            onEditingEnabled={setEmbedEditingEnabled}
            warning={embedWarning}
            status={embedStatus}
            snippetTab={embedSnippetTab}
            onSnippetTab={setEmbedSnippetTab}
            serverTab={embedServerTab}
            onServerTab={setEmbedServerTab}
            revealed={embedEditing ? revealedKeys[embedEditing.id] === true : false}
            onReveal={() => { if (embedEditing) setRevealedKeys((current) => ({ ...current, [embedEditing.id]: !(current[embedEditing.id] === true) })); }}
            onRotate={rotateEmbedKey}
            previewLoading={embedPreviewLoading}
            onPreview={previewEmbedChannel}
            busy={busy}
            canSubmit={Boolean(actions.onCreateEmbed || actions.onUpdateEmbed)}
            onNext={embedNext}
            onBack={embedBack}
            onGoTo={embedGoTo}
            onSave={saveEmbedWizard}
            onCancel={closeEmbedWizard}
          /> : null}
        /> : null}
        {!loading && !error && tab === 'api' ? <ApiIntegrationPanel apiBaseUrl={apiBaseUrl} actions={actions} principalMode={principalMode} setPrincipalMode={setPrincipalMode} requireDirectHeader={requireDirectHeader} setRequireDirectHeader={setRequireDirectHeader} hmacSecret={hmacSecret} setHmacSecret={setHmacSecret} externalUserId={externalUserId} setExternalUserId={setExternalUserId} principalToken={principalToken} onSavePrincipal={savePrincipal} onCreatePrincipalToken={createPrincipalToken} apiKey={apiKey} setApiKey={setApiKey} sessionId={sessionId} setSessionId={setSessionId} playgroundPath={playgroundPath} setPlaygroundPath={setPlaygroundPath} playgroundBody={playgroundBody} setPlaygroundBody={setPlaygroundBody} playgroundOutput={playgroundOutput} onRunPlayground={runPlayground} busy={busy} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} freshApiKeyId={freshApiKeyId} newApiKeyName={newApiKeyName} setNewApiKeyName={setNewApiKeyName} showApiKeyForm={showApiKeyForm} setShowApiKeyForm={setShowApiKeyForm} onCreateApiKey={createApiKey} onRevokeApiKey={revokeApiKey} onCopyApiKey={(key) => { void navigator.clipboard.writeText(key.api_key).catch(() => undefined); }} onOpenApiPlayground={onOpenApiPlayground} t={t} /> : null}
        {!loading && !error && section.external ? <ExternalLandingPanel tab={tab} locale={locale} externalUrl={section.externalUrl} apiBaseUrl={apiBaseUrl} t={t} /> : null}
      </section>
    </main>
    {embedPreview ? <EmbedChannelPreviewPanel preview={embedPreview} locale={locale} t={t} onClose={() => setEmbedPreview(null)} /> : null}
    </>
  );
}

type Translator = (key: string, values?: Record<string, string | number>) => string;

function EmbedChannelPreviewPanel({ preview, locale, t, onClose }: { preview: { channel: IntegrationResource; token: string; mode: 'iframe' | 'widget' }; locale?: string; t: Translator; onClose: () => void }) {
  const [ready, setReady] = useState(false);
  const [widgetOpen, setWidgetOpen] = useState(true);
  const channelId = encodeURIComponent(preview.channel.id);
  // Vue bumps r=N per open so re-previews reload instead of serving the
  // cached iframe; the nonce doubles as the deferred-mount gate (Vue waits
  // for the drawer layout before mounting so embed autosize never reads 0).
  const [refreshKey] = useState(() => Date.now());
  const [layoutReady] = useState(true);
  const src = embedChannelUrl(preview.channel.id, preview.token, { locale, refreshKey });
  useEffect(() => { setReady(false); setWidgetOpen(true); }, [preview.channel.id, preview.token, preview.mode]);
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [onClose]);
  return <div className="wk-embed-preview-overlay" role="presentation" onClick={onClose}>
    <aside className="wk-embed-preview-drawer" role="dialog" aria-modal="true" aria-label={preview.channel.name || t('embedPublish.preview')} onClick={(event) => event.stopPropagation()}>
      <header className="wk-embed-preview-header"><h2>{preview.channel.name || t('embedPublish.preview')}</h2><button type="button" className="wk-integration-drawer-close" aria-label="关闭" title="关闭" onClick={onClose}>×</button></header>
      <div className="wk-embed-preview-body">
        <p className="wk-embed-preview-hint">{t(preview.mode === 'iframe' ? 'embedPublish.previewIframeHint' : 'embedPublish.previewWidgetHint')}</p>
        {preview.mode === 'iframe' ? <div className="wk-embed-preview-device">
          <div className="wk-embed-preview-chrome"><span>●</span><span>●</span><span>●</span><code>/embed/{channelId}</code></div>
          <div className="wk-embed-preview-screen">{!ready ? <span className="wk-muted">{t('embedPublish.previewLoading')}</span> : null}{layoutReady ? <iframe title={preview.channel.name || t('embedPublish.preview')} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'is-loading'} allow="clipboard-write" /> : null}</div>
        </div> : <div className="wk-embed-preview-widget">
          <div className="wk-embed-preview-mock-page"><strong>{t('embedPublish.previewMockPage')}</strong><span /><span className="short" /></div>
          {widgetOpen && layoutReady ? <div className="wk-embed-preview-widget-panel"><iframe title={preview.channel.name || t('embedPublish.preview')} src={src} onLoad={() => setReady(true)} allow="clipboard-write" /></div> : null}
          <button type="button" className="wk-embed-preview-launcher" style={{ background: typeof preview.channel.primary_color === 'string' ? preview.channel.primary_color : '#07c05f' }} onClick={() => setWidgetOpen((open) => !open)} aria-label={widgetOpen ? '关闭' : t('embedPublish.preview')}>{widgetOpen ? '×' : '◔'}</button>
        </div>}
      </div>
    </aside>
  </div>;
}

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

function ChannelListPanel({ variant, copy, locale, items, showCreate, onToggleCreate, busy, t, renamingId, renameValue, onRenameValue, onStartRename, onSaveRename, onCancelRename, onOpenCard, onToggle, onDelete, imCreateSlot, embedCreateSlot }: {
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
            {onToggle ? <label className="wk-switch" title={item.enabled === false ? t('agentEditor.im.enabled') : copy.disabledLabel} onClick={(event) => event.stopPropagation()}>
              <input type="checkbox" role="switch" aria-label={t('agentEditor.im.enabled')} checked={item.enabled !== false} onChange={() => onToggle(item.id)} />
              <span className="wk-switch-knob" aria-hidden="true" />
            </label> : null}
            {/* Vue edits both channel kinds through the wizard drawer opened by
                the card click, so the card keeps only the switch and delete. */}
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
    {showCreate ? <div className="wk-integration-drawer-overlay" role="presentation" onClick={onToggleCreate}>
      <aside className="wk-integration-drawer" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
        <button className="wk-integration-drawer-close" type="button" aria-label="关闭" title="关闭" onClick={onToggleCreate}>×</button>
        {imCreateSlot ?? embedCreateSlot}
      </aside>
    </div> : null}
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
      {/* Vue gates the bound agent via validateWizardStep (warning toast), not
          native required validation — keep the same semantics here. */}
      <label>{t('integrations.boundAgent')}
        {agents.length > 0
          ? <select value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })}>
              <option value="" disabled>{t('integrations.selectAgentPlaceholder')}</option>
              {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
            </select>
          : <input value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })} placeholder={t('integrations.selectAgentPlaceholder')} />}
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

// The embed wizard drawer (Vue AgentEmbedChannelPanel.vue SettingDrawer,
// lines 70-386): 渠道 → 安全限流 → 对话能力 → 外观展示 → 事件回调, plus the
// edit-only 部署 step with snippet tabs, server examples and the channel key
// controls. Copy follows the embedPublish.* verbatim fallback layer.
function EmbedWizardPanel({ t, apiBaseUrl, agents = [], title, form, onForm, onAgentPicked, step, steps, originsText, onOriginsText, onNameTouched, editing, detail, editingEnabled, onEditingEnabled, warning, status, snippetTab, onSnippetTab, serverTab, onServerTab, revealed, onReveal, onRotate, previewLoading, onPreview, busy, canSubmit, onNext, onBack, onGoTo, onSave, onCancel }: {
  t: Translator;
  apiBaseUrl: string;
  agents?: readonly IntegrationAgentOption[];
  title: string;
  form: EmbedWizardForm;
  onForm: (form: EmbedWizardForm) => void;
  onAgentPicked: (agentId: string) => void;
  step: number;
  steps: readonly EmbedWizardStep[];
  originsText: string;
  onOriginsText: (value: string) => void;
  onNameTouched: (touched: boolean) => void;
  editing: IntegrationResource | null;
  detail: IntegrationResource | null;
  editingEnabled: boolean;
  onEditingEnabled: (enabled: boolean) => void;
  warning: string;
  status: string;
  snippetTab: 'iframe' | 'widget' | 'secure';
  onSnippetTab: (tab: 'iframe' | 'widget' | 'secure') => void;
  serverTab: 'node' | 'go';
  onServerTab: (tab: 'node' | 'go') => void;
  revealed: boolean;
  onReveal: () => void;
  onRotate: (channelId: string) => void;
  previewLoading: boolean;
  onPreview: (channel: IntegrationResource) => void;
  busy: boolean;
  canSubmit: boolean;
  onNext: () => void;
  onBack: () => void;
  onGoTo: (step: number) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  const isEditing = editing !== null;
  const submit = (event: React.FormEvent) => { event.preventDefault(); if (step < steps.length - 1) onNext(); else onSave(); };
  const patch = (values: Partial<EmbedWizardForm>) => onForm({ ...form, ...values });
  const chip = (active: boolean) => (active ? 'wk-option-chip wk-option-chip--active' : 'wk-option-chip');
  const channel = detail ?? editing;
  const channelId = channel && typeof channel.id === 'string' ? channel.id : '';
  const token = channel && typeof channel.publish_token === 'string' ? channel.publish_token : '';
  const hasWebhookSecret = channel?.has_webhook_secret === true;
  // Vue drawerSnippet reads the saved channel (drawerChannel), not the draft form.
  const snippetPosition = ((typeof channel?.widget_position === 'string' && channel.widget_position) || form.widgetPosition) || 'bottom-right';
  const snippetColor = (typeof channel?.primary_color === 'string' && channel.primary_color) || form.primaryColor;
  const snippetTitle = (typeof channel?.page_title === 'string' && channel.page_title)
    || (typeof channel?.name === 'string' && channel.name)
    || form.pageTitle.trim() || form.name.trim() || '';
  const snippetBase = { primaryColor: snippetColor || undefined, title: snippetTitle || undefined, position: snippetPosition };
  const tokenlessSnippet = '<!-- ' + t('embedPublish.tokenHint') + ' -->';
  const snippet = snippetTab === 'secure'
    ? embedSecureWidgetSnippet(channelId, apiBaseUrl, snippetBase)
    : snippetTab === 'widget'
      ? (token ? embedWidgetSnippet(channelId, token, apiBaseUrl, snippetBase) : tokenlessSnippet)
      : (token ? embedIframeSnippet(channelId, token, apiBaseUrl) : tokenlessSnippet);
  const serverExample = serverTab === 'go'
    ? embedSecureServerGoExample(channelId, apiBaseUrl)
    : embedSecureServerNodeExample(channelId, apiBaseUrl);
  // Vue agentWebSearchEnabledEffective / agentImageUploadEnabledEffective (lines 484-490).
  const drawerAgent = agents.find((agent) => agent.id === form.agentId);
  const agentWebSearchEnabled = drawerAgent?.config?.web_search_enabled === true;
  const agentImageUploadEnabled = drawerAgent?.config?.image_upload_enabled === true;
  const secretPlaceholder = hasWebhookSecret ? t('embedPublish.webhookSecretKeep') : t('embedPublish.webhookSecretPlaceholder');
  return <form className="wk-integration-form wk-channel-create wk-embed-wizard" onSubmit={submit}>
    {/* Vue drawerTitle (lines 567-576). */}
    <h3>{title}</h3>
    <div className="wk-im-steps" role="list">
      {steps.map((item, index) => (
        <button role="listitem" key={item.key} type="button" className={step === index ? 'wk-embed-step is-active' : step > index ? 'wk-embed-step is-done' : 'wk-embed-step'} onClick={() => onGoTo(index)}>
          <span className="wk-im-step-num" aria-hidden="true" style={step > index ? { background: '#eff4ff' } : step === index ? { background: '#2e6de6', color: '#fff', borderColor: '#2e6de6' } : undefined}>{step > index ? '✓' : index + 1}</span>
          <span className="wk-im-step-title">{t(item.titleKey)}</span>
        </button>
      ))}
    </div>
    {warning ? <p className="wk-status wk-status-error" role="alert">{warning}</p> : null}
    {status ? <p className="wk-status wk-status-ok" role="status">{status}</p> : null}

    {step === 0 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionChannel')}</legend>
      {/* Vue gates the bound agent via validateWizardStep (warning), not native required. */}
      <label>{t('integrations.boundAgent')}
        {agents.length > 0
          ? <select value={form.agentId} onChange={(event) => onAgentPicked(event.target.value)}>
              <option value="">{t('integrations.selectAgentPlaceholder')}</option>
              {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
            </select>
          : <input value={form.agentId} onChange={(event) => onAgentPicked(event.target.value)} placeholder={t('integrations.selectAgentPlaceholder')} />}
      </label>
      {isEditing ? <label className="wk-check-row">
        <input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />
        {t('embedPublish.enabled')}
      </label> : null}
      <label>{t('embedPublish.name')}
        <input value={form.name} onFocus={() => onNameTouched(true)} onChange={(event) => { onNameTouched(true); patch({ name: event.target.value }); }} placeholder={t('embedPublish.namePlaceholder')} />
      </label>
      <p className="wk-muted">{isEditing ? t('embedPublish.nameDesc') : t('embedPublish.nameDefaultHint')}</p>
    </fieldset> : null}

    {step === 1 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionSecurity')}</legend>
      <label>{t('embedPublish.allowedOrigins')}
        <textarea rows={2} value={originsText} onChange={(event) => onOriginsText(event.target.value)} placeholder={t('embedPublish.originsPlaceholder')} />
      </label>
      <p className="wk-muted">{t('embedPublish.originsHint')}</p>
      <label>{t('embedPublish.rateLimitLabel')}
        <input type="number" min={1} max={600} value={form.rateLimitPerMinute} onChange={(event) => { const next = Number(event.target.value); if (Number.isFinite(next)) patch({ rateLimitPerMinute: next }); }} />
      </label>
      <p className="wk-muted">{t('embedPublish.rateLimitDesc')}</p>
      <label>{t('embedPublish.rateLimitDayLabel')}
        <input type="number" min={1} max={1000000} value={form.rateLimitPerDay} onChange={(event) => { const next = Number(event.target.value); if (Number.isFinite(next)) patch({ rateLimitPerDay: next }); }} />
      </label>
      <p className="wk-muted">{t('embedPublish.rateLimitDayDesc')}</p>
    </fieldset> : null}

    {step === 2 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionCapabilities')}</legend>
      <label>{t('embedPublish.welcomeMessage')}
        <textarea rows={2} value={form.welcomeMessage} onChange={(event) => patch({ welcomeMessage: event.target.value })} placeholder={t('embedPublish.welcomePlaceholder')} />
      </label>
      <p className="wk-muted">{t('embedPublish.welcomeMessageDesc')}</p>
      <label className="wk-check-row">
        <input type="checkbox" checked={form.showSuggestedQuestions} onChange={(event) => patch({ showSuggestedQuestions: event.target.checked })} />
        <span>{t('embedPublish.showSuggestedQuestions')}<br />{t('embedPublish.showSuggestedQuestionsDesc')}</span>
      </label>
      <label className="wk-check-row">
        <input type="checkbox" checked={form.allowWebSearch} onChange={(event) => patch({ allowWebSearch: event.target.checked })} />
        <span>{t('embedPublish.allowWebSearch')}<br />{t('embedPublish.allowWebSearchDesc')}</span>
      </label>
      {form.allowWebSearch && !agentWebSearchEnabled ? <p className="wk-muted wk-muted--warn">{t('embedPublish.agentWebSearchDisabledHint')}</p> : null}
      <label className="wk-check-row">
        <input type="checkbox" checked={form.allowFileUpload} onChange={(event) => patch({ allowFileUpload: event.target.checked })} />
        <span>{t('embedPublish.allowFileUpload')}<br />{t('embedPublish.allowFileUploadDesc')}</span>
      </label>
      {form.allowFileUpload && !agentImageUploadEnabled ? <p className="wk-muted wk-muted--warn">{t('embedPublish.agentImageUploadDisabledHint')}</p> : null}
    </fieldset> : null}

    {step === 3 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionAppearance')}</legend>
      <label>{t('embedPublish.pageTitle')}
        <input value={form.pageTitle} onChange={(event) => patch({ pageTitle: event.target.value })} placeholder={t('embedPublish.pageTitlePlaceholder')} />
      </label>
      <p className="wk-muted">{t('embedPublish.pageTitleDesc')}</p>
      <label>{t('embedPublish.headerTitleMode')}
        <select value={form.headerTitleMode} onChange={(event) => patch({ headerTitleMode: event.target.value as EmbedWizardForm['headerTitleMode'] })}>
          <option value="channel">{t('embedPublish.headerTitleModeChannel')}</option>
          <option value="session">{t('embedPublish.headerTitleModeSession')}</option>
        </select>
      </label>
      <p className="wk-muted">{t('embedPublish.headerTitleModeDesc')}</p>
      <label>{t('embedPublish.widgetPosition')}
        <select value={form.widgetPosition} onChange={(event) => patch({ widgetPosition: event.target.value as EmbedWizardForm['widgetPosition'] })}>
          <option value="bottom-right">{t('embedPublish.positionBottomRight')}</option>
          <option value="bottom-left">{t('embedPublish.positionBottomLeft')}</option>
          <option value="top-right">{t('embedPublish.positionTopRight')}</option>
          <option value="top-left">{t('embedPublish.positionTopLeft')}</option>
        </select>
      </label>
      <label>{t('embedPublish.defaultLocale')}
        <select value={form.defaultLocale} onChange={(event) => patch({ defaultLocale: event.target.value as EmbedWizardForm['defaultLocale'] })}>
          <option value="">{t('embedPublish.defaultLocaleBrowser')}</option>
          <option value="zh-CN">简体中文</option>
          <option value="en-US">English</option>
          <option value="ko-KR">한국어</option>
          <option value="ja-JP">日本語</option>
          <option value="ru-RU">Русский</option>
        </select>
      </label>
      <p className="wk-muted">{t('embedPublish.defaultLocaleDesc')}</p>
      <label>{t('embedPublish.primaryColor')}
        <input type="color" value={form.primaryColor} onChange={(event) => patch({ primaryColor: event.target.value })} />
      </label>
      <label>{t('embedPublish.widgetPreview')}
        <span className={'wk-embed-widget-preview pos-' + form.widgetPosition}>
          <span className="wk-embed-preview-launcher" style={{ background: form.primaryColor }} aria-hidden="true" />
        </span>
      </label>
    </fieldset> : null}

    {step === 4 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionWebhook')}</legend>
      <label>{t('embedPublish.webhookUrl')}
        <input autoComplete="off" value={form.webhookUrl} onChange={(event) => patch({ webhookUrl: event.target.value })} placeholder={t('embedPublish.webhookUrlPlaceholder')} />
      </label>
      <p className="wk-muted">{t('embedPublish.webhookUrlDesc')}</p>
      <label>{t('embedPublish.webhookSecret')}
        <input type="password" autoComplete="new-password" value={form.webhookSecret} onChange={(event) => patch({ webhookSecret: event.target.value })} placeholder={secretPlaceholder} />
      </label>
      <p className="wk-muted">{t('embedPublish.webhookSecretDesc')}</p>
    </fieldset> : null}
    {/* Vue renders the deploy-after-save hint inside step 5 for create mode. */}
    {step === 4 && !isEditing ? <div className="wk-embed-deploy-hint" role="note">ℹ️<p>{t('embedPublish.deployAfterSaveHint')}</p></div> : null}

    {/* Step 6 exists only while editing (Vue template v-else-if="editingId"). */}
    {step >= 5 && channel ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionDeploy')}</legend>
      <p className="wk-muted">{t('embedPublish.deployIntro')}</p>
      <h5>{t('embedPublish.deployStepEmbed')}</h5>
      <p className="wk-muted">{t('embedPublish.deployStepEmbedDesc')}</p>
      <div className="wk-embed-snippet-tabs wk-option-chips" role="tablist" aria-label={t('embedPublish.deployStepEmbed')}>
        {([['iframe', 'embedPublish.tabIframe'], ['widget', 'embedPublish.tabWidget'], ['secure', 'embedPublish.tabSecure']] as const).map(([value, key]) => (
          <button key={value} type="button" role="tab" aria-selected={snippetTab === value} className={chip(snippetTab === value)} onClick={() => onSnippetTab(value)}>{t(key)}</button>
        ))}
      </div>
      <p className="wk-muted">{t(embedSnippetScenarioKey(snippetTab))}</p>
      {snippetTab === 'widget' ? <p className="wk-muted">{t('embedPublish.widgetTokenNote')}</p> : null}
      {snippetTab === 'secure' ? <p className="wk-muted">{t('embedPublish.secureTokenNote')}</p> : null}
      {snippetTab !== 'secure' ? <div className="wk-embed-deploy-hint" role="note">⚠️<p>{t('embedPublish.publishTokenWarning')}</p></div> : null}
      <div className="wk-embed-code-panel">
        <div className="wk-code-toolbar">
          <span>{snippetTab === 'iframe' ? t('embedPublish.embedCode') : t('embedPublish.widgetCode')}</span>
          <span>
            {snippetTab !== 'secure' ? <button className="wk-button wk-button--text" type="button" disabled={previewLoading || busy} onClick={() => onPreview(channel)}>{previewLoading ? t('common.loading') : t('embedPublish.preview')}</button> : null}
            <button className="wk-button wk-button--text" type="button" onClick={() => { void navigator.clipboard.writeText(snippet).catch(() => undefined); }}>{t('embedPublish.copyCode')}</button>
          </span>
        </div>
        <pre>{snippet}</pre>
      </div>
      {snippetTab === 'secure' ? <div>
        <p className="wk-muted">{t('embedPublish.secureServerLabel')}</p>
        <div className="wk-embed-server-tabs wk-option-chips" role="tablist" aria-label={t('embedPublish.secureServerLabel')}>
          {([['node', 'embedPublish.tabServerNode'], ['go', 'embedPublish.tabServerGo']] as const).map(([value, key]) => (
            <button key={value} type="button" role="tab" aria-selected={serverTab === value} className={chip(serverTab === value)} onClick={() => onServerTab(value)}>{t(key)}</button>
          ))}
        </div>
        <div className="wk-embed-server-panel">
          <div className="wk-code-toolbar">
            <span>{serverTab === 'go' ? t('embedPublish.tabServerGo') : t('embedPublish.tabServerNode')}</span>
            <button className="wk-button wk-button--text" type="button" onClick={() => { void navigator.clipboard.writeText(serverExample).catch(() => undefined); }}>{t('embedPublish.copyCode')}</button>
          </div>
          <pre>{serverExample}</pre>
        </div>
      </div> : null}
      <h5>{t('embedPublish.channelKey')}</h5>
      <p className="wk-muted">{t('embedPublish.channelKeyDesc')}</p>
      <div className="wk-channel-key-control">
        <input className="wk-mono-input wk-embed-key-input" readOnly type="text" value={embedChannelKeyDisplay(token, revealed)} placeholder={token ? '' : t('embedPublish.channelKeyUnavailable')} aria-label={t('embedPublish.channelKey')} />
        {token ? <button className="wk-button wk-button--text" type="button" title={revealed ? t('embedPublish.hideKey') : t('embedPublish.revealKey')} onClick={onReveal}>{revealed ? '🙈' : '👁'}</button> : null}
        {token ? <button className="wk-button wk-button--text" type="button" title={t('embedPublish.copyChannelKeyTitle')} onClick={() => { void navigator.clipboard.writeText(token).catch(() => undefined); }}>⧉</button> : null}
        {canSubmit ? <button className="wk-button wk-button--text wk-button--danger" type="button" title={t('embedPublish.resetKeyTitle')} disabled={busy} onClick={() => onRotate(channelId)}>{busy ? t('common.loading') : '↻'}</button> : null}
      </div>
      {!token ? <p className="wk-muted">{t('embedPublish.channelKeyHint')}</p> : null}
    </fieldset> : null}

    <div className="wk-form-actions">
      {step > 0 ? <button className="wk-button" type="button" onClick={onBack}>{t('integrations.wizard.back')}</button> : null}
      <button className="wk-button" type="submit" disabled={busy || !canSubmit}>{step < steps.length - 1 ? t('integrations.wizard.next') : t('common.save')}</button>
      <button className="wk-button wk-button--text" type="button" onClick={onCancel}>{t('common.cancel')}</button>
    </div>
  </form>;
}


function ApiIntegrationPanel({ apiBaseUrl, actions, principalMode, setPrincipalMode, requireDirectHeader, setRequireDirectHeader, hmacSecret, setHmacSecret, externalUserId, setExternalUserId, principalToken, onSavePrincipal, onCreatePrincipalToken, apiKey, setApiKey, sessionId, setSessionId, playgroundPath, setPlaygroundPath, playgroundBody, setPlaygroundBody, playgroundOutput, onRunPlayground, busy, apiKeys, apiKeysLoading, freshApiKeyId, newApiKeyName, setNewApiKeyName, showApiKeyForm, setShowApiKeyForm, onCreateApiKey, onRevokeApiKey, onCopyApiKey, onOpenApiPlayground, t }: {
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
  onOpenApiPlayground?: () => void;
  t: Translator;
}) {
  const principal = actions.principal;
  return <div className="grid gap-5">
    <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <div className="flex items-center justify-between gap-4 py-[0.35rem] max-[720px]:flex-col max-[720px]:items-start">
        <div>
          <label className="block font-semibold text-ink">{t('integrations.api.baseUrl')}</label>
          <p className="m-0 mt-[0.15rem] text-[13px] text-muted-strong">{t('integrations.api.baseUrlDesc')}</p>
        </div>
        <div className="flex min-w-0 flex-nowrap items-center gap-[0.4rem]">
          <input className="wk-mono-input" readOnly value={apiBaseUrl} aria-label={t('integrations.api.baseUrl')} />
          <button className="wk-button wk-button--text shrink-0 whitespace-nowrap" type="button" title={t('integrations.api.copy')} onClick={() => { void navigator.clipboard.writeText(apiBaseUrl).catch(() => undefined); }}>{t('integrations.api.copy')}</button>
        </div>
      </div>
      <div className="flex items-center justify-between gap-4 border-t border-[#f2f5fa] py-[0.35rem] max-[720px]:flex-col max-[720px]:items-start">
        <div>
          <label className="block font-semibold text-ink">OpenAPI /docs</label>
          <p className="m-0 mt-[0.15rem] text-[13px] text-muted-strong"><a href={apiBaseUrl.replace(/\/+$/, '') + '/docs'} target="_blank" rel="noreferrer">{apiBaseUrl.replace(/\/+$/, '') + '/docs'}</a></p>
        </div>
      </div>
    </section>

    <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <div className="mb-[0.75rem] flex items-start justify-between gap-4">
        <div>
          <label className="block font-semibold text-ink">{t('integrations.api.apiKeys')}</label>
          <p className="m-0 mt-[0.15rem] text-[13px] text-muted-strong">{t('integrations.api.apiKeysDesc')}</p>
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
      {apiKeysLoading ? <p className="wk-status">{t('integrations.api.loading')}</p> : (apiKeys ?? []).length === 0 ? <p className="wk-status">{t('integrations.api.noApiKeys')}</p> : <div className="overflow-x-auto">
        <table className="w-full border-collapse text-[13px]">
          <thead><tr>
            <th className="whitespace-nowrap border-b border-[#eef1f5] px-[0.6rem] py-[0.45rem] text-left text-[12px] font-medium text-muted">{t('integrations.api.apiKeyName')}</th>
            <th className="whitespace-nowrap border-b border-[#eef1f5] px-[0.6rem] py-[0.45rem] text-left text-[12px] font-medium text-muted">{t('integrations.api.apiKeyValue')}</th>
            <th className="whitespace-nowrap border-b border-[#eef1f5] px-[0.6rem] py-[0.45rem] text-left text-[12px] font-medium text-muted">{t('integrations.api.apiKeyAccessMode')}</th>
            <th className="whitespace-nowrap border-b border-[#eef1f5] px-[0.6rem] py-[0.45rem] text-left text-[12px] font-medium text-muted">{t('integrations.api.createdAt')}</th>
            <th className="whitespace-nowrap border-b border-[#eef1f5] px-[0.6rem] py-[0.45rem] text-left text-[12px] font-medium text-muted">{t('integrations.api.actions')}</th>
          </tr></thead>
          <tbody>
            {(apiKeys ?? []).map((key) => {
              const reveal = isFreshKeyVisible({ fresh: key.id === freshApiKeyId, hasValue: key.api_key !== '' });
              return <tr key={String(key.id)}>
                <td className="border-b border-[#f2f5fa] px-[0.6rem] py-[0.5rem] [overflow-wrap:anywhere]">{key.name}</td>
                <td className="border-b border-[#f2f5fa] px-[0.6rem] py-[0.5rem] [overflow-wrap:anywhere]"><code className="rounded-[5px] bg-canvas px-[0.4rem] py-[0.15rem] text-[12px]">{apiKeyValueDisplay(key, reveal)}</code></td>
                <td className="border-b border-[#f2f5fa] px-[0.6rem] py-[0.5rem] [overflow-wrap:anywhere]">{apiKeyAccessMode(key)}</td>
                <td className="border-b border-[#f2f5fa] px-[0.6rem] py-[0.5rem] [overflow-wrap:anywhere]">{key.created_at ?? ''}</td>
                <td className="flex items-center gap-[0.4rem] whitespace-nowrap border-b border-[#f2f5fa] px-[0.6rem] py-[0.5rem] [overflow-wrap:anywhere]">
                  {key.api_key ? <button className="wk-button" type="button" onClick={() => onCopyApiKey?.(key)}>{t('integrations.api.copy')}</button> : null}
                  <button className="wk-button wk-button--danger" type="button" onClick={() => onRevokeApiKey?.(key)}>{t('integrations.api.deleteApiKey')}</button>
                </td>
              </tr>;
            })}
          </tbody>
        </table>
      </div>}
    </section>

    <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <div className="mb-[0.75rem] flex items-start justify-between gap-4">
        <div>
          <label className="block font-semibold text-ink">{t('integrations.api.principalMode')}</label>
          <p className="m-0 mt-[0.15rem] text-[13px] text-muted-strong">{t('integrations.api.principalModeDesc')}</p>
        </div>
      </div>
      <p className="wk-muted">{t('integrations.api.principalScope')}</p>
      <div className="wk-option-chips" role="radiogroup" aria-label={t('integrations.api.principalMode')}>
        {([['tenant', 'integrations.api.modeTenant'], ['direct_header', 'integrations.api.modeDirect'], ['signed_token', 'integrations.api.modeSigned']] as const).map(([value, key]) => (
          <button key={value} type="button" role="radio" aria-checked={principalMode === value} className={principalMode === value ? 'wk-option-chip wk-option-chip--active' : 'wk-option-chip'} onClick={() => setPrincipalMode(value)}>{t(key)}</button>
        ))}
      </div>
      {principalMode === 'direct_header' ? <div className="mt-[0.6rem] grid gap-[0.5rem]">
        <p className="wk-muted wk-muted--warn">{t('integrations.api.directWarning')}</p>
        <label className="wk-check-row"><input className="box-border w-full max-w-[420px] rounded-[6px] border border-line-control px-[0.6rem] py-[0.5rem] [font:inherit]" type="checkbox" checked={requireDirectHeader} onChange={(event) => setRequireDirectHeader(event.target.checked)} />{t('integrations.api.requireDirectHeader')}</label>
        <p className="wk-muted">{t('integrations.api.requireDirectHeaderDesc')}</p>
      </div> : null}
      {principalMode === 'signed_token' ? <div className="mt-[0.6rem] grid gap-[0.5rem]">
        <label className="grid gap-[0.3rem] font-semibold">{t('integrations.api.hmacSecret')}<input className="box-border w-full max-w-[420px] rounded-[6px] border border-line-control px-[0.6rem] py-[0.5rem] [font:inherit]" type="password" value={hmacSecret} onChange={(event) => setHmacSecret(event.target.value)} placeholder={principal?.has_hmac_secret ? t('integrations.api.secretConfigured') : ''} /></label>
        <p className="wk-muted">{t('integrations.api.hmacSecretDesc')}</p>
      </div> : null}
      {principalMode !== 'tenant' ? <div className="wk-form-actions">
        <button className="wk-button" type="button" disabled={busy} onClick={onSavePrincipal}>{t('common.save')}</button>
      </div> : null}
      <div className="mt-[0.6rem] grid gap-[0.5rem]">
        <label className="grid gap-[0.3rem] font-semibold">{t('integrations.api.playgroundExternalUser')}<input className="box-border w-full max-w-[420px] rounded-[6px] border border-line-control px-[0.6rem] py-[0.5rem] [font:inherit]" value={externalUserId} onChange={(event) => setExternalUserId(event.target.value)} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} /></label>
        <div className="wk-form-actions">
          <button className="wk-button" type="button" disabled={busy} onClick={onCreatePrincipalToken}>{t('integrations.api.generateSecret')}</button>
        </div>
        {principalToken ? <p className="wk-status">{t('integrations.api.playgroundGeneratedToken')}: <code>{principalToken.token}</code> ({principalToken.headerName})</p> : null}
      </div>
    </section>

    <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <div className="mb-[0.75rem] flex items-start justify-between gap-4">
        <div>
          <label className="block font-semibold text-ink">{t('integrations.api.playgroundTitle')}</label>
          <p className="m-0 mt-[0.15rem] text-[13px] text-muted-strong">{t('integrations.api.playgroundDesc')}</p>
        </div>
      </div>
      {onOpenApiPlayground ? <button className="wk-button wk-button--primary" type="button" onClick={onOpenApiPlayground}>{t('integrations.api.playgroundOpen')}</button> : null}
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
    {tab === 'cli' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.cli.quickstart')}</h4>
      <ol className="m-0 list-none grid gap-[0.9rem] p-0">
        {cliSteps.map((step, index) => <li key={step.key} className="flex gap-[10px]">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[rgba(46,109,230,0.1)] text-[12px] font-semibold text-primary">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="text-[14px] font-semibold text-ink">{step.title}</div>
            <p className="m-0 mt-[0.2rem] mb-[0.5rem] text-[13px] leading-[1.6] text-muted-strong">{step.desc}</p>
            <div className="wk-code-toolbar"><pre>{step.command}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy(step.command)}>⧉</button></div>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {tab === 'cli' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.cli.commandsTitle')}</h4>
      <p className="wk-muted m-0 mb-[0.6rem] text-[13px]">{t('integrations.cli.commandsDesc')}</p>
      <div className="wk-code-toolbar"><pre>{'weknora doc upload ./document.pdf --kb "KB_ID"\nweknora search chunks "query" --kb "KB_ID"\nweknora chat "question" --kb "KB_ID" --format text\nweknora agent list'}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy('weknora doc upload')}>⧉</button></div>
    </section> : null}
    {tab === 'cli' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.cli.mcpTitle')}</h4>
      <p className="wk-muted m-0 mb-[0.6rem] text-[13px]">{t('integrations.cli.mcpDesc')}</p>
      <div className="wk-code-toolbar"><pre>{JSON.stringify({ mcpServers: { weknora: { command: 'weknora', args: ['--profile', 'weknora', 'mcp', 'serve'] } } }, null, 2)}</pre><button className="wk-button wk-button--text" type="button" title={t('integrations.cli.copy')} onClick={() => copy('mcp')}>⧉</button></div>
    </section> : null}
    {tab === 'chrome' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.chrome.capabilitiesTitle')}</h4>
      <div className="wk-capability-grid">
        {chromeCapabilities.map((key) => <div key={key} className="wk-capability-card">
          <h5>{t('integrations.chrome.capabilities.' + key + '.title')}</h5>
          <p>{t('integrations.chrome.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {tab === 'chrome' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.chrome.stepsTitle')}</h4>
      <ol className="m-0 list-none grid gap-[0.9rem] p-0">
        {chromeSteps.map((key, index) => <li key={key} className="flex gap-[10px]">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[rgba(46,109,230,0.1)] text-[12px] font-semibold text-primary">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="text-[14px] font-semibold text-ink">{t('integrations.chrome.steps.' + key + '.title')}</div>
            <p className="m-0 mt-[0.2rem] mb-[0.5rem] text-[13px] leading-[1.6] text-muted-strong">{t('integrations.chrome.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {tab === 'claw' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.claw.capabilitiesTitle')}</h4>
      <div className="wk-capability-grid">
        {clawCapabilities.map((key) => <div key={key} className="wk-capability-card">
          <h5>{t('integrations.claw.capabilities.' + key + '.title')}</h5>
          <p>{t('integrations.claw.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {tab === 'claw' ? <section className="rounded-[10px] border border-[#eef1f5] p-4">
      <h4 className="m-0 mb-[0.6rem] text-[14px] font-semibold text-ink">{t('integrations.claw.stepsTitle')}</h4>
      <ol className="m-0 list-none grid gap-[0.9rem] p-0">
        {clawSteps.map((key, index) => <li key={key} className="flex gap-[10px]">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[rgba(46,109,230,0.1)] text-[12px] font-semibold text-primary">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="text-[14px] font-semibold text-ink">{t('integrations.claw.steps.' + key + '.title')}</div>
            <p className="m-0 mt-[0.2rem] mb-[0.5rem] text-[13px] leading-[1.6] text-muted-strong">{t('integrations.claw.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
  </div>;
}
