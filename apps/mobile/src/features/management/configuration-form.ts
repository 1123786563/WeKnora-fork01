import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration } from '@weknora/api-client';

export type NativeConfigurationSection = 'agents' | 'models' | 'mcp';

export interface NativeConfigurationDraft {
  section: NativeConfigurationSection;
  id?: string;
  name: string;
  description: string;
  avatar: string;
  type: string;
  source: string;
  url: string;
  transportType: 'sse' | 'http-streamable' | 'stdio';
  enabled: boolean;
  details: string;
  apiKey: string;
  appSecret: string;
  token: string;
}

const SECRET_KEYS = new Set(['apikey', 'appsecret', 'accesstoken', 'refreshtoken', 'token', 'clientsecret', 'password', 'secret']);
function isSecretKey(key: string): boolean { return SECRET_KEYS.has(key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase()); }

function safeObject(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).filter(([key]) => !isSecretKey(key)).map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeObject(item) : item]));
}

function parseDetails(value: string, section: string): Record<string, unknown> {
  let parsed: unknown;
  try { parsed = JSON.parse(value || '{}'); } catch { throw new Error(`${section} config must be valid JSON`); }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(`${section} config must be a JSON object`);
  function rejectSecrets(item: unknown, path: string): void {
    if (Array.isArray(item)) { item.forEach((child, index) => rejectSecrets(child, `${path}[${index}]`)); return; }
    if (item === null || typeof item !== 'object') return;
    Object.entries(item as Record<string, unknown>).forEach(([key, child]) => { if (isSecretKey(key)) throw new Error(`${path}.${key} is a secret field; use the dedicated credentials endpoint`); rejectSecrets(child, `${path}.${key}`); });
  }
  rejectSecrets(parsed, `${section} config`);
  return parsed as Record<string, unknown>;
}

export function configurationPayload(draft: NativeConfigurationDraft): Record<string, unknown> {
  if (!draft.name.trim()) throw new Error('Name is required');
  const details = parseDetails(draft.details, draft.section);
  if (draft.section === 'agents') return { name: draft.name.trim(), description: draft.description, avatar: draft.avatar, config: details };
  if (draft.section === 'models') return { name: draft.name.trim(), display_name: draft.name.trim(), description: draft.description, type: draft.type, source: draft.source, parameters: details };
  return { name: draft.name.trim(), url: draft.url, enabled: draft.enabled, transport_type: draft.transportType, auth_config: details };
}

export function credentialInput(draft: NativeConfigurationDraft): Record<string, string> {
  const fields = draft.section === 'models' ? [['apiKey', draft.apiKey], ['appSecret', draft.appSecret]] : draft.section === 'mcp' ? [['apiKey', draft.apiKey], ['token', draft.token]] : [];
  return Object.fromEntries(fields.filter((entry): entry is [string, string] => entry[1].trim() !== '').map(([key, value]) => [key, value.trim()]));
}

export function nativeConfigurationDraftFrom(section: NativeConfigurationSection, value: ConfigurationRecord): NativeConfigurationDraft {
  const row = value as Record<string, unknown>;
  const details = section === 'models' ? row.parameters : section === 'agents' ? row.config : row.auth_config;
  return {
    section,
    id: typeof row.id === 'string' ? row.id : undefined,
    name: typeof row.name === 'string' ? row.name : '',
    description: typeof row.description === 'string' ? row.description : '',
    avatar: typeof row.avatar === 'string' ? row.avatar : '',
    type: typeof row.type === 'string' ? row.type : '',
    source: typeof row.source === 'string' ? row.source : '',
    url: typeof row.url === 'string' ? row.url : '',
    transportType: row.transport_type === 'http-streamable' || row.transport_type === 'stdio' ? row.transport_type : 'sse',
    enabled: row.enabled !== false,
    details: JSON.stringify(safeObject(details ?? {})),
    apiKey: '', appSecret: '', token: '',
  };
}

export function newNativeConfigurationDraft(section: NativeConfigurationSection): NativeConfigurationDraft {
  return { section, name: '', description: '', avatar: '', type: section === 'models' ? 'KnowledgeQA' : '', source: section === 'models' ? 'custom' : '', url: '', transportType: 'sse', enabled: true, details: '{}', apiKey: '', appSecret: '', token: '' };
}

export type NativeConfigurationRecord = AgentConfiguration | ModelConfiguration | McpConfiguration;
