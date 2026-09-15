export type SettingsRole = 'viewer' | 'contributor' | 'admin' | 'owner' | 'systemAdmin';

export type SettingsTab =
  | 'general' | 'ollama' | 'weknoracloud' | 'models' | 'websearch' | 'chathistory' | 'memory'
  | 'vectorstore' | 'parser' | 'storage' | 'sandbox' | 'skills' | 'mcp' | 'system'
  | 'system-global' | 'runtime-queues' | 'platform-api-keys' | 'system-audit-log'
  | 'userprofile' | 'mymemory' | 'envvars' | 'tenant' | 'members'
  | 'integration-api' | 'integration-cli' | 'integration-claw' | 'integration-im';

const roleRank: Record<SettingsRole, number> = {
  viewer: 0,
  contributor: 1,
  admin: 2,
  owner: 3,
  systemAdmin: 4,
};

const minimumRole: Partial<Record<SettingsTab, SettingsRole>> = {
  general: 'viewer',
  models: 'viewer',
  system: 'viewer',
  userprofile: 'viewer',
  mymemory: 'viewer',
  envvars: 'viewer',
  tenant: 'viewer',
  members: 'viewer',
  ollama: 'admin',
  weknoracloud: 'admin',
  websearch: 'admin',
  chathistory: 'admin',
  memory: 'admin',
  vectorstore: 'admin',
  parser: 'admin',
  storage: 'admin',
  sandbox: 'admin',
  skills: 'admin',
  mcp: 'admin',
  'integration-api': 'admin',
  'integration-cli': 'admin',
  'integration-claw': 'admin',
  'integration-im': 'admin',
};

const systemAdminTabs = new Set<SettingsTab>([
  'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log',
]);

const tabOrder: SettingsTab[] = [
  'general', 'ollama', 'weknoracloud', 'models', 'websearch', 'chathistory', 'memory',
  'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'mcp', 'system', 'system-global',
  'runtime-queues', 'platform-api-keys', 'system-audit-log', 'userprofile', 'mymemory',
  'envvars', 'tenant', 'members', 'integration-api', 'integration-cli', 'integration-claw',
  'integration-im',
];

const integrationTabs = new Set<SettingsTab>([
  'integration-api', 'integration-cli', 'integration-claw', 'integration-im',
]);

export function normalizeSettingsTab(section: string | null | undefined, tab?: string | null): SettingsTab {
  const candidate = section === 'integrations' ? `integration-${tab || 'im'}` : section;
  const aliases: Record<string, SettingsTab> = {
    api: 'integration-api',
    cli: 'integration-cli',
    claw: 'integration-claw',
    im: 'integration-im',
  };
  const normalized = candidate ? aliases[candidate] ?? candidate : 'general';
  return tabOrder.includes(normalized as SettingsTab) ? normalized as SettingsTab : 'general';
}

export function canViewSection(section: string, role: SettingsRole): boolean {
  const tab = normalizeSettingsTab(section);
  if (systemAdminTabs.has(tab)) return role === 'systemAdmin';
  const required = minimumRole[tab] ?? (integrationTabs.has(tab) ? 'admin' : 'viewer');
  return roleRank[role] >= roleRank[required];
}

export function getVisibleSettingsTabs(role: SettingsRole): SettingsTab[] {
  return tabOrder.filter((tab) => canViewSection(tab, role));
}

export interface SettingsFormValues {
  name: string;
  provider: string;
}

export interface SettingsFormErrors {
  name?: string;
  provider?: string;
}

export function validateSettingsForm(values: SettingsFormValues): SettingsFormErrors {
  const errors: SettingsFormErrors = {};
  if (!values.name.trim()) errors.name = 'Name is required';
  if (!values.provider.trim()) errors.provider = 'Provider is required';
  return errors;
}

export type SettingsLoadState<T> =
  | { status: 'loading' }
  | { status: 'ready'; value: T }
  | { status: 'error'; message: string };

export type SettingsSaveState =
  | { status: 'idle' }
  | { status: 'saving' }
  | { status: 'saved' }
  | { status: 'error'; message: string };

export function saveSettingsOnce<T>(
  save: () => Promise<T>,
  inFlight: { current: Promise<T> | null },
): Promise<T> {
  if (inFlight.current) return inFlight.current;
  const request = save().finally(() => { inFlight.current = null; });
  inFlight.current = request;
  return request;
}
