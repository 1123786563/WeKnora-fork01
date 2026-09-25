import * as React from 'react';
import { useEffect, useRef, useState } from 'react';
import type { Locale } from '../../../i18n/src/index.ts';
import { INTEGRATION_SECTIONS, integrationSection, type IntegrationKey } from './registry.ts';
import { buildEmbedUpdatePayload } from './form.ts';
import {
  apiKeyAccessMode,
  apiKeyValueDisplay,
  buildApiKeyCreatePayload,
  createDefaultApiKeySelections,
  isFreshKeyVisible,
  apiKeyKnowledgeScopeApplies,
  selectedApiKeyCapabilities,
  TENANT_API_KEY_CAPABILITY_GROUPS,
  type ApiKeyCapabilitySelections,
  type ApiKeyCreatePayload,
  type ApiKeyRow,
} from './apiKeys.ts';
// T12c：图标对齐 Vue t-icon sprite glyph（台账 #10 本地镜像 0.4.5）——
// 手绘 path 近似版（LandingIcon/CopyIcon/JumpIcon/PlusIcon/⧉ 文本）全数改走
// 注入式 sprite 渲染器（apps/web 侧 setIntegrationSpriteIconRenderer 注入
// tdesign-icons-react Icon；名称按 Vue 各组件 capabilityIcons/template
// #icon 映射：url→link、manual/notes→edit、browse→view-list、qa→
// chat-bubble、clip→file-copy、shortcuts→jump）。@weknora/views 无
// tdesign-icons-react 依赖，未注入时回退原手绘 path（本包测试直渲染口径）。
type SpriteIconRenderer = (name: string, size?: number | string) => React.ReactNode;
let spriteIconRenderer: SpriteIconRenderer | null = null;
export function setIntegrationSpriteIconRenderer(renderer: SpriteIconRenderer | null): void { spriteIconRenderer = renderer; }
function SpriteIcon({ name, size, fallback }: { name: string; size?: number | string; fallback?: React.ReactNode }) {
  if (spriteIconRenderer) return <>{spriteIconRenderer(name, size)}</>;
  return <>{fallback}</>;
}
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
  onCreatePrincipalTestToken?: (externalUserId: string) => Promise<IntegrationPrincipalToken>;  /** Vue createTenantAPIKey payload (ApiIntegrationSettings.vue createScopedAPIKey):
   *  { name, full_access, knowledge_base_ids, capabilities }. */
  onCreateApiKey?: (payload: ApiKeyCreatePayload) => Promise<ApiKeyRow>;  onRevokeApiKey?: (keyId: ApiKeyRow['id']) => Promise<void>;
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
  /** Vue IM/Embed panels expose mutation controls only to tenant admins. */
  canEdit?: boolean;
  /**
   * GET /system/info `swagger_enabled`. The API tab's docs row renders only
   * when this is explicitly true — undefined (older backend / fetch failure)
   * and false (release build) both hide the entry (SP14 Task 2 dangling-link fix).
   */
  swaggerEnabled?: boolean;
}

function initialLocale(): Locale {
  try {
    const stored = window.localStorage.getItem('locale');
    return integrationsLocale(stored);
  } catch {
    return 'zh-CN';
  }
}

export function IntegrationsPage({ embedded = false, embedChannels, imChannels, apiBaseUrl, apiKeys = [], apiKeysLoading = false, activeTab, onTabChange, initialTab = 'embed', loading = false, error, onReload, onOpenEmbed, onOpenApiPlayground, actions = {}, locale: localeProp, agents = [], knowledgeBases = [], canEdit = true, swaggerEnabled }: IntegrationsPageProps) {
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
  // Vue IntegrationSettingsSection keeps one shared filterAgentId ref bound to
  // both IMChannelPanel and AgentEmbedChannelPanel (v-model:filter-agent-id);
  // the panel filters its channel list by resource.agent_id.
  const [imAgentFilter, setImAgentFilter] = useState('');
  const [embedAgentFilter, setEmbedAgentFilter] = useState('');
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
    // Vue resetForm (IMChannelPanel.vue:995-1015)：name 预填 defaultChannelName('wecom')。
    setImForm({ ...createImWizardForm(), targetAgentId: imForm.targetAgentId, name: imPlatformLabel('wecom', locale) });
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
      setWechatQrError(cause instanceof Error ? cause.message : t('agentEditor.im.wechatQrFailed'));
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
  // Vue createScopedAPIKey: the dialog builds the { name, full_access,
  // knowledge_base_ids, capabilities } payload and the route page posts it to
  // POST /tenants/{id}/api-keys unchanged.
  const createApiKey = (payload: ApiKeyCreatePayload) => run(async () => { if (!actions.onCreateApiKey) return; const created = await actions.onCreateApiKey(payload); setFreshApiKeyId(created.id); setShowApiKeyForm(false); });
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
    {/* Former .wk-integrations-page/.wk-integrations-header (apps/web styles.css
        181-183) as Tailwind utilities; the h1 margin uses the important suffix
        to outrank the global unlayered h1 rule still in styles.css. Vue settings
        embeds the section inside .content-wrapper--full (30px 34px 40px), so the
        embedded mode drops the route-shell paddings. */}
    <main className={'integrations-settings wk-vi-154' + (embedded ? '' : ' wk-vi-155')}>
      {!embedded ? <><header className="wk-vi-1">
        <div><h1 className="wk-vi-2">{t('integrations.title')}</h1><p className="wk-muted wk-vi-3">{t('integrations.agentEditor.desc')}</p></div>
        {onReload ? <button className="wk-button wk-vi-4" type="button" onClick={onReload}>{t('common.retry')}</button> : null}
      </header>
      <nav className="wk-vi-5" aria-label={t('integrations.title')}>
        {INTEGRATION_SECTIONS.map((item) => <button type="button" key={item.key} className={item.key === tab ? INT_TAB_ACTIVE_CLASS : INT_TAB_CLASS} onClick={() => setTab(item.key)}>{t('integrations.tabs.' + item.key)}</button>)}
      </nav></> : null}
      {/* Former .wk-integrations-panel / .wk-int-section-header / -desc / -doc-link / -icon.
          Vue IntegrationSettingsSection.vue renders the section-header card only
          for im/embed/api; the cli/chrome/claw landings mount bare inside
          .integrations-settings__body--landing (max-width 760px, no card). */}
        <section className="wk-vi-6">
        {/* Vue IntegrationSettingsSection.vue renders the section header bare on
            the body (no card / divider / role pill): h2 18px + 13px description. */}
        {!section.external ? <div className="wk-int-section-heading wk-vi-7">
            <h2 className="wk-vi-8">{copy.heading}</h2>
            <p className="wk-vi-9">
              {/* Vue 模板凝结：描述文本与尾随空格为单一文本节点（"…云之家 "），
                  JSX 单表达式拼接防跨节点 kerning 漂移（台账 #11 同族）。 */}
              {copy.docLinkLabel && copy.docUrl ? copy.description + ' ' : copy.description}
              {copy.docLinkLabel && copy.docUrl ? <a className="wk-vi-10" href={copy.docUrl} target="_blank" rel="noreferrer noopener">{copy.docLinkLabel}<LandingIcon name="url" size={13} /></a> : null}
            </p>
          </div> : null}
        {loading ? <p className="wk-status wk-vi-11">{t('integrations.api.loading')}</p> : null}
        {error || localError ? <p className="wk-status wk-status-error wk-vi-12" role="alert">{error || localError}</p> : null}
        {!loading && !error && (tab === 'im' || tab === 'embed') ? <ChannelListPanel
          variant={tab}
          copy={copy}
          locale={locale}
          // Vue IMChannelPanel/AgentEmbedChannelPanel filter channels by the
          // selected bound agent (channels computed on filterAgentId).
          items={(tab === 'im' ? imChannels : embedChannels).filter((item) => {
            const filter = tab === 'im' ? imAgentFilter : embedAgentFilter;
            return !filter || item.agent_id === filter;
          })}
          agents={agents}
          agentFilter={tab === 'im' ? imAgentFilter : embedAgentFilter}
          onAgentFilter={tab === 'im' ? setImAgentFilter : setEmbedAgentFilter}
          showCreate={tab === 'im' ? imWizardOpen : embedWizardOpen}
          onToggleCreate={() => (tab === 'im' ? (imWizardOpen ? closeImWizard() : openImCreate()) : (embedWizardOpen ? closeEmbedWizard() : openEmbedCreate()))}
          canEdit={canEdit}
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
          onToggle={canEdit && tab === 'im' && actions.onToggleIm ? (id) => run(async () => { await actions.onToggleIm?.(id); onReload?.(); }) : undefined}
          onDelete={canEdit && (actions.onDeleteEmbed || actions.onDeleteIm) ? deleteChannel : undefined}
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
            canSubmit={canEdit && Boolean(actions.onCreateIm || actions.onUpdateIm)}
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
            canEdit={canEdit}
            canSubmit={canEdit && Boolean(actions.onCreateEmbed || actions.onUpdateEmbed)}
            onNext={embedNext}
            onBack={embedBack}
            onGoTo={embedGoTo}
            onSave={saveEmbedWizard}
            onCancel={closeEmbedWizard}
          /> : null}
        /> : null}
        {!loading && !error && tab === 'api' ? <ApiIntegrationPanel apiBaseUrl={apiBaseUrl} swaggerEnabled={swaggerEnabled} actions={actions} principalMode={principalMode} setPrincipalMode={setPrincipalMode} requireDirectHeader={requireDirectHeader} setRequireDirectHeader={setRequireDirectHeader} hmacSecret={hmacSecret} setHmacSecret={setHmacSecret} externalUserId={externalUserId} setExternalUserId={setExternalUserId} principalToken={principalToken} onSavePrincipal={savePrincipal} onCreatePrincipalToken={createPrincipalToken} apiKey={apiKey} setApiKey={setApiKey} sessionId={sessionId} setSessionId={setSessionId} playgroundPath={playgroundPath} setPlaygroundPath={setPlaygroundPath} playgroundBody={playgroundBody} setPlaygroundBody={setPlaygroundBody} playgroundOutput={playgroundOutput} onRunPlayground={runPlayground} busy={busy} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} freshApiKeyId={freshApiKeyId} knowledgeBases={knowledgeBases} showApiKeyForm={showApiKeyForm} setShowApiKeyForm={setShowApiKeyForm} onCreateApiKey={createApiKey} onRevokeApiKey={revokeApiKey} onCopyApiKey={(key) => { void navigator.clipboard.writeText(key.api_key).catch(() => undefined); }} onOpenApiPlayground={onOpenApiPlayground} t={t} /> : null}
        {!loading && !error && section.external ? <ExternalLandingPanel tab={tab} locale={locale} externalUrl={section.externalUrl} apiBaseUrl={apiBaseUrl} onOpenApiSettings={() => setTab('api')} t={t} /> : null}
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
  // Former .wk-embed-preview-overlay/.wk-embed-preview-drawer/.wk-embed-preview-header
  // (apps/web styles.css) as Tailwind utilities; the @media (max-width: 720px)
  // tweaks became the max-[720px]: variants on the drawer/body/widget-panel.
  return <div className="wk-vi-13" role="presentation" onClick={onClose}>
    <aside className="wk-vi-14" role="dialog" aria-modal="true" aria-label={preview.channel.name || t('embedPublish.preview')} onClick={(event) => event.stopPropagation()}>
      <header className="wk-vi-15"><h2 className="wk-vi-16">{preview.channel.name || t('embedPublish.preview')}</h2><button type="button" className={INTEGRATION_DRAWER_CLOSE_CLASS} aria-label={t('common.close')} title={t('common.close')} onClick={onClose}>×</button></header>
      {/* Former .wk-embed-preview-body + .wk-embed-preview-hint. */}
      <div className="wk-vi-17">
        <p className="wk-vi-18">{t(preview.mode === 'iframe' ? 'embedPublish.previewIframeHint' : 'embedPublish.previewWidgetHint')}</p>
        {preview.mode === 'iframe' ? <div className={EMBED_PREVIEW_FRAME_CLASS}>
          {/* Former .wk-embed-preview-chrome + span:nth-child(1..3) + code. */}
          <div className="wk-vi-19"><span className="wk-vi-20">●</span><span className="wk-vi-21">●</span><span className="wk-vi-22">●</span><code className="wk-vi-23">/embed/{channelId}</code></div>
          {/* Former .wk-embed-preview-screen + iframe rules; the old dynamic
              .is-loading class became this static 'invisible' condition. */}
          <div className="wk-vi-24">{!ready ? <span className="wk-muted wk-vi-3">{t('embedPublish.previewLoading')}</span> : null}{layoutReady ? <iframe title={preview.channel.name || t('embedPublish.preview')} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'wk-vi-156'} allow="clipboard-write" /> : null}</div>
        </div> : <div className={EMBED_PREVIEW_FRAME_CLASS}>
          {/* Former .wk-embed-preview-mock-page + span / span.short. */}
          <div className="wk-vi-25"><strong>{t('embedPublish.previewMockPage')}</strong><span className="wk-vi-26" /><span className="wk-vi-27" /></div>
          {/* Former .wk-embed-preview-widget-panel (+ ≤720px right tweak). */}
          {widgetOpen && layoutReady ? <div className="wk-vi-28"><iframe title={preview.channel.name || t('embedPublish.preview')} src={src} onLoad={() => setReady(true)} allow="clipboard-write" /></div> : null}
          <button type="button" className={EMBED_PREVIEW_LAUNCHER_CLASS} style={{ background: typeof preview.channel.primary_color === 'string' ? preview.channel.primary_color : '#07c05f' }} onClick={() => setWidgetOpen((open) => !open)} aria-label={widgetOpen ? '关闭' : t('embedPublish.preview')}>{widgetOpen ? '×' : '◔'}</button>
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
    {step === 0 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.agentLabel')}<select value={form.targetAgentId} onChange={(event) => update('targetAgentId', event.target.value)} disabled={Boolean(editing)}><option value="">{t('agentEditor.im.selectAgent')}</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}</select></label><label>{t('agentEditor.im.nameLabel')}<input value={form.name} onChange={(event) => { onNameTouched(true); update('name', event.target.value); }} placeholder={t('agentEditor.im.namePlaceholder')} /></label><label>{t('agentEditor.im.platformLabel')}<select value={form.platform} onChange={(event) => onPlatformPicked(event.target.value)}>{imPlatformOrder().map((platform) => <option key={platform} value={platform}>{imPlatformLabel(platform, locale)}</option>)}</select></label>{nameTouched && !form.name.trim() ? <small className="wk-status-error wk-vi-29">{t('agentEditor.im.nameRequired')}</small> : null}</div> : null}
    {step === 1 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.connectionMode')}<select value={form.mode} onChange={(event) => update('mode', event.target.value as ImWizardForm['mode'])}><option value="websocket">WebSocket</option><option value="webhook">Webhook</option><option value="longpoll">Long Poll</option></select></label><label>{t('agentEditor.im.outputMode')}<select value={form.outputMode} onChange={(event) => update('outputMode', event.target.value as ImWizardForm['outputMode'])}><option value="stream">Stream</option><option value="full">Full</option></select></label>{imPlatformSupportsThread(form.platform) ? <label>{t('agentEditor.im.sessionMode')}<select value={form.sessionMode} onChange={(event) => update('sessionMode', event.target.value as ImWizardForm['sessionMode'])}><option value="user">User</option><option value="thread">Thread</option></select></label> : null}</div> : null}
    {step === 2 ? <div className="wk-im-wizard-fields"><label>{t('agentEditor.im.knowledgeBaseLabel')}<select value={form.knowledgeBaseId} onChange={(event) => update('knowledgeBaseId', event.target.value)}><option value="">{t('agentEditor.im.noKnowledgeBase')}</option>{knowledgeBases.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}</select></label></div> : null}
    {step === 3 ? <div className="wk-im-wizard-fields">{editing ? <label className="wk-switch-row"><input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />{t('agentEditor.im.enabled')}</label> : null}{form.platform === 'wechat' ? <div className="wk-im-wechat-bind">{wechatQr ? <img src={wechatQr.imgSrc} alt={t('agentEditor.im.wechatQrAlt')} /> : null}<button className="wk-button wk-vi-4" type="button" disabled={wechatQrLoading || busy} onClick={onStartWeChatBinding}>{wechatQrLoading ? t('common.loading') : t('agentEditor.im.wechatScanBind')}</button>{wechatQrError ? <p className="wk-status-error wk-vi-29">{wechatQrError}</p> : null}</div> : fields.map((field) => <label key={field.key}>{field.label ?? t(field.labelKey ?? field.key)}<input type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'} value={String(form.credentials[field.key] ?? '')} min={field.min} max={field.max} placeholder={field.placeholder ?? (field.placeholderKey ? t(field.placeholderKey) : undefined)} onChange={(event) => onForm({ ...form, credentials: { ...form.credentials, [field.key]: field.type === 'number' ? Number(event.target.value) : event.target.value } })} />{field.hintKey ? <small className="wk-muted wk-vi-3">{t(field.hintKey)}</small> : null}</label>)}{consoleLink ? <a href={consoleLink.url} target="_blank" rel="noreferrer noopener">{t(consoleLink.labelKey)}</a> : null}</div> : null}
    {warning ? <p className="wk-status-error wk-vi-29" role="alert">{warning}</p> : null}<small className="wk-muted wk-vi-3">{apiBaseUrl}</small><div className="wk-list-actions wk-vi-30"><button className="wk-button wk-vi-4" type="button" onClick={onCancel}>{t('common.cancel')}</button>{step > 0 ? <button className="wk-button wk-vi-4" type="button" disabled={busy} onClick={onBack}>{t('common.previous')}</button> : null}{step < IM_WIZARD_STEPS.length - 1 ? <button className="wk-button wk-button--primary wk-vi-4" type="button" disabled={busy} onClick={onNext}>{t('common.next')}</button> : <button className="wk-button wk-button--primary wk-vi-4" type="button" disabled={busy || !canSubmit} onClick={onSave}>{t('common.save')}</button>}</div>
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

// Tailwind port of the former .wk-channel-card family in apps/web/src/styles.css
// (356-375, 581). Shared card chrome lives in CHANNEL_CARD_CLASS; color,
// background and hover state are per-variant so no two utilities of the same
// property compete on one element. max-[720px] carries the old
// @media (max-width: 720px) card wrap.
// Vue channel-panel-list.less: .channel-card min-height is the static
// calc(20px + 14px*1.4 + 4px + 12px*1.4) = 60.4px two-line body, so the add
// tile matches the existing channel cards even in an empty list.
const CHANNEL_CARD_CLASS = 'wk-vi-channel-card-class'; /* 原 ease-[ease] 为死样式（unlayered CSS 已自带 timing） */

const CHANNEL_CARD_CLICKABLE_CLASS = CHANNEL_CARD_CLASS + ' wk-vi-channel-card-clickable-class';

const CHANNEL_CARD_STATIC_CLASS = CHANNEL_CARD_CLASS + ' wk-vi-channel-card-static-class';

// Vue .channel-card--add inherits --td-text-color-placeholder rgba(0,0,0,0.4).
const CHANNEL_CARD_ADD_CLASS = CHANNEL_CARD_CLASS + ' wk-vi-channel-card-add-class';

const CHANNEL_BADGE_CLASS = 'wk-vi-channel-badge-class';

const CHANNEL_BADGE_STATIC_CLASS = CHANNEL_BADGE_CLASS + ' wk-vi-channel-badge-static-class';

const CHANNEL_BADGE_ADD_CLASS = CHANNEL_BADGE_CLASS + ' wk-vi-channel-badge-add-class';

const CHANNEL_CARD_BODY_CLASS = 'wk-vi-channel-card-body-class';

const CHANNEL_CARD_HEADER_CLASS = 'wk-vi-channel-card-header-class';

const CHANNEL_CARD_TITLE_CLASS = 'wk-vi-channel-card-title-class';

const CHANNEL_CARD_TITLE_STATIC_CLASS = CHANNEL_CARD_TITLE_CLASS + ' wk-vi-channel-card-title-static-class';

const CHANNEL_CARD_TITLE_ADD_CLASS = CHANNEL_CARD_TITLE_CLASS + ' wk-vi-channel-card-title-add-class';

const CHANNEL_CARD_AGENT_CLASS = 'wk-vi-channel-card-agent-class';

const CHANNEL_CARD_ACTIONS_CLASS = 'wk-vi-channel-card-actions-class';


// Tailwind ports of the former .wk-integrations-tabs button / .wk-int-doc-link /
// .wk-code-toolbar / .wk-integration-form families in apps/web styles.css.
// .wk-integration-form keeps its legacy class name: INTEGRATION_DRAWER_CLASS
// below targets it with [&_.wk-integration-form] variants, and those overrides
// keep the important suffix so they beat this layered base regardless of
// source order (the base itself no longer competes with unlayered css).
const INT_TAB_CLASS = 'wk-vi-int-tab-class';

const INT_TAB_ACTIVE_CLASS = INT_TAB_CLASS + ' wk-vi-int-tab-active-class';

const INT_DOC_LINK_CLASS = 'wk-vi-int-doc-link-class';

const CODE_TOOLBAR_CLASS = 'wk-vi-code-toolbar-class';

const CODE_TOOLBAR_PRE_CLASS = 'wk-vi-code-toolbar-pre-class';

const CODE_TOOLBAR_BUTTON_CLASS = 'wk-vi-code-toolbar-button-class';

// Tailwind port of the former .wk-option-chip / .wk-option-chip--active family
// in apps/web styles.css. Static literals; the two states share no utility that
// sets the same property, so no stylesheet-order dependence. [font-*:inherit]
// longhands replace the old font:inherit shorthand so text-[13px] cannot lose
// to shorthand expansion order.
const CHIP_BASE = 'wk-vi-chip-base';

// T15：主题色 utility 语义化为 .wk-vi-chip--active/--idle（views-integrations-u.css）。
const chip = (active: boolean) => (active ? CHIP_BASE + ' wk-vi-chip--active' : CHIP_BASE + ' wk-vi-chip--idle');
const INTEGRATION_FORM_CLASS = 'wk-integration-form wk-vi-integration-form-class';

// Tailwind port of the former .wk-integration-drawer family in apps/web
// styles.css (Vue SettingDrawer parity: 560px overlay + scroll + focus colors).
// The aside carries the drawer subtree in arbitrary variants (the wizard slots
// below render .wk-integration-form markup, so the subtree stays
// self-contained); properties that must beat the INTEGRATION_FORM_CLASS base
// (or the text-muted utility carried by .wk-muted elements) use the
// important suffix.
// .wk-check-row labels keep the class as a :not() selector hook (the styles.css
// rule is deleted; the labels now carry flex! items-center gap-[0.45rem]
// font-normal! themselves): the drawer's [&_label:not(.wk-check-row)]:font-medium!
// must stay excluded from them, else its higher specificity would outrank their
// font-normal!. The two @keyframes stay in
// styles.css and are referenced via animate-[...]; motion-reduce:animate-none
// carries the old reduced-motion block. Legacy class names stay on the
// elements as DOM hooks (embedWizardRender.test.tsx queries
// .wk-integration-drawer / .wk-integration-drawer-close).
const INTEGRATION_DRAWER_OVERLAY_CLASS = 'wk-integration-drawer-overlay wk-vi-integration-drawer-overlay-class wk-mr-none';

const INTEGRATION_DRAWER_CLASS = 'wk-integration-drawer wk-vi-integration-drawer-class wk-mr-none';

// .wk-integration-drawer .wk-im-step (static literal: Tailwind extracts candidates from raw text,
// so interpolated selectors would never be generated)
const IM_STEP_CHROME = 'wk-vi-im-step-chrome';

// .wk-integration-drawer .wk-embed-step (static literal: Tailwind extracts candidates from raw text,
// so interpolated selectors would never be generated)
const EMBED_STEP_CHROME = 'wk-vi-embed-step-chrome';

const INTEGRATION_DRAWER_CLASS_STEPS = INTEGRATION_DRAWER_CLASS + ' ' + IM_STEP_CHROME + ' ' + EMBED_STEP_CHROME + ' wk-vi-integration-drawer-class-steps';

// .wk-integration-drawer-close (hover mirrors the old :hover/:focus-visible
// rule; the class name remains as a test/DOM hook on every consumer).
const INTEGRATION_DRAWER_CLOSE_CLASS = 'wk-integration-drawer-close wk-vi-integration-drawer-close-class';

// —— B4：抽屉族 t_drawer 同构层（chat/message-face.tsx t-button 同构判例口径）——
// 本包不带 tdesign-react 依赖（package.json 由并行流共享）；宿主 app 全局加载
// tdesign.css（apps/web styles.css:2），此处按 tdesign-vue-next 1.20.7 +
// SettingDrawer.vue 的实测 DOM 输出 t-* 类名（Vue 端唯一事实源），获得与 Vue
// 端一致的 chrome/控件视觉。样式补块（setting-drawer__* / im-* / form-* /
// api-key-dialog*，Vue scoped 块平移）见 views-integrations-u.css。
type PlatformLogoRenderer = (platform: string) => string;
let platformLogoRenderer: PlatformLogoRenderer | null = null;
/** apps/web 侧注入平台 logo 映射（frontend/src/assets/img/im/* 同源资产）。 */
export function setIntegrationPlatformLogoRenderer(renderer: PlatformLogoRenderer | null): void { platformLogoRenderer = renderer; }

/** tdesign t-fake-arrow（t-select 右侧箭头，drawer/select DOM 内联版）。 */
function FakeArrow() {
  return <svg className="t-fake-arrow t-select__right-icon" width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M3.75 5.7998L7.99274 10.0425L12.2361 5.79921" stroke="black" strokeOpacity="0.9" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>;
}

/** tdesign Button 类族（td.tsx Button 同款输出；footer/分组头按钮共用）。 */
function TButtonLike({ variant = 'outline', theme = 'default', size, disabled, onClick, children, type = 'button' }: {
  variant?: 'base' | 'outline' | 'text'; theme?: 'default' | 'primary' | 'danger'; size?: 'small'; disabled?: boolean; onClick?: () => void; children?: React.ReactNode; type?: 'button' | 'submit';
}) {
  return <button type={type} disabled={disabled} onClick={onClick}
    className={['t-button', `t-button--variant-${variant}`, `t-button--theme-${theme}`, 't-button--shape-rectangle', 'wk-button', size === 'small' ? 't-size-s' : '', disabled ? 't-is-disabled' : ''].filter(Boolean).join(' ')}>
    <span className="t-button__text">{children}</span>
  </button>;
}

/** t-select 关闭态（Vue t-select__wrap>t-select-input>t-input__wrap>t-input 实测 DOM）。 */
function TSelectLike({ value, placeholder, prefixLogo, prefixAlt, className }: { value: string; placeholder?: string; prefixLogo?: string; prefixAlt?: string; className?: string }) {
  const empty = value === '';
  return <div className={'t-select__wrap' + (className ? ' ' + className : '')}>
    <div className={'t-select-input' + (empty ? ' t-select-input--empty' : '') + ' t-select'}>
      <div className="t-input__wrap">
        <div className={'t-input' + (prefixLogo ? ' t-is-readonly t-input--prefix t-input--suffix' : ' t-input--suffix')}>
          {prefixLogo ? <span className="t-input__prefix t-input__prefix-icon"><img src={prefixLogo} alt={prefixAlt} className="im-platform-select-prefix" /></span> : null}
          <input className="t-input__inner" type="text" readOnly placeholder={placeholder} value={value} />
          <span className="t-input__suffix t-input__suffix-icon"><FakeArrow /></span>
        </div>
      </div>
    </div>
  </div>;
}

/** t-input（t-input__wrap>t-input>t-input__inner 实测 DOM）。 */
function TInputLike({ value, placeholder, onChange, onFocus, onEnter }: { value: string; placeholder?: string; onChange: (next: string) => void; onFocus?: () => void; onEnter?: () => void }) {
  return <div className="t-input__wrap">
    <div className="t-input">
      <input className="t-input__inner" type="text" placeholder={placeholder} value={value} autoComplete="off"
        onFocus={onFocus}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => { if (onEnter && event.key === 'Enter') onEnter(); }} />
    </div>
  </div>;
}

/** t-radio-button 组（Vue mode-radio t-radio-group__outline 实测 DOM）。 */
function TRadioGroupLike({ value, options, onPick, className }: { value: string; options: Array<{ value: string; label: string }>; onPick: (next: string) => void; className?: string }) {
  return <div className={'t-radio-group t-size-m t-radio-group__outline' + (className ? ' ' + className : '')} role="radiogroup">
    {options.map((option) => (
      <label key={option.value} role="radio" aria-checked={option.value === value} className={'t-radio-button' + (option.value === value ? ' t-is-checked' : '')} tabIndex={0}>
        <input type="radio" className="t-radio-button__former" checked={option.value === value} tabIndex={-1} value={option.value} autoComplete="off" onChange={() => onPick(option.value)} />
        <span className="t-radio-button__input"></span>
        <span className="t-radio-button__label">{option.label}</span>
      </label>
    ))}
  </div>;
}

/** t-checkbox（Vue t-checkbox 实测 DOM；wk-check-row 为既有测试 seam）。 */
function TCheckboxLike({ checked, onChange, children }: { checked: boolean; onChange: (next: boolean) => void; children?: React.ReactNode }) {
  return <label className={'t-checkbox wk-check-row' + (checked ? ' t-is-checked' : '')} tabIndex={0}>
    <input type="checkbox" className="t-checkbox__former" tabIndex={-1} checked={checked} autoComplete="off" onChange={(event) => onChange(event.target.checked)} />
    <span className="t-checkbox__input"></span>
    <span className="t-checkbox__label">{children}</span>
  </label>;
}

/** t-icon（sprite <use>，message-face 同款）。 */
function TIconLike({ name, size = '16px' }: { name: string; size?: string }) {
  return <svg className={'t-icon t-icon-' + name} viewBox="0 0 24 24" width={size} height={size} fill="none" aria-hidden="true"><use href={'#t-icon-' + name} /></svg>;
}

/**
 * Vue SettingDrawer.vue 同构输出：t-drawer(attach body / z-index 2500 / 右滑
 * 560px) + 自定义 header（icon 块 + title + subtitle）+ narrow-scrollbar body
 * + footer（footer-left / footer-right）。Vue 端不渲染关闭钮（closeBtn 关），
 * 遮罩点击关闭由 closeOnOverlayClick 控制（im/embed 开、api-key 关）。
 */
function SettingDrawerChrome({ className, width = '560px', headerIcon, title, subtitle, closeOnOverlayClick = true, onClose, footerLeft, footerRight, bodyChildren }: {
  className?: string; width?: string; headerIcon?: React.ReactNode; title: string; subtitle?: string; closeOnOverlayClick?: boolean; onClose: () => void; footerLeft?: React.ReactNode; footerRight?: React.ReactNode; bodyChildren: React.ReactNode;
}) {
  return <div
    className={'t-drawer t-drawer--right t-drawer--open setting-drawer ' + (className ?? '')}
    role="dialog" aria-modal="true" aria-label={title} style={{ zIndex: 2500 }}
    onClick={(event) => event.stopPropagation()}
  >
    <div className="t-drawer__mask" onClick={closeOnOverlayClick ? onClose : undefined} />
    {/* tdesign open 态只置 visibility，滑入位移由组件内联 transform 驱动——
        同构层直接给 translateX(0) 定格在滑入完成态（截图稳态）。 */}
    <div className="t-drawer__content-wrapper t-drawer__content-wrapper--right" style={{ width, transform: 'translateX(0)', visibility: 'visible' }}>
      <div className="t-drawer__header">
        <div className="setting-drawer__header-block">
          <div className="setting-drawer__header">
            {headerIcon ? <div className="setting-drawer__header-icon">{headerIcon}</div> : null}
            <div className="setting-drawer__header-text">
              <div className="setting-drawer__title">{title}</div>
              {subtitle ? <div className="setting-drawer__subtitle">{subtitle}</div> : null}
            </div>
            <div className="setting-drawer__header-actions" />
          </div>
        </div>
      </div>
      <div className="t-drawer__body narrow-scrollbar">
        <div className="setting-drawer__body">{bodyChildren}</div>
      </div>
      <div className="t-drawer__footer">
        <div className="setting-drawer__footer">
          <div className="setting-drawer__footer-left">{footerLeft}</div>
          <div className="setting-drawer__footer-right">{footerRight}</div>
        </div>
      </div>
    </div>
  </div>;
}

// Tailwind port of the former .wk-embed-preview-device / .wk-embed-preview-widget
// rules in apps/web styles.css (Vue EmbedChannelPreview.vue device-frame parity;
// the route-shell modal in apps/web EmbedPreviewModal.tsx carries the same
// utilities). The shared @media (max-width: 720px) tweaks became max-[720px]:
// variants and the iframe .is-loading visibility hook became a static
// 'invisible' condition in the panel below.
const EMBED_PREVIEW_FRAME_CLASS = 'wk-vi-embed-preview-frame-class';

// Former .wk-embed-preview-launcher (also reused as the widget-position
// color swatch in the embed wizard form below).
const EMBED_PREVIEW_LAUNCHER_CLASS = 'wk-vi-embed-preview-launcher-class';


// Vue IntegrationsAgentFilter.vue ported: a small filter icon + chevron button
// (18px tall, 6px radius) that opens a dropdown of bound agents; picking one
// filters the channel list (the panel-level filter state). The button keeps
// the Vue geometry: 14px filter icon, 12px chevron, 4px gap, padding
// 2px 6px 2px 4px, placeholder color rgba(0,0,0,0.4).
function AgentFilterButton({ agents, value, locale, onPick, panelNudgeClass }: { agents: readonly IntegrationAgentOption[]; value: string; locale: Locale; onPick: (id: string) => void; panelNudgeClass?: string }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLSpanElement | null>(null);
  useEffect(() => {
    if (!open) return undefined;
    const onDocClick = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);
  const selectedName = agents.find((agent) => agent.id === value)?.name || '';
  const label = selectedName
    ? integrationsT(locale, 'integrations.filterByAgentWithName', { name: selectedName })
    : integrationsT(locale, 'integrations.filterByAgent');
  return <span className="wk-vi-31" ref={rootRef}>
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-haspopup="listbox"
      aria-expanded={open}
      className={'wk-vi-157 ' + (value ? 'wk-vi-158' : 'wk-vi-159')}
      onClick={() => setOpen((current) => !current)}
    >
      {/* Vue IntegrationsAgentFilter.vue:6/8 — t-icon filter 14px + chevron-down 12px。 */}
      <SpriteIcon name="filter" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M19.5 4H4.5L10.5 12.5V20H13.5V12.5L19.5 4Z" /></svg>} />
      {selectedName ? <span className="wk-vi-32">{selectedName}</span> : null}
      <SpriteIcon name="chevron-down" size="12px" fallback={<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M17.5 9.5L12 15L6.5 9.5" /></svg>} />
    </button>
    {open ? <div role="listbox" aria-label={label} className={'wk-vi-33 ' + (panelNudgeClass ?? '')}>
      {[{ id: '', name: integrationsT(locale, 'integrations.filterAllAgents') }, ...agents].map((agent) => (
        <button
          type="button"
          role="option"
          key={agent.id || '__all__'}
          /* Vue IntegrationsAgentFilter.vue:40-47——active 只标真实命中的智能体；
             value=''（全部智能体）不发 active（B4 像素取证：React 旧码把 all 项
             判成选中渲染品牌色，Vue 端为普通项）。 */
          aria-selected={value !== '' && agent.id === value}
          className={'wk-vi-160 ' + (value !== '' && agent.id === value ? 'wk-vi-161' : 'wk-vi-162')}
          onClick={() => { onPick(agent.id); setOpen(false); }}
        >{agent.name}</button>
      ))}
    </div> : null}
  </span>;
}

function ChannelListPanel({ variant, copy, locale, items, agents, agentFilter, onAgentFilter, showCreate, onToggleCreate, canEdit, busy, t, renamingId, renameValue, onRenameValue, onStartRename, onSaveRename, onCancelRename, onOpenCard, onToggle, onDelete, imCreateSlot, embedCreateSlot }: {
  variant: 'im' | 'embed';
  copy: ChannelListCopy;
  locale: Locale;
  items: readonly IntegrationResource[];
  /** Vue IntegrationsAgentFilter options (bound-agent dropdown). */
  agents?: readonly IntegrationAgentOption[];
  agentFilter?: string;
  onAgentFilter?: (id: string) => void;
  showCreate: boolean;
  onToggleCreate: () => void;
  canEdit: boolean;
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
    {/* Vue channel-panel-list.less .channels-header: 8px gap, 12px bottom
        margin; the agent filter (IntegrationsAgentFilter) sits between the
        title and the count pill. leading values reproduce the Vue `normal`
        line boxes (14px→20px title, 12px→15px pill). */}
    <div className="wk-vi-34">
      <span className="wk-vi-35">{copy.channelsTitle}</span>
      {/* Vue renders IntegrationsAgentFilter unconditionally (an empty agent
          list just leaves the 全部智能体-only dropdown). */}
      {onAgentFilter ? <AgentFilterButton
        agents={agents ?? []}
        value={agentFilter ?? ''}
        locale={locale}
        onPick={onAgentFilter}
        /* B4：embed 页触发行横向分数坐标与 im 页不同（标签宽度差），popper 取整
           相位需单独微调（扫描口径 1280×720，互相关取证）。 */
        panelNudgeClass={variant === 'embed' ? 'wk-vi-33-embed' : undefined}
      /> : null}
      <span className="wk-vi-36">{items.length}</span>
    </div>
    {/* Vue IMChannelPanel/AgentEmbedChannelPanel: the empty description is a
        viewer-only branch — admins see the bare grid with the add tile. */}
    {items.length === 0 && !showCreate && !canEdit ? <div className="wk-vi-37"><p className="wk-status wk-vi-11">{copy.emptyText}</p></div> : null}
    <div className="wk-vi-38">
      {items.map((item) => {
        const platform = variant === 'im' && typeof item.platform === 'string' ? item.platform : '';
        const badgeText = platform ? imPlatformLabel(platform, locale).slice(0, 2) : '</>';
        const agentLine = typeof item.agent_name === 'string' && item.agent_name ? item.agent_name : typeof item.agent_id === 'string' && item.agent_id ? 'ID ' + item.agent_id : '';
        const name = editedNameOf(item) || item.name || copy.unnamedLabel;
        return <article
          className={onOpenCard ? CHANNEL_CARD_CLICKABLE_CLASS : CHANNEL_CARD_STATIC_CLASS}
          key={item.id}
          onClick={onOpenCard ? () => onOpenCard(item) : undefined}
          onKeyDown={onOpenCard ? (event) => {
            if (event.target !== event.currentTarget || (event.key !== 'Enter' && event.key !== ' ')) return;
            event.preventDefault();
            onOpenCard(item);
          } : undefined}
          role={onOpenCard ? 'button' : undefined}
          tabIndex={onOpenCard ? 0 : undefined}
          aria-label={onOpenCard ? name : undefined}
        >
          <span className={CHANNEL_BADGE_STATIC_CLASS} aria-hidden="true">{badgeText}</span>
          <div className={CHANNEL_CARD_BODY_CLASS}>
            <div className={CHANNEL_CARD_HEADER_CLASS}>
              <h3 className={CHANNEL_CARD_TITLE_STATIC_CLASS}>{name}</h3>
              {item.enabled === false ? <span className="wk-tag wk-tag--warning wk-vi-39">{copy.disabledLabel}</span> : null}
            </div>
            {agentLine ? <span className={CHANNEL_CARD_AGENT_CLASS}>{agentLine}</span> : null}
          </div>
          <div className={CHANNEL_CARD_ACTIONS_CLASS} onClick={(event) => event.stopPropagation()}>
            {onToggle ? <label className="wk-switch wk-vi-40" title={item.enabled === false ? t('agentEditor.im.enabled') : copy.disabledLabel} onClick={(event) => event.stopPropagation()}>
              <input className="wk-vi-41" type="checkbox" role="switch" aria-label={t('agentEditor.im.enabled')} checked={item.enabled !== false} onChange={() => onToggle(item.id)} />
              <span className="wk-switch-knob wk-switch-knob--vi wk-vi-42" aria-hidden="true" />
            </label> : null}
            {/* Vue edits both channel kinds through the wizard drawer opened by
                the card click, so the card keeps only the switch and delete. */}
            {onDelete ? <button className="wk-button wk-button--text wk-button--danger wk-vi-43" type="button" onClick={() => onDelete(item.id)}>{t('common.delete')}</button> : null}
          </div>
        </article>;
      })}
      {canEdit ? <button type="button" className={CHANNEL_CARD_ADD_CLASS} onClick={onToggleCreate}>
        <span className={CHANNEL_BADGE_ADD_CLASS} aria-hidden="true"><SpriteIcon name="add" size="20px" fallback="+" /></span>
        <div className={CHANNEL_CARD_BODY_CLASS}>
          <div className={CHANNEL_CARD_HEADER_CLASS}>
            <span className={CHANNEL_CARD_TITLE_ADD_CLASS}>{copy.addTileLabel}</span>
          </div>
        </div>
      </button> : null}
    </div>
    {/* B4：抽屉族换 SettingDrawer t-drawer 同构 chrome（Vue SettingDrawer.vue），
        槽位自渲染完整抽屉；旧 aside+× 关闭钮随 Vue closeBtn 关闭删除。 */}
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
  const submit = (event: React.FormEvent) => { event.preventDefault(); if (step < IM_WIZARD_STEPS.length - 1) onNext(); else onSave(); };
  const renderCredentialField = (item: ImCredentialField) => {
    if (item.type === 'switch') {
      return <label className="wk-check-row wk-vi-44" key={item.key}>
        <input type="checkbox" checked={form.credentials[item.key] === true} onChange={(event) => patch({ credentials: { ...form.credentials, [item.key]: event.target.checked } })} />
        {item.labelKey ? t(item.labelKey) : item.label}
        {item.hintKey ? <span className="wk-muted wk-vi-3">{t(item.hintKey)}</span> : null}
      </label>;
    }
    const value = form.credentials[item.key];
    const placeholder = item.placeholderKey ? t(item.placeholderKey) : item.placeholder;
    const hint = item.hintKey
      ? <span className="wk-muted wk-vi-3">{t(item.hintKey)}{item.hintLink ? <a className={INT_DOC_LINK_CLASS} href={item.hintLink.url} target="_blank" rel="noreferrer noopener"> {t(item.hintLink.labelKey)}</a> : null}</span>
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
  /* B4：Vue SettingDrawer 同构 chrome 化——title/subtitle/headerIcon/footer 由
     drawer 状态派生（IMChannelPanel.vue:72-96）；body 保留 form 元素与测试
     seam（form select / .wk-im-step.is-active / .wk-form-actions）。 */
  const stepTitles = IM_WIZARD_STEPS.map((item) => t(item.titleKey));
  const platformLogo = platformLogoRenderer ? platformLogoRenderer(form.platform) : null;
  const isLastStep = step >= IM_WIZARD_STEPS.length - 1;
  return <SettingDrawerChrome
    className="im-channel-drawer"
    width="560px"
    closeOnOverlayClick
    onClose={onCancel}
    headerIcon={platformLogo
      ? <img src={platformLogo} alt={imPlatformLabel(form.platform, locale)} className="drawer-platform-icon" />
      : <TIconLike name="chat-message" />}
    title={isEditing ? (form.name.trim() || t('agentEditor.im.unnamed')) : t('agentEditor.im.addChannel')}
    subtitle={stepTitles[step] ?? ''}
    footerLeft={step > 0 ? <TButtonLike variant="outline" onClick={onBack}>{t('integrations.wizard.back')}</TButtonLike> : undefined}
    footerRight={<div className="wk-form-actions">
      <TButtonLike variant="outline" onClick={onCancel}>{t('common.cancel')}</TButtonLike>
      <TButtonLike variant="base" theme="primary" disabled={busy || !canSubmit} onClick={() => { if (isLastStep) onSave(); else onNext(); }}>{isLastStep ? t('common.save') : t('integrations.wizard.next')}</TButtonLike>
    </div>}
    bodyChildren={<form className={INTEGRATION_FORM_CLASS} onSubmit={submit}>
    <div className="wk-im-steps" role="list">
      {IM_WIZARD_STEPS.map((item, index) => (
        <span role="listitem" key={item.key} className={step === index ? 'wk-im-step is-active' : step > index ? 'wk-im-step is-done' : 'wk-im-step'}>
          <span className="wk-im-step-num" aria-hidden="true">{step > index ? '✓' : index + 1}</span>
          <span className="wk-im-step-title">{t(item.titleKey)}</span>
        </span>
      ))}
    </div>
    {warning ? <p className="wk-status wk-status-error wk-vi-12" role="alert">{warning}</p> : null}

    {step === 0 ? <section className="setting-drawer__section im-drawer__section">
      <h4 className="setting-drawer__section-title">{t('agentEditor.im.sectionChannel')}</h4>
      {/* Vue gates the bound agent via validateWizardStep (warning toast), not
          native required validation — keep the same semantics here. */}
      <div className="form-item">
        <label className="form-label required">{t('integrations.boundAgent')}</label>
        <div className="agent-field-row">
          {agents.length > 0
            ? <select className={'im-drawer-select' + (form.targetAgentId ? '' : ' is-empty')} value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })}>
                <option value="" disabled>{t('integrations.selectAgentPlaceholder')}</option>
                {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
              </select>
            : <input value={form.targetAgentId} onChange={(event) => patch({ targetAgentId: event.target.value })} placeholder={t('integrations.selectAgentPlaceholder')} />}
        </div>
      </div>
      <div className="form-item">
        <label className="form-label required">{t('agentEditor.im.platform')}</label>
        {/* Vue disables the platform select while editing (line 114). 视觉层
            TSelectLike（t-select 实测 DOM）+ 原生 select 透明覆盖层保功能与
            测试 seam（form select 首个仍是 agent select）。 */}
        <div className="im-platform-select-wrap">
          <TSelectLike value={imPlatformLabel(form.platform, locale)} prefixLogo={platformLogo ?? undefined} prefixAlt={imPlatformLabel(form.platform, locale)} />
          <select className="im-drawer-native-overlay" value={form.platform} disabled={isEditing} onChange={(event) => onPlatformPicked(event.target.value)} aria-label={t('agentEditor.im.platform')}>
            {imPlatformOrder().map((key) => <option key={key} value={key}>{imPlatformLabel(key, locale)}</option>)}
          </select>
        </div>
      </div>
      <div className="form-item">
        <label className="form-label">{t('agentEditor.im.channelName')}</label>
        <TInputLike value={form.name} placeholder={t('agentEditor.im.channelNamePlaceholder')} onFocus={() => onNameTouched(true)} onChange={(next) => { onNameTouched(true); patch({ name: next }); }} />
        {!isEditing ? <p className="form-desc">{t('agentEditor.im.channelNameDefaultHint')}</p> : null}
      </div>
      {isEditing ? <label className="wk-check-row wk-vi-44">
        <input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />
        {t('agentEditor.im.enabled')}
      </label> : null}
    </section> : null}

    {step === 1 ? <div className="wk-im-step-body">
      {/* Vue hides the access section for wechat (fixed longpoll/full, line 149). */}
      {form.platform !== 'wechat' ? <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionAccess')}</legend>
        <label>{t('agentEditor.im.mode')}
          <span className="wk-vi-45" role="radiogroup" aria-label={t('agentEditor.im.mode')}>
            <button type="button" role="radio" aria-checked={form.mode === 'websocket'} className={chip(form.mode === 'websocket')} disabled={form.platform === 'mattermost'} onClick={() => patch({ mode: 'websocket' })}>WebSocket</button>
            <button type="button" role="radio" aria-checked={form.mode === 'webhook'} className={chip(form.mode === 'webhook')} onClick={() => patch({ mode: 'webhook' })}>Webhook</button>
          </span>
        </label>
        <p className="wk-muted wk-vi-3">{form.platform === 'mattermost' ? t('agentEditor.im.mattermostModeHint') : form.platform === 'yunzhijia' ? t('agentEditor.im.yunzhijiaModeHint') : t('agentEditor.im.modeHint')}</p>
        <label>{t('agentEditor.im.outputMode')}
          <span className="wk-vi-45" role="radiogroup" aria-label={t('agentEditor.im.outputMode')}>
            <button type="button" role="radio" aria-checked={form.outputMode === 'stream'} className={chip(form.outputMode === 'stream')} onClick={() => patch({ outputMode: 'stream' })}>{t('agentEditor.im.outputStream')}</button>
            <button type="button" role="radio" aria-checked={form.outputMode === 'full'} className={chip(form.outputMode === 'full')} onClick={() => patch({ outputMode: 'full' })}>{t('agentEditor.im.outputFull')}</button>
          </span>
        </label>
      </fieldset> : null}
      <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionSession')}</legend>
        <label>{t('agentEditor.im.sessionMode')}
          <span className="wk-vi-45" role="radiogroup" aria-label={t('agentEditor.im.sessionMode')}>
            <button type="button" role="radio" aria-checked={form.sessionMode === 'user'} className={chip(form.sessionMode === 'user')} onClick={() => patch({ sessionMode: 'user' })}>{t('agentEditor.im.sessionModeUser')}</button>
            <button type="button" role="radio" aria-checked={form.sessionMode === 'thread'} className={chip(form.sessionMode === 'thread')} disabled={!imPlatformSupportsThread(form.platform)} onClick={() => patch({ sessionMode: 'thread' })}>{t('agentEditor.im.sessionModeThread')}</button>
          </span>
        </label>
        <p className="wk-muted wk-vi-3">{t('agentEditor.im.sessionModeHint')}</p>
      </fieldset>
      {isEditing && form.mode === 'webhook' ? <fieldset className="wk-im-step-body">
        <legend className="wk-im-legend">{t('agentEditor.im.sectionCallback')}</legend>
        <label>{t('agentEditor.im.callbackUrl')}
          <span className={CODE_TOOLBAR_CLASS}>
            <input className="wk-mono-input wk-vi-46" readOnly value={imCallbackUrl(editing.id, apiBaseUrl)} />
            <button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" title={t('integrations.api.copy')} onClick={() => { void navigator.clipboard.writeText(imCallbackUrl(editing.id, apiBaseUrl)).catch(() => undefined); }}><CopyIcon /></button>
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
      <p className="wk-muted wk-vi-3">{t('agentEditor.im.fileKnowledgeBaseHint')}</p>
    </fieldset> : null}

    {step === 3 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('agentEditor.im.sectionCredentials')}</legend>
      {form.platform === 'wechat' ? <div>
        <p className="wk-muted wk-vi-3">{t('agentEditor.im.wechatHint')}</p>
        {bound ? <p className="wk-status wk-status-ok wk-vi-47" role="status">
          {t('agentEditor.im.wechatBindSuccess')}
          <button className="wk-button wk-button--text wk-vi-48" type="button" onClick={onStartWeChatBinding}>{t('agentEditor.im.wechatRebind')}</button>
        </p> : wechatQr ? <div>
          <img src={wechatQr.imgSrc} alt="WeChat QR Code" width={200} height={200} style={{ background: '#fff' }} />
          {wechatQr.status === 'expired' ? <button className="wk-button wk-vi-4" type="button" onClick={onStartWeChatBinding}>↻ {t('agentEditor.im.wechatQRExpired')}</button> : null}
          <p className="wk-muted wk-vi-3">{wechatQr.status === 'scaned' ? t('agentEditor.im.wechatBinding') : t('agentEditor.im.wechatScanning')}</p>
        </div> : <div>
          <button className="wk-button wk-vi-4" type="button" disabled={wechatQrLoading} onClick={onStartWeChatBinding}>{t('agentEditor.im.wechatScanBind')}</button>
        </div>}
        {wechatQrError ? <p className="wk-status wk-status-error wk-vi-12" role="alert">{wechatQrError}</p> : null}
      </div> : <div>
        {consoleLink ? <p className="wk-muted wk-vi-3">
          <a className={INT_DOC_LINK_CLASS} href={consoleLink.url} target="_blank" rel="noreferrer noopener">{t(consoleLink.labelKey)}</a>
          {' · '}{t('agentEditor.im.consoleTip')}
        </p> : null}
        {imCredentialFields(form.platform, form.mode).map(renderCredentialField)}
      </div>}
    </fieldset> : null}

    </form>} />;
}

// The embed wizard drawer (Vue AgentEmbedChannelPanel.vue SettingDrawer,
// lines 70-386): 渠道 → 安全限流 → 对话能力 → 外观展示 → 事件回调, plus the
// edit-only 部署 step with snippet tabs, server examples and the channel key
// controls. Copy follows the embedPublish.* verbatim fallback layer.
function EmbedWizardPanel({ t, apiBaseUrl, agents = [], title, form, onForm, onAgentPicked, step, steps, originsText, onOriginsText, onNameTouched, editing, detail, editingEnabled, onEditingEnabled, warning, status, snippetTab, onSnippetTab, serverTab, onServerTab, revealed, onReveal, onRotate, previewLoading, onPreview, busy, canEdit, canSubmit, onNext, onBack, onGoTo, onSave, onCancel }: {
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
  canEdit: boolean;
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
  /* B4：embed 抽屉换 SettingDrawer 同构 chrome（Vue AgentEmbedChannelPanel.vue:70，
     icon=code）；body 结构保持（embed 向导无面板扫描项，功能与测试 seam 不动）。 */
  const isLastEmbedStep = step >= steps.length - 1;
  return <SettingDrawerChrome
    className="embed-channel-drawer"
    width="560px"
    closeOnOverlayClick
    onClose={onCancel}
    headerIcon={<TIconLike name="code" />}
    title={title}
    subtitle={t(steps[step]?.titleKey ?? '')}
    footerLeft={step > 0 ? <TButtonLike variant="outline" onClick={onBack}>{t('integrations.wizard.back')}</TButtonLike> : undefined}
    footerRight={canEdit ? <div className="wk-form-actions">
      <TButtonLike variant="outline" onClick={onCancel}>{t('common.cancel')}</TButtonLike>
      <TButtonLike variant="base" theme="primary" disabled={busy || !canSubmit} onClick={() => { if (isLastEmbedStep) onSave(); else onNext(); }}>{isLastEmbedStep ? t('common.save') : t('integrations.wizard.next')}</TButtonLike>
    </div> : undefined}
    bodyChildren={<form className={INTEGRATION_FORM_CLASS + ' wk-embed-wizard wk-vi-163'} onSubmit={submit}>
    <div className="wk-im-steps" role="list">
      {steps.map((item, index) => (
        <button role="listitem" key={item.key} type="button" className={step === index ? 'wk-embed-step is-active' : step > index ? 'wk-embed-step is-done' : 'wk-embed-step'} onClick={() => onGoTo(index)}>
          <span className="wk-im-step-num" aria-hidden="true" style={step > index ? { background: '#eff4ff' } : step === index ? { background: '#2e6de6', color: '#fff', borderColor: '#2e6de6' } : undefined}>{step > index ? '✓' : index + 1}</span>
          <span className="wk-im-step-title">{t(item.titleKey)}</span>
        </button>
      ))}
    </div>
    <fieldset disabled={!canEdit} className="wk-vi-49">
    {warning ? <p className="wk-status wk-status-error wk-vi-12" role="alert">{warning}</p> : null}
    {status ? <p className="wk-status wk-status-ok wk-vi-47" role="status">{status}</p> : null}

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
      {isEditing ? <label className="wk-check-row wk-vi-44">
        <input type="checkbox" checked={editingEnabled} onChange={(event) => onEditingEnabled(event.target.checked)} />
        {t('embedPublish.enabled')}
      </label> : null}
      <label>{t('embedPublish.name')}
        <input value={form.name} onFocus={() => onNameTouched(true)} onChange={(event) => { onNameTouched(true); patch({ name: event.target.value }); }} placeholder={t('embedPublish.namePlaceholder')} />
      </label>
      <p className="wk-muted wk-vi-3">{isEditing ? t('embedPublish.nameDesc') : t('embedPublish.nameDefaultHint')}</p>
    </fieldset> : null}

    {step === 1 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionSecurity')}</legend>
      <label>{t('embedPublish.allowedOrigins')}
        <textarea rows={2} value={originsText} onChange={(event) => onOriginsText(event.target.value)} placeholder={t('embedPublish.originsPlaceholder')} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.originsHint')}</p>
      <label>{t('embedPublish.rateLimitLabel')}
        <input type="number" min={1} max={600} value={form.rateLimitPerMinute} onChange={(event) => { const next = Number(event.target.value); if (Number.isFinite(next)) patch({ rateLimitPerMinute: next }); }} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.rateLimitDesc')}</p>
      <label>{t('embedPublish.rateLimitDayLabel')}
        <input type="number" min={1} max={1000000} value={form.rateLimitPerDay} onChange={(event) => { const next = Number(event.target.value); if (Number.isFinite(next)) patch({ rateLimitPerDay: next }); }} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.rateLimitDayDesc')}</p>
    </fieldset> : null}

    {step === 2 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionCapabilities')}</legend>
      <label>{t('embedPublish.welcomeMessage')}
        <textarea rows={2} value={form.welcomeMessage} onChange={(event) => patch({ welcomeMessage: event.target.value })} placeholder={t('embedPublish.welcomePlaceholder')} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.welcomeMessageDesc')}</p>
      <label className="wk-check-row wk-vi-44">
        <input type="checkbox" checked={form.showSuggestedQuestions} onChange={(event) => patch({ showSuggestedQuestions: event.target.checked })} />
        <span>{t('embedPublish.showSuggestedQuestions')}<br />{t('embedPublish.showSuggestedQuestionsDesc')}</span>
      </label>
      <label className="wk-check-row wk-vi-44">
        <input type="checkbox" checked={form.allowWebSearch} onChange={(event) => patch({ allowWebSearch: event.target.checked })} />
        <span>{t('embedPublish.allowWebSearch')}<br />{t('embedPublish.allowWebSearchDesc')}</span>
      </label>
      {form.allowWebSearch && !agentWebSearchEnabled ? <p className="wk-muted wk-muted--warn wk-vi-50">{t('embedPublish.agentWebSearchDisabledHint')}</p> : null}
      <label className="wk-check-row wk-vi-44">
        <input type="checkbox" checked={form.allowFileUpload} onChange={(event) => patch({ allowFileUpload: event.target.checked })} />
        <span>{t('embedPublish.allowFileUpload')}<br />{t('embedPublish.allowFileUploadDesc')}</span>
      </label>
      {form.allowFileUpload && !agentImageUploadEnabled ? <p className="wk-muted wk-muted--warn wk-vi-50">{t('embedPublish.agentImageUploadDisabledHint')}</p> : null}
    </fieldset> : null}

    {step === 3 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionAppearance')}</legend>
      <label>{t('embedPublish.pageTitle')}
        <input value={form.pageTitle} onChange={(event) => patch({ pageTitle: event.target.value })} placeholder={t('embedPublish.pageTitlePlaceholder')} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.pageTitleDesc')}</p>
      <label>{t('embedPublish.headerTitleMode')}
        <select value={form.headerTitleMode} onChange={(event) => patch({ headerTitleMode: event.target.value as EmbedWizardForm['headerTitleMode'] })}>
          <option value="channel">{t('embedPublish.headerTitleModeChannel')}</option>
          <option value="session">{t('embedPublish.headerTitleModeSession')}</option>
        </select>
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.headerTitleModeDesc')}</p>
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
      <p className="wk-muted wk-vi-3">{t('embedPublish.defaultLocaleDesc')}</p>
      <label>{t('embedPublish.primaryColor')}
        <input type="color" value={form.primaryColor} onChange={(event) => patch({ primaryColor: event.target.value })} />
      </label>
      <label>{t('embedPublish.widgetPreview')}
        <span className={'wk-embed-widget-preview pos-' + form.widgetPosition}>
          <span className={EMBED_PREVIEW_LAUNCHER_CLASS} style={{ background: form.primaryColor }} aria-hidden="true" />
        </span>
      </label>
    </fieldset> : null}

    {step === 4 ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionWebhook')}</legend>
      <label>{t('embedPublish.webhookUrl')}
        <input autoComplete="off" value={form.webhookUrl} onChange={(event) => patch({ webhookUrl: event.target.value })} placeholder={t('embedPublish.webhookUrlPlaceholder')} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.webhookUrlDesc')}</p>
      <label>{t('embedPublish.webhookSecret')}
        <input type="password" autoComplete="new-password" value={form.webhookSecret} onChange={(event) => patch({ webhookSecret: event.target.value })} placeholder={secretPlaceholder} />
      </label>
      <p className="wk-muted wk-vi-3">{t('embedPublish.webhookSecretDesc')}</p>
    </fieldset> : null}
    {/* Vue renders the deploy-after-save hint inside step 5 for create mode. */}
    {step === 4 && !isEditing ? <div className="wk-embed-deploy-hint" role="note">ℹ️<p>{t('embedPublish.deployAfterSaveHint')}</p></div> : null}
    </fieldset>

    {/* Step 6 exists only while editing (Vue template v-else-if="editingId"). */}
    {step >= 5 && channel ? <fieldset className="wk-im-step-body">
      <legend className="wk-im-legend">{t('embedPublish.sectionDeploy')}</legend>
      <p className="wk-muted wk-vi-3">{t('embedPublish.deployIntro')}</p>
      <h5>{t('embedPublish.deployStepEmbed')}</h5>
      <p className="wk-muted wk-vi-3">{t('embedPublish.deployStepEmbedDesc')}</p>
      <div className="wk-embed-snippet-tabs wk-vi-45" role="tablist" aria-label={t('embedPublish.deployStepEmbed')}>
        {([['iframe', 'embedPublish.tabIframe'], ['widget', 'embedPublish.tabWidget'], ['secure', 'embedPublish.tabSecure']] as const).map(([value, key]) => (
          <button key={value} type="button" role="tab" aria-selected={snippetTab === value} className={chip(snippetTab === value)} onClick={() => onSnippetTab(value)}>{t(key)}</button>
        ))}
      </div>
      <p className="wk-muted wk-vi-3">{t(embedSnippetScenarioKey(snippetTab))}</p>
      {snippetTab === 'widget' ? <p className="wk-muted wk-vi-3">{t('embedPublish.widgetTokenNote')}</p> : null}
      {snippetTab === 'secure' ? <p className="wk-muted wk-vi-3">{t('embedPublish.secureTokenNote')}</p> : null}
      {snippetTab !== 'secure' ? <div className="wk-embed-deploy-hint" role="note">⚠️<p>{t('embedPublish.publishTokenWarning')}</p></div> : null}
      <div className="wk-embed-code-panel">
        <div className={CODE_TOOLBAR_CLASS}>
          <span>{snippetTab === 'iframe' ? t('embedPublish.embedCode') : t('embedPublish.widgetCode')}</span>
          <span>
            {snippetTab !== 'secure' ? <button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" disabled={previewLoading || busy} onClick={() => onPreview(channel)}>{previewLoading ? t('common.loading') : t('embedPublish.preview')}</button> : null}
            <button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" onClick={() => { void navigator.clipboard.writeText(snippet).catch(() => undefined); }}>{t('embedPublish.copyCode')}</button>
          </span>
        </div>
        <pre>{snippet}</pre>
      </div>
      {snippetTab === 'secure' ? <div>
        <p className="wk-muted wk-vi-3">{t('embedPublish.secureServerLabel')}</p>
        <div className="wk-embed-server-tabs wk-vi-45" role="tablist" aria-label={t('embedPublish.secureServerLabel')}>
          {([['node', 'embedPublish.tabServerNode'], ['go', 'embedPublish.tabServerGo']] as const).map(([value, key]) => (
            <button key={value} type="button" role="tab" aria-selected={serverTab === value} className={chip(serverTab === value)} onClick={() => onServerTab(value)}>{t(key)}</button>
          ))}
        </div>
        <div className="wk-embed-server-panel">
          <div className={CODE_TOOLBAR_CLASS}>
            <span>{serverTab === 'go' ? t('embedPublish.tabServerGo') : t('embedPublish.tabServerNode')}</span>
            <button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" onClick={() => { void navigator.clipboard.writeText(serverExample).catch(() => undefined); }}>{t('embedPublish.copyCode')}</button>
          </div>
          <pre>{serverExample}</pre>
        </div>
      </div> : null}
      <h5>{t('embedPublish.channelKey')}</h5>
      <p className="wk-muted wk-vi-3">{t('embedPublish.channelKeyDesc')}</p>
      <div className="wk-channel-key-control">
        <input className="wk-mono-input wk-embed-key-input wk-vi-46" readOnly type="text" value={embedChannelKeyDisplay(token, revealed)} placeholder={token ? '' : t('embedPublish.channelKeyUnavailable')} aria-label={t('embedPublish.channelKey')} />
        {token ? <button className="wk-button wk-button--text wk-vi-48" type="button" title={revealed ? t('embedPublish.hideKey') : t('embedPublish.revealKey')} onClick={onReveal}>{revealed ? '🙈' : '👁'}</button> : null}
        {token ? <button className="wk-button wk-button--text wk-vi-48" type="button" title={t('embedPublish.copyChannelKeyTitle')} onClick={() => { void navigator.clipboard.writeText(token).catch(() => undefined); }}><CopyIcon /></button> : null}
        {canSubmit ? <button className="wk-button wk-button--text wk-button--danger wk-vi-43" type="button" title={t('embedPublish.resetKeyTitle')} disabled={busy} onClick={() => onRotate(channelId)}>{busy ? t('common.loading') : '↻'}</button> : null}
      </div>
      {!token ? <p className="wk-muted wk-vi-3">{t('embedPublish.channelKeyHint')}</p> : null}
    </fieldset> : null}

    </form>} />;
}


function ApiIntegrationPanel({ apiBaseUrl, actions, principalMode, setPrincipalMode, requireDirectHeader, setRequireDirectHeader, hmacSecret, setHmacSecret, externalUserId, setExternalUserId, principalToken, onSavePrincipal, onCreatePrincipalToken, busy, apiKeys, apiKeysLoading, freshApiKeyId, knowledgeBases, showApiKeyForm, setShowApiKeyForm, onCreateApiKey, onRevokeApiKey, onCopyApiKey, onOpenApiPlayground, t }: {
  apiBaseUrl: string;
  swaggerEnabled?: boolean;
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
  /** Vue loadKnowledgeBaseOptions feeds the create-dialog KB scope select. */
  knowledgeBases?: readonly IntegrationKnowledgeBaseOption[];
  showApiKeyForm?: boolean;
  setShowApiKeyForm?: (value: boolean) => void;
  onCreateApiKey?: (payload: ApiKeyCreatePayload) => void;
  onRevokeApiKey?: (key: ApiKeyRow) => void;
  onCopyApiKey?: (key: ApiKeyRow) => void;
  onOpenApiPlayground?: () => void;
  t: Translator;
}) {
  const principal = actions.principal;
  // Vue useApiBaseUrlDisplay: configured base, else window origin + '/api/v1'
  // — the raw apiBaseUrl prop may be empty when settings embeds the page.
  const apiBaseDisplay = apiBaseUrl || (typeof window === 'undefined' ? '' : `${window.location.origin}/api/v1`);
  // Vue requestExample: curl session-create + SSE agent-chat with the
  // principal headers per mode (ApiIntegrationSettings.vue L1081-1114).
  const principalHeaders: string[] = [];
  if (principalMode === 'direct_header') principalHeaders.push('  -H "X-External-User-ID: user_123"');
  if (principalMode === 'signed_token') principalHeaders.push(`  -H "X-External-User-Token: ${t('integrations.api.requestExampleJwtPlaceholder')}"`);
  const commonHeaders = ['  -H "X-API-Key: <YOUR_API_KEY>"', '  -H "Content-Type: application/json"', ...principalHeaders].join(' \\\n');
  const requestExample = [
    t('integrations.api.requestExampleCreateSession'),
    `curl -X POST ${apiBaseDisplay}/sessions \\`,
    commonHeaders,
    `  -d '{}'`,
    '',
    t('integrations.api.requestExampleAgentChat'),
    `curl -N -X POST ${apiBaseDisplay}/agent-chat/<session_id> \\`,
    commonHeaders,
    `  -d '{"query":"hello","agent_enabled":true,"agent_id":"agent-smart-reasoning","channel":"api"}'`,
  ].join('\n');
  const openApiDoc = () => { window.open('https://github.com/Tencent/WeKnora/blob/main/docs/api/README.md', '_blank', 'noopener'); };
  return <div className="api-settings wk-vi-51">
    <section className="wk-vi-52">
      <div className="wk-vi-53">
        <div>
          <label className="wk-vi-54">{t('integrations.api.baseUrl')}</label>
          <p className="wk-vi-55">{t('integrations.api.baseUrlDesc')}</p>
        </div>
        <div className="wk-vi-56">
          <input className="wk-vi-57" readOnly value={apiBaseDisplay} aria-label={t('integrations.api.baseUrl')} />
          <button className="wk-vi-58" type="button" title={t('integrations.api.copy')} aria-label={t('integrations.api.copy')} onClick={() => { void navigator.clipboard.writeText(apiBaseDisplay).catch(() => undefined); }}><CopyIcon /></button>
        </div>
      </div>
      {/* Vue .row--doc renders unconditionally (ApiIntegrationSettings.vue
          L78-89): 打开文档 opens the static GitHub API guide, no swagger flag. */}
      <div className="wk-vi-59">
        <div>
          <label className="wk-vi-54">{t('tenant.api.docLabel')}</label>
          <p className="wk-vi-55">{t('tenant.api.docDescription')}{' '}<a className="wk-vi-60" onClick={openApiDoc}>{t('tenant.api.openDoc')}<LandingIcon name="url" size={13} /></a></p>
        </div>
      </div>
      <div className="wk-vi-61">
        <div className="wk-vi-62">
          <div className="wk-vi-6">
            <label className="wk-vi-63">{t('integrations.api.apiKeys')}</label>
            <p className="wk-vi-64">{t('integrations.api.apiKeysDesc')}</p>
          </div>
          <button className="wk-vi-65" type="button" onClick={() => setShowApiKeyForm?.(!showApiKeyForm)}><PlusIcon />{` ${t('integrations.api.createApiKey')}`}</button>
        </div>
      {freshApiKeyId !== null ? <p className="wk-status wk-status-ok wk-vi-47" role="status">{t('integrations.api.apiKeyCreated')} · {t('integrations.api.secretSavedCopyHint')}</p> : null}
      {showApiKeyForm ? <ApiKeyCreateForm t={t} busy={busy} knowledgeBases={knowledgeBases ?? []} canSubmit={Boolean(actions.onCreateApiKey)} onCreate={onCreateApiKey} onCancel={() => setShowApiKeyForm?.(false)} /> : null}
      <div className="wk-vi-66">
      {apiKeysLoading ? <div className="wk-vi-67">{t('integrations.api.loading')}</div> : (apiKeys ?? []).length === 0 ? <div className="wk-vi-67">{t('integrations.api.noApiKeys')}</div> : <div className="wk-vi-68">
        <table className="wk-vi-69">
          <thead><tr>
            <th className="wk-vi-70">{t('integrations.api.apiKeyName')}</th>
            <th className="wk-vi-70">{t('integrations.api.apiKeyValue')}</th>
            <th className="wk-vi-70">{t('integrations.api.apiKeyAccessMode')}</th>
            <th className="wk-vi-70">{t('integrations.api.createdAt')}</th>
            <th className="wk-vi-70">{t('integrations.api.actions')}</th>
          </tr></thead>
          <tbody>
            {(apiKeys ?? []).map((key) => {
              const reveal = isFreshKeyVisible({ fresh: key.id === freshApiKeyId, hasValue: key.api_key !== '' });
              return <tr key={String(key.id)}>
                <td className="wk-vi-71">{key.name}</td>
                <td className="wk-vi-71"><code className="wk-vi-72">{apiKeyValueDisplay(key, reveal)}</code></td>
                <td className="wk-vi-71">{apiKeyAccessMode(key)}</td>
                <td className="wk-vi-71">{key.created_at ?? ''}</td>
                <td className="wk-vi-73">
                  {key.api_key ? <button className="wk-button wk-vi-4" type="button" onClick={() => onCopyApiKey?.(key)}>{t('integrations.api.copy')}</button> : null}
                  <button className="wk-button wk-button--danger wk-vi-4" type="button" onClick={() => onRevokeApiKey?.(key)}>{t('integrations.api.deleteApiKey')}</button>
                </td>
              </tr>;
            })}
          </tbody>
        </table>
      </div>}
      </div>
      </div>
    </section>

    {/* Vue .principal-section: bare band (border-top only), header + joined
        t-radio-group + per-mode detail + the curl 请求示例 code panel and the
        playground-entry row (ApiIntegrationSettings.vue L196-336). The
        external-user/token controls live in the Vue playground drawer, so they
        stay behind the non-tenant mode detail here. */}
    <section className="wk-vi-74">
      <div>
        <label className="wk-vi-63">{t('integrations.api.principalMode')}</label>
        <p className="wk-vi-64">{t('integrations.api.principalModeDesc')}</p>
        <p className="wk-vi-75">{t('integrations.api.principalScope')}</p>
      </div>
      <div className="wk-vi-76" role="radiogroup" aria-label={t('integrations.api.principalMode')}>
        {([['tenant', 'integrations.api.modeTenant'], ['direct_header', 'integrations.api.modeDirect'], ['signed_token', 'integrations.api.modeSigned']] as const).map(([value, key], index) => (
          <button key={value} type="button" role="radio" aria-checked={principalMode === value} className={'wk-vi-seg wk-vi-164 ' + (index > 0 ? 'wk-vi-seg-gap ' : '') + (principalMode === value ? 'wk-vi-165' : 'wk-vi-166') + (index === 2 ? ' wk-vi-167' : '')} onClick={() => setPrincipalMode(value)}><span>{t(key)}</span></button>
        ))}
      </div>
      {principalMode === 'direct_header' ? <div className="wk-vi-77">
        {/* Vue ApiIntegrationSettings.vue:211-219 mode-callout--warning（B4 静态项
            settings-integration-api 4.284% 归因：React 缺警示框+detail 第二行）。 */}
        <div className="mode-callout mode-callout--warning">
          <div className="mode-callout__body">
            <strong>{t('integrations.api.directWarning')}</strong>
            <p>{t('integrations.api.directWarningDetail')}</p>
          </div>
        </div>
        <label className="wk-check-row wk-vi-44"><input className="wk-vi-78 wk-vi-accent-primary" type="checkbox" checked={requireDirectHeader} onChange={(event) => setRequireDirectHeader(event.target.checked)} />{t('integrations.api.requireDirectHeader')}</label>
        <p className="wk-muted wk-vi-3">{t('integrations.api.requireDirectHeaderDesc')}</p>
      </div> : null}
      {principalMode === 'signed_token' ? <div className="wk-vi-77">
        <div className="mode-callout">
          <div className="mode-callout__body">
            <strong>{t('integrations.api.signedRecommended')}</strong>
            <p>{t('integrations.api.signedFlowDetail')}</p>
          </div>
        </div>
        <label className="wk-vi-79">{t('integrations.api.hmacSecret')}<input className="wk-vi-80" type="password" value={hmacSecret} onChange={(event) => setHmacSecret(event.target.value)} placeholder={principal?.has_hmac_secret ? t('integrations.api.secretConfigured') : ''} /></label>
        <p className="wk-muted wk-vi-3">{t('integrations.api.hmacSecretDesc')}</p>
      </div> : null}
      {principalMode !== 'tenant' ? <>
        <div className="wk-form-actions">
          <button className="wk-button wk-vi-4" type="button" disabled={busy} onClick={onSavePrincipal}>{t('common.save')}</button>
        </div>
        <div className="wk-vi-81">
          <label className="wk-vi-79">{t('integrations.api.playgroundExternalUser')}<input className="wk-vi-80" value={externalUserId} onChange={(event) => setExternalUserId(event.target.value)} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} /></label>
          <div className="wk-form-actions">
            <button className="wk-button wk-vi-4" type="button" disabled={busy} onClick={onCreatePrincipalToken}>{t('integrations.api.generateSecret')}</button>
          </div>
          {principalToken ? <p className="wk-status wk-vi-11">{t('integrations.api.playgroundGeneratedToken')}: <code>{principalToken.token}</code> ({principalToken.headerName})</p> : null}
        </div>
      </> : null}
      {/* Vue .examples .code-panel (L305-324): white toolbar with the
          请求示例 label + icon 复制 button over the mono curl pre. */}
      <div className="wk-vi-82">
        <div className="wk-vi-83">
          <div className="wk-vi-84">
            <span className="wk-vi-85">{t('integrations.api.requestExample')}</span>
            <button className="wk-vi-86" type="button" onClick={() => { void navigator.clipboard.writeText(requestExample).catch(() => undefined); }}><SpriteIcon name="file-copy" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="M14 2V8H20M14 2H7V18H20V8" /><path d="M3 6L3 22H14" /></svg>} />{t('integrations.api.copy')}</button>
          </div>
          <pre className="wk-vi-87">{requestExample}</pre>
        </div>
      </div>
      <div className="wk-vi-88">
        <div className="wk-vi-6">
          <label className="wk-vi-89">{t('integrations.api.playgroundTitle')}</label>
          <p className="wk-vi-90">{t('integrations.api.playgroundDesc')}</p>
        </div>
        {onOpenApiPlayground ? <button className="wk-vi-91" type="button" onClick={onOpenApiPlayground}><LandingIcon name="code" size={16} />{t('integrations.api.playgroundOpen')}</button> : null}
      </div>
    </section>
  </div>;
}

// The create-key drawer (Vue ApiIntegrationSettings.vue L462-564): 名称,
// 访问类型 radio (能力授权 / 空间完全访问) with its switching hint, the
// four-group capability matrix (知识库数据 / 智能体与集成 / 成员与空间 / 空间配置)
// with a 全选/清空 toggle per group and a checkbox + hint per capability, and
// the 知识库范围 multi-select that only applies below full access while a
// KB-scoped capability is selected. The submit payload is assembled by
// buildApiKeyCreatePayload to stay byte-equivalent to Vue createScopedAPIKey.
function ApiKeyCreateForm({ t, busy, knowledgeBases, canSubmit, onCreate, onCancel }: {
  t: Translator;
  busy: boolean;
  knowledgeBases: readonly IntegrationKnowledgeBaseOption[];
  canSubmit: boolean;
  onCreate?: (payload: ApiKeyCreatePayload) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [fullAccess, setFullAccess] = useState(false);
  const [selections, setSelections] = useState<ApiKeyCapabilitySelections>(createDefaultApiKeySelections);
  const [knowledgeBaseIds, setKnowledgeBaseIds] = useState<string[]>([]);
  const [warning, setWarning] = useState('');
  const selected = selectedApiKeyCapabilities(selections);
  const knowledgeScopeApplies = apiKeyKnowledgeScopeApplies(fullAccess, selected);
  const groupAllSelected = (group: (typeof TENANT_API_KEY_CAPABILITY_GROUPS)[number]) => group.capabilities.every((capability) => selections[capability.value]);
  const toggleGroup = (group: (typeof TENANT_API_KEY_CAPABILITY_GROUPS)[number], selected: boolean) => setSelections((current) => {
    const next = { ...current };
    group.capabilities.forEach((capability) => { next[capability.value] = selected; });
    return next;
  });
  /* B4/B6：Vue ApiIntegrationSettings.vue:462-560 的 api-key-create-drawer 同构——
     SettingDrawer(lock-on 图标、遮罩点击不关) + .api-key-dialog 行结构（绿杆
     label + t-input / mode-radio / capability 分组），footer 取消+创建。 */
  const submitAction = () => {
    const trimmed = name.trim();
    if (!trimmed) { setWarning(t('integrations.api.apiKeyNameRequired')); return; }
    if (!fullAccess && selected.length === 0) { setWarning(t('integrations.api.apiKeyCapabilitiesRequired')); return; }
    setWarning('');
    onCreate?.(buildApiKeyCreatePayload({ name: trimmed, fullAccess, capabilities: selected, knowledgeBaseIds }));
  };
  const submit = (event?: React.FormEvent) => { event?.preventDefault(); submitAction(); };
  return <SettingDrawerChrome
    className="api-key-create-drawer"
    width="560px"
    closeOnOverlayClick={false}
    onClose={onCancel}
    headerIcon={<SpriteIcon name="lock-on" size="16px" fallback={<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><rect x="4" y="11" width="16" height="10" rx="2" /><path d="M8 11V7a4 4 0 018 0v4" /></svg>} />}
    title={t('integrations.api.createApiKey')}
    subtitle={t('integrations.api.createApiKeyDialogDesc')}
    footerRight={<div className="wk-form-actions">
      <TButtonLike variant="outline" onClick={onCancel}>{t('common.cancel')}</TButtonLike>
      <TButtonLike variant="base" theme="primary" disabled={busy || !canSubmit} onClick={() => submit()}>{t('integrations.api.createApiKey')}</TButtonLike>
    </div>}
    bodyChildren={<form className="api-key-dialog" onSubmit={submit}>
      {warning ? <p className="wk-status wk-status-error wk-vi-12" role="alert">{warning}</p> : null}
      <div className="api-key-dialog-row">
        <div className="api-key-dialog-row__label"><label>{t('integrations.api.apiKeyName')}</label></div>
        <TInputLike value={name} placeholder={t('integrations.api.apiKeyNamePlaceholder')} onChange={setName} onEnter={() => submit()} />
      </div>
      <div className="api-key-dialog-row">
        <div className="api-key-dialog-row__label"><label>{t('integrations.api.apiKeyAccessType')}</label></div>
        <TRadioGroupLike
          value={fullAccess ? 'full' : 'scoped'}
          options={[{ value: 'scoped', label: t('integrations.api.apiKeyScopedAccess') }, { value: 'full', label: t('integrations.api.capabilityTenantFull') }]}
          onPick={(next) => setFullAccess(next === 'full')}
          className="mode-radio api-key-access-type-radio"
        />
        <p className="scope-hint">{t(fullAccess ? 'integrations.api.capabilityTenantFullHint' : 'integrations.api.apiKeyAccessTypeHint')}</p>
      </div>
      {!fullAccess ? <div className="api-key-dialog-row">
        <div className="api-key-dialog-row__label"><label>{t('integrations.api.apiKeyCapabilities')}</label></div>
        <div className="api-key-capability-list">
          {TENANT_API_KEY_CAPABILITY_GROUPS.map((group) => <div className="api-key-capability-group" key={group.key}>
            <div className="api-key-capability-group__header">
              <span>{t(group.labelKey)}</span>
              <button className="wk-button wk-button--text wk-vi-48" type="button" onClick={() => toggleGroup(group, !groupAllSelected(group))}>{t(groupAllSelected(group) ? 'integrations.api.apiKeyCapabilityClearGroup' : 'integrations.api.apiKeyCapabilitySelectGroup')}</button>
            </div>
            <div className="api-key-capability-group__items">
              {group.capabilities.map((capability) => <div className="api-key-capability-item" key={capability.value}>
                <TCheckboxLike checked={selections[capability.value]} onChange={(next) => setSelections((current) => ({ ...current, [capability.value]: next }))}>{t(capability.labelKey)}</TCheckboxLike>
                <p className="scope-hint">{t(capability.hintKey)}</p>
              </div>)}
            </div>
          </div>)}
        </div>
      </div> : null}
      {knowledgeScopeApplies ? <div className="api-key-dialog-row">
        <div className="api-key-dialog-row__label"><label>{t('integrations.api.apiKeyKnowledgeScope')}</label></div>
        <select multiple value={knowledgeBaseIds} size={Math.min(6, Math.max(3, knowledgeBases.length || 3))} onChange={(event) => setKnowledgeBaseIds(Array.from(event.target.selectedOptions).map((option) => option.value))}>
          {knowledgeBases.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
        </select>
        <p className="scope-hint">{t('integrations.api.apiKeyKnowledgeScopePlaceholder')}</p>
      </div> : null}
    </form>} />;
}

function ExternalLandingPanel({ tab, locale, externalUrl, apiBaseUrl, onOpenApiSettings, t }: { tab: IntegrationKey; locale: Locale; externalUrl?: string; apiBaseUrl: string; onOpenApiSettings?: () => void; t: Translator }) {
  const cta = tab === 'cli'
    ? { label: t('integrations.cli.docs'), hint: t('integrations.cli.docsHint') }
    : tab === 'chrome'
      ? { label: t('integrations.chrome.installCta'), hint: t('integrations.chrome.installCtaHint') }
      : { label: t('integrations.claw.installCta'), hint: t('integrations.claw.installCtaHint') };
  const copy = (value: string) => { void navigator.clipboard.writeText(value).catch(() => undefined); };
  // Vue useApiBaseUrlDisplay: configured base, else window origin + '/api/v1'
  // — the raw apiBaseUrl prop may be empty when settings embeds the page.
  const apiBaseDisplay = apiBaseUrl || (typeof window === 'undefined' ? '' : `${window.location.origin}/api/v1`);
  const cliConnectCommand = buildCLIConnectCommand(apiBaseDisplay, typeof window === 'undefined' ? '' : window.location.origin);
  const cliSteps = [
    { key: 'install', title: t('integrations.cli.installTitle'), desc: t('integrations.cli.installDesc'), command: 'git clone https://github.com/Tencent/WeKnora.git\ncd WeKnora/cli\ngo build -o weknora .\nexport PATH="$PWD:$PATH"' },
    { key: 'connect', title: t('integrations.cli.connectTitle'), desc: t('integrations.cli.connectDesc'), command: cliConnectCommand },
    { key: 'verify', title: t('integrations.cli.verifyTitle'), desc: t('integrations.cli.verifyDesc'), command: 'weknora doctor\nweknora kb list' },
  ];
  // Vue base orders: ChromeExtensionLanding.vue L104-106 /
  // ClawSkillLanding.vue L119-120.
  const chromeCapabilities = ['qa', 'clip', 'notes', 'shortcuts'] as const;
  const chromeSteps = ['api', 'port', 'install', 'connect'] as const;
  const clawCapabilities = ['upload', 'url', 'manual', 'search', 'browse'] as const;
  const clawSteps = ['api', 'env', 'install', 'verify'] as const;
  const openExternal = () => { if (externalUrl) window.open(externalUrl, '_blank', 'noopener,noreferrer'); };
  const isClaw = tab === 'claw';
  // Vue integration-landing.less: brand = --td-brand-color #07c05f; claw swaps
  // in @claw-accent #e85d2a / @claw-accent-dark #c44d1f.
  const accent = isClaw ? '#c44d1f' : undefined;
  const brand = isClaw ? '#e85d2a' : '#07c05f';
  const brandDark = accent ?? '#07c05f';
  // Vue landing-hero background (135deg brand tint fading to the container) +
  // the inset top highlight.
  const heroBackground = isClaw
    ? 'linear-gradient(135deg, rgba(232,93,42,0.12) 0%, rgba(232,93,42,0.05) 45%, #ffffff 72%)'
    : 'linear-gradient(135deg, rgba(7,192,95,0.10) 0%, rgba(7,192,95,0.04) 42%, #ffffff 72%)';
  const heroInsetShadow = isClaw ? 'inset 0 1px 0 rgba(232,93,42,0.10)' : 'inset 0 1px 0 rgba(7,192,95,0.08)';
  // .ext-cta border: color-mix(brand 28%, --td-component-stroke #e7e7e7)
  // = rgb(231,190,174) for claw / rgb(168,220,193) for chrome.
  const ctaBorder = isClaw ? '#e7beae' : '#a8dcc1';
  const envExample = `export WEKNORA_BASE_URL="${apiBaseDisplay || 'https://your-server.com/api/v1'}"\nexport WEKNORA_API_KEY="sk-your-api-key"`;
  // Vue IntegrationLandingLayout: hero, external CTA, constrained two-column
  // content and footer metadata are part of the page contract, not decoration.
  return <div className={'integration-landing wk-vi-168' + (tab === 'claw' ? ' integration-landing--claw' : '')}>
    <header className="landing-hero wk-vi-99" style={{ background: heroBackground, boxShadow: heroInsetShadow }}>
      <div className="wk-vi-100">
        <h2 className="wk-vi-101">{t(`integrations.${tab}.title`)}</h2>
        <p className="wk-vi-64">{t(`integrations.${tab}.subtitle`)}</p>
        {tab === 'chrome' ? <div className="wk-vi-102">{['research', 'learning', 'tech', 'work'].map((key) => <span key={key} className="wk-vi-103">{t(`integrations.chrome.scenarios.${key}`)}</span>)}</div> : null}
        {/* Vue ext-cta is a <button style="font: inherit">; Tailwind preflight
            would otherwise leave the label/hint on the UA button font (Arial).
            The body column carries Vue's gap:1px + justify-center. */}
        <button type="button" className="ext-cta wk-vi-104" style={{ borderColor: ctaBorder }} onClick={openExternal}>
          <span className="wk-vi-105" style={{ background: isClaw ? 'color-mix(in srgb, #e85d2a 12%, #fff)' : 'color-mix(in srgb, #07c05f 10%, #fff)', color: brandDark }}>{tab === 'cli' ? <LandingIcon name="code" size={14} /> : tab === 'chrome' ? <LandingIcon name="extension" size={18} /> : <span aria-hidden="true" className="wk-vi-106">🦞</span>}</span>
          <span className="wk-vi-107"><span className="wk-vi-108">{cta.label}</span><span className="wk-vi-109">{cta.hint}</span></span>
          <span aria-hidden="true" className="wk-vi-110" style={{ background: isClaw ? 'rgba(232,93,42,0.1)' : 'rgba(7,192,95,0.08)', color: isClaw ? '#c44d1f' : 'rgba(0,0,0,0.6)' }}><JumpIcon /></span>
        </button>
      </div>
    </header>
    <div className="wk-vi-111">
      <div className="wk-vi-112"><div className="wk-vi-113">
    {/* Vue .setting-drawer__section: padding 12px 0 14px, first-child pt 10,
        last-child pb 10 + border-bottom none. Each landing panel mounts a
        single section, so it renders 10px/10px with no divider. */}
    {tab === 'cli' ? <section className="wk-vi-114">
      <LandingSectionHead label={t('integrations.cli.quickstart')} />
      {/* Vue landing-steps/landing-step: 9px row padding with row dividers. */}
      <ol className="wk-vi-115">
        {cliSteps.map((step, index) => <li key={step.key} className="wk-vi-116">
          <span className="wk-vi-117">{index + 1}</span>
          <div className="wk-landing-step-body wk-vi-100">
            <div className="wk-vi-118">{step.title}</div>
            <p className="wk-vi-119">{step.desc}</p>
            <div className="wk-vi-120"><LandingCodeToolbar code={step.command} copyLabel={t('integrations.cli.copy')} onCopy={() => copy(step.command)} /></div>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {false && tab === 'cli' ? <section className="wk-vi-121">
      <h4 className="wk-vi-122">{t('integrations.cli.commandsTitle')}</h4>
      <p className="wk-muted wk-vi-123">{t('integrations.cli.commandsDesc')}</p>
      <div className={CODE_TOOLBAR_CLASS}><pre className={CODE_TOOLBAR_PRE_CLASS}>{'weknora doc upload ./document.pdf --kb "KB_ID"\nweknora search chunks "query" --kb "KB_ID"\nweknora chat "question" --kb "KB_ID" --format text\nweknora agent list'}</pre><button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" title={t('integrations.cli.copy')} onClick={() => copy('weknora doc upload')}><CopyIcon /></button></div>
    </section> : null}
    {false && tab === 'cli' ? <section className="wk-vi-121">
      <h4 className="wk-vi-122">{t('integrations.cli.mcpTitle')}</h4>
      <p className="wk-muted wk-vi-123">{t('integrations.cli.mcpDesc')}</p>
      <div className={CODE_TOOLBAR_CLASS}><pre className={CODE_TOOLBAR_PRE_CLASS}>{JSON.stringify({ mcpServers: { weknora: { command: 'weknora', args: ['--profile', 'weknora', 'mcp', 'serve'] } } }, null, 2)}</pre><button className={'wk-button wk-button--text wk-vi-48 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" title={t('integrations.cli.copy')} onClick={() => copy('mcp')}><CopyIcon /></button></div>
    </section> : null}
    {tab === 'chrome' ? <section className="wk-vi-114">
      <LandingSectionHead label={t('integrations.chrome.capabilitiesTitle')} count={chromeCapabilities.length} />
      <div className="wk-vi-124">
        {chromeCapabilities.map((key) => <div key={key} className="wk-vi-125">
          <span className="wk-vi-126"><LandingIcon name={key} size={14} /></span>
          <h5 className="wk-vi-127">{t('integrations.chrome.capabilities.' + key + '.title')}</h5>
          <p className="wk-vi-90">{t('integrations.chrome.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {false && tab === 'chrome' ? <section className="wk-vi-121">
      <h4 className="wk-vi-122">{t('integrations.chrome.stepsTitle')}</h4>
      <ol className="wk-vi-128">
        {chromeSteps.map((key, index) => <li key={key} className="wk-vi-129">
          <span className="wk-vi-130">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="wk-vi-131">{t('integrations.chrome.steps.' + key + '.title')}</div>
            <p className="wk-vi-132">{t('integrations.chrome.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
    {tab === 'claw' ? <section className="wk-vi-114">
      <LandingSectionHead label={t('integrations.claw.capabilitiesTitle')} count={clawCapabilities.length} accent={accent} barColor="#e85d2a" />
      <div className="wk-vi-124">
        {clawCapabilities.map((key, index) => <div key={key} className={'wk-vi-125' + (isClaw && index === clawCapabilities.length - 1 && clawCapabilities.length % 2 === 1 ? ' wk-vi-169' : '')}>
          <span className="wk-vi-133" style={accent ? { color: accent, background: 'color-mix(in srgb, #e85d2a 10%, #fff)' } : undefined}><LandingIcon name={key} size={14} /></span>
          <h5 className="wk-vi-127">{t('integrations.claw.capabilities.' + key + '.title')}</h5>
          <p className="wk-vi-90">{t('integrations.claw.capabilities.' + key + '.desc')}</p>
        </div>)}
      </div>
    </section> : null}
    {false && tab === 'claw' ? <section className="wk-vi-121">
      <h4 className="wk-vi-122">{t('integrations.claw.stepsTitle')}</h4>
      <ol className="wk-vi-128">
        {clawSteps.map((key, index) => <li key={key} className="wk-vi-129">
          <span className="wk-vi-130">{index + 1}</span>
          <div className="wk-landing-step-body">
            <div className="wk-vi-131">{t('integrations.claw.steps.' + key + '.title')}</div>
            <p className="wk-vi-132">{t('integrations.claw.steps.' + key + '.desc')}</p>
          </div>
        </li>)}
      </ol>
    </section> : null}
      </div></div>
      <aside className="wk-vi-112"><div className="wk-vi-113">
        {tab === 'cli' ? <>
          <section className="wk-vi-134"><LandingSectionHead label={t('integrations.cli.commandsTitle')} /><p className="wk-vi-135">{t('integrations.cli.commandsDesc')}</p><LandingCodeToolbar code={'weknora doc upload ./document.pdf --kb "KB_ID"\nweknora search chunks "query" --kb "KB_ID"\nweknora chat "question" --kb "KB_ID" --format text\nweknora agent list'} copyLabel={t('integrations.cli.copy')} onCopy={() => copy('weknora doc upload')} /></section>
          <section className="wk-vi-136"><LandingSectionHead label={t('integrations.cli.mcpTitle')} /><p className="wk-vi-135">{t('integrations.cli.mcpDesc')}</p><LandingCodeToolbar code={JSON.stringify({ mcpServers: { weknora: { command: 'weknora', args: ['--profile', 'weknora', 'mcp', 'serve'] } } }, null, 2)} copyLabel={t('integrations.cli.copy')} onCopy={() => copy('mcp')} /></section>
        </> : tab === 'chrome' ? <section className="wk-vi-114"><LandingSectionHead label={t('integrations.chrome.stepsTitle')} /><ol className="wk-vi-115">{chromeSteps.map((key, index) => <li key={key} className="wk-vi-116"><span className="wk-vi-117">{index + 1}</span><div className="wk-vi-100"><div className="wk-vi-118">{t('integrations.chrome.steps.' + key + '.title')}</div><p className="wk-vi-119">{t('integrations.chrome.steps.' + key + '.desc')}</p>{key === 'api' && onOpenApiSettings ? <button className="wk-button wk-vi-137" type="button" onClick={onOpenApiSettings}>{t('integrations.chrome.openApiSettings')}</button> : null}{key === 'connect' ? <div className="wk-vi-138"><input readOnly value={apiBaseDisplay} aria-label={apiBaseDisplay} className="wk-vi-139" /><button className="wk-vi-140" type="button" title={t('integrations.chrome.copy')} aria-label={t('integrations.chrome.copy')} onClick={() => copy(apiBaseDisplay)}><CopyIcon /></button></div> : null}</div></li>)}</ol></section> : <section className="wk-vi-114"><LandingSectionHead label={t('integrations.claw.stepsTitle')} accent={accent} barColor="#e85d2a" /><ol className="wk-vi-115">{clawSteps.map((key, index) => <li key={key} className="wk-vi-116"><span className="wk-vi-141">{index + 1}</span><div className="wk-vi-100"><div className="wk-vi-118">{t('integrations.claw.steps.' + key + '.title')}</div><p className="wk-vi-119">{t('integrations.claw.steps.' + key + '.desc')}</p>{key === 'api' && onOpenApiSettings ? <button className="wk-button wk-vi-142" style={{ color: 'rgba(0, 0, 0, 0.9)', borderColor: '#dcdcdc' }} type="button" onClick={onOpenApiSettings}>{t('integrations.claw.openApiSettings')}</button> : null}{key === 'env' ? <div className="wk-vi-120"><LandingCodeToolbar code={envExample} copyLabel={t('integrations.claw.copy')} onCopy={() => copy(envExample)} /></div> : null}{key === 'install' ? <div className="wk-vi-120"><LandingCodeToolbar code={'openclaw skills install @lyingbug/weknora'} copyLabel={t('integrations.claw.copy')} onCopy={() => copy('openclaw skills install @lyingbug/weknora')} /></div> : null}</div></li>)}</ol></section>}
      </div></aside>
    </div>
    {tab === 'chrome' ? <footer className="wk-vi-143">{t('integrations.chrome.storeMeta')}</footer> : tab === 'claw' ? <footer className="wk-vi-144" style={{ borderColor: '#e6cabd' }}><p className="wk-vi-145">{t('integrations.claw.ecosystemNote')}</p><span className="wk-vi-146">{t('integrations.claw.hubMeta')}</span></footer> : null}
  </div>;
}

// Vue integration-landing.less .setting-drawer__section-title: brand color
// bar (3x14, @claw-accent for claw) + label + optional section-head-extra
// count pill pushed to the right edge. leading-[18px] reproduces the Vue
// line-height: normal box (Tailwind's 1.5 layer default made it 20px and
// pushed every step row below down 2px); the count pill is a 13px line box
// (1px 7px padding → 15px tall, same as Vue).
function LandingSectionHead({ label, count, accent, barColor }: { label: string; count?: number; accent?: string; barColor?: string }) {
  // Vue .setting-drawer__section-title::before 条色：默认 brand，claw 用
  // @claw-accent #e85d2a（integration-landing.less:234-238）；文字/计数 pill
  // 才是 accent-dark。barColor 单独传避免混用。
  return <h4 className="wk-vi-147">
    <span aria-hidden="true" className="wk-vi-148" style={{ width: 3, height: 14, background: barColor || '#07c05f' }} />
    <span className="wk-vi-149">{label}</span>
    {typeof count === 'number' ? <span className="wk-vi-150" style={accent ? { color: accent, background: '#f2e0db' } : { color: 'rgba(0,0,0,0.26)', background: '#f3f3f3' }}>{count}</span> : null}
  </h4>;
}

// Vue .code-toolbar: light bordered strip, mono pre on the left, square copy
// button on the right (not the dark bg-ink playground style). The Vue copy
// button computes to 24x24 (t-button small) with 4px side margins — the
// earlier size-8 left the mono pre 8px narrower than the Vue line measure.
// Vue keeps the TDesign default text color rgba(0,0,0,0.9) on the copy icon
// (the claw :deep text-variant recolor does not reach it) with a 3px radius.
function LandingCodeToolbar({ code, copyLabel, onCopy }: { code: string; copyLabel: string; onCopy(): void }) {
  return <div className="wk-vi-151">
    <pre className="wk-vi-152">{code}</pre>
    <button className="wk-vi-153" type="button" title={copyLabel} aria-label={copyLabel} onClick={onCopy}><CopyIcon /></button>
  </div>;
}

// Line icons lifted verbatim from the TDesign icon set (tdesign-icons-vue-next
// esm/components/*.js): the Vue landings render t-icon upload/link/edit/search/
// view-list (ClawSkillLanding.vue L122-128) and chat-bubble/file-copy/edit/
// jump (ChromeExtensionLanding.vue L108-113) at 14px inside the capability
// cards, 1em=14px in the CTA badge (18px for the chrome extension icon), so the
// svg size follows the call site. TDesign defaults: strokeWidth 2, square caps.
const LANDING_ICON_PATHS: Record<string, React.ReactNode> = {
  upload: <><path d="M12 15V4" /><path d="M7.5 8 12 3.5 16.5 8" /><path d="M4.5 19.5h15" /></>,
  url: <path d="M11.6762 6.99023L13.9998 4.6666C15.4726 3.19382 17.8604 3.19382 19.3332 4.6666C20.806 6.13938 20.806 8.52723 19.3332 10L17.0096 12.3236M6.98974 11.6766L4.66611 14.0003C3.19333 15.4731 3.19333 17.8609 4.66611 19.3337C6.13889 20.8065 8.52675 20.8065 9.99953 19.3337L12.3232 17.0101M13.9985 9.99989L9.99847 14" />,
  manual: <path d="M14.1048 6.00427L4.78744 15.3216L3.99805 20.0001L8.67652 19.2107L17.9939 9.89336M14.1048 6.00427L17.9939 9.89336M14.1048 6.00427L17.1615 2.94727L21.0506 6.83635L17.9939 9.89336" />,
  search: <><path d="M15.8033 15.8033C12.8744 18.7322 8.12563 18.7322 5.1967 15.8033C2.26777 12.8744 2.26777 8.12563 5.1967 5.1967C8.12563 2.26777 12.8744 2.26777 15.8033 5.1967C18.7322 8.12563 18.7322 12.8744 15.8033 15.8033Z" /><path d="M15.8027 15.8037L21.106 21.107" /></>,
  browse: <><path d="M3 5H21M3 12H21M3 19H21" /></>,
  qa: <path d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 14.6624 3.04042 17.0817 4.73686 18.8737L3 22H12Z" />,
  code: <><path d="M5.53553 15.5355L2 12L5.53553 8.46448M18.4644 15.5355L21.9999 12L18.4644 8.46448" /><path d="M14 4.00049L10 20.0005" /></>,
  extension: <path d="M9 4C9 2.89543 9.89543 2 11 2C12.1046 2 13 2.89543 13 4V5H19V11H20C21.1046 11 22 11.8954 22 13C22 14.1046 21.1046 15 20 15H19V21H14.4646C14.2219 19.3039 12.7632 18 11 18C9.23676 18 7.77806 19.3039 7.53544 21H3V16.4646C4.69615 16.2219 6 14.7632 6 13C6 11.2368 4.69615 9.77806 3 9.53544V5H9V4Z" />,
  clip: <><rect x="8" y="8" width="12" height="12" rx="1.5" /><path d="M16 4H5.5A1.5 1.5 0 0 0 4 5.5V16" /></>,
  notes: <path d="M14.1048 6.00427L4.78744 15.3216L3.99805 20.0001L8.67652 19.2107L17.9939 9.89336M14.1048 6.00427L17.9939 9.89336M14.1048 6.00427L17.1615 2.94727L21.0506 6.83635L17.9939 9.89336" />,
  shortcuts: <path d="M9 4L4 4L4 20L20 20L20 15M19.25 4.75L12 12M14 4H20L20 10" />,
};

// React 侧 key → Vue t-icon 名称（ClawSkillLanding.vue L122-126 /
// ChromeExtensionLanding.vue L108-113 capabilityIcons + 各 template #icon）。
const LANDING_ICON_TDESIGN_NAMES: Record<string, string> = {
  upload: 'upload',
  url: 'link',
  manual: 'edit',
  search: 'search',
  browse: 'view-list',
  qa: 'chat-bubble',
  clip: 'file-copy',
  notes: 'edit',
  shortcuts: 'jump',
  code: 'code',
  extension: 'extension',
};

function LandingIcon({ name, size = 14 }: { name: string; size?: number }) {
  return <SpriteIcon name={LANDING_ICON_TDESIGN_NAMES[name] ?? name} size={`${size}px`} fallback={<svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" strokeLinejoin="miter" aria-hidden="true">{LANDING_ICON_PATHS[name] ?? null}</svg>} />;
}

// Vue t-icon "file-copy" 16px used by every code-toolbar copy button and the
// connect-row copy control.
function CopyIcon() {
  return <SpriteIcon name="file-copy" size="16px" fallback={<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M14 2V8H20M14 2H15L20 7V8M14 2H7V18H20V8" /><path d="M3 6L3 22H14" /></svg>} />;
}

// Vue t-icon "add" (14px) on the small 创建 API Key outline button.
function PlusIcon() {
  return <SpriteIcon name="add" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M12 5v14" /><path d="M5 12h14" /></svg>} />;
}

// Vue t-icon "jump" in the ext-cta arrow wrap (1em = 14px).
function JumpIcon() {
  return <SpriteIcon name="jump" size="15px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
}

function copyButtonForExternal(t: Translator, key: string, copy: (value: string) => void, value: string) {
  return <button className={'wk-button wk-button--text wk-vi-170 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" title={t(key)} aria-label={t(key)} onClick={() => copy(value)}><CopyIcon /></button>;
}
