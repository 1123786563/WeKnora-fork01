import { SETTINGS_SECTIONS, type SettingsSection, type SettingsOperation, type SettingsRole, type SettingsScope } from '@weknora/views';

export interface SettingsSectionMeta extends SettingsSection {
  readonly title: string;
  readonly description: string;
}

const descriptions: Record<string, { title: string; description: string }> = {
  general: { title: 'General and preferences', description: 'Local display preferences are read from the authenticated user profile.' },
  tenant: { title: 'Tenant information', description: 'The active tenant identity and server-owned metadata.' },
  userprofile: { title: 'User profile', description: 'Authenticated user profile and account metadata.' },
  ollama: { title: 'Ollama', description: 'Local-model availability and the current model inventory.' },
  parser: { title: 'Parser engines', description: 'Parser and document-reader connectivity reported by the server.' },
  retrieval: { title: 'Retrieval', description: 'Tenant retrieval configuration returned by the settings endpoint.' },
  memory: { title: 'Memory workspace', description: 'Shared memory workspace configuration for the active tenant.' },
  mymemory: { title: 'Personal memory', description: 'Personal memory settings and items owned by the current user.' },
  envvars: { title: 'Personal environment variables', description: 'Caller-owned skill and sandbox variable metadata.' },
  storage: { title: 'Storage', description: 'Configured storage backends and legacy engine status.' },
  vectorstore: { title: 'Vector stores', description: 'Configured vector-store resources returned by the server.' },
  websearch: { title: 'Web search', description: 'Configured web-search providers and availability metadata.' },
  chathistory: { title: 'Chat history', description: 'Chat-history configuration and aggregate statistics.' },
  system: { title: 'System information', description: 'Deployment information; this section never treats presence as health.' },
  weknoracloud: { title: 'WeKnora Cloud', description: 'Cloud connection status with credentials redacted at the API boundary.' },
};

const meta = new Map<string, SettingsSectionMeta>(SETTINGS_SECTIONS.map((section) => {
  const copy = descriptions[section.key] ?? { title: section.viewId, description: section.apiDomain };
  return [section.key, { ...section, ...copy }];
}));

export function settingsSectionMeta(key: string): SettingsSectionMeta | undefined { return meta.get(key); }

export function settingsOperationLabel(operation: SettingsOperation): string {
  return ({ read: 'Read', save: 'Save', reset: 'Reset', test: 'Test', delete: 'Delete', unavailable: 'Unavailable' } as Record<SettingsOperation, string>)[operation];
}

export function settingsScopeLabel(scope: SettingsScope): string { return ({ local: 'Local', user: 'User', tenant: 'Tenant', platform: 'Platform' } as Record<SettingsScope, string>)[scope]; }

export function settingsRoleLabel(role: SettingsRole): string { return ({ viewer: 'Viewer', admin: 'Admin', owner: 'Owner', 'system-admin': 'System admin' } as Record<SettingsRole, string>)[role]; }

function isSecretKey(key: string): boolean {
  const normalized = key.toLowerCase();
  return normalized.includes('secret') || normalized.includes('password') || normalized.includes('token') || normalized.includes('api_key') || normalized.includes('access_key');
}

function printable(value: unknown): string {
  if (typeof value === 'string') return value;
  if (value === undefined) return '—';
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) return String(value);
  try { return JSON.stringify(value); } catch { return '[unavailable]'; }
}

export function settingsValueEntries(value: unknown): Array<[string, string]> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return [['value', printable(value)]];
  return Object.entries(value as Record<string, unknown>)
    .filter(([key]) => !isSecretKey(key))
    .map(([key, item]) => [key, printable(item)]);
}

export interface TenantEditState { name: string; description: string }

export function tenantEditState(value: unknown): TenantEditState {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return { name: '', description: '' };
  const row = value as Record<string, unknown>;
  return { name: typeof row.name === 'string' ? row.name : '', description: typeof row.description === 'string' ? row.description : '' };
}

export function tenantPatch(name: string, description: string): { name: string; description: string } {
  const nextName = name.trim();
  if (!nextName) throw new Error('Tenant name is required');
  return { name: nextName, description: description.trim() };
}

export function profilePasswordPatch(oldPassword: string, newPassword: string, confirmation: string): { old_password: string; new_password: string } {
  if (!oldPassword.trim()) throw new Error('Current password is required');
  if (!newPassword.trim()) throw new Error('New password is required');
  if (newPassword === oldPassword) throw new Error('New password must be different from the current password');
  if (newPassword !== confirmation) throw new Error('New passwords must match');
  return { old_password: oldPassword, new_password: newPassword };
}

export function memoryItemPatch(content: string): { content: string } {
  const nextContent = content.trim();
  if (!nextContent) throw new Error('Memory content is required');
  return { content: nextContent };
}

export function memoryEnabledPatch(enabled: boolean): { enabled: boolean } { return { enabled }; }

export function memoryWorkspacePatch(enabled: boolean, writeMode: string, maxItems: number, vectorRecall: boolean, retrievalConditioning: boolean): Record<string, unknown> {
  if (writeMode !== 'explicit_only' && writeMode !== 'auto') throw new Error('Unsupported memory write mode');
  if (!Number.isInteger(maxItems) || maxItems < 10 || maxItems > 2000) throw new Error('Memory max items must be between 10 and 2000');
  return { enabled, write_mode: writeMode, max_items: maxItems, vector_recall: vectorRecall, retrieval_conditioning: retrievalConditioning };
}

export function settingsResourceRows(value: unknown, section: 'storage' | 'vectorstore' | 'websearch'): Array<Record<string, unknown>> {
  const candidate = section === 'storage' && value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>).backends
    : value;
  return Array.isArray(candidate)
    ? candidate.filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    : [];
}

export function settingsResourceInput(name: string, type: string, configText: string): { name: string; type: string; config: Record<string, unknown> } {
  const nextName = name.trim();
  const nextType = type.trim();
  if (!nextName) throw new Error('Resource name is required');
  if (!nextType) throw new Error('Resource type is required');
  let parsed: unknown;
  try { parsed = JSON.parse(configText.trim() || '{}'); } catch { throw new Error('Resource config must be valid JSON'); }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Resource config must be a JSON object');
  return { name: nextName, type: nextType, config: parsed as Record<string, unknown> };
}

type SettingsConfigSection = 'retrieval' | 'chathistory' | 'parser';

function finiteNumber(value: unknown, key: string, min: number, max: number): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(parsed) || parsed < min || parsed > max) throw new Error(`${key} must be between ${min} and ${max}`);
  return parsed;
}

export function settingsConfigPatch(section: SettingsConfigSection, values: Record<string, unknown>): Record<string, unknown> {
  if (section === 'retrieval') {
    return {
      embedding_top_k: Math.trunc(finiteNumber(values.embedding_top_k, 'embedding_top_k', 1, 100)),
      vector_threshold: finiteNumber(values.vector_threshold, 'vector_threshold', 0, 1),
      keyword_threshold: finiteNumber(values.keyword_threshold, 'keyword_threshold', 0, 1),
      rerank_top_k: Math.trunc(finiteNumber(values.rerank_top_k, 'rerank_top_k', 1, 100)),
      rerank_threshold: finiteNumber(values.rerank_threshold, 'rerank_threshold', -10, 10),
      rerank_model_id: typeof values.rerank_model_id === 'string' ? values.rerank_model_id.trim() : '',
    };
  }
  if (section === 'chathistory') {
    if (typeof values.enabled !== 'boolean') throw new Error('Chat history enabled must be a boolean');
    return { enabled: values.enabled, embedding_model_id: typeof values.embedding_model_id === 'string' ? values.embedding_model_id.trim() : '' };
  }
  const endpoint = typeof values.mineru_endpoint === 'string' ? values.mineru_endpoint.trim() : '';
  if (endpoint) {
    let parsed: URL;
    try { parsed = new URL(endpoint); } catch { throw new Error('Parser endpoint must be an absolute URL'); }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') throw new Error('Parser endpoint must use HTTP(S)');
  }
  const patch: Record<string, unknown> = { mineru_endpoint: endpoint };
  const apiKey = typeof values.mineru_api_key === 'string' ? values.mineru_api_key.trim() : '';
  if (apiKey) patch.mineru_api_key = apiKey;
  return patch;
}

export function ollamaModelInput(value: string): string {
  const model = value.trim();
  if (!model) throw new Error('Ollama model name is required');
  return model;
}

export function cloudCredentialPatch(appId: string, appSecret: string): { app_id: string; app_secret: string } {
  const id = appId.trim();
  const secret = appSecret.trim();
  if (!id) throw new Error('WeKnora Cloud app ID is required');
  if (!secret) throw new Error('WeKnora Cloud app secret is required');
  return { app_id: id, app_secret: secret };
}
