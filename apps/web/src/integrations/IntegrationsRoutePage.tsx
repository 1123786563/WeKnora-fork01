import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { IntegrationsPage, type IntegrationResource } from '@weknora/views';

export function IntegrationsRoutePage({ client }: { client: WeKnoraClient }) {
  const [embedChannels, setEmbedChannels] = useState<IntegrationResource[]>([]);
  const [imChannels, setImChannels] = useState<IntegrationResource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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

  async function openEmbed(channel: IntegrationResource) {
    try {
      const preview = await client.embed.channels.previewSession(channel.id);
      const url = `${window.location.origin}/embed/${encodeURIComponent(channel.id)}#token=${encodeURIComponent(preview.sessionToken)}`;
      window.open(url, '_blank', 'noopener,noreferrer');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to open Embed preview.');
    }
  }

  return <IntegrationsPage embedChannels={embedChannels} imChannels={imChannels} apiBaseUrl={window.location.origin} loading={loading} error={error} onReload={() => void load()} onOpenEmbed={(channel) => void openEmbed(channel)} />;
}
