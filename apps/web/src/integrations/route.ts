export type IntegrationTab = 'im' | 'embed' | 'api' | 'cli' | 'chrome' | 'claw';

export const integrationTabs: readonly IntegrationTab[] = ['im', 'embed', 'api', 'cli', 'chrome', 'claw'];

export const integrationLabels: Record<IntegrationTab, string> = {
  im: 'IM',
  embed: 'Web embed',
  api: 'API',
  cli: 'CLI',
  chrome: 'Chrome extension',
  claw: 'Claw Skill',
};

export type IntegrationRoute = { tab: IntegrationTab; agentId: string | null };

function tabFromSection(section: string | null, legacyTab: string | null): IntegrationTab | null {
  const candidate = section === null || section === 'integrations'
    ? legacyTab || 'im'
    : section.replace(/^integration-/, '');
  return integrationTabs.includes(candidate as IntegrationTab) ? candidate as IntegrationTab : null;
}

export function parseIntegrationRoute(input: string): IntegrationRoute | null {
  const url = new URL(input, 'http://weknora.local');
  if (url.pathname !== '/platform/integrations' && url.pathname !== '/platform/settings') return null;
  const tab = tabFromSection(url.searchParams.get('section'), url.searchParams.get('tab'));
  return tab === null ? null : { tab, agentId: url.searchParams.get('agentId') };
}

export function buildIntegrationPath(tab: IntegrationTab, agentId?: string): string {
  const params = new URLSearchParams({ section: `integration-${tab}` });
  if (agentId) params.set('agentId', agentId);
  return `/platform/settings?${params.toString()}`;
}
