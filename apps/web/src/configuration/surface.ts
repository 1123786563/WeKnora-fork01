export type ConfigurationSectionKey = 'agents' | 'models' | 'mcp' | 'skills';

export const configurationSections: Array<{ key: ConfigurationSectionKey; title: string; description: string; writeSupport: 'read-only' | 'supported' }> = [
  { key: 'agents', title: 'Agents', description: 'Custom and built-in agent configurations available in this workspace.', writeSupport: 'supported' },
  { key: 'models', title: 'Models', description: 'Model bindings returned by the workspace configuration service.', writeSupport: 'supported' },
  { key: 'mcp', title: 'MCP services', description: 'MCP endpoints and server-reported tool metadata.', writeSupport: 'supported' },
  { key: 'skills', title: 'Skills', description: 'Skill catalog availability associated with the workspace sandbox.', writeSupport: 'read-only' },
];

export interface ConfigurationDraft {
  id?: string;
  name: string;
  description?: string;
  type?: string;
  source?: string;
  transportType?: string;
  details: string;
  apiKey?: string;
  appSecret?: string;
  token?: string;
  enabled?: boolean;
  url?: string;
  credentialStatus?: Record<string, unknown>;
}

export function parseConfigurationObject(value: string, label: string): Record<string, unknown> {
  let parsed: unknown;
  try { parsed = JSON.parse(value); } catch { throw new Error(`${label} must be valid JSON`); }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(`${label} must be a JSON object`);
  return parsed as Record<string, unknown>;
}

export function configurationPayload(section: ConfigurationSectionKey, draft: ConfigurationDraft): Record<string, unknown> {
  if (draft.name.trim() === '') throw new Error('Name is required');
  const details = parseConfigurationObject(draft.details || '{}', `${section} config`);
  if (section === 'models') {
    return { name: draft.name.trim(), display_name: draft.name.trim(), description: draft.description ?? '', type: draft.type ?? '', source: draft.source ?? '', parameters: details };
  }
  if (section === 'agents') return { name: draft.name.trim(), description: draft.description ?? '', config: details };
  if (section === 'mcp') return { name: draft.name.trim(), url: draft.url ?? '', enabled: draft.enabled !== false, auth_config: details };
  throw new Error('Skills are read-only');
}

export function configurationStatus(value: { source?: unknown; enabled?: unknown; id?: unknown; name?: unknown }): string {
  if (value.source === 'env') return 'environment-managed';
  if (value.enabled === false) return 'disabled';
  if (value.enabled === true) return 'enabled';
  if (typeof value.id === 'string' || typeof value.name === 'string') return 'configured';
  return 'unknown';
}
