export type IntegrationKey = 'im' | 'embed' | 'api' | 'cli' | 'chrome' | 'claw' | 'plugins';
export type IntegrationOperation = 'manage' | 'open' | 'external';

export interface IntegrationSection {
  readonly key: IntegrationKey;
  readonly viewId: string;
  readonly apiDomain: 'im' | 'channels' | 'api-principal' | null;
  readonly capability?: string;
  readonly minRole: 'viewer' | 'owner';
  readonly external: boolean;
  readonly externalUrl?: string;
  readonly operations: readonly IntegrationOperation[];
}

export const INTEGRATION_SECTIONS: readonly IntegrationSection[] = [
  { key: 'im', capability: 'integrations.im', viewId: 'IMChannelPanel', apiDomain: 'im', minRole: 'viewer', external: false, operations: ['manage'] },
  { key: 'embed', capability: 'integrations.embed', viewId: 'EmbedChannelPanel', apiDomain: 'channels', minRole: 'viewer', external: false, operations: ['manage', 'open'] },
  { key: 'api', capability: 'integrations.api', viewId: 'ApiIntegrationSettings', apiDomain: 'api-principal', minRole: 'owner', external: false, operations: ['manage'] },
  { key: 'cli', viewId: 'CliIntegrationLanding', apiDomain: null, minRole: 'viewer', external: true, externalUrl: 'https://github.com/Tencent/WeKnora/blob/main/cli/README.md', operations: ['external', 'open'] },
  { key: 'chrome', viewId: 'ChromeExtensionLanding', apiDomain: null, minRole: 'viewer', external: true, externalUrl: 'https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd', operations: ['external', 'open'] },
  { key: 'claw', viewId: 'ClawSkillLanding', apiDomain: null, minRole: 'viewer', external: true, externalUrl: 'https://clawhub.ai/lyingbug/weknora', operations: ['external', 'open'] },
  // T08 (issue #110): member plugin discovery — every workspace member sees
  // WHAT is installed; the read surface lives in apps/web/src/integrations/
  // PluginsPanel.tsx. No capability: the section ships with the integrations
  // page by default (same tier as im/embed); governance actions stay on the
  // admin settings panel, never here.
  { key: 'plugins', viewId: 'PluginDiscoverPanel', apiDomain: null, minRole: 'viewer', external: false, operations: ['manage'] },
] as const;

export function integrationKeyFromQuery(search: string): IntegrationKey {
  const query = new URLSearchParams(search.startsWith('?') ? search : `?${search}`);
  const section = query.get('section')?.trim() || '';
  const tab = query.get('tab')?.trim() || '';
  if (!section && !tab) return 'embed';
  const candidate = section === 'integrations' ? tab || 'im' : section || tab;
  const normalized = candidate.startsWith('integration-') ? candidate.slice('integration-'.length) : candidate;
  return INTEGRATION_SECTIONS.some((item) => item.key === normalized) ? normalized as IntegrationKey : 'embed';
}

export function integrationSection(key: string): IntegrationSection | undefined {
  return INTEGRATION_SECTIONS.find((item) => item.key === key);
}
