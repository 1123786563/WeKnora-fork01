import { useEffect, useMemo, useRef, useState } from 'react';
import { formatMessage } from '@weknora/i18n';
import type { WeKnoraClient } from '@weknora/api-client';
import type { IntegrationAgentOption, IntegrationKnowledgeBaseOption, IntegrationWeChatQrPorts, APIPrincipalConfig, IntegrationResource } from '@weknora/views/integrations/page';
import type { ApiKeyRow } from '@weknora/views/integrations/apiKeys';
import type { IntegrationKey } from '@weknora/views/integrations/registry';
import { IntegrationsPage, setIntegrationSpriteIconRenderer } from '@weknora/views/integrations/page';
import { integrationKeyFromQuery } from '@weknora/views/integrations/registry';
// T12c：向集成视图注入 tdesign sprite 图标渲染器（本地镜像 0.4.5 与 Vue 端
// t-icon glyph 同构，台账 #10）——@weknora/views 无 tdesign-icons-react 依赖，
// 未注入时视图回退内置手绘 path。模块加载时注册一次。
import { Icon as TIcon } from 'tdesign-icons-react';
import './integrations-u.css';
import './views-integrations-u.css';
import './integrations.td.css';

// S1 评审回收：外层 try/catch 已删——setIntegrationSpriteIconRenderer 为同步
// 纯赋值不会抛，属死防御。tdesign-icons-react dist 未带 .t-icon{width/
// height:1em} 基础规则（tdesign-vue-next dist 有，见 integrations.td.css
// 注记）。与 Vue t-icon 渲染路径一致：仅传 font-size（1em 方形由
// integrations.td.css 的 .t-icon 基础规则提供）；inline width/height 会引入
// 亚像素光栅化差（chevron-down 实测 12-14px 单级 AA 残差，台账 #18）。
setIntegrationSpriteIconRenderer((name, size) => {
  const px = typeof size === 'number' ? `${size}px` : (size ?? '16px');
  return <TIcon name={name} size={px} />;
});
import { parseIntegrationTenantId } from './tenant.ts';
import { ApiPlaygroundDrawer } from './ApiPlaygroundDrawer.tsx';
import { EmbedPreviewModal } from './EmbedPreviewModal.tsx';
import { resolveApiBaseUrl } from '../platform/api-base.ts';
import { integrationsLocale, integrationsT } from '../../../../packages/views/src/integrations/messages.ts';

function currentIntegrationsLocale() {
  try { return integrationsLocale(window.localStorage.getItem('locale')); } catch { return integrationsLocale(null); }
}

export function restoreApiPlaygroundFocus(trigger: HTMLElement | null, open: boolean) {
  if (!open) trigger?.focus();
}

export function resolveIntegrationsTab({ activeTab, localTab, requestedTab }: { activeTab?: IntegrationKey; localTab: IntegrationKey; requestedTab?: IntegrationKey }): IntegrationKey {
  return requestedTab ?? activeTab ?? localTab;
}

// Each integrations tab fetches only the data it renders, so a missing or
// empty collection on one tab can never break the others.
export function IntegrationsRoutePage({ client, tenantId, activeTab, activeAgentId, embedded = false, apiBaseUrl = resolveApiBaseUrl(), canEdit = true, onTabChange }: { client: WeKnoraClient; tenantId: string | null; activeTab?: IntegrationKey; activeAgentId?: string | null; embedded?: boolean; apiBaseUrl?: string; canEdit?: boolean; onTabChange?: (tab: IntegrationKey) => void }) {
  const [localTab, setTab] = useState<IntegrationKey>(integrationKeyFromQuery(window.location.search));
  const [requestedTab, setRequestedTab] = useState<IntegrationKey>();
  const tab = resolveIntegrationsTab({ activeTab, localTab, requestedTab });
  const [embedChannels, setEmbedChannels] = useState<IntegrationResource[]>([]);
  const [imChannels, setImChannels] = useState<IntegrationResource[]>([]);
  const [apiKeys, setApiKeys] = useState<ApiKeyRow[]>([]);
  const [apiKeysLoading, setApiKeysLoading] = useState(false);
  // SP14 Task 2 — the API tab's swagger docs row renders only when
  // /system/info confirms swagger_enabled (undefined hides it: older
  // backends lack the field and release builds disable the route).
  const [swaggerEnabled, setSwaggerEnabled] = useState<boolean | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [principal, setPrincipal] = useState<APIPrincipalConfig | null>(null);
  const [agents, setAgents] = useState<IntegrationAgentOption[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(false);
  const [agentsError, setAgentsError] = useState('');
  const [knowledgeBases, setKnowledgeBases] = useState<IntegrationKnowledgeBaseOption[]>([]);
  const [apiPlaygroundOpen, setApiPlaygroundOpen] = useState(false);
  const playgroundTrigger = useRef<HTMLElement | null>(null);
  // Vue keeps the raw create-response token in memory for the Playground (the
  // list endpoint only returns masked keys). Same lifecycle here.
  const [playgroundApiKey, setPlaygroundApiKey] = useState('');
  // Vue EmbedChannelPreview: the preview opens in-page, never a new tab. The
  // deploy-step preview panel lives in @weknora/views; this route-level modal
  // serves its no-token fallback arm (page.tsx onOpenEmbed call, L338).
  const [embedPreview, setEmbedPreview] = useState<{ channelId: string; token: string; title?: string; locale?: string; refreshKey: number } | null>(null);
  // Vue previewNonce (AgentEmbedChannelPanel.vue L1022): bumped per open and
  // folded into the iframe URL as ?r= so a re-preview with the same token
  // fully reloads the latest saved config.
  const embedPreviewNonce = useRef(0);
  // Vue openPreviewForChannel warns previewUnavailable in place (L1007-1010):
  // a shell-local alert keeps the page mounted, unlike the page-level error
  // which unmounts every integrations panel. The key is not migrated into
  // @weknora/i18n yet, so the zh-CN verbatim string is used (the shell fixes
  // zh-CN like the API playground translator on this page).
  const [embedPreviewNotice, setEmbedPreviewNotice] = useState('');
  const activeTenantId = parseIntegrationTenantId(tenantId);
  const visibleEmbedChannels = useMemo(() => activeAgentId ? embedChannels.filter((channel) => String(channel.agent_id ?? '') === activeAgentId) : embedChannels, [activeAgentId, embedChannels]);
  const visibleImChannels = useMemo(() => activeAgentId ? imChannels.filter((channel) => String(channel.agent_id ?? '') === activeAgentId) : imChannels, [activeAgentId, imChannels]);

  async function loadEmbed() {
    try { setEmbedChannels(await client.embed.channels.listAll()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : integrationsT(currentIntegrationsLocale(), 'common.error')); }
  }
  async function loadIm() {
    try { setImChannels(await client.embed.im.listAll()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : integrationsT(currentIntegrationsLocale(), 'common.error')); }
  }
  async function loadApiKeys() {
    if (activeTenantId === null) { setApiKeys([]); return; }
    setApiKeysLoading(true);
    try { setApiKeys(await client.administration.tenantApiKeys.list(activeTenantId)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : integrationsT(currentIntegrationsLocale(), 'common.error')); }
    finally { setApiKeysLoading(false); }
  }

  // IM wizard options: bound-agent select (Vue listAgents) and the step-3
  // file knowledge-base select (Vue chatResources.ensureKnowledgeBases).
  // The embed wizard reuses the agent list for its bound-agent select and the
  // capability warnings (AgentEmbedChannelPanel.vue agentWebSearchEnabledEffective).
  async function loadAgents() {
    setAgentsLoading(true);
    setAgentsError('');
    try { setAgents((await client.configuration.agents.list()).map((agent) => ({ id: agent.id, name: agent.name, config: agent.config }))); }
    catch (cause) { setAgents([]); setAgentsError(cause instanceof Error ? cause.message : integrationsT(currentIntegrationsLocale(), 'common.error')); }
    finally { setAgentsLoading(false); }
  }
  async function loadKnowledgeBases() {
    try { setKnowledgeBases((await client.knowledgeBases.list()).map((kb) => ({ id: kb.id, name: kb.name }))); }
    catch { setKnowledgeBases([]); }
  }

  useEffect(() => {
    setError('');
    let current = true;
    setLoading(tab === 'embed' || tab === 'im' || tab === 'api');
    void (async () => {
      if (tab === 'embed') await Promise.all([loadEmbed(), loadAgents()]);
      if (tab === 'im') await Promise.all([loadIm(), loadAgents(), loadKnowledgeBases()]);
      if (tab === 'api') await Promise.all([loadApiKeys(), loadAgents(), loadKnowledgeBases()]);
      if (current) setLoading(false);
    })();
    return () => { current = false; };
  }, [client, tab, activeTenantId]);

  useEffect(() => { setRequestedTab(undefined); }, [activeTab]);

  useEffect(() => {
    if (tab !== 'api') return;
    let current = true;
    void client.settings.system.info()
      .then((info) => { if (current) setSwaggerEnabled(info.swagger_enabled === true); })
      .catch(() => { if (current) setSwaggerEnabled(undefined); });
    return () => { current = false; };
  }, [client, tab]);

  useEffect(() => { if (tab === 'api' && activeTenantId !== null) void client.administration.tenantApiKeys.principalConfig(activeTenantId).then(setPrincipal).catch(() => setPrincipal(null)); else setPrincipal(null); }, [activeTenantId, client, tab]);
  useEffect(() => { restoreApiPlaygroundFocus(playgroundTrigger.current, apiPlaygroundOpen); }, [apiPlaygroundOpen]);

  async function openEmbed(channel: IntegrationResource) {
    // Vue openPreviewForChannel (AgentEmbedChannelPanel.vue L993-1036): an
    // unavailable preview session warns in place and never falls back to a
    // new tab; a fresh token opens the in-page modal with the channel locale
    // (previewLocale L1021) and a fresh ?r= nonce (previewNonce L1022).
    try {
      const preview = await client.embed.channels.previewSession(channel.id);
      if (!preview.sessionToken) throw new Error('Embed preview session is unavailable.');
      setEmbedPreviewNotice('');
      embedPreviewNonce.current += 1;
      setEmbedPreview({
        channelId: channel.id,
        token: preview.sessionToken,
        title: typeof channel.name === 'string' && channel.name ? channel.name : undefined,
        locale: typeof channel.default_locale === 'string' && channel.default_locale ? channel.default_locale : undefined,
        refreshKey: embedPreviewNonce.current,
      });
    } catch {
      setEmbedPreviewNotice(integrationsT(currentIntegrationsLocale(), 'embedPublish.previewUnavailable'));
    }
  }

  const actions = {
    principal,
    onCreateEmbed: async (input: { agentId: string; payload: Record<string, unknown> }) => client.embed.channels.create(input.agentId, input.payload),
    onUpdateEmbed: async (id: string, input: Record<string, unknown>) => { await client.embed.channels.update(id, input); },
    onDeleteEmbed: async (id: string) => { await client.embed.channels.remove(id); },
    onRotateEmbed: async (id: string) => { await client.embed.channels.rotateToken(id); },
    onPreviewSession: async (id: string) => (await client.embed.channels.previewSession(id)).sessionToken,
    onEmbedDetail: async (id: string) => client.embed.channels.get(id),
    onCreateIm: async (input: { agentId: string; payload: Record<string, unknown> }) => { await client.embed.im.create(input.agentId, input.payload); },
    onUpdateIm: async (id: string, input: Record<string, unknown>) => { await client.embed.im.update(id, input); },
    onToggleIm: async (id: string) => { await client.embed.im.toggle(id); },
    onDeleteIm: async (id: string) => { await client.embed.im.remove(id); },
    onSavePrincipal: async (input: { mode: APIPrincipalConfig['mode']; requireDirectHeader: boolean; hmacSecret?: string }) => { if (activeTenantId === null) throw new Error('No active workspace selected.'); setPrincipal(await client.administration.tenantApiKeys.updatePrincipalConfig(activeTenantId, input)); },
    onCreatePrincipalTestToken: async (externalUserId: string) => { if (activeTenantId === null) throw new Error('No active workspace selected.'); return client.administration.tenantApiKeys.createPrincipalTestToken(activeTenantId, externalUserId); },
    // Vue createScopedAPIKey posts { name, full_access, knowledge_base_ids,
    // capabilities } to POST /tenants/{id}/api-keys; the dialog assembles the
    // payload (packages/views apiKeys.ts buildApiKeyCreatePayload) and this
    // port forwards it unchanged.
    onCreateApiKey: async (payload: { name: string; full_access: boolean; knowledge_base_ids: string[]; capabilities: string[] }): Promise<ApiKeyRow> => {
      if (activeTenantId === null) throw new Error('No active workspace selected.');
      const created = await client.administration.tenantApiKeys.create(activeTenantId, {
        name: payload.name,
        full_access: payload.full_access,
        knowledge_base_ids: payload.knowledge_base_ids,
        capabilities: payload.capabilities,
      });
      if (created.token) setPlaygroundApiKey(created.token);
      return created;
    },
    onRevokeApiKey: async (keyId: ApiKeyRow['id']) => {
      if (activeTenantId === null) throw new Error('No active workspace selected.');
      await client.administration.tenantApiKeys.revoke(activeTenantId, Number(keyId));
    },
    // Vue getWeChatQRCode / pollWeChatQRCodeStatus (frontend/src/api/agent/index.ts).
    wechatQr: {
      create: async () => {
        const data = await client.embed.im.wechat.qrCode();
        return { qrcodeUrl: String(data.qrcode_url ?? ''), qrcode: String(data.qrcode ?? '') };
      },
      poll: async (qrcode: string) => {
        const data = await client.embed.im.wechat.status(qrcode);
        const credentials = (data.credentials ?? {}) as { bot_token?: unknown; ilink_bot_id?: unknown; ilink_user_id?: unknown };
        return {
          status: String(data.status ?? ''),
          bot_token: typeof credentials.bot_token === 'string' ? credentials.bot_token : undefined,
          ilink_bot_id: typeof credentials.ilink_bot_id === 'string' ? credentials.ilink_bot_id : undefined,
          ilink_user_id: typeof credentials.ilink_user_id === 'string' ? credentials.ilink_user_id : undefined,
        };
      },
    } satisfies IntegrationWeChatQrPorts,
  };

  const reload = () => {
    if (tab === 'embed') void loadEmbed();
    if (tab === 'im') void loadIm();
    if (tab === 'api') void loadApiKeys();
  };

  return <>
    {embedPreviewNotice ? <p className="wk-status wk-status-error wk-irp-1" role="alert">{embedPreviewNotice}</p> : null}
    <EmbedPreviewModal open={embedPreview !== null} channelId={embedPreview?.channelId ?? ''} token={embedPreview?.token ?? ''} title={embedPreview?.title} apiBaseUrl={window.location.origin} locale={embedPreview?.locale} refreshKey={embedPreview?.refreshKey} onClose={() => setEmbedPreview(null)} />
    <IntegrationsPage embedded={embedded} initialTab={tab} activeTab={tab} onTabChange={(nextTab) => { setTab(nextTab); setRequestedTab(nextTab); onTabChange?.(nextTab); }} embedChannels={visibleEmbedChannels} imChannels={visibleImChannels} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} apiBaseUrl={apiBaseUrl} swaggerEnabled={swaggerEnabled} loading={loading} error={error} onReload={reload} onOpenEmbed={(channel) => void openEmbed(channel)} onOpenApiPlayground={() => { playgroundTrigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null; setApiPlaygroundOpen(true); }} actions={actions} agents={agents} knowledgeBases={knowledgeBases} canEdit={canEdit} />
    <ApiPlaygroundDrawer open={apiPlaygroundOpen} onClose={() => setApiPlaygroundOpen(false)} apiKey={playgroundApiKey || apiKeys.find((key) => key.api_key)?.api_key || ''} mode={principal?.mode ?? 'tenant'} agents={agents.map((agent) => ({ id: agent.id, name: agent.name }))} agentsLoading={agentsLoading} agentsError={agentsError || undefined} apiBaseUrl={apiBaseUrl} mintToken={actions.onCreatePrincipalTestToken} t={(key, values) => integrationsT(currentIntegrationsLocale(), key, values)} />
  </>;
}
