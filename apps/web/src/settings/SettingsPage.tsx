import { isCapabilitySupported, type CapabilityMap } from '@weknora/domain';
import { integrationTabForSection, integrationSettingsQuery, selectSettingsQuery, INTEGRATION_SECTIONS } from '@weknora/views';
import { IntegrationsRoutePage } from '../integrations/IntegrationsRoutePage.tsx';
import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { SettingsRole } from '@weknora/views';
import { roleAtLeast, SETTINGS_SECTIONS, settingsSectionsForRole } from '@weknora/views';
import { Button, Status } from '@weknora/ui';
import { profilePasswordPatch, settingsSectionMeta, settingsValueEntries, tenantEditState, tenantPatch } from './surface.ts';
import { TenantDeleteZone } from './TenantDeleteZone.tsx';
import { MemoryWorkspacePanel, PersonalMemoryPanel, PersonalMemorySettingsPanel } from './PersonalMemoryPanel.tsx';
import { ResourceSettingsPanel } from './ResourceSettingsPanel.tsx';
import { ConfigSettingsPanel, type SettingsModelOption } from './ConfigSettingsPanel.tsx';
import { OllamaSettingsPanel } from './OllamaSettingsPanel.tsx';
import { CloudSettingsPanel } from './CloudSettingsPanel.tsx';
import { EnvVarSettingsPanel } from './EnvVarSettingsPanel.tsx';
import { LiveSectionsPanel, PortedSectionsPanel, readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { McpSettingsPanel } from './McpSettingsPanel.tsx';
import { ModelSettingsPanel } from './ModelSettingsPanel.tsx';
import { SandboxSettingsPanel } from './SandboxSettingsPanel.tsx';
import { SkillSettingsPanel } from './SkillSettingsPanel.tsx';
import { TenantMembersPanel } from './TenantMembersPanel.tsx';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

const PARTIALLY_PORTED_SECTIONS = new Set(['models', 'members', 'mcp', 'sandbox', 'skills', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log']);

export async function readSettingsSection(client: WeKnoraClient, key: string, tenantId: number): Promise<unknown> {
  switch (key) {
    case 'general': return client.settings.preferences.get();
    case 'tenant': return client.settings.tenant.get();
    case 'userprofile': return client.settings.profile.get();
    case 'ollama': return Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]).then(([status, models]) => ({ status, models }));
    case 'parser': return Promise.all([client.settings.parser.engines(), client.settings.parser.config.get()]).then(([engines, config]) => ({ engines, config }));
    case 'retrieval': return client.settings.retrieval.get();
    case 'memory': return Promise.all([client.settings.memory.workspace.get(), client.settings.memory.personal.settings()]).then(([workspace, personal]) => ({ workspace, personal }));
    case 'mymemory': return client.settings.memory.personal.items.list({ limit: 50 });
    case 'envvars': return client.settings.envVars.list();
    case 'storage': return Promise.all([client.settings.storage.backends.list(), client.settings.storage.legacy.status()]).then(([backends, legacy]) => ({ backends, legacy }));
    case 'vectorstore': return client.settings.vectorStores.list();
    case 'websearch': return client.settings.webSearch.providers.list();
    case 'chathistory': return Promise.all([client.settings.chatHistory.config.get(), client.settings.chatHistory.stats()]).then(([config, stats]) => ({ config, stats }));
    case 'system': return client.settings.system.info();
    case 'weknoracloud': return client.settings.weknoraCloud.status();
    case 'models': return client.configuration.models.list();
    case 'members': return client.identity.tenants.members.list(tenantId, { pageSize: 50 });
    case 'mcp': return client.configuration.mcp.list();
    case 'skills': return client.configuration.skills.list();
    case 'sandbox': return client.sandboxConfigurations.list();
    case 'system-global': return client.administration.settings.list();
    case 'runtime-queues': return client.administration.runtime.queues();
    case 'platform-api-keys': return client.administration.apiKeys.list();
    case 'system-audit-log': return client.administration.auditLog.list({ limit: 50 });
    default: throw new Error(`No read operation is registered for settings section: ${key}`);
  }
}

function requestedSection(search: string): string {
  const requested = integrationSettingsQuery(search).get('section');
  return requested && settingsSectionMeta(requested) ? requested : SETTINGS_SECTIONS[0]!.key;
}

function sectionTitleFor(locale: Locale, key: string, fallback: string): string {
  const titleKey = SECTION_TITLE_KEYS[key];
  return titleKey ? formatMessage(locale, titleKey) : fallback;
}

export function SettingsPage({ client, tenantId, role = 'owner', capabilities = {}, liteMode = false }: { client: WeKnoraClient; tenantId: number; role?: SettingsRole; capabilities?: CapabilityMap; liteMode?: boolean }) {
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const [selectedKey, setSelectedKey] = useState(() => requestedSection(window.location.search));
  const [payload, setPayload] = useState<unknown>(null);
  const [models, setModels] = useState<readonly SettingsModelOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tenantDraft, setTenantDraft] = useState({ name: '', description: '' });
  const [passwordDraft, setPasswordDraft] = useState({ oldPassword: '', newPassword: '', confirmation: '' });
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const section = settingsSectionMeta(selectedKey)!;
  const integrationSupported = (key: string) => { const capability = INTEGRATION_SECTIONS.find((item) => item.key === integrationTabForSection(key))?.capability; return !capability || isCapabilitySupported(capabilities, capability, { liteMode }); };
  const integrationTab = integrationTabForSection(selectedKey);
  const visibleSections = settingsSectionsForRole(role).filter((item) => integrationSupported(item.key));
  const roleDenied = !roleAtLeast(role, section.minRole);

  async function load() {
    if (integrationTab || roleDenied) { setPayload(null); setError(null); setLoading(false); return; }
    setLoading(true); setError(null); setNotice(null);
    try {
      const next = await readSettingsSection(client, selectedKey, tenantId); setPayload(next);
      if (selectedKey === 'tenant') setTenantDraft(tenantEditState(next));
      if (selectedKey === 'retrieval' || selectedKey === 'chathistory') {
        try { setModels(await client.configuration.models.list()); } catch { setModels([]); }
      }
    }
    catch (reason) { setPayload(null); setError(errorText(reason, `Unable to load ${section.title}`)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, [client, selectedKey, role]);
  useEffect(() => {
    if (!integrationTab) return;
    const next = integrationSupported(selectedKey) ? selectedKey : 'general';
    if (next !== selectedKey) { setSelectedKey(next); setNotice(t('settings.capabilityUnavailable')); }
    const query = selectSettingsQuery(next, window.location.search);
    window.history.replaceState(null, '', `/platform/settings?${query}`);
  }, [selectedKey, capabilities, liteMode]);
  useEffect(() => { void client.auth.registrationConfig().then((config) => setComplexPasswordEnabled(config.complexPasswordEnabled)).catch(() => setComplexPasswordEnabled(false)); }, [client]);

  useEffect(() => {
    function onPopState() { setSelectedKey(requestedSection(window.location.search)); }
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const select = useCallback((key: string) => {
    const next = settingsSectionMeta(key) ? key : SETTINGS_SECTIONS[0]!.key;
    setSelectedKey(next);
    window.history.pushState(null, '', `/platform/settings?${selectSettingsQuery(next, window.location.search)}`);
  }, []);

  async function saveTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try { const next = await client.settings.tenant.update(tenantId, tenantPatch(tenantDraft.name, tenantDraft.description)); setPayload(next); setTenantDraft(tenantEditState(next)); }
    catch (reason) { setError(errorText(reason, 'Unable to save tenant information; the server value was kept.')); }
    finally { setSaving(false); }
  }

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try {
      await client.settings.profile.changePassword(profilePasswordPatch(passwordDraft.oldPassword, passwordDraft.newPassword, passwordDraft.confirmation, { complexPasswordEnabled }));
      setPasswordDraft({ oldPassword: '', newPassword: '', confirmation: '' });
      setNotice('Password changed. Existing sessions may be signed out by the server.');
    } catch (reason) { setError(errorText(reason, 'Unable to change password; your current credentials were kept.')); }
    finally { setSaving(false); }
  }

  const resourcePanel = selectedKey === 'storage' || selectedKey === 'vectorstore' || selectedKey === 'websearch'
    ? <ResourceSettingsPanel client={client} section={selectedKey} initialValue={payload} />
    : null;
  const configPanel = selectedKey === 'retrieval'
    ? <ConfigSettingsPanel client={client} section="retrieval" initialValue={payload} models={models} />
    : selectedKey === 'chathistory'
      ? <ConfigSettingsPanel
          client={client}
          section="chathistory"
          initialValue={((payload as Record<string, unknown> | null)?.config)}
          models={models}
          embeddingLocked={((payload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined)?.has_indexed_messages === true}
          onSaved={() => void load()}
        />
      : selectedKey === 'parser'
        ? <ConfigSettingsPanel client={client} section="parser" initialValue={((payload as Record<string, unknown> | null)?.config)} />
        : null;
  const ollamaPanel = selectedKey === 'ollama' ? <OllamaSettingsPanel client={client} initialValue={payload} /> : null;
  const cloudPanel = selectedKey === 'weknoracloud' ? <CloudSettingsPanel client={client} initialValue={payload} /> : null;
  const envVarPanel = selectedKey === 'envvars' ? <EnvVarSettingsPanel client={client} initialPayload={payload} onMutated={() => void load()} /> : null;
  const mcpPanel = selectedKey === 'mcp'
    ? <McpSettingsPanel client={client} role={role} initialServices={Array.isArray(payload) ? payload as never : []} />
    : null;
  const modelPanel = selectedKey === 'models'
    ? <ModelSettingsPanel client={client} role={role} initialModels={Array.isArray(payload) ? payload as never : []} />
    : null;
  const sandboxPanel = selectedKey === 'sandbox'
    ? <SandboxSettingsPanel client={client} role={role} dockerBackendEnabled={isCapabilitySupported(capabilities, 'settings.sandbox.docker', { liteMode })} />
    : null;
  const skillPanel = selectedKey === 'skills'
    ? <SkillSettingsPanel client={client} role={role} initialSkills={Array.isArray(payload) ? payload as never : (((payload as { items?: unknown } | null)?.items ?? []) as never)} />
    : null;
  const membersPanel = selectedKey === 'members'
    ? <TenantMembersPanel client={client} tenantId={tenantId} role={role} initialMembers={payload as never} />
    : null;
  const portedPanel = selectedKey === 'mcp' ? mcpPanel : selectedKey === 'models' ? modelPanel : selectedKey === 'sandbox' ? sandboxPanel : selectedKey === 'skills' ? skillPanel : selectedKey === 'members' ? membersPanel : PARTIALLY_PORTED_SECTIONS.has(selectedKey)
    ? (selectedKey === 'sandbox'
        ? <PortedSectionsPanel section={selectedKey} />
        : <LiveSectionsPanel client={client} section={selectedKey} payload={payload} />)
    : null;
  const deniedPanel = roleDenied
    ? <div data-testid="role-denied-panel"><Status tone="error">{t('settings.roleDenied.title')}</Status><p className="wk-muted">{t('settings.roleDenied.desc')}</p></div>
    : null;
  return (
    <main className="wk-settings-drawer-root">
      <div className="wks-overlay">
        <div className="wks-modal" role="dialog" aria-modal="true" aria-label={t('general.settings')}>
          <button
            type="button"
            className="wks-close"
            aria-label={t('general.close')}
            data-testid="settings-close"
            onClick={() => { window.location.assign('/platform/knowledge-bases'); }}
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
          <div className="wks-container">
            <nav aria-label="Settings sections" className="wks-sidebar">
              <div className="wks-sidebar-header"><h2 className="wks-sidebar-title">{t('general.settings')}</h2></div>
              <div className="wks-nav">
                {settingsNavGroups(locale, visibleSections.map((item) => item.key)).map((group) => (
                  <div key={group.key}>
                    <div className="wks-nav-group-title">{group.label}</div>
                    {group.items.map((item) => (
                      <button
                        key={item.key}
                        type="button"
                        className={`wks-nav-item${item.key === selectedKey ? ' is-active' : ''}`}
                        aria-current={item.key === selectedKey ? 'page' : undefined}
                        onClick={() => select(item.key)}
                      >
                        <span className="wks-nav-icon">{item.icon}</span>
                        <span className="wks-nav-label">{item.label}</span>
                      </button>
                    ))}
                  </div>
                ))}
              </div>
            </nav>
            <section className="wks-content" aria-live="polite">
              <div className="wks-content-wrapper">
                {integrationTab ? (deniedPanel ?? <IntegrationsRoutePage key={`${tenantId}:${integrationTab}`} client={client} tenantId={String(tenantId)} activeTab={integrationTab} embedded />) : <div className="wk-settings-section wks-section">
                  <div className="wk-settings-panel-heading">
                    <div><h2>{sectionTitleFor(locale, selectedKey, section.title)}</h2><p className="wk-muted">{section.description}</p></div>
                    <button type="button" className="wks-reload" onClick={() => void load()} disabled={loading}>{t('common.refresh')}</button>
                  </div>
                  {selectedKey === 'tenant' && role === 'owner' ? <TenantDeleteZone client={client} tenantId={tenantId} tenantName={tenantDraft.name || String(tenantId)} onDeleted={() => { window.location.assign('/login'); }} /> : null}
                  {deniedPanel ?? (error ? <Status tone="error">{error}</Status> : loading ? <Status>Loading from {section.apiDomain}…</Status> : <>{notice ? <Status tone="success">{notice}</Status> : null}{resourcePanel ?? configPanel ?? ollamaPanel ?? cloudPanel ?? envVarPanel ?? portedPanel ?? (selectedKey === 'tenant' ? <form className="wk-settings-editor" onSubmit={(event) => void saveTenant(event)}><label>Name<input required value={tenantDraft.name} onChange={(event) => setTenantDraft((current) => ({ ...current, name: event.target.value }))} /></label><label>Description<textarea rows={3} value={tenantDraft.description} onChange={(event) => setTenantDraft((current) => ({ ...current, description: event.target.value }))} /></label><Button type="submit" loading={saving}>Save tenant information</Button></form> : selectedKey === 'userprofile' ? <form className="wk-settings-editor" onSubmit={(event) => void changePassword(event)}><p className="wk-muted">Profile identity fields are server-owned. Change your password only after entering the current credential and confirming the new one.</p><label>Current password<input required type="password" autoComplete="current-password" value={passwordDraft.oldPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, oldPassword: event.target.value }))} /></label><label>New password<input required type="password" autoComplete="new-password" value={passwordDraft.newPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, newPassword: event.target.value }))} /></label><label>Confirm new password<input required type="password" autoComplete="new-password" value={passwordDraft.confirmation} onChange={(event) => setPasswordDraft((current) => ({ ...current, confirmation: event.target.value }))} /></label><Button type="submit" loading={saving}>Change password</Button></form> : selectedKey === 'memory' ? <div className="wk-settings-memory"><MemoryWorkspacePanel client={client} initialConfig={((payload as Record<string, unknown> | null)?.workspace)} /><PersonalMemorySettingsPanel client={client} initialSettings={((payload as Record<string, unknown> | null)?.personal)} /></div> : selectedKey === 'mymemory' ? <PersonalMemoryPanel client={client} initialItems={payload} /> : <p className="wk-settings-read-note">Read result received from the server. This inventory view does not turn unsupported save, reset, test, or delete operations into a generic editor.</p>)}<dl className="wk-settings-values">{settingsValueEntries(payload).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></>)}
                </div>}
              </div>
            </section>
          </div>
        </div>
      </div>
    </main>
  );
}

// BEGIN settings nav grouping + inline lucide-style icons (ported from
// frontend/src/views/settings/Settings.vue navGroups/navItems).
import type { ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';

// Ported from frontend/src/views/settings/Settings.vue navGroups (账户 / 空间 /
// 模型 / 发布集成 / 数据与扩展 / 系统管理 / 平台). Every React registry section
// must land in one of these groups; anything unknown falls into the fallback
// group at the bottom so role gating can still surface it.
// Localized section titles (settings.* keys exist in packages/i18n/src/settings.ts).
const SECTION_TITLE_KEYS: Record<string, string> = {
  general: 'general.title',
  userprofile: 'userProfile.title',
  memory: 'memoryWorkspaceSettings.title',
  tenant: 'tenant.title',
  members: 'tenantMember.title',
  chathistory: 'chatHistorySettings.title',
  models: 'settings.modelManagement',
  ollama: 'ollamaSettings.title',
  weknoracloud: 'settings.weknoraCloud.title',
  vectorstore: 'vectorStoreSettings.title',
  parser: 'settings.parser.title',
  websearch: 'webSearchSettings.title',
  mcp: 'settings.mcpService',
  system: 'system.title',
  skills: 'settings.skills.title',
};

const NAV_GROUP_DEFS: ReadonlyArray<{ key: string; labelKey: string; sections: readonly string[] }> = [
  { key: 'account', labelKey: 'settings.navGroups.account', sections: ['general', 'userprofile', 'mymemory', 'envvars'] },
  { key: 'workspace', labelKey: 'settings.navGroups.workspace', sections: ['tenant', 'members', 'chathistory', 'memory'] },
  { key: 'models_runtime', labelKey: 'settings.navGroups.modelsRuntime', sections: ['models', 'ollama', 'weknoracloud'] },
  { key: 'integrations', labelKey: 'integrations.title', sections: INTEGRATION_SECTIONS.map((item) => `integration-${item.key}`) },
  { key: 'data_extensions', labelKey: 'settings.navGroups.dataExtensions', sections: ['vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'websearch', 'mcp'] },
  { key: 'system_administration', labelKey: 'settings.navGroups.systemAdministration', sections: ['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log'] },
  { key: 'platform', labelKey: 'settings.navGroups.platform', sections: ['system'] },
];

// Vue label sources (Settings.vue navItems): every label is a settings.* i18n
// string auto-ported into packages/i18n/src/settings.ts; ollama/weknoracloud
// stay literal there too.
const SECTION_LABEL_KEYS: Record<string, string> = {
  general: 'general.title',
  userprofile: 'userProfile.title',
  mymemory: 'memorySettings.title',
  envvars: 'envVarSettings.title',
  tenant: 'settings.tenantInfo',
  members: 'tenantMember.title',
  chathistory: 'chatHistorySettings.title',
  memory: 'memoryWorkspaceSettings.title',
  models: 'settings.modelManagement',
  websearch: 'settings.webSearchConfig',
  vectorstore: 'settings.vectorStoreEngine',
  parser: 'settings.parserEngine',
  storage: 'settings.storageEngine',
  sandbox: 'settings.sandbox.title',
  skills: 'settings.skills.title',
  mcp: 'settings.mcpService',
  system: 'settings.versionInfo',
  'system-global': 'settings.system',
  'runtime-queues': 'settings.taskQueue',
  'platform-api-keys': 'platformApiKeys.title',
  'system-audit-log': 'system.globalSettings.audit.tabLabel',
};

export interface SettingsNavItem {
  readonly key: string;
  readonly label: string;
  readonly icon: ReactNode;
}

export interface SettingsNavGroupView {
  readonly key: string;
  readonly label: string;
  readonly items: readonly SettingsNavItem[];
}

export function settingsSectionLabel(locale: Locale, key: string): string {
  const integrationTab = integrationTabForSection(key);
  if (integrationTab) return formatMessage(locale, `integrations.tabs.${integrationTab}`);
  const labelKey = SECTION_LABEL_KEYS[key];
  if (labelKey) return formatMessage(locale, labelKey);
  return settingsSectionMeta(key)?.title ?? key;
}

// Inline lucide-style stroke icons (24x24 viewBox), mirroring the t-icon names
// used by Settings.vue (setting / user / usergroup / key / chat / server / …).
function icon(paths: ReactNode): ReactNode {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths}
    </svg>
  );
}

const SECTION_ICONS: Record<string, ReactNode> = {
  general: icon(<><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></>),
  userprofile: icon(<><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></>),
  mymemory: icon(<path d="M19 21l-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z" />),
  envvars: icon(<><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" /></>),
  tenant: icon(<><rect x="4" y="3" width="16" height="18" rx="2" /><path d="M9 8h.01M15 8h.01M9 12h.01M15 12h.01M9 16h.01M15 16h.01" /></>),
  members: icon(<><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>),
  chathistory: icon(<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />),
  memory: icon(<><circle cx="5" cy="6" r="1.4" /><circle cx="5" cy="12" r="1.4" /><circle cx="5" cy="18" r="1.4" /><path d="M9 6h11M9 12h11M9 18h11" /></>),
  models: icon(<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9.5" y="9.5" width="5" height="5" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>),
  ollama: icon(<><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></>),
  weknoracloud: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <rect x="1.5" y="1.5" width="15" height="15" rx="3.5" stroke="currentColor" strokeWidth="1.2" />
      <path d="M4.5 5.5L6.5 12.5L9 7.5L11.5 12.5L13.5 5.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  vectorstore: icon(<><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" /><path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" /></>),
  parser: icon(<><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /><circle cx="11" cy="13" r="2.5" /><path d="M13 15l2.5 2.5" /></>),
  storage: icon(<path d="M17.5 19a4.5 4.5 0 0 0 .38-8.98 7 7 0 0 0-13.76 1.86A4 4 0 0 0 6 19z" />),
  sandbox: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <rect x="2.5" y="3" width="13" height="12" rx="2" stroke="currentColor" strokeWidth="1.2" />
      <path d="M2.5 6.5h13" stroke="currentColor" strokeWidth="1.2" />
      <path d="M5.5 10h4M5.5 12.5h2.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  ),
  skills: icon(<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />),
  websearch: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.2" />
      <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" strokeWidth="1.2" />
      <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" strokeWidth="1.2" />
      <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
      <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  ),
  mcp: icon(<><path d="M14.7 6.3a4.5 4.5 0 0 0 6 6l-7.4 7.4a2.1 2.1 0 0 1-3-3z" /><path d="M14.7 6.3l3-3 3 3-3 3" /></>),
  system: icon(<><circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" /></>),
  'system-global': icon(<><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></>),
  'runtime-queues': icon(<><path d="M8 6h13M8 12h13M8 18h13" /><path d="M3 6h.01M3 12h.01M3 18h.01" /></>),
  'platform-api-keys': icon(<><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="M9.5 12l2 2 3.5-3.5" /></>),
  'system-audit-log': icon(<><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /><path d="M12 7v5l3 3" /></>),
};

const FALLBACK_ICON = icon(<circle cx="12" cy="12" r="9" />);

export function settingsNavGroups(locale: Locale, visibleKeys: readonly string[]): SettingsNavGroupView[] {
  const labels = new Map(visibleKeys.map((key) => [key, settingsSectionLabel(locale, key)] as const));
  const groups: SettingsNavGroupView[] = NAV_GROUP_DEFS.map((def) => ({
    key: def.key,
    label: formatMessage(locale, def.labelKey),
    items: def.sections
      .filter((key) => labels.has(key))
      .map((key) => ({ key, label: labels.get(key)!, icon: SECTION_ICONS[key] ?? FALLBACK_ICON })),
  })).filter((group) => group.items.length > 0);
  // Unknown registry sections keep a fallback group so role-gated entries are
  // never silently dropped from the drawer navigation.
  const assigned = new Set(NAV_GROUP_DEFS.flatMap((def) => def.sections as readonly string[]));
  const unknown = visibleKeys.filter((key) => !assigned.has(key));
  if (unknown.length > 0) {
    groups.push({ key: 'other', label: formatMessage(locale, 'settings.navGroups.platform'), items: unknown.map((key) => ({ key, label: labels.get(key)!, icon: SECTION_ICONS[key] ?? FALLBACK_ICON })) });
  }
  return groups;
}
