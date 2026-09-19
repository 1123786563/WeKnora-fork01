import { INTEGRATION_SECTIONS } from '../integrations/registry.ts';
export type SettingsScope = 'local' | 'user' | 'tenant' | 'platform';
export type SettingsRole = 'viewer' | 'admin' | 'owner' | 'system-admin';
export type SettingsOperation = 'read' | 'save' | 'reset' | 'test' | 'delete' | 'unavailable';

export interface SettingsSection {
  readonly key: string;
  readonly viewId: string;
  readonly apiDomain: string;
  readonly scope: SettingsScope;
  readonly minRole: SettingsRole;
  readonly operations: readonly SettingsOperation[];
  readonly ported: boolean;
}

const ROLE_RANK: Record<SettingsRole, number> = { viewer: 0, admin: 1, owner: 2, 'system-admin': 3 };

export function roleAtLeast(role: SettingsRole, minRole: SettingsRole): boolean {
  return ROLE_RANK[role] >= ROLE_RANK[minRole];
}

export function settingsSectionsForRole(role: SettingsRole): readonly SettingsSection[] {
  return SETTINGS_SECTIONS.filter((section) => roleAtLeast(role, section.minRole));
}

export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { key: 'general', viewId: 'GeneralSettings', apiDomain: 'preferences', scope: 'local', minRole: 'viewer', operations: ['read'], ported: true },
  { key: 'tenant', viewId: 'TenantInfo', apiDomain: 'tenant', scope: 'tenant', minRole: 'viewer', operations: ['read', 'save'], ported: true },
  { key: 'userprofile', viewId: 'UserProfile', apiDomain: 'profile', scope: 'user', minRole: 'viewer', operations: ['read', 'save'], ported: true },
  { key: 'ollama', viewId: 'OllamaSettings', apiDomain: 'ollama', scope: 'tenant', minRole: 'admin', operations: ['read', 'test', 'save', 'unavailable'], ported: true },
  { key: 'models', viewId: 'ModelManagement', apiDomain: 'configuration.models', scope: 'tenant', minRole: 'viewer', operations: ['read', 'save', 'test', 'delete'], ported: false },
  { key: 'parser', viewId: 'ParserEngineSettings', apiDomain: 'parser', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'unavailable'], ported: true },
  { key: 'retrieval', viewId: 'RetrievalSettings', apiDomain: 'retrieval', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'], ported: true },
  { key: 'memory', viewId: 'MemoryWorkspaceSettings', apiDomain: 'memory.workspace', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'], ported: true },
  { key: 'mymemory', viewId: 'MemorySettings', apiDomain: 'memory.personal', scope: 'user', minRole: 'viewer', operations: ['read', 'save', 'delete'], ported: true },
  { key: 'envvars', viewId: 'EnvVarSettings', apiDomain: 'envVars', scope: 'user', minRole: 'viewer', operations: ['read', 'save', 'delete'], ported: true },
  { key: 'usage', viewId: 'UsageSettings', apiDomain: 'usage', scope: 'user', minRole: 'viewer', operations: ['read'], ported: true },
  { key: 'members', viewId: 'TenantMemberSettings', apiDomain: 'identity.tenants.members', scope: 'tenant', minRole: 'viewer', operations: ['read', 'save', 'delete'], ported: false },
  { key: 'mcp', viewId: 'McpServiceSettings', apiDomain: 'configuration.mcp', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete'], ported: false },
  { key: 'sandbox', viewId: 'SandboxSettings', apiDomain: 'sandbox.configs', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'delete'], ported: false },
  { key: 'skills', viewId: 'SkillSettings', apiDomain: 'configuration.skills', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'delete'], ported: false },
  { key: 'storage', viewId: 'StorageEngineSettings', apiDomain: 'storage', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'], ported: true },
  { key: 'vectorstore', viewId: 'VectorStoreSettings', apiDomain: 'vectorStores', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'], ported: true },
  { key: 'websearch', viewId: 'WebSearchSettings', apiDomain: 'webSearch', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'], ported: true },
  { key: 'chathistory', viewId: 'ChatHistorySettings', apiDomain: 'chatHistory', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'unavailable'], ported: true },
  { key: 'system', viewId: 'SystemInfo', apiDomain: 'system', scope: 'platform', minRole: 'viewer', operations: ['read', 'unavailable'], ported: true },
  { key: 'weknoracloud', viewId: 'WeKnoraCloudSettings', apiDomain: 'weknoraCloud', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'unavailable'], ported: true },
  { key: 'system-global', viewId: 'SystemGlobalSettings', apiDomain: 'administration.settings', scope: 'platform', minRole: 'system-admin', operations: ['read', 'save'], ported: false },
  { key: 'runtime-queues', viewId: 'RuntimeQueueSettings', apiDomain: 'administration.runtime.queues', scope: 'platform', minRole: 'system-admin', operations: ['read', 'save'], ported: false },
  { key: 'platform-api-keys', viewId: 'PlatformApiKeys', apiDomain: 'administration.apiKeys', scope: 'platform', minRole: 'system-admin', operations: ['read', 'save', 'delete'], ported: false },
  { key: 'system-audit-log', viewId: 'SystemAuditLog', apiDomain: 'administration.auditLog', scope: 'platform', minRole: 'system-admin', operations: ['read'], ported: false },
  ...INTEGRATION_SECTIONS.map((item): SettingsSection => ({ key: `integration-${item.key}`, viewId: item.viewId, apiDomain: item.apiDomain ?? 'integration-guide', scope: 'tenant', minRole: item.minRole, operations: ['read'], ported: false })),
] as const;

export function settingsSection(key: string): SettingsSection | undefined {
  return SETTINGS_SECTIONS.find((section) => section.key === key);
}
