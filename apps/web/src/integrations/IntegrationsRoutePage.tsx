import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { IntegrationsPage, type APIPrincipalConfig, type IntegrationResource } from '@weknora/views';

export function IntegrationsRoutePage({ client }: { client: WeKnoraClient }) {
  const [embedChannels, setEmbedChannels] = useState<IntegrationResource[]>([]);
  const [imChannels, setImChannels] = useState<IntegrationResource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [principal, setPrincipal] = useState<APIPrincipalConfig | null>(null);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [embed, im] = await Promise.all([client.embed.channels.listAll(), client.embed.im.listAll()]);
      setEmbedChannels(embed);
      setImChannels(im);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to load integrations.');
    } finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, [client]);
  useEffect(() => { const id = Number(window.localStorage.getItem('weknora_selected_tenant_id') || 0); if (id) void client.administration.tenantApiKeys.principalConfig(id).then(setPrincipal).catch(() => setPrincipal(null)); }, [client]);

  async function openEmbed(channel: IntegrationResource) {
    try {
      const preview = await client.embed.channels.previewSession(channel.id);
      const url = `${window.location.origin}/embed/${encodeURIComponent(channel.id)}#token=${encodeURIComponent(preview.sessionToken)}`;
      window.open(url, '_blank', 'noopener,noreferrer');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to open Embed preview.');
    }
  }

  const tenantId = Number(window.localStorage.getItem('weknora_selected_tenant_id') || 0);
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
    onSavePrincipal: async (input: { mode: APIPrincipalConfig['mode']; requireDirectHeader: boolean; hmacSecret?: string }) => { if (!tenantId) throw new Error('No active workspace selected.'); setPrincipal(await client.administration.tenantApiKeys.updatePrincipalConfig(tenantId, input)); },
    onCreatePrincipalTestToken: async (externalUserId: string) => { if (!tenantId) throw new Error('No active workspace selected.'); return client.administration.tenantApiKeys.createPrincipalTestToken(tenantId, externalUserId); },
  };

  return <IntegrationsPage embedChannels={embedChannels} imChannels={imChannels} apiBaseUrl={window.location.origin} loading={loading} error={error} onReload={() => void load()} onOpenEmbed={(channel) => void openEmbed(channel)} actions={actions} />;
}
