import type { ConfigurationRecord } from '@weknora/api-client';
import type { ConfigurationDraft, ConfigurationSectionKey } from './surface.ts';

type EditableConfigurationSection = Exclude<ConfigurationSectionKey, 'skills'>;

const secretKeys = new Set(['apikey', 'appsecret', 'token', 'password', 'secret', 'clientsecret', 'accesstoken', 'refreshtoken']);
function isSecretKey(key: string): boolean { return secretKeys.has(key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase()); }

function safeObject(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>)
    .filter(([key]) => !isSecretKey(key))
    .map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeObject(item) : item]));
}

function text(value: unknown): string { return typeof value === 'string' ? value : ''; }

export function configurationDraftFromRecord(section: Exclude<ConfigurationSectionKey, 'skills'>, value: ConfigurationRecord): ConfigurationDraft {
  const row = value as Record<string, unknown>;
  const details = section === 'models' ? row.parameters : section === 'agents' ? row.config : row.auth_config;
  const safeDetails = safeObject(details ?? {});
  // Agents lift the typed prompt/memory fields out of the raw config JSON so
  // the editor never has to expose them as hand-edited JSON.
  const agentTyped = section === 'agents' ? {
    systemPrompt: text(safeDetails.system_prompt),
    memoryEnabled: safeDetails.memory_enabled === true,
  } : {};
  const remainingDetails = section === 'agents' ? safeObject(Object.fromEntries(Object.entries(safeDetails).filter(([key]) => key !== 'system_prompt' && key !== 'memory_enabled'))) : safeDetails;
  return {
    id: text(row.id), name: text(row.name), description: text(row.description), avatar: text(row.avatar), type: text(row.type), source: text(row.source),
    details: JSON.stringify(remainingDetails), apiKey: '', appSecret: '', token: '',
    isBuiltin: row.is_builtin === true,
    ...(section === 'mcp' ? { transportType: text(row.transport_type) || 'sse' } : {}),
    enabled: row.enabled !== false, url: text(row.url),
    credentialStatus: safeObject(row.credentials),
    ...agentTyped,
  };
}

export function newConfigurationDraft(section: EditableConfigurationSection): ConfigurationDraft {
  const agentTyped = section === 'agents' ? { systemPrompt: '', memoryEnabled: false } : {};
  return {
    name: '', description: '', type: section === 'models' ? 'KnowledgeQA' : '', source: section === 'models' ? 'custom' : '', details: '{}',
    apiKey: '', appSecret: '', token: '', ...(section === 'mcp' ? { transportType: 'sse' } : {}),
    enabled: true, url: '', credentialStatus: {},
    isBuiltin: false,
    ...agentTyped,
  };
}

export function credentialStatusAfterClear(status: Record<string, unknown> | undefined, field: string): Record<string, unknown> {
  return { ...safeObject(status ?? {}), [field]: { configured: false } };
}

export function credentialInput(section: ConfigurationSectionKey, input: Pick<ConfigurationDraft, 'apiKey' | 'appSecret' | 'token'>): Record<string, string> {
  if (section === 'models') return Object.fromEntries([
    ['apiKey', input.apiKey], ['appSecret', input.appSecret],
  ].filter((entry): entry is [string, string] => typeof entry[1] === 'string' && entry[1].trim() !== '').map(([key, value]) => [key, value.trim()]));
  if (section === 'mcp') return Object.fromEntries([
    ['apiKey', input.apiKey], ['token', input.token],
  ].filter((entry): entry is [string, string] => typeof entry[1] === 'string' && entry[1].trim() !== '').map(([key, value]) => [key, value.trim()]));
  return {};
}

export function savedConfigurationId(existingId: string | undefined, serverId: string): string {
  const id = (existingId ?? serverId).trim();
  if (id === '') throw new Error('server id is required before writing credentials');
  return id;
}
