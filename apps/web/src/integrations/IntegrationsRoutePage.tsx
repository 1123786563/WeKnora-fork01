import { useEffect, useRef, useState } from 'react';
import { formatMessage } from '@weknora/i18n';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ApiKeyRow, IntegrationAgentOption, IntegrationKnowledgeBaseOption, IntegrationWeChatQrPorts } from '@weknora/views';
import { integrationKeyFromQuery, IntegrationsPage, type APIPrincipalConfig, type IntegrationKey, type IntegrationResource } from '@weknora/views';
import { parseIntegrationTenantId } from './tenant.ts';
import { ApiPlaygroundDrawer } from './ApiPlaygroundDrawer.tsx';
import { EmbedPreviewModal } from './EmbedPreviewModal.tsx';

// Each integrations tab fetches only the data it renders, so a missing or
// empty collection on one tab can never break the others.
export function IntegrationsRoutePage({ client, tenantId, activeTab, embedded = false }: { client: WeKnoraClient; tenantId: string | null; activeTab?: IntegrationKey; embedded?: boolean }) {
  const [localTab, setTab] = useState<IntegrationKey>(integrationKeyFromQuery(window.location.search));
  const tab = activeTab ?? localTab;
  const [embedChannels, setEmbedChannels] = useState<IntegrationResource[]>([]);
  const [imChannels, setImChannels] = useState<IntegrationResource[]>([]);
  const [apiKeys, setApiKeys] = useState<ApiKeyRow[]>([]);
  const [apiKeysLoading, setApiKeysLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [principal, setPrincipal] = useState<APIPrincipalConfig | null>(null);
  const [agents, setAgents] = useState<IntegrationAgentOption[]>([]);
  const [knowledgeBases, setKnowledgeBases] = useState<IntegrationKnowledgeBaseOption[]>([]);
  const [apiPlaygroundOpen, setApiPlaygroundOpen] = useState(false);
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

  async function loadEmbed() {
    try { setEmbedChannels(await client.embed.channels.listAll()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load embed channels.'); }
  }
  async function loadIm() {
    try { setImChannels(await client.embed.im.listAll()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load IM channels.'); }
  }
  async function loadApiKeys() {
    if (activeTenantId === null) { setApiKeys([]); return; }
    setApiKeysLoading(true);
    try { setApiKeys(await client.administration.tenantApiKeys.list(activeTenantId)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load API keys.'); }
    finally { setApiKeysLoading(false); }
  }

  // IM wizard options: bound-agent select (Vue listAgents) and the step-3
  // file knowledge-base select (Vue chatResources.ensureKnowledgeBases).
  // The embed wizard reuses the agent list for its bound-agent select and the
  // capability warnings (AgentEmbedChannelPanel.vue agentWebSearchEnabledEffective).
  async function loadAgents() {
    try { setAgents((await client.configuration.agents.list()).map((agent) => ({ id: agent.id, name: agent.name, config: agent.config }))); }
    catch { setAgents([]); }
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
      if (tab === 'api') await loadApiKeys();
      if (current) setLoading(false);
    })();
    return () => { current = false; };
  }, [client, tab, activeTenantId]);

  useEffect(() => { if (tab === 'api' && activeTenantId !== null) void client.administration.tenantApiKeys.principalConfig(activeTenantId).then(setPrincipal).catch(() => setPrincipal(null)); else setPrincipal(null); }, [activeTenantId, client, tab]);

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
      setEmbedPreviewNotice('预览暂时不可用，请确认渠道已启用且 Redis 可用');
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
    onCreateApiKey: async (name: string): Promise<ApiKeyRow> => {
      if (activeTenantId === null) throw new Error('No active workspace selected.');
      return client.administration.tenantApiKeys.create(activeTenantId, { name, full_access: true });
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
    {embedPreviewNotice ? <p className="wk-status wk-status-error" role="alert">{embedPreviewNotice}</p> : null}
    <EmbedPreviewModal open={embedPreview !== null} channelId={embedPreview?.channelId ?? ''} token={embedPreview?.token ?? ''} title={embedPreview?.title} apiBaseUrl={window.location.origin} locale={embedPreview?.locale} refreshKey={embedPreview?.refreshKey} onClose={() => setEmbedPreview(null)} />
    <IntegrationsPage embedded={embedded} initialTab={tab} activeTab={tab} onTabChange={setTab} embedChannels={embedChannels} imChannels={imChannels} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} apiBaseUrl={window.location.origin} loading={loading} error={error} onReload={reload} onOpenEmbed={(channel) => void openEmbed(channel)} onOpenApiPlayground={() => setApiPlaygroundOpen(true)} actions={actions} agents={agents} knowledgeBases={knowledgeBases} />
    <ApiPlaygroundDrawer open={apiPlaygroundOpen} onClose={() => setApiPlaygroundOpen(false)} apiKey={apiKeys.find((key) => key.api_key)?.api_key ?? ''} mode={principal?.mode ?? 'tenant'} agents={agents.map((agent) => ({ id: agent.id, name: agent.name }))} apiBaseUrl={window.location.origin} mintToken={actions.onCreatePrincipalTestToken} t={(key, values) => formatMessage('zh-CN', key, values)} />
  </>;
}
