import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ApiKeyRow } from '@weknora/views';
import { integrationKeyFromQuery, IntegrationsPage, type APIPrincipalConfig, type IntegrationKey, type IntegrationResource } from '@weknora/views';
import { parseIntegrationTenantId } from './tenant.ts';

// Each integrations tab fetches only the data it renders, so a missing or
// empty collection on one tab can never break the others.
export function IntegrationsRoutePage({ client, tenantId }: { client: WeKnoraClient; tenantId: string | null }) {
  const [tab, setTab] = useState<IntegrationKey>(integrationKeyFromQuery(window.location.search));
  const [embedChannels, setEmbedChannels] = useState<IntegrationResource[]>([]);
  const [imChannels, setImChannels] = useState<IntegrationResource[]>([]);
  const [apiKeys, setApiKeys] = useState<ApiKeyRow[]>([]);
  const [apiKeysLoading, setApiKeysLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [principal, setPrincipal] = useState<APIPrincipalConfig | null>(null);
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

  useEffect(() => {
    setError('');
    if (tab === 'embed') void loadEmbed();
    if (tab === 'im') void loadIm();
    if (tab === 'api') void loadApiKeys();
    setLoading(false);
  }, [client, tab, activeTenantId]);

  useEffect(() => { if (activeTenantId !== null) void client.administration.tenantApiKeys.principalConfig(activeTenantId).then(setPrincipal).catch(() => setPrincipal(null)); else setPrincipal(null); }, [activeTenantId, client]);

  async function openEmbed(channel: IntegrationResource) {
    try {
      const preview = await client.embed.channels.previewSession(channel.id);
      const url = window.location.origin + '/embed/' + encodeURIComponent(channel.id) + '#token=' + encodeURIComponent(preview.sessionToken);
      window.open(url, '_blank', 'noopener,noreferrer');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to open Embed preview.');
    }
  }

  const actions = {
    principal,
    onCreateEmbed: async (input: Record<string, unknown>) => { await client.embed.channels.create(String(input.agent_id || ''), input); },
    onUpdateEmbed: async (id: string, input: Record<string, unknown>) => { await client.embed.channels.update(id, input); },
    onDeleteEmbed: async (id: string) => { await client.embed.channels.remove(id); },
    onRotateEmbed: async (id: string) => { await client.embed.channels.rotateToken(id); },
    onCreateIm: async (input: { agentId: string; platform: string; name: string; credentials: Record<string, unknown> }) => { await client.embed.im.create(input.agentId, { platform: input.platform, name: input.name, credentials: input.credentials }); },
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
  };

  const reload = () => {
    if (tab === 'embed') void loadEmbed();
    if (tab === 'im') void loadIm();
    if (tab === 'api') void loadApiKeys();
  };

  return <IntegrationsPage initialTab={tab} activeTab={tab} onTabChange={setTab} embedChannels={embedChannels} imChannels={imChannels} apiKeys={apiKeys} apiKeysLoading={apiKeysLoading} apiBaseUrl={window.location.origin} loading={loading} error={error} onReload={reload} onOpenEmbed={(channel) => void openEmbed(channel)} actions={actions} />;
}