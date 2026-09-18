// R465-A2 — retrieval-settings surface for the command palette drawer.
// Vue contract (GlobalCommandPalette.vue:153-157): the palette hosts the
// settings page's RetrievalSettings form inside a 420px drawer. The React
// settings surface already exists as ConfigSettingsPanel(section='retrieval')
// (apps/web/src/settings/ConfigSettingsPanel.tsx, registry key 'retrieval',
// ported: true), so this thin wrapper lazily loads the same payload the
// settings page feeds it — GET retrieval config + the tenant model list —
// and renders the panel. It lives under platform/ so the palette/shell own
// the wiring without touching the settings pages.
import { Suspense, lazy, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
import type { SettingsModelOption } from '../settings/ConfigSettingsPanel.tsx';

const ConfigSettingsPanel = lazy(() => import('../settings/ConfigSettingsPanel.tsx').then((m) => ({ default: m.ConfigSettingsPanel })));

export interface PaletteRetrievalSettingsProps {
  client: WeKnoraClient;
  locale: Locale;
}

export function PaletteRetrievalSettings({ client, locale }: PaletteRetrievalSettingsProps): React.ReactNode {
  const [value, setValue] = useState<unknown>(null);
  const [models, setModels] = useState<readonly SettingsModelOption[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    void Promise.all([
      client.settings.retrieval.get(),
      client.configuration.models.list().catch(() => [] as readonly SettingsModelOption[]),
    ]).then(([config, modelList]) => {
      if (!active) return;
      setValue(config);
      setModels(Array.isArray(modelList) ? modelList : []);
    }).catch((reason: unknown) => {
      if (!active) return;
      setError(reason instanceof Error ? reason.message : String(reason));
    });
    return () => { active = false; };
  }, [client]);

  if (error) return <p className="m-0 text-xs text-[#b42318]">{error}</p>;
  if (value === null) return <p className="m-0 text-xs text-[#8a94a3]">{formatMessage(locale, 'common.loading')}</p>;
  return (
    <Suspense fallback={<p className="m-0 text-xs text-[#8a94a3]">{formatMessage(locale, 'common.loading')}</p>}>
      <ConfigSettingsPanel client={client} section="retrieval" initialValue={value} models={models} />
    </Suspense>
  );
}
