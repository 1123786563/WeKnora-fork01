import { INTEGRATION_SECTIONS, type IntegrationKey } from './registry.ts';
import { SETTINGS_SECTIONS } from '../settings/registry.ts';

export function integrationTabForSection(section: string): IntegrationKey | undefined {
  if (!section.startsWith('integration-')) return undefined;
  return INTEGRATION_SECTIONS.find((item) => `integration-${item.key}` === section)?.key;
}

export function normalizeIntegrationSettingsSection(section: string, tab?: string | null): string {
  if (section === 'integrations') {
    const raw = tab?.startsWith('integration-') ? tab.slice('integration-'.length) : tab;
    return `integration-${INTEGRATION_SECTIONS.find((item) => item.key === raw)?.key ?? 'im'}`;
  }
  // OCR 终局第 2 轮 f27: a bare key that names a REGISTERED settings section
  // ('plugins' — the admin plugin settings panel) stays verbatim. It also
  // happens to name an integrations tab (the member discovery panel); on the
  // settings route a bare ?section= can only mean the settings section, so
  // prefixing it here hijacked the panel's deep link / popstate into the
  // IntegrationsRoutePage tab. Integration-only bare keys keep the prefixed
  // canonical tab URL. The import direction (settings-route → settings
  // registry → integrations registry) is acyclic.
  if (SETTINGS_SECTIONS.some((item) => item.key === section)) return section;
  return INTEGRATION_SECTIONS.some((item) => item.key === section) ? `integration-${section}` : section;
}

export function integrationSettingsQuery(search: string, legacy = false): URLSearchParams {
  const query = new URLSearchParams(search);
  const sections = query.getAll('section');
  const tabs = query.getAll('tab');
  const section = sections.length === 1 ? sections[0]! : legacy ? 'integrations' : 'general';
  query.set('section', normalizeIntegrationSettingsSection(section, tabs.length === 1 ? tabs[0] : undefined));
  query.delete('tab');
  return query;
}

export function selectSettingsQuery(section: string, search: string): URLSearchParams {
  const query = new URLSearchParams(search);
  query.set('section', section);
  query.delete('tab');
  if (!integrationTabForSection(section)) query.delete('agentId');
  return query;
}
