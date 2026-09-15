export type SettingsRole = 'viewer' | 'contributor' | 'admin' | 'owner' | 'systemAdmin' | 'system-admin';

export type SettingsTab =
  | 'general' | 'ollama' | 'weknoracloud' | 'models' | 'websearch' | 'chathistory' | 'memory'
  | 'vectorstore' | 'parser' | 'storage' | 'sandbox' | 'skills' | 'mcp' | 'system'
  | 'system-global' | 'runtime-queues' | 'platform-api-keys' | 'system-audit-log'
  | 'userprofile' | 'mymemory' | 'envvars' | 'tenant' | 'members'
  | 'integration-api' | 'integration-cli' | 'integration-claw' | 'integration-im'
  | 'integration-embed' | 'integration-chrome';

const roleRank: Record<SettingsRole, number> = {
  viewer: 0,
  contributor: 1,
  admin: 2,
  owner: 3,
  systemAdmin: 4,
  'system-admin': 4,
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
  'integration-api': 'owner',
};

const systemAdminTabs = new Set<SettingsTab>([
  'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log',
]);

const tabOrder: SettingsTab[] = [
  'general', 'ollama', 'weknoracloud', 'models', 'websearch', 'chathistory', 'memory',
  'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'mcp', 'system', 'system-global',
  'runtime-queues', 'platform-api-keys', 'system-audit-log', 'userprofile', 'mymemory',
  'envvars', 'tenant', 'members', 'integration-im', 'integration-embed', 'integration-api',
  'integration-cli', 'integration-chrome', 'integration-claw',
];

export function normalizeSettingsTab(section: string | null | undefined, tab?: string | null): SettingsTab {
  const candidate = section === 'integrations' ? `integration-${tab || 'im'}` : section;
  const aliases: Record<string, SettingsTab> = {
    api: 'integration-api',
    cli: 'integration-cli',
    claw: 'integration-claw',
    im: 'integration-im',
    embed: 'integration-embed',
    chrome: 'integration-chrome',
  };
  const normalized = candidate ? aliases[candidate] ?? candidate : 'general';
  return tabOrder.includes(normalized as SettingsTab) ? normalized as SettingsTab : 'general';
}

export function canViewSection(section: string, role: SettingsRole): boolean {
  const tab = normalizeSettingsTab(section);
  if (systemAdminTabs.has(tab)) return role === 'systemAdmin' || role === 'system-admin';
  const required = minimumRole[tab] ?? 'viewer';
  return roleRank[role] >= roleRank[required];
}

export function getVisibleSettingsTabs(role: SettingsRole): SettingsTab[] {
  return tabOrder.filter((tab) => canViewSection(tab, role));
}

export interface SettingsNavGroup {
  readonly key: 'account' | 'workspace' | 'models_runtime' | 'integrations' | 'data_extensions' | 'system_administration' | 'platform';
  readonly items: SettingsTab[];
}

const navGroupItems: Record<SettingsNavGroup['key'], SettingsTab[]> = {
  account: ['general', 'userprofile', 'mymemory', 'envvars'],
  workspace: ['tenant', 'members', 'chathistory', 'memory'],
  models_runtime: ['models', 'ollama', 'weknoracloud'],
  integrations: ['integration-im', 'integration-embed', 'integration-api', 'integration-cli', 'integration-chrome', 'integration-claw'],
  data_extensions: ['vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'websearch', 'mcp'],
  system_administration: ['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log'],
  platform: ['system'],
};

const navGroupOrder: SettingsNavGroup['key'][] = [
  'account', 'workspace', 'models_runtime', 'integrations', 'data_extensions', 'system_administration', 'platform',
];

export function getSettingsNavGroups(role: SettingsRole): SettingsNavGroup[] {
  const visible = new Set(getVisibleSettingsTabs(role));
  return navGroupOrder
    .map((key) => ({ key, items: navGroupItems[key].filter((item) => visible.has(item)) }))
    .filter((group) => group.items.length > 0);
}

export type SettingsCloseMode = 'history' | 'knowledge-bases';

export function settingsCloseMode(section: string | null | undefined): SettingsCloseMode {
  return section && systemAdminTabs.has(normalizeSettingsTab(section)) ? 'knowledge-bases' : 'history';
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
