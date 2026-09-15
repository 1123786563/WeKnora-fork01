export type ConfigurationSectionKey = 'agents' | 'models' | 'mcp' | 'skills';
export type McpTransportType = 'sse' | 'http-streamable' | 'stdio';

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
  avatar?: string;
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
  /** Typed agent fields (lifted out of the raw config JSON). */
  systemPrompt?: string;
  memoryEnabled?: boolean;
  isBuiltin?: boolean;
}

export function parseConfigurationObject(value: string, label: string): Record<string, unknown> {
  let parsed: unknown;
  try { parsed = JSON.parse(value); } catch { throw new Error(`${label} must be valid JSON`); }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(`${label} must be a JSON object`);
  rejectSecretKeys(parsed, label);
  return parsed as Record<string, unknown>;
}

const secretKeys = new Set(['apikey', 'appsecret', 'accesstoken', 'refreshtoken', 'token', 'clientsecret', 'password', 'secret']);
function isSecretKey(key: string): boolean { return secretKeys.has(key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase()); }
function rejectSecretKeys(value: unknown, path: string): void {
  if (Array.isArray(value)) {
    value.forEach((item, index) => rejectSecretKeys(item, `${path}[${index}]`));
    return;
  }
  if (value === null || typeof value !== 'object') return;
  Object.entries(value as Record<string, unknown>).forEach(([key, item]) => {
    if (isSecretKey(key)) throw new Error(`${path}.${key} is a secret field; use the dedicated credentials endpoint`);
    rejectSecretKeys(item, `${path}.${key}`);
  });
}

export function configurationPayload(section: ConfigurationSectionKey, draft: ConfigurationDraft): Record<string, unknown> {
  if (draft.name.trim() === '') throw new Error('Name is required');
  if (section === 'agents' && draft.isBuiltin !== true && !(draft.systemPrompt ?? '').trim()) throw new Error('System prompt is required');
  const details = parseConfigurationObject(draft.details || '{}', `${section} config`);
  if (section === 'models') {
    return { name: draft.name.trim(), display_name: draft.name.trim(), description: draft.description ?? '', type: draft.type ?? '', source: draft.source ?? '', parameters: details };
  }
  if (section === 'agents') {
    const config: Record<string, unknown> = { ...details, system_prompt: draft.systemPrompt ?? '', memory_enabled: draft.memoryEnabled === true };
    return { name: draft.name.trim(), description: draft.description ?? '', avatar: draft.avatar ?? '', config };
  }
  if (section === 'mcp') {
    const transportType = draft.transportType ?? 'sse';
    if (transportType !== 'sse' && transportType !== 'http-streamable' && transportType !== 'stdio') throw new Error('MCP transport type is invalid');
    return { name: draft.name.trim(), url: draft.url ?? '', enabled: draft.enabled !== false, transport_type: transportType, auth_config: details };
  }
  throw new Error('Skills are read-only');
}

export function configurationStatus(value: { source?: unknown; enabled?: unknown; id?: unknown; name?: unknown }): string {
  if (value.source === 'env') return 'environment-managed';
  if (value.enabled === false) return 'disabled';
  if (value.enabled === true) return 'enabled';
  if (typeof value.id === 'string' || typeof value.name === 'string') return 'configured';
  return 'unknown';
}
