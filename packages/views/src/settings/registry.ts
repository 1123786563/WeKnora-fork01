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
}

export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { key: 'general', viewId: 'GeneralSettings', apiDomain: 'preferences', scope: 'local', minRole: 'viewer', operations: ['read'] },
  { key: 'tenant', viewId: 'TenantInfo', apiDomain: 'tenant', scope: 'tenant', minRole: 'viewer', operations: ['read', 'save'] },
  { key: 'userprofile', viewId: 'UserProfile', apiDomain: 'profile', scope: 'user', minRole: 'viewer', operations: ['read', 'save'] },
  { key: 'ollama', viewId: 'OllamaSettings', apiDomain: 'ollama', scope: 'tenant', minRole: 'admin', operations: ['read', 'test', 'save', 'unavailable'] },
  { key: 'parser', viewId: 'ParserEngineSettings', apiDomain: 'parser', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'unavailable'] },
  { key: 'retrieval', viewId: 'RetrievalSettings', apiDomain: 'retrieval', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'] },
  { key: 'memory', viewId: 'MemoryWorkspaceSettings', apiDomain: 'memory.workspace', scope: 'tenant', minRole: 'admin', operations: ['read', 'save'] },
  { key: 'mymemory', viewId: 'MemorySettings', apiDomain: 'memory.personal', scope: 'user', minRole: 'viewer', operations: ['read', 'save', 'delete'] },
  { key: 'envvars', viewId: 'EnvVarSettings', apiDomain: 'envVars', scope: 'user', minRole: 'viewer', operations: ['read', 'save', 'delete'] },
  { key: 'storage', viewId: 'StorageEngineSettings', apiDomain: 'storage', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'] },
  { key: 'vectorstore', viewId: 'VectorStoreSettings', apiDomain: 'vectorStores', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'] },
  { key: 'websearch', viewId: 'WebSearchSettings', apiDomain: 'webSearch', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'delete', 'unavailable'] },
  { key: 'chathistory', viewId: 'ChatHistorySettings', apiDomain: 'chatHistory', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'unavailable'] },
  { key: 'system', viewId: 'SystemInfo', apiDomain: 'system', scope: 'platform', minRole: 'viewer', operations: ['read', 'unavailable'] },
  { key: 'weknoracloud', viewId: 'WeKnoraCloudSettings', apiDomain: 'weknoraCloud', scope: 'tenant', minRole: 'admin', operations: ['read', 'save', 'test', 'unavailable'] },
] as const;

export function settingsSection(key: string): SettingsSection | undefined {
  return SETTINGS_SECTIONS.find((section) => section.key === key);
}
