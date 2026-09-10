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
