import type { ConfigurationRecord } from '@weknora/api-client';
import type { ConfigurationDraft, ConfigurationSectionKey } from './surface.ts';

const secretKeys = new Set(['api_key', 'app_secret', 'token', 'password', 'secret', 'client_secret', 'access_token', 'refresh_token']);

function safeObject(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>)
    .filter(([key]) => !secretKeys.has(key.toLowerCase()))
    .map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeObject(item) : item]));
}

function text(value: unknown): string { return typeof value === 'string' ? value : ''; }

export function configurationDraftFromRecord(section: Exclude<ConfigurationSectionKey, 'skills'>, value: ConfigurationRecord): ConfigurationDraft {
  const row = value as Record<string, unknown>;
  const details = section === 'models' ? row.parameters : section === 'agents' ? row.config : row.auth_config;
  return {
    id: text(row.id), name: text(row.name), description: text(row.description), type: text(row.type), source: text(row.source),
    details: JSON.stringify(safeObject(details ?? {})), apiKey: '', appSecret: '', token: '',
    enabled: row.enabled !== false, url: text(row.url),
    credentialStatus: safeObject(row.credentials),
  };
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
